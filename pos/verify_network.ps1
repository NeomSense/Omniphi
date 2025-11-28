# Verify Testnet Network Status - Windows PowerShell

$VALIDATOR_HOME = ".\testnet-2nodes\validator1"

# Check if posd binary exists
$POSD_BIN = ".\posd.exe"
if (-not (Test-Path $POSD_BIN)) {
    $POSD_BIN = "posd"
    if (-not (Get-Command posd -ErrorAction SilentlyContinue)) {
        Write-Host "Error: posd binary not found!" -ForegroundColor Red
        exit 1
    }
}

Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  Network Status Check" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Check node status
Write-Host "1. Node Status:" -ForegroundColor Yellow
try {
    $status = & $POSD_BIN status --home $VALIDATOR_HOME 2>&1 | Out-String
    if ($status -match "latest_block_height") {
        $status | Select-String -Pattern "latest_block_height|catching_up|voting_power"
    } else {
        Write-Host "   Node not responding" -ForegroundColor Red
    }
} catch {
    Write-Host "   Node not responding" -ForegroundColor Red
}

Write-Host ""
Write-Host "2. Peer Connections:" -ForegroundColor Yellow
try {
    & $POSD_BIN status --home $VALIDATOR_HOME 2>&1 | Select-String -Pattern "n_peers"
} catch {
    Write-Host "   No status available" -ForegroundColor Red
}

Write-Host ""
Write-Host "3. Validators:" -ForegroundColor Yellow
try {
    & $POSD_BIN query staking validators --home $VALIDATOR_HOME --output json 2>&1 | Select-String -Pattern "moniker|status|tokens" | Select-Object -First 20
} catch {
    Write-Host "   Cannot query validators" -ForegroundColor Red
}

Write-Host ""
Write-Host "4. Latest Block:" -ForegroundColor Yellow
try {
    & $POSD_BIN query block --home $VALIDATOR_HOME 2>&1 | Select-String -Pattern "height|time|num_txs" | Select-Object -First 5
} catch {
    Write-Host "   Cannot query block" -ForegroundColor Red
}

Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  Quick Health Check" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Get sync info
try {
    $syncInfo = & $POSD_BIN status --home $VALIDATOR_HOME 2>&1 | Out-String

    if ($syncInfo -match "latest_block_height") {
        if ($syncInfo -match '"latest_block_height":"(\d+)"') {
            $height = $matches[1]
        }
        if ($syncInfo -match '"catching_up":(true|false)') {
            $catchingUp = $matches[1]
        }

        Write-Host "✓ Node is running" -ForegroundColor Green
        Write-Host "  Block Height: $height"
        Write-Host "  Catching Up: $catchingUp"

        if ($catchingUp -eq "false") {
            Write-Host "  Status: ✓ SYNCED" -ForegroundColor Green
        } else {
            Write-Host "  Status: ⏳ SYNCING" -ForegroundColor Yellow
        }
    } else {
        Write-Host "✗ Node is not responding" -ForegroundColor Red
        Write-Host "  Make sure the validator is running: .\start_validator1.ps1"
    }
} catch {
    Write-Host "✗ Node is not responding" -ForegroundColor Red
    Write-Host "  Make sure the validator is running: .\start_validator1.ps1"
}

Write-Host ""
Write-Host "For more detailed information, run:"
Write-Host "  $POSD_BIN status --home $VALIDATOR_HOME" -ForegroundColor Cyan
Write-Host ""
