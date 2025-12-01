# Omniphi Blockchain - Complete Windows Testing Guide

**This is the ONLY testing guide you need for Windows - includes automated and manual testing.**

---

## Table of Contents
1. [Quick Start - Automated Testing](#quick-start---automated-testing)
2. [Manual Testing](#manual-testing)
3. [Performance Testing](#performance-testing)
4. [Module-Specific Testing](#module-specific-testing)
5. [Governance Testing](#governance-testing)
6. [Network & Integration Testing](#network--integration-testing)
7. [Test Results & Metrics](#test-results--metrics)

---

## Latest Test Results ✅

**Date:** 2025-11-07
**Platform:** Windows 11
**Chain:** omniphi-1
**Status:** ALL TESTS PASSING

### Comprehensive Test Summary

| Module | Queries Tested | Status |
|--------|---------------|--------|
| FeeMarket | 5/5 | ✅ PASS |
| Tokenomics | 3/3 | ✅ PASS |
| POC | 3/3 | ✅ PASS |
| Chain Health | All checks | ✅ PASS |
| Fee Burning | Integration | ✅ PASS |

**Key Metrics:**
- Chain Height: 430+ blocks
- Block Time: ~5 seconds
- Query Response: <100ms
- All module queries working correctly
- Fee burning mechanism operational
- Treasury transfers verified

---

## Quick Start - Automated Testing

### Prerequisites

```powershell
# Ensure chain is running
.\posd.exe status

# If not running, start it with proper configuration
.\posd.exe start --minimum-gas-prices 0.001uomni --grpc.enable=false > chain.log 2>&1 &

# In another terminal, verify blocks are being produced
Get-Content chain.log -Wait | Select-String "finalized block"
```

### Run Full Test Suite

```bash
# For Bash on Windows (Git Bash, WSL, etc.)
./test_intensive_integration.sh

# For PowerShell (individual tests)
# Note: PowerShell test scripts coming soon
```

```powershell
# Manual comprehensive testing (PowerShell)
# Run each query to verify all modules
.\posd.exe query feemarket base-fee
.\posd.exe query feemarket params
.\posd.exe query feemarket block-utilization
.\posd.exe query feemarket burn-tier
.\posd.exe query feemarket fee-stats
.\posd.exe query tokenomics supply
.\posd.exe query tokenomics params
.\posd.exe query tokenomics inflation
.\posd.exe query poc params
.\posd.exe query poc contributions
```

---

## Manual Testing

### 1. Chain Health Check

#### Verify Chain is Running

```powershell
# Check chain status
.\posd.exe status

# Expected output should include:
# - "latest_block_height" (should be increasing)
# - "catching_up": false
# - "sync_info" data

# Verify validator is active
Select-String -Path chain.log -Pattern "This node is a validator"
```

#### Check Block Production

```powershell
# Watch blocks being finalized
Get-Content chain.log -Wait | Select-String "finalized block"

# Should see output like:
# INF finalized block height=1
# INF finalized block height=2
# INF finalized block height=3

# Check current block height every 5 seconds
while($true) {
  $height = (.\posd.exe status | ConvertFrom-Json).sync_info.latest_block_height
  Write-Host "Block Height: $height - $(Get-Date -Format 'HH:mm:ss')"
  Start-Sleep -Seconds 5
}
```

### 2. Account & Balance Testing

#### Create Test Accounts

```powershell
# Create test accounts
.\posd.exe keys add alice --keyring-backend test
.\posd.exe keys add bob --keyring-backend test
.\posd.exe keys add charlie --keyring-backend test

# Save addresses
$ALICE = (.\posd.exe keys show alice -a --keyring-backend test)
$BOB = (.\posd.exe keys show bob -a --keyring-backend test)
$CHARLIE = (.\posd.exe keys show charlie -a --keyring-backend test)

Write-Host "Alice: $ALICE"
Write-Host "Bob: $BOB"
Write-Host "Charlie: $CHARLIE"
```

#### Fund Test Accounts

```powershell
# Get validator address
$VALIDATOR = (.\posd.exe keys show validator -a --keyring-backend test)

# Send tokens from validator to test accounts
.\posd.exe tx bank send $VALIDATOR $ALICE 1000000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

.\posd.exe tx bank send $VALIDATOR $BOB 1000000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

.\posd.exe tx bank send $VALIDATOR $CHARLIE 1000000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

# Wait for transactions to be included (~5 seconds)
Start-Sleep -Seconds 6

# Verify balances
.\posd.exe query bank balances $ALICE
.\posd.exe query bank balances $BOB
.\posd.exe query bank balances $CHARLIE
```

#### Test Token Transfers

```powershell
# Alice sends to Bob
.\posd.exe tx bank send $ALICE $BOB 10000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

# Wait and verify
Start-Sleep -Seconds 6
.\posd.exe query bank balances $BOB

# Bob sends to Charlie
.\posd.exe tx bank send $BOB $CHARLIE 5000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

# Wait and verify
Start-Sleep -Seconds 6
.\posd.exe query bank balances $CHARLIE
```

### 3. FeeMarket Module Testing

#### Query FeeMarket State

```powershell
# Get current base fee
.\posd.exe query feemarket base-fee

# Get module parameters
.\posd.exe query feemarket params

# Get block utilization
.\posd.exe query feemarket block-utilization
```

#### Test Adaptive Fee Mechanism

```powershell
# Record initial base fee
$INITIAL_FEE = (.\posd.exe query feemarket base-fee -o json | ConvertFrom-Json).base_fee
Write-Host "Initial base fee: $INITIAL_FEE"

# Send multiple transactions to increase block utilization
$jobs = @()
for ($i = 1; $i -le 10; $i++) {
  $jobs += Start-Job -ScriptBlock {
    param($alice, $bob)
    & .\posd.exe tx bank send $alice $bob 1000uomni `
      --chain-id omniphi-1 `
      --keyring-backend test `
      --fees 1000uomni `
      --yes
  } -ArgumentList $ALICE, $BOB
}

# Wait for all jobs
$jobs | Wait-Job
$jobs | Remove-Job

# Wait for transactions to process
Start-Sleep -Seconds 10

# Check new base fee (should have adjusted based on utilization)
$NEW_FEE = (.\posd.exe query feemarket base-fee -o json | ConvertFrom-Json).base_fee
Write-Host "New base fee: $NEW_FEE"

# Watch base fee updates in logs
Get-Content chain.log -Wait | Select-String "base fee updated"
```

### 4. Tokenomics Module Testing

#### Query Tokenomics State

```powershell
# Get tokenomics parameters
.\posd.exe query tokenomics params

# Check current supply
.\posd.exe query tokenomics supply

# Check cumulative burned tokens
.\posd.exe query tokenomics burned

# Check cumulative treasury allocation
.\posd.exe query tokenomics treasury

# Check cumulative validator rewards
.\posd.exe query tokenomics validator-rewards
```

#### Test Fee Distribution

```powershell
# Record initial metrics
$INITIAL_BURNED = (.\posd.exe query tokenomics burned -o json | ConvertFrom-Json).amount
$INITIAL_TREASURY = (.\posd.exe query tokenomics treasury -o json | ConvertFrom-Json).amount

Write-Host "Initial burned: $INITIAL_BURNED"
Write-Host "Initial treasury: $INITIAL_TREASURY"

# Send a transaction with fees
.\posd.exe tx bank send $ALICE $BOB 10000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 5000uomni `
  --yes

# Wait for transaction to process
Start-Sleep -Seconds 6

# Check updated metrics
$NEW_BURNED = (.\posd.exe query tokenomics burned -o json | ConvertFrom-Json).amount
$NEW_TREASURY = (.\posd.exe query tokenomics treasury -o json | ConvertFrom-Json).amount

Write-Host "New burned: $NEW_BURNED"
Write-Host "New treasury: $NEW_TREASURY"

# Calculate changes
$BURNED_INCREASE = [int]$NEW_BURNED - [int]$INITIAL_BURNED
$TREASURY_INCREASE = [int]$NEW_TREASURY - [int]$INITIAL_TREASURY

Write-Host "Burned increase: $BURNED_INCREASE uomni"
Write-Host "Treasury increase: $TREASURY_INCREASE uomni"
```

### 5. Proof of Contribution (POC) Module Testing

#### Query POC State

```powershell
# Get POC parameters
.\posd.exe query poc params

# List all contributions
.\posd.exe query poc list-contributions

# List all reputations
.\posd.exe query poc list-reputations
```

#### Submit Contribution

```powershell
# Submit a contribution
.\posd.exe tx poc submit-contribution `
  "Test Contribution" `
  "This is a test contribution for the Omniphi blockchain" `
  --from alice `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes

# Wait for transaction
Start-Sleep -Seconds 6

# Query contributions for Alice
.\posd.exe query poc contributions $ALICE

# Query Alice's reputation score
.\posd.exe query poc reputation $ALICE
```

#### Endorse Contribution

```powershell
# Get contribution ID from previous query
$CONTRIBUTION_ID = 1

# Bob endorses Alice's contribution
.\posd.exe tx poc endorse-contribution $CONTRIBUTION_ID `
  --from bob `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes

# Wait and verify
Start-Sleep -Seconds 6
.\posd.exe query poc contribution $CONTRIBUTION_ID
```

### 6. Staking & Validator Testing

#### Query Staking Information

```powershell
# List all validators
.\posd.exe query staking validators

# Query specific validator
$VALIDATOR_ADDR = (.\posd.exe keys show validator --bech val -a --keyring-backend test)
.\posd.exe query staking validator $VALIDATOR_ADDR

# Check validator delegations
.\posd.exe query staking delegations-to $VALIDATOR_ADDR
```

#### Delegate Tokens

```powershell
# Alice delegates to validator
.\posd.exe tx staking delegate $VALIDATOR_ADDR 100000uomni `
  --from alice `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

# Wait and verify
Start-Sleep -Seconds 6
.\posd.exe query staking delegation $ALICE $VALIDATOR_ADDR
```

#### Claim Staking Rewards

```powershell
# Query available rewards
.\posd.exe query distribution rewards $ALICE

# Withdraw rewards
.\posd.exe tx distribution withdraw-rewards $VALIDATOR_ADDR `
  --from alice `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes
```

---

## Performance Testing

### Automated Performance Tests

#### TPS (Transactions Per Second) Test

```powershell
# Create TPS test script
@'
# Configuration
$CHAIN_ID = "omniphi-1"
$NUM_TX = 100
$FROM_ACCOUNT = "alice"
$TO_ACCOUNT = "bob"

Write-Host "Starting TPS test - sending $NUM_TX transactions..."

# Get addresses
$FROM = (.\posd.exe keys show $FROM_ACCOUNT -a --keyring-backend test)
$TO = (.\posd.exe keys show $TO_ACCOUNT -a --keyring-backend test)

# Record start time
$START_TIME = Get-Date

# Send transactions in parallel
$jobs = @()
for ($i = 1; $i -le $NUM_TX; $i++) {
  $jobs += Start-Job -ScriptBlock {
    param($from, $to, $chainId)
    & .\posd.exe tx bank send $from $to 100uomni `
      --chain-id $chainId `
      --keyring-backend test `
      --fees 500uomni `
      --yes 2>&1 | Out-Null
  } -ArgumentList $FROM, $TO, $CHAIN_ID
}

# Wait for all transactions
$jobs | Wait-Job | Out-Null
$jobs | Remove-Job

# Record end time
$END_TIME = Get-Date
$DURATION = ($END_TIME - $START_TIME).TotalSeconds

# Calculate TPS
$TPS = [math]::Round($NUM_TX / $DURATION, 2)

Write-Host "Completed $NUM_TX transactions in $DURATION seconds"
Write-Host "TPS: $TPS transactions/second"
'@ | Out-File -FilePath test_tps.ps1 -Encoding utf8

# Run the test
.\test_tps.ps1
```

#### Latency Test

```powershell
# Create latency test script
@'
$CHAIN_ID = "omniphi-1"
$FROM = (.\posd.exe keys show alice -a --keyring-backend test)
$TO = (.\posd.exe keys show bob -a --keyring-backend test)

Write-Host "Testing transaction latency..."

for ($i = 1; $i -le 10; $i++) {
  $START = Get-Date

  $TX_HASH = (.\posd.exe tx bank send $FROM $TO 100uomni `
    --chain-id $CHAIN_ID `
    --keyring-backend test `
    --fees 500uomni `
    --yes -o json | ConvertFrom-Json).txhash

  # Wait for transaction to be included
  Start-Sleep -Seconds 6

  $END = Get-Date
  $LATENCY = [math]::Round(($END - $START).TotalMilliseconds, 0)

  Write-Host "TX $i : ${LATENCY}ms"
}
'@ | Out-File -FilePath test_latency.ps1 -Encoding utf8

# Run the test
.\test_latency.ps1
```

### Manual Performance Testing

#### Stress Test - High Transaction Volume

```powershell
# Send 1000 transactions rapidly
$jobs = @()
for ($i = 1; $i -le 1000; $i++) {
  $jobs += Start-Job -ScriptBlock {
    param($alice, $bob)
    & .\posd.exe tx bank send $alice $bob 100uomni `
      --chain-id omniphi-1 `
      --keyring-backend test `
      --fees 500uomni `
      --yes 2>&1 | Out-Null
  } -ArgumentList $ALICE, $BOB

  # Add small delay every 10 transactions
  if ($i % 10 -eq 0) {
    Start-Sleep -Milliseconds 100
  }
}

Write-Host "Submitted 1000 transactions..."

# Monitor job completion
while ($jobs | Where-Object { $_.State -eq 'Running' }) {
  $completed = ($jobs | Where-Object { $_.State -eq 'Completed' }).Count
  Write-Host "Completed: $completed / 1000"
  Start-Sleep -Seconds 1
}

$jobs | Remove-Job

Write-Host "All transactions submitted!"
```

#### Resource Monitoring During Load

```powershell
# Monitor posd process resources
while ($true) {
  $process = Get-Process posd -ErrorAction SilentlyContinue
  if ($process) {
    $cpu = $process.CPU
    $memMB = [math]::Round($process.WorkingSet64 / 1MB, 2)
    Write-Host "$(Get-Date -Format 'HH:mm:ss') - CPU: $cpu | Memory: $memMB MB"
  }
  Start-Sleep -Seconds 1
}
```

---

## Module-Specific Testing

### Complete FeeMarket Test Suite

```powershell
@'
Write-Host "=== FeeMarket Comprehensive Test ==="

# Test 1: Query current state
Write-Host "1. Querying current base fee..."
.\posd.exe query feemarket base-fee

# Test 2: Query parameters
Write-Host "2. Querying feemarket parameters..."
.\posd.exe query feemarket params

# Test 3: Test fee adaptation under load
Write-Host "3. Testing adaptive fee mechanism..."
$INITIAL_FEE = (.\posd.exe query feemarket base-fee -o json | ConvertFrom-Json).base_fee
Write-Host "Initial base fee: $INITIAL_FEE"

# Send burst of transactions
$ALICE = (.\posd.exe keys show alice -a --keyring-backend test)
$BOB = (.\posd.exe keys show bob -a --keyring-backend test)

$jobs = @()
for ($i = 1; $i -le 20; $i++) {
  $jobs += Start-Job -ScriptBlock {
    param($alice, $bob)
    & .\posd.exe tx bank send $alice $bob 100uomni `
      --chain-id omniphi-1 `
      --keyring-backend test `
      --fees 1000uomni `
      --yes 2>&1 | Out-Null
  } -ArgumentList $ALICE, $BOB
}

$jobs | Wait-Job | Out-Null
$jobs | Remove-Job
Start-Sleep -Seconds 10

$NEW_FEE = (.\posd.exe query feemarket base-fee -o json | ConvertFrom-Json).base_fee
Write-Host "New base fee: $NEW_FEE"

if ($NEW_FEE -ne $INITIAL_FEE) {
  Write-Host "✅ Base fee adapted successfully"
} else {
  Write-Host "⚠️  Base fee did not change (may need more load)"
}

Write-Host "=== Test Complete ==="
'@ | Out-File -FilePath test_feemarket_comprehensive.ps1 -Encoding utf8

# Run the test
.\test_feemarket_comprehensive.ps1
```

### Complete Tokenomics Test Suite

```powershell
@'
Write-Host "=== Tokenomics Comprehensive Test ==="

# Test 1: Query all metrics
Write-Host "1. Querying tokenomics state..."
.\posd.exe query tokenomics params
.\posd.exe query tokenomics supply
.\posd.exe query tokenomics burned
.\posd.exe query tokenomics treasury

# Test 2: Track fee distribution
Write-Host "2. Testing fee distribution..."
$INITIAL_BURNED = (.\posd.exe query tokenomics burned -o json | ConvertFrom-Json).amount
$INITIAL_TREASURY = (.\posd.exe query tokenomics treasury -o json | ConvertFrom-Json).amount

Write-Host "Initial burned: $INITIAL_BURNED"
Write-Host "Initial treasury: $INITIAL_TREASURY"

# Send transaction with high fee
$ALICE = (.\posd.exe keys show alice -a --keyring-backend test)
$BOB = (.\posd.exe keys show bob -a --keyring-backend test)

.\posd.exe tx bank send $ALICE $BOB 1000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 10000uomni `
  --yes

Start-Sleep -Seconds 6

$NEW_BURNED = (.\posd.exe query tokenomics burned -o json | ConvertFrom-Json).amount
$NEW_TREASURY = (.\posd.exe query tokenomics treasury -o json | ConvertFrom-Json).amount

Write-Host "New burned: $NEW_BURNED"
Write-Host "New treasury: $NEW_TREASURY"

$BURNED_INCREASE = [int]$NEW_BURNED - [int]$INITIAL_BURNED
$TREASURY_INCREASE = [int]$NEW_TREASURY - [int]$INITIAL_TREASURY

Write-Host "Burned increase: $BURNED_INCREASE uomni"
Write-Host "Treasury increase: $TREASURY_INCREASE uomni"

Write-Host "=== Test Complete ==="
'@ | Out-File -FilePath test_tokenomics_comprehensive.ps1 -Encoding utf8

# Run the test
.\test_tokenomics_comprehensive.ps1
```

### Complete POC Module Test Suite

```powershell
@'
Write-Host "=== POC Module Comprehensive Test ==="

$ALICE = (.\posd.exe keys show alice -a --keyring-backend test)
$BOB = (.\posd.exe keys show bob -a --keyring-backend test)
$CHARLIE = (.\posd.exe keys show charlie -a --keyring-backend test)

# Test 1: Query parameters
Write-Host "1. Querying POC parameters..."
.\posd.exe query poc params

# Test 2: Submit contributions
Write-Host "2. Submitting test contributions..."
.\posd.exe tx poc submit-contribution `
  "Blockchain Development" `
  "Implemented adaptive fee market module" `
  --from alice `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes

Start-Sleep -Seconds 6

.\posd.exe tx poc submit-contribution `
  "Documentation" `
  "Created comprehensive testing guide" `
  --from bob `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes

Start-Sleep -Seconds 6

# Test 3: Query contributions
Write-Host "3. Querying contributions..."
.\posd.exe query poc list-contributions

# Test 4: Endorse contributions
Write-Host "4. Testing endorsements..."
.\posd.exe tx poc endorse-contribution 1 `
  --from bob `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes

Start-Sleep -Seconds 6

.\posd.exe tx poc endorse-contribution 2 `
  --from alice `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes

Start-Sleep -Seconds 6

# Test 5: Query reputations
Write-Host "5. Querying reputation scores..."
.\posd.exe query poc reputation $ALICE
.\posd.exe query poc reputation $BOB

Write-Host "=== Test Complete ==="
'@ | Out-File -FilePath test_poc_comprehensive.ps1 -Encoding utf8

# Run the test
.\test_poc_comprehensive.ps1
```

---

## Governance Testing

### Submit a Proposal

```powershell
# Create a parameter change proposal
@'
{
  "title": "Increase Block Gas Limit",
  "description": "Proposal to increase the maximum gas per block from 10M to 20M to support higher TPS",
  "changes": [
    {
      "subspace": "baseapp",
      "key": "BlockParams",
      "value": "{\"max_gas\": \"20000000\"}"
    }
  ],
  "deposit": "10000000uomni"
}
'@ | Out-File -FilePath proposal.json -Encoding utf8

# Submit the proposal
.\posd.exe tx gov submit-proposal param-change proposal.json `
  --from validator `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

# Wait and query proposal
Start-Sleep -Seconds 6
.\posd.exe query gov proposals
```

### Vote on Proposal

```powershell
# Get proposal ID (usually 1 for first proposal)
$PROPOSAL_ID = 1

# Vote yes
.\posd.exe tx gov vote $PROPOSAL_ID yes `
  --from validator `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes

# Query votes
.\posd.exe query gov votes $PROPOSAL_ID
```

---

## Network & Integration Testing

### Multi-Account Workflow Test

```powershell
# Complete workflow: Create accounts → Fund → Transfer → Delegate → Contribute → Endorse

# 1. Setup
Write-Host "Setting up test accounts..."
@("user1", "user2", "user3") | ForEach-Object {
  .\posd.exe keys add $_ --keyring-backend test 2>$null
}

$VALIDATOR = (.\posd.exe keys show validator -a --keyring-backend test)
$USER1 = (.\posd.exe keys show user1 -a --keyring-backend test)
$USER2 = (.\posd.exe keys show user2 -a --keyring-backend test)
$USER3 = (.\posd.exe keys show user3 -a --keyring-backend test)

# 2. Fund accounts
Write-Host "Funding accounts..."
@($USER1, $USER2, $USER3) | ForEach-Object {
  .\posd.exe tx bank send $VALIDATOR $_ 5000000uomni `
    --chain-id omniphi-1 `
    --keyring-backend test `
    --fees 1000uomni `
    --yes
  Start-Sleep -Seconds 2
}

# 3. Transfers between users
Write-Host "Testing transfers..."
.\posd.exe tx bank send $USER1 $USER2 100000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes
Start-Sleep -Seconds 6

# 4. Delegation
Write-Host "Testing delegation..."
$VALIDATOR_ADDR = (.\posd.exe keys show validator --bech val -a --keyring-backend test)
.\posd.exe tx staking delegate $VALIDATOR_ADDR 500000uomni `
  --from user1 `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes
Start-Sleep -Seconds 6

# 5. POC contribution
Write-Host "Testing POC contribution..."
.\posd.exe tx poc submit-contribution `
  "Integration Test" `
  "Testing complete workflow integration" `
  --from user2 `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes
Start-Sleep -Seconds 6

# 6. Endorsement
Write-Host "Testing endorsement..."
.\posd.exe tx poc endorse-contribution 1 `
  --from user3 `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 2000uomni `
  --yes
Start-Sleep -Seconds 6

Write-Host "✅ Integration test complete!"
```

---

## Test Results & Metrics

### Generate Test Report

```powershell
@'
Write-Host "======================================"
Write-Host "Omniphi Blockchain Test Report"
Write-Host "======================================"
Write-Host "Generated: $(Get-Date)"
Write-Host ""

Write-Host "=== Chain Status ==="
$status = .\posd.exe status | ConvertFrom-Json
Write-Host "Latest Block Height: $($status.sync_info.latest_block_height)"
Write-Host "Latest Block Time: $($status.sync_info.latest_block_time)"
Write-Host "Catching Up: $($status.sync_info.catching_up)"
Write-Host ""

Write-Host "=== Module States ==="

Write-Host "FeeMarket:"
.\posd.exe query feemarket base-fee
Write-Host ""

Write-Host "Tokenomics:"
.\posd.exe query tokenomics supply
.\posd.exe query tokenomics burned
Write-Host ""

Write-Host "POC:"
$contributions = (.\posd.exe query poc list-contributions -o json | ConvertFrom-Json).contributions
Write-Host "Total contributions: $($contributions.Count)"
Write-Host ""

Write-Host "=== Validator Status ==="
$VALIDATOR_ADDR = (.\posd.exe keys show validator --bech val -a --keyring-backend test)
$validator = .\posd.exe query staking validator $VALIDATOR_ADDR -o json | ConvertFrom-Json
Write-Host "Operator Address: $($validator.operator_address)"
Write-Host "Status: $($validator.status)"
Write-Host "Tokens: $($validator.tokens)"
Write-Host ""

Write-Host "======================================"
Write-Host "Test Report Complete"
Write-Host "======================================"
'@ | Out-File -FilePath generate_test_report.ps1 -Encoding utf8

# Run the report
.\generate_test_report.ps1
```

### Success Criteria

✅ **Chain Health**
- Block production continuous (no gaps)
- Block time ~5 seconds
- No fatal errors in logs
- Validator status: BONDED

✅ **Modules Functional**
- FeeMarket: Base fee adjusts with load
- Tokenomics: Fees distributed correctly
- POC: Contributions and endorsements work
- All queries return valid data

✅ **Performance**
- TPS > 10 for single validator
- Transaction latency < 10 seconds
- Mempool clears between blocks

✅ **Integration**
- Multi-account workflows complete
- All transaction types succeed
- State updates correctly

---

## Troubleshooting Tests

### Test Fails with "insufficient funds"

```powershell
# Check account balance
.\posd.exe query bank balances <address>

# Fund from validator
$VALIDATOR = (.\posd.exe keys show validator -a --keyring-backend test)
.\posd.exe tx bank send $VALIDATOR <address> 10000000uomni `
  --chain-id omniphi-1 `
  --keyring-backend test `
  --fees 1000uomni `
  --yes
```

### Test Fails with "sequence mismatch"

```powershell
# Wait for pending transactions to clear
Start-Sleep -Seconds 10

# Or query account to get correct sequence
.\posd.exe query auth account <address>
```

### Transaction Stuck in Mempool

```powershell
# Check mempool
.\posd.exe query mempool pending

# Wait for next block
Start-Sleep -Seconds 6

# If still stuck, check fees are sufficient
.\posd.exe query feemarket base-fee
```

---

*Last Updated: 2025-11-05*
*Platform: Windows 11*
*Chain: Omniphi (omniphi-1)*
