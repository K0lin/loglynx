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

	// Create all indexes in a single batch for faster execution
	// IF NOT EXISTS makes this idempotent and fast on subsequent runs
	indexes := []string{
		// ===== COMPOSITE INDEXES (for common query patterns) =====

		// Time + Status (error analysis over time)
		`CREATE INDEX IF NOT EXISTS idx_time_status
		 ON http_requests(timestamp DESC, status_code)`,

		// Time + Host (per-service time-range queries)
		`CREATE INDEX IF NOT EXISTS idx_time_host
		 ON http_requests(timestamp DESC, host)`,

		// ClientIP + Time (IP activity timeline)
		`CREATE INDEX IF NOT EXISTS idx_ip_time
		 ON http_requests(client_ip, timestamp DESC)`,

		// Status + Host (service error rates)
		`CREATE INDEX IF NOT EXISTS idx_status_host
		 ON http_requests(status_code, host)`,

		// Response time index for percentile calculations
		`CREATE INDEX IF NOT EXISTS idx_response_time
		 ON http_requests(response_time_ms)
		 WHERE response_time_ms > 0`,

		// Composite index for timestamp + response_time for optimized percentile queries
		`CREATE INDEX IF NOT EXISTS idx_timestamp_response_time
		 ON http_requests(timestamp DESC, response_time_ms)
		 WHERE response_time_ms > 0`,

		// Composite index for summary queries (timestamp, status_code, response_time_ms)
		`CREATE INDEX IF NOT EXISTS idx_summary_query
		 ON http_requests(timestamp DESC, status_code, response_time_ms)`,

		// ===== PARTIAL INDEXES (for specific queries) =====

		// Errors only (40x and 50x status codes)
		`CREATE INDEX IF NOT EXISTS idx_errors_only
		 ON http_requests(timestamp DESC, status_code, path, method, client_ip)
		 WHERE status_code >= 400`,

		// Slow requests only (>1 second)
		`CREATE INDEX IF NOT EXISTS idx_slow_requests
		 ON http_requests(timestamp DESC, response_time_ms, path, host, method)
		 WHERE response_time_ms > 1000`,

		// Server errors only (50x)
		`CREATE INDEX IF NOT EXISTS idx_server_errors
		 ON http_requests(timestamp DESC, status_code, path, backend_name)
		 WHERE status_code >= 500`,

		// Requests with retries
		`CREATE INDEX IF NOT EXISTS idx_retried_requests
		 ON http_requests(timestamp DESC, retry_attempts, backend_name, status_code)
		 WHERE retry_attempts > 0`,

		// ===== COVERING INDEXES (include data columns) =====

		// Dashboard covering index (includes most displayed columns)
		// Note: Removed datetime() WHERE clause as it's non-deterministic in indexes
		`CREATE INDEX IF NOT EXISTS idx_dashboard_covering
		 ON http_requests(timestamp DESC, status_code, response_time_ms, host, client_ip, method, path)`,

		// Error analysis covering index
		// Note: Removed datetime() WHERE clause as it's non-deterministic in indexes
		`CREATE INDEX IF NOT EXISTS idx_error_analysis
		 ON http_requests(timestamp DESC, status_code, path, method, client_ip, response_time_ms, backend_name)
		 WHERE status_code >= 400`,

		// ===== CLEANUP INDEX =====
		// Index for cleanup queries (timestamp for deletion)
		`CREATE INDEX IF NOT EXISTS idx_timestamp_cleanup
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
