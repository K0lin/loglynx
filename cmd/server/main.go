package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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

	"github.com/pterm/pterm"
)

func main() {
	// Initialize logger with INFO level for production (change to TRACE for debugging)
	logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelInfo)

	// Print banner
	banner.Print()

	logger.Info("Initializing LogLynx - Fast Log Analytics...")

	// Load configuration from .env file and environment variables
	cfg, err := config.Load()
	if err != nil {
		logger.WithCaller().Fatal("Failed to load configuration", logger.Args("error", err))
	}

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

	// Run discovery engine to auto-detect log sources
	logger.Debug("Running discovery engine...")
	discoveryEngine := discovery.NewEngine(sourceRepo, logger)
	if err := discoveryEngine.Run(logger); err != nil {
		logger.WithCaller().Warn("Discovery engine failed", logger.Args("error", err))
	}

	// Initialize ingestion coordinator
	logger.Debug("Initializing ingestion coordinator...")
	coordinator := ingestion.NewCoordinator(
		sourceRepo,
		httpRepo,
		parserRegistry,
		geoIP,
		logger,
	)

	// Start ingestion engine
	logger.Info("Starting ingestion engine...")
	if err := coordinator.Start(); err != nil {
		logger.WithCaller().Fatal("Failed to start ingestion coordinator", logger.Args("error", err))
	}

	logger.Info("Ingestion engine started",
		logger.Args("processors", coordinator.GetProcessorCount()))

	// Initialize real-time metrics collector with configured interval
	logger.Info("Initializing real-time metrics collector...")
	metricsCollector := realtime.NewMetricsCollector(db, logger)
	metricsCollector.Start(cfg.Performance.RealtimeMetricsInterval)

	// Initialize web server with configured settings
	logger.Info("Initializing web server...")
	dashboardHandler := handlers.NewDashboardHandler(statsRepo, httpRepo, logger)
	realtimeHandler := handlers.NewRealtimeHandler(metricsCollector, logger)
	webServer := api.NewServer(&api.Config{
		Host:       cfg.Server.Host,
		Port:       cfg.Server.Port,
		Production: cfg.Server.Production,
	}, dashboardHandler, realtimeHandler, logger)

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
