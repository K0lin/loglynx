package notifiers

import (
	"fmt"
	"strings"
	"time"
)

// AlertMessage is the normalized alert context sent to every notification channel.
type AlertMessage struct {
	RuleName       string
	Description    string
	Severity       string
	GroupBy        string
	GroupValue     string
	Count          int
	ThresholdCount int
	WindowSecs     int
	CooldownSecs   int
	TriggeredAt    time.Time
	ServerVersion  string // version of the running LogLynx instance
}

func (m AlertMessage) FooterText() string {
	if m.ServerVersion != "" {
		return fmt.Sprintf("LogLynx v%s · Alert System", m.ServerVersion)
	}
	return "LogLynx · Alert System"
}

func (m AlertMessage) GroupDisplay() string {
	if m.GroupValue == "" || m.GroupValue == "global" {
		return "all traffic"
	}
	return m.GroupValue
}

func (m AlertMessage) DescriptionDisplay() string {
	if strings.TrimSpace(m.Description) != "" {
		return m.Description
	}
	return fmt.Sprintf("%d matching requests were seen for %s.", m.Count, m.GroupDisplay())
}

func (m AlertMessage) WindowDisplay() string {
	return durationDisplay(m.WindowSecs)
}

func (m AlertMessage) CooldownDisplay() string {
	return durationDisplay(m.CooldownSecs)
}

func durationDisplay(seconds int) string {
	if seconds <= 0 {
		return "none"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
}
