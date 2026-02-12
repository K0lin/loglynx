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
 * Real-time Monitor Page
 * Live metrics streaming with SSE
 */

let liveChart, perServiceChart, miniLiveChart;
let eventSource = null;
let updateCount = 0;
let isStreamPaused = false;
let liveRequestsInterval = null;
let reconnectTimeout = null;
let pausedMetricsBuffer = []; // Buffer for metrics when paused
const MAX_BUFFER_SIZE = 300; // Max 5 minutes of data

// Mini chart data
const miniChartMaxPoints = 30;
let miniChartData = [];
let miniChartLabels = [];

// Exponential backoff for reconnection
let reconnectAttempts = 0;
const INITIAL_RECONNECT_DELAY = 1000; // Start with 1 second
const MAX_RECONNECT_DELAY = 30000; // Max 30 seconds
const BACKOFF_MULTIPLIER = 2;

// Performance monitoring for SSE
let sseMetrics = {
    messagesReceived: 0,
    connectionTime: null,
    lastMessageTime: null,
    avgMessageInterval: 0,
    messageIntervals: []
};

// Live chart data (keep last 60 data points = 1 minute at 1sec intervals)
const maxDataPoints = 60;
let liveChartLabels = [];
let liveRequestRateData = [];
let liveAvgResponseData = [];
let lastMetricsTimestamp = null;

// Initialize live chart (dual Y-axis)
function initLiveChart() {
    liveChart = LogLynxCharts.createDualAxisChart('liveChart', {
        labels: liveChartLabels,
        datasets: [
            {
                label: 'Request Rate (req/s)',
                data: liveRequestRateData,
                borderColor: '#28a745',
                backgroundColor: 'rgba(40, 167, 69, 0.1)',
                tension: 0.4,
                yAxisID: 'y',
                pointRadius: 2,
                fill: true
            },
            {
                label: 'Avg Response Time (ms)',
                data: liveAvgResponseData,
                borderColor: '#17a2b8',
                backgroundColor: 'rgba(23, 162, 184, 0.1)',
                tension: 0.4,
                yAxisID: 'y1',
                pointRadius: 2,
                fill: true
            }
        ]
    }, {
        scales: {
            y: {
                title: {
                    display: true,
                    text: 'Request Rate (req/s)',
                    color: '#28a745'
                },
                ticks: { color: '#28a745' }
            },
            y1: {
                title: {
                    display: true,
                    text: 'Response Time (ms)',
                    color: '#17a2b8'
                },
                ticks: { color: '#17a2b8' }
            }
        }
    });
}

// Initialize per-service chart
function initPerServiceChart() {
    perServiceChart = LogLynxCharts.createHorizontalBarChart('perServiceChart', {
        labels: [],
        datasets: [{
            label: 'Request Rate (req/s)',
            data: [],
            backgroundColor: 'rgba(244, 99, 25, 0.7)',
            borderColor: '#F36319',
            borderWidth: 1
        }]
    }, {
        plugins: {
            tooltip: {
                callbacks: {
                    label: function(context) {
                        return 'Request Rate: ' + context.parsed.x.toFixed(2) + ' req/s';
                    }
                }
            }
        },
        scales: {
            x: {
                title: {
                    display: true,
                    text: 'Requests per Second',
                    color: '#F3EFF3'
                }
            },
            y: {
                ticks: {
                    font: { size: 10 }
                }
            }
        }
    });
}

// Initialize mini live chart
function initMiniChart() {
    const ctx = document.getElementById('miniLiveChart').getContext('2d');
    
    // Initialize empty data
    for (let i = 0; i < miniChartMaxPoints; i++) {
        miniChartLabels.push('');
        miniChartData.push(0);
    }

    miniLiveChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: miniChartLabels,
            datasets: [{
                data: miniChartData,
                borderColor: '#28a745',
                backgroundColor: 'rgba(40, 167, 69, 0.1)',
                borderWidth: 2,
                tension: 0.4,
                pointRadius: 0,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { display: false },
                tooltip: { enabled: false }
            },
            scales: {
                x: { display: false },
                y: { 
                    display: false,
                    min: 0
                }
            },
            animation: false
        }
    });
}

// Connect to real-time SSE stream
function connectRealtimeStream() {
    // Clear any pending reconnection timeout to prevent race conditions
    if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
        reconnectTimeout = null;
    }

    // Close existing connection
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }

    // Show connecting status
    showConnectionStatus('Connecting...', 'info');

    // Connect to stream
    eventSource = LogLynxAPI.connectRealtimeStream(
        // On message callback
        (metrics) => {
            // Always update mini monitor
            updateMiniMonitor(metrics);

            if (!isStreamPaused) {
                // Update SSE performance metrics
                const now = Date.now();
                sseMetrics.messagesReceived++;

                if (sseMetrics.lastMessageTime) {
                    const interval = now - sseMetrics.lastMessageTime;
                    sseMetrics.messageIntervals.push(interval);

                    // Keep last 10 intervals for average calculation
                    if (sseMetrics.messageIntervals.length > 10) {
                        sseMetrics.messageIntervals.shift();
                    }

                    // Calculate average interval
                    sseMetrics.avgMessageInterval =
                        sseMetrics.messageIntervals.reduce((a, b) => a + b, 0) /
                        sseMetrics.messageIntervals.length;
                }

                sseMetrics.lastMessageTime = now;

                updateRealtimeMetrics(metrics);
                updateCount++;
                $('#updateCount').text(updateCount);
            } else {
                // Buffer metrics when stream is paused
                if (pausedMetricsBuffer.length < MAX_BUFFER_SIZE) {
                    pausedMetricsBuffer.push(metrics);
                }
            }
        },
        // On error callback
        (error) => {
            console.error('SSE connection error:', error);

            // Increment reconnect attempts for exponential backoff
            reconnectAttempts++;

            // Calculate delay with exponential backoff
            const delay = Math.min(
                INITIAL_RECONNECT_DELAY * Math.pow(BACKOFF_MULTIPLIER, reconnectAttempts - 1),
                MAX_RECONNECT_DELAY
            );

            showConnectionStatus(`Connection lost. Reconnecting in ${Math.round(delay / 1000)}s...`, 'error');

            // Clear any existing timeout
            if (reconnectTimeout) {
                clearTimeout(reconnectTimeout);
            }

            // Attempt to reconnect with exponential backoff
            reconnectTimeout = setTimeout(() => {
                if (eventSource && eventSource.readyState === EventSource.CLOSED) {
                    connectRealtimeStream();
                }
                reconnectTimeout = null;
            }, delay);
        }
    );

    // Connection opened
    eventSource.onopen = () => {
        // Reset reconnect attempts on successful connection
        reconnectAttempts = 0;

        // Track connection time for performance monitoring
        sseMetrics.connectionTime = Date.now();

        showConnectionStatus('Connected', 'success');
        setTimeout(() => {
            hideConnectionStatus();
        }, 3000);
    };
}

// Get SSE performance metrics (accessible via browser console for debugging)
function getSSEMetrics() {
    const uptime = sseMetrics.connectionTime
        ? ((Date.now() - sseMetrics.connectionTime) / 1000).toFixed(1)
        : 0;

    return {
        messagesReceived: sseMetrics.messagesReceived,
        avgMessageInterval: sseMetrics.avgMessageInterval.toFixed(0) + 'ms',
        uptime: uptime + 's',
        reconnectAttempts,
        apiMetrics: LogLynxAPI.getPerformanceMetrics()
    };
}

// Make available globally for debugging
window.getSSEMetrics = getSSEMetrics;

// Update real-time metrics
function updateRealtimeMetrics(metrics) {
    // Validate metrics timestamp to detect stale data
    const now = new Date();
    let metricsTimestamp = metrics.timestamp ? new Date(metrics.timestamp) : now;

    // Check if metrics are stale (older than 5 seconds - increased tolerance)
    const metricsAge = (now - metricsTimestamp) / 1000; // in seconds
    const isStale = metricsAge > 5;

    // Skip if truly stale (but allow duplicate timestamps from same second)
    if (isStale && lastMetricsTimestamp && metricsTimestamp.getTime() === lastMetricsTimestamp.getTime()) {
        console.debug('Skipping stale metrics update', {age: metricsAge, timestamp: metricsTimestamp});
        return;
    }
    lastMetricsTimestamp = metricsTimestamp;

    // Update KPI cards
    $('#liveRequestRate').text(metrics.request_rate.toFixed(2));
    $('#liveErrorRate').text(metrics.error_rate.toFixed(2));
    $('#liveAvgResponse').text(metrics.avg_response_time.toFixed(1) + 'ms');

    // Update status distribution
    $('#live2xx').text(metrics.status_2xx || 0);
    $('#live4xx').text(metrics.status_4xx || 0);
    $('#live5xx').text(metrics.status_5xx || 0);

    // Update live chart with millisecond precision to avoid duplicate keys
    const timeLabel = LogLynxUtils.formatTime(metricsTimestamp, { second: '2-digit' });

    // Use timestamp millis as unique key internally
    const uniqueKey = metricsTimestamp.getTime();

    // Check if this exact timestamp was already added
    if (liveChartLabels.length > 0) {
        const lastKey = liveChartLabels[liveChartLabels.length - 1]._uniqueKey;
        if (lastKey === uniqueKey) {
            // Same exact millisecond - update the last point
            liveRequestRateData[liveRequestRateData.length - 1] = metrics.request_rate;
            liveAvgResponseData[liveAvgResponseData.length - 1] = metrics.avg_response_time;
        } else {
            // New data point
            const labelWithKey = timeLabel;
            labelWithKey._uniqueKey = uniqueKey;
            liveChartLabels.push(labelWithKey);
            liveRequestRateData.push(metrics.request_rate);
            liveAvgResponseData.push(metrics.avg_response_time);
        }
    } else {
        // First data point
        const labelWithKey = timeLabel;
        labelWithKey._uniqueKey = uniqueKey;
        liveChartLabels.push(labelWithKey);
        liveRequestRateData.push(metrics.request_rate);
        liveAvgResponseData.push(metrics.avg_response_time);
    }

    // Keep only last 60 points (1 minute at 1sec intervals)
    if (liveChartLabels.length > maxDataPoints) {
        liveChartLabels.shift();
        liveRequestRateData.shift();
        liveAvgResponseData.shift();
    }

    if (liveChart) {
        liveChart.data.labels = liveChartLabels;
        liveChart.data.datasets[0].data = liveRequestRateData;
        liveChart.data.datasets[1].data = liveAvgResponseData;
        liveChart.update('none'); // No animation for smooth real-time updates
    }

    // Update per-service metrics (with null check)
    if (metrics.per_service !== undefined && metrics.per_service !== null) {
        updatePerServiceMetrics(metrics.per_service);
    }

    // Update Top IPs table (always update, even if empty/null to clear stale data)
    updateTopIPsTable(metrics.top_ips || []);

    // Update Live Requests Table (Prepend new requests)
    if (metrics.latest_requests && metrics.latest_requests.length > 0) {
        prependLatestRequests(metrics.latest_requests);
    }

    // Add visual feedback (with stale indicator if data is old)
    if (isStale) {
        $('.live-indicator').css('opacity', '0.5'); // Dimmed for stale data
    } else {
        $('.live-indicator').css('opacity', '1').animate({opacity: 0.3}, 150).animate({opacity: 1}, 150);
    }
}

// Prepend latest requests to table
function prependLatestRequests(requests) {
    const tbody = $('#liveRequestsBody');
    
    // Remove "No requests yet" row if present
    if (tbody.find('td[colspan="8"]').length > 0) {
        tbody.empty();
    }

    // Initialize seen IDs set if not exists
    if (!window.seenRequestIds) {
        window.seenRequestIds = new Set();
        // Populate from existing rows
        tbody.find('tr').each(function() {
            const id = $(this).data('id');
            if (id !== undefined && id !== null) window.seenRequestIds.add(parseInt(id));
        });
    }

    // Process requests (received in newest-first order from backend)
    // Iterate in reverse to maintain correct chronological order when prepending
    for (let i = requests.length - 1; i >= 0; i--) {
        const req = requests[i];
        if (window.seenRequestIds.has(req.id)) continue;
        window.seenRequestIds.add(req.id);

        const row = `
            <tr class="fade-in" data-id="${req.id}">
                <td>${LogLynxUtils.formatDateTime(req.timestamp)}</td>
                <td>${LogLynxUtils.getMethodBadge(req.method)}</td>
                <td>${LogLynxUtils.formatHostDisplay(req, '-')}</td>
                <td><code>${LogLynxUtils.truncate(req.path, 40)}</code></td>
                <td>${LogLynxUtils.getStatusBadge(req.status_code)}</td>
                <td>${LogLynxUtils.formatMs(req.response_time_ms || 0)}</td>
                <td>${req.geo_country ? `<span>${countryCodeToFlag(req.geo_country, req.geo_country)} ${countryToContinentMap[req.geo_country]?.name || 'Unknown'}</span>, <small class='text-muted'>${countryToContinentMap[req.geo_country]?.continent || 'Unknown'}</small>` : '-'}</td>
                <td>${req.client_ip}</td>
            </tr>
        `;
        tbody.prepend(row);
    }

    // Limit to 50 rows
    const rows = tbody.find('tr');
    if (rows.length > 50) {
        rows.slice(50).remove();
        // Rebuild Set to keep memory usage low
        window.seenRequestIds.clear();
        tbody.find('tr').each(function() {
            const id = $(this).data('id');
            if (id !== undefined && id !== null) window.seenRequestIds.add(parseInt(id));
        });
    }
}

// Update Top IPs Table
function updateTopIPsTable(topIPs) {
    const tbody = $('#topIPsBody');
    
    if (!topIPs || topIPs.length === 0) {
        tbody.html('<tr><td colspan="3" class="text-center text-muted">No active clients</td></tr>');
        return;
    }

    let html = '';
    topIPs.forEach(ip => {
        // Calculate width for progress bar background
        // Assuming max rate is the first one since it's sorted
        const maxRate = topIPs[0].request_rate;
        const percent = (ip.request_rate / maxRate) * 100;
        
        html += `
            <tr>
                <td>
                    <a href="/ip/${ip.ip}" class="text-decoration-none">
                        <code>${ip.ip}</code>
                    </a>
                </td>
                <td>
                    ${ip.country ? `<span>${countryCodeToFlag(ip.country, ip.country)} ${countryToContinentMap[ip.country]?.name || 'Unknown'}</span>, <small class='text-muted'>${countryToContinentMap[ip.country]?.continent || 'Unknown'}</small>` : '<span class="text-muted">-</span>'}
                </td>
                <td class="text-end">
                    <div class="d-flex align-items-center justify-content-end gap-2">
                        <span class="fw-bold">${ip.request_rate.toFixed(1)}</span>
                        <div class="progress" style="width: 50px; height: 4px;">
                            <div class="progress-bar bg-success" role="progressbar" style="width: ${percent}%"></div>
                        </div>
                    </div>
                </td>
            </tr>
        `;
    });

    tbody.html(html);
}

// Update per-service metrics
function updatePerServiceMetrics(services) {
    // Always keep the section visible
    $('#perServiceSection').show();

    if (services && services.length > 0) {
        // Sort by request rate descending
        services.sort((a, b) => b.request_rate - a.request_rate);

        if (perServiceChart) {
            perServiceChart.data.labels = services.map(s => s.service_name);
            perServiceChart.data.datasets[0].data = services.map(s => s.request_rate);
            perServiceChart.update('none');
        }
    } else {
        // No data - show empty chart with message
        if (perServiceChart) {
            perServiceChart.data.labels = ['No services with activity'];
            perServiceChart.data.datasets[0].data = [0];
            perServiceChart.update('none');
        }
    }
}

// Show connection status notification
function showConnectionStatus(message, type) {
    const notification = $('#connectionStatus');
    notification.removeClass('notification-success notification-error notification-info notification-warning');
    notification.addClass(`notification-${type}`);
    $('#connectionStatusText').text(message);
    notification.fadeIn();
}

// Hide connection status
function hideConnectionStatus() {
    $('#connectionStatus').fadeOut();
}

// Initialize DataTable for live requests
function initLiveRequestsTable() {
    // Clear any existing interval to prevent leaks
    if (liveRequestsInterval) {
        clearInterval(liveRequestsInterval);
    }

    // We'll manually update this table with real-time data
    // Start by loading recent requests
    loadRecentRequests();
}

// Load recent requests
async function loadRecentRequests() {
    if (isStreamPaused) return;

    const result = await LogLynxAPI.getRecentRequests(50);

    if (result.success && result.data) {
        updateLiveRequestsTable(result.data);
    }
}

// Update live requests table
function updateLiveRequestsTable(requests) {
    const tbody = $('#liveRequestsBody');
    let html = '';

    if (!requests || requests.length === 0) {
        html = '<tr><td colspan="8" class="text-center text-muted">No requests yet</td></tr>';
    } else {
        requests.forEach(req => {
            html += `
                <tr class="fade-in" data-id="${req.ID}">
                    <td>${LogLynxUtils.formatDateTime(req.Timestamp)}</td>
                    <td>${LogLynxUtils.getMethodBadge(req.Method)}</td>
                    <td>${LogLynxUtils.formatHostDisplay(req, '-')}</td>
                    <td><code>${LogLynxUtils.truncate(req.Path, 40)}</code></td>
                    <td>${LogLynxUtils.getStatusBadge(req.StatusCode)}</td>
                    <td>${LogLynxUtils.formatMs(req.ResponseTimeMs || 0)}</td>
                    <td>${req.GeoCountry ? `<span>${countryCodeToFlag(req.GeoCountry, req.GeoCountry)} ${countryToContinentMap[req.GeoCountry]?.name || 'Unknown'}</span>, <small class='text-muted'>${countryToContinentMap[req.GeoCountry]?.continent || 'Unknown'}</small>` : '-'}</td>
                    <td>${req.ClientIP}</td>
                </tr>
            `;
        });
    }

    tbody.html(html);
}

// Update Mini Monitor
function updateMiniMonitor(metrics) {
    // Update value
    $('#miniRequestRate').text(metrics.request_rate.toFixed(1));
    $('#miniErrorRate').text(metrics.error_rate.toFixed(2));
    $('#miniAvgResponse').text(metrics.avg_response_time.toFixed(0));

    // Update chart
    if (miniLiveChart) {
        miniChartData.push(metrics.request_rate);
        miniChartLabels.push('');
        
        if (miniChartData.length > miniChartMaxPoints) {
            miniChartData.shift();
            miniChartLabels.shift();
        }
        
        miniLiveChart.data.datasets[0].data = miniChartData;
        miniLiveChart.update('none');
    }
}

// Pause/resume stream
function toggleStreamPause() {
    isStreamPaused = !isStreamPaused;
    const btns = $('.pause-stream-btn');
    const indicator = $('.live-indicator');
    const miniMonitor = $('#miniLiveMonitor');

    if (isStreamPaused) {
        btns.html('<i class="fas fa-play"></i> Resume');
        btns.removeClass('btn-outline').addClass('btn-primary');
        
        // Update indicator
        indicator.addClass('paused').html('<i class="fas fa-pause" style="font-size: 0.7em; margin-right: 4px;"></i> PAUSED');
        
        // Show mini monitor
        if (!isMiniMonitorHidden) {
            miniMonitor.fadeIn(300);
        } else {
            $('#miniMonitorRestore').css('display', 'flex');
        }
        
        // Clear buffer when starting pause
        pausedMetricsBuffer = [];
        
        LogLynxUtils.showNotification('Stream paused - buffering data in background', 'info', 2000);
    } else {
        btns.html('<i class="fas fa-pause"></i> Pause');
        btns.removeClass('btn-primary').addClass('btn-outline');
        
        // Restore indicator
        indicator.removeClass('paused').html('<span class="live-indicator-dot"></span> STREAMING');
        
        // Hide mini monitor
        miniMonitor.fadeOut(300);
        $('#miniMonitorRestore').fadeOut(300);
        
        // Process buffered data
        if (pausedMetricsBuffer.length > 0) {
            LogLynxUtils.showNotification(`Resumed - catching up ${pausedMetricsBuffer.length} seconds of data...`, 'success', 2000);
            processBufferedMetrics();
        } else {
            LogLynxUtils.showNotification('Stream resumed', 'success', 2000);
        }
    }
}

// Process buffered metrics to fill gaps
function processBufferedMetrics() {
    if (pausedMetricsBuffer.length === 0) return;

    // 1. Update Charts Data Arrays (History)
    pausedMetricsBuffer.forEach(metrics => {
        const metricsTimestamp = metrics.timestamp ? new Date(metrics.timestamp) : new Date();
        const timeLabel = LogLynxUtils.formatTime(metricsTimestamp, { second: '2-digit' });

        // Logic to add to arrays
        if (liveChartLabels.length > 0 && liveChartLabels[liveChartLabels.length - 1] === timeLabel) {
             liveRequestRateData[liveRequestRateData.length - 1] = metrics.request_rate;
             liveAvgResponseData[liveAvgResponseData.length - 1] = metrics.avg_response_time;
        } else {
            liveChartLabels.push(timeLabel);
            liveRequestRateData.push(metrics.request_rate);
            liveAvgResponseData.push(metrics.avg_response_time);
        }
        
        // Maintain max points
        if (liveChartLabels.length > maxDataPoints) {
            liveChartLabels.shift();
            liveRequestRateData.shift();
            liveAvgResponseData.shift();
        }
    });

    // 2. Update Live Chart (Once)
    if (liveChart) {
        liveChart.data.labels = liveChartLabels;
        liveChart.data.datasets[0].data = liveRequestRateData;
        liveChart.data.datasets[1].data = liveAvgResponseData;
        liveChart.update('none');
    }

    // 3. Process Requests (All buffered packets)
    pausedMetricsBuffer.forEach(metrics => {
        if (metrics.latest_requests && metrics.latest_requests.length > 0) {
            prependLatestRequests(metrics.latest_requests);
        }
    });

    // 4. Update Current State (KPIs, PerService, TopIPs) using the LAST metric
    const lastMetric = pausedMetricsBuffer[pausedMetricsBuffer.length - 1];
    
    // Update KPIs
    $('#liveRequestRate').text(lastMetric.request_rate.toFixed(2));
    $('#liveErrorRate').text(lastMetric.error_rate.toFixed(2));
    $('#liveAvgResponse').text(lastMetric.avg_response_time.toFixed(1) + 'ms');
    $('#live2xx').text(lastMetric.status_2xx || 0);
    $('#live4xx').text(lastMetric.status_4xx || 0);
    $('#live5xx').text(lastMetric.status_5xx || 0);

    // Update Per Service
    if (lastMetric.per_service) {
        updatePerServiceMetrics(lastMetric.per_service);
    }
    
    // Update Top IPs
    updateTopIPsTable(lastMetric.top_ips);
    
    // Update timestamp
    if (lastMetric.timestamp) {
        lastMetricsTimestamp = new Date(lastMetric.timestamp);
    }
    
    // Update count
    updateCount += pausedMetricsBuffer.length;
    $('#updateCount').text(updateCount);
    
    // Clear buffer
    pausedMetricsBuffer = [];
}

// Clear live data
function clearLiveData() {
    liveChartLabels = [];
    liveRequestRateData = [];
    liveAvgResponseData = [];

    if (liveChart) {
        liveChart.data.labels = [];
        liveChart.data.datasets[0].data = [];
        liveChart.data.datasets[1].data = [];
        liveChart.update('none');
    }

    $('#liveRequestsBody').html('<tr><td colspan="8" class="text-center text-muted">Stream cleared</td></tr>');

    updateCount = 0;
    $('#updateCount').text('0');

    LogLynxUtils.showNotification('Stream data cleared', 'info', 2000);
}

// Export per-service chart
function exportPerServiceChart() {
    if (perServiceChart) {
        const canvas = document.getElementById('perServiceChart');
        LogLynxUtils.exportChartAsImage(canvas, 'per-service-metrics.png');
    }
}

// Initialize service filter with reconnect
function initServiceFilterWithReconnect() {
    LogLynxUtils.initServiceFilter(() => {
        // Reconnect stream with new filter
        connectRealtimeStream();

        // Reload live requests
        loadRecentRequests();
    });
}

// Initialize event listeners
function initEventListeners() {
    $('#reconnectStream').on('click', () => {
        connectRealtimeStream();
        LogLynxUtils.showNotification('Reconnecting to stream...', 'info', 2000);
    });

    $('.pause-stream-btn').on('click', toggleStreamPause);

    $('#clearStream').on('click', () => {
        if (confirm('Clear all live stream data?')) {
            clearLiveData();
        }
    });
}

// Mini Monitor State
let isMiniMonitorHidden = false;
let isDragging = false;
let dragStartX, dragStartY;
let initialLeft, initialTop;

// Hide Mini Monitor
function hideMiniMonitor() {
    isMiniMonitorHidden = true;
    const monitor = $('#miniLiveMonitor');
    
    // Calculate distance to move off-screen to the right
    const rect = monitor[0].getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const moveDistance = viewportWidth - rect.left;
    
    monitor.css('transform', `translateX(${moveDistance}px)`);
    monitor.addClass('hidden');
    
    // Show restore tab after animation
    setTimeout(() => {
        if (isStreamPaused && isMiniMonitorHidden) {
            const restoreTab = $('#miniMonitorRestore');
            restoreTab.css('display', 'flex').hide().fadeIn(200);
            
            // Position restore tab at the same vertical level
            restoreTab.css('top', rect.top + 'px').css('bottom', 'auto');
        }
    }, 300);
}

// Show Mini Monitor
function showMiniMonitor() {
    isMiniMonitorHidden = false;
    $('#miniMonitorRestore').fadeOut(200, () => {
        const monitor = $('#miniLiveMonitor');
        monitor.css('transform', ''); // Clear inline transform
        monitor.removeClass('hidden');
    });
}

// Initialize Draggable Mini Monitor
function initDraggableMonitor() {
    const monitor = document.getElementById('miniLiveMonitor');
    const header = document.getElementById('miniMonitorHeader');
    
    if (!monitor || !header) return;

    header.addEventListener('mousedown', dragStart);
    document.addEventListener('mousemove', drag);
    document.addEventListener('mouseup', dragEnd);

    function dragStart(e) {
        // Ignore clicks on buttons
        if (e.target.closest('button') || e.target.closest('.mini-monitor-btn')) return;
        
        initialLeft = monitor.offsetLeft;
        initialTop = monitor.offsetTop;
        dragStartX = e.clientX;
        dragStartY = e.clientY;
        
        // If it was positioned with bottom/right, convert to top/left for dragging
        const rect = monitor.getBoundingClientRect();
        monitor.style.bottom = 'auto';
        monitor.style.right = 'auto';
        monitor.style.left = rect.left + 'px';
        monitor.style.top = rect.top + 'px';
        
        initialLeft = rect.left;
        initialTop = rect.top;

        isDragging = true;
        monitor.classList.add('dragging');
    }

    function drag(e) {
        if (!isDragging) return;
        e.preventDefault();
        
        const currentX = e.clientX - dragStartX;
        const currentY = e.clientY - dragStartY;

        let newLeft = initialLeft + currentX;
        let newTop = initialTop + currentY;
        
        // Boundary checks
        const maxLeft = window.innerWidth - monitor.offsetWidth;
        const maxTop = window.innerHeight - monitor.offsetHeight;
        
        newLeft = Math.max(0, Math.min(newLeft, maxLeft));
        newTop = Math.max(0, Math.min(newTop, maxTop));

        monitor.style.left = newLeft + 'px';
        monitor.style.top = newTop + 'px';
    }

    function dragEnd(e) {
        if (!isDragging) return;
        initialLeft = monitor.offsetLeft;
        initialTop = monitor.offsetTop;
        isDragging = false;
        monitor.classList.remove('dragging');
    }
}

// Make available globally
window.hideMiniMonitor = hideMiniMonitor;
window.showMiniMonitor = showMiniMonitor;

// Initialize page
// Initialize hide my traffic filter with reconnect callback
function initHideTrafficFilterWithReconnect() {
    LogLynxUtils.initHideMyTrafficFilter(() => {
        // Reconnect to stream with new filter
        if (eventSource) {
            eventSource.close();
        }
        connectRealtimeStream();
    });
}

document.addEventListener('DOMContentLoaded', () => {
    // Initialize charts
    initLiveChart();
    initPerServiceChart();
    initMiniChart();

    // Initialize live requests table
    initLiveRequestsTable();

    // Initialize service filter
    initServiceFilterWithReconnect();
    initHideTrafficFilterWithReconnect();

    // Initialize event listeners
    initEventListeners();
    
    // Initialize draggable monitor
    initDraggableMonitor();

    // Connect to real-time stream
    connectRealtimeStream();

    // Note: No auto-refresh needed for this page as it uses SSE streaming
    // Disable the header refresh controls for this page
    $('#refreshInterval').prop('disabled', true);
    $('#playRefresh').prop('disabled', true);
    $('#pauseRefresh').prop('disabled', true);
    $('#refreshStatus').html('<i class="fas fa-broadcast-tower"></i> <span>Live Streaming</span>');
});

// Clean up on page unload
window.addEventListener('beforeunload', () => {
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }
    if (liveRequestsInterval) {
        clearInterval(liveRequestsInterval);
        liveRequestsInterval = null;
    }
    if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
        reconnectTimeout = null;
    }
});

