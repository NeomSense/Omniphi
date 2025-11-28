# Omniphi Windows Setup and Test Script
# Run this in PowerShell: .\Setup-OmniphiChain.ps1

param(
    [switch]$SkipBuild = $false,
    [switch]$SkipTests = $false
)

$ErrorActionPreference = "Stop"

Write-Host @"

========================================
Omniphi Windows Setup and Test
========================================

"@ -ForegroundColor Cyan

# Setup variables
$BINARY = ".\build\posd.exe"
$CHAIN_ID = "omniphi-1"
$HOME_DIR = "$env:USERPROFILE\.pos"
$DENOM = "omniphi"

# Step 1: Build
if (-not $SkipBuild) {
    Write-Host "[Step 1/7] Building binary..." -ForegroundColor Cyan

    if (Test-Path "build\posd.exe") {
        $size = [math]::Round((Get-Item "build\posd.exe").Length / 1MB, 2)
        Write-Host "  Binary already exists ($size MB)" -ForegroundColor Green
    } else {
        Write-Host "  Building (this may take 2-3 minutes)..." -ForegroundColor Yellow
        go build -o build\posd.exe .\cmd\posd

        if ($LASTEXITCODE -ne 0) {
            Write-Host "  Build failed!" -ForegroundColor Red
            exit 1
        }
        Write-Host "  Binary built successfully" -ForegroundColor Green
    }
} else {
    Write-Host "[Step 1/7] Skipping build" -ForegroundColor Cyan
}

# Step 2: Clean old data
Write-Host "`n[Step 2/7] Cleaning old chain data..." -ForegroundColor Cyan

if (Test-Path $HOME_DIR) {
    Write-Host "  Removing $HOME_DIR..." -ForegroundColor Yellow
    Remove-Item -Recurse -Force $HOME_DIR -ErrorAction SilentlyContinue
    Write-Host "  Old data removed" -ForegroundColor Green
} else {
    Write-Host "  No old data to clean" -ForegroundColor Green
}

# Step 3: Initialize chain
Write-Host "`n[Step 3/7] Initializing chain..." -ForegroundColor Cyan
Write-Host "  Running: $BINARY init validator-1..." -ForegroundColor Yellow

& $BINARY init validator-1 --chain-id $CHAIN_ID --home $HOME_DIR | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Host "  Chain initialized successfully" -ForegroundColor Green
} else {
    Write-Host "  Initialization failed!" -ForegroundColor Red
    exit 1
}

# Step 4: Create keys
Write-Host "`n[Step 4/7] Creating test keys..." -ForegroundColor Cyan
Write-Host "  This requires interactive input for mnemonics" -ForegroundColor Yellow
Write-Host ""

# Create alice key
Write-Host "  Creating alice key..." -ForegroundColor White
Write-Host "  When prompted:" -ForegroundColor Gray
Write-Host "    1. Enter mnemonic: abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about" -ForegroundColor Gray
Write-Host "    2. Press ENTER for empty BIP39 passphrase (twice)" -ForegroundColor Gray
Write-Host ""

& $BINARY keys add alice --recover --keyring-backend test --home $HOME_DIR

if ($LASTEXITCODE -ne 0) {
    Write-Host "  Failed to create alice key!" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "  Creating bob key..." -ForegroundColor White
Write-Host "  When prompted:" -ForegroundColor Gray
Write-Host "    1. Enter mnemonic: abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about" -ForegroundColor Gray
Write-Host "    2. Press ENTER for empty BIP39 passphrase (twice)" -ForegroundColor Gray
Write-Host "  Note: Using --account 1 to generate different address from alice" -ForegroundColor Gray
Write-Host ""

& $BINARY keys add bob --recover --account 1 --keyring-backend test --home $HOME_DIR

if ($LASTEXITCODE -ne 0) {
    Write-Host "  Failed to create bob key!" -ForegroundColor Red
    exit 1
}

# Get addresses
$ALICE = & $BINARY keys show -a alice --keyring-backend test --home $HOME_DIR
$BOB = & $BINARY keys show -a bob --keyring-backend test --home $HOME_DIR

Write-Host "`n  Keys created successfully:" -ForegroundColor Green
Write-Host "    Alice: $ALICE" -ForegroundColor White
Write-Host "    Bob:   $BOB" -ForegroundColor White

# Step 5: Add genesis accounts
Write-Host "`n[Step 5/7] Adding genesis accounts..." -ForegroundColor Cyan

& $BINARY genesis add-genesis-account $ALICE "1000000000000000$DENOM" --home $HOME_DIR | Out-Null
& $BINARY genesis add-genesis-account $BOB "1000000000000$DENOM" --home $HOME_DIR | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Host "  Genesis accounts added" -ForegroundColor Green
} else {
    Write-Host "  Failed to add genesis accounts!" -ForegroundColor Red
    exit 1
}

# Step 6: Create and collect gentx
Write-Host "`n[Step 6/7] Creating genesis transaction..." -ForegroundColor Cyan

New-Item -ItemType Directory -Force -Path "$HOME_DIR\config\gentx" | Out-Null

& $BINARY genesis gentx alice "500000000000$DENOM" `
    --chain-id $CHAIN_ID `
    --keyring-backend test `
    --home $HOME_DIR | Out-Null

if ($LASTEXITCODE -ne 0) {
    Write-Host "  Gentx creation failed!" -ForegroundColor Red
    exit 1
}

& $BINARY genesis collect-gentxs --home $HOME_DIR | Out-Null

if ($LASTEXITCODE -ne 0) {
    Write-Host "  Gentx collection failed!" -ForegroundColor Red
    exit 1
}

Write-Host "  Validating genesis..." -ForegroundColor Yellow
& $BINARY genesis validate --home $HOME_DIR

if ($LASTEXITCODE -eq 0) {
    Write-Host "  Genesis validated successfully" -ForegroundColor Green
} else {
    Write-Host "  Genesis validation failed!" -ForegroundColor Red
    exit 1
}

Write-Host "  Configuring minimum gas price..." -ForegroundColor Yellow
$appTomlPath = "$HOME_DIR\config\app.toml"
$appTomlContent = Get-Content $appTomlPath -Raw
$appTomlContent = $appTomlContent -replace 'minimum-gas-prices = ""', 'minimum-gas-prices = "0.025omniphi"'
$appTomlContent | Set-Content $appTomlPath
Write-Host "  Minimum gas price set to 0.025omniphi" -ForegroundColor Green

# Step 7: Run tests
if (-not $SkipTests) {
    Write-Host "`n[Step 7/7] Running quick comprehensive tests..." -ForegroundColor Cyan
    Write-Host "  Running tests TC001-TC005 (this may take 5-10 minutes)..." -ForegroundColor Yellow
    Write-Host ""

    $testOutput = go test -v .\test\comprehensive\... -run "TestTC00[1-5]" -timeout 10m 2>&1
    $testOutput | Out-File -FilePath "quick_test_results.txt"

    # Display output
    $testOutput | Write-Host

    # Count results
    $passed = ($testOutput | Select-String -Pattern "--- PASS:" | Measure-Object).Count
    $failed = ($testOutput | Select-String -Pattern "--- FAIL:" | Measure-Object).Count

    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "Test Results Summary" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  Passed: $passed" -ForegroundColor Green
    Write-Host "  Failed: $failed" -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Red" })
    Write-Host "  Logs:   quick_test_results.txt" -ForegroundColor Gray
    Write-Host ""

    if ($failed -eq 0 -and $passed -gt 0) {
        Write-Host "  ALL TESTS PASSED!" -ForegroundColor Green
    } elseif ($passed -eq 0 -and $failed -eq 0) {
        Write-Host "  WARNING: No tests were run!" -ForegroundColor Yellow
    } else {
        Write-Host "  SOME TESTS FAILED - Review quick_test_results.txt" -ForegroundColor Red
    }
} else {
    Write-Host "`n[Step 7/7] Skipping tests" -ForegroundColor Cyan
}

# Final summary
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Setup Complete!" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

Write-Host "`nChain Information:" -ForegroundColor White
Write-Host "  Chain ID:  $CHAIN_ID" -ForegroundColor Gray
Write-Host "  Home Dir:  $HOME_DIR" -ForegroundColor Gray
Write-Host "  Binary:    $BINARY" -ForegroundColor Gray
Write-Host "  Denom:     $DENOM" -ForegroundColor Gray

Write-Host "`nAccounts:" -ForegroundColor White
Write-Host "  Alice:     $ALICE" -ForegroundColor Gray
Write-Host "  Bob:       $BOB" -ForegroundColor Gray

Write-Host "`nUseful Commands:" -ForegroundColor White
Write-Host "  Start chain:" -ForegroundColor Cyan
Write-Host "    & `"$BINARY`" start --home `"$HOME_DIR`"" -ForegroundColor Yellow

Write-Host "`n  View 1x address format:" -ForegroundColor Cyan
Write-Host "    & `"$BINARY`" keys show-1x alice --keyring-backend test --home `"$HOME_DIR`"" -ForegroundColor Yellow

Write-Host "`n  Submit PoC contribution:" -ForegroundColor Cyan
Write-Host "    & `"$BINARY`" tx poc submit-contribution code ipfs://hash 0xhash --from alice --keyring-backend test --chain-id $CHAIN_ID --gas auto --fees 25000$DENOM -y" -ForegroundColor Yellow

Write-Host ""
