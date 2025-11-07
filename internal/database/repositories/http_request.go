package repositories

import (
	"loglynx/internal/database/models"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
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
}

type httpRequestRepo struct {
	db     *gorm.DB
	logger *pterm.Logger
}

// NewHTTPRequestRepository creates a new HTTP request repository
func NewHTTPRequestRepository(db *gorm.DB, logger *pterm.Logger) HTTPRequestRepository {
	return &httpRequestRepo{
		db:     db,
		logger: logger,
	}
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

	// Check if database is empty (first load optimization)
	// IMPORTANT: Check ONCE at the beginning, not per sub-batch
	// During first load of large files, subsequent sub-batches would incorrectly see records
	var count int64
	r.db.Model(&models.HTTPRequest{}).Count(&count)
	isFirstLoad := (count == 0)

	if isFirstLoad {
		r.logger.Debug("First load detected, skipping deduplication checks for all batches",
			r.logger.Args("total_records", len(requests)))
	}

	// SQLite has a variable limit (default 32766 for older versions, 999 in some configs)
	// HTTPRequest has 48 columns (as of schema optimization update), so max safe batch size is ~680 records
	const MaxSQLiteVariables = 32766
	const ColumnsPerRecord = 48 // Actual number of columns in HTTPRequest model
	const MaxRecordsPerBatch = MaxSQLiteVariables / ColumnsPerRecord // ~682 records

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
	// Start transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		r.logger.WithCaller().Error("Failed to begin transaction", r.logger.Args("error", tx.Error))
		return tx.Error
	}

	// Insert batch with duplicate handling
	// If we hit a unique constraint error on request_hash, insert records one by one
	// to identify and skip only the duplicates
	// OPTIMIZATION: Skip duplicate handling on first load (database is empty)
	err := tx.Create(&requests).Error
	if err != nil && !isFirstLoad && (strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "request_hash")) {
		// Batch insert failed due to duplicates
		// Rollback the failed transaction and start a new one
		tx.Rollback()

		r.logger.Debug("Batch insert hit duplicates, inserting individually",
			r.logger.Args("count", len(requests)))

		// Start a new transaction for individual inserts
		tx = r.db.Begin()
		if tx.Error != nil {
			r.logger.WithCaller().Error("Failed to begin new transaction", r.logger.Args("error", tx.Error))
			return tx.Error
		}

		inserted := 0
		duplicates := 0
		for _, req := range requests {
			if insertErr := tx.Create(req).Error; insertErr != nil {
				if strings.Contains(insertErr.Error(), "UNIQUE constraint failed") || strings.Contains(insertErr.Error(), "request_hash") {
					duplicates++
					// Skip this duplicate - don't log as error
					continue
				}
				// Other error - rollback everything
				tx.Rollback()
				r.logger.WithCaller().Error("Failed to insert request",
					r.logger.Args("error", insertErr))
				return insertErr
			}
			inserted++
		}

		if duplicates > 0 {
			r.logger.Debug("Skipped duplicate entries",
				r.logger.Args("total", len(requests), "inserted", inserted, "duplicates", duplicates))
		}
	} else if err != nil {
		// Some other error
		tx.Rollback()
		r.logger.WithCaller().Error("Failed to insert batch",
			r.logger.Args("count", len(requests), "error", err))
		return err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		r.logger.WithCaller().Error("Failed to commit transaction", r.logger.Args("error", err))
		return err
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
