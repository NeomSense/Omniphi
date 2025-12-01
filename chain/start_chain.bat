@echo off
echo Starting OmniPhi blockchain...
cd /d "%~dp0"
start /B posd.exe start --home %USERPROFILE%\.pos > posd.log 2>&1
timeout /t 2 >nul
echo Chain started in background. Check posd.log for status.
echo.
echo To stop: taskkill /F /IM posd.exe
echo To view status: posd.exe status --home %USERPROFILE%\.pos
