package ingestion

import (
	"bufio"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	parsers "loglynx/internal/parser"

	"github.com/pterm/pterm"
)

// IncrementalReader reads log files incrementally, tracking position
// and detecting log rotation
type IncrementalReader struct {
	filePath        string
	lastPosition    int64
	lastInode       int64 // File identifier (inode on Unix, file index on Windows)
	lastLineContent string
	logger          *pterm.Logger
}

// NewIncrementalReader creates a new incremental reader
func NewIncrementalReader(filePath string, lastPos int64, lastInode int64, lastLine string, logger *pterm.Logger) *IncrementalReader {
	return &IncrementalReader{
		filePath:        filePath,
		lastPosition:    lastPos,
		lastInode:       lastInode,
		lastLineContent: lastLine,
		logger:          logger,
	}
}

// ReadBatch reads up to maxLines new lines from the file
// Returns: lines read, new position, new inode, last line content (for continuity check), error
func (r *IncrementalReader) ReadBatch(maxLines int) ([]string, int64, int64, string, error) {
	// Check if file exists first
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		r.logger.Warn("Log file does not exist yet, waiting for creation",
			r.logger.Args("path", r.filePath))
		return []string{}, r.lastPosition, r.lastInode, r.lastLineContent, nil // Return empty, don't error
	}

	file, err := os.Open(r.filePath)
	if err != nil {
		// Check if it's a permission error
		if os.IsPermission(err) {
			r.logger.Error("Permission denied accessing log file",
				r.logger.Args("path", r.filePath, "error", err))
			return []string{}, r.lastPosition, r.lastInode, r.lastLineContent, nil // Don't crash, just skip this read
		}
		r.logger.Warn("Failed to open log file, will retry",
			r.logger.Args("path", r.filePath, "error", err))
		return []string{}, r.lastPosition, r.lastInode, r.lastLineContent, nil // Return empty, don't error
	}
	defer file.Close()

	// Check file size and inode for rotation detection
	stat, err := file.Stat()
	if err != nil {
		r.logger.WithCaller().Error("Failed to stat log file", r.logger.Args("path", r.filePath, "error", err))
		return nil, 0, 0, "", err
	}

	fileSize := stat.Size()

	// Get current file inode
	currentInode, err := getFileInode(file)
	if err != nil {
		r.logger.WithCaller().Warn("Failed to get file inode", r.logger.Args("path", r.filePath, "error", err))
		currentInode = 0 // Continue without inode check
	}

	// ROTATION DETECTION CASE 1: File identity changed (deleted and recreated)
	// This happens when inode changes, indicating the file was deleted and a new file created
	if r.lastInode != 0 && currentInode != 0 && currentInode != r.lastInode {
		r.logger.Info("Log rotation detected: file deleted and recreated (inode changed)",
			r.logger.Args(
				"path", r.filePath,
				"old_inode", r.lastInode,
				"new_inode", currentInode,
			))
		r.lastPosition = 0
		r.lastLineContent = ""
		r.lastInode = currentInode
	} else if currentInode != 0 {
		// Update inode for next check
		r.lastInode = currentInode
	}

	// ROTATION DETECTION CASE 2: File truncated (size < last position)
	if fileSize < r.lastPosition {
		r.logger.Info("Log rotation detected: file truncated",
			r.logger.Args(
				"path", r.filePath,
				"old_size", r.lastPosition,
				"new_size", fileSize,
			))
		r.lastPosition = 0
		r.lastLineContent = ""
	}

	// Seek to last known position
	_, err = file.Seek(r.lastPosition, 0)
	if err != nil {
		r.logger.WithCaller().Error("Failed to seek in log file",
			r.logger.Args("path", r.filePath, "position", r.lastPosition, "error", err))
		return nil, 0, 0, "", err
	}

	r.logger.Info("🔍 Starting ReadBatch",
		r.logger.Args(
			"path", r.filePath,
			"lastPosition", r.lastPosition,
			"maxLines", maxLines,
		))

	// The scanner will automatically handle line boundaries correctly from any position
	// by reading until it finds a complete line (ending with \n)

	lines := []string{}
	scanner := bufio.NewScanner(file)

	// Increase scanner buffer size to handle long log lines (default is 64KB)
	// Traefik logs with long URLs or large request bodies can exceed this
	const maxScanTokenSize = 1024 * 1024 // 1MB max line length
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() && len(lines) < maxLines {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Add line to batch
		lines = append(lines, line)

		// Log first few lines for debugging (increased to see more data)
		if len(lines) <= 10 {
			r.logger.Info("📄 Read line from file",
				r.logger.Args(
					"line_number", len(lines),
					"line_length", len(line),
					"line_full", line, // Log the FULL line to see if duplicates
				))
		}
	}

	if err := scanner.Err(); err != nil {
		r.logger.WithCaller().Error("Scanner error while reading log file",
			r.logger.Args("path", r.filePath, "error", err))
		return nil, 0, 0, "", err
	}

	// After reading a batch, the file pointer is at the start of the *next* line.
	// Get the current file position - this is where we'll start reading next time
	newPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		r.logger.WithCaller().Warn("Failed to get current position",
			r.logger.Args("path", r.filePath, "error", err))
		// If we can't get the position, stick with the old one to force a re-read
		newPos = r.lastPosition
	}

	// If we read any lines, update our tracking info
	if len(lines) > 0 {
		lastLineRead := lines[len(lines)-1]

		// Sanity check: new position should be >= old position
		if newPos < r.lastPosition {
			r.logger.Warn("New position is less than old position - possible file rotation",
				r.logger.Args(
					"path", r.filePath,
					"old_position", r.lastPosition,
					"new_position", newPos,
				))
		}

		// Get last line for tracking (used for debugging)
		lastLineForCheck := getTail(lastLineRead, 100)

		r.logger.Trace("📖 Read batch from log file",
			r.logger.Args(
				"path", r.filePath,
				"lines_read", len(lines),
				"old_position", r.lastPosition,
				"new_position", newPos,
				"position_delta", newPos-r.lastPosition,
				"last_line_preview", lastLineForCheck,
			))

		r.logger.Info("✅ ReadBatch completed - returning new position",
			r.logger.Args(
				"lines_read", len(lines),
				"old_position", r.lastPosition,
				"new_position", newPos,
				"will_update_internal_position", false, // Position updated by processor calling UpdatePosition()
			))

		return lines, newPos, r.lastInode, lastLineForCheck, nil
	}

	// No new lines were read
	r.logger.Trace("No new lines in file",
		r.logger.Args("path", r.filePath, "position", r.lastPosition))

	return []string{}, r.lastPosition, r.lastInode, r.lastLineContent, nil
}

// UpdatePosition is called by the processor to confirm the position after a successful batch write.
func (r *IncrementalReader) UpdatePosition(position int64, inode int64, lastLine string) {
	// This function is now less critical as ReadBatch returns the correct state,
	// but we keep it for explicit state management by the caller if needed.

	r.logger.Info("🔄 UpdatePosition called by processor",
		r.logger.Args(
			"path", r.filePath,
			"old_position", r.lastPosition,
			"new_position", position,
			"position_delta", position-r.lastPosition,
			"inode", inode,
		))

	r.lastPosition = position
	r.lastInode = inode
	r.lastLineContent = lastLine
}

// Reset resets the reader to the beginning of the file
func (r *IncrementalReader) Reset() {
	r.logger.Info("Resetting reader to beginning", r.logger.Args("path", r.filePath))
	r.lastPosition = 0
	r.lastInode = 0
	r.lastLineContent = ""
}

// getTail returns the last maxLen characters of a string
func getTail(s string, maxLen int) string {
	if s == "" {
		return ""
	}

	// Remove trailing whitespace for comparison
	s = strings.TrimRight(s, " \t\n\r")

	if len(s) <= maxLen {
		return s
	}
	return s[len(s)-maxLen:]
}

// FindStartPositionByDate finds the file position to start reading from based on a cutoff date
// This is used for initial import limiting (e.g., only import last N days)
// Returns: starting position, error
func (r *IncrementalReader) FindStartPositionByDate(cutoffDate time.Time, parser parsers.LogParser) (int64, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	fileSize := stat.Size()

	// Use binary search to find approximate position
	// Start from the middle of the file
	low := int64(0)
	high := fileSize
	bestPosition := int64(0)
	foundRecentEnough := false // Track if we found any records after cutoff

	r.logger.Debug("Searching for start position by date",
		r.logger.Args("cutoff_date", cutoffDate.Format("2006-01-02 15:04:05"), "file_size", fileSize))

	// Binary search with max 20 iterations
	for i := 0; i < 20 && low < high; i++ {
		mid := (low + high) / 2

		// Seek to mid position
		if _, err := file.Seek(mid, 0); err != nil {
			return 0, err
		}

		// Find next line boundary
		scanner := bufio.NewScanner(file)
		if mid > 0 {
			// Skip partial line
			scanner.Scan()
		}

		// Read the first complete line
		if !scanner.Scan() {
			// No line found, move lower
			high = mid
			continue
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Try to parse timestamp from this line
		if !parser.CanParse(line) {
			// Can't parse, skip
			r.logger.Trace("Line not parseable during binary search", r.logger.Args("line", getTail(line, 100)))
			low = mid + 1
			continue
		}

		event, err := parser.Parse(line)
		if err != nil {
			// Can't parse, skip
			r.logger.Trace("Failed to parse line during binary search", r.logger.Args("error", err))
			low = mid + 1
			continue
		}

		// Extract timestamp using reflection
		lineTimestamp := extractTimestamp(event)
		if lineTimestamp.IsZero() {
			// No timestamp, skip
			low = mid + 1
			continue
		}

		r.logger.Trace("Binary search iteration",
			r.logger.Args("position", mid, "timestamp", lineTimestamp.Format("2006-01-02 15:04:05"), "target", cutoffDate.Format("2006-01-02 15:04:05")))

		// Compare timestamp
		if lineTimestamp.Before(cutoffDate) {
			// This line is too old, search in upper half
			low = mid + 1
			// Don't update bestPosition when line is too old
		} else {
			// This line is recent enough, search in lower half
			high = mid
			bestPosition = mid
			foundRecentEnough = true
		}
	}

	// If we never found a "recent enough" record, all records are before cutoff
	// In this case, return file size (EOF) to skip the entire file
	if !foundRecentEnough {
		r.logger.Info("All records appear to be before cutoff date, skipping entire file",
			r.logger.Args("position", fileSize, "cutoff_date", cutoffDate.Format("2006-01-02")))
		return fileSize, nil
	}

	r.logger.Info("Found starting position for initial import",
		r.logger.Args("position", bestPosition, "cutoff_date", cutoffDate.Format("2006-01-02")))

	return bestPosition, nil
}

// extractTimestamp extracts timestamp from parsed event using reflection
func extractTimestamp(event interface{}) time.Time {
	// Try to get Timestamp field using type assertion
	type timestampInterface interface {
		GetTimestamp() time.Time
	}

	if ts, ok := event.(timestampInterface); ok {
		return ts.GetTimestamp()
	}

	// Fallback: use reflection to find Timestamp field
	// This is handled by the parser, so we'll just return zero time if not available
	return time.Time{}
}

// getFileInode returns a stable identifier for the file using reflection to access system-specific inode
// This works across platforms (Linux, macOS, Windows) without build tags
func getFileInode(file *os.File) (int64, error) {
	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Try to get the real inode using reflection on stat.Sys()
	// This works on Unix/Linux/macOS where Sys() returns *syscall.Stat_t with Ino field
	sys := stat.Sys()
	if sys != nil {
		// Use reflection to safely access Ino field if it exists
		v := reflect.ValueOf(sys)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			// Try to get Ino field (Unix/Linux/macOS)
			inoField := v.FieldByName("Ino")
			if inoField.IsValid() && inoField.CanUint() {
				return int64(inoField.Uint()), nil
			}

			// Try FileIndex for Windows (similar to inode)
			fileIndexField := v.FieldByName("FileIndexHigh")
			if fileIndexField.IsValid() && fileIndexField.CanUint() {
				fileIndexHigh := fileIndexField.Uint()
				fileIndexLow := uint64(0)
				if lowField := v.FieldByName("FileIndexLow"); lowField.IsValid() && lowField.CanUint() {
					fileIndexLow = lowField.Uint()
				}
				return int64((fileIndexHigh << 32) | fileIndexLow), nil
			}
		}
	}

	// Fallback: Since we can't get a real inode, we return 0 and rely only on file size changes
	// This means we won't detect rotation by inode, but we'll still detect truncation
	return 0, nil
}
