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