package parsers

import (
	"fmt"
	"loglynx/internal/parser/traefik"

	"github.com/pterm/pterm"
)

// Registry manages all available log parsers
type Registry struct {
	parsers map[string]LogParser
	logger  *pterm.Logger
}

// traefikParserWrapper wraps traefik.Parser to implement LogParser interface
type traefikParserWrapper struct {
	*traefik.Parser
}

// Parse adapts traefik.Parser.Parse to return Event interface
func (w *traefikParserWrapper) Parse(line string) (Event, error) {
	return w.Parser.Parse(line)
}

// NewRegistry creates a new parser registry with all built-in parsers
func NewRegistry(logger *pterm.Logger) *Registry {
	registry := &Registry{
		parsers: make(map[string]LogParser),
		logger:  logger,
	}

	// Register built-in parsers with wrappers
	traefikParser := traefik.NewParser(logger)
	registry.Register("traefik", &traefikParserWrapper{traefikParser})
	logger.Debug("Registered parser", logger.Args("type", "traefik"))

	return registry
}

// Register adds a parser to the registry
func (r *Registry) Register(name string, parser LogParser) {
	r.parsers[name] = parser
}

// Get retrieves a parser by type
func (r *Registry) Get(parserType string) (LogParser, error) {
	parser, exists := r.parsers[parserType]
	if !exists {
		r.logger.WithCaller().Warn("Parser not found", r.logger.Args("type", parserType))
		return nil, fmt.Errorf("parser not found: %s", parserType)
	}
	return parser, nil
}

// GetAll returns all registered parsers
func (r *Registry) GetAll() map[string]LogParser {
	return r.parsers
}