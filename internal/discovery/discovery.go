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

func NewEngine(repo repositories.LogSourceRepository, logger *pterm.Logger) *Engine {
    return &Engine{
        repo: repo,
        detectors: []ServiceDetector{
            NewTraefikDetector(logger),
            NewCaddyDetector(logger),
        },
    }
}

func (e *Engine) Run(logger *pterm.Logger) error {
	logger.Trace("Check if the discovery is needed.")
    existing, err := e.repo.FindAll()
    if err != nil {
        return err
    }
    
    if len(existing) > 0 {
	    logger.Trace("Discovery is not needed.")
        return nil
    }

    logger.Debug("Starting discovery...")
	
    logger.Trace("Running service detectors...")
    for _, detector := range e.detectors {
        sources, err := detector.Detect()
		logger.Trace("Detector executed.", logger.Args("Name", detector.Name()))
        if err != nil {
            logger.WithCaller().Warn("Detection failed,", logger.Args("detector", detector.Name(), "error", err))
            continue
        }

        logger.Trace("Registering discovered sources...")
        for _, source := range sources {
            if err := e.repo.Create(source); err != nil {
				logger.WithCaller().Error("Detection failed,", logger.Args("detector", source.Name, "error", err))
            } else {
				logger.Info("Registered new log source.", logger.Args("Name", source.Name, "Path", source.Path))
            }
        }
    }

    logger.Debug("Discovery completed")
    return nil
}
