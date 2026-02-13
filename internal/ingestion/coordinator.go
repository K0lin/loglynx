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
package ingestion

import (
	"fmt"
	"sync"
	"time"

	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"
	"loglynx/internal/enrichment"
	parsers "loglynx/internal/parser"
	"loglynx/internal/realtime"

	"github.com/pterm/pterm"
)

// Coordinator manages multiple source processors
type Coordinator struct {
	sourceRepo          repositories.LogSourceRepository
	httpRepo            repositories.HTTPRequestRepository
	parserReg           *parsers.Registry
	geoIP               *enrichment.GeoIPEnricher
	metricsCollector    *realtime.MetricsCollector
	processors          map[string]*SourceProcessor
	logger              *pterm.Logger
	mu                  sync.RWMutex
	isRunning           bool
	initialImportDays   int
	initialImportEnable bool
	batchSize           int
	workerPoolSize      int
	hasExistingData     bool
}

// NewCoordinator creates a new ingestion coordinator
func NewCoordinator(
	sourceRepo repositories.LogSourceRepository,
	httpRepo repositories.HTTPRequestRepository,
	parserReg *parsers.Registry,
	geoIP *enrichment.GeoIPEnricher,
	metricsCollector *realtime.MetricsCollector,
	logger *pterm.Logger,
	initialImportDays int,
	initialImportEnable bool,
	batchSize int,
	workerPoolSize int,
) *Coordinator {
	return &Coordinator{
		sourceRepo:          sourceRepo,
		httpRepo:            httpRepo,
		parserReg:           parserReg,
		geoIP:               geoIP,
		metricsCollector:    metricsCollector,
		processors:          make(map[string]*SourceProcessor),
		logger:              logger,
		isRunning:           false,
		initialImportDays:   initialImportDays,
		initialImportEnable: initialImportEnable,
		batchSize:           batchSize,
		workerPoolSize:      workerPoolSize,
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

	c.hasExistingData = c.httpRepo.HasExistingData()

	// Create and start a processor for each source
	successCount := 0
	for _, source := range sources {
		if err := c.startSourceProcessorLocked(source); err != nil {
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

// startSourceProcessorLocked creates and starts a processor for a single source
// IMPORTANT: Caller must hold c.mu lock
func (c *Coordinator) startSourceProcessorLocked(source *models.LogSource) error {
	// Check if processor already exists
	if _, exists := c.processors[source.Name]; exists {
		c.logger.Debug("Processor already exists for source, skipping", c.logger.Args("source", source.Name))
		return nil
	}

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
		c.metricsCollector,
		c.logger,
		c.batchSize,
		c.workerPoolSize,
		c.hasExistingData,
	)

	// Apply initial import limit if enabled and this is a new source
	if c.initialImportEnable && c.initialImportDays > 0 {
		if err := processor.ApplyInitialImportLimit(c.initialImportDays); err != nil {
			c.logger.WithCaller().Warn("Failed to apply initial import limit (will import all data)",
				c.logger.Args("source", source.Name, "error", err))
			// Don't fail - just proceed with normal import
		}
	}

	// Start processor
	processor.Start()

	// Add to active processors map
	c.processors[source.Name] = processor

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
	for name, processor := range c.processors {
		wg.Add(1)
		go func(sourceName string, proc *SourceProcessor) {
			defer wg.Done()
			c.logger.Debug("Stopping processor", c.logger.Args("source", sourceName))
			proc.Stop()
		}(name, processor)
	}

	// Wait for all processors to stop
	wg.Wait()

	// Clear processors map
	c.processors = make(map[string]*SourceProcessor)
	c.isRunning = false

	c.logger.Info("Ingestion coordinator stopped successfully")
}

// PauseAll pauses all active processors
func (c *Coordinator) PauseAll() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.logger.Info("Pausing all processors", c.logger.Args("count", len(c.processors)))
	for _, processor := range c.processors {
		processor.Pause()
	}
}

// ResumeAll resumes all paused processors
func (c *Coordinator) ResumeAll() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.logger.Info("Resuming all processors", c.logger.Args("count", len(c.processors)))
	for _, processor := range c.processors {
		processor.Resume()
	}
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

// IsInitialLoadComplete returns whether all processors have completed their initial load
// AND whether database indexes have been created
// This is used to determine when the application is ready to serve API requests
func (c *Coordinator) IsInitialLoadComplete() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// If no processors, check if indexes are being created
	if len(c.processors) == 0 {
		// Even with no processors, check if indexes are being created
		if c.httpRepo.IsIndexCreationActive() {
			return false
		}
		return true
	}

	// Check if all processors have completed initial load
	for _, processor := range c.processors {
		processor.initialLoadMu.Lock()
		isInitial := processor.isInitialLoad && !processor.initialLoadComplete
		processor.initialLoadMu.Unlock()

		// If any processor is still in initial load, return false
		if isInitial {
			return false
		}
	}

	// All processors done, but check if indexes are being created
	if c.httpRepo.IsIndexCreationActive() {
		return false
	}

	return true
}

// Restart stops and restarts the coordinator
func (c *Coordinator) Restart() error {
	c.logger.Info("Restarting ingestion coordinator...")
	c.Stop()
	return c.Start()
}

// AddProcessor dynamically adds a processor for a new log source
// This allows adding sources without stopping existing processors
func (c *Coordinator) AddProcessor(source *models.LogSource) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return fmt.Errorf("coordinator is not running")
	}

	c.logger.Info("Adding new processor dynamically", c.logger.Args("source", source.Name))

	// Use the internal locked method to start the processor
	if err := c.startSourceProcessorLocked(source); err != nil {
		c.logger.WithCaller().Error("Failed to add processor",
			c.logger.Args("source", source.Name, "error", err))
		return fmt.Errorf("failed to add processor: %w", err)
	}

	c.logger.Info("Successfully added new processor",
		c.logger.Args("source", source.Name, "total_processors", len(c.processors)))

	return nil
}

// RemoveProcessor gracefully stops and removes a processor for a log source
// This allows removing sources without stopping other processors
func (c *Coordinator) RemoveProcessor(sourceName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return fmt.Errorf("coordinator is not running")
	}

	processor, exists := c.processors[sourceName]
	if !exists {
		c.logger.Debug("Processor not found, nothing to remove", c.logger.Args("source", sourceName))
		return nil
	}

	c.logger.Info("Removing processor", c.logger.Args("source", sourceName))

	// Stop the processor gracefully
	processor.Stop()

	// Remove from map
	delete(c.processors, sourceName)

	c.logger.Info("Successfully removed processor",
		c.logger.Args("source", sourceName, "remaining_processors", len(c.processors)))

	return nil
}

// SyncWithDatabase reconciles active processors with database log sources
// Adds processors for new sources and removes processors for deleted sources
func (c *Coordinator) SyncWithDatabase() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		c.logger.Debug("Coordinator not running, skipping database sync")
		return nil
	}

	c.logger.Debug("Syncing processors with database...")

	// Load all sources from database
	sources, err := c.sourceRepo.FindAll()
	if err != nil {
		c.logger.WithCaller().Error("Failed to load log sources during sync",
			c.logger.Args("error", err))
		return fmt.Errorf("failed to load log sources: %w", err)
	}

	// Build map of database sources for efficient lookup
	dbSources := make(map[string]*models.LogSource)
	for _, source := range sources {
		dbSources[source.Name] = source
	}

	// Phase 1: Remove processors for sources that no longer exist in DB
	for name := range c.processors {
		if _, exists := dbSources[name]; !exists {
			c.logger.Info("Source removed from database, stopping processor",
				c.logger.Args("source", name))

			// Stop and remove processor
			processor := c.processors[name]
			processor.Stop()
			delete(c.processors, name)
		}
	}

	// Phase 2: Add processors for new sources in DB
	addedCount := 0
	for _, source := range sources {
		if _, exists := c.processors[source.Name]; !exists {
			c.logger.Info("New source found in database, starting processor",
				c.logger.Args("source", source.Name))

			// Start processor for new source
			if err := c.startSourceProcessorLocked(source); err != nil {
				c.logger.WithCaller().Warn("Failed to start processor for new source",
					c.logger.Args("source", source.Name, "error", err))
				// Continue with other sources
				continue
			}
			addedCount++
		}
	}

	if addedCount > 0 {
		c.logger.Info("Database sync completed - processors added",
			c.logger.Args("added", addedCount, "total_processors", len(c.processors)))
	} else {
		c.logger.Debug("Database sync completed - no changes",
			c.logger.Args("total_processors", len(c.processors)))
	}

	return nil
}

// StartSyncLoop starts a background goroutine that periodically syncs with the database
// This ensures new log sources are automatically picked up without manual intervention
func (c *Coordinator) StartSyncLoop(interval time.Duration) {
	c.logger.Info("Starting database sync loop",
		c.logger.Args("interval", interval.String()))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			// Only sync if coordinator is still running
			c.mu.RLock()
			isRunning := c.isRunning
			c.mu.RUnlock()

			if !isRunning {
				c.logger.Debug("Coordinator stopped, exiting sync loop")
				return
			}

			// Perform sync
			if err := c.SyncWithDatabase(); err != nil {
				c.logger.WithCaller().Warn("Database sync failed",
					c.logger.Args("error", err))
			}
		}
	}()
}
