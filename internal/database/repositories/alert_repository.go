package repositories

import (
	"loglynx/internal/database/models"
	"time"

	"gorm.io/gorm"
)

type AlertRepository interface {
	// Channels
	CreateChannel(ch *models.AlertChannel) error
	GetChannel(id uint) (*models.AlertChannel, error)
	ListChannels() ([]*models.AlertChannel, error)
	UpdateChannel(ch *models.AlertChannel) error
	DeleteChannel(id uint) error

	// Rules
	CreateRule(rule *models.AlertRule) error
	GetRule(id uint) (*models.AlertRule, error)
	ListRules() ([]*models.AlertRule, error)
	UpdateRule(rule *models.AlertRule) error
	DeleteRule(id uint) error
	RuleExistsByName(name string) (bool, error)

	// Events
	CreateEvent(event *models.AlertEvent) error
	ListEvents(limit, offset int) ([]*models.AlertEvent, int64, error)
	GetLastEventForRuleAndGroup(ruleID uint, groupValue string) (*models.AlertEvent, error)
	CountEventsLast24h() (int64, error)
	DeleteAllEvents() error
}

type alertRepo struct {
	db *gorm.DB
}

func NewAlertRepository(db *gorm.DB) AlertRepository {
	return &alertRepo{db: db}
}

// ─── Channels ───────────────────────────────────────────────────────────────

func (r *alertRepo) CreateChannel(ch *models.AlertChannel) error {
	return r.db.Create(ch).Error
}

func (r *alertRepo) GetChannel(id uint) (*models.AlertChannel, error) {
	var ch models.AlertChannel
	if err := r.db.First(&ch, id).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *alertRepo) ListChannels() ([]*models.AlertChannel, error) {
	var channels []*models.AlertChannel
	return channels, r.db.Order("name").Find(&channels).Error
}

func (r *alertRepo) UpdateChannel(ch *models.AlertChannel) error {
	return r.db.Save(ch).Error
}

func (r *alertRepo) DeleteChannel(id uint) error {
	return r.db.Delete(&models.AlertChannel{}, id).Error
}

// ─── Rules ──────────────────────────────────────────────────────────────────

func (r *alertRepo) CreateRule(rule *models.AlertRule) error {
	return r.db.Create(rule).Error
}

func (r *alertRepo) GetRule(id uint) (*models.AlertRule, error) {
	var rule models.AlertRule
	if err := r.db.First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *alertRepo) ListRules() ([]*models.AlertRule, error) {
	var rules []*models.AlertRule
	return rules, r.db.Order("is_preset DESC, name").Find(&rules).Error
}

func (r *alertRepo) UpdateRule(rule *models.AlertRule) error {
	return r.db.Save(rule).Error
}

func (r *alertRepo) DeleteRule(id uint) error {
	return r.db.Delete(&models.AlertRule{}, id).Error
}

func (r *alertRepo) RuleExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&models.AlertRule{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

// ─── Events ─────────────────────────────────────────────────────────────────

func (r *alertRepo) CreateEvent(event *models.AlertEvent) error {
	return r.db.Create(event).Error
}

func (r *alertRepo) ListEvents(limit, offset int) ([]*models.AlertEvent, int64, error) {
	var events []*models.AlertEvent
	var total int64
	r.db.Model(&models.AlertEvent{}).Count(&total)
	err := r.db.Order("triggered_at DESC").Limit(limit).Offset(offset).Find(&events).Error
	return events, total, err
}

func (r *alertRepo) GetLastEventForRuleAndGroup(ruleID uint, groupValue string) (*models.AlertEvent, error) {
	var event models.AlertEvent
	err := r.db.Where("rule_id = ? AND group_value = ?", ruleID, groupValue).
		Order("triggered_at DESC").
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *alertRepo) CountEventsLast24h() (int64, error) {
	var count int64
	since := time.Now().Add(-24 * time.Hour)
	err := r.db.Model(&models.AlertEvent{}).Where("triggered_at >= ?", since).Count(&count).Error
	return count, err
}

func (r *alertRepo) DeleteAllEvents() error {
	return r.db.Where("1 = 1").Delete(&models.AlertEvent{}).Error
}
