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
package enrichment

import (
	"fmt"
	"loglynx/internal/database/models"
	"net"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/pterm/pterm"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GeoIPEnricher provides GeoIP enrichment with caching
type GeoIPEnricher struct {
	cityDB    *geoip2.Reader
	countryDB *geoip2.Reader
	asnDB     *geoip2.Reader
	db        *gorm.DB
	logger    *pterm.Logger
	cache     map[string]*models.IPReputation
	cacheMu   sync.RWMutex
	enabled   bool
	cacheSize int // Maximum cache size from config (GEOIP_CACHE_SIZE)
}

// NewGeoIPEnricher creates a new GeoIP enricher
// Handles City, Country, and ASN databases - works with any combination available
func NewGeoIPEnricher(cityDBPath, countryDBPath, asnDBPath string, db *gorm.DB, logger *pterm.Logger, cacheSize int) (*GeoIPEnricher, error) {
	if cacheSize <= 0 {
		cacheSize = 10000 // Default fallback
	}

	enricher := &GeoIPEnricher{
		db:        db,
		logger:    logger,
		cache:     make(map[string]*models.IPReputation, cacheSize), // Pre-allocate with capacity
		enabled:   false,
		cacheSize: cacheSize,
	}

	// Try to load City database (provides most detailed location data)
	if cityDBPath != "" {
		cityDB, err := geoip2.Open(cityDBPath)
		if err != nil {
			logger.Warn("GeoIP City database not available",
				logger.Args("path", cityDBPath, "error", err))
		} else {
			enricher.cityDB = cityDB
			enricher.enabled = true
			logger.Info("Loaded GeoIP City database", logger.Args("path", cityDBPath))
		}
	}

	// Try to load Country database (fallback if City is not available)
	if countryDBPath != "" {
		countryDB, err := geoip2.Open(countryDBPath)
		if err != nil {
			logger.Warn("GeoIP Country database not available",
				logger.Args("path", countryDBPath, "error", err))
		} else {
			enricher.countryDB = countryDB
			enricher.enabled = true
			logger.Info("Loaded GeoIP Country database", logger.Args("path", countryDBPath))
		}
	}

	// Try to load ASN database (provides ISP/organization data)
	if asnDBPath != "" {
		asnDB, err := geoip2.Open(asnDBPath)
		if err != nil {
			logger.Warn("GeoIP ASN database not available",
				logger.Args("path", asnDBPath, "error", err))
		} else {
			enricher.asnDB = asnDB
			logger.Info("Loaded GeoIP ASN database", logger.Args("path", asnDBPath))
		}
	}

	if !enricher.enabled {
		logger.Warn("GeoIP enrichment disabled - no databases available")
	}

	return enricher, nil
}

// Enrich enriches an HTTP request with GeoIP data
func (g *GeoIPEnricher) Enrich(request *models.HTTPRequest) error {
	if !g.enabled || request.ClientIP == "" {
		return nil
	}

	// Check cache first
	g.cacheMu.RLock()
	cached, exists := g.cache[request.ClientIP]
	g.cacheMu.RUnlock()

	if exists {
		// Use cached data
		request.GeoCountry = cached.Country
		request.GeoCity = cached.City
		request.GeoLat = cached.Latitude
		request.GeoLon = cached.Longitude
		request.ASN = cached.ASN
		request.ASNOrg = cached.ASNOrg

		g.logger.Trace("GeoIP cache hit", g.logger.Args("ip", request.ClientIP, "country", cached.Country))
		return nil
	}

	// Cache miss - lookup and store
	g.logger.Trace("GeoIP cache miss, performing lookup", g.logger.Args("ip", request.ClientIP))
	return g.lookupAndCache(request)
}

// lookupAndCache performs GeoIP lookup and caches the result
func (g *GeoIPEnricher) lookupAndCache(request *models.HTTPRequest) error {
	ip := net.ParseIP(request.ClientIP)
	if ip == nil {
		g.logger.Debug("Invalid IP address for GeoIP lookup", g.logger.Args("ip", request.ClientIP))
		return fmt.Errorf("invalid IP: %s", request.ClientIP)
	}

	reputation := &models.IPReputation{
		IPAddress: request.ClientIP,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}

	// Lookup City data (preferred - provides city, country, and coordinates)
	cityLookupSuccess := false
	if g.cityDB != nil {
		record, err := g.cityDB.City(ip)
		if err == nil {
			reputation.Country = record.Country.IsoCode
			reputation.CountryName = record.Country.Names["en"]
			reputation.City = record.City.Names["en"]
			reputation.Latitude = record.Location.Latitude
			reputation.Longitude = record.Location.Longitude

			// Populate request
			request.GeoCountry = reputation.Country
			request.GeoCity = reputation.City
			request.GeoLat = reputation.Latitude
			request.GeoLon = reputation.Longitude

			cityLookupSuccess = true
			g.logger.Debug("GeoIP City lookup successful",
				g.logger.Args("ip", request.ClientIP, "country", reputation.Country, "city", reputation.City))
		} else {
			g.logger.Debug("GeoIP City lookup failed", g.logger.Args("ip", request.ClientIP, "error", err))
		}
	}

	// Fallback to Country database if City lookup failed or unavailable
	if !cityLookupSuccess && g.countryDB != nil {
		record, err := g.countryDB.Country(ip)
		if err == nil {
			reputation.Country = record.Country.IsoCode
			reputation.CountryName = record.Country.Names["en"]
			// Country DB doesn't provide city or coordinates, but we get country at least

			// Populate request
			request.GeoCountry = reputation.Country

			g.logger.Debug("GeoIP Country lookup successful",
				g.logger.Args("ip", request.ClientIP, "country", reputation.Country))
		} else {
			g.logger.Debug("GeoIP Country lookup failed", g.logger.Args("ip", request.ClientIP, "error", err))
		}
	}

	// Lookup ASN data
	if g.asnDB != nil {
		record, err := g.asnDB.ASN(ip)
		if err == nil {
			reputation.ASN = int(record.AutonomousSystemNumber)
			reputation.ASNOrg = record.AutonomousSystemOrganization

			// Populate request
			request.ASN = reputation.ASN
			request.ASNOrg = reputation.ASNOrg

			g.logger.Debug("GeoIP ASN lookup successful",
				g.logger.Args("ip", request.ClientIP, "asn", reputation.ASN, "org", reputation.ASNOrg))
		} else {
			g.logger.Debug("GeoIP ASN lookup failed", g.logger.Args("ip", request.ClientIP, "error", err))
		}
	}

	// Store in memory cache first (fast, thread-safe)
	g.cacheMu.Lock()

	// Check if cache is full before adding
	if len(g.cache) >= g.cacheSize {
		// Cache is full - implement LRU-style eviction
		// Remove oldest entries (simple strategy: remove 10% of cache)
		evictCount := g.cacheSize / 10
		if evictCount < 1 {
			evictCount = 1
		}

		// Find and remove oldest entries based on LastSeen
		type ipAge struct {
			ip       string
			lastSeen time.Time
		}
		ages := make([]ipAge, 0, len(g.cache))
		for ip, rep := range g.cache {
			ages = append(ages, ipAge{ip: ip, lastSeen: rep.LastSeen})
		}

		// Sort by oldest first
		for i := 0; i < len(ages)-1; i++ {
			for j := i + 1; j < len(ages); j++ {
				if ages[i].lastSeen.After(ages[j].lastSeen) {
					ages[i], ages[j] = ages[j], ages[i]
				}
			}
		}

		// Remove oldest entries
		evicted := 0
		for _, age := range ages {
			if evicted >= evictCount {
				break
			}
			delete(g.cache, age.ip)
			evicted++
		}

		g.logger.Debug("GeoIP cache eviction performed",
			g.logger.Args(
				"evicted", evicted,
				"cache_size", len(g.cache),
				"max_size", g.cacheSize,
			))
	}

	g.cache[request.ClientIP] = reputation
	g.cacheMu.Unlock()

	// Store in database cache asynchronously to avoid blocking
	// Use goroutine to prevent concurrent insert errors from slowing down processing
	go func(rep *models.IPReputation) {
		// Try to insert - silently ignore errors as they're expected race conditions
		// Create a session with Silent mode to suppress all GORM logging for this operation
		_ = g.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(rep).Error
		// We don't check the error because:
		// 1. Memory cache is already updated (primary cache)
		// 2. Duplicate key errors are expected with parallel workers
		// 3. Database cache is just a persistent backup
	}(reputation)

	return nil
}

// LoadCache preloads the memory cache from database
// Optimized to load only hot IPs (recent activity) and skip if cache is already large
func (g *GeoIPEnricher) LoadCache() error {
	if !g.enabled {
		return nil
	}

	// Skip cache loading if already populated (avoids startup delay on restart)
	g.cacheMu.RLock()
	currentSize := len(g.cache)
	g.cacheMu.RUnlock()

	if currentSize > (g.cacheSize / 2) {
		g.logger.Info("GeoIP cache already populated, skipping load",
			g.logger.Args("entries", currentSize, "max_size", g.cacheSize))
		return nil
	}

	// Only load IPs that have been active in the last 7 days and have >5 requests
	// This is much more efficient than loading all IPs
	type IPCount struct {
		ClientIP   string
		Repetition int64
	}

	// Get hot IPs from recent activity only (last 7 days)
	sevenDaysAgo := time.Now().Add(-168 * time.Hour)
	var topIPs []IPCount
	err := g.db.Model(&models.HTTPRequest{}).
		Select("client_ip, COUNT(*) as repetition").
		Where("timestamp > ?", sevenDaysAgo).
		Group("client_ip").
		Having("COUNT(*) > 5"). // Only IPs with >5 requests
		Order("repetition DESC").
		Limit(g.cacheSize). // Use configured cache size
		Scan(&topIPs).
		Error

	if err != nil {
		g.logger.Warn("Failed to query hot IPs from http_requests", g.logger.Args("error", err))
		// Fall back to loading from ip_reputation (most recent)
		var reputations []models.IPReputation
		if err := g.db.Order("last_seen DESC").Limit(g.cacheSize).Find(&reputations).Error; err != nil {
			g.logger.WithCaller().Error("Failed to load IP reputation cache", g.logger.Args("error", err))
			return err
		}
		g.cacheMu.Lock()
		for i := range reputations {
			g.cache[reputations[i].IPAddress] = &reputations[i]
		}
		g.cacheMu.Unlock()
		g.logger.Info("Loaded GeoIP cache from ip_reputation", g.logger.Args("entries", len(reputations)))
		return nil
	}

	if len(topIPs) == 0 {
		g.logger.Info("No hot IPs to load into cache yet")
		return nil
	}

	// Load GeoIP data for these hot IPs from ip_reputation table
	ipAddresses := make([]string, len(topIPs))
	for i, ip := range topIPs {
		ipAddresses[i] = ip.ClientIP
	}

	var reputations []models.IPReputation
	if err := g.db.Where("ip_address IN ?", ipAddresses).Find(&reputations).Error; err != nil {
		g.logger.WithCaller().Error("Failed to load IP reputation data", g.logger.Args("error", err))
		return err
	}

	g.cacheMu.Lock()
	for i := range reputations {
		g.cache[reputations[i].IPAddress] = &reputations[i]
	}
	g.cacheMu.Unlock()

	g.logger.Info("Loaded GeoIP cache for hot IPs",
		g.logger.Args("hot_ips", len(topIPs), "cached", len(reputations), "min_requests", 5))
	return nil
}

// Close closes the GeoIP databases
func (g *GeoIPEnricher) Close() error {
	if g.cityDB != nil {
		g.cityDB.Close()
	}
	if g.countryDB != nil {
		g.countryDB.Close()
	}
	if g.asnDB != nil {
		g.asnDB.Close()
	}
	g.logger.Info("Closed GeoIP databases")
	return nil
}

// IsEnabled returns whether GeoIP enrichment is available
func (g *GeoIPEnricher) IsEnabled() bool {
	return g.enabled
}

// GetCacheSize returns the number of entries in memory cache
func (g *GeoIPEnricher) GetCacheSize() int {
	g.cacheMu.RLock()
	defer g.cacheMu.RUnlock()
	return len(g.cache)
}

