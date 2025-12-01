# 2-Node Testnet Setup Script (Computer 1) - Windows PowerShell
# This script generates configuration for 2 validators on different computers

Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  2-Node Testnet Generator" -ForegroundColor Cyan
Write-Host "  Omniphi Blockchain" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Configuration
$CHAIN_ID = "omniphi-testnet-1"
$OUTPUT_DIR = ".\testnet-2nodes"
$VALIDATOR_STAKE = "100000000000omniphi"  # 100,000 OMNI per validator
$KEYRING_BACKEND = "test"

# Check if posd binary exists
$POSD_BIN = ".\posd.exe"
if (-not (Test-Path $POSD_BIN)) {
    $POSD_BIN = "posd"
    if (-not (Get-Command posd -ErrorAction SilentlyContinue)) {
        Write-Host "Error: posd binary not found!" -ForegroundColor Red
        Write-Host "Please build the binary first: make install or go build -o posd.exe ./cmd/posd"
        exit 1
    }
}

Write-Host "Using binary: $POSD_BIN" -ForegroundColor Green
Write-Host ""

# Clean up old testnet directory if exists
if (Test-Path $OUTPUT_DIR) {
    Write-Host "Removing old testnet directory..." -ForegroundColor Yellow
    Remove-Item -Recurse -Force $OUTPUT_DIR
}

Write-Host "Generating 2-node testnet configuration..." -ForegroundColor Cyan
Write-Host "Chain ID: $CHAIN_ID"
Write-Host "Output Directory: $OUTPUT_DIR"
Write-Host ""

# Try using built-in testnet command
$testnetSuccess = $false
try {
    & $POSD_BIN testnet init-files `
        --v 2 `
        --output-dir $OUTPUT_DIR `
        --chain-id $CHAIN_ID `
        --keyring-backend $KEYRING_BACKEND `
        --starting-ip-address "192.168.1.100" 2>$null
    $testnetSuccess = $?
} catch {
    $testnetSuccess = $false
}

if (-not $testnetSuccess) {
    Write-Host "Built-in testnet command not available. Setting up manually..." -ForegroundColor Yellow

    New-Item -ItemType Directory -Force -Path $OUTPUT_DIR | Out-Null

    # Initialize validator 1
    Write-Host "Initializing Validator 1..." -ForegroundColor Cyan
    & $POSD_BIN init validator1 --chain-id $CHAIN_ID --home "$OUTPUT_DIR\validator1" --overwrite

    # Initialize validator 2
    Write-Host "Initializing Validator 2..." -ForegroundColor Cyan
    & $POSD_BIN init validator2 --chain-id $CHAIN_ID --home "$OUTPUT_DIR\validator2" --overwrite

    # Create keys for validator 1
    Write-Host "Creating validator1 key..." -ForegroundColor Cyan
    & $POSD_BIN keys add validator1 --keyring-backend $KEYRING_BACKEND --home "$OUTPUT_DIR\validator1" | Tee-Object -FilePath "$OUTPUT_DIR\validator1\key_info.txt"

    # Create keys for validator 2
    Write-Host "Creating validator2 key..." -ForegroundColor Cyan
    & $POSD_BIN keys add validator2 --keyring-backend $KEYRING_BACKEND --home "$OUTPUT_DIR\validator2" | Tee-Object -FilePath "$OUTPUT_DIR\validator2\key_info.txt"

    # Get validator addresses
    $VAL1_ADDR = & $POSD_BIN keys show validator1 -a --keyring-backend $KEYRING_BACKEND --home "$OUTPUT_DIR\validator1"
    $VAL2_ADDR = & $POSD_BIN keys show validator2 -a --keyring-backend $KEYRING_BACKEND --home "$OUTPUT_DIR\validator2"

    Write-Host "Validator 1 address: $VAL1_ADDR" -ForegroundColor Green
    Write-Host "Validator 2 address: $VAL2_ADDR" -ForegroundColor Green

    # Add genesis accounts to validator1's genesis
    & $POSD_BIN genesis add-genesis-account $VAL1_ADDR 200000000000omniphi --home "$OUTPUT_DIR\validator1"
    & $POSD_BIN genesis add-genesis-account $VAL2_ADDR 200000000000omniphi --home "$OUTPUT_DIR\validator1"

    # Add a test account with funds
    & $POSD_BIN keys add testuser --keyring-backend $KEYRING_BACKEND --home "$OUTPUT_DIR\validator1" | Tee-Object -FilePath "$OUTPUT_DIR\testuser_key.txt"
    $TEST_ADDR = & $POSD_BIN keys show testuser -a --keyring-backend $KEYRING_BACKEND --home "$OUTPUT_DIR\validator1"
    & $POSD_BIN genesis add-genesis-account $TEST_ADDR 50000000000omniphi --home "$OUTPUT_DIR\validator1"

    # Create gentx for validator 1
    Write-Host "Creating genesis transaction for validator1..." -ForegroundColor Cyan
    & $POSD_BIN genesis gentx validator1 $VALIDATOR_STAKE `
        --chain-id $CHAIN_ID `
        --keyring-backend $KEYRING_BACKEND `
        --home "$OUTPUT_DIR\validator1"

    # Copy validator1's gentx to validator2's gentx directory
    New-Item -ItemType Directory -Force -Path "$OUTPUT_DIR\validator2\config\gentx" | Out-Null
    Copy-Item "$OUTPUT_DIR\validator1\config\gentx\*.json" "$OUTPUT_DIR\validator2\config\gentx\"

    # Create gentx for validator 2
    Write-Host "Creating genesis transaction for validator2..." -ForegroundColor Cyan
    Copy-Item "$OUTPUT_DIR\validator1\config\genesis.json" "$OUTPUT_DIR\validator2\config\genesis.json"

    & $POSD_BIN genesis gentx validator2 $VALIDATOR_STAKE `
        --chain-id $CHAIN_ID `
        --keyring-backend $KEYRING_BACKEND `
        --home "$OUTPUT_DIR\validator2"

    # Copy validator2's gentx to validator1
    Copy-Item "$OUTPUT_DIR\validator2\config\gentx\*.json" "$OUTPUT_DIR\validator1\config\gentx\"

    # Collect all gentxs on validator1
    Write-Host "Collecting genesis transactions..." -ForegroundColor Cyan
    & $POSD_BIN genesis collect-gentxs --home "$OUTPUT_DIR\validator1"

    # Copy final genesis to validator2
    Copy-Item "$OUTPUT_DIR\validator1\config\genesis.json" "$OUTPUT_DIR\validator2\config\genesis.json" -Force

    # Get node IDs
    $NODE1_ID = & $POSD_BIN comet show-node-id --home "$OUTPUT_DIR\validator1"
    $NODE2_ID = & $POSD_BIN comet show-node-id --home "$OUTPUT_DIR\validator2"

    # Configure persistent peers
    $config1 = Get-Content "$OUTPUT_DIR\validator1\config\config.toml" -Raw
    $config1 = $config1 -replace 'persistent_peers = ""', "persistent_peers = `"$NODE2_ID@VALIDATOR2_IP:26656`""
    $config1 | Set-Content "$OUTPUT_DIR\validator1\config\config.toml"

    $config2 = Get-Content "$OUTPUT_DIR\validator2\config\config.toml" -Raw
    $config2 = $config2 -replace 'persistent_peers = ""', "persistent_peers = `"$NODE1_ID@VALIDATOR1_IP:26656`""
    $config2 | Set-Content "$OUTPUT_DIR\validator2\config\config.toml"

    # Enable API and unsafe CORS for testing
    $app1 = Get-Content "$OUTPUT_DIR\validator1\config\app.toml" -Raw
    $app1 = $app1 -replace 'enable = false', 'enable = true'
    $app1 = $app1 -replace 'enabled-unsafe-cors = false', 'enabled-unsafe-cors = true'
    $app1 = $app1 -replace 'minimum-gas-prices = ""', 'minimum-gas-prices = "0.001omniphi"'
    $app1 | Set-Content "$OUTPUT_DIR\validator1\config\app.toml"

    $app2 = Get-Content "$OUTPUT_DIR\validator2\config\app.toml" -Raw
    $app2 = $app2 -replace 'enable = false', 'enable = true'
    $app2 = $app2 -replace 'enabled-unsafe-cors = false', 'enabled-unsafe-cors = true'
    $app2 = $app2 -replace 'minimum-gas-prices = ""', 'minimum-gas-prices = "0.001omniphi"'
    $app2 | Set-Content "$OUTPUT_DIR\validator2\config\app.toml"

    # Allow external RPC connections
    $config1 = Get-Content "$OUTPUT_DIR\validator1\config\config.toml" -Raw
    $config1 = $config1 -replace 'laddr = "tcp://127.0.0.1:26657"', 'laddr = "tcp://0.0.0.0:26657"'
    $config1 | Set-Content "$OUTPUT_DIR\validator1\config\config.toml"

    $config2 = Get-Content "$OUTPUT_DIR\validator2\config\config.toml" -Raw
    $config2 = $config2 -replace 'laddr = "tcp://127.0.0.1:26657"', 'laddr = "tcp://0.0.0.0:26657"'
    $config2 | Set-Content "$OUTPUT_DIR\validator2\config\config.toml"

    # Save node IDs to files
    $NODE1_ID | Set-Content "$OUTPUT_DIR\validator1\node_id.txt"
    $NODE2_ID | Set-Content "$OUTPUT_DIR\validator2\node_id.txt"
}

Write-Host ""
Write-Host "======================================" -ForegroundColor Green
Write-Host "  Testnet Setup Complete!" -ForegroundColor Green
Write-Host "======================================" -ForegroundColor Green
Write-Host ""

# Get node IDs
$NODE1_ID = Get-Content "$OUTPUT_DIR\validator1\node_id.txt" -ErrorAction SilentlyContinue
if (-not $NODE1_ID) {
    $NODE1_ID = & $POSD_BIN comet show-node-id --home "$OUTPUT_DIR\validator1"
}

$NODE2_ID = Get-Content "$OUTPUT_DIR\validator2\node_id.txt" -ErrorAction SilentlyContinue
if (-not $NODE2_ID) {
    $NODE2_ID = & $POSD_BIN comet show-node-id --home "$OUTPUT_DIR\validator2"
}

Write-Host "Configuration created for 2 validators:" -ForegroundColor Cyan
Write-Host ""
Write-Host "Validator 1 (Computer 1 - THIS COMPUTER):" -ForegroundColor Yellow
Write-Host "  Home Directory: $OUTPUT_DIR\validator1"
Write-Host "  Node ID: $NODE1_ID"
Write-Host "  Ports: P2P=26656, RPC=26657, gRPC=9090, API=1317"
Write-Host ""
Write-Host "Validator 2 (Computer 2 - REMOTE COMPUTER):" -ForegroundColor Yellow
Write-Host "  Home Directory: $OUTPUT_DIR\validator2"
Write-Host "  Node ID: $NODE2_ID"
Write-Host "  Ports: P2P=26656, RPC=26657, gRPC=9090, API=1317"
Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  NEXT STEPS:" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "1. Find your Computer 1's IP address:"
Write-Host "   Run: ipconfig"
Write-Host "   Look for 'IPv4 Address'"
Write-Host ""
Write-Host "2. Update peer configurations:"
Write-Host "   Run: .\update_peer_ip.ps1 <COMPUTER1_IP> <COMPUTER2_IP>"
Write-Host "   Example: .\update_peer_ip.ps1 192.168.1.100 192.168.1.101"
Write-Host ""
Write-Host "3. Package validator2 for Computer 2:"
Write-Host "   Run: .\package_validator2.ps1"
Write-Host ""
Write-Host "4. Transfer the package to Computer 2 and extract it"
Write-Host ""
Write-Host "5. Start validators (on both computers):"
Write-Host "   Computer 1: .\start_validator1.ps1"
Write-Host "   Computer 2: .\start_validator2.ps1"
Write-Host ""
Write-Host "6. Verify network is running:"
Write-Host "   Run: .\verify_network.ps1"
Write-Host ""
Write-Host "See MULTI_NODE_TESTNET_GUIDE.md for detailed instructions!" -ForegroundColor Green
Write-Host ""
