/**
 * LogLynx API Client
 * Handles all API requests with error handling and caching
 */

const LogLynxAPI = {
    baseURL: '/api/v1',
    cache: new Map(),
    cacheTimeout: 30000, // 30 seconds default cache
    currentHost: '', // Currently selected host filter

    /**
     * Make a GET request with optional caching
     */
    async get(endpoint, params = {}, useCache = false) {
        // Build URL with parameters
        const url = this.buildURL(endpoint, params);

        // Check cache if enabled
        if (useCache) {
            const cached = this.getFromCache(url);
            if (cached) return cached;
        }

        try {
            const response = await fetch(url);

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();

            // Store in cache if enabled
            if (useCache) {
                this.setCache(url, data);
            }

            return { success: true, data };
        } catch (error) {
            console.error(`API Error [${endpoint}]:`, error);
            return { success: false, error: error.message };
        }
    },

    /**
     * Build URL with query parameters and host filter
     */
    buildURL(endpoint, params = {}) {
        const url = new URL(this.baseURL + endpoint, window.location.origin);

        // Add host filter if set
        if (this.currentHost) {
            params.host = this.currentHost;
        }

        // Add all parameters
        Object.keys(params).forEach(key => {
            if (params[key] !== null && params[key] !== undefined) {
                url.searchParams.append(key, params[key]);
            }
        });

        return url.toString();
    },

    /**
     * Set host filter for all requests
     */
    setHostFilter(host) {
        this.currentHost = host;
        this.clearCache(); // Clear cache when filter changes
    },

    /**
     * Get current host filter
     */
    getHostFilter() {
        return this.currentHost;
    },

    /**
     * Cache management
     */
    getFromCache(url) {
        const cached = this.cache.get(url);
        if (!cached) return null;

        const now = Date.now();
        if (now - cached.timestamp > this.cacheTimeout) {
            this.cache.delete(url);
            return null;
        }

        return cached.data;
    },

    setCache(url, data) {
        this.cache.set(url, {
            data,
            timestamp: Date.now()
        });
    },

    clearCache() {
        this.cache.clear();
    },

    // ======================
    // Stats API Methods
    // ======================

    /**
     * Get summary statistics
     */
    async getSummary() {
        return this.get('/stats/summary');
    },

    /**
     * Get timeline data
     * @param {number} hours - Number of hours to fetch (1-8760)
     */
    async getTimeline(hours = 168) {
        return this.get('/stats/timeline', { hours });
    },

    /**
     * Get status code timeline
     * @param {number} hours - Number of hours to fetch
     */
    async getStatusCodeTimeline(hours = 168) {
        return this.get('/stats/timeline/status-codes', { hours });
    },

    /**
     * Get traffic heatmap data
     * @param {number} days - Number of days (1-365)
     */
    async getTrafficHeatmap(days = 7) {
        return this.get('/stats/heatmap/traffic', { days });
    },

    /**
     * Get top paths
     * @param {number} limit - Number of results (1-100)
     */
    async getTopPaths(limit = 10) {
        return this.get('/stats/top/paths', { limit });
    },

    /**
     * Get top countries
     * @param {number} limit - Number of results
     */
    async getTopCountries(limit = 10) {
        return this.get('/stats/top/countries', { limit });
    },

    /**
     * Get top IP addresses
     * @param {number} limit - Number of results
     */
    async getTopIPs(limit = 10) {
        return this.get('/stats/top/ips', { limit });
    },

    /**
     * Get top user agents
     * @param {number} limit - Number of results
     */
    async getTopUserAgents(limit = 10) {
        return this.get('/stats/top/user-agents', { limit });
    },

    /**
     * Get top browsers
     * @param {number} limit - Number of results
     */
    async getTopBrowsers(limit = 10) {
        return this.get('/stats/top/browsers', { limit });
    },

    /**
     * Get top operating systems
     * @param {number} limit - Number of results
     */
    async getTopOperatingSystems(limit = 10) {
        return this.get('/stats/top/operating-systems', { limit });
    },

    /**
     * Get top ASNs
     * @param {number} limit - Number of results
     */
    async getTopASNs(limit = 10) {
        return this.get('/stats/top/asns', { limit });
    },

    /**
     * Get top backends
     * @param {number} limit - Number of results
     */
    async getTopBackends(limit = 10) {
        return this.get('/stats/top/backends', { limit });
    },

    /**
     * Get top referrers
     * @param {number} limit - Number of results
     */
    async getTopReferrers(limit = 10) {
        return this.get('/stats/top/referrers', { limit });
    },

    /**
     * Get top referrer domains
     * @param {number} limit - Number of results (0 = unlimited)
     */
    async getTopReferrerDomains(limit = 10) {
        return this.get('/stats/top/referrer-domains', { limit });
    },

    /**
     * Get status code distribution
     */
    async getStatusCodeDistribution() {
        return this.get('/stats/distribution/status-codes');
    },

    /**
     * Get HTTP method distribution
     */
    async getMethodDistribution() {
        return this.get('/stats/distribution/methods');
    },

    /**
     * Get protocol distribution
     */
    async getProtocolDistribution() {
        return this.get('/stats/distribution/protocols');
    },

    /**
     * Get TLS version distribution
     */
    async getTLSVersionDistribution() {
        return this.get('/stats/distribution/tls-versions');
    },

    /**
     * Get device type distribution
     */
    async getDeviceTypeDistribution() {
        return this.get('/stats/distribution/device-types');
    },

    /**
     * Get response time statistics
     */
    async getResponseTimeStats() {
        return this.get('/stats/performance/response-time');
    },

    /**
     * Get log processing statistics
     */
    async getLogProcessingStats() {
        return this.get('/stats/log-processing');
    },

    /**
     * Get recent requests
     * @param {number} limit - Number of results (1-1000)
     * @param {number} offset - Pagination offset
     */
    async getRecentRequests(limit = 100, offset = 0) {
        return this.get('/requests/recent', { limit, offset });
    },

    /**
     * Get available domains/services
     */
    async getDomains() {
        return this.get('/domains', {}, true); // Cache this
    },

    // ======================
    // Real-time API Methods
    // ======================

    /**
     * Get current real-time metrics (single snapshot)
     */
    async getRealtimeMetrics() {
        return this.get('/realtime/metrics');
    },

    /**
     * Get per-service metrics
     */
    async getPerServiceMetrics() {
        return this.get('/realtime/services');
    },

    /**
     * Connect to real-time SSE stream
     * @param {Function} onMessage - Callback for each message
     * @param {Function} onError - Error callback
     * @returns {EventSource} The event source connection
     */
    connectRealtimeStream(onMessage, onError) {
        const url = this.buildURL('/realtime/stream');
        const eventSource = new EventSource(url);

        eventSource.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                onMessage(data);
            } catch (error) {
                console.error('Failed to parse SSE data:', error);
            }
        };

        eventSource.onerror = (error) => {
            console.error('SSE connection error:', error);
            if (onError) onError(error);
        };

        return eventSource;
    },

    // ======================
    // Batch Loading Methods
    // ======================

    /**
     * Load all data for overview dashboard
     */
    async loadOverviewData(timeRange = 168) {
        const promises = {
            summary: this.getSummary(),
            timeline: this.getTimeline(timeRange),
            statusTimeline: this.getStatusCodeTimeline(timeRange),
            statusDist: this.getStatusCodeDistribution(),
            topCountries: this.getTopCountries(5),
            topPaths: this.getTopPaths(5),
            recentRequests: this.getRecentRequests(10)
        };

        const results = {};
        for (const [key, promise] of Object.entries(promises)) {
            const result = await promise;
            results[key] = result.success ? result.data : null;
        }

        return results;
    },

    /**
     * Load all data for traffic analysis dashboard
     */
    async loadTrafficData(timeRange = 168, heatmapDays = 7) {
        const promises = {
            timeline: this.getTimeline(timeRange),
            statusTimeline: this.getStatusCodeTimeline(timeRange),
            heatmap: this.getTrafficHeatmap(heatmapDays),
            topCountries: this.getTopCountries(20),
            topIPs: this.getTopIPs(20),
            topASNs: this.getTopASNs(15)
        };

        const results = {};
        for (const [key, promise] of Object.entries(promises)) {
            const result = await promise;
            results[key] = result.success ? result.data : null;
        }

        return results;
    },

    /**
     * Load all data for performance dashboard
     */
    async loadPerformanceData(timeRange = 168) {
        const promises = {
            responseTime: this.getResponseTimeStats(),
            timeline: this.getTimeline(timeRange),
            topPaths: this.getTopPaths(20),
            backends: this.getTopBackends(15)
        };

        const results = {};
        for (const [key, promise] of Object.entries(promises)) {
            const result = await promise;
            results[key] = result.success ? result.data : null;
        }

        return results;
    },

    /**
     * Load all data for security dashboard
     */
    async loadSecurityData() {
        const promises = {
            topASNs: this.getTopASNs(20),
            topIPs: this.getTopIPs(20),
            tlsVersions: this.getTLSVersionDistribution(),
            protocols: this.getProtocolDistribution(),
            statusDist: this.getStatusCodeDistribution()
        };

        const results = {};
        for (const [key, promise] of Object.entries(promises)) {
            const result = await promise;
            results[key] = result.success ? result.data : null;
        }

        return results;
    },

    /**
     * Load all data for user analytics dashboard
     */
    async loadUserAnalyticsData() {
        const promises = {
            browsers: this.getTopBrowsers(15),
            operatingSystems: this.getTopOperatingSystems(15),
            deviceTypes: this.getDeviceTypeDistribution(),
            referrers: this.getTopReferrers(20),
            referrerDomains: this.getTopReferrerDomains(20),
            topCountries: this.getTopCountries(15)
        };

        const results = {};
        for (const [key, promise] of Object.entries(promises)) {
            const result = await promise;
            results[key] = result.success ? result.data : null;
        }

        return results;
    },

    /**
     * Load all data for content analytics dashboard
     */
    async loadContentData() {
        const promises = {
            topPaths: this.getTopPaths(50),
            methods: this.getMethodDistribution(),
            statusDist: this.getStatusCodeDistribution()
        };

        const results = {};
        for (const [key, promise] of Object.entries(promises)) {
            const result = await promise;
            results[key] = result.success ? result.data : null;
        }

        return results;
    },

    /**
     * Load all data for backend health dashboard
     */
    async loadBackendData(timeRange = 168) {
        const promises = {
            backends: this.getTopBackends(30),
            timeline: this.getTimeline(timeRange),
            statusDist: this.getStatusCodeDistribution(),
            responseTime: this.getResponseTimeStats()
        };

        const results = {};
        for (const [key, promise] of Object.entries(promises)) {
            const result = await promise;
            results[key] = result.success ? result.data : null;
        }

        return results;
    }
};

// Export for use in other scripts
window.LogLynxAPI = LogLynxAPI;
