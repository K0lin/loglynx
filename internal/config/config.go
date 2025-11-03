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

	// Log Sources Configuration
	LogSources LogSourcesConfig

	// Server Configuration
	Server ServerConfig

	// Performance Configuration
	Performance PerformanceConfig
}

// DatabaseConfig contains database-related settings
type DatabaseConfig struct {
	Path         string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
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
	TraefikLogPath string
	AutoDiscover   bool
	WatchInterval  time.Duration
}

// ServerConfig contains web server settings
type ServerConfig struct {
	Host       string
	Port       int
	Production bool
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
			Path:         getEnv("DB_PATH", "loglynx.db"),
			MaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 3),
			ConnMaxLife:  getEnvAsDuration("DB_CONN_MAX_LIFE", time.Hour),
		},
		GeoIP: GeoIPConfig{
			CityDBPath:    getEnv("GEOIP_CITY_DB", "geoip/GeoLite2-City.mmdb"),
			CountryDBPath: getEnv("GEOIP_COUNTRY_DB", "geoip/GeoLite2-Country.mmdb"),
			ASNDBPath:     getEnv("GEOIP_ASN_DB", "geoip/GeoLite2-ASN.mmdb"),
			Enabled:       getEnvAsBool("GEOIP_ENABLED", true),
		},
		LogSources: LogSourcesConfig{
			TraefikLogPath: getEnv("TRAEFIK_LOG_PATH", "traefik/logs/access.log"),
			AutoDiscover:   getEnvAsBool("LOG_AUTO_DISCOVER", true),
			WatchInterval:  getEnvAsDuration("LOG_WATCH_INTERVAL", 5*time.Second),
		},
		Server: ServerConfig{
			Host:       getEnv("SERVER_HOST", "0.0.0.0"),
			Port:       getEnvAsInt("SERVER_PORT", 8080),
			Production: getEnvAsBool("SERVER_PRODUCTION", false),
		},
		Performance: PerformanceConfig{
			RealtimeMetricsInterval: getEnvAsDuration("METRICS_INTERVAL", 5*time.Second),
			GeoIPCacheSize:          getEnvAsInt("GEOIP_CACHE_SIZE", 10000),
			BatchSize:               getEnvAsInt("BATCH_SIZE", 1000),
			WorkerPoolSize:          getEnvAsInt("WORKER_POOL_SIZE", 4),
		},
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
