/**
 * LogLynx Startup Loader
 * Handles initial application loading state until logs are processed
 */

const LogLynxStartupLoader = {
    MIN_PROCESSING_PERCENTAGE: 95,
    CHECK_INTERVAL: 1000, // Check every 1 second
    CHECK_INTERVAL_ERROR: 3000, // Slower polling when errors occur (3 seconds)
    MAX_CONSECUTIVE_ERRORS: 5, // Max errors before showing warning
    isReady: false,
    checkTimer: null,
    alreadyChecked: false, // Flag to avoid re-checking on subsequent page loads
    previousPercentage: 0, // Track previous percentage for ETA calculation
    lastCheckTime: null, // Track last check time for velocity calculation
    processingHistory: [], // Store recent processing speeds for better ETA
    consecutiveErrors: 0, // Track consecutive API errors
    databaseUnderLoad: false, // Flag for database load warning
    lastSuccessfulPercentage: 0, // Track last known good percentage
    
    /**
     * Initialize the startup loader
     */
    async init() {
        console.log('[StartupLoader] Initializing...');
        
        // If already checked and ready, skip entirely
        if (this.alreadyChecked && this.isReady) {
            console.log('[StartupLoader] Already ready, skipping loader');
            return;
        }
        
        // If already checked but not ready, show loader immediately
        if (this.alreadyChecked && !this.isReady) {
            console.log('[StartupLoader] Previously checked and not ready, showing loader');
            this.showLoadingScreen();
            await this.checkProcessingStatus();
            return;
        }
        
        // First time check - do a quick check before showing loader
        console.log('[StartupLoader] First time check, verifying status before showing loader...');
        this.alreadyChecked = true;
        
        try {
            const result = await LogLynxAPI.getLogProcessingStats();
            
            if (result.success && result.data) {
                const stats = result.data;
                
                // Check if already ready
                if (stats.length === 0) {
                    // No sources, allow immediate access
                    console.log('[StartupLoader] No log sources, allowing immediate access');
                    this.isReady = true;
                    return;
                }
                
                // Calculate average percentage
                let totalPercentage = 0;
                stats.forEach(source => {
                    totalPercentage += source.percentage || 0;
                });
                const avgPercentage = totalPercentage / stats.length;
                
                if (avgPercentage >= this.MIN_PROCESSING_PERCENTAGE) {
                    // Already ready, don't show loader
                    console.log(`[StartupLoader] Already at ${avgPercentage.toFixed(2)}%, skipping loader`);
                    this.isReady = true;
                    return;
                }
                
                // Not ready yet, show loader and start monitoring
                console.log(`[StartupLoader] At ${avgPercentage.toFixed(2)}%, showing loader`);
                this.showLoadingScreen();
                await this.checkProcessingStatus();
            } else {
                // Error checking, assume ready
                console.log('[StartupLoader] Error checking initial status, assuming ready');
                this.isReady = true;
            }
        } catch (error) {
            // Error checking, assume ready
            console.error('[StartupLoader] Error in initial check:', error);
            this.isReady = true;
        }
    },
    
    /**
     * Show the startup loading screen
     */
    showLoadingScreen() {
        // Check if loading screen already exists
        let loadingScreen = document.getElementById('startupLoadingScreen');
        if (loadingScreen) {
            loadingScreen.style.display = 'flex';
            return;
        }
        
        // Create loading screen
        loadingScreen = document.createElement('div');
        loadingScreen.id = 'startupLoadingScreen';
        loadingScreen.style.cssText = `
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            z-index: 9999;
            color: #FFFFFF;
        `;
        
        loadingScreen.innerHTML = `
            <div style="text-align: center; max-width: 600px; padding: 20px;">
                <div style="margin-bottom: 30px;">
                    <i class="fas fa-bolt" style="font-size: 64px; color: #F46319; animation: pulse 2s infinite;"></i>
                </div>
                
                <h1 style="font-size: 36px; margin-bottom: 10px; color: #FFFFFF;">
                    LogLynx
                </h1>
                
                <p style="font-size: 18px; color: #999; margin-bottom: 40px;">
                    Advanced Log Analytics Platform
                </p>
                
                <div style="margin-bottom: 20px;">
                    <div id="loadingMessage" style="font-size: 16px; color: #FFFFFF; margin-bottom: 15px;">
                        Initializing log processing...
                    </div>
                    
                    <div style="background: rgba(255, 255, 255, 0.1); border-radius: 12px; height: 24px; overflow: hidden; position: relative;">
                        <div id="loadingProgressBar" style="
                            background: linear-gradient(90deg, #F46319 0%, #FF8C42 100%);
                            height: 100%;
                            width: 0%;
                            transition: width 0.3s ease;
                            border-radius: 12px;
                            box-shadow: 0 0 10px rgba(244, 99, 25, 0.5);
                        "></div>
                        <div id="loadingPercentageText" style="
                            position: absolute;
                            top: 50%;
                            left: 50%;
                            transform: translate(-50%, -50%);
                            font-size: 14px;
                            font-weight: bold;
                            color: #FFFFFF;
                            text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
                        ">0%</div>
                    </div>
                </div>
                
                <div id="loadingDetails" style="font-size: 13px; color: #666; font-family: monospace;">
                    Waiting for log sources...
                </div>
                
                <!-- Database Load Warning -->
                <div id="databaseWarning" style="
                    display: none;
                    margin-top: 20px;
                    padding: 12px 20px;
                    background: rgba(255, 152, 0, 0.1);
                    border: 1px solid rgba(255, 152, 0, 0.3);
                    border-radius: 8px;
                    color: #FFA726;
                ">
                    <i class="fas fa-exclamation-triangle" style="margin-right: 8px;"></i>
                    <span id="warningMessage">Database under heavy load, processing may be slower...</span>
                </div>
            </div>
            
            <style>
                @keyframes pulse {
                    0%, 100% { opacity: 1; transform: scale(1); }
                    50% { opacity: 0.7; transform: scale(1.05); }
                }
            </style>
        `;
        
        document.body.appendChild(loadingScreen);
        
        // Hide main content
        const mainContent = document.querySelector('.app-container');
        if (mainContent) {
            mainContent.style.visibility = 'hidden';
        }
    },
    
    /**
     * Hide the startup loading screen
     */
    hideLoadingScreen() {
        const loadingScreen = document.getElementById('startupLoadingScreen');
        if (loadingScreen) {
            // Fade out animation
            loadingScreen.style.transition = 'opacity 0.5s ease';
            loadingScreen.style.opacity = '0';
            
            setTimeout(() => {
                loadingScreen.remove();
                
                // Show main content
                const mainContent = document.querySelector('.app-container');
                if (mainContent) {
                    mainContent.style.visibility = 'visible';
                }
            }, 500);
        }
    },
    
    /**
     * Update loading progress display
     */
    updateProgress(stats) {
        const messageEl = document.getElementById('loadingMessage');
        const progressBar = document.getElementById('loadingProgressBar');
        const percentageText = document.getElementById('loadingPercentageText');
        const detailsEl = document.getElementById('loadingDetails');

        if (!stats || stats.length === 0) {
            if (messageEl) messageEl.textContent = 'Waiting for log sources...';
            if (progressBar) progressBar.style.width = '0%';
            if (percentageText) percentageText.textContent = '0%';
            if (detailsEl) detailsEl.textContent = 'No log sources configured yet';
            return;
        }

        // Calculate average percentage across all sources
        let totalPercentage = 0;
        let totalBytes = 0;
        let processedBytes = 0;

        stats.forEach(source => {
            totalPercentage += source.percentage || 0;
            totalBytes += source.file_size || 0;
            processedBytes += source.bytes_processed || 0;
        });

        const avgPercentage = stats.length > 0 ? (totalPercentage / stats.length) : 0;
        const roundedPercentage = Math.round(avgPercentage * 10) / 10; // Round to 1 decimal

        // Store last successful percentage
        this.lastSuccessfulPercentage = avgPercentage;

        // Calculate ETA based on processing speed
        const eta = this.calculateETA(avgPercentage);

        // Update progress bar
        if (progressBar) {
            progressBar.style.width = `${avgPercentage}%`;
        }

        if (percentageText) {
            percentageText.textContent = `${roundedPercentage.toFixed(1)}%`;
        }

        // Update message based on progress
        if (messageEl) {
            if (avgPercentage < 50) {
                messageEl.textContent = eta ? `Processing logs... ETA: ${eta}` : 'Processing logs... This may take a moment';
            } else if (avgPercentage < 90) {
                messageEl.textContent = eta ? `Almost there... ETA: ${eta}` : 'Almost there... Loading log data';
            } else if (avgPercentage < this.MIN_PROCESSING_PERCENTAGE) {
                messageEl.textContent = eta ? `Finalizing... ETA: ${eta}` : 'Finalizing... Just a few more seconds';
            } else {
                messageEl.textContent = 'Ready! Loading dashboard...';
            }
        }

        // Update details
        if (detailsEl) {
            const bytesText = this.formatBytes(processedBytes) + ' / ' + this.formatBytes(totalBytes);
            const sourcesText = stats.length === 1 ? '1 source' : `${stats.length} sources`;
            const speed = this.getProcessingSpeed();
            const speedText = speed ? ` • ${speed}%/s` : '';
            detailsEl.textContent = `Processing ${bytesText} from ${sourcesText}${speedText}`;
        }
    },

    /**
     * Show database load warning
     */
    showDatabaseWarning() {
        const warningEl = document.getElementById('databaseWarning');
        const messageEl = document.getElementById('warningMessage');

        if (warningEl && !this.databaseUnderLoad) {
            this.databaseUnderLoad = true;
            warningEl.style.display = 'block';

            if (messageEl) {
                if (this.consecutiveErrors >= this.MAX_CONSECUTIVE_ERRORS) {
                    messageEl.textContent = 'Database under heavy load. Please wait, processing continues...';
                } else {
                    messageEl.textContent = 'Temporary connection issues. Retrying...';
                }
            }
        }
    },

    /**
     * Hide database load warning
     */
    hideDatabaseWarning() {
        const warningEl = document.getElementById('databaseWarning');

        if (warningEl && this.databaseUnderLoad) {
            warningEl.style.transition = 'opacity 0.5s ease';
            warningEl.style.opacity = '0';

            setTimeout(() => {
                warningEl.style.display = 'none';
                warningEl.style.opacity = '1';
                this.databaseUnderLoad = false;
            }, 500);
        }
    },
    
    /**
     * Calculate ETA based on processing speed (improved algorithm)
     */
    calculateETA(currentPercentage) {
        const now = Date.now();

        // First check, no ETA yet
        if (this.lastCheckTime === null) {
            this.lastCheckTime = now;
            this.previousPercentage = currentPercentage;
            return null;
        }

        // Calculate time elapsed since last check (in seconds)
        const timeElapsed = (now - this.lastCheckTime) / 1000;

        // Calculate percentage progress since last check
        const percentageProgress = currentPercentage - this.previousPercentage;

        // Calculate speed (percentage per second)
        const speed = timeElapsed > 0 ? percentageProgress / timeElapsed : 0;

        // Store in history (keep last 10 measurements for better smoothing)
        if (speed > 0 && percentageProgress > 0) {
            this.processingHistory.push({
                speed: speed,
                timestamp: now,
                percentage: currentPercentage
            });

            // Keep only last 10 measurements
            if (this.processingHistory.length > 10) {
                this.processingHistory.shift();
            }
        }

        // Update tracking variables
        this.lastCheckTime = now;
        this.previousPercentage = currentPercentage;

        // Need at least 3 measurements for reliable ETA
        if (this.processingHistory.length < 3) {
            return null;
        }

        // Remove outliers and calculate weighted average (recent measurements more important)
        const speeds = this.processingHistory.map(h => h.speed);
        const sortedSpeeds = [...speeds].sort((a, b) => a - b);

        // Remove top and bottom 20% if we have enough samples
        let filteredSpeeds = speeds;
        if (speeds.length >= 5) {
            const removeCount = Math.floor(speeds.length * 0.2);
            const min = sortedSpeeds[removeCount];
            const max = sortedSpeeds[sortedSpeeds.length - removeCount - 1];
            filteredSpeeds = speeds.filter(s => s >= min && s <= max);
        }

        // Weighted average - give more weight to recent measurements
        let weightedSum = 0;
        let weightSum = 0;
        filteredSpeeds.forEach((speed, index) => {
            const weight = index + 1; // Linear weight increase
            weightedSum += speed * weight;
            weightSum += weight;
        });

        const avgSpeed = weightSum > 0 ? weightedSum / weightSum : 0;

        // If speed is too slow or negative, no reliable ETA
        if (avgSpeed <= 0.001) {
            return null;
        }

        // Calculate remaining percentage
        const remainingPercentage = this.MIN_PROCESSING_PERCENTAGE - currentPercentage;

        // If already past target, no ETA needed
        if (remainingPercentage <= 0) {
            return 'a few seconds';
        }

        // Calculate ETA in seconds
        let etaSeconds = remainingPercentage / avgSpeed;

        // Add some buffer for final processing (last 5% typically slower)
        if (currentPercentage > 90) {
            etaSeconds *= 1.2; // 20% buffer for final phase
        }

        // Format ETA
        return this.formatETA(etaSeconds);
    },
    
    /**
     * Get current processing speed
     */
    getProcessingSpeed() {
        if (this.processingHistory.length === 0) {
            return null;
        }

        // Get average speed from recent history
        const speeds = this.processingHistory.map(h => h.speed);
        const avgSpeed = speeds.reduce((a, b) => a + b, 0) / speeds.length;
        return avgSpeed.toFixed(2);
    },
    
    /**
     * Format ETA in human readable format
     */
    formatETA(seconds) {
        if (seconds < 0) return null;
        
        if (seconds < 10) {
            return 'a few seconds';
        } else if (seconds < 60) {
            return `${Math.round(seconds)}s`;
        } else if (seconds < 3600) {
            const minutes = Math.floor(seconds / 60);
            const secs = Math.round(seconds % 60);
            return secs > 0 ? `${minutes}m ${secs}s` : `${minutes}m`;
        } else {
            const hours = Math.floor(seconds / 3600);
            const minutes = Math.floor((seconds % 3600) / 60);
            return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
        }
    },
    
    /**
     * Check processing status (with retry logic and error handling)
     */
    async checkProcessingStatus() {
        try {
            const result = await LogLynxAPI.getLogProcessingStats();

            if (result.success && result.data) {
                const stats = result.data;

                // Reset error counter on success
                if (this.consecutiveErrors > 0) {
                    console.log('[StartupLoader] Connection restored after errors');
                    this.hideDatabaseWarning();
                }
                this.consecutiveErrors = 0;

                // Update progress display
                this.updateProgress(stats);

                // Check if all sources are processed enough
                if (stats.length === 0) {
                    // No sources yet, keep waiting
                    console.log('[StartupLoader] No log sources found, waiting...');
                    this.scheduleNextCheck();
                    return;
                }

                // Calculate average percentage
                let totalPercentage = 0;
                stats.forEach(source => {
                    totalPercentage += source.percentage || 0;
                });
                const avgPercentage = totalPercentage / stats.length;

                console.log(`[StartupLoader] Processing status: ${avgPercentage.toFixed(2)}%`);

                if (avgPercentage >= this.MIN_PROCESSING_PERCENTAGE) {
                    // Ready to show application
                    console.log('[StartupLoader] Processing complete, showing application');
                    this.isReady = true;
                    this.onReady();
                } else {
                    // Not ready yet, check again
                    this.scheduleNextCheck();
                }
            } else {
                // API returned error or invalid data
                this.handleCheckError(result.error || 'Invalid response from API');
            }
        } catch (error) {
            // Network or other error
            this.handleCheckError(error);
        }
    },

    /**
     * Handle check errors with retry logic
     */
    handleCheckError(error) {
        this.consecutiveErrors++;

        console.warn(`[StartupLoader] Error checking processing status (attempt ${this.consecutiveErrors}/${this.MAX_CONSECUTIVE_ERRORS}):`, error);

        // Show warning after consecutive errors
        if (this.consecutiveErrors >= this.MAX_CONSECUTIVE_ERRORS) {
            this.showDatabaseWarning();
        }

        // Keep last known progress visible
        if (this.lastSuccessfulPercentage > 0) {
            const progressBar = document.getElementById('loadingProgressBar');
            const percentageText = document.getElementById('loadingPercentageText');

            if (progressBar) {
                progressBar.style.opacity = '0.7'; // Dim to indicate stale data
            }
            if (percentageText) {
                percentageText.textContent = `${this.lastSuccessfulPercentage.toFixed(1)}%`;
            }
        }

        // Continue checking with slower interval during errors
        this.scheduleNextCheck(true);
    },
    
    /**
     * Schedule next status check
     */
    scheduleNextCheck(isError = false) {
        if (this.checkTimer) {
            clearTimeout(this.checkTimer);
        }

        // Use slower interval during errors to reduce load
        const interval = isError ? this.CHECK_INTERVAL_ERROR : this.CHECK_INTERVAL;

        this.checkTimer = setTimeout(() => {
            this.checkProcessingStatus();
        }, interval);
    },
    
    /**
     * Called when application is ready
     */
    onReady() {
        console.log('[StartupLoader] Application is ready!');
        
        // Clear check timer
        if (this.checkTimer) {
            clearTimeout(this.checkTimer);
            this.checkTimer = null;
        }
        
        // Hide loading screen
        this.hideLoadingScreen();
        
        // Dispatch ready event
        window.dispatchEvent(new CustomEvent('loglynx:ready'));
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
    }
};

// Initialize the startup loader when DOM is ready or immediately if already loaded
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        LogLynxStartupLoader.init();
    });
} else {
    // DOM already loaded, init immediately
    LogLynxStartupLoader.init();
}

// Export for use in other scripts
window.LogLynxStartupLoader = LogLynxStartupLoader;
