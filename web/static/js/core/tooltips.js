/**
 * LogLynx Tooltip Utility
 * Helper functions for creating and managing tooltips
 */

const LogLynxTooltips = {
    /**
     * Create a tooltip element
     * @param {string} text - Tooltip text content
     * @param {string} position - Tooltip position (top, bottom, left, right)
     * @param {string} title - Optional tooltip title
     * @returns {string} HTML string for tooltip
     */
    create: function(text, position = 'top', title = '') {
        const titleHtml = title ? `<div class="info-tooltip-title">${title}</div>` : '';
        const positionClass = position !== 'top' ? `tooltip-${position}` : '';

        return `
            <span class="info-tooltip ${positionClass}">
                <i class="info-tooltip-icon fas fa-info"></i>
                <div class="info-tooltip-content">
                    ${titleHtml}
                    <div class="info-tooltip-description">${text}</div>
                </div>
            </span>
        `;
    },

    /**
     * Add tooltip to an existing element
     * @param {HTMLElement|string} element - Element or selector
     * @param {string} text - Tooltip text
     * @param {string} position - Tooltip position
     * @param {string} title - Optional title
     */
    add: function(element, text, position = 'top', title = '') {
        const el = typeof element === 'string' ? document.querySelector(element) : element;
        if (!el) return;

        const tooltip = this.create(text, position, title);
        el.insertAdjacentHTML('beforeend', tooltip);
    },

    /**
     * Initialize all tooltips with data attributes
     * Usage: <div data-tooltip="Your tooltip text" data-tooltip-position="bottom">
     */
    initAll: function() {
        document.querySelectorAll('[data-tooltip]').forEach(element => {
            const text = element.getAttribute('data-tooltip');
            const position = element.getAttribute('data-tooltip-position') || 'top';
            const title = element.getAttribute('data-tooltip-title') || '';

            if (!element.querySelector('.info-tooltip')) {
                const tooltip = this.create(text, position, title);
                element.insertAdjacentHTML('beforeend', tooltip);
            }
        });
    },

    /**
     * Enhanced tooltip texts for all pages with detailed descriptions
     */
    texts: {
        // === OVERVIEW PAGE ===
        totalRequests: "Total number of HTTP requests received in the selected time period. Includes all status codes (2xx, 3xx, 4xx, 5xx) and request types (GET, POST, PUT, DELETE, etc.). Use this metric to track overall traffic volume and identify trends.",
        successRate: "Percentage of successful requests (2xx and 3xx status codes). A healthy system should maintain 95%+ success rate. Values below 90% may indicate system issues, errors, or attacks. Monitor this closely for service health.",
        avgResponseTime: "Average time taken by your server to process and respond to requests, measured in milliseconds. Excellent: <200ms, Good: 200-500ms, Needs optimization: >1000ms. This includes database queries, API calls, and rendering time.",
        uniqueVisitors: "Number of unique IP addresses that accessed your service during the time period. Each IP typically represents a distinct user or device. Note: Multiple users behind corporate NAT may share one IP, so actual user count may be higher.",
        errorRate: "Percentage of failed requests including 4xx client errors (bad requests, not found, unauthorized) and 5xx server errors (crashes, timeouts, unavailable). Target: under 2% for production. High error rates require immediate investigation.",
        bandwidth: "Total amount of data transferred to clients, measured in bytes. Useful for monitoring bandwidth costs (if metered), identifying heavy content, and optimizing payload sizes. Large spikes may indicate DDoS or scraping.",
        requestsPerHour: "Average number of requests your service receives per hour. Helps identify traffic patterns, peak usage times, and capacity planning needs. Use this to schedule maintenance during low-traffic periods.",

        // === TRAFFIC ANALYSIS ===
        peakTraffic: "Maximum number of requests per hour during the selected period. Critical for capacity planning and infrastructure sizing. Your system must handle at least this load during peak times. Consider 2x margin for safety.",
        countries: "Number of different countries accessing your service. Indicates global reach and international audience. Use for CDN placement, localization priorities, and geo-blocking decisions.",
        cities: "Number of distinct cities with traffic to your service. Provides granular geographic distribution. Useful for identifying regional markets and planning local content/services.",
        continents: "Number of continents with active users. Shows true global distribution. All 6 inhabited continents means worldwide reach.",
        heatmap: "Visual representation of traffic patterns by day and hour of the week. Darker colors indicate higher traffic. Use to identify peak hours, weekend patterns, and schedule maintenance during quiet periods.",
        deviceTypes: "Distribution of traffic by device category (Desktop, Mobile, Tablet, Bot). Essential for responsive design priorities and understanding your audience. Mobile-first design if mobile >60%.",
        asn: "Autonomous System Number - identifies the network provider or hosting company. Useful for understanding traffic sources, identifying bot networks, and blocking malicious hosts.",
        referrerDomains: "Domains that linked to your site, driving traffic. Shows marketing effectiveness, partner traffic, and organic discovery. Empty referrer often means direct visits or bookmarks.",

        // === PERFORMANCE ===
        p50: "Median response time (50th percentile) - half of all requests are faster than this value. Good baseline metric for typical user experience. Target: <300ms for good UX.",
        p95: "95th percentile response time - 95% of requests are faster than this. Helps identify performance outliers and worst-case scenarios. Use this for SLA targets.",
        p99: "99th percentile response time - 99% of requests are faster than this. Critical for user experience as even 1% slow requests impact real users. Premium services target P99 <500ms.",
        fastRequests: "Percentage of requests completed in under 100ms. Excellent performance indicator - these requests feel instant to users. Higher is better, aim for 50%+.",
        slowRequests: "Percentage of requests taking over 1 second. These create poor user experience and may cause users to leave. Each second of delay can reduce conversions by 7%. Investigate and optimize.",
        percentileBreakdown: "Distribution of response times across different percentile ranges (P50, P75, P90, P95, P99). Shows performance consistency and helps identify if slowness affects few or many users.",
        minResponseTime: "Fastest request processed during the period. Usually cached or very simple requests. Represents ideal performance under optimal conditions.",
        maxResponseTime: "Slowest request processed during the period. May indicate database queries, external API calls, or resource-intensive operations. Outliers may need optimization or timeout limits.",
        medianResponseTime: "Middle value of all response times. Less affected by outliers than average. Better metric for typical user experience than mean.",

        // === GEOGRAPHIC ANALYTICS ===
        geoCoordinates: "Latitude and longitude of the IP address location. Approximate location based on GeoIP database. Accuracy varies: usually city-level for ISPs, country-level for mobile/VPN.",
        geoAccuracy: "GeoIP data provides city-level accuracy for most fixed-line ISPs. Mobile and VPN traffic may only resolve to country level. Coordinates are approximate (±50km typical).",
        mapClusters: "Groups nearby markers for better map visualization at zoomed-out levels. Click clusters to zoom in and see individual IP locations. Improves map performance with many markers.",
        heatmapIntensity: "Heat intensity represents request volume from that geographic location. Red areas = highest traffic, yellow = moderate, green = low. Use to identify your primary markets.",
        topCountry: "Country with the most traffic to your service. Your primary market. Consider local hosting, CDN presence, and language support for this region.",
        geoSpread: "Geographic distribution classification based on traffic diversity. Global = all continents, Regional = one region, Local = one country. Indicates market maturity.",
        ipLookup: "Search for specific IP address to see its location, ISP, and request history. Useful for investigating suspicious activity or understanding user origins.",

        // === SECURITY & NETWORK ===
        statusCodes: "HTTP status codes indicate request outcomes. 2xx=success (OK, Created), 3xx=redirect (Moved, Not Modified), 4xx=client error (Not Found, Forbidden, Unauthorized), 5xx=server error (Internal Error, Unavailable).",
        protocol: "HTTP protocol version used for requests. HTTP/1.1 is legacy, HTTP/2 offers multiplexing and compression, HTTP/3 uses QUIC for even better performance. Newer = faster.",
        tlsVersion: "TLS/SSL version used for encrypted HTTPS connections. TLS 1.0/1.1 are deprecated and insecure. TLS 1.2 is minimum acceptable. TLS 1.3 is latest and most secure. Disable old versions.",
        cipher: "Encryption cipher suite used for securing connections. Modern ciphers (AES-GCM, ChaCha20) are secure. Avoid RC4, 3DES, and non-forward-secret ciphers. Use Mozilla SSL Configuration Generator.",
        ipReputation: "Analysis of IP addresses for suspicious patterns: high request rates, error patterns, user-agent spoofing, or known malicious IPs. Helps identify bots, scrapers, and attackers.",
        attackPatterns: "Detection of common attack patterns: SQL injection attempts, path traversal, brute force login, DDoS. Monitor for security threats requiring immediate response.",
        blockedRequests: "Requests blocked by firewall, rate limiting, or security rules. High numbers may indicate attack attempts. Very high numbers may indicate misconfigured firewall rules blocking legitimate traffic.",

        // === REAL-TIME MONITOR ===
        liveMetrics: "Real-time data streaming with 2-second updates via WebSocket connection. Shows current system activity as it happens. Perfect for monitoring deployments, incidents, or live events.",
        requestRate: "Current rate of incoming requests per second. Calculated from last 60 seconds of data. Helps identify traffic spikes, DDoS attacks, or sudden popularity (viral content).",
        errorRateLive: "Current rate of errors per second. Sudden spikes may indicate deployment issues, database problems, or API failures. Set up alerts for rapid response.",
        activeConnections: "Number of currently open HTTP connections to your server. High numbers may indicate slow clients, long-polling, or connection exhaustion. Monitor for resource limits.",
        connectionStatus: "WebSocket connection status for real-time updates. Green = connected and streaming, Yellow = reconnecting, Red = disconnected (refresh page).",
        perServiceMetrics: "Real-time metrics broken down by service/backend. Helps identify which microservice is experiencing issues. Essential for microservices architecture troubleshooting.",

        // === USER ANALYTICS ===
        browserDistribution: "Distribution of web browsers your visitors use (Chrome, Firefox, Safari, Edge, etc.). Helps prioritize browser compatibility testing and optimize for popular choices. Chrome usually dominates at 60-70%.",
        browserVersion: "Specific browser versions in use. Shows how many users have outdated browsers. Use to decide when to drop support for old versions and use modern web features.",
        osDistribution: "Operating system breakdown of visitors (Windows, macOS, Linux, iOS, Android). Useful for platform-specific optimization and bug prioritization. Mobile OS % indicates mobile traffic share.",
        osVersion: "Specific OS versions. Shows user base modernity. High Windows 7/8 usage may indicate corporate users slow to upgrade. iOS versions update faster than Android.",
        deviceType: "Device category classification: Desktop (mouse/keyboard), Mobile (phone), Tablet (iPad-sized), Bot (automated). Mobile-first design crucial if mobile >50%.",
        botDetection: "Automated detection of bot traffic vs human visitors. Bots include search engine crawlers (Google, Bing), monitoring tools, scrapers, and malicious bots. Good bots are beneficial, bad bots waste resources.",
        referrerTraffic: "Sources of incoming traffic showing which sites or campaigns drive visitors. Direct = no referrer (bookmarks, apps), Organic = search engines, Social = social media, Referral = other sites.",
        userAgent: "User-Agent string sent by client identifying browser, OS, and device. Used for bot detection and analytics. Can be spoofed but useful for general patterns.",

        // === CONTENT ANALYTICS ===
        topPaths: "Most frequently accessed URLs on your site ranked by request count. Identifies your most popular pages/content. Use to optimize important pages and understand user interests.",
        uniquePaths: "Number of distinct URLs accessed during period. Higher numbers indicate rich content and good site exploration. Low numbers may indicate poor navigation or limited content.",
        httpMethods: "Distribution of HTTP request methods. GET = reading/viewing (should be most common), POST = form submissions/actions, PUT/DELETE = API operations. Unusual patterns may indicate attacks.",
        requestsByPath: "Request volume per URL path. Shows traffic distribution across site. Helps identify hotspots for caching, optimization priorities, and content that needs scaling.",
        pathErrors: "Error rates per URL path. Identifies broken pages (404s) and problematic endpoints (500s). Fix high-error pages to improve overall success rate and user experience.",
        contentTypes: "Distribution of content types served (HTML pages, JSON APIs, images, CSS/JS, etc.). Shows how your server is used. High image % may benefit from image CDN.",

        // === BACKEND HEALTH ===
        backendHealth: "Overall health status of backend service based on error rate and response time. Green=healthy (<2% errors, <500ms), Yellow=degraded (2-5% errors or 500-1000ms), Red=critical (>5% errors or >1000ms).",
        backendName: "Name/identifier of the backend service handling requests. Typically configured in your reverse proxy (Traefik, Nginx, etc.). Helps track traffic routing and service usage.",
        avgResponseBackend: "Average time this specific backend takes to process requests, measured in milliseconds. Does NOT include network latency, load balancer overhead, or client connection time. Pure backend processing time.",
        errorRateBackend: "Percentage of requests to this backend that resulted in errors (4xx/5xx). High error rates indicate backend problems: bugs, resource exhaustion, database issues, or dependency failures.",
        backendUptime: "Percentage of time this backend has been healthy and responding. Calculated from successful health checks. 99.9% uptime = 43 minutes downtime per month. 99.99% = 4.3 minutes.",
        backendLoad: "Current load on this backend measured by request rate and resource utilization. Helps identify overloaded services needing scaling or load balancing adjustments.",
        healthCheck: "Status of automated health check probes to this backend. Checks HTTP endpoint response. Failed health checks trigger alerts and traffic rerouting in load balancers.",

        // === GENERAL UI ELEMENTS ===
        timeRange: "Select time period to analyze (last 1h, 24h, 7 days, 30 days, custom range). Longer periods show long-term trends and seasonality. Shorter periods show recent changes and current state. Default: 7 days for good balance.",
        autoRefresh: "Automatically reload data at the specified interval (5s, 15s, 30s, 1m, 5m, off). Enable for live monitoring and dashboards. Disable when doing detailed analysis to avoid losing current view.",
        serviceFilter: "Filter all data to show only requests for a specific service, domain, or backend. Useful in multi-tenant or microservices setups to isolate one service's metrics. Select 'All Traffic' to remove filter.",
        exportData: "Download current table data as CSV or JSON format for external analysis, reporting, or archival. CSV works in Excel/Sheets. JSON for programmatic access. Respects current filters and time range.",
        dataTable: "Interactive data table with sorting, filtering, pagination, and search. Click column headers to sort ascending/descending. Use search box to filter rows. Pagination at bottom shows 10/25/50/100 rows per page.",
        search: "Search/filter table data. Type to instantly filter visible rows. Searches across all columns. Use for finding specific IPs, paths, or other values. Clear search to see all data again.",
        pagination: "Navigate through table pages. Shows current page and total pages. Use Previous/Next or page numbers. Adjust 'Show entries' to display more rows per page.",
        sortColumn: "Click column header to sort by that column. First click = ascending order (A-Z, 0-9), second click = descending order (Z-A, 9-0). Sorted column shows up/down arrow indicator.",

        // === CHARTS & VISUALIZATIONS ===
        lineChart: "Time-series line chart showing metric changes over time. X-axis = time, Y-axis = metric value. Hover points to see exact values. Use to identify trends, spikes, and patterns over time.",
        barChart: "Bar chart for comparing values across categories. Taller bars = higher values. Hover bars for exact numbers. Useful for comparing top N items (top pages, countries, etc.).",
        pieChart: "Pie/donut chart showing distribution as percentage of total. Larger slices = bigger share. Useful for seeing proportions like browser market share, traffic sources, or error types.",
        heatmapChart: "2D heatmap showing intensity across two dimensions (typically day × hour). Darker colors = more activity. Perfect for visualizing traffic patterns and finding peak times.",
        areaChart: "Area chart similar to line chart but with filled area below line. Good for showing volume over time and stacked comparison of multiple metrics.",
        sparkline: "Mini inline chart showing recent trend without axes or labels. Gives quick visual indication of whether metric is going up, down, or stable. Common in dashboards.",

        // === TOOLTIPS ===
        info: "Additional information about this metric or feature. Hover or click the info icon to learn more about what this means and how to use it effectively."
    },

    /**
     * Get tooltip text by key
     * @param {string} key - Tooltip key from texts object
     * @returns {string} Tooltip text
     */
    getText: function(key) {
        return this.texts[key] || 'Additional information about this metric or feature.';
    }
};

// Initialize tooltips when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    LogLynxTooltips.initAll();
});

// Re-initialize tooltips after dynamic content loads
if (typeof MutationObserver !== 'undefined') {
    const observer = new MutationObserver((mutations) => {
        mutations.forEach((mutation) => {
            if (mutation.addedNodes.length) {
                LogLynxTooltips.initAll();
            }
        });
    });

    document.addEventListener('DOMContentLoaded', () => {
        observer.observe(document.body, {
            childList: true,
            subtree: true
        });
    });
}
