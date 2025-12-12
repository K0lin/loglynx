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

		// Backend aggregation - CRITICAL for GetTopBackends (was taking 3s)
		// Covering index for backend_name grouping with all aggregation columns
		`CREATE INDEX IF NOT EXISTS idx_backend_agg
		 ON http_requests(backend_name, timestamp, backend_url, host, response_size, status_code)
		 WHERE backend_name != ''`,

		// Backend URL fallback aggregation - for requests without backend_name
		`CREATE INDEX IF NOT EXISTS idx_backend_url_agg
		 ON http_requests(backend_url, timestamp, host, response_size, status_code)
		 WHERE backend_name = '' AND backend_url != ''`,

		// Host fallback aggregation - for requests without backend_name or backend_url
		`CREATE INDEX IF NOT EXISTS idx_host_agg
		 ON http_requests(host, timestamp, response_size, status_code)
		 WHERE backend_name = '' AND backend_url = '' AND host != ''`,

		// ===== LOOKUP INDEXES (for filtering and detail queries) =====

		// Client IP aggregation - CRITICAL for GetTopIPAddresses and GetIPDetailedStats
		// Covering index includes geo data, ASN, and aggregation columns to avoid table lookups
		`CREATE INDEX IF NOT EXISTS idx_ip_agg
		 ON http_requests(client_ip, timestamp, geo_country, geo_city, geo_lat, geo_lon, response_size, asn, asn_org, status_code)`,

		// IP + Browser aggregation - for GetIPTopBrowsers
		// Partial index matches WHERE condition in query exactly
		`CREATE INDEX IF NOT EXISTS idx_ip_browser_agg
		 ON http_requests(client_ip, timestamp, browser)
		 WHERE browser != ''`,

		// IP + Backend aggregation - for GetIPTopBackends
		// Partial index matches WHERE condition, includes all aggregation columns
		`CREATE INDEX IF NOT EXISTS idx_ip_backend_agg
		 ON http_requests(client_ip, timestamp, backend_name, backend_url, response_size, response_time_ms, status_code)
		 WHERE backend_name != ''`,

		// IP + Device aggregation - for GetIPDeviceTypeDistribution
		// Partial index matches WHERE condition in query
		`CREATE INDEX IF NOT EXISTS idx_ip_device_agg
		 ON http_requests(client_ip, timestamp, device_type)
		 WHERE device_type != ''`,

		// IP + OS aggregation - for GetIPTopOperatingSystems
		// Partial index matches WHERE condition in query
		`CREATE INDEX IF NOT EXISTS idx_ip_os_agg
		 ON http_requests(client_ip, timestamp, os)
		 WHERE os != ''`,

		// IP + Status Code aggregation - for GetIPStatusCodeDistribution
		// Covering index for status code grouping
		`CREATE INDEX IF NOT EXISTS idx_ip_status_agg
		 ON http_requests(client_ip, timestamp, status_code)`,

		// IP + Path aggregation - for GetIPTopPaths
		// Covering index includes all columns needed for aggregation
		`CREATE INDEX IF NOT EXISTS idx_ip_path_agg
		 ON http_requests(client_ip, timestamp, path, response_time_ms, response_size, backend_name, host, backend_url)`,

		// IP + Heatmap/Timeline aggregation - for GetIPTrafficHeatmap and GetIPTimelineStats
		// Covering index for time-based aggregations with response metrics
		`CREATE INDEX IF NOT EXISTS idx_ip_heatmap_agg
		 ON http_requests(client_ip, timestamp, response_time_ms, response_size, backend_name)`,

		// Status code lookup - for distribution queries
		// Used by: GetStatusCodeDistribution
		`CREATE INDEX IF NOT EXISTS idx_status_code
		 ON http_requests(status_code, timestamp)`,

		// Method lookup - for distribution queries
		// Used by: GetMethodDistribution
		`CREATE INDEX IF NOT EXISTS idx_method
		 ON http_requests(method, timestamp)`,

		// ASN aggregation - for GetTopASNs
		// Partial index for records with valid ASN
		`CREATE INDEX IF NOT EXISTS idx_asn_agg
		 ON http_requests(asn, timestamp, asn_org, geo_country, response_size)
		 WHERE asn > 0`,

		// Device type distribution - for GetDeviceTypeDistribution  
		// Partial index for records with device type
		`CREATE INDEX IF NOT EXISTS idx_device_type
		 ON http_requests(device_type, timestamp)
		 WHERE device_type != ''`,

		// Protocol distribution - for GetProtocolDistribution
		// Partial index for records with protocol
		`CREATE INDEX IF NOT EXISTS idx_protocol
		 ON http_requests(protocol, timestamp)
		 WHERE protocol != ''`,

		// TLS version distribution - for GetTLSVersionDistribution
		// Partial index for records with TLS version
		`CREATE INDEX IF NOT EXISTS idx_tls_version
		 ON http_requests(tls_version, timestamp)
		 WHERE tls_version != ''`,

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
