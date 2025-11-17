/**
 * IP Analytics Detail Page
 * Comprehensive IP-specific statistics with interactive visualizations
 */

let currentIPAddress = '';
let ipMap = null;
let ipMarker = null;

// Chart instances
let timelineChart = null;
let heatmapChart = null;
let statusCodeChart = null;
let browserChart = null;
let osChart = null;
let deviceChart = null;

// DataTables instances
let backendsTable = null;
let pathsTable = null;
let recentRequestsTable = null;

// Search debounce timer
let searchDebounceTimer = null;

// Store loaded data for export
let ipAnalyticsData = {
    stats: null,
    timeline: null,
    heatmap: null,
    paths: null,
    backends: null,
    statusCodes: null,
    browsers: null,
    os: null,
    devices: null,
    responseTime: null,
    recentRequests: null
};

/**
 * Load all IP analytics data
 */
async function loadIPAnalytics(ipAddress) {
    if (!ipAddress) {
        LogLynxUtils.showNotification('IP address is required', 'error');
        return;
    }

    currentIPAddress = ipAddress;
    document.getElementById('currentIP').textContent = ipAddress;

    // Show loading overlay
    showLoading();

    try {
        // Load all data in parallel
        const [statsResult, timelineResult, heatmapResult, pathsResult, backendsResult, 
               statusCodesResult, browsersResult, osResult, devicesResult, responseTimeResult, recentRequestsResult] = await Promise.all([
            LogLynxAPI.getIPStats(ipAddress),
            LogLynxAPI.getIPTimeline(ipAddress, 168),
            LogLynxAPI.getIPHeatmap(ipAddress, 30),
            LogLynxAPI.getIPTopPaths(ipAddress, 50),
            LogLynxAPI.getIPTopBackends(ipAddress, 20),
            LogLynxAPI.getIPStatusCodes(ipAddress),
            LogLynxAPI.getIPTopBrowsers(ipAddress, 10),
            LogLynxAPI.getIPTopOperatingSystems(ipAddress, 10),
            LogLynxAPI.getIPDeviceTypes(ipAddress),
            LogLynxAPI.getIPResponseTime(ipAddress),
            LogLynxAPI.getIPRecentRequests(ipAddress, 50)
        ]);

        // Update all visualizations and store data
        if (statsResult.success) {
            ipAnalyticsData.stats = statsResult.data;
            updateIPKPIs(statsResult.data);
            initIPMap(statsResult.data);
        }

        if (timelineResult.success) {
            ipAnalyticsData.timeline = timelineResult.data;
            updateTimelineChart(timelineResult.data);
        }

        if (heatmapResult.success) {
            ipAnalyticsData.heatmap = heatmapResult.data;
            updateHeatmapChart(heatmapResult.data);
        }

        if (pathsResult.success) {
            ipAnalyticsData.paths = pathsResult.data;
            initPathsTable(pathsResult.data);
        }

        if (backendsResult.success) {
            ipAnalyticsData.backends = backendsResult.data;
            initBackendsTable(backendsResult.data);
        }

        if (statusCodesResult.success) {
            ipAnalyticsData.statusCodes = statusCodesResult.data;
            updateStatusCodeChart(statusCodesResult.data);
        }

        if (browsersResult.success) {
            ipAnalyticsData.browsers = browsersResult.data;
            updateBrowserChart(browsersResult.data);
        }

        if (osResult.success) {
            ipAnalyticsData.os = osResult.data;
            updateOSChart(osResult.data);
        }

        if (devicesResult.success) {
            ipAnalyticsData.devices = devicesResult.data;
            updateDeviceChart(devicesResult.data);
        }

        if (responseTimeResult.success) {
            ipAnalyticsData.responseTime = responseTimeResult.data;
            updateResponseTimeStats(responseTimeResult.data);
        }

        if (recentRequestsResult.success) {
            ipAnalyticsData.recentRequests = recentRequestsResult.data;
            initRecentRequestsTable(recentRequestsResult.data);
        }

        hideLoading();
        LogLynxUtils.showNotification('IP analytics loaded successfully', 'success');

    } catch (error) {
        console.error('Error loading IP analytics:', error);
        hideLoading();
        LogLynxUtils.showNotification('Failed to load IP analytics', 'error');
    }
}

/**
 * Update IP KPIs
 */
function updateIPKPIs(stats) {
    $('#totalRequests').text(LogLynxUtils.formatNumber(stats.total_requests));
    $('#successRate').text(stats.success_rate.toFixed(1) + '%');
    $('#avgResponseTime').text(LogLynxUtils.formatDuration(stats.avg_response_time));
    $('#totalBandwidth').text(LogLynxUtils.formatBytes(stats.total_bandwidth));

    // Location
    if (stats.geo_city && stats.geo_country) {
        $('#ipLocation').text(stats.geo_city);
        $('#ipCountry').text(countryToContinentMap[stats.geo_country]?.name || stats.geo_country);
    } else if (stats.geo_country) {
        $('#ipLocation').text(countryToContinentMap[stats.geo_country]?.name || stats.geo_country);
        $('#ipCountry').text('Country');
    } else {
        $('#ipLocation').text('Unknown');
        $('#ipCountry').text('Location not available');
    }

    // Timestamps
    $('#firstSeen').text(LogLynxUtils.formatDateTime(stats.first_seen));
    $('#lastSeen').text(LogLynxUtils.formatDateTime(stats.last_seen));

    // ASN
    if (stats.asn && stats.asn > 0) {
        $('#asn').text('AS' + stats.asn);
        $('#asnOrg').text(stats.asn_org || 'Unknown Organization');
    } else {
        $('#asn').text('N/A');
        $('#asnOrg').text('No ASN data');
    }

    // Unique targets
    $('#uniqueBackends').text(stats.unique_backends);
    $('#uniquePaths').text(stats.unique_paths);
    $('#uniqueTargets').text(stats.unique_backends + stats.unique_paths);

    // Threat level based on error rate
    $('#errorRateDisplay').text(stats.error_rate.toFixed(1) + '%');
    let threatLevel = 'Low';
    let threatClass = 'badge-success';
    if (stats.error_rate > 50) {
        threatLevel = 'High';
        threatClass = 'badge-danger';
    } else if (stats.error_rate > 20) {
        threatLevel = 'Medium';
        threatClass = 'badge-warning';
    }
    $('#threatLevel').text(threatLevel).removeClass('badge-success badge-warning badge-danger').addClass(threatClass);

    // Calculate bandwidth subtitle
    const avgBandwidthPerRequest = stats.total_requests > 0 ? stats.total_bandwidth / stats.total_requests : 0;
    $('#bandwidthSubtitle').text(LogLynxUtils.formatBytes(avgBandwidthPerRequest) + ' avg/request');

    // Response time subtitle
    if (stats.avg_response_time < 100) {
        $('#responseTimeSubtitle').text('Excellent').css('color', '#28a745');
    } else if (stats.avg_response_time < 500) {
        $('#responseTimeSubtitle').text('Good').css('color', '#ffc107');
    } else {
        $('#responseTimeSubtitle').text('Slow').css('color', '#dc3545');
    }
}

/**
 * Initialize IP location map with Leaflet
 */
function initIPMap(stats) {
    // Remove existing map if any
    if (ipMap) {
        ipMap.remove();
        ipMap = null;
    }

    // Check if we have coordinates
    if (!stats.geo_lat || !stats.geo_lon) {
        $('#ipMap').html('<div class="d-flex align-items-center justify-content-center h-100"><p class="text-muted">Location data not available</p></div>');
        return;
    }

    // Initialize map centered on IP location
    ipMap = L.map('ipMap').setView([stats.geo_lat, stats.geo_lon], 10);

    // Dark tile layer (default)
    const darkLayer = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
        subdomains: 'abcd',
        maxZoom: 20
    });

    // Street tile layer
    const streetLayer = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        maxZoom: 19
    });

    // Add default layer
    darkLayer.addTo(ipMap);

    // Custom marker icon
    const customIcon = L.divIcon({
        className: 'custom-marker',
        html: '<div style="background: #F46319; width: 20px; height: 20px; border-radius: 50%; border: 3px solid white; box-shadow: 0 2px 5px rgba(0,0,0,0.3);"></div>',
        iconSize: [20, 20],
        iconAnchor: [10, 10]
    });

    // Add marker
    ipMarker = L.marker([stats.geo_lat, stats.geo_lon], { icon: customIcon }).addTo(ipMap);

    // Popup content
    const popupContent = `
        <div style="min-width: 200px;">
            <strong style="font-size: 1.1em;">${currentIPAddress}</strong><br>
            <strong>Location:</strong> ${stats.geo_city || 'Unknown'}, ${countryToContinentMap[stats.geo_country]?.name || stats.geo_country || 'Unknown'} (${countryToContinentMap[stats.geo_country]?.continent || 'Unknown'})<br>
            <strong>Coordinates:</strong> ${stats.geo_lat.toFixed(4)}, ${stats.geo_lon.toFixed(4)}<br>
            ${stats.asn ? `<strong>ASN:</strong> AS${stats.asn}<br>` : ''}
            ${stats.asn_org ? `<strong>Org:</strong> ${stats.asn_org}<br>` : ''}
        </div>
    `;
    ipMarker.bindPopup(popupContent).openPopup();

    // Update map footer
    $('#mapCoordinates').text(`${stats.geo_lat.toFixed(6)}, ${stats.geo_lon.toFixed(6)}`);
    $('#mapCity').text(`${stats.geo_city || 'Unknown'}, ${stats.geo_country || 'Unknown'}`);

    // Map view toggle handlers
    $('#mapViewDark').on('click', function() {
        ipMap.removeLayer(streetLayer);
        ipMap.addLayer(darkLayer);
        $('#mapViewDark').addClass('active');
        $('#mapViewStreet').removeClass('active');
    });

    $('#mapViewStreet').on('click', function() {
        ipMap.removeLayer(darkLayer);
        ipMap.addLayer(streetLayer);
        $('#mapViewStreet').addClass('active');
        $('#mapViewDark').removeClass('active');
    });
}

/**
 * Zoom to IP location
 */
function zoomToLocation() {
    if (ipMap && ipMarker) {
        ipMap.setView(ipMarker.getLatLng(), 13);
        ipMarker.openPopup();
    }
}

/**
 * Update timeline chart
 */
function updateTimelineChart(data) {
    const ctx = document.getElementById('timelineChart');
    if (!ctx) return;

    // Destroy existing chart
    if (timelineChart) {
        timelineChart.destroy();
    }

    const labels = data.map(d => d.hour);
    const requests = data.map(d => d.requests);
    const bandwidth = data.map(d => d.bandwidth);

    timelineChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Requests',
                    data: requests,
                    borderColor: LogLynxCharts.colors.primary,
                    backgroundColor: LogLynxCharts.colors.primaryAlpha,
                    tension: 0.4,
                    fill: true,
                    yAxisID: 'y'
                },
                {
                    label: 'Bandwidth (MB)',
                    data: bandwidth.map(b => b / 1024 / 1024),
                    borderColor: LogLynxCharts.colors.warning,
                    backgroundColor: LogLynxCharts.colors.warningAlpha,
                    tension: 0.4,
                    fill: true,
                    yAxisID: 'y1'
                }
            ]
        },
        options: {
            ...LogLynxCharts.defaultOptions,
            interaction: {
                mode: 'index',
                intersect: false
            },
            scales: {
                y: {
                    type: 'linear',
                    display: true,
                    position: 'left',
                    grid: { color: 'rgba(255, 255, 255, 0.1)' },
                    ticks: { color: '#B0B0B0' }
                },
                y1: {
                    type: 'linear',
                    display: true,
                    position: 'right',
                    grid: { drawOnChartArea: false },
                    ticks: { color: '#B0B0B0' }
                },
                x: {
                    grid: { color: 'rgba(255, 255, 255, 0.1)' },
                    ticks: { 
                        color: '#B0B0B0',
                        maxRotation: 45,
                        minRotation: 45
                    }
                }
            }
        }
    });
}

/**
 * Update traffic heatmap
 */
function updateHeatmapChart(data) {
    const ctx = document.getElementById('heatmapChart');
    if (!ctx) return;

    // Destroy existing chart
    if (heatmapChart) {
        heatmapChart.destroy();
    }

    const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
    const hourLabels = Array.from({length: 24}, (_, i) => i.toString().padStart(2, '0') + ':00');

    // Generate heatmap data using the charts utility
    const heatmapData = LogLynxCharts.generateHeatmapData(data, dayNames, hourLabels);

    heatmapChart = LogLynxCharts.createHeatmapChart('heatmapChart', {
        datasets: [{
            label: 'Requests',
            data: heatmapData.data,
            backgroundColor: LogLynxCharts.heatmapColorFunction(heatmapData.maxValue),
            borderWidth: 1,
            borderColor: 'rgba(22, 22, 25, 0.8)',
            width: (ctx) => {
                const a = ctx.chart.chartArea;
                return a ? (a.right - a.left) / 24 - 1 : 0;
            },
            height: (ctx) => {
                const a = ctx.chart.chartArea;
                return a ? (a.bottom - a.top) / 7 - 1 : 0;
            }
        }]
    }, {
        scales: {
            x: {
                type: 'category',
                labels: hourLabels,
                ticks: {
                    maxRotation: 0,
                    autoSkip: true,
                    maxTicksLimit: 12
                }
            },
            y: {
                type: 'category',
                labels: dayNames,
                offset: true,
                reverse: true
            }
        },
        plugins: {
            tooltip: {
                callbacks: {
                    title: function(context) {
                        const data = context[0].raw;
                        return `${data.y}, ${data.x}`;
                    },
                    label: function(context) {
                        const data = context.raw;
                        return [
                            `Requests: ${LogLynxUtils.formatNumber(data.v || 0)}`,
                            `Avg Response: ${LogLynxUtils.formatDuration(data.avg || 0)}`
                        ];
                    }
                }
            }
        }
    });
}

/**
 * Update status code chart
 */
function updateStatusCodeChart(data) {
    const ctx = document.getElementById('statusCodeChart');
    if (!ctx) return;

    if (statusCodeChart) {
        statusCodeChart.destroy();
    }

    const labels = data.map(d => `${d.status_code}`);
    const values = data.map(d => d.count);
    const colors = data.map(d => {
        const code = d.status_code;
        if (code >= 200 && code < 300) return LogLynxCharts.colors.success;
        if (code >= 300 && code < 400) return LogLynxCharts.colors.info;
        if (code >= 400 && code < 500) return LogLynxCharts.colors.warning;
        return LogLynxCharts.colors.danger;
    });

    statusCodeChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: labels,
            datasets: [{
                data: values,
                backgroundColor: colors,
                borderColor: '#1f1f21',
                borderWidth: 2
            }]
        },
        options: {
            ...LogLynxCharts.defaultOptions,
            plugins: {
                legend: { display: true, position: 'bottom' }
            }
        }
    });
}

/**
 * Update browser chart
 */
function updateBrowserChart(data) {
    const ctx = document.getElementById('browserChart');
    if (!ctx) return;

    if (browserChart) {
        browserChart.destroy();
    }

    const labels = data.map(d => d.browser);
    const values = data.map(d => d.count);

    browserChart = new Chart(ctx, {
        type: 'pie',
        data: {
            labels: labels,
            datasets: [{
                data: values,
                backgroundColor: LogLynxCharts.colors.chartPalette,
                borderColor: '#1f1f21',
                borderWidth: 2
            }]
        },
        options: {
            ...LogLynxCharts.defaultOptions,
            plugins: {
                legend: { display: true, position: 'bottom' }
            }
        }
    });
}

/**
 * Update OS chart
 */
function updateOSChart(data) {
    const ctx = document.getElementById('osChart');
    if (!ctx) return;

    if (osChart) {
        osChart.destroy();
    }

    const labels = data.map(d => d.os);
    const values = data.map(d => d.count);

    osChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [{
                label: 'Requests',
                data: values,
                backgroundColor: LogLynxCharts.colors.primary,
                borderColor: LogLynxCharts.colors.primary,
                borderWidth: 1
            }]
        },
        options: {
            ...LogLynxCharts.defaultOptions,
            indexAxis: 'y',
            scales: {
                x: {
                    grid: { color: 'rgba(255, 255, 255, 0.1)' },
                    ticks: { color: '#B0B0B0' }
                },
                y: {
                    grid: { color: 'rgba(255, 255, 255, 0.1)' },
                    ticks: { color: '#B0B0B0' }
                }
            }
        }
    });
}

/**
 * Update device type chart
 */
function updateDeviceChart(data) {
    const ctx = document.getElementById('deviceChart');
    if (!ctx) return;

    if (deviceChart) {
        deviceChart.destroy();
    }

    const labels = data.map(d => d.device_type);
    const values = data.map(d => d.count);
    
    // Calculate bot percentage for threat intelligence
    const totalRequests = values.reduce((sum, val) => sum + val, 0);
    const botData = data.find(d => d.device_type === 'bot');
    const botRequests = botData ? botData.count : 0;
    const botPercentage = totalRequests > 0 ? ((botRequests / totalRequests) * 100).toFixed(1) : 0;
    
    // Update bot detection in Threat Intelligence section
    updateBotDetection(botPercentage, botRequests);

    deviceChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: labels,
            datasets: [{
                data: values,
                backgroundColor: [
                    LogLynxCharts.colors.primary,
                    LogLynxCharts.colors.success,
                    LogLynxCharts.colors.warning,
                    LogLynxCharts.colors.danger
                ],
                borderColor: '#1f1f21',
                borderWidth: 2
            }]
        },
        options: {
            ...LogLynxCharts.defaultOptions,
            plugins: {
                legend: { display: true, position: 'bottom' }
            }
        }
    });
}

/**
 * Update bot detection display in Threat Intelligence
 */
function updateBotDetection(percentage, requests) {
    const percentageElem = $('#botPercentage');
    const labelElem = $('#botLabel');
    const iconElem = $('#botIcon');
    
    percentageElem.text(percentage + '%');
    
    if (percentage == 0) {
        iconElem.removeClass('text-warning text-danger').addClass('text-success');
        labelElem.text('No bot traffic detected');
    } else if (percentage < 20) {
        iconElem.removeClass('text-success text-danger').addClass('text-warning');
        labelElem.text(`Low bot activity (${LogLynxUtils.formatNumber(requests)} requests)`);
    } else if (percentage < 50) {
        iconElem.removeClass('text-success text-danger').addClass('text-warning');
        labelElem.text(`Moderate bot activity (${LogLynxUtils.formatNumber(requests)} requests)`);
    } else {
        iconElem.removeClass('text-success text-warning').addClass('text-danger');
        labelElem.text(`High bot activity (${LogLynxUtils.formatNumber(requests)} requests)`);
    }
}

/**
 * Update response time statistics
 */
function updateResponseTimeStats(stats) {
    $('#rtMin').text(LogLynxUtils.formatDuration(stats.min));
    $('#rtMax').text(LogLynxUtils.formatDuration(stats.max));
    $('#rtAvg').text(LogLynxUtils.formatDuration(stats.avg));
    $('#rtP50').text(LogLynxUtils.formatDuration(stats.p50));
    $('#rtP95').text(LogLynxUtils.formatDuration(stats.p95));
    $('#rtP99').text(LogLynxUtils.formatDuration(stats.p99));
}

/**
 * Initialize backends table
 */
function initBackendsTable(data) {
    if (backendsTable) {
        backendsTable.destroy();
    }

    const tableData = data.map((item, index) => [
        index + 1,
        LogLynxUtils.formatHostDisplay(item, '-'),
        LogLynxUtils.formatNumber(item.hits),
        LogLynxUtils.formatBytes(item.bandwidth),
        LogLynxUtils.formatDuration(item.avg_response_time),
        item.error_count || 0
    ]);

    backendsTable = $('#backendsTable').DataTable({
        data: tableData,
        pageLength: 10,
        order: [[3, 'desc']],
        columnDefs: [
            { targets: [3, 5], className: 'text-end' }
        ],
        ...LogLynxCharts.defaultDataTableOptions
    });
}

/**
 * Initialize paths table
 */
function initPathsTable(data) {
    if (pathsTable) {
        pathsTable.destroy();
    }

    const tableData = data.map((item, index) => [
        index + 1,
        LogLynxUtils.formatHostDisplay(item, '-'),
        `<code>${item.path}</code>`,
        LogLynxUtils.formatNumber(item.hits),
        LogLynxUtils.formatBytes(item.total_bandwidth),
        LogLynxUtils.formatDuration(item.avg_response_time),
        `<button class="btn btn-sm btn-outline" onclick="copyPath('${item.path.replace(/'/g, "\\'")}')"><i class="fas fa-copy"></i></button>`
    ]);

    pathsTable = $('#pathsTable').DataTable({
        data: tableData,
        pageLength: 10,
        order: [[3, 'desc']], // Update order column index (was 2, now 3 for Hits)
        columnDefs: [
            { targets: [3], className: 'text-end' }, // Hits column
            { targets: [6], orderable: false } // Actions column
        ],
        ...LogLynxCharts.defaultDataTableOptions
    });
}

/**
 * Copy path to clipboard
 */
function copyPath(path) {
    navigator.clipboard.writeText(path).then(() => {
        LogLynxUtils.showNotification('Path copied to clipboard', 'success');
    });
}

/**
 * Initialize recent requests table
 */
function initRecentRequestsTable(data) {
    if (recentRequestsTable) {
        recentRequestsTable.destroy();
    }

    const tableData = data.map(request => [
        LogLynxUtils.formatDateTime(request.Timestamp),
        LogLynxUtils.getMethodBadge(request.Method),
        LogLynxUtils.formatHostDisplay(request, '-'),
        `<code>${LogLynxUtils.truncate(request.Path, 60)}</code>`,
        LogLynxUtils.getStatusBadge(request.StatusCode),
        LogLynxUtils.formatDuration(request.ResponseTimeMs),
        LogLynxUtils.formatBytes(request.ResponseSize)
    ]);

    recentRequestsTable = $('#recentRequestsTable').DataTable({
        data: tableData,
        pageLength: 25,
        order: [[0, 'desc']], // Order by time descending
        columnDefs: [
            { targets: [5, 6], className: 'text-end' }
        ],
        ...LogLynxCharts.defaultDataTableOptions
    });
}

/**
 * Change recent requests limit
 */
async function changeRecentRequestsLimit() {
    const limit = parseInt($('#recentRequestsLimit').val());
    const result = await LogLynxAPI.getIPRecentRequests(currentIPAddress, limit);
    if (result.success) {
        ipAnalyticsData.recentRequests = result.data;
        initRecentRequestsTable(result.data);
    }
}

/**
 * IP Search with autocomplete
 */
$('#ipSearchInput').on('input', function() {
    const query = $(this).val().trim();
    
    // Clear previous timer
    if (searchDebounceTimer) {
        clearTimeout(searchDebounceTimer);
    }

    if (query.length < 2) {
        $('#ipSearchResults').hide();
        return;
    }

    // Debounce search
    searchDebounceTimer = setTimeout(async () => {
        const results = await LogLynxAPI.searchIPs(query, 10);
        
        if (results.success && results.data.length > 0) {
            displaySearchResults(results.data);
        } else {
            $('#ipSearchResults').hide();
        }
    }, 300);
});

/**
 * Display search results dropdown
 */
function displaySearchResults(results) {
    const dropdown = $('#ipSearchResults');
    let html = '<div class="list-group">';
    
    results.forEach(result => {
        html += `
            <a href="#" class="list-group-item list-group-item-action" onclick="selectIP('${result.ip_address}'); return false;">
                <div class="d-flex justify-content-between align-items-center">
                    <div>
                        <strong>${result.ip_address}</strong>
                        <br>
                        <small class="text-muted">${result.city || 'Unknown'}, ${result.country || 'Unknown'}</small>
                    </div>
                    <div class="text-end">
                        <span class="badge badge-primary">${LogLynxUtils.formatNumber(result.hits)} hits</span>
                        <br>
                        <small class="text-muted">${LogLynxUtils.formatDateTime(result.last_seen)}</small>
                    </div>
                </div>
            </a>
        `;
    });
    
    html += '</div>';
    dropdown.html(html).show();
}

/**
 * Select IP from search results
 */
function selectIP(ipAddress) {
    $('#ipSearchResults').hide();
    $('#ipSearchInput').val(ipAddress);
    // Navigate to IP page
    window.location.href = `/ip/${ipAddress}`;
}

/**
 * Search and navigate to IP
 */
function searchAndNavigateIP() {
    const ip = $('#ipSearchInput').val().trim();
    if (ip) {
        window.location.href = `/ip/${ip}`;
    }
}

/**
 * Hide search results when clicking outside
 */
$(document).on('click', function(e) {
    if (!$(e.target).closest('#ipSearchInput, #ipSearchResults').length) {
        $('#ipSearchResults').hide();
    }
});

/**
 * Change timeline range
 */
async function changeTimelineRange() {
    const hours = parseInt($('#timelineRange').val());
    const result = await LogLynxAPI.getIPTimeline(currentIPAddress, hours);
    if (result.success) {
        updateTimelineChart(result.data);
    }
}

/**
 * Change heatmap range
 */
async function changeHeatmapRange() {
    const days = parseInt($('#heatmapDays').val());
    const result = await LogLynxAPI.getIPHeatmap(currentIPAddress, days);
    if (result.success) {
        updateHeatmapChart(result.data);
    }
}

/**
 * Export functions
 */
function exportFullReport() {
    if (!currentIPAddress || !ipAnalyticsData.stats) {
        LogLynxUtils.showNotification('No data available to export', 'error');
        return;
    }

    // Create a printable window with comprehensive report
    const printWindow = window.open('', '_blank');
    const stats = ipAnalyticsData.stats;
    
    // Calculate bot percentage
    let botPercentage = 0;
    if (ipAnalyticsData.devices) {
        const totalRequests = ipAnalyticsData.devices.reduce((sum, d) => sum + d.count, 0);
        const botData = ipAnalyticsData.devices.find(d => d.device_type === 'bot');
        botPercentage = botData && totalRequests > 0 ? ((botData.count / totalRequests) * 100).toFixed(1) : 0;
    }
    
    const reportHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>IP Analytics Report - ${currentIPAddress}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; color: #333; }
        h1 { color: #F46319; border-bottom: 3px solid #F46319; padding-bottom: 10px; }
        h2 { color: #555; border-bottom: 1px solid #ddd; padding-bottom: 5px; margin-top: 30px; }
        .header { text-align: center; margin-bottom: 30px; }
        .stat-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; margin: 20px 0; }
        .stat-box { border: 1px solid #ddd; padding: 15px; border-radius: 5px; }
        .stat-box strong { display: block; color: #F46319; font-size: 24px; margin-bottom: 5px; }
        .stat-box span { color: #666; font-size: 14px; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f5f5f5; font-weight: bold; }
        tr:hover { background-color: #f9f9f9; }
        .footer { margin-top: 40px; text-align: center; color: #999; font-size: 12px; }
        @media print {
            body { margin: 20px; }
            .no-print { display: none; }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔍 IP Analytics Report</h1>
        <p><strong>IP Address:</strong> ${currentIPAddress}</p>
        <p><strong>Generated:</strong> ${new Date().toLocaleString()}</p>
    </div>

    <h2>📊 Overview Statistics</h2>
    <div class="stat-grid">
        <div class="stat-box">
            <strong>${LogLynxUtils.formatNumber(stats.total_requests)}</strong>
            <span>Total Requests</span>
        </div>
        <div class="stat-box">
            <strong>${LogLynxUtils.formatBytes(stats.total_bandwidth)}</strong>
            <span>Bandwidth</span>
        </div>
        <div class="stat-box">
            <strong>${LogLynxUtils.formatDuration(stats.avg_response_time)}</strong>
            <span>Avg Response Time</span>
        </div>
        <div class="stat-box">
            <strong>${stats.unique_paths || 0}</strong>
            <span>Unique Paths</span>
        </div>
        <div class="stat-box">
            <strong>${stats.unique_backends || 0}</strong>
            <span>Unique Backends</span>
        </div>
        <div class="stat-box">
            <strong>${stats.error_count || 0}</strong>
            <span>Errors</span>
        </div>
    </div>

    <h2>🌍 Geographic Information</h2>
    <table>
        <tr><th>Country</th><td>${stats.country || 'Unknown'}</td></tr>
        <tr><th>City</th><td>${stats.city || 'Unknown'}</td></tr>
        <tr><th>ISP</th><td>${stats.isp || 'Unknown'}</td></tr>
        <tr><th>Coordinates</th><td>${stats.latitude}, ${stats.longitude}</td></tr>
    </table>

    <h2>🤖 Threat Intelligence</h2>
    <table>
        <tr><th>Bot Traffic</th><td>${botPercentage}%</td></tr>
        <tr><th>Error Rate</th><td>${((stats.error_count / stats.total_requests) * 100).toFixed(2)}%</td></tr>
        <tr><th>First Seen</th><td>${stats.first_seen || 'N/A'}</td></tr>
        <tr><th>Last Seen</th><td>${stats.last_seen || 'N/A'}</td></tr>
    </table>

    ${ipAnalyticsData.backends && ipAnalyticsData.backends.length > 0 ? `
    <h2>🔧 Top Backends</h2>
    <table>
        <thead>
            <tr>
                <th>#</th>
                <th>Backend</th>
                <th>Hits</th>
                <th>Bandwidth</th>
                <th>Avg Response</th>
            </tr>
        </thead>
        <tbody>
            ${ipAnalyticsData.backends.slice(0, 10).map((b, i) => `
                <tr>
                    <td>${i + 1}</td>
                    <td>${b.backend_name || 'N/A'}</td>
                    <td>${LogLynxUtils.formatNumber(b.hits)}</td>
                    <td>${LogLynxUtils.formatBytes(b.bandwidth)}</td>
                    <td>${LogLynxUtils.formatDuration(b.avg_response_time)}</td>
                </tr>
            `).join('')}
        </tbody>
    </table>
    ` : ''}

    ${ipAnalyticsData.paths && ipAnalyticsData.paths.length > 0 ? `
    <h2>📁 Top Paths</h2>
    <table>
        <thead>
            <tr>
                <th>#</th>
                <th>Path</th>
                <th>Hits</th>
                <th>Bandwidth</th>
            </tr>
        </thead>
        <tbody>
            ${ipAnalyticsData.paths.slice(0, 10).map((p, i) => `
                <tr>
                    <td>${i + 1}</td>
                    <td>${p.path}</td>
                    <td>${LogLynxUtils.formatNumber(p.count)}</td>
                    <td>${LogLynxUtils.formatBytes(p.bandwidth)}</td>
                </tr>
            `).join('')}
        </tbody>
    </table>
    ` : ''}

    ${ipAnalyticsData.statusCodes && ipAnalyticsData.statusCodes.length > 0 ? `
    <h2>📊 Status Codes</h2>
    <table>
        <thead>
            <tr>
                <th>Status Code</th>
                <th>Count</th>
                <th>Percentage</th>
            </tr>
        </thead>
        <tbody>
            ${ipAnalyticsData.statusCodes.map(s => `
                <tr>
                    <td>${s.status_code}</td>
                    <td>${LogLynxUtils.formatNumber(s.count)}</td>
                    <td>${((s.count / stats.total_requests) * 100).toFixed(2)}%</td>
                </tr>
            `).join('')}
        </tbody>
    </table>
    ` : ''}

    <div class="footer">
        <p>Generated by LogLynx - Log Analytics Dashboard</p>
        <p>${new Date().toLocaleString()}</p>
        <button class="no-print" onclick="window.print()" style="margin-top: 20px; padding: 10px 20px; background: #F46319; color: white; border: none; border-radius: 5px; cursor: pointer;">Print Report</button>
    </div>
</body>
</html>
    `;
    
    printWindow.document.write(reportHTML);
    printWindow.document.close();
    
    LogLynxUtils.showNotification('Report generated successfully. Click Print in the new window.', 'success');
}

function exportCSV() {
    if (!currentIPAddress || !ipAnalyticsData.stats) {
        LogLynxUtils.showNotification('No data available to export', 'error');
        return;
    }

    const stats = ipAnalyticsData.stats;
    let csvContent = 'IP Analytics Report - ' + currentIPAddress + '\n';
    csvContent += 'Generated: ' + new Date().toLocaleString() + '\n\n';
    
    // Overview
    csvContent += 'OVERVIEW\n';
    csvContent += 'Metric,Value\n';
    csvContent += `Total Requests,${stats.total_requests}\n`;
    csvContent += `Total Bandwidth,${stats.total_bandwidth}\n`;
    csvContent += `Average Response Time,${stats.avg_response_time}\n`;
    csvContent += `Unique Paths,${stats.unique_paths || 0}\n`;
    csvContent += `Unique Backends,${stats.unique_backends || 0}\n`;
    csvContent += `Error Count,${stats.error_count || 0}\n\n`;
    
    // Geographic
    csvContent += 'GEOGRAPHIC INFORMATION\n';
    csvContent += 'Field,Value\n';
    csvContent += `Country,${stats.country || 'Unknown'}\n`;
    csvContent += `City,${stats.city || 'Unknown'}\n`;
    csvContent += `ISP,${stats.isp || 'Unknown'}\n`;
    csvContent += `Latitude,${stats.latitude || 'N/A'}\n`;
    csvContent += `Longitude,${stats.longitude || 'N/A'}\n\n`;
    
    // Top Backends
    if (ipAnalyticsData.backends && ipAnalyticsData.backends.length > 0) {
        csvContent += 'TOP BACKENDS\n';
        csvContent += 'Rank,Backend,Hits,Bandwidth,Avg Response Time\n';
        ipAnalyticsData.backends.forEach((b, i) => {
            csvContent += `${i + 1},"${b.backend_name || 'N/A'}",${b.hits},${b.bandwidth},${b.avg_response_time}\n`;
        });
        csvContent += '\n';
    }
    
    // Top Paths
    if (ipAnalyticsData.paths && ipAnalyticsData.paths.length > 0) {
        csvContent += 'TOP PATHS\n';
        csvContent += 'Rank,Path,Count,Bandwidth\n';
        ipAnalyticsData.paths.forEach((p, i) => {
            csvContent += `${i + 1},"${p.path}",${p.count},${p.bandwidth}\n`;
        });
        csvContent += '\n';
    }
    
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `ip-analytics-${currentIPAddress}-${Date.now()}.csv`;
    link.click();
    URL.revokeObjectURL(link.href);
    
    LogLynxUtils.showNotification('CSV exported successfully', 'success');
}

function exportBackendsData() {
    if (!ipAnalyticsData.backends || ipAnalyticsData.backends.length === 0) {
        LogLynxUtils.showNotification('No backends data available', 'error');
        return;
    }
    
    let csvContent = 'Backend Name,Backend URL,Hits,Bandwidth,Avg Response Time,Errors\n';
    ipAnalyticsData.backends.forEach(b => {
        csvContent += `"${b.backend_name || 'N/A'}","${b.backend_url || 'N/A'}",${b.hits},${b.bandwidth},${b.avg_response_time},${b.error_count || 0}\n`;
    });
    
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `backends-${currentIPAddress}-${Date.now()}.csv`;
    link.click();
    URL.revokeObjectURL(link.href);
    
    LogLynxUtils.showNotification('Backends data exported successfully', 'success');
}

function exportPathsData() {
    if (!ipAnalyticsData.paths || ipAnalyticsData.paths.length === 0) {
        LogLynxUtils.showNotification('No paths data available', 'error');
        return;
    }
    
    let csvContent = 'Backend,Path,Hits,Bandwidth\n';
    ipAnalyticsData.paths.forEach(p => {
        const backend = p.backend_name || p.backend_url || 'N/A';
        csvContent += `"${backend}","${p.path}",${p.hits || p.count},${p.total_bandwidth || p.bandwidth}\n`;
    });
    
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `paths-${currentIPAddress}-${Date.now()}.csv`;
    link.click();
    URL.revokeObjectURL(link.href);
    
    LogLynxUtils.showNotification('Paths data exported successfully', 'success');
}

function exportJSON() {
    if (!currentIPAddress || !ipAnalyticsData.stats) {
        LogLynxUtils.showNotification('No data available to export', 'error');
        return;
    }

    const exportData = {
        ip_address: currentIPAddress,
        generated_at: new Date().toISOString(),
        data: ipAnalyticsData
    };
    
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `ip-analytics-${currentIPAddress}-${Date.now()}.json`;
    a.click();
    URL.revokeObjectURL(url);
    
    LogLynxUtils.showNotification('JSON exported successfully', 'success');
}

function blockIP() {
    if (!currentIPAddress) {
        LogLynxUtils.showNotification('No IP address selected', 'error');
        return;
    }

    // IP blocking functionality not yet implemented
    LogLynxUtils.showNotification('IP blocking feature coming soon', 'info');
}

/**
 * Show/hide loading overlay
 */
function showLoading() {
    $('#loadingOverlay').show();
}

function hideLoading() {
    $('#loadingOverlay').hide();
}
