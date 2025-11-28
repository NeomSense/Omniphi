# Omniphi Windows - Complete Setup and Test Automation
# This script builds, initializes, and tests the entire Omniphi blockchain on Windows

param(
    [switch]$SkipBuild = $false,
    [switch]$SkipTests = $false,
    [switch]$QuickTest = $false
)

$ErrorActionPreference = "Stop"

# Setup variables
$PROJECT_ROOT = "C:\Users\herna\omniphi\pos"
$env:BINARY = ".\build\posd.exe"
$env:CHAIN_ID = "omniphi-1"
$env:HOME_DIR = "$env:USERPROFILE\.pos"
$env:DENOM = "omniphi"
$env:GAS_PRICES = "0.025$env:DENOM"

cd $PROJECT_ROOT

Write-Host "`n========================================"  -ForegroundColor Cyan
Write-Host "Omniphi Windows Setup and Test"  -ForegroundColor Cyan
Write-Host "========================================`n"  -ForegroundColor Cyan

# Step 1: Build Binary
if (-not $SkipBuild) {
    Write-Host "`n[Step 1/7] Building posd.exe..." -ForegroundColor Cyan

    if (Test-Path "build\posd.exe") {
        Remove-Item "build\posd.exe" -Force
    }

    Write-Host "  Building binary (this may take 2-3 minutes)..." -ForegroundColor Yellow
    go build -o build\posd.exe .\cmd\posd

    if (Test-Path "build\posd.exe") {
        $sizeMB = [math]::Round((Get-Item "build\posd.exe").Length / 1MB, 2)
        Write-Host "  Binary built successfully ($sizeMB MB)" -ForegroundColor Green
    } else {
        Write-Host "  Build failed" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "`n[Step 1/7] Skipping build (using existing binary)" -ForegroundColor Cyan
}

# Step 2: Clean Old Data
Write-Host "`n[Step 2/7] Cleaning Old Chain Data..." -ForegroundColor Cyan

if (Test-Path $env:HOME_DIR) {
    Write-Host "  Removing old data at $env:HOME_DIR..." -ForegroundColor Yellow
    Remove-Item -Recurse -Force $env:HOME_DIR -ErrorAction SilentlyContinue
    Write-Host "  Old data removed" -ForegroundColor Green
} else {
    Write-Host "  No old data to clean" -ForegroundColor Green
}

# Step 3: Initialize Chain
Write-Host "`n[Step 3/7] Initializing Chain..." -ForegroundColor Cyan

Write-Host "  Initializing validator-1..." -ForegroundColor Yellow
& $env:BINARY init validator-1 --chain-id $env:CHAIN_ID --home $env:HOME_DIR | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Host "  Chain initialized" -ForegroundColor Green
} else {
    Write-Host "  Chain initialization failed" -ForegroundColor Red
    exit 1
}

# Step 4: Create Keys
Write-Host "`n[Step 4/7] Creating Test Keys..." -ForegroundColor Cyan

# Use temp files for mnemonics to avoid interactive prompts
$aliceMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
$bobMnemonic = "easy abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

Write-Host "  Creating alice key..." -ForegroundColor Yellow
$aliceMnemonic | Out-File -FilePath "$env:TEMP\alice_mnemonic.txt" -Encoding ascii -NoNewline
Get-Content "$env:TEMP\alice_mnemonic.txt" | & $env:BINARY keys add alice --recover --keyring-backend test --home $env:HOME_DIR 2>&1 | Out-Null

Write-Host "  Creating bob key..." -ForegroundColor Yellow
$bobMnemonic | Out-File -FilePath "$env:TEMP\bob_mnemonic.txt" -Encoding ascii -NoNewline
Get-Content "$env:TEMP\bob_mnemonic.txt" | & $env:BINARY keys add bob --recover --keyring-backend test --home $env:HOME_DIR 2>&1 | Out-Null

# Cleanup temp files
Remove-Item "$env:TEMP\alice_mnemonic.txt" -ErrorAction SilentlyContinue
Remove-Item "$env:TEMP\bob_mnemonic.txt" -ErrorAction SilentlyContinue

$ALICE = & $env:BINARY keys show -a alice --keyring-backend test --home $env:HOME_DIR
$BOB = & $env:BINARY keys show -a bob --keyring-backend test --home $env:HOME_DIR

Write-Host "  Keys created" -ForegroundColor Green
Write-Host "    Alice: $ALICE" -ForegroundColor White
Write-Host "    Bob:   $BOB" -ForegroundColor White

# Step 5: Add Genesis Accounts
Write-Host "`n[Step 5/7] Adding Genesis Accounts..." -ForegroundColor Cyan

Write-Host "  Adding alice (1B OMNI)..." -ForegroundColor Yellow
& $env:BINARY genesis add-genesis-account $ALICE "1000000000000000$env:DENOM" --home $env:HOME_DIR | Out-Null

Write-Host "  Adding bob (1M OMNI)..." -ForegroundColor Yellow
& $env:BINARY genesis add-genesis-account $BOB "1000000000000$env:DENOM" --home $env:HOME_DIR | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Host "  Genesis accounts added" -ForegroundColor Green
} else {
    Write-Host "  Failed to add genesis accounts" -ForegroundColor Red
    exit 1
}

# Step 6: Create and Collect Gentx
Write-Host "`n[Step 6/7] Creating Genesis Transaction..." -ForegroundColor Cyan

Write-Host "  Creating gentx directory..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path "$env:HOME_DIR\config\gentx" | Out-Null

Write-Host "  Creating gentx for alice (500K OMNI stake)..." -ForegroundColor Yellow
& $env:BINARY genesis gentx alice "500000000000$env:DENOM" --chain-id $env:CHAIN_ID --keyring-backend test --home $env:HOME_DIR | Out-Null

if ($LASTEXITCODE -ne 0) {
    Write-Host "  Gentx creation failed" -ForegroundColor Red
    exit 1
}

Write-Host "  Collecting gentxs..." -ForegroundColor Yellow
& $env:BINARY genesis collect-gentxs --home $env:HOME_DIR | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Host "  Gentx created and collected" -ForegroundColor Green
} else {
    Write-Host "  Gentx collection failed" -ForegroundColor Red
    exit 1
}

Write-Host "  Validating genesis..." -ForegroundColor Yellow
& $env:BINARY genesis validate --home $env:HOME_DIR | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Host "  Genesis validated successfully" -ForegroundColor Green
} else {
    Write-Host "  Genesis validation failed" -ForegroundColor Red
    exit 1
}

# Step 7: Run Tests
if (-not $SkipTests) {
    Write-Host "`n[Step 7/7] Running Tests..." -ForegroundColor Cyan

    $totalPassed = 0
    $totalFailed = 0
    $totalTests = 0

    # Run Comprehensive Tests
    if ($QuickTest) {
        Write-Host "  Running quick comprehensive tests (TC001-TC010)..." -ForegroundColor Yellow
        $output = go test -v .\test\comprehensive\... -run "TestTC00[1-9]|TestTC010" -timeout 10m 2>&1
    } else {
        Write-Host "  Running full comprehensive test suite (20-30 minutes)..." -ForegroundColor Yellow
        $output = go test -v .\test\comprehensive\... -timeout 30m 2>&1
    }

    $output | Out-File -FilePath "test_comprehensive_results.txt"

    $passCount = ($output | Select-String -Pattern "--- PASS:" | Measure-Object).Count
    $failCount = ($output | Select-String -Pattern "--- FAIL:" | Measure-Object).Count

    $totalPassed += $passCount
    $totalFailed += $failCount
    $totalTests += ($passCount + $failCount)

    if ($failCount -eq 0) {
        Write-Host "  Comprehensive tests PASSED ($passCount tests)" -ForegroundColor Green
    } else {
        Write-Host "  Comprehensive tests FAILED ($failCount failures)" -ForegroundColor Red
    }

    # Run Unit Tests
    Write-Host "  Running unit tests for poc and tokenomics..." -ForegroundColor Yellow
    $unitOutput = go test -v .\x\poc\... .\x\tokenomics\... -timeout 10m 2>&1

    $unitOutput | Out-File -FilePath "test_unit_results.txt"

    $unitPass = ($unitOutput | Select-String -Pattern "--- PASS:" | Measure-Object).Count
    $unitFail = ($unitOutput | Select-String -Pattern "--- FAIL:" | Measure-Object).Count

    $totalPassed += $unitPass
    $totalFailed += $unitFail
    $totalTests += ($unitPass + $unitFail)

    if ($unitFail -eq 0) {
        Write-Host "  Unit tests PASSED ($unitPass tests)" -ForegroundColor Green
    } else {
        Write-Host "  Unit tests FAILED ($unitFail failures)" -ForegroundColor Red
    }

    # Final Test Summary
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "Test Results Summary" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  Total Tests: $totalTests" -ForegroundColor White
    Write-Host "  Passed:      $totalPassed" -ForegroundColor Green
    Write-Host "  Failed:      $totalFailed" -ForegroundColor $(if ($totalFailed -eq 0) { "Green" } else { "Red" })
    Write-Host "`nDetailed logs:" -ForegroundColor White
    Write-Host "  test_comprehensive_results.txt" -ForegroundColor Gray
    Write-Host "  test_unit_results.txt" -ForegroundColor Gray

    if ($totalFailed -eq 0) {
        Write-Host "`n ALL TESTS PASSED!" -ForegroundColor Green
    } else {
        Write-Host "`n SOME TESTS FAILED - Review logs" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "`n[Step 7/7] Skipping Tests" -ForegroundColor Cyan
}

# Final Summary
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Setup Complete!" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

Write-Host "`nChain Information:" -ForegroundColor White
Write-Host "  Chain ID:  $env:CHAIN_ID" -ForegroundColor Gray
Write-Host "  Home Dir:  $env:HOME_DIR" -ForegroundColor Gray
Write-Host "  Binary:    $env:BINARY" -ForegroundColor Gray
Write-Host "  Denom:     $env:DENOM" -ForegroundColor Gray

Write-Host "`nAccounts:" -ForegroundColor White
Write-Host "  Alice:     $ALICE" -ForegroundColor Gray
Write-Host "  Bob:       $BOB" -ForegroundColor Gray

Write-Host "`nTo start the chain:" -ForegroundColor White
Write-Host "  & $env:BINARY start --home `"$env:HOME_DIR`"" -ForegroundColor Yellow

Write-Host "`nTo view addresses in 1x format:" -ForegroundColor White
Write-Host "  & $env:BINARY keys show-1x alice --keyring-backend test --home `"$env:HOME_DIR`"" -ForegroundColor Yellow
Write-Host ""
