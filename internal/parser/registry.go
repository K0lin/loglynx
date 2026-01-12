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
package parsers

import (
	"fmt"
	"loglynx/internal/parser/caddy"
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

// caddyParserWrapper wraps caddy.Parser to implement LogParser interface
type caddyParserWrapper struct {
	*caddy.Parser
}

// Parse adapts caddy.Parser.Parse to return Event interface
func (w *caddyParserWrapper) Parse(line string) (Event, error) {
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

	caddyParser := caddy.NewParser(logger)
	registry.Register("caddy", &caddyParserWrapper{caddyParser})
	logger.Debug("Registered parser", logger.Args("type", "caddy"))

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
