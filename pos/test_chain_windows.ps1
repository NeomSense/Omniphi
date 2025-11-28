# Windows Chain Test Script
# Tests the full Omniphi chain with proper gentx validator creation
# This validates the fix for "validator set is empty after InitGenesis"

Write-Host "================================" -ForegroundColor Cyan
Write-Host "Omniphi Chain Test - Windows" -ForegroundColor Cyan
Write-Host "Testing gentx validator setup" -ForegroundColor Yellow
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Configuration
$CHAIN_ID = "omniphi-test"
$MONIKER = "test-validator"
$KEY_NAME = "testkey"
$HOME_DIR = "$env:USERPROFILE\.pos"
$DENOM = "uomni"

# Step 1: Clean old data
Write-Host "[1/10] Cleaning old chain data..." -ForegroundColor Yellow
if (Test-Path $HOME_DIR) {
    Remove-Item -Recurse -Force $HOME_DIR
    Write-Host "  ✓ Removed $HOME_DIR" -ForegroundColor Green
}

# Step 2: Initialize chain
Write-Host "[2/10] Initializing chain..." -ForegroundColor Yellow
& .\posd.exe init $MONIKER --chain-id $CHAIN_ID --home $HOME_DIR 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ Chain initialized" -ForegroundColor Green
} else {
    Write-Host "  ✗ Failed to initialize chain" -ForegroundColor Red
    exit 1
}

# Step 3: Create test key
Write-Host "[3/10] Creating test key..." -ForegroundColor Yellow
& .\posd.exe keys add $KEY_NAME --keyring-backend test --home $HOME_DIR 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    $ADDRESS = & .\posd.exe keys show $KEY_NAME -a --keyring-backend test --home $HOME_DIR
    Write-Host "  ✓ Key created: $ADDRESS" -ForegroundColor Green
} else {
    Write-Host "  ✗ Failed to create key" -ForegroundColor Red
    exit 1
}

# Step 4: Add genesis account
Write-Host "[4/13] Adding genesis account..." -ForegroundColor Yellow
& .\posd.exe genesis add-genesis-account $KEY_NAME 1000000000000000$DENOM --keyring-backend test --home $HOME_DIR 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ Genesis account added (1,000,000 OMNI)" -ForegroundColor Green
} else {
    Write-Host "  ✗ Failed to add genesis account" -ForegroundColor Red
    exit 1
}

# Step 5: Fix bond_denom (FIRST TIME - before gentx)
Write-Host "[5/13] Fixing staking bond_denom..." -ForegroundColor Yellow
$GENESIS_FILE = "$HOME_DIR\config\genesis.json"
(Get-Content $GENESIS_FILE) -replace '"bond_denom": "stake"', '"bond_denom": "uomni"' | Set-Content $GENESIS_FILE
Write-Host "  ✓ bond_denom set to uomni" -ForegroundColor Green

# Step 6: Configure minimum gas prices
Write-Host "[6/13] Configuring gas prices..." -ForegroundColor Yellow
$APP_TOML = "$HOME_DIR\config\app.toml"
(Get-Content $APP_TOML) -replace 'minimum-gas-prices = ""', 'minimum-gas-prices = "0.001uomni"' | Set-Content $APP_TOML
Write-Host "  ✓ Set minimum gas price to 0.001uomni" -ForegroundColor Green

# Step 7: Create gentx (CRITICAL - creates validator transaction)
Write-Host "[7/13] Creating genesis transaction..." -ForegroundColor Yellow
& .\posd.exe genesis gentx $KEY_NAME 100000000000$DENOM `
    --chain-id $CHAIN_ID `
    --moniker $MONIKER `
    --commission-rate 0.1 `
    --commission-max-rate 0.2 `
    --commission-max-change-rate 0.01 `
    --min-self-delegation 1 `
    --keyring-backend test `
    --home $HOME_DIR 2>&1 | Out-Null

if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ gentx created successfully" -ForegroundColor Green
    $gentxFile = Get-ChildItem "$HOME_DIR\config\gentx" -File | Select-Object -First 1
    Write-Host "    File: $($gentxFile.Name)" -ForegroundColor Gray
} else {
    Write-Host "  ✗ gentx creation failed" -ForegroundColor Red
    exit 1
}

# Step 8: Collect gentxs (adds to genutil.gen_txs)
Write-Host "[8/13] Collecting genesis transactions..." -ForegroundColor Yellow
& .\posd.exe genesis collect-gentxs --home $HOME_DIR 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ gentxs collected" -ForegroundColor Green
} else {
    Write-Host "  ✗ collect-gentxs failed" -ForegroundColor Red
    exit 1
}

# Step 9: Fix bond_denom AGAIN (collect-gentxs overwrites it!)
Write-Host "[9/13] Re-fixing bond_denom (collect-gentxs overwrites it)..." -ForegroundColor Yellow
(Get-Content $GENESIS_FILE) -replace '"bond_denom": "omniphi"', '"bond_denom": "uomni"' | Set-Content $GENESIS_FILE
Write-Host "  ✓ bond_denom re-fixed to uomni" -ForegroundColor Green

# Step 10: Verify modules are registered
Write-Host "[10/13] Verifying module registration..." -ForegroundColor Yellow
$modules = & .\posd.exe query --help --home $HOME_DIR | Select-String -Pattern "feemarket|poc|tokenomics"
if ($modules.Count -eq 3) {
    Write-Host "  ✓ All 3 modules registered:" -ForegroundColor Green
    $modules | ForEach-Object { Write-Host "    - $($_.Line.Trim())" -ForegroundColor Gray }
} else {
    Write-Host "  ✗ Missing modules (found $($modules.Count)/3)" -ForegroundColor Red
    exit 1
}

# Step 11: Validate genesis
Write-Host "[11/13] Validating final genesis..." -ForegroundColor Yellow
& .\posd.exe genesis validate-genesis --home $HOME_DIR 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ Genesis valid" -ForegroundColor Green
} else {
    Write-Host "  ✗ Genesis validation failed" -ForegroundColor Red
    exit 1
}

# Step 12: Check genutil.gen_txs (should have 1 transaction)
Write-Host "[12/13] Verifying genutil.gen_txs..." -ForegroundColor Yellow
$GENESIS_CONTENT = Get-Content $GENESIS_FILE | ConvertFrom-Json
$GEN_TXS_COUNT = ($GENESIS_CONTENT.app_state.genutil.gen_txs | Measure-Object).Count
if ($GEN_TXS_COUNT -gt 0) {
    Write-Host "  ✓ Found $GEN_TXS_COUNT gentx in genesis" -ForegroundColor Green
} else {
    Write-Host "  ✗ No gentxs found - validator won't be created!" -ForegroundColor Red
    exit 1
}

# Step 13: Check feemarket genesis
Write-Host "[13/13] Checking feemarket genesis..." -ForegroundColor Yellow
$GENESIS = Get-Content "$HOME_DIR\config\genesis.json" | ConvertFrom-Json
if ($GENESIS.app_state.feemarket.params) {
    Write-Host "  ✓ FeeMarket params present in genesis" -ForegroundColor Green
    Write-Host "    - Base Fee: $($GENESIS.app_state.feemarket.params.base_fee_initial)" -ForegroundColor Gray
    Write-Host "    - Min Gas Price: $($GENESIS.app_state.feemarket.params.min_gas_price)" -ForegroundColor Gray
} elseif ($null -eq $GENESIS.app_state.feemarket.params -or $GENESIS.app_state.feemarket.params.PSObject.Properties.Count -eq 0) {
    Write-Host "  ⚠ FeeMarket genesis empty (will use defaults on startup)" -ForegroundColor Yellow
} else {
    Write-Host "  ✓ FeeMarket ready" -ForegroundColor Green
}

Write-Host ""
Write-Host "================================" -ForegroundColor Cyan
Write-Host "✅ Genesis Setup Complete!" -ForegroundColor Green
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "The chain is ready to start. Look for this message:" -ForegroundColor Yellow
Write-Host "  'This node is a validator addr=... pubKey=...'" -ForegroundColor Cyan
Write-Host ""
Write-Host "Press Enter to start chain for 5 second test..." -ForegroundColor Yellow
Read-Host

# Start chain test
Write-Host "Starting chain (5 second test)..." -ForegroundColor Yellow
$JOB = Start-Job -ScriptBlock {
    param($exe, $home)
    & $exe start --minimum-gas-prices 0.001uomni --home $home 2>&1
} -ArgumentList (Resolve-Path ".\posd.exe"), $HOME_DIR

Start-Sleep -Seconds 5

# Check if chain is running
Write-Host "Checking chain status..." -ForegroundColor Yellow
try {
    $STATUS = & .\posd.exe status --home $HOME_DIR 2>&1 | ConvertFrom-Json
    if ($STATUS.SyncInfo.latest_block_height -gt 0) {
        Write-Host "  ✓ Chain running at block height: $($STATUS.SyncInfo.latest_block_height)" -ForegroundColor Green
        Write-Host ""
        Write-Host "================================" -ForegroundColor Cyan
        Write-Host "✅ ALL TESTS PASSED!" -ForegroundColor Green
        Write-Host "================================" -ForegroundColor Cyan
    } else {
        Write-Host "  ⚠ Chain started but no blocks yet" -ForegroundColor Yellow
    }
} catch {
    Write-Host "  ✗ Chain not responding" -ForegroundColor Red
    Write-Host "  Error: $_" -ForegroundColor Red
}

# Cleanup
Stop-Job $JOB -ErrorAction SilentlyContinue
Remove-Job $JOB -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Test complete. Chain data preserved in: $HOME_DIR" -ForegroundColor Gray
Write-Host "To start manually: .\posd.exe start --home $HOME_DIR" -ForegroundColor Gray
