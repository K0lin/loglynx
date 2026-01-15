// MIT License
//
// # Copyright (c) 2026 Kolin
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
package repositories

import (
    "loglynx/internal/database/indexes"
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
	// Index creation status
	IsIndexCreationActive() bool
	// Set processor pauser for coordinated pause during index creation
	SetProcessorPauser(pauser ProcessorPauser)
}

// ProcessorPauser allows pausing/resuming processors during index creation
type ProcessorPauser interface {
	PauseAll()
	ResumeAll()
}

type httpRequestRepo struct {
	db                  *gorm.DB
	logger              *pterm.Logger
	processorPauser     ProcessorPauser // Optional: pauses processors during index creation
	isFirstLoad         bool       // Global flag: true when database is empty at startup
	firstLoadMu         sync.Mutex // Protects isFirstLoad flag
	firstLoadOnce       sync.Once  // Ensures first-load check happens only once
	indexCreationActive bool       // True while indexes are being created
	indexCreationMu     sync.RWMutex // Protects indexCreationActive flag
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

// SetProcessorPauser sets the processor pauser for coordinated pausing during index creation
func (r *httpRequestRepo) SetProcessorPauser(pauser ProcessorPauser) {
	r.processorPauser = pauser
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
            // For existing databases, reconcile indexes before processing
            r.reconcileIndexesBackground()
        }
    })
}

func (r *httpRequestRepo) reconcileIndexesBackground() {
    r.logger.Debug("Reconciling database indexes in background for existing database")
    go func() {
        created, dropped, err := indexes.Ensure(r.db, r.logger)
        if err != nil {
            r.logger.Error("Failed to reconcile database indexes", r.logger.Args("error", err))
            return
        }
        r.logger.Info("Database indexes reconciled",
            r.logger.Args("created", created, "dropped", dropped))
    }()
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

// IsIndexCreationActive returns true if indexes are currently being created
func (r *httpRequestRepo) IsIndexCreationActive() bool {
	r.indexCreationMu.RLock()
	defer r.indexCreationMu.RUnlock()
	return r.indexCreationActive
}

// createDeferredIndexes creates performance indexes after initial data load
func (r *httpRequestRepo) createDeferredIndexes() {
    // Pause all processors to prevent data loss during index creation
    if r.processorPauser != nil {
        r.logger.Info("Pausing log processors for safe index creation...")
        r.processorPauser.PauseAll()

        // Ensure we resume processors when done
        defer func() {
            r.logger.Info("Resuming log processors...")
            r.processorPauser.ResumeAll()
        }()
    }

    // Mark index creation as active
    r.indexCreationMu.Lock()
    r.indexCreationActive = true
    r.indexCreationMu.Unlock()

    // Ensure we mark as complete when done
    defer func() {
        r.indexCreationMu.Lock()
        r.indexCreationActive = false
        r.indexCreationMu.Unlock()
    }()

    r.logger.Info("Creating performance indexes in background (this may take a few minutes)...")

    startTime := time.Now()

    created, dropped, err := indexes.Ensure(r.db, r.logger)
    if err != nil {
        r.logger.Error("Failed to create performance indexes",
            r.logger.Args("error", err, "elapsed", time.Since(startTime)))
        return
    }

    elapsed := time.Since(startTime)
    r.logger.Info("Performance indexes created successfully",
        r.logger.Args("elapsed_seconds", elapsed.Seconds(), "created", created, "dropped", dropped))
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

