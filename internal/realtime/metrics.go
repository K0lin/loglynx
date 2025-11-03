package realtime

import (
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
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
	}
}

// Start begins collecting metrics at regular intervals
func (m *MetricsCollector) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.collectMetrics()
		}
	}()
	m.logger.Info("Real-time metrics collector started",
		m.logger.Args("interval", interval.String()))
}

// collectMetrics gathers current statistics from the database
// Optimized to use a single query instead of 5 separate queries
func (m *MetricsCollector) collectMetrics() {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// Single aggregated query to get all metrics at once
	type MetricsResult struct {
		TotalCount  int64   `gorm:"column:total_count"`
		ErrorCount  int64   `gorm:"column:error_count"`
		AvgRespTime float64 `gorm:"column:avg_response_time"`
		Status2xx   int64   `gorm:"column:status_2xx"`
		Status4xx   int64   `gorm:"column:status_4xx"`
		Status5xx   int64   `gorm:"column:status_5xx"`
	}

	var result MetricsResult
	err := m.db.Table("http_requests").
		Select(`
			COUNT(*) as total_count,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count,
			COALESCE(AVG(response_time_ms), 0) as avg_response_time,
			SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as status_2xx,
			SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) as status_4xx,
			SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) as status_5xx
		`).
		Where("timestamp > ?", oneMinuteAgo).
		Scan(&result).Error

	if err != nil {
		m.logger.Warn("Failed to collect real-time metrics", m.logger.Args("error", err))
		return
	}

	// Calculate rates (per second)
	requestRate := float64(result.TotalCount) / 60.0
	errorRate := float64(result.ErrorCount) / 60.0

	// Update metrics with lock
	m.mu.Lock()
	m.requestRate = requestRate
	m.errorRate = errorRate
	m.avgResponseTime = result.AvgRespTime
	m.last2xxCount = result.Status2xx
	m.last4xxCount = result.Status4xx
	m.last5xxCount = result.Status5xx
	m.lastUpdate = now
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

// GetMetricsWithHost returns real-time metrics filtered by host
// This queries the database on-demand to provide accurate per-host metrics
func (m *MetricsCollector) GetMetricsWithHost(host string) *RealtimeMetrics {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// If no host specified, return global metrics
	if host == "" {
		return m.GetMetrics()
	}

	// Query database for host-specific metrics
	type MetricsResult struct {
		TotalCount  int64   `gorm:"column:total_count"`
		ErrorCount  int64   `gorm:"column:error_count"`
		AvgRespTime float64 `gorm:"column:avg_response_time"`
		Status2xx   int64   `gorm:"column:status_2xx"`
		Status4xx   int64   `gorm:"column:status_4xx"`
		Status5xx   int64   `gorm:"column:status_5xx"`
	}

	var result MetricsResult
	err := m.db.Table("http_requests").
		Select(`
			COUNT(*) as total_count,
	SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count,
	COALESCE(AVG(response_time_ms), 0) as avg_response_time,
	SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as status_2xx,
	SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) as status_4xx,
	SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) as status_5xx
`).
		Where("timestamp > ? AND backend_name LIKE ?", oneMinuteAgo, "%-"+strings.ReplaceAll(host, " ", "-")+"-%").
		Scan(&result).Error

	if err != nil {
		m.logger.Warn("Failed to collect host-specific real-time metrics",
			m.logger.Args("error", err, "host", host))
		// Return empty metrics on error
		return &RealtimeMetrics{
			Timestamp: now,
		}
	}

	// Calculate rates (per second)
	requestRate := float64(result.TotalCount) / 60.0
	errorRate := float64(result.ErrorCount) / 60.0

	return &RealtimeMetrics{
		RequestRate:       requestRate,
		ErrorRate:         errorRate,
		AvgResponseTime:   result.AvgRespTime,
		ActiveConnections: 0, // Not applicable for per-host metrics
		Status2xx:         result.Status2xx,
		Status4xx:         result.Status4xx,
		Status5xx:         result.Status5xx,
		Timestamp:         now,
	}
}

// ServiceMetrics represents metrics for a single service
type ServiceMetrics struct {
	ServiceName string  `json:"service_name"`
	RequestRate float64 `json:"request_rate"` // req/sec
}

// GetPerServiceMetrics returns real-time metrics for each service
func (m *MetricsCollector) GetPerServiceMetrics() []ServiceMetrics {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// Query database for per-service metrics
	type ServiceResult struct {
		BackendName string `gorm:"column:backend_name"`
		TotalCount  int64  `gorm:"column:total_count"`
	}

	var results []ServiceResult
	err := m.db.Table("http_requests").
		Select("backend_name, COUNT(*) as total_count").
		Where("timestamp > ? AND backend_name != ? AND backend_name != ? AND backend_name != ?",
			oneMinuteAgo, "next-service@file", "api-service@file", "").
		Group("backend_name").
		Scan(&results).Error

	if err != nil {
		m.logger.Warn("Failed to collect per-service metrics", m.logger.Args("error", err))
		return []ServiceMetrics{}
	}

	// Extract service names and calculate rates
	serviceMetrics := make([]ServiceMetrics, 0, len(results))
	for _, result := range results {
		serviceName := extractServiceName(result.BackendName)
		if serviceName == "" {
			continue
		}

		requestRate := float64(result.TotalCount) / 60.0
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
