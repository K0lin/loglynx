package alerting

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"loglynx/internal/alerting/notifiers"
	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"
	"loglynx/internal/version"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

// evalResult holds one row from the GROUP BY evaluation query.
type evalResult struct {
	GroupVal string
	Count    int
}

// Engine evaluates alert rules on a fixed interval and dispatches notifications.
type Engine struct {
	db        *gorm.DB
	alertRepo repositories.AlertRepository
	logger    *pterm.Logger
	interval  time.Duration
	stopCh    chan struct{}

	// cooldowns tracks the last trigger time per "ruleID:groupValue" key.
	// In-memory only: resets on restart (acceptable — brief false re-alert after restart).
	cooldowns map[string]time.Time
	mu        sync.Mutex
}

func NewEngine(
	db *gorm.DB,
	alertRepo repositories.AlertRepository,
	logger *pterm.Logger,
	interval time.Duration,
) *Engine {
	return &Engine{
		db:        db,
		alertRepo: alertRepo,
		logger:    logger,
		interval:  interval,
		stopCh:    make(chan struct{}),
		cooldowns: make(map[string]time.Time),
	}
}

func (e *Engine) Start() {
	go func() {
		e.logger.Info("Alert engine started", e.logger.Args("interval", e.interval))
		ticker := time.NewTicker(e.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.evaluateAll()
			case <-e.stopCh:
				e.logger.Info("Alert engine stopped")
				return
			}
		}
	}()
}

func (e *Engine) Stop() {
	close(e.stopCh)
}

func (e *Engine) evaluateAll() {
	rules, err := e.alertRepo.ListRules()
	if err != nil {
		e.logger.Warn("Alert engine: failed to load rules", e.logger.Args("error", err))
		return
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if err := e.evaluateRule(rule); err != nil {
			e.logger.Debug("Alert rule evaluation error",
				e.logger.Args("rule", rule.Name, "error", err))
		}
	}
}

func (e *Engine) evaluateRule(rule *models.AlertRule) error {
	conds, err := ParseConditions(rule.Conditions)
	if err != nil {
		return fmt.Errorf("parse conditions: %w", err)
	}

	where, args, err := BuildWhereClause(conds)
	if err != nil {
		return fmt.Errorf("build where: %w", err)
	}

	// Append time-window filter.
	since := time.Now().Add(-time.Duration(rule.WindowSecs) * time.Second)
	where += " AND timestamp >= ?"
	args = append(args, since)

	groupCol := GroupByColumn(rule.GroupBy)

	// Append threshold as last arg.
	args = append(args, rule.ThresholdCount)

	query := fmt.Sprintf(
		`SELECT %s AS group_val, COUNT(*) AS count
		 FROM http_requests
		 WHERE %s
		 GROUP BY %s
		 HAVING COUNT(*) >= ?`,
		groupCol, where, groupCol,
	)

	var results []evalResult
	if err := e.db.Raw(query, args...).Scan(&results).Error; err != nil {
		return fmt.Errorf("query: %w", err)
	}

	for _, r := range results {
		if e.isInCooldown(rule.ID, r.GroupVal, time.Duration(rule.CooldownSecs)*time.Second) {
			continue
		}
		e.fire(rule, r.GroupVal, r.Count)
	}
	return nil
}

func (e *Engine) isInCooldown(ruleID uint, groupVal string, cooldown time.Duration) bool {
	key := fmt.Sprintf("%d:%s", ruleID, groupVal)
	e.mu.Lock()
	defer e.mu.Unlock()
	last, ok := e.cooldowns[key]
	return ok && time.Since(last) < cooldown
}

func (e *Engine) fire(rule *models.AlertRule, groupVal string, count int) {
	key := fmt.Sprintf("%d:%s", rule.ID, groupVal)
	e.mu.Lock()
	e.cooldowns[key] = time.Now()
	e.mu.Unlock()

	e.logger.Info("Alert fired",
		e.logger.Args("rule", rule.Name, "group", groupVal, "count", count))

	event := &models.AlertEvent{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		GroupValue:  groupVal,
		Count:       count,
		Severity:    rule.Severity,
		Details:     buildDetails(rule, groupVal, count),
		TriggeredAt: time.Now(),
	}
	if err := e.alertRepo.CreateEvent(event); err != nil {
		e.logger.Warn("Failed to persist alert event", e.logger.Args("error", err))
	}

	// Update rule's LastTriggeredAt.
	now := time.Now()
	rule.LastTriggeredAt = &now
	_ = e.alertRepo.UpdateRule(rule)

	// Dispatch to channels.
	e.dispatch(rule, groupVal, count)
}

func (e *Engine) dispatch(rule *models.AlertRule, groupVal string, count int) {
	var channelIDs []uint
	if err := json.Unmarshal([]byte(rule.ChannelIDs), &channelIDs); err != nil || len(channelIDs) == 0 {
		return
	}

	channels, err := e.alertRepo.ListChannels()
	if err != nil {
		e.logger.Warn("Failed to load channels for dispatch", e.logger.Args("error", err))
		return
	}

	// Index channels by ID for fast lookup.
	byID := make(map[uint]*models.AlertChannel, len(channels))
	for _, ch := range channels {
		byID[ch.ID] = ch
	}

	msg := notifiers.AlertMessage{
		RuleName:       rule.Name,
		Description:    rule.Description,
		Severity:       rule.Severity,
		GroupBy:        rule.GroupBy,
		GroupValue:     groupVal,
		Count:          count,
		ThresholdCount: rule.ThresholdCount,
		WindowSecs:     rule.WindowSecs,
		CooldownSecs:   rule.CooldownSecs,
		TriggeredAt:    time.Now(),
		ServerVersion:  version.Version,
	}

	for _, id := range channelIDs {
		ch, ok := byID[id]
		if !ok || !ch.Enabled {
			continue
		}
		var sendErr error
		switch ch.Type {
		case "discord":
			sendErr = notifiers.SendDiscord(ch.Config, msg)
		case "email":
			sendErr = notifiers.SendEmail(ch.Config, msg)
		case "telegram":
			sendErr = notifiers.SendTelegram(ch.Config, msg)
		case "webhook":
			sendErr = notifiers.SendWebhook(ch.Config, msg)
		}
		if sendErr != nil {
			e.logger.Warn("Notification send failed",
				e.logger.Args("channel", ch.Name, "type", ch.Type, "error", sendErr))
		} else {
			e.logger.Debug("Notification sent",
				e.logger.Args("channel", ch.Name, "type", ch.Type))
		}
	}
}

func buildDetails(rule *models.AlertRule, groupVal string, count int) string {
	d := map[string]interface{}{
		"rule_id":         rule.ID,
		"threshold_count": rule.ThresholdCount,
		"window_secs":     rule.WindowSecs,
		"group_by":        rule.GroupBy,
		"group_value":     groupVal,
		"matched_count":   count,
	}
	b, _ := json.Marshal(d)
	return string(b)
}
