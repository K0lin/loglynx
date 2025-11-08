package models

import (
	"time"

	"gorm.io/gorm"
)

type HTTPRequest struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	SourceName  string    `gorm:"type:varchar(255);not null"` // index created by OptimizeDatabase
	Timestamp   time.Time `gorm:"not null"`                   // index created by OptimizeDatabase
	RequestHash string    `gorm:"type:char(64);uniqueIndex:idx_request_hash"` // SHA256 hash for deduplication (UNIQUE index is essential)

	// Partition key for future scaling (YYYY-MM format)
	PartitionKey string `gorm:"type:varchar(7)"` // index created by OptimizeDatabase

	// Client info
	ClientIP   string `gorm:"type:varchar(45);not null"` // index created by OptimizeDatabase (IPv6 support)
	ClientPort int    `gorm:"check:client_port >= 0 AND client_port <= 65535"`
	ClientUser string `gorm:"type:varchar(255)"` // HTTP authentication user (NPM: remote_user)

	// Request info
	Method        string `gorm:"type:varchar(10);not null"` // GET, POST, PUT, DELETE, etc.
	Protocol      string `gorm:"type:varchar(10)"`          // HTTP/1.1, HTTP/2.0, HTTP/3.0
	Host          string `gorm:"type:varchar(255);not null"` // index created by OptimizeDatabase
	Path          string `gorm:"type:varchar(2048);not null"`                                                 // Paths can be long
	QueryString   string `gorm:"type:text"`                                                                   // Can be very long
	RequestLength int64  `gorm:"check:request_length >= 0"`                                                   // Request size in bytes (NPM/Caddy)
	RequestScheme string `gorm:"type:varchar(10);check:request_scheme IN ('http', 'https', 'ws', 'wss', '')"` // Request scheme: http, https, ws (WebSocket), wss (WebSocket Secure)

	// Response info
	// Allow 0 to represent unknown/invalid status codes produced by some parsers
	// Parser code may set StatusCode=0 for invalid values, so permit 0 in DB
	StatusCode          int     `gorm:"not null;check:status_code >= 0 AND status_code < 600"` // index created by OptimizeDatabase
	ResponseSize        int64   `gorm:"check:response_size >= 0"`
	ResponseTimeMs      float64 `gorm:"check:response_time_ms >= 0"` // index created by OptimizeDatabase - Total response time
	ResponseContentType string  `gorm:"type:varchar(255)"`           // downstream Content-Type

	// Detailed timing (optional, for advanced proxies)
	Duration               int64   `gorm:"check:duration >= 0"`              // Duration in nanoseconds (for precise hash calculation)
	StartUTC               string  `gorm:"type:varchar(35)"`                 // Start timestamp with nanosecond precision (RFC3339Nano format)
	UpstreamResponseTimeMs float64 `gorm:"check:upstream_response_time_ms >= 0"` // Time spent waiting for upstream/backend
	RetryAttempts          int     `gorm:"check:retry_attempts >= 0"`        // Number of retry attempts (Traefik) - index created by OptimizeDatabase
	RequestsTotal          int     `gorm:"check:requests_total >= 0"`        // Total number of requests at router level (Traefik CLF field)

	// Headers
	UserAgent string `gorm:"type:varchar(512)"` // Most user agents are <512 chars
	Referer   string `gorm:"type:varchar(512)"` // Most referers are <512 chars

	// Parsed User-Agent fields
	Browser        string `gorm:"type:varchar(50)"` // index created by OptimizeDatabase
	BrowserVersion string `gorm:"type:varchar(20)"`
	OS             string `gorm:"type:varchar(50)"` // index created by OptimizeDatabase
	OSVersion      string `gorm:"type:varchar(20)"`
	DeviceType     string `gorm:"type:varchar(20)"` // desktop, mobile, tablet, bot - index created by OptimizeDatabase

	// Proxy/Upstream info (proxy-agnostic naming)
	// These fields work for Traefik, NPM, Caddy, HAProxy, etc.
	BackendName         string `gorm:"type:varchar(255)"`                                    // Traefik: BackendName, NPM: proxy_upstream_name, Caddy: upstream_addr
	BackendURL          string `gorm:"type:varchar(512)"`                                    // Full backend URL (Traefik: BackendURL, others: constructed)
	RouterName          string `gorm:"type:varchar(255)"`                                    // Traefik: RouterName, NPM: server_name, Caddy: logger name - index created by OptimizeDatabase
	UpstreamStatus      int    `gorm:"check:upstream_status >= 0 AND upstream_status < 600"` // Upstream/backend response status
	UpstreamContentType string `gorm:"type:varchar(255)"`                                    // Origin/backend Content-Type (origin_Content-Type in Traefik)
	ClientHostname      string `gorm:"type:varchar(255)"`                                    // Client hostname (if reverse DNS available, from ClientHost)

	// TLS info
	TLSVersion    string `gorm:"type:varchar(10)"` // 1.2, 1.3
	TLSCipher     string `gorm:"type:varchar(255)"`
	TLSServerName string `gorm:"type:varchar(255)"` // SNI server name

	// Tracing & IDs
	RequestID string `gorm:"type:varchar(100)"` // X-Request-ID or similar - index created by OptimizeDatabase
	TraceID   string `gorm:"type:varchar(100)"` // Distributed tracing ID (optional) - index created by OptimizeDatabase

	// GeoIP enrichment
	GeoCountry string `gorm:"type:varchar(2)"` // ISO 3166-1 alpha-2 - index created by OptimizeDatabase
	GeoCity    string `gorm:"type:varchar(100)"`
	GeoLat     float64
	GeoLon     float64
	ASN        int
	ASNOrg     string `gorm:"type:varchar(255)"`

	// Extensibility: JSON field for proxy-specific data
	// This allows storing proxy-specific fields without schema changes
	// Examples: Traefik middlewares, NPM custom fields, Caddy logger details
	ProxyMetadata string `gorm:"type:text"` // JSON string for flexible data

	CreatedAt time.Time `gorm:"autoCreateTime"` // index created by OptimizeDatabase

	// Foreign key
	LogSource LogSource `gorm:"foreignKey:SourceName;references:Name"`
}

func (HTTPRequest) TableName() string {
	return "http_requests"
}

// BeforeCreate hook to automatically set partition key
func (r *HTTPRequest) BeforeCreate(tx *gorm.DB) error {
	// Set partition key for future partitioning support (YYYY-MM format)
	if r.PartitionKey == "" {
		r.PartitionKey = r.Timestamp.Format("2006-01")
	}
	return nil
}
