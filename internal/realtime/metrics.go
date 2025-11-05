package realtime

import (
	"strings"
	"sync"
	"time"

	"loglynx/internal/database/repositories"

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
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// If no filters specified, return global metrics
	if host == "" && len(serviceFilters) == 0 && excludeIPFilter == nil {
		return m.GetMetrics()
	}

	// Query database with filters
	type MetricsResult struct {
		TotalCount  int64   `gorm:"column:total_count"`
		ErrorCount  int64   `gorm:"column:error_count"`
		AvgRespTime float64 `gorm:"column:avg_response_time"`
		Status2xx   int64   `gorm:"column:status_2xx"`
		Status4xx   int64   `gorm:"column:status_4xx"`
		Status5xx   int64   `gorm:"column:status_5xx"`
	}

	var result MetricsResult
	query := m.db.Table("http_requests").
		Select(`
			COUNT(*) as total_count,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count,
			COALESCE(AVG(response_time_ms), 0) as avg_response_time,
			SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as status_2xx,
			SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) as status_4xx,
			SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) as status_5xx
		`).
		Where("timestamp > ?", oneMinuteAgo)

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
			// Build condition: (client_ip != ? AND (service matches))
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

	err := query.Scan(&result).Error

	if err != nil {
		m.logger.Warn("Failed to collect filtered real-time metrics",
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
		ActiveConnections: 0, // Not applicable for filtered metrics
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
func (m *MetricsCollector) GetPerServiceMetrics(filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) []ServiceMetrics {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// Query database for per-service metrics
	type ServiceResult struct {
		BackendName string `gorm:"column:backend_name"`
		BackendURL  string `gorm:"column:backend_url"`
		Host        string `gorm:"column:host"`
		TotalCount  int64  `gorm:"column:total_count"`
	}

	query := m.db.Table("http_requests").
		Select("backend_name, backend_url, host, COUNT(*) as total_count").
		Where("timestamp > ?", oneMinuteAgo)

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

	// Extract service names and calculate rates
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
