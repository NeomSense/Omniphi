# Complete Chain Reset & Start Script for Windows
# This script resets the chain and starts it fresh with proper configuration

Write-Host "================================" -ForegroundColor Cyan
Write-Host "Complete Chain Reset & Start" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Function to print status
function Print-Status {
    param([string]$Message)
    Write-Host "✓" -ForegroundColor Green -NoNewline
    Write-Host " $Message"
}

function Print-Error {
    param([string]$Message)
    Write-Host "✗" -ForegroundColor Red -NoNewline
    Write-Host " $Message"
}

function Print-Warning {
    param([string]$Message)
    Write-Host "⚠" -ForegroundColor Yellow -NoNewline
    Write-Host " $Message"
}

# 1. Stop any running chain
Write-Host ""
Write-Host "Step 1: Stopping chain..."
taskkill /F /IM posd.exe 2>&1 | Out-Null
Start-Sleep -Seconds 2
Print-Status "Chain stopped"

# 2. Reset chain data
Write-Host ""
Write-Host "Step 2: Resetting chain data..."
.\posd.exe comet unsafe-reset-all | Out-Null
Print-Status "Chain data reset"

# 3. Initialize genesis
Write-Host ""
Write-Host "Step 3: Initializing genesis..."
.\posd.exe init test-node --chain-id omniphi-1 --default-denom uomni --overwrite | Out-Null
Print-Status "Genesis initialized"

# 4. Clean old gentx files
Write-Host ""
Write-Host "Step 4: Cleaning old gentx files..."
Remove-Item "$env:USERPROFILE\.pos\config\gentx\*.json" -ErrorAction SilentlyContinue
Print-Status "Old gentx files removed"

# 5. Check for validator key, create if doesn't exist
Write-Host ""
Write-Host "Step 5: Setting up validator key..."
$validatorExists = .\posd.exe keys show validator --keyring-backend test 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Creating new validator key..."
    .\posd.exe keys add validator --keyring-backend test 2>&1 | Out-Null
    Print-Status "Validator key created"
} else {
    Print-Status "Validator key already exists"
}

# 6. Add validator account with balance
Write-Host ""
Write-Host "Step 6: Adding validator account to genesis..."
$VALIDATOR_ADDR = (.\posd.exe keys show validator -a --keyring-backend test 2>&1)
if ($LASTEXITCODE -ne 0) {
    Print-Error "Validator key not found in keyring"
    exit 1
}
Write-Host "Validator address: $VALIDATOR_ADDR"

# Give 11M OMNI (11000000000000 uomni)
# After staking 1M, validator will have 10M spendable
.\posd.exe genesis add-genesis-account validator 11000000000000uomni --keyring-backend test | Out-Null

# Verify it was added
$genesisContent = Get-Content "$env:USERPROFILE\.pos\config\genesis.json" -Raw
if ($genesisContent -notmatch $VALIDATOR_ADDR) {
    Print-Error "Failed to add validator to genesis"
    exit 1
}
Print-Status "Validator account added with 11M OMNI"

# 7. Create validator gentx
Write-Host ""
Write-Host "Step 7: Creating validator gentx (staking 1M OMNI)..."
.\posd.exe genesis gentx validator 1000000000000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --moniker "Validator" | Out-Null

$gentxFiles = Get-ChildItem "$env:USERPROFILE\.pos\config\gentx\*.json" -ErrorAction SilentlyContinue
if ($gentxFiles.Count -eq 0) {
    Print-Error "Failed to create gentx"
    exit 1
}
Print-Status "Gentx created"

# 8. Collect gentx
Write-Host ""
Write-Host "Step 8: Collecting gentx..."
.\posd.exe genesis collect-gentxs | Out-Null
Print-Status "Gentx collected"

# 9. Set treasury address in genesis
Write-Host ""
Write-Host "Step 9: Setting treasury address..."

# Create treasury account if it doesn't exist
$treasuryExists = .\posd.exe keys show treasury --keyring-backend test 2>&1
if ($LASTEXITCODE -ne 0) {
    .\posd.exe keys add treasury --keyring-backend test 2>&1 | Out-Null
    $TREASURY_ADDR = (.\posd.exe keys show treasury -a --keyring-backend test 2>&1)
} else {
    $TREASURY_ADDR = (.\posd.exe keys show treasury -a --keyring-backend test 2>&1)
}

Write-Host "Treasury address: $TREASURY_ADDR"

# Set treasury address in genesis
$genesisPath = "$env:USERPROFILE\.pos\config\genesis.json"
$genesisContent = Get-Content $genesisPath -Raw
$genesisContent = $genesisContent -replace '"treasury_address": ""', """treasury_address"": ""$TREASURY_ADDR"""
$genesisContent | Set-Content $genesisPath

# Verify it was set
$genesisContent = Get-Content $genesisPath -Raw
if ($genesisContent -match """treasury_address"": ""$TREASURY_ADDR""") {
    Print-Status "Treasury address set in genesis"
} else {
    Print-Error "Failed to set treasury address in genesis"
    exit 1
}

# 10. Validate genesis
Write-Host ""
Write-Host "Step 10: Validating genesis..."
.\posd.exe genesis validate | Out-Null
if ($LASTEXITCODE -ne 0) {
    Print-Error "Genesis validation failed"
    exit 1
}
Print-Status "Genesis validated successfully"

# 11. Start chain
Write-Host ""
Write-Host "Step 11: Starting chain..."
Start-Process -FilePath ".\posd.exe" -ArgumentList "start --minimum-gas-prices 0.001uomni --grpc.enable=false" -RedirectStandardOutput "chain.log" -RedirectStandardError "chain_error.log" -NoNewWindow
Start-Sleep -Seconds 2
$chainProcess = Get-Process -Name "posd" -ErrorAction SilentlyContinue
if ($chainProcess) {
    Print-Status "Chain started (PID: $($chainProcess.Id))"
} else {
    Print-Error "Chain failed to start"
    exit 1
}

# 12. Wait for chain to be ready
Write-Host ""
Write-Host "Step 12: Waiting for chain to be ready..."
$ready = $false
for ($i = 1; $i -le 30; $i++) {
    Start-Sleep -Seconds 1
    $chainProcess = Get-Process -Name "posd" -ErrorAction SilentlyContinue
    if ($chainProcess) {
        Print-Status "Chain process running"
        $ready = $true
        break
    }
    if ($i -eq 30) {
        Print-Error "Chain failed to start"
        Get-Content "chain.log" -Tail 50
        exit 1
    }
}

# Wait for first block
Write-Host "Waiting for first block..."
$blockProduced = $false
for ($i = 1; $i -le 30; $i++) {
    Start-Sleep -Seconds 1
    $logContent = Get-Content "chain.log" -ErrorAction SilentlyContinue
    if ($logContent -match "finalized block") {
        Print-Status "Chain producing blocks"
        $blockProduced = $true
        break
    }
    if ($i -eq 30) {
        Print-Error "Chain not producing blocks after 30 seconds"
        Get-Content "chain.log" -Tail 50
        exit 1
    }
}

# 13. Verify chain status
Write-Host ""
Write-Host "Step 13: Verifying chain status..."
Start-Sleep -Seconds 2

# Check if chain is responding
.\posd.exe status 2>&1 | Out-Null
if ($LASTEXITCODE -eq 0) {
    Print-Status "Chain RPC responding"
} else {
    Print-Error "Chain RPC not responding"
    exit 1
}

# 14. Check validator balance
Write-Host ""
Write-Host "Step 14: Checking validator balance..."
$balanceOutput = .\posd.exe query bank balances $VALIDATOR_ADDR -o json 2>&1
if ($LASTEXITCODE -eq 0) {
    $balance = ($balanceOutput | ConvertFrom-Json).balances[0].amount
    $balanceOMNI = [math]::Floor([decimal]$balance / 1000000)
    if ($balance -eq "10000000000000") {
        Print-Status "Validator balance correct: 10M OMNI (10M spendable after 1M staked)"
    } else {
        Print-Warning "Validator balance: ${balanceOMNI}M OMNI (expected 10M)"
    }
} else {
    Print-Warning "Could not query balance (chain may still be starting)"
}

# 15. Summary
Write-Host ""
Write-Host "================================" -ForegroundColor Cyan
Write-Host "Chain Setup Complete!" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Chain Status:"
Write-Host "  • Chain ID: omniphi-1"
Write-Host "  • Validator: $VALIDATOR_ADDR"
Write-Host "  • Genesis Balance: 11M OMNI"
Write-Host "  • Staked: 1M OMNI"
Write-Host "  • Spendable: 10M OMNI"
Write-Host ""
Write-Host "Next Steps:"
Write-Host "  1. Monitor blocks: Get-Content chain.log -Wait | Select-String 'finalized block'"
Write-Host "  2. Check balance: .\posd.exe query bank balances $VALIDATOR_ADDR"
Write-Host "  3. Run tests: .\test_modules.ps1"
Write-Host ""
Write-Host "To stop chain: taskkill /F /IM posd.exe"
Write-Host ""
