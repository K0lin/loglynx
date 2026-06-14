(function() {
    const params = new URLSearchParams(window.location.search);
    const theme = params.get('theme') || document.body.className.replace('theme-', '') || 'dark';
    const initialMode = document.getElementById('widget')?.dataset.initialMode || params.get('time') || 'realtime';

    const TIME_CONFIG = {
        realtime: { hours: 1, label: 'Live', badge: 'live', refresh: 5000 },
        '1h': { hours: 1, label: '1H', badge: '1h', refresh: 60000 },
        '24h': { hours: 24, label: '24H', badge: '24h', refresh: 60000 },
        '7d': { hours: 168, label: '7D', badge: '7d', refresh: 60000 },
        '30d': { hours: 720, label: '30D', badge: '30d', refresh: 60000 }
    };

    let mode = TIME_CONFIG[initialMode] ? initialMode : 'realtime';
    let refreshTimer = null;
    let timelinePoints = [];
    let hoverIndex = null;

    const els = {
        statusDot: document.getElementById('statusDot'),
        statusText: document.getElementById('statusText'),
        statusMeta: document.getElementById('statusMeta'),
        timeBadge: document.getElementById('timeBadge'),
        labelPrimary: document.getElementById('labelPrimary'),
        valuePrimary: document.getElementById('valuePrimary'),
        labelSecondary: document.getElementById('labelSecondary'),
        valueSecondary: document.getElementById('valueSecondary'),
        labelChart: document.getElementById('labelChart'),
        timelineSubtitle: document.getElementById('timelineSubtitle'),
        lastUpdated: document.getElementById('lastUpdated'),
        errorRate: document.getElementById('errorRate'),
        avgResponse: document.getElementById('avgResponse'),
        canvas: document.getElementById('sparkline'),
        tooltip: document.getElementById('chartTooltip')
    };

    function currentConfig() {
        return TIME_CONFIG[mode] || TIME_CONFIG.realtime;
    }

    function apiURL() {
        if (mode === 'realtime') return '/api/v1/widget/data';
        return `/api/v1/widget/summary?hours=${currentConfig().hours}`;
    }

    function timelineURL() {
        return `/api/v1/widget/timeline?hours=${currentConfig().hours}`;
    }

    function formatNumber(num) {
        const value = Number(num) || 0;
        if (value >= 1000000) return (value / 1000000).toFixed(1) + 'M';
        if (value >= 1000) return (value / 1000).toFixed(1) + 'K';
        return Math.round(value).toString();
    }

    function formatBytes(bytes) {
        const value = Number(bytes) || 0;
        if (value >= 1024 * 1024 * 1024) return (value / 1024 / 1024 / 1024).toFixed(1) + ' GB';
        if (value >= 1024 * 1024) return (value / 1024 / 1024).toFixed(1) + ' MB';
        if (value >= 1024) return (value / 1024).toFixed(1) + ' KB';
        return Math.round(value) + ' B';
    }

    function formatMs(ms) {
        const value = Number(ms) || 0;
        if (value >= 1000) return (value / 1000).toFixed(2) + 's';
        return Math.round(value) + 'ms';
    }

    function formatPointTime(raw) {
        if (!raw) return '-';
        if (String(raw).includes('-W')) return raw;
        const date = new Date(raw);
        if (Number.isNaN(date.getTime())) return raw;
        const datePart = date.toLocaleDateString([], { month: 'short', day: 'numeric' });
        const timePart = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        return `${datePart} ${timePart}`;
    }

    function setStatus(status, isStatic) {
        els.statusDot.className = 'status-dot' + (isStatic ? ' static' : '');

        if (status === 'healthy') {
            els.statusText.textContent = mode === 'realtime' ? 'Live traffic' : 'Healthy';
        } else if (status === 'warning') {
            els.statusDot.classList.add('warning');
            els.statusText.textContent = 'Warning';
        } else if (status === 'danger') {
            els.statusDot.classList.add('danger');
            els.statusText.textContent = 'Critical';
        } else {
            els.statusDot.classList.add('danger');
            els.statusText.textContent = 'Error';
        }
    }

    function setValue(el, value, className, unit) {
        el.className = 'metric-value' + (className ? ' ' + className : '');
        el.innerHTML = unit ? `${value}<span class="unit">${unit}</span>` : value;
    }

    function responseClass(ms) {
        if (ms > 1000) return 'danger';
        if (ms > 500) return 'warning';
        return 'success';
    }

    function errorClass(rate) {
        if (rate > 5) return 'danger';
        if (rate > 1) return 'warning';
        return 'success';
    }

    function updateStaticLabels() {
        const config = currentConfig();
        els.timeBadge.textContent = config.badge;
        els.labelChart.textContent = mode === 'realtime' ? 'Traffic (live)' : `Traffic (${config.label})`;
        els.timelineSubtitle.textContent = 'Hover the line for time and traffic';

        if (mode === 'realtime') {
            els.labelPrimary.textContent = 'Requests/min';
            els.labelSecondary.textContent = 'Unique IPs';
        } else {
            els.labelPrimary.textContent = 'Total Requests';
            els.labelSecondary.textContent = 'Unique IPs';
        }

        document.querySelectorAll('.widget-range-tabs button').forEach(button => {
            button.classList.toggle('active', button.dataset.mode === mode);
        });
    }

    function setupCanvas() {
        const rect = els.canvas.getBoundingClientRect();
        const dpr = window.devicePixelRatio || 1;
        els.canvas.width = Math.max(1, Math.floor(rect.width * dpr));
        els.canvas.height = Math.max(1, Math.floor(rect.height * dpr));
        const ctx = els.canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        return { ctx, width: rect.width, height: rect.height };
    }

    function drawTimeline() {
        const { ctx, width, height } = setupCanvas();
        ctx.clearRect(0, 0, width, height);

        const values = timelinePoints.map(point => Number(point.requests) || 0);
        if (values.length === 0) return;

        const padding = { top: 8, right: 8, bottom: 16, left: 8 };
        const chartWidth = Math.max(1, width - padding.left - padding.right);
        const chartHeight = Math.max(1, height - padding.top - padding.bottom);
        const max = Math.max(...values, 1);
        const min = Math.min(...values, 0);
        const range = max - min || 1;
        const stepX = values.length > 1 ? chartWidth / (values.length - 1) : chartWidth;
        const isLight = document.body.classList.contains('theme-light');

        ctx.strokeStyle = isLight ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.08)';
        ctx.lineWidth = 1;
        for (let i = 0; i < 3; i++) {
            const y = padding.top + (chartHeight / 2) * i;
            ctx.beginPath();
            ctx.moveTo(padding.left, y);
            ctx.lineTo(width - padding.right, y);
            ctx.stroke();
        }

        const points = values.map((value, index) => ({
            x: padding.left + stepX * index,
            y: padding.top + chartHeight - ((value - min) / range) * chartHeight,
            value
        }));

        if (points.length === 1) {
            points.push({ x: width - padding.right, y: points[0].y, value: points[0].value });
        }

        const gradient = ctx.createLinearGradient(0, padding.top, 0, height - padding.bottom);
        gradient.addColorStop(0, 'rgba(255, 107, 53, 0.34)');
        gradient.addColorStop(1, 'rgba(255, 107, 53, 0.02)');

        ctx.beginPath();
        points.forEach((point, index) => {
            if (index === 0) ctx.moveTo(point.x, point.y);
            else ctx.lineTo(point.x, point.y);
        });
        ctx.lineTo(points[points.length - 1].x, height - padding.bottom);
        ctx.lineTo(points[0].x, height - padding.bottom);
        ctx.closePath();
        ctx.fillStyle = gradient;
        ctx.fill();

        ctx.beginPath();
        points.forEach((point, index) => {
            if (index === 0) ctx.moveTo(point.x, point.y);
            else ctx.lineTo(point.x, point.y);
        });
        ctx.strokeStyle = '#ff6b35';
        ctx.lineWidth = 2;
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
        ctx.stroke();

        if (hoverIndex !== null && points[hoverIndex]) {
            const point = points[hoverIndex];
            ctx.strokeStyle = 'rgba(255, 107, 53, 0.45)';
            ctx.beginPath();
            ctx.moveTo(point.x, padding.top);
            ctx.lineTo(point.x, height - padding.bottom);
            ctx.stroke();

            ctx.fillStyle = '#ff6b35';
            ctx.beginPath();
            ctx.arc(point.x, point.y, 3.5, 0, Math.PI * 2);
            ctx.fill();
        }
    }

    function showTooltip(event) {
        if (timelinePoints.length === 0) return;

        const rect = els.canvas.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const ratio = Math.max(0, Math.min(1, x / Math.max(1, rect.width)));
        hoverIndex = Math.round(ratio * (timelinePoints.length - 1));
        const point = timelinePoints[hoverIndex];
        if (!point) return;

        els.tooltip.hidden = false;
        els.tooltip.style.left = `${Math.max(68, Math.min(rect.width - 68, x))}px`;
        els.tooltip.style.top = `${Math.max(20, event.clientY - rect.top)}px`;
        els.tooltip.innerHTML = `
            <strong>${formatPointTime(point.hour)}</strong><br>
            ${formatNumber(point.requests)} requests<br>
            ${formatBytes(point.bandwidth || 0)} transferred<br>
            ${formatMs(point.avg_response_time || 0)} avg response
        `;
        drawTimeline();
    }

    function hideTooltip() {
        hoverIndex = null;
        els.tooltip.hidden = true;
        drawTimeline();
    }

    async function fetchTimeline() {
        const response = await fetch(timelineURL());
        if (!response.ok) return;
        const data = await response.json();
        timelinePoints = Array.isArray(data) ? data : [];
        drawTimeline();
    }

    async function fetchRealtimeData() {
        const response = await fetch(apiURL());
        if (!response.ok) throw new Error('API error');
        const data = await response.json();

        setStatus(data.status, false);
        setValue(els.valuePrimary, formatNumber(data.requests_per_minute || 0));

        const rate = Number(data.error_rate) || 0;
        setValue(els.errorRate, rate.toFixed(1), errorClass(rate), '%');

        const avgResponse = Number(data.avg_response_ms ?? data.avg_response_time) || 0;
        setValue(els.avgResponse, Math.round(avgResponse), responseClass(avgResponse), 'ms');
        setValue(els.valueSecondary, formatNumber(data.unique_ips || 0));

        timelinePoints.push({
            hour: new Date().toISOString(),
            requests: data.requests_per_minute || 0,
            bandwidth: data.bandwidth_per_minute || 0,
            avg_response_time: avgResponse
        });
        if (timelinePoints.length > 30) timelinePoints.shift();
        drawTimeline();
    }

    async function fetchSummaryData() {
        const [summaryRes, timelineRes] = await Promise.all([fetch(apiURL()), fetch(timelineURL())]);
        if (!summaryRes.ok) throw new Error('API error');
        const data = await summaryRes.json();

        setStatus(data.status, true);
        setValue(els.valuePrimary, formatNumber(data.total_requests || 0));

        const rate = Number(data.error_rate) || 0;
        setValue(els.errorRate, rate.toFixed(1), errorClass(rate), '%');

        const avgResponse = Number(data.avg_response_ms ?? data.avg_response_time) || 0;
        setValue(els.avgResponse, Math.round(avgResponse), responseClass(avgResponse), 'ms');
        setValue(els.valueSecondary, formatNumber(data.unique_ips || 0));

        if (timelineRes.ok) {
            const timeline = await timelineRes.json();
            timelinePoints = Array.isArray(timeline) ? timeline : [];
            drawTimeline();
        }
    }

    async function fetchData() {
        try {
            if (mode === 'realtime') await fetchRealtimeData();
            else await fetchSummaryData();

            const now = new Date();
            els.lastUpdated.textContent = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            els.statusMeta.textContent = mode === 'realtime' ? 'Refreshes every 5 seconds' : `Last ${currentConfig().label}`;
        } catch (error) {
            console.error('Widget fetch error:', error);
            setStatus('error', mode !== 'realtime');
            els.statusMeta.textContent = 'Unable to load widget data';
        }
    }

    function setMode(nextMode) {
        mode = TIME_CONFIG[nextMode] ? nextMode : 'realtime';
        hoverIndex = null;
        timelinePoints = [];
        updateStaticLabels();
        if (refreshTimer) clearInterval(refreshTimer);
        fetchData();
        refreshTimer = setInterval(fetchData, currentConfig().refresh);

        const url = new URL(window.location.href);
        url.searchParams.set('time', mode);
        if (theme) url.searchParams.set('theme', theme);
        window.history.replaceState({}, '', url.toString());
    }

    function init() {
        document.body.classList.toggle('theme-light', theme === 'light');
        document.querySelectorAll('.widget-range-tabs button').forEach(button => {
            button.addEventListener('click', () => setMode(button.dataset.mode));
        });

        els.canvas.addEventListener('mousemove', showTooltip);
        els.canvas.addEventListener('mouseleave', hideTooltip);
        window.addEventListener('resize', drawTimeline);

        setMode(mode);
    }

    init();
})();
