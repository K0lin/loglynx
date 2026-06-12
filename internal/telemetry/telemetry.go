// MIT License
//
// # Copyright (c) 2026 Kolin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// BuildEndpoint can be set at build time with:
// -ldflags "-X loglynx/internal/telemetry.BuildEndpoint=https://example.com/ping"
var BuildEndpoint string

// Config contains anonymous usage telemetry settings.
type Config struct {
	Enabled  bool
	Endpoint string
	Interval time.Duration
	StoreDir string
	Version  string
}

// Payload is the JSON sent to the configured telemetry endpoint.
type Payload struct {
	InstanceID string `json:"instance_id"`
	Version    string `json:"version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Event      string `json:"event"`
}

// Start begins non-blocking anonymous usage pings and returns a stop function.
func Start(cfg Config, logger *pterm.Logger) func() {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(BuildEndpoint)
	}

	if !cfg.Enabled {
		logger.Debug("Anonymous usage telemetry disabled")
		return func() {}
	}

	if endpoint == "" {
		logger.Debug("Anonymous usage telemetry endpoint not configured")
		return func() {}
	}

	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Scheme != "https" || parsedEndpoint.Host == "" {
		logger.Debug("Anonymous usage telemetry endpoint must be HTTPS")
		return func() {}
	}

	if cfg.Interval <= 0 {
		cfg.Interval = 24 * time.Hour
	}

	instanceID, err := getOrCreateInstanceID(cfg.StoreDir)
	if err != nil {
		logger.Debug("Anonymous usage telemetry unavailable", logger.Args("error", err))
		return func() {}
	}

	// Ensure version is not empty
	appVersion := strings.TrimSpace(cfg.Version)
	if appVersion == "" {
		appVersion = "1.1.1" // Fallback to current stable
	}

	ctx, cancel := context.WithCancel(context.Background())
	payload := Payload{
		InstanceID: hashInstanceID(instanceID),
		Version:    appVersion,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Event:      "heartbeat",
	}

	go func() {
		logger.Debug("Anonymous usage telemetry enabled", logger.Args("interval", cfg.Interval))

		// Send immediate heartbeat on startup
		ping(ctx, endpoint, payload, logger)

		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ping(ctx, endpoint, payload, logger)
			}
		}
	}()

	return cancel
}

func ping(parent context.Context, endpoint string, payload Payload, logger *pterm.Logger) bool {
	body, err := json.Marshal(payload)
	if err != nil {
		logger.Debug("Failed to encode telemetry payload", logger.Args("error", err))
		return false
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		logger.Debug("Failed to create telemetry request", logger.Args("error", err))
		return false
	}

	req.Header.Set("Content-Type", "application/json")
	// EXACT User-Agent as requested by the user
	req.Header.Set("User-Agent", "logLynx/"+payload.Version)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "close")

	resp, err := client.Do(req)
	if err != nil {
		errMsg := err.Error()
		logger.Debug("Telemetry ping failed", logger.Args("error", errMsg))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Debug("Telemetry ping returned non-success status", logger.Args("status", resp.StatusCode))
		return false
	}

	logger.Debug("Telemetry heartbeat sent successfully")
	return true
}

func getOrCreateInstanceID(storeDir string) (string, error) {
	if strings.TrimSpace(storeDir) == "" {
		storeDir = "."
	}

	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(storeDir, ".loglynx_instance_id")
	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	id, err := newRandomID()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return "", err
	}

	return id, nil
}

func newRandomID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashInstanceID(instanceID string) string {
	sum := sha256.Sum256([]byte(instanceID))
	return hex.EncodeToString(sum[:])
}
