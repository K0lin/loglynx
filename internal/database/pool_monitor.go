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
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// PoolStats contains detailed connection pool statistics
type PoolStats struct {
	MaxOpenConns      int           // Maximum number of open connections
	OpenConns         int           // Current number of open connections
	InUse             int           // Number of connections in use
	Idle              int           // Number of idle connections
	WaitCount         int64         // Total number of connections waited for
	WaitDuration      time.Duration // Total time waited for connections
	MaxIdleClosed     int64         // Total number of connections closed due to SetMaxIdleConns
	MaxLifetimeClosed int64         // Total number of connections closed due to SetConnMaxLifetime
	Timestamp         time.Time

	// Calculated metrics
	Utilization float64       // Percentage of connections in use (InUse / MaxOpenConns)
	IdleRatio   float64       // Percentage of idle connections (Idle / OpenConns)
	AvgWaitTime time.Duration // Average wait time per connection

	// Alert flags
	IsHighUtilization bool // True if utilization > threshold
	IsSaturated       bool // True if all connections in use
}

// PoolMonitor monitors database connection pool health
type PoolMonitor struct {
	db        *sql.DB
	logger    *pterm.Logger
	interval  time.Duration
	threshold float64
	autoTune  bool
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Stats tracking
	mu               sync.RWMutex
	currentStats     *PoolStats
	alertCount       int64
	lastAlert        time.Time
	totalAdjustments int
}

// NewPoolMonitor creates a new connection pool monitor
func NewPoolMonitor(db *sql.DB, logger *pterm.Logger, interval time.Duration, threshold float64, autoTune bool) *PoolMonitor {
	return &PoolMonitor{
		db:        db,
		logger:    logger,
		interval:  interval,
		threshold: threshold,
		autoTune:  autoTune,
	}
}

// Start begins monitoring the connection pool
func (pm *PoolMonitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	pm.cancel = cancel

	pm.wg.Add(1)
	go pm.monitorLoop(ctx)

	pm.logger.Info("Connection pool monitoring started",
		pm.logger.Args(
			"interval", pm.interval,
			"threshold", pm.threshold,
			"auto_tuning", pm.autoTune,
		))
}

// Stop stops the pool monitor
func (pm *PoolMonitor) Stop() {
	if pm.cancel != nil {
		pm.cancel()
	}
	pm.wg.Wait()
	pm.logger.Info("Connection pool monitoring stopped")
}

// GetCurrentStats returns the current pool statistics
func (pm *PoolMonitor) GetCurrentStats() *PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.currentStats == nil {
		return nil
	}

	// Return a copy to prevent race conditions
	statsCopy := *pm.currentStats
	return &statsCopy
}

// monitorLoop continuously monitors the connection pool
func (pm *PoolMonitor) monitorLoop(ctx context.Context) {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	// Initial stats collection
	pm.collectAndAnalyze()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.collectAndAnalyze()
		}
	}
}

// collectAndAnalyze collects pool stats and performs analysis
func (pm *PoolMonitor) collectAndAnalyze() {
	stats := pm.collectStats()

	pm.mu.Lock()
	pm.currentStats = stats
	pm.mu.Unlock()

	// Log stats at trace level
	pm.logger.Trace("Connection pool stats",
		pm.logger.Args(
			"max_open", stats.MaxOpenConns,
			"open", stats.OpenConns,
			"in_use", stats.InUse,
			"idle", stats.Idle,
			"utilization", fmt.Sprintf("%.1f%%", stats.Utilization*100),
			"wait_count", stats.WaitCount,
		))

	// Check for high utilization
	if stats.IsHighUtilization {
		pm.mu.Lock()
		pm.alertCount++
		pm.lastAlert = time.Now()
		pm.mu.Unlock()

		pm.logger.Warn("⚠️  Connection pool high utilization detected",
			pm.logger.Args(
				"utilization", fmt.Sprintf("%.1f%%", stats.Utilization*100),
				"in_use", stats.InUse,
				"max_open", stats.MaxOpenConns,
				"threshold", fmt.Sprintf("%.1f%%", pm.threshold*100),
				"wait_count", stats.WaitCount,
			))

		// Auto-tune if enabled
		if pm.autoTune {
			pm.performAutoTuning(stats)
		}
	}

	// Check for saturation
	if stats.IsSaturated {
		pm.logger.Error("🚨 Connection pool SATURATED - all connections in use!",
			pm.logger.Args(
				"in_use", stats.InUse,
				"max_open", stats.MaxOpenConns,
				"wait_count", stats.WaitCount,
				"avg_wait_time", stats.AvgWaitTime,
			))

		// Force auto-tune on saturation
		if pm.autoTune {
			pm.performAutoTuning(stats)
		}
	}

	// Log performance warnings
	if stats.WaitCount > 0 {
		pm.logger.Debug("Connections waiting for availability",
			pm.logger.Args(
				"wait_count", stats.WaitCount,
				"avg_wait_time", stats.AvgWaitTime,
				"total_wait_time", stats.WaitDuration,
			))
	}
}

// collectStats collects current pool statistics
func (pm *PoolMonitor) collectStats() *PoolStats {
	dbStats := pm.db.Stats()

	stats := &PoolStats{
		MaxOpenConns:      dbStats.MaxOpenConnections,
		OpenConns:         dbStats.OpenConnections,
		InUse:             dbStats.InUse,
		Idle:              dbStats.Idle,
		WaitCount:         dbStats.WaitCount,
		WaitDuration:      dbStats.WaitDuration,
		MaxIdleClosed:     dbStats.MaxIdleClosed,
		MaxLifetimeClosed: dbStats.MaxLifetimeClosed,
		Timestamp:         time.Now(),
	}

	// Calculate metrics
	if stats.MaxOpenConns > 0 {
		stats.Utilization = float64(stats.InUse) / float64(stats.MaxOpenConns)
	}

	if stats.OpenConns > 0 {
		stats.IdleRatio = float64(stats.Idle) / float64(stats.OpenConns)
	}

	if stats.WaitCount > 0 {
		stats.AvgWaitTime = stats.WaitDuration / time.Duration(stats.WaitCount)
	}

	// Set alert flags
	stats.IsHighUtilization = stats.Utilization >= pm.threshold
	stats.IsSaturated = stats.InUse >= stats.MaxOpenConns

	return stats
}

// performAutoTuning adjusts connection pool settings based on system resources
func (pm *PoolMonitor) performAutoTuning(stats *PoolStats) {
	// Only tune once every 5 minutes to avoid thrashing
	pm.mu.RLock()
	timeSinceLastAdjustment := time.Since(pm.lastAlert)
	pm.mu.RUnlock()

	if timeSinceLastAdjustment < 5*time.Minute {
		return
	}

	// Calculate optimal pool size based on CPU cores
	cpuCores := runtime.NumCPU()

	// SQLite with WAL mode:
	// - 1 writer + multiple readers
	// - Recommended: 2-3 connections per CPU core for read-heavy workloads
	optimalMaxOpen := cpuCores * 3
	if optimalMaxOpen < 25 {
		optimalMaxOpen = 25 // Minimum production setting
	}
	if optimalMaxOpen > 100 {
		optimalMaxOpen = 100 // Cap at 100 to prevent excessive overhead
	}

	// Adjust idle connections to 40% of max open
	optimalMaxIdle := optimalMaxOpen * 40 / 100
	if optimalMaxIdle < 10 {
		optimalMaxIdle = 10
	}

	currentMaxOpen := stats.MaxOpenConns

	// Only increase if current utilization is high
	if stats.Utilization >= pm.threshold && optimalMaxOpen > currentMaxOpen {
		pm.logger.Info("🔧 Auto-tuning connection pool (increasing capacity)",
			pm.logger.Args(
				"current_max_open", currentMaxOpen,
				"new_max_open", optimalMaxOpen,
				"current_max_idle", "auto",
				"new_max_idle", optimalMaxIdle,
				"cpu_cores", cpuCores,
				"utilization", fmt.Sprintf("%.1f%%", stats.Utilization*100),
			))

		pm.db.SetMaxOpenConns(optimalMaxOpen)
		pm.db.SetMaxIdleConns(optimalMaxIdle)

		pm.mu.Lock()
		pm.totalAdjustments++
		pm.mu.Unlock()
	} else if stats.IdleRatio > 0.7 && optimalMaxOpen < currentMaxOpen {
		// Decrease if too many idle connections (optimization)
		pm.logger.Info("🔧 Auto-tuning connection pool (optimizing idle)",
			pm.logger.Args(
				"current_max_open", currentMaxOpen,
				"new_max_open", optimalMaxOpen,
				"idle_ratio", fmt.Sprintf("%.1f%%", stats.IdleRatio*100),
			))

		pm.db.SetMaxOpenConns(optimalMaxOpen)
		pm.db.SetMaxIdleConns(optimalMaxIdle)

		pm.mu.Lock()
		pm.totalAdjustments++
		pm.mu.Unlock()
	}
}

// GetAlertCount returns the total number of high utilization alerts
func (pm *PoolMonitor) GetAlertCount() int64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.alertCount
}

// GetTotalAdjustments returns the total number of auto-tuning adjustments
func (pm *PoolMonitor) GetTotalAdjustments() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.totalAdjustments
}

// PrintSummary prints a human-readable summary of pool statistics
func (pm *PoolMonitor) PrintSummary() {
	stats := pm.GetCurrentStats()
	if stats == nil {
		pm.logger.Info("No pool statistics available yet")
		return
	}

	pm.mu.RLock()
	alerts := pm.alertCount
	adjustments := pm.totalAdjustments
	pm.mu.RUnlock()

	pm.logger.Info("📊 Connection Pool Summary",
		pm.logger.Args(
			"max_open_conns", stats.MaxOpenConns,
			"current_open", stats.OpenConns,
			"in_use", stats.InUse,
			"idle", stats.Idle,
			"utilization", fmt.Sprintf("%.1f%%", stats.Utilization*100),
			"total_waits", stats.WaitCount,
			"avg_wait_time", stats.AvgWaitTime,
			"total_alerts", alerts,
			"total_adjustments", adjustments,
		))
}

