package repositories

import (
	"loglynx/internal/database/models"
	"time"

	"gorm.io/gorm"
)

type LogSourceRepository interface {
	Create(source *models.LogSource) error
	FindByName(name string) (*models.LogSource, error)
	FindAll() ([]*models.LogSource, error)
	Update(source *models.LogSource) error
	UpdateTracking(name string, position int64, lastLine string) error
}

type logSourceRepo struct {
	db *gorm.DB
}

func NewLogSourceRepository(db *gorm.DB) LogSourceRepository {
	return &logSourceRepo{db: db}
}

func (r *logSourceRepo) Create(source *models.LogSource) error {
	return r.db.Create(source).Error
}

func (r *logSourceRepo) FindByName(name string) (*models.LogSource, error) {
	var source models.LogSource
	err := r.db.Where("name = ?", name).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *logSourceRepo) FindAll() ([]*models.LogSource, error) {
	var sources []*models.LogSource
	err := r.db.Find(&sources).Error
	return sources, err
}

func (r *logSourceRepo) Update(source *models.LogSource) error {
	return r.db.Save(source).Error
}

func (r *logSourceRepo) UpdateTracking(name string, position int64, lastLine string) error {
	// Use Exec for better performance with direct SQL execution
	return r.db.Exec(
		"UPDATE log_sources SET last_position = ?, last_line_content = ?, last_read_at = ?, updated_at = ? WHERE name = ?",
		position, lastLine, time.Now(), time.Now(), name,
	).Error
}
