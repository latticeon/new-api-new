@echo off
REM ============================================================
REM  new-api build script (Windows)
REM  1. Clean old frontend build output (web\dist)
REM  2. Build frontend with bun
REM  3. Cross-compile linux amd64 binary -> build\new-api-linux-amd64
REM  4. Zip it -> build\new-api-linux-amd64-<version>.zip
REM     version = git describe --tags, e.g. v0.11.7-11-g0195d8e8
REM ============================================================
setlocal
cd /d "%~dp0"

set "BIN_NAME=new-api-linux-amd64"
set "BUILD_DIR=build"

echo.
echo ==============================================
echo  [1/5] Checking required tools ...
echo ==============================================
where git >nul 2>nul
if errorlevel 1 (
    echo ERROR: git not found in PATH
    goto :end
)
where bun >nul 2>nul
if errorlevel 1 (
    echo ERROR: bun not found in PATH
    goto :end
)
where go >nul 2>nul
if errorlevel 1 (
    echo ERROR: go not found in PATH
    goto :end
)
where powershell >nul 2>nul
if errorlevel 1 (
    echo ERROR: powershell not found in PATH
    goto :end
)

echo.
echo ==============================================
echo  [2/5] Resolving version ...
echo ==============================================
set "VER="
for /f "delims=" %%i in ('git describe --tags 2^>nul') do set "VER=%%i"
if not defined VER (
    for /f "delims=" %%i in ('git rev-parse --short HEAD 2^>nul') do set "VER=%%i"
)
if not defined VER set "VER=dev"
echo Version: %VER%

echo.
echo ==============================================
echo  [3/5] Cleaning old frontend build output ...
echo ==============================================
if exist "web\dist" rmdir /s /q "web\dist"
if exist "web\dist" (
    echo ERROR: failed to delete web\dist, close programs locking it and retry
    goto :end
)

echo.
echo ==============================================
echo  [4/5] Building frontend with bun ...
echo ==============================================
pushd web
set "DISABLE_ESLINT_PLUGIN=true"
set "VITE_REACT_APP_VERSION=%VER%"
call bun install
if errorlevel 1 (
    popd
    echo ERROR: bun install failed
    goto :end
)
call bun run build
if errorlevel 1 (
    popd
    echo ERROR: frontend build failed
    goto :end
)
popd
if not exist "web\dist\index.html" (
    echo ERROR: web\dist\index.html not found, frontend build output is incomplete
    goto :end
)

echo.
echo ==============================================
echo  [5/5] Building linux amd64 binary ...
echo ==============================================
if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"
set "CGO_ENABLED=0"
set "GOOS=linux"
set "GOARCH=amd64"
go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=%VER%'" -o "%BUILD_DIR%\%BIN_NAME%"
if errorlevel 1 (
    echo ERROR: go build failed
    goto :end
)

echo.
echo ==============================================
echo  Packaging zip archive ...
echo ==============================================
powershell -NoProfile -ExecutionPolicy Bypass -Command "Compress-Archive -Path '%BUILD_DIR%\%BIN_NAME%' -DestinationPath '%BUILD_DIR%\%BIN_NAME%-%VER%.zip' -Force"
if errorlevel 1 (
    echo ERROR: failed to create zip archive
    goto :end
)

echo.
echo ==============================================
echo  BUILD SUCCESS
echo  Binary : %BUILD_DIR%\%BIN_NAME%
echo  Zip    : %BUILD_DIR%\%BIN_NAME%-%VER%.zip
echo ==============================================

:end
echo.
echo Finished. Press any key to close this window ...
pause >nul
endlocal
