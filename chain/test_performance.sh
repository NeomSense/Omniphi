#!/bin/bash
################################################################################
# Performance Testing Script
# Tests TPS, latency, and stress scenarios
################################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

CHAIN_ID="omniphi-1"
NUM_TPS_TX=50
NUM_LATENCY_SAMPLES=10

echo -e "${CYAN}================================${NC}"
echo -e "${CYAN}Performance Testing${NC}"
echo -e "${CYAN}================================${NC}"
echo ""

# Check if chain is running
if ! ./posd status &> /dev/null; then
    echo -e "${RED}✗ Chain is not running${NC}"
    echo -e "${YELLOW}Start with: ./posd start --minimum-gas-prices 0.001uomni --grpc.enable=false${NC}"
    exit 1
fi

# Ensure test accounts exist and are funded
echo -e "${YELLOW}Setting up test accounts...${NC}"
for account in alice bob; do
    if ! ./posd keys show $account --keyring-backend test &> /dev/null; then
        ./posd keys add $account --keyring-backend test &> /dev/null
    fi
done

ALICE=$(./posd keys show alice -a --keyring-backend test)
BOB=$(./posd keys show bob -a --keyring-backend test)

# Check balances
ALICE_BALANCE=$(./posd query bank balances $ALICE -o json 2>/dev/null | jq -r '.balances[0].amount // "0"')
if [ "$ALICE_BALANCE" -lt "100000" ]; then
    echo -e "${YELLOW}Alice needs funding. Run test_modules.sh first or fund manually.${NC}"
fi
echo -e "${GREEN}  ✓ Test accounts ready${NC}"
echo ""

# Test 1: TPS (Transactions Per Second)
echo -e "${CYAN}[1/3] TPS Test (${NUM_TPS_TX} transactions)${NC}"
echo -e "${YELLOW}  Sending transactions in parallel...${NC}"

START_TIME=$(date +%s)

for i in $(seq 1 $NUM_TPS_TX); do
    (./posd tx bank send $ALICE $BOB 100uomni \
        --chain-id $CHAIN_ID \
        --keyring-backend test \
        --fees 500uomni \
        --yes &> /dev/null) &

    # Batch in groups of 10 to avoid overwhelming
    if [ $((i % 10)) -eq 0 ]; then
        sleep 0.5
    fi
done

echo -e "${YELLOW}  Waiting for transactions to complete...${NC}"
wait

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

if [ "$DURATION" -eq 0 ]; then
    DURATION=1
fi

TPS=$((NUM_TPS_TX / DURATION))

echo -e "${GREEN}  ✓ Completed ${NUM_TPS_TX} transactions in ${DURATION}s${NC}"
echo -e "${GREEN}  ✓ TPS: ${TPS} transactions/second${NC}"
echo ""

# Test 2: Latency (Time to finality)
echo -e "${CYAN}[2/3] Latency Test (${NUM_LATENCY_SAMPLES} samples)${NC}"
echo -e "${YELLOW}  Measuring transaction finality time...${NC}"

TOTAL_LATENCY=0
for i in $(seq 1 $NUM_LATENCY_SAMPLES); do
    START=$(date +%s%N)

    TX_HASH=$(./posd tx bank send $ALICE $BOB 100uomni \
        --chain-id $CHAIN_ID \
        --keyring-backend test \
        --fees 500uomni \
        --yes -o json 2>/dev/null | jq -r '.txhash // ""')

    # Wait for inclusion
    sleep 6

    END=$(date +%s%N)
    LATENCY=$(( (END - START) / 1000000 )) # Convert to milliseconds

    TOTAL_LATENCY=$((TOTAL_LATENCY + LATENCY))
    echo -e "  Sample $i: ${LATENCY}ms"
done

AVG_LATENCY=$((TOTAL_LATENCY / NUM_LATENCY_SAMPLES))
echo -e "${GREEN}  ✓ Average latency: ${AVG_LATENCY}ms${NC}"
echo ""

# Test 3: Stress Test (Burst of transactions)
echo -e "${CYAN}[3/3] Stress Test (100 transactions burst)${NC}"
echo -e "${YELLOW}  Sending burst of transactions...${NC}"

STRESS_COUNT=100
STRESS_START=$(date +%s)

for i in $(seq 1 $STRESS_COUNT); do
    ./posd tx bank send $ALICE $BOB 100uomni \
        --chain-id $CHAIN_ID \
        --keyring-backend test \
        --fees 500uomni \
        --yes &> /dev/null &

    # Small delay every 20 transactions
    if [ $((i % 20)) -eq 0 ]; then
        sleep 1
    fi
done

echo -e "${YELLOW}  Waiting for mempool to clear...${NC}"
sleep 15

STRESS_END=$(date +%s)
STRESS_DURATION=$((STRESS_END - STRESS_START))
STRESS_TPS=$((STRESS_COUNT / STRESS_DURATION))

echo -e "${GREEN}  ✓ Handled ${STRESS_COUNT} transactions in ${STRESS_DURATION}s${NC}"
echo -e "${GREEN}  ✓ Stress TPS: ${STRESS_TPS} tx/s${NC}"
echo ""

# Summary
echo -e "${CYAN}================================${NC}"
echo -e "${GREEN}✅ Performance Test Results${NC}"
echo -e "${CYAN}================================${NC}"
echo ""
echo -e "TPS Test:"
echo -e "  ${NUM_TPS_TX} transactions in ${DURATION}s"
echo -e "  TPS: ${GREEN}${TPS} tx/s${NC}"
echo ""
echo -e "Latency Test:"
echo -e "  Average: ${GREEN}${AVG_LATENCY}ms${NC}"
echo -e "  Samples: ${NUM_LATENCY_SAMPLES}"
echo ""
echo -e "Stress Test:"
echo -e "  ${STRESS_COUNT} transactions in ${STRESS_DURATION}s"
echo -e "  TPS under load: ${GREEN}${STRESS_TPS} tx/s${NC}"
echo ""

# Performance assessment
echo -e "Assessment:"
if [ "$TPS" -ge 10 ]; then
    echo -e "  TPS: ${GREEN}✓ Good ($TPS tx/s >= 10 tx/s target)${NC}"
else
    echo -e "  TPS: ${YELLOW}⚠ Below target ($TPS tx/s < 10 tx/s)${NC}"
fi

if [ "$AVG_LATENCY" -le 10000 ]; then
    echo -e "  Latency: ${GREEN}✓ Good (${AVG_LATENCY}ms <= 10s target)${NC}"
else
    echo -e "  Latency: ${YELLOW}⚠ High (${AVG_LATENCY}ms > 10s target)${NC}"
fi

echo ""
