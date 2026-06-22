package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordConfig is stored as JSON in AlertChannel.Config.
type DiscordConfig struct {
	WebhookURL    string `json:"webhook_url"`
	Username      string `json:"username"`       // optional bot display name
	AvatarURL     string `json:"avatar_url"`     // optional bot avatar
	MentionPolicy string `json:"mention_policy"` // never | critical | always
}

func SendDiscord(configJSON string, msg AlertMessage) error {
	var cfg DiscordConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("discord config: %w", err)
	}
	if cfg.WebhookURL == "" {
		return fmt.Errorf("discord webhook_url is empty")
	}

	color, emoji := severityStyle(msg.Severity)
	mention := discordMention(cfg.MentionPolicy, msg.Severity)

	embed := map[string]interface{}{
		"title": fmt.Sprintf("%s LogLynx alert: %s", emoji, msg.RuleName),
		"description": fmt.Sprintf(
			"%s\n\n**What happened**\n%d requests matched this rule in the last %s. Threshold is %d requests.\n\n**Details**\n%s",
			msg.DescriptionDisplay(), msg.Count, msg.WindowDisplay(), msg.ThresholdCount, msgDetailsMarkdown(msg),
		),
		"color":     color,
		"timestamp": msg.TriggeredAt.UTC().Format(time.RFC3339),
		"footer": map[string]string{
			"text": "LogLynx · Alert System",
		},
		"fields": []map[string]interface{}{
			{"name": "Severity", "value": msg.Severity, "inline": true},
			{"name": "Group", "value": fmt.Sprintf("%s = `%s`", msg.GroupBy, msg.GroupDisplay()), "inline": true},
			{"name": "Cooldown", "value": msg.CooldownDisplay(), "inline": true},
		},
	}

	username := cfg.Username
	if username == "" {
		username = "LogLynx Alerts"
	}

	payload := map[string]interface{}{
		"content":    mention,
		"username":   username,
		"avatar_url": cfg.AvatarURL,
		"embeds":     []interface{}{embed},
		"allowed_mentions": map[string]interface{}{
			"parse": []string{},
		},
	}
	if mention != "" {
		payload["allowed_mentions"] = map[string]interface{}{"parse": []string{"everyone"}}
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func discordMention(policy, severity string) string {
	switch policy {
	case "always":
		return "@everyone"
	case "critical":
		if severity == "critical" {
			return "@everyone"
		}
	}
	return ""
}

func msgDetailsMarkdown(msg AlertMessage) string {
	return fmt.Sprintf("- Grouped by: `%s`\n- Group value: `%s`\n- Matched requests: `%d`\n- Evaluation window: `%s`",
		msg.GroupBy, msg.GroupDisplay(), msg.Count, msg.WindowDisplay())
}

func severityStyle(severity string) (int, string) {
	switch severity {
	case "critical":
		return 0xef4444, "🚨"
	case "info":
		return 0x3b82f6, "ℹ️"
	default: // warning
		return 0xf59e0b, "⚠️"
	}
}
