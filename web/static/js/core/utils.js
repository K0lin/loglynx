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
 * LogLynx Utilities
 * Common helper functions and UI utilities
 */

const LogLynxUtils = {
    // Event handler references for cleanup
    _serviceFilterDocClickHandler: null,
    _hideTrafficDocClickHandler: null,
    _filterChangeDebounceTimer: null,
    _hideTrafficChangeDebounceTimer: null,

    /**
     * Persisted time range helpers (shared across pages)
     */
    getPreferredTimeRangeHours(defaultHours = 168) {
        try {
            const stored = localStorage.getItem('loglynx_time_range_hours');
            const parsed = parseInt(stored, 10);
            if (!Number.isNaN(parsed) && parsed >= 0 && parsed <= 8760) {
                return parsed;
            }
        } catch (error) {
            // Ignore storage errors and fall back to default
            console.debug('Time range preference unavailable:', error);
        }
        return defaultHours;
    },

    setPreferredTimeRangeHours(hours) {
        const parsed = parseInt(hours, 10);
        if (Number.isNaN(parsed) || parsed < 0 || parsed > 8760) {
            return;
        }
        try {
            localStorage.setItem('loglynx_time_range_hours', parsed.toString());
        } catch (error) {
            console.debug('Unable to persist time range preference:', error);
        }
    },

    /**
     * Show notification
     */
    showNotification(message, type = 'info', duration = 5000) {
        // Remove existing notification
        const existing = document.querySelector('.notification');
        if (existing) {
            existing.remove();
        }

        // Create notification
        const notification = document.createElement('div');
        notification.className = `notification notification-${type} show`;
        notification.innerHTML = `
            <i class="notification-icon fas ${this.getNotificationIcon(type)}"></i>
            <div class="notification-content">${message}</div>
            <button class="notification-close" onclick="this.parentElement.remove()">
                <i class="fas fa-times"></i>
            </button>
        `;

        document.body.appendChild(notification);

        // Auto-hide
        if (duration > 0) {
            setTimeout(() => {
                notification.classList.remove('show');
                setTimeout(() => notification.remove(), 300);
            }, duration);
        }

        return notification;
    },

    /**
     * Get icon for notification type
     */
    getNotificationIcon(type) {
        const icons = {
            success: 'fa-check-circle',
            error: 'fa-exclamation-circle',
            warning: 'fa-exclamation-triangle',
            info: 'fa-info-circle'
        };
        return icons[type] || icons.info;
    },

    /**
     * Show loading overlay
     */
    showLoading(text = 'Loading...') {
        let overlay = document.getElementById('loadingOverlay');
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'loadingOverlay';
            overlay.className = 'loading-overlay';
            overlay.innerHTML = `
                <div class="loading-content">
                    <div class="loading-spinner-large"></div>
                    <div class="loading-text">${text}</div>
                </div>
            `;
            document.body.appendChild(overlay);
        }
        overlay.classList.add('show');
    },

    /**
     * Hide loading overlay
     */
    hideLoading() {
        const overlay = document.getElementById('loadingOverlay');
        if (overlay) {
            overlay.classList.remove('show');
        }
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
        if (ms < 1000) return ms.toFixed(1) + 'ms';
        return (ms / 1000).toFixed(2) + 's';
    },

    /**
     * Format duration (milliseconds) to human readable
     */
    formatDuration(ms) {
        if (!ms || ms === 0) return '0ms';
        if (ms < 1) return ms.toFixed(3) + 'ms';
        if (ms < 1000) return ms.toFixed(1) + 'ms';
        if (ms < 60000) return (ms / 1000).toFixed(2) + 's';
        return (ms / 60000).toFixed(2) + 'm';
    },

    /**
     * Format percentage
     */
    formatPercentage(value, total, decimals = 1) {
        if (total === 0) return '0%';
        return ((value / total) * 100).toFixed(decimals) + '%';
    },

    /**
     * Format date/time
     */
    formatDateTime: function(dateStr, options = {}) {
        if (!dateStr) return '-';
        try {
            const date = new Date(dateStr);
            const defaultOptions = {
                year: 'numeric',
                month: 'short',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                hour12: false,
                timeZone: window.LOGLYNX_CONFIG?.timeZone || 'UTC'
            };
            return new Intl.DateTimeFormat(undefined, { ...defaultOptions, ...options }).format(date);
        } catch (e) {
            return dateStr;
        }
    },

    /**
     * Format date only
     */
    formatDate: function(dateStr, options = {}) {
        if (!dateStr) return '-';
        try {
            const date = new Date(dateStr);
            const defaultOptions = {
                year: 'numeric',
                month: 'short',
                day: '2-digit',
                timeZone: window.LOGLYNX_CONFIG?.timeZone || 'UTC'
            };
            return new Intl.DateTimeFormat(undefined, { ...defaultOptions, ...options }).format(date);
        } catch (e) {
            return dateStr;
        }
    },

    /**
     * Format time only
     */
    formatTime: function(dateStr, options = {}) {
        if (!dateStr) return '-';
        try {
            const date = new Date(dateStr);
            const defaultOptions = {
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                hour12: false,
                timeZone: window.LOGLYNX_CONFIG?.timeZone || 'UTC'
            };
            return new Intl.DateTimeFormat(undefined, { ...defaultOptions, ...options }).format(date);
        } catch (e) {
            return dateStr;
        }
    },

    /**
     * Format relative time (e.g., "2 minutes ago")
     */
    formatRelativeTime(dateString) {
        const date = new Date(dateString);
        const now = new Date();
        const diff = now - date;
        const seconds = Math.floor(diff / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (days > 0) return `${days} day${days > 1 ? 's' : ''} ago`;
        if (hours > 0) return `${hours} hour${hours > 1 ? 's' : ''} ago`;
        if (minutes > 0) return `${minutes} minute${minutes > 1 ? 's' : ''} ago`;
        return `${seconds} second${seconds > 1 ? 's' : ''} ago`;
    },

    /**
     * Get status badge HTML
     */
    getStatusBadge(statusCode) {
        let badgeClass = 'badge-success';
        if (statusCode >= 400 && statusCode < 500) badgeClass = 'badge-warning';
        if (statusCode >= 500) badgeClass = 'badge-danger';
        if (statusCode >= 300 && statusCode < 400) badgeClass = 'badge-info';

        return `<span class="badge ${badgeClass}">${statusCode}</span>`;
    },

    /**
     * Get method badge HTML
     */
    getMethodBadge(method) {
        const colors = {
            'GET': 'badge-primary',
            'POST': 'badge-success',
            'PUT': 'badge-warning',
            'DELETE': 'badge-danger',
            'PATCH': 'badge-info'
        };
        const badgeClass = colors[method] || 'badge-secondary';
        return `<span class="badge ${badgeClass}">${method}</span>`;
    },

    /**
     * Truncate string
     */
    truncate(str, maxLength = 50) {
        if (!str) return '';
        if (str.length <= maxLength) return str;
        return str.substring(0, maxLength) + '...';
    },

    /**
     * Debounce function
     */
    debounce(func, wait = 250) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    },

    /**
     * Throttle function
     */
    throttle(func, limit = 1000) {
        let inThrottle;
        return function(...args) {
            if (!inThrottle) {
                func.apply(this, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    },

    /**
     * Deep clone object
     */
    deepClone(obj) {
        return JSON.parse(JSON.stringify(obj));
    },

    /**
     * Get query parameter from URL
     */
    getQueryParam(param) {
        const urlParams = new URLSearchParams(window.location.search);
        return urlParams.get(param);
    },

    /**
     * Set query parameter in URL
     */
    setQueryParam(param, value) {
        const url = new URL(window.location);
        url.searchParams.set(param, value);
        window.history.pushState({}, '', url);
    },

    /**
     * Set active navigation item
     */
    setActiveNavItem(pageName) {
        document.querySelectorAll('.nav-item').forEach(item => {
            item.classList.remove('active');
            if (item.getAttribute('data-page') === pageName) {
                item.classList.add('active');
            }
        });
    },

    /**
     * Toggle sidebar (mobile)
     */
    toggleSidebar() {
        const sidebar = document.querySelector('.sidebar');
        const overlay = document.querySelector('.sidebar-overlay');

        if (sidebar && overlay) {
            sidebar.classList.toggle('open');
            overlay.classList.toggle('show');
        }
    },

    /**
     * Initialize sidebar events
     */
    initSidebar() {
        const toggle = document.querySelector('.sidebar-toggle');
        const overlay = document.querySelector('.sidebar-overlay');
        const navItems = document.querySelectorAll('.nav-item');

        if (toggle) {
            toggle.addEventListener('click', () => this.toggleSidebar());
        }

        if (overlay) {
            overlay.addEventListener('click', () => this.toggleSidebar());
        }

        // Close sidebar on navigation (mobile)
        navItems.forEach(item => {
            item.addEventListener('click', () => {
                if (window.innerWidth <= 1024) {
                    this.toggleSidebar();
                }
            });
        });
    },

    /**
     * Initialize service filter with multi-select
     */
    initServiceFilter(onChangeCallback) {
        const toggleBtn = document.getElementById('serviceFilterToggle');
        const dropdown = document.getElementById('serviceDropdownMenu');
        const filterTypeSelect = document.getElementById('filterType');
        const searchInput = document.getElementById('serviceSearchInput');
        const clearBtn = document.getElementById('clearServiceSelection');
        const serviceOptionsContainer = document.getElementById('serviceOptions');

        // Only require filterTypeSelect - dropdown elements are optional (removed for modal approach)
        if (!filterTypeSelect) return;

        // Restore from sessionStorage
        const savedServices = sessionStorage.getItem('selectedServices');
        const savedType = sessionStorage.getItem('selectedServiceType') || 'auto';

        if (savedServices) {
            try {
                const services = JSON.parse(savedServices);
                LogLynxAPI.setServiceFilters(services);
            } catch (e) {
                console.error('Failed to parse saved services:', e);
            }
        }
        filterTypeSelect.value = savedType;

        // Old dropdown functionality (only if elements exist)
        if (toggleBtn && dropdown) {
            // Toggle dropdown visibility
            toggleBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                dropdown.classList.toggle('show');
                toggleBtn.classList.toggle('open');
            });

            // Remove old document click handler to prevent accumulation
            if (this._serviceFilterDocClickHandler) {
                document.removeEventListener('click', this._serviceFilterDocClickHandler);
            }

            // Create new handler and store reference
            this._serviceFilterDocClickHandler = (e) => {
                if (!toggleBtn.contains(e.target) && !dropdown.contains(e.target)) {
                    dropdown.classList.remove('show');
                    toggleBtn.classList.remove('open');
                }
            };

            // Add new handler
            document.addEventListener('click', this._serviceFilterDocClickHandler);
        }

        // Handle filter type changes
        filterTypeSelect.addEventListener('change', () => {
            const newType = filterTypeSelect.value;

            // Remove selected services that don't match the new type
            const currentServices = LogLynxAPI.getServiceFilters();
            let validServices = currentServices;

            if (newType !== 'auto') {
                validServices = currentServices.filter(s => s.type === newType);
            }

            // Update selection with only valid services
            LogLynxAPI.setServiceFilters(validServices);
            if (validServices.length === 0) {
                sessionStorage.removeItem('selectedServices');
            } else {
                sessionStorage.setItem('selectedServices', JSON.stringify(validServices));
            }

            // Save the selected type
            sessionStorage.setItem('selectedServiceType', newType);

            // Reload service list and update label
            this.loadServiceFilter();
            this.updateServiceFilterLabel();

            // Trigger callback if selection changed
            if (this._serviceFilterCallback) {
                this._serviceFilterCallback();
            }
        });

        // Handle search input
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                const searchTerm = e.target.value.toLowerCase();
                if (serviceOptionsContainer) {
                    serviceOptionsContainer.querySelectorAll('.service-option-modal').forEach(option => {
                        const text = option.textContent.toLowerCase();
                        option.style.display = text.includes(searchTerm) ? 'flex' : 'none';
                    });
                }
            });
        }

        // Handle clear button
        if (clearBtn) {
            clearBtn.addEventListener('click', () => {
                if (serviceOptionsContainer) {
                    serviceOptionsContainer.querySelectorAll('input[type="checkbox"]').forEach(cb => {
                        cb.checked = false;
                    });
                }
                LogLynxAPI.setServiceFilters([]);
                sessionStorage.removeItem('selectedServices');
                this.updateServiceFilterLabel();

                // Update count display
                const countDisplay = document.getElementById('selectedServiceCount');
                if (countDisplay) {
                    countDisplay.textContent = 'No services selected';
                }

                if (onChangeCallback) onChangeCallback();
            });
        }

        // Store callback for later use
        this._serviceFilterCallback = onChangeCallback;

        // Initial load
        this.loadServiceFilter();
        this.updateServiceFilterLabel();

        // Reload services when modal is shown
        const filterModal = document.getElementById('filterSettingsModal');
        if (filterModal) {
            filterModal.addEventListener('shown.bs.modal', () => {
                this.loadServiceFilter();
            });
        }
    },

    /**
     * Load services into filter with type information
     */
    async loadServiceFilter() {
        const optionsContainer = document.getElementById('serviceOptions');
        const filterTypeSelect = document.getElementById('filterType');
        if (!optionsContainer) return;

        const result = await LogLynxAPI.getServices();
        if (result.success && result.data) {
            // Get current filter type
            const currentType = filterTypeSelect ? filterTypeSelect.value : 'auto';

            // Clear existing options
            optionsContainer.innerHTML = '';

            // Filter services based on selected type
            let services = result.data;
            if (currentType !== 'auto') {
                services = services.filter(s => s.type === currentType);
            }

            // Get currently selected services, but only those matching the current type
            const currentServices = LogLynxAPI.getServiceFilters();
            const validCurrentServices = currentType === 'auto'
                ? currentServices
                : currentServices.filter(s => s.type === currentType);
            const currentServiceNames = validCurrentServices.map(s => s.name);

            // Add service options with checkboxes
            services.forEach(service => {
                const label = document.createElement('label');
                label.className = 'service-option-modal';

                const checkbox = document.createElement('input');
                checkbox.type = 'checkbox';
                checkbox.value = service.name;
                checkbox.setAttribute('data-type', service.type);

                // Check if this service is currently selected
                if (currentServiceNames.includes(service.name)) {
                    checkbox.checked = true;
                }

                // Create a row object for formatHostDisplay based on service type
                let rowObj = {};
                if (service.type === 'backend_name') {
                    rowObj.backend_name = service.name;
                } else if (service.type === 'backend_url') {
                    rowObj.backend_url = service.name;
                } else if (service.type === 'host') {
                    rowObj.host = service.name;
                } else {
                    rowObj.backend_name = service.name;
                }

                // Format display using formatHostDisplay
                const displayName = this.formatHostDisplay(rowObj, service.name);
                const typeLabel = this.formatServiceType(service.type);
                const displayText = `${displayName} (${typeLabel}) - ${this.formatNumber(service.count)}`;

                const span = document.createElement('span');
                span.textContent = displayText;
                span.title = service.name; // Tooltip with full name

                label.appendChild(checkbox);
                label.appendChild(span);
                optionsContainer.appendChild(label);

                // Handle checkbox changes
                checkbox.addEventListener('change', () => {
                    this.handleServiceCheckboxChange();
                });
            });

            // Update count display after loading
            const countDisplay = document.getElementById('selectedServiceCount');
            if (countDisplay) {
                const count = validCurrentServices.length;
                countDisplay.textContent = count === 0 ? 'No services selected' :
                                          count === 1 ? '1 service selected' :
                                          `${count} services selected`;
            }
        }
    },

    /**
     * Handle service checkbox change (with debouncing)
     */
    handleServiceCheckboxChange() {
        const serviceOptionsContainer = document.getElementById('serviceOptions');
        if (!serviceOptionsContainer) return;

        const checkboxes = serviceOptionsContainer.querySelectorAll('input[type="checkbox"]');

        // Get all checked services (excluding empty values)
        const selectedServices = [];
        checkboxes.forEach(cb => {
            if (cb.checked && cb.value !== '') {
                selectedServices.push({
                    name: cb.value,
                    type: cb.getAttribute('data-type')
                });
            }
        });

        // Filter out any invalid services (empty names or "all" type)
        const validServices = selectedServices.filter(s => s.name && s.name !== '' && s.type !== 'all');

        // Update API filters (empty array means "All Traffic")
        if (validServices.length === 0) {
            LogLynxAPI.setServiceFilters([]);
            sessionStorage.removeItem('selectedServices');
        } else {
            LogLynxAPI.setServiceFilters(validServices);
            sessionStorage.setItem('selectedServices', JSON.stringify(validServices));
        }

        // Update label immediately for responsive UI
        this.updateServiceFilterLabel();

        // Update service count display
        const countDisplay = document.getElementById('selectedServiceCount');
        if (countDisplay) {
            const count = validServices.length;
            countDisplay.textContent = count === 0 ? 'No services selected' :
                                      count === 1 ? '1 service selected' :
                                      `${count} services selected`;
        }

        // Debounce the callback to prevent rapid API calls
        if (this._filterChangeDebounceTimer) {
            clearTimeout(this._filterChangeDebounceTimer);
        }

        this._filterChangeDebounceTimer = setTimeout(() => {
            if (this._serviceFilterCallback) {
                this._serviceFilterCallback();
            }
        }, 300); // Wait 300ms after last change before triggering callback
    },

    /**
     * Update the service filter button label
     */
    updateServiceFilterLabel() {
        const label = document.getElementById('serviceFilterLabel');
        if (!label) return;

        const selectedServices = LogLynxAPI.getServiceFilters();

        if (selectedServices.length === 0) {
            label.textContent = 'All Traffic';
        } else if (selectedServices.length === 1) {
            // Create row object for display
            const service = selectedServices[0];
            let rowObj = {};
            if (service.type === 'backend_name') {
                rowObj.backend_name = service.name;
            } else if (service.type === 'backend_url') {
                rowObj.backend_url = service.name;
            } else if (service.type === 'host') {
                rowObj.host = service.name;
            }
            label.textContent = this.formatHostDisplay(rowObj, service.name);
        } else {
            label.textContent = `${selectedServices.length} Services Selected`;
        }

        // Update modal trigger button status
        this.updateFilterButtonStatus();
    },

    /**
     * Update filter trigger button status badge
     */
    updateFilterButtonStatus() {
        const filterStatus = document.getElementById('filterStatus');
        if (!filterStatus) return;

        const selectedServices = LogLynxAPI.getServiceFilters();

        if (selectedServices.length === 0) {
            filterStatus.textContent = 'All Traffic';
        } else if (selectedServices.length === 1) {
            // Show service name (shortened if too long)
            const service = selectedServices[0];
            let rowObj = {};
            if (service.type === 'backend_name') {
                rowObj.backend_name = service.name;
            } else if (service.type === 'backend_url') {
                rowObj.backend_url = service.name;
            } else if (service.type === 'host') {
                rowObj.host = service.name;
            }
            const displayName = this.formatHostDisplay(rowObj, service.name);
            // Truncate if too long
            filterStatus.textContent = displayName.length > 20 ? displayName.substring(0, 20) + '...' : displayName;
        } else {
            filterStatus.textContent = `${selectedServices.length} Services`;
        }
    },

    /**
     * Update hide traffic trigger button status badge
     */
    updateHideTrafficButtonStatus() {
        const hideTrafficBtn = document.getElementById('hideTrafficBtn');
        const hideTrafficStatus = document.getElementById('hideTrafficStatus');
        if (!hideTrafficBtn || !hideTrafficStatus) return;

        const hideMyTrafficCheckbox = document.getElementById('hideMyTrafficCheckbox');
        const isEnabled = hideMyTrafficCheckbox && hideMyTrafficCheckbox.checked;

        if (isEnabled) {
            hideTrafficBtn.classList.add('active');

            // Show number of services if any are selected
            const selectedServices = LogLynxAPI.getHideTrafficFilters();
            if (selectedServices.length === 0) {
                hideTrafficStatus.textContent = 'Enabled (All)';
            } else if (selectedServices.length === 1) {
                hideTrafficStatus.textContent = 'Enabled (1 service)';
            } else {
                hideTrafficStatus.textContent = `Enabled (${selectedServices.length} services)`;
            }
        } else {
            hideTrafficBtn.classList.remove('active');
            hideTrafficStatus.textContent = 'Disabled';
        }
    },

    /**
     * Format service type for display
     */
    formatServiceType(type) {
        const typeMap = {
            'backend_name': 'Backend',
            'backend_url': 'URL',
            'host': 'Host',
            'auto': 'Auto'
        };
        return typeMap[type] || type;
    },

    /**
     * Initialize refresh controls with RefreshManager integration
     */
    initRefreshControls(loadDataCallback, defaultInterval = 30) {
        // Initialize RefreshManager for this page
    
        //check if RefreshManager is defined (before calling init)
        if (typeof RefreshManager === 'undefined') {
            document.querySelector('.header-refresh-controls').style.display = 'none';
        }else{
            RefreshManager.init();
        }

        // Load settings from RefreshManager
        const settings = RefreshManager.getCurrentSettings();

        let refreshTimer = null;
        let lastRefreshTimer = null;
        let isAutoRefreshEnabled = false;
        let refreshInterval = settings.interval * 1000; // Use saved interval
        let lastRefreshTime = null;

        const intervalSelect = document.getElementById('refreshInterval');
        const playBtn = document.getElementById('playRefresh');
        const pauseBtn = document.getElementById('pauseRefresh');
        const pinBtn = document.getElementById('pinPageRefresh');
        const unpinBtn = document.getElementById('unpinPageRefresh');
        const statusSpan = document.getElementById('refreshStatus');
        const statusTextSpan = document.getElementById('refreshStatusText');
        const modeBadge = document.getElementById('refreshModeBadge');
        const customInput = document.getElementById('refreshCustomInput');
        const customValueInput = document.getElementById('customRefreshValue');
        const applyCustomBtn = document.getElementById('applyCustomRefresh');

        // Compact button elements
        const refreshIcon = document.getElementById('refreshIcon');
        const compactStatus = document.getElementById('refreshCompactStatus');
        const compactBadge = document.getElementById('refreshCompactBadge');
        const compactLast = document.getElementById('refreshCompactLast');
        const modeAlert = document.getElementById('refreshModeAlert');
        const modeText = document.getElementById('refreshModeText');

        const updateLastRefreshDisplay = () => {
            if (!lastRefreshTime) return;

            const now = Date.now();
            const secondsAgo = Math.floor((now - lastRefreshTime) / 1000);

            let timeText;
            if (secondsAgo < 60) {
                timeText = `${secondsAgo}s ago`;
            } else if (secondsAgo < 3600) {
                const minutes = Math.floor(secondsAgo / 60);
                const seconds = secondsAgo % 60;
                timeText = `${minutes}m ${seconds}s ago`;
            } else {
                const hours = Math.floor(secondsAgo / 3600);
                const minutes = Math.floor((secondsAgo % 3600) / 60);
                timeText = `${hours}h ${minutes}m ago`;
            }

            // Update modal status
            if (statusSpan) {
                const lastRefreshElement = statusSpan.querySelector('.last-refresh');
                if (lastRefreshElement) {
                    lastRefreshElement.textContent = `Last: ${timeText}`;
                }
            }

            // Update compact button
            if (compactLast) {
                compactLast.textContent = `Last: ${timeText}`;
            }
        };

        const updateStatus = () => {
            let intervalText;
            if (intervalSelect) {
                const selectedValue = intervalSelect.value;
                if (selectedValue === 'custom' && customValueInput && customValueInput.value) {
                    const customSeconds = parseInt(customValueInput.value);
                    intervalText = customSeconds < 60 ? `${customSeconds}s` : `${Math.floor(customSeconds / 60)}m`;
                } else if (selectedValue.startsWith('custom-')) {
                    // Custom value already applied
                    intervalText = intervalSelect.options[intervalSelect.selectedIndex].text;
                } else {
                    intervalText = intervalSelect.options[intervalSelect.selectedIndex].text;
                }
            } else {
                intervalText = '30s';
            }

            // Update modal status
            if (statusSpan) {
                const icon = isAutoRefreshEnabled ?
                    '<i class="fas fa-sync-alt fa-spin"></i>' :
                    '<i class="fas fa-pause"></i>';
                const text = isAutoRefreshEnabled ?
                    `Auto-refresh: ${intervalText}` :
                    `Paused: ${intervalText}`;

                const lastRefreshText = lastRefreshTime ?
                    `<span class="last-refresh" style="margin-left: 10px; color: #999; font-size: 0.9em;">Last: ${Math.floor((Date.now() - lastRefreshTime) / 1000)}s ago</span>` :
                    '';

                statusSpan.innerHTML = `${icon} <span>${text}</span>${lastRefreshText}`;
            }

            if (statusTextSpan) {
                statusTextSpan.textContent = isAutoRefreshEnabled ? `Auto-refresh: ${intervalText}` : `Paused: ${intervalText}`;
            }

            // Update compact button
            if (compactStatus) {
                compactStatus.textContent = intervalText;
            }

            if (refreshIcon) {
                if (isAutoRefreshEnabled) {
                    refreshIcon.classList.add('fa-spin');
                } else {
                    refreshIcon.classList.remove('fa-spin');
                }
            }
        };

        const updateButtons = () => {
            if (playBtn) {
                playBtn.disabled = isAutoRefreshEnabled;
                if (isAutoRefreshEnabled) {
                    playBtn.classList.add('active');
                } else {
                    playBtn.classList.remove('active');
                }
            }
            if (pauseBtn) {
                pauseBtn.disabled = !isAutoRefreshEnabled;
                if (!isAutoRefreshEnabled) {
                    pauseBtn.classList.add('active');
                } else {
                    pauseBtn.classList.remove('active');
                }
            }
        };

        const updateModeBadge = () => {
            const settings = RefreshManager.getCurrentSettings();
            const isPageSpecific = settings.isPageSpecific;

            // Update modal badge
            if (modeBadge) {
                if (isPageSpecific) {
                    modeBadge.innerHTML = '<i class="fas fa-thumbtack"></i> Page-Specific';
                    modeBadge.style.display = 'inline-block';
                    modeBadge.className = 'badge bg-warning text-dark';
                } else {
                    modeBadge.innerHTML = '<i class="fas fa-globe"></i> Global';
                    modeBadge.style.display = 'inline-block';
                    modeBadge.className = 'badge bg-secondary';
                }
            }

            // Update compact badge
            if (compactBadge) {
                if (isPageSpecific) {
                    compactBadge.textContent = 'PAGE';
                    compactBadge.style.display = 'inline-block';
                } else {
                    compactBadge.style.display = 'none';
                }
            }

            // Update modal alert
            if (modeAlert) {
                const iconEl = modeAlert.querySelector('.status-icon i');
                const titleEl = modeAlert.querySelector('.status-title');
                const subtitleEl = modeAlert.querySelector('.status-subtitle');

                if (isPageSpecific) {
                    modeAlert.style.background = 'rgba(255, 193, 7, 0.1)';
                    modeAlert.style.borderColor = 'rgba(255, 193, 7, 0.3)';
                    if (iconEl) {
                        iconEl.className = 'fas fa-thumbtack';
                        iconEl.style.color = '#ffc107';
                    }
                    if (titleEl) titleEl.textContent = 'Page-Specific Settings';
                    if (subtitleEl) subtitleEl.textContent = `Custom settings for ${settings.pageId || 'this page'} only`;
                } else {
                    modeAlert.style.background = 'rgba(13, 110, 253, 0.1)';
                    modeAlert.style.borderColor = 'rgba(13, 110, 253, 0.3)';
                    if (iconEl) {
                        iconEl.className = 'fas fa-globe';
                        iconEl.style.color = '#0d6efd';
                    }
                    if (titleEl) titleEl.textContent = 'Global Settings';
                    if (subtitleEl) subtitleEl.textContent = 'These settings apply to all pages';
                }
            }
        };

        const updatePinButtons = () => {
            const hasOverride = RefreshManager.hasPageOverride();
            if (pinBtn) pinBtn.style.display = hasOverride ? 'none' : 'inline-block';
            if (unpinBtn) unpinBtn.style.display = hasOverride ? 'inline-block' : 'none';

            // Show/hide explanations
            const pinExplanation = document.getElementById('pinExplanation');
            const unpinExplanation = document.getElementById('unpinExplanation');
            if (pinExplanation) pinExplanation.style.display = hasOverride ? 'none' : 'flex';
            if (unpinExplanation) unpinExplanation.style.display = hasOverride ? 'flex' : 'none';
        };

        const wrappedLoadCallback = async () => {
            await loadDataCallback();
            lastRefreshTime = Date.now();
            updateStatus();
        };

        const startRefresh = () => {
            if (isAutoRefreshEnabled) return;
            isAutoRefreshEnabled = true;
            refreshTimer = setInterval(wrappedLoadCallback, refreshInterval);
            // Update last refresh time display every second
            lastRefreshTimer = setInterval(updateLastRefreshDisplay, 1000);
            updateStatus();
            updateButtons();
        };

        const stopRefresh = () => {
            if (!isAutoRefreshEnabled) return;
            isAutoRefreshEnabled = false;
            if (refreshTimer) {
                clearInterval(refreshTimer);
                refreshTimer = null;
            }
            if (lastRefreshTimer) {
                clearInterval(lastRefreshTimer);
                lastRefreshTimer = null;
            }
            updateStatus();
            updateButtons();
        };

        // Interval change
        if (intervalSelect) {
            intervalSelect.addEventListener('change', (e) => {
                const selectedValue = e.target.value;
                
                // Check if it's a custom value (starts with "custom-")
                if (selectedValue === 'custom' || selectedValue.startsWith('custom-')) {
                    // Show custom input
                    if (customInput) {
                        customInput.style.display = 'flex';
                        if (customValueInput) {
                            // Pre-fill with existing custom value if available
                            if (selectedValue.startsWith('custom-')) {
                                const existingValue = selectedValue.split('-')[1];
                                customValueInput.value = existingValue;
                            }
                            customValueInput.focus();
                        }
                    }
                    // Don't change the refresh interval yet, wait for user to apply
                } else {
                    // Hide custom input and apply preset interval
                    if (customInput) {
                        customInput.style.display = 'none';
                    }
                    refreshInterval = parseInt(selectedValue) * 1000;

                    // Save to RefreshManager
                    RefreshManager.updateCurrent(parseInt(selectedValue), isAutoRefreshEnabled);

                    updateStatus();
                    updateModeBadge();
                    if (isAutoRefreshEnabled) {
                        stopRefresh();
                        wrappedLoadCallback(); // Immediate refresh on interval change
                        startRefresh();
                    }
                }
            });
        }

        // Apply custom interval
        if (applyCustomBtn && customValueInput && intervalSelect) {
            const applyCustomInterval = () => {
                const customValue = parseInt(customValueInput.value);
                if (customValue && customValue > 0 && customValue <= 3600) {
                    refreshInterval = customValue * 1000;
                    
                    // Remove all previous custom options (those starting with "custom-")
                    const existingCustomOptions = intervalSelect.querySelectorAll('option[value^="custom-"]');
                    existingCustomOptions.forEach(option => option.remove());
                    
                    // Create new option for this custom value (disabled, only for display)
                    const customId = `custom-${customValue}`;
                    const newOption = document.createElement('option');
                    newOption.value = customId;
                    newOption.textContent = `${customValue}s`;
                    newOption.disabled = true;  // Make it disabled (not selectable)
                    
                    // Insert before the "Custom..." option
                    const customOption = intervalSelect.querySelector('option[value="custom"]');
                    if (customOption) {
                        intervalSelect.insertBefore(newOption, customOption);
                    } else {
                        intervalSelect.appendChild(newOption);
                    }
                    
                    // Select the custom value option
                    intervalSelect.value = customId;
                    
                    // Hide the custom input after applying
                    if (customInput) {
                        customInput.style.display = 'none';
                    }

                    // Save to RefreshManager
                    RefreshManager.updateCurrent(customValue, isAutoRefreshEnabled);

                    updateStatus();
                    updateModeBadge();
                    if (isAutoRefreshEnabled) {
                        stopRefresh();
                        wrappedLoadCallback(); // Immediate refresh on interval change
                        startRefresh();
                    }
                } else {
                    alert('Please enter a value between 1 and 3600 seconds');
                }
            };

            applyCustomBtn.addEventListener('click', applyCustomInterval);
            
            // Also apply on Enter key
            customValueInput.addEventListener('keypress', (e) => {
                if (e.key === 'Enter') {
                    applyCustomInterval();
                }
            });
        }

        // Play/Pause buttons
        if (playBtn) {
            playBtn.addEventListener('click', () => {
                wrappedLoadCallback(); // Immediate refresh when starting
                startRefresh();
                // Save auto-start preference
                RefreshManager.updateCurrent(refreshInterval / 1000, true);
            });
        }

        if (pauseBtn) {
            pauseBtn.addEventListener('click', () => {
                stopRefresh();
                // Save auto-start preference
                RefreshManager.updateCurrent(refreshInterval / 1000, false);
            });
        }

        // Pin/Unpin buttons
        if (pinBtn) {
            pinBtn.addEventListener('click', () => {
                // Enable page-specific override with current settings
                const currentIntervalSeconds = refreshInterval / 1000;
                RefreshManager.enablePageOverride(currentIntervalSeconds, isAutoRefreshEnabled);
                updateModeBadge();
                updatePinButtons();
                console.log('[RefreshControls] Page override enabled');
            });
        }

        if (unpinBtn) {
            unpinBtn.addEventListener('click', () => {
                // Disable page override and reload global settings
                RefreshManager.disablePageOverride();
                const globalSettings = RefreshManager.getCurrentSettings();

                // Apply global settings
                refreshInterval = globalSettings.interval * 1000;

                // Update UI
                if (intervalSelect) {
                    // Find matching option or use closest
                    const options = Array.from(intervalSelect.options);
                    const matchingOption = options.find(opt => parseInt(opt.value) === globalSettings.interval);
                    if (matchingOption) {
                        intervalSelect.value = matchingOption.value;
                    }
                }

                // Restart with new interval if was running
                if (isAutoRefreshEnabled) {
                    stopRefresh();
                    startRefresh();
                }

                updateStatus();
                updateModeBadge();
                updatePinButtons();
                console.log('[RefreshControls] Page override disabled, using global settings');
            });
        }

        // Initialize Bootstrap tooltips when modal opens
        const refreshModal = document.getElementById('refreshSettingsModal');
        if (refreshModal) {
            refreshModal.addEventListener('shown.bs.modal', () => {
                // Initialize tooltips
                const tooltipTriggerList = refreshModal.querySelectorAll('[data-bs-toggle="tooltip"]');
                [...tooltipTriggerList].map(tooltipTriggerEl => new bootstrap.Tooltip(tooltipTriggerEl));
            });
        }

        // Initialize UI state
        updateStatus();
        updateButtons();
        updateModeBadge();
        updatePinButtons();

        // Set interval select to match current settings
        if (intervalSelect) {
            const currentIntervalSeconds = settings.interval;
            const options = Array.from(intervalSelect.options);
            const matchingOption = options.find(opt => parseInt(opt.value) === currentIntervalSeconds);
            if (matchingOption) {
                intervalSelect.value = matchingOption.value;
            }
        }

        // Start auto-refresh based on saved preference
        wrappedLoadCallback(); // Initial load
        lastRefreshTime = Date.now();

        if (settings.autoStart) {
            isAutoRefreshEnabled = true;
            refreshTimer = setInterval(wrappedLoadCallback, refreshInterval);
            lastRefreshTimer = setInterval(updateLastRefreshDisplay, 1000);
            updateStatus();
            updateButtons();
        }

        // Return control functions
        return {
            start: startRefresh,
            stop: stopRefresh,
            isRunning: () => isAutoRefreshEnabled
        };
    },

    /**
     * Create table from data
     */
    createTable(data, columns) {
        if (!data || data.length === 0) {
            return '<tr><td colspan="' + columns.length + '" class="text-center text-muted">No data available</td></tr>';
        }

        let html = '';
        data.forEach(row => {
            html += '<tr>';
            columns.forEach(col => {
                let value = row[col.field];

                // Apply formatter if provided
                if (col.formatter) {
                    value = col.formatter(value, row);
                }

                html += `<td>${value !== null && value !== undefined ? value : '-'}</td>`;
            });
            html += '</tr>';
        });

        return html;
    },

    /**
     * Export chart as image
     */
    exportChartAsImage(chartCanvas, filename = 'chart.png') {
        const url = chartCanvas.toDataURL('image/png');
        const link = document.createElement('a');
        link.download = filename;
        link.href = url;
        link.click();
    },

    /**
     * Export data as CSV
     */
    exportAsCSV(data, filename = 'export.csv') {
        if (!data || data.length === 0) return;

        const headers = Object.keys(data[0]);
        const csv = [
            headers.join(','),
            ...data.map(row =>
                headers.map(header =>
                    JSON.stringify(row[header] || '')
                ).join(',')
            )
        ].join('\n');

        const blob = new Blob([csv], { type: 'text/csv' });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.setAttribute('href', url);
        link.setAttribute('download', filename);
        link.click();
    },

    /**
     * Copy text to clipboard
     */
    copyToClipboard(text) {
        if (navigator.clipboard) {
            navigator.clipboard.writeText(text).then(() => {
                this.showNotification('Copied to clipboard', 'success', 2000);
            }).catch(err => {
                console.error('Failed to copy:', err);
                this.showNotification('Failed to copy', 'error', 2000);
            });
        } else {
            // Fallback for older browsers
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            try {
                document.execCommand('copy');
                this.showNotification('Copied to clipboard', 'success', 2000);
            } catch (err) {
                console.error('Failed to copy:', err);
                this.showNotification('Failed to copy', 'error', 2000);
            }
            document.body.removeChild(textarea);
        }
    },

    /**
     * Initialize tooltips (if using Bootstrap tooltips)
     */
    initTooltips() {
        const tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {
            return new bootstrap.Tooltip(tooltipTriggerEl);
        });
    },

    /**
     * Smooth scroll to element
     */
    scrollToElement(elementId) {
        const element = document.getElementById(elementId);
        if (element) {
            element.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
    },

    /**
     * Initialize Hide My Traffic filter
     */
    initHideMyTrafficFilter(onChangeCallback) {
        const checkbox = document.getElementById('hideMyTrafficCheckbox');
        const container = document.getElementById('hideTrafficServicesContainer');
        const toggleBtn = document.getElementById('hideTrafficToggle');
        const dropdown = document.getElementById('hideTrafficDropdownMenu');
        const searchInput = document.getElementById('hideTrafficSearchInput');
        const clearBtn = document.getElementById('clearHideTrafficSelection');

        if (!checkbox) return;

        // Restore from sessionStorage
        const hideEnabled = sessionStorage.getItem('hideMyTraffic') === 'true';
        const hideServices = sessionStorage.getItem('hideMyTrafficServices');

        checkbox.checked = hideEnabled;
        LogLynxAPI.setHideMyTraffic(hideEnabled); // Restore in API object

        if (hideEnabled && container) {
            container.style.display = 'block';
        }

        if (hideServices) {
            try {
                const services = JSON.parse(hideServices);
                LogLynxAPI.setHideTrafficFilters(services);
            } catch (e) {
                console.error('Failed to parse hide traffic services:', e);
            }
        }

        // Handle checkbox toggle
        if (checkbox) {
            checkbox.addEventListener('change', (e) => {
                const isEnabled = e.target.checked;
                sessionStorage.setItem('hideMyTraffic', isEnabled);
                LogLynxAPI.setHideMyTraffic(isEnabled);

                if (container) {
                    container.style.display = isEnabled ? 'block' : 'none';
                }

                // Update trigger button status
                this.updateHideTrafficButtonStatus();

                if (onChangeCallback) {
                    onChangeCallback();
                }
            });
        }

        // Initial button status update
        this.updateHideTrafficButtonStatus();

        // Toggle dropdown
        if (toggleBtn && dropdown) {
            toggleBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                dropdown.classList.toggle('show');
                toggleBtn.classList.toggle('open');
            });

            // Remove old document click handler to prevent accumulation
            if (this._hideTrafficDocClickHandler) {
                document.removeEventListener('click', this._hideTrafficDocClickHandler);
            }

            // Create new handler and store reference
            this._hideTrafficDocClickHandler = (e) => {
                if (!toggleBtn.contains(e.target) && !dropdown.contains(e.target)) {
                    dropdown.classList.remove('show');
                    toggleBtn.classList.remove('open');
                }
            };

            // Add new handler
            document.addEventListener('click', this._hideTrafficDocClickHandler);
        }

        // Handle search
        if (searchInput) {
            searchInput.addEventListener('input', (e) => {
                const searchTerm = e.target.value.toLowerCase();
                document.querySelectorAll('#hideTrafficOptions .service-option-modal').forEach(option => {
                    const text = option.textContent.toLowerCase();
                    option.style.display = text.includes(searchTerm) ? 'flex' : 'none';
                });
            });
        }

        // Handle clear button
        if (clearBtn) {
            clearBtn.addEventListener('click', () => {
                document.querySelectorAll('#hideTrafficOptions input[type="checkbox"]').forEach(cb => {
                    cb.checked = false;
                });
                LogLynxAPI.setHideTrafficFilters([]);
                sessionStorage.removeItem('hideMyTrafficServices');
                this.updateHideTrafficLabel();

                // Update count display
                const countDisplay = document.getElementById('selectedHideTrafficCount');
                if (countDisplay) {
                    countDisplay.textContent = 'No services selected';
                }

                // Update button status
                this.updateHideTrafficButtonStatus();

                if (onChangeCallback) onChangeCallback();
            });
        }

        // Store callback
        this._hideTrafficCallback = onChangeCallback;

        // Initial load
        this.loadHideTrafficServices();
        this.updateHideTrafficLabel();

        // Reload services when modal is shown
        const hideTrafficModal = document.getElementById('hideTrafficModal');
        if (hideTrafficModal) {
            hideTrafficModal.addEventListener('shown.bs.modal', () => {
                this.loadHideTrafficServices();
            });
        }
    },

    /**
     * Load services into hide traffic filter
     */
    async loadHideTrafficServices() {
        const optionsContainer = document.getElementById('hideTrafficOptions');
        if (!optionsContainer) return;

        const result = await LogLynxAPI.getServices();
        if (result.success && result.data) {
            // Clear existing options
            optionsContainer.innerHTML = '';

            const currentServices = LogLynxAPI.getHideTrafficFilters();
            const currentServiceNames = currentServices.map(s => s.name);

            // Add all services (no filtering by type)
            result.data.forEach(service => {
                const label = document.createElement('label');
                label.className = 'service-option-modal';

                const checkbox = document.createElement('input');
                checkbox.type = 'checkbox';
                checkbox.value = service.name;
                checkbox.setAttribute('data-type', service.type);

                if (currentServiceNames.includes(service.name)) {
                    checkbox.checked = true;
                }

                let rowObj = {};
                if (service.type === 'backend_name') {
                    rowObj.backend_name = service.name;
                } else if (service.type === 'backend_url') {
                    rowObj.backend_url = service.name;
                } else if (service.type === 'host') {
                    rowObj.host = service.name;
                }

                const displayName = this.formatHostDisplay(rowObj, service.name);
                const typeLabel = this.formatServiceType(service.type);
                const displayText = `${displayName} (${typeLabel}) - ${this.formatNumber(service.count)}`;

                const span = document.createElement('span');
                span.textContent = displayText;
                span.title = service.name;

                label.appendChild(checkbox);
                label.appendChild(span);
                optionsContainer.appendChild(label);

                checkbox.addEventListener('change', () => {
                    this.handleHideTrafficCheckboxChange();
                });
            });

            // Update count display after loading
            const countDisplay = document.getElementById('selectedHideTrafficCount');
            if (countDisplay) {
                const count = currentServices.length;
                countDisplay.textContent = count === 0 ? 'No services selected' :
                                          count === 1 ? '1 service selected' :
                                          `${count} services selected`;
            }
        }
    },

    /**
     * Handle hide traffic checkbox change (with debouncing)
     */
    handleHideTrafficCheckboxChange() {
        const checkboxes = document.querySelectorAll('#hideTrafficOptions input[type="checkbox"]');

        const selectedServices = [];
        checkboxes.forEach(cb => {
            if (cb.checked) {
                selectedServices.push({
                    name: cb.value,
                    type: cb.getAttribute('data-type')
                });
            }
        });

        // Update API filters (empty array means "All Services")
        if (selectedServices.length === 0) {
            LogLynxAPI.setHideTrafficFilters([]);
            sessionStorage.removeItem('hideMyTrafficServices');
        } else {
            LogLynxAPI.setHideTrafficFilters(selectedServices);
            sessionStorage.setItem('hideMyTrafficServices', JSON.stringify(selectedServices));
        }

        // Update label immediately for responsive UI
        this.updateHideTrafficLabel();

        // Update service count display
        const countDisplay = document.getElementById('selectedHideTrafficCount');
        if (countDisplay) {
            const count = selectedServices.length;
            countDisplay.textContent = count === 0 ? 'No services selected' :
                                      count === 1 ? '1 service selected' :
                                      `${count} services selected`;
        }

        // Update button status to reflect service count
        this.updateHideTrafficButtonStatus();

        // Debounce the callback to prevent rapid API calls
        if (this._hideTrafficChangeDebounceTimer) {
            clearTimeout(this._hideTrafficChangeDebounceTimer);
        }

        this._hideTrafficChangeDebounceTimer = setTimeout(() => {
            if (this._hideTrafficCallback) {
                this._hideTrafficCallback();
            }
        }, 300); // Wait 300ms after last change before triggering callback
    },

    /**
     * Update hide traffic label
     */
    updateHideTrafficLabel() {
        const label = document.getElementById('hideTrafficLabel');
        if (!label) return;

        const selectedServices = LogLynxAPI.getHideTrafficFilters();

        if (selectedServices.length === 0) {
            label.textContent = 'All Services';
        } else if (selectedServices.length === 1) {
            const service = selectedServices[0];
            let rowObj = {};
            if (service.type === 'backend_name') {
                rowObj.backend_name = service.name;
            } else if (service.type === 'backend_url') {
                rowObj.backend_url = service.name;
            } else if (service.type === 'host') {
                rowObj.host = service.name;
            }
            label.textContent = this.formatHostDisplay(rowObj, service.name);
        } else {
            label.textContent = `${selectedServices.length} Services`;
        }
    },

    extractBackendName(backendName) {
        if (!backendName || backendName === '') {
            return '';
        }

        // Remove protocol suffix (e.g., @file, @docker, @http)
        let name = backendName.split('@')[0];

        // Remove -service suffix if present
        name = name.replace(/-service$/, '');

        // Split by dash
        const parts = name.split('-');

        // If first part is a number (ID), skip it
        let startIndex = 0;
        if (parts.length > 1 && /^\d+$/.test(parts[0])) {
            startIndex = 1;
        }

        // Join remaining parts with spaces
        const result = parts.slice(startIndex).join(' ');

        return result || backendName; // Fallback to original if empty
    },

    /**
     * Format host/backend display with intelligent fallbacks
     * Priority: Host → BackendName (formatted) → BackendURL (hostname) → fallback
     * Automatically appends :port if host contains a port number
     */
    formatHostDisplay(row, fallback = '-') {
        // Get display preference from sessionStorage (default: auto)
        const displayPreference = sessionStorage.getItem('displayPreference') || 'auto';

        const backendName = row.BackendName || row.backend_name;
        const backendURL = row.BackendURL || row.backend_url;
        const host = row.Host || row.host;

        let displayValue = '';

        // If specific preference is set, try to use only that field
        if (displayPreference !== 'auto') {
            if (displayPreference === 'backend_name' && backendName && backendName !== '') {
                displayValue = this.extractBackendName(backendName);
            } else if (displayPreference === 'backend_url' && backendURL && backendURL !== '') {
                try {
                    const url = new URL(backendURL);
                    displayValue = url.hostname || backendURL;
                } catch (e) {
                    displayValue = this.extractBackendName(backendURL);
                }
            } else if (displayPreference === 'host' && host && host !== '') {
                displayValue = host; // Keep original host (may include port)
            }
            // If preferred field is not available, fall through to auto mode
        }

        // Auto mode: Priority 1: BackendName → 2: BackendURL → 3: Host
        if (!displayValue) {
            if (backendName && backendName !== '') {
                displayValue = this.extractBackendName(backendName);
            } else if (backendURL && backendURL !== '') {
                try {
                    const url = new URL(backendURL);
                    displayValue = url.hostname || backendURL;
                } catch (e) {
                    displayValue = this.extractBackendName(backendURL);
                }
            } else if (host && host !== '') {
                displayValue = host; // Keep original host (may include port)
            }
        }

        // If we still don't have a value, return fallback
        if (!displayValue) {
            return fallback;
        }

        // Check if the original host field contains a port (format: hostname:port)
        // If host contains :port and our display value doesn't, add it
        if (host && host.includes(':')) {
            const hostParts = host.split(':');
            // Verify the last part is a valid port number
            const portPart = hostParts[hostParts.length - 1];
            if (/^\d+$/.test(portPart)) {
                const port = parseInt(portPart);
                // Add port if it's not a standard port (80/443) and not already in display value
                if (port !== 80 && port !== 443 && !displayValue.includes(':')) {
                    // Extract hostname from display value (in case it has @ or other suffix)
                    displayValue = displayValue + ':' + port;
                }
            }
        }

        return displayValue;
    },

    showEmptyState(containerId, type = 'chart', message = null, options = {}) {
        const container = document.getElementById(containerId);
        if (!container) {
            console.warn(`Container with ID '${containerId}' not found`);
            return;
        }

        // Default messages based on type
        const defaultMessages = {
            chart: 'No chart data available',
            table: 'No table data available',
            datatable: 'No data available in table',
            map: 'No map data available',
            heatmap: 'No heatmap data available'
        };

        // Default icons based on type
        const defaultIcons = {
            chart: 'fas fa-chart-line',
            table: 'fas fa-table',
            datatable: 'fas fa-table',
            map: 'fas fa-map-marked-alt',
            heatmap: 'fas fa-th'
        };

        // Use provided message or default
        const displayMessage = message || defaultMessages[type] || 'No data available';
        const displayIcon = options.icon || defaultIcons[type] || 'fas fa-exclamation-circle';

        // For DataTables, we need to handle it differently
        if (type === 'datatable' && $.fn.DataTable.isDataTable(container)) {
            // Update DataTable's language option to show custom message
            const dataTable = $(container).DataTable();
            
            // Store original empty table message if not already stored
            if (!container._originalEmptyTable) {
                container._originalEmptyTable = dataTable.settings()[0].oLanguage.sEmptyTable || 'No data available';
            }
            
            // Create custom empty message HTML
            const customEmptyHtml = `
                <div class="datatable-empty-state">
                    <div class="datatable-empty-content">
                        <i class="${displayIcon} datatable-empty-icon"></i>
                        <div class="datatable-empty-message">${displayMessage}</div>
                        ${options.subtitle ? `<div class="datatable-empty-subtitle">${options.subtitle}</div>` : ''}
                    </div>
                </div>
            `;
            
            // Update the DataTable's empty table message
            dataTable.settings()[0].oLanguage.sEmptyTable = customEmptyHtml;
            
            // If table is already empty, redraw to show the new message
            if (dataTable.data().length === 0) {
                dataTable.draw();
            }
            return;
        }

        // For regular containers (charts, tables, etc.)
        // Clear existing content
        container.innerHTML = '';

        // Create empty state element with consistent styling
        const emptyState = document.createElement('div');
        emptyState.className = 'empty-state-container';
        
        // Use existing chart-empty CSS for consistency
        if (type === 'chart' || type === 'heatmap') {
            emptyState.className = 'chart-empty';
        }
        
        emptyState.innerHTML = `
            <div class="empty-state-content">
                <i class="${displayIcon} empty-state-icon"></i>
                <div class="empty-state-message">${displayMessage}</div>
                ${options.subtitle ? `<div class="empty-state-subtitle">${options.subtitle}</div>` : ''}
                ${options.action ? `<button class="btn btn-primary empty-state-action" onclick="${options.action}">${options.actionText || 'Retry'}</button>` : ''}
            </div>
        `;
        
        container.appendChild(emptyState);
        
        // Add custom styles if provided
        if (options.customClass) {
            emptyState.classList.add(options.customClass);
        }
    },

    /**
     * Clear empty state from a container
     * @param {string} containerId - The ID of the container element
     * @param {string} type - The type of container: 'chart', 'table', 'datatable'
     */
    clearEmptyState(containerId, type = 'chart') {
        const container = document.getElementById(containerId);
        if (!container) return;

        // For DataTables, restore original empty message
        if (type === 'datatable' && $.fn.DataTable.isDataTable(container) && container._originalEmptyTable) {
            const dataTable = $(container).DataTable();
            dataTable.settings()[0].oLanguage.sEmptyTable = container._originalEmptyTable;
            if (dataTable.data().length === 0) {
                dataTable.draw();
            }
            return;
        }

        // For regular containers, remove empty state elements
        const emptyElements = container.querySelectorAll('.empty-state-container, .chart-empty');
        emptyElements.forEach(el => el.remove());
    },

    /**
     * Check if data is empty or null and show empty state if needed
     * @param {Array|Object} data - The data to check
     * @param {string} containerId - The container ID to show empty state in
     * @param {string} type - Container type
     * @param {string} message - Optional custom message
     * @param {object} options - Additional options
     * @returns {boolean} - True if data is empty, false otherwise
     */
    checkAndShowEmptyState(data, containerId, type, message, options) {
        let isEmpty = false;
        
        // Check for different types of empty data
        if (data === null || data === undefined) {
            isEmpty = true;
        } else if (Array.isArray(data)) {
            isEmpty = data.length === 0;
        } else if (typeof data === 'object') {
            // For objects, check if it has any properties or if all arrays are empty
            const hasNonEmptyArray = Object.values(data).some(value =>
                Array.isArray(value) && value.length > 0
            );
            isEmpty = !hasNonEmptyArray && Object.keys(data).length === 0;
        }
        
        if (isEmpty) {
            this.showEmptyState(containerId, type, message, options);
        } else {
            this.clearEmptyState(containerId, type);
        }
        
        return isEmpty;
    }
};

// Export for use in other scripts
window.LogLynxUtils = LogLynxUtils;

// ============================================
// Global IP Search Functionality
// ============================================

let globalIPSearchDebounce = null;

// Initialize IP search trigger
function initIPSearch() {
    const trigger = document.getElementById('ipSearchTrigger');
    if (trigger) {
        trigger.addEventListener('click', () => {
            const modal = new bootstrap.Modal(document.getElementById('ipSearchModal'));
            modal.show();
            // Focus on input after modal is shown
            setTimeout(() => {
                document.getElementById('globalIPSearchInput').focus();
            }, 300);
        });
    }

    // Setup autocomplete
    const input = document.getElementById('globalIPSearchInput');
    if (input) {
        input.addEventListener('input', function() {
            const query = this.value.trim();
            
            if (globalIPSearchDebounce) {
                clearTimeout(globalIPSearchDebounce);
            }

            if (query.length < 2) {
                document.getElementById('globalIPSearchResults').innerHTML = '';
                return;
            }

            globalIPSearchDebounce = setTimeout(async () => {
                const results = await LogLynxAPI.searchIPs(query, 10);
                displayGlobalIPSearchResults(results.data || []);
            }, 300);
        });

        // Enter key to search
        input.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                performGlobalIPSearch();
            }
        });
    }
}

// Display search results
function displayGlobalIPSearchResults(results) {
    const container = document.getElementById('globalIPSearchResults');
    
    if (!results || results.length === 0) {
        container.innerHTML = '<p class="text-muted text-center">No results found</p>';
        return;
    }

    let html = '<div class="list-group">';
    results.forEach(result => {
        html += `
            <a href="/ip/${result.ip_address}" class="list-group-item list-group-item-action" 
               style="background: var(--loglynx-card); border-color: var(--border-color); color: #FFFFFF;">
                <div class="d-flex justify-content-between align-items-center">
                    <div>
                        <strong style="color: #F46319;">${result.ip_address}</strong>
                        <br>
                        <small class="text-muted">${result.city || 'Unknown'}, ${result.country || 'Unknown'}</small>
                    </div>
                    <div class="text-end">
                        <span class="badge badge-primary">${LogLynxUtils.formatNumber(result.hits)} hits</span>
                        <br>
                        <small class="text-muted">${LogLynxUtils.formatDateTime(result.last_seen)}</small>
                    </div>
                </div>
            </a>
        `;
    });
    html += '</div>';
    
    container.innerHTML = html;
}

// Perform IP search and navigate
function performGlobalIPSearch() {
    const ip = document.getElementById('globalIPSearchInput').value.trim();
    if (ip) {
        window.location.href = `/ip/${ip}`;
    }
}

// Initialize common features on DOM ready
document.addEventListener('DOMContentLoaded', () => {
    LogLynxUtils.initSidebar();
    initIPSearch();
    initDisplayPreference();
    // Hide filters on specific pages
    LogLynxUtils.hideFiltersOnSpecificPages();
});

/**
 * Hide filter service and hide my traffic options on specific pages
 * This function checks the current URL path and hides the filter elements
 * on System, backend, and IP detail pages
 */
LogLynxUtils.hideFiltersOnSpecificPages = function() {
    // Get the current path
    const currentPath = window.location.pathname;
    
    // Check if current page should have filters hidden
    const shouldHideFilters =
        currentPath === '/system' ||
        currentPath === '/backends' ||
        currentPath === '/content' ||
        currentPath === '/security' ||
        currentPath.startsWith('/ip/');
    
    if (shouldHideFilters) {
        // Hide the entire filters group
        const filtersGroup = document.querySelector('.header-filters-group');
        if (filtersGroup) {
            filtersGroup.style.display = 'none';
        }
    }
};

// Initialize Display Preference selector(s)
function initDisplayPreference() {
    // Find all display preference selectors (can be multiple on different pages)
    const selectors = [
        document.getElementById('displayPreference'),           // Global (if exists)
        document.getElementById('overviewDisplayPreference'),   // Overview page
        document.getElementById('realtimeDisplayPreference')    // Realtime page
    ].filter(el => el !== null);

    if (selectors.length === 0) return;

    // Restore from sessionStorage
    const savedPreference = sessionStorage.getItem('displayPreference') || 'auto';

    selectors.forEach(select => {
        select.value = savedPreference;

        // Handle change event
        select.addEventListener('change', (e) => {
            const newPreference = e.target.value;
            sessionStorage.setItem('displayPreference', newPreference);

            // Update all other selectors on the page
            selectors.forEach(s => {
                if (s !== select) s.value = newPreference;
            });

            // Reload all visible DataTables to apply new display preference
            if ($.fn.DataTable) {
                $('table.dataTable').each(function() {
                    if ($.fn.DataTable.isDataTable(this)) {
                        $(this).DataTable().ajax.reload(null, false);
                    }
                });
            }

            LogLynxUtils.showNotification('Display preference updated', 'success', 2000);
        });
    });
}

// Make initDisplayPreference available globally
window.initDisplayPreference = initDisplayPreference;

