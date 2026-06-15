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

// haproxyPattern matches the core of a HAProxy HTTP log line (after optional syslog prefix).
// Requires the bracketed timestamp and timing fields that are distinctive to HAProxy.
var haproxyPattern = regexp.MustCompile(
	`\[(\d{2}/\w{3}/\d{4}:\d{2}:\d{2}:\d{2}[.\d]*)\] \S+ \S+ [-\d]+/[-\d]+/[-\d]+/[-\d]+/[-\d]+ \d{3}`,
)

// haproxySyslogPrefix strips the syslog prefix from a log line.
var haproxySyslogPrefix = regexp.MustCompile(`^\w+ +\d+ +\d+:\d+:\d+ \S+ \S+\[\d+\]: `)

// HAProxyDetector detects HAProxy HTTP log files configured via HAPROXY_LOG_PATH(S).
type HAProxyDetector struct {
	logger          *pterm.Logger
	configuredPaths []string
}

func NewHAProxyDetector(logger *pterm.Logger, configuredPaths []string) ServiceDetector {
	return &HAProxyDetector{
		logger:          logger,
		configuredPaths: configuredPaths,
	}
}

func (d *HAProxyDetector) Name() string { return "haproxy" }

func (d *HAProxyDetector) Detect() ([]*models.LogSource, error) {
	sources := []*models.LogSource{}

	if len(d.configuredPaths) == 0 {
		return sources, nil
	}

	for _, path := range d.configuredPaths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			d.logger.Warn("Configured HAProxy log path not accessible",
				d.logger.Args("path", path, "error", err))
			continue
		}
		if fileInfo.IsDir() || fileInfo.Size() == 0 {
			d.logger.Debug("Skipping empty or directory path", d.logger.Args("path", path))
			continue
		}
		if !isHAProxyFormat(path) {
			d.logger.Warn("File does not appear to be a HAProxy HTTP log", d.logger.Args("path", path))
			continue
		}
		name := generateSourceName("haproxy", path, sources)
		d.logger.Info("HAProxy log source registered", d.logger.Args("path", path, "name", name))
		sources = append(sources, &models.LogSource{
			Name:       name,
			Path:       path,
			ParserType: "haproxy",
		})
	}

	return sources, nil
}

func isHAProxyFormat(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Strip syslog prefix if present
		if loc := haproxySyslogPrefix.FindStringIndex(line); loc != nil {
			line = line[loc[1]:]
		}
		return haproxyPattern.MatchString(line)
	}
	return false
}
