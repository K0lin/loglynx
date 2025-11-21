package handlers

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"loglynx/internal/database/repositories"
	"loglynx/internal/realtime"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

const (
	// MaxSSEConnections is the maximum number of simultaneous SSE connections
	MaxSSEConnections = 100
)

// RealtimeHandler handles real-time streaming endpoints
type RealtimeHandler struct {
	collector           *realtime.MetricsCollector
	logger              *pterm.Logger
	activeConnections   int
	maxConnections      int
	connectionMutex     sync.Mutex
}

// NewRealtimeHandler creates a new realtime handler
func NewRealtimeHandler(collector *realtime.MetricsCollector, logger *pterm.Logger) *RealtimeHandler {
	return &RealtimeHandler{
		collector:      collector,
		logger:         logger,
		maxConnections: MaxSSEConnections,
	}
}

// getServiceFilter extracts service filter parameters from request (legacy single service)
// Returns serviceName and serviceType separately
// Falls back to legacy "host" parameter for backward compatibility
func (h *RealtimeHandler) getServiceFilter(c *gin.Context) (string, string) {
	// Try new parameters first
	service := c.Query("service")
	serviceType := c.Query("service_type")

	// Fallback to legacy "host" parameter
	if service == "" {
		service = c.Query("host")
	}

	// Default service type to "auto" if not specified
	if serviceType == "" {
		serviceType = "auto"
	}

	// Return combined format for new filter system
	if service != "" {
		return service, serviceType
	}

	return "", "auto"
}

// getServiceFilters extracts multiple service filters from request
// Returns array of {name, type} service filters
func (h *RealtimeHandler) getServiceFilters(c *gin.Context) []realtime.ServiceFilter {
	// Try new multi-service parameters
	serviceNames := c.QueryArray("services[]")
	serviceTypes := c.QueryArray("service_types[]")

	// If we have multiple services, use them
	if len(serviceNames) > 0 && len(serviceNames) == len(serviceTypes) {
		filters := make([]realtime.ServiceFilter, len(serviceNames))
		for i := range serviceNames {
			filters[i] = realtime.ServiceFilter{
				Name: serviceNames[i],
				Type: serviceTypes[i],
			}
		}
		return filters
	}

	// Fall back to single service filter (legacy)
	service, serviceType := h.getServiceFilter(c)
	if service != "" {
		return []realtime.ServiceFilter{{Name: service, Type: serviceType}}
	}

	return nil
}

// getExcludeOwnIP extracts exclude_own_ip and related parameters
// Returns ExcludeIPFilter or nil
func (h *RealtimeHandler) getExcludeOwnIP(c *gin.Context) *realtime.ExcludeIPFilter {
	excludeIP := c.Query("exclude_own_ip") == "true"
	if !excludeIP {
		return nil
	}

	// Get client IP
	clientIP := c.ClientIP()

	// Get exclude services
	serviceNames := c.QueryArray("exclude_services[]")
	serviceTypes := c.QueryArray("exclude_service_types[]")

	var excludeServices []realtime.ServiceFilter
	if len(serviceNames) > 0 && len(serviceNames) == len(serviceTypes) {
		excludeServices = make([]realtime.ServiceFilter, len(serviceNames))
		for i := range serviceNames {
			excludeServices[i] = realtime.ServiceFilter{
				Name: serviceNames[i],
				Type: serviceTypes[i],
			}
		}
	}

	return &realtime.ExcludeIPFilter{
		ClientIP:        clientIP,
		ExcludeServices: excludeServices,
	}
}

// StreamMetrics streams real-time metrics via Server-Sent Events
func (h *RealtimeHandler) StreamMetrics(c *gin.Context) {
	// Check connection limit
	h.connectionMutex.Lock()
	if h.activeConnections >= h.maxConnections {
		h.connectionMutex.Unlock()
		c.JSON(503, gin.H{"error": "Maximum concurrent connections reached. Please try again later."})
		return
	}
	h.activeConnections++
	currentConnections := h.activeConnections
	// Update collector with active connection count
	h.collector.SetActiveConnections(h.activeConnections)
	h.connectionMutex.Unlock()

	// Ensure we decrement on exit
	defer func() {
		h.connectionMutex.Lock()
		h.activeConnections--
		// Update collector with active connection count
		h.collector.SetActiveConnections(h.activeConnections)
		h.connectionMutex.Unlock()

		// Recover from panics
		if r := recover(); r != nil {
			h.logger.Error("Panic in SSE stream", h.logger.Args("panic", r, "client_ip", c.ClientIP()))
		}
	}()

	// Get filters
	serviceName, _ := h.getServiceFilter(c) // Legacy single service filter
	serviceFilters := h.getServiceFilters(c)
	excludeIPFilter := h.getExcludeOwnIP(c)

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Create a ticker for sending updates
	ticker := time.NewTicker(1 * time.Second) // Send updates every 1 second for real-time responsiveness
	defer ticker.Stop()

	h.logger.Debug("Client connected to real-time metrics stream",
		h.logger.Args("client_ip", c.ClientIP(), "host_filter", serviceName, "exclude_own_ip", excludeIPFilter != nil, "active_connections", currentConnections))

	for {
		select {
		case <-c.Request.Context().Done():
			// Server is shutting down or request context cancelled
			h.logger.Debug("Request context cancelled (server shutdown or timeout)",
				h.logger.Args("client_ip", c.ClientIP()))
			return

		case <-ticker.C:
			// Get current metrics with filters
			var metrics *realtime.RealtimeMetrics

			// Optimization: If no filters are active, use the cached global metrics JSON directly
			// This avoids hitting the database AND avoids JSON marshaling for every connected client
			if serviceName == "" && len(serviceFilters) == 0 && excludeIPFilter == nil {
				if cachedJSON := h.collector.GetCachedJSON(); cachedJSON != nil {
					// Write cached JSON directly to stream
					_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", cachedJSON)
					if err != nil {
						h.logger.Debug("Failed to write SSE data", h.logger.Args("error", err))
						return
					}
					c.Writer.Flush()
					continue
				}
				// Fallback if cache is empty (should rarely happen)
				metrics = h.collector.GetMetrics()
			} else if len(serviceFilters) > 0 || excludeIPFilter != nil {
				// Use new filter system
				metrics = h.collector.GetMetricsWithFilters(serviceName, serviceFilters, excludeIPFilter)
			} else {
				// Use legacy single service filter
				metrics = h.collector.GetMetricsWithHost(serviceName)
			}

			// Marshal to JSON
			data, err := json.Marshal(metrics)
			if err != nil {
				h.logger.Error("Failed to marshal metrics", h.logger.Args("error", err))
				continue
			}

			// Send SSE event (always send for heartbeat, frontend handles duplicates)
			_, err = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			if err != nil {
				h.logger.Debug("Failed to write SSE data", h.logger.Args("error", err))
				return
			}

			// Flush the data immediately
			c.Writer.Flush()
		}
	}
}

// GetCurrentMetrics returns a single snapshot of current metrics
func (h *RealtimeHandler) GetCurrentMetrics(c *gin.Context) {
	serviceName, _ := h.getServiceFilter(c)
	serviceFilters := h.getServiceFilters(c)
	excludeIPFilter := h.getExcludeOwnIP(c)

	var metrics *realtime.RealtimeMetrics
	if len(serviceFilters) > 0 || excludeIPFilter != nil {
		metrics = h.collector.GetMetricsWithFilters(serviceName, serviceFilters, excludeIPFilter)
	} else {
		metrics = h.collector.GetMetricsWithHost(serviceName)
	}

	c.JSON(200, metrics)
}

// GetPerServiceMetrics returns current metrics for each service
func (h *RealtimeHandler) GetPerServiceMetrics(c *gin.Context) {
	serviceFilters := h.getServiceFilters(c)
	excludeIPFilter := h.getExcludeOwnIP(c)

	// Convert realtime filters to repositories filters
	var repoFilters []repositories.ServiceFilter
	if len(serviceFilters) > 0 {
		repoFilters = make([]repositories.ServiceFilter, len(serviceFilters))
		for i, f := range serviceFilters {
			repoFilters[i] = repositories.ServiceFilter{Name: f.Name, Type: f.Type}
		}
	}

	var repoExcludeIP *repositories.ExcludeIPFilter
	if excludeIPFilter != nil {
		repoExcludeIP = &repositories.ExcludeIPFilter{
			ClientIP: excludeIPFilter.ClientIP,
		}
		if len(excludeIPFilter.ExcludeServices) > 0 {
			repoExcludeIP.ExcludeServices = make([]repositories.ServiceFilter, len(excludeIPFilter.ExcludeServices))
			for i, f := range excludeIPFilter.ExcludeServices {
				repoExcludeIP.ExcludeServices[i] = repositories.ServiceFilter{Name: f.Name, Type: f.Type}
			}
		}
	}

	metrics := h.collector.GetPerServiceMetrics(repoFilters, repoExcludeIP)
	c.JSON(200, metrics)
}
