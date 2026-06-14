package models

import "time"

// ComparisonSnapshot stores immutable comparison reports for shareable links.
type ComparisonSnapshot struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Token     string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"token"`
	OwnerID   string     `gorm:"type:varchar(64);index" json:"-"`
	Title     string     `gorm:"type:varchar(255);not null" json:"title"`
	Payload   string     `gorm:"type:text;not null" json:"payload"`
	Active    bool       `gorm:"not null;default:true" json:"active"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ComparisonSnapshot) TableName() string {
	return "comparison_snapshots"
}
