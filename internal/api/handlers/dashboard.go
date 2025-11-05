package handlers

import (
	"net/http"
	"strconv"

	"loglynx/internal/database/repositories"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// DashboardHandler handles dashboard requests
type DashboardHandler struct {
	statsRepo repositories.StatsRepository
	httpRepo  repositories.HTTPRequestRepository
	logger    *pterm.Logger
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(
	statsRepo repositories.StatsRepository,
	httpRepo repositories.HTTPRequestRepository,
	logger *pterm.Logger,
) *DashboardHandler {
	return &DashboardHandler{
		statsRepo: statsRepo,
		httpRepo:  httpRepo,
		logger:    logger,
	}
}

// ServiceFilter represents a single service filter
type ServiceFilter struct {
	Name string
	Type string
}

// getServiceFilters extracts service filter parameters from request
// Returns array of service filters
// Supports both new multi-select (services[], service_types[]) and legacy single-select (service, service_type)
func (h *DashboardHandler) getServiceFilters(c *gin.Context) []ServiceFilter {
	// Try new multi-select parameters first
	serviceNames := c.QueryArray("services[]")
	serviceTypes := c.QueryArray("service_types[]")

	if len(serviceNames) > 0 && len(serviceNames) == len(serviceTypes) {
		filters := make([]ServiceFilter, len(serviceNames))
		for i := range serviceNames {
			filters[i] = ServiceFilter{
				Name: serviceNames[i],
				Type: serviceTypes[i],
			}
		}
		return filters
	}

	// Fallback to legacy single-select parameters
	service := c.Query("service")
	serviceType := c.Query("service_type")

	// Fallback to even older "host" parameter
	if service == "" {
		service = c.Query("host")
	}

	// Default service type to "auto" if not specified
	if serviceType == "" {
		serviceType = "auto"
	}

	// Return single filter if specified
	if service != "" {
		return []ServiceFilter{{Name: service, Type: serviceType}}
	}

	return []ServiceFilter{}
}

// convertToRepoFilters converts handler ServiceFilter to repository ServiceFilter
func (h *DashboardHandler) convertToRepoFilters(filters []ServiceFilter) []repositories.ServiceFilter {
	repoFilters := make([]repositories.ServiceFilter, len(filters))
	for i, f := range filters {
		repoFilters[i] = repositories.ServiceFilter{
			Name: f.Name,
			Type: f.Type,
		}
	}
	return repoFilters
}

// getExcludeOwnIP extracts exclude_own_ip and related parameters
// Returns (excludeIP bool, clientIP string, excludeServices []ServiceFilter)
func (h *DashboardHandler) getExcludeOwnIP(c *gin.Context) (bool, string, []ServiceFilter) {
	excludeIP := c.Query("exclude_own_ip") == "true"
	if !excludeIP {
		return false, "", nil
	}

	// Get client IP
	clientIP := c.ClientIP()

	// Get exclude services
	serviceNames := c.QueryArray("exclude_services[]")
	serviceTypes := c.QueryArray("exclude_service_types[]")

	var excludeServices []ServiceFilter
	if len(serviceNames) > 0 && len(serviceNames) == len(serviceTypes) {
		excludeServices = make([]ServiceFilter, len(serviceNames))
		for i := range serviceNames {
			excludeServices[i] = ServiceFilter{
				Name: serviceNames[i],
				Type: serviceTypes[i],
			}
		}
	}

	return true, clientIP, excludeServices
}

// buildExcludeIPFilter builds ExcludeIPFilter from request
func (h *DashboardHandler) buildExcludeIPFilter(c *gin.Context) *repositories.ExcludeIPFilter {
	excludeIPEnabled, clientIP, excludeServices := h.getExcludeOwnIP(c)
	if !excludeIPEnabled {
		return nil
	}

	return &repositories.ExcludeIPFilter{
		ClientIP:        clientIP,
		ExcludeServices: h.convertToRepoFilters(excludeServices),
	}
}

// DEPRECATED: Use getServiceFilters instead
func (h *DashboardHandler) getServiceFilter(c *gin.Context) (string, string) {
	filters := h.getServiceFilters(c)
	if len(filters) > 0 {
		return filters[0].Name, filters[0].Type
	}
	return "", "auto"
}

// HandleDashboard renders the main dashboard page
func (h *DashboardHandler) HandleDashboard(c *gin.Context) {

	summary, err := h.statsRepo.GetSummary(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get summary stats", h.logger.Args("error", err))
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to load dashboard data",
		})
		return
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":   "LogLynx",
		"summary": summary,
	})
}

// GetSummary returns summary statistics
func (h *DashboardHandler) GetSummary(c *gin.Context) {

	summary, err := h.statsRepo.GetSummary(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get summary", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetTimeline returns timeline statistics
func (h *DashboardHandler) GetTimeline(c *gin.Context) {
	hours := 168            // Default to 7 days
	if hoursParam := c.Query("hours"); hoursParam != "" {
		if h, err := strconv.Atoi(hoursParam); err == nil && h > 0 {
			// Support various time ranges: 1h, 24h, 168h (7d), 720h (30d), or larger
			if h <= 8760 { // Max 1 year (365 days)
				hours = h
			} else {
				hours = 8760 // Cap at 1 year
			}
		}
	}

	timeline, err := h.statsRepo.GetTimelineStats(hours, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get timeline", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get timeline"})
		return
	}

	c.JSON(http.StatusOK, timeline)
}

// GetStatusCodeTimeline returns status code distribution over time
func (h *DashboardHandler) GetStatusCodeTimeline(c *gin.Context) {
	hours := 168            // Default to 7 days
	if hoursParam := c.Query("hours"); hoursParam != "" {
		if h, err := strconv.Atoi(hoursParam); err == nil && h > 0 {
			if h <= 8760 {
				hours = h
			} else {
				hours = 8760
			}
		}
	}

	timeline, err := h.statsRepo.GetStatusCodeTimeline(hours, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get status code timeline", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status code timeline"})
		return
	}

	c.JSON(http.StatusOK, timeline)
}

// GetTrafficHeatmap returns traffic heatmap data grouped by day and hour
func (h *DashboardHandler) GetTrafficHeatmap(c *gin.Context) {
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 {
			if d <= 365 {
				days = d
			} else {
				days = 365
			}
		}
	}

	data, err := h.statsRepo.GetTrafficHeatmap(days, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get traffic heatmap", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic heatmap"})
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetTopPaths returns top paths
func (h *DashboardHandler) GetTopPaths(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	paths, err := h.statsRepo.GetTopPaths(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top paths", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top paths"})
		return
	}

	c.JSON(http.StatusOK, paths)
}

// GetTopCountries returns top countries
func (h *DashboardHandler) GetTopCountries(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l >= 0 && l <= 500 {
			limit = l
		}
	}

	countries, err := h.statsRepo.GetTopCountries(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top countries", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top countries"})
		return
	}

	c.JSON(http.StatusOK, countries)
}

// GetTopIPs returns top IP addresses
func (h *DashboardHandler) GetTopIPs(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	ips, err := h.statsRepo.GetTopIPAddresses(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top IPs", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top IPs"})
		return
	}

	c.JSON(http.StatusOK, ips)
}

// GetTopUserAgents returns top user agents
func (h *DashboardHandler) GetTopUserAgents(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	agents, err := h.statsRepo.GetTopUserAgents(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top user agents", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top user agents"})
		return
	}

	c.JSON(http.StatusOK, agents)
}

// GetTopReferrers returns top referrers
func (h *DashboardHandler) GetTopReferrers(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	referrers, err := h.statsRepo.GetTopReferrers(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top referrers", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top referrers"})
		return
	}

	c.JSON(http.StatusOK, referrers)
}

// GetTopReferrerDomains returns top referrer domains
func (h *DashboardHandler) GetTopReferrerDomains(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			// Allow up to 500 domains, or unlimited if limit=0
			if l <= 500 || l == 0 {
				limit = l
			} else {
				limit = 500
			}
		}
	}

	domains, err := h.statsRepo.GetTopReferrerDomains(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top referrer domains", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top referrer domains"})
		return
	}

	c.JSON(http.StatusOK, domains)
}

// GetTopBackends returns top backends
func (h *DashboardHandler) GetTopBackends(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	backends, err := h.statsRepo.GetTopBackends(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top backends", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top backends"})
		return
	}

	c.JSON(http.StatusOK, backends)
}

// GetTopASNs returns top ASNs
func (h *DashboardHandler) GetTopASNs(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	asns, err := h.statsRepo.GetTopASNs(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top ASNs", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top ASNs"})
		return
	}

	c.JSON(http.StatusOK, asns)
}

// GetStatusCodeDistribution returns status code distribution
func (h *DashboardHandler) GetStatusCodeDistribution(c *gin.Context) {

	stats, err := h.statsRepo.GetStatusCodeDistribution(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get status code distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status code distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetMethodDistribution returns HTTP method distribution
func (h *DashboardHandler) GetMethodDistribution(c *gin.Context) {

	stats, err := h.statsRepo.GetMethodDistribution(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get method distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get method distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetProtocolDistribution returns HTTP protocol distribution
func (h *DashboardHandler) GetProtocolDistribution(c *gin.Context) {

	stats, err := h.statsRepo.GetProtocolDistribution(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get protocol distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get protocol distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetTLSVersionDistribution returns TLS version distribution
func (h *DashboardHandler) GetTLSVersionDistribution(c *gin.Context) {

	stats, err := h.statsRepo.GetTLSVersionDistribution(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get TLS version distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get TLS version distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetResponseTimeStats returns response time statistics
func (h *DashboardHandler) GetResponseTimeStats(c *gin.Context) {

	stats, err := h.statsRepo.GetResponseTimeStats(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get response time stats", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get response time stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetRecentRequests returns recent HTTP requests
func (h *DashboardHandler) GetRecentRequests(c *gin.Context) {
	limit := 100
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0
	if offsetParam := c.Query("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	serviceName, serviceType := h.getServiceFilter(c)

	excludeIPEnabled, clientIP, excludeServices := h.getExcludeOwnIP(c)
	var excludeIP string
	var excludeSvcs []repositories.ServiceFilter
	if excludeIPEnabled {
		excludeIP = clientIP
		excludeSvcs = h.convertToRepoFilters(excludeServices)
	}
	
	requests, err := h.httpRepo.FindAll(limit, offset, serviceName, serviceType, excludeIP, excludeSvcs)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get recent requests", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recent requests"})
		return
	}

	c.JSON(http.StatusOK, requests)
}

// GetLogProcessingStats returns log processing statistics
func (h *DashboardHandler) GetLogProcessingStats(c *gin.Context) {
	stats, err := h.statsRepo.GetLogProcessingStats()
	if err != nil {
		h.logger.WithCaller().Error("Failed to get log processing stats", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get log processing stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetTopBrowsers returns top browsers
func (h *DashboardHandler) GetTopBrowsers(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	browsers, err := h.statsRepo.GetTopBrowsers(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top browsers", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top browsers"})
		return
	}

	c.JSON(http.StatusOK, browsers)
}

// GetTopOperatingSystems returns top operating systems
func (h *DashboardHandler) GetTopOperatingSystems(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	osList, err := h.statsRepo.GetTopOperatingSystems(limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top operating systems", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top operating systems"})
		return
	}

	c.JSON(http.StatusOK, osList)
}

// GetDeviceTypeDistribution returns device type distribution
func (h *DashboardHandler) GetDeviceTypeDistribution(c *gin.Context) {

	devices, err := h.statsRepo.GetDeviceTypeDistribution(h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		h.logger.WithCaller().Error("Failed to get device type distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get device type distribution"})
		return
	}

	c.JSON(http.StatusOK, devices)
}

// GetDomains returns all unique domains/hosts with request counts
// DEPRECATED: Use GetServices() instead
func (h *DashboardHandler) GetDomains(c *gin.Context) {
	domains, err := h.statsRepo.GetDomains()
	if err != nil {
		h.logger.WithCaller().Error("Failed to get domains", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get domains"})
		return
	}

	c.JSON(http.StatusOK, domains)
}

// GetServices returns all unique services with their types and request counts
// Supports filtering by backend_name, backend_url, and host with priority fallback
func (h *DashboardHandler) GetServices(c *gin.Context) {
	services, err := h.statsRepo.GetServices()
	if err != nil {
		h.logger.WithCaller().Error("Failed to get services", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get services"})
		return
	}

	c.JSON(http.StatusOK, services)
}

// ============================================
// IP Analytics Handlers
// ============================================

// GetIPDetailedStats returns comprehensive statistics for a specific IP
func (h *DashboardHandler) GetIPDetailedStats(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	stats, err := h.statsRepo.GetIPDetailedStats(ip)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP stats", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetIPTimeline returns timeline data for a specific IP
func (h *DashboardHandler) GetIPTimeline(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	hours := 168 // Default to 7 days
	if hoursParam := c.Query("hours"); hoursParam != "" {
		if h, err := strconv.Atoi(hoursParam); err == nil && h > 0 && h <= 8760 {
			hours = h
		}
	}

	timeline, err := h.statsRepo.GetIPTimelineStats(ip, hours)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP timeline", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP timeline"})
		return
	}

	c.JSON(http.StatusOK, timeline)
}

// GetIPHeatmap returns traffic heatmap for a specific IP
func (h *DashboardHandler) GetIPHeatmap(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 && d <= 365 {
			days = d
		}
	}

	heatmap, err := h.statsRepo.GetIPTrafficHeatmap(ip, days)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP heatmap", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP heatmap"})
		return
	}

	c.JSON(http.StatusOK, heatmap)
}

// GetIPTopPaths returns top paths for a specific IP
func (h *DashboardHandler) GetIPTopPaths(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 20
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	paths, err := h.statsRepo.GetIPTopPaths(ip, limit)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP top paths", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP top paths"})
		return
	}

	c.JSON(http.StatusOK, paths)
}

// GetIPTopBackends returns top backends for a specific IP
func (h *DashboardHandler) GetIPTopBackends(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	backends, err := h.statsRepo.GetIPTopBackends(ip, limit)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP top backends", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP top backends"})
		return
	}

	c.JSON(http.StatusOK, backends)
}

// GetIPStatusCodeDistribution returns status code distribution for a specific IP
func (h *DashboardHandler) GetIPStatusCodeDistribution(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	stats, err := h.statsRepo.GetIPStatusCodeDistribution(ip)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP status codes", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP status codes"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetIPTopBrowsers returns top browsers for a specific IP
func (h *DashboardHandler) GetIPTopBrowsers(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	browsers, err := h.statsRepo.GetIPTopBrowsers(ip, limit)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP top browsers", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP top browsers"})
		return
	}

	c.JSON(http.StatusOK, browsers)
}

// GetIPTopOperatingSystems returns top operating systems for a specific IP
func (h *DashboardHandler) GetIPTopOperatingSystems(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	osList, err := h.statsRepo.GetIPTopOperatingSystems(ip, limit)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP top OS", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP top operating systems"})
		return
	}

	c.JSON(http.StatusOK, osList)
}

// GetIPDeviceTypeDistribution returns device type distribution for a specific IP
func (h *DashboardHandler) GetIPDeviceTypeDistribution(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	devices, err := h.statsRepo.GetIPDeviceTypeDistribution(ip)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP device types", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP device types"})
		return
	}

	c.JSON(http.StatusOK, devices)
}

// GetIPResponseTimeStats returns response time statistics for a specific IP
func (h *DashboardHandler) GetIPResponseTimeStats(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	stats, err := h.statsRepo.GetIPResponseTimeStats(ip)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP response time stats", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP response time stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetIPRecentRequests returns recent requests for a specific IP
func (h *DashboardHandler) GetIPRecentRequests(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 50
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	requests, err := h.statsRepo.GetIPRecentRequests(ip, limit)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get IP recent requests", h.logger.Args("ip", ip, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP recent requests"})
		return
	}

	c.JSON(http.StatusOK, requests)
}

// SearchIPs searches for IPs matching a query string
func (h *DashboardHandler) SearchIPs(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	limit := 20
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	results, err := h.statsRepo.SearchIPs(query, limit)
	if err != nil {
		h.logger.WithCaller().Error("Failed to search IPs", h.logger.Args("query", query, "error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search IPs"})
		return
	}

	c.JSON(http.StatusOK, results)
}

