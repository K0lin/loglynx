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

	"github.com/pterm/pterm"
)

// SourceProcessor processes logs from a single source
type SourceProcessor struct {
	source         *models.LogSource
	parser         parsers.LogParser
	reader         *IncrementalReader
	httpRepo       repositories.HTTPRequestRepository
	sourceRepo     repositories.LogSourceRepository
	geoIP          *enrichment.GeoIPEnricher
	logger         *pterm.Logger
	batchSize      int
	workerPoolSize int
	batchTimeout   time.Duration
	pollInterval   time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	// Statistics
	totalProcessed int64
	totalErrors    int64
	startTime      time.Time
	statsMu        sync.Mutex
	// First-load tracking
	isInitialLoad       bool // True if this is the first time reading this file (lastPosition == 0)
	initialLoadComplete bool // True after reaching EOF on first load
	initialLoadMu       sync.Mutex
}

// NewSourceProcessor creates a new source processor
func NewSourceProcessor(
	source *models.LogSource,
	parser parsers.LogParser,
	httpRepo repositories.HTTPRequestRepository,
	sourceRepo repositories.LogSourceRepository,
	geoIP *enrichment.GeoIPEnricher,
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

	return &SourceProcessor{
		source:              source,
		parser:              parser,
		reader:              reader,
		httpRepo:            httpRepo,
		sourceRepo:          sourceRepo,
		geoIP:               geoIP,
		logger:              logger,
		batchSize:           batchSize,       // Configurable via BATCH_SIZE env var
		workerPoolSize:      workerPoolSize,  // Configurable via WORKER_POOL_SIZE env var
		batchTimeout:        2 * time.Second, // Or flush after 2 seconds (faster processing)
		pollInterval:        1 * time.Second, // Check for new logs every second
		ctx:                 ctx,
		cancel:              cancel,
		totalProcessed:      0,
		totalErrors:         0,
		startTime:           time.Now(),
		isInitialLoad:       isInitialLoad,
		initialLoadComplete: false,
	}
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

	// Check if database is empty
	recordCount, err := sp.httpRepo.Count()
	if err != nil {
		sp.logger.WithCaller().Warn("Failed to check database record count, skipping import limit",
			sp.logger.Args("source", sp.source.Name, "error", err))
		return nil
	}

	if recordCount == 0 {
		// Database is empty - use binary search for fast initial positioning
		sp.logger.Info("Database is empty, using binary search to find starting position",
			sp.logger.Args("source", sp.source.Name, "cutoff_date", cutoffDate.Format("2006-01-02")))

		startPos, err := sp.reader.FindStartPositionByDate(cutoffDate, sp.parser)
		if err != nil {
			sp.logger.WithCaller().Error("Failed to find start position by date",
				sp.logger.Args("source", sp.source.Name, "error", err))
			return err
		}

		// Update reader position to start from the found position
		sp.reader.UpdatePosition(startPos, 0, "")

		// Update source tracking in database
		if err := sp.sourceRepo.UpdateTracking(sp.source.Name, startPos, 0, ""); err != nil {
			sp.logger.WithCaller().Error("Failed to update source position",
				sp.logger.Args("source", sp.source.Name, "error", err))
			return err
		}

		sp.logger.Info("Initial import limit applied successfully",
			sp.logger.Args("source", sp.source.Name, "start_position", startPos))
	} else {
		// Database already has records - no need for cutoff filtering
		sp.logger.Debug("Database has existing records, skipping import limit",
			sp.logger.Args("source", sp.source.Name, "record_count", recordCount))
	}

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
		select {
		case <-sp.ctx.Done():
			// Flush remaining batch before exit
			if len(batch) > 0 {
				sp.logger.Debug("Flushing remaining batch on shutdown",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))

				// If flush fails, don't update position so we can retry on next startup
				if err := sp.flushBatchWithResult(batch); err == nil {
					sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
					sp.logger.Info("Shutdown complete - final batch flushed successfully",
						sp.logger.Args("source", sp.source.Name))
				} else {
					sp.logger.Error("Failed to flush final batch on shutdown - will retry on next startup",
						sp.logger.Args("source", sp.source.Name, "batch_size", len(batch), "error", err))
					// Don't update position - unflushed data will be re-processed on restart
				}
			}
			return

		case <-positionUpdateTicker.C:
			// CRITICAL FIX: Only update position if batch is empty (already flushed)
			// Never update position for unflushed data - this prevents data loss
			// if the process crashes before the batch is written to database
			if len(batch) == 0 && lastReadPos > 0 && lastReadPos != lastUpdatedPos {
				sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
				lastUpdatedPos = lastReadPos
			}

		case <-flushTimer.C:
			// Timeout: flush batch even if not full
			if len(batch) > 0 {
				sp.logger.Trace("Batch timeout reached, flushing",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))

				// Flush batch and only update position if successful
				if err := sp.flushBatchWithResult(batch); err == nil {
					// Update position only after successful flush
					if lastReadPos > 0 {
						sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
						lastUpdatedPos = lastReadPos
					}
					batch = []*models.HTTPRequest{} // Clear batch only on success
				} else {
					sp.logger.Error("Batch flush failed, will retry on next cycle",
						sp.logger.Args("source", sp.source.Name, "batch_size", len(batch), "error", err))
					// Keep batch for retry, don't update position
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
				// If this is the initial load and we haven't marked it complete yet, do so now
				sp.initialLoadMu.Lock()
				if sp.isInitialLoad && !sp.initialLoadComplete {
					sp.initialLoadComplete = true
					sp.initialLoadMu.Unlock()

					sp.logger.Info("Initial file load completed - reached end of file",
						sp.logger.Args("source", sp.source.Name))
				} else {
					sp.initialLoadMu.Unlock()
				}

				continue // No new lines
			}

			sp.logger.Trace("Read new log lines",
				sp.logger.Args("source", sp.source.Name, "count", len(lines)))

			// Store the position for later update after flush
			lastReadPos = newPos
			lastReadInode = newInode
			lastReadLine = newLastLine

			// Parse lines in parallel
			parsedRequests := sp.parseAndEnrichParallel(lines)
			batch = append(batch, parsedRequests...)

			// Flush if batch is full AND update position only after successful flush
			if len(batch) >= sp.batchSize {
				sp.logger.Trace("Batch full, flushing",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))

				// Flush batch and only update position if successful
				if err := sp.flushBatchWithResult(batch); err == nil {
					batch = []*models.HTTPRequest{} // Clear batch only on success
					flushTimer.Reset(sp.batchTimeout)

					// Update source tracking AFTER successful flush
					sp.updatePosition(lastReadPos, lastReadInode, lastReadLine)
					lastUpdatedPos = lastReadPos
				} else {
					sp.logger.Error("Batch flush failed, will retry on next cycle",
						sp.logger.Args("source", sp.source.Name, "batch_size", len(batch), "error", err))
					// Keep batch for retry, don't update position
				}
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

				// Skip if filtered out (e.g., date before cutoff during initial import)
				if dbRequest == nil {
					continue
				}

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

// flushBatch inserts the batch into the database (legacy method for backward compatibility)
func (sp *SourceProcessor) flushBatch(batch []*models.HTTPRequest) {
	_ = sp.flushBatchWithResult(batch) // Ignore error for legacy callers
}

// flushBatchWithResult inserts the batch into the database and returns error
func (sp *SourceProcessor) flushBatchWithResult(batch []*models.HTTPRequest) error {
	if len(batch) == 0 {
		return nil
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
		return err // Return error to caller
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

	return nil // Success
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

	// Debug logging for first few requests to understand hash generation
	if sp.totalProcessed < 5 {
		sp.logger.Debug("🔐 Hash generation details",
			sp.logger.Args(
				"source", sp.source.Name,
				"timestamp", dbModel.Timestamp.Format("2006-01-02 15:04:05"),
				"timestamp_unix", dbModel.Timestamp.Unix(),
				"client_ip", dbModel.ClientIP,
				"method", dbModel.Method,
				"host", dbModel.Host,
				"path", dbModel.Path,
				"status", dbModel.StatusCode,
				"duration", dbModel.Duration,
				"start_utc", dbModel.StartUTC,
				"requests_total", dbModel.RequestsTotal,
				"hash_input", hashInput,
				"hash", dbModel.RequestHash[:16]+"...",
			))
	}

	return dbModel
}

// truncate truncates a string to maxLen characters for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
