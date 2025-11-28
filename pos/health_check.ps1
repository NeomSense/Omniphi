# Omniphi Blockchain Health Check - Windows PowerShell

Write-Host "=== Omniphi Blockchain Health Check ===" -ForegroundColor Cyan
Write-Host ""

# Determine binary
$POSD_BIN = ".\posd.exe"
if (-not (Test-Path $POSD_BIN)) {
    $POSD_BIN = "posd"
    if (-not (Get-Command posd -ErrorAction SilentlyContinue)) {
        Write-Host "✗ posd binary not found!" -ForegroundColor Red
        exit 1
    }
}

try {
    $status = & $POSD_BIN status --home .\testnet-2nodes\validator1 2>&1 | ConvertFrom-Json

    $height = $status.sync_info.latest_block_height
    $catching = $status.sync_info.catching_up
    $peers = if ($status.node_info.other.n_peers) { $status.node_info.other.n_peers } else { "0" }

    Write-Host "✓ Node is running" -ForegroundColor Green
    Write-Host "  Block Height: $height"
    Write-Host "  Peers: $peers"

    if ($catching -eq $false) {
        if ([int]$height -gt 0) {
            Write-Host "  Status: ✓ SYNCED AND PRODUCING BLOCKS" -ForegroundColor Green
        } else {
            Write-Host "  Status: ⏳ WAITING FOR PEER" -ForegroundColor Yellow
        }
    } else {
        Write-Host "  Status: ⏳ SYNCING" -ForegroundColor Yellow
    }
}
catch {
    Write-Host "✗ Node is not responding" -ForegroundColor Red
    Write-Host "  Make sure the validator is running:"
    Write-Host "  .\start_validator1.ps1"
}

Write-Host ""
