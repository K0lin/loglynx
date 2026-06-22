// MIT License
//
// Copyright (c) 2026 Kolin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package haproxy

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// HAProxy HTTP log format (RFC 5424 syslog-based or plain):
//
// Plain:   client:port [accept_date] frontend backend/server Tq/Tw/Tc/Tr/Tt status bytes req_cookie res_cookie term actconn/feconn/beconn/srv/retries sq/bq "request"
// Syslog:  Month DD HH:MM:SS host process[pid]: <above>
//
// Reference: https://www.haproxy.com/documentation/haproxy/latest/observability/logs/log-formats/
const (
	// Syslog prefix: "Jan  1 12:34:56 host haproxy[123]: " or "Jan 01 12:34:56 host haproxy[123]: "
	syslogPrefixPattern = `^\w+ +\d+ +\d+:\d+:\d+ \S+ \S+\[\d+\]: `

	// Core HAProxy HTTP log fields.
	// Groups: 1=client:port, 2=date, 3=frontend, 4=backend/server, 5=Tq, 6=Tw, 7=Tc, 8=Tr, 9=Tt,
	//         10=status, 11=bytes, 12=req_cookie, 13=res_cookie, 14=term_flags,
	//         15=actconn, 16=feconn, 17=beconn, 18=srv_conn, 19=retries,
	//         20=srv_queue, 21=backend_queue, 22=request_line
	corePattern = `^(\S+) \[([^\]]+)\] (\S+) (\S+) (-?\d+)/(-?\d+)/(-?\d+)/(-?\d+)/(-?\d+) (\d{3}|-) (\d+|\-) (\S+) (\S+) (\S+) (\d+)/(\d+)/(\d+)/(\d+)/(\d+) (\d+)/(\d+) "([^"]*)"`
)

// Parser implements the LogParser interface for HAProxy HTTP access logs.
type Parser struct {
	logger        *pterm.Logger
	syslogPrefix  *regexp.Regexp
	coreRegex     *regexp.Regexp
}

func NewParser(logger *pterm.Logger) *Parser {
	return &Parser{
		logger:       logger,
		syslogPrefix: regexp.MustCompile(syslogPrefixPattern),
		coreRegex:    regexp.MustCompile(corePattern),
	}
}

func (p *Parser) Name() string { return "haproxy" }

func (p *Parser) CanParse(line string) bool {
	if line == "" {
		return false
	}
	return p.coreRegex.MatchString(p.stripSyslog(line))
}

func (p *Parser) Parse(line string) (*AccessLogEvent, error) {
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	core := p.stripSyslog(line)
	m := p.coreRegex.FindStringSubmatch(core)
	if m == nil {
		return nil, fmt.Errorf("line does not match HAProxy HTTP log format")
	}

	// m[1] = client:port
	clientAddr := m[1]
	ip, port := splitHostPort(clientAddr)

	// m[2] = accept date: "DD/Mon/YYYY:HH:MM:SS.mmm"
	ts := parseHAProxyTime(m[2])

	// m[3] = frontend name
	frontend := m[3]

	// m[4] = backend/server, e.g. "bk_app/srv1"
	backendServer := m[4]
	backendName, _ := splitBackend(backendServer)

	// Timers (ms): Tq/Tw/Tc/Tr/Tt
	// Tt (total time) is m[9]
	totalTimeMs, _ := strconv.ParseFloat(m[9], 64)

	statusCode, _ := strconv.Atoi(m[10])
	bytesRead, _ := strconv.ParseInt(m[11], 10, 64)

	// m[19] = retries
	retries, _ := strconv.Atoi(m[19])

	// m[22] = request line: "GET /path HTTP/1.1" or "<BADREQ>"
	requestLine := m[22]
	method, path, protocol, queryString := splitRequest(requestLine)

	durationNs := int64(totalTimeMs * 1e6)

	return &AccessLogEvent{
		Timestamp:      ts,
		ClientIP:       ip,
		ClientPort:     port,
		Method:         strings.ToUpper(method),
		Protocol:       protocol,
		Path:           path,
		QueryString:    queryString,
		StatusCode:     statusCode,
		ResponseSize:   bytesRead,
		ResponseTimeMs: totalTimeMs,
		Duration:       durationNs,
		StartUTC:       ts.Format(time.RFC3339Nano),
		BackendName:    backendName,
		RouterName:     frontend,
		RetryAttempts:  retries,
	}, nil
}

// stripSyslog removes the syslog prefix if present.
func (p *Parser) stripSyslog(line string) string {
	loc := p.syslogPrefix.FindStringIndex(line)
	if loc != nil {
		return line[loc[1]:]
	}
	return line
}

// parseHAProxyTime parses HAProxy accept date format: DD/Mon/YYYY:HH:MM:SS.mmm
func parseHAProxyTime(s string) time.Time {
	// Try with milliseconds
	if t, err := time.Parse("02/Jan/2006:15:04:05.000", s); err == nil {
		return t
	}
	// Try without milliseconds
	if t, err := time.Parse("02/Jan/2006:15:04:05", s); err == nil {
		return t
	}
	return time.Now()
}

// splitBackend splits "backend/server" into ("backend", "server").
func splitBackend(s string) (string, string) {
	if idx := strings.Index(s, "/"); idx != -1 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func splitRequest(req string) (method, path, protocol, queryString string) {
	if req == "" || req == "-" || req == "<BADREQ>" {
		return "UNKNOWN", "/", "", ""
	}
	parts := strings.SplitN(req, " ", 3)
	if len(parts) < 2 {
		return "UNKNOWN", req, "", ""
	}
	method = parts[0]
	rawPath := parts[1]
	if len(parts) == 3 {
		protocol = parts[2]
	}
	if idx := strings.Index(rawPath, "?"); idx != -1 {
		queryString = rawPath[idx+1:]
		path = rawPath[:idx]
	} else {
		path = rawPath
	}
	return
}

func splitHostPort(s string) (string, int) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return s, 0
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}
