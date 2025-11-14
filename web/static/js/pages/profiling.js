// Profiling page JavaScript
class ProfilingPage {
    constructor() {
        this.activeSessions = new Map();
        this.pollingInterval = null;
        this.init();
    }

    init() {
        this.bindEvents();
        this.loadMemoryStats();
        this.startSessionPolling();
    }

    bindEvents() {
        // CPU Profile Form
        document.getElementById('cpuProfileForm').addEventListener('submit', (e) => {
            e.preventDefault();
            this.startCPUProfile();
        });

        // Heap Profile Button
        document.getElementById('heapProfileBtn').addEventListener('click', () => {
            this.captureHeapProfile();
        });

        // Goroutine Profile Button
        document.getElementById('goroutineProfileBtn').addEventListener('click', () => {
            this.captureGoroutineProfile();
        });

        // Refresh Memory Stats
        document.getElementById('refreshMemoryStats').addEventListener('click', () => {
            this.loadMemoryStats();
        });

        // Flamegraph controls
        document.getElementById('closeFlamegraph').addEventListener('click', () => {
            this.hideFlamegraph();
        });

        document.getElementById('zoomInFlamegraph').addEventListener('click', () => {
            this.zoomFlamegraph(1.2);
        });

        document.getElementById('zoomOutFlamegraph').addEventListener('click', () => {
            this.zoomFlamegraph(0.8);
        });

        document.getElementById('resetFlamegraph').addEventListener('click', () => {
            this.resetFlamegraph();
        });

        document.getElementById('downloadFlamegraph').addEventListener('click', () => {
            this.downloadFlamegraph();
        });
    }

    async startCPUProfile() {
        const duration = document.getElementById('duration').value;
        
        try {
            const response = await LogLynxAPI.get(`/profiling/cpu/start?duration=${encodeURIComponent(duration)}`);
            
            if (response.success) {
                this.showNotification('CPU profiling started successfully', 'success');
                this.activeSessions.set(response.data.session_id, response.data);
                this.updateSessionsList();
            } else {
                this.showNotification(response.error || 'Failed to start CPU profiling', 'error');
            }
        } catch (error) {
            this.showNotification('Failed to start CPU profiling: ' + error.message, 'error');
        }
    }

    async captureHeapProfile() {
        try {
            // Trigger download
            window.location.href = '/api/v1/profiling/heap';
            this.showNotification('Heap profile download started', 'success');
        } catch (error) {
            this.showNotification('Failed to capture heap profile: ' + error.message, 'error');
        }
    }

    async captureGoroutineProfile() {
        try {
            // Trigger download
            window.location.href = '/api/v1/profiling/goroutine';
            this.showNotification('Goroutine profile download started', 'success');
        } catch (error) {
            this.showNotification('Failed to capture goroutine profile: ' + error.message, 'error');
        }
    }

    async loadMemoryStats() {
        try {
            const response = await LogLynxAPI.get('/profiling/memory');
            
            if (response.success) {
                this.renderMemoryStats(response.data);
            } else {
                this.showNotification('Failed to load memory stats', 'error');
            }
        } catch (error) {
            this.showNotification('Failed to load memory stats: ' + error.message, 'error');
        }
    }

    renderMemoryStats(stats) {
        const container = document.getElementById('memoryStats');
        
        const statsToShow = [
            { label: 'Heap Alloc', value: this.formatBytes(stats.heap_alloc), description: 'Current heap allocation' },
            { label: 'Heap Objects', value: stats.heap_objects.toLocaleString(), description: 'Number of heap objects' },
            { label: 'Stack In Use', value: this.formatBytes(stats.stack_in_use), description: 'Stack memory in use' },
            { label: 'Goroutines', value: stats.num_goroutines.toLocaleString(), description: 'Active goroutines' },
            { label: 'GC Cycles', value: stats.num_gc.toLocaleString(), description: 'Garbage collection cycles' },
            { label: 'Total Alloc', value: this.formatBytes(stats.total_alloc), description: 'Total bytes allocated' }
        ];

        container.innerHTML = statsToShow.map(stat => `
            <div class="stat-card">
                <div class="stat-value">${stat.value}</div>
                <div class="stat-label">${stat.label}</div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: ${this.calculateProgress(stats, stat.label)}%"></div>
                </div>
            </div>
        `).join('');
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    calculateProgress(stats, statLabel) {
        // Simple progress calculation for visualization
        const maxValues = {
            'Heap Alloc': 100 * 1024 * 1024, // 100MB
            'Heap Objects': 100000,
            'Stack In Use': 10 * 1024 * 1024, // 10MB
            'Goroutines': 1000,
            'GC Cycles': 100,
            'Total Alloc': 1 * 1024 * 1024 * 1024 // 1GB
        };

        const value = stats[statLabel.toLowerCase().replace(' ', '_')];
        const max = maxValues[statLabel] || 100;
        return Math.min((value / max) * 100, 100);
    }

    startSessionPolling() {
        this.pollingInterval = setInterval(() => {
            this.updateSessionsStatus();
        }, 2000); // Poll every 2 seconds
    }

    async updateSessionsStatus() {
        for (const [sessionId, session] of this.activeSessions) {
            if (!session.done) {
                try {
                    const response = await LogLynxAPI.get(`/profiling/cpu/status/${sessionId}`);
                    if (response.success) {
                        this.activeSessions.set(sessionId, response.data);
                        this.updateSessionsList();
                    }
                } catch (error) {
                    console.error('Failed to update session status:', error);
                }
            }
        }
    }

    updateSessionsList() {
        const container = document.getElementById('activeSessions');
        
        if (this.activeSessions.size === 0) {
            container.innerHTML = `
                <div class="session-item">
                    <div class="session-info">
                        <span class="session-id">No active sessions</span>
                        <span class="session-details">Start a profiling session to see details here</span>
                    </div>
                </div>
            `;
            return;
        }

        container.innerHTML = Array.from(this.activeSessions.entries()).map(([sessionId, session]) => {
            const statusClass = session.done ? 
                (session.error ? 'status-failed' : 'status-completed') : 
                'status-running';
            
            const statusText = session.done ? 
                (session.error ? 'Failed' : 'Completed') : 
                'Running';

            const progress = session.progress ? Math.round(session.progress * 100) : 0;

            return `
                <div class="session-item">
                    <div class="session-info">
                        <span class="session-id">${session.type.toUpperCase()} Profile - ${sessionId}</span>
                        <span class="session-details">
                            Duration: ${session.duration} | 
                            Started: ${new Date(session.started_at).toLocaleTimeString()} |
                            ${session.done ? '' : `Progress: ${progress}%`}
                        </span>
                    </div>
                    <div class="session-actions">
                        <span class="status-indicator ${statusClass}">
                            <i class="fas ${session.done ? (session.error ? 'fa-times' : 'fa-check') : 'fa-circle-notch fa-spin'}"></i>
                            ${statusText}
                        </span>
                        ${session.done && !session.error ? `
                            <button class="btn btn-primary btn-sm" onclick="profilingPage.downloadProfile('${sessionId}')">
                                <i class="fas fa-download"></i>
                                Download
                            </button>
                            <button class="btn btn-secondary btn-sm" onclick="profilingPage.viewFlamegraph('${sessionId}')">
                                <i class="fas fa-fire"></i>
                                View Flame Graph
                            </button>
                        ` : ''}
                        ${!session.done ? `
                            <button class="btn btn-danger btn-sm" onclick="profilingPage.cancelSession('${sessionId}')">
                                <i class="fas fa-times"></i>
                                Cancel
                            </button>
                        ` : ''}
                    </div>
                </div>
            `;
        }).join('');
    }

    async downloadProfile(sessionId) {
        try {
            window.location.href = `/api/v1/profiling/cpu/download/${sessionId}`;
        } catch (error) {
            this.showNotification('Failed to download profile: ' + error.message, 'error');
        }
    }

    async cancelSession(sessionId) {
        // Note: Currently we can't cancel running CPU profiles in Go, but we can remove from UI
        this.activeSessions.delete(sessionId);
        this.updateSessionsList();
        this.showNotification('Session removed from UI (CPU profiling cannot be cancelled once started)', 'warning');
    }

    async viewFlamegraph(sessionId) {
        try {
            this.showNotification('Generating flame graph...', 'info');
            
            // Download the profile data
            const response = await fetch(`/api/v1/profiling/cpu/download/${sessionId}`);
            if (!response.ok) {
                throw new Error(`Failed to download profile: ${response.status}`);
            }
            
            const profileData = await response.arrayBuffer();
            
            // Convert to base64 for the flame graph library
            const base64Data = btoa(new Uint8Array(profileData).reduce(
                (data, byte) => data + String.fromCharCode(byte), ''
            ));
            
            // Show flamegraph section
            this.showFlamegraph(base64Data);
            
        } catch (error) {
            this.showNotification('Failed to generate flame graph: ' + error.message, 'error');
        }
    }

    showFlamegraph(profileData) {
        const flamegraphSection = document.getElementById('flamegraphSection');
        const flamegraphViewer = document.getElementById('flamegraphViewer');
        
        // Show the section
        flamegraphSection.style.display = 'block';
        
        // Clear previous flamegraph
        flamegraphViewer.innerHTML = '';
        
        // Create flamegraph container
        const container = document.createElement('div');
        container.style.width = '100%';
        container.style.height = '100%';
        flamegraphViewer.appendChild(container);
        
        // Simple flamegraph implementation using d3-flamegraph
        this.renderSimpleFlamegraph(container, profileData);
        
        // Scroll to flamegraph section
        flamegraphSection.scrollIntoView({ behavior: 'smooth' });
    }

    renderSimpleFlamegraph(container, profileData) {
        // Clear container and show loading message
        container.innerHTML = `
            <div style="text-align: center; padding: 40px; color: var(--text-secondary);">
                <i class="fas fa-spinner fa-spin" style="font-size: 3rem; margin-bottom: 20px;"></i>
                <p>Processing profile data and generating flame graph...</p>
            </div>
        `;

        // Process the profile data and generate flame graph
        setTimeout(() => {
            this.generateFlameGraph(container, profileData);
        }, 100);
    }

    async generateFlameGraph(container, base64Data) {
        try {
            // Convert base64 back to binary
            const binaryData = atob(base64Data);
            const bytes = new Uint8Array(binaryData.length);
            for (let i = 0; i < binaryData.length; i++) {
                bytes[i] = binaryData.charCodeAt(i);
            }

            // Create a blob and object URL
            const blob = new Blob([bytes], { type: 'application/octet-stream' });
            const profileUrl = URL.createObjectURL(blob);

            // Use simple flame graph implementation since we can't parse pprof in browser
            this.renderBasicFlameGraph(container, profileUrl);
            
        } catch (error) {
            console.error('Failed to generate flame graph:', error);
            container.innerHTML = `
                <div style="text-align: center; padding: 40px; color: var(--danger-color);">
                    <i class="fas fa-exclamation-triangle" style="font-size: 3rem; margin-bottom: 20px;"></i>
                    <h3>Flame Graph Generation Failed</h3>
                    <p>Error: ${error.message}</p>
                    <p>You can still download the profile and use:</p>
                    <code style="background: var(--card-bg-secondary); padding: 10px; border-radius: 4px; display: block; margin: 10px 0;">
                        go tool pprof -http=:8081 path/to/profile.pprof
                    </code>
                    <button class="btn btn-primary" onclick="profilingPage.downloadFlamegraphData('${base64Data}')">
                        <i class="fas fa-download"></i>
                        Download Profile
                    </button>
                </div>
            `;
        }
    }

    renderBasicFlameGraph(container, profileUrl) {
        // Simple visualization since pprof parsing in browser is complex
        container.innerHTML = `
            <div style="text-align: center; padding: 20px;">
                <h3 style="color: var(--text-primary); margin-bottom: 20px;">
                    <i class="fas fa-fire"></i>
                    CPU Profile Visualization
                </h3>
                
                <div style="background: var(--card-bg-secondary); padding: 20px; border-radius: 8px; margin-bottom: 20px;">
                    <p style="color: var(--text-secondary); margin-bottom: 15px;">
                        For detailed flame graph analysis, download the profile and use:
                    </p>
                    <code style="background: var(--card-bg); padding: 10px; border-radius: 4px; display: block; margin: 10px 0; font-family: monospace;">
                        go tool pprof -http=:8081 downloaded_profile.pprof
                    </code>
                </div>

                <div style="background: var(--card-bg); padding: 20px; border-radius: 8px; margin-bottom: 20px; border-left: 4px solid var(--warning-color);">
                    <h4 style="color: var(--warning-color); margin-bottom: 15px;">
                        <i class="fas fa-exclamation-triangle"></i>
                        Important: Install Graphviz
                    </h4>
                    <p style="color: var(--text-secondary); margin-bottom: 15px;">
                        For visualizations, you need Graphviz installed. Download from:
                    </p>
                    <a href="https://graphviz.org/download/" target="_blank" class="btn btn-warning" style="margin-bottom: 15px;">
                        <i class="fas fa-download"></i>
                        Download Graphviz
                    </a>
                    <p style="color: var(--text-secondary); font-size: 0.9em;">
                        Or use: <code>winget install graphviz</code> on Windows
                    </p>
                </div>

                <div style="display: flex; gap: 15px; justify-content: center; flex-wrap: wrap;">
                    <div style="flex: 1; min-width: 300px; background: var(--card-bg-secondary); padding: 20px; border-radius: 8px;">
                        <h4 style="color: var(--text-primary); margin-bottom: 15px;">
                            <i class="fas fa-download"></i>
                            Quick Analysis
                        </h4>
                        <p style="color: var(--text-secondary); margin-bottom: 15px;">
                            Download the profile and analyze with Go tools.
                        </p>
                        <button class="btn btn-primary" style="width: 100%; margin-bottom: 10px;" onclick="window.open('${profileUrl}')">
                            <i class="fas fa-external-link-alt"></i>
                            Download Profile
                        </button>
                    </div>

                    <div style="flex: 1; min-width: 300px; background: var(--card-bg-secondary); padding: 20px; border-radius: 8px;">
                        <h4 style="color: var(--text-primary); margin-bottom: 15px;">
                            <i class="fas fa-terminal"></i>
                            Command Line (No Graphviz)
                        </h4>
                        <p style="color: var(--text-secondary); margin-bottom: 15px;">
                            Text-based analysis without visuals:
                        </p>
                        <div style="text-align: left; font-family: monospace; font-size: 0.9em; color: var(--text-secondary);">
                            <div style="margin-bottom: 8px;">$ go tool pprof -text profile.pprof</div>
                            <div style="margin-bottom: 8px;">$ go tool pprof -top profile.pprof</div>
                            <div>$ go tool pprof -list=main.profile.pprof</div>
                        </div>
                    </div>

                    <div style="flex: 1; min-width: 300px; background: var(--card-bg-secondary); padding: 20px; border-radius: 8px;">
                        <h4 style="color: var(--text-primary); margin-bottom: 15px;">
                            <i class="fas fa-desktop"></i>
                            With Graphviz
                        </h4>
                        <p style="color: var(--text-secondary); margin-bottom: 15px;">
                            Visual analysis (requires Graphviz):
                        </p>
                        <div style="text-align: left; font-family: monospace; font-size: 0.9em; color: var(--text-secondary);">
                            <div style="margin-bottom: 8px;">$ go tool pprof -web profile.pprof</div>
                            <div>$ go tool pprof -http=:8081 profile.pprof</div>
                        </div>
                    </div>
                </div>

                <div style="margin-top: 20px;">
                    <button class="btn btn-secondary" onclick="profilingPage.showSampleFlamegraph()">
                        <i class="fas fa-eye"></i>
                        Show Sample Flame Graph
                    </button>
                </div>
            </div>
        `;
    }

    showSampleFlamegraph() {
        // Create a simple sample visualization
        const flamegraphViewer = document.getElementById('flamegraphViewer');
        flamegraphViewer.innerHTML = `
            <div style="text-align: center; padding: 30px; color: var(--text-secondary);">
                <i class="fas fa-info-circle" style="font-size: 2rem; margin-bottom: 15px;"></i>
                <h3>Sample Flame Graph</h3>
                <p>This would show an interactive flame graph visualization if we had proper pprof parsing in the browser.</p>
                <div style="background: linear-gradient(90deg, #ff6b6b, #4ecdc4, #45b7d1);
                            height: 200px; margin: 20px 0; border-radius: 4px; position: relative;">
                    <div style="position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%);
                                color: white; font-weight: bold;">
                        Interactive Flame Graph Visualization
                    </div>
                </div>
                <p>For real flame graphs, use the download option and analyze with Go tools.</p>
            </div>
        `;
    }

    hideFlamegraph() {
        document.getElementById('flamegraphSection').style.display = 'none';
    }

    zoomFlamegraph(factor) {
        // Simple zoom effect for the sample visualization
        const flamegraphViewer = document.getElementById('flamegraphViewer');
        const content = flamegraphViewer.querySelector('div');
        if (content) {
            const currentScale = parseFloat(content.style.transform?.match(/scale\(([^)]+)\)/)?.[1] || 1);
            content.style.transform = `scale(${currentScale * factor})`;
            content.style.transformOrigin = 'center center';
        }
    }

    resetFlamegraph() {
        // Reset zoom for the sample visualization
        const flamegraphViewer = document.getElementById('flamegraphViewer');
        const content = flamegraphViewer.querySelector('div');
        if (content) {
            content.style.transform = 'scale(1)';
        }
    }

    downloadFlamegraph() {
        // Download the current profile data
        const flamegraphSection = document.getElementById('flamegraphSection');
        const downloadBtn = flamegraphSection.querySelector('#downloadFlamegraph');
        const sessionId = downloadBtn.getAttribute('data-session-id');
        
        if (sessionId) {
            this.downloadProfile(sessionId);
        } else {
            this.showNotification('No profile data available for download', 'warning');
        }
    }

    downloadFlamegraphData(base64Data) {
        try {
            const binaryData = atob(base64Data);
            const bytes = new Uint8Array(binaryData.length);
            for (let i = 0; i < binaryData.length; i++) {
                bytes[i] = binaryData.charCodeAt(i);
            }
            
            const blob = new Blob([bytes], { type: 'application/octet-stream' });
            const url = URL.createObjectURL(blob);
            
            const a = document.createElement('a');
            a.href = url;
            a.download = `profile_${Date.now()}.pprof`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            
            this.showNotification('Profile downloaded successfully', 'success');
        } catch (error) {
            this.showNotification('Failed to download profile: ' + error.message, 'error');
        }
    }

    showNotification(message, type = 'info') {
        // Simple notification implementation
        const notification = document.createElement('div');
        notification.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 12px 20px;
            border-radius: 6px;
            color: white;
            font-weight: 500;
            z-index: 1000;
            opacity: 0;
            transform: translateX(100%);
            transition: all 0.3s ease;
        `;

        const colors = {
            success: '#10b981',
            error: '#ef4444',
            warning: '#f59e0b',
            info: '#3b82f6'
        };

        notification.style.background = colors[type] || colors.info;
        notification.textContent = message;

        document.body.appendChild(notification);

        // Animate in
        setTimeout(() => {
            notification.style.opacity = '1';
            notification.style.transform = 'translateX(0)';
        }, 100);

        // Auto remove after 5 seconds
        setTimeout(() => {
            notification.style.opacity = '0';
            notification.style.transform = 'translateX(100%)';
            setTimeout(() => notification.remove(), 300);
        }, 5000);
    }
}

// Initialize profiling page when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    window.profilingPage = new ProfilingPage();
});