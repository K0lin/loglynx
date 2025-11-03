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
	FindAll(limit int, offset int, host string) ([]*models.HTTPRequest, error)
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
func (r *httpRequestRepo) CreateBatch(requests []*models.HTTPRequest) error {
	if len(requests) == 0 {
		r.logger.Debug("Empty batch, skipping insert")
		return nil
	}

	// Start transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		r.logger.WithCaller().Error("Failed to begin transaction", r.logger.Args("error", tx.Error))
		return tx.Error
	}

	// Insert batch
	if err := tx.Create(&requests).Error; err != nil {
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

	r.logger.Debug("Successfully inserted batch",
		r.logger.Args("count", len(requests), "source", requests[0].SourceName))

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
func (r *httpRequestRepo) FindAll(limit int, offset int, host string) ([]*models.HTTPRequest, error) {
	var requests []*models.HTTPRequest
	query := r.db.Order("timestamp DESC")

	// Apply host filter if provided (filter by backend_name with LIKE pattern)
	if host != "" {
		// Convert spaces back to dashes for pattern matching
		pattern := strings.ReplaceAll(host, " ", "-")
		query = query.Where("backend_name LIKE ?", "%-"+pattern+"-%")
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

	r.logger.Trace("Found HTTP requests", r.logger.Args("count", len(requests), "limit", limit, "offset", offset, "host", host))
	return requests, nil
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
