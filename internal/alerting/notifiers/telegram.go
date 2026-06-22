package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"time"
)

// TelegramConfig is stored as JSON in AlertChannel.Config.
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func SendTelegram(configJSON string, msg AlertMessage) error {
	var cfg TelegramConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("telegram config: %w", err)
	}
	if cfg.BotToken == "" {
		return fmt.Errorf("telegram bot_token is empty")
	}
	if cfg.ChatID == "" {
		return fmt.Errorf("telegram chat_id is empty")
	}

	_, emoji := severityStyle(msg.Severity)

	text := fmt.Sprintf("%s <b>LogLynx alert: %s</b>\n\n<b>Severity:</b> %s\n<b>Grouped by:</b> <code>%s</code>\n<b>Group value:</b> <code>%s</code>\n<b>Matched:</b> %d requests\n<b>Threshold:</b> %d requests\n<b>Window:</b> %s\n<b>Cooldown:</b> %s\n<b>Time:</b> %s\n\n%s\n\n<i>%s</i>",
		emoji,
		html.EscapeString(msg.RuleName),
		html.EscapeString(msg.Severity),
		html.EscapeString(msg.GroupBy),
		html.EscapeString(msg.GroupDisplay()),
		msg.Count,
		msg.ThresholdCount,
		html.EscapeString(msg.WindowDisplay()),
		html.EscapeString(msg.CooldownDisplay()),
		msg.TriggeredAt.UTC().Format(time.RFC3339),
		html.EscapeString(msg.DescriptionDisplay()),
		html.EscapeString(msg.FooterText()),
	)

	payload := map[string]interface{}{
		"chat_id":    cfg.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}
	return nil
}
