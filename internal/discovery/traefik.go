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
	"fmt"
	"loglynx/internal/database/models"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
)

type TraefikDetector struct {
	logger          *pterm.Logger
	configuredPaths []string
	autoDiscover    bool
}

func NewTraefikDetector(logger *pterm.Logger, configuredPaths []string, autoDiscover bool) ServiceDetector {
	return &TraefikDetector{
		logger:          logger,
		configuredPaths: configuredPaths,
		autoDiscover:    autoDiscover,
	}
}

func (d *TraefikDetector) Name() string { return "traefik" }

func (d *TraefikDetector) Detect() ([]*models.LogSource, error) {
	sources := []*models.LogSource{}

	paths := d.resolvePaths()
	if len(paths) == 0 {
		d.logger.Info("No Traefik log sources configured",
			d.logger.Args("hint", "Set TRAEFIK_LOG_PATH or TRAEFIK_LOG_PATHS in .env, or enable LOG_AUTO_DISCOVER=true"))
		return sources, nil
	}

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			d.logger.Debug("Traefik log path not accessible", d.logger.Args("path", path, "error", err))
			continue
		}
		if fileInfo.IsDir() || fileInfo.Size() == 0 {
			d.logger.Debug("Skipping empty or directory path", d.logger.Args("path", path))
			continue
		}
		if !isTraefikFormat(path) {
			d.logger.Warn("File does not appear to be a Traefik access log", d.logger.Args("path", path))
			continue
		}
		name := generateSourceName("traefik", path, sources)
		d.logger.Info("Traefik log source detected", d.logger.Args("path", path, "name", name))
		sources = append(sources, &models.LogSource{
			Name:       name,
			Path:       path,
			ParserType: "traefik",
		})
	}

	if len(sources) == 0 && len(d.configuredPaths) > 0 {
		d.logger.Warn("No valid Traefik log sources found at configured paths",
			d.logger.Args("paths", strings.Join(d.configuredPaths, ", ")))
	}

	return sources, nil
}

func (d *TraefikDetector) resolvePaths() []string {
	if len(d.configuredPaths) > 0 {
		return d.configuredPaths
	}
	if d.autoDiscover {
		return []string{"traefik/logs/access.log", "traefik/logs/error.log"}
	}
	return nil
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

		var logEntry map[string]any
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			traefikFields := []string{"ClientHost", "RequestMethod", "RequestPath", "DownstreamStatus", "RouterName"}
			matchCount := 0
			for _, field := range traefikFields {
				if _, ok := logEntry[field]; ok {
					matchCount++
				}
			}
			if matchCount >= 2 {
				return true
			}
		}

		traefikCLFPattern := `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)" (\d+) "([^"]*)" "([^"]*)" (\d+)ms`
		if matched, _ := regexp.MatchString(traefikCLFPattern, line); matched {
			return true
		}

		genericCLFPattern := `^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+)? HTTP/[0-9.]+" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"`
		if matched, _ := regexp.MatchString(genericCLFPattern, line); matched {
			return true
		}
	}
	return false
}

// generateSourceName creates a unique source name from a file path.
// If the simple filename-based name collides with an existing source, it adds the parent directory.
func generateSourceName(prefix, path string, existing []*models.LogSource) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	name := fmt.Sprintf("%s-%s", prefix, base)

	for _, s := range existing {
		if s.Name == name {
			// Collision: add parent directory segment
			parent := filepath.Base(filepath.Dir(path))
			name = fmt.Sprintf("%s-%s-%s", prefix, sanitize(parent), base)
			break
		}
	}

	return name
}

// sanitize replaces non-alphanumeric characters with hyphens for use in source names.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}
