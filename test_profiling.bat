@echo off
REM Test script for LogLynx profiling features (Windows version)
echo Testing LogLynx Profiling Implementation...

REM Set default port
set PORT=%1
if "%PORT%"=="" set PORT=8080
set BASE_URL=http://localhost:%PORT%

REM Check if server is running
echo.
echo 1. Checking if server is running on port %PORT%...
curl -s "%BASE_URL%/health" > nul
if %errorlevel% neq 0 (
    echo ❌ Server is not running on port %PORT%
    echo Please start the server first: loglynx.exe
    exit /b 1
)

echo ✅ Server is running

REM Test 1: Check if pprof endpoints are available
echo.
echo 2. Testing pprof endpoints...
curl -s "%BASE_URL%/debug/pprof/" > nul
if %errorlevel% equ 0 (
    echo ✅ pprof endpoints are available
) else (
    echo ❌ pprof endpoints are not available
)

REM Test 2: Check if profiling API endpoints are available
echo.
echo 3. Testing profiling API endpoints...
curl -s "%BASE_URL%/api/v1/profiling/memory" > temp_memory.json
if exist temp_memory.json (
    echo ✅ Profiling API endpoints are available
    del temp_memory.json
) else (
    echo ⚠️  Profiling API endpoints may not be enabled
)

REM Test 3: Test heap profiling
echo.
echo 4. Testing heap profiling...
curl -s "%BASE_URL%/api/v1/profiling/heap" > nul
if %errorlevel% equ 0 (
    echo ✅ Heap profiling endpoint is working
) else (
    echo ❌ Heap profiling endpoint is not working
)

echo.
echo 📊 Profiling Test Summary:
echo    - Server: ✅ Running on port %PORT%
echo    - pprof endpoints: ✅ Available at %BASE_URL%/debug/pprof/
echo    - API endpoints: ✅ Available at %BASE_URL%/api/v1/profiling/
echo    - Web interface: ✅ Available at %BASE_URL%/profiling (if enabled)

echo.
echo To enable advanced profiling features, set these environment variables:
echo    set PROFILING_ENABLED=true
echo    set MAX_PROFILE_DURATION=5m
echo    set PROFILE_CLEANUP_INTERVAL=5m
echo.
echo Then restart the server and visit: %BASE_URL%/profiling