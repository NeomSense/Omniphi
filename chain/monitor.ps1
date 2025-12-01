# Continuous Blockchain Monitor - Windows PowerShell

# Determine binary
$POSD_BIN = ".\posd.exe"
if (-not (Test-Path $POSD_BIN)) {
    $POSD_BIN = "posd"
    if (-not (Get-Command posd -ErrorAction SilentlyContinue)) {
        Write-Host "Error: posd binary not found!" -ForegroundColor Red
        exit 1
    }
}

Write-Host "Starting continuous monitor..."
Write-Host "Press Ctrl+C to stop"
Write-Host ""
Start-Sleep -Seconds 2

while ($true) {
    Clear-Host
    Write-Host "===================================================" -ForegroundColor Cyan
    Write-Host "       Omniphi Testnet - Live Monitor" -ForegroundColor Cyan
    Write-Host "===================================================" -ForegroundColor Cyan
    Write-Host ""

    try {
        $status = & $POSD_BIN status --home .\testnet-2nodes\validator1 2>&1 | ConvertFrom-Json

        $height = $status.sync_info.latest_block_height
        $time = $status.sync_info.latest_block_time
        $catching = $status.sync_info.catching_up
        $hash = $status.sync_info.latest_block_hash
        $peers = if ($status.node_info.other.n_peers) { $status.node_info.other.n_peers } else { "unknown" }

        Write-Host "Block Height:    " -NoNewline
        Write-Host $height -ForegroundColor Green

        Write-Host "Block Time:      " -NoNewline
        Write-Host $time -ForegroundColor Yellow

        Write-Host "Block Hash:      " -NoNewline
        Write-Host $hash.Substring(0, 16)"..." -ForegroundColor Gray

        Write-Host ""

        Write-Host "Sync Status:     " -NoNewline
        if ($catching -eq $false) {
            Write-Host "✓ SYNCED" -ForegroundColor Green
        } else {
            Write-Host "⏳ SYNCING" -ForegroundColor Yellow
        }

        Write-Host "Peers Connected: " -NoNewline
        Write-Host $peers -ForegroundColor Cyan

        Write-Host ""
        Write-Host "---------------------------------------------------" -ForegroundColor DarkGray
        Write-Host "Updating every 3 seconds... (Ctrl+C to exit)" -ForegroundColor DarkGray

    }
    catch {
        Write-Host "✗ Unable to connect to validator" -ForegroundColor Red
        Write-Host ""
        Write-Host "Make sure validator is running:"
        Write-Host "  .\start_validator1.ps1"
    }

    Start-Sleep -Seconds 3
}
