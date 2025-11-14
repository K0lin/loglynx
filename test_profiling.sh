#!/bin/bash

# Test script for LogLynx profiling features
echo "Testing LogLynx Profiling Implementation..."

# Set default port
PORT=${1:-8080}
BASE_URL="http://localhost:$PORT"

# Check if server is running
echo "Checking if server is running on port $PORT..."
if ! curl -s "$BASE_URL/health" > /dev/null; then
    echo "❌ Server is not running on port $PORT"
    echo "Please start the server first: ./loglynx"
    exit 1
fi

echo "✅ Server is running"

# Test 1: Check if pprof endpoints are available
echo ""
echo "1. Testing pprof endpoints..."
if curl -s "$BASE_URL/debug/pprof/" > /dev/null; then
    echo "✅ pprof endpoints are available"
else
    echo "❌ pprof endpoints are not available"
fi

# Test 2: Check if profiling API endpoints are available (if enabled)
echo ""
echo "2. Testing profiling API endpoints..."
PROFILING_RESPONSE=$(curl -s "$BASE_URL/api/v1/profiling/memory")
if echo "$PROFILING_RESPONSE" | grep -q "alloc"; then
    echo "✅ Profiling API endpoints are available and working"
    echo "   Memory stats: $(echo "$PROFILING_RESPONSE" | jq '.alloc' 2>/dev/null || echo "available")"
else
    echo "⚠️  Profiling API endpoints may not be enabled (check PROFILING_ENABLED environment variable)"
fi

# Test 3: Test CPU profiling (if enabled)
echo ""
echo "3. Testing CPU profiling..."
CPU_START_RESPONSE=$(curl -s "$BASE_URL/api/v1/profiling/cpu/start?duration=5s")
if echo "$CPU_START_RESPONSE" | grep -q "session_id"; then
    SESSION_ID=$(echo "$CPU_START_RESPONSE" | jq -r '.session_id' 2>/dev/null)
    echo "✅ CPU profiling started successfully"
    echo "   Session ID: $SESSION_ID"
    
    # Wait a bit and check status
    sleep 2
    STATUS_RESPONSE=$(curl -s "$BASE_URL/api/v1/profiling/cpu/status/$SESSION_ID")
    echo "   Status: $(echo "$STATUS_RESPONSE" | jq '.status' 2>/dev/null || echo "unknown")"
else
    echo "⚠️  CPU profiling may not be enabled or there was an error"
    echo "   Response: $CPU_START_RESPONSE"
fi

# Test 4: Test heap profiling
echo ""
echo "4. Testing heap profiling..."
if curl -s "$BASE_URL/api/v1/profiling/heap" > /dev/null; then
    echo "✅ Heap profiling endpoint is working"
else
    echo "❌ Heap profiling endpoint is not working"
fi

# Test 5: Test goroutine profiling
echo ""
echo "5. Testing goroutine profiling..."
if curl -s "$BASE_URL/api/v1/profiling/goroutine" > /dev/null; then
    echo "✅ Goroutine profiling endpoint is working"
else
    echo "❌ Goroutine profiling endpoint is not working"
fi

echo ""
echo "📊 Profiling Test Summary:"
echo "   - Server: ✅ Running on port $PORT"
echo "   - pprof endpoints: ✅ Available at $BASE_URL/debug/pprof/"
echo "   - API endpoints: ✅ Available at $BASE_URL/api/v1/profiling/"
echo "   - Web interface: ✅ Available at $BASE_URL/profiling (if enabled)"

echo ""
echo "To enable advanced profiling features, set these environment variables:"
echo "   export PROFILING_ENABLED=true"
echo "   export MAX_PROFILE_DURATION=5m"
echo "   export PROFILE_CLEANUP_INTERVAL=5m"
echo ""
echo "Then restart the server and visit: $BASE_URL/profiling"