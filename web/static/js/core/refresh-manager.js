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

const RefreshManager = (() => {
    const STORAGE_KEY = 'loglynx_refresh_global';
    const DEFAULT_INTERVAL = 30;
    const DEFAULT_AUTO_START = true;
    
    // Run the check on script load
    checkAndHideFilters();   

    // Session-only state (cleared on navigation)
    let pageOverride = null;
    let currentPage = null;

    /**
     * Get current page identifier from URL
     */
    function getCurrentPageId() {
        const path = window.location.pathname;
        if (path === '/' || path === '/index' || path === '/overview') return 'overview';
        return path.replace(/^\//, '').replace(/\//g, '-') || 'overview';
    }

    /**
     * Check current path and hide refresh filters if needed
     */
    function checkAndHideFilters() {
        const currentPath = window.location.pathname;

        const shouldHideFilters =
        currentPath === '/system' ||
        currentPath === '/backends' ||
        currentPath === '/content' ||
        currentPath === '/security' ||
        currentPath === '/realtime' ||
        currentPath.startsWith('/ip/');
        
        if (shouldHideFilters) {
            // Hide the entire refresh-controls container
            const refreshControls = document.querySelector('.header-refresh-controls');
                if (refreshControls) {
                    refreshControls.style.display = 'none';
                }
        }
    }


    /**
     * Load global settings from localStorage
     */
    function loadGlobalSettings() {
        try {
            const stored = localStorage.getItem(STORAGE_KEY);
            if (stored) {
                const parsed = JSON.parse(stored);
                return {
                    interval: parsed.interval || DEFAULT_INTERVAL,
                    autoStart: parsed.autoStart !== undefined ? parsed.autoStart : DEFAULT_AUTO_START
                };
            }
        } catch (e) {
            console.warn('[RefreshManager] Failed to load global settings from localStorage:', e);
        }

        return {
            interval: DEFAULT_INTERVAL,
            autoStart: DEFAULT_AUTO_START
        };
    }

    /**
     * Save global settings to localStorage
     */
    function saveGlobalSettings(settings) {
        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
            console.log('[RefreshManager] Global settings saved:', settings);
        } catch (e) {
            console.warn('[RefreshManager] Failed to save global settings to localStorage:', e);
        }
    }

    /**
     * Initialize RefreshManager for current page
     */
    function init() {
        currentPage = getCurrentPageId();
        // Clear any previous page override when initializing
        pageOverride = null;

        console.log('[RefreshManager] Initialized for page:', currentPage);
    }

    /**
     * Get current settings (page override OR global)
     */
    function getCurrentSettings() {
        if (pageOverride) {
            return {
                ...pageOverride,
                isPageSpecific: true,
                pageId: currentPage
            };
        }

        const global = loadGlobalSettings();
        return {
            ...global,
            isPageSpecific: false,
            pageId: null
        };
    }

    /**
     * Update global settings
     */
    function updateGlobal(interval, autoStart) {
        const settings = {
            interval: interval,
            autoStart: autoStart !== undefined ? autoStart : true
        };

        saveGlobalSettings(settings);
        console.log('[RefreshManager] Global settings updated:', settings);
    }

    /**
     * Enable page-specific override (session-only)
     */
    function enablePageOverride(interval, autoStart) {
        pageOverride = {
            interval: interval,
            autoStart: autoStart !== undefined ? autoStart : true
        };

        console.log('[RefreshManager] Page override enabled for', currentPage, ':', pageOverride);
    }

    /**
     * Disable page-specific override (revert to global)
     */
    function disablePageOverride() {
        pageOverride = null;
        console.log('[RefreshManager] Page override disabled for', currentPage);
    }

    /**
     * Check if page has active override
     */
    function hasPageOverride() {
        return pageOverride !== null;
    }

    /**
     * Update current settings (global or page-specific)
     */
    function updateCurrent(interval, autoStart) {
        if (hasPageOverride()) {
            // Update page override
            enablePageOverride(interval, autoStart);
        } else {
            // Update global
            updateGlobal(interval, autoStart);
        }
    }

    // Public API
    return {
        init,
        getCurrentSettings,
        updateGlobal,
        enablePageOverride,
        disablePageOverride,
        hasPageOverride,
        updateCurrent,
        getCurrentPageId
    };
})();

// Export for use in other modules
window.RefreshManager = RefreshManager;

