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
package handlers

import (
	"net/http"
	"time"

	"loglynx/internal/database/repositories"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// ProcessorStatusProvider is satisfied by ingestion.Coordinator.
type ProcessorStatusProvider interface {
	GetActiveSourceNames() []string
}

// SourcesHandler handles log source status requests.
type SourcesHandler struct {
	sourceRepo repositories.LogSourceRepository
	coordinator ProcessorStatusProvider
	logger     *pterm.Logger
}

// SourceStatus is the API response shape for a single log source.
type SourceStatus struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	ParserType  string     `json:"parser_type"`
	Status      string     `json:"status"`   // "active" | "standby"
	LastReadAt  *time.Time `json:"last_read_at"`
	LastPosition int64     `json:"last_position_bytes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func NewSourcesHandler(
	sourceRepo repositories.LogSourceRepository,
	coordinator ProcessorStatusProvider,
	logger *pterm.Logger,
) *SourcesHandler {
	return &SourcesHandler{
		sourceRepo:  sourceRepo,
		coordinator: coordinator,
		logger:      logger,
	}
}

// GetSources returns all registered log sources with their current status.
func (h *SourcesHandler) GetSources(c *gin.Context) {
	sources, err := h.sourceRepo.FindAll()
	if err != nil {
		h.logger.WithCaller().Error("Failed to load log sources", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load log sources"})
		return
	}

	// Build set of active processor names for O(1) lookup.
	active := make(map[string]bool)
	for _, name := range h.coordinator.GetActiveSourceNames() {
		active[name] = true
	}

	result := make([]SourceStatus, 0, len(sources))
	for _, s := range sources {
		status := "standby"
		if active[s.Name] {
			status = "active"
		}
		result = append(result, SourceStatus{
			Name:         s.Name,
			Path:         s.Path,
			ParserType:   s.ParserType,
			Status:       status,
			LastReadAt:   s.LastReadAt,
			LastPosition: s.LastPosition,
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, result)
}
