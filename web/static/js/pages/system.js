/**
 * System Statistics Page
 */

let recordsTimelineChart;
let currentTimeRange = 30; // Default 30 days
let retentionDays = 365; // Default retention, will be updated from server

// Load system stats
async function loadSystemStats() {
    try {
        const result = await LogLynxAPI.getSystemStats();
        if (result.success) {
            // Update retention days from server response
            if (result.data.retention_days && result.data.retention_days > 0) {
                retentionDays = result.data.retention_days;
                updateAllButtonLabel();
            }
            updateSystemStats(result.data);
        } else {
            LogLynxUtils.showNotification('Failed to load system stats', 'error');
        }
    } catch (error) {
        console.error('Error loading system stats:', error);
        LogLynxUtils.showNotification('Failed to load system stats', 'error');
    }
}

// Update the "All" button label with retention days info
function updateAllButtonLabel() {
    const allBtn = document.getElementById('allTimeBtn');
    if (allBtn) {
        if (retentionDays > 0) {
            allBtn.textContent = `All (${retentionDays}d)`;
            allBtn.title = `Show all data within ${retentionDays} days retention period`;
        } else {
            allBtn.textContent = 'All';
            allBtn.title = 'Show all available data (no retention limit)';
        }
    }
}

// Load records timeline chart data
async function loadRecordsTimeline() {
    try {
        const result = await LogLynxAPI.getSystemTimeline(currentTimeRange);
        if (result.success) {
            updateRecordsTimelineChart(result.data);
        } else {
            console.error('Failed to load records timeline');
        }
    } catch (error) {
        console.error('Error loading records timeline:', error);
    }
}

// Update all system stat cards and tables
function updateSystemStats(data) {
    // Process Information
    $('#uptime').text(data.uptime || '-');
    $('#startTime').text('Started: ' + formatStartTime(data.start_time));
    $('#memoryAlloc').text(formatMB(data.memory_alloc_mb));
    $('#memorySys').text('System: ' + formatMB(data.memory_sys_mb));
    $('#numGoroutines').text(LogLynxUtils.formatNumber(data.num_goroutines || 0));
    $('#numCPU').text(`CPUs: ${data.num_cpu || 0}`);
    $('#gcPause').text(formatMs(data.gc_pause_ms));
    $('#goVersion').text(data.go_version || '-');
    $('#appVersion').text(data.app_version ? `v${data.app_version}` : '-');

    // Database Information
    $('#totalRecords').text(LogLynxUtils.formatNumber(data.total_records || 0));
    $('#databaseSize').text(formatMB(data.database_size_mb));
    $('#databasePath').text(truncatePath(data.database_path, 40));
    $('#recordsToCleanup').text(LogLynxUtils.formatNumber(data.records_to_cleanup || 0));

    // Retention info
    if (data.retention_days > 0) {
        $('#retentionDays').text(`Retention: ${data.retention_days} days`);
    } else {
        $('#retentionDays').text('Retention: Disabled');
    }

    $('#requestsPerSecond').text(data.requests_per_second ? data.requests_per_second.toFixed(2) : '0.00');

    // Cleanup Information
    $('#nextCleanupCountdown').text(data.next_cleanup_countdown || 'N/A');
    $('#nextCleanupTime').text('Scheduled: ' + (data.next_cleanup_time || 'N/A'));
    $('#lastCleanupTime').text(data.last_cleanup_time || 'Never');
    $('#oldestRecordAge').text(data.oldest_record_age || 'No records');
    $('#newestRecordAge').text('Newest: ' + (data.newest_record_age || 'No records'));

    // Update detailed table
    updateSystemDetailsTable(data);
}

// Update the detailed system information table
function updateSystemDetailsTable(data) {
    const details = [
        { label: 'Application Version', value: data.app_version ? `<a href="https://github.com/K0lin/loglynx/tree/v${data.app_version}" target="_blank" rel="noopener">v${data.app_version}</a>` : '-', icon: 'code-branch' },
        { label: 'Process Uptime', value: data.uptime, icon: 'clock' },
        { label: 'Uptime (seconds)', value: LogLynxUtils.formatNumber(data.uptime_seconds || 0), icon: 'stopwatch' },
        { label: 'Go Version', value: data.go_version, icon: 'code' },
        { label: 'CPU Cores', value: data.num_cpu, icon: 'microchip' },
        { label: 'Active Goroutines', value: LogLynxUtils.formatNumber(data.num_goroutines || 0), icon: 'stream' },
        { label: 'Memory Allocated', value: formatMB(data.memory_alloc_mb), icon: 'memory' },
        { label: 'Total Memory Allocated', value: formatMB(data.memory_total_mb), icon: 'hdd' },
        { label: 'System Memory', value: formatMB(data.memory_sys_mb), icon: 'server' },
        { label: 'GC Pause Time', value: formatMs(data.gc_pause_ms), icon: 'pause' },
        { label: 'Database Path', value: data.database_path, icon: 'folder-open' },
        { label: 'Database Size', value: formatMB(data.database_size_mb), icon: 'database' },
        { label: 'Total Records', value: LogLynxUtils.formatNumber(data.total_records || 0), icon: 'table' },
        { label: 'Records to Cleanup', value: LogLynxUtils.formatNumber(data.records_to_cleanup || 0), icon: 'trash' },
        { label: 'Retention Policy', value: data.retention_days > 0 ? `${data.retention_days} days` : 'Disabled', icon: 'calendar-alt' },
        { label: 'Next Cleanup', value: data.next_cleanup_time || 'N/A', icon: 'clock' },
        { label: 'Countdown to Cleanup', value: data.next_cleanup_countdown || 'N/A', icon: 'hourglass-half' },
        { label: 'Last Cleanup', value: data.last_cleanup_time || 'Never', icon: 'history' },
        { label: 'Oldest Record Age', value: data.oldest_record_age || 'No records', icon: 'calendar-times' },
        { label: 'Newest Record Age', value: data.newest_record_age || 'No records', icon: 'calendar-check' },
        { label: 'Ingestion Rate', value: data.requests_per_second ? `${data.requests_per_second.toFixed(4)} req/s` : '0.0000 req/s', icon: 'tachometer-alt' },
    ];

    let html = '';
    details.forEach(detail => {
        html += `
            <tr>
                <td style="width: 35%;"><i class="fas fa-${detail.icon} text-muted"></i> <strong>${detail.label}</strong></td>
                <td>${detail.value || '-'}</td>
            </tr>
        `;
    });

    $('#systemDetailsTable').html(html);
}

// Format megabytes
function formatMB(mb) {
    if (mb === undefined || mb === null) return '-';
    return mb.toFixed(2) + ' MB';
}

// Format milliseconds
function formatMs(ms) {
    if (ms === undefined || ms === null) return '-';
    return ms.toFixed(2) + ' ms';
}

// Format start time
function formatStartTime(isoString) {
    if (!isoString) return '-';
    const date = new Date(isoString);
    return date.toLocaleString();
}

// Truncate path for display
function truncatePath(path, maxLength) {
    if (!path) return '-';
    if (path.length <= maxLength) return path;

    // Show beginning and end of path
    const start = path.substring(0, maxLength / 2 - 2);
    const end = path.substring(path.length - (maxLength / 2 - 2));
    return start + '...' + end;
}

// Initialize records timeline chart
function initRecordsTimelineChart() {
    recordsTimelineChart = LogLynxCharts.createLineChart('recordsTimelineChart', {
        labels: [],
        datasets: [{
            label: 'Records Count',
            data: [],
            borderColor: LogLynxCharts.colors.primary,
            backgroundColor: LogLynxCharts.colors.primaryLight + '20',
            tension: 0.4,
            fill: true,
            pointRadius: 0,
            pointHitRadius: 20,
            borderWidth: 2
        }]
    }, {
        interaction: {
            mode: 'index',
            intersect: false,
            axis: 'x'
        },
        plugins: {
            legend: { display: false },
            tooltip: {
                callbacks: {
                    label: function(context) {
                        return context.dataset.label + ': ' +
                               LogLynxUtils.formatNumber(context.parsed.y);
                    }
                }
            }
        },
        scales: {
            x: {
                ticks: {
                    maxTicksLimit: 15,
                    autoSkip: true
                }
            },
            y: {
                beginAtZero: true,
                ticks: {
                    callback: function(value) {
                        return LogLynxUtils.formatNumber(value);
                    }
                }
            }
        }
    });
}

// Update records timeline chart
function updateRecordsTimelineChart(data) {
    // Check for empty data and show empty state if needed
    if (LogLynxCharts.checkAndShowEmptyState(
        { datasets: [{ data: data }] },
        'recordsTimelineChart',
        'No system records data available'
    )) {
        // Clear chart data when empty
        if (recordsTimelineChart) {
            recordsTimelineChart.data.labels = [];
            recordsTimelineChart.data.datasets[0].data = [];
            recordsTimelineChart.update('none');
        }
        return;
    }

    // Format labels based on time range
    const labels = data.map(d => {
        const date = new Date(d.hour);
        if (currentTimeRange <= 30) {
            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
        } else {
            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
        }
    });

    const records = data.map(d => d.requests);

    if (recordsTimelineChart) {
        recordsTimelineChart.data.labels = labels;
        recordsTimelineChart.data.datasets[0].data = records;
        recordsTimelineChart.update('none');
    }
}

// Initialize time range selector for chart
function initTimeRangeSelector() {
    document.querySelectorAll('.time-range-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            document.querySelectorAll('.time-range-btn').forEach(b => b.classList.remove('active'));
            this.classList.add('active');

            const daysAttr = this.getAttribute('data-days');

            // Handle "all" or numeric days
            if (daysAttr === 'all') {
                // Use retention days if set, otherwise use 365 as default
                currentTimeRange = retentionDays > 0 ? retentionDays : 365;
            } else {
                currentTimeRange = parseInt(daysAttr);
            }

            // Reload chart data
            loadRecordsTimeline();
        });
    });
}

// Initialize page
document.addEventListener('DOMContentLoaded', () => {
    // Initialize chart
    initRecordsTimelineChart();

    // Initialize time range selector
    initTimeRangeSelector();

    // Load all data initially
    loadSystemStats();
    loadRecordsTimeline();

    // Set up auto-refresh every 5 seconds
    LogLynxUtils.initRefreshControls(() => {
        loadSystemStats();
        loadRecordsTimeline();
    }, 5);
});
