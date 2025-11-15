/**
 * LogLynx Startup Loader
 * Handles initial application loading state until logs are processed
 */

const LogLynxStartupLoader = {
    MIN_PROCESSING_PERCENTAGE: 99,
    CHECK_INTERVAL: 1000, // Check every 1 second
    CHECK_INTERVAL_ERROR: 3000, // Slower polling when errors occur (3 seconds)
    DATA_VERIFICATION_WAIT: 5000, // Wait 5 seconds before retrying data verification
    MAX_CONSECUTIVE_ERRORS: 5, // Max errors before showing warning
    isReady: false,
    checkTimer: null,
    countdownTimer: null, // Timer for ETA countdown animation
    elapsedTimer: null, // Timer for elapsed time counter
    startProcessingTime: null, // When processing started
    alreadyChecked: false, // Flag to avoid re-checking on subsequent page loads
    previousPercentage: 0, // Track previous percentage for ETA calculation
    lastCheckTime: null, // Track last check time for velocity calculation
    processingHistory: [], // Store recent processing speeds for better ETA
    consecutiveErrors: 0, // Track consecutive API errors
    databaseUnderLoad: false, // Flag for database load warning
    lastSuccessfulPercentage: 0, // Track last known good percentage
    currentEtaSeconds: null, // Current ETA in seconds for countdown
    lastEtaUpdateTime: null, // When ETA was last calculated
    splashScreenEnabled: true, // Default to enabled, will be loaded from config
    isInitialLoad: true, // Flag to track if this is the first load (needs data verification)
    
    /**
     * Initialize the startup loader
     */
    async init() {
        console.log('[StartupLoader] Initializing...');

        // Load configuration from window variable (set by server in template)
        if (window.LOGLYNX_CONFIG && typeof window.LOGLYNX_CONFIG.splashScreenEnabled === 'boolean') {
            this.splashScreenEnabled = window.LOGLYNX_CONFIG.splashScreenEnabled;
            console.log('[StartupLoader] Configuration loaded, splash screen enabled:', this.splashScreenEnabled);
        }

        // If splash screen is disabled, skip everything
        if (!this.splashScreenEnabled) {
            console.log('[StartupLoader] Splash screen disabled by configuration');
            this.isReady = true;
            return;
        }

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
            // Check processing status first
            const result = await LogLynxAPI.getLogProcessingStats();
            console.log('[StartupLoader] Processing stats:', result);

            if (result.success) {
                // Handle null data (no log sources) as empty array
                const stats = result.data || [];

                // If no log sources configured, check if database is empty
                if (stats.length === 0) {
                    const summaryResult = await LogLynxAPI.getSummary();
                    console.log('[StartupLoader] No log sources, checking summary:', summaryResult);

                    if (summaryResult.success && summaryResult.data) {
                        console.log('[StartupLoader] Summary total_requests:', summaryResult.data.total_requests);

                        if (summaryResult.data.total_requests === 0) {
                            console.log('[StartupLoader] Database is empty (0 requests), showing setup screen');
                            this.showEmptyDatabaseScreen();
                            return; // Don't mark as ready
                        }

                        console.log('[StartupLoader] No log sources but has data, allowing immediate access');
                        this.isReady = true;
                        return;
                    } else {
                        // API call failed, assume empty database and show setup screen
                        console.log('[StartupLoader] Failed to get summary, assuming empty database');
                        this.showEmptyDatabaseScreen();
                        return;
                    }
                }
                
                // Calculate average percentage
                let totalPercentage = 0;
                stats.forEach(source => {
                    totalPercentage += source.percentage || 0;
                });
                const avgPercentage = totalPercentage / stats.length;
                
                if (avgPercentage >= this.MIN_PROCESSING_PERCENTAGE) {
                    // Already ready, don't show loader and skip data verification
                    console.log(`[StartupLoader] Already at ${avgPercentage.toFixed(2)}%, skipping loader`);
                    this.isReady = true;
                    this.isInitialLoad = false; // Not really initial load if already complete
                    return;
                }
                
                // Not ready yet, show loader and start monitoring
                console.log(`[StartupLoader] At ${avgPercentage.toFixed(2)}%, showing loader`);
                this.showLoadingScreen();
                this.startProcessingTime = Date.now(); // Record start time
                this.startElapsedTimer(); // Start elapsed time counter
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
            this.loadVersion(); // Reload version in case it wasn't loaded
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

                <!-- Version Footer -->
                <div id="splashVersion" style="
                    position: absolute;
                    bottom: 20px;
                    left: 0;
                    right: 0;
                    text-align: center;
                    font-size: 12px;
                    color: #666;
                    font-family: monospace;
                ">
                    Loading version info...
                </div>

                <!-- Repository Link -->
                <div style="
                    position: absolute;
                    bottom: 5px;
                    left: 0;
                    right: 0;
                    text-align: center;
                    font-size: 11px;
                ">
                    <a href="https://github.com/K0lin/loglynx" target="_blank" rel="noopener noreferrer" style="
                        color: #888;
                        text-decoration: none;
                        transition: color 0.3s ease;
                    " onmouseover="this.style.color='#F46319'" onmouseout="this.style.color='#888'">
                        <i class="fab fa-github" style="margin-right: 5px;"></i>GitHub
                    </a>
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

        // Load version info
        this.loadVersion();
    },

    /**
     * Load and display version information
     */
    async loadVersion() {
        try {
            const response = await fetch('/api/v1/version');
            if (response.ok) {
                const data = await response.json();
                const versionEl = document.getElementById('splashVersion');
                if (versionEl) {
                    versionEl.innerHTML = `LogLynx v${data.version}`;
                }
            }
        } catch (error) {
            console.warn('[StartupLoader] Failed to load version:', error);
            const versionEl = document.getElementById('splashVersion');
            if (versionEl) {
                versionEl.innerHTML = 'LogLynx';
            }
        }
    },

    /**
     * Show empty database setup screen
     */
    showEmptyDatabaseScreen() {
        let loadingScreen = document.getElementById('startupLoadingScreen');
        if (loadingScreen) {
            loadingScreen.style.display = 'flex';
            return;
        }

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
            <div style="text-align: center; max-width: 700px; padding: 20px;">
                <div style="margin-bottom: 30px;">
                    <i class="fas fa-database" style="font-size: 64px; color: #F46319; animation: pulse 2s infinite;"></i>
                </div>

                <h1 style="font-size: 36px; margin-bottom: 10px; color: #FFFFFF;">
                    Welcome to LogLynx!
                </h1>

                <p style="font-size: 18px; color: #999; margin-bottom: 40px;">
                    Your analytics platform is ready, but needs data to analyze
                </p>

                <div style="background: rgba(255, 255, 255, 0.05); border-radius: 12px; padding: 30px; margin-bottom: 30px; text-align: left;">
                    <h3 style="color: #F46319; margin-bottom: 20px;">
                        <i class="fas fa-info-circle"></i> Getting Started
                    </h3>

                    <div style="margin-bottom: 15px;">
                        <strong style="color: #FFF;">1. Configure Log Sources</strong>
                        <p style="color: #999; margin: 5px 0 0 0; font-size: 14px;">
                            Set <code style="background: rgba(0,0,0,0.3); padding: 2px 6px; border-radius: 3px;">TRAEFIK_LOG_PATH</code> in your .env file
                        </p>
                    </div>

                    <div style="margin-bottom: 15px;">
                        <strong style="color: #FFF;">2. Ensure Log Files Exist</strong>
                        <p style="color: #999; margin: 5px 0 0 0; font-size: 14px;">
                            LogLynx will automatically discover and process access logs
                        </p>
                    </div>

                    <div>
                        <strong style="color: #FFF;">3. Wait for Processing</strong>
                        <p style="color: #999; margin: 5px 0 0 0; font-size: 14px;">
                            This page will automatically update when data is available
                        </p>
                    </div>
                </div>

                <div style="display: flex; gap: 15px; justify-content: center; margin-bottom: 30px;">
                    <button onclick="window.open('https://github.com/K0lin/loglynx/wiki', '_blank')"
                            style="
                                background: #F46319;
                                color: white;
                                border: none;
                                padding: 12px 24px;
                                border-radius: 6px;
                                font-size: 16px;
                                cursor: pointer;
                                transition: background 0.3s;
                            "
                            onmouseover="this.style.background='#d45417'"
                            onmouseout="this.style.background='#F46319'">
                        <i class="fas fa-book"></i> View Documentation
                    </button>

                    <button onclick="location.reload()"
                            style="
                                background: rgba(255,255,255,0.1);
                                color: white;
                                border: 1px solid rgba(255,255,255,0.2);
                                padding: 12px 24px;
                                border-radius: 6px;
                                font-size: 16px;
                                cursor: pointer;
                                transition: background 0.3s;
                            "
                            onmouseover="this.style.background='rgba(255,255,255,0.2)'"
                            onmouseout="this.style.background='rgba(255,255,255,0.1)'">
                        <i class="fas fa-sync-alt"></i> Check Again
                    </button>
                </div>

                <div style="font-size: 13px; color: #666;">
                    <i class="fas fa-clock"></i> Checking for data every 10 seconds...
                </div>

                <!-- Version Footer -->
                <div id="splashVersion" style="
                    position: absolute;
                    bottom: 20px;
                    left: 0;
                    right: 0;
                    text-align: center;
                    font-size: 12px;
                    color: #666;
                    font-family: monospace;
                ">
                    Loading version info...
                </div>

                <!-- Repository Link -->
                <div style="
                    position: absolute;
                    bottom: 5px;
                    left: 0;
                    right: 0;
                    text-align: center;
                    font-size: 11px;
                ">
                    <a href="https://github.com/K0lin/loglynx" target="_blank" rel="noopener noreferrer" style="
                        color: #888;
                        text-decoration: none;
                        transition: color 0.3s ease;
                    " onmouseover="this.style.color='#F46319'" onmouseout="this.style.color='#888'">
                        <i class="fab fa-github" style="margin-right: 5px;"></i>GitHub
                    </a>
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

        // Load version
        this.loadVersion();

        // Poll every 10 seconds to check if data is available
        this.emptyDatabaseCheckTimer = setInterval(async () => {
            console.log('[StartupLoader] Checking for data...');
            const summaryResult = await LogLynxAPI.getSummary();
            if (summaryResult.success && summaryResult.data.total_requests > 0) {
                console.log('[StartupLoader] Data found! Reloading page...');
                clearInterval(this.emptyDatabaseCheckTimer);
                location.reload();
            }
        }, 10000); // Check every 10 seconds
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

        // Start countdown animation if we have an ETA
        if (eta && this.currentEtaSeconds !== null) {
            this.startEtaCountdown();
        }

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
            const elapsed = this.getElapsedTime();
            const elapsedText = elapsed ? ` • waiting ${elapsed}` : '';
            detailsEl.textContent = `Processing ${bytesText} from ${sourcesText}${speedText}${elapsedText}`;
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

        // Add realistic buffers based on progress stage
        if (currentPercentage < 30) {
            // Early stage: add 50% buffer (parsing overhead, cold start)
            etaSeconds *= 1.5;
        } else if (currentPercentage < 60) {
            // Mid stage: add 40% buffer
            etaSeconds *= 1.4;
        } else if (currentPercentage < 85) {
            // Late stage: add 30% buffer
            etaSeconds *= 1.3;
        } else if (currentPercentage < 95) {
            // Final stage: add 50% buffer (indexes, final processing)
            etaSeconds *= 1.5;
        } else {
            // Very final: add 60% buffer (finishing touches are slow)
            etaSeconds *= 1.6;
        }

        // Store ETA in seconds for countdown
        this.currentEtaSeconds = etaSeconds;

        // Format ETA
        return this.formatETA(etaSeconds);
    },

    /**
     * Start ETA countdown animation between API calls
     */
    startEtaCountdown() {
        // Clear any existing countdown timer
        if (this.countdownTimer) {
            clearInterval(this.countdownTimer);
        }

        // Store when we started countdown
        this.lastEtaUpdateTime = Date.now();

        // Update countdown every second
        this.countdownTimer = setInterval(() => {
            if (this.currentEtaSeconds === null || this.currentEtaSeconds <= 0) {
                this.stopEtaCountdown();
                return;
            }

            // Decrease ETA by 1 second
            this.currentEtaSeconds = Math.max(0, this.currentEtaSeconds - 1);

            // Update the displayed ETA
            this.updateEtaDisplay();
        }, 1000);
    },

    /**
     * Stop ETA countdown animation
     */
    stopEtaCountdown() {
        if (this.countdownTimer) {
            clearInterval(this.countdownTimer);
            this.countdownTimer = null;
        }
    },

    /**
     * Update ETA display with current countdown value
     */
    updateEtaDisplay() {
        const messageEl = document.getElementById('loadingMessage');
        if (!messageEl || this.currentEtaSeconds === null) {
            return;
        }

        const etaText = this.formatETA(this.currentEtaSeconds);
        const currentText = messageEl.textContent;

        // Only update if message contains ETA
        if (currentText.includes('ETA:')) {
            if (this.lastSuccessfulPercentage < 50) {
                messageEl.textContent = `Processing logs... ETA: ${etaText}`;
            } else if (this.lastSuccessfulPercentage < 90) {
                messageEl.textContent = `Almost there... ETA: ${etaText}`;
            } else if (this.lastSuccessfulPercentage < this.MIN_PROCESSING_PERCENTAGE) {
                messageEl.textContent = `Finalizing... ETA: ${etaText}`;
            }
        }
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
     * Get elapsed time since processing started
     */
    getElapsedTime() {
        if (!this.startProcessingTime) {
            return null;
        }

        const elapsedMs = Date.now() - this.startProcessingTime;
        const elapsedSeconds = Math.floor(elapsedMs / 1000);

        return this.formatElapsedTime(elapsedSeconds);
    },

    /**
     * Format elapsed time in human readable format
     */
    formatElapsedTime(seconds) {
        if (seconds < 60) {
            return `${seconds}s`;
        } else if (seconds < 3600) {
            const minutes = Math.floor(seconds / 60);
            const secs = seconds % 60;
            return `${minutes}m ${secs}s`;
        } else {
            const hours = Math.floor(seconds / 3600);
            const minutes = Math.floor((seconds % 3600) / 60);
            return `${hours}h ${minutes}m`;
        }
    },

    /**
     * Start elapsed time counter
     */
    startElapsedTimer() {
        // Clear any existing timer
        if (this.elapsedTimer) {
            clearInterval(this.elapsedTimer);
        }

        // Update elapsed time every second
        this.elapsedTimer = setInterval(() => {
            this.updateElapsedDisplay();
        }, 1000);
    },

    /**
     * Stop elapsed time counter
     */
    stopElapsedTimer() {
        if (this.elapsedTimer) {
            clearInterval(this.elapsedTimer);
            this.elapsedTimer = null;
        }
    },

    /**
     * Update elapsed time display
     */
    updateElapsedDisplay() {
        const detailsEl = document.getElementById('loadingDetails');
        if (!detailsEl || !this.startProcessingTime) {
            return;
        }

        const currentText = detailsEl.textContent;
        const elapsed = this.getElapsedTime();

        if (elapsed && currentText) {
            // Replace or add elapsed time in the details text
            const elapsedPattern = / • waiting \d+[smh]( \d+[smh])?/;
            if (elapsedPattern.test(currentText)) {
                // Update existing elapsed time
                detailsEl.textContent = currentText.replace(elapsedPattern, ` • waiting ${elapsed}`);
            } else if (currentText.includes('Processing')) {
                // Add elapsed time if not present
                detailsEl.textContent = currentText + ` • waiting ${elapsed}`;
            }
        }
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

            if (result.success) {
                // Handle null data (no log sources) as empty array
                const stats = result.data || [];

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
                    // Processing complete, verify data availability before showing application
                    console.log('[StartupLoader] Processing complete, verifying data availability...');
                    await this.verifyDataAndFinish();
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
     * Verify that data is actually available before finishing
     * Only done during initial load to ensure data is ready
     */
    async verifyDataAndFinish() {
        // If not initial load, skip verification
        if (!this.isInitialLoad) {
            console.log('[StartupLoader] Not initial load, skipping data verification');
            this.isReady = true;
            this.onReady();
            return;
        }

        try {
            // Check if summary data is available
            const summaryResult = await LogLynxAPI.getSummary();

            if (summaryResult.success && summaryResult.data && summaryResult.data.total_requests > 0) {
                // Data is available, refresh page to load fresh data
                console.log('[StartupLoader] Data verified, refreshing page to load fresh data');
                this.updateLoadingMessage('Ready! Refreshing page...');
                this.isInitialLoad = false; // Mark that initial load is complete

                // Small delay to show the message, then refresh
                setTimeout(() => {
                    location.reload();
                }, 500);
            } else {
                // Data not yet available, wait and retry
                console.log('[StartupLoader] Data not yet available, waiting...');
                this.updateLoadingMessage('Finalizing... Data indexing in progress');

                setTimeout(async () => {
                    await this.verifyDataAndFinish();
                }, this.DATA_VERIFICATION_WAIT);
            }
        } catch (error) {
            // Error checking data, wait and retry
            console.warn('[StartupLoader] Error verifying data:', error);
            this.updateLoadingMessage('Finalizing... Please wait');

            setTimeout(async () => {
                await this.verifyDataAndFinish();
            }, this.DATA_VERIFICATION_WAIT);
        }
    },

    /**
     * Update loading message
     */
    updateLoadingMessage(message) {
        const messageEl = document.getElementById('loadingMessage');
        if (messageEl) {
            messageEl.textContent = message;
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

        // Clear countdown timer
        this.stopEtaCountdown();

        // Clear elapsed timer
        this.stopElapsedTimer();

        // Hide loading screen
        this.hideLoadingScreen();

        // Dispatch ready event
        window.dispatchEvent(new CustomEvent('loglynx:ready'));
    },    /**
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
