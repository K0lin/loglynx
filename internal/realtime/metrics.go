package realtime

import (
	"encoding/json"
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
	RequestRate       float64   `json:"request_rate"`      // req/sec
	ErrorRate         float64   `json:"error_rate"`        // errors/sec
	AvgResponseTime   float64   `json:"avg_response_time"` // ms
	ActiveConnections int       `json:"active_connections"`
	Status2xx         int64     `json:"status_2xx"`
	Status4xx         int64     `json:"status_4xx"`
	Status5xx         int64     `json:"status_5xx"`
	Timestamp         time.Time `json:"timestamp"`
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
func (m *MetricsCollector) Ingest(req *models.HTTPRequest) {
	m.bufferMu.Lock()
	m.requestBuffer = append(m.requestBuffer, req)
	m.bufferMu.Unlock()
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

	// Use a 2-second sliding window for immediate real-time responsiveness
	twoSecondsAgo := now.Add(-2 * time.Second)
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

	if validIndex > 0 {
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
		totalCount2s    int64
		errorCount2s    int64
		status2xx       int64
		status4xx       int64
		status5xx       int64
		totalRespTime   float64
		count1m         int64
		lastRequestTime time.Time
	)

	for _, req := range m.requestBuffer {
		// For rates (last 2s)
		if req.Timestamp.After(twoSecondsAgo) {
			totalCount2s++
			if req.StatusCode >= 400 {
				errorCount2s++
			}
			if req.Timestamp.After(lastRequestTime) {
				lastRequestTime = req.Timestamp
			}
		}

		// For distribution (last 1m)
		count1m++
		totalRespTime += req.ResponseTimeMs
		if req.StatusCode >= 200 && req.StatusCode < 300 {
			status2xx++
		} else if req.StatusCode >= 400 && req.StatusCode < 500 {
			status4xx++
		} else if req.StatusCode >= 500 {
			status5xx++
		}
	}

	// Calculate averages
	avgRespTime := 0.0
	if count1m > 0 {
		avgRespTime = totalRespTime / float64(count1m)
	}

	// Calculate rates per second based on 2-second window
	requestRate := float64(totalCount2s) / 2.0
	errorRate := float64(errorCount2s) / 2.0

	// If no recent requests (>2 seconds old), force rates to zero immediately
	if !lastRequestTime.IsZero() {
		timeSinceLastRequest := now.Sub(lastRequestTime)
		if timeSinceLastRequest > 2*time.Second {
			requestRate = 0.0
			errorRate = 0.0
		}
	} else {
		// No requests in window
		requestRate = 0.0
		errorRate = 0.0
		// Use previous lastRequestTime if available
		m.mu.RLock()
		lastRequestTime = m.lastRequestTime
		m.mu.RUnlock()
	}

	// Collect per-service metrics (global) - passing nil filters uses buffer
	perServiceMetrics := m.calculatePerServiceMetrics(m.requestBuffer, nil, nil)

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
		Timestamp:         m.lastUpdate,
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
	twoSecondsAgo := now.Add(-2 * time.Second)
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// If no filters specified, return global metrics
	if host == "" && len(serviceFilters) == 0 && excludeIPFilter == nil {
		return m.GetMetrics()
	}

	m.bufferMu.RLock()
	defer m.bufferMu.RUnlock()

	var (
		totalCount2s    int64
		errorCount2s    int64
		status2xx       int64
		status4xx       int64
		status5xx       int64
		totalRespTime   float64
		count1m         int64
		lastRequestTime time.Time
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

		// For rates (last 2s)
		if req.Timestamp.After(twoSecondsAgo) {
			totalCount2s++
			if req.StatusCode >= 400 {
				errorCount2s++
			}
			if req.Timestamp.After(lastRequestTime) {
				lastRequestTime = req.Timestamp
			}
		}

		// For distribution (last 1m)
		if req.Timestamp.After(oneMinuteAgo) {
			count1m++
			totalRespTime += req.ResponseTimeMs
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
	if count1m > 0 {
		avgRespTime = totalRespTime / float64(count1m)
	}

	requestRate := float64(totalCount2s) / 2.0
	errorRate := float64(errorCount2s) / 2.0

	// If no recent requests (>2 seconds old), force rates to zero immediately
	if !lastRequestTime.IsZero() {
		timeSinceLastRequest := now.Sub(lastRequestTime)
		if timeSinceLastRequest > 2*time.Second {
			requestRate = 0.0
			errorRate = 0.0
		}
	}

	return &RealtimeMetrics{
		RequestRate:       requestRate,
		ErrorRate:         errorRate,
		AvgResponseTime:   avgRespTime,
		ActiveConnections: 0, // Not tracked per filter
		Status2xx:         status2xx,
		Status4xx:         status4xx,
		Status5xx:         status5xx,
		Timestamp:         now,
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
	// Use 2-second window for accurate real-time rates
	twoSecondsAgo := time.Now().Add(-2 * time.Second)

	// Map to aggregate counts by service
	serviceCounts := make(map[string]int64)

	for _, req := range buffer {
		if !req.Timestamp.After(twoSecondsAgo) {
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
			RequestRate: float64(count) / 2.0,
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
