package caddy

import "time"

// CaddyRequestEvent represents a parsed Caddy access log entry.
// This struct maps Caddy's JSON log format to LogLynx's HTTPRequest model.
type CaddyRequestEvent struct {
	// Core fields
	Timestamp  time.Time
	SourceName string

	// Client info
	ClientIP   string
	ClientPort int
	ClientUser string

	// Request info
	Method        string
	Protocol      string
	Host          string
	Path          string
	QueryString   string
	RequestLength int64
	RequestScheme string

	// Response info
	StatusCode          int
	ResponseSize        int64
	ResponseTimeMs      float64
	ResponseContentType string

	// Detailed timing
	Duration               int64   // Nanoseconds
	StartUTC               string  // RFC3339Nano for hash calculation
	UpstreamResponseTimeMs float64
	RetryAttempts          int
	RequestsTotal          int

	// Headers
	UserAgent string
	Referer   string

	// Proxy/Upstream info
	BackendName         string
	BackendURL          string
	RouterName          string
	UpstreamStatus      int
	UpstreamContentType string
	ClientHostname      string

	// TLS info
	TLSVersion    string
	TLSCipher     string
	TLSServerName string

	// Tracing
	RequestID string
	TraceID   string

	// GeoIP (populated later by enrichment)
	GeoCountry string
	GeoCity    string
	GeoLat     float64
	GeoLon     float64
	ASN        int
	ASNOrg     string

	// Extensibility - Caddy-specific data stored as JSON
	ProxyMetadata string
}

// GetTimestamp implements the parser.Event interface
func (e *CaddyRequestEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetSourceName implements the parser.Event interface
func (e *CaddyRequestEvent) GetSourceName() string {
	return e.SourceName
}
