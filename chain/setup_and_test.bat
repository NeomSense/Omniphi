@echo off
REM Omniphi Windows Setup and Test (CMD Batch Script)
REM This script sets up the chain and runs quick tests

cd /d C:\Users\herna\omniphi\pos

echo ========================================
echo Omniphi Windows Setup and Test
echo ========================================
echo.

REM Set environment variables
set BINARY=.\build\posd.exe
set CHAIN_ID=omniphi-1
set HOME_DIR=%USERPROFILE%\.pos
set DENOM=omniphi
set GAS_PRICES=0.025%DENOM%

REM Step 1: Build (if needed)
echo [Step 1/7] Checking binary...
if not exist build\posd.exe (
    echo   Building binary...
    go build -o build\posd.exe .\cmd\posd
    if errorlevel 1 (
        echo   Build failed!
        exit /b 1
    )
    echo   Binary built successfully
) else (
    echo   Binary already exists (218 MB)
)

REM Step 2: Clean old data
echo.
echo [Step 2/7] Cleaning old chain data...
if exist "%HOME_DIR%" (
    echo   Removing old data...
    rmdir /s /q "%HOME_DIR%"
)
echo   Old data cleaned

REM Step 3: Initialize chain
echo.
echo [Step 3/7] Initializing chain...
%BINARY% init validator-1 --chain-id %CHAIN_ID% --home "%HOME_DIR%" > nul
if errorlevel 1 (
    echo   Initialization failed!
    exit /b 1
)
echo   Chain initialized

REM Step 4: Create keys with test mnemonic
echo.
echo [Step 4/7] Creating test keys...
echo   NOTE: You'll need to provide test mnemonics interactively
echo   Press ENTER twice when prompted for BIP39 passphrase
echo.
echo   Creating alice key...
echo   Use mnemonic: abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about
%BINARY% keys add alice --keyring-backend test --home "%HOME_DIR%"
if errorlevel 1 (
    echo   Failed to create alice key!
    exit /b 1
)

echo.
echo   Creating bob key...
echo   Use mnemonic: ability abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about
%BINARY% keys add bob --keyring-backend test --home "%HOME_DIR%"
if errorlevel 1 (
    echo   Failed to create bob key!
    exit /b 1
)

REM Get addresses
for /f "tokens=*" %%i in ('%BINARY% keys show -a alice --keyring-backend test --home "%HOME_DIR%"') do set ALICE=%%i
for /f "tokens=*" %%i in ('%BINARY% keys show -a bob --keyring-backend test --home "%HOME_DIR%"') do set BOB=%%i

echo.
echo   Keys created:
echo     Alice: %ALICE%
echo     Bob:   %BOB%

REM Step 5: Add genesis accounts
echo.
echo [Step 5/7] Adding genesis accounts...
%BINARY% genesis add-genesis-account %ALICE% 1000000000000000%DENOM% --home "%HOME_DIR%"
%BINARY% genesis add-genesis-account %BOB% 1000000000000%DENOM% --home "%HOME_DIR%"
echo   Genesis accounts added

REM Step 6: Create and collect gentx
echo.
echo [Step 6/7] Creating genesis transaction...
mkdir "%HOME_DIR%\config\gentx" 2>nul
%BINARY% genesis gentx alice 500000000000%DENOM% --chain-id %CHAIN_ID% --keyring-backend test --home "%HOME_DIR%" > nul
if errorlevel 1 (
    echo   Gentx creation failed!
    exit /b 1
)

%BINARY% genesis collect-gentxs --home "%HOME_DIR%" > nul
if errorlevel 1 (
    echo   Gentx collection failed!
    exit /b 1
)

%BINARY% genesis validate --home "%HOME_DIR%"
if errorlevel 1 (
    echo   Genesis validation failed!
    exit /b 1
)
echo   Genesis validated successfully

REM Step 7: Run quick tests
echo.
echo [Step 7/7] Running quick comprehensive tests...
echo   This will run tests TC001-TC005 (5-10 minutes)...
echo.
go test -v .\test\comprehensive\... -run "TestTC00[1-5]" -timeout 10m > quick_test_results.txt 2>&1
if errorlevel 1 (
    echo   Some tests failed - see quick_test_results.txt
) else (
    echo   All tests passed!
)

REM Count results
findstr /c:"--- PASS:" quick_test_results.txt > nul
if not errorlevel 1 (
    for /f %%i in ('findstr /c:"--- PASS:" quick_test_results.txt ^| find /c /v ""') do set PASSED=%%i
) else (
    set PASSED=0
)

findstr /c:"--- FAIL:" quick_test_results.txt > nul
if not errorlevel 1 (
    for /f %%i in ('findstr /c:"--- FAIL:" quick_test_results.txt ^| find /c /v ""') do set FAILED=%%i
) else (
    set FAILED=0
)

echo.
echo ========================================
echo Test Results Summary
echo ========================================
echo   Passed: %PASSED%
echo   Failed: %FAILED%
echo   Logs: quick_test_results.txt
echo.

if "%FAILED%"=="0" (
    echo ALL TESTS PASSED!
    echo.
    echo Next steps:
    echo   - Start the chain: %BINARY% start --home "%HOME_DIR%"
    echo   - View 1x addresses: %BINARY% keys show-1x alice --keyring-backend test --home "%HOME_DIR%"
    exit /b 0
) else (
    echo SOME TESTS FAILED - Review quick_test_results.txt
    exit /b 1
)
