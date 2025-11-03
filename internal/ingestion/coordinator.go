package ingestion

import (
	"fmt"
	"sync"

	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"
	"loglynx/internal/enrichment"
	parsers "loglynx/internal/parser"

	"github.com/pterm/pterm"
)

// Coordinator manages multiple source processors
type Coordinator struct {
	sourceRepo   repositories.LogSourceRepository
	httpRepo     repositories.HTTPRequestRepository
	parserReg    *parsers.Registry
	geoIP        *enrichment.GeoIPEnricher
	processors   []*SourceProcessor
	logger       *pterm.Logger
	mu           sync.RWMutex
	isRunning    bool
}

// NewCoordinator creates a new ingestion coordinator
func NewCoordinator(
	sourceRepo repositories.LogSourceRepository,
	httpRepo repositories.HTTPRequestRepository,
	parserReg *parsers.Registry,
	geoIP *enrichment.GeoIPEnricher,
	logger *pterm.Logger,
) *Coordinator {
	return &Coordinator{
		sourceRepo: sourceRepo,
		httpRepo:   httpRepo,
		parserReg:  parserReg,
		geoIP:      geoIP,
		processors: make([]*SourceProcessor, 0),
		logger:     logger,
		isRunning:  false,
	}
}

// Start initializes and starts all source processors
func (c *Coordinator) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		c.logger.Warn("Coordinator already running, skipping start")
		return nil
	}

	c.logger.Info("Starting ingestion coordinator...")

	// Load all sources from database
	sources, err := c.sourceRepo.FindAll()
	if err != nil {
		c.logger.WithCaller().Error("Failed to load log sources from database",
			c.logger.Args("error", err))
		return fmt.Errorf("failed to load log sources: %w", err)
	}

	if len(sources) == 0 {
		c.logger.Warn("No log sources found in database. Please run discovery first or configure log sources manually.")
		c.logger.Info("Ingestion coordinator will run in standby mode, waiting for log sources to be added.")
		c.isRunning = true
		return nil // Don't error, just run in standby mode
	}

	c.logger.Info("Found log sources", c.logger.Args("count", len(sources)))

	// Create and start a processor for each source
	successCount := 0
	for _, source := range sources {
		if err := c.startSourceProcessor(source); err != nil {
			c.logger.WithCaller().Warn("Failed to start processor for source (will retry)",
				c.logger.Args("source", source.Name, "error", err))
			// Continue with other sources instead of failing completely
			continue
		}
		successCount++
	}

	if successCount == 0 {
		c.logger.Warn("No source processors could be started yet. Coordinator will run in standby mode.")
		c.logger.Info("Log files may not exist yet or may have permission issues. Processors will retry automatically.")
	}

	c.isRunning = true
	c.logger.Info("Ingestion coordinator started",
		c.logger.Args("active_processors", successCount, "total_sources", len(sources)))

	return nil
}

// startSourceProcessor creates and starts a processor for a single source
func (c *Coordinator) startSourceProcessor(source *models.LogSource) error {
	// Get the appropriate parser for this source
	parser, err := c.parserReg.Get(source.ParserType)
	if err != nil {
		c.logger.WithCaller().Warn("Parser not found for source",
			c.logger.Args("source", source.Name, "parser_type", source.ParserType, "error", err))
		return fmt.Errorf("parser not found: %w", err)
	}

	c.logger.Debug("Creating processor for source",
		c.logger.Args(
			"source", source.Name,
			"parser", source.ParserType,
			"path", source.Path,
		))

	// Create processor
	processor := NewSourceProcessor(
		source,
		parser,
		c.httpRepo,
		c.sourceRepo,
		c.geoIP,
		c.logger,
	)

	// Start processor
	processor.Start()

	// Add to active processors list
	c.processors = append(c.processors, processor)

	c.logger.Info("Started processor for source",
		c.logger.Args(
			"source", source.Name,
			"path", source.Path,
			"last_position", source.LastPosition,
		))

	return nil
}

// Stop gracefully stops all source processors
func (c *Coordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		c.logger.Debug("Coordinator not running, skipping stop")
		return
	}

	c.logger.Info("Stopping ingestion coordinator...",
		c.logger.Args("active_processors", len(c.processors)))

	// Stop all processors
	var wg sync.WaitGroup
	for i, processor := range c.processors {
		wg.Add(1)
		go func(idx int, proc *SourceProcessor) {
			defer wg.Done()
			c.logger.Debug("Stopping processor", c.logger.Args("index", idx))
			proc.Stop()
		}(i, processor)
	}

	// Wait for all processors to stop
	wg.Wait()

	// Clear processors list
	c.processors = make([]*SourceProcessor, 0)
	c.isRunning = false

	c.logger.Info("Ingestion coordinator stopped successfully")
}

// GetStatus returns the current status of the coordinator
func (c *Coordinator) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"is_running":        c.isRunning,
		"active_processors": len(c.processors),
	}
}

// IsRunning returns whether the coordinator is currently running
func (c *Coordinator) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}

// GetProcessorCount returns the number of active processors
func (c *Coordinator) GetProcessorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// Restart stops and restarts the coordinator
func (c *Coordinator) Restart() error {
	c.logger.Info("Restarting ingestion coordinator...")
	c.Stop()
	return c.Start()
}
