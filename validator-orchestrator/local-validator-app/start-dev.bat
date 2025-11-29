@echo off
REM ============================================
REM Omniphi Local Validator - Development Mode
REM ============================================

REM Get the directory where this script is located
cd /d "%~dp0"

echo ============================================
echo   Omniphi Local Validator
echo   Development Mode (Hot Reload)
echo ============================================
echo.

REM Check if node_modules exists
if not exist "node_modules" (
    echo ERROR: node_modules not found!
    echo Please run: npm install
    echo.
    pause
    exit /b 1
)

echo Starting in development mode...
echo.
echo - Vite dev server will start on http://127.0.0.1:4200
echo - Electron will load from Vite dev server
echo - Hot reload is enabled for React components
echo.

REM Clear ELECTRON_RUN_AS_NODE to ensure Electron runs as GUI app
set ELECTRON_RUN_AS_NODE=

REM Run npm dev script
npm run dev

pause
