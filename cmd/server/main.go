package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"loglynx/internal/api"
	"loglynx/internal/api/handlers"
	"loglynx/internal/banner"
	"loglynx/internal/config"
	"loglynx/internal/database"
	"loglynx/internal/database/repositories"
	"loglynx/internal/discovery"
	"loglynx/internal/enrichment"
	"loglynx/internal/ingestion"
	parsers "loglynx/internal/parser"
	"loglynx/internal/realtime"

	"strings"

	"github.com/pterm/pterm"
)

func main() {
	// Configure Go runtime to use all available CPU cores
	// This enables goroutines to run in parallel on all cores
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Initialize logger with INFO level for production as a sensible default
	// We'll reconfigure the level after loading the configuration (LOG_LEVEL)
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelInfo)

	// Print banner
	banner.Print()

	logger.Info("Initializing LogLynx - Fast Log Analytics...",
		logger.Args("cpu_cores", runtime.NumCPU(), "gomaxprocs", runtime.GOMAXPROCS(0)))

	// Load configuration from .env file and environment variables
	cfg, err := config.Load()
	if err != nil {
		logger.WithCaller().Fatal("Failed to load configuration", logger.Args("error", err))
	}

	// Apply configured log level from environment variable LOG_LEVEL (default: info)
	// Supported values: trace, debug, info, warn, error, fatal
	lvl := strings.ToLower(cfg.LogLevel)
	var ptermLevel pterm.LogLevel
	switch lvl {
	case "trace":
		ptermLevel = pterm.LogLevelTrace
	case "debug":
		ptermLevel = pterm.LogLevelDebug
	case "info":
		ptermLevel = pterm.LogLevelInfo
	case "warn", "warning":
		ptermLevel = pterm.LogLevelWarn
	case "error":
		ptermLevel = pterm.LogLevelError
	case "fatal":
		ptermLevel = pterm.LogLevelFatal
	default:
		ptermLevel = pterm.LogLevelInfo
	}
	logger = pterm.DefaultLogger.WithLevel(ptermLevel)
	logger.Debug("Log level set", logger.Args("level", lvl))

	logger.Debug("Configuration loaded",
		logger.Args(
			"db_path", cfg.Database.Path,
			"server_port", cfg.Server.Port,
			"geoip_enabled", cfg.GeoIP.Enabled,
		))

	// Initialize database connection with configured settings
	db, err := database.NewConnection(&database.Config{
		Path:         cfg.Database.Path,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
		ConnMaxLife:  cfg.Database.ConnMaxLife,

		// Pool Monitoring
		PoolMonitoringEnabled:   cfg.Database.PoolMonitoringEnabled,
		PoolMonitoringInterval:  cfg.Database.PoolMonitoringInterval,
		PoolSaturationThreshold: cfg.Database.PoolSaturationThreshold,
		AutoTuning:              cfg.Database.AutoTuning,
	}, logger)
	if err != nil {
		logger.WithCaller().Fatal("Failed to connect to database", logger.Args("error", err))
	}

	// Initialize repositories
	logger.Debug("Initializing repositories...")
	sourceRepo := repositories.NewLogSourceRepository(db)
	httpRepo := repositories.NewHTTPRequestRepository(db, logger)
	statsRepo := repositories.NewStatsRepository(db, logger)

	// Initialize GeoIP enricher (optional - will work without GeoIP databases)
	var geoIP *enrichment.GeoIPEnricher
	if cfg.GeoIP.Enabled {
		logger.Debug("Initializing GeoIP enricher...")
		geoIP, err = enrichment.NewGeoIPEnricher(
			cfg.GeoIP.CityDBPath,
			cfg.GeoIP.CountryDBPath,
			cfg.GeoIP.ASNDBPath,
			db,
			logger,
			cfg.Performance.GeoIPCacheSize, // Pass configured cache size
		)
		if err != nil {
			logger.Warn("GeoIP enricher initialization failed, continuing without GeoIP", logger.Args("error", err))
		} else if geoIP.IsEnabled() {
			logger.Info("GeoIP enrichment enabled successfully")
			// Load cache from database in background (non-blocking)
			go func() {
				logger.Debug("Loading GeoIP cache in background...")
				if err := geoIP.LoadCache(); err != nil {
					logger.Warn("Failed to load GeoIP cache", logger.Args("error", err))
				} else {
					logger.Info("GeoIP cache loaded", logger.Args("entries", geoIP.GetCacheSize()))
				}
			}()
		}
	} else {
		logger.Info("GeoIP enrichment disabled by configuration")
	}

	// Initialize parser registry
	logger.Debug("Initializing parser registry...")
	parserRegistry := parsers.NewRegistry(logger)

	// Run initial discovery SYNCHRONOUSLY to ensure log sources are found before starting ingestion
	logger.Info("Discovering log sources...")
	discoveryEngine := discovery.NewEngine(sourceRepo, logger)
	if err := discoveryEngine.Run(logger); err != nil {
		logger.Warn("Initial discovery failed", logger.Args("error", err))
	} else {
		logger.Info("Log source discovery completed")
	}

	// Run periodic discovery in background for late-arriving files
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
		defer ticker.Stop()

		for range ticker.C {
			logger.Debug("Running periodic log source discovery...")
			if err := discoveryEngine.Run(logger); err != nil {
				logger.Warn("Periodic discovery failed", logger.Args("error", err))
			} else {
				// Get updated source count
				sources, err := sourceRepo.FindAll()
				if err == nil {
					logger.Debug("Periodic discovery completed", logger.Args("sources", len(sources)))
				}
			}
		}
	}()

	// Initialize database cleanup service
	logger.Debug("Initializing database cleanup service...")
	cleanupService := database.NewCleanupService(
		db,
		logger,
		cfg.Database.RetentionDays,
		cfg.Database.CleanupInterval,
		cfg.Database.CleanupTime,
		cfg.Database.VacuumEnabled,
	)
	cleanupService.Start()

	// Initialize ingestion coordinator with initial import limiting and performance config
	logger.Debug("Initializing ingestion coordinator...")
	coordinator := ingestion.NewCoordinator(
		sourceRepo,
		httpRepo,
		parserRegistry,
		geoIP,
		logger,
		cfg.LogSources.InitialImportDays,
		cfg.LogSources.InitialImportEnable,
		cfg.Performance.BatchSize,
		cfg.Performance.WorkerPoolSize,
	)

	// Start ingestion engine
	logger.Info("Starting ingestion engine...")
	if err := coordinator.Start(); err != nil {
		logger.WithCaller().Fatal("Failed to start ingestion coordinator", logger.Args("error", err))
	}

	logger.Info("Ingestion engine started",
		logger.Args("processors", coordinator.GetProcessorCount()))

	// Give the ingestion engine a moment to start processing before accepting web requests
	// This improves initial user experience by ensuring some data is available
	if coordinator.GetProcessorCount() > 0 {
		logger.Info("Waiting for initial log processing to begin...")
		// Wait up to 3 seconds for processing to start
		time.Sleep(3 * time.Second)
	}

	// Initialize real-time metrics collector with configured interval
	logger.Info("Initializing real-time metrics collector...")
	metricsCollector := realtime.NewMetricsCollector(db, logger)
	metricsCollector.Start(cfg.Performance.RealtimeMetricsInterval)

	// Initialize web server with configured settings
	logger.Info("Initializing web server...")
	dashboardHandler := handlers.NewDashboardHandler(statsRepo, httpRepo, logger)
	realtimeHandler := handlers.NewRealtimeHandler(metricsCollector, logger)
	systemHandler := handlers.NewSystemHandler(
		statsRepo,
		httpRepo,
		cleanupService,
		logger,
		cfg.Database.Path,
		cfg.Database.RetentionDays,
	)
	profilingHandler := handlers.NewProfilingHandler(&handlers.ProfilingConfig{
		Enabled:           cfg.Performance.ProfilingEnabled,
		MaxProfileDuration: cfg.Performance.MaxProfileDuration,
		CleanupInterval:   cfg.Performance.ProfileCleanupInterval,
	}, logger)
	
	webServer := api.NewServer(&api.Config{
		Host:             cfg.Server.Host,
		Port:             cfg.Server.Port,
		Production:       cfg.Server.Production,
		DashboardEnabled: cfg.Server.DashboardEnabled,
	}, dashboardHandler, realtimeHandler, systemHandler, profilingHandler, logger)

	// Start web server in goroutine
	go func() {
		if err := webServer.Run(); err != nil {
			logger.WithCaller().Error("Web server error", logger.Args("error", err))
		}
	}()

	logger.Info("🐱 LogLynx is running",
		logger.Args(
			"url", pterm.Sprintf("http://localhost:%d", cfg.Server.Port),
			"processors", coordinator.GetProcessorCount(),
		))

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan

	logger.Info("Shutdown signal received, stopping services...")

	// Stop ingestion coordinator first (prevents new data writes)
	logger.Debug("Stopping ingestion coordinator...")
	coordinator.Stop()

	// Stop cleanup service
	logger.Debug("Stopping cleanup service...")
	cleanupService.Stop()

	// Create shutdown context with timeout (30s to handle SSE connections gracefully)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*cfg.Performance.RealtimeMetricsInterval)
	defer cancel()

	// Stop web server (this will close SSE connections)
	logger.Debug("Stopping web server...")
	if err := webServer.Shutdown(shutdownCtx); err != nil {
		logger.WithCaller().Error("Web server shutdown error", logger.Args("error", err))
	} else {
		logger.Info("Web server stopped successfully")
	}

	// Close GeoIP
	if geoIP != nil {
		geoIP.Close()
	}

	logger.Info("LogLynx stopped gracefully")
}
