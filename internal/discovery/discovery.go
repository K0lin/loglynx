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
	"loglynx/internal/config"
	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"

	"github.com/pterm/pterm"
)

type ServiceDetector interface {
	Name() string
	Detect() ([]*models.LogSource, error)
}

type Engine struct {
	repo      repositories.LogSourceRepository
	detectors []ServiceDetector
}

func NewEngine(repo repositories.LogSourceRepository, logger *pterm.Logger, cfg *config.LogSourcesConfig) *Engine {
	return &Engine{
		repo: repo,
		detectors: []ServiceDetector{
			NewTraefikDetector(logger, cfg.TraefikLogPaths, cfg.AutoDiscover),
			NewCaddyDetector(logger, cfg.CaddyLogPaths, cfg.AutoDiscover),
			NewNginxDetector(logger, cfg.NginxLogPaths),
			NewApacheDetector(logger, cfg.ApacheLogPaths),
			NewHAProxyDetector(logger, cfg.HAProxyLogPaths),
		},
	}
}

// Run registers newly discovered log sources that are not yet in the database.
// It always runs all detectors so that adding a new parser type (e.g. nginx) is
// picked up even when Traefik/Caddy sources already exist.
func (e *Engine) Run(logger *pterm.Logger) error {
	existing, err := e.repo.FindAll()
	if err != nil {
		return err
	}

	// Index already-registered source names for O(1) lookup.
	registered := make(map[string]bool, len(existing))
	for _, s := range existing {
		registered[s.Name] = true
	}

	for _, detector := range e.detectors {
		logger.Trace("Running detector", logger.Args("detector", detector.Name()))

		sources, err := detector.Detect()
		if err != nil {
			logger.WithCaller().Warn("Detector failed",
				logger.Args("detector", detector.Name(), "error", err))
			continue
		}

		for _, source := range sources {
			if registered[source.Name] {
				logger.Trace("Source already registered, skipping",
					logger.Args("name", source.Name))
				continue
			}

			if err := e.repo.Create(source); err != nil {
				logger.WithCaller().Error("Failed to register log source",
					logger.Args("name", source.Name, "error", err))
			} else {
				registered[source.Name] = true
				logger.Info("Registered new log source",
					logger.Args("name", source.Name, "path", source.Path, "parser", source.ParserType))
			}
		}
	}

	return nil
}
