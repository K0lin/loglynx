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
	WebhookURL string `json:"webhook_url"`
	Username   string `json:"username"`   // optional bot display name
	AvatarURL  string `json:"avatar_url"` // optional bot avatar
}

func SendDiscord(configJSON, ruleName, description, severity, groupValue string, count int) error {
	var cfg DiscordConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("discord config: %w", err)
	}
	if cfg.WebhookURL == "" {
		return fmt.Errorf("discord webhook_url is empty")
	}

	color, emoji := severityStyle(severity)

	groupDisplay := groupValue
	if groupValue == "global" {
		groupDisplay = "all traffic"
	}

	embed := map[string]interface{}{
		"title": fmt.Sprintf("%s %s", emoji, ruleName),
		"description": fmt.Sprintf(
			"**%d requests** matched in the last window\n**Group:** `%s`\n\n%s",
			count, groupDisplay, description,
		),
		"color":     color,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"footer": map[string]string{
			"text": "LogLynx · Alert System",
		},
		"fields": []map[string]interface{}{
			{"name": "Severity", "value": severity, "inline": true},
			{"name": "Group by", "value": groupDisplay, "inline": true},
			{"name": "Count", "value": fmt.Sprintf("%d req", count), "inline": true},
		},
	}

	username := cfg.Username
	if username == "" {
		username = "LogLynx Alerts"
	}

	payload := map[string]interface{}{
		"username":   username,
		"avatar_url": cfg.AvatarURL,
		"embeds":     []interface{}{embed},
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
