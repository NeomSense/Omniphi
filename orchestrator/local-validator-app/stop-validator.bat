@echo off
echo ============================================
echo Omniphi Local Validator - Stopping...
echo ============================================
echo.

tasklist /FI "IMAGENAME eq posd.exe" 2>NUL | find /I "posd.exe" >NUL
if %ERRORLEVEL% NEQ 0 (
    echo Validator is not running.
    echo.
    pause
    exit /b
)

echo Stopping validator...
taskkill /IM posd.exe /F

echo.
echo Validator stopped.
echo.
pause
