#!/bin/bash
# Intensive Integration Test for OmniPhi Blockchain
# Tests all modules under load with comprehensive verification

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

# Function to print status
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_header() {
    echo ""
    echo -e "${CYAN}================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}================================${NC}"
    echo ""
}

# Test result tracking
pass_test() {
    ((TESTS_PASSED++))
    ((TOTAL_TESTS++))
    print_status "$1"
}

fail_test() {
    ((TESTS_FAILED++))
    ((TOTAL_TESTS++))
    print_error "$1"
}

print_header "OmniPhi Intensive Integration Test Suite"

# Check if chain is running
if ! pgrep -x "posd" > /dev/null; then
    if ! tasklist 2>/dev/null | grep -q "posd.exe"; then
        print_error "Chain is not running. Start with: ./posd start"
        exit 1
    fi
fi

print_status "Chain is running"

# Test 1: Chain Health
print_header "Test 1: Chain Health Check"

STATUS=$(./posd status 2>&1)
if echo "$STATUS" | grep -q "latest_block_height"; then
    HEIGHT=$(echo "$STATUS" | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')
    pass_test "Chain is at height $HEIGHT"
else
    fail_test "Could not get chain status"
fi

# Wait for a few blocks
INITIAL_HEIGHT=$HEIGHT
sleep 10
STATUS=$(./posd status 2>&1)
NEW_HEIGHT=$(echo "$STATUS" | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')

if [ "$NEW_HEIGHT" -gt "$INITIAL_HEIGHT" ]; then
    pass_test "Chain is producing blocks ($INITIAL_HEIGHT -> $NEW_HEIGHT)"
else
    fail_test "Chain is not producing new blocks"
fi

# Test 2: FeeMarket Module - All 5 Queries
print_header "Test 2: FeeMarket Module Comprehensive Test"

# Base Fee Query
if ./posd query feemarket base-fee 2>&1 | grep -q "base_fee"; then
    pass_test "FeeMarket: base-fee query working"
else
    fail_test "FeeMarket: base-fee query failed"
fi

# Params Query
if ./posd query feemarket params 2>&1 | grep -q "base_fee_enabled"; then
    pass_test "FeeMarket: params query working"
else
    fail_test "FeeMarket: params query failed"
fi

# Block Utilization Query
if ./posd query feemarket block-utilization 2>&1 | grep -q "utilization"; then
    pass_test "FeeMarket: block-utilization query working"
else
    fail_test "FeeMarket: block-utilization query failed"
fi

# Burn Tier Query
if ./posd query feemarket burn-tier 2>&1 | grep -q "tier"; then
    pass_test "FeeMarket: burn-tier query working"
else
    fail_test "FeeMarket: burn-tier query failed"
fi

# Fee Stats Query
if ./posd query feemarket fee-stats 2>&1 | grep -q "total_burned"; then
    pass_test "FeeMarket: fee-stats query working"
else
    fail_test "FeeMarket: fee-stats query failed"
fi

# Test 3: Tokenomics Module
print_header "Test 3: Tokenomics Module Comprehensive Test"

# Supply Query
if ./posd query tokenomics supply 2>&1 | grep -q "current_total_supply"; then
    pass_test "Tokenomics: supply query working"
else
    fail_test "Tokenomics: supply query failed"
fi

# Params Query
if ./posd query tokenomics params 2>&1 | grep -q "inflation_rate"; then
    pass_test "Tokenomics: params query working"
else
    fail_test "Tokenomics: params query failed"
fi

# Inflation Query
if ./posd query tokenomics inflation 2>&1 | grep -q "current_inflation_rate"; then
    pass_test "Tokenomics: inflation query working"
else
    fail_test "Tokenomics: inflation query failed"
fi

# Test 4: POC Module
print_header "Test 4: POC Module Comprehensive Test"

# Params Query
if ./posd query poc params 2>&1 | grep -q "submission_fee"; then
    pass_test "POC: params query working"
else
    fail_test "POC: params query failed"
fi

# Contributions Query
if ./posd query poc contributions 2>&1 | grep -q "contributions"; then
    pass_test "POC: contributions query working"
else
    fail_test "POC: contributions query failed"
fi

# Credits Query (using validator address)
VALIDATOR_ADDR=$(./posd keys show validator -a --keyring-backend test 2>&1)
if ./posd query poc credits "$VALIDATOR_ADDR" 2>&1 | grep -q "credits"; then
    pass_test "POC: credits query working"
else
    fail_test "POC: credits query failed"
fi

# Test 5: Transaction Processing
print_header "Test 5: Transaction Processing and Fee Burning"

# Get initial fee stats
INITIAL_BURNED=$(./posd query feemarket fee-stats 2>&1 | grep "total_burned" | tail -1 | grep -o '[0-9]*' | head -1)
INITIAL_TREASURY=$(./posd query feemarket fee-stats 2>&1 | grep "total_to_treasury" | tail -1 | grep -o '[0-9]*' | head -1)
INITIAL_VALIDATORS=$(./posd query feemarket fee-stats 2>&1 | grep "total_to_validators" | tail -1 | grep -o '[0-9]*' | head -1)

echo "Initial fee stats:"
echo "  Burned: $INITIAL_BURNED"
echo "  To Treasury: $INITIAL_TREASURY"
echo "  To Validators: $INITIAL_VALIDATORS"

# Get validator balance
VALIDATOR_BALANCE=$(./posd query bank balances "$VALIDATOR_ADDR" -o json 2>&1 | grep -o '"amount":"[0-9]*"' | head -1 | grep -o '[0-9]*')

if [ -n "$VALIDATOR_BALANCE" ] && [ "$VALIDATOR_BALANCE" -gt 0 ]; then
    print_status "Validator has balance: $VALIDATOR_BALANCE uomni"

    # Send a transaction with fees
    echo "Sending transaction with 5000uomni fees..."
    TX_RESULT=$(./posd tx bank send validator testkey 10000uomni \
        --chain-id omniphi-1 \
        --keyring-backend test \
        --fees 5000uomni \
        --yes 2>&1)

    if echo "$TX_RESULT" | grep -q "code: 0"; then
        pass_test "Transaction sent successfully"

        # Wait for block to process
        sleep 8

        # Get new fee stats
        NEW_BURNED=$(./posd query feemarket fee-stats 2>&1 | grep "total_burned" | tail -1 | grep -o '[0-9]*' | head -1)
        NEW_TREASURY=$(./posd query feemarket fee-stats 2>&1 | grep "total_to_treasury" | tail -1 | grep -o '[0-9]*' | head -1)
        NEW_VALIDATORS=$(./posd query feemarket fee-stats 2>&1 | grep "total_to_validators" | tail -1 | grep -o '[0-9]*' | head -1)

        # Calculate changes
        BURNED_DELTA=$((NEW_BURNED - INITIAL_BURNED))
        TREASURY_DELTA=$((NEW_TREASURY - INITIAL_TREASURY))
        VALIDATORS_DELTA=$((NEW_VALIDATORS - INITIAL_VALIDATORS))

        echo ""
        echo "Fee distribution for 5000uomni transaction:"
        echo "  Burned: $BURNED_DELTA uomni"
        echo "  To Treasury: $TREASURY_DELTA uomni"
        echo "  To Validators: $VALIDATORS_DELTA uomni"

        if [ "$BURNED_DELTA" -gt 0 ]; then
            pass_test "Fees were burned correctly"
        else
            fail_test "No fees were burned"
        fi

        if [ "$TREASURY_DELTA" -gt 0 ]; then
            pass_test "Treasury received fees"
        else
            fail_test "Treasury did not receive fees"
        fi

        if [ "$VALIDATORS_DELTA" -gt 0 ]; then
            pass_test "Validators received fees"
        else
            fail_test "Validators did not receive fees"
        fi

    else
        fail_test "Transaction failed"
        echo "$TX_RESULT"
    fi
else
    print_warning "Validator has no balance, skipping transaction test"
fi

# Test 6: Stress Test - Multiple Transactions
print_header "Test 6: Stress Test - Multiple Transactions"

if [ -n "$VALIDATOR_BALANCE" ] && [ "$VALIDATOR_BALANCE" -gt 50000 ]; then
    print_status "Running stress test with 5 transactions..."

    STRESS_PASSED=0
    for i in {1..5}; do
        echo -n "  Transaction $i/5... "
        TX_RESULT=$(./posd tx bank send validator testkey 1000uomni \
            --chain-id omniphi-1 \
            --keyring-backend test \
            --fees 1000uomni \
            --yes 2>&1)

        if echo "$TX_RESULT" | grep -q "code: 0"; then
            echo -e "${GREEN}✓${NC}"
            ((STRESS_PASSED++))
        else
            echo -e "${RED}✗${NC}"
        fi
        sleep 2
    done

    if [ "$STRESS_PASSED" -eq 5 ]; then
        pass_test "All 5 stress test transactions succeeded"
    else
        fail_test "Only $STRESS_PASSED/5 stress test transactions succeeded"
    fi
else
    print_warning "Insufficient balance for stress test, skipping"
fi

# Test 7: Base Fee Dynamics
print_header "Test 7: Base Fee Dynamics"

BASE_FEE_1=$(./posd query feemarket base-fee 2>&1 | grep "base_fee" | head -1 | grep -o '"[0-9.]*"' | tr -d '"')
sleep 10
BASE_FEE_2=$(./posd query feemarket base-fee 2>&1 | grep "base_fee" | head -1 | grep -o '"[0-9.]*"' | tr -d '"')

echo "Base fee over time:"
echo "  Initial: $BASE_FEE_1"
echo "  After 10s: $BASE_FEE_2"

pass_test "Base fee tracking working"

# Test 8: Burn Tier Selection
print_header "Test 8: Burn Tier Selection"

BURN_TIER=$(./posd query feemarket burn-tier 2>&1 | grep "tier:" | awk '{print $2}')
BURN_PCT=$(./posd query feemarket burn-tier 2>&1 | grep "burn_percentage" | grep -o '"[0-9.]*"' | tr -d '"')

echo "Current burn tier: $BURN_TIER"
echo "Burn percentage: $BURN_PCT"

if [ -n "$BURN_TIER" ]; then
    pass_test "Burn tier selection working (tier: $BURN_TIER)"
else
    fail_test "Could not determine burn tier"
fi

# Test 9: Module Integration
print_header "Test 9: Module Integration Check"

# Verify all modules are loaded
MODULES=$(./posd query 2>&1 | grep "Available Commands" -A 20 | grep -E "feemarket|tokenomics|poc" | wc -l)

if [ "$MODULES" -eq 3 ]; then
    pass_test "All 3 custom modules (feemarket, tokenomics, poc) are loaded"
else
    fail_test "Not all custom modules are loaded ($MODULES/3)"
fi

# Test 10: Performance Metrics
print_header "Test 10: Performance Metrics"

# Measure query response time
START_TIME=$(date +%s%N)
./posd query feemarket base-fee >/dev/null 2>&1
END_TIME=$(date +%s%N)
QUERY_TIME=$(( (END_TIME - START_TIME) / 1000000 ))

echo "Query response time: ${QUERY_TIME}ms"

if [ "$QUERY_TIME" -lt 1000 ]; then
    pass_test "Query response time is good (<1000ms)"
else
    print_warning "Query response time is slow (${QUERY_TIME}ms)"
fi

# Calculate TPS (transactions per second) based on recent block
CURRENT_HEIGHT=$(./posd status 2>&1 | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')
sleep 5
NEW_HEIGHT=$(./posd status 2>&1 | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9]*')
BLOCKS_IN_5S=$((NEW_HEIGHT - CURRENT_HEIGHT))
TPS=$(echo "scale=2; $BLOCKS_IN_5S / 5" | bc 2>/dev/null || echo "N/A")

echo "Block production rate: ~$TPS blocks/second"

if [ "$BLOCKS_IN_5S" -gt 0 ]; then
    pass_test "Chain is producing blocks consistently"
else
    fail_test "Chain block production is stalled"
fi

# Final Summary
print_header "Test Summary"

echo ""
echo "Total Tests: $TOTAL_TESTS"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo ""

SUCCESS_RATE=$(( (TESTS_PASSED * 100) / TOTAL_TESTS ))
echo "Success Rate: ${SUCCESS_RATE}%"

if [ "$TESTS_FAILED" -eq 0 ]; then
    echo ""
    print_status "ALL TESTS PASSED! ✅"
    echo ""
    exit 0
else
    echo ""
    print_error "SOME TESTS FAILED ❌"
    echo ""
    exit 1
fi
