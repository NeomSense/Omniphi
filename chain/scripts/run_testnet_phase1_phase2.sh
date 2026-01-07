#!/bin/bash
# =============================================================================
# Omniphi Testnet Phase 1 & Phase 2 Comprehensive Test Suite
# =============================================================================
# Chain ID: omniphi-testnet-2
# Run this on VPS1 (46.202.179.182)
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
CHAIN_ID="omniphi-testnet-2"
VPS1_IP="46.202.179.182"
VPS2_IP="148.230.125.9"
DENOM="omniphi"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
WARNED_TESTS=0

# Results array
declare -A RESULTS

log_header() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
}

log_test() {
    echo -e "${CYAN}[$1]${NC} $2"
}

log_pass() {
    echo -e "${GREEN}  ✓ PASS:${NC} $1"
    PASSED_TESTS=$((PASSED_TESTS + 1))
    RESULTS["$2"]="PASS"
}

log_fail() {
    echo -e "${RED}  ✗ FAIL:${NC} $1"
    FAILED_TESTS=$((FAILED_TESTS + 1))
    RESULTS["$2"]="FAIL"
}

log_warn() {
    echo -e "${YELLOW}  ! WARN:${NC} $1"
    WARNED_TESTS=$((WARNED_TESTS + 1))
    RESULTS["$2"]="WARN"
}

log_info() {
    echo -e "  ${NC}$1${NC}"
}

# =============================================================================
# PHASE 1: CHAIN STABILITY & BLOCK PRODUCTION
# =============================================================================

run_phase1() {
    log_header "PHASE 1: CHAIN STABILITY & BLOCK PRODUCTION"

    # -------------------------------------------------------------------------
    # Test 1.1: Block Time Consistency
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "1.1" "Block Time Consistency (monitoring 10 blocks)"

    HEIGHTS=()
    for i in {1..10}; do
        HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height' 2>/dev/null || echo "0")
        HEIGHTS+=($HEIGHT)
        sleep 4
    done

    # Calculate block progression
    FIRST_HEIGHT=${HEIGHTS[0]}
    LAST_HEIGHT=${HEIGHTS[9]}
    BLOCKS_PRODUCED=$((LAST_HEIGHT - FIRST_HEIGHT))

    if [ $BLOCKS_PRODUCED -ge 8 ] && [ $BLOCKS_PRODUCED -le 12 ]; then
        log_pass "Produced $BLOCKS_PRODUCED blocks in 40s (~4s/block)" "1.1"
    elif [ $BLOCKS_PRODUCED -ge 5 ]; then
        log_warn "Produced $BLOCKS_PRODUCED blocks in 40s (slightly off target)" "1.1"
    else
        log_fail "Only produced $BLOCKS_PRODUCED blocks in 40s" "1.1"
    fi

    # -------------------------------------------------------------------------
    # Test 1.2: Consensus Health Check
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "1.2" "Consensus Health Check"

    VALIDATORS=$(posd query staking validators --output json 2>/dev/null | jq '.validators | length' 2>/dev/null || echo "0")
    BONDED=$(posd query staking validators --output json 2>/dev/null | jq '[.validators[] | select(.status=="BOND_STATUS_BONDED")] | length' 2>/dev/null || echo "0")
    JAILED=$(posd query staking validators --output json 2>/dev/null | jq '[.validators[] | select(.jailed==true)] | length' 2>/dev/null || echo "0")

    log_info "Total validators: $VALIDATORS"
    log_info "Bonded validators: $BONDED"
    log_info "Jailed validators: $JAILED"

    if [ "$BONDED" -ge 2 ] && [ "$JAILED" -eq 0 ]; then
        log_pass "All $BONDED validators bonded, none jailed" "1.2"
    elif [ "$BONDED" -ge 1 ]; then
        log_warn "Only $BONDED validators bonded" "1.2"
    else
        log_fail "No bonded validators found" "1.2"
    fi

    # -------------------------------------------------------------------------
    # Test 1.3: Block Stall Detection (60 second test)
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "1.3" "Block Stall Detection (60s monitoring)"

    START_HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height' 2>/dev/null || echo "0")
    log_info "Start height: $START_HEIGHT"
    sleep 60
    END_HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height' 2>/dev/null || echo "0")
    log_info "End height: $END_HEIGHT"

    BLOCKS_60S=$((END_HEIGHT - START_HEIGHT))
    log_info "Blocks in 60s: $BLOCKS_60S (expected: 12-18)"

    if [ $BLOCKS_60S -ge 12 ] && [ $BLOCKS_60S -le 20 ]; then
        log_pass "Chain producing blocks normally ($BLOCKS_60S blocks)" "1.3"
    elif [ $BLOCKS_60S -ge 8 ]; then
        log_warn "Block production slightly slow ($BLOCKS_60S blocks)" "1.3"
    else
        log_fail "Chain stalled or very slow ($BLOCKS_60S blocks)" "1.3"
    fi

    # -------------------------------------------------------------------------
    # Test 1.4: Height Sync Across Validators
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "1.4" "Height Sync Across Validators"

    VPS1_HEIGHT=$(curl -s --connect-timeout 5 http://${VPS1_IP}:26657/status 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")
    VPS2_HEIGHT=$(curl -s --connect-timeout 5 http://${VPS2_IP}:26657/status 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")

    log_info "VPS1 height: $VPS1_HEIGHT"
    log_info "VPS2 height: $VPS2_HEIGHT"

    if [ "$VPS1_HEIGHT" != "0" ] && [ "$VPS2_HEIGHT" != "0" ]; then
        DIFF=$((VPS1_HEIGHT - VPS2_HEIGHT))
        ABS_DIFF=${DIFF#-}
        log_info "Height difference: $ABS_DIFF blocks"

        if [ $ABS_DIFF -le 2 ]; then
            log_pass "Validators in sync (diff: $ABS_DIFF)" "1.4"
        elif [ $ABS_DIFF -le 5 ]; then
            log_warn "Slight height difference ($ABS_DIFF blocks)" "1.4"
        else
            log_fail "Validators out of sync ($ABS_DIFF blocks)" "1.4"
        fi
    else
        log_warn "Could not reach one or both validators" "1.4"
    fi

    # -------------------------------------------------------------------------
    # Test 2.1: Max Block Gas Verification
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "2.1" "Max Block Gas Verification (60M expected)"

    MAX_BLOCK_GAS=$(posd query consensus params --output json 2>/dev/null | jq -r '.params.block.max_gas' 2>/dev/null || echo "0")
    log_info "Max block gas: $MAX_BLOCK_GAS"

    if [ "$MAX_BLOCK_GAS" = "60000000" ]; then
        log_pass "Max block gas is 60M" "2.1"
    elif [ "$MAX_BLOCK_GAS" = "-1" ]; then
        log_warn "Max block gas is unlimited (-1)" "2.1"
    else
        log_fail "Unexpected max block gas: $MAX_BLOCK_GAS" "2.1"
    fi

    # -------------------------------------------------------------------------
    # Test 2.2: Max Transaction Gas (2M)
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "2.2" "Max Transaction Gas (2M expected)"

    MAX_TX_GAS=$(posd query feemarket params --output json 2>/dev/null | jq -r '.params.max_tx_gas' 2>/dev/null || echo "0")
    log_info "Max tx gas: $MAX_TX_GAS"

    if [ "$MAX_TX_GAS" = "2000000" ]; then
        log_pass "Max tx gas is 2M" "2.2"
    else
        log_fail "Unexpected max tx gas: $MAX_TX_GAS" "2.2"
    fi

    # -------------------------------------------------------------------------
    # Test 2.3: Block Signatures (both validators signing)
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "2.3" "Block Signatures Verification"

    CURRENT_HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height' 2>/dev/null || echo "0")
    SIG_COUNT=$(curl -s http://localhost:26657/block?height=$CURRENT_HEIGHT 2>/dev/null | jq '.result.block.last_commit.signatures | length' 2>/dev/null || echo "0")

    log_info "Signatures on block $CURRENT_HEIGHT: $SIG_COUNT"

    if [ "$SIG_COUNT" -ge 2 ]; then
        log_pass "Block has $SIG_COUNT signatures (both validators signing)" "2.3"
    elif [ "$SIG_COUNT" -eq 1 ]; then
        log_warn "Only 1 signature on block" "2.3"
    else
        log_fail "No signatures found" "2.3"
    fi

    # -------------------------------------------------------------------------
    # Test 2.4: Proposer Rotation
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "2.4" "Proposer Rotation Check"

    PROPOSERS=()
    CURRENT_HEIGHT=$(posd status 2>&1 | jq -r '.sync_info.latest_block_height' 2>/dev/null || echo "0")

    for i in {0..9}; do
        H=$((CURRENT_HEIGHT - i))
        PROPOSER=$(curl -s http://localhost:26657/block?height=$H 2>/dev/null | jq -r '.result.block.header.proposer_address' 2>/dev/null || echo "")
        if [ -n "$PROPOSER" ]; then
            PROPOSERS+=("$PROPOSER")
        fi
    done

    UNIQUE_PROPOSERS=$(printf '%s\n' "${PROPOSERS[@]}" | sort -u | wc -l)
    log_info "Unique proposers in last 10 blocks: $UNIQUE_PROPOSERS"

    if [ $UNIQUE_PROPOSERS -ge 2 ]; then
        log_pass "Proposer rotation working ($UNIQUE_PROPOSERS proposers)" "2.4"
    elif [ $UNIQUE_PROPOSERS -eq 1 ]; then
        log_warn "Only 1 proposer seen (may need more blocks for rotation)" "2.4"
    else
        log_fail "No proposers found" "2.4"
    fi
}

# =============================================================================
# PHASE 2: MEMPOOL, FEEMARKET, POC, NETWORK
# =============================================================================

run_phase2() {
    log_header "PHASE 2: MEMPOOL, FEEMARKET, POC, NETWORK TESTS"

    # -------------------------------------------------------------------------
    # Test 3.1: Basic Send Transaction
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "3.1" "Basic Send Transaction"

    VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test 2>/dev/null || echo "")

    if [ -z "$VALIDATOR_ADDR" ]; then
        log_fail "Could not get validator address" "3.1"
    else
        log_info "Validator: $VALIDATOR_ADDR"

        # Send 1 OMNI to self
        TX_RESULT=$(posd tx bank send $VALIDATOR_ADDR $VALIDATOR_ADDR 1000000${DENOM} \
            --from validator \
            --chain-id $CHAIN_ID \
            --keyring-backend test \
            --fees 100000${DENOM} \
            --gas 100000 \
            -y 2>&1)

        TX_CODE=$(echo "$TX_RESULT" | grep -o '"code":[0-9]*' | head -1 | grep -o '[0-9]*' || echo "999")

        if [ "$TX_CODE" = "0" ] || echo "$TX_RESULT" | grep -q "txhash"; then
            log_pass "Transaction submitted successfully" "3.1"
        else
            log_fail "Transaction failed (code: $TX_CODE)" "3.1"
        fi
    fi

    sleep 6

    # -------------------------------------------------------------------------
    # Test 3.2: Mempool Status
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "3.2" "Mempool Status Check"

    MEMPOOL_SIZE=$(curl -s http://localhost:26657/num_unconfirmed_txs 2>/dev/null | jq -r '.result.n_txs' 2>/dev/null || echo "-1")
    log_info "Mempool size: $MEMPOOL_SIZE"

    if [ "$MEMPOOL_SIZE" = "0" ]; then
        log_pass "Mempool is empty (transactions processed)" "3.2"
    elif [ "$MEMPOOL_SIZE" -lt 10 ]; then
        log_pass "Mempool has $MEMPOOL_SIZE pending transactions" "3.2"
    elif [ "$MEMPOOL_SIZE" -lt 50 ]; then
        log_warn "Mempool has $MEMPOOL_SIZE pending transactions" "3.2"
    else
        log_fail "Mempool congested ($MEMPOOL_SIZE txs)" "3.2"
    fi

    # -------------------------------------------------------------------------
    # Test 4.1: Fee Market Parameters
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "4.1" "Fee Market Parameters Query"

    FEEMARKET_PARAMS=$(posd query feemarket params --output json 2>/dev/null)

    if [ -n "$FEEMARKET_PARAMS" ]; then
        MIN_GAS_PRICE=$(echo "$FEEMARKET_PARAMS" | jq -r '.params.min_gas_price' 2>/dev/null || echo "N/A")
        log_info "Min gas price: $MIN_GAS_PRICE"
        log_pass "Fee market params queryable" "4.1"
    else
        log_fail "Could not query feemarket params" "4.1"
    fi

    # -------------------------------------------------------------------------
    # Test 4.2: Min Gas Price Enforcement
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "4.2" "Min Gas Price Enforcement (should reject 0 fee)"

    VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test 2>/dev/null || echo "")

    if [ -n "$VALIDATOR_ADDR" ]; then
        # Try with 0 fee - should fail
        ZERO_FEE_RESULT=$(posd tx bank send $VALIDATOR_ADDR $VALIDATOR_ADDR 1${DENOM} \
            --from validator \
            --chain-id $CHAIN_ID \
            --keyring-backend test \
            --fees 0${DENOM} \
            --gas 100000 \
            -y 2>&1)

        if echo "$ZERO_FEE_RESULT" | grep -qi "insufficient\|error\|fail"; then
            log_pass "Zero-fee transaction correctly rejected" "4.2"
        else
            log_fail "Zero-fee transaction was not rejected" "4.2"
        fi
    else
        log_fail "Could not get validator address" "4.2"
    fi

    # -------------------------------------------------------------------------
    # Test 4.3: MaxTxGas Enforcement
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "4.3" "MaxTxGas Enforcement (should reject >2M gas)"

    VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test 2>/dev/null || echo "")

    if [ -n "$VALIDATOR_ADDR" ]; then
        # Try with 3M gas - should fail
        OVERSIZED_RESULT=$(posd tx bank send $VALIDATOR_ADDR $VALIDATOR_ADDR 1${DENOM} \
            --from validator \
            --chain-id $CHAIN_ID \
            --keyring-backend test \
            --fees 100000${DENOM} \
            --gas 3000000 \
            -y 2>&1)

        if echo "$OVERSIZED_RESULT" | grep -qi "exceeds\|max\|error\|fail\|invalid"; then
            log_pass "Oversized transaction correctly rejected" "4.3"
        else
            log_fail "Oversized transaction was not rejected" "4.3"
        fi
    else
        log_fail "Could not get validator address" "4.3"
    fi

    # -------------------------------------------------------------------------
    # Test 4.4: Token Supply Query
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "4.4" "Token Supply Query"

    TOTAL_SUPPLY=$(posd query bank total --output json 2>/dev/null | jq -r ".supply[] | select(.denom==\"${DENOM}\") | .amount" 2>/dev/null || echo "0")

    if [ "$TOTAL_SUPPLY" != "0" ] && [ -n "$TOTAL_SUPPLY" ]; then
        # Format supply
        SUPPLY_OMNI=$(echo "scale=2; $TOTAL_SUPPLY / 1000000" | bc 2>/dev/null || echo "$TOTAL_SUPPLY")
        log_info "Total supply: $SUPPLY_OMNI OMNI"
        log_pass "Supply queryable" "4.4"
    else
        log_fail "Could not query supply" "4.4"
    fi

    # -------------------------------------------------------------------------
    # Test 5.1: PoC Module Parameters
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "5.1" "PoC Module Parameters"

    POC_PARAMS=$(posd query poc params --output json 2>/dev/null)

    if [ -n "$POC_PARAMS" ]; then
        log_info "PoC params: $(echo "$POC_PARAMS" | jq -c '.' 2>/dev/null || echo "$POC_PARAMS")"
        log_pass "PoC params queryable" "5.1"
    else
        log_warn "Could not query PoC params (module may not be active)" "5.1"
    fi

    # -------------------------------------------------------------------------
    # Test 5.2: Tokenomics Module
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "5.2" "Tokenomics Module Query"

    TOKENOMICS=$(posd query tokenomics params --output json 2>/dev/null)

    if [ -n "$TOKENOMICS" ]; then
        log_info "Tokenomics: $(echo "$TOKENOMICS" | jq -c '.' 2>/dev/null || echo "$TOKENOMICS")"
        log_pass "Tokenomics queryable" "5.2"
    else
        log_warn "Could not query tokenomics (module may not be active)" "5.2"
    fi

    # -------------------------------------------------------------------------
    # Test 6.1: Peer Discovery
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "6.1" "Peer Discovery"

    PEER_COUNT=$(curl -s http://localhost:26657/net_info 2>/dev/null | jq -r '.result.n_peers' 2>/dev/null || echo "0")
    log_info "Connected peers: $PEER_COUNT"

    if [ "$PEER_COUNT" -ge 1 ]; then
        log_pass "Node has $PEER_COUNT peer(s)" "6.1"
    else
        log_fail "No peers connected" "6.1"
    fi

    # -------------------------------------------------------------------------
    # Test 6.2: RPC Endpoint Health (VPS1)
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "6.2" "RPC Endpoint Health (VPS1)"

    VPS1_HEALTH=$(curl -s --connect-timeout 5 http://${VPS1_IP}:26657/health 2>/dev/null)

    if echo "$VPS1_HEALTH" | grep -q "result"; then
        log_pass "VPS1 RPC healthy" "6.2"
    else
        log_fail "VPS1 RPC not responding" "6.2"
    fi

    # -------------------------------------------------------------------------
    # Test 6.3: RPC Endpoint Health (VPS2)
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "6.3" "RPC Endpoint Health (VPS2)"

    VPS2_HEALTH=$(curl -s --connect-timeout 5 http://${VPS2_IP}:26657/health 2>/dev/null)

    if echo "$VPS2_HEALTH" | grep -q "result"; then
        log_pass "VPS2 RPC healthy" "6.3"
    else
        log_warn "VPS2 RPC not responding (may be down)" "6.3"
    fi

    # -------------------------------------------------------------------------
    # Test 6.4: Staking Queries
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "6.4" "Staking Module Queries"

    STAKING_PARAMS=$(posd query staking params --output json 2>/dev/null)

    if [ -n "$STAKING_PARAMS" ]; then
        BOND_DENOM=$(echo "$STAKING_PARAMS" | jq -r '.params.bond_denom' 2>/dev/null || echo "N/A")
        log_info "Bond denom: $BOND_DENOM"
        log_pass "Staking params queryable" "6.4"
    else
        log_fail "Could not query staking params" "6.4"
    fi

    # -------------------------------------------------------------------------
    # Test 6.5: Distribution Module
    # -------------------------------------------------------------------------
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_test "6.5" "Distribution Module Query"

    VALIDATOR_ADDR=$(posd keys show validator -a --keyring-backend test 2>/dev/null || echo "")

    if [ -n "$VALIDATOR_ADDR" ]; then
        REWARDS=$(posd query distribution rewards $VALIDATOR_ADDR --output json 2>/dev/null)
        if [ -n "$REWARDS" ]; then
            log_info "Rewards query successful"
            log_pass "Distribution module working" "6.5"
        else
            log_warn "No rewards data returned" "6.5"
        fi
    else
        log_fail "Could not get validator address" "6.5"
    fi
}

# =============================================================================
# SUMMARY REPORT
# =============================================================================

generate_report() {
    log_header "TEST SUMMARY REPORT"

    echo -e "Chain ID:       ${CYAN}$CHAIN_ID${NC}"
    echo -e "Test Date:      ${CYAN}$(date)${NC}"
    echo -e "Block Height:   ${CYAN}$(posd status 2>&1 | jq -r '.sync_info.latest_block_height' 2>/dev/null || echo 'N/A')${NC}"
    echo ""

    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "  RESULTS"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""

    echo -e "  Total Tests:    $TOTAL_TESTS"
    echo -e "  ${GREEN}Passed:${NC}         $PASSED_TESTS"
    echo -e "  ${YELLOW}Warnings:${NC}       $WARNED_TESTS"
    echo -e "  ${RED}Failed:${NC}         $FAILED_TESTS"
    echo ""

    # Calculate pass rate
    if [ $TOTAL_TESTS -gt 0 ]; then
        PASS_RATE=$(echo "scale=1; ($PASSED_TESTS * 100) / $TOTAL_TESTS" | bc 2>/dev/null || echo "N/A")
        echo -e "  Pass Rate:      ${CYAN}${PASS_RATE}%${NC}"
    fi

    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "  DETAILED RESULTS"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""

    # Print all results
    for test_id in $(echo "${!RESULTS[@]}" | tr ' ' '\n' | sort); do
        result="${RESULTS[$test_id]}"
        case $result in
            "PASS") echo -e "  ${GREEN}[PASS]${NC} Test $test_id" ;;
            "WARN") echo -e "  ${YELLOW}[WARN]${NC} Test $test_id" ;;
            "FAIL") echo -e "  ${RED}[FAIL]${NC} Test $test_id" ;;
        esac
    done

    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

    # Overall assessment
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}"
        echo "  ╔═══════════════════════════════════════════════════════════╗"
        echo "  ║           ALL TESTS PASSED - CHAIN HEALTHY                ║"
        echo "  ╚═══════════════════════════════════════════════════════════╝"
        echo -e "${NC}"
    elif [ $FAILED_TESTS -le 2 ]; then
        echo -e "${YELLOW}"
        echo "  ╔═══════════════════════════════════════════════════════════╗"
        echo "  ║     MINOR ISSUES DETECTED - REVIEW FAILED TESTS           ║"
        echo "  ╚═══════════════════════════════════════════════════════════╝"
        echo -e "${NC}"
    else
        echo -e "${RED}"
        echo "  ╔═══════════════════════════════════════════════════════════╗"
        echo "  ║       CRITICAL ISSUES - IMMEDIATE ACTION REQUIRED         ║"
        echo "  ╚═══════════════════════════════════════════════════════════╝"
        echo -e "${NC}"
    fi
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

main() {
    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║     OMNIPHI TESTNET - PHASE 1 & PHASE 2 TEST SUITE           ║${NC}"
    echo -e "${BLUE}║     Chain ID: $CHAIN_ID                            ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Check if posd is available
    if ! command -v posd &> /dev/null; then
        echo -e "${RED}ERROR: posd command not found${NC}"
        exit 1
    fi

    # Check if node is running
    if ! posd status &> /dev/null; then
        echo -e "${RED}ERROR: Node not running or not accessible${NC}"
        exit 1
    fi

    echo -e "${GREEN}Node is running. Starting tests...${NC}"
    echo ""

    # Run Phase 1
    run_phase1

    # Run Phase 2
    run_phase2

    # Generate report
    generate_report
}

# Run main
main "$@"
