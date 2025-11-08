package traefik

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// LogFormat represents the format of Traefik logs
type LogFormat int

const (
	// FormatUnknown represents an unknown log format
	FormatUnknown LogFormat = iota
	// FormatJSON represents JSON formatted logs
	FormatJSON
	// FormatCLF represents Common Log Format (text) logs
	FormatCLF
)

// Parser implements the LogParser interface for Traefik logs
type Parser struct {
	logger   *pterm.Logger
	clfRegex *regexp.Regexp
}

// CLF regex pattern for Traefik Common Log Format
// Format: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>" <requestsTotal> "<router>" "<server_URL>" <duration>ms
const traefikCLFPattern = `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)" (\d+) "([^"]*)" "([^"]*)" (\d+)ms`

// Generic CLF pattern (without Traefik-specific fields)
// Format: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>"
const genericCLFPattern = `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"`

// NewParser creates a new Traefik parser instance
func NewParser(logger *pterm.Logger) *Parser {
	// Compile regex patterns (try Traefik CLF first, fall back to generic CLF)
	clfRegex := regexp.MustCompile(traefikCLFPattern)

	return &Parser{
		logger:   logger,
		clfRegex: clfRegex,
	}
}

// Name returns the parser identifier
func (p *Parser) Name() string {
	return "traefik"
}

// CanParse checks if the log line is in Traefik JSON or CLF format
func (p *Parser) CanParse(line string) bool {
	if line == "" {
		return false
	}

	// Try to detect format
	format := p.detectFormat(line)
	return format != FormatUnknown
}

// detectFormat determines whether the log line is JSON, CLF, or unknown
func (p *Parser) detectFormat(line string) LogFormat {
	if line == "" {
		return FormatUnknown
	}

	// Try JSON first
	if line[0] == '{' {
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			// Check for required fields (time and client IP)
			_, hasTime := raw["time"]
			_, hasClientIP := raw["request_X-Real-Ip"]

			if hasTime && hasClientIP {
				return FormatJSON
			}
		}
	}

	// Try CLF format (both Traefik and generic)
	if p.clfRegex.MatchString(line) {
		return FormatCLF
	}

	// Try generic CLF pattern as fallback
	genericRegex := regexp.MustCompile(genericCLFPattern)
	if genericRegex.MatchString(line) {
		return FormatCLF
	}

	return FormatUnknown
}

// Parse parses a Traefik log line (JSON or CLF format) into an HTTPRequestEvent
func (p *Parser) Parse(line string) (*HTTPRequestEvent, error) {
	if line == "" {
		return nil, fmt.Errorf("empty log line")
	}

	// Detect format and route to appropriate parser
	format := p.detectFormat(line)
	switch format {
	case FormatJSON:
		return p.parseJSON(line)
	case FormatCLF:
		return p.parseCLF(line)
	default:
		return nil, fmt.Errorf("unknown log format")
	}
}

// parseJSON parses a Traefik JSON log line into an HTTPRequestEvent
func (p *Parser) parseJSON(line string) (*HTTPRequestEvent, error) {
	var raw map[string]any
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

	// Extract client hostname (may be same as IP or actual hostname)
	clientHostname := getString(raw, "ClientHost")

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
		ClientIP:       ip,
		ClientPort:     port,
		ClientHostname: clientHostname, // May be hostname or same as IP

		// Request info
		Method:        strings.ToUpper(method),
		Protocol:      getString(raw, "RequestProtocol"),
		Host:          getString(raw, "request_Host"),
		Path:          path,
		QueryString:   queryString,
		RequestScheme: getString(raw, "request_X-Forwarded-Proto"), // http or https

		// Response info
		StatusCode:          getInt(raw, "DownstreamStatus"),
		ResponseSize:        getInt64(raw, "DownstreamContentSize"),
		ResponseTimeMs:      getDuration(raw, "Duration") / 1000000, // Convert nanoseconds to milliseconds
		ResponseContentType: getString(raw, "downstream_Content-Type"),

		// Detailed timing (for hash calculation precision)
		Duration:      int64(getDuration(raw, "Duration")), // Nanoseconds
		StartUTC:      getString(raw, "StartUTC"),          // Timestamp with nanosecond precision
		RetryAttempts: getInt(raw, "RetryAttempts"),
		RequestsTotal: getInt(raw, "RequestsTotal"), // Total requests at router level (defaults to 0 if not present)

		// Headers
		UserAgent: getString(raw, "request_User-Agent"),
		Referer:   getString(raw, "request_Referer"),

		// Traefik-specific (may not be present)
		BackendName:         getString(raw, "ServiceName"),
		BackendURL:          getString(raw, "backend_URL"),
		RouterName:          getString(raw, "router_Name"),
		UpstreamContentType: getString(raw, "origin_Content-Type"),

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

// parseCLF parses a Traefik Common Log Format (CLF) log line into an HTTPRequestEvent
func (p *Parser) parseCLF(line string) (*HTTPRequestEvent, error) {
	// Try Traefik CLF pattern first
	matches := p.clfRegex.FindStringSubmatch(line)

	// If Traefik pattern doesn't match, try generic CLF
	if matches == nil {
		genericRegex := regexp.MustCompile(genericCLFPattern)
		matches = genericRegex.FindStringSubmatch(line)

		if matches == nil {
			return nil, fmt.Errorf("line does not match CLF format")
		}

		// Parse generic CLF (fewer fields)
		return p.parseGenericCLF(matches)
	}

	// Parse Traefik-specific CLF (more fields)
	return p.parseTraefikCLF(matches)
}

// parseTraefikCLF parses a Traefik CLF line with all Traefik-specific fields
// Format: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>" <requestsTotal> "<router>" "<server_URL>" <duration>ms
func (p *Parser) parseTraefikCLF(matches []string) (*HTTPRequestEvent, error) {
	if len(matches) < 14 {
		return nil, fmt.Errorf("invalid Traefik CLF format: insufficient fields")
	}

	// Extract fields from regex capture groups
	clientHost := matches[1] // Client IP (possibly with port)
	// matches[2] is userid (usually "-")
	timestampStr := matches[3]      // Timestamp
	method := matches[4]            // HTTP method
	requestPath := matches[5]       // Request path
	statusStr := matches[6]         // Status code
	sizeStr := matches[7]           // Response size
	referer := matches[8]           // Referer
	userAgent := matches[9]         // User agent
	requestsTotalStr := matches[10] // Total requests at router level
	backendName := matches[11]      // Traefik backend (also known as router) name
	backendURL := matches[12]       // Backend URL
	durationStr := matches[13]      // Request duration in ms

	// Parse timestamp (CLF format: "02/Jan/2006:15:04:05 -0700")
	timestamp, err := time.Parse("02/Jan/2006:15:04:05 -0700", timestampStr)
	if err != nil {
		p.logger.WithCaller().Debug("Failed to parse timestamp, using current time",
			p.logger.Args("timestamp", timestampStr, "error", err))
		timestamp = time.Now()
	}

	// Parse client IP and port
	ip, port := parseClientHost(clientHost)

	// Parse status code
	statusCode, _ := strconv.Atoi(statusStr)
	if statusCode < 100 || statusCode >= 600 {
		p.logger.WithCaller().Debug("Invalid status code", p.logger.Args("status", statusCode))
		statusCode = 0
	}

	// Parse response size
	var responseSize int64
	if sizeStr != "-" {
		responseSize, _ = strconv.ParseInt(sizeStr, 10, 64)
	}

	// Parse duration (already in milliseconds in CLF format)
	durationMs, _ := strconv.ParseFloat(durationStr, 64)

	// Convert milliseconds to nanoseconds for Duration field (consistent with JSON format)
	durationNs := int64(durationMs * 1000000) // ms to ns

	// Parse requestsTotal (total number of requests at router level)
	requestsTotal, _ := strconv.Atoi(requestsTotalStr)
	if requestsTotal < 0 {
		requestsTotal = 0 // Ensure non-negative
	}

	// For CLF logs, we construct a StartUTC from timestamp since it's not in the log
	// This gives us second-level precision (CLF only has second precision)
	// Format matches Traefik's StartUTC format for consistency
	startUTC := timestamp.Format(time.RFC3339Nano)

	// Extract path and query string
	path := requestPath
	queryString := ""
	if idx := strings.Index(path, "?"); idx != -1 {
		queryString = path[idx+1:]
		path = path[:idx]
	}

	// Handle empty or "-" values
	if referer == "-" {
		referer = ""
	}
	if userAgent == "-" {
		userAgent = ""
	}
	if backendName == "-" {
		backendName = ""
	}
	if backendURL == "-" {
		backendURL = ""
	}

	redirectTarget := extractRedirectTarget(queryString)

	// Build event
	event := &HTTPRequestEvent{
		Timestamp:  timestamp,
		SourceName: "", // Will be set by ingestion engine

		// Client info
		ClientIP:       ip,
		ClientPort:     port,
		ClientHostname: "", // Not available in CLF

		// Request info
		Method:        strings.ToUpper(method),
		Protocol:      "", // Not available in CLF
		Host:          "", // Not available in CLF
		Path:          path,
		QueryString:   queryString,
		RequestScheme: "", // Not available in CLF

		// Response info
		StatusCode:          statusCode,
		ResponseSize:        responseSize,
		ResponseTimeMs:      durationMs,
		ResponseContentType: "", // Not available in CLF

		// Detailed timing (for hash calculation precision)
		Duration:      durationNs,    // Converted from ms to ns
		StartUTC:      startUTC,      // Constructed from CLF timestamp
		RetryAttempts: 0,             // Not available in CLF
		RequestsTotal: requestsTotal, // Total requests at router level

		// Headers
		UserAgent: userAgent,
		Referer:   referer,

		// Traefik-specific
		BackendName:         backendName, // ServiceName not in CLF
		BackendURL:          backendURL,
		RouterName:          "",
		UpstreamContentType: "", // Not available in CLF

		// TLS info
		TLSVersion: "", // Not available in CLF
		TLSCipher:  "", // Not available in CLF

		// Tracing
		RequestID: "", // Not available in CLF
	}

	if event.Referer == "" && redirectTarget != "" {
		event.Referer = redirectTarget
		p.logger.Trace("Filled referer from redirect parameter",
			p.logger.Args("client_ip", event.ClientIP, "redirect", redirectTarget))
	}

	// Log trace for successful parse
	p.logger.Trace("Successfully parsed Traefik CLF log",
		p.logger.Args(
			"timestamp", event.Timestamp.Format(time.RFC3339),
			"client_ip", event.ClientIP,
			"method", event.Method,
			"path", event.Path,
			"status", event.StatusCode,
		))

	return event, nil
}

// parseGenericCLF parses a generic CLF line (without Traefik-specific fields)
// Format: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>"
func (p *Parser) parseGenericCLF(matches []string) (*HTTPRequestEvent, error) {
	if len(matches) < 10 {
		return nil, fmt.Errorf("invalid generic CLF format: insufficient fields")
	}

	// Extract fields from regex capture groups
	clientHost := matches[1]   // Client IP
	timestampStr := matches[3] // Timestamp
	method := matches[4]       // HTTP method
	requestPath := matches[5]  // Request path
	statusStr := matches[6]    // Status code
	sizeStr := matches[7]      // Response size
	referer := matches[8]      // Referer
	userAgent := matches[9]    // User agent

	// Parse timestamp
	timestamp, err := time.Parse("02/Jan/2006:15:04:05 -0700", timestampStr)
	if err != nil {
		p.logger.WithCaller().Debug("Failed to parse timestamp, using current time",
			p.logger.Args("timestamp", timestampStr, "error", err))
		timestamp = time.Now()
	}

	// Parse client IP and port
	ip, port := parseClientHost(clientHost)

	// Parse status code
	statusCode, _ := strconv.Atoi(statusStr)
	if statusCode < 100 || statusCode >= 600 {
		statusCode = 0
	}

	// Parse response size
	var responseSize int64
	if sizeStr != "-" {
		responseSize, _ = strconv.ParseInt(sizeStr, 10, 64)
	}

	// For generic CLF, we don't have duration in the log, so it will be 0
	// StartUTC is constructed from timestamp (second-level precision)
	startUTC := timestamp.Format(time.RFC3339Nano)

	// Extract path and query string
	path := requestPath
	queryString := ""
	if idx := strings.Index(path, "?"); idx != -1 {
		queryString = path[idx+1:]
		path = path[:idx]
	}

	// Handle "-" values
	if referer == "-" {
		referer = ""
	}
	if userAgent == "-" {
		userAgent = ""
	}

	redirectTarget := extractRedirectTarget(queryString)

	// Build event
	event := &HTTPRequestEvent{
		Timestamp:  timestamp,
		SourceName: "",

		// Client info
		ClientIP:       ip,
		ClientPort:     port,
		ClientHostname: "",

		// Request info
		Method:        strings.ToUpper(method),
		Protocol:      "",
		Host:          "",
		Path:          path,
		QueryString:   queryString,
		RequestScheme: "",

		// Response info
		StatusCode:          statusCode,
		ResponseSize:        responseSize,
		ResponseTimeMs:      0, // Not available in generic CLF
		ResponseContentType: "",

		// Detailed timing (for hash calculation precision)
		Duration:      0,        // Not available in generic CLF
		StartUTC:      startUTC, // Constructed from CLF timestamp
		RetryAttempts: 0,

		// Headers
		UserAgent: userAgent,
		Referer:   referer,

		// Proxy/Upstream info
		BackendName:         "",
		BackendURL:          "",
		RouterName:          "",
		UpstreamContentType: "",

		// TLS info
		TLSVersion: "",
		TLSCipher:  "",

		// Tracing
		RequestID: "",
	}

	if event.Referer == "" && redirectTarget != "" {
		event.Referer = redirectTarget
	}

	p.logger.Trace("Successfully parsed generic CLF log",
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
func getString(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

// getInt safely extracts an integer value from the map
func getInt(m map[string]any, key string) int {
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
func getInt64(m map[string]any, key string) int64 {
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
func getDuration(m map[string]any, key string) float64 {
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
func parseTime(val any) time.Time {
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
