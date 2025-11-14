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
	// - page_size=4096 for optimal performance (default is 1024)
	// - NORMAL synchronous for balance between safety and speed
	// - cache_size=64MB (64000 KB) for better query performance
	// - busy_timeout=5000ms (5 seconds) to prevent SQLITE_BUSY errors
	// - txlock=immediate to prevent lock escalation deadlocks
	dsn := cfg.Path + "?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=64000&_page_size=4096&_busy_timeout=5000&_txlock=immediate"
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

	if cfg.AutoTuning {
		// Auto-tune based on CPU cores
		cpuCores := runtime.NumCPU()
		optimalMaxOpen := cpuCores * 3 // 3 connections per core for read-heavy workloads

		if optimalMaxOpen > maxOpenConns {
			maxOpenConns = optimalMaxOpen
			maxIdleConns = maxOpenConns * 40 / 100 // 40% idle

			if maxIdleConns < 10 {
				maxIdleConns = 10
			}

			logger.Info("🔧 Auto-tuned connection pool based on CPU cores",
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
		logger.Info("🚀 Empty database detected - deferring index creation until after first data load for optimal performance")
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

// NewReadOnlyConnection creates a read-only database connection pool for analytics queries
// This prevents long-running SELECT queries from blocking write operations
func NewReadOnlyConnection(cfg *Config, logger *pterm.Logger) (*gorm.DB, error) {
	// Read-only DSN with:
	// - mode=ro for read-only access
	// - query_only=1 to enforce read-only at SQLite level
	// - Shared cache_size for better performance
	// - No busy_timeout needed (reads don't block reads)
	dsn := cfg.Path + "?mode=ro&_query_only=1&_cache_shared=true&_cache_size=64000"

	_, err := os.Stat(cfg.Path)
	if errors.Is(err, os.ErrPermission) {
		logger.WithCaller().Fatal("Permission denied to access database file (read-only).", logger.Args("error", err))
	}

	if os.IsNotExist(err) {
		logger.WithCaller().Fatal("Database file does not exist (read-only connection).", logger.Args("path", cfg.Path))
	}

	logger.Debug("Initializing read-only database connection for analytics queries.")

	// Create slow query logger (log queries taking >100ms)
	slowQueryLogger := NewSlowQueryLogger(logger, 100*time.Millisecond)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		PrepareStmt: true,
		Logger:      slowQueryLogger,
	})

	if err != nil {
		logger.WithCaller().Fatal("Failed to connect to the database (read-only).", logger.Args("error", err))
	}

	// Get underlying SQL DB for connection pool
	sqlDB, err := db.DB()
	if err != nil {
		logger.WithCaller().Fatal("Failed to get database instance (read-only).", logger.Args("error", err))
	}

	// Configure connection pool (read-only can have more connections)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns * 2) // 2x connections for reads
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns * 2)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLife)

	logger.Info("Read-only database connection established successfully.",
		logger.Args("max_conns", cfg.MaxOpenConns*2))
	return db, nil
}
