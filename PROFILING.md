# LogLynx Performance Profiling Guide

This document describes the performance profiling capabilities added to LogLynx for identifying and diagnosing performance bottlenecks.

## Overview

LogLynx now includes comprehensive performance profiling features that allow you to:

- Capture CPU profiles to identify hotspots
- Analyze memory usage and heap allocations
- Monitor goroutine activity
- Access real-time memory statistics
- Generate flame graphs for visual analysis

## Configuration

Profiling can be configured using environment variables:

```bash
# Enable advanced profiling features (default: false)
export PROFILING_ENABLED=true

# Maximum duration for CPU profiles (default: 5m)
export MAX_PROFILE_DURATION=5m

# How often to clean up old profile sessions (default: 5m)
export PROFILE_CLEANUP_INTERVAL=5m
```

## Available Endpoints

### Standard pprof Endpoints

These endpoints are available in non-production mode or when profiling is explicitly enabled:

- `GET /debug/pprof/` - pprof main page
- `GET /debug/pprof/profile` - CPU profile (30s)
- `GET /debug/pprof/heap` - Heap profile
- `GET /debug/pprof/goroutine` - Goroutine profile
- `GET /debug/pprof/block` - Blocking profile
- `GET /debug/pprof/threadcreate` - Thread creation profile

### Advanced Profiling API

These endpoints are available when `PROFILING_ENABLED=true`:

#### CPU Profiling
- `GET /api/v1/profiling/cpu/start?duration=30s` - Start CPU profiling session
- `GET /api/v1/profiling/cpu/status/:session_id` - Check session status
- `GET /api/v1/profiling/cpu/download/:session_id` - Download profile data

#### Memory Profiling
- `GET /api/v1/profiling/heap` - Download heap profile
- `GET /api/v1/profiling/goroutine` - Download goroutine profile
- `GET /api/v1/profiling/memory` - Get memory statistics

## Web Interface

When profiling is enabled, a web interface is available at `/profiling` that provides:

- **CPU Profiling Controls**: Start and monitor CPU profiling sessions
- **Memory Statistics**: Real-time memory usage metrics
- **Profile Downloads**: One-click downloads of heap and goroutine profiles
- **Session Management**: View and manage active profiling sessions

## Graphviz Installation

For visual flame graphs and other visualizations, you need Graphviz installed:

**Windows:**
```bash
winget install graphviz
```
Or download from: https://graphviz.org/download/

**macOS:**
```bash
brew install graphviz
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install graphviz
```

## Usage Examples

### 1. Starting a CPU Profile

```bash
# Start a 30-second CPU profile
curl "http://localhost:8080/api/v1/profiling/cpu/start?duration=30s"

# Response: {"session_id":"cpu_123456789","duration":"30s","started_at":"2024-01-01T12:00:00Z","status":"started"}
```

### 2. Checking Profile Status

```bash
# Check status of a profiling session
curl "http://localhost:8080/api/v1/profiling/cpu/status/cpu_123456789"

# Response: {"session_id":"cpu_123456789","type":"cpu","duration":"30s",...}
```

### 3. Downloading Profiles

```bash
# Download CPU profile
curl -o profile.pprof "http://localhost:8080/api/v1/profiling/cpu/download/cpu_123456789"

# Download heap profile
curl -o heap.pprof "http://localhost:8080/api/v1/profiling/heap"

# Download goroutine profile  
curl -o goroutine.pprof "http://localhost:8080/api/v1/profiling/goroutine"
```

### 4. Analyzing Profiles

Use the Go pprof tool to analyze the downloaded profiles:

**Without Graphviz (text-only analysis):**
```bash
# Top functions by CPU usage
go tool pprof -top profile.pprof

# Text output
go tool pprof -text profile.pprof

# List specific function
go tool pprof -list=function_name profile.pprof
```

**With Graphviz (visual analysis):**
```bash
# Web interface (requires Graphviz)
go tool pprof -http=:8081 profile.pprof

# Open in default browser (requires Graphviz)
go tool pprof -web profile.pprof

# Analyze heap profile
go tool pprof -http=:8081 heap.pprof

# Analyze goroutine profile
go tool pprof -http=:8081 goroutine.pprof
```

## Real-time Memory Statistics

The memory statistics endpoint provides detailed information about Go runtime memory usage:

```bash
curl "http://localhost:8080/api/v1/profiling/memory"

# Response includes:
# - Heap allocation statistics
# - Garbage collection metrics  
# - Goroutine count
# - Stack memory usage
```

## Best Practices

1. **Use in Development/Staging**: Enable profiling in non-production environments
2. **Limit Duration**: Keep CPU profiles short (30s-2m) to avoid performance impact
3. **Monitor Memory**: Use heap profiles to identify memory leaks
4. **Analyze Goroutines**: Check for goroutine leaks with goroutine profiles
5. **Use Web Interface**: The web UI provides the easiest way to manage profiling sessions

## Troubleshooting

### Profiling Endpoints Not Available
- Check that `PROFILING_ENABLED=true` is set
- Ensure you're not in production mode (or explicitly enable profiling)

### CPU Profiling Fails
- The maximum duration is 5 minutes by default
- Only one CPU profile can run at a time

### Memory Issues
- Large profiles may take time to generate
- Consider increasing memory limits if profiling large applications

## Security Considerations

- Profiling endpoints should be protected in production environments
- Consider using authentication/authorization for profiling endpoints
- Profile data may contain sensitive information about application internals

## Performance Impact

- CPU profiling has minimal overhead (1-5% typically)
- Memory profiling has negligible overhead
- Web interface adds minimal additional load

## Related Tools

- **go tool pprof**: Standard Go profiling tool
- **pprof**: Web-based profile visualization
- **FlameGraph**: Generate flame graphs from pprof data
- **Grafana**: Can visualize memory statistics

## Support

For issues with profiling features, check:
1. Server logs for error messages
2. Environment variable configuration
3. Available system resources
4. Network connectivity to profiling endpoints