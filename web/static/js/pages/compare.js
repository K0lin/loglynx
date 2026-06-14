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

let compareData = null;
let isSnapshotMode = false;
let requestsChart = null;
let responseChart = null;
let aggregateChart = null;
let statusChart = null;
let heatmapChart = null;
let statusCodeStackedChart = null;
let deviceMixChart = null;
let serviceVolumeChart = null;
let compareServiceOptions = [];
let currentInsights = [];

const compareColors = ['#F46319', '#17a2b8', '#28a745', '#ffc107'];
const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

document.addEventListener('DOMContentLoaded', async () => {
    isSnapshotMode = !!window.LogLynxCompareSnapshotToken;
    wireCompareEvents();

    if (isSnapshotMode) {
        await loadSnapshot(window.LogLynxCompareSnapshotToken);
    } else if (loadPeriodsFromUrl()) {
        await runComparison();
    } else {
        applyPreset('24v24');
        $('#presetSelector').val('24v24');
    }
});

function wireCompareEvents() {
    $('#presetSelector').on('change', function() {
        applyPreset(this.value);
    });
    $('#periodCount').on('change', function() {
        renderPeriodEditor(Number(this.value));
    });
    $('#runCompareBtn').on('click', runComparison);
    $('#createSnapshotBtn').on('click', openSnapshotModal);
    $('#confirmCreateSnapshotBtn').on('click', createSnapshot);
    $('#copyShareUrlBtn').on('click', () => copyText($('#shareUrl').val()));
    $('#exportJsonBtn').on('click', exportComparisonJson);
    $('#exportCsvBtn').on('click', exportComparisonCsv);
    $('#manageSnapshotsBtn').on('click', loadSnapshotManager);
    $('#toggleCompareSetupBtn').on('click', toggleCompareSetup);
    $(document).on('click', '.insight-item-clickable', function () {
        const idx = parseInt($(this).data('insight-idx'), 10);
        if (!isNaN(idx) && currentInsights[idx]) showInsightModal(currentInsights[idx]);
    });
    $('#serviceFocusSelect').on('change', () => {
        if (!compareData) return;
        if (isSnapshotMode) {
            renderComparison(compareData, { preserveServiceFocus: true });
            return;
        }
        runComparison({ preserveServiceFocus: true });
    });
}

function setSnapshotMode(enabled, title = '') {
    isSnapshotMode = enabled;
    $('#snapshotBanner').toggle(enabled);
    $('#compareControls').toggle(!enabled);
    $('#createSnapshotBtn').prop('disabled', enabled);
    $('#manageSnapshotsBtn').toggle(!enabled);
    $('#serviceFocusSelect').prop('disabled', enabled);
    if (enabled && title) {
        $('#snapshotBanner span').text(`Snapshot: ${title}. This report is frozen at creation time and does not re-query logs.`);
    }
}

async function loadSnapshot(token) {
    setSnapshotMode(true);
    const result = await LogLynxAPI.getComparisonSnapshot(token);
    if (!result.success) {
        $('#compareEmpty .card-body').html(`<h5>Snapshot unavailable</h5><p class="text-muted mb-0">${result.error || 'This link may be expired, disabled, or deleted.'}</p>`);
        return;
    }
    compareData = result.data.payload;
    setSnapshotMode(true, result.data.title || 'Comparison snapshot');
    renderComparison(compareData);
}

function applyPreset(preset) {
    const now = new Date();
    let periods;

    if (preset === '7v7') {
        periods = buildRollingPeriods(now, 7 * 24, 2, 'Last 7d');
    } else if (preset === '30v30') {
        periods = buildRollingPeriods(now, 30 * 24, 2, 'Last 30d');
    } else if (preset === 'same-hour-week') {
        const startToday = new Date(now);
        startToday.setHours(0, 0, 0, 0);
        const previousStart = new Date(startToday.getTime() - 7 * 24 * 60 * 60 * 1000);
        const previousEnd = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
        periods = [
            { label: 'Today so far', start: startToday, end: now },
            { label: 'Same hours last week', start: previousStart, end: previousEnd }
        ];
    } else if (preset === '24v24') {
        periods = buildRollingPeriods(now, 24, 2, 'Last 24h');
    } else {
        const currentCount = Number($('#periodCount').val() || 2);
        renderPeriodEditor(currentCount);
        return;
    }

    $('#periodCount').val(String(periods.length));
    renderPeriodEditor(periods.length, periods);
}

function buildRollingPeriods(end, hours, count, currentLabel) {
    const periods = [];
    for (let i = 0; i < count; i++) {
        const periodEnd = new Date(end.getTime() - i * hours * 60 * 60 * 1000);
        const periodStart = new Date(periodEnd.getTime() - hours * 60 * 60 * 1000);
        periods.unshift({
            label: i === 0 ? currentLabel : `Previous ${hours}h`,
            start: periodStart,
            end: periodEnd
        });
    }
    return periods.reverse();
}

function renderPeriodEditor(count, periods = null) {
    const now = new Date();
    const defaultPeriods = periods || buildRollingPeriods(now, 24, count, 'Period 1');
    let html = '';
    for (let i = 0; i < count; i++) {
        const period = defaultPeriods[i] || {
            label: `Period ${i + 1}`,
            start: new Date(now.getTime() - (i + 1) * 24 * 60 * 60 * 1000),
            end: new Date(now.getTime() - i * 24 * 60 * 60 * 1000)
        };
        html += `
            <div class="compare-period-card" data-period-index="${i}">
                <label class="form-label">Label</label>
                <input class="form-control form-control-sm period-label mb-2" value="${escapeHtml(period.label)}">
                <label class="form-label">Start</label>
                <input type="datetime-local" class="form-control form-control-sm period-start mb-2" value="${toDateTimeLocal(period.start)}">
                <label class="form-label">End</label>
                <input type="datetime-local" class="form-control form-control-sm period-end" value="${toDateTimeLocal(period.end)}">
            </div>
        `;
    }
    $('#periodEditor').html(html);
}

function collectPeriods() {
    return $('#periodEditor .compare-period-card').map(function(index) {
        const label = $(this).find('.period-label').val().trim() || `Period ${index + 1}`;
        const start = new Date($(this).find('.period-start').val());
        const end = new Date($(this).find('.period-end').val());
        return { label, start: start.toISOString(), end: end.toISOString() };
    }).get();
}

function loadPeriodsFromUrl() {
    const params = new URLSearchParams(window.location.search);
    const encoded = params.get('periods');
    if (!encoded) return false;
    try {
        const periods = JSON.parse(atob(encoded)).map(period => ({
            label: period.label,
            start: new Date(period.start),
            end: new Date(period.end)
        }));
        if (periods.length < 2 || periods.length > 4) return false;
        $('#periodCount').val(String(periods.length));
        $('#topLimit').val(params.get('top') || '10');
        $('#presetSelector').val('custom');
        renderPeriodEditor(periods.length, periods);
        return true;
    } catch (error) {
        console.warn('Failed to restore comparison periods from URL', error);
        return false;
    }
}

async function runComparison(options = {}) {
    const periods = collectPeriods();
    if (periods.length < 2 || periods.length > 4) {
        LogLynxUtils.showNotification('Choose 2 to 4 periods', 'warning');
        return;
    }
    if (periods.some(p => new Date(p.end) <= new Date(p.start))) {
        LogLynxUtils.showNotification('Each period end must be after its start', 'warning');
        return;
    }

    $('#runCompareBtn').prop('disabled', true).html('<i class="fas fa-circle-notch fa-spin"></i> Comparing...');
    const serviceFocus = $('#serviceFocusSelect').val() || '__all__';
    const apiOptions = serviceFocus === '__all__' ? {} : {
        'services[]': [serviceFocus],
        'service_types[]': ['auto']
    };
    const result = await LogLynxAPI.getComparison(periods, Number($('#topLimit').val() || 10), apiOptions);
    $('#runCompareBtn').prop('disabled', false).html('<i class="fas fa-chart-line"></i> Compare');

    if (!result.success) {
        LogLynxUtils.showNotification(result.error || 'Failed to run comparison', 'error');
        return;
    }
    compareData = result.data;
    renderComparison(compareData, { preserveServiceFocus: options.preserveServiceFocus });
    setCompareSetupCollapsed(true);
    updateLiveUrl(periods);
}

function toggleCompareSetup() {
    setCompareSetupCollapsed($('#compareSetupBody').is(':visible'));
}

function setCompareSetupCollapsed(collapsed) {
    $('#compareSetupBody').toggle(!collapsed);
    $('#toggleCompareSetupBtn')
        .attr('title', collapsed ? 'Expand setup' : 'Collapse setup')
        .html(`<i class="fas fa-chevron-${collapsed ? 'down' : 'up'}"></i>`);
}

function renderComparison(data, options = {}) {
    if (!data || !data.periods || data.periods.length === 0) return;
    $('#compareEmpty').hide();
    $('#compareResults').show();
    renderKpiCards(data);
    renderServiceFocusOptions(data, options.preserveServiceFocus);
    renderDrasticInsights(data);
    renderTimelineCharts(data);
    renderAggregateCharts(data);
    renderHeatmap(data);
    renderSummaryMatrix(data);
    renderVisualBreakdowns(data);
    renderComparisonTables(data);
}

function renderKpiCards(data) {
    const first = data.periods[0];
    const last = data.periods[data.periods.length - 1];
    const cards = [
        { label: 'Requests', key: 'total_requests', fmt: LogLynxUtils.formatNumber },
        { label: 'Bandwidth', key: 'total_bandwidth', fmt: LogLynxUtils.formatBytes },
        { label: 'Avg Response', key: 'avg_response_time', fmt: LogLynxUtils.formatMs, inverse: true },
        { label: 'Server Error Rate', key: 'server_error_rate', fmt: v => `${Number(v || 0).toFixed(2)}%`, inverse: true }
    ];
    $('#kpiDeltaCards').html(cards.map(card => {
        const base = first.summary[card.key] || 0;
        const current = last.summary[card.key] || 0;
        const delta = base === 0 ? 0 : ((current - base) / base) * 100;
        const cls = delta === 0 ? 'delta-neutral' : ((delta > 0) !== !!card.inverse ? 'delta-positive' : 'delta-negative');
        const sign = delta > 0 ? '+' : '';
        return `
            <div class="stat-card">
                <div class="stat-label">${card.label}</div>
                <div class="stat-value">${card.fmt(current)}</div>
                <div class="stat-subtitle ${cls}">${sign}${delta.toFixed(1)}% vs ${escapeHtml(first.label)}</div>
            </div>
        `;
    }).join(''));
}

function renderDrasticInsights(data) {
    const insights = buildDrasticInsights(data);
    currentInsights = insights;
    const counts = insights.reduce((acc, item) => {
        acc.total += 1;
        acc[item.severity] = (acc[item.severity] || 0) + 1;
        return acc;
    }, { total: 0, high: 0, medium: 0, low: 0 });

    const $header = $('#smartChangesHeaderBadges');
    if (insights.length === 0) {
        $header.hide();
        $('#drasticInsights').html(`
            <div class="card-header">
                <h5 class="card-title"><i class="fas fa-wand-magic-sparkles"></i> Smart Change Analysis</h5>
                <span class="badge badge-secondary">0 changes</span>
            </div>
            <div class="card-body text-muted">No drastic changes detected across the selected periods.</div>
        `);
        return;
    }

    const badgesHtml = `
        <span class="badge badge-secondary">${counts.total} changes</span>
        ${counts.high   ? `<span class="badge badge-danger">${counts.high} high</span>`     : ''}
        ${counts.medium ? `<span class="badge badge-warning">${counts.medium} medium</span>` : ''}
        ${counts.low    ? `<span class="badge badge-info">${counts.low} low</span>`          : ''}
    `;
    $header.html(`<small class="text-muted me-1"><i class="fas fa-wand-magic-sparkles"></i> Smart Changes</small>${badgesHtml}`).show();

    $('#drasticInsights').html(`
        <div class="card-header">
            <h5 class="card-title"><i class="fas fa-wand-magic-sparkles"></i> Smart Change Analysis</h5>
            <div class="d-flex gap-2 flex-wrap">${badgesHtml}</div>
        </div>
        <div class="card-body">
            <div class="list-group list-group-flush">
                ${insights.map((item, idx) => `
                    <div class="list-group-item insight-item-clickable" data-insight-idx="${idx}"
                         style="background: transparent; color: inherit; border-color: var(--border-color); cursor: pointer; transition: background 0.15s;"
                         onmouseover="this.style.background='rgba(255,255,255,0.04)'" onmouseout="this.style.background='transparent'">
                        <div class="d-flex justify-content-between gap-3 align-items-start">
                            <strong class="${item.className}">${escapeHtml(item.title)} ${item.trend || ''}</strong>
                            <div class="d-flex gap-2 align-items-center flex-shrink-0">
                                <span class="badge ${item.badge}">${item.severity}</span>
                                <i class="fas fa-chevron-right text-muted" style="font-size: 0.7rem;"></i>
                            </div>
                        </div>
                        <div class="text-muted small mt-1">${escapeHtml(item.detail)}</div>
                    </div>
                `).join('')}
            </div>
        </div>
    `);
}

function buildDrasticInsights(data) {
    const first = data.periods[0];
    const last = data.periods[data.periods.length - 1];
    const periodLabels = data.periods.map((p, i) => p.label || `Period ${i + 1}`);
    const insights = [];

    const periodSummaries = data.periods.map(p => p.summary || {});
    addMetricInsight(insights, 'Traffic volume', first.summary.total_requests, last.summary.total_requests, 'requests', false,
        { type: 'metric', metricKey: 'total_requests', allValues: data.periods.map(p => Number(p.summary.total_requests || 0)), periodLabels, periodSummaries });
    addMetricInsight(insights, 'Bandwidth', first.summary.total_bandwidth, last.summary.total_bandwidth, 'bytes', false,
        { type: 'metric', metricKey: 'total_bandwidth', allValues: data.periods.map(p => Number(p.summary.total_bandwidth || 0)), periodLabels, periodSummaries });
    addMetricInsight(insights, 'Average response time', first.summary.avg_response_time, last.summary.avg_response_time, 'ms', true,
        { type: 'metric', metricKey: 'avg_response_time', allValues: data.periods.map(p => Number(p.summary.avg_response_time || 0)), periodLabels, periodSummaries });
    addMetricInsight(insights, 'Server error rate', first.summary.server_error_rate, last.summary.server_error_rate, 'percent', true,
        { type: 'metric', metricKey: 'server_error_rate', allValues: data.periods.map(p => Number(p.summary.server_error_rate || 0)), periodLabels, periodSummaries });

    addEntityInsights(insights, data, 'service_breakdown', 'backend_name', 'Service');
    addEntityInsights(insights, data, 'top_paths', 'path', 'Path');
    addEntityInsights(insights, data, 'top_ips', 'ip_address', 'IP');
    addEntityInsights(insights, data, 'top_countries', 'country', 'Country');

    return insights.sort((a, b) => b.score - a.score);
}

function addMetricInsight(insights, title, before, after, unit, inverse = false, extraData = {}) {
    before = Number(before || 0);
    after = Number(after || 0);
    const delta = after - before;
    const pct = before === 0 ? (after > 0 ? 100 : 0) : (delta / before) * 100;
    if (Math.abs(pct) < 20 && Math.abs(delta) < 50) return;
    const bad = inverse ? delta > 0 : delta < 0;
    const severity = Math.abs(pct) >= 100 ? 'high' : Math.abs(pct) >= 40 ? 'medium' : 'low';
    insights.push({
        title: `${title} ${delta >= 0 ? 'increased' : 'decreased'} ${Math.abs(pct).toFixed(1)}%`,
        detail: `${formatInsightValue(before, unit)} → ${formatInsightValue(after, unit)} across first and last selected period.`,
        severity,
        badge: severity === 'high' ? 'badge-danger' : severity === 'medium' ? 'badge-warning' : 'badge-info',
        className: bad ? 'delta-negative' : 'delta-positive',
        score: Math.abs(pct) + Math.log10(Math.abs(delta) + 1),
        unit,
        ...extraData
    });
}

function addEntityInsights(insights, data, collectionKey, key, label) {
    const periodLabels = data.periods.map((p, i) => p.label || `Period ${i + 1}`);
    const totalRequestsPerPeriod = data.periods.map(p => Number((p.summary || {}).total_requests || 0));
    buildMatrixRows(data, collectionKey, key).slice(0, 8).forEach(row => {
        if (Math.abs(row.last_delta_pct) < 30 && Math.abs(row.last_delta) < 25) return;
        const severity = Math.abs(row.last_delta_pct) >= 150 ? 'high' : Math.abs(row.last_delta_pct) >= 75 ? 'medium' : 'low';
        insights.push({
            type: 'entity',
            entityName: row.name,
            entityLabel: label,
            values: row.values,
            bandwidthValues: row.bandwidthValues,
            responseValues: row.responseValues,
            totalRequestsPerPeriod,
            periodLabels,
            title: `${label} changed sharply: ${LogLynxUtils.truncate(row.name, 80)}`,
            detail: `${LogLynxUtils.formatNumber(row.values[0] || 0)} → ${LogLynxUtils.formatNumber(row.values[row.values.length - 1] || 0)} requests (${row.last_delta_pct >= 0 ? '+' : ''}${row.last_delta_pct.toFixed(1)}%).`,
            trend: row.trend,
            severity,
            badge: severity === 'high' ? 'badge-danger' : severity === 'medium' ? 'badge-warning' : 'badge-info',
            className: row.last_delta >= 0 ? 'delta-positive' : 'delta-negative',
            score: Math.abs(row.last_delta_pct) + Math.abs(row.last_delta)
        });
    });
}

function formatInsightValue(value, unit) {
    if (unit === 'bytes') return LogLynxUtils.formatBytes(value);
    if (unit === 'ms') return LogLynxUtils.formatMs(value);
    if (unit === 'percent') return `${Number(value || 0).toFixed(2)}%`;
    return LogLynxUtils.formatNumber(value);
}

function showInsightModal(insight) {
    const body = insight.type === 'entity' ? buildEntityModalBody(insight) : buildMetricModalBody(insight);
    $('#insightModalTitle').html(
        `<span class="${insight.className}">${escapeHtml(insight.title)}</span>` +
        ` <span class="badge ${insight.badge} ms-2">${insight.severity}</span>`
    );
    $('#insightModalBody').html(body);
    bootstrap.Modal.getOrCreateInstance(document.getElementById('insightDetailModal')).show();
}

function generateTrendDescription(values) {
    if (!values || values.length < 2) return '';
    const nonZeroCount = values.filter(v => v > 0).length;
    if (nonZeroCount === 0) return 'No activity detected in any of the selected periods.';

    const firstNonZero = values.findIndex(v => v > 0);
    const lastNonZero  = values.map((v, i) => v > 0 ? i : -1).filter(i => i >= 0).at(-1);
    if (firstNonZero > 0) return `Not present in the first ${firstNonZero} period(s), appeared starting from period ${firstNonZero + 1}.`;
    if (lastNonZero  < values.length - 1) return `Active through period ${lastNonZero + 1}, then disappeared in subsequent periods.`;

    let up = true, down = true;
    for (let i = 1; i < values.length; i++) {
        if (values[i] <= values[i - 1]) up   = false;
        if (values[i] >= values[i - 1]) down = false;
    }
    if (up)   return 'Consistent upward trend, values increased in every consecutive period.';
    if (down) return 'Consistent downward trend, values decreased in every consecutive period.';

    const maxIdx = values.indexOf(Math.max(...values));
    const minIdx = values.indexOf(Math.min(...values));
    if (maxIdx > 0 && maxIdx < values.length - 1) return `Peak in period ${maxIdx + 1}, with lower values before and after, possible temporary spike.`;
    if (minIdx > 0 && minIdx < values.length - 1) return `Dip in period ${minIdx + 1}, with higher values before and after, possible temporary drop or incident.`;

    return 'Mixed trend, no clear monotonic pattern across the selected periods.';
}

function insightDeltaCell(pct, absDeltaStr) {
    if (pct === null) return `<span class="text-muted">—</span>`;
    const cls  = pct >= 0 ? 'delta-positive' : 'delta-negative';
    const sign = pct >= 0 ? '+' : '';
    return `<span class="${cls}">${sign}${pct.toFixed(1)}%</span><br><small class="text-muted">${absDeltaStr}</small>`;
}

function buildMetricModalBody(insight) {
    const vals = insight.allValues || [];
    const rows = vals.map((val, i) => {
        const prev    = i === 0 ? null : vals[i - 1];
        const delta   = prev === null ? null : val - prev;
        const pct     = prev === null ? null : (prev === 0 ? (val > 0 ? 100 : 0) : (delta / prev) * 100);
        const absDelta = delta === null ? '' : `${delta >= 0 ? '+' : ''}${formatInsightValue(delta, insight.unit)}`;
        return `<tr>
            <td><small class="text-muted">${escapeHtml(insight.periodLabels[i] || `Period ${i + 1}`)}</small></td>
            <td><strong>${formatInsightValue(val, insight.unit)}</strong></td>
            <td>${insightDeltaCell(pct, absDelta)}</td>
        </tr>`;
    }).join('');

    // Related metrics section
    const allContextDefs = [
        { key: 'total_requests',    label: 'Total requests',          fmt: LogLynxUtils.formatNumber, unit: 'requests' },
        { key: 'unique_visitors',   label: 'Unique visitors',         fmt: LogLynxUtils.formatNumber, unit: 'requests' },
        { key: 'valid_requests',    label: 'Successful (2xx/3xx)',     fmt: LogLynxUtils.formatNumber, unit: 'requests' },
        { key: 'failed_requests',   label: 'Failed (4xx/5xx)',         fmt: LogLynxUtils.formatNumber, unit: 'requests' },
        { key: 'total_bandwidth',   label: 'Total bandwidth',          fmt: LogLynxUtils.formatBytes,  unit: 'bytes' },
        { key: 'avg_response_time', label: 'Avg response time',        fmt: LogLynxUtils.formatMs,     unit: 'ms' },
        { key: 'server_error_rate', label: 'Server error rate',        fmt: v => `${Number(v||0).toFixed(2)}%`, unit: 'percent' },
    ];
    let contextHtml = '';
    if (insight.periodSummaries) {
        const ctxRows = allContextDefs
            .filter(m => m.key !== insight.metricKey)
            .map(m => {
                const mVals  = insight.periodSummaries.map(s => Number(s[m.key] || 0));
                const mFirst = mVals[0], mLast = mVals[mVals.length - 1];
                const mDelta = mLast - mFirst;
                const mPct   = mFirst === 0 ? (mLast > 0 ? 100 : 0) : (mDelta / mFirst) * 100;
                const cls    = Math.abs(mPct) < 3 ? 'text-muted' : mPct > 0 ? 'delta-positive' : 'delta-negative';
                const sign   = mPct >= 0 ? '+' : '';
                return `<tr>
                    <td class="text-muted small">${m.label}</td>
                    <td class="small">${m.fmt(mFirst)}</td>
                    <td class="small">${m.fmt(mLast)}</td>
                    <td><small class="${cls}">${sign}${mPct.toFixed(1)}%</small></td>
                </tr>`;
            }).join('');
        contextHtml = `
            <div class="mt-4">
                <small class="text-muted fw-semibold text-uppercase" style="font-size:0.7rem;letter-spacing:.05em;">Related metrics (first → last period)</small>
                <div class="table-responsive mt-2">
                    <table class="table table-sm mb-0" style="color:inherit;">
                        <thead><tr>
                            <th class="text-muted fw-normal small">Metric</th>
                            <th class="text-muted fw-normal small">First period</th>
                            <th class="text-muted fw-normal small">Last period</th>
                            <th class="text-muted fw-normal small">Δ overall</th>
                        </tr></thead>
                        <tbody>${ctxRows}</tbody>
                    </table>
                </div>
            </div>`;
    }

    const trend = generateTrendDescription(vals);
    return `
        <p class="text-muted small mb-3">${escapeHtml(insight.detail)}</p>
        ${trend ? `<div class="mb-3 px-3 py-2" style="background:rgba(255,255,255,0.04);border-radius:8px;border-left:3px solid var(--border-color);">
            <small><i class="fas fa-chart-line me-1 text-muted"></i>${escapeHtml(trend)}</small>
        </div>` : ''}
        <div class="table-responsive">
            <table class="table table-sm" style="color:inherit;">
                <thead><tr>
                    <th class="text-muted fw-normal small">Period</th>
                    <th class="text-muted fw-normal small">Value</th>
                    <th class="text-muted fw-normal small">Δ vs previous</th>
                </tr></thead>
                <tbody>${rows}</tbody>
            </table>
        </div>
        ${contextHtml}`;
}

function buildEntityModalBody(insight) {
    const hasBw      = (insight.bandwidthValues      || []).some(v => v > 0);
    const hasResp    = (insight.responseValues        || []).some(v => v > 0);
    const hasTotals  = (insight.totalRequestsPerPeriod || []).some(v => v > 0);
    const maxVal     = Math.max(...(insight.values || [1]));

    const rows = (insight.values || []).map((val, i) => {
        const prev   = i === 0 ? null : insight.values[i - 1];
        const delta  = prev === null ? null : val - prev;
        const pct    = prev === null ? null : (prev === 0 ? (val > 0 ? 100 : 0) : (delta / prev) * 100);
        const absDelta = delta === null ? '' : `${delta >= 0 ? '+' : ''}${LogLynxUtils.formatNumber(delta)}`;
        const total  = hasTotals ? (insight.totalRequestsPerPeriod[i] || 0) : 0;
        const share  = (hasTotals && total > 0) ? `${((val / total) * 100).toFixed(1)}%` : '—';
        const barW   = maxVal > 0 ? Math.round((val / maxVal) * 60) : 0;
        const barHtml = `<span style="display:inline-block;width:${barW}px;height:6px;background:var(--primary-color,#F46319);border-radius:3px;vertical-align:middle;opacity:.7;"></span>`;
        return `<tr>
            <td><small class="text-muted">${escapeHtml(insight.periodLabels[i] || `Period ${i + 1}`)}</small></td>
            <td><strong>${LogLynxUtils.formatNumber(val)}</strong> ${barHtml}</td>
            <td>${insightDeltaCell(pct, absDelta)}</td>
            ${hasTotals ? `<td class="text-muted small">${share}</td>` : ''}
            ${hasBw   ? `<td class="text-muted small">${LogLynxUtils.formatBytes(insight.bandwidthValues[i] || 0)}</td>` : ''}
            ${hasResp ? `<td class="text-muted small">${LogLynxUtils.formatMs(insight.responseValues[i] || 0)}</td>` : ''}
        </tr>`;
    }).join('');

    const trend = generateTrendDescription(insight.values || []);
    return `
        <div class="mb-3 px-3 py-2" style="background:rgba(255,255,255,0.04);border-radius:8px;">
            <small class="text-muted fw-semibold text-uppercase" style="font-size:0.7rem;letter-spacing:.05em;">${escapeHtml(insight.entityLabel)}</small>
            <div class="mt-1" style="word-break:break-all;font-family:monospace;font-size:0.9rem;">${escapeHtml(insight.entityName)}</div>
        </div>
        <p class="text-muted small mb-3">${escapeHtml(insight.detail)}</p>
        ${trend ? `<div class="mb-3 px-3 py-2" style="background:rgba(255,255,255,0.04);border-radius:8px;border-left:3px solid var(--border-color);">
            <small><i class="fas fa-chart-line me-1 text-muted"></i>${escapeHtml(trend)}</small>
        </div>` : ''}
        <div class="table-responsive">
            <table class="table table-sm" style="color:inherit;">
                <thead><tr>
                    <th class="text-muted fw-normal small">Period</th>
                    <th class="text-muted fw-normal small">Requests</th>
                    <th class="text-muted fw-normal small">Δ vs previous</th>
                    ${hasTotals ? `<th class="text-muted fw-normal small">% of total</th>` : ''}
                    ${hasBw   ? `<th class="text-muted fw-normal small">Bandwidth</th>` : ''}
                    ${hasResp ? `<th class="text-muted fw-normal small">Avg response</th>` : ''}
                </tr></thead>
                <tbody>${rows}</tbody>
            </table>
        </div>`;
}

function renderTimelineCharts(data) {
    destroyChart(requestsChart);
    destroyChart(responseChart);
    const serviceName = $('#serviceFocusSelect').val() || '__all__';
    const labels = buildTimelineLabels(data, serviceName);

    requestsChart = LogLynxCharts.createLineChart('requestsCompareChart', {
        labels,
        datasets: data.periods.map((period, index) => buildTimelineDataset(period, index, 'requests', 'Requests', serviceName))
    }, compareLineOptions(v => LogLynxUtils.formatNumber(v)));

    responseChart = LogLynxCharts.createLineChart('responseCompareChart', {
        labels,
        datasets: data.periods.map((period, index) => buildTimelineDataset(period, index, 'avg_response_time', 'Avg response', serviceName))
    }, compareLineOptions(v => LogLynxUtils.formatMs(v)));
}

function buildTimelineLabels(data, serviceName = '__all__') {
    const maxPoints = Math.max(...data.periods.map(p => getPeriodTimeline(p, serviceName).length));
    return Array.from({ length: maxPoints }, (_, i) => `Bucket ${i + 1}`);
}

function buildTimelineDataset(period, index, key, label, serviceName = '__all__') {
    const timeline = getPeriodTimeline(period, serviceName);
    return {
        label: `${period.label} ${label}`,
        data: timeline.map(point => point[key] || 0),
        borderColor: compareColors[index % compareColors.length],
        backgroundColor: compareColors[index % compareColors.length] + '22',
        tension: 0.3,
        fill: false,
        pointRadius: 0,
        pointHitRadius: 14,
        borderWidth: 2
    };
}

function getPeriodTimeline(period, serviceName) {
    if (serviceName === '__all__') return period.timeline || [];
    return (period.service_timeline || []).filter(point => point.service_name === serviceName);
}

function renderServiceFocusOptions(data, preserveCurrent = false) {
    const current = $('#serviceFocusSelect').val() || '__all__';
    if (!preserveCurrent || compareServiceOptions.length === 0 || current === '__all__') {
        compareServiceOptions = Array.from(new Set(data.periods.flatMap(period =>
            (period.service_breakdown || []).map(service => service.backend_name).filter(Boolean)
        ))).sort();
    }
    const options = ['<option value="__all__">All services</option>'].concat(
        compareServiceOptions.map(service => `<option value="${escapeHtml(service)}">${escapeHtml(LogLynxUtils.truncate(service, 80))}</option>`)
    );
    $('#serviceFocusSelect').html(options.join(''));
    if (compareServiceOptions.includes(current)) $('#serviceFocusSelect').val(current);
}

function compareLineOptions(formatter) {
    return {
        interaction: { mode: 'index', intersect: false },
        plugins: {
            tooltip: {
                callbacks: {
                    label: context => `${context.dataset.label}: ${formatter(context.parsed.y || 0)}`
                }
            }
        },
        scales: { x: { ticks: { maxTicksLimit: 12 } } }
    };
}

function renderAggregateCharts(data) {
    destroyChart(aggregateChart);
    destroyChart(statusChart);
    const labels = data.periods.map(p => p.label);

    aggregateChart = LogLynxCharts.createBarChart('aggregateCompareChart', {
        labels,
        datasets: [
            { label: 'Requests', data: data.periods.map(p => p.summary.total_requests || 0), backgroundColor: '#F46319' },
            { label: 'Unique visitors', data: data.periods.map(p => p.summary.unique_visitors || 0), backgroundColor: '#17a2b8' }
        ]
    });

    statusChart = LogLynxCharts.createBarChart('statusCompareChart', {
        labels,
        datasets: [
            { label: '2xx/3xx', data: data.periods.map(p => p.summary.valid_requests || 0), backgroundColor: '#28a745' },
            { label: '4xx/5xx', data: data.periods.map(p => p.summary.failed_requests || 0), backgroundColor: '#dc3545' }
        ]
    }, { scales: { x: { stacked: true }, y: { stacked: true } } });
}

function renderSummaryMatrix(data) {
    const metrics = [
        { name: 'Total requests', key: 'total_requests', fmt: LogLynxUtils.formatNumber },
        { name: 'Unique visitors', key: 'unique_visitors', fmt: LogLynxUtils.formatNumber },
        { name: 'Valid requests', key: 'valid_requests', fmt: LogLynxUtils.formatNumber },
        { name: 'Failed requests', key: 'failed_requests', fmt: LogLynxUtils.formatNumber },
        { name: 'Total bandwidth', key: 'total_bandwidth', fmt: LogLynxUtils.formatBytes },
        { name: 'Avg response time', key: 'avg_response_time', fmt: LogLynxUtils.formatMs },
        { name: 'Requests/hour', key: 'requests_per_hour', fmt: v => LogLynxUtils.formatNumber(Math.round(v || 0)) },
        { name: 'Success rate', key: 'success_rate', fmt: v => `${Number(v || 0).toFixed(2)}%` },
        { name: '404 rate', key: 'not_found_rate', fmt: v => `${Number(v || 0).toFixed(2)}%` },
        { name: 'Server error rate', key: 'server_error_rate', fmt: v => `${Number(v || 0).toFixed(2)}%` }
    ];
    const rows = metrics.map(metric => {
        const values = data.periods.map(period => period.summary[metric.key] || 0);
        return {
            metric: metric.name,
            values,
            fmt: metric.fmt,
            delta: values[values.length - 1] - values[0],
            deltaPct: values[0] === 0 ? (values[values.length - 1] > 0 ? 100 : 0) : ((values[values.length - 1] - values[0]) / values[0]) * 100,
            trend: buildSparkline(values)
        };
    });
    if ($.fn.DataTable.isDataTable('#summaryMatrixTable')) $('#summaryMatrixTable').DataTable().destroy();
    $('#summaryMatrixTable').empty().DataTable({
        data: rows,
        columns: [
            { title: 'Metric', data: 'metric' },
            ...data.periods.map((period, index) => ({
                title: period.label,
                data: null,
                render: (row, type) => type === 'display' ? row.fmt(row.values[index]) : row.values[index]
            })),
            { title: 'First -> Last', data: 'delta', render: (value, type) => type === 'display' ? renderDelta(Number(value.toFixed ? value.toFixed(2) : value)) : value },
            { title: 'Change %', data: 'deltaPct', render: (value, type) => type === 'display' ? renderDeltaPct(value) : value },
            { title: 'Trend', data: 'trend', orderable: false }
        ],
        paging: false,
        searching: false,
        info: false,
        autoWidth: false
    });
}

function renderVisualBreakdowns(data) {
    renderStatusCodeStackedChart(data);
    renderDeviceMixChart(data);
    renderServiceVolumeChart(data);
}

function renderStatusCodeStackedChart(data) {
    destroyChart(statusCodeStackedChart);
    const labels = data.periods.map(period => period.label);
    const codes = Array.from(new Set(data.periods.flatMap(period =>
        (period.status_code_distribution || []).map(row => row.status_code)
    ))).sort((a, b) => Number(a) - Number(b));
    const datasets = codes.map((code, index) => ({
        label: String(code),
        data: data.periods.map(period => {
            const row = (period.status_code_distribution || []).find(item => item.status_code === code);
            return row ? row.count || 0 : 0;
        }),
        backgroundColor: statusCodeColor(code, index)
    }));

    statusCodeStackedChart = LogLynxCharts.createBarChart('statusCodeStackedChart', { labels, datasets }, {
        scales: { x: { stacked: true }, y: { stacked: true } },
        plugins: {
            tooltip: {
                callbacks: {
                    label: context => `${context.dataset.label}: ${LogLynxUtils.formatNumber(context.parsed.y || 0)}`
                }
            }
        }
    });
}

function renderDeviceMixChart(data) {
    destroyChart(deviceMixChart);
    const labels = data.periods.map(period => period.label);
    const types = Array.from(new Set(data.periods.flatMap(period =>
        (period.device_types || []).map(row => row.device_type || 'unknown')
    ))).sort();
    const datasets = types.map((type, index) => ({
        label: type,
        data: data.periods.map(period => {
            const row = (period.device_types || []).find(item => (item.device_type || 'unknown') === type);
            return row ? row.count || 0 : 0;
        }),
        backgroundColor: compareColors[index % compareColors.length]
    }));

    deviceMixChart = LogLynxCharts.createBarChart('deviceMixChart', { labels, datasets }, {
        scales: { x: { stacked: true }, y: { stacked: true } },
        plugins: {
            tooltip: {
                callbacks: {
                    label: context => `${context.dataset.label}: ${LogLynxUtils.formatNumber(context.parsed.y || 0)}`
                }
            }
        }
    });
}

function renderServiceVolumeChart(data) {
    destroyChart(serviceVolumeChart);
    const rows = buildMatrixRows(data, 'service_breakdown', 'backend_name').slice(0, 8);
    const labels = rows.map(row => LogLynxUtils.truncate(row.name, 42));
    const datasets = data.periods.map((period, index) => ({
        label: period.label,
        data: rows.map(row => row.values[index] || 0),
        backgroundColor: compareColors[index % compareColors.length]
    }));

    serviceVolumeChart = LogLynxCharts.createBarChart('serviceVolumeChart', { labels, datasets }, {
        plugins: {
            tooltip: {
                callbacks: {
                    label: context => `${context.dataset.label}: ${LogLynxUtils.formatNumber(context.parsed.y || 0)} requests`
                }
            }
        },
        scales: { x: { ticks: { maxRotation: 35, minRotation: 0 } } }
    });
}

function statusCodeColor(code, index) {
    const numericCode = Number(code);
    const variant = (index % 4) * 18;
    if (numericCode >= 200 && numericCode < 300) return `hsl(134, 61%, ${38 + variant / 2}%)`;
    if (numericCode >= 300 && numericCode < 400) return `hsl(28, 95%, ${48 + variant / 2}%)`;
    if (numericCode >= 400 && numericCode < 500) return `hsl(45, 100%, ${45 + variant / 2}%)`;
    if (numericCode >= 500) return `hsl(354, 70%, ${46 + variant / 2}%)`;
    return compareColors[index % compareColors.length];
}

function renderHeatmap(data) {
    destroyChart(heatmapChart);
    const canvas = document.getElementById('heatmapCompareChart');
    if (!canvas) return;

    const yLabels = [];
    const matrixData = [];
    const compactHourly = data.periods.every(period => {
        const start = new Date(period.start);
        const end = new Date(period.end);
        return (end - start) <= 25 * 60 * 60 * 1000;
    });

    data.periods.forEach((period, periodIndex) => {
        if (compactHourly) {
            yLabels.push(period.label);
            const hourMap = new Map((period.timeline || []).map(point => [new Date(point.hour).getHours(), point.requests || 0]));
            for (let hour = 0; hour < 24; hour++) {
                matrixData.push({
                    x: hour,
                    y: periodIndex,
                    v: hourMap.get(hour) || 0
                });
            }
            return;
        }

        dayNames.forEach(day => yLabels.push(`${period.label} ${day}`));
        (period.heatmap || []).forEach(point => {
            matrixData.push({
                x: point.hour,
                y: periodIndex * 7 + point.day_of_week,
                v: point.requests || 0
            });
        });
    });
    const maxValue = Math.max(1, ...matrixData.map(d => d.v));
    heatmapChart = new Chart(canvas.getContext('2d'), {
        type: 'matrix',
        data: {
            datasets: [{
                label: 'Requests',
                data: matrixData,
                width: ({ chart }) => Math.max((chart.chartArea || {}).width / 24 - 2, 6),
                height: ({ chart }) => Math.max((chart.chartArea || {}).height / yLabels.length - 2, 6),
                backgroundColor: ctx => `rgba(244, 99, 25, ${Math.max(0.08, (ctx.raw.v || 0) / maxValue)})`,
                borderColor: 'rgba(255,255,255,0.08)',
                borderWidth: 1
            }]
        },
        options: LogLynxCharts.mergeOptions({
            plugins: {
                legend: { display: false },
                tooltip: {
                    callbacks: {
                        title: items => `Hour ${items[0].raw.x}:00`,
                        label: item => `${yLabels[item.raw.y]}: ${LogLynxUtils.formatNumber(item.raw.v)} requests`
                    }
                }
            },
            scales: {
                x: { type: 'linear', min: -0.5, max: 23.5, ticks: { stepSize: 1 } },
                y: { type: 'linear', min: -0.5, max: yLabels.length - 0.5, ticks: { callback: value => yLabels[value] || '' } }
            }
        })
    });
}

function renderComparisonTables(data) {
    renderMatrixTable('#pathDiffTable', data, 'top_paths', 'path', 'Path');
    renderMatrixTable('#ipDiffTable', data, 'top_ips', 'ip_address', 'IP');
    renderMatrixTable('#backendDiffTable', data, 'top_backends', 'backend_name', 'Backend');
    renderMatrixTable('#countryDiffTable', data, 'top_countries', 'country', 'Country');
    renderMatrixTable('#serviceBreakdownTable', data, 'service_breakdown', 'backend_name', 'Service');
    renderMatrixTable('#deviceTypeTable', data, 'device_types', 'device_type', 'Device Type', { valueKey: 'count', bytes: false });
}

function buildMatrixRows(data, collectionKey, key, options = {}) {
    const valueKey = options.valueKey || 'hits';
    const names = new Set();
    const maps = data.periods.map(period => {
        const map = new Map();
        (period[collectionKey] || []).forEach(row => {
            const name = String(row[key] || 'Unknown');
            names.add(name);
            map.set(name, row);
        });
        return map;
    });

    return Array.from(names).map(name => {
        const values = maps.map(map => {
            const row = map.get(name) || {};
            return row[valueKey] || 0;
        });
        const bandwidthValues = maps.map(map => {
            const row = map.get(name) || {};
            return row.total_bandwidth || row.bandwidth || 0;
        });
        const responseValues = maps.map(map => {
            const row = map.get(name) || {};
            return row.avg_response_time || 0;
        });
        const min = Math.min(...values);
        const max = Math.max(...values);
        const first = values[0] || 0;
        const last = values[values.length - 1] || 0;
        return {
            name,
            values,
            bandwidthValues,
            responseValues,
            max_delta: max - min,
            last_delta: last - first,
            last_delta_pct: first === 0 ? (last > 0 ? 100 : 0) : ((last - first) / first) * 100,
            trend: buildSparkline(values)
        };
    }).sort((a, b) => Math.abs(b.max_delta) - Math.abs(a.max_delta)).slice(0, 50);
}

function renderMatrixTable(selector, data, collectionKey, key, firstColumn, options = {}) {
    const rows = buildMatrixRows(data, collectionKey, key, options);
    if ($.fn.DataTable.isDataTable(selector)) {
        $(selector).DataTable().destroy();
    }
    $(selector).empty();
    const periodColumns = data.periods.map((period, index) => ({
        title: period.label,
        data: null,
        render: (row, type) => {
            const value = row.values[index] || 0;
            return type === 'display' ? LogLynxUtils.formatNumber(value) : value;
        }
    }));
    $(selector).DataTable({
        data: rows,
        columns: [
            { title: firstColumn, data: 'name', render: data => `<code>${escapeHtml(LogLynxUtils.truncate(data, 55))}</code>` },
            ...periodColumns,
            { title: 'Range', data: 'max_delta', render: (value, type) => type === 'display' ? LogLynxUtils.formatNumber(value) : value },
            { title: 'First -> Last', data: 'last_delta', render: (value, type) => type === 'display' ? renderDelta(value) : value },
            { title: 'Change %', data: 'last_delta_pct', render: (value, type) => type === 'display' ? renderDeltaPct(value) : value },
            { title: 'Trend', data: 'trend', orderable: false }
        ],
        order: [[data.periods.length + 1, 'desc']],
        pageLength: 10,
        autoWidth: false,
        responsive: true
    });
}

function buildSparkline(values) {
    const first = values[0] || 0;
    const last = values[values.length - 1] || 0;
    const min = Math.min(...values);
    const max = Math.max(...values);
    const range = max - min;
    const pct = first === 0 ? (last > 0 ? 100 : 0) : ((last - first) / first) * 100;

    if (range === 0 || Math.abs(pct) < 5) {
        return '<span class="delta-neutral" title="Stable"><i class="fas fa-minus"></i></span>';
    }
    if (pct > 0) {
        return '<span class="delta-positive" title="Increasing"><i class="fas fa-arrow-trend-up"></i></span>';
    }
    if (pct < 0) {
        return '<span class="delta-negative" title="Decreasing"><i class="fas fa-arrow-trend-down"></i></span>';
    }
    return '<span class="delta-neutral" title="Variable"><i class="fas fa-chart-line"></i></span>';
}

function renderDelta(value) {
    const cls = value > 0 ? 'delta-positive' : value < 0 ? 'delta-negative' : 'delta-neutral';
    const sign = value > 0 ? '+' : '';
    return `<span class="${cls}">${sign}${LogLynxUtils.formatNumber(value)}</span>`;
}

function renderDeltaPct(value) {
    const cls = value > 0 ? 'delta-positive' : value < 0 ? 'delta-negative' : 'delta-neutral';
    const sign = value > 0 ? '+' : '';
    return `<span class="${cls}">${sign}${Number(value || 0).toFixed(1)}%</span>`;
}

function openSnapshotModal() {
    if (!compareData) {
        LogLynxUtils.showNotification('Run a comparison before creating a snapshot', 'warning');
        return;
    }
    $('#shareUrlBox').hide();
    const modal = new bootstrap.Modal(document.getElementById('shareSnapshotModal'));
    modal.show();
}

async function createSnapshot() {
    if (!compareData) return;
    const title = $('#snapshotTitle').val().trim() || 'Comparison snapshot';
    const expiresRaw = $('#snapshotExpiresAt').val();
    const expiresAt = expiresRaw ? new Date(expiresRaw).toISOString() : null;
    const result = await LogLynxAPI.createComparisonSnapshot(title, compareData, expiresAt);
    if (!result.success) {
        LogLynxUtils.showNotification(result.error || 'Failed to create snapshot', 'error');
        return;
    }
    const absoluteUrl = new URL(result.data.url, window.location.origin).toString();
    $('#shareUrl').val(absoluteUrl);
    $('#shareUrlBox').show();
    LogLynxUtils.showNotification('Snapshot link created', 'success');
}

async function loadSnapshotManager() {
    const result = await LogLynxAPI.listComparisonSnapshots();
    if (!result.success) {
        LogLynxUtils.showNotification(result.error || 'Failed to load snapshots', 'error');
        return;
    }
    if ($.fn.DataTable.isDataTable('#snapshotTable')) {
        $('#snapshotTable').DataTable().destroy();
    }
    $('#snapshotTable').DataTable({
        data: result.data || [],
        columns: [
            { title: 'Title', data: 'title' },
            { title: 'Created', data: 'created_at', render: data => data ? new Date(data).toLocaleString() : '-' },
            { title: 'Expires', data: null, render: row => renderSnapshotExpiration(row) },
            { title: 'Status', data: null, render: row => renderSnapshotStatus(row) },
            {
                title: 'Actions',
                data: null,
                orderable: false,
                render: row => {
                    const url = `/compare/${row.token}`;
                    const isExpired = row.expires_at && new Date(row.expires_at).getTime() <= Date.now();
                    const toggleLabel = row.active && !isExpired ? 'Disable' : 'Enable';
                    const nextActive = !(row.active && !isExpired);
                    return `
                        <div class="d-flex gap-1 flex-wrap">
                            <a class="btn btn-sm btn-outline" href="${url}" target="_blank">Open</a>
                            <button class="btn btn-sm btn-outline" title="Copy link" onclick="copySnapshotLink('${row.token}')"><i class="fas fa-copy"></i></button>
                            <button class="btn btn-sm btn-outline" onclick="toggleSnapshot('${row.token}', ${nextActive})">${toggleLabel}</button>
                            <button class="btn btn-sm btn-danger" onclick="deleteSnapshot('${row.token}')">Delete</button>
                        </div>
                    `;
                }
            }
        ],
        order: [[1, 'desc']],
        pageLength: 10
    });
}

function renderSnapshotExpiration(row) {
    const display = row.expires_at ? new Date(row.expires_at).toLocaleString() : 'Never';
    const pickerValue = row.expires_at ? toDateTimeLocal(row.expires_at) : '';
    return `
        <div class="d-flex align-items-center gap-2 flex-wrap">
            <span>${display}</span>
            <input type="datetime-local"
                   class="form-control form-control-sm"
                   style="width: 1px; height: 1px; opacity: 0; position: absolute; pointer-events: none;"
                   id="snapshot-exp-${row.token}"
                   value="${pickerValue}"
                   onchange="updateSnapshotExpirationFromPicker('${row.token}', this.value)">
            <button class="btn btn-sm btn-outline" title="Edit expiration" onclick="openSnapshotExpirationPicker('${row.token}')">
                <i class="fas fa-calendar-alt"></i>
            </button>
            ${row.expires_at ? `<button class="btn btn-sm btn-outline" title="Remove expiration" onclick="clearSnapshotExpiration('${row.token}')"><i class="fas fa-infinity"></i></button>` : ''}
        </div>
    `;
}

function renderSnapshotStatus(row) {
    if (row.expires_at && new Date(row.expires_at).getTime() <= Date.now()) {
        return '<span class="badge badge-warning">Expired</span>';
    }
    if (!row.active) {
        return '<span class="badge badge-secondary">Disabled</span>';
    }
    return '<span class="badge badge-success">Active</span>';
}

function copySnapshotLink(token) {
    copyText(new URL(`/compare/${token}`, window.location.origin).toString());
}

function openSnapshotExpirationPicker(token) {
    const input = document.getElementById(`snapshot-exp-${token}`);
    if (!input) return;
    if (typeof input.showPicker === 'function') {
        input.showPicker();
    } else {
        input.click();
        input.focus();
    }
}

async function toggleSnapshot(token, active) {
    const updates = { active };
    if (active) {
        updates.expires_at = null;
    }
    const result = await LogLynxAPI.updateComparisonSnapshot(token, updates);
    if (!result.success) {
        LogLynxUtils.showNotification(result.error || 'Failed to update snapshot', 'error');
        return;
    }
    await loadSnapshotManager();
}

async function updateSnapshotExpirationFromPicker(token, value) {
    const expiresAt = value.trim() ? new Date(value).toISOString() : null;
    const result = await LogLynxAPI.updateComparisonSnapshot(token, {
        active: true,
        expires_at: expiresAt
    });
    if (!result.success) {
        LogLynxUtils.showNotification(result.error || 'Failed to update expiration', 'error');
        return;
    }
    await loadSnapshotManager();
}

async function clearSnapshotExpiration(token) {
    const result = await LogLynxAPI.updateComparisonSnapshot(token, {
        active: true,
        expires_at: null
    });
    if (!result.success) {
        LogLynxUtils.showNotification(result.error || 'Failed to remove expiration', 'error');
        return;
    }
    await loadSnapshotManager();
}

async function deleteSnapshot(token) {
    if (!confirm('Delete this snapshot link?')) return;
    const result = await LogLynxAPI.deleteComparisonSnapshot(token);
    if (!result.success) {
        LogLynxUtils.showNotification(result.error || 'Failed to delete snapshot', 'error');
        return;
    }
    await loadSnapshotManager();
}

function exportComparisonJson() {
    if (!compareData) return;
    downloadFile('loglynx-comparison.json', JSON.stringify(compareData, null, 2), 'application/json');
}

function exportComparisonCsv() {
    if (!compareData) return;
    const lines = ['period,start,end,total_requests,unique_visitors,total_bandwidth,avg_response_time,server_error_rate'];
    compareData.periods.forEach(period => {
        lines.push([
            csv(period.label), period.start, period.end,
            period.summary.total_requests || 0,
            period.summary.unique_visitors || 0,
            period.summary.total_bandwidth || 0,
            period.summary.avg_response_time || 0,
            period.summary.server_error_rate || 0
        ].join(','));
    });
    downloadFile('loglynx-comparison.csv', lines.join('\n'), 'text/csv');
}

function updateLiveUrl(periods) {
    const params = new URLSearchParams();
    params.set('periods', btoa(JSON.stringify(periods)));
    params.set('top', $('#topLimit').val());
    window.history.replaceState({}, '', `${window.location.pathname}?${params.toString()}`);
}

function toDateTimeLocal(dateLike) {
    const date = new Date(dateLike);
    const offsetMs = date.getTimezoneOffset() * 60 * 1000;
    return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16);
}

function destroyChart(chart) {
    if (chart && typeof chart.destroy === 'function') chart.destroy();
}

function escapeHtml(value) {
    return String(value || '').replace(/[&<>'"]/g, char => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
    }[char]));
}

function csv(value) {
    return `"${String(value || '').replace(/"/g, '""')}"`;
}

function downloadFile(filename, content, type) {
    const blob = new Blob([content], { type });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
}

async function copyText(text) {
    await navigator.clipboard.writeText(text);
    LogLynxUtils.showNotification('Copied to clipboard', 'success');
}
