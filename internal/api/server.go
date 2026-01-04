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
//
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"loglynx/internal/api/handlers"
	"loglynx/internal/version"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// Server represents the HTTP server
type Server struct {
	router              *gin.Engine
	server              *http.Server
	logger              *pterm.Logger
	port                int
	splashScreenEnabled bool
}

// Config holds server configuration
type Config struct {
	Host                string
	Port                int
	Production          bool
	DashboardEnabled    bool // If false, only API routes are exposed
	SplashScreenEnabled bool // If false, splash screen is disabled on startup
}

// NewServer creates a new HTTP server
func NewServer(cfg *Config, dashboardHandler *handlers.DashboardHandler, realtimeHandler *handlers.RealtimeHandler, systemHandler *handlers.SystemHandler, logger *pterm.Logger) *Server {
	// Set Gin mode
	if cfg.Production {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now(),
			"version":   version.Version,
		})
	})

	// Helper function to render pages with common config
	splashScreenEnabled := cfg.SplashScreenEnabled
	renderPage := func(c *gin.Context, pageName, pageTitle, pageIcon string) {
		c.HTML(http.StatusOK, pageName+".html", gin.H{
			"Title":               pageTitle,
			"PageName":            pageName,
			"PageTitle":           pageTitle,
			"PageIcon":            pageIcon,
			"AppVersion":          version.Version,
			"SplashScreenEnabled": splashScreenEnabled,
		})
	}

	// Dashboard UI routes (only if dashboard is enabled)
	if cfg.DashboardEnabled {
		// Load HTML templates with pattern for nested directories
		router.LoadHTMLGlob("web/templates/**/*.html")

		// Static files
		router.Static("/static", "./web/static")

		// Dashboard pages (HTML)
		router.GET("/", func(c *gin.Context) {
			renderPage(c, "overview", "Executive Overview", "fas fa-home")
		})

		router.GET("/realtime", func(c *gin.Context) {
			renderPage(c, "realtime", "Real-time Monitor", "fas fa-broadcast-tower")
		})

		router.GET("/traffic", func(c *gin.Context) {
			renderPage(c, "traffic", "Traffic Analysis", "fas fa-globe")
		})

		router.GET("/performance", func(c *gin.Context) {
			renderPage(c, "performance", "Performance Monitoring", "fas fa-tachometer-alt")
		})

		router.GET("/security", func(c *gin.Context) {
			renderPage(c, "security", "Security & Network", "fas fa-shield-alt")
		})

		router.GET("/users", func(c *gin.Context) {
			renderPage(c, "users", "User Analytics", "fas fa-users")
		})

		router.GET("/content", func(c *gin.Context) {
			renderPage(c, "content", "Content Analytics", "fas fa-file-alt")
		})

		router.GET("/backends", func(c *gin.Context) {
			renderPage(c, "backends", "Backend Health", "fas fa-server")
		})

		router.GET("/geographic", func(c *gin.Context) {
			renderPage(c, "geographic", "Geographic Analytics", "fas fa-map-marked-alt")
		})

		router.GET("/system", func(c *gin.Context) {
			renderPage(c, "system", "System Statistics", "fas fa-server")
		})

		// IP Analytics page
		router.GET("/ip/:ip", func(c *gin.Context) {
			ip := c.Param("ip")
			c.HTML(http.StatusOK, "ip-detail.html", gin.H{
				"Title":               "IP Analytics - " + ip,
				"PageName":            "ip-detail",
				"PageTitle":           "IP Analytics",
				"PageIcon":            "fas fa-network-wired",
				"AppVersion":          version.Version,
				"IPAddress":           ip,
				"SplashScreenEnabled": splashScreenEnabled,
			})
		})

		logger.Info("Dashboard UI routes enabled")
	} else {
		logger.Info("Dashboard UI disabled - API-only mode")
		// Serve a simple message at root when dashboard is disabled
		router.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "LogLynx API Server - Dashboard UI is disabled",
				"api":     "/api/v1",
				"health":  "/health",
				"version": version.Version,
			})
		})
	}

	// API routes
	api := router.Group("/api/v1")
	{
		api.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"version": version.Version})
		})

		// Summary stats
		api.GET("/stats/summary", dashboardHandler.GetSummary)

		// Timeline data
		api.GET("/stats/timeline", dashboardHandler.GetTimeline)
		api.GET("/stats/timeline/status-codes", dashboardHandler.GetStatusCodeTimeline)
		api.GET("/stats/heatmap/traffic", dashboardHandler.GetTrafficHeatmap)

		// Top stats
		api.GET("/stats/top/paths", dashboardHandler.GetTopPaths)
		api.GET("/stats/top/countries", dashboardHandler.GetTopCountries)
		api.GET("/stats/top/ips", dashboardHandler.GetTopIPs)
		api.GET("/stats/top/user-agents", dashboardHandler.GetTopUserAgents)
		api.GET("/stats/top/browsers", dashboardHandler.GetTopBrowsers)
		api.GET("/stats/top/operating-systems", dashboardHandler.GetTopOperatingSystems)
		api.GET("/stats/top/asns", dashboardHandler.GetTopASNs)
		api.GET("/stats/top/backends", dashboardHandler.GetTopBackends)
		api.GET("/stats/top/referrers", dashboardHandler.GetTopReferrers)
		api.GET("/stats/top/referrer-domains", dashboardHandler.GetTopReferrerDomains)

		// Distribution stats
		api.GET("/stats/distribution/status-codes", dashboardHandler.GetStatusCodeDistribution)
		api.GET("/stats/distribution/methods", dashboardHandler.GetMethodDistribution)
		api.GET("/stats/distribution/protocols", dashboardHandler.GetProtocolDistribution)
		api.GET("/stats/distribution/tls-versions", dashboardHandler.GetTLSVersionDistribution)
		api.GET("/stats/distribution/device-types", dashboardHandler.GetDeviceTypeDistribution)

		// Performance stats
		api.GET("/stats/performance/response-time", dashboardHandler.GetResponseTimeStats)
		api.GET("/stats/log-processing", dashboardHandler.GetLogProcessingStats)

		// Recent requests
		api.GET("/requests/recent", dashboardHandler.GetRecentRequests)

		// Real-time metrics
		api.GET("/realtime/metrics", realtimeHandler.GetCurrentMetrics)
		api.GET("/realtime/stream", realtimeHandler.StreamMetrics)
		api.GET("/realtime/services", realtimeHandler.GetPerServiceMetrics)

		// Domains list (deprecated)
		api.GET("/domains", dashboardHandler.GetDomains)

		// Services list (with types)
		api.GET("/services", dashboardHandler.GetServices)

		// IP Analytics
		api.GET("/ip/:ip/stats", dashboardHandler.GetIPDetailedStats)
		api.GET("/ip/:ip/timeline", dashboardHandler.GetIPTimeline)
		api.GET("/ip/:ip/heatmap", dashboardHandler.GetIPHeatmap)
		api.GET("/ip/:ip/top/paths", dashboardHandler.GetIPTopPaths)
		api.GET("/ip/:ip/top/backends", dashboardHandler.GetIPTopBackends)
		api.GET("/ip/:ip/distribution/status-codes", dashboardHandler.GetIPStatusCodeDistribution)
		api.GET("/ip/:ip/top/browsers", dashboardHandler.GetIPTopBrowsers)
		api.GET("/ip/:ip/top/operating-systems", dashboardHandler.GetIPTopOperatingSystems)
		api.GET("/ip/:ip/distribution/device-types", dashboardHandler.GetIPDeviceTypeDistribution)
		api.GET("/ip/:ip/performance/response-time", dashboardHandler.GetIPResponseTimeStats)
		api.GET("/ip/:ip/recent-requests", dashboardHandler.GetIPRecentRequests)
		api.GET("/ip/search", dashboardHandler.SearchIPs)

		// System Statistics
		api.GET("/system/stats", systemHandler.GetSystemStats)
		api.GET("/system/timeline", systemHandler.GetRecordsTimeline)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return &Server{
		router: router,
		server: &http.Server{
			Addr:           addr,
			Handler:        router,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   300 * time.Second, // Long timeout for SSE streams
			MaxHeaderBytes: 1 << 20,
		},
		logger:              logger,
		port:                cfg.Port,
		splashScreenEnabled: cfg.SplashScreenEnabled,
	}
}

// Run starts the HTTP server
func (s *Server) Run() error {
	s.logger.Info("Starting web server", s.logger.Args("address", s.server.Addr))
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.WithCaller().Error("Web server failed", s.logger.Args("error", err))
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down web server...")
	return s.server.Shutdown(ctx)
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

