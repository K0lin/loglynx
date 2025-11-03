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

// HandleDashboard renders the main dashboard page
func (h *DashboardHandler) HandleDashboard(c *gin.Context) {
	host := c.Query("host") // Optional host filter

	summary, err := h.statsRepo.GetSummary(host)
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
	host := c.Query("host") // Optional host filter

	summary, err := h.statsRepo.GetSummary(host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get summary", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetTimeline returns timeline statistics
func (h *DashboardHandler) GetTimeline(c *gin.Context) {
	host := c.Query("host") // Optional host filter
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

	timeline, err := h.statsRepo.GetTimelineStats(hours, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get timeline", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get timeline"})
		return
	}

	c.JSON(http.StatusOK, timeline)
}

// GetStatusCodeTimeline returns status code distribution over time
func (h *DashboardHandler) GetStatusCodeTimeline(c *gin.Context) {
	host := c.Query("host") // Optional host filter
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

	timeline, err := h.statsRepo.GetStatusCodeTimeline(hours, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get status code timeline", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status code timeline"})
		return
	}

	c.JSON(http.StatusOK, timeline)
}

// GetTrafficHeatmap returns traffic heatmap data grouped by day and hour
func (h *DashboardHandler) GetTrafficHeatmap(c *gin.Context) {
	host := c.Query("host") // Optional host filter
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

	data, err := h.statsRepo.GetTrafficHeatmap(days, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get traffic heatmap", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic heatmap"})
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetTopPaths returns top paths
func (h *DashboardHandler) GetTopPaths(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	paths, err := h.statsRepo.GetTopPaths(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top paths", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top paths"})
		return
	}

	c.JSON(http.StatusOK, paths)
}

// GetTopCountries returns top countries
func (h *DashboardHandler) GetTopCountries(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l >= 0 && l <= 500 {
			limit = l
		}
	}

	countries, err := h.statsRepo.GetTopCountries(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top countries", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top countries"})
		return
	}

	c.JSON(http.StatusOK, countries)
}

// GetTopIPs returns top IP addresses
func (h *DashboardHandler) GetTopIPs(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	ips, err := h.statsRepo.GetTopIPAddresses(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top IPs", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top IPs"})
		return
	}

	c.JSON(http.StatusOK, ips)
}

// GetTopUserAgents returns top user agents
func (h *DashboardHandler) GetTopUserAgents(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	agents, err := h.statsRepo.GetTopUserAgents(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top user agents", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top user agents"})
		return
	}

	c.JSON(http.StatusOK, agents)
}

// GetTopReferrers returns top referrers
func (h *DashboardHandler) GetTopReferrers(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	referrers, err := h.statsRepo.GetTopReferrers(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top referrers", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top referrers"})
		return
	}

	c.JSON(http.StatusOK, referrers)
}

// GetTopReferrerDomains returns top referrer domains
func (h *DashboardHandler) GetTopReferrerDomains(c *gin.Context) {
	host := c.Query("host") // Optional host filter
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

	domains, err := h.statsRepo.GetTopReferrerDomains(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top referrer domains", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top referrer domains"})
		return
	}

	c.JSON(http.StatusOK, domains)
}

// GetTopBackends returns top backends
func (h *DashboardHandler) GetTopBackends(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	backends, err := h.statsRepo.GetTopBackends(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top backends", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top backends"})
		return
	}

	c.JSON(http.StatusOK, backends)
}

// GetTopASNs returns top ASNs
func (h *DashboardHandler) GetTopASNs(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	asns, err := h.statsRepo.GetTopASNs(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top ASNs", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top ASNs"})
		return
	}

	c.JSON(http.StatusOK, asns)
}

// GetStatusCodeDistribution returns status code distribution
func (h *DashboardHandler) GetStatusCodeDistribution(c *gin.Context) {
	host := c.Query("host") // Optional host filter

	stats, err := h.statsRepo.GetStatusCodeDistribution(host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get status code distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status code distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetMethodDistribution returns HTTP method distribution
func (h *DashboardHandler) GetMethodDistribution(c *gin.Context) {
	host := c.Query("host") // Optional host filter

	stats, err := h.statsRepo.GetMethodDistribution(host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get method distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get method distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetProtocolDistribution returns HTTP protocol distribution
func (h *DashboardHandler) GetProtocolDistribution(c *gin.Context) {
	host := c.Query("host") // Optional host filter

	stats, err := h.statsRepo.GetProtocolDistribution(host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get protocol distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get protocol distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetTLSVersionDistribution returns TLS version distribution
func (h *DashboardHandler) GetTLSVersionDistribution(c *gin.Context) {
	host := c.Query("host") // Optional host filter

	stats, err := h.statsRepo.GetTLSVersionDistribution(host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get TLS version distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get TLS version distribution"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetResponseTimeStats returns response time statistics
func (h *DashboardHandler) GetResponseTimeStats(c *gin.Context) {
	host := c.Query("host") // Optional host filter

	stats, err := h.statsRepo.GetResponseTimeStats(host)
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

	host := c.Query("host") // Optional host filter

	requests, err := h.httpRepo.FindAll(limit, offset, host)
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
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	browsers, err := h.statsRepo.GetTopBrowsers(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top browsers", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top browsers"})
		return
	}

	c.JSON(http.StatusOK, browsers)
}

// GetTopOperatingSystems returns top operating systems
func (h *DashboardHandler) GetTopOperatingSystems(c *gin.Context) {
	host := c.Query("host") // Optional host filter
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	osList, err := h.statsRepo.GetTopOperatingSystems(limit, host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get top operating systems", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top operating systems"})
		return
	}

	c.JSON(http.StatusOK, osList)
}

// GetDeviceTypeDistribution returns device type distribution
func (h *DashboardHandler) GetDeviceTypeDistribution(c *gin.Context) {
	host := c.Query("host") // Optional host filter

	devices, err := h.statsRepo.GetDeviceTypeDistribution(host)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get device type distribution", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get device type distribution"})
		return
	}

	c.JSON(http.StatusOK, devices)
}

// GetDomains returns all unique domains/hosts with request counts
func (h *DashboardHandler) GetDomains(c *gin.Context) {
	domains, err := h.statsRepo.GetDomains()
	if err != nil {
		h.logger.WithCaller().Error("Failed to get domains", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get domains"})
		return
	}

	c.JSON(http.StatusOK, domains)
}
