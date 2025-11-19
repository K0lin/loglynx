/**
 * Real-time Monitor Page
 * Live metrics streaming with SSE
 */

let liveChart, perServiceChart;
let eventSource = null;
let updateCount = 0;
let isStreamPaused = false;
let liveRequestsInterval = null;
let reconnectTimeout = null;

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
                $('#lastUpdate').text(LogLynxUtils.formatRelativeTime(metrics.timestamp || new Date()));
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

    // Check if metrics are stale (older than 3 seconds)
    const metricsAge = (now - metricsTimestamp) / 1000; // in seconds
    const isStale = metricsAge > 3;

    // If same timestamp as before and stale, skip update to avoid showing old data
    if (lastMetricsTimestamp && metricsTimestamp.getTime() === lastMetricsTimestamp.getTime() && isStale) {
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

    // Update live chart
    const timeLabel = metricsTimestamp.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });

    // Avoid duplicate points for the same timestamp
    if (liveChartLabels.length > 0 && liveChartLabels[liveChartLabels.length - 1] === timeLabel) {
        // Update the last point instead of adding a new one
        liveRequestRateData[liveRequestRateData.length - 1] = metrics.request_rate;
        liveAvgResponseData[liveAvgResponseData.length - 1] = metrics.avg_response_time;
    } else {
        liveChartLabels.push(timeLabel);
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

    // Update per-service metrics
    updatePerServiceMetrics();

    // Update Top IPs table
    updateTopIPsTable(metrics.top_ips);

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
            if (id) window.seenRequestIds.add(parseInt(id));
        });
    }

    // Process requests
    requests.forEach(req => {
        if (window.seenRequestIds.has(req.id)) return;
        window.seenRequestIds.add(req.id);

        const row = `
            <tr class="fade-in" data-id="${req.id}">
                <td>${LogLynxUtils.formatDateTime(req.timestamp)}</td>
                <td>${LogLynxUtils.getMethodBadge(req.method)}</td>
                <td>${LogLynxUtils.formatHostDisplay(req, '-')}</td>
                <td><code>${LogLynxUtils.truncate(req.path, 40)}</code></td>
                <td>${LogLynxUtils.getStatusBadge(req.status_code)}</td>
                <td>${LogLynxUtils.formatMs(req.response_time_ms || 0)}</td>
                <td>${req.geo_country || '-'}</td>
                <td>${req.client_ip}</td>
            </tr>
        `;
        tbody.prepend(row);
    });

    // Limit to 50 rows
    const rows = tbody.find('tr');
    if (rows.length > 50) {
        rows.slice(50).remove();
        // Rebuild Set to keep memory usage low
        window.seenRequestIds.clear();
        tbody.find('tr').each(function() {
            const id = $(this).data('id');
            if (id) window.seenRequestIds.add(parseInt(id));
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
                    ${ip.country ? `<span class="badge bg-secondary">${ip.country}</span>` : '<span class="text-muted">-</span>'}
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
async function updatePerServiceMetrics() {
    const result = await LogLynxAPI.getPerServiceMetrics();

    // Always keep the section visible
    $('#perServiceSection').show();

    if (result.success && result.data && result.data.length > 0) {
        const services = result.data;

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
                    <td>${req.GeoCountry || '-'}</td>
                    <td>${req.ClientIP}</td>
                </tr>
            `;
        });
    }

    tbody.html(html);
}

// Pause/resume stream
function toggleStreamPause() {
    isStreamPaused = !isStreamPaused;
    const btn = $('#pauseStream');

    if (isStreamPaused) {
        btn.html('<i class="fas fa-play"></i> Resume');
        btn.removeClass('btn-outline').addClass('btn-primary');
        LogLynxUtils.showNotification('Stream paused', 'info', 2000);
    } else {
        btn.html('<i class="fas fa-pause"></i> Pause');
        btn.removeClass('btn-primary').addClass('btn-outline');
        LogLynxUtils.showNotification('Stream resumed', 'success', 2000);
    }
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

    $('#pauseStream').on('click', toggleStreamPause);

    $('#clearStream').on('click', () => {
        if (confirm('Clear all live stream data?')) {
            clearLiveData();
        }
    });
}

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

    // Initialize live requests table
    initLiveRequestsTable();

    // Initialize service filter
    initServiceFilterWithReconnect();
    initHideTrafficFilterWithReconnect();

    // Initialize event listeners
    initEventListeners();

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
