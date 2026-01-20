/*
MIT License

Copyright (c) 2026 Kolin

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

/**
 * LogLynx API Client
 * Handles all API requests with error handling and caching
 */

const LogLynxAPI = {
    baseURL: '/api/v1',
    cache: new Map(),
    cacheTimeout: 30000, // 30 seconds default cache
    currentServices: [], // Array of selected services [{name: 'X', type: 'backend_name'}, ...]
    currentServiceType: 'auto', // Currently selected service type (auto, backend_name, backend_url, host)
    hideMyTraffic: false, // Whether to hide own IP traffic
    hideTrafficServices: [], // Array of services to hide traffic on [{name: 'X', type: 'backend_name'}, ...]
    pendingRequests: new Map(), // Track in-flight requests to prevent duplicates
    abortControllers: new Map(), // Track abort controllers for request cancellation

    // Performance monitoring
    _performanceMetrics: {
        totalRequests: 0,
        successfulRequests: 0,
        failedRequests: 0,
        totalResponseTime: 0,
        slowestRequest: { url: '', time: 0 },
        recentRequests: [] // Keep last 10 requests for debugging
    },

    /**
     * Make a GET request with optional caching and request deduplication
     */
    async get(endpoint, params = {}, useCache = false) {
        // NOTE: Backend middleware now handles blocking during initial load
        // No need for client-side blocking - this prevents double-blocking issues
        
        // Build URL with parameters
        const url = this.buildURL(endpoint, params);

        // Check cache if enabled
        if (useCache) {
            const cached = this.getFromCache(url);
            if (cached) return cached;
        }

        // Request deduplication: If same request is already in-flight, wait for it
        if (this.pendingRequests.has(url)) {
            return this.pendingRequests.get(url);
        }

        // Create abort controller for request cancellation
        const abortController = new AbortController();
        this.abortControllers.set(url, abortController);

        // Create the request promise
        const requestPromise = (async () => {
            const startTime = performance.now();
            this._performanceMetrics.totalRequests++;

            try {
                const response = await fetch(url, {
                    signal: abortController.signal
                });

                // Handle 503 Service Unavailable (initial load in progress)
                if (response.status === 503) {
                    const data = await response.json();
                    console.log(`[API] Service initializing: ${endpoint} - ${data.message}`);
                    return {
                        success: false,
                        error: data.message || 'Service initializing. Please wait...',
                        status: 503,
                        initializing: true,
                        data: data
                    };
                }

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }

                const data = await response.json();
                const responseTime = performance.now() - startTime;

                // Update performance metrics
                this._performanceMetrics.successfulRequests++;
                this._performanceMetrics.totalResponseTime += responseTime;

                // Track slowest request
                if (responseTime > this._performanceMetrics.slowestRequest.time) {
                    this._performanceMetrics.slowestRequest = { url: endpoint, time: responseTime };
                }

                // Track recent requests (keep last 10)
                this._performanceMetrics.recentRequests.unshift({
                    endpoint,
                    time: responseTime,
                    timestamp: Date.now(),
                    success: true
                });
                if (this._performanceMetrics.recentRequests.length > 10) {
                    this._performanceMetrics.recentRequests.pop();
                }

                // Store in cache if enabled
                if (useCache) {
                    this.setCache(url, data);
                }

                return { success: true, data };
            } catch (error) {
                const responseTime = performance.now() - startTime;

                // Update failure metrics (except for aborts)
                if (error.name !== 'AbortError') {
                    this._performanceMetrics.failedRequests++;
                    this._performanceMetrics.recentRequests.unshift({
                        endpoint,
                        time: responseTime,
                        timestamp: Date.now(),
                        success: false,
                        error: error.message
                    });
                    if (this._performanceMetrics.recentRequests.length > 10) {
                        this._performanceMetrics.recentRequests.pop();
                    }
                }

                // Don't log abort errors (they're intentional cancellations)
                if (error.name !== 'AbortError') {
                    console.error(`API Error [${endpoint}]:`, error);
                }
                return { success: false, error: error.message, aborted: error.name === 'AbortError' };
            } finally {
                // Cleanup
                this.pendingRequests.delete(url);
                this.abortControllers.delete(url);
            }
        })();

        // Store in pending requests
        this.pendingRequests.set(url, requestPromise);

        return requestPromise;
    },

    /**
     * Build URL with query parameters and service filter
     */
    buildURL(endpoint, params = {}) {
        const url = new URL(this.baseURL + endpoint, window.location.origin);

        // Add service filters if set (multiple services)
        if (this.currentServices && this.currentServices.length > 0) {
            this.currentServices.forEach(service => {
                url.searchParams.append('services[]', service.name);
                url.searchParams.append('service_types[]', service.type);
            });
        }

        // Add hide my traffic parameters
        if (this.hideMyTraffic) {
            url.searchParams.append('exclude_own_ip', 'true');

            // Add exclude services if specified
            if (this.hideTrafficServices && this.hideTrafficServices.length > 0) {
                this.hideTrafficServices.forEach(service => {
                    url.searchParams.append('exclude_services[]', service.name);
                    url.searchParams.append('exclude_service_types[]', service.type);
                });
            }
        }

        // Add all other parameters
        Object.keys(params).forEach(key => {
            if (params[key] !== null && params[key] !== undefined) {
                url.searchParams.append(key, params[key]);
            }
        });

        return url.toString();
    },

    /**
     * Set service filters for all requests (multiple services)
     * @param {Array} services - Array of {name: string, type: string} objects
     */
    setServiceFilters(services) {
        this.currentServices = services || [];
        this.clearCache(); // Clear cache when filter changes
    },

    /**
     * Get current service filters
     */
    getServiceFilters() {
        return this.currentServices;
    },

    /**
     * DEPRECATED: Use setServiceFilters instead
     */
    setServiceFilter(service, serviceType = 'auto') {
        if (service) {
            this.setServiceFilters([{name: service, type: serviceType}]);
        } else {
            this.setServiceFilters([]);
        }
    },

    /**
     * DEPRECATED: Use getServiceFilters instead
     */
    getServiceFilter() {
        if (this.currentServices.length > 0) {
            return {
                service: this.currentServices[0].name,
                type: this.currentServices[0].type
            };
        }
        return { service: '', type: 'auto' };
    },

    /**
     * DEPRECATED: Use setServiceFilter instead
     */
    setHostFilter(host) {
        this.setServiceFilter(host, 'auto');
    },

    /**
     * DEPRECATED: Use getServiceFilter instead
     */
    getHostFilter() {
        return this.currentService;
    },

    /**
     * Set hide my traffic filter
     * @param {boolean} enabled - Whether to hide own IP traffic
     */
    setHideMyTraffic(enabled) {
        this.hideMyTraffic = enabled;
        this.clearCache();
    },

    /**
     * Get hide my traffic status
     */
    getHideMyTraffic() {
        return this.hideMyTraffic;
    },

    /**
     * Set services to hide traffic on
     * @param {Array} services - Array of {name: string, type: string} objects
     */
    setHideTrafficFilters(services) {
        this.hideTrafficServices = services || [];
        this.clearCache();
    },

    /**
     * Get hide traffic service filters
     */
    getHideTrafficFilters() {
        return this.hideTrafficServices;
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

    /**
     * Cancel a specific pending request by URL
     */
    cancelRequest(endpoint, params = {}) {
        const url = this.buildURL(endpoint, params);
        const controller = this.abortControllers.get(url);
        if (controller) {
            controller.abort();
            this.abortControllers.delete(url);
            this.pendingRequests.delete(url);
        }
    },

    /**
     * Cancel all pending requests
     */
    cancelAllRequests() {
        for (const controller of this.abortControllers.values()) {
            controller.abort();
        }
        this.abortControllers.clear();
        this.pendingRequests.clear();
    },

    /**
     * Get API performance metrics
     */
    getPerformanceMetrics() {
        const avgResponseTime = this._performanceMetrics.successfulRequests > 0
            ? this._performanceMetrics.totalResponseTime / this._performanceMetrics.successfulRequests
            : 0;

        return {
            totalRequests: this._performanceMetrics.totalRequests,
            successfulRequests: this._performanceMetrics.successfulRequests,
            failedRequests: this._performanceMetrics.failedRequests,
            successRate: this._performanceMetrics.totalRequests > 0
                ? (this._performanceMetrics.successfulRequests / this._performanceMetrics.totalRequests * 100).toFixed(1)
                : 0,
            avgResponseTime: avgResponseTime.toFixed(2),
            slowestRequest: this._performanceMetrics.slowestRequest,
            recentRequests: this._performanceMetrics.recentRequests,
            pendingRequests: this.pendingRequests.size
        };
    },

    /**
     * Reset performance metrics
     */
    resetPerformanceMetrics() {
        this._performanceMetrics = {
            totalRequests: 0,
            successfulRequests: 0,
            failedRequests: 0,
            totalResponseTime: 0,
            slowestRequest: { url: '', time: 0 },
            recentRequests: []
        };
    },

    // ======================
    // Stats API Methods
    // ======================

    /**
     * Get summary statistics
     * @param {number} hours - Number of hours to fetch (0-8760, 0 = all time)
     */
    async getSummary(hours = 168) {
        return this.get('/stats/summary', { hours });
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
     * @param {number} hours - Number of hours to fetch
     */
    async getTopPaths(limit = 10, hours = 168) {
        return this.get('/stats/top/paths', { limit, hours });
    },

    /**
     * Get top countries
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopCountries(limit = 10, hours = 168) {
        return this.get('/stats/top/countries', { limit, hours });
    },

    /**
     * Get top IP addresses
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopIPs(limit = 10, hours = 168) {
        return this.get('/stats/top/ips', { limit, hours });
    },

    /**
     * Get top user agents
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopUserAgents(limit = 10, hours = 168) {
        return this.get('/stats/top/user-agents', { limit, hours });
    },

    /**
     * Get top browsers
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopBrowsers(limit = 10, hours = 168) {
        return this.get('/stats/top/browsers', { limit, hours });
    },

    /**
     * Get top operating systems
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopOperatingSystems(limit = 10, hours = 168) {
        return this.get('/stats/top/operating-systems', { limit, hours });
    },

    /**
     * Get top ASNs
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopASNs(limit = 10, hours = 168) {
        return this.get('/stats/top/asns', { limit, hours });
    },

    /**
     * Get top backends
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopBackends(limit = 10, hours = 168) {
        return this.get('/stats/top/backends', { limit, hours });
    },

    /**
     * Get top referrers
     * @param {number} limit - Number of results
     * @param {number} hours - Number of hours to fetch
     */
    async getTopReferrers(limit = 10, hours = 168) {
        return this.get('/stats/top/referrers', { limit, hours });
    },

    /**
     * Get top referrer domains
     * @param {number} limit - Number of results (0 = unlimited)
     * @param {number} hours - Number of hours to fetch
     */
    async getTopReferrerDomains(limit = 10, hours = 168) {
        return this.get('/stats/top/referrer-domains', { limit, hours });
    },

    /**
     * Get status code distribution
     * @param {number} hours - Number of hours to fetch
     */
    async getStatusCodeDistribution(hours = 168) {
        return this.get('/stats/distribution/status-codes', { hours });
    },

    /**
     * Get HTTP method distribution
     * @param {number} hours - Number of hours to fetch
     */
    async getMethodDistribution(hours = 168) {
        return this.get('/stats/distribution/methods', { hours });
    },

    /**
     * Get protocol distribution
     * @param {number} hours - Number of hours to fetch
     */
    async getProtocolDistribution(hours = 168) {
        return this.get('/stats/distribution/protocols', { hours });
    },

    /**
     * Get TLS version distribution
     * @param {number} hours - Number of hours to fetch
     */
    async getTLSVersionDistribution(hours = 168) {
        return this.get('/stats/distribution/tls-versions', { hours });
    },

    /**
     * Get device type distribution
     * @param {number} hours - Number of hours to fetch
     */
    async getDeviceTypeDistribution(hours = 168) {
        return this.get('/stats/distribution/device-types', { hours });
    },

    /**
     * Get response time statistics
     * @param {number} hours - Number of hours to fetch
     */
    async getResponseTimeStats(hours = 168) {
        return this.get('/stats/performance/response-time', { hours });
    },

    /**
     * Get log processing statistics
     */
    async getLogProcessingStats() {
        return this.get('/stats/log-processing');
    },

    /**
     * Get system statistics (uptime, memory, database info, etc.)
     */
    async getSystemStats() {
        return this.get('/system/stats');
    },

    /**
     * Get system records timeline
     * @param {number} days - Number of days (default 30, max 365)
     */
    async getSystemTimeline(days = 30) {
        return this.get('/system/timeline', { days });
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
     * DEPRECATED: Use getServices() instead
     */
    async getDomains() {
        return this.get('/domains', {}, true); // Cache this
    },

    /**
     * Get available services with types (backend_name, backend_url, host)
     */
    async getServices() {
        return this.get('/services', {}, true); // Cache this
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
    },

    // ======================
    // IP Analytics Methods
    // ======================

    /**
     * Get comprehensive statistics for a specific IP
     * @param {string} ip - IP address
     */
    async getIPStats(ip) {
        return this.get(`/ip/${ip}/stats`);
    },

    /**
     * Get timeline data for a specific IP
     * @param {string} ip - IP address
     * @param {number} hours - Number of hours (1-8760)
     */
    async getIPTimeline(ip, hours = 168) {
        return this.get(`/ip/${ip}/timeline`, { hours });
    },

    /**
     * Get traffic heatmap for a specific IP
     * @param {string} ip - IP address
     * @param {number} days - Number of days (1-365)
     */
    async getIPHeatmap(ip, days = 30) {
        return this.get(`/ip/${ip}/heatmap`, { days });
    },

    /**
     * Get top paths for a specific IP
     * @param {string} ip - IP address
     * @param {number} limit - Number of results (1-100)
     */
    async getIPTopPaths(ip, limit = 20) {
        return this.get(`/ip/${ip}/top/paths`, { limit });
    },

    /**
     * Get top backends for a specific IP
     * @param {string} ip - IP address
     * @param {number} limit - Number of results (1-100)
     */
    async getIPTopBackends(ip, limit = 10) {
        return this.get(`/ip/${ip}/top/backends`, { limit });
    },

    /**
     * Get status code distribution for a specific IP
     * @param {string} ip - IP address
     */
    async getIPStatusCodes(ip) {
        return this.get(`/ip/${ip}/distribution/status-codes`);
    },

    /**
     * Get top browsers for a specific IP
     * @param {string} ip - IP address
     * @param {number} limit - Number of results (1-100)
     */
    async getIPTopBrowsers(ip, limit = 10) {
        return this.get(`/ip/${ip}/top/browsers`, { limit });
    },

    /**
     * Get top operating systems for a specific IP
     * @param {string} ip - IP address
     * @param {number} limit - Number of results (1-100)
     */
    async getIPTopOperatingSystems(ip, limit = 10) {
        return this.get(`/ip/${ip}/top/operating-systems`, { limit });
    },

    /**
     * Get device type distribution for a specific IP
     * @param {string} ip - IP address
     */
    async getIPDeviceTypes(ip) {
        return this.get(`/ip/${ip}/distribution/device-types`);
    },

    /**
     * Get response time statistics for a specific IP
     * @param {string} ip - IP address
     */
    async getIPResponseTime(ip) {
        return this.get(`/ip/${ip}/performance/response-time`);
    },

    /**
     * Get recent requests for a specific IP
     * @param {string} ip - IP address
     * @param {number} limit - Number of results (1-100)
     */
    async getIPRecentRequests(ip, limit = 50) {
        return this.get(`/ip/${ip}/recent-requests`, { limit });
    },

    /**
     * Search for IPs matching a query
     * @param {string} query - Search query (partial IP)
     * @param {number} limit - Number of results (1-100)
     */
    async searchIPs(query, limit = 20) {
        return this.get('/ip/search', { q: query, limit });
    }
};

// Export for use in other scripts
window.LogLynxAPI = LogLynxAPI;
