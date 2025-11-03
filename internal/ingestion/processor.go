package ingestion

import (
	"context"
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
}

// NewSourceProcessor creates a new source processor
func NewSourceProcessor(
	source *models.LogSource,
	parser parsers.LogParser,
	httpRepo repositories.HTTPRequestRepository,
	sourceRepo repositories.LogSourceRepository,
	geoIP *enrichment.GeoIPEnricher,
	logger *pterm.Logger,
) *SourceProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	reader := NewIncrementalReader(
		source.Path,
		source.LastPosition,
		source.LastLineContent,
		logger,
	)

	return &SourceProcessor{
		source:         source,
		parser:         parser,
		reader:         reader,
		httpRepo:       httpRepo,
		sourceRepo:     sourceRepo,
		geoIP:          geoIP,
		logger:         logger,
		batchSize:      1000,            // Process 1000 events per batch (10x improvement)
		batchTimeout:   2 * time.Second, // Or flush after 2 seconds (faster processing)
		pollInterval:   1 * time.Second, // Check for new logs every second
		ctx:            ctx,
		cancel:         cancel,
		totalProcessed: 0,
		totalErrors:    0,
		startTime:      time.Now(),
	}
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

	for {
		select {
		case <-sp.ctx.Done():
			// Flush remaining batch before exit
			if len(batch) > 0 {
				sp.logger.Debug("Flushing remaining batch on shutdown",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))
				sp.flushBatch(batch)
			}
			return

		case <-flushTimer.C:
			// Timeout: flush batch even if not full
			if len(batch) > 0 {
				sp.logger.Trace("Batch timeout reached, flushing",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))
				sp.flushBatch(batch)
				batch = []*models.HTTPRequest{}
			}
			flushTimer.Reset(sp.batchTimeout)

		case <-ticker.C:
			// Poll for new log lines
			lines, newPos, newLastLine, err := sp.reader.ReadBatch(sp.batchSize - len(batch))
			if err != nil {
				sp.logger.WithCaller().Error("Failed to read from log file",
					sp.logger.Args("source", sp.source.Name, "error", err))
				continue
			}

			if len(lines) == 0 {
				continue // No new lines
			}

			sp.logger.Trace("Read new log lines",
				sp.logger.Args("source", sp.source.Name, "count", len(lines)))

			// Parse lines in parallel
			parsedRequests := sp.parseAndEnrichParallel(lines)
			batch = append(batch, parsedRequests...)

			// Flush if batch is full
			if len(batch) >= sp.batchSize {
				sp.logger.Trace("Batch full, flushing",
					sp.logger.Args("source", sp.source.Name, "count", len(batch)))
				sp.flushBatch(batch)
				batch = []*models.HTTPRequest{}
				flushTimer.Reset(sp.batchTimeout)
			}

			// Update source tracking
			if err := sp.sourceRepo.UpdateTracking(sp.source.Name, newPos, newLastLine); err != nil {
				sp.logger.WithCaller().Error("Failed to update source tracking",
					sp.logger.Args("source", sp.source.Name, "error", err))
			} else {
				sp.logger.Trace("Updated source tracking",
					sp.logger.Args("source", sp.source.Name, "position", newPos))
				sp.reader.UpdatePosition(newPos, newLastLine)
			}
		}
	}
}

// parseAndEnrichParallel processes lines in parallel using worker pool
func (sp *SourceProcessor) parseAndEnrichParallel(lines []string) []*models.HTTPRequest {
	if len(lines) == 0 {
		return nil
	}

	// Number of workers optimized for low-resource environments (0.5 CPU core)
	// Reduced from 8 to 4 to minimize CPU usage and goroutine overhead
	numWorkers := 4
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

	// Update stats
	sp.statsMu.Lock()
	sp.totalProcessed += int64(len(batch))
	totalProcessed := sp.totalProcessed
	sp.statsMu.Unlock()

	duration := time.Since(startTime)
	elapsed := time.Since(sp.startTime)
	rate := float64(totalProcessed) / elapsed.Seconds()

	sp.logger.Info("Batch processed successfully",
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

	sp.logger.Trace("Converted event to DB model",
		sp.logger.Args("source", sp.source.Name, "timestamp", dbModel.Timestamp))

	return dbModel
}

// truncate truncates a string to maxLen characters for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
