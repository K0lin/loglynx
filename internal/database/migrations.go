package database

import (
	"loglynx/internal/database/models"

	"gorm.io/gorm"
)

func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.LogSource{},
		&models.HTTPRequest{},
		&models.IPReputation{},
	)
}