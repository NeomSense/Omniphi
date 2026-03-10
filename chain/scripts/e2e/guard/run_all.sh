#!/bin/bash
# =============================================================================
# Guard E2E Test Runner
# =============================================================================
# Runs all x/guard E2E scenarios in order. Fails fast on first failure.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

echo ""
echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}  Omniphi x/guard E2E Test Suite${NC}"
echo -e "${BLUE}============================================================${NC}"
echo ""
echo -e "${CYAN}Node:       $POSD_NODE${NC}"
echo -e "${CYAN}Chain ID:   $POSD_CHAIN_ID${NC}"
echo -e "${CYAN}Validator:  $VALIDATOR_KEY${NC}"
echo -e "${CYAN}Block time: ${BLOCK_TIME}s${NC}"
echo ""

check_prerequisites

# ---- Optional: Set short guard delays for fast E2E ----

setup_short_guard_delays

# ---- Run scenarios ---------------------------------------------------------

SCENARIOS=(
    "01_software_upgrade_critical.sh"
    "02_community_pool_spend_treasury_bps.sh"
    "03_gate_transitions.sh"
    "04_confirm_execution_critical.sh"
    "05_stability_extension_validator_churn.sh"
    "06_emergency_veto_abort.sh"
)

TOTAL=${#SCENARIOS[@]}
PASSED=0
FAILED=0
SKIPPED=0

START_TIME=$(date +%s)

for i in "${!SCENARIOS[@]}"; do
    SCENARIO="${SCENARIOS[$i]}"
    NUM=$((i + 1))

    echo ""
    echo -e "${MAGENTA}------------------------------------------------------------${NC}"
    echo -e "${MAGENTA}  [$NUM/$TOTAL] $SCENARIO${NC}"
    echo -e "${MAGENTA}------------------------------------------------------------${NC}"
    echo ""

    SCENARIO_PATH="$SCRIPT_DIR/$SCENARIO"

    if [[ ! -f "$SCENARIO_PATH" ]]; then
        echo -e "${YELLOW}  SKIPPED (file not found)${NC}"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    if ! chmod +x "$SCENARIO_PATH"; then
        echo -e "${RED}  Could not make executable${NC}"
        FAILED=$((FAILED + 1))
        break
    fi

    SCENARIO_START=$(date +%s)

    if bash "$SCENARIO_PATH"; then
        SCENARIO_ELAPSED=$(( $(date +%s) - SCENARIO_START ))
        echo -e "${GREEN}  Completed in ${SCENARIO_ELAPSED}s${NC}"
        PASSED=$((PASSED + 1))
    else
        SCENARIO_ELAPSED=$(( $(date +%s) - SCENARIO_START ))
        echo -e "${RED}  FAILED after ${SCENARIO_ELAPSED}s${NC}"
        FAILED=$((FAILED + 1))
        echo ""
        echo -e "${RED}Stopping on first failure.${NC}"
        break
    fi
done

# ---- Summary ---------------------------------------------------------------

TOTAL_ELAPSED=$(( $(date +%s) - START_TIME ))

echo ""
echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}  E2E Test Summary${NC}"
echo -e "${BLUE}============================================================${NC}"
echo ""
echo -e "  Total:   $TOTAL"
echo -e "  ${GREEN}Passed:  $PASSED${NC}"
if (( FAILED > 0 )); then
    echo -e "  ${RED}Failed:  $FAILED${NC}"
fi
if (( SKIPPED > 0 )); then
    echo -e "  ${YELLOW}Skipped: $SKIPPED${NC}"
fi
echo -e "  Time:    ${TOTAL_ELAPSED}s"
echo ""

if (( FAILED > 0 )); then
    echo -e "${RED}SUITE FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}ALL SCENARIOS PASSED${NC}"
    exit 0
fi
