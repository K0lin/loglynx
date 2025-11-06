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

	// Response info
	StatusCode     int
	ResponseSize   int64
	ResponseTimeMs float64

	// Detailed timing (for hash calculation precision)
	Duration       int64   // Duration in nanoseconds (Traefik's Duration field)
	StartUTC       string  // Start timestamp with nanosecond precision (Traefik's StartUTC field)
	UpstreamResponseTimeMs float64

	// Headers
	UserAgent      string
	Referer        string

	// Proxy/Upstream info
	BackendName    string
	BackendURL     string
	RouterName     string
	UpstreamStatus int

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