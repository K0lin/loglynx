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
	"loglynx/internal/database/models"

	"gorm.io/gorm"
)

// IPTagRepository handles CRUD operations for IP tags
type IPTagRepository interface {
	Create(tag *models.IPTag) error
	Update(tag *models.IPTag) error
	Delete(ip string) error
	FindByIP(ip string) (*models.IPTag, error)
	FindAll() ([]*models.IPTag, error)
}

type ipTagRepository struct {
	db *gorm.DB
}

func NewIPTagRepository(db *gorm.DB) IPTagRepository {
	return &ipTagRepository{db: db}
}

func (r *ipTagRepository) Create(tag *models.IPTag) error {
	return r.db.Create(tag).Error
}

func (r *ipTagRepository) Update(tag *models.IPTag) error {
	return r.db.Save(tag).Error
}

func (r *ipTagRepository) Delete(ip string) error {
	return r.db.Where("ip_address = ?", ip).Delete(&models.IPTag{}).Error
}

func (r *ipTagRepository) FindByIP(ip string) (*models.IPTag, error) {
	var tag models.IPTag
	err := r.db.Where("ip_address = ?", ip).First(&tag).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *ipTagRepository) FindAll() ([]*models.IPTag, error) {
	var tags []*models.IPTag
	err := r.db.Find(&tags).Error
	return tags, err
}
