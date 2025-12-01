@echo off
REM ============================================
REM Omniphi Testnet Validator Setup Script
REM ============================================
REM This script sets up a new validator node to join
REM the Omniphi testnet network.
REM ============================================

setlocal enabledelayedexpansion

echo.
echo ============================================
echo   Omniphi Testnet Validator Setup
echo ============================================
echo.

REM Configuration
set "VPS_RPC=http://46.202.179.182:26657"
set "PERSISTENT_PEER=66a136f96b171ecb7b4b0bc42062fde959623b4e@46.202.179.182:26656"
set "CHAIN_ID=omniphi-testnet-1"
set "VALIDATOR_HOME=%USERPROFILE%\.omniphi-validator"

REM Get moniker from user
set /p MONIKER="Enter your validator name (moniker): "
if "%MONIKER%"=="" set "MONIKER=validator-%RANDOM%"

echo.
echo Configuration:
echo   Validator Home: %VALIDATOR_HOME%
echo   Moniker: %MONIKER%
echo   Chain ID: %CHAIN_ID%
echo   VPS RPC: %VPS_RPC%
echo.

REM Check if posd.exe exists
if not exist "%~dp0posd.exe" (
    echo ERROR: posd.exe not found in current directory!
    echo Please ensure posd.exe is in the same folder as this script.
    pause
    exit /b 1
)

REM Create validator home directory
echo [1/6] Creating validator home directory...
if not exist "%VALIDATOR_HOME%" mkdir "%VALIDATOR_HOME%"
if not exist "%VALIDATOR_HOME%\bin" mkdir "%VALIDATOR_HOME%\bin"

REM Copy posd.exe
echo [2/6] Copying posd.exe...
copy /Y "%~dp0posd.exe" "%VALIDATOR_HOME%\bin\posd.exe" >nul
if errorlevel 1 (
    echo ERROR: Failed to copy posd.exe
    pause
    exit /b 1
)

set "POSD=%VALIDATOR_HOME%\bin\posd.exe"

REM Initialize the node
echo [3/6] Initializing validator node...
"%POSD%" init "%MONIKER%" --chain-id %CHAIN_ID% --home "%VALIDATOR_HOME%" 2>nul
if errorlevel 1 (
    echo WARNING: Init returned error (may already be initialized)
)

REM Download genesis from VPS
echo [4/6] Downloading genesis file from VPS...
curl -s "%VPS_RPC%/genesis" > "%TEMP%\genesis_response.json"
if errorlevel 1 (
    echo ERROR: Failed to download genesis from VPS
    echo Please check your internet connection and try again.
    pause
    exit /b 1
)

REM Extract genesis from response using PowerShell
powershell -Command "(Get-Content '%TEMP%\genesis_response.json' | ConvertFrom-Json).result.genesis | ConvertTo-Json -Depth 100" > "%VALIDATOR_HOME%\config\genesis.json"
if errorlevel 1 (
    echo ERROR: Failed to extract genesis file
    pause
    exit /b 1
)

REM Configure persistent peers
echo [5/6] Configuring persistent peers...
set "CONFIG_FILE=%VALIDATOR_HOME%\config\config.toml"

REM Use PowerShell to update config.toml
powershell -Command "(Get-Content '%CONFIG_FILE%') -replace 'persistent_peers = \"\"', 'persistent_peers = \"%PERSISTENT_PEER%\"' | Set-Content '%CONFIG_FILE%'"

REM Also ensure external_address is empty (for non-public validators)
powershell -Command "(Get-Content '%CONFIG_FILE%') -replace 'external_address = \"\"', 'external_address = \"\"' | Set-Content '%CONFIG_FILE%'"

REM Create start script
echo [6/6] Creating start script...
(
echo @echo off
echo echo Starting Omniphi Validator: %MONIKER%
echo echo.
echo "%VALIDATOR_HOME%\bin\posd.exe" start --home "%VALIDATOR_HOME%"
echo pause
) > "%VALIDATOR_HOME%\start-validator.bat"

REM Create stop script
(
echo @echo off
echo echo Stopping Omniphi Validator...
echo taskkill /IM posd.exe /F 2^>nul
echo echo Validator stopped.
echo pause
) > "%VALIDATOR_HOME%\stop-validator.bat"

echo.
echo ============================================
echo   Setup Complete!
echo ============================================
echo.
echo Validator Home: %VALIDATOR_HOME%
echo Moniker: %MONIKER%
echo.
echo To start your validator, run:
echo   %VALIDATOR_HOME%\start-validator.bat
echo.
echo Or manually:
echo   "%VALIDATOR_HOME%\bin\posd.exe" start --home "%VALIDATOR_HOME%"
echo.
echo Your validator will connect to the VPS and sync blocks.
echo This may take a few minutes depending on network speed.
echo.
echo ============================================

pause
