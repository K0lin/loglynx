package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"loglynx/internal/database/models"
	"loglynx/internal/database/repositories"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStatsRepository is a mock implementation of repositories.StatsRepository
type MockStatsRepository struct {
	mock.Mock
}

func (m *MockStatsRepository) GetSummary(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) (*repositories.StatsSummary, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).(*repositories.StatsSummary), args.Error(1)
}

func (m *MockStatsRepository) GetTimelineStats(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.TimelineData, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).([]*repositories.TimelineData), args.Error(1)
}

func (m *MockStatsRepository) GetStatusCodeTimeline(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.StatusCodeTimelineData, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).([]*repositories.StatusCodeTimelineData), args.Error(1)
}

func (m *MockStatsRepository) GetTrafficHeatmap(days int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.TrafficHeatmapData, error) {
	args := m.Called(days, filters, excludeIP)
	return args.Get(0).([]*repositories.TrafficHeatmapData), args.Error(1)
}

func (m *MockStatsRepository) GetTopPaths(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.PathStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.PathStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopCountries(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.CountryStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.CountryStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopIPAddresses(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter, tagFilter string, ipFilter *repositories.IPStatsFilter) ([]*repositories.IPStats, error) {
	args := m.Called(hours, limit, filters, excludeIP, tagFilter, ipFilter)
	return args.Get(0).([]*repositories.IPStats), args.Error(1)
}

func (m *MockStatsRepository) GetStatusCodeDistribution(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.StatusCodeStats, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).([]*repositories.StatusCodeStats), args.Error(1)
}

func (m *MockStatsRepository) GetMethodDistribution(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.MethodStats, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).([]*repositories.MethodStats), args.Error(1)
}

func (m *MockStatsRepository) GetProtocolDistribution(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.ProtocolStats, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).([]*repositories.ProtocolStats), args.Error(1)
}

func (m *MockStatsRepository) GetTLSVersionDistribution(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.TLSVersionStats, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).([]*repositories.TLSVersionStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopUserAgents(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.UserAgentStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.UserAgentStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopBrowsers(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.BrowserStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.BrowserStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopOperatingSystems(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.OSStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.OSStats), args.Error(1)
}

func (m *MockStatsRepository) GetDeviceTypeDistribution(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.DeviceTypeStats, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).([]*repositories.DeviceTypeStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopASNs(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.ASNStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.ASNStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopBackends(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.BackendStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.BackendStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopReferrers(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.ReferrerStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.ReferrerStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopReferrerDomains(hours int, limit int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) ([]*repositories.ReferrerDomainStats, error) {
	args := m.Called(hours, limit, filters, excludeIP)
	return args.Get(0).([]*repositories.ReferrerDomainStats), args.Error(1)
}

func (m *MockStatsRepository) GetResponseTimeStats(hours int, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter) (*repositories.ResponseTimeStats, error) {
	args := m.Called(hours, filters, excludeIP)
	return args.Get(0).(*repositories.ResponseTimeStats), args.Error(1)
}

func (m *MockStatsRepository) GetComparison(periods []repositories.ComparisonPeriodRequest, filters []repositories.ServiceFilter, excludeIP *repositories.ExcludeIPFilter, topLimit int) (*repositories.ComparisonResult, error) {
	args := m.Called(periods, filters, excludeIP, topLimit)
	return args.Get(0).(*repositories.ComparisonResult), args.Error(1)
}

func (m *MockStatsRepository) CreateComparisonSnapshot(ownerID string, title string, payload string, expiresAt *time.Time) (*models.ComparisonSnapshot, error) {
	args := m.Called(ownerID, title, payload, expiresAt)
	return args.Get(0).(*models.ComparisonSnapshot), args.Error(1)
}

func (m *MockStatsRepository) GetComparisonSnapshot(token string) (*models.ComparisonSnapshot, error) {
	args := m.Called(token)
	return args.Get(0).(*models.ComparisonSnapshot), args.Error(1)
}

func (m *MockStatsRepository) ListComparisonSnapshots(ownerID string) ([]*models.ComparisonSnapshot, error) {
	args := m.Called(ownerID)
	return args.Get(0).([]*models.ComparisonSnapshot), args.Error(1)
}

func (m *MockStatsRepository) UpdateComparisonSnapshot(ownerID string, token string, active bool, expiresAt *time.Time) (*models.ComparisonSnapshot, error) {
	args := m.Called(ownerID, token, active, expiresAt)
	return args.Get(0).(*models.ComparisonSnapshot), args.Error(1)
}

func (m *MockStatsRepository) DeleteComparisonSnapshot(ownerID string, token string) error {
	args := m.Called(ownerID, token)
	return args.Error(0)
}

func (m *MockStatsRepository) GetLogProcessingStats() ([]*repositories.LogProcessingStats, error) {
	args := m.Called()
	return args.Get(0).([]*repositories.LogProcessingStats), args.Error(1)
}

func (m *MockStatsRepository) GetDomains() ([]*repositories.DomainStats, error) {
	args := m.Called()
	return args.Get(0).([]*repositories.DomainStats), args.Error(1)
}

func (m *MockStatsRepository) GetServices() ([]*repositories.ServiceInfo, error) {
	args := m.Called()
	return args.Get(0).([]*repositories.ServiceInfo), args.Error(1)
}

// IP-specific analytics (The ones we are updating)

func (m *MockStatsRepository) GetIPDetailedStats(ip string, hours int, filters []repositories.ServiceFilter) (*repositories.IPDetailedStats, error) {
	args := m.Called(ip, hours, filters)
	return args.Get(0).(*repositories.IPDetailedStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPTimelineStats(ip string, hours int, filters []repositories.ServiceFilter) ([]*repositories.TimelineData, error) {
	args := m.Called(ip, hours, filters)
	return args.Get(0).([]*repositories.TimelineData), args.Error(1)
}

func (m *MockStatsRepository) GetIPTrafficHeatmap(ip string, days int, filters []repositories.ServiceFilter) ([]*repositories.TrafficHeatmapData, error) {
	args := m.Called(ip, days, filters)
	return args.Get(0).([]*repositories.TrafficHeatmapData), args.Error(1)
}

func (m *MockStatsRepository) GetIPTopPaths(ip string, hours int, limit int, filters []repositories.ServiceFilter) ([]*repositories.PathStats, error) {
	args := m.Called(ip, hours, limit, filters)
	return args.Get(0).([]*repositories.PathStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPTopBackends(ip string, hours int, limit int, filters []repositories.ServiceFilter) ([]*repositories.BackendStats, error) {
	args := m.Called(ip, hours, limit, filters)
	return args.Get(0).([]*repositories.BackendStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPStatusCodeDistribution(ip string, hours int, filters []repositories.ServiceFilter) ([]*repositories.StatusCodeStats, error) {
	args := m.Called(ip, hours, filters)
	return args.Get(0).([]*repositories.StatusCodeStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPTopBrowsers(ip string, hours int, limit int, filters []repositories.ServiceFilter) ([]*repositories.BrowserStats, error) {
	args := m.Called(ip, hours, limit, filters)
	return args.Get(0).([]*repositories.BrowserStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPTopOperatingSystems(ip string, hours int, limit int, filters []repositories.ServiceFilter) ([]*repositories.OSStats, error) {
	args := m.Called(ip, hours, limit, filters)
	return args.Get(0).([]*repositories.OSStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPDeviceTypeDistribution(ip string, hours int, filters []repositories.ServiceFilter) ([]*repositories.DeviceTypeStats, error) {
	args := m.Called(ip, hours, filters)
	return args.Get(0).([]*repositories.DeviceTypeStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPResponseTimeStats(ip string, hours int, filters []repositories.ServiceFilter) (*repositories.ResponseTimeStats, error) {
	args := m.Called(ip, hours, filters)
	return args.Get(0).(*repositories.ResponseTimeStats), args.Error(1)
}

func (m *MockStatsRepository) GetIPRecentRequests(ip string, limit int, hours int, filters []repositories.ServiceFilter) ([]*models.HTTPRequest, error) {
	args := m.Called(ip, limit, hours, filters)
	return args.Get(0).([]*models.HTTPRequest), args.Error(1)
}

func (m *MockStatsRepository) SearchIPs(query string, limit int) ([]*repositories.IPSearchResult, error) {
	args := m.Called(query, limit)
	return args.Get(0).([]*repositories.IPSearchResult), args.Error(1)
}

func (m *MockStatsRepository) CountRecordsOlderThan(cutoffDate time.Time) (int64, error) {
	args := m.Called(cutoffDate)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStatsRepository) GetRecordTimeRange() (time.Time, time.Time, error) {
	args := m.Called()
	return args.Get(0).(time.Time), args.Get(1).(time.Time), args.Error(2)
}

func (m *MockStatsRepository) GetRecordsTimeline(days int) ([]*repositories.TimelineData, error) {
	args := m.Called(days)
	return args.Get(0).([]*repositories.TimelineData), args.Error(1)
}

func TestIPAnalyticsHoursAndScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockRepo := new(MockStatsRepository)
	logger := pterm.DefaultLogger
	handler := NewDashboardHandler(mockRepo, nil, &logger)

	t.Run("GetIPTimeline forwards hours and scope", func(t *testing.T) {
		ip := "1.2.3.4"
		expectedHours := 48
		expectedFilters := []repositories.ServiceFilter{
			{Name: "svc1", Type: "backend_name"},
		}

		mockRepo.On("GetIPTimelineStats", ip, expectedHours, expectedFilters).Return([]*repositories.TimelineData{}, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{{Key: "ip", Value: ip}}
		c.Request, _ = http.NewRequest("GET", "/api/v1/ip/1.2.3.4/timeline?hours=48&services[]=svc1&service_types[]=backend_name", nil)

		handler.GetIPTimeline(c)

		assert.Equal(t, http.StatusOK, w.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetIPTimeline accepts hours=0 for all-time", func(t *testing.T) {
		ip := "1.2.3.4"
		expectedHours := 0
		var expectedFilters []repositories.ServiceFilter

		mockRepo.On("GetIPTimelineStats", ip, expectedHours, expectedFilters).Return([]*repositories.TimelineData{}, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{{Key: "ip", Value: ip}}
		c.Request, _ = http.NewRequest("GET", "/api/v1/ip/1.2.3.4/timeline?hours=0", nil)

		handler.GetIPTimeline(c)

		assert.Equal(t, http.StatusOK, w.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetIPDetailedStats forwards hours and scope", func(t *testing.T) {
		ip := "1.2.3.4"
		expectedHours := 720 // 30 days
		expectedFilters := []repositories.ServiceFilter{
			{Name: "svc2", Type: "host"},
		}

		mockRepo.On("GetIPDetailedStats", ip, expectedHours, expectedFilters).Return(&repositories.IPDetailedStats{}, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{{Key: "ip", Value: ip}}
		c.Request, _ = http.NewRequest("GET", "/api/v1/ip/1.2.3.4/stats?hours=720&service=svc2&service_type=host", nil)

		handler.GetIPDetailedStats(c)

		assert.Equal(t, http.StatusOK, w.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Hours are clamped to 8760", func(t *testing.T) {
		ip := "1.2.3.4"
		expectedHours := 8760
		var expectedFilters []repositories.ServiceFilter

		mockRepo.On("GetIPTimelineStats", ip, expectedHours, expectedFilters).Return([]*repositories.TimelineData{}, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{{Key: "ip", Value: ip}}
		c.Request, _ = http.NewRequest("GET", "/api/v1/ip/1.2.3.4/timeline?hours=999999", nil)

		handler.GetIPTimeline(c)

		assert.Equal(t, http.StatusOK, w.Code)
		mockRepo.AssertExpectations(t)
	})
}
