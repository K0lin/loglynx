package models

import (
	"time"
)

// IPReputation stores GeoIP and reputation data for IP addresses
type IPReputation struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	IPAddress string `gorm:"uniqueIndex;not null;index:idx_ip_lookup"`

	// GeoIP data
	Country     string `gorm:"index"`
	CountryName string
	City        string
	Latitude    float64
	Longitude   float64

	// ASN data
	ASN    int `gorm:"index"`
	ASNOrg string

	// Reputation/Usage tracking
	FirstSeen   time.Time `gorm:"not null"`
	LastSeen    time.Time `gorm:"not null;index"`
	LookupCount int64     `gorm:"default:0;index:idx_lookup_count"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (IPReputation) TableName() string {
	return "ip_reputation"
}
