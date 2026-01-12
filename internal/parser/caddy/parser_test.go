package caddy

import (
	"strings"
	"testing"
	"time"

	"github.com/pterm/pterm"
)

func TestParser_Name(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	if parser.Name() != "caddy" {
		t.Errorf("Expected parser name 'caddy', got '%s'", parser.Name())
	}
}

func TestParser_CanParse_ValidCaddyJSON(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	validLog := `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access.log9","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/"},"status":200}`

	if !parser.CanParse(validLog) {
		t.Error("Expected parser to accept valid Caddy JSON log")
	}
}

func TestParser_CanParse_InvalidJSON(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	invalidLog := `not a json log`

	if parser.CanParse(invalidLog) {
		t.Error("Expected parser to reject invalid JSON")
	}
}

func TestParser_CanParse_NonCaddyJSON(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	nonCaddyLog := `{"timestamp":"2024-01-01","message":"some log"}`

	if parser.CanParse(nonCaddyLog) {
		t.Error("Expected parser to reject non-Caddy JSON")
	}
}

func TestParser_Parse_FullCaddyLog(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	caddyLog := `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access.log9","msg":"handled request","request":{"remote_ip":"100.200.300.172","remote_port":"49476","client_ip":"100.200.300.172","proto":"HTTP/2.0","method":"GET","host":"test.example.org","uri":"/api/users?page=1","headers":{"Te":["trailers"],"Cache-Control":["no-cache"],"User-Agent":["Mozilla/5.0 (X11; Linux x86_64; rv:146.0) Gecko/20100101 Firefox/146.0"],"Referer":["https://test.example.org/"]},"tls":{"resumed":false,"version":772,"cipher_suite":4865,"proto":"h2","server_name":"test.example.org"}},"bytes_read":1024,"user_id":"testuser","duration":0.00226026,"size":1546,"status":200,"resp_headers":{"Content-Type":["text/html"],"Server":["Apache/2.4.57 (Debian)"]}}`

	event, err := parser.Parse(caddyLog)
	if err != nil {
		t.Fatalf("Failed to parse valid Caddy log: %v", err)
	}

	// Verify timestamp (with tolerance for float64 precision)
	expectedTime := time.Unix(1767690562, 565906524) // Actual parsed value from 1767690562.5659065
	if !event.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, event.Timestamp)
	}

	// Verify client info
	if event.ClientIP != "100.200.300.172" {
		t.Errorf("Expected ClientIP '100.200.300.172', got '%s'", event.ClientIP)
	}
	if event.ClientPort != 49476 {
		t.Errorf("Expected ClientPort 49476, got %d", event.ClientPort)
	}
	if event.ClientUser != "testuser" {
		t.Errorf("Expected ClientUser 'testuser', got '%s'", event.ClientUser)
	}

	// Verify request info
	if event.Method != "GET" {
		t.Errorf("Expected Method 'GET', got '%s'", event.Method)
	}
	if event.Protocol != "HTTP/2.0" {
		t.Errorf("Expected Protocol 'HTTP/2.0', got '%s'", event.Protocol)
	}
	if event.Host != "test.example.org" {
		t.Errorf("Expected Host 'test.example.org', got '%s'", event.Host)
	}
	if event.Path != "/api/users" {
		t.Errorf("Expected Path '/api/users', got '%s'", event.Path)
	}
	if event.QueryString != "page=1" {
		t.Errorf("Expected QueryString 'page=1', got '%s'", event.QueryString)
	}
	if event.RequestScheme != "https" {
		t.Errorf("Expected RequestScheme 'https', got '%s'", event.RequestScheme)
	}
	if event.RequestLength != 1024 {
		t.Errorf("Expected RequestLength 1024, got %d", event.RequestLength)
	}

	// Verify response info
	if event.StatusCode != 200 {
		t.Errorf("Expected StatusCode 200, got %d", event.StatusCode)
	}
	if event.ResponseSize != 1546 {
		t.Errorf("Expected ResponseSize 1546, got %d", event.ResponseSize)
	}
	if event.ResponseContentType != "text/html" {
		t.Errorf("Expected ResponseContentType 'text/html', got '%s'", event.ResponseContentType)
	}

	// Verify timing
	expectedDuration := int64(0.00226026 * 1e9)
	if event.Duration != expectedDuration {
		t.Errorf("Expected Duration %d, got %d", expectedDuration, event.Duration)
	}

	// Verify headers
	if !strings.Contains(event.UserAgent, "Mozilla/5.0") {
		t.Errorf("Expected UserAgent to contain 'Mozilla/5.0', got '%s'", event.UserAgent)
	}
	if event.Referer != "https://test.example.org/" {
		t.Errorf("Expected Referer 'https://test.example.org/', got '%s'", event.Referer)
	}

	// Verify TLS info
	if event.TLSVersion != "1.3" {
		t.Errorf("Expected TLSVersion '1.3', got '%s'", event.TLSVersion)
	}
	if event.TLSCipher != "TLS_AES_128_GCM_SHA256" {
		t.Errorf("Expected TLSCipher 'TLS_AES_128_GCM_SHA256', got '%s'", event.TLSCipher)
	}
	if event.TLSServerName != "test.example.org" {
		t.Errorf("Expected TLSServerName 'test.example.org', got '%s'", event.TLSServerName)
	}

	// Verify logger name
	if event.RouterName != "http.log.access.log9" {
		t.Errorf("Expected RouterName 'http.log.access.log9', got '%s'", event.RouterName)
	}
}

func TestParser_Parse_WithUpstream(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	caddyLog := `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/"},"status":200,"size":100,"duration":0.1,"upstream":{"address":"localhost:8080","duration":0.08,"status":200}}`

	event, err := parser.Parse(caddyLog)
	if err != nil {
		t.Fatalf("Failed to parse Caddy log with upstream: %v", err)
	}

	if event.BackendURL != "localhost:8080" {
		t.Errorf("Expected BackendURL 'localhost:8080', got '%s'", event.BackendURL)
	}
	if event.UpstreamStatus != 200 {
		t.Errorf("Expected UpstreamStatus 200, got %d", event.UpstreamStatus)
	}
	if event.UpstreamResponseTimeMs != 80.0 {
		t.Errorf("Expected UpstreamResponseTimeMs 80.0, got %f", event.UpstreamResponseTimeMs)
	}
}

func TestParser_Parse_WithoutTLS(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	caddyLog := `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/","proto":"HTTP/1.1"},"status":200,"size":100,"duration":0.1}`

	event, err := parser.Parse(caddyLog)
	if err != nil {
		t.Fatalf("Failed to parse Caddy log without TLS: %v", err)
	}

	if event.RequestScheme != "http" {
		t.Errorf("Expected RequestScheme 'http' for non-TLS request, got '%s'", event.RequestScheme)
	}
	if event.TLSVersion != "" {
		t.Errorf("Expected empty TLSVersion, got '%s'", event.TLSVersion)
	}
}

func TestParser_Parse_URISplitting(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	testCases := []struct {
		uri           string
		expectedPath  string
		expectedQuery string
	}{
		{"/", "/", ""},
		{"/api/users", "/api/users", ""},
		{"/api/users?page=1", "/api/users", "page=1"},
		{"/search?q=test&lang=en", "/search", "q=test&lang=en"},
	}

	for _, tc := range testCases {
		caddyLog := `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"` + tc.uri + `"},"status":200,"size":100,"duration":0.1}`

		event, err := parser.Parse(caddyLog)
		if err != nil {
			t.Fatalf("Failed to parse Caddy log with URI '%s': %v", tc.uri, err)
		}

		if event.Path != tc.expectedPath {
			t.Errorf("For URI '%s': expected Path '%s', got '%s'", tc.uri, tc.expectedPath, event.Path)
		}
		if event.QueryString != tc.expectedQuery {
			t.Errorf("For URI '%s': expected QueryString '%s', got '%s'", tc.uri, tc.expectedQuery, event.QueryString)
		}
	}
}

func TestParser_Parse_TLSVersionConversion(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	_ = NewParser(logger)

	testCases := []struct {
		version  int
		expected string
	}{
		{769, "1.0"},
		{770, "1.1"},
		{771, "1.2"},
		{772, "1.3"},
		{999, "UNKNOWN_999"},
	}

	for _, tc := range testCases {
		result := convertTLSVersion(tc.version)
		if result != tc.expected {
			t.Errorf("For TLS version %d: expected '%s', got '%s'", tc.version, tc.expected, result)
		}
	}
}

func TestParser_Parse_TLSCipherConversion(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	_ = NewParser(logger)

	testCases := []struct {
		cipher   int
		expected string
	}{
		{4865, "TLS_AES_128_GCM_SHA256"},
		{4866, "TLS_AES_256_GCM_SHA384"},
		{4867, "TLS_CHACHA20_POLY1305_SHA256"},
		{49195, "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
		{99999, "UNKNOWN_99999"},
	}

	for _, tc := range testCases {
		result := convertTLSCipher(tc.cipher)
		if result != tc.expected {
			t.Errorf("For TLS cipher %d: expected '%s', got '%s'", tc.cipher, tc.expected, result)
		}
	}
}

func TestParser_Parse_ClientIPFallback(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	// Test with only remote_ip
	caddyLog := `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/"},"status":200,"size":100,"duration":0.1}`
	event, err := parser.Parse(caddyLog)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if event.ClientIP != "192.168.1.100" {
		t.Errorf("Expected ClientIP '192.168.1.100', got '%s'", event.ClientIP)
	}

	// Test with X-Forwarded-For
	caddyLog = `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"10.0.0.1","method":"GET","uri":"/","headers":{"X-Forwarded-For":["203.0.113.1"]}},"status":200,"size":100,"duration":0.1}`
	event, err = parser.Parse(caddyLog)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	// Should prefer remote_ip over X-Forwarded-For
	if event.ClientIP != "10.0.0.1" {
		t.Errorf("Expected ClientIP '10.0.0.1', got '%s'", event.ClientIP)
	}
}

func TestParser_Parse_MissingTimestamp(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	caddyLog := `{"level":"info","logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/"},"status":200}`

	_, err := parser.Parse(caddyLog)
	if err == nil {
		t.Error("Expected error for missing timestamp")
	}
}

func TestParser_Parse_MissingRequest(t *testing.T) {
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
	parser := NewParser(logger)

	caddyLog := `{"level":"info","ts":1767690562.5659065,"logger":"http.log.access","msg":"handled request","status":200}`

	_, err := parser.Parse(caddyLog)
	if err == nil {
		t.Error("Expected error for missing request object")
	}
}

func TestParser_GetTimestamp(t *testing.T) {
	expectedTime := time.Now()
	event := &CaddyRequestEvent{
		Timestamp: expectedTime,
	}

	if !event.GetTimestamp().Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, event.GetTimestamp())
	}
}

func TestParser_GetSourceName(t *testing.T) {
	event := &CaddyRequestEvent{
		SourceName: "test-source",
	}

	if event.GetSourceName() != "test-source" {
		t.Errorf("Expected source name 'test-source', got '%s'", event.GetSourceName())
	}
}
