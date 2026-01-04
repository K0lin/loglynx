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
package handlers

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"loglynx/internal/database"
	"loglynx/internal/database/repositories"
	"loglynx/internal/version"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// SystemHandler handles system statistics requests
type SystemHandler struct {
	statsRepo      repositories.StatsRepository
	httpRepo       repositories.HTTPRequestRepository
	cleanupService *database.CleanupService
	logger         *pterm.Logger
	startTime      time.Time
	dbPath         string
	retentionDays  int
}

// SystemStats holds comprehensive system statistics
type SystemStats struct {
	// Process Info
	AppVersion    string  `json:"app_version"`
	Uptime        string  `json:"uptime"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	StartTime     string  `json:"start_time"`
	GoVersion     string  `json:"go_version"`
	NumCPU        int     `json:"num_cpu"`
	NumGoroutines int     `json:"num_goroutines"`
	MemoryAllocMB float64 `json:"memory_alloc_mb"`
	MemoryTotalMB float64 `json:"memory_total_mb"`
	MemorySysMB   float64 `json:"memory_sys_mb"`
	GCPauseMs     float64 `json:"gc_pause_ms"`

	// Database Info
	TotalRecords     int64   `json:"total_records"`
	RecordsToCleanup int64   `json:"records_to_cleanup"`
	DatabaseSizeMB   float64 `json:"database_size_mb"`
	DatabasePath     string  `json:"database_path"`

	// Cleanup Info
	RetentionDays        int    `json:"retention_days"`
	NextCleanupTime      string `json:"next_cleanup_time"`
	NextCleanupCountdown string `json:"next_cleanup_countdown"`
	LastCleanupTime      string `json:"last_cleanup_time"`

	// Additional Stats
	OldestRecordAge   string  `json:"oldest_record_age"`
	NewestRecordAge   string  `json:"newest_record_age"`
	RequestsPerSecond float64 `json:"requests_per_second"`
}

// NewSystemHandler creates a new system handler
func NewSystemHandler(
	statsRepo repositories.StatsRepository,
	httpRepo repositories.HTTPRequestRepository,
	cleanupService *database.CleanupService,
	logger *pterm.Logger,
	dbPath string,
	retentionDays int,
) *SystemHandler {
	return &SystemHandler{
		statsRepo:      statsRepo,
		httpRepo:       httpRepo,
		cleanupService: cleanupService,
		logger:         logger,
		startTime:      time.Now(),
		dbPath:         dbPath,
		retentionDays:  retentionDays,
	}
}

// HandleSystemStatsPage renders the system stats page
func (h *SystemHandler) HandleSystemStatsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "system.html", gin.H{
		"title": "System Stats",
	})
}

// GetSystemStats returns comprehensive system statistics
func (h *SystemHandler) GetSystemStats(c *gin.Context) {
	stats, err := h.collectSystemStats()
	if err != nil {
		h.logger.WithCaller().Error("Failed to collect system stats", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to collect system stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetRecordsTimeline returns records count timeline for system stats chart
func (h *SystemHandler) GetRecordsTimeline(c *gin.Context) {
	// Get days parameter (default 30)
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 {
			if d <= 365 {
				days = d
			} else {
				days = 365 // Cap at 1 year
			}
		}
	}

	timeline, err := h.statsRepo.GetRecordsTimeline(days)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get records timeline", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get records timeline"})
		return
	}

	c.JSON(http.StatusOK, timeline)
}

// collectSystemStats gathers all system statistics
func (h *SystemHandler) collectSystemStats() (*SystemStats, error) {
	stats := &SystemStats{
		AppVersion:    version.Version,
		StartTime:     h.startTime.Format(time.RFC3339),
		GoVersion:     runtime.Version(),
		NumCPU:        runtime.NumCPU(),
		NumGoroutines: runtime.NumGoroutine(),
		DatabasePath:  h.dbPath,
		RetentionDays: h.retentionDays,
	}

	// Calculate uptime
	uptime := time.Since(h.startTime)
	stats.UptimeSeconds = int64(uptime.Seconds())
	stats.Uptime = formatDuration(uptime)

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats.MemoryAllocMB = float64(m.Alloc) / 1024 / 1024
	stats.MemoryTotalMB = float64(m.TotalAlloc) / 1024 / 1024
	stats.MemorySysMB = float64(m.Sys) / 1024 / 1024
	stats.GCPauseMs = float64(m.PauseNs[(m.NumGC+255)%256]) / 1000000

	// Database record count
	totalRecords, err := h.httpRepo.Count()
	if err != nil {
		h.logger.WithCaller().Warn("Failed to get total records", h.logger.Args("error", err))
	}
	stats.TotalRecords = totalRecords

	// Calculate records to cleanup (if retention is enabled)
	if h.retentionDays > 0 {
		cutoffDate := time.Now().AddDate(0, 0, -h.retentionDays)
		recordsToCleanup, err := h.statsRepo.CountRecordsOlderThan(cutoffDate)
		if err != nil {
			h.logger.WithCaller().Warn("Failed to count records to cleanup", h.logger.Args("error", err))
		}
		stats.RecordsToCleanup = recordsToCleanup
	}

	// Database file size
	if fileInfo, err := os.Stat(h.dbPath); err == nil {
		stats.DatabaseSizeMB = float64(fileInfo.Size()) / 1024 / 1024
	}

	// Cleanup schedule info
	if h.cleanupService != nil && h.retentionDays > 0 {
		cleanupStats := h.cleanupService.GetStats()
		stats.NextCleanupTime = cleanupStats.NextScheduledRun.Format(time.DateTime)

		timeUntilCleanup := time.Until(cleanupStats.NextScheduledRun)
		stats.NextCleanupCountdown = formatDuration(timeUntilCleanup)

		if !cleanupStats.LastRunTime.IsZero() {
			stats.LastCleanupTime = cleanupStats.LastRunTime.Format(time.DateTime)
		} else {
			stats.LastCleanupTime = "Never"
		}
	} else {
		stats.NextCleanupTime = "Disabled"
		stats.NextCleanupCountdown = "N/A"
		stats.LastCleanupTime = "N/A"
	}

	// Oldest and newest record ages
	oldestTime, newestTime, err := h.statsRepo.GetRecordTimeRange()
	if err == nil {
		if !oldestTime.IsZero() {
			stats.OldestRecordAge = formatDuration(time.Since(oldestTime))
		} else {
			stats.OldestRecordAge = "No records"
		}

		if !newestTime.IsZero() {
			stats.NewestRecordAge = formatDuration(time.Since(newestTime))
		} else {
			stats.NewestRecordAge = "No records"
		}
	}

	// Calculate requests per second since startup
	if stats.TotalRecords > 0 && stats.UptimeSeconds > 0 {
		stats.RequestsPerSecond = float64(stats.TotalRecords) / float64(stats.UptimeSeconds)
	}

	return stats, nil
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return formatPlural(days, "day", hours, "hour")
	}
	if hours > 0 {
		return formatPlural(hours, "hour", minutes, "minute")
	}
	if minutes > 0 {
		return formatPlural(minutes, "minute", seconds, "second")
	}
	return formatPlural(seconds, "second", 0, "")
}

// formatPlural formats numbers with proper pluralization
func formatPlural(n1 int, unit1 string, n2 int, unit2 string) string {
	result := formatSingle(n1, unit1)
	if n2 > 0 && unit2 != "" {
		result += ", " + formatSingle(n2, unit2)
	}
	return result
}

// formatSingle formats a single value with pluralization
func formatSingle(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

