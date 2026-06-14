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
package handlers

import (
	"loglynx/internal/database/repositories"
	"loglynx/internal/realtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// RealtimeHandler handles real-time metrics requests
type RealtimeHandler struct {
	collector *realtime.MetricsCollector
	logger    *pterm.Logger
}

// NewRealtimeHandler creates a new real-time handler
func NewRealtimeHandler(collector *realtime.MetricsCollector, logger *pterm.Logger) *RealtimeHandler {
	return &RealtimeHandler{
		collector: collector,
		logger:    logger,
	}
}

// getServiceFilter extracts service filter parameters from request
// Supported: service (auto), service_type (backend_name, backend_url, host)
func (h *RealtimeHandler) getServiceFilter(c *gin.Context) (string, string) {
	service := c.Query("service")
	serviceType := c.Query("service_type")
	return service, serviceType
}

// getServiceFilters extracts multiple service filters from array parameters
func (h *RealtimeHandler) getServiceFilters(c *gin.Context) []realtime.ServiceFilter {
	names := c.QueryArray("services[]")
	types := c.QueryArray("service_types[]")

	if len(names) == 0 {
		// Fallback to single service param if array is empty
		service, serviceType := h.getServiceFilter(c)
		if service != "" {
			return []realtime.ServiceFilter{{Name: service, Type: serviceType}}
		}
		return nil
	}

	filters := make([]realtime.ServiceFilter, 0, len(names))
	for i := range names {
		// Basic validation: ensure we have a name
		if names[i] == "" {
			continue
		}

		serviceType := "auto"
		if i < len(types) && types[i] != "" {
			serviceType = types[i]
		}

		filters = append(filters, realtime.ServiceFilter{
			Name: names[i],
			Type: serviceType,
		})
	}

	if len(filters) == 0 {
		// Final fallback to single service param
		service, serviceType := h.getServiceFilter(c)
		if service != "" {
			return []realtime.ServiceFilter{{Name: service, Type: serviceType}}
		}
	}

	return filters
}

// getExcludeOwnIP extracts exclude_own_ip and related parameters
// Returns ExcludeIPFilter or nil
func (h *RealtimeHandler) getExcludeOwnIP(c *gin.Context) *realtime.ExcludeIPFilter {
	// Get manual IPs from query array "excluded_ips[]"
	manualIPs := c.QueryArray("excluded_ips[]")

	// Check if current IP should be excluded
	excludeOwn := c.Query("exclude_own_ip") == "true"

	allIPs := manualIPs
	if excludeOwn {
		allIPs = append(allIPs, c.ClientIP())
	}

	if len(allIPs) == 0 {
		return nil
	}

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
		ClientIPs:       allIPs,
		ExcludeServices: excludeServices,
	}
}

// StreamMetrics streams real-time metrics via Server-Sent Events
func (h *RealtimeHandler) StreamMetrics(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Get filters
	serviceName, _ := h.getServiceFilter(c)
	serviceFilters := h.getServiceFilters(c)
	excludeIPFilter := h.getExcludeOwnIP(c)

	// Add a channel to keep track of active connections
	h.collector.AdjustActiveConnections(1)
	defer h.collector.AdjustActiveConnections(-1)

	// Track connection state
	notify := c.Request.Context().Done()

	// Create ticker for periodic updates (every 1 second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Wave 2: Detailed logging for connection status
	currentConnections := 0 // This would ideally come from collector but simpler for logging
	h.logger.Debug("New SSE connection established",
		h.logger.Args("client_ip", c.ClientIP(), "host_filter", serviceName, "exclude_own_ip", excludeIPFilter != nil, "active_connections", currentConnections))

	for {
		select {
		case <-notify:
			h.logger.Debug("SSE connection closed by client", h.logger.Args("client_ip", c.ClientIP()))
			return
		case <-ticker.C:
			var metrics *realtime.RealtimeMetrics

			// Optimized: if no filter, use global cached JSON
			if serviceName == "" && len(serviceFilters) == 0 && excludeIPFilter == nil {
				jsonBytes := h.collector.GetCachedJSON()
				if jsonBytes != nil {
					c.SSEvent("message", string(jsonBytes))
					c.Writer.Flush()
					continue
				}
				// Fallback to GetMetrics if cache empty
				metrics = h.collector.GetMetrics()
			} else if len(serviceFilters) > 0 || excludeIPFilter != nil {
				// Use new filter-aware method
				metrics = h.collector.GetMetricsWithFilters(serviceName, serviceFilters, excludeIPFilter)
			} else {
				// Use host-specific method (legacy)
				metrics = h.collector.GetMetricsWithHost(serviceName)
			}

			if metrics != nil {
				c.SSEvent("message", metrics)
				c.Writer.Flush()
			}
		}
	}
}

// GetCurrentMetrics returns a single snapshot of real-time metrics
func (h *RealtimeHandler) GetCurrentMetrics(c *gin.Context) {
	serviceName, _ := h.getServiceFilter(c)
	serviceFilters := h.getServiceFilters(c)
	excludeIPFilter := h.getExcludeOwnIP(c)

	var metrics *realtime.RealtimeMetrics
	if len(serviceFilters) > 0 || excludeIPFilter != nil {
		metrics = h.collector.GetMetricsWithFilters(serviceName, serviceFilters, excludeIPFilter)
	} else if serviceName != "" {
		metrics = h.collector.GetMetricsWithHost(serviceName)
	} else {
		metrics = h.collector.GetMetrics()
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
			ClientIPs: excludeIPFilter.ClientIPs,
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

// Shutdown stops the real-time collector
func (h *RealtimeHandler) Shutdown() {
	h.collector.Stop()
}
