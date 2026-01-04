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
	"time"
)

// HTTPRequestEvent represents a complete Traefik HTTP request log entry
type HTTPRequestEvent struct {
	Timestamp      time.Time
	SourceName     string

	// Client info
	ClientIP       string
	ClientPort     int
	ClientUser     string

	// Request info
	Method         string
	Protocol       string
	Host           string
	Path           string
	QueryString    string
	RequestLength  int64
	RequestScheme  string // Request scheme: http, https (from request_X-Forwarded-Proto)

	// Response info
	StatusCode          int
	ResponseSize        int64
	ResponseTimeMs      float64
	ResponseContentType string // downstream_Content-Type

	// Detailed timing (for hash calculation precision)
	Duration       int64   // Duration in nanoseconds (Traefik's Duration field)
	StartUTC       string  // Start timestamp with nanosecond precision (Traefik's StartUTC field)
	UpstreamResponseTimeMs float64
	RetryAttempts  int     // Number of retry attempts
	RequestsTotal  int     // Total number of requests at router level (Traefik CLF field)

	// Headers
	UserAgent      string
	Referer        string

	// Proxy/Upstream info
	BackendName         string
	BackendURL          string
	RouterName          string
	UpstreamStatus      int
	UpstreamContentType string // origin_Content-Type
	ClientHostname      string // ClientHost field (may contain hostname)

	// TLS info
	TLSVersion     string
	TLSCipher      string
	TLSServerName  string

	// Tracing & IDs
	RequestID      string
	TraceID        string

	// GeoIP enrichment (populated later by enrichment layer)
	GeoCountry     string
	GeoCity        string
	GeoLat         float64
	GeoLon         float64
	ASN            int
	ASNOrg         string

	// Proxy-specific metadata
	ProxyMetadata  string
}

func (e *HTTPRequestEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

func (e *HTTPRequestEvent) GetSourceName() string {
	return e.SourceName
}
