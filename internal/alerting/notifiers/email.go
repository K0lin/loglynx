package notifiers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// EmailConfig is stored as JSON in AlertChannel.Config.
type EmailConfig struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`       // one or more recipients
	UseTLS   bool     `json:"use_tls"`  // implicit TLS (port 465)
	StartTLS bool     `json:"starttls"` // STARTTLS upgrade (port 587)
}

func SendEmail(configJSON string, msg AlertMessage) error {
	var cfg EmailConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("email config: %w", err)
	}
	if cfg.SMTPHost == "" {
		return fmt.Errorf("smtp_host is empty")
	}
	if len(cfg.To) == 0 {
		return fmt.Errorf("no recipients configured")
	}

	port := cfg.SMTPPort
	if port == 0 {
		if cfg.UseTLS {
			port = 465
		} else {
			port = 587
		}
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, port)

	subject := fmt.Sprintf("[LogLynx Alert] %s - %s", severityLabel(msg.Severity), msg.RuleName)
	body := fmt.Sprintf(`LogLynx Alert Triggered

Rule:       %s
Severity:   %s
Group by:   %s
Group value:%s
Matched:    %d requests
Threshold:  %d requests
Window:     %s
Cooldown:   %s
Time:       %s

What happened:
%s

--
LogLynx Alert System
`, msg.RuleName, strings.ToUpper(msg.Severity), msg.GroupBy, msg.GroupDisplay(), msg.Count,
		msg.ThresholdCount, msg.WindowDisplay(), msg.CooldownDisplay(), msg.TriggeredAt.UTC().Format("2006-01-02 15:04:05 UTC"),
		msg.DescriptionDisplay())

	mimeMessage := buildMIMEMessage(cfg.From, cfg.To, subject, body)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	if cfg.UseTLS {
		return sendWithTLS(addr, cfg.SMTPHost, auth, cfg.From, cfg.To, mimeMessage)
	}
	return sendWithSTARTTLS(addr, cfg.SMTPHost, auth, cfg.From, cfg.To, mimeMessage, cfg.StartTLS)
}

func buildMIMEMessage(from string, to []string, subject, body string) []byte {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return []byte(sb.String())
}

func sendWithTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsCfg := &tls.Config{ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()
	return sendViaSMTPClient(client, auth, from, to, msg)
}

func sendWithSTARTTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte, useStartTLS bool) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp dial: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if useStartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	return sendViaSMTPClient(client, auth, from, to, msg)
}

func sendViaSMTPClient(client *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp RCPT %s: %w", recipient, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	return w.Close()
}

func severityLabel(s string) string {
	switch s {
	case "critical":
		return "CRITICAL"
	case "info":
		return "INFO"
	default:
		return "WARNING"
	}
}
