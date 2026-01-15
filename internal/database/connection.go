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
package database

import (
	"context"
	"errors"
	"loglynx/internal/database/repositories"
	"loglynx/internal/discovery"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Path         string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration

	// Pool Monitoring
	PoolMonitoringEnabled   bool
	PoolMonitoringInterval  time.Duration
	PoolSaturationThreshold float64
	AutoTuning              bool
}

// SlowQueryLogger logs slow database queries for performance monitoring
type SlowQueryLogger struct {
	logger            *pterm.Logger
	slowThreshold     time.Duration
	logLevel          logger.LogLevel
	ignoreNotFoundErr bool
}

func NewSlowQueryLogger(ptermLogger *pterm.Logger, slowThreshold time.Duration) *SlowQueryLogger {
	return &SlowQueryLogger{
		logger:            ptermLogger,
		slowThreshold:     slowThreshold,
		logLevel:          logger.Warn,
		ignoreNotFoundErr: true,
	}
}

func (l *SlowQueryLogger) LogMode(level logger.LogLevel) logger.Interface {
	l.logLevel = level
	return l
}

func (l *SlowQueryLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Info {
		l.logger.Info(msg, l.logger.Args("data", data))
	}
}

func (l *SlowQueryLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Warn {
		l.logger.Warn(msg, l.logger.Args("data", data))
	}
}

func (l *SlowQueryLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Error {
		l.logger.Error(msg, l.logger.Args("data", data))
	}
}

func (l *SlowQueryLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	// Log slow queries (debug level to avoid console noise in normal runs)
	if elapsed >= l.slowThreshold {
		l.logger.Debug("SLOW QUERY DETECTED",
			l.logger.Args(
				"duration_ms", elapsed.Milliseconds(),
				"rows", rows,
				"sql", sql,
			))
	} else if l.logLevel >= logger.Info {
		// Trace all queries in debug mode
		l.logger.Trace("Database query",
			l.logger.Args(
				"duration_ms", elapsed.Milliseconds(),
				"rows", rows,
				"sql", sql,
			))
	}

	// Log errors (but ignore UNIQUE constraint violations - they're handled by the application)
	if err != nil && (!l.ignoreNotFoundErr || !errors.Is(err, gorm.ErrRecordNotFound)) {
		// Ignore UNIQUE constraint errors - these are expected during deduplication
		// The application handles them gracefully in the repository layer
		errStr := err.Error()
		if strings.Contains(errStr, "UNIQUE constraint failed") || strings.Contains(errStr, "request_hash") {
			// This is a duplicate - silently skip logging (summary is logged in repository)
			return
		}

		l.logger.Error("Database query error",
			l.logger.Args(
				"error", err,
				"duration_ms", elapsed.Milliseconds(),
				"sql", sql,
			))
	}
}

func NewConnection(cfg *Config, logger *pterm.Logger) (*gorm.DB, error) {
	// Optimized DSN with:
	// - WAL mode for concurrent reads/writes
	// - NORMAL synchronous for balance between safety and speed
	// - cache_size=-64000 (negative means KB, 64MB) for better query performance
	// - busy_timeout=5000ms (5 seconds) to prevent SQLITE_BUSY errors
	// Note: mattn/go-sqlite3 uses different parameter names than glebarez
	dsn := cfg.Path + "?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-64000&_busy_timeout=5000"
	_, err := os.Stat(cfg.Path)

	if errors.Is(err, os.ErrPermission) {
		logger.WithCaller().Fatal("Permission denied to access database file.", logger.Args("error", err))
		// Fatal() terminates the program, so no code after this will execute
	}

	logger.Debug("Permission to access database file granted.", logger.Args("path", cfg.Path))
	logger.Debug("Initialization of the database with optimized settings (WAL mode, page_size=4096).")

	// Create slow query logger (log queries taking >100ms)
	slowQueryLogger := NewSlowQueryLogger(logger, 100*time.Millisecond)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		PrepareStmt: true,
		Logger:      slowQueryLogger,
	})

	if err != nil {
		logger.WithCaller().Fatal("Failed to connect to the database.", logger.Args("error", err))
		// Fatal() terminates the program, so no code after this will execute
	}

	// Get underlying SQL DB for connection pool
	sqlDB, err := db.DB()
	if err != nil {
		logger.WithCaller().Fatal("Failed to get database instance.", logger.Args("error", err))
		// Fatal() terminates the program, so no code after this will execute
	}

	// Configure connection pool with auto-tuning if enabled
	maxOpenConns := cfg.MaxOpenConns
	maxIdleConns := cfg.MaxIdleConns

	// Ensure sensible defaults if config is unset or very low
	if maxOpenConns <= 0 {
		maxOpenConns = 25
	}
	if maxIdleConns <= 0 {
		maxIdleConns = 10
	}

	if cfg.AutoTuning {
		// Auto-tune based on CPU cores for read-heavy analytics
		cpuCores := runtime.NumCPU()
		optimalMaxOpen := cpuCores * 5 // allow higher concurrency for aggregation workloads

		if optimalMaxOpen > maxOpenConns {
			maxOpenConns = optimalMaxOpen
			maxIdleConns = maxOpenConns * 40 / 100 // 40% idle

			if maxIdleConns < 10 {
				maxIdleConns = 10
			}

			logger.Info("Auto-tuned connection pool based on CPU cores",
				logger.Args(
					"cpu_cores", cpuCores,
					"max_open_conns", maxOpenConns,
					"max_idle_conns", maxIdleConns,
				))
		}
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLife)

	logger.Debug("Connection pool configured",
		logger.Args(
			"max_open_conns", maxOpenConns,
			"max_idle_conns", maxIdleConns,
			"conn_max_life", cfg.ConnMaxLife,
		))

	// Run migrations
	logger.Trace("Running database migrations.")
	if err := RunMigrations(db); err != nil {
		logger.WithCaller().Fatal("Failed to run database migrations.", logger.Args("error", err))
		// Fatal() terminates the program, so no code after this will execute
	}

	// Check if database is empty (first load)
	// We need to import models for this check
	var count int64
	db.Table("http_requests").Count(&count)
	isDatabaseEmpty := (count == 0)

	if isDatabaseEmpty {
		logger.Info("Empty database detected - deferring index creation until after first data load for optimal performance")
		logger.Info("   Indexes will be created automatically when initial data load completes")
	} else {
		// Database has data - create/verify indexes now
		logger.Debug("Existing data found - verifying database indexes")
		if err := OptimizeDatabase(db, logger); err != nil {
			logger.Warn("Database optimization had warnings", logger.Args("error", err))
			// Don't fail on optimization errors, just warn
		}
	}

	// Run discovery engine in background to speed up startup
	go func() {
		logger.Debug("Running log source discovery in background...")
		engine := discovery.NewEngine(repositories.NewLogSourceRepository(db), logger)
		if err := engine.Run(logger); err != nil {
			logger.Warn("Failed to run discovery engine", logger.Args("error", err))
			return
		}

		logSourceRepo, err := repositories.NewLogSourceRepository(db).FindAll()
		if err != nil {
			logger.Warn("Failed to retrieve log sources", logger.Args("error", err))
			return
		}

		logger.Info("Discovered log sources", logger.Args("count", len(logSourceRepo)))
	}()

	// Start pool monitoring if enabled
	if cfg.PoolMonitoringEnabled {
		monitor := NewPoolMonitor(
			sqlDB,
			logger,
			cfg.PoolMonitoringInterval,
			cfg.PoolSaturationThreshold,
			cfg.AutoTuning,
		)
		monitor.Start(context.Background())

		// Log initial stats after a short delay
		go func() {
			time.Sleep(2 * time.Second)
			monitor.PrintSummary()
		}()
	}

	logger.Info("Database connection established successfully.")
	return db, nil
}
