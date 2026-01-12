package caddy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// Parser implements the LogParser interface for Caddy access logs
type Parser struct {
	logger *pterm.Logger
}

// NewParser creates a new Caddy parser instance
func NewParser(logger *pterm.Logger) *Parser {
	return &Parser{
		logger: logger,
	}
}

// Name returns the parser name
func (p *Parser) Name() string {
	return "caddy"
}

// CanParse checks if the log line is in Caddy JSON format
func (p *Parser) CanParse(line string) bool {
	if len(line) == 0 || line[0] != '{' {
		return false
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return false
	}

	// Check for Caddy-specific fields
	logger, hasLogger := raw["logger"].(string)
	_, hasRequest := raw["request"]

	// Caddy access logs have logger starting with "http.log.access"
	return hasLogger && strings.HasPrefix(logger, "http.log.access") && hasRequest
}

// Parse parses a Caddy JSON log line into a CaddyRequestEvent
func (p *Parser) Parse(line string) (*CaddyRequestEvent, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Extract timestamp (Unix float)
	ts := getFloat64(raw, "ts")
	if ts == 0 {
		return nil, fmt.Errorf("missing or invalid timestamp")
	}
	timestamp := parseUnixTimestamp(ts)

	// Extract request object
	request, ok := raw["request"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing request object")
	}

	// Extract client IP with fallback logic
	clientIP := getStringFromMap(request, "client_ip")
	if clientIP == "" {
		clientIP = getStringFromMap(request, "remote_ip")
	}
	if clientIP == "" {
		// Try X-Forwarded-For header
		headers, _ := request["headers"].(map[string]any)
		clientIP = extractHeaderArray(headers, "X-Forwarded-For")
	}

	// Extract client port
	clientPort := getIntFromMap(request, "remote_port")

	// Extract URI and split into path + query
	uri := getStringFromMap(request, "uri")
	path, queryString := splitURI(uri)

	// Extract method, protocol, host
	method := getStringFromMap(request, "method")
	protocol := getStringFromMap(request, "proto")
	host := getStringFromMap(request, "host")

	// Determine request scheme from TLS presence
	tls, hasTLS := request["tls"].(map[string]any)
	requestScheme := "http"
	if hasTLS {
		requestScheme = "https"
	}

	// Extract TLS info
	tlsVersion := ""
	tlsCipher := ""
	tlsServerName := ""
	if hasTLS {
		tlsVersion = convertTLSVersion(getIntFromMap(tls, "version"))
		tlsCipher = convertTLSCipher(getIntFromMap(tls, "cipher_suite"))
		tlsServerName = getStringFromMap(tls, "server_name")
	}

	// Extract status code, response size, duration
	statusCode := getInt(raw, "status")
	responseSize := getInt64(raw, "size")
	duration := getFloat64(raw, "duration")
	responseTimeMs := duration * 1000 // Convert to milliseconds

	// Extract response content type
	responseContentType := extractResponseHeader(raw, "Content-Type")

	// Extract headers
	headers, _ := request["headers"].(map[string]any)
	userAgent := extractHeaderArray(headers, "User-Agent")
	referer := extractHeaderArray(headers, "Referer")

	// Extract upstream info
	upstream, hasUpstream := raw["upstream"].(map[string]any)
	backendURL := ""
	upstreamStatus := 0
	upstreamResponseTimeMs := 0.0
	if hasUpstream {
		backendURL = getStringFromMap(upstream, "address")
		upstreamStatus = getIntFromMap(upstream, "status")
		upstreamDuration := getFloat64FromMap(upstream, "duration")
		upstreamResponseTimeMs = upstreamDuration * 1000
	}

	// Extract logger name (can be used as RouterName)
	loggerName := getString(raw, "logger")

	// Extract user_id if present
	userID := getString(raw, "user_id")

	// Extract bytes_read
	bytesRead := getInt64(raw, "bytes_read")

	// Build event
	event := &CaddyRequestEvent{
		Timestamp:  timestamp,
		SourceName: "", // Set by processor

		ClientIP:   clientIP,
		ClientPort: clientPort,
		ClientUser: userID,

		Method:        method,
		Protocol:      protocol,
		Host:          host,
		Path:          path,
		QueryString:   queryString,
		RequestLength: bytesRead,
		RequestScheme: requestScheme,

		StatusCode:          statusCode,
		ResponseSize:        responseSize,
		ResponseTimeMs:      responseTimeMs,
		ResponseContentType: responseContentType,

		Duration:               int64(duration * 1e9), // Convert to nanoseconds
		StartUTC:               timestamp.Format(time.RFC3339Nano),
		UpstreamResponseTimeMs: upstreamResponseTimeMs,

		UserAgent: userAgent,
		Referer:   referer,

		BackendURL:     backendURL,
		RouterName:     loggerName,
		UpstreamStatus: upstreamStatus,

		TLSVersion:    tlsVersion,
		TLSCipher:     tlsCipher,
		TLSServerName: tlsServerName,
	}

	return event, nil
}

// Helper functions

// parseUnixTimestamp converts a Unix timestamp (float) to time.Time
func parseUnixTimestamp(ts float64) time.Time {
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

// splitURI splits a URI into path and query string
func splitURI(uri string) (path, query string) {
	if idx := strings.Index(uri, "?"); idx != -1 {
		return uri[:idx], uri[idx+1:]
	}
	return uri, ""
}

// convertTLSVersion converts TLS version code to string
func convertTLSVersion(version int) string {
	switch version {
	case 769:
		return "1.0"
	case 770:
		return "1.1"
	case 771:
		return "1.2"
	case 772:
		return "1.3"
	default:
		if version > 0 {
			return fmt.Sprintf("UNKNOWN_%d", version)
		}
		return ""
	}
}

// convertTLSCipher converts TLS cipher suite code to string
func convertTLSCipher(cipher int) string {
	// Map common cipher suites
	cipherMap := map[int]string{
		// TLS 1.3 cipher suites
		4865: "TLS_AES_128_GCM_SHA256",
		4866: "TLS_AES_256_GCM_SHA384",
		4867: "TLS_CHACHA20_POLY1305_SHA256",
		// TLS 1.2 cipher suites (common ones)
		49195: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		49199: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		49200: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		49196: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		52392: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
		52393: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
	}

	if name, ok := cipherMap[cipher]; ok {
		return name
	}

	if cipher > 0 {
		return fmt.Sprintf("UNKNOWN_%d", cipher)
	}
	return ""
}

// extractHeaderArray extracts a header value from an array
func extractHeaderArray(headers map[string]any, name string) string {
	if headers == nil {
		return ""
	}

	headerValue, ok := headers[name].([]any)
	if !ok || len(headerValue) == 0 {
		return ""
	}

	value, _ := headerValue[0].(string)
	return value
}

// extractResponseHeader extracts a response header value
func extractResponseHeader(raw map[string]any, name string) string {
	respHeaders, ok := raw["resp_headers"].(map[string]any)
	if !ok {
		return ""
	}
	return extractHeaderArray(respHeaders, name)
}

// Type-safe extraction helpers

func getString(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getStringFromMap(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	switch val := m[key].(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return 0
}

func getIntFromMap(m map[string]any, key string) int {
	switch val := m[key].(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return 0
}

func getInt64(m map[string]any, key string) int64 {
	switch val := m[key].(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func getFloat64(m map[string]any, key string) float64 {
	switch val := m[key].(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}

func getFloat64FromMap(m map[string]any, key string) float64 {
	switch val := m[key].(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}
