package alerting

import (
	"encoding/json"
	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"

	"github.com/pterm/pterm"
)

type presetDef struct {
	Name           string
	Description    string
	Severity       string
	Conditions     []Condition
	ThresholdCount int
	WindowSecs     int
	GroupBy        string
	CooldownSecs   int
}

var builtinPresets = []presetDef{
	{
		Name:           "DDoS Detection",
		Description:    "Detects a flood of requests from a single IP, a classic DDoS pattern. Fires when one IP sends an unusually high volume of requests in a short window.",
		Severity:       "critical",
		Conditions:     []Condition{},
		ThresholdCount: 500,
		WindowSecs:     60,
		GroupBy:        "client_ip",
		CooldownSecs:   300,
	},
	{
		Name:        "WordPress Brute Force (Login)",
		Description: "Detects repeated POST requests to /wp-login.php from a single IP, a sign of credential stuffing or brute-force attacks.",
		Severity:    "critical",
		Conditions: []Condition{
			{Field: "path", Operator: "starts_with", Value: "/wp-login"},
			{Field: "method", Operator: "eq", Value: "POST"},
		},
		ThresholdCount: 20,
		WindowSecs:     300,
		GroupBy:        "client_ip",
		CooldownSecs:   600,
	},
	{
		Name:        "WordPress Admin Scanner",
		Description: "Detects repeated access to the WordPress admin area from one IP, typical of automated vulnerability scanners.",
		Severity:    "warning",
		Conditions: []Condition{
			{Field: "path", Operator: "starts_with", Value: "/wp-admin"},
		},
		ThresholdCount: 50,
		WindowSecs:     300,
		GroupBy:        "client_ip",
		CooldownSecs:   600,
	},
	{
		Name:        "HTTP 4xx Error Storm",
		Description: "Fires when a single IP generates a high number of 4xx errors, usually path-scanning or broken client behaviour.",
		Severity:    "warning",
		Conditions: []Condition{
			{Field: "status_code", Operator: "gte", Value: "400"},
			{Field: "status_code", Operator: "lt", Value: "500"},
		},
		ThresholdCount: 100,
		WindowSecs:     60,
		GroupBy:        "client_ip",
		CooldownSecs:   300,
	},
	{
		Name:        "Server Error (5xx) Spike",
		Description: "Detects a global spike in 5xx server errors, often signals a backend outage, misconfiguration, or resource exhaustion.",
		Severity:    "critical",
		Conditions: []Condition{
			{Field: "status_code", Operator: "gte", Value: "500"},
		},
		ThresholdCount: 20,
		WindowSecs:     300,
		GroupBy:        "global",
		CooldownSecs:   600,
	},
	{
		Name:        "Slow Response Anomaly",
		Description: "Triggers when many responses take more than 5 seconds globally, a sign of backend latency or overload.",
		Severity:    "warning",
		Conditions: []Condition{
			{Field: "response_time_ms", Operator: "gte", Value: "5000"},
		},
		ThresholdCount: 10,
		WindowSecs:     120,
		GroupBy:        "global",
		CooldownSecs:   600,
	},
	{
		Name:        "High Country Traffic",
		Description: "Detects an unusually high volume of requests from a single country in a short window, useful for geo-targeted attack detection.",
		Severity:    "info",
		Conditions:  []Condition{},
		ThresholdCount: 1000,
		WindowSecs:     300,
		GroupBy:        "geo_country",
		CooldownSecs:   600,
	},
}

// SeedPresets inserts built-in preset rules that don't already exist in the DB.
func SeedPresets(repo repositories.AlertRepository, logger *pterm.Logger) {
	for _, p := range builtinPresets {
		exists, err := repo.RuleExistsByName(p.Name)
		if err != nil {
			logger.Warn("Failed to check preset existence", logger.Args("preset", p.Name, "error", err))
			continue
		}
		if exists {
			continue
		}

		condsJSON, _ := json.Marshal(p.Conditions)
		channelIDs, _ := json.Marshal([]uint{})

		rule := &models.AlertRule{
			Name:           p.Name,
			Description:    p.Description,
			Enabled:        false, // presets start disabled — user must opt in
			IsPreset:       true,
			Severity:       p.Severity,
			Conditions:     string(condsJSON),
			ThresholdCount: p.ThresholdCount,
			WindowSecs:     p.WindowSecs,
			GroupBy:        p.GroupBy,
			CooldownSecs:   p.CooldownSecs,
			ChannelIDs:     string(channelIDs),
		}
		if err := repo.CreateRule(rule); err != nil {
			logger.Warn("Failed to seed preset rule", logger.Args("preset", p.Name, "error", err))
		} else {
			logger.Debug("Seeded preset alert rule", logger.Args("name", p.Name))
		}
	}
}
