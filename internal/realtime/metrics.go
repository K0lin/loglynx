package realtime

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

const (
	// QueryTimeout is the maximum time for a database query
	QueryTimeout = 5 * time.Second
	// BufferDuration is the duration of data to keep in memory
	BufferDuration = 60 * time.Second
)

// MetricsCollector collects real-time metrics
type MetricsCollector struct {
	db     *gorm.DB
	logger *pterm.Logger

	// In-memory buffer for real-time metrics
	requestBuffer []*models.HTTPRequest
	bufferMu      sync.RWMutex

	// Current metrics
	mu                sync.RWMutex
	requestRate       float64 // requests per second
	errorRate         float64 // errors per second
	avgResponseTime   float64 // milliseconds
	activeConnections int
	last2xxCount      int64
	last4xxCount      int64
	last5xxCount      int64
	lastUpdate        time.Time
	lastRequestTime   time.Time // timestamp of last request seen

	// Cached per-service metrics (global)
	perServiceMetrics []ServiceMetrics

	// Cached JSON for global metrics (optimization)
	cachedJSON []byte

	// Lifecycle management
	stopChan chan struct{}
	stopped  bool
}

// RealtimeMetrics represents current real-time statistics
type RealtimeMetrics struct {
	RequestRate       float64          `json:"request_rate"`      // req/sec
	ErrorRate         float64          `json:"error_rate"`        // errors/sec
	AvgResponseTime   float64          `json:"avg_response_time"` // ms
	ActiveConnections int              `json:"active_connections"`
	Status2xx         int64            `json:"status_2xx"`
	Status4xx         int64            `json:"status_4xx"`
	Status5xx         int64            `json:"status_5xx"`
	Timestamp         time.Time        `json:"timestamp"`
	TopIPs            []IPMetrics      `json:"top_ips"`
	LatestRequests    []RequestSummary `json:"latest_requests"`
	PerService        []ServiceMetrics `json:"per_service"`
}

// RequestSummary is a lightweight representation of a request for the real-time table
type RequestSummary struct {
	ID             uint      `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Method         string    `json:"method"`
	Host           string    `json:"host"`
	BackendName    string    `json:"backend_name"`
	Path           string    `json:"path"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs float64   `json:"response_time_ms"`
	GeoCountry     string    `json:"geo_country"`
	ClientIP       string    `json:"client_ip"`
}

// IPMetrics represents metrics for a single IP
type IPMetrics struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	RequestRate float64 `json:"request_rate"`
}

// NewMetricsCollector creates a new real-time metrics collector
func NewMetricsCollector(db *gorm.DB, logger *pterm.Logger) *MetricsCollector {
	return &MetricsCollector{
		db:            db,
		logger:        logger,
		lastUpdate:    time.Now(),
		stopChan:      make(chan struct{}),
		requestBuffer: make([]*models.HTTPRequest, 0, 10000),
	}
}

// Ingest adds a new request to the in-memory buffer
// Maintains chronological order by timestamp using optimized insertion
func (m *MetricsCollector) Ingest(req *models.HTTPRequest) {
	m.bufferMu.Lock()
	defer m.bufferMu.Unlock()

	bufLen := len(m.requestBuffer)

	// Fast path: empty buffer or new request is newest
	if bufLen == 0 || !req.Timestamp.Before(m.requestBuffer[bufLen-1].Timestamp) {
		m.requestBuffer = append(m.requestBuffer, req)
		return
	}

	// Slow path: need to insert in correct position
	// Binary search for insertion point
	insertIdx := sort.Search(bufLen, func(i int) bool {
		return !m.requestBuffer[i].Timestamp.Before(req.Timestamp)
	})

	// Insert at correct position
	m.requestBuffer = append(m.requestBuffer, nil)           // Expand slice
	copy(m.requestBuffer[insertIdx+1:], m.requestBuffer[insertIdx:]) // Shift right
	m.requestBuffer[insertIdx] = req                         // Insert
}

// Start begins collecting metrics at regular intervals
func (m *MetricsCollector) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.collectMetrics()
			case <-m.stopChan:
				m.logger.Info("Real-time metrics collector stopped")
				return
			}
		}
	}()
	m.logger.Info("Real-time metrics collector started",
		m.logger.Args("interval", interval.String()))
}

// Stop gracefully stops the metrics collector
func (m *MetricsCollector) Stop() {
	m.mu.Lock()
	if !m.stopped {
		m.stopped = true
		close(m.stopChan)
	}
	m.mu.Unlock()
}

// SetActiveConnections updates the active connection count
func (m *MetricsCollector) SetActiveConnections(n int) {
	m.mu.Lock()
	m.activeConnections = n
	m.mu.Unlock()
}

// GetCachedJSON returns the cached JSON representation of global metrics
func (m *MetricsCollector) GetCachedJSON() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cachedJSON
}

// collectMetrics gathers current statistics from the in-memory buffer
// Uses a sliding window approach for accurate rate calculation without DB queries
func (m *MetricsCollector) collectMetrics() {
	now := time.Now()

	// Use a 5-second sliding window for smoother rates and latency tolerance
	windowDuration := 5 * time.Second
	windowStart := now.Add(-windowDuration)
	oneMinuteAgo := now.Add(-1 * time.Minute)

	m.bufferMu.Lock()
	defer m.bufferMu.Unlock()

	// 1. Prune old requests from buffer (keep only last 60s)
	validIndex := -1
	for i, req := range m.requestBuffer {
		if req.Timestamp.After(oneMinuteAgo) {
			validIndex = i
			break
		}
	}

	if validIndex >= 0 {
		// Create a new slice to allow GC to collect the old array backing
		newBuffer := make([]*models.HTTPRequest, len(m.requestBuffer)-validIndex)
		copy(newBuffer, m.requestBuffer[validIndex:])
		m.requestBuffer = newBuffer
	} else if validIndex == -1 && len(m.requestBuffer) > 0 {
		// All requests are too old
		m.requestBuffer = m.requestBuffer[:0]
	}

	// 2. Calculate metrics from buffer
	var (
		totalCountWindow int64
		errorCountWindow int64
		totalRespTime    float64
		status2xx        int64
		status4xx        int64
		status5xx        int64
		count1m          int64
		lastRequestTime  time.Time
	)

	for _, req := range m.requestBuffer {
		// For rates (last 5s)
		if req.Timestamp.After(windowStart) {
			totalCountWindow++
			totalRespTime += req.ResponseTimeMs
			if req.StatusCode >= 400 {
				errorCountWindow++
			}
			if req.Timestamp.After(lastRequestTime) {
				lastRequestTime = req.Timestamp
			}
		}

		// For distribution (last 1m)
		count1m++
		if req.StatusCode >= 200 && req.StatusCode < 300 {
			status2xx++
		} else if req.StatusCode >= 400 && req.StatusCode < 500 {
			status4xx++
		} else if req.StatusCode >= 500 {
			status5xx++
		}
	}

	// Calculate averages (Instant - last 5s)
	avgRespTime := 0.0
	if totalCountWindow > 0 {
		avgRespTime = totalRespTime / float64(totalCountWindow)
	}

	// Calculate rates per second
	// Use fixed window duration if traffic is active, zero if traffic stopped
	requestRate := 0.0
	errorRate := 0.0

	if totalCountWindow > 0 && !lastRequestTime.IsZero() {
		// Check if traffic is still active (last request within window)
		timeSinceLastRequest := now.Sub(lastRequestTime)

		if timeSinceLastRequest <= windowDuration {
			// Traffic is active - calculate rate over the full window
			// This gives accurate req/s even for bursty traffic
			requestRate = float64(totalCountWindow) / windowDuration.Seconds()
			errorRate = float64(errorCountWindow) / windowDuration.Seconds()
		}
		// else: timeSinceLastRequest > windowDuration means traffic stopped, rates stay 0
	}

	// Calculate Top IPs (last 5s)
	ipCounts := make(map[string]int)
	ipCountries := make(map[string]string)

	for _, req := range m.requestBuffer {
		if req.Timestamp.After(windowStart) {
			// Check filters if any (this logic is shared with GetMetricsWithFilters)
			// But here we are in collectMetrics which is global.
			// Wait, collectMetrics is global. GetMetricsWithFilters is per-request.
			// We should calculate TopIPs here for the global cache.

			ipCounts[req.ClientIP]++
			if _, ok := ipCountries[req.ClientIP]; !ok && req.GeoCountry != "" {
				ipCountries[req.ClientIP] = req.GeoCountry
			}
		}
	}

	var topIPs []IPMetrics
	for ip, count := range ipCounts {
		rate := float64(count) / windowDuration.Seconds()
		// Only include IPs with meaningful activity (> 0.1 req/s)
		// This prevents showing nearly-inactive IPs that are about to expire from window
		if rate > 0.1 {
			topIPs = append(topIPs, IPMetrics{
				IP:          ip,
				Country:     ipCountries[ip],
				RequestRate: rate,
			})
		}
	}

	// Sort by rate desc
	sort.Slice(topIPs, func(i, j int) bool {
		return topIPs[i].RequestRate > topIPs[j].RequestRate
	})

	// Limit to top 10
	if len(topIPs) > 10 {
		topIPs = topIPs[:10]
	}

	// If no recent requests (>windowDuration old), force all metrics to zero immediately
	if !lastRequestTime.IsZero() {
		timeSinceLastRequest := now.Sub(lastRequestTime)
		if timeSinceLastRequest > windowDuration {
			requestRate = 0.0
			errorRate = 0.0
			topIPs = nil
			// Also reset status counts for zero-state
			status2xx = 0
			status4xx = 0
			status5xx = 0
		}
	} else {
		// No requests in window at all
		requestRate = 0.0
		errorRate = 0.0
		topIPs = nil
		status2xx = 0
		status4xx = 0
		status5xx = 0
		// Use previous lastRequestTime if available
		m.mu.RLock()
		lastRequestTime = m.lastRequestTime
		m.mu.RUnlock()
	}

	// Collect per-service metrics (global) - passing nil filters uses buffer
	perServiceMetrics := m.calculatePerServiceMetrics(m.requestBuffer, nil, nil)

	// Get Latest Requests (last 20 from buffer)
	latestRequests := m.getLatestRequests(m.requestBuffer, 20)

	// Prepare metrics struct for JSON caching
	metrics := &RealtimeMetrics{
		RequestRate:       requestRate,
		ErrorRate:         errorRate,
		AvgResponseTime:   avgRespTime,
		ActiveConnections: m.activeConnections,
		Status2xx:         status2xx,
		Status4xx:         status4xx,
		Status5xx:         status5xx,
		Timestamp:         now,
		TopIPs:            topIPs,
		LatestRequests:    latestRequests,
		PerService:        perServiceMetrics,
	}

	// Marshal to JSON immediately for caching
	jsonBytes, _ := json.Marshal(metrics)

	// Update metrics with lock
	m.mu.Lock()
	m.perServiceMetrics = perServiceMetrics
	m.requestRate = requestRate
	m.errorRate = errorRate
	m.avgResponseTime = avgRespTime
	m.last2xxCount = status2xx
	m.last4xxCount = status4xx
	m.last5xxCount = status5xx
	m.lastUpdate = now
	m.lastRequestTime = lastRequestTime
	if jsonBytes != nil {
		m.cachedJSON = jsonBytes
	}
	m.mu.Unlock()

	m.logger.Trace("Collected real-time metrics (in-memory)",
		m.logger.Args(
			"request_rate", requestRate,
			"buffer_size", len(m.requestBuffer),
		))
}

// GetMetrics returns the current metrics snapshot
func (m *MetricsCollector) GetMetrics() *RealtimeMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &RealtimeMetrics{
		RequestRate:       m.requestRate,
		ErrorRate:         m.errorRate,
		AvgResponseTime:   m.avgResponseTime,
		ActiveConnections: m.activeConnections,
		Status2xx:         m.last2xxCount,
		Status4xx:         m.last4xxCount,
		Status5xx:         m.last5xxCount,
		Timestamp:         time.Now(), // Use current time, not lastUpdate
		PerService:        m.perServiceMetrics,
	}
}

// ServiceFilter represents a service filter
type ServiceFilter struct {
	Name string
	Type string
}

// ExcludeIPFilter represents IP exclusion filter
type ExcludeIPFilter struct {
	ClientIP        string
	ExcludeServices []ServiceFilter
}

// GetMetricsWithHost returns real-time metrics filtered by host
// This queries the database on-demand to provide accurate per-host metrics
func (m *MetricsCollector) GetMetricsWithHost(host string) *RealtimeMetrics {
	return m.GetMetricsWithFilters(host, nil, nil)
}

// GetMetricsWithFilters returns real-time metrics with service and IP exclusion filters
func (m *MetricsCollector) GetMetricsWithFilters(host string, serviceFilters []ServiceFilter, excludeIPFilter *ExcludeIPFilter) *RealtimeMetrics {
	now := time.Now()
	windowDuration := 5 * time.Second
	windowStart := now.Add(-windowDuration)
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// If no filters specified, return global metrics
	if host == "" && len(serviceFilters) == 0 && excludeIPFilter == nil {
		return m.GetMetrics()
	}

	m.bufferMu.RLock()
	defer m.bufferMu.RUnlock()

	var (
		totalCountWindow int64
		errorCountWindow int64
		totalRespTime    float64
		status2xx        int64
		status4xx        int64
		status5xx        int64
		count1m          int64
		lastRequestTime  time.Time
		filteredRequests []*models.HTTPRequest
	)

	// Convert local ServiceFilter to repositories.ServiceFilter for helper compatibility
	repoFilters := make([]repositories.ServiceFilter, len(serviceFilters))
	for i, f := range serviceFilters {
		repoFilters[i] = repositories.ServiceFilter{Name: f.Name, Type: f.Type}
	}
	
	// Convert local ExcludeIPFilter to repositories.ExcludeIPFilter
	var repoExcludeIP *repositories.ExcludeIPFilter
	if excludeIPFilter != nil {
		repoExcludeIP = &repositories.ExcludeIPFilter{
			ClientIP: excludeIPFilter.ClientIP,
			ExcludeServices: make([]repositories.ServiceFilter, len(excludeIPFilter.ExcludeServices)),
		}
		for i, f := range excludeIPFilter.ExcludeServices {
			repoExcludeIP.ExcludeServices[i] = repositories.ServiceFilter{Name: f.Name, Type: f.Type}
		}
	}

	for _, req := range m.requestBuffer {
		// Check host filter
		if host != "" && !strings.Contains(req.BackendName, strings.ReplaceAll(host, " ", "-")) {
			continue
		}

		// Check other filters
		if !m.matchesFilters(req, repoFilters, repoExcludeIP) {
			continue
		}

		// Collect matching requests for latest list
		filteredRequests = append(filteredRequests, req)

		// For rates (last 5s)
		if req.Timestamp.After(windowStart) {
			totalCountWindow++
			totalRespTime += req.ResponseTimeMs
			if req.StatusCode >= 400 {
				errorCountWindow++
			}
			if req.Timestamp.After(lastRequestTime) {
				lastRequestTime = req.Timestamp
			}
		}

		// For distribution (last 1m)
		if req.Timestamp.After(oneMinuteAgo) {
			count1m++
			if req.StatusCode >= 200 && req.StatusCode < 300 {
				status2xx++
			} else if req.StatusCode >= 400 && req.StatusCode < 500 {
				status4xx++
			} else if req.StatusCode >= 500 {
				status5xx++
			}
		}
	}

	// Calculate averages
	avgRespTime := 0.0
	if totalCountWindow > 0 {
		avgRespTime = totalRespTime / float64(totalCountWindow)
	}

	// Calculate rates similar to global metrics for consistency
	requestRate := 0.0
	errorRate := 0.0

	if totalCountWindow > 0 && !lastRequestTime.IsZero() {
		// Check if traffic is still active (last request within window)
		timeSinceLastRequest := now.Sub(lastRequestTime)

		if timeSinceLastRequest <= windowDuration {
			// Traffic is active - calculate rate over the full window
			requestRate = float64(totalCountWindow) / windowDuration.Seconds()
			errorRate = float64(errorCountWindow) / windowDuration.Seconds()
		}
		// else: traffic stopped, rates stay 0
	}

	// Calculate Top IPs (last 5s)
	ipCounts := make(map[string]int)
	ipCountries := make(map[string]string)

	for _, req := range m.requestBuffer {
		// Check host filter
		if host != "" && !strings.Contains(req.BackendName, strings.ReplaceAll(host, " ", "-")) {
			continue
		}

		// Check other filters
		if !m.matchesFilters(req, repoFilters, repoExcludeIP) {
			continue
		}

		if req.Timestamp.After(windowStart) {
			ipCounts[req.ClientIP]++
			if _, ok := ipCountries[req.ClientIP]; !ok && req.GeoCountry != "" {
				ipCountries[req.ClientIP] = req.GeoCountry
			}
		}
	}

	var topIPs []IPMetrics
	for ip, count := range ipCounts {
		rate := float64(count) / windowDuration.Seconds()
		// Only include IPs with meaningful activity (> 0.1 req/s)
		// This prevents showing nearly-inactive IPs that are about to expire from window
		if rate > 0.1 {
			topIPs = append(topIPs, IPMetrics{
				IP:          ip,
				Country:     ipCountries[ip],
				RequestRate: rate,
			})
		}
	}

	// Sort by rate desc
	sort.Slice(topIPs, func(i, j int) bool {
		return topIPs[i].RequestRate > topIPs[j].RequestRate
	})

	// Limit to top 10
	if len(topIPs) > 10 {
		topIPs = topIPs[:10]
	}

	// If no recent requests (>windowDuration old), force all metrics to zero immediately
	if !lastRequestTime.IsZero() {
		timeSinceLastRequest := now.Sub(lastRequestTime)
		if timeSinceLastRequest > windowDuration {
			requestRate = 0.0
			errorRate = 0.0
			topIPs = nil
			status2xx = 0
			status4xx = 0
			status5xx = 0
		}
	} else {
		// No requests at all
		requestRate = 0.0
		errorRate = 0.0
		topIPs = nil
		status2xx = 0
		status4xx = 0
		status5xx = 0
	}

	// Get Latest Requests (last 20 from filtered)
	latestRequests := m.getLatestRequests(filteredRequests, 20)

	// Calculate per service metrics for filtered view
	perServiceMetrics := m.calculatePerServiceMetrics(m.requestBuffer, repoFilters, repoExcludeIP)

	return &RealtimeMetrics{
		RequestRate:       requestRate,
		ErrorRate:         errorRate,
		AvgResponseTime:   avgRespTime,
		ActiveConnections: 0, // Not tracked per filter
		Status2xx:         status2xx,
		Status4xx:         status4xx,
		Status5xx:         status5xx,
		Timestamp:         now,
		TopIPs:            topIPs,
		LatestRequests:    latestRequests,
		PerService:        perServiceMetrics,
	}
}

// ServiceMetrics represents metrics for a single service
type ServiceMetrics struct {
	ServiceName string  `json:"service_name"`
	RequestRate float64 `json:"request_rate"` // req/sec
}

// GetPerServiceMetrics returns real-time metrics for each service
func (m *MetricsCollector) GetPerServiceMetrics(filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) []ServiceMetrics {
	// If no filters, return cached global metrics
	if len(filters) == 0 && (excludeIP == nil || excludeIP.ClientIP == "") {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return m.perServiceMetrics
	}

	m.bufferMu.RLock()
	defer m.bufferMu.RUnlock()
	return m.calculatePerServiceMetrics(m.requestBuffer, filters, excludeIP)
}

// calculatePerServiceMetrics calculates per-service metrics from the buffer
func (m *MetricsCollector) calculatePerServiceMetrics(buffer []*models.HTTPRequest, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) []ServiceMetrics {
	// Use 5-second window for accurate real-time rates
	// Use parent's now timestamp for consistency
	now := time.Now()
	windowDuration := 5 * time.Second
	windowStart := now.Add(-windowDuration)

	// Map to aggregate counts by service
	serviceCounts := make(map[string]int64)

	for _, req := range buffer {
		if !req.Timestamp.After(windowStart) {
			continue
		}

		// Apply filters
		if !m.matchesFilters(req, filters, excludeIP) {
			continue
		}

		// Determine service name
		serviceName := ""
		if req.BackendName != "" {
			serviceName = extractServiceName(req.BackendName)
		} else if req.BackendURL != "" {
			serviceName = req.BackendURL
		} else if req.Host != "" {
			serviceName = req.Host
		}

		if serviceName != "" {
			serviceCounts[serviceName]++
		}
	}

	// Convert map to slice
	metrics := make([]ServiceMetrics, 0, len(serviceCounts))
	for name, count := range serviceCounts {
		metrics = append(metrics, ServiceMetrics{
			ServiceName: name,
			RequestRate: float64(count) / windowDuration.Seconds(),
		})
	}

	return metrics
}

// matchesFilters checks if a request matches the given filters
func (m *MetricsCollector) matchesFilters(req *models.HTTPRequest, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) bool {
	// Apply IP exclusion
	if excludeIP != nil && excludeIP.ClientIP != "" {
		if req.ClientIP == excludeIP.ClientIP {
			if len(excludeIP.ExcludeServices) == 0 {
				return false // Exclude from all
			}
			// Check if excluded from this specific service
			for _, filter := range excludeIP.ExcludeServices {
				if m.matchesServiceFilter(req, filter) {
					return false
				}
			}
		}
	}

	// Apply service filters (OR logic)
	if len(filters) > 0 {
		matched := false
		for _, filter := range filters {
			if m.matchesServiceFilter(req, filter) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// matchesServiceFilter checks if a request matches a single service filter
func (m *MetricsCollector) matchesServiceFilter(req *models.HTTPRequest, filter repositories.ServiceFilter) bool {
	switch filter.Type {
	case "backend_name":
		return req.BackendName == filter.Name
	case "backend_url":
		return req.BackendURL == filter.Name
	case "host":
		return req.Host == filter.Name
	default:
		return req.BackendName == filter.Name || req.BackendURL == filter.Name || req.Host == filter.Name
	}
}

// extractServiceName extracts the readable name from backend_name
func extractServiceName(backendName string) string {
	if backendName == "" {
		return ""
	}

	// Remove protocol suffix
	parts := strings.Split(backendName, "@")
	if len(parts) > 0 {
		backendName = parts[0]
	}

	// Remove -service suffix
	backendName = strings.TrimSuffix(backendName, "-service")

	// Split by dash and remove first element (id)
	parts = strings.Split(backendName, "-")
	if len(parts) > 1 {
		details := parts[1:]
		return strings.Join(details, " ")
	}

	return backendName
}

// getLatestRequests returns the last N requests from the buffer as summaries
// Returns in reverse chronological order (newest first)
func (m *MetricsCollector) getLatestRequests(requests []*models.HTTPRequest, limit int) []RequestSummary {
	count := len(requests)
	if count == 0 {
		return []RequestSummary{}
	}

	start := count - limit
	if start < 0 {
		start = 0
	}

	summary := make([]RequestSummary, 0, count-start)
	// Iterate backwards to get newest first
	for i := count - 1; i >= start; i-- {
		req := requests[i]
		summary = append(summary, RequestSummary{
			ID:             req.ID,
			Timestamp:      req.Timestamp,
			Method:         req.Method,
			Host:           req.Host,
			BackendName:    req.BackendName,
			Path:           req.Path,
			StatusCode:     req.StatusCode,
			ResponseTimeMs: req.ResponseTimeMs,
			GeoCountry:     req.GeoCountry,
			ClientIP:       req.ClientIP,
		})
	}
	return summary
}
