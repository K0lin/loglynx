package models

import "time"

// AlertChannel is a notification destination (Discord, Email, Telegram, or generic Webhook).
type AlertChannel struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name               string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Type               string    `gorm:"type:varchar(50);not null" json:"type"` // discord | email | telegram | webhook
	Config             string    `gorm:"type:text;not null" json:"config"`      // JSON blob, shape depends on Type
	Enabled            bool      `gorm:"default:true" json:"enabled"`
	DefaultForNewRules bool      `gorm:"default:false" json:"default_for_new_rules"`
	CreatedAt          time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// AlertRule defines when an alert fires and how to notify.
// Conditions is a JSON-encoded []Condition. ChannelIDs is a JSON-encoded []uint.
type AlertRule struct {
	ID              uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Description     string     `gorm:"type:text" json:"description"`
	Enabled         bool       `gorm:"default:true" json:"enabled"`
	IsPreset        bool       `gorm:"default:false" json:"is_preset"`
	Severity        string     `gorm:"type:varchar(20);default:'warning'" json:"severity"` // info | warning | critical
	Conditions      string     `gorm:"type:text;not null" json:"conditions"`               // JSON []Condition
	ThresholdCount  int        `gorm:"not null;default:10" json:"threshold_count"`
	WindowSecs      int        `gorm:"not null;default:60" json:"window_secs"`
	GroupBy         string     `gorm:"type:varchar(50);default:'client_ip'" json:"group_by"` // global | client_ip | geo_country | host | backend_name | source_name
	CooldownSecs    int        `gorm:"not null;default:300" json:"cooldown_secs"`
	ChannelIDs      string     `gorm:"type:text" json:"channel_ids"` // JSON []uint
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// AlertEvent records one firing of an AlertRule.
type AlertEvent struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleID      uint      `gorm:"not null;index" json:"rule_id"`
	RuleName    string    `gorm:"type:varchar(255)" json:"rule_name"`
	GroupValue  string    `gorm:"type:varchar(255)" json:"group_value"` // e.g. IP, country code, "global"
	Count       int       `json:"count"`
	Severity    string    `gorm:"type:varchar(20)" json:"severity"`
	Details     string    `gorm:"type:text" json:"details"` // JSON extra context
	TriggeredAt time.Time `gorm:"not null;index" json:"triggered_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}
