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
//
package traefik

import (
	"testing"
	"time"

	"github.com/pterm/pterm"
)

func TestParser_CanParse_JSON(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	jsonLog := `{"ClientHost":"103.4.250.66","DownstreamStatus":200,"Duration":299425702,"RequestMethod":"GET","RequestPath":"/","RequestProtocol":"HTTP/1.1","ServiceName":"next-service@file","TLSCipher":"TLS_AES_128_GCM_SHA256","TLSVersion":"1.3","request_User-Agent":"Mozilla/5.0","request_X-Real-Ip":"103.4.250.66","time":"2025-10-25T21:11:49Z"}`

	if !parser.CanParse(jsonLog) {
		t.Error("Expected parser to accept JSON log format")
	}
}

func TestParser_CanParse_TraefikCLF(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	clfLog := `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "GET /api/endpoint HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0" 42 "my-router" "http://backend:8080" 150ms`

	if !parser.CanParse(clfLog) {
		t.Error("Expected parser to accept Traefik CLF format")
	}
}

func TestParser_CanParse_GenericCLF(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	genericCLF := `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "GET /api/endpoint HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0"`

	if !parser.CanParse(genericCLF) {
		t.Error("Expected parser to accept generic CLF format")
	}
}

func TestParser_CanParse_Invalid(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	tests := []string{
		"",
		"invalid log line",
		"192.168.1.1 - just some random text",
	}

	for _, tc := range tests {
		if parser.CanParse(tc) {
			t.Errorf("Expected parser to reject invalid log: %q", tc)
		}
	}
}

func TestParser_ParseJSON(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	jsonLog := `{"ClientHost":"103.4.250.66","DownstreamContentSize":31869,"DownstreamStatus":200,"Duration":299425702,"RequestMethod":"GET","RequestPath":"/test?redirect=https://example.com","RequestProtocol":"HTTP/1.1","ServiceName":"next-service@file","TLSCipher":"TLS_AES_128_GCM_SHA256","TLSVersion":"1.3","request_User-Agent":"Mozilla/5.0 (Test)","request_Referer":"https://referrer.com","request_X-Real-Ip":"103.4.250.66","time":"2025-10-25T21:11:49Z"}`

	event, err := parser.Parse(jsonLog)
	if err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify key fields
	if event.ClientIP != "103.4.250.66" {
		t.Errorf("Expected ClientIP '103.4.250.66', got '%s'", event.ClientIP)
	}
	if event.Method != "GET" {
		t.Errorf("Expected Method 'GET', got '%s'", event.Method)
	}
	if event.Path != "/test" {
		t.Errorf("Expected Path '/test', got '%s'", event.Path)
	}
	if event.QueryString != "redirect=https://example.com" {
		t.Errorf("Expected QueryString 'redirect=https://example.com', got '%s'", event.QueryString)
	}
	if event.StatusCode != 200 {
		t.Errorf("Expected StatusCode 200, got %d", event.StatusCode)
	}
	if event.ResponseSize != 31869 {
		t.Errorf("Expected ResponseSize 31869, got %d", event.ResponseSize)
	}
	if event.TLSVersion != "1.3" {
		t.Errorf("Expected TLSVersion '1.3', got '%s'", event.TLSVersion)
	}
	if event.UserAgent != "Mozilla/5.0 (Test)" {
		t.Errorf("Expected UserAgent 'Mozilla/5.0 (Test)', got '%s'", event.UserAgent)
	}
}

func TestParser_ParseTraefikCLF(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	clfLog := `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "GET /api/endpoint HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0" 42 "my-router" "http://backend:8080" 150ms`

	event, err := parser.Parse(clfLog)
	if err != nil {
		t.Fatalf("Failed to parse Traefik CLF log: %v", err)
	}

	// Verify key fields
	if event.ClientIP != "192.168.1.100" {
		t.Errorf("Expected ClientIP '192.168.1.100', got '%s'", event.ClientIP)
	}
	if event.Method != "GET" {
		t.Errorf("Expected Method 'GET', got '%s'", event.Method)
	}
	if event.Path != "/api/endpoint" {
		t.Errorf("Expected Path '/api/endpoint', got '%s'", event.Path)
	}
	if event.StatusCode != 200 {
		t.Errorf("Expected StatusCode 200, got %d", event.StatusCode)
	}
	if event.ResponseSize != 1024 {
		t.Errorf("Expected ResponseSize 1024, got %d", event.ResponseSize)
	}
	if event.ResponseTimeMs != 150 {
		t.Errorf("Expected ResponseTimeMs 150, got %f", event.ResponseTimeMs)
	}
	if event.BackendName != "my-router" {
		t.Errorf("Expected RouterName 'my-router', got '%s'", event.RouterName)
	}
	if event.BackendURL != "http://backend:8080" {
		t.Errorf("Expected BackendURL 'http://backend:8080', got '%s'", event.BackendURL)
	}
	if event.UserAgent != "Mozilla/5.0" {
		t.Errorf("Expected UserAgent 'Mozilla/5.0', got '%s'", event.UserAgent)
	}

	// Check timestamp parsing
	expectedTime, _ := time.Parse("02/Jan/2006:15:04:05 -0700", "15/May/2025:12:06:30 +0000")
	if !event.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected Timestamp %v, got %v", expectedTime, event.Timestamp)
	}
}

func TestParser_ParseGenericCLF(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	genericCLF := `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "POST /api/users HTTP/1.1" 201 512 "https://app.example.com" "curl/7.68.0"`

	event, err := parser.Parse(genericCLF)
	if err != nil {
		t.Fatalf("Failed to parse generic CLF log: %v", err)
	}

	// Verify key fields
	if event.ClientIP != "192.168.1.100" {
		t.Errorf("Expected ClientIP '192.168.1.100', got '%s'", event.ClientIP)
	}
	if event.Method != "POST" {
		t.Errorf("Expected Method 'POST', got '%s'", event.Method)
	}
	if event.Path != "/api/users" {
		t.Errorf("Expected Path '/api/users', got '%s'", event.Path)
	}
	if event.StatusCode != 201 {
		t.Errorf("Expected StatusCode 201, got %d", event.StatusCode)
	}
	if event.ResponseSize != 512 {
		t.Errorf("Expected ResponseSize 512, got %d", event.ResponseSize)
	}
	if event.UserAgent != "curl/7.68.0" {
		t.Errorf("Expected UserAgent 'curl/7.68.0', got '%s'", event.UserAgent)
	}
	// Generic CLF doesn't have duration
	if event.ResponseTimeMs != 0 {
		t.Errorf("Expected ResponseTimeMs 0, got %f", event.ResponseTimeMs)
	}
}

func TestParser_DetectFormat(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	tests := []struct {
		name     string
		line     string
		expected LogFormat
	}{
		{
			name:     "JSON format",
			line:     `{"time":"2025-10-25T21:11:49Z","request_X-Real-Ip":"103.4.250.66"}`,
			expected: FormatJSON,
		},
		{
			name:     "Traefik CLF format",
			line:     `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "GET /api HTTP/1.1" 200 1024 "-" "Mozilla" 42 "router" "http://backend" 150ms`,
			expected: FormatCLF,
		},
		{
			name:     "Generic CLF format",
			line:     `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "GET /api HTTP/1.1" 200 1024 "-" "Mozilla"`,
			expected: FormatCLF,
		},
		{
			name:     "Unknown format",
			line:     `invalid log line`,
			expected: FormatUnknown,
		},
		{
			name:     "Empty line",
			line:     "",
			expected: FormatUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			format := parser.detectFormat(tc.line)
			if format != tc.expected {
				t.Errorf("Expected format %d, got %d", tc.expected, format)
			}
		})
	}
}

func TestParser_ParseCLFWithDashValues(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	// Test with "-" values for referer and user agent
	clfLog := `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "GET /api HTTP/1.1" 200 - "-" "-" 42 "-" "-" 150ms`

	event, err := parser.Parse(clfLog)
	if err != nil {
		t.Fatalf("Failed to parse CLF log with dash values: %v", err)
	}

	// Verify "-" values are converted to empty strings
	if event.Referer != "" {
		t.Errorf("Expected empty Referer, got '%s'", event.Referer)
	}
	if event.UserAgent != "" {
		t.Errorf("Expected empty UserAgent, got '%s'", event.UserAgent)
	}
	if event.RouterName != "" {
		t.Errorf("Expected empty RouterName, got '%s'", event.RouterName)
	}
	if event.BackendURL != "" {
		t.Errorf("Expected empty BackendURL, got '%s'", event.BackendURL)
	}
	if event.ResponseSize != 0 {
		t.Errorf("Expected ResponseSize 0, got %d", event.ResponseSize)
	}
}

func TestParser_ParseCLFWithQueryString(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	clfLog := `192.168.1.100 - - [15/May/2025:12:06:30 +0000] "GET /api/search?q=test&limit=10 HTTP/1.1" 200 1024 "-" "Mozilla" 42 "router" "http://backend" 150ms`

	event, err := parser.Parse(clfLog)
	if err != nil {
		t.Fatalf("Failed to parse CLF log with query string: %v", err)
	}

	if event.Path != "/api/search" {
		t.Errorf("Expected Path '/api/search', got '%s'", event.Path)
	}
	if event.QueryString != "q=test&limit=10" {
		t.Errorf("Expected QueryString 'q=test&limit=10', got '%s'", event.QueryString)
	}
}

