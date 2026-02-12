package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *DashboardHandler) GetWidgetData(c *gin.Context) {
	summary, err := h.statsRepo.GetSummary(1, nil, nil)
	if err != nil {
		h.logger.Debug("Widget data fetch error", h.logger.Args("error", err))
		c.JSON(http.StatusOK, gin.H{
			"status":              "error",
			"requests_per_minute": 0,
			"error_rate":          0,
			"avg_response_time":   0,
			"unique_ips":          0,
		})
		return
	}

	reqPerMin := 0.0
	if summary.TotalRequests > 0 {
		reqPerMin = float64(summary.TotalRequests) / 60.0
	}

	errorRate := summary.ServerErrorRate + summary.NotFoundRate
	status := "healthy"
	if errorRate > 5 {
		status = "danger"
	} else if errorRate > 1 {
		status = "warning"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":              status,
		"requests_per_minute": reqPerMin,
		"error_rate":          errorRate,
		"avg_response_time":   summary.AvgResponseTime,
		"unique_ips":          summary.UniqueVisitors,
	})
}

func (h *DashboardHandler) GetWidgetSummary(c *gin.Context) {
	hours := 24
	if h := c.Query("hours"); h != "" {
		if val, err := strconv.Atoi(h); err == nil && val > 0 && val <= 720 {
			hours = val
		}
	}

	summary, err := h.statsRepo.GetSummary(hours, nil, nil)
	if err != nil {
		h.logger.Debug("Widget summary fetch error", h.logger.Args("error", err))
		c.JSON(http.StatusOK, gin.H{
			"status":          "error",
			"total_requests":  0,
			"requests_per_hr": 0,
			"error_rate":      0,
			"avg_response_ms": 0,
			"unique_ips":      0,
			"bandwidth_mb":    0,
		})
		return
	}

	errorRate := summary.ServerErrorRate + summary.NotFoundRate
	status := "healthy"
	if errorRate > 5 {
		status = "danger"
	} else if errorRate > 1 {
		status = "warning"
	}

	reqPerHr := 0.0
	if summary.TotalRequests > 0 && hours > 0 {
		reqPerHr = float64(summary.TotalRequests) / float64(hours)
	}

	bandwidthMB := float64(summary.TotalBandwidth) / (1024 * 1024)

	c.JSON(http.StatusOK, gin.H{
		"status":          status,
		"total_requests":  summary.TotalRequests,
		"requests_per_hr": reqPerHr,
		"error_rate":      errorRate,
		"avg_response_ms": summary.AvgResponseTime,
		"unique_ips":      summary.UniqueVisitors,
		"bandwidth_mb":    bandwidthMB,
	})
}

func (h *DashboardHandler) GetWidgetTimeline(c *gin.Context) {
	hours := 1
	if h := c.Query("hours"); h != "" {
		if val, err := strconv.Atoi(h); err == nil && val > 0 && val <= 720 {
			hours = val
		}
	}

	timeline, err := h.statsRepo.GetTimelineStats(hours, nil, nil)
	if err != nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	points := len(timeline)
	maxPoints := 30
	if hours > 24 {
		maxPoints = 50
	}
	if points > maxPoints {
		timeline = timeline[points-maxPoints:]
	}

	result := make([]gin.H, len(timeline))
	for i, t := range timeline {
		result[i] = gin.H{
			"hour":     t.Hour,
			"requests": t.Requests,
		}
	}

	c.JSON(http.StatusOK, result)
}
