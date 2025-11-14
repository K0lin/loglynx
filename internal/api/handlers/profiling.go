package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
)

// ProfilingHandler handles performance profiling endpoints
type ProfilingHandler struct {
	logger           *pterm.Logger
	cfg              *ProfilingConfig
	// Active profiling sessions
	activeProfiles   map[string]*profileSession
	activeProfilesMu sync.Mutex
}

// ProfilingConfig contains profiling configuration
type ProfilingConfig struct {
	Enabled           bool
	MaxProfileDuration time.Duration
	CleanupInterval   time.Duration
}

// IsEnabled returns true if profiling is enabled
func (h *ProfilingHandler) IsEnabled() bool {
	return h.cfg.Enabled
}

// profileSession represents an active profiling session
type profileSession struct {
	ID        string
	Type      string
	Duration  time.Duration
	StartedAt time.Time
	Data      []byte
	Error     error
	Done      bool
}

// NewProfilingHandler creates a new profiling handler
func NewProfilingHandler(cfg *ProfilingConfig, logger *pterm.Logger) *ProfilingHandler {
	handler := &ProfilingHandler{
		logger:         logger,
		cfg:            cfg,
		activeProfiles: make(map[string]*profileSession),
	}
	
	// Start cleanup routine if profiling is enabled
	if cfg.Enabled {
		go handler.CleanupOldProfiles()
	}
	
	return handler
}

// StartCPUProfile starts a CPU profiling session
func (h *ProfilingHandler) StartCPUProfile(c *gin.Context) {
	durationStr := c.DefaultQuery("duration", "30s")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid duration: %v", err),
		})
		return
	}

	if duration > h.cfg.MaxProfileDuration {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Maximum profiling duration is %s", h.cfg.MaxProfileDuration),
		})
		return
	}

	sessionID := fmt.Sprintf("cpu_%d", time.Now().UnixNano())
	session := &profileSession{
		ID:        sessionID,
		Type:      "cpu",
		Duration:  duration,
		StartedAt: time.Now(),
	}

	h.activeProfilesMu.Lock()
	h.activeProfiles[sessionID] = session
	h.activeProfilesMu.Unlock()

	// Start profiling in background
	go h.runCPUProfile(session)

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"duration":   duration.String(),
		"started_at": session.StartedAt.Format(time.RFC3339),
		"status":     "started",
	})
}

// runCPUProfile runs the CPU profiling for the specified duration
func (h *ProfilingHandler) runCPUProfile(session *profileSession) {
	var buf bytes.Buffer

	// Start CPU profiling
	if err := pprof.StartCPUProfile(&buf); err != nil {
		session.Error = err
		session.Done = true
		return
	}

	// Wait for the specified duration
	time.Sleep(session.Duration)

	// Stop profiling
	pprof.StopCPUProfile()

	// Store the profile data
	session.Data = buf.Bytes()
	session.Done = true

	h.logger.Info("CPU profiling completed",
		h.logger.Args("session_id", session.ID, "duration", session.Duration))
}

// GetProfileStatus returns the status of a profiling session
func (h *ProfilingHandler) GetProfileStatus(c *gin.Context) {
	sessionID := c.Param("session_id")

	h.activeProfilesMu.Lock()
	session, exists := h.activeProfiles[sessionID]
	h.activeProfilesMu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Profile session not found",
		})
		return
	}

	status := gin.H{
		"session_id": session.ID,
		"type":       session.Type,
		"duration":   session.Duration.String(),
		"started_at": session.StartedAt.Format(time.RFC3339),
		"done":       session.Done,
	}

	if session.Done {
		if session.Error != nil {
			status["error"] = session.Error.Error()
			status["status"] = "failed"
		} else {
			status["status"] = "completed"
			status["data_size"] = len(session.Data)
		}
	} else {
		elapsed := time.Since(session.StartedAt)
		status["elapsed"] = elapsed.String()
		status["remaining"] = (session.Duration - elapsed).String()
		status["progress"] = float64(elapsed) / float64(session.Duration)
		status["status"] = "running"
	}

	c.JSON(http.StatusOK, status)
}

// DownloadProfile downloads the completed profile data
func (h *ProfilingHandler) DownloadProfile(c *gin.Context) {
	sessionID := c.Param("session_id")

	h.activeProfilesMu.Lock()
	session, exists := h.activeProfiles[sessionID]
	h.activeProfilesMu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Profile session not found",
		})
		return
	}

	if !session.Done {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Profile session not completed yet",
		})
		return
	}

	if session.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": session.Error.Error(),
		})
		return
	}

	// Set appropriate headers for download
	filename := fmt.Sprintf("%s_%s.pprof", session.Type, session.StartedAt.Format("20060102_150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Data(http.StatusOK, "application/octet-stream", session.Data)
}

// HeapProfile captures and returns a heap profile
func (h *ProfilingHandler) HeapProfile(c *gin.Context) {
	var buf bytes.Buffer

	// Write heap profile
	if err := pprof.WriteHeapProfile(&buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to capture heap profile: %v", err),
		})
		return
	}

	// Set appropriate headers for download
	filename := fmt.Sprintf("heap_%s.pprof", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Data(http.StatusOK, "application/octet-stream", buf.Bytes())
}

// GoroutineProfile captures and returns a goroutine profile
func (h *ProfilingHandler) GoroutineProfile(c *gin.Context) {
	var buf bytes.Buffer

	// Get goroutine profile
	profile := pprof.Lookup("goroutine")
	if profile == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get goroutine profile",
		})
		return
	}

	if err := profile.WriteTo(&buf, 1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to write goroutine profile: %v", err),
		})
		return
	}

	// Set appropriate headers for download
	filename := fmt.Sprintf("goroutine_%s.pprof", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Data(http.StatusOK, "application/octet-stream", buf.Bytes())
}

// MemoryStats returns current memory statistics
func (h *ProfilingHandler) MemoryStats(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := gin.H{
		"alloc":           m.Alloc,
		"total_alloc":     m.TotalAlloc,
		"sys":             m.Sys,
		"lookups":         m.Lookups,
		"mallocs":         m.Mallocs,
		"frees":           m.Frees,
		"heap_alloc":      m.HeapAlloc,
		"heap_sys":        m.HeapSys,
		"heap_idle":       m.HeapIdle,
		"heap_in_use":     m.HeapInuse,
		"heap_released":   m.HeapReleased,
		"heap_objects":    m.HeapObjects,
		"stack_in_use":    m.StackInuse,
		"stack_sys":       m.StackSys,
		"next_gc":         m.NextGC,
		"last_gc":         m.LastGC,
		"pause_total_ns":  m.PauseTotalNs,
		"num_gc":          m.NumGC,
		"gc_cpu_fraction": m.GCCPUFraction,
		"num_goroutines":  runtime.NumGoroutine(),
	}

	c.JSON(http.StatusOK, stats)
}

// CleanupOldProfiles periodically cleans up old profile sessions
func (h *ProfilingHandler) CleanupOldProfiles() {
	ticker := time.NewTicker(h.cfg.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		h.activeProfilesMu.Lock()
		now := time.Now()
		for id, session := range h.activeProfiles {
			// Remove sessions older than 1 hour
			if now.Sub(session.StartedAt) > time.Hour {
				delete(h.activeProfiles, id)
				h.logger.Debug("Cleaned up old profile session",
					h.logger.Args("session_id", id, "age", now.Sub(session.StartedAt)))
			}
		}
		h.activeProfilesMu.Unlock()
	}
}