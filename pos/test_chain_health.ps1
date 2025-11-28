#################################################################################
# Chain Health Test Script (PowerShell)
# Tests basic chain functionality and validator status
#################################################################################

Write-Host "================================" -ForegroundColor Cyan
Write-Host "Chain Health Check" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Test 1: Check if posd binary exists
Write-Host "[1/5] Checking posd binary..." -ForegroundColor Yellow
if (Test-Path ".\posd.exe") {
    Write-Host "  ✓ posd binary found" -ForegroundColor Green
    & .\posd.exe version
} else {
    Write-Host "  ✗ posd binary not found" -ForegroundColor Red
    Write-Host "  Run: go build -o posd.exe ./cmd/posd" -ForegroundColor Yellow
    exit 1
}
Write-Host ""

# Test 2: Check if chain is running
Write-Host "[2/5] Checking if chain is running..." -ForegroundColor Yellow
try {
    $status = & .\posd.exe status 2>$null | ConvertFrom-Json
    Write-Host "  ✓ Chain is running" -ForegroundColor Green
    $height = $status.sync_info.latest_block_height
    Write-Host "  Current block height: $height" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Chain is not running" -ForegroundColor Red
    Write-Host "  Start with: .\posd.exe start --minimum-gas-prices 0.001uomni --grpc.enable=false" -ForegroundColor Yellow
    exit 1
}
Write-Host ""

# Test 3: Check validator status
Write-Host "[3/5] Checking validator status..." -ForegroundColor Yellow
if (Test-Path "chain.log") {
    if (Select-String -Path chain.log -Pattern "This node is a validator" -Quiet) {
        Write-Host "  ✓ Node is a validator" -ForegroundColor Green
        $validatorLine = Select-String -Path chain.log -Pattern "This node is a validator" | Select-Object -Last 1
        if ($validatorLine -match 'addr=([A-F0-9]+)') {
            $validatorAddr = $matches[1]
            Write-Host "  Validator address: $validatorAddr" -ForegroundColor Green
        }
    } else {
        Write-Host "  ⚠ Node is not a validator" -ForegroundColor Yellow
    }
} else {
    Write-Host "  ⚠ chain.log not found (chain may be running in foreground)" -ForegroundColor Yellow
}
Write-Host ""

# Test 4: Check block production
Write-Host "[4/5] Testing block production..." -ForegroundColor Yellow
$initialHeight = (& .\posd.exe status 2>$null | ConvertFrom-Json).sync_info.latest_block_height
Write-Host "  Initial height: $initialHeight"
Write-Host "  Waiting 10 seconds..."
Start-Sleep -Seconds 10
$newHeight = (& .\posd.exe status 2>$null | ConvertFrom-Json).sync_info.latest_block_height
Write-Host "  New height: $newHeight"

if ($newHeight -gt $initialHeight) {
    $blocksProduced = $newHeight - $initialHeight
    Write-Host "  ✓ Blocks are being produced ($blocksProduced blocks in 10s)" -ForegroundColor Green
} else {
    Write-Host "  ✗ No blocks produced" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Test 5: Check module queries
Write-Host "[5/5] Checking module availability..." -ForegroundColor Yellow
$modulesOK = $true

try {
    & .\posd.exe query feemarket base-fee 2>$null | Out-Null
    Write-Host "  ✓ FeeMarket module responding" -ForegroundColor Green
} catch {
    Write-Host "  ✗ FeeMarket module not responding" -ForegroundColor Red
    $modulesOK = $false
}

try {
    & .\posd.exe query tokenomics params 2>$null | Out-Null
    Write-Host "  ✓ Tokenomics module responding" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Tokenomics module not responding" -ForegroundColor Red
    $modulesOK = $false
}

try {
    & .\posd.exe query poc params 2>$null | Out-Null
    Write-Host "  ✓ POC module responding" -ForegroundColor Green
} catch {
    Write-Host "  ✗ POC module not responding" -ForegroundColor Red
    $modulesOK = $false
}
Write-Host ""

# Summary
Write-Host "================================" -ForegroundColor Cyan
if ($modulesOK) {
    Write-Host "✅ All health checks passed!" -ForegroundColor Green
    Write-Host "================================" -ForegroundColor Cyan
    exit 0
} else {
    Write-Host "⚠️  Some checks failed" -ForegroundColor Red
    Write-Host "================================" -ForegroundColor Cyan
    exit 1
}
