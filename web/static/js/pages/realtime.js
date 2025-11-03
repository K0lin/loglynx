/**
 * Real-time Monitor Page
 * Live metrics streaming with SSE
 */

let liveChart, perServiceChart;
let eventSource = null;
let updateCount = 0;
let isStreamPaused = false;

// Live chart data (keep last 30 data points = 1 minute at 2sec intervals)
const maxDataPoints = 30;
let liveChartLabels = [];
let liveRequestRateData = [];
let liveAvgResponseData = [];

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
    // Close existing connection
    if (eventSource) {
        eventSource.close();
    }

    // Show connecting status
    showConnectionStatus('Connecting...', 'info');

    // Connect to stream
    eventSource = LogLynxAPI.connectRealtimeStream(
        // On message callback
        (metrics) => {
            if (!isStreamPaused) {
                updateRealtimeMetrics(metrics);
                updateCount++;
                $('#updateCount').text(updateCount);
                $('#lastUpdate').text(LogLynxUtils.formatRelativeTime(metrics.timestamp || new Date()));
            }
        },
        // On error callback
        (error) => {
            console.error('SSE connection error:', error);
            showConnectionStatus('Connection lost. Reconnecting...', 'error');

            // Attempt to reconnect after 5 seconds
            setTimeout(() => {
                if (eventSource.readyState === EventSource.CLOSED) {
                    connectRealtimeStream();
                }
            }, 5000);
        }
    );

    // Connection opened
    eventSource.onopen = () => {
        showConnectionStatus('Connected', 'success');
        setTimeout(() => {
            hideConnectionStatus();
        }, 3000);
    };
}

// Update real-time metrics
function updateRealtimeMetrics(metrics) {
    // Update KPI cards
    $('#liveRequestRate').text(metrics.request_rate.toFixed(2));
    $('#liveErrorRate').text(metrics.error_rate.toFixed(2));
    $('#liveAvgResponse').text(metrics.avg_response_time.toFixed(1) + 'ms');

    // Update status distribution
    $('#live2xx').text(metrics.status_2xx || 0);
    $('#live4xx').text(metrics.status_4xx || 0);
    $('#live5xx').text(metrics.status_5xx || 0);

    // Update live chart
    const now = new Date();
    const timeLabel = now.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });

    liveChartLabels.push(timeLabel);
    liveRequestRateData.push(metrics.request_rate);
    liveAvgResponseData.push(metrics.avg_response_time);

    // Keep only last 30 points
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

    // Add visual feedback
    $('.live-indicator').css('opacity', '1').animate({opacity: 0.3}, 150).animate({opacity: 1}, 150);
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
    // We'll manually update this table with real-time data
    // Start by loading recent requests
    loadRecentRequests();

    // Refresh every 10 seconds
    setInterval(loadRecentRequests, 10000);
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
                <tr class="fade-in">
                    <td>${LogLynxUtils.formatDateTime(req.Timestamp)}</td>
                    <td>${LogLynxUtils.getMethodBadge(req.Method)}</td>
                    <td>${req.Host || '-'}</td>
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
document.addEventListener('DOMContentLoaded', () => {
    // Initialize charts
    initLiveChart();
    initPerServiceChart();

    // Initialize live requests table
    initLiveRequestsTable();

    // Initialize service filter
    initServiceFilterWithReconnect();

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
    }
});
