/**
 * Traffic Analysis Dashboard Page
 */

let trafficTimelineChart, trafficHeatmapChart, deviceTypeChart, topCountriesPieChart;
let trafficByHourChart, trafficByDayChart;
let currentTimeRange = 168; // Default 7 days
let currentHeatmapDays = 7;
let allTrafficData = {};

// Load all traffic data
async function loadTrafficData() {
    console.log('Loading traffic analysis data...');

    try {
        // Load summary for KPIs
        const summaryResult = await LogLynxAPI.getSummary();
        if (summaryResult.success) {
            updateTrafficKPIs(summaryResult.data);
        }

        // Load timeline data
        const timelineResult = await LogLynxAPI.getTimeline(currentTimeRange);
        if (timelineResult.success) {
            allTrafficData.timeline = timelineResult.data;
            updateTrafficTimelineChart(timelineResult.data);
            calculatePeakTraffic(timelineResult.data);
        }

        // Load heatmap data
        const heatmapResult = await LogLynxAPI.getTrafficHeatmap(currentHeatmapDays);
        if (heatmapResult.success) {
            allTrafficData.heatmap = heatmapResult.data;
            updateTrafficHeatmapChart(heatmapResult.data);
            updateHourlyAndDailyCharts(heatmapResult.data);
        }

        // Load device type distribution
        const deviceResult = await LogLynxAPI.getDeviceTypeDistribution();
        if (deviceResult.success) {
            allTrafficData.devices = deviceResult.data;
            updateDeviceTypeChart(deviceResult.data);
            updateDeviceMetrics(deviceResult.data);
        }

        // Load top countries
        const countriesResult = await LogLynxAPI.getTopCountries(20);
        if (countriesResult.success) {
            allTrafficData.countries = countriesResult.data;
            updateTopCountriesTable(countriesResult.data);
            updateTopCountriesPieChart(countriesResult.data.slice(0, 10));
        }

        // Load top IPs
        const ipsResult = await LogLynxAPI.getTopIPs(20);
        if (ipsResult.success) {
            allTrafficData.ips = ipsResult.data;
            updateTopIPsTable(ipsResult.data);
        }

        // Initialize ASN DataTable
        initASNDataTable();

    } catch (error) {
        console.error('Error loading traffic data:', error);
        LogLynxUtils.showNotification('Failed to load traffic data', 'error');
    }
}

// Update traffic KPIs
function updateTrafficKPIs(data) {
    $('#totalTraffic').text(LogLynxUtils.formatNumber(data.total_requests || 0));
    $('#uniqueVisitors').text(LogLynxUtils.formatNumber(data.unique_visitors || 0));
}

// Calculate peak traffic
function calculatePeakTraffic(timelineData) {
    if (!timelineData || timelineData.length === 0) return;

    let maxRequests = 0;
    let peakTime = '';

    timelineData.forEach(point => {
        if (point.requests > maxRequests) {
            maxRequests = point.requests;
            peakTime = point.hour;
        }
    });

    $('#peakTraffic').text(LogLynxUtils.formatNumber(maxRequests));
    if (peakTime) {
        const date = new Date(peakTime);
        $('#peakTrafficTime').text(date.toLocaleDateString('en-US', {
            month: 'short',
            day: 'numeric',
            hour: '2-digit'
        }));
    }
}

// Initialize traffic timeline chart
function initTrafficTimelineChart() {
    trafficTimelineChart = LogLynxCharts.createDualAxisChart('trafficTimelineChart', {
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
                label: 'Unique Visitors',
                data: [],
                borderColor: LogLynxCharts.colors.info,
                backgroundColor: LogLynxCharts.colors.info + '40',
                tension: 0.4,
                fill: true,
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
                },
                ticks: { color: LogLynxCharts.colors.primary }
            },
            y1: {
                title: {
                    display: true,
                    text: 'Unique Visitors',
                    color: LogLynxCharts.colors.info
                },
                ticks: { color: LogLynxCharts.colors.info }
            }
        }
    });
}

// Update traffic timeline chart
function updateTrafficTimelineChart(data) {
    if (!data || data.length === 0) {
        if (trafficTimelineChart) {
            trafficTimelineChart.data.labels = [];
            trafficTimelineChart.data.datasets[0].data = [];
            trafficTimelineChart.data.datasets[1].data = [];
            trafficTimelineChart.update('none');
        }
        return;
    }

    const labels = LogLynxCharts.formatTimelineLabels(data, currentTimeRange);
    const requests = data.map(d => d.requests);
    const visitors = data.map(d => d.unique_visitors || 0);

    if (trafficTimelineChart) {
        trafficTimelineChart.data.labels = labels;
        trafficTimelineChart.data.datasets[0].data = requests;
        trafficTimelineChart.data.datasets[1].data = visitors;
        trafficTimelineChart.update('none');
    }
}

// Initialize traffic heatmap chart
function initTrafficHeatmapChart() {
    const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
    const hourLabels = Array.from({length: 24}, (_, i) => i.toString().padStart(2, '0') + ':00');

    trafficHeatmapChart = LogLynxCharts.createHeatmapChart('trafficHeatmapChart', {
        datasets: [{
            label: 'Requests',
            data: [],
            backgroundColor: (ctx) => {
                return LogLynxCharts.heatmapColorFunction(1)(ctx);
            },
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
                            `Avg Response: ${LogLynxUtils.formatMs(data.avg || 0)}`
                        ];
                    }
                }
            }
        }
    });
}

// Update traffic heatmap chart
function updateTrafficHeatmapChart(data) {
    if (!trafficHeatmapChart) return;

    const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
    const hourLabels = Array.from({length: 24}, (_, i) => i.toString().padStart(2, '0') + ':00');

    const heatmapData = LogLynxCharts.generateHeatmapData(data, dayNames, hourLabels);

    trafficHeatmapChart.data.datasets[0].data = heatmapData.data;
    trafficHeatmapChart.data.datasets[0].backgroundColor = LogLynxCharts.heatmapColorFunction(heatmapData.maxValue);
    trafficHeatmapChart.update('none');

    // Update total countries count
    $('#totalCountries').text(allTrafficData.countries ? allTrafficData.countries.length : 0);
}

// Initialize device type chart
function initDeviceTypeChart() {
    deviceTypeChart = LogLynxCharts.createDoughnutChart('deviceTypeChart', {
        labels: ['Desktop', 'Mobile', 'Tablet', 'Bot'],
        datasets: [{
            data: [0, 0, 0, 0],
            backgroundColor: [
                LogLynxCharts.colors.primary,
                LogLynxCharts.colors.info,
                LogLynxCharts.colors.success,
                LogLynxCharts.colors.warning
            ]
        }]
    });
}

// Update device type chart
function updateDeviceTypeChart(data) {
    if (!data || data.length === 0 || !deviceTypeChart) return;

    const deviceMap = { 'desktop': 0, 'mobile': 0, 'tablet': 0, 'bot': 0 };

    data.forEach(d => {
        const type = (d.device_type || 'unknown').toLowerCase();
        if (deviceMap.hasOwnProperty(type)) {
            deviceMap[type] = d.count;
        }
    });

    deviceTypeChart.data.datasets[0].data = [
        deviceMap.desktop,
        deviceMap.mobile,
        deviceMap.tablet,
        deviceMap.bot
    ];
    deviceTypeChart.update();
}

// Update device metrics
function updateDeviceMetrics(data) {
    if (!data || data.length === 0) return;

    let total = 0;
    const deviceMap = { 'desktop': 0, 'mobile': 0, 'tablet': 0, 'bot': 0 };

    data.forEach(d => {
        const type = (d.device_type || 'unknown').toLowerCase();
        const count = d.count || 0;
        total += count;
        if (deviceMap.hasOwnProperty(type)) {
            deviceMap[type] = count;
        }
    });

    if (total > 0) {
        $('#desktopTraffic').text(((deviceMap.desktop / total) * 100).toFixed(1) + '%');
        $('#desktopCount').text(LogLynxUtils.formatNumber(deviceMap.desktop) + ' requests');

        $('#mobileTraffic').text(((deviceMap.mobile / total) * 100).toFixed(1) + '%');
        $('#mobileCount').text(LogLynxUtils.formatNumber(deviceMap.mobile) + ' requests');

        $('#botTraffic').text(((deviceMap.bot / total) * 100).toFixed(1) + '%');
        $('#botCount').text(LogLynxUtils.formatNumber(deviceMap.bot) + ' requests');

        $('#tabletTraffic').text(((deviceMap.tablet / total) * 100).toFixed(1) + '%');
        $('#tabletCount').text(LogLynxUtils.formatNumber(deviceMap.tablet) + ' requests');
    }
}

// Initialize top countries pie chart
function initTopCountriesPieChart() {
    topCountriesPieChart = LogLynxCharts.createDoughnutChart('topCountriesPieChart', {
        labels: [],
        datasets: [{
            data: [],
            backgroundColor: LogLynxCharts.colors.chartPalette
        }]
    }, {
        cutout: '50%'
    });
}

// Update top countries pie chart
function updateTopCountriesPieChart(data) {
    if (!data || data.length === 0 || !topCountriesPieChart) return;

    const labels = data.map(d => d.country_name || d.country || 'Unknown');
    const values = data.map(d => d.hits);

    topCountriesPieChart.data.labels = labels;
    topCountriesPieChart.data.datasets[0].data = values;
    topCountriesPieChart.update();
}

// Update top countries table
function updateTopCountriesTable(data) {
    let html = '';

    if (!data || data.length === 0) {
        html = '<tr><td colspan="6" class="text-center text-muted">No data available</td></tr>';
    } else {
        const total = data.reduce((sum, item) => sum + (item.hits || 0), 0);

        data.forEach((item, index) => {
            const percentage = total > 0 ? ((item.hits / total) * 100).toFixed(1) : 0;
            html += `
                <tr>
                    <td>${index + 1}</td>
                    <td>
                        <i class="fas fa-flag"></i>
                        <strong>${item.country || 'Unknown'}</strong>
                        ${item.country_name ? `<br><small class="text-muted">${item.country_name}</small>` : ''}
                    </td>
                    <td>${LogLynxUtils.formatNumber(item.hits)}</td>
                    <td>${LogLynxUtils.formatNumber(item.unique_visitors || 0)}</td>
                    <td>${LogLynxCharts.formatBytes(item.bandwidth || 0)}</td>
                    <td>
                        <div class="d-flex align-items-center gap-2">
                            <div style="width: 50px; height: 6px; background: #1f1f21; border-radius: 3px; overflow: hidden;">
                                <div style="width: ${percentage}%; height: 100%; background: ${LogLynxCharts.colors.primary};"></div>
                            </div>
                            <span>${percentage}%</span>
                        </div>
                    </td>
                </tr>
            `;
        });
    }

    $('#topCountriesTable').html(html);
}

// Update top IPs table
function updateTopIPsTable(data) {
    let html = '';

    if (!data || data.length === 0) {
        html = '<tr><td colspan="6" class="text-center text-muted">No data available</td></tr>';
    } else {
        data.forEach((item, index) => {
            html += `
                <tr>
                    <td>${index + 1}</td>
                    <td><a href="/ip/${item.ip_address}" class="ip-link"><code>${item.ip_address}</code></a></td>
                    <td>${item.country || '-'}</td>
                    <td>${item.city || '-'}</td>
                    <td>${LogLynxUtils.formatNumber(item.hits)}</td>
                    <td>${LogLynxCharts.formatBytes(item.bandwidth || 0)}</td>
                </tr>
            `;
        });
    }

    $('#topIPsTable').html(html);
}

// Initialize ASN DataTable
function initASNDataTable() {
    // Destroy existing DataTable if it exists
    if ($.fn.DataTable.isDataTable('#asnTable')) {
        $('#asnTable').DataTable().destroy();
    }

    $('#asnTable').DataTable({
        ajax: function(data, callback, settings) {
            // Custom ajax function that rebuilds URL with current filters
            const url = LogLynxAPI.buildURL('/stats/top/asns', { limit: 50 });

            fetch(url)
                .then(response => response.json())
                .then(json => {
                    callback({ data: json });
                })
                .catch(error => {
                    console.error('[ASN Table] AJAX error:', error);
                    callback({ data: [] });
                });
        },
        columns: [
            {
                data: null,
                render: (data, type, row, meta) => meta.row + 1
            },
            {
                data: 'asn',
                render: (d) => `<strong>AS${d}</strong>`
            },
            {
                data: 'asn_org',
                render: (d) => LogLynxUtils.truncate(d || 'Unknown', 50)
            },
            { data: 'country' },
            {
                data: 'hits',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: 'bandwidth',
                render: (d) => LogLynxCharts.formatBytes(d || 0)
            },
            {
                data: null,
                render: (data, type, row) => {
                    // Calculate percentage based on total (approximate)
                    const total = allTrafficData.timeline ?
                        allTrafficData.timeline.reduce((sum, t) => sum + t.requests, 0) : 1;
                    const pct = ((row.hits / total) * 100).toFixed(2);
                    return `${pct}%`;
                }
            },
            {
                data: null,
                render: (data, type, row) => {
                    const avgSize = row.hits > 0 ? row.bandwidth / row.hits : 0;
                    return LogLynxCharts.formatBytes(avgSize);
                }
            }
        ],
        order: [[4, 'desc']],
        pageLength: 15,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No ASN data available'
        }
    });
}

// Initialize traffic by hour chart
function initTrafficByHourChart() {
    trafficByHourChart = LogLynxCharts.createBarChart('trafficByHourChart', {
        labels: Array.from({length: 24}, (_, i) => i.toString().padStart(2, '0') + ':00'),
        datasets: [{
            label: 'Requests',
            data: Array(24).fill(0),
            backgroundColor: LogLynxCharts.colors.primaryLight + '80',
            borderColor: LogLynxCharts.colors.primary,
            borderWidth: 1
        }]
    }, {
        plugins: {
            legend: { display: false }
        }
    });
}

// Initialize traffic by day chart
function initTrafficByDayChart() {
    trafficByDayChart = LogLynxCharts.createBarChart('trafficByDayChart', {
        labels: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'],
        datasets: [{
            label: 'Requests',
            data: Array(7).fill(0),
            backgroundColor: LogLynxCharts.colors.info + '80',
            borderColor: LogLynxCharts.colors.info,
            borderWidth: 1
        }]
    }, {
        plugins: {
            legend: { display: false }
        }
    });
}

// Update hourly and daily charts from heatmap data
function updateHourlyAndDailyCharts(heatmapData) {
    if (!heatmapData || heatmapData.length === 0) return;

    // Aggregate by hour
    const hourlyData = Array(24).fill(0);
    const dailyData = Array(7).fill(0);

    heatmapData.forEach(entry => {
        const hour = parseInt(entry.hour, 10);
        const day = parseInt(entry.day_of_week, 10);

        if (!isNaN(hour)) {
            hourlyData[hour] += entry.requests || 0;
        }
        if (!isNaN(day)) {
            dailyData[day] += entry.requests || 0;
        }
    });

    // Update charts
    if (trafficByHourChart) {
        trafficByHourChart.data.datasets[0].data = hourlyData;
        trafficByHourChart.update('none');
    }

    if (trafficByDayChart) {
        trafficByDayChart.data.datasets[0].data = dailyData;
        trafficByDayChart.update('none');
    }
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
        allTrafficData.timeline = result.data;
        updateTrafficTimelineChart(result.data);
        calculatePeakTraffic(result.data);
    }
}

// Initialize heatmap days selector
function initHeatmapDaysSelector() {
    $('#heatmapDays').on('change', async function() {
        currentHeatmapDays = parseInt($(this).val());

        const result = await LogLynxAPI.getTrafficHeatmap(currentHeatmapDays);
        if (result.success) {
            allTrafficData.heatmap = result.data;
            updateTrafficHeatmapChart(result.data);
            updateHourlyAndDailyCharts(result.data);
        }
    });
}

// Export functions
function exportTrafficReport() {
    // Combine all data for export
    const report = {
        summary: {
            total_traffic: $('#totalTraffic').text(),
            unique_visitors: $('#uniqueVisitors').text(),
            countries: $('#totalCountries').text(),
            peak_traffic: $('#peakTraffic').text()
        },
        countries: allTrafficData.countries,
        ips: allTrafficData.ips,
        devices: allTrafficData.devices
    };

    const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `traffic-report-${new Date().toISOString().split('T')[0]}.json`;
    link.click();

    LogLynxUtils.showNotification('Traffic report exported', 'success', 3000);
}

function exportCountriesData() {
    if (allTrafficData.countries) {
        LogLynxUtils.exportAsCSV(allTrafficData.countries, 'top-countries.csv');
    }
}

function exportIPsData() {
    if (allTrafficData.ips) {
        LogLynxUtils.exportAsCSV(allTrafficData.ips, 'top-ips.csv');
    }
}

function exportASNData() {
    const table = $('#asnTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'asn-analysis.csv');
}

// Initialize service filter with reload callback
function initServiceFilterWithReload() {
    LogLynxUtils.initServiceFilter(() => {
        loadTrafficData();
    });
}

// Initialize page
// Initialize hide my traffic filter with reload callback
function initHideTrafficFilterWithReload() {
    LogLynxUtils.initHideMyTrafficFilter(() => {
        loadTrafficData();
    });
}

document.addEventListener('DOMContentLoaded', () => {
    // Initialize all charts
    initTrafficTimelineChart();
    initTrafficHeatmapChart();
    initDeviceTypeChart();
    initTopCountriesPieChart();
    initTrafficByHourChart();
    initTrafficByDayChart();

    // Initialize controls
    initTimeRangeSelector();
    initHeatmapDaysSelector();
    initServiceFilterWithReload();
    initHideTrafficFilterWithReload();

    // Initial data load
    loadTrafficData();

    // Initialize refresh controls
    LogLynxUtils.initRefreshControls(loadTrafficData, 30);
});
