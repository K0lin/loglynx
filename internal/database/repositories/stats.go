package repositories

import (
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"loglynx/internal/database/models"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

// StatsRepository provides dashboard statistics
// All methods now accept an optional host parameter for filtering by domain
type StatsRepository interface {
	GetSummary(host string) (*StatsSummary, error)
	GetTimelineStats(hours int, host string) ([]*TimelineData, error)
	GetStatusCodeTimeline(hours int, host string) ([]*StatusCodeTimelineData, error)
	GetTrafficHeatmap(days int, host string) ([]*TrafficHeatmapData, error)
	GetTopPaths(limit int, host string) ([]*PathStats, error)
	GetTopCountries(limit int, host string) ([]*CountryStats, error)
	GetTopIPAddresses(limit int, host string) ([]*IPStats, error)
	GetStatusCodeDistribution(host string) ([]*StatusCodeStats, error)
	GetMethodDistribution(host string) ([]*MethodStats, error)
	GetProtocolDistribution(host string) ([]*ProtocolStats, error)
	GetTLSVersionDistribution(host string) ([]*TLSVersionStats, error)
	GetTopUserAgents(limit int, host string) ([]*UserAgentStats, error)
	GetTopBrowsers(limit int, host string) ([]*BrowserStats, error)
	GetTopOperatingSystems(limit int, host string) ([]*OSStats, error)
	GetDeviceTypeDistribution(host string) ([]*DeviceTypeStats, error)
	GetTopASNs(limit int, host string) ([]*ASNStats, error)
	GetTopBackends(limit int, host string) ([]*BackendStats, error)
	GetTopReferrers(limit int, host string) ([]*ReferrerStats, error)
	GetTopReferrerDomains(limit int, host string) ([]*ReferrerDomainStats, error)
	GetResponseTimeStats(host string) (*ResponseTimeStats, error)
	GetLogProcessingStats() ([]*LogProcessingStats, error)
	GetDomains() ([]*DomainStats, error)
}

type statsRepo struct {
	db     *gorm.DB
	logger *pterm.Logger
}

const (
	// DefaultLookbackHours is the default time range for stats queries (7 days)
	DefaultLookbackHours = 168
)

// NewStatsRepository creates a new stats repository
func NewStatsRepository(db *gorm.DB, logger *pterm.Logger) StatsRepository {
	return &statsRepo{
		db:     db,
		logger: logger,
	}
}

// getTimeRange returns the time range for stats queries
func (r *statsRepo) getTimeRange() time.Time {
	return time.Now().Add(-DefaultLookbackHours * time.Hour)
}

// applyHostFilter applies host filter to a query if host is not empty
// Filters by referer field (domain) using LIKE pattern matching
func (r *statsRepo) applyHostFilter(query *gorm.DB, host string) *gorm.DB {
	if host != "" {
		// Convert spaces back to dashes for pattern matching
		// e.g., "detail1 detail2" -> "detail1-detail2"
		pattern := strings.ReplaceAll(host, " ", "-")
		// Filter by backend_name using LIKE to match the pattern
		return query.Where("backend_name LIKE ?", "%-"+pattern+"-%")
	}
	return query
}

// StatsSummary holds overall statistics
type StatsSummary struct {
	TotalRequests   int64   `json:"total_requests"`
	ValidRequests   int64   `json:"valid_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	UniqueVisitors  int64   `json:"unique_visitors"`
	UniqueFiles     int64   `json:"unique_files"`
	Unique404       int64   `json:"unique_404"`
	TotalBandwidth  int64   `json:"total_bandwidth"`
	AvgResponseTime float64 `json:"avg_response_time"`
	SuccessRate     float64 `json:"success_rate"`
	NotFoundRate    float64 `json:"not_found_rate"`
	ServerErrorRate float64 `json:"server_error_rate"`
	RequestsPerHour float64 `json:"requests_per_hour"`
	TopCountry      string  `json:"top_country"`
	TopPath         string  `json:"top_path"`
}

// TimelineData holds timeline statistics
type TimelineData struct {
	Hour            string  `json:"hour"`
	Requests        int64   `json:"requests"`
	UniqueVisitors  int64   `json:"unique_visitors"`
	Bandwidth       int64   `json:"bandwidth"`
	AvgResponseTime float64 `json:"avg_response_time"`
}

// StatusCodeTimelineData holds status code timeline data for stacked chart
type StatusCodeTimelineData struct {
	Hour      string `gorm:"column:hour" json:"hour"`
	Status2xx int64  `gorm:"column:status_2xx" json:"status_2xx"`
	Status3xx int64  `gorm:"column:status_3xx" json:"status_3xx"`
	Status4xx int64  `gorm:"column:status_4xx" json:"status_4xx"`
	Status5xx int64  `gorm:"column:status_5xx" json:"status_5xx"`
}

// TrafficHeatmapData holds hourly traffic metrics for heatmap visualisation
type TrafficHeatmapData struct {
	DayOfWeek       int     `json:"day_of_week"`
	Hour            int     `json:"hour"`
	Requests        int64   `json:"requests"`
	AvgResponseTime float64 `json:"avg_response_time"`
}

// PathStats holds path statistics
type PathStats struct {
	Path            string  `json:"path"`
	Hits            int64   `json:"hits"`
	UniqueVisitors  int64   `json:"unique_visitors"`
	AvgResponseTime float64 `json:"avg_response_time"`
	TotalBandwidth  int64   `json:"total_bandwidth"`
}

// CountryStats holds country statistics
type CountryStats struct {
	Country        string `json:"country"`
	CountryName    string `json:"country_name"`
	Hits           int64  `json:"hits"`
	UniqueVisitors int64  `json:"unique_visitors"`
	Bandwidth      int64  `json:"bandwidth"`
}

// IPStats holds IP address statistics
type IPStats struct {
	IPAddress string  `json:"ip_address"`
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Hits      int64   `json:"hits"`
	Bandwidth int64   `json:"bandwidth"`
}

// StatusCodeStats holds status code distribution
type StatusCodeStats struct {
	StatusCode int   `json:"status_code"`
	Count      int64 `json:"count"`
}

// MethodStats holds HTTP method distribution
type MethodStats struct {
	Method string `json:"method"`
	Count  int64  `json:"count"`
}

// ProtocolStats holds HTTP protocol distribution
type ProtocolStats struct {
	Protocol string `json:"protocol"`
	Count    int64  `json:"count"`
}

// TLSVersionStats holds TLS version distribution
type TLSVersionStats struct {
	TLSVersion string `json:"tls_version"`
	Count      int64  `json:"count"`
}

// UserAgentStats holds user agent statistics
type UserAgentStats struct {
	UserAgent string `json:"user_agent"`
	Count     int64  `json:"count"`
}

// BrowserStats holds browser statistics
type BrowserStats struct {
	Browser string `json:"browser"`
	Count   int64  `json:"count"`
}

// OSStats holds operating system statistics
type OSStats struct {
	OS    string `json:"os"`
	Count int64  `json:"count"`
}

// DeviceTypeStats holds device type statistics
type DeviceTypeStats struct {
	DeviceType string `json:"device_type"`
	Count      int64  `json:"count"`
}

// ReferrerStats holds referrer statistics
type ReferrerStats struct {
	Referrer       string `json:"referrer"`
	Hits           int64  `json:"hits"`
	UniqueVisitors int64  `json:"unique_visitors"`
}

// ReferrerDomainStats holds aggregated referrer domains
type ReferrerDomainStats struct {
	Domain         string `json:"domain"`
	Hits           int64  `json:"hits"`
	UniqueVisitors int64  `json:"unique_visitors"`
}

// BackendStats holds backend statistics
type BackendStats struct {
	BackendName     string  `json:"backend_name"`
	BackendURL      string  `json:"backend_url"`
	Hits            int64   `json:"hits"`
	Bandwidth       int64   `json:"bandwidth"`
	AvgResponseTime float64 `json:"avg_response_time"`
	ErrorCount      int64   `json:"error_count"`
}

// ASNStats holds ASN statistics
type ASNStats struct {
	ASN       int    `json:"asn"`
	ASNOrg    string `json:"asn_org"`
	Hits      int64  `json:"hits"`
	Bandwidth int64  `json:"bandwidth"`
	Country   string `json:"country"`
}

// ResponseTimeStats holds response time statistics
type ResponseTimeStats struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// LogProcessingStats holds log processing statistics
type LogProcessingStats struct {
	LogSourceName   string     `json:"log_source_name"`
	FileSize        int64      `json:"file_size"`
	BytesProcessed  int64      `json:"bytes_processed"`
	Percentage      float64    `json:"percentage"`
	LastProcessedAt *time.Time `json:"last_processed_at"`
}

// DomainStats holds domain/host statistics with request count
type DomainStats struct {
	Host  string `gorm:"column:host" json:"host"`
	Count int64  `gorm:"column:count" json:"count"`
}

// GetSummary returns overall statistics
func (r *statsRepo) GetSummary(host string) (*StatsSummary, error) {
	summary := &StatsSummary{}

	// Get time range (last 7 days by default)
	since := r.getTimeRange()

	// Total requests (with optional host filter)
	query := r.db.Table("http_requests").Where("timestamp > ?", since)
	query = r.applyHostFilter(query, host)
	query.Count(&summary.TotalRequests)

	// Valid requests (2xx + 3xx)
	query = r.db.Table("http_requests").Where("timestamp > ? AND status_code >= 200 AND status_code < 400", since)
	query = r.applyHostFilter(query, host)
	query.Count(&summary.ValidRequests)

	// Failed requests (4xx + 5xx)
	query = r.db.Table("http_requests").Where("timestamp > ? AND status_code >= 400", since)
	query = r.applyHostFilter(query, host)
	query.Count(&summary.FailedRequests)

	// Unique visitors
	query = r.db.Table("http_requests").Where("timestamp > ?", since)
	query = r.applyHostFilter(query, host)
	query.Distinct("client_ip").Count(&summary.UniqueVisitors)

	// Unique files (unique paths)
	query = r.db.Table("http_requests").Where("timestamp > ?", since)
	query = r.applyHostFilter(query, host)
	query.Distinct("path").Count(&summary.UniqueFiles)

	// Unique 404s
	query = r.db.Table("http_requests").Where("timestamp > ? AND status_code = 404", since)
	query = r.applyHostFilter(query, host)
	query.Distinct("path").Count(&summary.Unique404)

	// Total bandwidth
	query = r.db.Table("http_requests").Where("timestamp > ?", since)
	query = r.applyHostFilter(query, host)
	query.Select("COALESCE(SUM(response_size), 0)").Scan(&summary.TotalBandwidth)

	// Average response time
	query = r.db.Table("http_requests").Where("timestamp > ? AND response_time_ms > 0", since)
	query = r.applyHostFilter(query, host)
	query.Select("COALESCE(AVG(response_time_ms), 0)").Scan(&summary.AvgResponseTime)

	// Success rate (2xx and 3xx)
	if summary.TotalRequests > 0 {
		summary.SuccessRate = float64(summary.ValidRequests) / float64(summary.TotalRequests) * 100
	}

	// 404 rate
	var notFoundCount int64
	query = r.db.Table("http_requests").Where("timestamp > ? AND status_code = 404", since)
	query = r.applyHostFilter(query, host)
	query.Count(&notFoundCount)

	if summary.TotalRequests > 0 {
		summary.NotFoundRate = float64(notFoundCount) / float64(summary.TotalRequests) * 100
	}

	// Server error rate (5xx)
	var serverErrorCount int64
	query = r.db.Table("http_requests").Where("timestamp > ? AND status_code >= 500 AND status_code < 600", since)
	query = r.applyHostFilter(query, host)
	query.Count(&serverErrorCount)

	if summary.TotalRequests > 0 {
		summary.ServerErrorRate = float64(serverErrorCount) / float64(summary.TotalRequests) * 100
	}

	// Requests per hour
	summary.RequestsPerHour = float64(summary.TotalRequests) / 24.0

	// Top country
	query = r.db.Table("http_requests").Select("geo_country").Where("timestamp > ? AND geo_country != ''", since)
	query = r.applyHostFilter(query, host)
	query.Group("geo_country").Order("COUNT(*) DESC").Limit(1).Pluck("geo_country", &summary.TopCountry)

	// Top path
	query = r.db.Table("http_requests").Select("path").Where("timestamp > ?", since)
	query = r.applyHostFilter(query, host)
	query.Group("path").Order("COUNT(*) DESC").Limit(1).Pluck("path", &summary.TopPath)

	r.logger.Trace("Generated stats summary", r.logger.Args("total_requests", summary.TotalRequests, "host_filter", host))
	return summary, nil
}

// GetTimelineStats returns time-based statistics with adaptive granularity
func (r *statsRepo) GetTimelineStats(hours int, host string) ([]*TimelineData, error) {
	var timeline []*TimelineData
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	// Adaptive grouping based on time range
	var timeFormat string
	var groupBy string

	if hours <= 24 {
		// For 1 hour or 24 hours: group by hour
		timeFormat = "strftime('%Y-%m-%d %H:00', timestamp)"
		groupBy = timeFormat
	} else if hours <= 168 {
		// For 7 days: group by 6-hour blocks
		timeFormat = "strftime('%Y-%m-%d %H', timestamp)"
		groupBy = "strftime('%Y-%m-%d', timestamp) || ' ' || CAST((CAST(strftime('%H', timestamp) AS INTEGER) / 6) * 6 AS TEXT) || ':00'"
	} else if hours <= 720 {
		// For 30 days: group by day
		timeFormat = "strftime('%Y-%m-%d', timestamp)"
		groupBy = timeFormat
	} else {
		// For longer periods: group by week
		timeFormat = "strftime('%Y-W%W', timestamp)"
		groupBy = timeFormat
	}

	query := r.db.Model(&models.HTTPRequest{}).
		Select(groupBy+" as hour, COUNT(*) as requests, COUNT(DISTINCT client_ip) as unique_visitors, COALESCE(SUM(response_size), 0) as bandwidth, COALESCE(AVG(response_time_ms), 0) as avg_response_time").
		Where("timestamp > ?", since)

	query = r.applyHostFilter(query, host)
	query = query.Group(groupBy).Order("hour")

	err := query.Scan(&timeline).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get timeline stats", r.logger.Args("error", err))
		return nil, err
	}

	r.logger.Trace("Generated timeline stats", r.logger.Args("hours", hours, "data_points", len(timeline), "host_filter", host))
	return timeline, nil
}

// GetStatusCodeTimeline returns status code distribution over time
func (r *statsRepo) GetStatusCodeTimeline(hours int, host string) ([]*StatusCodeTimelineData, error) {
	var timeline []*StatusCodeTimelineData
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	// Simplified grouping - use only simple expressions that work in SQLite
	var groupBy string
	if hours <= 24 {
		// Group by hour for last 24 hours
		groupBy = "strftime('%Y-%m-%d %H:00', timestamp)"
	} else if hours <= 168 {
		// Group by day for last 7 days (simpler, works reliably)
		groupBy = "strftime('%Y-%m-%d', timestamp)"
	} else if hours <= 720 {
		// Group by day for last 30 days
		groupBy = "strftime('%Y-%m-%d', timestamp)"
	} else {
		// Group by week for longer periods
		groupBy = "strftime('%Y-W%W', timestamp)"
	}

	// Build the query with explicit grouping
	// Use COUNT instead of SUM(CASE WHEN) for better reliability
	// Note: In SQLite, we need to be careful with type comparisons
	query := r.db.Table("http_requests").
		Select(groupBy+" as hour, "+
			"COUNT(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 END) as status_2xx, "+
			"COUNT(CASE WHEN status_code >= 300 AND status_code < 400 THEN 1 END) as status_3xx, "+
			"COUNT(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 END) as status_4xx, "+
			"COUNT(CASE WHEN status_code >= 500 THEN 1 END) as status_5xx").
		Where("timestamp > ?", since)

	query = r.applyHostFilter(query, host)
	query = query.Group(groupBy).Order("hour")

	// Log the query for debugging
	r.logger.Debug("Executing status code timeline query",
		r.logger.Args("hours", hours, "since", since.Format("2006-01-02 15:04:05"), "groupBy", groupBy, "host_filter", host))

	err := query.Scan(&timeline).Error
	if err != nil {
		r.logger.WithCaller().Error("Failed to get status code timeline", r.logger.Args("error", err))
		return nil, err
	}

	if len(timeline) == 0 {
		r.logger.Warn("Status code timeline returned 0 data points",
			r.logger.Args("hours", hours, "since", since.Format("2006-01-02 15:04:05"), "host_filter", host))
	} else {
		r.logger.Info("Generated status code timeline",
			r.logger.Args("hours", hours, "data_points", len(timeline), "host_filter", host))
	}

	return timeline, nil
}

// GetTrafficHeatmap returns traffic metrics grouped by day of week and hour for heatmap visualisation
func (r *statsRepo) GetTrafficHeatmap(days int, host string) ([]*TrafficHeatmapData, error) {
	if days <= 0 {
		days = 30
	} else if days > 365 {
		days = 365
	}

	var heatmap []*TrafficHeatmapData
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	query := r.db.Model(&models.HTTPRequest{}).
		Select("CAST(strftime('%w', timestamp) AS INTEGER) as day_of_week, "+
			"CAST(strftime('%H', timestamp) AS INTEGER) as hour, "+
			"COUNT(*) as requests, COALESCE(AVG(response_time_ms), 0) as avg_response_time").
		Where("timestamp > ?", since)

	query = r.applyHostFilter(query, host)
	query = query.Group("day_of_week, hour").Order("day_of_week, hour")

	if err := query.Scan(&heatmap).Error; err != nil {
		r.logger.WithCaller().Error("Failed to get traffic heatmap", r.logger.Args("error", err))
		return nil, err
	}

	r.logger.Trace("Generated traffic heatmap", r.logger.Args("days", days, "data_points", len(heatmap), "host_filter", host))
	return heatmap, nil
}

// GetTopPaths returns most accessed paths
func (r *statsRepo) GetTopPaths(limit int, host string) ([]*PathStats, error) {
	var paths []*PathStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("path, COUNT(*) as hits, COUNT(DISTINCT client_ip) as unique_visitors, COALESCE(AVG(response_time_ms), 0) as avg_response_time, COALESCE(SUM(response_size), 0) as total_bandwidth").
		Where("timestamp > ?", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("path").Order("hits DESC").Limit(limit).Scan(&paths).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top paths", r.logger.Args("error", err))
		return nil, err
	}

	return paths, nil
}

// GetTopCountries returns top countries by requests
func (r *statsRepo) GetTopCountries(limit int, host string) ([]*CountryStats, error) {
	var countries []*CountryStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("geo_country as country, '' as country_name, COUNT(*) as hits, COUNT(DISTINCT client_ip) as unique_visitors, COALESCE(SUM(response_size), 0) as bandwidth").
		Where("timestamp > ? AND geo_country != ''", since)

	query = r.applyHostFilter(query, host)

	// Only apply limit if > 0 (0 means no limit - return all)
	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Group("geo_country").Order("hits DESC").Scan(&countries).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top countries", r.logger.Args("error", err))
		return nil, err
	}

	return countries, nil
}

// GetTopIPAddresses returns most active IP addresses
func (r *statsRepo) GetTopIPAddresses(limit int, host string) ([]*IPStats, error) {
	var ips []*IPStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("client_ip as ip_address, MAX(geo_country) as country, MAX(geo_city) as city, MAX(geo_lat) as latitude, MAX(geo_lon) as longitude, COUNT(*) as hits, COALESCE(SUM(response_size), 0) as bandwidth").
		Where("timestamp > ?", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("client_ip").Order("hits DESC").Limit(limit).Scan(&ips).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top IPs", r.logger.Args("error", err))
		return nil, err
	}

	return ips, nil
}

// GetStatusCodeDistribution returns status code distribution
func (r *statsRepo) GetStatusCodeDistribution(host string) ([]*StatusCodeStats, error) {
	var stats []*StatusCodeStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("status_code, COUNT(*) as count").
		Where("timestamp > ?", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("status_code").Order("count DESC").Scan(&stats).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get status code distribution", r.logger.Args("error", err))
		return nil, err
	}

	return stats, nil
}

// GetMethodDistribution returns HTTP method distribution
func (r *statsRepo) GetMethodDistribution(host string) ([]*MethodStats, error) {
	var stats []*MethodStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("method, COUNT(*) as count").
		Where("timestamp > ?", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("method").Order("count DESC").Scan(&stats).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get method distribution", r.logger.Args("error", err))
		return nil, err
	}

	return stats, nil
}

// GetProtocolDistribution returns HTTP protocol distribution
func (r *statsRepo) GetProtocolDistribution(host string) ([]*ProtocolStats, error) {
	var stats []*ProtocolStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("protocol, COUNT(*) as count").
		Where("timestamp > ? AND protocol != ''", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("protocol").Order("count DESC").Scan(&stats).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get protocol distribution", r.logger.Args("error", err))
		return nil, err
	}

	return stats, nil
}

// GetTLSVersionDistribution returns TLS version distribution
func (r *statsRepo) GetTLSVersionDistribution(host string) ([]*TLSVersionStats, error) {
	var stats []*TLSVersionStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("tls_version, COUNT(*) as count").
		Where("timestamp > ? AND tls_version != ''", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("tls_version").Order("count DESC").Scan(&stats).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get TLS version distribution", r.logger.Args("error", err))
		return nil, err
	}

	return stats, nil
}

// GetTopUserAgents returns most common user agents
func (r *statsRepo) GetTopUserAgents(limit int, host string) ([]*UserAgentStats, error) {
	var agents []*UserAgentStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("user_agent, COUNT(*) as count").
		Where("timestamp > ? AND user_agent != ''", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("user_agent").Order("count DESC").Limit(limit).Scan(&agents).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top user agents", r.logger.Args("error", err))
		return nil, err
	}

	return agents, nil
}

// GetTopReferrers returns most common referrers
func (r *statsRepo) GetTopReferrers(limit int, host string) ([]*ReferrerStats, error) {
	var referrers []*ReferrerStats
	since := r.getTimeRange()

	// Get actual referer headers with unique visitors
	query := r.db.Model(&models.HTTPRequest{}).
		Select("referer as referrer, COUNT(*) as hits, COUNT(DISTINCT client_ip) as unique_visitors").
		Where("timestamp > ? AND referer != ''", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("referer").Order("hits DESC").Limit(limit).Scan(&referrers).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top referrers", r.logger.Args("error", err))
		return nil, err
	}

	return referrers, nil
}

// GetTopReferrerDomains returns referrer domains aggregated by host
func (r *statsRepo) GetTopReferrerDomains(limit int, host string) ([]*ReferrerDomainStats, error) {
	var referrers []*ReferrerStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("referer as referrer, COUNT(*) as hits, COUNT(DISTINCT client_ip) as unique_visitors").
		Where("timestamp > ? AND referer != ''", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("referer").Order("hits DESC").Scan(&referrers).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get referrer domains", r.logger.Args("error", err))
		return nil, err
	}

	// Aggregate by domain
	domainData := make(map[string]*ReferrerDomainStats)
	for _, ref := range referrers {
		domain := extractDomain(ref.Referrer)
		if domain == "" {
			continue
		}
		if existing, ok := domainData[domain]; ok {
			existing.Hits += ref.Hits
			existing.UniqueVisitors += ref.UniqueVisitors
		} else {
			domainData[domain] = &ReferrerDomainStats{
				Domain:         domain,
				Hits:           ref.Hits,
				UniqueVisitors: ref.UniqueVisitors,
			}
		}
	}

	if len(domainData) == 0 {
		return []*ReferrerDomainStats{}, nil
	}

	domains := make([]*ReferrerDomainStats, 0, len(domainData))
	for _, stats := range domainData {
		domains = append(domains, stats)
	}

	sort.Slice(domains, func(i, j int) bool {
		if domains[i].Hits == domains[j].Hits {
			return domains[i].Domain < domains[j].Domain
		}
		return domains[i].Hits > domains[j].Hits
	})

	if limit > 0 && len(domains) > limit {
		domains = domains[:limit]
	}

	return domains, nil
}

// extractRedirectURL extracts the redirect URL from a query string
func extractRedirectURL(queryString string) string {
	if queryString == "" {
		return ""
	}

	// Find redirect parameter
	redirectIndex := strings.Index(queryString, "redirect=")
	if redirectIndex == -1 {
		return ""
	}

	// Extract the value after redirect=
	value := queryString[redirectIndex+9:]

	// Find the end of the parameter (next & or end of string)
	ampIndex := strings.Index(value, "&")
	if ampIndex != -1 {
		value = value[:ampIndex]
	}

	// URL decode if needed
	if decoded, err := url.QueryUnescape(value); err == nil {
		return decoded
	}

	return value
}

// extractDomain returns the host portion for a referrer URL
func extractDomain(raw string) string {
	if raw == "" {
		return ""
	}

	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return ""
	}

	if parsed, err := url.Parse(cleaned); err == nil {
		host := parsed.Host
		if host == "" {
			host = parsed.Path
		}
		host = strings.TrimSpace(host)
		if host != "" {
			host = strings.Split(host, ":")[0]
			host = strings.TrimPrefix(strings.ToLower(host), "www.")
			return host
		}
	}

	// Manual extraction fallback
	if idx := strings.Index(cleaned, "//"); idx != -1 {
		cleaned = cleaned[idx+2:]
	}

	cleaned = strings.Split(cleaned, "/")[0]
	cleaned = strings.Split(cleaned, "?")[0]
	cleaned = strings.Split(cleaned, "#")[0]
	cleaned = strings.Split(cleaned, ":")[0]
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}

	cleaned = strings.TrimPrefix(strings.ToLower(cleaned), "www.")
	return cleaned
}

// extractBackendName extracts the readable name from backend_name format
// Format: id-detail1-detail2-detailN-service@protocol
// Returns: detail1 detail2 detailN (with spaces instead of dashes)
func extractBackendName(backendName string) string {
	if backendName == "" {
		return ""
	}

	// Remove protocol suffix (e.g., @file, @docker, @http)
	parts := strings.Split(backendName, "@")
	if len(parts) > 0 {
		backendName = parts[0]
	}

	// Remove -service suffix
	backendName = strings.TrimSuffix(backendName, "-service")

	// Split by dash to get all parts
	parts = strings.Split(backendName, "-")

	// Skip first part (id) and last part if it's empty
	if len(parts) > 1 {
		// Remove first element (id) and join the rest with spaces
		details := parts[1:]
		return strings.Join(details, " ")
	}

	return backendName
}

// GetTopBackends returns backend statistics
func (r *statsRepo) GetTopBackends(limit int, host string) ([]*BackendStats, error) {
	var backends []*BackendStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("backend_name, MAX(backend_url) as backend_url, COUNT(*) as hits, COALESCE(SUM(response_size), 0) as bandwidth, COALESCE(AVG(response_time_ms), 0) as avg_response_time, SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) as error_count").
		Where("timestamp > ? AND backend_name != ''", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("backend_name").Order("hits DESC").Limit(limit).Scan(&backends).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top backends", r.logger.Args("error", err))
		return nil, err
	}

	return backends, nil
}

// GetTopASNs returns top ASNs by requests
func (r *statsRepo) GetTopASNs(limit int, host string) ([]*ASNStats, error) {
	var asns []*ASNStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("asn, MAX(asn_org) as asn_org, COUNT(*) as hits, COALESCE(SUM(response_size), 0) as bandwidth, MAX(geo_country) as country").
		Where("timestamp > ? AND asn > 0", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("asn").Order("hits DESC").Limit(limit).Scan(&asns).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top ASNs", r.logger.Args("error", err))
		return nil, err
	}

	return asns, nil
}

// GetResponseTimeStats returns response time statistics
// Optimized to use SQL for percentile calculation instead of loading all rows into memory
func (r *statsRepo) GetResponseTimeStats(host string) (*ResponseTimeStats, error) {
	stats := &ResponseTimeStats{}
	since := r.getTimeRange()

	// Get min, max, avg
	query := r.db.Model(&models.HTTPRequest{}).
		Select("COALESCE(MIN(response_time_ms), 0) as min, COALESCE(MAX(response_time_ms), 0) as max, COALESCE(AVG(response_time_ms), 0) as avg").
		Where("timestamp > ? AND response_time_ms > 0", since)

	query = r.applyHostFilter(query, host)
	err := query.Scan(stats).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get response time stats", r.logger.Args("error", err))
		return nil, err
	}

	// Get total count for percentile calculation
	var totalCount int64
	query = r.db.Model(&models.HTTPRequest{}).Where("timestamp > ? AND response_time_ms > 0", since)
	query = r.applyHostFilter(query, host)
	query.Count(&totalCount)

	if totalCount == 0 {
		return stats, nil
	}

	// Calculate percentiles using SQL LIMIT/OFFSET (memory efficient)
	// P50 (median)
	var p50 float64
	query = r.db.Model(&models.HTTPRequest{}).
		Select("response_time_ms").
		Where("timestamp > ? AND response_time_ms > 0", since)
	query = r.applyHostFilter(query, host)
	query.Order("response_time_ms").Limit(1).Offset(int(float64(totalCount)*0.50)).Pluck("response_time_ms", &p50)
	stats.P50 = p50

	// P95
	var p95 float64
	query = r.db.Model(&models.HTTPRequest{}).
		Select("response_time_ms").
		Where("timestamp > ? AND response_time_ms > 0", since)
	query = r.applyHostFilter(query, host)
	query.Order("response_time_ms").Limit(1).Offset(int(float64(totalCount)*0.95)).Pluck("response_time_ms", &p95)
	stats.P95 = p95

	// P99
	var p99 float64
	query = r.db.Model(&models.HTTPRequest{}).
		Select("response_time_ms").
		Where("timestamp > ? AND response_time_ms > 0", since)
	query = r.applyHostFilter(query, host)
	query.Order("response_time_ms").Limit(1).Offset(int(float64(totalCount)*0.99)).Pluck("response_time_ms", &p99)
	stats.P99 = p99

	return stats, nil
}

// GetLogProcessingStats returns log processing statistics
func (r *statsRepo) GetLogProcessingStats() ([]*LogProcessingStats, error) {
	var sources []models.LogSource

	// Get all log sources
	err := r.db.Find(&sources).Error
	if err != nil {
		r.logger.WithCaller().Error("Failed to get log sources", r.logger.Args("error", err))
		return nil, err
	}

	var stats []*LogProcessingStats

	for _, source := range sources {
		// Get file size
		fileInfo, err := os.Stat(source.Path)
		fileSize := int64(0)
		if err == nil {
			fileSize = fileInfo.Size()
		}

		percentage := 0.0
		if fileSize > 0 {
			percentage = float64(source.LastPosition) / float64(fileSize) * 100.0
		}

		stats = append(stats, &LogProcessingStats{
			LogSourceName:   source.Name,
			FileSize:        fileSize,
			BytesProcessed:  source.LastPosition,
			Percentage:      percentage,
			LastProcessedAt: source.LastReadAt,
		})
	}

	return stats, nil
}

// GetTopBrowsers returns most common browsers
func (r *statsRepo) GetTopBrowsers(limit int, host string) ([]*BrowserStats, error) {
	var browsers []*BrowserStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("browser, COUNT(*) as count").
		Where("timestamp > ? AND browser != '' AND browser != 'Unknown'", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("browser").Order("count DESC").Limit(limit).Scan(&browsers).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top browsers", r.logger.Args("error", err))
		return nil, err
	}

	return browsers, nil
}

// GetTopOperatingSystems returns most common operating systems
func (r *statsRepo) GetTopOperatingSystems(limit int, host string) ([]*OSStats, error) {
	var osList []*OSStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("os, COUNT(*) as count").
		Where("timestamp > ? AND os != '' AND os != 'Unknown'", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("os").Order("count DESC").Limit(limit).Scan(&osList).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get top operating systems", r.logger.Args("error", err))
		return nil, err
	}

	return osList, nil
}

// GetDeviceTypeDistribution returns distribution of device types
func (r *statsRepo) GetDeviceTypeDistribution(host string) ([]*DeviceTypeStats, error) {
	var devices []*DeviceTypeStats
	since := r.getTimeRange()

	query := r.db.Model(&models.HTTPRequest{}).
		Select("device_type, COUNT(*) as count").
		Where("timestamp > ? AND device_type != ''", since)

	query = r.applyHostFilter(query, host)
	err := query.Group("device_type").Order("count DESC").Scan(&devices).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get device type distribution", r.logger.Args("error", err))
		return nil, err
	}

	return devices, nil
}

// GetDomains returns all unique domains/hosts with their request counts
// Uses referer field as the domain identifier
func (r *statsRepo) GetDomains() ([]*DomainStats, error) {
	var rawDomains []*DomainStats

	err := r.db.Table("http_requests").
		Select("backend_name as host, COUNT(*) as count").
		Where("backend_name != ? AND backend_name != ? AND backend_name != ?", "next-service@file", "api-service@file", "").
		Group("backend_name").
		Order("count DESC").
		Scan(&rawDomains).Error

	if err != nil {
		r.logger.WithCaller().Error("Failed to get domains", r.logger.Args("error", err))
		return nil, err
	}

	// Extract and aggregate by formatted backend name
	domainMap := make(map[string]*DomainStats)
	for _, domain := range rawDomains {
		extractedName := extractBackendName(domain.Host)
		if extractedName == "" {
			continue
		}

		if existing, ok := domainMap[extractedName]; ok {
			// Aggregate counts for same extracted name
			existing.Count += domain.Count
		} else {
			domainMap[extractedName] = &DomainStats{
				Host:  extractedName,
				Count: domain.Count,
			}
		}
	}

	// Convert map to slice and sort by count
	domains := make([]*DomainStats, 0, len(domainMap))
	for _, domain := range domainMap {
		domains = append(domains, domain)
	}

	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Count > domains[j].Count
	})

	r.logger.Debug("Retrieved domains list (from backend_name)", r.logger.Args("count", len(domains)))
	return domains, nil
}
