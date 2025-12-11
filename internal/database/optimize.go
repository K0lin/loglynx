package database

import (
	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

// OptimizeDatabase applies additional optimizations after initial migrations
// This includes creating performance indexes and verifying SQLite settings
func OptimizeDatabase(db *gorm.DB, logger *pterm.Logger) error {
	logger.Debug("Applying database optimizations...")

	// Verify WAL mode is enabled (debug level - only show if there's a problem)
	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		logger.Warn("Failed to check journal mode", logger.Args("error", err))
	} else if journalMode != "wal" {
		logger.Warn("Database not in WAL mode", logger.Args("mode", journalMode))
	} else {
		logger.Trace("Database journal mode verified", logger.Args("mode", journalMode))
	}

	// Verify page size (trace level - not critical)
	var pageSize int
	if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		logger.Debug("Failed to check page size", logger.Args("error", err))
	} else {
		logger.Trace("Database page size", logger.Args("bytes", pageSize))
	}

	indexes := []string{
		// ===== PRIMARY COMPOSITE INDEXES (for time-range queries) =====

		// Main timeline index - covers most dashboard queries
		// Used by: GetTimelineStats, GetStatusCodeTimeline, GetSummary
		`CREATE INDEX IF NOT EXISTS idx_timestamp_status
		 ON http_requests(timestamp DESC, status_code)`,

		// Time + Host - for per-service filtering
		// Used by: service-filtered queries
		`CREATE INDEX IF NOT EXISTS idx_time_host
		 ON http_requests(timestamp DESC, host)`,

		// Time + Backend - for backend analytics
		// Used by: GetTopBackends, backend-filtered queries
		`CREATE INDEX IF NOT EXISTS idx_time_backend
		 ON http_requests(timestamp DESC, backend_name, status_code)`,

		// ===== AGGREGATION INDEXES (for GROUP BY queries) =====

		// Path aggregation - CRITICAL for GetTopPaths
		// Covering index includes all columns needed for the query
		`CREATE INDEX IF NOT EXISTS idx_path_agg
		 ON http_requests(path, timestamp, client_ip, response_time_ms, response_size)`,

		// Country aggregation - CRITICAL for GetTopCountries
		// Partial index excludes empty geo_country for efficiency
		`CREATE INDEX IF NOT EXISTS idx_geo_agg
		 ON http_requests(geo_country, timestamp, client_ip, response_size)
		 WHERE geo_country != ''`,

		// Referrer aggregation - CRITICAL for GetTopReferrerDomains
		// Partial index excludes empty referrers
		`CREATE INDEX IF NOT EXISTS idx_referer_agg
		 ON http_requests(referer, timestamp, client_ip)
		 WHERE referer != ''`,

		// Service identification - for GetServices
		// Covers the UNION ALL query pattern
		`CREATE INDEX IF NOT EXISTS idx_service_id
		 ON http_requests(backend_name, backend_url, host)`,

		// ===== LOOKUP INDEXES (for filtering and detail queries) =====

		// Client IP lookup - for IP detail pages and exclusion
		// Used by: GetIPDetailedStats, exclude_own_ip filter
		`CREATE INDEX IF NOT EXISTS idx_client_ip
		 ON http_requests(client_ip, timestamp DESC)`,

		// Status code lookup - for distribution queries
		// Used by: GetStatusCodeDistribution
		`CREATE INDEX IF NOT EXISTS idx_status_code
		 ON http_requests(status_code, timestamp)`,

		// Method lookup - for distribution queries
		// Used by: GetMethodDistribution
		`CREATE INDEX IF NOT EXISTS idx_method
		 ON http_requests(method, timestamp)`,

		// ===== PARTIAL INDEXES (for specific filtered queries) =====

		// Errors only - for error analysis dashboards
		// Partial index significantly reduces size (typically <5% of data)
		`CREATE INDEX IF NOT EXISTS idx_errors
		 ON http_requests(timestamp DESC, status_code, path, client_ip)
		 WHERE status_code >= 400`,

		// Slow requests - for performance analysis
		// Partial index for requests >1 second
		`CREATE INDEX IF NOT EXISTS idx_slow
		 ON http_requests(timestamp DESC, response_time_ms, path, host)
		 WHERE response_time_ms > 1000`,

		// Response time percentiles - for P50/P95/P99 calculations
		// Partial index excludes zero/null response times
		`CREATE INDEX IF NOT EXISTS idx_response_time
		 ON http_requests(timestamp DESC, response_time_ms)
		 WHERE response_time_ms > 0`,

		// ===== MAINTENANCE INDEX =====

		// Cleanup index - for data retention queries
		// Simple timestamp index for DELETE operations
		`CREATE INDEX IF NOT EXISTS idx_cleanup
		 ON http_requests(timestamp)`,
	}

	indexCount := 0
	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			logger.Warn("Failed to create index", logger.Args("error", err))
			return err
		}
		indexCount++
	}

	logger.Debug("Performance indexes verified", logger.Args("count", indexCount))

	// Analyze tables for query optimizer (only log if it fails)
	if err := db.Exec("ANALYZE").Error; err != nil {
		logger.Warn("Failed to analyze database", logger.Args("error", err))
	} else {
		logger.Trace("Database statistics analyzed")
	}

	logger.Debug("Database optimizations completed")
	return nil
}
