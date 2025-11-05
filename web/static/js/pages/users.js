/**
 * User Analytics Dashboard Page
 */

let deviceTypeChart, browserChart, osChart;
let allUserData = {};

// Load all user analytics data
async function loadUserAnalyticsData() {
    console.log('Loading user analytics data...');

    try {
        // Load summary for KPIs
        const summaryResult = await LogLynxAPI.getSummary();
        if (summaryResult.success) {
            updateUserKPIs(summaryResult.data);
        }

        // Load device type distribution
        const deviceResult = await LogLynxAPI.getDeviceTypeDistribution();
        if (deviceResult.success) {
            allUserData.devices = deviceResult.data;
            updateDeviceTypeChart(deviceResult.data);
            updateBotDetection(deviceResult.data);
            updatePlatformDistribution(deviceResult.data);
            updateTopInsights(deviceResult.data);
        }

        // Load browsers
        const browsersResult = await LogLynxAPI.getTopBrowsers(15);
        if (browsersResult.success) {
            allUserData.browsers = browsersResult.data;
            updateBrowserChart(browsersResult.data);
            initBrowserTable(browsersResult.data);
        }

        // Load operating systems
        const osResult = await LogLynxAPI.getTopOperatingSystems(15);
        if (osResult.success) {
            allUserData.os = osResult.data;
            updateOSChart(osResult.data);
            initOSTable(osResult.data);
        }

        // Load referrers
        const referrersResult = await LogLynxAPI.getTopReferrers(20);
        if (referrersResult.success) {
            allUserData.referrers = referrersResult.data;
            updateTopReferrersTable(referrersResult.data);
        }

        // Load referrer domains
        const domainsResult = await LogLynxAPI.getTopReferrerDomains(20);
        if (domainsResult.success) {
            allUserData.referrerDomains = domainsResult.data;
            updateTopReferrerDomainsTable(domainsResult.data);
            calculateReferralTraffic(domainsResult.data, summaryResult.data);
        }

        // Load user agents
        const userAgentsResult = await LogLynxAPI.getTopUserAgents(20);
        if (userAgentsResult.success) {
            allUserData.userAgents = userAgentsResult.data;
            initUserAgentTable(userAgentsResult.data);
        }

        // Load countries for geographic distribution
        const countriesResult = await LogLynxAPI.getTopCountries(15);
        if (countriesResult.success) {
            allUserData.countries = countriesResult.data;
            updateGeoVisitorsTable(countriesResult.data);
        }

    } catch (error) {
        console.error('Error loading user analytics data:', error);
        LogLynxUtils.showNotification('Failed to load user analytics data', 'error');
    }
}

// Update user KPIs
function updateUserKPIs(data) {
    $('#totalVisitors').text(LogLynxUtils.formatNumber(data.unique_visitors || 0));
}

// Calculate referral traffic percentage
function calculateReferralTraffic(referrerData, summaryData) {
    if (!referrerData || referrerData.length === 0 || !summaryData) {
        $('#referralTraffic').text('0%');
        return;
    }

    const totalReferrals = referrerData.reduce((sum, ref) => sum + (ref.hits || 0), 0);
    const totalRequests = summaryData.total_requests || 1;
    const percentage = ((totalReferrals / totalRequests) * 100).toFixed(1);

    $('#referralTraffic').text(percentage + '%');
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

// Initialize browser chart
function initBrowserChart() {
    browserChart = LogLynxCharts.createDoughnutChart('browserChart', {
        labels: [],
        datasets: [{
            data: [],
            backgroundColor: LogLynxCharts.colors.chartPalette
        }]
    });
}

// Update browser chart
function updateBrowserChart(data) {
    if (!data || data.length === 0 || !browserChart) return;

    const topBrowsers = data.slice(0, 8);
    const labels = topBrowsers.map(b => b.browser || 'Unknown');
    const values = topBrowsers.map(b => b.count);

    browserChart.data.labels = labels;
    browserChart.data.datasets[0].data = values;
    browserChart.update();

    $('#browserTypes').text(data.length);
}

// Initialize OS chart
function initOSChart() {
    osChart = LogLynxCharts.createDoughnutChart('osChart', {
        labels: [],
        datasets: [{
            data: [],
            backgroundColor: LogLynxCharts.colors.chartPalette
        }]
    });
}

// Update OS chart
function updateOSChart(data) {
    if (!data || data.length === 0 || !osChart) return;

    const topOS = data.slice(0, 8);
    const labels = topOS.map(os => os.os || 'Unknown');
    const values = topOS.map(os => os.count);

    osChart.data.labels = labels;
    osChart.data.datasets[0].data = values;
    osChart.update();

    $('#osTypes').text(data.length);
}

// Initialize browser DataTable
function initBrowserTable(browsersData) {
    // Destroy existing DataTable if it exists
    if ($.fn.DataTable.isDataTable('#browserTable')) {
        $('#browserTable').DataTable().destroy();
    }

    const total = browsersData.reduce((sum, b) => sum + b.count, 0);

    $('#browserTable').DataTable({
        data: browsersData,
        columns: [
            {
                data: null,
                render: (data, type, row, meta) => meta.row + 1
            },
            {
                data: 'browser',
                render: (d) => `<strong>${d || 'Unknown'}</strong>`
            },
            {
                data: 'count',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: null,
                render: (data) => {
                    const pct = total > 0 ? ((data.count / total) * 100).toFixed(2) : 0;
                    return `${pct}%`;
                }
            },
            {
                data: null,
                render: (data) => {
                    const pct = total > 0 ? ((data.count / total) * 100) : 0;
                    return `
                        <div style="width: 100%; height: 20px; background: #1f1f21; border-radius: 4px; overflow: hidden; position: relative;">
                            <div style="width: ${pct}%; height: 100%; background: ${LogLynxCharts.colors.primary}; transition: width 0.3s;"></div>
                            <span style="position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%); font-size: 0.7rem; color: #F3EFF3;">
                                ${pct.toFixed(1)}%
                            </span>
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
            emptyTable: 'No browser data available'
        }
    });
}

// Initialize OS DataTable
function initOSTable(osData) {
    // Destroy existing DataTable if it exists
    if ($.fn.DataTable.isDataTable('#osTable')) {
        $('#osTable').DataTable().destroy();
    }

    const total = osData.reduce((sum, os) => sum + os.count, 0);

    $('#osTable').DataTable({
        data: osData,
        columns: [
            {
                data: null,
                render: (data, type, row, meta) => meta.row + 1
            },
            {
                data: 'os',
                render: (d) => `<strong>${d || 'Unknown'}</strong>`
            },
            {
                data: 'count',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: null,
                render: (data) => {
                    const pct = total > 0 ? ((data.count / total) * 100).toFixed(2) : 0;
                    return `${pct}%`;
                }
            },
            {
                data: null,
                render: (data) => {
                    const pct = total > 0 ? ((data.count / total) * 100) : 0;
                    return `
                        <div style="width: 100%; height: 20px; background: #1f1f21; border-radius: 4px; overflow: hidden; position: relative;">
                            <div style="width: ${pct}%; height: 100%; background: ${LogLynxCharts.colors.info}; transition: width 0.3s;"></div>
                            <span style="position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%); font-size: 0.7rem; color: #F3EFF3;">
                                ${pct.toFixed(1)}%
                            </span>
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
            emptyTable: 'No operating system data available'
        }
    });
}

// Update top referrers table
function updateTopReferrersTable(data) {
    let html = '';

    if (!data || data.length === 0) {
        html = '<tr><td colspan="4" class="text-center text-muted">No referrer data available</td></tr>';
    } else {
        const total = data.reduce((sum, ref) => sum + ref.hits, 0);

        data.forEach((item, index) => {
            const percentage = total > 0 ? ((item.hits / total) * 100).toFixed(1) : 0;
            html += `
                <tr>
                    <td>${index + 1}</td>
                    <td><code>${LogLynxUtils.truncate(item.referrer || '-', 50)}</code></td>
                    <td>${LogLynxUtils.formatNumber(item.hits)}</td>
                    <td>${percentage}%</td>
                </tr>
            `;
        });
    }

    $('#topReferrersTable').html(html);
}

// Update top referrer domains table
function updateTopReferrerDomainsTable(data) {
    let html = '';

    if (!data || data.length === 0) {
        html = '<tr><td colspan="4" class="text-center text-muted">No referrer domain data available</td></tr>';
    } else {
        const total = data.reduce((sum, ref) => sum + ref.hits, 0);

        data.forEach((item, index) => {
            const percentage = total > 0 ? ((item.hits / total) * 100).toFixed(1) : 0;
            html += `
                <tr>
                    <td>${index + 1}</td>
                    <td><strong>${item.domain || 'Direct'}</strong></td>
                    <td>${LogLynxUtils.formatNumber(item.hits)}</td>
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

    $('#topReferrerDomainsTable').html(html);
}

// Initialize user agent DataTable
function initUserAgentTable(userAgentsData) {
    // Destroy existing DataTable if it exists
    if ($.fn.DataTable.isDataTable('#userAgentTable')) {
        $('#userAgentTable').DataTable().destroy();
    }

    $('#userAgentTable').DataTable({
        data: userAgentsData,
        columns: [
            {
                data: null,
                render: (data, type, row, meta) => meta.row + 1
            },
            {
                data: 'user_agent',
                render: (d) => `<code style="font-size: 0.75rem;">${LogLynxUtils.truncate(d || 'Unknown', 80)}</code>`
            },
            {
                data: 'count',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: 'user_agent',
                render: (d) => {
                    const ua = (d || '').toLowerCase();
                    if (ua.includes('bot') || ua.includes('crawl') || ua.includes('spider')) {
                        return '<span class="badge badge-warning">Bot</span>';
                    } else if (ua.includes('mobile')) {
                        return '<span class="badge badge-info">Mobile</span>';
                    } else {
                        return '<span class="badge badge-success">Browser</span>';
                    }
                }
            }
        ],
        order: [[2, 'desc']],
        pageLength: 20,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No user agent data available'
        }
    });
}

// Update geographic visitors table
function updateGeoVisitorsTable(data) {
    let html = '';

    if (!data || data.length === 0) {
        html = '<tr><td colspan="6" class="text-center text-muted">No geographic data available</td></tr>';
    } else {
        const totalVisitors = data.reduce((sum, c) => sum + (c.unique_visitors || 0), 0);

        data.forEach((item, index) => {
            const avgHits = item.unique_visitors > 0 ? (item.hits / item.unique_visitors).toFixed(1) : 0;
            const marketShare = totalVisitors > 0 ? ((item.unique_visitors / totalVisitors) * 100).toFixed(1) : 0;

            html += `
                <tr>
                    <td>${index + 1}</td>
                    <td>
                        <i class="fas fa-flag"></i>
                        <strong>${item.country || 'Unknown'}</strong>
                        ${item.country_name ? `<br><small class="text-muted">${item.country_name}</small>` : ''}
                    </td>
                    <td>${LogLynxUtils.formatNumber(item.unique_visitors || 0)}</td>
                    <td>${LogLynxUtils.formatNumber(item.hits)}</td>
                    <td>${avgHits}</td>
                    <td>
                        <div class="d-flex align-items-center gap-2">
                            <div style="width: 60px; height: 6px; background: #1f1f21; border-radius: 3px; overflow: hidden;">
                                <div style="width: ${marketShare}%; height: 100%; background: ${LogLynxCharts.colors.success};"></div>
                            </div>
                            <span>${marketShare}%</span>
                        </div>
                    </td>
                </tr>
            `;
        });
    }

    $('#geoVisitorsTable').html(html);
}

// Update bot detection
function updateBotDetection(deviceData) {
    if (!deviceData || deviceData.length === 0) return;

    let humanCount = 0;
    let botCount = 0;
    let unknownCount = 0;

    deviceData.forEach(d => {
        const type = (d.device_type || '').toLowerCase();
        if (type === 'bot') {
            botCount += d.count;
        } else if (type === 'desktop' || type === 'mobile' || type === 'tablet') {
            humanCount += d.count;
        } else {
            unknownCount += d.count;
        }
    });

    const total = humanCount + botCount + unknownCount;

    if (total > 0) {
        const humanPct = ((humanCount / total) * 100).toFixed(1);
        const botPct = ((botCount / total) * 100).toFixed(1);
        const unknownPct = ((unknownCount / total) * 100).toFixed(1);

        $('#humanTraffic').text(LogLynxUtils.formatNumber(humanCount));
        $('#humanPercent').text(humanPct + '%');

        $('#botTraffic').text(LogLynxUtils.formatNumber(botCount));
        $('#botPercent').text(botPct + '%');

        $('#unknownTraffic').text(LogLynxUtils.formatNumber(unknownCount));
        $('#unknownPercent').text(unknownPct + '%');
    }
}

// Update platform distribution
function updatePlatformDistribution(deviceData) {
    if (!deviceData || deviceData.length === 0) return;

    const deviceMap = { 'desktop': 0, 'mobile': 0, 'tablet': 0 };

    deviceData.forEach(d => {
        const type = (d.device_type || '').toLowerCase();
        if (deviceMap.hasOwnProperty(type)) {
            deviceMap[type] = d.count;
        }
    });

    const total = deviceMap.desktop + deviceMap.mobile + deviceMap.tablet;

    if (total > 0) {
        const desktopPct = ((deviceMap.desktop / total) * 100).toFixed(1);
        const mobilePct = ((deviceMap.mobile / total) * 100).toFixed(1);
        const tabletPct = ((deviceMap.tablet / total) * 100).toFixed(1);

        $('#desktopBar').css('width', desktopPct + '%');
        $('#desktopPlatformPercent').text(desktopPct + '%');

        $('#mobileBar').css('width', mobilePct + '%');
        $('#mobilePlatformPercent').text(mobilePct + '%');

        $('#tabletBar').css('width', tabletPct + '%');
        $('#tabletPlatformPercent').text(tabletPct + '%');
    }
}

// Update top insights
function updateTopInsights(deviceData) {
    // Top device
    if (deviceData && deviceData.length > 0) {
        const topDevice = deviceData.reduce((max, d) => d.count > max.count ? d : max, deviceData[0]);
        $('#topDevice').text((topDevice.device_type || 'Unknown').charAt(0).toUpperCase() + (topDevice.device_type || 'Unknown').slice(1));
    }

    // Top browser (set when browser data loads)
    if (allUserData.browsers && allUserData.browsers.length > 0) {
        $('#topBrowser').text(allUserData.browsers[0].browser || 'Unknown');
    }

    // Top OS (set when OS data loads)
    if (allUserData.os && allUserData.os.length > 0) {
        $('#topOS').text(allUserData.os[0].os || 'Unknown');
    }
}

// Export functions
function exportUserAnalytics() {
    const report = {
        browsers: allUserData.browsers,
        operating_systems: allUserData.os,
        devices: allUserData.devices,
        referrers: allUserData.referrers,
        referrer_domains: allUserData.referrerDomains,
        user_agents: allUserData.userAgents,
        countries: allUserData.countries
    };

    const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `user-analytics-${new Date().toISOString().split('T')[0]}.json`;
    link.click();

    LogLynxUtils.showNotification('User analytics exported', 'success', 3000);
}

function exportBrowserData() {
    const table = $('#browserTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'browser-analysis.csv');
}

function exportOSData() {
    const table = $('#osTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'os-analysis.csv');
}

function exportReferrersData() {
    if (allUserData.referrers) {
        LogLynxUtils.exportAsCSV(allUserData.referrers, 'top-referrers.csv');
    }
}

function exportReferrerDomainsData() {
    if (allUserData.referrerDomains) {
        LogLynxUtils.exportAsCSV(allUserData.referrerDomains, 'top-referrer-domains.csv');
    }
}

function exportUserAgentsData() {
    const table = $('#userAgentTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'user-agents.csv');
}

// Initialize service filter with reload callback
function initServiceFilterWithReload() {
    LogLynxUtils.initServiceFilter(() => {
        loadUserAnalyticsData();
    });
}

// Initialize page
// Initialize hide my traffic filter with reload callback
function initHideTrafficFilterWithReload() {
    LogLynxUtils.initHideMyTrafficFilter(() => {
        loadUserAnalyticsData();
    });
}

document.addEventListener('DOMContentLoaded', () => {
    // Initialize all charts
    initDeviceTypeChart();
    initBrowserChart();
    initOSChart();

    // Initialize controls
    initServiceFilterWithReload();
    initHideTrafficFilterWithReload();

    // Initial data load
    loadUserAnalyticsData();

    // Initialize refresh controls
    LogLynxUtils.initRefreshControls(loadUserAnalyticsData, 30);
});
