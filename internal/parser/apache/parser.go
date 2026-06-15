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
package apache

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// Apache combined:       %h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"
// Apache common:         %h %l %u %t "%r" %>s %b
// Apache vhost_combined: %v:%p %h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"
//
// vhost_combined is distinguished by the leading "hostname:port" token.
// We detect it by requiring :\d+ at the end of the first field.

const (
	// vhost_combined: vhost:port IP - user [time] "M P H/v" status bytes "ref" "ua"
	// Groups: 1=vhost:port, 2=IP, 3=user, 4=time, 5=method, 6=path, 7=ver, 8=status, 9=size, 10=ref, 11=ua
	vhostCombinedPattern = `^(\S+:\d+) (\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+) HTTP/([0-9.]+)" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"`

	// combined: IP - user [time] "M P H/v" status bytes "ref" "ua"
	// Groups: 1=IP, 2=user, 3=time, 4=method, 5=path, 6=ver, 7=status, 8=size, 9=ref, 10=ua
	combinedPattern = `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+) HTTP/([0-9.]+)" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"`

	// common: IP - user [time] "M P H/v" status bytes
	// Groups: 1=IP, 2=user, 3=time, 4=method, 5=path, 6=ver, 7=status, 8=size
	commonPattern = `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+) HTTP/([0-9.]+)" (\d{3}) (\d+|-)`
)

// Parser implements the LogParser interface for Apache HTTP Server access logs.
// Supports combined, common, and vhost_combined log formats (auto-detected).
type Parser struct {
	logger              *pterm.Logger
	vhostCombinedRegex  *regexp.Regexp
	combinedRegex       *regexp.Regexp
	commonRegex         *regexp.Regexp
}

func NewParser(logger *pterm.Logger) *Parser {
	return &Parser{
		logger:             logger,
		vhostCombinedRegex: regexp.MustCompile(vhostCombinedPattern),
		combinedRegex:      regexp.MustCompile(combinedPattern),
		commonRegex:        regexp.MustCompile(commonPattern),
	}
}

func (p *Parser) Name() string { return "apache" }

func (p *Parser) CanParse(line string) bool {
	if line == "" {
		return false
	}
	return p.vhostCombinedRegex.MatchString(line) ||
		p.combinedRegex.MatchString(line) ||
		p.commonRegex.MatchString(line)
}

func (p *Parser) Parse(line string) (*AccessLogEvent, error) {
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	if m := p.vhostCombinedRegex.FindStringSubmatch(line); m != nil {
		return p.parseVhostCombined(m)
	}
	if m := p.combinedRegex.FindStringSubmatch(line); m != nil {
		return p.parseCombined(m)
	}
	if m := p.commonRegex.FindStringSubmatch(line); m != nil {
		return p.parseCommon(m)
	}
	return nil, fmt.Errorf("unrecognized apache log format")
}

// parseVhostCombined parses vhost_combined format.
// Groups: 1=vhost:port, 2=IP, 3=user, 4=time, 5=method, 6=path, 7=ver, 8=status, 9=size, 10=ref, 11=ua
func (p *Parser) parseVhostCombined(m []string) (*AccessLogEvent, error) {
	if len(m) < 12 {
		return nil, fmt.Errorf("insufficient fields in vhost_combined")
	}

	vhostPort := m[1] // e.g. "mysite.com:443"
	host := vhostPort
	if idx := strings.LastIndex(vhostPort, ":"); idx != -1 {
		host = vhostPort[:idx]
	}

	ts := parseCLFTime(m[4])
	ip, port := splitHostPort(m[2])
	statusCode, _ := strconv.Atoi(m[8])
	responseSize := parseSize(m[9])
	path, queryString := splitPath(m[6])

	return &AccessLogEvent{
		Timestamp:    ts,
		ClientIP:     ip,
		ClientPort:   port,
		Method:       strings.ToUpper(m[5]),
		Protocol:     "HTTP/" + m[7],
		Host:         host,
		Path:         path,
		QueryString:  queryString,
		StatusCode:   statusCode,
		ResponseSize: responseSize,
		StartUTC:     ts.Format(time.RFC3339Nano),
		Referer:      clean(m[10]),
		UserAgent:    clean(m[11]),
	}, nil
}

// parseCombined parses combined format.
// Groups: 1=IP, 2=user, 3=time, 4=method, 5=path, 6=ver, 7=status, 8=size, 9=ref, 10=ua
func (p *Parser) parseCombined(m []string) (*AccessLogEvent, error) {
	if len(m) < 11 {
		return nil, fmt.Errorf("insufficient fields in combined")
	}

	ts := parseCLFTime(m[3])
	ip, port := splitHostPort(m[1])
	statusCode, _ := strconv.Atoi(m[7])
	responseSize := parseSize(m[8])
	path, queryString := splitPath(m[5])

	return &AccessLogEvent{
		Timestamp:    ts,
		ClientIP:     ip,
		ClientPort:   port,
		Method:       strings.ToUpper(m[4]),
		Protocol:     "HTTP/" + m[6],
		Path:         path,
		QueryString:  queryString,
		StatusCode:   statusCode,
		ResponseSize: responseSize,
		StartUTC:     ts.Format(time.RFC3339Nano),
		Referer:      clean(m[9]),
		UserAgent:    clean(m[10]),
	}, nil
}

// parseCommon parses common format.
// Groups: 1=IP, 2=user, 3=time, 4=method, 5=path, 6=ver, 7=status, 8=size
func (p *Parser) parseCommon(m []string) (*AccessLogEvent, error) {
	if len(m) < 9 {
		return nil, fmt.Errorf("insufficient fields in common")
	}

	ts := parseCLFTime(m[3])
	ip, port := splitHostPort(m[1])
	statusCode, _ := strconv.Atoi(m[7])
	responseSize := parseSize(m[8])
	path, queryString := splitPath(m[5])

	return &AccessLogEvent{
		Timestamp:    ts,
		ClientIP:     ip,
		ClientPort:   port,
		Method:       strings.ToUpper(m[4]),
		Protocol:     "HTTP/" + m[6],
		Path:         path,
		QueryString:  queryString,
		StatusCode:   statusCode,
		ResponseSize: responseSize,
		StartUTC:     ts.Format(time.RFC3339Nano),
	}, nil
}

func parseCLFTime(s string) time.Time {
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", s)
	if err != nil {
		return time.Now()
	}
	return t
}

func splitHostPort(s string) (string, int) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return s, 0
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

func splitPath(raw string) (path, query string) {
	if idx := strings.Index(raw, "?"); idx != -1 {
		return raw[:idx], raw[idx+1:]
	}
	return raw, ""
}

func parseSize(s string) int64 {
	if s == "-" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func clean(s string) string {
	if s == "-" {
		return ""
	}
	return s
}
