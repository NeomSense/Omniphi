@echo off
echo ============================================
echo Omniphi Local Validator - Starting...
echo ============================================
echo.

cd /d "%~dp0"

echo Chain ID: omniphi-testnet-1
echo Home Directory: %USERPROFILE%\.pos-validator2
echo.

echo Checking if validator is already running...
tasklist /FI "IMAGENAME eq posd.exe" 2>NUL | find /I "posd.exe" >NUL
if %ERRORLEVEL% EQU 0 (
    echo Validator is already running!
    echo.
    pause
    exit /b
)

echo Starting validator node...
echo.

start "Omniphi Validator" /MIN cmd /c "bin\posd.exe start --home %USERPROFILE%\.pos-validator2 --minimum-gas-prices 0.001omniphi 2>&1 | tee %USERPROFILE%\.pos-validator2\node.log"

echo.
echo ============================================
echo Validator started in background!
echo ============================================
echo.
echo To check status:
echo   curl http://localhost:26657/status
echo.
echo To view logs:
echo   type %USERPROFILE%\.pos-validator2\node.log
echo.
echo To stop:
echo   taskkill /IM posd.exe
echo.
pause
