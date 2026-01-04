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
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

// CoordinatorController interface for controlling ingestion during maintenance
type CoordinatorController interface {
	Stop()
	Start() error
	GetProcessorCount() int
}

// CleanupService manages database cleanup and retention
type CleanupService struct {
	db              *gorm.DB
	logger          *pterm.Logger
	retentionDays   int
	cleanupInterval time.Duration
	cleanupTime     string
	vacuumEnabled   bool
	coordinator     CoordinatorController
	stopChan        chan struct{}
	running         bool
	// Stats tracking
	lastRunTime     time.Time
	recordsDeleted  int64
	cleanupDuration time.Duration
}

// CleanupStats holds statistics about cleanup operations
type CleanupStats struct {
	LastRunTime      time.Time
	RecordsDeleted   int64
	SpaceFreed       int64
	VacuumDuration   time.Duration
	CleanupDuration  time.Duration
	NextScheduledRun time.Time
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(db *gorm.DB, logger *pterm.Logger, retentionDays int, cleanupInterval time.Duration, cleanupTime string, vacuumEnabled bool, coordinator CoordinatorController) *CleanupService {
	return &CleanupService{
		db:              db,
		logger:          logger,
		retentionDays:   retentionDays,
		cleanupInterval: cleanupInterval,
		cleanupTime:     cleanupTime,
		vacuumEnabled:   vacuumEnabled,
		coordinator:     coordinator,
		stopChan:        make(chan struct{}),
		running:         false,
	}
}

// Start begins the cleanup service
func (s *CleanupService) Start() {
	if s.retentionDays <= 0 {
		s.logger.Info("Data retention disabled (DB_RETENTION_DAYS=0), cleanup service not started")
		return
	}

	s.running = true
	s.logger.Info("Starting database cleanup service",
		s.logger.Args(
			"retention_days", s.retentionDays,
			"cleanup_time", s.cleanupTime,
			"vacuum_enabled", s.vacuumEnabled,
		))

	go s.scheduledCleanupLoop()
}

// Stop stops the cleanup service
func (s *CleanupService) Stop() {
	if !s.running {
		return
	}

	s.logger.Info("Stopping database cleanup service")
	close(s.stopChan)
	s.running = false
}

// scheduledCleanupLoop runs cleanup at scheduled time daily
func (s *CleanupService) scheduledCleanupLoop() {
	// Run initial cleanup check after 1 minute
	time.Sleep(1 * time.Minute)

	for {
		select {
		case <-s.stopChan:
			return
		default:
			// Check if it's time to run cleanup
			now := time.Now()
			targetTime := s.parseCleanupTime(now)

			// If target time has passed today, schedule for tomorrow
			if now.After(targetTime) {
				targetTime = targetTime.Add(24 * time.Hour)
			}

			waitDuration := time.Until(targetTime)
			s.logger.Debug("Next cleanup scheduled",
				s.logger.Args("next_run", targetTime.Format(time.DateTime), "wait_duration", waitDuration.Round(time.Minute)))

			// Wait until target time or check interval
			select {
			case <-s.stopChan:
				return
			case <-time.After(min(waitDuration, s.cleanupInterval)):
				// Check if we're at target time
				if time.Now().After(targetTime.Add(-1 * time.Minute)) {
					s.runCleanup()
				}
			}
		}
	}
}

// parseCleanupTime parses the cleanup time string (HH:MM) and returns today's time
func (s *CleanupService) parseCleanupTime(baseTime time.Time) time.Time {
	// Parse HH:MM format
	cleanupTime, err := time.Parse("15:04", s.cleanupTime)
	if err != nil {
		s.logger.Warn("Invalid cleanup time format, using 02:00",
			s.logger.Args("configured", s.cleanupTime, "error", err))
		cleanupTime, _ = time.Parse("15:04", "02:00")
	}

	// Combine with today's date
	return time.Date(
		baseTime.Year(), baseTime.Month(), baseTime.Day(),
		cleanupTime.Hour(), cleanupTime.Minute(), 0, 0,
		baseTime.Location(),
	)
}

// runCleanup performs the cleanup operation
func (s *CleanupService) runCleanup() {
	s.logger.Info("Starting scheduled database cleanup",
		s.logger.Args("retention_days", s.retentionDays))

	startTime := time.Now()

	// Calculate cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -s.retentionDays)

	// Delete old records in batches to avoid long locks
	totalDeleted, err := s.deleteOldRecords(cutoffDate)
	if err != nil {
		s.logger.WithCaller().Error("Failed to delete old records",
			s.logger.Args("error", err, "cutoff_date", cutoffDate.Format("2006-01-02")))
		return
	}

	cleanupDuration := time.Since(startTime)

	// Update stats
	s.lastRunTime = startTime
	s.recordsDeleted = totalDeleted
	s.cleanupDuration = cleanupDuration

	s.logger.Info("Cleanup completed",
		s.logger.Args(
			"records_deleted", totalDeleted,
			"duration", cleanupDuration.Round(time.Second),
			"cutoff_date", cutoffDate.Format("2006-01-02"),
		))

	// Run VACUUM if enabled and significant space was freed
	if s.vacuumEnabled && totalDeleted > 0 {
		s.runVacuum()
	}
}

// deleteOldRecords deletes records older than cutoff date in batches
func (s *CleanupService) deleteOldRecords(cutoffDate time.Time) (int64, error) {
	const batchSize = 1000
	totalDeleted := int64(0)

	s.logger.Debug("Deleting records in batches",
		s.logger.Args("batch_size", batchSize, "cutoff_date", cutoffDate.Format("2006-01-02")))

	for {
		// Delete in batches using subquery to avoid full table scan
		result := s.db.Exec(`
			DELETE FROM http_requests
			WHERE id IN (
				SELECT id FROM http_requests
				WHERE timestamp < ?
				LIMIT ?
			)
		`, cutoffDate, batchSize)

		if result.Error != nil {
			return totalDeleted, result.Error
		}

		deleted := result.RowsAffected
		totalDeleted += deleted

		if deleted == 0 {
			break // No more records to delete
		}

		s.logger.Trace("Deleted batch",
			s.logger.Args("batch_deleted", deleted, "total_deleted", totalDeleted))

		// Small pause between batches to avoid hogging the database
		time.Sleep(100 * time.Millisecond)
	}

	return totalDeleted, nil
}

// runVacuum runs VACUUM to reclaim space
// pauses ingestion to prevent "database locked" errors
func (s *CleanupService) runVacuum() {
	s.logger.Info("Starting VACUUM maintenance window")

	startTime := time.Now()

	// Phase 1: Stop ingestion coordinator to prevent query conflicts
	if s.coordinator != nil {
		processorCount := s.coordinator.GetProcessorCount()
		if processorCount > 0 {
			s.logger.Info("Pausing ingestion for maintenance",
				s.logger.Args("active_processors", processorCount))
			s.coordinator.Stop()

			// Give processors time to finish current operations
			time.Sleep(2 * time.Second)
		}
	}

	// Phase 2: Run VACUUM with exclusive database access
	s.logger.Info("Running VACUUM to reclaim disk space (maintenance window active)")

	vacuumStart := time.Now()

	// Create context with timeout (max 10 minutes for VACUUM)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Run VACUUM - no "database locked" errors because no active queries
	if err := s.db.WithContext(ctx).Exec("VACUUM").Error; err != nil {
		s.logger.WithCaller().Error("Failed to run VACUUM",
			s.logger.Args("error", err))

		// Restart coordinator even if VACUUM failed
		if s.coordinator != nil {
			s.logger.Info("Restarting ingestion after VACUUM failure")
			if err := s.coordinator.Start(); err != nil {
				s.logger.WithCaller().Error("Failed to restart coordinator after VACUUM failure",
					s.logger.Args("error", err))
			}
		}
		return
	}

	vacuumDuration := time.Since(vacuumStart)

	// Phase 3: Restart ingestion coordinator
	if s.coordinator != nil {
		s.logger.Info("Restarting ingestion after VACUUM")
		if err := s.coordinator.Start(); err != nil {
			s.logger.WithCaller().Error("Failed to restart coordinator",
				s.logger.Args("error", err))
			// Critical error - coordinator should always restart
			return
		}

		// Verify processors restarted successfully
		processorCount := s.coordinator.GetProcessorCount()
		s.logger.Info("Ingestion resumed",
			s.logger.Args("active_processors", processorCount))
	}

	totalDuration := time.Since(startTime)

	s.logger.Info("VACUUM maintenance completed",
		s.logger.Args(
			"vacuum_duration", vacuumDuration.Round(time.Second),
			"total_duration", totalDuration.Round(time.Second),
		))
}

// GetStats returns cleanup statistics
func (s *CleanupService) GetStats() *CleanupStats {
	// Calculate next scheduled run
	now := time.Now()
	targetTime := s.parseCleanupTime(now)

	// If target time has passed today, schedule for tomorrow
	if now.After(targetTime) {
		targetTime = targetTime.Add(24 * time.Hour)
	}

	return &CleanupStats{
		LastRunTime:      s.lastRunTime,
		RecordsDeleted:   s.recordsDeleted,
		CleanupDuration:  s.cleanupDuration,
		NextScheduledRun: targetTime,
	}
}

// ManualCleanup triggers cleanup immediately (useful for testing/admin)
func (s *CleanupService) ManualCleanup() error {
	if s.retentionDays <= 0 {
		return fmt.Errorf("retention disabled (DB_RETENTION_DAYS=0)")
	}

	s.logger.Info("Manual cleanup triggered")
	go s.runCleanup()
	return nil
}

// min returns the minimum of two durations
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

