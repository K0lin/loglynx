package discovery

import (
	"bufio"
	"encoding/json"
	"fmt"
	"loglynx/internal/database/models"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// CaddyDetector detects Caddy log files
type CaddyDetector struct {
	logger         *pterm.Logger
	configuredPath string
	autoDiscover   bool
}

// NewCaddyDetector creates a new Caddy detector
func NewCaddyDetector(logger *pterm.Logger) ServiceDetector {
	autoDiscover := true
	if autoDiscoverEnv := os.Getenv("LOG_AUTO_DISCOVER"); autoDiscoverEnv != "" {
		autoDiscover = autoDiscoverEnv == "true"
	}

	return &CaddyDetector{
		logger:         logger,
		configuredPath: os.Getenv("CADDY_LOG_PATH"),
		autoDiscover:   autoDiscover,
	}
}

// Name returns the detector name
func (d *CaddyDetector) Name() string {
	return "caddy"
}

// Detect discovers Caddy log sources
func (d *CaddyDetector) Detect() ([]*models.LogSource, error) {
	sources := []*models.LogSource{}

	paths := []string{}

	// Priority 1: Use CADDY_LOG_PATH if set and valid
	if d.configuredPath != "" {
		if fileInfo, err := os.Stat(d.configuredPath); err == nil && !fileInfo.IsDir() {
			paths = append(paths, d.configuredPath)
			d.logger.Info("Using configured CADDY_LOG_PATH", d.logger.Args("path", d.configuredPath))
		} else {
			d.logger.Warn("Configured CADDY_LOG_PATH is invalid", d.logger.Args("path", d.configuredPath, "error", err))
		}
	} else if d.autoDiscover {
		// Priority 2: Auto-discovery
		d.logger.Info("Auto-discovering Caddy log files...")
		paths = append(paths,
			"caddy/logs/access.log",
			"/var/log/caddy/access.log",
			"/var/log/caddy/access.json",
		)
	}

	// Validate each path
	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			d.logger.Debug("Caddy log path not found", d.logger.Args("path", path))
			continue
		}

		if fileInfo.IsDir() {
			d.logger.Debug("Path is a directory, skipping", d.logger.Args("path", path))
			continue
		}

		if fileInfo.Size() == 0 {
			d.logger.Debug("Log file is empty, skipping", d.logger.Args("path", path))
			continue
		}

		if isCaddyFormat(path, d.logger) {
			d.logger.Info("Caddy log source detected", d.logger.Args("path", path))
			sources = append(sources, &models.LogSource{
				Name:       generateCaddySourceName(path),
				Path:       path,
				ParserType: "caddy",
			})
			break // Only use first valid source
		}
	}

	if len(sources) == 0 {
		d.logger.Info("No Caddy log sources detected")
	}

	return sources, nil
}

// isCaddyFormat checks if a file contains Caddy JSON logs
func isCaddyFormat(path string, logger *pterm.Logger) bool {
	file, err := os.Open(path)
	if err != nil {
		logger.Debug("Failed to open file", logger.Args("path", path, "error", err))
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()

		// Try to parse as JSON
		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			// Check for Caddy-specific fields
			loggerField, hasLogger := logEntry["logger"].(string)
			_, hasRequest := logEntry["request"]

			if hasLogger && strings.HasPrefix(loggerField, "http.log.access") && hasRequest {
				logger.Debug("File matches Caddy format", logger.Args("path", path))
				return true
			}
		}
	}

	logger.Debug("File does not match Caddy format", logger.Args("path", path))
	return false
}

// generateCaddySourceName generates a unique source name from the file path
func generateCaddySourceName(path string) string {
	// Split path and get filename
	pathSplit := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	fileNameExtension := pathSplit[len(pathSplit)-1]

	// Remove extension
	fileName := strings.Split(fileNameExtension, ".")[0]

	return fmt.Sprintf("caddy-%s", fileName)
}
