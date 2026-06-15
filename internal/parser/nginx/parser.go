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
package nginx

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// nginx combined CLF: IP - user [time] "METHOD path HTTP/ver" status bytes "referer" "ua"
const combinedPattern = `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+) HTTP/([0-9.]+)" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"`

// nginx common CLF: IP - user [time] "METHOD path HTTP/ver" status bytes
const commonPattern = `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+) HTTP/([0-9.]+)" (\d{3}) (\d+|-)`

// Parser implements the LogParser interface for nginx access logs.
// Supports combined CLF, common CLF, and JSON log formats.
type Parser struct {
	logger        *pterm.Logger
	combinedRegex *regexp.Regexp
	commonRegex   *regexp.Regexp
}

func NewParser(logger *pterm.Logger) *Parser {
	return &Parser{
		logger:        logger,
		combinedRegex: regexp.MustCompile(combinedPattern),
		commonRegex:   regexp.MustCompile(commonPattern),
	}
}

func (p *Parser) Name() string { return "nginx" }

func (p *Parser) CanParse(line string) bool {
	if line == "" {
		return false
	}
	if line[0] == '{' {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err == nil {
			_, hasAddr := m["remote_addr"]
			return hasAddr
		}
		return false
	}
	return p.combinedRegex.MatchString(line) || p.commonRegex.MatchString(line)
}

func (p *Parser) Parse(line string) (*AccessLogEvent, error) {
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}
	if line[0] == '{' {
		return p.parseJSON(line)
	}
	if matches := p.combinedRegex.FindStringSubmatch(line); matches != nil {
		return p.parseCombined(matches)
	}
	if matches := p.commonRegex.FindStringSubmatch(line); matches != nil {
		return p.parseCommon(matches)
	}
	return nil, fmt.Errorf("unrecognized nginx log format")
}

func (p *Parser) parseJSON(line string) (*AccessLogEvent, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	clientIP := getStr(m, "remote_addr")
	if clientIP == "" {
		return nil, fmt.Errorf("missing remote_addr field")
	}

	ts := parseNginxTime(m)

	// Parse request line "METHOD /path HTTP/ver" or use individual fields
	method, path, protocol, queryString := splitRequest(getStr(m, "request"))
	if method == "" {
		method = firstNonEmpty(getStr(m, "method"), getStr(m, "request_method"))
	}
	if path == "" {
		rawPath := firstNonEmpty(getStr(m, "uri"), getStr(m, "request_uri"))
		if idx := strings.Index(rawPath, "?"); idx != -1 {
			queryString = rawPath[idx+1:]
			path = rawPath[:idx]
		} else {
			path = rawPath
		}
	}
	if protocol == "" {
		protocol = getStr(m, "server_protocol")
	}

	statusCode := getInt(m, "status")
	responseSize := firstInt64(getInt64(m, "body_bytes_sent"), getInt64(m, "bytes_sent"))

	// nginx logs request_time in seconds as float
	var responseTimeMs float64
	if rtStr := getStr(m, "request_time"); rtStr != "" {
		if rt, err := strconv.ParseFloat(rtStr, 64); err == nil {
			responseTimeMs = rt * 1000
		}
	} else {
		responseTimeMs = getFloat64(m, "request_time") * 1000
	}

	host := firstNonEmpty(getStr(m, "host"), getStr(m, "http_host"), getStr(m, "server_name"))
	userAgent := getStr(m, "http_user_agent")
	referer := getStr(m, "http_referer")
	if referer == "-" {
		referer = ""
	}

	ip, port := splitHostPort(clientIP)

	return &AccessLogEvent{
		Timestamp:      ts,
		ClientIP:       ip,
		ClientPort:     port,
		Method:         strings.ToUpper(method),
		Protocol:       protocol,
		Host:           host,
		Path:           path,
		QueryString:    queryString,
		StatusCode:     statusCode,
		ResponseSize:   responseSize,
		ResponseTimeMs: responseTimeMs,
		Duration:       int64(responseTimeMs * 1e6),
		StartUTC:       ts.Format(time.RFC3339Nano),
		UserAgent:      userAgent,
		Referer:        referer,
	}, nil
}

// parseCombined handles combined CLF: IP - user [time] "M P H/v" status bytes "ref" "ua"
// Capture groups: 1=IP, 2=user, 3=time, 4=method, 5=path, 6=proto_ver, 7=status, 8=size, 9=referer, 10=ua
func (p *Parser) parseCombined(m []string) (*AccessLogEvent, error) {
	if len(m) < 11 {
		return nil, fmt.Errorf("insufficient fields in combined CLF")
	}

	ts := parseCLFTime(m[3])
	ip, port := splitHostPort(m[1])
	statusCode, _ := strconv.Atoi(m[7])
	responseSize := parseSize(m[8])
	path, queryString := splitPath(m[5])
	referer := clean(m[9])
	userAgent := clean(m[10])

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
		UserAgent:    userAgent,
		Referer:      referer,
	}, nil
}

// parseCommon handles common CLF: IP - user [time] "M P H/v" status bytes
// Capture groups: 1=IP, 2=user, 3=time, 4=method, 5=path, 6=proto_ver, 7=status, 8=size
func (p *Parser) parseCommon(m []string) (*AccessLogEvent, error) {
	if len(m) < 9 {
		return nil, fmt.Errorf("insufficient fields in common CLF")
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

// parseNginxTime tries time_local (CLF format) then time (ISO 8601).
func parseNginxTime(m map[string]any) time.Time {
	if tl := getStr(m, "time_local"); tl != "" {
		if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", tl); err == nil {
			return t
		}
	}
	for _, key := range []string{"time", "timestamp", "time_iso8601"} {
		if ts := getStr(m, key); ts != "" {
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				return t
			}
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				return t
			}
		}
	}
	return time.Now()
}

func parseCLFTime(s string) time.Time {
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", s)
	if err != nil {
		return time.Now()
	}
	return t
}

func splitRequest(req string) (method, path, protocol, queryString string) {
	if req == "" || req == "-" {
		return
	}
	parts := strings.SplitN(req, " ", 3)
	if len(parts) < 2 {
		return
	}
	method = parts[0]
	rawPath := parts[1]
	if len(parts) == 3 {
		protocol = parts[2]
	}
	path, queryString = splitPath(rawPath)
	return
}

func splitPath(raw string) (path, query string) {
	if idx := strings.Index(raw, "?"); idx != -1 {
		return raw[:idx], raw[idx+1:]
	}
	return raw, ""
}

func splitHostPort(s string) (string, int) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return s, 0
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstInt64(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return 0
}

func getInt64(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func getFloat64(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}
