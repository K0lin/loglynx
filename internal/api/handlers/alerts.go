package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"loglynx/internal/alerting"
	"loglynx/internal/alerting/notifiers"
	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// AlertsHandler manages alert rules, channels, and event history.
type AlertsHandler struct {
	repo   repositories.AlertRepository
	logger *pterm.Logger
}

func NewAlertsHandler(repo repositories.AlertRepository, logger *pterm.Logger) *AlertsHandler {
	return &AlertsHandler{repo: repo, logger: logger}
}

// ─── Channels ────────────────────────────────────────────────────────────────

func (h *AlertsHandler) ListChannels(c *gin.Context) {
	channels, err := h.repo.ListChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list channels"})
		return
	}
	c.JSON(http.StatusOK, channels)
}

func (h *AlertsHandler) CreateChannel(c *gin.Context) {
	var ch models.AlertChannel
	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ch.Name == "" || ch.Type == "" || ch.Config == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, type, and config are required"})
		return
	}
	if err := validateChannel(&ch, ""); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.CreateChannel(&ch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create channel"})
		return
	}
	c.JSON(http.StatusCreated, ch)
}

func (h *AlertsHandler) UpdateChannel(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	existing, err := h.repo.GetChannel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	oldConfig := existing.Config
	if err := c.ShouldBindJSON(existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	existing.ID = id
	if err := validateChannel(existing, oldConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.UpdateChannel(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update channel"})
		return
	}
	c.JSON(http.StatusOK, existing)
}

func (h *AlertsHandler) DeleteChannel(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.repo.DeleteChannel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete channel"})
		return
	}
	c.Status(http.StatusNoContent)
}

// TestChannel sends a test notification on the given channel.
func (h *AlertsHandler) TestChannel(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ch, err := h.repo.GetChannel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}

	const testRule = "Test Alert"
	const testDesc = "This is a test notification from LogLynx. Your channel is configured correctly!"
	testMsg := notifiers.AlertMessage{
		RuleName:       testRule,
		Description:    testDesc,
		Severity:       "info",
		GroupBy:        "test",
		GroupValue:     "test",
		Count:          1,
		ThresholdCount: 1,
		WindowSecs:     60,
		CooldownSecs:   300,
		TriggeredAt:    time.Now(),
	}
	var sendErr error
	switch ch.Type {
	case "discord":
		sendErr = notifiers.SendDiscord(ch.Config, testMsg)
	case "email":
		sendErr = notifiers.SendEmail(ch.Config, testMsg)
	case "telegram":
		sendErr = notifiers.SendTelegram(ch.Config, testMsg)
	case "webhook":
		sendErr = notifiers.SendWebhook(ch.Config, testMsg)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown channel type"})
		return
	}

	if sendErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": sendErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Test notification sent successfully"})
}

// ─── Rules ───────────────────────────────────────────────────────────────────

func (h *AlertsHandler) ListRules(c *gin.Context) {
	rules, err := h.repo.ListRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rules"})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *AlertsHandler) CreateRule(c *gin.Context) {
	var rule models.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rule.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if rule.Conditions == "" {
		rule.Conditions = "[]"
	}
	if rule.ChannelIDs == "" {
		defaultIDs, err := h.defaultChannelIDsForNewRules()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load default channels"})
			return
		}
		rule.ChannelIDs = defaultIDs
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if rule.GroupBy == "" {
		rule.GroupBy = "client_ip"
	}
	if rule.ThresholdCount == 0 {
		rule.ThresholdCount = 10
	}
	if rule.WindowSecs == 0 {
		rule.WindowSecs = 60
	}
	if rule.CooldownSecs == 0 {
		rule.CooldownSecs = 300
	}

	if err := validateRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule.IsPreset = false
	if err := h.repo.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create rule"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *AlertsHandler) GetRule(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	rule, err := h.repo.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *AlertsHandler) UpdateRule(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	existing, err := h.repo.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	// Decode update payload into a map so we only apply sent fields.
	var patch map[string]json.RawMessage
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	applyStringPatch(patch, "name", &existing.Name)
	applyStringPatch(patch, "description", &existing.Description)
	applyStringPatch(patch, "severity", &existing.Severity)
	applyStringPatch(patch, "conditions", &existing.Conditions)
	applyStringPatch(patch, "group_by", &existing.GroupBy)
	applyStringPatch(patch, "channel_ids", &existing.ChannelIDs)
	applyIntPatch(patch, "threshold_count", &existing.ThresholdCount)
	applyIntPatch(patch, "window_secs", &existing.WindowSecs)
	applyIntPatch(patch, "cooldown_secs", &existing.CooldownSecs)
	applyBoolPatch(patch, "enabled", &existing.Enabled)

	if err := validateRule(existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.UpdateRule(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update rule"})
		return
	}
	c.JSON(http.StatusOK, existing)
}

func (h *AlertsHandler) DeleteRule(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	rule, err := h.repo.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	if rule.IsPreset {
		c.JSON(http.StatusForbidden, gin.H{"error": "preset rules cannot be deleted"})
		return
	}
	if err := h.repo.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete rule"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ToggleRule enables or disables a rule without a full update.
func (h *AlertsHandler) ToggleRule(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	rule, err := h.repo.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.Enabled = body.Enabled
	if err := h.repo.UpdateRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to toggle rule"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// ─── History ─────────────────────────────────────────────────────────────────

type alertEventResponse struct {
	ID          uint      `json:"id"`
	RuleID      uint      `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	GroupValue  string    `json:"group_value"`
	Count       int       `json:"count"`
	Severity    string    `json:"severity"`
	TriggeredAt time.Time `json:"triggered_at"`
}

func (h *AlertsHandler) ListHistory(c *gin.Context) {
	limit := 100
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	events, total, err := h.repo.ListEvents(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load history"})
		return
	}

	resp := make([]alertEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, alertEventResponse{
			ID:          e.ID,
			RuleID:      e.RuleID,
			RuleName:    e.RuleName,
			GroupValue:  e.GroupValue,
			Count:       e.Count,
			Severity:    e.Severity,
			TriggeredAt: e.TriggeredAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "events": resp})
}

func (h *AlertsHandler) ClearHistory(c *gin.Context) {
	if err := h.repo.DeleteAllEvents(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear history"})
		return
	}
	c.Status(http.StatusNoContent)
}

// AlertStats returns summary counts used by the UI header cards.
func (h *AlertsHandler) AlertStats(c *gin.Context) {
	rules, _ := h.repo.ListRules()
	channels, _ := h.repo.ListChannels()
	alerts24h, _ := h.repo.CountEventsLast24h()

	total := len(rules)
	active := 0
	for _, r := range rules {
		if r.Enabled {
			active++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"total_rules":    total,
		"active_rules":   active,
		"total_channels": len(channels),
		"alerts_24h":     alerts24h,
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func parseUintParam(c *gin.Context, param string) (uint, error) {
	v, err := strconv.ParseUint(c.Param(param), 10, 64)
	return uint(v), err
}

func applyStringPatch(patch map[string]json.RawMessage, key string, dest *string) {
	if raw, ok := patch[key]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			*dest = s
		}
	}
}

func applyIntPatch(patch map[string]json.RawMessage, key string, dest *int) {
	if raw, ok := patch[key]; ok {
		var v int
		if json.Unmarshal(raw, &v) == nil {
			*dest = v
		}
	}
}

func applyBoolPatch(patch map[string]json.RawMessage, key string, dest *bool) {
	if raw, ok := patch[key]; ok {
		var v bool
		if json.Unmarshal(raw, &v) == nil {
			*dest = v
		}
	}
}

func validateChannel(ch *models.AlertChannel, oldConfig string) error {
	ch.Type = strings.ToLower(strings.TrimSpace(ch.Type))
	ch.Name = strings.TrimSpace(ch.Name)

	switch ch.Type {
	case "discord":
		var cfg notifiers.DiscordConfig
		if err := decodeConfig(ch.Config, &cfg); err != nil {
			return err
		}
		if err := requireHTTPSURL("discord webhook_url", cfg.WebhookURL); err != nil {
			return err
		}
		if cfg.MentionPolicy == "" {
			cfg.MentionPolicy = "never"
		}
		if cfg.MentionPolicy != "never" && cfg.MentionPolicy != "critical" && cfg.MentionPolicy != "always" {
			return fmt.Errorf("discord mention_policy must be never, critical, or always")
		}
		ch.Config = mustMarshalConfig(cfg)
	case "email":
		var cfg notifiers.EmailConfig
		if err := decodeConfig(ch.Config, &cfg); err != nil {
			return err
		}
		if cfg.Password == "" && oldConfig != "" {
			var old notifiers.EmailConfig
			if json.Unmarshal([]byte(oldConfig), &old) == nil {
				cfg.Password = old.Password
			}
		}
		if cfg.SMTPHost == "" {
			return fmt.Errorf("email smtp_host is required")
		}
		if cfg.SMTPPort <= 0 || cfg.SMTPPort > 65535 {
			return fmt.Errorf("email smtp_port must be between 1 and 65535")
		}
		if cfg.From == "" {
			return fmt.Errorf("email from is required")
		}
		if len(cfg.To) == 0 {
			return fmt.Errorf("email to must contain at least one recipient")
		}
		ch.Config = mustMarshalConfig(cfg)
	case "telegram":
		var cfg notifiers.TelegramConfig
		if err := decodeConfig(ch.Config, &cfg); err != nil {
			return err
		}
		if cfg.BotToken == "" && oldConfig != "" {
			var old notifiers.TelegramConfig
			if json.Unmarshal([]byte(oldConfig), &old) == nil {
				cfg.BotToken = old.BotToken
			}
		}
		if strings.TrimSpace(cfg.BotToken) == "" {
			return fmt.Errorf("telegram bot_token is required")
		}
		if strings.TrimSpace(cfg.ChatID) == "" {
			return fmt.Errorf("telegram chat_id is required")
		}
		ch.Config = mustMarshalConfig(cfg)
	case "webhook":
		var cfg notifiers.WebhookConfig
		if err := decodeConfig(ch.Config, &cfg); err != nil {
			return err
		}
		if err := requireHTTPSURL("webhook url", cfg.URL); err != nil {
			return err
		}
		method := strings.ToUpper(cfg.Method)
		if method == "" {
			method = "POST"
		}
		if method != "POST" && method != "PUT" {
			return fmt.Errorf("webhook method must be POST or PUT")
		}
		cfg.Method = method
		ch.Config = mustMarshalConfig(cfg)
	default:
		return fmt.Errorf("unsupported channel type %q", ch.Type)
	}
	return nil
}

func validateRule(rule *models.AlertRule) error {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		return fmt.Errorf("name is required")
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if rule.Severity != "info" && rule.Severity != "warning" && rule.Severity != "critical" {
		return fmt.Errorf("severity must be info, warning, or critical")
	}
	if rule.GroupBy == "" {
		rule.GroupBy = "client_ip"
	}
	if !validGroupBy(rule.GroupBy) {
		return fmt.Errorf("group_by is invalid")
	}
	if rule.ThresholdCount <= 0 {
		return fmt.Errorf("threshold_count must be greater than 0")
	}
	if rule.WindowSecs <= 0 {
		return fmt.Errorf("window_secs must be greater than 0")
	}
	if rule.CooldownSecs < 0 {
		return fmt.Errorf("cooldown_secs cannot be negative")
	}
	conds, err := alerting.ParseConditions(rule.Conditions)
	if err != nil {
		return fmt.Errorf("invalid conditions: %w", err)
	}
	if _, _, err := alerting.BuildWhereClause(conds); err != nil {
		return fmt.Errorf("invalid conditions: %w", err)
	}
	var channelIDs []uint
	if rule.ChannelIDs == "" {
		rule.ChannelIDs = "[]"
	}
	if err := json.Unmarshal([]byte(rule.ChannelIDs), &channelIDs); err != nil {
		return fmt.Errorf("channel_ids must be a JSON array of IDs")
	}
	return nil
}

func validGroupBy(groupBy string) bool {
	switch groupBy {
	case "global", "client_ip", "geo_country", "host", "backend_name", "source_name":
		return true
	default:
		return false
	}
}

func (h *AlertsHandler) defaultChannelIDsForNewRules() (string, error) {
	channels, err := h.repo.ListChannels()
	if err != nil {
		return "", err
	}
	ids := make([]uint, 0)
	for _, ch := range channels {
		if ch.Enabled && ch.DefaultForNewRules {
			ids = append(ids, ch.ID)
		}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeConfig(configJSON string, dest interface{}) error {
	if err := json.Unmarshal([]byte(configJSON), dest); err != nil {
		return fmt.Errorf("invalid channel config: %w", err)
	}
	return nil
}

func requireHTTPSURL(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid URL", name)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s must use https", name)
	}
	return nil
}

func mustMarshalConfig(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
