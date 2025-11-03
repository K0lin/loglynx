/**
 * Overview Dashboard Page
 */

let timelineChart, statusChart, statusTimelineChart;
let currentTimeRange = 168; // Default 7 days

// Load all dashboard data
async function loadDashboardData() {
    console.log('Loading overview dashboard data...');

    try {
        // Load summary stats
        const summaryResult = await LogLynxAPI.getSummary();
        if (summaryResult.success) {
            updateSummaryCards(summaryResult.data);
        }

        // Load timeline data
        const timelineHours = currentTimeRange === 'all' ? 8760 : currentTimeRange;
        const timelineResult = await LogLynxAPI.getTimeline(timelineHours);
        if (timelineResult.success) {
            updateTimelineChart(timelineResult.data);
        }

        // Load status code timeline
        const statusTimelineResult = await LogLynxAPI.getStatusCodeTimeline(timelineHours);
        if (statusTimelineResult.success) {
            updateStatusTimelineChart(statusTimelineResult.data);
        }

        // Load status code distribution
        const statusDistResult = await LogLynxAPI.getStatusCodeDistribution();
        if (statusDistResult.success) {
            updateStatusChart(statusDistResult.data);
        }

        // Load top countries
        const countriesResult = await LogLynxAPI.getTopCountries(5);
        if (countriesResult.success) {
            updateTopCountriesTable(countriesResult.data);
        }

        // Load top paths
        const pathsResult = await LogLynxAPI.getTopPaths(5);
        if (pathsResult.success) {
            updateTopPathsTable(pathsResult.data);
        }

        // Reload DataTable
        if ($.fn.DataTable.isDataTable('#recentRequestsTable')) {
            $('#recentRequestsTable').DataTable().ajax.reload(null, false);
        }

    } catch (error) {
        console.error('Error loading dashboard data:', error);
        LogLynxUtils.showNotification('Failed to load dashboard data', 'error');
    }
}

// Update summary KPI cards
function updateSummaryCards(data) {
    $('#totalRequests').text(LogLynxUtils.formatNumber(data.total_requests || 0));
    $('#uniqueVisitors').text(LogLynxUtils.formatNumber(data.unique_visitors || 0));
    $('#avgResponseTime').text(LogLynxUtils.formatMs(data.avg_response_time || 0));
    $('#unique404').text(LogLynxUtils.formatNumber(data.unique_404 || 0));
    $('#totalBandwidth').text(LogLynxCharts.formatBytes(data.total_bandwidth || 0));

    // Calculate rates
    const total = data.total_requests || 0;
    const valid = data.valid_requests || 0;
    const failed = data.failed_requests || 0;

    const successRate = total > 0 ? ((valid / total) * 100).toFixed(1) : 0;
    const errorRate = total > 0 ? ((failed / total) * 100).toFixed(1) : 0;

    $('#successRate').text(successRate + '%');
    $('#errorRate').text(errorRate + '%');

    $('#requestsPerHour').text(LogLynxUtils.formatNumber(Math.round(data.requests_per_hour || 0)));
}

// Initialize timeline chart
function initTimelineChart() {
    timelineChart = LogLynxCharts.createLineChart('timelineChart', {
        labels: [],
        datasets: [{
            label: 'Requests',
            data: [],
            borderColor: LogLynxCharts.colors.primary,
            backgroundColor: LogLynxCharts.colors.primaryLight + '40',
            tension: 0.4,
            fill: true
        }]
    }, {
        plugins: {
            legend: { display: false }
        },
        scales: {
            x: {
                ticks: {
                    maxTicksLimit: 12,
                    autoSkip: true
                }
            }
        }
    });
}

// Update timeline chart
function updateTimelineChart(data) {
    if (!data || data.length === 0) {
        if (timelineChart) {
            timelineChart.data.labels = [];
            timelineChart.data.datasets[0].data = [];
            timelineChart.update('none');
        }
        return;
    }

    const labels = LogLynxCharts.formatTimelineLabels(data, currentTimeRange);
    const requests = data.map(d => d.requests);

    if (timelineChart) {
        timelineChart.data.labels = labels;
        timelineChart.data.datasets[0].data = requests;
        timelineChart.update('none');
    }
}

// Initialize status chart
function initStatusChart() {
    statusChart = LogLynxCharts.createDoughnutChart('statusChart', {
        labels: ['2xx', '3xx', '4xx', '5xx'],
        datasets: [{
            data: [0, 0, 0, 0],
            backgroundColor: [
                LogLynxCharts.colors.http2xx,
                LogLynxCharts.colors.http3xx,
                LogLynxCharts.colors.http4xx,
                LogLynxCharts.colors.http5xx
            ]
        }]
    });
}

// Update status chart
function updateStatusChart(data) {
    if (!data || data.length === 0) {
        if (statusChart) {
            statusChart.data.datasets[0].data = [0, 0, 0, 0];
            statusChart.update();
        }
        return;
    }

    const grouped = { '2xx': 0, '3xx': 0, '4xx': 0, '5xx': 0 };

    data.forEach(d => {
        const code = d.status_code;
        if (code >= 200 && code < 300) grouped['2xx'] += d.count;
        else if (code >= 300 && code < 400) grouped['3xx'] += d.count;
        else if (code >= 400 && code < 500) grouped['4xx'] += d.count;
        else if (code >= 500) grouped['5xx'] += d.count;
    });

    if (statusChart) {
        statusChart.data.datasets[0].data = [
            grouped['2xx'],
            grouped['3xx'],
            grouped['4xx'],
            grouped['5xx']
        ];
        statusChart.update();
    }
}

// Initialize status timeline chart
function initStatusTimelineChart() {
    statusTimelineChart = LogLynxCharts.createStackedAreaChart('statusTimelineChart', {
        labels: [],
        datasets: [
            {
                label: '2xx',
                data: [],
                borderColor: LogLynxCharts.colors.http2xx,
                backgroundColor: LogLynxCharts.colors.http2xx + '40',
                tension: 0.3,
                fill: true,
                pointRadius: 0
            },
            {
                label: '3xx',
                data: [],
                borderColor: LogLynxCharts.colors.http3xx,
                backgroundColor: LogLynxCharts.colors.http3xx + '40',
                tension: 0.3,
                fill: true,
                pointRadius: 0
            },
            {
                label: '4xx',
                data: [],
                borderColor: LogLynxCharts.colors.http4xx,
                backgroundColor: LogLynxCharts.colors.http4xx + '40',
                tension: 0.3,
                fill: true,
                pointRadius: 0
            },
            {
                label: '5xx',
                data: [],
                borderColor: LogLynxCharts.colors.http5xx,
                backgroundColor: LogLynxCharts.colors.http5xx + '40',
                tension: 0.3,
                fill: true,
                pointRadius: 0
            }
        ]
    }, {
        plugins: {
            legend: {
                position: 'top',
                labels: {
                    color: '#F3EFF3',
                    font: { size: 11 }
                }
            }
        }
    });
}

// Update status timeline chart
function updateStatusTimelineChart(data) {
    if (!data || data.length === 0) {
        if (statusTimelineChart) {
            statusTimelineChart.data.labels = [];
            statusTimelineChart.data.datasets.forEach(ds => ds.data = []);
            statusTimelineChart.update('none');
        }
        return;
    }

    const labels = LogLynxCharts.formatTimelineLabels(data, currentTimeRange);

    if (statusTimelineChart) {
        statusTimelineChart.data.labels = labels;
        statusTimelineChart.data.datasets[0].data = data.map(d => d.status_2xx || 0);
        statusTimelineChart.data.datasets[1].data = data.map(d => d.status_3xx || 0);
        statusTimelineChart.data.datasets[2].data = data.map(d => d.status_4xx || 0);
        statusTimelineChart.data.datasets[3].data = data.map(d => d.status_5xx || 0);
        statusTimelineChart.update('none');
    }
}

// Update top countries table
function updateTopCountriesTable(data) {
    let html = '';

    if (!data || data.length === 0) {
        html = '<tr><td colspan="3" class="text-center text-muted">No data available</td></tr>';
    } else {
        data.forEach(item => {
            html += `
                <tr>
                    <td><i class="fas fa-flag"></i> ${item.country || 'Unknown'}</td>
                    <td>${LogLynxUtils.formatNumber(item.hits)}</td>
                    <td>${item.unique_visitors || 0}</td>
                </tr>
            `;
        });
    }

    $('#topCountriesTable').html(html);
}

// Update top paths table
function updateTopPathsTable(data) {
    let html = '';

    if (!data || data.length === 0) {
        html = '<tr><td colspan="3" class="text-center text-muted">No data available</td></tr>';
    } else {
        data.forEach(item => {
            html += `
                <tr>
                    <td><code>${LogLynxUtils.truncate(item.path, 40)}</code></td>
                    <td>${LogLynxUtils.formatNumber(item.hits)}</td>
                    <td>${LogLynxUtils.formatMs(item.avg_response_time || 0)}</td>
                </tr>
            `;
        });
    }

    $('#topPathsTable').html(html);
}

// Initialize DataTable for recent requests
function initDataTable() {
    $('#recentRequestsTable').DataTable({
        ajax: {
            url: LogLynxAPI.buildURL('/requests/recent', { limit: 100 }),
            dataSrc: ''
        },
        columns: [
            {
                data: 'Timestamp',
                render: (d) => LogLynxUtils.formatDateTime(d)
            },
            {
                data: 'Method',
                render: (d) => LogLynxUtils.getMethodBadge(d)
            },
            { data: 'Host' },
            {
                data: 'Path',
                render: (d) => `<code>${LogLynxUtils.truncate(d, 50)}</code>`
            },
            {
                data: 'StatusCode',
                render: (d) => LogLynxUtils.getStatusBadge(d)
            },
            {
                data: 'ResponseTimeMs',
                render: (d) => LogLynxUtils.formatMs(d || 0)
            },
            {
                data: 'GeoCountry',
                render: (d) => d || '-'
            },
            { data: 'ClientIP' }
        ],
        order: [[0, 'desc']],
        pageLength: 10,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No requests data available'
        }
    });
}

// Initialize time range selector
function initTimeRangeSelector() {
    document.querySelectorAll('.time-range-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            document.querySelectorAll('.time-range-btn').forEach(b => b.classList.remove('active'));
            this.classList.add('active');

            const range = this.getAttribute('data-range');
            currentTimeRange = range === 'all' ? 'all' : parseInt(range);

            // Reload timeline data
            loadTimelineData();
        });
    });
}

// Load only timeline-related data
async function loadTimelineData() {
    const hours = currentTimeRange === 'all' ? 8760 : currentTimeRange;

    const [timelineResult, statusTimelineResult] = await Promise.all([
        LogLynxAPI.getTimeline(hours),
        LogLynxAPI.getStatusCodeTimeline(hours)
    ]);

    if (timelineResult.success) {
        updateTimelineChart(timelineResult.data);
    }

    if (statusTimelineResult.success) {
        updateStatusTimelineChart(statusTimelineResult.data);
    }
}

// Initialize service filter with reload callback
function initServiceFilterWithReload() {
    LogLynxUtils.initServiceFilter(() => {
        loadDashboardData();
    });
}

// Initialize page
document.addEventListener('DOMContentLoaded', () => {
    // Initialize charts
    initTimelineChart();
    initStatusChart();
    initStatusTimelineChart();

    // Initialize DataTable
    initDataTable();

    // Initialize controls
    initTimeRangeSelector();
    initServiceFilterWithReload();

    // Initial data load
    loadDashboardData();

    // Initialize refresh controls
    LogLynxUtils.initRefreshControls(loadDashboardData, 30);
});
