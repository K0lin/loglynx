// MIT License
//
// Copyright (c) 2026 Kolin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
package repositories

import (
	"loglynx/internal/database/models"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Old indexes that need to be dropped during database optimization
var oldIndexes = []string{
	"idx_source_name",
	"idx_timestamp",
	"idx_partition_key",
	"idx_client_ip", // Will be recreated with different definition
	"idx_host",
	"idx_status",
	"idx_response_time",
	"idx_retry_attempts",
	"idx_browser",
	"idx_os",
	"idx_device_type", // Will be recreated
	"idx_router_name",
	"idx_request_id",
	"idx_trace_id",
	"idx_geo_country",
	"idx_created_at",
	"idx_time_status",
	"idx_ip_time",
	"idx_status_host",
	"idx_timestamp_response_time",
	"idx_summary_query",
	"idx_errors_only",
	"idx_slow_requests",
	"idx_server_errors",
	"idx_retried_requests",
	"idx_dashboard_covering",
	"idx_error_analysis",
	"idx_timestamp_cleanup",
}

// New optimized indexes that should exist after optimization
var newIndexes = []string{
	"idx_timestamp_status",
	"idx_time_host",
	"idx_time_backend",
	"idx_path_agg",
	"idx_geo_agg",
	"idx_referer_agg",
	"idx_service_id",
	"idx_backend_agg",
	"idx_backend_url_agg",
	"idx_host_agg",
	"idx_client_ip",
	"idx_status_code",
	"idx_method",
	"idx_errors",
	"idx_slow",
	"idx_response_time",
	"idx_ip_agg",
	"idx_ip_browser_agg",
	"idx_ip_backend_agg",
	"idx_ip_device_agg",
	"idx_ip_os_agg",
	"idx_ip_status_agg",
	"idx_ip_path_agg",
	"idx_ip_heatmap_agg",
	"idx_asn_agg",
	"idx_device_type",
	"idx_protocol",
	"idx_tls_version",
	"idx_cleanup",
}

// HTTPRequestRepository handles CRUD operations for HTTP requests
type HTTPRequestRepository interface {
	Create(request *models.HTTPRequest) error
	CreateBatch(requests []*models.HTTPRequest) error
	FindByID(id uint) (*models.HTTPRequest, error)
	FindAll(limit int, offset int, serviceName string, serviceType string, clientIP string, excludeServices []ServiceFilter) ([]*models.HTTPRequest, error)
	FindBySourceName(sourceName string, limit int) ([]*models.HTTPRequest, error)
	FindByTimeRange(start, end time.Time, limit int) ([]*models.HTTPRequest, error)
	Count() (int64, error)
	CountBySourceName(sourceName string) (int64, error)
	// First-load optimization control
	DisableFirstLoadMode()
}

type httpRequestRepo struct {
	db            *gorm.DB
	logger        *pterm.Logger
	isFirstLoad   bool       // Global flag: true when database is empty at startup
	firstLoadMu   sync.Mutex // Protects isFirstLoad flag
	firstLoadOnce sync.Once  // Ensures first-load check happens only once
}

// NewHTTPRequestRepository creates a new HTTP request repository
func NewHTTPRequestRepository(db *gorm.DB, logger *pterm.Logger) HTTPRequestRepository {
	repo := &httpRequestRepo{
		db:          db,
		logger:      logger,
		isFirstLoad: false, // Will be checked on first CreateBatch call
	}
	return repo
}

// checkFirstLoad checks if database is empty (only once, at startup)
// This is thread-safe and executes only on the first call
func (r *httpRequestRepo) checkFirstLoad() {
	r.firstLoadOnce.Do(func() {
		var count int64
		r.db.Model(&models.HTTPRequest{}).Count(&count)

		r.firstLoadMu.Lock()
		r.isFirstLoad = (count == 0)
		r.firstLoadMu.Unlock()

		if r.isFirstLoad {
			r.logger.Info("First load detected - deduplication checks will be skipped for optimal performance")
		} else {
			// For existing databases, check if they need index upgrade
			r.checkAndUpgradeIndexes()
		}
	})
}

// checkAndUpgradeIndexes checks if the database has the new optimized indexes
// If not, runs the optimization in the background
func (r *httpRequestRepo) checkAndUpgradeIndexes() {
	// Check if all new indexes exist (if any are missing, upgrade is needed)
	var missingCount int64
	query := `SELECT COUNT(*) FROM (SELECT 1 FROM sqlite_master WHERE type='index' AND name IN (`
	args := make([]interface{}, len(newIndexes))
	for i, indexName := range newIndexes {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = indexName
	}
	query += ")) WHERE 1 = 0" // This will count how many are missing

	// Actually, let's count how many new indexes exist
	query = `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name IN (`
	args = make([]interface{}, len(newIndexes))
	for i, indexName := range newIndexes {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = indexName
	}
	query += ")"

	err := r.db.Raw(query, args...).Scan(&missingCount).Error
	if err != nil {
		r.logger.Warn("Failed to check database indexes", r.logger.Args("error", err))
		return
	}

	// If not all new indexes exist, we need to upgrade
	if int(missingCount) < len(newIndexes) {
		r.logger.Info("Database indexes need upgrade - applying optimizations in background...",
			r.logger.Args("existing_indexes", missingCount, "required_indexes", len(newIndexes)))
		go func() {
			if err := r.optimizeDatabase(); err != nil {
				r.logger.Error("Failed to upgrade database indexes", r.logger.Args("error", err))
			} else {
				r.logger.Info("Database indexes upgraded successfully")
			}
		}()
	} else {
		r.logger.Debug("Database indexes are up to date")
	}
}

// DisableFirstLoadMode disables first-load optimization
// Called after the initial file load is complete
// Also triggers deferred index creation if this was the first load
func (r *httpRequestRepo) DisableFirstLoadMode() {
	r.firstLoadMu.Lock()
	wasFirstLoad := r.isFirstLoad
	if r.isFirstLoad {
		r.isFirstLoad = false
	}
	r.firstLoadMu.Unlock()

	if wasFirstLoad {
		r.logger.Info("First load completed - deduplication checks now enabled")

		// Create indexes in background (don't block log processing)
		go r.createDeferredIndexes()
	}
}

// createDeferredIndexes creates performance indexes after initial data load
func (r *httpRequestRepo) createDeferredIndexes() {
	r.logger.Info("🔨 Creating performance indexes in background (this may take a few minutes)...")

	startTime := time.Now()

	// Import optimize package function
	// Note: This requires OptimizeDatabase to be accessible
	// For now, we'll implement a simplified version here
	if err := r.optimizeDatabase(); err != nil {
		r.logger.Error("Failed to create performance indexes",
			r.logger.Args("error", err, "elapsed", time.Since(startTime)))
		return
	}

	elapsed := time.Since(startTime)
	r.logger.Info("✅ Performance indexes created successfully",
		r.logger.Args("elapsed_seconds", elapsed.Seconds()))
}

// optimizeDatabase creates all performance indexes
// This is a copy of OptimizeDatabase function to avoid circular dependencies
// INDEX STRATEGY: ~15 optimized indexes for read-heavy analytics workload
func (r *httpRequestRepo) optimizeDatabase() error {
	// Drop old indexes that are no longer used or have been replaced
	r.logger.Debug("Dropping old indexes", r.logger.Args("count", len(oldIndexes)))
	for _, indexName := range oldIndexes {
		dropSQL := "DROP INDEX IF EXISTS " + indexName
		if err := r.db.Exec(dropSQL).Error; err != nil {
			r.logger.Warn("Failed to drop old index", r.logger.Args("index", indexName, "error", err))
			// Continue with other drops
		} else {
			r.logger.Trace("Dropped old index", r.logger.Args("index", indexName))
		}
	}

	indexes := []string{
		// ===== PRIMARY COMPOSITE INDEXES (for time-range queries) =====

		// Main timeline index - covers most dashboard queries
		`CREATE INDEX IF NOT EXISTS idx_timestamp_status
		 ON http_requests(timestamp DESC, status_code)`,

		// Time + Host - for per-service filtering
		`CREATE INDEX IF NOT EXISTS idx_time_host
		 ON http_requests(timestamp DESC, host)`,

		// Time + Backend - for backend analytics
		`CREATE INDEX IF NOT EXISTS idx_time_backend
		 ON http_requests(timestamp DESC, backend_name, status_code)`,

		// ===== AGGREGATION INDEXES (for GROUP BY queries) =====

		// Path aggregation - CRITICAL for GetTopPaths
		`CREATE INDEX IF NOT EXISTS idx_path_agg
		 ON http_requests(path, timestamp, client_ip, response_time_ms, response_size)`,

		// Country aggregation - CRITICAL for GetTopCountries
		`CREATE INDEX IF NOT EXISTS idx_geo_agg
		 ON http_requests(geo_country, timestamp, client_ip, response_size)
		 WHERE geo_country != ''`,

		// Referrer aggregation - CRITICAL for GetTopReferrerDomains
		`CREATE INDEX IF NOT EXISTS idx_referer_agg
		 ON http_requests(referer, timestamp, client_ip)
		 WHERE referer != ''`,

		// Service identification - for GetServices
		`CREATE INDEX IF NOT EXISTS idx_service_id
		 ON http_requests(backend_name, backend_url, host)`,

		// Backend aggregation - CRITICAL for GetTopBackends
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

		// Client IP lookup - for IP detail pages
		`CREATE INDEX IF NOT EXISTS idx_client_ip
		 ON http_requests(client_ip, timestamp DESC)`,

		// Status code lookup - for distribution queries
		`CREATE INDEX IF NOT EXISTS idx_status_code
		 ON http_requests(status_code, timestamp)`,

		// Method lookup - for distribution queries
		`CREATE INDEX IF NOT EXISTS idx_method
		 ON http_requests(method, timestamp)`,

		// ===== PARTIAL INDEXES (for specific filtered queries) =====

		// Errors only - for error analysis
		`CREATE INDEX IF NOT EXISTS idx_errors
		 ON http_requests(timestamp DESC, status_code, path, client_ip)
		 WHERE status_code >= 400`,

		// Slow requests - for performance analysis
		`CREATE INDEX IF NOT EXISTS idx_slow
		 ON http_requests(timestamp DESC, response_time_ms, path, host)
		 WHERE response_time_ms > 1000`,

		// Response time percentiles
		`CREATE INDEX IF NOT EXISTS idx_response_time
		 ON http_requests(timestamp DESC, response_time_ms)
		 WHERE response_time_ms > 0`,

		// ===== IP AND CLIENT AGGREGATION INDEXES =====

		// IP aggregation - CRITICAL for GetTopIPAddresses and GetIPDetailedStats
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

		// ASN aggregation - for GetTopASNs
		`CREATE INDEX IF NOT EXISTS idx_asn_agg
		 ON http_requests(asn, asn_org, timestamp, client_ip, response_size)
		 WHERE asn != ''`,

		// Device type aggregation - for GetBrowserStats
		`CREATE INDEX IF NOT EXISTS idx_device_type
		 ON http_requests(device_type, timestamp)
		 WHERE device_type != ''`,

		// Protocol aggregation
		`CREATE INDEX IF NOT EXISTS idx_protocol
		 ON http_requests(protocol, timestamp)
		 WHERE protocol != ''`,

		// TLS version aggregation
		`CREATE INDEX IF NOT EXISTS idx_tls_version
		 ON http_requests(tls_version, timestamp)
		 WHERE tls_version != ''`,

		// ===== MAINTENANCE INDEX =====

		// Cleanup index - for data retention
		`CREATE INDEX IF NOT EXISTS idx_cleanup
		 ON http_requests(timestamp)`,
	}

	indexCount := 0
	for i, indexSQL := range indexes {
		r.logger.Debug("Creating index",
			r.logger.Args("progress", i+1, "total", len(indexes)))

		if err := r.db.Exec(indexSQL).Error; err != nil {
			r.logger.Warn("Failed to create index", r.logger.Args("error", err))
			return err
		}
		indexCount++
	}

	r.logger.Debug("Performance indexes created", r.logger.Args("count", indexCount))

	// Analyze tables for query optimizer
	if err := r.db.Exec("ANALYZE").Error; err != nil {
		r.logger.Warn("Failed to analyze database", r.logger.Args("error", err))
	} else {
		r.logger.Trace("Database statistics analyzed")
	}

	return nil
}

// getFirstLoadStatus returns current first-load status (thread-safe)
func (r *httpRequestRepo) getFirstLoadStatus() bool {
	r.firstLoadMu.Lock()
	defer r.firstLoadMu.Unlock()
	return r.isFirstLoad
}

// Create inserts a single HTTP request
func (r *httpRequestRepo) Create(request *models.HTTPRequest) error {
	if err := r.db.Create(request).Error; err != nil {
		r.logger.WithCaller().Error("Failed to create HTTP request", r.logger.Args("error", err))
		return err
	}
	r.logger.Trace("Created HTTP request", r.logger.Args("id", request.ID, "source", request.SourceName))
	return nil
}

// CreateBatch inserts multiple HTTP requests in a single transaction
// OPTIMIZED: Automatically splits large batches to avoid SQLite variable limit (32766)
// OPTIMIZED: Skips deduplication checks on first load (when database is empty)
func (r *httpRequestRepo) CreateBatch(requests []*models.HTTPRequest) error {
	if len(requests) == 0 {
		r.logger.Debug("Empty batch, skipping insert")
		return nil
	}

	// Check first-load status (thread-safe, happens only once globally)
	r.checkFirstLoad()
	isFirstLoad := r.getFirstLoadStatus()

	// SQLite has a variable limit (default 32766 for older versions, 999 in some configs)
	// HTTPRequest has 49 columns (including requests_total field), so max safe batch size is ~668 records
	// OPTIMIZATION: Increased from 15 to 500+ for significantly better throughput
	// 500 records * 49 columns = 24,500 variables (well under 32,766 limit)
	const MaxRecordsPerBatch = 50 // Slight safety margin under theoretical limit

	// If batch is small enough, insert directly
	if len(requests) <= MaxRecordsPerBatch {
		return r.insertSubBatch(requests, isFirstLoad)
	}

	// Split large batches into smaller chunks
	r.logger.Debug("Splitting large batch to avoid variable limit",
		r.logger.Args("total_records", len(requests), "max_per_batch", MaxRecordsPerBatch))

	totalInserted := 0
	for i := 0; i < len(requests); i += MaxRecordsPerBatch {
		end := i + MaxRecordsPerBatch
		if end > len(requests) {
			end = len(requests)
		}

		subBatch := requests[i:end]
		if err := r.insertSubBatch(subBatch, isFirstLoad); err != nil {
			r.logger.WithCaller().Error("Failed to insert sub-batch",
				r.logger.Args("batch_num", (i/MaxRecordsPerBatch)+1, "count", len(subBatch), "error", err))
			return err
		}

		totalInserted += len(subBatch)
		r.logger.Trace("Inserted sub-batch",
			r.logger.Args("progress", totalInserted, "total", len(requests)))
	}

	r.logger.Debug("Successfully inserted large batch in chunks",
		r.logger.Args("total_records", len(requests), "source", requests[0].SourceName))

	return nil
}

// insertSubBatch performs the actual batch insert within SQLite variable limits
func (r *httpRequestRepo) insertSubBatch(requests []*models.HTTPRequest, isFirstLoad bool) error {
	// OPTIMIZATION: Deduplicate in-memory BEFORE inserting to avoid rollbacks
	// This prevents expensive transaction rollbacks and re-inserts
	uniqueRequests := make([]*models.HTTPRequest, 0, len(requests))
	seen := make(map[string]bool, len(requests))
	inBatchDuplicates := 0

	for _, req := range requests {
		if req.RequestHash == "" {
			// Should never happen, but handle gracefully
			uniqueRequests = append(uniqueRequests, req)
			continue
		}

		if seen[req.RequestHash] {
			inBatchDuplicates++
			continue // Skip duplicate within this batch
		}

		seen[req.RequestHash] = true
		uniqueRequests = append(uniqueRequests, req)
	}

	if inBatchDuplicates > 0 {
		r.logger.Debug("Removed in-batch duplicates before insert",
			r.logger.Args("original", len(requests), "unique", len(uniqueRequests), "duplicates", inBatchDuplicates))
	}

	// If all were duplicates, skip the insert entirely
	if len(uniqueRequests) == 0 {
		r.logger.Debug("All records in batch were duplicates, skipping insert")
		return nil
	}

	if isFirstLoad {
		inserted, err := r.insertSubBatchRaw(uniqueRequests)
		if err != nil {
			r.logger.WithCaller().Error("Failed to insert batch via raw SQL",
				r.logger.Args("count", len(uniqueRequests), "error", err))
			return err
		}

		duplicates := len(uniqueRequests) - inserted
		if duplicates > 0 {
			r.logger.Debug("Initial load raw insert skipped duplicates",
				r.logger.Args("batch_size", len(uniqueRequests), "inserted", inserted, "duplicates", duplicates))
		}
		return nil
	}

	// Start transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		r.logger.WithCaller().Error("Failed to begin transaction", r.logger.Args("error", tx.Error))
		return tx.Error
	}

	// Use INSERT OR IGNORE semantics to skip duplicates without per-row retries
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "request_hash"}},
		DoNothing: true,
	}).Create(&uniqueRequests)
	if result.Error != nil {
		tx.Rollback()
		r.logger.WithCaller().Error("Failed to insert batch",
			r.logger.Args("count", len(uniqueRequests), "error", result.Error))
		return result.Error
	}

	if err := tx.Commit().Error; err != nil {
		r.logger.WithCaller().Error("Failed to commit transaction", r.logger.Args("error", err))
		return err
	}

	inserted := int(result.RowsAffected)
	duplicates := len(uniqueRequests) - inserted
	if duplicates > 0 {
		logFn := r.logger.Debug
		message := "Skipped duplicate entries"
		if isFirstLoad {
			message = "Skipped duplicate entries from initial file"
		}
		logFn(message,
			r.logger.Args(
				"batch_size", len(uniqueRequests),
				"inserted", inserted,
				"duplicates", duplicates,
			))
	}

	return nil
}

// insertSubBatchRaw performs a high-throughput INSERT for initial load using raw SQL
func (r *httpRequestRepo) insertSubBatchRaw(requests []*models.HTTPRequest) (int, error) {
	columns := []string{
		"source_name",
		"timestamp",
		"request_hash",
		"partition_key",
		"client_ip",
		"client_port",
		"client_user",
		"method",
		"protocol",
		"host",
		"path",
		"query_string",
		"request_length",
		"request_scheme",
		"status_code",
		"response_size",
		"response_time_ms",
		"response_content_type",
		"duration",
		"start_utc",
		"upstream_response_time_ms",
		"retry_attempts",
		"requests_total",
		"user_agent",
		"referer",
		"browser",
		"browser_version",
		"os",
		"os_version",
		"device_type",
		"backend_name",
		"backend_url",
		"router_name",
		"upstream_status",
		"upstream_content_type",
		"client_hostname",
		"tls_version",
		"tls_cipher",
		"tls_server_name",
		"request_id",
		"trace_id",
		"geo_country",
		"geo_city",
		"geo_lat",
		"geo_lon",
		"asn",
		"asn_org",
		"proxy_metadata",
		"created_at",
	}

	placeholder := "(" + strings.TrimRight(strings.Repeat("?,", len(columns)), ",") + ")"
	var queryBuilder strings.Builder
	queryBuilder.WriteString("INSERT INTO http_requests (")
	queryBuilder.WriteString(strings.Join(columns, ","))
	queryBuilder.WriteString(") VALUES ")

	args := make([]interface{}, 0, len(columns)*len(requests))
	now := time.Now()
	for i, req := range requests {
		if i > 0 {
			queryBuilder.WriteString(",")
		}
		queryBuilder.WriteString(placeholder)

		if req.Timestamp.IsZero() {
			req.Timestamp = now
		}
		if req.PartitionKey == "" {
			req.PartitionKey = req.Timestamp.Format("2006-01")
		}
		if req.CreatedAt.IsZero() {
			req.CreatedAt = now
		}

		args = append(args,
			req.SourceName,
			req.Timestamp,
			req.RequestHash,
			req.PartitionKey,
			req.ClientIP,
			req.ClientPort,
			req.ClientUser,
			req.Method,
			req.Protocol,
			req.Host,
			req.Path,
			req.QueryString,
			req.RequestLength,
			req.RequestScheme,
			req.StatusCode,
			req.ResponseSize,
			req.ResponseTimeMs,
			req.ResponseContentType,
			req.Duration,
			req.StartUTC,
			req.UpstreamResponseTimeMs,
			req.RetryAttempts,
			req.RequestsTotal,
			req.UserAgent,
			req.Referer,
			req.Browser,
			req.BrowserVersion,
			req.OS,
			req.OSVersion,
			req.DeviceType,
			req.BackendName,
			req.BackendURL,
			req.RouterName,
			req.UpstreamStatus,
			req.UpstreamContentType,
			req.ClientHostname,
			req.TLSVersion,
			req.TLSCipher,
			req.TLSServerName,
			req.RequestID,
			req.TraceID,
			req.GeoCountry,
			req.GeoCity,
			req.GeoLat,
			req.GeoLon,
			req.ASN,
			req.ASNOrg,
			req.ProxyMetadata,
			req.CreatedAt,
		)
	}

	queryBuilder.WriteString(" ON CONFLICT(request_hash) DO NOTHING")

	result := r.db.Exec(queryBuilder.String(), args...)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// FindByID retrieves an HTTP request by ID
func (r *httpRequestRepo) FindByID(id uint) (*models.HTTPRequest, error) {
	var request models.HTTPRequest
	if err := r.db.First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			r.logger.Trace("HTTP request not found", r.logger.Args("id", id))
			return nil, err
		}
		r.logger.WithCaller().Error("Failed to find HTTP request", r.logger.Args("id", id, "error", err))
		return nil, err
	}
	return &request, nil
}

// FindAll retrieves all HTTP requests with pagination
func (r *httpRequestRepo) FindAll(limit int, offset int, serviceName string, serviceType string, clientIP string, excludeServices []ServiceFilter) ([]*models.HTTPRequest, error) {
	var requests []*models.HTTPRequest
	query := r.db.Order("timestamp DESC")

	// Apply service filter if provided
	query = r.applyServiceFilter(query, serviceName, serviceType)

	// Apply exclude own IP if specified
	if clientIP != "" {
		if len(excludeServices) == 0 {
			query = query.Where("client_ip != ?", clientIP)
		} else {
			// Build exclude condition for specific services
			serviceConds := []string{}
			args := []interface{}{clientIP}
			for _, filter := range excludeServices {
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
			if len(serviceConds) > 0 {
				whereClause := "NOT (client_ip = ? AND (" + strings.Join(serviceConds, " OR ") + "))"
				query = query.Where(whereClause, args...)
			}
		}
	}

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&requests).Error; err != nil {
		r.logger.WithCaller().Error("Failed to find HTTP requests", r.logger.Args("error", err))
		return nil, err
	}

	r.logger.Trace("Found HTTP requests", r.logger.Args("count", len(requests), "limit", limit, "offset", offset, "service_filter", serviceName))
	return requests, nil
}

// applyServiceFilter applies service filter based on service name and type
func (r *httpRequestRepo) applyServiceFilter(query *gorm.DB, serviceName string, serviceType string) *gorm.DB {
	if serviceName == "" {
		return query
	}

	switch serviceType {
	case "backend_name":
		return query.Where("backend_name = ?", serviceName)
	case "backend_url":
		return query.Where("backend_url = ?", serviceName)
	case "host":
		return query.Where("host = ?", serviceName)
	case "auto", "":
		// Auto-detection with priority
		return query.Where("backend_name = ? OR (backend_name = '' AND backend_url = ?) OR (backend_name = '' AND backend_url = '' AND host = ?)",
			serviceName, serviceName, serviceName)
	default:
		r.logger.Warn("Unknown service type, defaulting to auto", r.logger.Args("type", serviceType))
		return query.Where("backend_name = ? OR (backend_name = '' AND backend_url = ?) OR (backend_name = '' AND backend_url = '' AND host = ?)",
			serviceName, serviceName, serviceName)
	}
}

// FindBySourceName retrieves HTTP requests for a specific log source
func (r *httpRequestRepo) FindBySourceName(sourceName string, limit int) ([]*models.HTTPRequest, error) {
	var requests []*models.HTTPRequest
	query := r.db.Where("source_name = ?", sourceName).Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&requests).Error; err != nil {
		r.logger.WithCaller().Error("Failed to find HTTP requests by source",
			r.logger.Args("source", sourceName, "error", err))
		return nil, err
	}

	r.logger.Trace("Found HTTP requests by source",
		r.logger.Args("count", len(requests), "source", sourceName))
	return requests, nil
}

// FindByTimeRange retrieves HTTP requests within a time range
func (r *httpRequestRepo) FindByTimeRange(start, end time.Time, limit int) ([]*models.HTTPRequest, error) {
	var requests []*models.HTTPRequest
	query := r.db.Where("timestamp BETWEEN ? AND ?", start, end).Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&requests).Error; err != nil {
		r.logger.WithCaller().Error("Failed to find HTTP requests by time range",
			r.logger.Args("start", start, "end", end, "error", err))
		return nil, err
	}

	r.logger.Trace("Found HTTP requests by time range",
		r.logger.Args("count", len(requests), "start", start, "end", end))
	return requests, nil
}

// Count returns the total number of HTTP requests
func (r *httpRequestRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&models.HTTPRequest{}).Count(&count).Error; err != nil {
		r.logger.WithCaller().Error("Failed to count HTTP requests", r.logger.Args("error", err))
		return 0, err
	}
	return count, nil
}

// CountBySourceName returns the number of HTTP requests for a specific source
func (r *httpRequestRepo) CountBySourceName(sourceName string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.HTTPRequest{}).
		Where("source_name = ?", sourceName).
		Count(&count).Error; err != nil {
		r.logger.WithCaller().Error("Failed to count HTTP requests by source",
			r.logger.Args("source", sourceName, "error", err))
		return 0, err
	}
	return count, nil
}

