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
	"loglynx/internal/parser/apache"
	"loglynx/internal/parser/caddy"
	"loglynx/internal/parser/haproxy"
	"loglynx/internal/parser/nginx"
	"loglynx/internal/parser/traefik"

	"github.com/pterm/pterm"
)

// Registry manages all available log parsers
type Registry struct {
	parsers map[string]LogParser
	logger  *pterm.Logger
}

// --- parser wrappers (adapt concrete *T to the LogParser interface) ---

type traefikParserWrapper struct{ *traefik.Parser }

func (w *traefikParserWrapper) Parse(line string) (Event, error) { return w.Parser.Parse(line) }

type caddyParserWrapper struct{ *caddy.Parser }

func (w *caddyParserWrapper) Parse(line string) (Event, error) { return w.Parser.Parse(line) }

type nginxParserWrapper struct{ *nginx.Parser }

func (w *nginxParserWrapper) Parse(line string) (Event, error) { return w.Parser.Parse(line) }

type apacheParserWrapper struct{ *apache.Parser }

func (w *apacheParserWrapper) Parse(line string) (Event, error) { return w.Parser.Parse(line) }

type haproxyParserWrapper struct{ *haproxy.Parser }

func (w *haproxyParserWrapper) Parse(line string) (Event, error) { return w.Parser.Parse(line) }

// NewRegistry creates a new parser registry with all built-in parsers registered.
func NewRegistry(logger *pterm.Logger) *Registry {
	r := &Registry{
		parsers: make(map[string]LogParser),
		logger:  logger,
	}

	r.Register("traefik", &traefikParserWrapper{traefik.NewParser(logger)})
	r.Register("caddy", &caddyParserWrapper{caddy.NewParser(logger)})
	r.Register("nginx", &nginxParserWrapper{nginx.NewParser(logger)})
	r.Register("apache", &apacheParserWrapper{apache.NewParser(logger)})
	r.Register("haproxy", &haproxyParserWrapper{haproxy.NewParser(logger)})

	logger.Debug("Parser registry initialised",
		logger.Args("parsers", []string{"traefik", "caddy", "nginx", "apache", "haproxy"}))

	return r
}

// Register adds a parser to the registry.
func (r *Registry) Register(name string, parser LogParser) {
	r.parsers[name] = parser
}

// Get retrieves a parser by type.
func (r *Registry) Get(parserType string) (LogParser, error) {
	parser, exists := r.parsers[parserType]
	if !exists {
		r.logger.WithCaller().Warn("Parser not found", r.logger.Args("type", parserType))
		return nil, fmt.Errorf("parser not found: %s", parserType)
	}
	return parser, nil
}

// GetAll returns all registered parsers.
func (r *Registry) GetAll() map[string]LogParser {
	return r.parsers
}
