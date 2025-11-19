package realtime

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"loglynx/internal/database/repositories"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

const (
	// QueryTimeout is the maximum time for a database query
	QueryTimeout = 5 * time.Second
)

// MetricsCollector collects real-time metrics
type MetricsCollector struct {
	db     *gorm.DB
	logger *pterm.Logger

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
		db:         db,
		logger:     logger,
		lastUpdate: time.Now(),
		stopChan:   make(chan struct{}),
	}
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

// collectMetrics gathers current statistics from the database
// Uses a sliding window approach for accurate rate calculation
func (m *MetricsCollector) collectMetrics() {
	now := time.Now()

	// Use a 2-second sliding window for immediate real-time responsiveness
	// This ensures the chart shows zero quickly when traffic stops
	twoSecondsAgo := now.Add(-2 * time.Second)
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()

	// Query for rate calculation (last 2 seconds for real-time accuracy)
	type RateResult struct {
		TotalCount    int64     `gorm:"column:total_count"`
		ErrorCount    int64     `gorm:"column:error_count"`
		LastTimestamp time.Time `gorm:"column:last_timestamp"`
	}

	var rateResult RateResult
	err := m.db.WithContext(ctx).Table("http_requests").
		Select(`
			COUNT(*) as total_count,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count,
			MAX(timestamp) as last_timestamp
		`).
		Where("timestamp > ?", twoSecondsAgo).
		Scan(&rateResult).Error

	if err != nil {
		m.logger.Warn("Failed to collect rate metrics", m.logger.Args("error", err))
		return
	}

	// Query for status distribution and avg response time (last 1 minute for smoother stats)
	type MetricsResult struct {
		AvgRespTime float64 `gorm:"column:avg_response_time"`
		Status2xx   int64   `gorm:"column:status_2xx"`
		Status4xx   int64   `gorm:"column:status_4xx"`
		Status5xx   int64   `gorm:"column:status_5xx"`
	}

	var result MetricsResult
	err = m.db.WithContext(ctx).Table("http_requests").
		Select(`
			COALESCE(AVG(response_time_ms), 0) as avg_response_time,
			SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as status_2xx,
			SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) as status_4xx,
			SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) as status_5xx
		`).
		Where("timestamp > ?", oneMinuteAgo).
		Scan(&result).Error

	if err != nil {
		m.logger.Warn("Failed to collect status metrics", m.logger.Args("error", err))
		return
	}

	// Calculate rates per second based on 2-second window
	requestRate := float64(rateResult.TotalCount) / 2.0
	errorRate := float64(rateResult.ErrorCount) / 2.0

	// Track last request time for proactive zero detection
	var lastRequestTime time.Time
	if !rateResult.LastTimestamp.IsZero() {
		lastRequestTime = rateResult.LastTimestamp
	} else {
		// No requests in window - use previous lastRequestTime
		m.mu.RLock()
		lastRequestTime = m.lastRequestTime
		m.mu.RUnlock()
	}

	// If no recent requests (>2 seconds old), force rates to zero immediately
	timeSinceLastRequest := now.Sub(lastRequestTime)
	if timeSinceLastRequest > 2*time.Second {
		requestRate = 0.0
		errorRate = 0.0
	}

	// Collect per-service metrics (global)
	perServiceMetrics := m.fetchPerServiceMetrics(nil, nil)

	// Prepare metrics struct for JSON caching
	metrics := &RealtimeMetrics{
		RequestRate:       requestRate,
		ErrorRate:         errorRate,
		AvgResponseTime:   result.AvgRespTime,
		ActiveConnections: m.activeConnections, // Use current active connections
		Status2xx:         result.Status2xx,
		Status4xx:         result.Status4xx,
		Status5xx:         result.Status5xx,
		Timestamp:         now,
	}

	// Marshal to JSON immediately for caching
	// This avoids repeated marshaling for every connected client
	jsonBytes, _ := json.Marshal(metrics)

	// Update metrics with lock
	m.mu.Lock()
	m.perServiceMetrics = perServiceMetrics
	m.requestRate = requestRate
	m.errorRate = errorRate
	m.avgResponseTime = result.AvgRespTime
	m.last2xxCount = result.Status2xx
	m.last4xxCount = result.Status4xx
	m.last5xxCount = result.Status5xx
	m.lastUpdate = now
	m.lastRequestTime = lastRequestTime
	if jsonBytes != nil {
		m.cachedJSON = jsonBytes
	}
	m.mu.Unlock()

	m.logger.Trace("Collected real-time metrics",
		m.logger.Args(
			"request_rate", requestRate,
			"error_rate", errorRate,
			"avg_response_time", result.AvgRespTime,
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

// applyFilters applies service and IP exclusion filters to a query
func (m *MetricsCollector) applyFilters(query *gorm.DB, host string, serviceFilters []ServiceFilter, excludeIPFilter *ExcludeIPFilter) *gorm.DB {
	// Apply host filter (legacy single host filter)
	if host != "" {
		query = query.Where("backend_name LIKE ?", "%-"+strings.ReplaceAll(host, " ", "-")+"-%")
	}

	// Apply service filters (new multi-service filter)
	if len(serviceFilters) > 0 {
		conditions := make([]string, len(serviceFilters))
		args := make([]interface{}, len(serviceFilters))

		for i, filter := range serviceFilters {
			switch filter.Type {
			case "backend_name":
				conditions[i] = "backend_name = ?"
				args[i] = filter.Name
			case "backend_url":
				conditions[i] = "backend_url = ?"
				args[i] = filter.Name
			case "host":
				conditions[i] = "host = ?"
				args[i] = filter.Name
			default:
				// Auto-detect: try all fields
				conditions[i] = "(backend_name = ? OR backend_url = ? OR host = ?)"
				args[i] = filter.Name
			}
		}

		// Combine conditions with OR
		whereClause := strings.Join(conditions, " OR ")
		query = query.Where("("+whereClause+")", args...)
	}

	// Apply IP exclusion filter
	if excludeIPFilter != nil && excludeIPFilter.ClientIP != "" {
		if len(excludeIPFilter.ExcludeServices) == 0 {
			// Exclude IP from all services
			query = query.Where("client_ip != ?", excludeIPFilter.ClientIP)
		} else {
			// Exclude IP only from specific services
			serviceConditions := make([]string, len(excludeIPFilter.ExcludeServices))
			serviceArgs := make([]interface{}, 0, len(excludeIPFilter.ExcludeServices)*3)

			for i, filter := range excludeIPFilter.ExcludeServices {
				switch filter.Type {
				case "backend_name":
					serviceConditions[i] = "backend_name = ?"
					serviceArgs = append(serviceArgs, filter.Name)
				case "backend_url":
					serviceConditions[i] = "backend_url = ?"
					serviceArgs = append(serviceArgs, filter.Name)
				case "host":
					serviceConditions[i] = "host = ?"
					serviceArgs = append(serviceArgs, filter.Name)
				default:
					serviceConditions[i] = "(backend_name = ? OR backend_url = ? OR host = ?)"
					serviceArgs = append(serviceArgs, filter.Name, filter.Name, filter.Name)
				}
			}

			serviceWhere := strings.Join(serviceConditions, " OR ")
			allArgs := append([]interface{}{excludeIPFilter.ClientIP}, serviceArgs...)
			query = query.Where("NOT (client_ip = ? AND ("+serviceWhere+"))", allArgs...)
		}
	}

	return query
}

// GetMetricsWithFilters returns real-time metrics with service and IP exclusion filters
func (m *MetricsCollector) GetMetricsWithFilters(host string, serviceFilters []ServiceFilter, excludeIPFilter *ExcludeIPFilter) *RealtimeMetrics {
	now := time.Now()

	// Use 2-second window for rates, 1 minute for status distribution
	twoSecondsAgo := now.Add(-2 * time.Second)
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// If no filters specified, return global metrics
	if host == "" && len(serviceFilters) == 0 && excludeIPFilter == nil {
		return m.GetMetrics()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()

	// Query 1: Get rates from last 2 seconds
	type RateResult struct {
		TotalCount    int64     `gorm:"column:total_count"`
		ErrorCount    int64     `gorm:"column:error_count"`
		LastTimestamp time.Time `gorm:"column:last_timestamp"`
	}

	var rateResult RateResult
	rateQuery := m.db.WithContext(ctx).Table("http_requests").
		Select(`
			COUNT(*) as total_count,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count,
			MAX(timestamp) as last_timestamp
		`).
		Where("timestamp > ?", twoSecondsAgo)

	rateQuery = m.applyFilters(rateQuery, host, serviceFilters, excludeIPFilter)

	if err := rateQuery.Scan(&rateResult).Error; err != nil {
		m.logger.Warn("Failed to collect filtered rate metrics", m.logger.Args("error", err))
		return &RealtimeMetrics{Timestamp: now}
	}

	// Query 2: Get status distribution from last 1 minute
	type StatusResult struct {
		AvgRespTime float64 `gorm:"column:avg_response_time"`
		Status2xx   int64   `gorm:"column:status_2xx"`
		Status4xx   int64   `gorm:"column:status_4xx"`
		Status5xx   int64   `gorm:"column:status_5xx"`
	}

	var statusResult StatusResult
	statusQuery := m.db.WithContext(ctx).Table("http_requests").
		Select(`
			COALESCE(AVG(response_time_ms), 0) as avg_response_time,
			SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as status_2xx,
			SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) as status_4xx,
			SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) as status_5xx
		`).
		Where("timestamp > ?", oneMinuteAgo)

	statusQuery = m.applyFilters(statusQuery, host, serviceFilters, excludeIPFilter)

	if err := statusQuery.Scan(&statusResult).Error; err != nil {
		m.logger.Warn("Failed to collect filtered status metrics", m.logger.Args("error", err))
		return &RealtimeMetrics{Timestamp: now}
	}

	// Calculate rates per second based on 2-second window
	requestRate := float64(rateResult.TotalCount) / 2.0
	errorRate := float64(rateResult.ErrorCount) / 2.0

	// If no recent requests (>2 seconds old), force rates to zero immediately
	if !rateResult.LastTimestamp.IsZero() {
		timeSinceLastRequest := now.Sub(rateResult.LastTimestamp)
		if timeSinceLastRequest > 2*time.Second {
			requestRate = 0.0
			errorRate = 0.0
		}
	}

	return &RealtimeMetrics{
		RequestRate:       requestRate,
		ErrorRate:         errorRate,
		AvgResponseTime:   statusResult.AvgRespTime,
		ActiveConnections: 0, // Not applicable for filtered metrics
		Status2xx:         statusResult.Status2xx,
		Status4xx:         statusResult.Status4xx,
		Status5xx:         statusResult.Status5xx,
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

	return m.fetchPerServiceMetrics(filters, excludeIP)
}

// fetchPerServiceMetrics queries the database for per-service metrics
func (m *MetricsCollector) fetchPerServiceMetrics(filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) []ServiceMetrics {
	// Use 2-second window for accurate real-time rates
	twoSecondsAgo := time.Now().Add(-2 * time.Second)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()

	// Query database for per-service metrics
	type ServiceResult struct {
		BackendName string `gorm:"column:backend_name"`
		BackendURL  string `gorm:"column:backend_url"`
		Host        string `gorm:"column:host"`
		TotalCount  int64  `gorm:"column:total_count"`
	}

	query := m.db.WithContext(ctx).Table("http_requests").
		Select("backend_name, backend_url, host, COUNT(*) as total_count").
		Where("timestamp > ?", twoSecondsAgo)

	// Apply service filters (if any)
	if len(filters) > 0 {
		serviceQuery := m.db.Where("1 = 0") // Start with false condition
		for _, filter := range filters {
			switch filter.Type {
			case "backend_name":
				serviceQuery = serviceQuery.Or("backend_name = ?", filter.Name)
			case "backend_url":
				serviceQuery = serviceQuery.Or("backend_url = ?", filter.Name)
			case "host":
				serviceQuery = serviceQuery.Or("host = ?", filter.Name)
			}
		}
		query = query.Where(serviceQuery)
	}

	// Apply IP exclusion filter (if any)
	if excludeIP != nil && excludeIP.ClientIP != "" {
		if len(excludeIP.ExcludeServices) == 0 {
			// Exclude IP from all services
			query = query.Where("client_ip != ?", excludeIP.ClientIP)
		} else {
			// Exclude IP only on specific services
			serviceConds := []string{}
			args := []interface{}{excludeIP.ClientIP}

			for _, filter := range excludeIP.ExcludeServices {
				switch filter.Type {
				case "backend_name":
					serviceConds = append(serviceConds, "backend_name = ?")
					args = append(args, filter.Name)
				case "backend_url":
					serviceConds = append(serviceConds, "backend_url = ?")
					args = append(args, filter.Name)
				case "host":
					serviceConds = append(serviceConds, "host = ?")
					args = append(args, filter.Name)
				}
			}

			whereClause := "NOT (client_ip = ? AND (" + strings.Join(serviceConds, " OR ") + "))"
			query = query.Where(whereClause, args...)
		}
	}

	var results []ServiceResult
	err := query.Group("backend_name, backend_url, host").
		Scan(&results).Error

	if err != nil {
		m.logger.Warn("Failed to collect per-service metrics", m.logger.Args("error", err))
		return []ServiceMetrics{}
	}

	// Extract service names and calculate rates (per second based on 10-second window)
	serviceMetrics := make([]ServiceMetrics, 0, len(results))
	for _, result := range results {
		// Determine service name based on priority: backend_name > backend_url > host
		serviceName := ""
		if result.BackendName != "" {
			serviceName = extractServiceName(result.BackendName)
		} else if result.BackendURL != "" {
			serviceName = result.BackendURL
		} else if result.Host != "" {
			serviceName = result.Host
		}

		if serviceName == "" {
			continue
		}

		requestRate := float64(result.TotalCount) / 2.0 // Divide by 2 seconds
		serviceMetrics = append(serviceMetrics, ServiceMetrics{
			ServiceName: serviceName,
			RequestRate: requestRate,
		})
	}

	return serviceMetrics
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
