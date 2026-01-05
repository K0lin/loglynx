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
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sync"
	"time"

	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"
	"loglynx/internal/enrichment"
	parsers "loglynx/internal/parser"
	"loglynx/internal/parser/useragent"
	"loglynx/internal/realtime"

	"github.com/pterm/pterm"
)

// SourceProcessor processes logs from a single source
type SourceProcessor struct {
	source           *models.LogSource
	parser           parsers.LogParser
	reader           *IncrementalReader
	httpRepo         repositories.HTTPRequestRepository
	sourceRepo       repositories.LogSourceRepository
	geoIP            *enrichment.GeoIPEnricher
	metricsCollector *realtime.MetricsCollector
	logger           *pterm.Logger
	batchSize        int
	workerPoolSize   int
	batchTimeout     time.Duration
	pollInterval     time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	// Statistics
	totalProcessed int64
	totalErrors    int64
	startTime      time.Time
	statsMu        sync.Mutex
	// First-load tracking
	isInitialLoad       bool // True if this is the first time reading this file (lastPosition == 0)
	initialLoadComplete bool // True after reaching EOF on first load
	initialLoadMu       sync.Mutex
	// Pause/resume control
	isPaused   bool
	pauseCond  *sync.Cond
	pauseMu    sync.Mutex
}

// NewSourceProcessor creates a new source processor
func NewSourceProcessor(
	source *models.LogSource,
	parser parsers.LogParser,
	httpRepo repositories.HTTPRequestRepository,
	sourceRepo repositories.LogSourceRepository,
	geoIP *enrichment.GeoIPEnricher,
	metricsCollector *realtime.MetricsCollector,
	logger *pterm.Logger,
	batchSize int,
	workerPoolSize int,
) *SourceProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	reader := NewIncrementalReader(
		source.Path,
		source.LastPosition,
		source.LastInode,
		source.LastLineContent,
		logger,
	)

	// Apply defaults if not configured
	if batchSize <= 0 {
		batchSize = 1000
	}
	if workerPoolSize <= 0 {
		workerPoolSize = 4
	}

	// Check if this is an initial load (first time reading this file)
	isInitialLoad := (source.LastPosition == 0)

	sp := &SourceProcessor{
		source:              source,
		parser:              parser,
		reader:              reader,
		httpRepo:            httpRepo,
		sourceRepo:          sourceRepo,
		geoIP:               geoIP,
		metricsCollector:    metricsCollector,
		logger:              logger,
		batchSize:           batchSize,       // Configurable via BATCH_SIZE env var
		workerPoolSize:      workerPoolSize,  // Configurable via WORKER_POOL_SIZE env var
		batchTimeout:        500 * time.Millisecond, // OPTIMIZED: Very short timeout for realtime updates (was 1s)
		pollInterval:        100 * time.Millisecond, // OPTIMIZED: Poll more frequently during initial load (was 1s)
		ctx:                 ctx,
		cancel:              cancel,
		totalProcessed:      0,
		totalErrors:         0,
		startTime:           time.Now(),
		isInitialLoad:       isInitialLoad,
		initialLoadComplete: false,
		isPaused:            false,
	}
	sp.pauseCond = sync.NewCond(&sp.pauseMu)
	return sp
}

// ApplyInitialImportLimit applies date-based limiting for initial imports
// This is called before starting the processor to skip old data
func (sp *SourceProcessor) ApplyInitialImportLimit(importDays int) error {
	// Only apply if this is a new source (position = 0)
	if sp.source.LastPosition != 0 {
		sp.logger.Debug("Skipping initial import limit (source already has position)",
			sp.logger.Args("source", sp.source.Name, "position", sp.source.LastPosition))
		return nil
	}

	if importDays <= 0 {
		sp.logger.Debug("Initial import limit disabled (days=0)",
			sp.logger.Args("source", sp.source.Name))
		return nil
	}

	// Calculate cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -importDays)

	sp.logger.Info("Applying initial import limit",
		sp.logger.Args("source", sp.source.Name, "import_days", importDays, "cutoff_date", cutoffDate.Format("2006-01-02")))

	// Find starting position based on cutoff date
	startPos, err := sp.reader.FindStartPositionByDate(cutoffDate, sp.parser)
	if err != nil {
		sp.logger.WithCaller().Error("Failed to find start position by date",
			sp.logger.Args("source", sp.source.Name, "error", err))
		return err
	}

	// Update reader position
	sp.reader.UpdatePosition(startPos, 0, "")

	// Update source tracking in database
	if err := sp.sourceRepo.UpdateTracking(sp.source.Name, startPos, 0, ""); err != nil {
		sp.logger.WithCaller().Error("Failed to update source position",
			sp.logger.Args("source", sp.source.Name, "error", err))
		return err
	}

	sp.logger.Info("Initial import limit applied successfully",
		sp.logger.Args("source", sp.source.Name, "start_position", startPos))

	return nil
}

// Start begins processing logs from the source
func (sp *SourceProcessor) Start() {
	sp.wg.Add(1)
	go sp.processLoop()
	sp.logger.Info("Started source processor",
		sp.logger.Args("source", sp.source.Name, "path", sp.source.Path))
}

// Stop gracefully stops the processor
func (sp *SourceProcessor) Stop() {
	sp.logger.Debug("Stopping source processor", sp.logger.Args("source", sp.source.Name))
	sp.cancel()
	sp.wg.Wait()
	sp.logger.Info("Stopped source processor", sp.logger.Args("source", sp.source.Name))
}

// Pause temporarily pauses the processor
func (sp *SourceProcessor) Pause() {
	sp.pauseMu.Lock()
	defer sp.pauseMu.Unlock()
	if !sp.isPaused {
		sp.isPaused = true
		sp.logger.Debug("Paused source processor", sp.logger.Args("source", sp.source.Name))
	}
}

// Resume resumes a paused processor
func (sp *SourceProcessor) Resume() {
	sp.pauseMu.Lock()
	defer sp.pauseMu.Unlock()
	if sp.isPaused {
		sp.isPaused = false
		sp.pauseCond.Broadcast() // Wake up all waiting goroutines
		sp.logger.Debug("Resumed source processor", sp.logger.Args("source", sp.source.Name))
	}
}

// waitIfPaused blocks if the processor is paused
func (sp *SourceProcessor) waitIfPaused() {
	sp.pauseMu.Lock()
	for sp.isPaused {
		sp.pauseCond.Wait()
	}
	sp.pauseMu.Unlock()
}

// processLoop is the main processing loop
func (sp *SourceProcessor) processLoop() {
	defer sp.wg.Done()

	batch := []*models.HTTPRequest{}
	ticker := time.NewTicker(sp.pollInterval)
	defer ticker.Stop()

	flushTimer := time.NewTimer(sp.batchTimeout)
	defer flushTimer.Stop()

	// Periodic position update for progress tracking (every 500ms)
	positionUpdateTicker := time.NewTicker(500 * time.Millisecond)
	defer positionUpdateTicker.Stop()

	// Track the position of the last read batch
	var lastReadPos int64
	var lastReadInode int64
	var lastReadLine string
	var lastUpdatedPos int64 // Track last position that was saved to DB

	for {
		// Check if paused and wait
		sp.waitIfPaused()

		select {
		case <-sp.ctx.Done():
			// Flush remaining batch before exit
			if len(batch) > 0 {
				sp.logger.Debug("Flushing remaining batch on shutdown",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))
				sp.flushBatch(batch)
				// Update position after final flush
				sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
			}
			return

		case <-positionUpdateTicker.C:
			// Periodically update position even if batch is not flushed yet
			// This ensures the progress bar updates smoothly
			if lastReadPos > 0 && lastReadPos != lastUpdatedPos {
				sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
				lastUpdatedPos = lastReadPos
			}

		case <-flushTimer.C:
			// Timeout: flush batch even if not full
			if len(batch) > 0 {
				sp.logger.Trace("Batch timeout reached, flushing",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))
				sp.flushBatch(batch)
				batch = []*models.HTTPRequest{}
				// Update position after timeout flush
				if lastReadPos > 0 {
					sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
					lastUpdatedPos = lastReadPos
				}
			}
			flushTimer.Reset(sp.batchTimeout)

		case <-ticker.C:
			// Poll for new log lines
			lines, newPos, newInode, newLastLine, err := sp.reader.ReadBatch(sp.batchSize - len(batch))
			if err != nil {
				sp.logger.WithCaller().Error("Failed to read from log file",
					sp.logger.Args("source", sp.source.Name, "error", err))
				continue
			}

			if len(lines) == 0 {
				// No new lines - reached EOF
				// BUGFIX: Check if we need to mark initial load as complete AND flush pending batch
				sp.initialLoadMu.Lock()
				needsCompletion := sp.isInitialLoad && !sp.initialLoadComplete
				sp.initialLoadMu.Unlock()

				if needsCompletion {
					// Flush any pending batch BEFORE marking load complete
					if len(batch) > 0 {
						sp.logger.Debug("Flushing final batch before marking initial load complete",
							sp.logger.Args("source", sp.source.Name, "count", len(batch)))
						sp.flushBatch(batch)
						batch = []*models.HTTPRequest{}
						if lastReadPos > 0 {
							sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
							lastUpdatedPos = lastReadPos
						}
					}

					// NOW we can safely disable first-load mode
					sp.initialLoadMu.Lock()
					sp.initialLoadComplete = true
					sp.initialLoadMu.Unlock()

					sp.httpRepo.DisableFirstLoadMode()
					sp.logger.Info("Initial file load completed - reached end of file",
						sp.logger.Args("source", sp.source.Name))
				}

				continue // No new lines
			}

			sp.logger.Trace("Read new log lines",
				sp.logger.Args("source", sp.source.Name, "count", len(lines)))

			// Parse lines in parallel
			parsedRequests := sp.parseAndEnrichParallel(lines)
			batch = append(batch, parsedRequests...)

			lastReadPos = newPos
			lastReadInode = newInode
			lastReadLine = newLastLine

			// Update reader position immediately to prevent re-reading the same lines
			sp.reader.UpdatePosition(newPos, newInode, newLastLine)

			// Flush if batch is full AND update position only after successful flush
			if len(batch) >= sp.batchSize {
				sp.logger.Trace("Batch full, flushing",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))
				sp.flushBatch(batch)
				batch = []*models.HTTPRequest{}
				flushTimer.Reset(sp.batchTimeout)

				// Update source tracking AFTER successful flush
				sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
				lastUpdatedPos = lastReadPos
			}
			// Note: Position is updated periodically by positionUpdateTicker
			// even if batch is not full yet (for progress tracking)
		}
	}
}

// updatePosition updates the file position in the database after a successful flush
func (sp *SourceProcessor) updatePosition(position int64, inode int64, lastLine string) {
	if err := sp.sourceRepo.UpdateTracking(sp.source.Name, position, inode, lastLine); err != nil {
		sp.logger.WithCaller().Error("Failed to update source tracking",
			sp.logger.Args("source", sp.source.Name, "error", err))
	} else {
		sp.logger.Trace("Updated source tracking",
			sp.logger.Args("source", sp.source.Name, "position", position, "inode", inode))
		sp.reader.UpdatePosition(position, inode, lastLine)
	}
}

// parseAndEnrichParallel processes lines in parallel using worker pool
func (sp *SourceProcessor) parseAndEnrichParallel(lines []string) []*models.HTTPRequest {
	if len(lines) == 0 {
		return nil
	}

	// Use configured worker pool size (from WORKER_POOL_SIZE env var)
	numWorkers := sp.workerPoolSize
	if numWorkers > len(lines) {
		numWorkers = len(lines)
	}

	// Channels for work distribution
	jobs := make(chan string, len(lines))
	results := make(chan *models.HTTPRequest, len(lines))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range jobs {
				// Skip lines that this parser cannot handle
				if !sp.parser.CanParse(line) {
					sp.logger.Trace("Skipping line not supported by parser",
						sp.logger.Args("source", sp.source.Name, "parser", sp.parser.Name()))
					continue
				}

				event, err := sp.parser.Parse(line)
				if err != nil {
					sp.logger.Warn("Failed to parse log line",
						sp.logger.Args("source", sp.source.Name, "error", err, "line_preview", truncate(line, 100)))
					continue
				}

				// Convert to database model
				dbRequest := sp.convertToDBModel(event)

				// Enrich with GeoIP data
				if sp.geoIP != nil {
					if err := sp.geoIP.Enrich(dbRequest); err != nil {
						sp.logger.Debug("GeoIP enrichment failed",
							sp.logger.Args("ip", dbRequest.ClientIP, "error", err))
					}
				}

				// Parse User-Agent string
				if dbRequest.UserAgent != "" {
					uaInfo := useragent.Parse(dbRequest.UserAgent)
					dbRequest.Browser = uaInfo.Browser
					dbRequest.BrowserVersion = uaInfo.BrowserVersion
					dbRequest.OS = uaInfo.OS
					dbRequest.OSVersion = uaInfo.OSVersion
					dbRequest.DeviceType = uaInfo.DeviceType
				}

				results <- dbRequest
			}
		}()
	}

	// Send jobs
	for _, line := range lines {
		jobs <- line
	}
	close(jobs)

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	parsedRequests := make([]*models.HTTPRequest, 0, len(lines))
	for req := range results {
		parsedRequests = append(parsedRequests, req)
	}

	return parsedRequests
}

// flushBatch inserts the batch into the database
func (sp *SourceProcessor) flushBatch(batch []*models.HTTPRequest) {
	if len(batch) == 0 {
		return
	}

	startTime := time.Now()

	if err := sp.httpRepo.CreateBatch(batch); err != nil {
		sp.logger.WithCaller().Error("Failed to insert batch into database",
			sp.logger.Args(
				"source", sp.source.Name,
				"count", len(batch),
				"error", err,
			))
		// Update error stats
		sp.statsMu.Lock()
		sp.totalErrors += int64(len(batch))
		sp.statsMu.Unlock()
		return
	}

	// Send to real-time metrics collector (now that we have IDs)
	if sp.metricsCollector != nil {
		for _, req := range batch {
			sp.metricsCollector.Ingest(req)
		}
	}

	// Update stats
	sp.statsMu.Lock()
	sp.totalProcessed += int64(len(batch))
	totalProcessed := sp.totalProcessed
	sp.statsMu.Unlock()

	duration := time.Since(startTime)
	elapsed := time.Since(sp.startTime)
	rate := float64(totalProcessed) / elapsed.Seconds()

	sp.logger.Debug("Batch processed successfully",
		sp.logger.Args(
			"source", sp.source.Name,
			"batch_count", len(batch),
			"batch_duration_ms", duration.Milliseconds(),
			"total_processed", totalProcessed,
			"rate_per_sec", int(rate),
			"elapsed", elapsed.Round(time.Second).String(),
		))
}

// convertToDBModel converts a parser event to a database model using reflection
// This avoids import cycles by not importing specific parser packages
func (sp *SourceProcessor) convertToDBModel(event interface{}) *models.HTTPRequest {
	dbModel := &models.HTTPRequest{
		SourceName: sp.source.Name,
		Timestamp:  time.Now(),
	}

	// Use reflection to map fields from event to dbModel
	eventValue := reflect.ValueOf(event)
	if eventValue.Kind() == reflect.Ptr {
		eventValue = eventValue.Elem()
	}

	if eventValue.Kind() != reflect.Struct {
		sp.logger.WithCaller().Warn("Event is not a struct, creating minimal record",
			sp.logger.Args("source", sp.source.Name, "type", eventValue.Kind()))
		return dbModel
	}

	dbModelValue := reflect.ValueOf(dbModel).Elem()

	// Map fields by name from event to dbModel
	for i := 0; i < eventValue.NumField(); i++ {
		eventField := eventValue.Type().Field(i)
		eventFieldValue := eventValue.Field(i)

		// Skip SourceName as we set it explicitly
		if eventField.Name == "SourceName" {
			continue
		}

		// Find corresponding field in dbModel
		dbField := dbModelValue.FieldByName(eventField.Name)
		if dbField.IsValid() && dbField.CanSet() {
			// Set the value if types match
			if dbField.Type() == eventFieldValue.Type() {
				dbField.Set(eventFieldValue)
			}
		}
	}

	// Generate hash for deduplication
	// Hash is based on: timestamp + client IP + method + host + path + query string + status code + duration + startUTC + requestsTotal
	// Duration and StartUTC provide nanosecond precision for better deduplication accuracy
	// RequestsTotal provides additional context for distinguishing requests at router level
	// This uniquely identifies a request while allowing for legitimate duplicates
	// (e.g., same endpoint hit multiple times in same second from different IPs)
	// If Duration or StartUTC are not available (CLF logs), they will be empty/zero and hash will use other fields
	hashInput := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%d|%d|%s|%d",
		dbModel.Timestamp.Unix(),
		dbModel.ClientIP,
		dbModel.Method,
		dbModel.Host,
		dbModel.Path,
		dbModel.QueryString,
		dbModel.StatusCode,
		dbModel.Duration,      // Nanosecond precision duration
		dbModel.StartUTC,      // Nanosecond precision start time
		dbModel.RequestsTotal, // Total requests at router level
	)
	hash := sha256.Sum256([]byte(hashInput))
	dbModel.RequestHash = fmt.Sprintf("%x", hash)

	sp.logger.Trace("Converted event to DB model",
		sp.logger.Args("source", sp.source.Name, "timestamp", dbModel.Timestamp, "hash", dbModel.RequestHash[:16]))

	return dbModel
}

// truncate truncates a string to maxLen characters for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

