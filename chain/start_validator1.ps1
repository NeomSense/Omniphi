# Start Validator 1 (Computer 1) - Windows PowerShell

$VALIDATOR_HOME = ".\testnet-2nodes\validator1"

# Check if validator directory exists
if (-not (Test-Path $VALIDATOR_HOME)) {
    Write-Host "Error: Validator directory not found: $VALIDATOR_HOME" -ForegroundColor Red
    Write-Host "Please run setup_2node_testnet.ps1 first!"
    exit 1
}

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
Write-Host "  Starting Validator 1" -ForegroundColor Cyan
Write-Host "  Omniphi Testnet" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Home Directory: $VALIDATOR_HOME" -ForegroundColor Green
Write-Host "Binary: $POSD_BIN" -ForegroundColor Green
Write-Host ""
Write-Host "Ports:" -ForegroundColor Yellow
Write-Host "  P2P: 26656"
Write-Host "  RPC: 26657"
Write-Host "  gRPC: 9090"
Write-Host "  API: 1317"
Write-Host ""
Write-Host "Press Ctrl+C to stop the validator" -ForegroundColor Yellow
Write-Host ""

# Start the validator
& $POSD_BIN start `
    --home $VALIDATOR_HOME `
    --minimum-gas-prices "0.001uomni"
