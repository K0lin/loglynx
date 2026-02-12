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
package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// Database Configuration
	Database DatabaseConfig

	// GeoIP Configuration
	GeoIP GeoIPConfig

	// Log configuration
	LogLevel string

	// Log Sources Configuration
	LogSources LogSourcesConfig

	// Server Configuration
	Server ServerConfig

	// Performance Configuration
	Performance PerformanceConfig
}

// DatabaseConfig contains database-related settings
type DatabaseConfig struct {
	Path            string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLife     time.Duration
	RetentionDays   int           // Number of days to retain data (0 = unlimited)
	CleanupInterval time.Duration // How often to check for cleanup (default: 1 hour)
	CleanupTime     string        // Time of day to run cleanup (24-hour format, e.g., "02:00")
	VacuumEnabled   bool          // Run VACUUM after cleanup to reclaim space

	// Connection Pool Monitoring
	PoolMonitoringEnabled   bool          // Enable connection pool monitoring
	PoolMonitoringInterval  time.Duration // How often to check pool stats
	PoolSaturationThreshold float64       // Alert threshold (0.0-1.0, default: 0.85)
	AutoTuning              bool          // Enable auto-tuning based on CPU cores
}

// GeoIPConfig contains GeoIP database paths
type GeoIPConfig struct {
	CityDBPath    string
	CountryDBPath string
	ASNDBPath     string
	Enabled       bool
}

// LogSourcesConfig contains log source paths
type LogSourcesConfig struct {
	TraefikLogPath      string
	TraefikLogFormat    string // auto, json, clf
	CaddyLogPath        string
	AutoDiscover        bool
	InitialImportDays   int  // Only import last N days on first run (0 = import all)
	InitialImportEnable bool // Enable initial import limiting
}

// ServerConfig contains web server settings
type ServerConfig struct {
	Host                string
	Port                int
	Production          bool
	DashboardEnabled    bool   // If false, only API routes are exposed
	SplashScreenEnabled bool   // If false, splash screen is disabled on startup
	TimeZone            string // Dashboard timezone (e.g., "UTC")
	WidgetEnabled       bool   // If false, widget page and API endpoints are disabled
}

// PerformanceConfig contains performance tuning settings
type PerformanceConfig struct {
	RealtimeMetricsInterval time.Duration
	GeoIPCacheSize          int
	BatchSize               int
	WorkerPoolSize          int
}

// Load reads configuration from .env file and environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Database: DatabaseConfig{
			Path:            getEnv("DB_PATH", "loglynx.db"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLife:     getEnvAsDuration("DB_CONN_MAX_LIFE", time.Hour),
			RetentionDays:   getEnvAsInt("DB_RETENTION_DAYS", 60),
			CleanupInterval: getEnvAsDuration("DB_CLEANUP_INTERVAL", 1*time.Hour),
			CleanupTime:     getEnv("DB_CLEANUP_TIME", "02:00"),
			VacuumEnabled:   getEnvAsBool("DB_VACUUM_ENABLED", true),

			// Connection Pool Monitoring
			PoolMonitoringEnabled:   getEnvAsBool("DB_POOL_MONITORING", true),
			PoolMonitoringInterval:  getEnvAsDuration("DB_POOL_MONITOR_INTERVAL", 30*time.Second),
			PoolSaturationThreshold: getEnvAsFloat("DB_POOL_SATURATION_THRESHOLD", 0.85),
			AutoTuning:              getEnvAsBool("DB_AUTO_TUNING", true),
		},
		GeoIP: GeoIPConfig{
			CityDBPath:    getEnv("GEOIP_CITY_DB", "geoip/GeoLite2-City.mmdb"),
			CountryDBPath: getEnv("GEOIP_COUNTRY_DB", "geoip/GeoLite2-Country.mmdb"),
			ASNDBPath:     getEnv("GEOIP_ASN_DB", "geoip/GeoLite2-ASN.mmdb"),
			Enabled:       getEnvAsBool("GEOIP_ENABLED", true),
		},
		LogSources: LogSourcesConfig{
			TraefikLogPath:      getEnv("TRAEFIK_LOG_PATH", "traefik/logs/access.log"),
			TraefikLogFormat:    getEnv("TRAEFIK_LOG_FORMAT", "auto"),
			CaddyLogPath:        getEnv("CADDY_LOG_PATH", "caddy/logs/access.log"),
			AutoDiscover:        getEnvAsBool("LOG_AUTO_DISCOVER", true),
			InitialImportDays:   getEnvAsInt("INITIAL_IMPORT_DAYS", 60),
			InitialImportEnable: getEnvAsBool("INITIAL_IMPORT_ENABLE", true),
		},
		Server: ServerConfig{
			Host:                getEnv("SERVER_HOST", "0.0.0.0"),
			Port:                getEnvAsInt("SERVER_PORT", 8080),
			Production:          getEnvAsBool("SERVER_PRODUCTION", false),
			DashboardEnabled:    getEnvAsBool("DASHBOARD_ENABLED", true),
			SplashScreenEnabled: getEnvAsBool("SPLASH_SCREEN_ENABLED", true),
			TimeZone:            getEnv("TIMEZONE", "UTC"),
			WidgetEnabled:       getEnvAsBool("WIDGET_ENABLED", false),

		},
		Performance: PerformanceConfig{
			RealtimeMetricsInterval: getEnvAsDuration("METRICS_INTERVAL", 1*time.Second),
			GeoIPCacheSize:          getEnvAsInt("GEOIP_CACHE_SIZE", 10000),
			BatchSize:               getEnvAsInt("BATCH_SIZE", 1000),
			WorkerPoolSize:          getEnvAsInt("WORKER_POOL_SIZE", 4),
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	return cfg, nil
}

// Helper functions to read environment variables with defaults

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return value
	}
	return defaultValue
}
