@echo off
REM ============================================
REM Omniphi Local Validator - Desktop App Launcher
REM ============================================

REM Get the directory where this script is located
cd /d "%~dp0"

echo ============================================
echo   Omniphi Local Validator
echo   Desktop Application
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

REM Check if Electron binary exists
if not exist "node_modules\electron\dist\electron.exe" (
    echo ERROR: Electron not found!
    echo Please run: npm install
    echo.
    pause
    exit /b 1
)

echo Starting Omniphi Local Validator...
echo.

REM Clear ELECTRON_RUN_AS_NODE to ensure Electron runs as GUI app
REM (This is commonly set by IDEs like VSCode/Cursor)
set ELECTRON_RUN_AS_NODE=

REM Start the Electron app
start "" "node_modules\electron\dist\electron.exe" .

echo Application started!
echo.
echo The validator dashboard should open in a new window.
echo You can close this terminal window.
echo.
