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
 * LogLynx Chart Utilities
 * Chart.js configuration and helper functions
 */

const LogLynxCharts = {
    // Chart.js default theme configuration
    defaultOptions: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
            legend: {
                labels: {
                    color: '#F3EFF3',
                    font: {
                        size: 13,
                        weight: '500'
                    },
                    padding: 15
                }
            },
            tooltip: {
                backgroundColor: '#1f1f21',
                titleColor: '#F3EFF3',
                bodyColor: '#F3EFF3',
                borderColor: '#444',
                borderWidth: 1,
                padding: 12,
                displayColors: true,
                titleFont: { size: 14, weight: 'bold' },
                bodyFont: { size: 13 },
                callbacks: {}
            }
        },
        scales: {
            x: {
                ticks: {
                    color: '#F3EFF3',
                    font: { size: 12 }
                },
                grid: {
                    color: 'rgba(243, 239, 243, 0.1)',
                    display: true
                },
                title: {
                    color: '#F3EFF3',
                    font: { size: 13, weight: '600' }
                }
            },
            y: {
                ticks: {
                    color: '#F3EFF3',
                    font: { size: 12 }
                },
                grid: {
                    color: 'rgba(243, 239, 243, 0.1)',
                    display: true
                },
                beginAtZero: true,
                title: {
                    color: '#F3EFF3',
                    font: { size: 13, weight: '600' }
                }
            }
        }
    },

    // Color schemes
    colors: {
        primary: '#F46319',
        primaryLight: '#ff7b4a',
        success: '#28a745',
        warning: '#ffc107',
        danger: '#dc3545',
        info: '#17a2b8',
        http2xx: '#28a745',
        http3xx: '#fd7e14',
        http4xx: '#ffc107',
        http5xx: '#dc3545',
        chartPalette: [
            '#F46319', '#28a745', '#17a2b8', '#ffc107',
            '#dc3545', '#6f42c1', '#fd7e14', '#20c997',
            '#e83e8c', '#6c757d'
        ]
    },

    /**
     * Create a line chart
     */
    createLineChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error(`Canvas ${canvasId} not found`);
            return null;
        }

        const config = {
            type: 'line',
            data: data,
            options: this.mergeOptions(options, {
                plugins: {
                    legend: { position: 'top' }
                }
            })
        };

        return new Chart(ctx.getContext('2d'), config);
    },

    /**
     * Create a bar chart
     */
    createBarChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error(`Canvas ${canvasId} not found`);
            return null;
        }

        const config = {
            type: 'bar',
            data: data,
            options: this.mergeOptions(options)
        };

        return new Chart(ctx.getContext('2d'), config);
    },

    /**
     * Create a horizontal bar chart
     */
    createHorizontalBarChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error(`Canvas ${canvasId} not found`);
            return null;
        }

        const config = {
            type: 'bar',
            data: data,
            options: this.mergeOptions(options, {
                indexAxis: 'y',
                plugins: {
                    legend: { display: false }
                }
            })
        };

        return new Chart(ctx.getContext('2d'), config);
    },

    /**
     * Create a doughnut/pie chart
     */
    createDoughnutChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error(`Canvas ${canvasId} not found`);
            return null;
        }

        const config = {
            type: 'doughnut',
            data: data,
            options: this.mergeOptions(options, {
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            color: '#F3EFF3',
                            font: { size: 12 },
                            padding: 15
                        }
                    }
                },
                cutout: '65%'
            })
        };

        return new Chart(ctx.getContext('2d'), config);
    },

    /**
     * Create an area chart (non-stacked) for status code timeline
     */
    createStackedAreaChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error(`Canvas ${canvasId} not found`);
            return null;
        }

        const config = {
            type: 'line',
            data: data,
            options: this.mergeOptions(options, {
                interaction: {
                    mode: 'index',
                    intersect: false
                },
                scales: {
                    x: {
                        ticks: { color: '#F3EFF3' },
                        grid: { color: 'rgba(243, 239, 243, 0.08)' }
                    },
                    y: {
                        beginAtZero: true,
                        ticks: { color: '#F3EFF3' },
                        grid: { color: 'rgba(243, 239, 243, 0.08)' }
                    }
                }
            })
        };

        return new Chart(ctx.getContext('2d'), config);
    },

    /**
     * Create a matrix/heatmap chart
     */
    createHeatmapChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error(`Canvas ${canvasId} not found`);
            return null;
        }

        const config = {
            type: 'matrix',
            data: data,
            options: this.mergeOptions(options, {
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    x: {
                        type: 'category',
                        offset: true,
                        ticks: {
                            color: '#F3EFF3',
                            maxRotation: 0,
                            autoSkip: true
                        },
                        grid: { display: false }
                    },
                    y: {
                        type: 'category',
                        offset: true,
                        reverse: true,
                        ticks: { color: '#F3EFF3' },
                        grid: { display: false }
                    }
                }
            })
        };

        return new Chart(ctx.getContext('2d'), config);
    },

    /**
     * Create a dual-axis chart (e.g., requests + response time)
     */
    createDualAxisChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error(`Canvas ${canvasId} not found`);
            return null;
        }

        const config = {
            type: 'line',
            data: data,
            options: this.mergeOptions(options, {
                interaction: {
                    mode: 'index',
                    intersect: false
                },
                scales: {
                    x: {
                        ticks: { color: '#F3EFF3' },
                        grid: { color: 'rgba(243, 239, 243, 0.08)' }
                    },
                    y: {
                        type: 'linear',
                        display: true,
                        position: 'left',
                        beginAtZero: true,
                        ticks: { color: '#28a745' },
                        grid: { color: 'rgba(40, 167, 69, 0.1)' }
                    },
                    y1: {
                        type: 'linear',
                        display: true,
                        position: 'right',
                        beginAtZero: true,
                        ticks: { color: '#17a2b8' },
                        grid: { drawOnChartArea: false }
                    }
                }
            })
        };

        return new Chart(ctx.getContext('2d'), config);
    },

    /**
     * Update chart data without recreating
     */
    updateChart(chart, newData, animationMode = 'active') {
        if (!chart) return;

        if (newData.labels) {
            chart.data.labels = newData.labels;
        }

        if (newData.datasets) {
            chart.data.datasets.forEach((dataset, i) => {
                if (newData.datasets[i]) {
                    dataset.data = newData.datasets[i].data || dataset.data;
                    if (newData.datasets[i].label) {
                        dataset.label = newData.datasets[i].label;
                    }
                }
            });
        }

        chart.update(animationMode);
    },

    /**
     * Destroy chart safely
     */
    destroyChart(chart) {
        if (chart && typeof chart.destroy === 'function') {
            chart.destroy();
        }
    },

    /**
     * Merge options with defaults
     */
    mergeOptions(customOptions, additionalDefaults = {}) {
        return this.deepMerge(
            {},
            this.defaultOptions,
            additionalDefaults,
            customOptions
        );
    },

    /**
     * Deep merge objects
     */
    deepMerge(target, ...sources) {
        if (!sources.length) return target;
        const source = sources.shift();

        if (this.isObject(target) && this.isObject(source)) {
            for (const key in source) {
                if (this.isObject(source[key])) {
                    if (!target[key]) Object.assign(target, { [key]: {} });
                    this.deepMerge(target[key], source[key]);
                } else {
                    Object.assign(target, { [key]: source[key] });
                }
            }
        }

        return this.deepMerge(target, ...sources);
    },

    isObject(item) {
        return item && typeof item === 'object' && !Array.isArray(item);
    },

    /**
     * Format timeline labels based on time range
     */
    formatTimelineLabels(dataPoints, hours) {
        const timeZone = (window.LOGLYNX_CONFIG && window.LOGLYNX_CONFIG.timeZone) ? window.LOGLYNX_CONFIG.timeZone : undefined;

        if (hours > 0 && hours <= 24) {
            // Hourly labels (HH:MM format)
            return dataPoints.map(d => {
                const date = new Date(d.hour);
                return date.toLocaleTimeString('en-US', {
                    hour: '2-digit',
                    minute: '2-digit',
                    hour12: false,
                    timeZone: timeZone
                });
            });
        } else if (hours > 0 && hours <= 168) {
            // Daily labels with day of week
            return dataPoints.map(d => {
                const date = new Date(d.hour);
                return date.toLocaleDateString('en-US', {
                    weekday: 'short',
                    month: 'short',
                    day: 'numeric',
                    timeZone: timeZone
                });
            });
        } else if (hours > 0 && hours <= 720) {
            // Daily labels for 30-day range
            return dataPoints.map(d => {
                const date = new Date(d.hour);
                return date.toLocaleDateString('en-US', {
                    month: 'short',
                    day: 'numeric',
                    timeZone: timeZone
                });
            });
        } else {
            // Weekly labels for longer periods (format: "2023-W52")
            return dataPoints.map(d => {
                // Check if d.hour is in week format (YYYY-WNN)
                if (typeof d.hour === 'string' && d.hour.match(/^\d{4}-W\d{1,2}$/)) {
                    const [year, weekStr] = d.hour.split('-W');
                    return `Week ${weekStr}, ${year}`;
                }
                // Fallback to date parsing
                const date = new Date(d.hour);
                if (!isNaN(date.getTime())) {
                    return date.toLocaleDateString('en-US', {
                        month: 'short',
                        day: 'numeric',
                        timeZone: timeZone
                    });
                }
                // If all else fails, return the raw value
                return d.hour;
            });
        }
    },

    /**
     * Generate heatmap data structure
     */
    generateHeatmapData(dataPoints, dayNames, hourLabels) {
        const lookup = new Map();

        if (Array.isArray(dataPoints)) {
            dataPoints.forEach(entry => {
                const day = parseInt(entry.day_of_week, 10);
                const hour = parseInt(entry.hour, 10);
                if (!isNaN(day) && !isNaN(hour)) {
                    const key = `${day}-${hour}`;
                    lookup.set(key, {
                        requests: entry.requests || 0,
                        avg: entry.avg_response_time || 0
                    });
                }
            });
        }

        const matrixData = [];
        let maxValue = 0;

        for (let day = 0; day < dayNames.length; day++) {
            for (let hour = 0; hour < hourLabels.length; hour++) {
                const key = `${day}-${hour}`;
                const stats = lookup.get(key) || { requests: 0, avg: 0 };

                matrixData.push({
                    x: hourLabels[hour],
                    y: dayNames[day],
                    v: stats.requests,
                    avg: stats.avg
                });

                if (stats.requests > maxValue) {
                    maxValue = stats.requests;
                }
            }
        }

        return { data: matrixData, maxValue: maxValue || 1 };
    },

    /**
     * Create heatmap color function
     */
    heatmapColorFunction(maxValue) {
        return (ctx) => {
            const raw = ctx.raw || {};
            const value = raw.v || 0;
            const intensity = maxValue === 0 ? 0 : value / maxValue;
            const alpha = intensity === 0 ? 0.05 : 0.25 + intensity * 0.6;
            return `rgba(244, 99, 25, ${Math.min(alpha, 0.9)})`;
        };
    },

    /**
     * Format number with locale
     */
    formatNumber(num) {
        return num.toLocaleString();
    },

    /**
     * Format bytes to human readable
     */
    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    },

    /**
     * Format milliseconds
     */
    formatMs(ms) {
        return ms.toFixed(2) + 'ms';
    },

    /**
     * Get color by index from palette
     */
    getColor(index) {
        return this.colors.chartPalette[index % this.colors.chartPalette.length];
    },

    /**
     * Get HTTP status color
     */
    getStatusColor(statusCode) {
        if (statusCode >= 200 && statusCode < 300) return this.colors.http2xx;
        if (statusCode >= 300 && statusCode < 400) return this.colors.http3xx;
        if (statusCode >= 400 && statusCode < 500) return this.colors.http4xx;
        if (statusCode >= 500) return this.colors.http5xx;
        return this.colors.chartPalette[9]; // gray for unknown
    },

    /**
     * Create dataset with defaults
     */
    createDataset(label, data, options = {}) {
        return {
            label: label,
            data: data,
            borderColor: options.borderColor || this.colors.primary,
            backgroundColor: options.backgroundColor || this.colors.primaryLight + '40', // Add transparency
            tension: options.tension !== undefined ? options.tension : 0.4,
            fill: options.fill !== undefined ? options.fill : false,
            pointRadius: options.pointRadius !== undefined ? options.pointRadius : 3,
            pointHoverRadius: options.pointHoverRadius !== undefined ? options.pointHoverRadius : 5,
            borderWidth: options.borderWidth !== undefined ? options.borderWidth : 2,
            ...options
        };
    },

    /**
     * Show loading state on chart
     */
    showLoading(containerId) {
        const container = document.getElementById(containerId);
        if (!container) return;

        const existingLoader = container.querySelector('.chart-loading');
        if (existingLoader) return;

        const loader = document.createElement('div');
        loader.className = 'chart-loading';
        loader.innerHTML = `
            <div class="loading-spinner-large"></div>
            <div class="chart-loading-text">Loading chart data...</div>
        `;
        container.appendChild(loader);
    },

    /**
     * Hide loading state
     */
    hideLoading(containerId) {
        const container = document.getElementById(containerId);
        if (!container) return;

        const loader = container.querySelector('.chart-loading');
        if (loader) {
            loader.remove();
        }
    },

    /**
     * Show empty state
     */
    showEmptyState(containerId, message = 'No data available') {
        // Delegate to the centralized utils function
        if (window.LogLynxUtils && LogLynxUtils.showEmptyState) {
            LogLynxUtils.showEmptyState(containerId, 'chart', message);
        } else {
            // Fallback to original implementation if utils is not available
            const container = document.getElementById(containerId);
            if (!container) return;

            container.innerHTML = `
                <div class="chart-empty">
                    <i class="fas fa-chart-line"></i>
                    <div class="chart-empty-text">${message}</div>
                </div>
            `;
        }
    },

    checkAndShowEmptyState(data, containerId, message = null) {
        let isEmpty = false;
        
        if (!data) {
            isEmpty = true;
        } else if (data.datasets && Array.isArray(data.datasets)) {
            // Check if all datasets are empty
            isEmpty = data.datasets.every(dataset =>
                !dataset.data ||
                (Array.isArray(dataset.data) && dataset.data.length === 0) ||
                (Array.isArray(dataset.data) && dataset.data.every(val => val === 0 || val === null))
            );
        } else if (data.labels && Array.isArray(data.labels)) {
            // Check if labels are empty
            isEmpty = data.labels.length === 0;
        }
        
        if (isEmpty) {
            this.showEmptyState(containerId, message || 'No chart data available');
        }
        
        return isEmpty;
    }
};

// Export for use in other scripts
window.LogLynxCharts = LogLynxCharts;

