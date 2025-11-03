package traefik

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// Parser implements the LogParser interface for Traefik logs
type Parser struct {
	logger *pterm.Logger
}

// NewParser creates a new Traefik parser instance
func NewParser(logger *pterm.Logger) *Parser {
	return &Parser{
		logger: logger,
	}
}

// Name returns the parser identifier
func (p *Parser) Name() string {
	return "traefik"
}

// CanParse checks if the log line is in Traefik JSON format
func (p *Parser) CanParse(line string) bool {
	if line == "" {
		return false
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return false
	}

	// Check for required fields (time and client IP)
	_, hasTime := raw["time"]
	_, hasClientIP := raw["request_X-Real-Ip"]

	return hasTime && hasClientIP
}

// Parse parses a Traefik JSON log line into an HTTPRequestEvent
func (p *Parser) Parse(line string) (*HTTPRequestEvent, error) {
	if line == "" {
		return nil, fmt.Errorf("empty log line")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		p.logger.WithCaller().Warn("Failed to parse JSON log line", p.logger.Args("error", err))
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Extract and validate required fields
	clientIP := getString(raw, "request_X-Real-Ip")
	method := getString(raw, "RequestMethod")
	if method == "" {
		method = "GET" // Default if not present
	}

	if clientIP == "" {
		p.logger.WithCaller().Warn("Missing required field: request_X-Real-Ip")
		return nil, fmt.Errorf("missing required field: request_X-Real-Ip")
	}

	// Parse timestamp
	timestamp := parseTime(raw["time"])
	if timestamp.IsZero() {
		p.logger.WithCaller().Debug("Invalid or missing timestamp, using current time")
		timestamp = time.Now()
	}

	// Extract client IP and port
	ip, port := parseClientHost(clientIP)

	// Extract query string from path if present
	path := getString(raw, "RequestPath")
	if path == "" {
		path = "/"
	}
	queryString := ""
	if idx := strings.Index(path, "?"); idx != -1 {
		queryString = path[idx+1:]
		path = path[:idx]
	}

	redirectTarget := extractRedirectTarget(queryString)

	// Build complete event
	event := &HTTPRequestEvent{
		Timestamp:  timestamp,
		SourceName: "", // Will be set by ingestion engine

		// Client info
		ClientIP:   ip,
		ClientPort: port,

		// Request info
		Method:      strings.ToUpper(method),
		Protocol:    getString(raw, "RequestProtocol"),
		Host:        getString(raw, "request_Host"),
		Path:        path,
		QueryString: queryString,

		// Response info
		StatusCode:     getInt(raw, "DownstreamStatus"),
		ResponseSize:   getInt64(raw, "DownstreamContentSize"),
		ResponseTimeMs: getDuration(raw, "Duration") / 1000000, // Convert nanoseconds to milliseconds

		// Headers
		UserAgent: getString(raw, "request_User-Agent"),
		Referer:   getString(raw, "request_Referer"),

		// Traefik-specific (may not be present)
		BackendName: getString(raw, "ServiceName"),
		BackendURL:  getString(raw, "backend_URL"),
		RouterName:  getString(raw, "router_Name"),

		// TLS info
		TLSVersion: getString(raw, "TLSVersion"),
		TLSCipher:  getString(raw, "TLSCipher"),

		// Tracing
		RequestID: getString(raw, "request_X-Request-Id"),
	}

	if event.Referer == "" && redirectTarget != "" {
		event.Referer = redirectTarget
		p.logger.Trace("Filled referer from redirect parameter",
			p.logger.Args("client_ip", event.ClientIP, "redirect", redirectTarget))
	}

	// Validate status code
	if event.StatusCode < 100 || event.StatusCode >= 600 {
		p.logger.WithCaller().Debug("Invalid status code, using 0", p.logger.Args("status", event.StatusCode))
		event.StatusCode = 0
	}

	// Log trace for successful parse
	p.logger.Trace("Successfully parsed Traefik log",
		p.logger.Args(
			"timestamp", event.Timestamp.Format(time.RFC3339),
			"client_ip", event.ClientIP,
			"method", event.Method,
			"path", event.Path,
			"status", event.StatusCode,
		))

	return event, nil
}

// Helper functions

func extractRedirectTarget(queryString string) string {
	if queryString == "" {
		return ""
	}

	values, err := url.ParseQuery(queryString)
	if err != nil {
		return ""
	}

	redirect := values.Get("redirect")
	if redirect == "" {
		return ""
	}

	return redirect
}

// getString safely extracts a string value from the map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

// getInt safely extracts an integer value from the map
func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		}
	}
	return 0
}

// getInt64 safely extracts an int64 value from the map
func getInt64(m map[string]interface{}, key string) int64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		}
	}
	return 0
}

// getDuration safely extracts a duration value from the map
func getDuration(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case int:
			return float64(v)
		}
	}
	return 0
}

// parseTime parses various time formats from Traefik logs
func parseTime(val interface{}) time.Time {
	if val == nil {
		return time.Time{}
	}

	str, ok := val.(string)
	if !ok {
		return time.Time{}
	}

	// Try RFC3339 format first (Traefik default)
	if t, err := time.Parse(time.RFC3339, str); err == nil {
		return t
	}

	// Try RFC3339Nano format
	if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
		return t
	}

	// Try ISO8601 format
	if t, err := time.Parse("2006-01-02T15:04:05Z07:00", str); err == nil {
		return t
	}

	return time.Time{}
}

// parseClientHost extracts IP and port from ClientHost field
// Format can be: "192.168.1.1:12345" or "[2001:db8::1]:12345" or "192.168.1.1"
func parseClientHost(clientHost string) (ip string, port int) {
	if clientHost == "" {
		return "", 0
	}

	// Try to split host and port
	host, portStr, err := net.SplitHostPort(clientHost)
	if err != nil {
		// No port present, return as-is
		return clientHost, 0
	}

	// Parse port
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	return host, port
}
