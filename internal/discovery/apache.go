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
	"loglynx/internal/database/models"
	"os"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
)

// apacheCLFPattern covers combined, common, and vhost_combined Apache formats.
var apacheCLFPattern = regexp.MustCompile(
	`^(\S+) (\S+) \S+ (\S+) \[([^\]]+)\] "([A-Z]+) ([^ "]+) HTTP/[0-9.]+" (\d{3}) (\d+|-)`,
)

// ApacheDetector detects Apache HTTP Server access log files configured via APACHE_LOG_PATH(S).
// Auto-discovery is not supported since CLF is indistinguishable from nginx without explicit configuration.
type ApacheDetector struct {
	logger          *pterm.Logger
	configuredPaths []string
}

func NewApacheDetector(logger *pterm.Logger, configuredPaths []string) ServiceDetector {
	return &ApacheDetector{
		logger:          logger,
		configuredPaths: configuredPaths,
	}
}

func (d *ApacheDetector) Name() string { return "apache" }

func (d *ApacheDetector) Detect() ([]*models.LogSource, error) {
	sources := []*models.LogSource{}

	if len(d.configuredPaths) == 0 {
		return sources, nil
	}

	for _, path := range d.configuredPaths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			d.logger.Warn("Configured Apache log path not accessible",
				d.logger.Args("path", path, "error", err))
			continue
		}
		if fileInfo.IsDir() || fileInfo.Size() == 0 {
			d.logger.Debug("Skipping empty or directory path", d.logger.Args("path", path))
			continue
		}
		if !isApacheCLFFormat(path) {
			d.logger.Warn("File does not appear to be an Apache access log", d.logger.Args("path", path))
			continue
		}
		name := generateSourceName("apache", path, sources)
		d.logger.Info("Apache log source registered", d.logger.Args("path", path, "name", name))
		sources = append(sources, &models.LogSource{
			Name:       name,
			Path:       path,
			ParserType: "apache",
		})
	}

	return sources, nil
}

func isApacheCLFFormat(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		return apacheCLFPattern.MatchString(line)
	}
	return false
}
