# AUTONOMOUS BLOCKCHAIN TPS TESTING ENGINE
# For: OmniPhi PoC Blockchain (Cosmos SDK v0.53.3)
#
# Features:
# - Graduated load testing (baseline → peak → stress)
# - Real-time TPS calculation and monitoring
# - Automated bottleneck detection
# - Resource utilization tracking
# - Comprehensive performance report generation

param(
    [int]$TestDuration = 60,
    [int]$NumAccounts = 20,
    [int]$BaselineTPS = 25,
    [int]$PeakTPS = 150,
    [int]$StressTPS = 500
)

$ErrorActionPreference = "Continue"

# Configuration
$BINARY = ".\posd.exe"
$CHAIN_ID = "pos"
$RESULTS_DIR = ".\tps_results"
$TIMESTAMP = Get-Date -Format "yyyyMMdd_HHmmss"
$LOG_FILE = "$RESULTS_DIR\tps_test_$TIMESTAMP.log"
$METRICS_FILE = "$RESULTS_DIR\metrics_$TIMESTAMP.csv"

# Create results directory
if (!(Test-Path $RESULTS_DIR)) {
    New-Item -ItemType Directory -Path $RESULTS_DIR | Out-Null
    New-Item -ItemType Directory -Path "$RESULTS_DIR\accounts" | Out-Null
}

# ============================================================================
# UTILITY FUNCTIONS
# ============================================================================

function Write-Log {
    param(
        [string]$Level,
        [string]$Message
    )

    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$Level] $timestamp - $Message"

    switch ($Level) {
        "INFO" { Write-Host $logMessage -ForegroundColor Blue }
        "SUCCESS" { Write-Host $logMessage -ForegroundColor Green }
        "WARN" { Write-Host $logMessage -ForegroundColor Yellow }
        "ERROR" { Write-Host $logMessage -ForegroundColor Red }
        "METRIC" { Write-Host $logMessage -ForegroundColor Cyan }
    }

    Add-Content -Path $LOG_FILE -Value $logMessage
}

function Write-Banner {
    param([string]$Text)

    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Magenta
    Write-Host "  $Text" -ForegroundColor Magenta
    Write-Host "============================================================" -ForegroundColor Magenta
    Write-Host ""
}

# ============================================================================
# BLOCKCHAIN MONITORING
# ============================================================================

function Get-BlockHeight {
    try {
        $status = & $BINARY status 2>&1 | ConvertFrom-Json
        return [int]$status.SyncInfo.latest_block_height
    } catch {
        return 0
    }
}

function Get-MempoolSize {
    try {
        $status = & $BINARY status 2>&1 | ConvertFrom-Json
        return [int]$status.SyncInfo.n_txs
    } catch {
        return 0
    }
}

function Get-NodeCPU {
    try {
        $process = Get-Process -Name "posd" -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($process) {
            return [Math]::Round($process.CPU, 2)
        }
    } catch {}
    return 0
}

function Get-NodeMemory {
    try {
        $process = Get-Process -Name "posd" -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($process) {
            return [Math]::Round($process.WorkingSet64 / 1MB, 2)
        }
    } catch {}
    return 0
}

# ============================================================================
# TEST ACCOUNT MANAGEMENT
# ============================================================================

function Setup-TestAccounts {
    Write-Log "INFO" "Setting up $NumAccounts test accounts..."

    $accountFile = "$RESULTS_DIR\accounts\accounts_$TIMESTAMP.txt"
    $accounts = @()

    for ($i = 1; $i -le $NumAccounts; $i++) {
        $accountName = "tpstest${i}_$TIMESTAMP"

        Write-Log "INFO" "  Creating account $i/$NumAccounts..."

        # Create account
        $null = echo "testpassword$i" | & $BINARY keys add $accountName --keyring-backend test 2>&1

        # Get address
        $addr = & $BINARY keys show $accountName -a --keyring-backend test 2>$null

        if ($addr) {
            "$accountName`:$addr" | Out-File -FilePath $accountFile -Append
            $accounts += @{Name=$accountName; Address=$addr}
            Write-Log "INFO" "  Created: $addr"
        }
    }

    Write-Log "SUCCESS" "Test accounts created: $accountFile"
    return $accounts
}

function Fund-TestAccounts {
    param($Accounts)

    Write-Log "INFO" "Funding test accounts..."

    # Get faucet
    $faucet = & $BINARY keys show validator -a --keyring-backend test 2>$null
    if (!$faucet) {
        $faucet = & $BINARY keys show alice -a --keyring-backend test 2>$null
    }

    if (!$faucet) {
        Write-Log "ERROR" "No faucet account found"
        return
    }

    Write-Log "INFO" "Using faucet: $faucet"

    foreach ($account in $Accounts) {
        Write-Log "INFO" "  Funding $($account.Name)..."
        $null = & $BINARY tx bank send $faucet $account.Address 100000000stake `
            --chain-id $CHAIN_ID `
            --keyring-backend test `
            --yes `
            --fees 10000stake `
            --broadcast-mode async 2>&1
    }

    Write-Log "INFO" "Waiting for funding transactions..."
    Start-Sleep -Seconds 10

    Write-Log "SUCCESS" "Test accounts funded"
}

# ============================================================================
# TRANSACTION GENERATORS
# ============================================================================

function Send-BankTransfer {
    param(
        [string]$From,
        [string]$To,
        [int]$Amount = 1000
    )

    try {
        $result = & $BINARY tx bank send $From $To "${Amount}stake" `
            --chain-id $CHAIN_ID `
            --keyring-backend test `
            --yes `
            --fees 100stake `
            --broadcast-mode async 2>&1

        return ($result -match "txhash")
    } catch {
        return $false
    }
}

function Send-PocContribution {
    param(
        [string]$From,
        [int]$ContribId
    )

    try {
        # Generate unique SHA256 hash
        $randomData = "tps_test_${ContribId}_$(Get-Date -Format 'yyyyMMddHHmmssfff')"
        $sha256 = [System.Security.Cryptography.SHA256]::Create()
        $hashBytes = $sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($randomData))
        $hash = ($hashBytes | ForEach-Object { $_.ToString("x2") }) -join ""

        $result = & $BINARY tx poc submit-contribution code `
            "https://github.com/test/tps/$ContribId" `
            $hash `
            --from $From `
            --chain-id $CHAIN_ID `
            --keyring-backend test `
            --yes `
            --fees 500stake `
            --broadcast-mode async 2>&1

        return ($result -match "txhash")
    } catch {
        return $false
    }
}

function Send-StakingDelegate {
    param(
        [string]$From,
        [int]$Amount = 1000000
    )

    try {
        $validators = & $BINARY q staking validators --output json 2>$null | ConvertFrom-Json
        if ($validators.validators.Count -eq 0) {
            return $false
        }

        $validator = $validators.validators[0].operator_address

        $result = & $BINARY tx staking delegate $validator "${Amount}stake" `
            --from $From `
            --chain-id $CHAIN_ID `
            --keyring-backend test `
            --yes `
            --fees 300stake `
            --broadcast-mode async 2>&1

        return ($result -match "txhash")
    } catch {
        return $false
    }
}

# ============================================================================
# LOAD TESTING ENGINE
# ============================================================================

function Run-LoadTest {
    param(
        [string]$TestName,
        [int]$TargetTPS,
        [int]$Duration,
        [string]$TxType,  # "bank", "poc", "staking", "mixed"
        $Accounts
    )

    Write-Log "INFO" "Starting load test: $TestName"
    Write-Log "INFO" "  Target TPS: $TargetTPS"
    Write-Log "INFO" "  Duration: ${Duration}s"
    Write-Log "INFO" "  Transaction Type: $TxType"
    Write-Log "INFO" "  Using $($Accounts.Count) test accounts"

    # Calculate sleep interval
    $sleepMs = [Math]::Floor(1000.0 / $TargetTPS)

    # Record start metrics
    $startTime = Get-Date
    $startHeight = Get-BlockHeight
    Write-Log "INFO" "  Start height: $startHeight"

    # Run test
    $txSent = 0
    $txSuccess = 0
    $txFailed = 0
    $endTime = $startTime.AddSeconds($Duration)

    Write-Log "INFO" "Running transactions..."

    while ((Get-Date) -lt $endTime) {
        # Select random account
        $accountIdx = Get-Random -Minimum 0 -Maximum $Accounts.Count
        $account = $Accounts[$accountIdx]

        # Send transaction
        $success = $false

        switch ($TxType) {
            "bank" {
                $toIdx = ($accountIdx + 1) % $Accounts.Count
                $toAddr = $Accounts[$toIdx].Address
                $success = Send-BankTransfer -From $account.Name -To $toAddr
            }
            "poc" {
                $success = Send-PocContribution -From $account.Name -ContribId $txSent
            }
            "staking" {
                $success = Send-StakingDelegate -From $account.Name
            }
            "mixed" {
                $rand = Get-Random -Minimum 0 -Maximum 100
                if ($rand -lt 60) {
                    $toIdx = ($accountIdx + 1) % $Accounts.Count
                    $toAddr = $Accounts[$toIdx].Address
                    $success = Send-BankTransfer -From $account.Name -To $toAddr
                } elseif ($rand -lt 80) {
                    $success = Send-PocContribution -From $account.Name -ContribId $txSent
                } else {
                    $success = Send-StakingDelegate -From $account.Name
                }
            }
        }

        $txSent++
        if ($success) {
            $txSuccess++
        } else {
            $txFailed++
        }

        # Progress indicator
        if ($txSent % 50 -eq 0) {
            $elapsed = ((Get-Date) - $startTime).TotalSeconds
            $currentTPS = [Math]::Round($txSent / $elapsed, 2)
            Write-Log "METRIC" "Progress: $txSent tx sent, $txSuccess success, TPS: $currentTPS"
        }

        # Throttle
        Start-Sleep -Milliseconds $sleepMs
    }

    # Wait for processing
    Write-Log "INFO" "Waiting for transaction processing..."
    Start-Sleep -Seconds 10

    # Record end metrics
    $actualEndTime = Get-Date
    $endHeight = Get-BlockHeight
    $actualDuration = ($actualEndTime - $startTime).TotalSeconds

    # Calculate results
    $blocksProduced = $endHeight - $startHeight
    $actualTPS = [Math]::Round($txSuccess / $actualDuration, 2)
    $successRate = [Math]::Round(100.0 * $txSuccess / $txSent, 2)
    $avgBlockTime = if ($blocksProduced -gt 0) { [Math]::Round($actualDuration / $blocksProduced, 3) } else { 0 }

    # Get final metrics
    $finalMempool = Get-MempoolSize
    $finalCPU = Get-NodeCPU
    $finalMemory = Get-NodeMemory

    # Log results
    Write-Log "SUCCESS" "Test completed: $TestName"
    Write-Log "METRIC" "  Transactions sent: $txSent"
    Write-Log "METRIC" "  Transactions successful: $txSuccess"
    Write-Log "METRIC" "  Transactions failed: $txFailed"
    Write-Log "METRIC" "  Success rate: ${successRate}%"
    Write-Log "METRIC" "  Actual TPS: $actualTPS"
    Write-Log "METRIC" "  Target TPS: $TargetTPS"
    Write-Log "METRIC" "  Blocks produced: $blocksProduced"
    Write-Log "METRIC" "  Avg block time: ${avgBlockTime}s"
    Write-Log "METRIC" "  Final mempool size: $finalMempool"
    Write-Log "METRIC" "  CPU: ${finalCPU}%"
    Write-Log "METRIC" "  Memory: ${finalMemory} MB"

    # Save to CSV
    "$TestName,$TargetTPS,$actualTPS,$txSent,$txSuccess,$txFailed,$successRate,$blocksProduced,$avgBlockTime,$finalMempool,$finalCPU,$finalMemory" |
        Out-File -FilePath $METRICS_FILE -Append

    # Detect bottlenecks
    if ($finalMempool -gt 100) {
        Write-Log "WARN" "BOTTLENECK DETECTED: High mempool size ($finalMempool)"
    }

    if ($avgBlockTime -gt 10) {
        Write-Log "WARN" "BOTTLENECK DETECTED: Slow block time (${avgBlockTime}s)"
    }

    if ($finalCPU -gt 80) {
        Write-Log "WARN" "BOTTLENECK DETECTED: High CPU usage (${finalCPU}%)"
    }

    if ($successRate -lt 90) {
        Write-Log "WARN" "PERFORMANCE ISSUE: Low success rate (${successRate}%)"
    }
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

Write-Banner "AUTONOMOUS BLOCKCHAIN TPS TESTING ENGINE"

# Initialize metrics CSV
"test_name,target_tps,actual_tps,tx_sent,tx_success,tx_failed,success_rate,blocks_produced,avg_block_time,mempool_size,cpu_usage,memory_usage" |
    Out-File -FilePath $METRICS_FILE

Write-Log "INFO" "Test session: $TIMESTAMP"
Write-Log "INFO" "Results directory: $RESULTS_DIR"
Write-Log "INFO" "Log file: $LOG_FILE"
Write-Log "INFO" "Metrics file: $METRICS_FILE"

# Check blockchain is running
try {
    $null = & $BINARY status 2>&1
    Write-Log "SUCCESS" "Blockchain is running"
} catch {
    Write-Log "ERROR" "Blockchain is not running. Please start:"
    Write-Log "ERROR" "  $BINARY start"
    exit 1
}

# Setup test accounts
Write-Banner "PHASE 1: TEST ACCOUNT SETUP"
$accounts = Setup-TestAccounts
Fund-TestAccounts -Accounts $accounts

# Warmup
Write-Banner "PHASE 2: WARMUP"
Write-Log "INFO" "Running warmup test..."
Run-LoadTest -TestName "warmup" -TargetTPS 10 -Duration 10 -TxType "bank" -Accounts $accounts

# Baseline test
Write-Banner "PHASE 3: BASELINE LOAD TEST"
Run-LoadTest -TestName "baseline_bank" -TargetTPS $BaselineTPS -Duration $TestDuration -TxType "bank" -Accounts $accounts
Start-Sleep -Seconds 5

# Peak load tests
Write-Banner "PHASE 4: PEAK LOAD TESTS"
Run-LoadTest -TestName "peak_bank" -TargetTPS $PeakTPS -Duration $TestDuration -TxType "bank" -Accounts $accounts
Start-Sleep -Seconds 5

Run-LoadTest -TestName "peak_poc" -TargetTPS ([Math]::Floor($PeakTPS / 2)) -Duration $TestDuration -TxType "poc" -Accounts $accounts
Start-Sleep -Seconds 5

Run-LoadTest -TestName "peak_staking" -TargetTPS ([Math]::Floor($PeakTPS / 3)) -Duration $TestDuration -TxType "staking" -Accounts $accounts
Start-Sleep -Seconds 5

# Mixed workload
Write-Banner "PHASE 5: MIXED WORKLOAD TEST"
Run-LoadTest -TestName "mixed_workload" -TargetTPS 100 -Duration $TestDuration -TxType "mixed" -Accounts $accounts
Start-Sleep -Seconds 5

# Stress test
Write-Banner "PHASE 6: STRESS LOAD TEST"
Write-Log "WARN" "Starting stress test - may cause node instability"
Run-LoadTest -TestName "stress_bank" -TargetTPS $StressTPS -Duration 30 -TxType "bank" -Accounts $accounts
Start-Sleep -Seconds 10

# Generate report
Write-Banner "PHASE 7: GENERATING REPORT"

$reportFile = "$RESULTS_DIR\report_$TIMESTAMP.md"

$metrics = Import-Csv $METRICS_FILE
$avgTPS = ($metrics | Measure-Object -Property actual_tps -Average).Average
$avgSuccess = ($metrics | Measure-Object -Property success_rate -Average).Average
$avgBlockTime = ($metrics | Measure-Object -Property avg_block_time -Average).Average

@"
# Autonomous TPS Testing Report

**Blockchain:** OmniPhi PoC (Cosmos SDK v0.53.3)
**Test Date:** $(Get-Date)
**Session ID:** $TIMESTAMP

---

## Test Results

``````
$(Import-Csv $METRICS_FILE | Format-Table | Out-String)
``````

---

## Performance Summary

- **Average TPS Achieved:** $([Math]::Round($avgTPS, 2))
- **Average Success Rate:** $([Math]::Round($avgSuccess, 2))%
- **Average Block Time:** $([Math]::Round($avgBlockTime, 3))s

---

## Bottleneck Analysis

$(Get-Content $LOG_FILE | Select-String "BOTTLENECK DETECTED" | Out-String)

---

## Recommendations

Based on the test results:

1. **Current Performance:** $([Math]::Round($avgTPS, 2)) TPS achieved
2. **Optimization Target:** $([Math]::Round($avgTPS * 2, 2)) TPS (2x improvement)
3. **Next Steps:**
   - Review validator cache hit rate
   - Optimize mempool configuration if mempool > 100
   - Consider pagination for queries
   - Monitor block time - target < 7s

---

## Raw Metrics

See: [$METRICS_FILE]($METRICS_FILE)

## Full Log

See: [$LOG_FILE]($LOG_FILE)

---

**Report Generated:** $(Get-Date)
"@ | Out-File -FilePath $reportFile

Write-Log "SUCCESS" "Report generated: $reportFile"
Write-Log "SUCCESS" "Autonomous TPS testing completed!"
Write-Log "INFO" "Results saved to: $RESULTS_DIR"
Write-Log "INFO" "View report: cat $reportFile"

Write-Host ""
Write-Host "============================================================" -ForegroundColor Green
Write-Host "  TPS TESTING COMPLETE" -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Green
Write-Host ""
Write-Host "Report: $reportFile" -ForegroundColor Cyan
Write-Host "Metrics: $METRICS_FILE" -ForegroundColor Cyan
Write-Host "Log: $LOG_FILE" -ForegroundColor Cyan
Write-Host ""
