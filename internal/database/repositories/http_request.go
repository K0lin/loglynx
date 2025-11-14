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
		}
	})
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
func (r *httpRequestRepo) optimizeDatabase() error {
	indexes := []string{
		// ===== BASIC SINGLE-COLUMN INDEXES =====
		// These were removed from GORM tags to defer creation until after first load

		`CREATE INDEX IF NOT EXISTS idx_source_name ON http_requests(source_name)`,
		`CREATE INDEX IF NOT EXISTS idx_timestamp ON http_requests(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_partition_key ON http_requests(partition_key)`,
		`CREATE INDEX IF NOT EXISTS idx_client_ip ON http_requests(client_ip)`,
		`CREATE INDEX IF NOT EXISTS idx_host ON http_requests(host)`,
		`CREATE INDEX IF NOT EXISTS idx_status ON http_requests(status_code)`,
		`CREATE INDEX IF NOT EXISTS idx_response_time ON http_requests(response_time_ms) WHERE response_time_ms > 0`,
		`CREATE INDEX IF NOT EXISTS idx_retry_attempts ON http_requests(retry_attempts) WHERE retry_attempts > 0`,
		`CREATE INDEX IF NOT EXISTS idx_browser ON http_requests(browser)`,
		`CREATE INDEX IF NOT EXISTS idx_os ON http_requests(os)`,
		`CREATE INDEX IF NOT EXISTS idx_device_type ON http_requests(device_type)`,
		`CREATE INDEX IF NOT EXISTS idx_router_name ON http_requests(router_name)`,
		`CREATE INDEX IF NOT EXISTS idx_request_id ON http_requests(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trace_id ON http_requests(trace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_geo_country ON http_requests(geo_country)`,
		`CREATE INDEX IF NOT EXISTS idx_created_at ON http_requests(created_at DESC)`,

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
		`CREATE INDEX IF NOT EXISTS idx_dashboard_covering
		 ON http_requests(timestamp DESC, status_code, response_time_ms, host, client_ip, method, path)`,

		// Error analysis covering index
		`CREATE INDEX IF NOT EXISTS idx_error_analysis
		 ON http_requests(timestamp DESC, status_code, path, method, client_ip, response_time_ms, backend_name)
		 WHERE status_code >= 400`,

		// ===== CLEANUP INDEX =====
		// Index for cleanup queries (timestamp for deletion)
		`CREATE INDEX IF NOT EXISTS idx_timestamp_cleanup
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
	// OPTIMIZATION: Increased from 15 to 500 for significantly better throughput
	// 500 records * 49 columns = 24,500 variables (well under 32,766 limit)
	const MaxSQLiteVariables = 32766
	const ColumnsPerRecord = 49                                      // Actual number of columns in HTTPRequest model
	const MaxRecordsPerBatch = MaxSQLiteVariables / ColumnsPerRecord // Optimized batch size for performance

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
