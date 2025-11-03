package models

import (
	"time"
)

type HTTPRequest struct {
    ID             uint      `gorm:"primaryKey;autoIncrement"`
    SourceName     string    `gorm:"not null;index"`
    Timestamp      time.Time `gorm:"not null;index:idx_timestamp"`

    // Client info
    ClientIP       string    `gorm:"not null;index:idx_client_ip"`
    ClientPort     int

    // Request info
    Method         string    `gorm:"not null"`
    Protocol       string
    Host           string    `gorm:"not null;index:idx_host"`
    Path           string    `gorm:"not null"`
    QueryString    string

    // Response info
    StatusCode     int       `gorm:"not null;index:idx_status"`
    ResponseSize   int64
    ResponseTimeMs float64   `gorm:"index:idx_response_time"` // Index for percentile calculations

    // Headers
    UserAgent      string
    Referer        string

    // Parsed User-Agent fields
    Browser        string    `gorm:"index:idx_browser"`
    BrowserVersion string
    OS             string    `gorm:"index:idx_os"`
    OSVersion      string
    DeviceType     string    `gorm:"index:idx_device_type"` // desktop, mobile, tablet, bot

    // Traefik-specific
    BackendName    string
    BackendURL     string
    RouterName     string

    // TLS info
    TLSVersion     string
    TLSCipher      string

    // Tracing
    RequestID      string

    // GeoIP enrichment
    GeoCountry     string    `gorm:"index:idx_geo_country"`
    GeoCity        string
    GeoLat         float64
    GeoLon         float64
    ASN            int
    ASNOrg         string

    CreatedAt      time.Time `gorm:"autoCreateTime"`

    // Foreign key
    LogSource      LogSource `gorm:"foreignKey:SourceName;references:Name"`
}

func (HTTPRequest) TableName() string {
    return "http_requests"
}