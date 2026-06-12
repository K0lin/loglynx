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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"loglynx/internal/database/repositories"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// DashboardHandler handles dashboard data requests
type DashboardHandler struct {
	statsRepo   repositories.StatsRepository
	requestRepo repositories.HTTPRequestRepository
	logger      *pterm.Logger
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(statsRepo repositories.StatsRepository, requestRepo repositories.HTTPRequestRepository, logger *pterm.Logger) *DashboardHandler {
	return &DashboardHandler{
		statsRepo:   statsRepo,
		requestRepo: requestRepo,
		logger:      logger,
	}
}

// ServiceFilter is a local struct for handlers, converted to repositories.ServiceFilter
type ServiceFilter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type comparisonRequest struct {
	Periods  []repositories.ComparisonPeriodRequest `json:"periods"`
	TopLimit int                                    `json:"top_limit"`
}

type createComparisonSnapshotRequest struct {
	Title     string          `json:"title"`
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt *time.Time      `json:"expires_at"`
}

type updateComparisonSnapshotRequest struct {
	Active    *bool      `json:"active"`
	ExpiresAt *time.Time `json:"expires_at"`
}

const comparisonOwnerCookie = "loglynx_compare_owner"

// getServiceFilter extracts service filter from request
func (h *DashboardHandler) getServiceFilter(c *gin.Context) (string, string) {
	service := c.Query("service")
	serviceType := c.Query("service_type")
	return service, serviceType
}

// getServiceFilters extracts multiple service filters from array parameters
func (h *DashboardHandler) getServiceFilters(c *gin.Context) []ServiceFilter {
	names := c.QueryArray("services[]")
	types := c.QueryArray("service_types[]")

	if len(names) == 0 {
		// Fallback to single service param if array is empty
		service, serviceType := h.getServiceFilter(c)
		if service != "" {
			return []ServiceFilter{{Name: service, Type: serviceType}}
		}
		return nil
	}

	filters := make([]ServiceFilter, 0, len(names))
	for i := range names {
		// Basic validation: ensure we have a name
		if names[i] == "" {
			continue
		}

		serviceType := "auto"
		if i < len(types) && types[i] != "" {
			serviceType = types[i]
		}

		filters = append(filters, ServiceFilter{
			Name: names[i],
			Type: serviceType,
		})
	}

	if len(filters) == 0 {
		// Final fallback to single service param
		service, serviceType := h.getServiceFilter(c)
		if service != "" {
			return []ServiceFilter{{Name: service, Type: serviceType}}
		}
	}

	return filters
}

// convertToRepoFilters converts local filters to repository filters
func (h *DashboardHandler) convertToRepoFilters(filters []ServiceFilter) []repositories.ServiceFilter {
	if len(filters) == 0 {
		return nil
	}
	repoFilters := make([]repositories.ServiceFilter, len(filters))
	for i, f := range filters {
		repoFilters[i] = repositories.ServiceFilter{
			Name: f.Name,
			Type: f.Type,
		}
	}
	return repoFilters
}

// getExcludeOwnIP extracts IP exclusion parameters
func (h *DashboardHandler) getExcludeOwnIP(c *gin.Context) (enabled bool, clientIPs []string, excludeServices []ServiceFilter) {
	// Get manual IPs
	manualIPs := c.QueryArray("excluded_ips[]")

	excludeOwn := c.Query("exclude_own_ip") == "true"

	allIPs := manualIPs
	if excludeOwn {
		allIPs = append(allIPs, c.ClientIP())
	}

	if len(allIPs) == 0 {
		return false, nil, nil
	}

	// Get exclude services
	serviceNames := c.QueryArray("exclude_services[]")
	serviceTypes := c.QueryArray("exclude_service_types[]")

	var services []ServiceFilter
	if len(serviceNames) > 0 && len(serviceNames) == len(serviceTypes) {
		services = make([]ServiceFilter, len(serviceNames))
		for i := range serviceNames {
			services[i] = ServiceFilter{
				Name: serviceNames[i],
				Type: serviceTypes[i],
			}
		}
	}

	return true, allIPs, services
}

// buildExcludeIPFilter builds ExcludeIPFilter from request
func (h *DashboardHandler) buildExcludeIPFilter(c *gin.Context) *repositories.ExcludeIPFilter {
	excludeIPEnabled, clientIPs, excludeServices := h.getExcludeOwnIP(c)
	if !excludeIPEnabled {
		return nil
	}

	return &repositories.ExcludeIPFilter{
		ClientIPs:       clientIPs,
		ExcludeServices: h.convertToRepoFilters(excludeServices),
	}
}

func (h *DashboardHandler) getComparisonOwnerID(c *gin.Context) string {
	if ownerID, err := c.Cookie(comparisonOwnerCookie); err == nil && ownerID != "" {
		return ownerID
	}

	ownerID := generateComparisonOwnerID()
	c.SetCookie(comparisonOwnerCookie, ownerID, 365*24*60*60, "/", "", false, true)
	return ownerID
}

func generateComparisonOwnerID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}

// getHours extracts hours parameter from request, defaulting to 168 (7 days)
func (h *DashboardHandler) getHours(c *gin.Context) int {
	hours := 168
	if hoursParam := c.Query("hours"); hoursParam != "" {
		if val, err := strconv.Atoi(hoursParam); err == nil && val >= 0 {
			hours = val
		}
	}
	// Clamp hours to a reasonable maximum (e.g., 1 year = 8760 hours)
	if hours > 8760 {
		hours = 8760
	}
	return hours
}

// GetSummary returns overall statistics
func (h *DashboardHandler) GetSummary(c *gin.Context) {
	summary, err := h.statsRepo.GetSummary(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get summary"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetTimeline returns timeline statistics
func (h *DashboardHandler) GetTimeline(c *gin.Context) {
	timeline, err := h.statsRepo.GetTimelineStats(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get timeline"})
		return
	}
	c.JSON(http.StatusOK, timeline)
}

// GetStatusCodeTimeline returns status code distribution over time
func (h *DashboardHandler) GetStatusCodeTimeline(c *gin.Context) {
	timeline, err := h.statsRepo.GetStatusCodeTimeline(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status code timeline"})
		return
	}
	c.JSON(http.StatusOK, timeline)
}

// GetTrafficHeatmap returns traffic heatmap data
func (h *DashboardHandler) GetTrafficHeatmap(c *gin.Context) {
	// Heatmap defaults to 30 days
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if val, err := strconv.Atoi(daysParam); err == nil && val > 0 {
			days = val
		}
	}

	heatmap, err := h.statsRepo.GetTrafficHeatmap(days, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic heatmap"})
		return
	}
	c.JSON(http.StatusOK, heatmap)
}

// GetTopPaths returns most accessed paths
func (h *DashboardHandler) GetTopPaths(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	paths, err := h.statsRepo.GetTopPaths(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top paths"})
		return
	}
	c.JSON(http.StatusOK, paths)
}

// GetTopCountries returns top countries
func (h *DashboardHandler) GetTopCountries(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	countries, err := h.statsRepo.GetTopCountries(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top countries"})
		return
	}
	c.JSON(http.StatusOK, countries)
}

// GetTopIPs returns most active IP addresses
func (h *DashboardHandler) GetTopIPs(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	tagFilter := c.Query("tag")
	ipFilter := &repositories.IPStatsFilter{
		Country:    c.Query("country"),
		DeviceType: c.Query("device_type"),
		Sort:       c.DefaultQuery("sort", "hits"),
	}
	if asnParam := c.Query("asn"); asnParam != "" {
		if val, err := strconv.Atoi(asnParam); err == nil && val > 0 {
			ipFilter.ASN = val
		}
	}
	if dayParam := c.Query("day_of_week"); dayParam != "" {
		if val, err := strconv.Atoi(dayParam); err == nil && val >= 0 && val <= 6 {
			ipFilter.DayOfWeek = &val
		}
	}
	if hourParam := c.Query("hour"); hourParam != "" {
		if val, err := strconv.Atoi(hourParam); err == nil && val >= 0 && val <= 23 {
			ipFilter.Hour = &val
		}
	}
	if ipFilter.Country == "" && ipFilter.DeviceType == "" && ipFilter.ASN == 0 && ipFilter.DayOfWeek == nil && ipFilter.Hour == nil && ipFilter.Sort == "hits" {
		ipFilter = nil
	}

	ips, err := h.statsRepo.GetTopIPAddresses(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c), tagFilter, ipFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top IPs"})
		return
	}
	c.JSON(http.StatusOK, ips)
}

// GetStatusCodeDistribution returns status code distribution
func (h *DashboardHandler) GetStatusCodeDistribution(c *gin.Context) {
	stats, err := h.statsRepo.GetStatusCodeDistribution(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status code distribution"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetMethodDistribution returns HTTP method distribution
func (h *DashboardHandler) GetMethodDistribution(c *gin.Context) {
	stats, err := h.statsRepo.GetMethodDistribution(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get method distribution"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetProtocolDistribution returns HTTP protocol distribution
func (h *DashboardHandler) GetProtocolDistribution(c *gin.Context) {
	stats, err := h.statsRepo.GetProtocolDistribution(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get protocol distribution"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetTLSVersionDistribution returns TLS version distribution
func (h *DashboardHandler) GetTLSVersionDistribution(c *gin.Context) {
	stats, err := h.statsRepo.GetTLSVersionDistribution(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get TLS version distribution"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetTopUserAgents returns most common user agents
func (h *DashboardHandler) GetTopUserAgents(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	agents, err := h.statsRepo.GetTopUserAgents(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top user agents"})
		return
	}
	c.JSON(http.StatusOK, agents)
}

// GetTopBrowsers returns most common browsers
func (h *DashboardHandler) GetTopBrowsers(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	browsers, err := h.statsRepo.GetTopBrowsers(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top browsers"})
		return
	}
	c.JSON(http.StatusOK, browsers)
}

// GetTopOperatingSystems returns most common operating systems
func (h *DashboardHandler) GetTopOperatingSystems(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	osList, err := h.statsRepo.GetTopOperatingSystems(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top operating systems"})
		return
	}
	c.JSON(http.StatusOK, osList)
}

// GetDeviceTypeDistribution returns distribution of device types
func (h *DashboardHandler) GetDeviceTypeDistribution(c *gin.Context) {
	stats, err := h.statsRepo.GetDeviceTypeDistribution(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get device type distribution"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetTopASNs returns top ASNs
func (h *DashboardHandler) GetTopASNs(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	asns, err := h.statsRepo.GetTopASNs(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top ASNs"})
		return
	}
	c.JSON(http.StatusOK, asns)
}

// GetTopBackends returns backend statistics
func (h *DashboardHandler) GetTopBackends(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	backends, err := h.statsRepo.GetTopBackends(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top backends"})
		return
	}
	c.JSON(http.StatusOK, backends)
}

// GetTopReferrers returns top referrers
func (h *DashboardHandler) GetTopReferrers(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	referrers, err := h.statsRepo.GetTopReferrers(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top referrers"})
		return
	}
	c.JSON(http.StatusOK, referrers)
}

// GetTopReferrerDomains returns top referrer domains
func (h *DashboardHandler) GetTopReferrerDomains(c *gin.Context) {
	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	domains, err := h.statsRepo.GetTopReferrerDomains(h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top referrer domains"})
		return
	}
	c.JSON(http.StatusOK, domains)
}

// GetResponseTimeStats returns response time statistics
func (h *DashboardHandler) GetResponseTimeStats(c *gin.Context) {
	stats, err := h.statsRepo.GetResponseTimeStats(h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get response time stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetComparison returns multi-period analytics for comparison dashboards.
func (h *DashboardHandler) GetComparison(c *gin.Context) {
	var req comparisonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid comparison request"})
		return
	}
	if len(req.Periods) < 2 || len(req.Periods) > 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Comparison requires 2 to 4 periods"})
		return
	}
	for i := range req.Periods {
		if req.Periods[i].Label == "" {
			req.Periods[i].Label = "Period " + strconv.Itoa(i+1)
		}
		if !req.Periods[i].End.After(req.Periods[i].Start) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Each period must have an end after start"})
			return
		}
	}

	comparison, err := h.statsRepo.GetComparison(req.Periods, h.convertToRepoFilters(h.getServiceFilters(c)), h.buildExcludeIPFilter(c), req.TopLimit)
	if err != nil {
		h.logger.WithCaller().Error("Failed to get comparison", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comparison"})
		return
	}
	c.JSON(http.StatusOK, comparison)
}

// CreateComparisonSnapshot stores a precomputed comparison response for sharing.
func (h *DashboardHandler) CreateComparisonSnapshot(c *gin.Context) {
	var req createComparisonSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Payload) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid snapshot request"})
		return
	}

	snapshot, err := h.statsRepo.CreateComparisonSnapshot(h.getComparisonOwnerID(c), req.Title, string(req.Payload), req.ExpiresAt)
	if err != nil {
		h.logger.WithCaller().Error("Failed to create comparison snapshot", h.logger.Args("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create snapshot"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token":      snapshot.Token,
		"title":      snapshot.Title,
		"active":     snapshot.Active,
		"expires_at": snapshot.ExpiresAt,
		"created_at": snapshot.CreatedAt,
		"url":        "/compare/" + snapshot.Token,
	})
}

// GetComparisonSnapshot returns a stored snapshot if it is active and not expired.
func (h *DashboardHandler) GetComparisonSnapshot(c *gin.Context) {
	snapshot, err := h.statsRepo.GetComparisonSnapshot(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Snapshot not found"})
		return
	}
	if !snapshot.Active {
		c.JSON(http.StatusGone, gin.H{"error": "Snapshot is disabled"})
		return
	}
	if snapshot.ExpiresAt != nil && time.Now().After(*snapshot.ExpiresAt) {
		c.JSON(http.StatusGone, gin.H{"error": "Snapshot expired"})
		return
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(snapshot.Payload), &payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Snapshot payload is invalid"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      snapshot.Token,
		"title":      snapshot.Title,
		"active":     snapshot.Active,
		"expires_at": snapshot.ExpiresAt,
		"created_at": snapshot.CreatedAt,
		"payload":    payload,
	})
}

// ListComparisonSnapshots returns stored comparison links for management.
func (h *DashboardHandler) ListComparisonSnapshots(c *gin.Context) {
	snapshots, err := h.statsRepo.ListComparisonSnapshots(h.getComparisonOwnerID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list snapshots"})
		return
	}
	c.JSON(http.StatusOK, snapshots)
}

// UpdateComparisonSnapshot toggles activity or expiration for a snapshot.
func (h *DashboardHandler) UpdateComparisonSnapshot(c *gin.Context) {
	ownerID := h.getComparisonOwnerID(c)
	current, err := h.statsRepo.GetComparisonSnapshot(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Snapshot not found"})
		return
	}
	if current.OwnerID != ownerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the creator can manage this snapshot"})
		return
	}

	var req updateComparisonSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid snapshot update"})
		return
	}
	active := current.Active
	if req.Active != nil {
		active = *req.Active
	}
	expiresAt := current.ExpiresAt
	if req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt
	}

	snapshot, err := h.statsRepo.UpdateComparisonSnapshot(ownerID, current.Token, active, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update snapshot"})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

// DeleteComparisonSnapshot removes a stored comparison link.
func (h *DashboardHandler) DeleteComparisonSnapshot(c *gin.Context) {
	if err := h.statsRepo.DeleteComparisonSnapshot(h.getComparisonOwnerID(c), c.Param("token")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete snapshot"})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetRecentRequests returns recent HTTP requests with pagination and filters
func (h *DashboardHandler) GetRecentRequests(c *gin.Context) {
	limit := 50
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	offset := 0
	if offsetParam := c.Query("offset"); offsetParam != "" {
		if val, err := strconv.Atoi(offsetParam); err == nil && val >= 0 {
			offset = val
		}
	}

	service, serviceType := h.getServiceFilter(c)

	// Handle IP exclusion for request list
	_, clientIPs, excludeServices := h.getExcludeOwnIP(c)

	requests, err := h.requestRepo.FindAll(limit, offset, service, serviceType, clientIPs, h.convertToRepoFilters(excludeServices))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get requests"})
		return
	}

	c.JSON(http.StatusOK, requests)
}

// GetIPDetailedStats returns comprehensive statistics for a specific IP address
func (h *DashboardHandler) GetIPDetailedStats(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	stats, err := h.statsRepo.GetIPDetailedStats(ip, h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetIPTimeline returns timeline statistics for a specific IP
func (h *DashboardHandler) GetIPTimeline(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	timeline, err := h.statsRepo.GetIPTimelineStats(ip, h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP timeline"})
		return
	}
	c.JSON(http.StatusOK, timeline)
}

// GetIPHeatmap returns traffic heatmap data for a specific IP
func (h *DashboardHandler) GetIPHeatmap(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	// Heatmap defaults to 30 days
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if val, err := strconv.Atoi(daysParam); err == nil && val > 0 {
			days = val
		}
	}

	heatmap, err := h.statsRepo.GetIPTrafficHeatmap(ip, days, h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP traffic heatmap"})
		return
	}
	c.JSON(http.StatusOK, heatmap)
}

// GetIPTopPaths returns most accessed paths for a specific IP
func (h *DashboardHandler) GetIPTopPaths(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	paths, err := h.statsRepo.GetIPTopPaths(ip, h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP top paths"})
		return
	}
	c.JSON(http.StatusOK, paths)
}

// GetIPTopBackends returns backend statistics for a specific IP
func (h *DashboardHandler) GetIPTopBackends(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	backends, err := h.statsRepo.GetIPTopBackends(ip, h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
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

	stats, err := h.statsRepo.GetIPStatusCodeDistribution(ip, h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP status code distribution"})
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
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	browsers, err := h.statsRepo.GetIPTopBrowsers(ip, h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
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
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	osList, err := h.statsRepo.GetIPTopOperatingSystems(ip, h.getHours(c), limit, h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
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

	stats, err := h.statsRepo.GetIPDeviceTypeDistribution(ip, h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP device type distribution"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetIPResponseTimeStats returns response time statistics for a specific IP
func (h *DashboardHandler) GetIPResponseTimeStats(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	stats, err := h.statsRepo.GetIPResponseTimeStats(ip, h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP response time stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetIPRecentRequests returns recent HTTP requests for a specific IP
func (h *DashboardHandler) GetIPRecentRequests(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP address is required"})
		return
	}

	limit := 20
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	requests, err := h.statsRepo.GetIPRecentRequests(ip, limit, h.getHours(c), h.convertToRepoFilters(h.getServiceFilters(c)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IP recent requests"})
		return
	}
	c.JSON(http.StatusOK, requests)
}

// SearchIPs searches for IP addresses
func (h *DashboardHandler) SearchIPs(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	limit := 10
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.Atoi(limitParam); err == nil && val > 0 {
			limit = val
		}
	}

	results, err := h.statsRepo.SearchIPs(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search IPs"})
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetLogProcessingStats returns log processing metrics
func (h *DashboardHandler) GetLogProcessingStats(c *gin.Context) {
	stats, err := h.statsRepo.GetLogProcessingStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get log processing stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetDomains returns all unique domains
func (h *DashboardHandler) GetDomains(c *gin.Context) {
	domains, err := h.statsRepo.GetDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get domains"})
		return
	}
	c.JSON(http.StatusOK, domains)
}

// GetServices returns all unique services
func (h *DashboardHandler) GetServices(c *gin.Context) {
	services, err := h.statsRepo.GetServices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get services"})
		return
	}
	c.JSON(http.StatusOK, services)
}

// GetSystemStats returns system-wide metrics (not filtered by host)
func (h *DashboardHandler) GetSystemStats(c *gin.Context) {
	// This is used for system health/cleanup stats
	// We'll return record count and time range
	count, err := h.requestRepo.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count records"})
		return
	}

	oldest, newest, err := h.statsRepo.GetRecordTimeRange()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get time range"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_records": count,
		"oldest_record": oldest,
		"newest_record": newest,
	})
}

// GetRecordsTimeline returns records count timeline for system stats
func (h *DashboardHandler) GetRecordsTimeline(c *gin.Context) {
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if val, err := strconv.Atoi(daysParam); err == nil && val > 0 {
			days = val
		}
	}

	timeline, err := h.statsRepo.GetRecordsTimeline(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get records timeline"})
		return
	}
	c.JSON(http.StatusOK, timeline)
}
