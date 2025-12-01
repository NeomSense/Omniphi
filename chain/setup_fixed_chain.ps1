# Setup Omniphi Chain with Fixed Reward Denom
# Run this in PowerShell

$ErrorActionPreference = "Stop"

# Set environment variables
$env:BINARY = "$PWD\build\posd.exe"
$env:CHAIN_ID = "omniphi-1"
$env:DENOM = "omniphi"
$env:HOME_DIR = "$env:USERPROFILE\.pos"

Write-Host "=== Setting up Omniphi Chain with Fixed Reward Denom ===" -ForegroundColor Cyan

# Add keys (will prompt for mnemonics or create new)
Write-Host "`nAdding alice key..." -ForegroundColor Yellow
& $env:BINARY keys add alice --keyring-backend test --home $env:HOME_DIR

Write-Host "`nAdding bob key..." -ForegroundColor Yellow
& $env:BINARY keys add bob --keyring-backend test --home $env:HOME_DIR

# Get addresses
$ALICE_ADDR = & $env:BINARY keys show alice -a --keyring-backend test --home $env:HOME_DIR
$BOB_ADDR = & $env:BINARY keys show bob -a --keyring-backend test --home $env:HOME_DIR

Write-Host "`nAlice address: $ALICE_ADDR" -ForegroundColor Green
Write-Host "Bob address: $BOB_ADDR" -ForegroundColor Green

# Add genesis accounts with large balances
Write-Host "`nAdding genesis accounts..." -ForegroundColor Yellow
& $env:BINARY genesis add-genesis-account $ALICE_ADDR "1000000000000$env:DENOM" --home $env:HOME_DIR
& $env:BINARY genesis add-genesis-account $BOB_ADDR "1000000000000$env:DENOM" --home $env:HOME_DIR

# Create gentx
Write-Host "`nCreating validator gentx for alice..." -ForegroundColor Yellow
& $env:BINARY genesis gentx alice "1000000$env:DENOM" `
    --chain-id $env:CHAIN_ID `
    --keyring-backend test `
    --home $env:HOME_DIR

# Collect gentxs
Write-Host "`nCollecting gentxs..." -ForegroundColor Yellow
& $env:BINARY genesis collect-gentxs --home $env:HOME_DIR

# Validate genesis
Write-Host "`nValidating genesis..." -ForegroundColor Yellow
& $env:BINARY genesis validate --home $env:HOME_DIR

# Set minimum gas prices in app.toml
Write-Host "`nSetting minimum gas prices..." -ForegroundColor Yellow
$appTomlPath = "$env:HOME_DIR\config\app.toml"
(Get-Content $appTomlPath) -replace 'minimum-gas-prices = ""', 'minimum-gas-prices = "0.025omniphi"' | Set-Content $appTomlPath

Write-Host "`n=== Setup Complete! ===" -ForegroundColor Green
Write-Host "`nNext steps:" -ForegroundColor Cyan
Write-Host "1. Start the chain: & `$env:BINARY start --home `$env:HOME_DIR" -ForegroundColor White
Write-Host "2. Fund PoC module: & `$env:BINARY tx bank send alice omni1rzyf5us62dlwrk0kmepx32wvl8e7txl7kxehp7 10000000omniphi --from alice --keyring-backend test --chain-id omniphi-1 --gas auto --fees 25000omniphi -y" -ForegroundColor White
Write-Host "3. Submit PoC contribution using bob" -ForegroundColor White
Write-Host "4. Endorse using alice (validator)" -ForegroundColor White
Write-Host "5. Wait for EndBlocker to process rewards" -ForegroundColor White