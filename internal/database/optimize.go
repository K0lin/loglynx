package database

import (
	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

// OptimizeDatabase applies additional optimizations after initial migrations
// This includes creating performance indexes and verifying SQLite settings
func OptimizeDatabase(db *gorm.DB, logger *pterm.Logger) error {
	logger.Info("Applying database optimizations...")

	// Verify WAL mode is enabled
	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		logger.Warn("Failed to check journal mode", logger.Args("error", err))
	} else {
		logger.Info("Database journal mode", logger.Args("mode", journalMode))
	}

	// Verify page size
	var pageSize int
	if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		logger.Warn("Failed to check page size", logger.Args("error", err))
	} else {
		logger.Info("Database page size", logger.Args("bytes", pageSize))
	}

	// Create index on response_time_ms if it doesn't exist
	// This dramatically improves percentile calculation performance
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_response_time
		ON http_requests(response_time_ms)
		WHERE response_time_ms > 0
	`).Error; err != nil {
		logger.Warn("Failed to create response_time index", logger.Args("error", err))
		return err
	}
	logger.Info("Response time index created/verified")

	// Create composite index for timestamp + response_time for optimized percentile queries
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_timestamp_response_time
		ON http_requests(timestamp, response_time_ms)
		WHERE response_time_ms > 0
	`).Error; err != nil {
		logger.Warn("Failed to create composite timestamp+response_time index", logger.Args("error", err))
		return err
	}
	logger.Info("Composite timestamp+response_time index created/verified")

	// Analyze tables for query optimizer
	if err := db.Exec("ANALYZE").Error; err != nil {
		logger.Warn("Failed to analyze database", logger.Args("error", err))
	} else {
		logger.Info("Database analysis completed")
	}

	logger.Info("Database optimizations applied successfully")
	return nil
}
