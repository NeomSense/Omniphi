# Setup Local Validator Node
# This script properly initializes a single-node blockchain for testing

$HOME_DIR = "$env:USERPROFILE\.omniphi"
$BINARY = ".\build\posd.exe"
$CHAIN_ID = "omniphi-localnet-1"
$MONIKER = "local-validator"
$KEYRING = "test"

Write-Host "Setting up local validator node..." -ForegroundColor Green

# 1. Clean existing data (optional - comment out if you want to keep existing config)
# Remove-Item -Recurse -Force $HOME_DIR -ErrorAction SilentlyContinue

# 2. Initialize node
Write-Host "Initializing node..." -ForegroundColor Yellow
& $BINARY init $MONIKER --chain-id $CHAIN_ID --home $HOME_DIR

# 3. Create a key for the validator
Write-Host "Creating validator key..." -ForegroundColor Yellow
& $BINARY keys add validator --keyring-backend $KEYRING --home $HOME_DIR

# 4. Add genesis account with tokens
Write-Host "Adding genesis account..." -ForegroundColor Yellow
$VALIDATOR_ADDR = & $BINARY keys show validator -a --keyring-backend $KEYRING --home $HOME_DIR
& $BINARY genesis add-genesis-account $VALIDATOR_ADDR 1000000000000omniphi --home $HOME_DIR

# 5. Create genesis transaction (gentx)
Write-Host "Creating genesis transaction..." -ForegroundColor Yellow
& $BINARY genesis gentx validator 100000000000omniphi --chain-id $CHAIN_ID --keyring-backend $KEYRING --home $HOME_DIR

# 6. Collect genesis transactions (this populates the validators array!)
Write-Host "Collecting genesis transactions..." -ForegroundColor Yellow
& $BINARY genesis collect-gentxs --home $HOME_DIR

# 7. Validate genesis
Write-Host "Validating genesis..." -ForegroundColor Yellow
& $BINARY genesis validate --home $HOME_DIR

Write-Host ""
Write-Host "==================================================================" -ForegroundColor Green
Write-Host "Local validator node setup complete!" -ForegroundColor Green
Write-Host "==================================================================" -ForegroundColor Green
Write-Host ""
Write-Host "Your validator address: $VALIDATOR_ADDR" -ForegroundColor Cyan
Write-Host ""
Write-Host "To start the node, run:" -ForegroundColor Yellow
Write-Host "  $BINARY start --home $HOME_DIR" -ForegroundColor White
Write-Host ""
Write-Host "Or use the desktop app to start the validator." -ForegroundColor Yellow
