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
	"time"

	"gorm.io/gorm"
)

type LogSourceRepository interface {
	Create(source *models.LogSource) error
	FindByName(name string) (*models.LogSource, error)
	FindAll() ([]*models.LogSource, error)
	Update(source *models.LogSource) error
	UpdateTracking(name string, position int64, inode int64, lastLine string) error
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

func (r *logSourceRepo) UpdateTracking(name string, position int64, inode int64, lastLine string) error {
	// Use Exec for better performance with direct SQL execution
	return r.db.Exec(
		"UPDATE log_sources SET last_position = ?, last_inode = ?, last_line_content = ?, last_read_at = ?, updated_at = ? WHERE name = ?",
		position, inode, lastLine, time.Now(), time.Now(), name,
	).Error
}

