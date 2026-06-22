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

func SendTelegram(configJSON, ruleName, description, severity, groupValue string, count int) error {
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

	_, emoji := severityStyle(severity)
	groupDisplay := groupValue
	if groupValue == "global" {
		groupDisplay = "all traffic"
	}

	text := fmt.Sprintf("%s <b>%s</b>\n\n<b>Severity:</b> %s\n<b>Group:</b> <code>%s</code>\n<b>Count:</b> %d requests\n<b>Time:</b> %s\n\n%s",
		emoji,
		html.EscapeString(ruleName),
		html.EscapeString(severity),
		html.EscapeString(groupDisplay),
		count,
		time.Now().UTC().Format(time.RFC3339),
		html.EscapeString(description),
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
