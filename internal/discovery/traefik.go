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
package discovery

import (
	"bufio"
	"encoding/json"
	"loglynx/internal/database/models"
	"os"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
)

type TraefikDetector struct{
    logger *pterm.Logger
	configuredPath string
	autoDiscover   bool
}

func NewTraefikDetector(logger *pterm.Logger) ServiceDetector {
	// Read LOG_AUTO_DISCOVER setting (default: true for backward compatibility)
	autoDiscover := true
	if autoDiscoverEnv := os.Getenv("LOG_AUTO_DISCOVER"); autoDiscoverEnv != "" {
		autoDiscover = autoDiscoverEnv == "true"
	}

    return &TraefikDetector{
        logger: logger,
		configuredPath: os.Getenv("TRAEFIK_LOG_PATH"),
		autoDiscover:   autoDiscover,
    }
}

func (d *TraefikDetector) Name() string {
    return "traefik"
}

func (d *TraefikDetector) Detect() ([]*models.LogSource, error) {
    sources := []*models.LogSource{}
    d.logger.Trace("Detecting Traefik log sources...")

	// Build paths list with priority logic:
	// 1. If TRAEFIK_LOG_PATH is set, use ONLY that path (no auto-discovery)
	// 2. If TRAEFIK_LOG_PATH is not set OR points to non-existent file, use auto-discovery
    paths := []string{}

	// Check if configured path exists and is valid
	configuredPathValid := false
	if d.configuredPath != "" {
		d.logger.Debug("Checking configured Traefik log path", d.logger.Args("path", d.configuredPath))
		if fileInfo, err := os.Stat(d.configuredPath); err == nil && !fileInfo.IsDir() {
			configuredPathValid = true
			d.logger.Info("Using configured TRAEFIK_LOG_PATH (auto-discovery disabled)",
				d.logger.Args("path", d.configuredPath))
		} else {
			d.logger.Warn("Configured TRAEFIK_LOG_PATH not accessible, falling back to auto-discovery",
				d.logger.Args("path", d.configuredPath, "error", err))
		}
	}

	// Priority 1: Use configured path if valid (disables auto-discovery)
	if configuredPathValid {
		paths = append(paths, d.configuredPath)
	} else if d.autoDiscover {
		// Priority 2: Auto-discovery - only if enabled AND configured path is not set or invalid
		d.logger.Debug("Using auto-discovery for Traefik log sources",
			d.logger.Args("LOG_AUTO_DISCOVER", true))
		paths = append(paths, "traefik/logs/access.log", "traefik/logs/error.log")
	} else {
		// Auto-discovery disabled and no valid configured path
		d.logger.Info("Auto-discovery disabled and no valid TRAEFIK_LOG_PATH configured",
			d.logger.Args("LOG_AUTO_DISCOVER", false, "TRAEFIK_LOG_PATH", d.configuredPath))
	}

    for _, path := range paths {
        d.logger.Trace("Checking", d.logger.Args("path", path))
        if fileInfo, err := os.Stat(path); err == nil {
            d.logger.Trace("File found", d.logger.Args("path", path))
            if !fileInfo.IsDir() && fileInfo.Size() > 0 {
                d.logger.Trace("Validating format", d.logger.Args("path", path))
                if isTraefikFormat(path) {
                    d.logger.Info("Traefik log source detected", d.logger.Args("path", path))
                    sources = append(sources, &models.LogSource{
                        Name:       generateName(path),
                        Path:       path,
                        ParserType: "traefik",
                    })
					// Only use the first valid source found
					break
                }else{
                    d.logger.WithCaller().Warn("Format invalid - not a Traefik access log", d.logger.Args("path", path))
                }
            } else {
				d.logger.Trace("File is directory or empty", d.logger.Args("path", path, "size", fileInfo.Size()))
			}
        } else {
			d.logger.Trace("File not accessible", d.logger.Args("path", path, "error", err.Error()))
		}
    }

	if len(sources) == 0 {
		if d.configuredPath != "" {
			d.logger.Warn("No valid Traefik log source found at configured path",
				d.logger.Args("TRAEFIK_LOG_PATH", d.configuredPath))
		} else if d.autoDiscover {
			d.logger.Warn("No Traefik log sources found via auto-discovery",
				d.logger.Args("hint", "Set TRAEFIK_LOG_PATH in .env or ensure traefik/logs/access.log exists"))
		} else {
			d.logger.Warn("No Traefik log sources configured",
				d.logger.Args("hint", "Set TRAEFIK_LOG_PATH in .env or enable LOG_AUTO_DISCOVER=true"))
		}
	}

    return sources, nil
}

func isTraefikFormat(path string) bool {
    file, err := os.Open(path)
    if err != nil {
        return false
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    if scanner.Scan() {
        line := scanner.Text()

        // Try JSON format first
        var logEntry map[string]any
        if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
            // Check for multiple Traefik-specific fields to improve detection accuracy
            // Traefik access logs typically contain these fields
            traefikFields := []string{"ClientHost", "RequestMethod", "RequestPath", "DownstreamStatus", "RouterName"}
            matchCount := 0

            for _, field := range traefikFields {
                if _, ok := logEntry[field]; ok {
                    matchCount++
                }
            }

            // If we find at least 2 Traefik-specific fields, consider it a Traefik log
            if matchCount >= 2 {
                return true
            }
        }

        // Try CLF format (both Traefik and generic)
        // Traefik CLF pattern: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>" <requestsTotal> "<router>" "<server_URL>" <duration>ms
        traefikCLFPattern := `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)" (\d+) "([^"]*)" "([^"]*)" (\d+)ms`
        if matched, _ := regexp.MatchString(traefikCLFPattern, line); matched {
            return true
        }

        // Generic CLF pattern: <client> - <userid> [<datetime>] "<method> <request> HTTP/<version>" <status> <size> "<referrer>" "<user_agent>"
        genericCLFPattern := `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"`
        if matched, _ := regexp.MatchString(genericCLFPattern, line); matched {
            return true
        }
    }
    return false
}

func generateName(path string) string {
    pathSplit := strings.Split(path, "/")
    fileNameExtension := pathSplit[(len(pathSplit)-1)]
    fileName := strings.Split(fileNameExtension, ".")[0]
    return "traefik-"+fileName
}
