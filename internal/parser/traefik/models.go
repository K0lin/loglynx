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

	// Request info
	Method         string
	Protocol       string
	Host           string
	Path           string
	QueryString    string

	// Response info
	StatusCode     int
	ResponseSize   int64
	ResponseTimeMs float64

	// Headers
	UserAgent      string
	Referer        string

	// Traefik-specific
	BackendName    string
	BackendURL     string
	RouterName     string

	// TLS info
	TLSVersion     string
	TLSCipher      string

	// Tracing
	RequestID      string

	// GeoIP enrichment (populated later by enrichment layer)
	GeoCountry     string
	GeoCity        string
	GeoLat         float64
	GeoLon         float64
	ASN            int
	ASNOrg         string
}

func (e *HTTPRequestEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

func (e *HTTPRequestEvent) GetSourceName() string {
	return e.SourceName
}