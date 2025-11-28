#!/bin/bash

###############################################################################
# AUTONOMOUS BLOCKCHAIN TPS TESTING ENGINE
# For: OmniPhi PoC Blockchain (Cosmos SDK v0.53.3)
#
# Features:
# - Graduated load testing (baseline → peak → stress)
# - Real-time TPS calculation and monitoring
# - Automated bottleneck detection
# - Resource utilization tracking
# - Comprehensive performance report generation
#
# Author: Autonomous Performance Testing Engine
# Date: October 18, 2025
###############################################################################

set -e

# ============================================================================
# CONFIGURATION
# ============================================================================

BINARY="./posd.exe"
CHAIN_ID="pos"
NODE_HOME="$HOME/.pos"
RESULTS_DIR="./tps_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$RESULTS_DIR/tps_test_${TIMESTAMP}.log"
METRICS_FILE="$RESULTS_DIR/metrics_${TIMESTAMP}.csv"

# Test configuration
TEST_DURATION_SECONDS=60
WARMUP_DURATION=10
NUM_TEST_ACCOUNTS=20

# Load profiles (TPS targets)
BASELINE_TPS=25
PEAK_TPS=150
STRESS_TPS=500
MIXED_TPS=100

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# ============================================================================
# UTILITY FUNCTIONS
# ============================================================================

log() {
    local level=$1
    shift
    local message="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    case $level in
        INFO)
            echo -e "${BLUE}[INFO]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
        SUCCESS)
            echo -e "${GREEN}[SUCCESS]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
        WARN)
            echo -e "${YELLOW}[WARN]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
        METRIC)
            echo -e "${CYAN}[METRIC]${NC} ${timestamp} - $message" | tee -a "$LOG_FILE"
            ;;
    esac
}

banner() {
    local text=$1
    echo ""
    echo -e "${MAGENTA}============================================================${NC}"
    echo -e "${MAGENTA}  $text${NC}"
    echo -e "${MAGENTA}============================================================${NC}"
    echo ""
}

# ============================================================================
# BLOCKCHAIN MONITORING
# ============================================================================

get_block_height() {
    $BINARY status 2>/dev/null | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*' | head -1
}

get_block_time() {
    $BINARY status 2>/dev/null | grep -o '"latest_block_time":"[^"]*"' | cut -d'"' -f4
}

get_mempool_size() {
    $BINARY status 2>/dev/null | grep -o '"n_txs":"[0-9]*"' | grep -o '[0-9]*' | head -1 || echo "0"
}

calculate_tps() {
    local start_height=$1
    local end_height=$2
    local duration=$3

    local height_diff=$((end_height - start_height))

    if [ $duration -gt 0 ]; then
        echo "scale=2; $height_diff / $duration" | bc
    else
        echo "0"
    fi
}

get_node_cpu() {
    local pid=$(pgrep -f "posd" | head -1)
    if [ ! -z "$pid" ]; then
        ps -p $pid -o %cpu | tail -1 | tr -d ' '
    else
        echo "0"
    fi
}

get_node_memory() {
    local pid=$(pgrep -f "posd" | head -1)
    if [ ! -z "$pid" ]; then
        ps -p $pid -o rss | tail -1
    else
        echo "0"
    fi
}

# ============================================================================
# TEST ACCOUNT MANAGEMENT
# ============================================================================

setup_test_accounts() {
    log INFO "Setting up $NUM_TEST_ACCOUNTS test accounts..."

    mkdir -p "$RESULTS_DIR/accounts"
    local account_file="$RESULTS_DIR/accounts/accounts_${TIMESTAMP}.txt"

    for i in $(seq 1 $NUM_TEST_ACCOUNTS); do
        local account_name="tpstest${i}_${TIMESTAMP}"

        # Create account
        echo "testpassword$i" | $BINARY keys add $account_name --keyring-backend test 2>&1 | grep -v "WARNING" > /dev/null

        # Get address
        local addr=$($BINARY keys show $account_name -a --keyring-backend test 2>/dev/null)

        if [ ! -z "$addr" ]; then
            echo "$account_name:$addr" >> "$account_file"
            log INFO "  Created account $i/$NUM_TEST_ACCOUNTS: $addr"
        else
            log ERROR "Failed to create account $account_name"
        fi
    done

    log SUCCESS "Test accounts created and saved to $account_file"
    echo "$account_file"
}

fund_test_accounts() {
    local account_file=$1
    log INFO "Funding test accounts..."

    # Get validator/faucet account
    local faucet=$($BINARY keys show validator -a --keyring-backend test 2>/dev/null || \
                   $BINARY keys show alice -a --keyring-backend test 2>/dev/null)

    if [ -z "$faucet" ]; then
        log ERROR "No faucet account found (validator or alice)"
        return 1
    fi

    log INFO "Using faucet: $faucet"

    # Fund each account
    while IFS=: read -r name addr; do
        log INFO "  Funding $name ($addr)..."
        $BINARY tx bank send $faucet $addr 100000000stake \
            --chain-id $CHAIN_ID \
            --keyring-backend test \
            --yes \
            --fees 10000stake \
            --broadcast-mode async &>/dev/null || true
    done < "$account_file"

    # Wait for transactions to be included
    log INFO "Waiting for funding transactions to be included..."
    sleep 10

    log SUCCESS "Test accounts funded"
}

# ============================================================================
# TRANSACTION GENERATORS
# ============================================================================

send_bank_transfer() {
    local from_addr=$1
    local to_addr=$2
    local amount=${3:-1000}

    $BINARY tx bank send $from_addr $to_addr ${amount}stake \
        --chain-id $CHAIN_ID \
        --keyring-backend test \
        --yes \
        --fees 100stake \
        --broadcast-mode async 2>&1 | grep -q "txhash" && echo "1" || echo "0"
}

send_poc_contribution() {
    local from_addr=$1
    local contrib_id=$2

    # Generate unique SHA256 hash
    local hash=$(echo -n "tps_test_${contrib_id}_$(date +%s%N)" | sha256sum | cut -d' ' -f1)

    $BINARY tx poc submit-contribution \
        code \
        "https://github.com/test/tps/${contrib_id}" \
        "$hash" \
        --from $from_addr \
        --chain-id $CHAIN_ID \
        --keyring-backend test \
        --yes \
        --fees 500stake \
        --broadcast-mode async 2>&1 | grep -q "txhash" && echo "1" || echo "0"
}

send_staking_delegate() {
    local from_addr=$1
    local amount=${2:-1000000}

    # Get first validator
    local validator=$($BINARY q staking validators --output json 2>/dev/null | \
                      jq -r '.validators[0].operator_address' 2>/dev/null || echo "")

    if [ -z "$validator" ]; then
        echo "0"
        return
    fi

    $BINARY tx staking delegate $validator ${amount}stake \
        --from $from_addr \
        --chain-id $CHAIN_ID \
        --keyring-backend test \
        --yes \
        --fees 300stake \
        --broadcast-mode async 2>&1 | grep -q "txhash" && echo "1" || echo "0"
}

# ============================================================================
# LOAD TESTING ENGINE
# ============================================================================

run_load_test() {
    local test_name=$1
    local target_tps=$2
    local duration=$3
    local tx_type=$4  # "bank", "poc", "staking", "mixed"

    log INFO "Starting load test: $test_name"
    log INFO "  Target TPS: $target_tps"
    log INFO "  Duration: ${duration}s"
    log INFO "  Transaction Type: $tx_type"

    # Calculate sleep interval for target TPS
    local sleep_interval=$(echo "scale=6; 1.0 / $target_tps" | bc)

    # Get test accounts
    local account_file=$(ls -t "$RESULTS_DIR/accounts/"*.txt | head -1)
    local accounts=()
    while IFS=: read -r name addr; do
        accounts+=("$name:$addr")
    done < "$account_file"

    local num_accounts=${#accounts[@]}
    if [ $num_accounts -eq 0 ]; then
        log ERROR "No test accounts available"
        return 1
    fi

    log INFO "  Using $num_accounts test accounts"

    # Record start metrics
    local start_time=$(date +%s)
    local start_height=$(get_block_height)
    local start_block_time=$(get_block_time)

    log INFO "  Start height: $start_height"
    log INFO "  Start time: $start_block_time"

    # Run test
    local tx_sent=0
    local tx_success=0
    local tx_failed=0
    local end_time=$((start_time + duration))

    log INFO "Running transactions..."

    while [ $(date +%s) -lt $end_time ]; do
        # Select random account
        local account_idx=$((RANDOM % num_accounts))
        local account_pair="${accounts[$account_idx]}"
        local from_name=$(echo $account_pair | cut -d: -f1)
        local from_addr=$(echo $account_pair | cut -d: -f2)

        # Send transaction based on type
        local result=0
        case $tx_type in
            "bank")
                local to_idx=$(( (account_idx + 1) % num_accounts ))
                local to_addr=$(echo "${accounts[$to_idx]}" | cut -d: -f2)
                result=$(send_bank_transfer $from_name $to_addr 1000)
                ;;
            "poc")
                result=$(send_poc_contribution $from_name $tx_sent)
                ;;
            "staking")
                result=$(send_staking_delegate $from_name 1000000)
                ;;
            "mixed")
                # 60% bank, 20% poc, 15% staking, 5% gov
                local rand=$((RANDOM % 100))
                if [ $rand -lt 60 ]; then
                    local to_idx=$(( (account_idx + 1) % num_accounts ))
                    local to_addr=$(echo "${accounts[$to_idx]}" | cut -d: -f2)
                    result=$(send_bank_transfer $from_name $to_addr 1000)
                elif [ $rand -lt 80 ]; then
                    result=$(send_poc_contribution $from_name $tx_sent)
                else
                    result=$(send_staking_delegate $from_name 1000000)
                fi
                ;;
        esac

        tx_sent=$((tx_sent + 1))

        if [ "$result" = "1" ]; then
            tx_success=$((tx_success + 1))
        else
            tx_failed=$((tx_failed + 1))
        fi

        # Progress indicator
        if [ $((tx_sent % 50)) -eq 0 ]; then
            local current_time=$(date +%s)
            local elapsed=$((current_time - start_time))
            local current_tps=$(echo "scale=2; $tx_sent / $elapsed" | bc)
            log METRIC "Progress: ${tx_sent} tx sent, ${tx_success} success, TPS: $current_tps"
        fi

        # Throttle to target TPS
        sleep $sleep_interval 2>/dev/null || true
    done

    # Wait for transactions to be processed
    log INFO "Waiting for transaction processing..."
    sleep 10

    # Record end metrics
    local actual_end_time=$(date +%s)
    local end_height=$(get_block_height)
    local end_block_time=$(get_block_time)
    local actual_duration=$((actual_end_time - start_time))

    # Calculate results
    local blocks_produced=$((end_height - start_height))
    local actual_tps=$(echo "scale=2; $tx_success / $actual_duration" | bc)
    local success_rate=$(echo "scale=2; 100 * $tx_success / $tx_sent" | bc)
    local avg_block_time=$(echo "scale=3; $actual_duration / $blocks_produced" | bc)

    # Get final metrics
    local final_mempool=$(get_mempool_size)
    local final_cpu=$(get_node_cpu)
    local final_memory=$(get_node_memory)

    # Log results
    log SUCCESS "Test completed: $test_name"
    log METRIC "  Transactions sent: $tx_sent"
    log METRIC "  Transactions successful: $tx_success"
    log METRIC "  Transactions failed: $tx_failed"
    log METRIC "  Success rate: ${success_rate}%"
    log METRIC "  Actual TPS: $actual_tps"
    log METRIC "  Target TPS: $target_tps"
    log METRIC "  Blocks produced: $blocks_produced"
    log METRIC "  Avg block time: ${avg_block_time}s"
    log METRIC "  Final mempool size: $final_mempool"
    log METRIC "  CPU usage: ${final_cpu}%"
    log METRIC "  Memory usage: ${final_memory} KB"

    # Save to CSV
    echo "$test_name,$target_tps,$actual_tps,$tx_sent,$tx_success,$tx_failed,$success_rate,$blocks_produced,$avg_block_time,$final_mempool,$final_cpu,$final_memory" >> "$METRICS_FILE"

    # Detect bottlenecks
    if [ "$final_mempool" -gt 100 ]; then
        log WARN "BOTTLENECK DETECTED: High mempool size ($final_mempool) - transactions not being processed fast enough"
    fi

    if [ $(echo "$avg_block_time > 10" | bc) -eq 1 ]; then
        log WARN "BOTTLENECK DETECTED: Slow block time (${avg_block_time}s) - consensus delays detected"
    fi

    if [ $(echo "$final_cpu > 80" | bc) -eq 1 ]; then
        log WARN "BOTTLENECK DETECTED: High CPU usage (${final_cpu}%) - computational bottleneck"
    fi

    if [ $(echo "$success_rate < 90" | bc) -eq 1 ]; then
        log WARN "PERFORMANCE ISSUE: Low success rate (${success_rate}%) - check logs for errors"
    fi
}

# ============================================================================
# MAIN TEST EXECUTION
# ============================================================================

main() {
    banner "AUTONOMOUS BLOCKCHAIN TPS TESTING ENGINE"

    # Create results directory
    mkdir -p "$RESULTS_DIR/accounts"

    # Initialize metrics CSV
    echo "test_name,target_tps,actual_tps,tx_sent,tx_success,tx_failed,success_rate,blocks_produced,avg_block_time,mempool_size,cpu_usage,memory_usage" > "$METRICS_FILE"

    log INFO "Test session: $TIMESTAMP"
    log INFO "Results directory: $RESULTS_DIR"
    log INFO "Log file: $LOG_FILE"
    log INFO "Metrics file: $METRICS_FILE"

    # Check blockchain is running
    if ! $BINARY status &>/dev/null; then
        log ERROR "Blockchain is not running. Please start the node first:"
        log ERROR "  $BINARY start"
        exit 1
    fi

    log SUCCESS "Blockchain is running"

    # Setup and fund test accounts
    banner "PHASE 1: TEST ACCOUNT SETUP"
    local account_file=$(setup_test_accounts)
    fund_test_accounts "$account_file"

    # Warmup
    banner "PHASE 2: WARMUP"
    log INFO "Running warmup test ($WARMUP_DURATION seconds)..."
    run_load_test "warmup" 10 $WARMUP_DURATION "bank"

    # Baseline test
    banner "PHASE 3: BASELINE LOAD TEST"
    run_load_test "baseline_bank" $BASELINE_TPS $TEST_DURATION_SECONDS "bank"
    sleep 5

    # Peak load tests
    banner "PHASE 4: PEAK LOAD TESTS"
    run_load_test "peak_bank" $PEAK_TPS $TEST_DURATION_SECONDS "bank"
    sleep 5

    run_load_test "peak_poc" $((PEAK_TPS / 2)) $TEST_DURATION_SECONDS "poc"
    sleep 5

    run_load_test "peak_staking" $((PEAK_TPS / 3)) $TEST_DURATION_SECONDS "staking"
    sleep 5

    # Mixed workload
    banner "PHASE 5: MIXED WORKLOAD TEST"
    run_load_test "mixed_workload" $MIXED_TPS $TEST_DURATION_SECONDS "mixed"
    sleep 5

    # Stress test
    banner "PHASE 6: STRESS LOAD TEST"
    log WARN "Starting stress test - may cause node instability"
    run_load_test "stress_bank" $STRESS_TPS 30 "bank"
    sleep 10

    # Generate final report
    banner "PHASE 7: GENERATING REPORT"
    generate_report

    log SUCCESS "Autonomous TPS testing completed!"
    log INFO "Results saved to: $RESULTS_DIR"
    log INFO "View report: cat $RESULTS_DIR/report_${TIMESTAMP}.md"
}

generate_report() {
    local report_file="$RESULTS_DIR/report_${TIMESTAMP}.md"

    cat > "$report_file" <<EOF
# Autonomous TPS Testing Report

**Blockchain:** OmniPhi PoC (Cosmos SDK v0.53.3)
**Test Date:** $(date)
**Session ID:** $TIMESTAMP

---

## Test Results

\`\`\`
$(cat "$METRICS_FILE" | column -t -s,)
\`\`\`

---

## Performance Summary

EOF

    # Calculate averages from CSV
    local avg_tps=$(awk -F, 'NR>1 {sum+=$3; count++} END {if(count>0) print sum/count; else print 0}' "$METRICS_FILE")
    local avg_success=$(awk -F, 'NR>1 {sum+=$7; count++} END {if(count>0) print sum/count; else print 0}' "$METRICS_FILE")
    local avg_block_time=$(awk -F, 'NR>1 {sum+=$9; count++} END {if(count>0) print sum/count; else print 0}' "$METRICS_FILE")

    cat >> "$report_file" <<EOF
- **Average TPS Achieved:** ${avg_tps}
- **Average Success Rate:** ${avg_success}%
- **Average Block Time:** ${avg_block_time}s

---

## Bottleneck Analysis

$(grep "BOTTLENECK DETECTED" "$LOG_FILE" || echo "No critical bottlenecks detected")

---

## Recommendations

Based on the test results:

1. **Current Performance:** ${avg_tps} TPS achieved
2. **Optimization Target:** $(echo "$avg_tps * 2" | bc) TPS (2x improvement)
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

**Report Generated:** $(date)
EOF

    log SUCCESS "Report generated: $report_file"
}

# Run main
main "$@"
