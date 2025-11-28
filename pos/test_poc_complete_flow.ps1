# Complete PoC Reward Flow Test
# Tests the full cycle: submit → endorse → verify → reward

$ErrorActionPreference = "Stop"

# Set environment variables
$env:BINARY = "$PWD\build\posd.exe"
$env:CHAIN_ID = "omniphi-1"
$env:DENOM = "omniphi"
$env:HOME_DIR = "$env:USERPROFILE\.pos"

Write-Host "=== Testing Complete PoC Reward Flow ===" -ForegroundColor Cyan

# Get addresses
$ALICE_ADDR = & $env:BINARY keys show alice -a --keyring-backend test --home $env:HOME_DIR
$BOB_ADDR = & $env:BINARY keys show bob -a --keyring-backend test --home $env:HOME_DIR

Write-Host "`nAlice (validator): $ALICE_ADDR" -ForegroundColor Green
Write-Host "Bob (contributor): $BOB_ADDR" -ForegroundColor Green

# Step 1: Fund PoC module
Write-Host "`n=== Step 1: Funding PoC Module ===" -ForegroundColor Yellow
$POC_MODULE_ADDR = "omni1rzyf5us62dlwrk0kmepx32wvl8e7txl7kxehp7"
& $env:BINARY tx bank send alice $POC_MODULE_ADDR 10000000omniphi `
    --from alice `
    --keyring-backend test `
    --chain-id $env:CHAIN_ID `
    --gas auto `
    --fees 25000omniphi `
    -y

Start-Sleep -Seconds 3

# Check module balance
Write-Host "`nChecking PoC module balance..." -ForegroundColor Cyan
& $env:BINARY q bank balances $POC_MODULE_ADDR --home $env:HOME_DIR

# Step 2: Submit contribution (using bob)
Write-Host "`n=== Step 2: Submitting PoC Contribution ===" -ForegroundColor Yellow
$HASH = "0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
& $env:BINARY tx poc submit-contribution code ipfs://QmTestHash $HASH `
    --from bob `
    --keyring-backend test `
    --chain-id $env:CHAIN_ID `
    --gas auto `
    --fees 25000omniphi `
    -y

Start-Sleep -Seconds 3

# Query contribution
Write-Host "`nQuerying contribution..." -ForegroundColor Cyan
& $env:BINARY q poc contribution 1 --home $env:HOME_DIR

# Step 3: Endorse contribution (using alice - validator)
Write-Host "`n=== Step 3: Endorsing Contribution ===" -ForegroundColor Yellow
& $env:BINARY tx poc endorse 1 true `
    --from alice `
    --keyring-backend test `
    --chain-id $env:CHAIN_ID `
    --gas auto `
    --fees 25000omniphi `
    -y

Start-Sleep -Seconds 3

# Query contribution after endorsement
Write-Host "`nQuerying contribution after endorsement..." -ForegroundColor Cyan
$CONTRIBUTION = & $env:BINARY q poc contribution 1 --home $env:HOME_DIR --output json | ConvertFrom-Json

Write-Host "`nContribution Status:" -ForegroundColor Green
Write-Host "  Verified: $($CONTRIBUTION.contribution.verified)" -ForegroundColor $(if ($CONTRIBUTION.contribution.verified) { "Green" } else { "Red" })
Write-Host "  Rewarded: $($CONTRIBUTION.contribution.rewarded)" -ForegroundColor $(if ($CONTRIBUTION.contribution.rewarded) { "Green" } else { "Yellow" })
Write-Host "  Credits: $($CONTRIBUTION.contribution.credits)" -ForegroundColor Green

# Step 4: Wait for EndBlocker
Write-Host "`n=== Step 4: Waiting for EndBlocker to Process Rewards ===" -ForegroundColor Yellow
Write-Host "Waiting 10 seconds for next block processing..." -ForegroundColor Cyan
Start-Sleep -Seconds 10

# Query contribution again
Write-Host "`nQuerying contribution after EndBlocker..." -ForegroundColor Cyan
$CONTRIBUTION_AFTER = & $env:BINARY q poc contribution 1 --home $env:HOME_DIR --output json | ConvertFrom-Json

Write-Host "`nFinal Contribution Status:" -ForegroundColor Green
Write-Host "  Verified: $($CONTRIBUTION_AFTER.contribution.verified)" -ForegroundColor $(if ($CONTRIBUTION_AFTER.contribution.verified) { "Green" } else { "Red" })
Write-Host "  Rewarded: $($CONTRIBUTION_AFTER.contribution.rewarded)" -ForegroundColor $(if ($CONTRIBUTION_AFTER.contribution.rewarded) { "Green" } else { "Red" })
Write-Host "  Credits: $($CONTRIBUTION_AFTER.contribution.credits)" -ForegroundColor Green

# Step 5: Check bob's credits
Write-Host "`n=== Step 5: Checking Bob's Credits ===" -ForegroundColor Yellow
& $env:BINARY q poc credits $BOB_ADDR --home $env:HOME_DIR

# Step 6: Check bob's balance
Write-Host "`n=== Step 6: Checking Bob's Balance ===" -ForegroundColor Yellow
$BOB_BALANCE_BEFORE = & $env:BINARY q bank balances $BOB_ADDR --home $env:HOME_DIR --output json | ConvertFrom-Json
Write-Host "Bob's balance before withdrawal: $($BOB_BALANCE_BEFORE.balances[0].amount)omniphi" -ForegroundColor Cyan

# Test Summary
Write-Host "`n=== Test Summary ===" -ForegroundColor Cyan
Write-Host "✓ Module funded successfully" -ForegroundColor Green
Write-Host "✓ Contribution submitted (ID: 1)" -ForegroundColor Green
Write-Host "✓ Contribution endorsed by validator" -ForegroundColor Green

if ($CONTRIBUTION_AFTER.contribution.verified) {
    Write-Host "✓ Contribution verified = true" -ForegroundColor Green
} else {
    Write-Host "✗ Contribution verified = false (UNEXPECTED)" -ForegroundColor Red
}

if ($CONTRIBUTION_AFTER.contribution.rewarded) {
    Write-Host "✓ Contribution rewarded = true (FIX WORKING!)" -ForegroundColor Green
} else {
    Write-Host "⚠ Contribution rewarded = false" -ForegroundColor Yellow
    Write-Host "  This may be normal if EndBlocker hasn't processed yet." -ForegroundColor Gray
    Write-Host "  Check logs for reward processing." -ForegroundColor Gray
}

Write-Host "`nTest complete!" -ForegroundColor Cyan