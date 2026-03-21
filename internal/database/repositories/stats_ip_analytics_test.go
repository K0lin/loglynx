package repositories

import (
	"testing"
	"time"

	"loglynx/internal/database/models"

	"github.com/pterm/pterm"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, StatsRepository) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	err = db.AutoMigrate(&models.HTTPRequest{}, &models.IPTag{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	repo := NewStatsRepository(db, pterm.DefaultLogger)
	return db, repo
}

func TestIPAnalyticsFiltering(t *testing.T) {
	db, repo := setupTestDB(t)
	ip := "1.2.3.4"
	now := time.Now()

	// Seed data
	requests := []models.HTTPRequest{
		// Recent, Service A
		{ClientIP: ip, Timestamp: now.Add(-1 * time.Hour), BackendName: "svc-a", StatusCode: 200, ResponseSize: 100},
		// Recent, Service B
		{ClientIP: ip, Timestamp: now.Add(-2 * time.Hour), BackendName: "svc-b", StatusCode: 200, ResponseSize: 200},
		// Old (10 days ago), Service A
		{ClientIP: ip, Timestamp: now.Add(-240 * time.Hour), BackendName: "svc-a", StatusCode: 200, ResponseSize: 400},
	}
	db.Create(&requests)

	t.Run("GetIPTimelineStats honors hours", func(t *testing.T) {
		// Last 24 hours should only have 2 requests
		timeline, err := repo.GetIPTimelineStats(ip, 24, []ServiceFilter{})
		assert.NoError(t, err)
		
		totalRequests := int64(0)
		for _, p := range timeline {
			totalRequests += p.Requests
		}
		assert.Equal(t, int64(2), totalRequests)

		// All time (hours=0) should have 3 requests
		timelineAll, err := repo.GetIPTimelineStats(ip, 0, []ServiceFilter{})
		assert.NoError(t, err)
		
		totalRequestsAll := int64(0)
		for _, p := range timelineAll {
			totalRequestsAll += p.Requests
		}
		assert.Equal(t, int64(3), totalRequestsAll)
	})

	t.Run("GetIPTimelineStats honors service filters", func(t *testing.T) {
		// All time, but only Service A
		filters := []ServiceFilter{{Name: "svc-a", Type: "backend_name"}}
		timeline, err := repo.GetIPTimelineStats(ip, 0, filters)
		assert.NoError(t, err)
		
		totalRequests := int64(0)
		for _, p := range timeline {
			totalRequests += p.Requests
		}
		assert.Equal(t, int64(2), totalRequests) // 1 recent + 1 old
	})

    t.Run("GetIPDetailedStats honors timeframe and filters", func(t *testing.T) {
        // Last 24 hours, all services
        stats, err := repo.GetIPDetailedStats(ip, 24, []ServiceFilter{})
        assert.NoError(t, err)
        assert.Equal(t, int64(2), stats.TotalRequests)

        // Last 24 hours, only svc-b
        filters := []ServiceFilter{{Name: "svc-b", Type: "backend_name"}}
        statsB, err := repo.GetIPDetailedStats(ip, 24, filters)
        assert.NoError(t, err)
        assert.Equal(t, int64(1), statsB.TotalRequests)
    })
}
