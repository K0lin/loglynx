package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WebhookConfig is stored as JSON in AlertChannel.Config.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`  // POST (default), PUT, GET
	Headers map[string]string `json:"headers"` // optional extra headers
}

// WebhookPayload is the JSON body sent to the webhook endpoint.
type WebhookPayload struct {
	Event       string    `json:"event"` // "alert_triggered"
	RuleName    string    `json:"rule_name"`
	Severity    string    `json:"severity"`
	GroupValue  string    `json:"group_value"`
	Count       int       `json:"count"`
	Description string    `json:"description"`
	TriggeredAt time.Time `json:"triggered_at"`
	Source      string    `json:"source"` // "loglynx"
}

func SendWebhook(configJSON, ruleName, description, severity, groupValue string, count int) error {
	var cfg WebhookConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("webhook config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook url is empty")
	}

	method := strings.ToUpper(cfg.Method)
	if method == "" {
		method = "POST"
	}
	if method != "POST" && method != "PUT" {
		return fmt.Errorf("webhook method must be POST or PUT")
	}

	payload := WebhookPayload{
		Event:       "alert_triggered",
		RuleName:    ruleName,
		Severity:    severity,
		GroupValue:  groupValue,
		Count:       count,
		Description: description,
		TriggeredAt: time.Now().UTC(),
		Source:      "loglynx",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook marshal: %w", err)
	}

	req, err := http.NewRequest(method, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LogLynx-Alerting/1.0")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
