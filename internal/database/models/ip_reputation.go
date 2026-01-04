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

