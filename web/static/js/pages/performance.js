/**
 * Performance Monitoring Dashboard Page
 */

let performanceTimelineChart, responseTimeDistributionChart, percentileChart, volumeVsPerformanceChart;
let currentTimeRange = 168; // Default 7 days
let allPerformanceData = {};

// Load all performance data
async function loadPerformanceData() {
    console.log('Loading performance monitoring data...');

    try {
        // Load response time statistics
        const responseTimeResult = await LogLynxAPI.getResponseTimeStats();
        if (responseTimeResult.success) {
            allPerformanceData.responseTime = responseTimeResult.data;
            updatePerformanceKPIs(responseTimeResult.data);
            updatePercentileChart(responseTimeResult.data);
        }

        // Load timeline data
        const timelineResult = await LogLynxAPI.getTimeline(currentTimeRange);
        if (timelineResult.success) {
            allPerformanceData.timeline = timelineResult.data;
            updatePerformanceTimelineChart(timelineResult.data);
            updateVolumeVsPerformanceChart(timelineResult.data);
            calculatePerformanceDistribution(timelineResult.data);
        }

        // Load top paths
        const pathsResult = await LogLynxAPI.getTopPaths(100);
        if (pathsResult.success) {
            allPerformanceData.paths = pathsResult.data;
            initSlowPathsTable(pathsResult.data);
            updateFastPathsTable(pathsResult.data);
        }

        // Load backends
        const backendsResult = await LogLynxAPI.getTopBackends(30);
        if (backendsResult.success) {
            allPerformanceData.backends = backendsResult.data;
            initBackendPerfTable(backendsResult.data);
        }

        // Load log processing stats
        const logProcResult = await LogLynxAPI.getLogProcessingStats();
        if (logProcResult.success) {
            updateLogProcessingStats(logProcResult.data);
        }

    } catch (error) {
        console.error('Error loading performance data:', error);
        LogLynxUtils.showNotification('Failed to load performance data', 'error');
    }
}

// Update performance KPIs
function updatePerformanceKPIs(data) {
    if (!data) return;

    $('#avgResponseTime').text(LogLynxUtils.formatMs(data.avg || 0));
    $('#p50ResponseTime').text(LogLynxUtils.formatMs(data.p50 || 0));
    $('#p95ResponseTime').text(LogLynxUtils.formatMs(data.p95 || 0));
    $('#p99ResponseTime').text(LogLynxUtils.formatMs(data.p99 || 0));
    $('#minResponseTime').text(LogLynxUtils.formatMs(data.min || 0));
    $('#maxResponseTime').text(LogLynxUtils.formatMs(data.max || 0));
}

// Calculate performance distribution
function calculatePerformanceDistribution(timelineData) {
    if (!timelineData || timelineData.length === 0) return;

    let fast = 0, slow = 0, moderate = 0, verySlow = 0;
    let total = 0;

    let under100 = 0, under500 = 0, under1000 = 0;

    timelineData.forEach(point => {
        const avgTime = point.avg_response_time || 0;
        const requests = point.requests || 0;
        total += requests;

        if (avgTime < 100) {
            fast += requests;
            under100 += requests;
            under500 += requests;
            under1000 += requests;
        } else if (avgTime < 500) {
            moderate += requests;
            under500 += requests;
            under1000 += requests;
        } else if (avgTime < 1000) {
            moderate += requests;
            under1000 += requests;
        } else if (avgTime < 2000) {
            slow += requests;
        } else {
            verySlow += requests;
        }
    });

    if (total > 0) {
        const fastPct = ((fast / total) * 100).toFixed(1);
        const slowPct = ((slow / total) * 100).toFixed(1);

        $('#fastRequests').text(fastPct + '%');
        $('#slowRequests').text(slowPct + '%');

        // Performance goals
        const goal100 = ((under100 / total) * 100).toFixed(1);
        const goal500 = ((under500 / total) * 100).toFixed(1);
        const goal1000 = ((under1000 / total) * 100).toFixed(1);

        $('#goal100pct').text(goal100 + '%');
        $('#goal100bar').css('width', goal100 + '%');

        $('#goal500pct').text(goal500 + '%');
        $('#goal500bar').css('width', goal500 + '%');

        $('#goal1000pct').text(goal1000 + '%');
        $('#goal1000bar').css('width', goal1000 + '%');

        // Performance issues
        $('#verySlow').text(LogLynxUtils.formatNumber(verySlow));
        $('#slow').text(LogLynxUtils.formatNumber(slow));
        $('#moderate').text(LogLynxUtils.formatNumber(moderate));
    }

    // Update response time distribution chart
    updateResponseTimeDistributionChart({
        fast, moderate, slow, verySlow
    });
}

// Initialize performance timeline chart
function initPerformanceTimelineChart() {
    performanceTimelineChart = LogLynxCharts.createLineChart('performanceTimelineChart', {
        labels: [],
        datasets: [{
            label: 'Avg Response Time (ms)',
            data: [],
            borderColor: LogLynxCharts.colors.info,
            backgroundColor: LogLynxCharts.colors.info + '40',
            tension: 0.4,
            fill: true
        }]
    }, {
        plugins: {
            legend: { display: false }
        }
    });
}

// Update performance timeline chart
function updatePerformanceTimelineChart(data) {
    if (!data || data.length === 0) {
        if (performanceTimelineChart) {
            performanceTimelineChart.data.labels = [];
            performanceTimelineChart.data.datasets[0].data = [];
            performanceTimelineChart.update('none');
        }
        return;
    }

    const labels = LogLynxCharts.formatTimelineLabels(data, currentTimeRange);
    const avgResponseTimes = data.map(d => d.avg_response_time || 0);

    if (performanceTimelineChart) {
        performanceTimelineChart.data.labels = labels;
        performanceTimelineChart.data.datasets[0].data = avgResponseTimes;
        performanceTimelineChart.update('none');
    }
}

// Initialize response time distribution chart
function initResponseTimeDistributionChart() {
    responseTimeDistributionChart = LogLynxCharts.createBarChart('responseTimeDistributionChart', {
        labels: ['Fast (<100ms)', 'Moderate (100ms-1s)', 'Slow (1-2s)', 'Very Slow (>2s)'],
        datasets: [{
            label: 'Requests',
            data: [0, 0, 0, 0],
            backgroundColor: [
                LogLynxCharts.colors.success + '80',
                LogLynxCharts.colors.info + '80',
                LogLynxCharts.colors.warning + '80',
                LogLynxCharts.colors.danger + '80'
            ],
            borderColor: [
                LogLynxCharts.colors.success,
                LogLynxCharts.colors.info,
                LogLynxCharts.colors.warning,
                LogLynxCharts.colors.danger
            ],
            borderWidth: 1
        }]
    }, {
        plugins: {
            legend: { display: false }
        }
    });
}

// Update response time distribution chart
function updateResponseTimeDistributionChart(data) {
    if (!responseTimeDistributionChart) return;

    responseTimeDistributionChart.data.datasets[0].data = [
        data.fast || 0,
        data.moderate || 0,
        data.slow || 0,
        data.verySlow || 0
    ];
    responseTimeDistributionChart.update();
}

// Initialize percentile chart
function initPercentileChart() {
    percentileChart = LogLynxCharts.createBarChart('percentileChart', {
        labels: ['Min', 'P50', 'Avg', 'P95', 'P99', 'Max'],
        datasets: [{
            label: 'Response Time (ms)',
            data: [0, 0, 0, 0, 0, 0],
            backgroundColor: [
                LogLynxCharts.colors.success + '80',
                LogLynxCharts.colors.success + '80',
                LogLynxCharts.colors.info + '80',
                LogLynxCharts.colors.warning + '80',
                LogLynxCharts.colors.danger + '80',
                LogLynxCharts.colors.danger + '80'
            ],
            borderColor: [
                LogLynxCharts.colors.success,
                LogLynxCharts.colors.success,
                LogLynxCharts.colors.info,
                LogLynxCharts.colors.warning,
                LogLynxCharts.colors.danger,
                LogLynxCharts.colors.danger
            ],
            borderWidth: 1
        }]
    }, {
        plugins: {
            legend: { display: false }
        }
    });
}

// Update percentile chart
function updatePercentileChart(data) {
    if (!percentileChart || !data) return;

    percentileChart.data.datasets[0].data = [
        data.min || 0,
        data.p50 || 0,
        data.avg || 0,
        data.p95 || 0,
        data.p99 || 0,
        data.max || 0
    ];
    percentileChart.update();
}

// Initialize volume vs performance chart
function initVolumeVsPerformanceChart() {
    volumeVsPerformanceChart = LogLynxCharts.createDualAxisChart('volumeVsPerformanceChart', {
        labels: [],
        datasets: [
            {
                label: 'Requests',
                data: [],
                borderColor: LogLynxCharts.colors.primary,
                backgroundColor: LogLynxCharts.colors.primaryLight + '40',
                tension: 0.4,
                fill: true,
                yAxisID: 'y'
            },
            {
                label: 'Avg Response Time (ms)',
                data: [],
                borderColor: LogLynxCharts.colors.info,
                backgroundColor: LogLynxCharts.colors.info + '40',
                tension: 0.4,
                fill: false,
                yAxisID: 'y1'
            }
        ]
    }, {
        scales: {
            y: {
                title: {
                    display: true,
                    text: 'Requests',
                    color: LogLynxCharts.colors.primary
                }
            },
            y1: {
                title: {
                    display: true,
                    text: 'Response Time (ms)',
                    color: LogLynxCharts.colors.info
                }
            }
        }
    });
}

// Update volume vs performance chart
function updateVolumeVsPerformanceChart(data) {
    if (!volumeVsPerformanceChart || !data || data.length === 0) return;

    const labels = LogLynxCharts.formatTimelineLabels(data, currentTimeRange);
    const requests = data.map(d => d.requests);
    const avgResponseTimes = data.map(d => d.avg_response_time || 0);

    volumeVsPerformanceChart.data.labels = labels;
    volumeVsPerformanceChart.data.datasets[0].data = requests;
    volumeVsPerformanceChart.data.datasets[1].data = avgResponseTimes;
    volumeVsPerformanceChart.update('none');
}

// Initialize slow paths DataTable
function initSlowPathsTable(pathsData) {
    // Sort by avg response time descending
    const slowPaths = pathsData.sort((a, b) =>
        (b.avg_response_time || 0) - (a.avg_response_time || 0)
    );

    if ($.fn.DataTable.isDataTable('#slowPathsTable')) {
        $('#slowPathsTable').DataTable().destroy();
    }

    $('#slowPathsTable').DataTable({
        data: slowPaths,
        columns: [
            {
                data: null,
                render: (data, type, row, meta) => meta.row + 1
            },
            {
                data: 'path',
                render: (d) => `<code>${LogLynxUtils.truncate(d, 60)}</code>`
            },
            {
                data: 'hits',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: 'avg_response_time',
                render: (d) => `<strong>${LogLynxUtils.formatMs(d || 0)}</strong>`
            },
            {
                data: null,
                render: (data) => {
                    // Estimate min/max (we don't have this data, so approximate)
                    const avg = data.avg_response_time || 0;
                    return LogLynxUtils.formatMs(avg * 0.5);
                }
            },
            {
                data: null,
                render: (data) => {
                    const avg = data.avg_response_time || 0;
                    return LogLynxUtils.formatMs(avg * 2);
                }
            },
            {
                data: 'total_bandwidth',
                render: (d) => LogLynxCharts.formatBytes(d || 0)
            },
            {
                data: 'avg_response_time',
                render: (d) => {
                    let badgeClass = 'badge-success';
                    let label = 'Excellent';
                    if (d > 2000) {
                        badgeClass = 'badge-danger';
                        label = 'Critical';
                    } else if (d > 1000) {
                        badgeClass = 'badge-warning';
                        label = 'Poor';
                    } else if (d > 500) {
                        badgeClass = 'badge-info';
                        label = 'Fair';
                    }
                    return `<span class="badge ${badgeClass}">${label}</span>`;
                }
            }
        ],
        order: [[3, 'desc']],
        pageLength: 20,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No path data available'
        }
    });
}

// Update fastest paths table
function updateFastPathsTable(pathsData) {
    // Sort by avg response time ascending
    const fastPaths = pathsData.sort((a, b) =>
        (a.avg_response_time || 0) - (b.avg_response_time || 0)
    ).slice(0, 10);

    let html = '';

    if (fastPaths.length === 0) {
        html = '<tr><td colspan="5" class="text-center text-muted">No data available</td></tr>';
    } else {
        fastPaths.forEach((path, index) => {
            let badgeClass = 'badge-success';
            let label = 'Excellent';
            const avgTime = path.avg_response_time || 0;

            if (avgTime > 100) {
                badgeClass = 'badge-info';
                label = 'Good';
            }

            html += `
                <tr>
                    <td>${index + 1}</td>
                    <td><code>${LogLynxUtils.truncate(path.path, 60)}</code></td>
                    <td>${LogLynxUtils.formatNumber(path.hits)}</td>
                    <td><strong>${LogLynxUtils.formatMs(avgTime)}</strong></td>
                    <td><span class="badge ${badgeClass}">${label}</span></td>
                </tr>
            `;
        });
    }

    $('#fastPathsTable').html(html);
}

// Initialize backend performance DataTable
function initBackendPerfTable(backendsData) {
    if ($.fn.DataTable.isDataTable('#backendPerfTable')) {
        $('#backendPerfTable').DataTable().destroy();
    }

    $('#backendPerfTable').DataTable({
        data: backendsData,
        columns: [
            {
                data: 'backend_name',
                render: (d) => `<strong>${d || 'Unknown'}</strong>`
            },
            {
                data: 'backend_url',
                render: (d) => `<code>${LogLynxUtils.truncate(d || '-', 40)}</code>`
            },
            {
                data: 'hits',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: 'avg_response_time',
                render: (d) => LogLynxUtils.formatMs(d || 0)
            },
            {
                data: 'error_count',
                render: (d) => `<span class="text-danger">${LogLynxUtils.formatNumber(d || 0)}</span>`
            },
            {
                data: null,
                render: (data) => {
                    const errorRate = data.hits > 0 ? (data.error_count / data.hits * 100) : 0;
                    let badgeClass = 'badge-success';
                    if (errorRate > 10) badgeClass = 'badge-danger';
                    else if (errorRate > 5) badgeClass = 'badge-warning';
                    else if (errorRate > 1) badgeClass = 'badge-info';
                    return `<span class="badge ${badgeClass}">${errorRate.toFixed(2)}%</span>`;
                }
            },
            {
                data: 'bandwidth',
                render: (d) => LogLynxCharts.formatBytes(d || 0)
            },
            {
                data: null,
                render: (data) => {
                    // Calculate health score (0-100)
                    const errorRate = data.hits > 0 ? (data.error_count / data.hits) : 0;
                    const responseTime = data.avg_response_time || 0;

                    let score = 100;
                    score -= errorRate * 50; // Errors heavily impact score
                    if (responseTime > 1000) score -= 30;
                    else if (responseTime > 500) score -= 15;
                    else if (responseTime > 200) score -= 5;

                    score = Math.max(0, Math.min(100, score));

                    let color = '#28a745';
                    if (score < 50) color = '#dc3545';
                    else if (score < 75) color = '#ffc107';
                    else if (score < 90) color = '#17a2b8';

                    return `
                        <div class="d-flex align-items-center gap-2">
                            <div style="width: 50px; height: 8px; background: #1f1f21; border-radius: 4px; overflow: hidden;">
                                <div style="width: ${score}%; height: 100%; background: ${color};"></div>
                            </div>
                            <strong style="color: ${color};">${score.toFixed(0)}</strong>
                        </div>
                    `;
                }
            }
        ],
        order: [[2, 'desc']],
        pageLength: 15,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No backend data available'
        }
    });
}

// Update log processing stats
function updateLogProcessingStats(data) {
    if (!data || data.length === 0) {
        $('#logProcessingList').html('<p class="text-muted">No log sources found</p>');
        return;
    }

    let html = '';
    data.forEach(source => {
        const percentage = source.percentage || 0;
        html += `
            <div class="mb-3">
                <div class="d-flex justify-content-between mb-1" >                
                    <small style="color: var(--loglynx-text);">${source.log_source_name || 'Unknown'}</small>
                    <small style="color: var(--loglynx-text);">${percentage.toFixed(1)}%</small>
                </div>
                <div style="width: 100%; height: 6px; background: #1f1f21; border-radius: 3px; overflow: hidden;">
                    <div style="width: ${percentage}%; height: 100%; background: ${LogLynxCharts.colors.primary}; transition: width 0.5s;"></div>
                </div>
            </div>
        `;
    });

    $('#logProcessingList').html(html);
}

// Initialize time range selector
function initTimeRangeSelector() {
    document.querySelectorAll('.time-range-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            document.querySelectorAll('.time-range-btn').forEach(b => b.classList.remove('active'));
            this.classList.add('active');

            const range = this.getAttribute('data-range');
            currentTimeRange = parseInt(range);

            // Reload timeline data
            loadTimelineData();
        });
    });
}

// Load only timeline data
async function loadTimelineData() {
    const result = await LogLynxAPI.getTimeline(currentTimeRange);

    if (result.success) {
        allPerformanceData.timeline = result.data;
        updatePerformanceTimelineChart(result.data);
        updateVolumeVsPerformanceChart(result.data);
        calculatePerformanceDistribution(result.data);
    }
}

// Export functions
function exportPerformanceReport() {
    const report = {
        response_time_stats: allPerformanceData.responseTime,
        slow_paths: allPerformanceData.paths ?
            allPerformanceData.paths.sort((a, b) => (b.avg_response_time || 0) - (a.avg_response_time || 0)).slice(0, 20) : [],
        backends: allPerformanceData.backends
    };

    const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `performance-report-${new Date().toISOString().split('T')[0]}.json`;
    link.click();

    LogLynxUtils.showNotification('Performance report exported', 'success', 3000);
}

function exportSlowPathsData() {
    const table = $('#slowPathsTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'slow-paths.csv');
}

function exportFastPathsData() {
    if (allPerformanceData.paths) {
        const fastPaths = allPerformanceData.paths.sort((a, b) =>
            (a.avg_response_time || 0) - (b.avg_response_time || 0)
        ).slice(0, 10);
        LogLynxUtils.exportAsCSV(fastPaths, 'fast-paths.csv');
    }
}

function exportBackendData() {
    const table = $('#backendPerfTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'backend-performance.csv');
}

// Initialize service filter with reload callback
function initServiceFilterWithReload() {
    LogLynxUtils.initServiceFilter(() => {
        loadPerformanceData();
    });
}

// Initialize page
document.addEventListener('DOMContentLoaded', () => {
    // Initialize all charts
    initPerformanceTimelineChart();
    initResponseTimeDistributionChart();
    initPercentileChart();
    initVolumeVsPerformanceChart();

    // Initialize controls
    initTimeRangeSelector();
    initServiceFilterWithReload();

    // Initial data load
    loadPerformanceData();

    // Initialize refresh controls
    LogLynxUtils.initRefreshControls(loadPerformanceData, 30);
});
