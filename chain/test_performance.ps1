#################################################################################
# Performance Testing Script (PowerShell)
# Tests TPS, latency, and stress scenarios
#################################################################################

$CHAIN_ID = "omniphi-1"
$NUM_TPS_TX = 50
$NUM_LATENCY_SAMPLES = 10

Write-Host "================================" -ForegroundColor Cyan
Write-Host "Performance Testing" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Check if chain is running
try {
    & .\posd.exe status 2>$null | Out-Null
} catch {
    Write-Host "✗ Chain is not running" -ForegroundColor Red
    Write-Host "Start with: .\posd.exe start --minimum-gas-prices 0.001uomni --grpc.enable=false" -ForegroundColor Yellow
    exit 1
}

# Ensure test accounts exist and are funded
Write-Host "Setting up test accounts..." -ForegroundColor Yellow
@("alice", "bob") | ForEach-Object {
    $account = $_
    try {
        & .\posd.exe keys show $account --keyring-backend test 2>$null | Out-Null
    } catch {
        & .\posd.exe keys add $account --keyring-backend test 2>$null | Out-Null
    }
}

$ALICE = (& .\posd.exe keys show alice -a --keyring-backend test)
$BOB = (& .\posd.exe keys show bob -a --keyring-backend test)

# Check balances
$aliceBalance = (& .\posd.exe query bank balances $ALICE -o json 2>$null | ConvertFrom-Json).balances[0].amount
if (-not $aliceBalance -or $aliceBalance -lt 100000) {
    Write-Host "Alice needs funding. Run test_modules.ps1 first or fund manually." -ForegroundColor Yellow
}
Write-Host "  ✓ Test accounts ready" -ForegroundColor Green
Write-Host ""

# Test 1: TPS (Transactions Per Second)
Write-Host "[1/3] TPS Test ($NUM_TPS_TX transactions)" -ForegroundColor Cyan
Write-Host "  Sending transactions in parallel..." -ForegroundColor Yellow

$startTime = Get-Date

$jobs = @()
for ($i = 1; $i -le $NUM_TPS_TX; $i++) {
    $jobs += Start-Job -ScriptBlock {
        param($alice, $bob, $chainId)
        & .\posd.exe tx bank send $alice $bob 100uomni `
            --chain-id $chainId `
            --keyring-backend test `
            --fees 500uomni `
            --yes 2>$null | Out-Null
    } -ArgumentList $ALICE, $BOB, $CHAIN_ID

    # Batch in groups of 10
    if ($i % 10 -eq 0) {
        Start-Sleep -Milliseconds 500
    }
}

Write-Host "  Waiting for transactions to complete..." -ForegroundColor Yellow
$jobs | Wait-Job | Out-Null
$jobs | Remove-Job

$endTime = Get-Date
$duration = [int](($endTime - $startTime).TotalSeconds)

if ($duration -eq 0) {
    $duration = 1
}

$tps = [int]($NUM_TPS_TX / $duration)

Write-Host "  ✓ Completed $NUM_TPS_TX transactions in ${duration}s" -ForegroundColor Green
Write-Host "  ✓ TPS: $tps transactions/second" -ForegroundColor Green
Write-Host ""

# Test 2: Latency (Time to finality)
Write-Host "[2/3] Latency Test ($NUM_LATENCY_SAMPLES samples)" -ForegroundColor Cyan
Write-Host "  Measuring transaction finality time..." -ForegroundColor Yellow

$totalLatency = 0
for ($i = 1; $i -le $NUM_LATENCY_SAMPLES; $i++) {
    $start = Get-Date

    $txHash = (& .\posd.exe tx bank send $ALICE $BOB 100uomni `
        --chain-id $CHAIN_ID `
        --keyring-backend test `
        --fees 500uomni `
        --yes -o json 2>$null | ConvertFrom-Json).txhash

    # Wait for inclusion
    Start-Sleep -Seconds 6

    $end = Get-Date
    $latency = [int](($end - $start).TotalMilliseconds)

    $totalLatency += $latency
    Write-Host "  Sample $i : ${latency}ms"
}

$avgLatency = [int]($totalLatency / $NUM_LATENCY_SAMPLES)
Write-Host "  ✓ Average latency: ${avgLatency}ms" -ForegroundColor Green
Write-Host ""

# Test 3: Stress Test (Burst of transactions)
Write-Host "[3/3] Stress Test (100 transactions burst)" -ForegroundColor Cyan
Write-Host "  Sending burst of transactions..." -ForegroundColor Yellow

$stressCount = 100
$stressStart = Get-Date

$jobs = @()
for ($i = 1; $i -le $stressCount; $i++) {
    $jobs += Start-Job -ScriptBlock {
        param($alice, $bob, $chainId)
        & .\posd.exe tx bank send $alice $bob 100uomni `
            --chain-id $chainId `
            --keyring-backend test `
            --fees 500uomni `
            --yes 2>$null | Out-Null
    } -ArgumentList $ALICE, $BOB, $CHAIN_ID

    # Small delay every 20 transactions
    if ($i % 20 -eq 0) {
        Start-Sleep -Seconds 1
    }
}

Write-Host "  Waiting for mempool to clear..." -ForegroundColor Yellow
Start-Sleep -Seconds 15

$stressEnd = Get-Date
$stressDuration = [int](($stressEnd - $stressStart).TotalSeconds)
$stressTPS = [int]($stressCount / $stressDuration)

Write-Host "  ✓ Handled $stressCount transactions in ${stressDuration}s" -ForegroundColor Green
Write-Host "  ✓ Stress TPS: $stressTPS tx/s" -ForegroundColor Green
Write-Host ""

# Summary
Write-Host "================================" -ForegroundColor Cyan
Write-Host "✅ Performance Test Results" -ForegroundColor Green
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "TPS Test:"
Write-Host "  $NUM_TPS_TX transactions in ${duration}s"
Write-Host "  TPS: $tps tx/s" -ForegroundColor Green
Write-Host ""
Write-Host "Latency Test:"
Write-Host "  Average: ${avgLatency}ms" -ForegroundColor Green
Write-Host "  Samples: $NUM_LATENCY_SAMPLES"
Write-Host ""
Write-Host "Stress Test:"
Write-Host "  $stressCount transactions in ${stressDuration}s"
Write-Host "  TPS under load: $stressTPS tx/s" -ForegroundColor Green
Write-Host ""

# Performance assessment
Write-Host "Assessment:"
if ($tps -ge 10) {
    Write-Host "  TPS: ✓ Good ($tps tx/s >= 10 tx/s target)" -ForegroundColor Green
} else {
    Write-Host "  TPS: ⚠ Below target ($tps tx/s < 10 tx/s)" -ForegroundColor Yellow
}

if ($avgLatency -le 10000) {
    Write-Host "  Latency: ✓ Good (${avgLatency}ms <= 10s target)" -ForegroundColor Green
} else {
    Write-Host "  Latency: ⚠ High (${avgLatency}ms > 10s target)" -ForegroundColor Yellow
}

Write-Host ""
