// MIT License
//
// Copyright (c) 2026 Kolin
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

// nginxCLFPattern covers both combined and common CLF used by nginx.
var nginxCLFPattern = regexp.MustCompile(
	`^(\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+) HTTP/[0-9.]+" (\d{3}) (\d+|-)`,
)

// NginxDetector detects nginx access log files configured via NGINX_LOG_PATH(S).
// nginx shares the CLF format with Apache, so auto-discovery without explicit paths
// is not supported to avoid misidentification.
type NginxDetector struct {
	logger          *pterm.Logger
	configuredPaths []string
}

func NewNginxDetector(logger *pterm.Logger, configuredPaths []string) ServiceDetector {
	return &NginxDetector{
		logger:          logger,
		configuredPaths: configuredPaths,
	}
}

func (d *NginxDetector) Name() string { return "nginx" }

func (d *NginxDetector) Detect() ([]*models.LogSource, error) {
	sources := []*models.LogSource{}

	if len(d.configuredPaths) == 0 {
		return sources, nil
	}

	for _, path := range d.configuredPaths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			d.logger.Warn("Configured nginx log path not accessible",
				d.logger.Args("path", path, "error", err))
			continue
		}
		if fileInfo.IsDir() || fileInfo.Size() == 0 {
			d.logger.Debug("Skipping empty or directory path", d.logger.Args("path", path))
			continue
		}
		if !isNginxOrCLFFormat(path) {
			d.logger.Warn("File does not appear to be an nginx access log", d.logger.Args("path", path))
			continue
		}
		name := generateSourceName("nginx", path, sources)
		d.logger.Info("Nginx log source registered", d.logger.Args("path", path, "name", name))
		sources = append(sources, &models.LogSource{
			Name:       name,
			Path:       path,
			ParserType: "nginx",
		})
	}

	return sources, nil
}

// isNginxOrCLFFormat checks if a file is either JSON with remote_addr or CLF format.
func isNginxOrCLFFormat(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return false
		}
		// JSON format: check for remote_addr field
		if line[0] == '{' {
			var m map[string]any
			if err := json.Unmarshal([]byte(line), &m); err == nil {
				_, hasAddr := m["remote_addr"]
				return hasAddr
			}
			return false
		}
		// CLF format
		return nginxCLFPattern.MatchString(line)
	}
	return false
}
