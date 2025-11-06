package models

import (
	"time"
)

type HTTPRequest struct {
    ID             uint      `gorm:"primaryKey;autoIncrement"`
    SourceName     string    `gorm:"not null;index"`
    Timestamp      time.Time `gorm:"not null;index:idx_timestamp"`
    RequestHash    string    `gorm:"uniqueIndex:idx_request_hash;size:64"` // SHA256 hash for deduplication

    // Client info
    ClientIP       string    `gorm:"not null;index:idx_client_ip"`
    ClientPort     int
    ClientUser     string    // HTTP authentication user (NPM: remote_user)

    // Request info
    Method         string    `gorm:"not null"`
    Protocol       string
    Host           string    `gorm:"not null;index:idx_host"`
    Path           string    `gorm:"not null"`
    QueryString    string
    RequestLength  int64     // Request size in bytes (NPM/Caddy)

    // Response info
    StatusCode     int       `gorm:"not null;index:idx_status"`
    ResponseSize   int64
    ResponseTimeMs float64   `gorm:"index:idx_response_time"` // Total response time

    // Detailed timing (optional, for advanced proxies)
    Duration       int64     // Duration in nanoseconds (for precise hash calculation)
    StartUTC       string    // Start timestamp with nanosecond precision (for precise hash calculation)
    UpstreamResponseTimeMs float64 // Time spent waiting for upstream/backend

    // Headers
    UserAgent      string
    Referer        string

    // Parsed User-Agent fields
    Browser        string    `gorm:"index:idx_browser"`
    BrowserVersion string
    OS             string    `gorm:"index:idx_os"`
    OSVersion      string
    DeviceType     string    `gorm:"index:idx_device_type"` // desktop, mobile, tablet, bot

    // Proxy/Upstream info (proxy-agnostic naming)
    // These fields work for Traefik, NPM, Caddy, HAProxy, etc.
    BackendName    string    // Traefik: BackendName, NPM: proxy_upstream_name, Caddy: upstream_addr
    BackendURL     string    // Full backend URL (Traefik: BackendURL, others: constructed)
    RouterName     string    // Traefik: RouterName, NPM: server_name, Caddy: logger name
    UpstreamStatus int       // Upstream/backend response status (if different from final status)

    // TLS info
    TLSVersion     string
    TLSCipher      string
    TLSServerName  string    // SNI server name

    // Tracing & IDs
    RequestID      string    // X-Request-ID or similar
    TraceID        string    // Distributed tracing ID (optional)

    // GeoIP enrichment
    GeoCountry     string    `gorm:"index:idx_geo_country"`
    GeoCity        string
    GeoLat         float64
    GeoLon         float64
    ASN            int
    ASNOrg         string

    // Extensibility: JSON field for proxy-specific data
    // This allows storing proxy-specific fields without schema changes
    // Examples: Traefik middlewares, NPM custom fields, Caddy logger details
    ProxyMetadata  string    `gorm:"type:text"` // JSON string for flexible data

    CreatedAt      time.Time `gorm:"autoCreateTime"`

    // Foreign key
    LogSource      LogSource `gorm:"foreignKey:SourceName;references:Name"`
}

func (HTTPRequest) TableName() string {
    return "http_requests"
}