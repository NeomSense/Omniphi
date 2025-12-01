# Update Peer IP Addresses in Config Files - Windows PowerShell

param(
    [Parameter(Mandatory=$true)]
    [string]$Computer1IP,

    [Parameter(Mandatory=$true)]
    [string]$Computer2IP
)

$OUTPUT_DIR = ".\testnet-2nodes"

if (-not (Test-Path $OUTPUT_DIR)) {
    Write-Host "Error: Testnet directory not found: $OUTPUT_DIR" -ForegroundColor Red
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

# Get node IDs
$NODE1_ID = & $POSD_BIN comet show-node-id --home "$OUTPUT_DIR\validator1"
$NODE2_ID = & $POSD_BIN comet show-node-id --home "$OUTPUT_DIR\validator2"

Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  Updating Peer IP Addresses" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Computer 1 IP: $Computer1IP" -ForegroundColor Green
Write-Host "Computer 2 IP: $Computer2IP" -ForegroundColor Green
Write-Host ""
Write-Host "Node 1 ID: $NODE1_ID" -ForegroundColor Yellow
Write-Host "Node 2 ID: $NODE2_ID" -ForegroundColor Yellow
Write-Host ""

# Update validator1 config (peer to validator2)
$CONFIG1 = "$OUTPUT_DIR\validator1\config\config.toml"
$config1Content = Get-Content $CONFIG1 -Raw
$config1Content = $config1Content -replace 'VALIDATOR2_IP', $Computer2IP
$config1Content = $config1Content -replace 'persistent_peers = "[^"]*"', "persistent_peers = `"$NODE2_ID@$($Computer2IP):26656`""
$config1Content | Set-Content $CONFIG1

# Update validator2 config (peer to validator1)
$CONFIG2 = "$OUTPUT_DIR\validator2\config\config.toml"
$config2Content = Get-Content $CONFIG2 -Raw
$config2Content = $config2Content -replace 'VALIDATOR1_IP', $Computer1IP
$config2Content = $config2Content -replace 'persistent_peers = "[^"]*"', "persistent_peers = `"$NODE1_ID@$($Computer1IP):26656`""
$config2Content | Set-Content $CONFIG2

Write-Host "✓ Updated validator1 persistent peer: $NODE2_ID@$($Computer2IP):26656" -ForegroundColor Green
Write-Host "✓ Updated validator2 persistent peer: $NODE1_ID@$($Computer1IP):26656" -ForegroundColor Green
Write-Host ""
Write-Host "Peer configuration updated successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:"
Write-Host "1. Package validator2: .\package_validator2.ps1"
Write-Host "2. Transfer to Computer 2"
Write-Host "3. Start both validators"
