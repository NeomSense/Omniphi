#!/bin/bash
# =============================================================================
# Scenario 02: Community Pool Spend → Treasury BPS Evaluation
# =============================================================================
# Verifies that x/guard computes treasury spend percentage (bps) for
# MsgCommunityPoolSpend proposals and assigns risk tier accordingly.
# Tests both small (LOW) and large (HIGH/CRITICAL) spend amounts.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

log_section "Scenario 02: Community Pool Spend Treasury BPS"

check_prerequisites

AUTHORITY=$(gov_authority)
VALIDATOR_ADDR=$(posd keys show "$VALIDATOR_KEY" --keyring-backend "$KEYRING_BACKEND" -a 2>/dev/null)
log_info "Governance authority: $AUTHORITY"
log_info "Validator address: $VALIDATOR_ADDR"

# ==========================================================================
# Test A: Small spend → LOW or MED tier
# ==========================================================================

log_section "Test A: Small Community Pool Spend"

FIXTURE_SMALL="$SCRIPT_DIR/fixtures/community_pool_spend_small.json"
PROPOSAL_SMALL="/tmp/e2e_guard_02_small_spend.json"

# Replace placeholders
sed -e "s|AUTHORITY_PLACEHOLDER|$AUTHORITY|g" \
    -e "s|RECIPIENT_PLACEHOLDER|$VALIDATOR_ADDR|g" \
    "$FIXTURE_SMALL" > "$PROPOSAL_SMALL"

log_step "Submitting small community pool spend proposal"
PID_SMALL=$(submit_gov_proposal "$VALIDATOR_KEY" "$PROPOSAL_SMALL")
log_info "Small spend proposal ID: $PID_SMALL"

vote_proposal "$VALIDATOR_KEY" "$PID_SMALL" "yes"

log_step "Waiting for small spend proposal to pass"
poll_until 360 "$BLOCK_TIME" \
    "query_proposal_status $PID_SMALL" \
    '.proposal.status // .status' \
    'PROPOSAL_STATUS_PASSED' || \
    log_fail "Small spend proposal did not pass"

wait_blocks 3 60

log_step "Querying risk report for small spend"
RISK_SMALL=$(poll_until 60 "$BLOCK_TIME" \
    "query_risk_report $PID_SMALL" \
    '.risk_report.proposal_id // .proposal_id' \
    "$PID_SMALL") || log_fail "No risk report for small spend proposal"

TIER_SMALL=$(jq_get "$RISK_SMALL" '.risk_report.tier // .tier')
REASON_SMALL=$(jq_get "$RISK_SMALL" '.risk_report.reason_codes // .reason_codes')

log_info "Small spend tier: $TIER_SMALL"
log_info "Small spend reasons: $REASON_SMALL"

# Small spend should be LOW or MED
assert_in "$TIER_SMALL" "RISK_TIER_LOW,RISK_TIER_MED,1,2" \
    "Small community pool spend is LOW or MED"

# Reason codes should contain a treasury code
assert_contains "$REASON_SMALL" "TREASURY_SPEND" \
    "Small spend reason includes TREASURY_SPEND"

# ==========================================================================
# Test B: Large spend → HIGH or CRITICAL tier
# ==========================================================================

log_section "Test B: Large Community Pool Spend"

FIXTURE_LARGE="$SCRIPT_DIR/fixtures/community_pool_spend_large.json"
PROPOSAL_LARGE="/tmp/e2e_guard_02_large_spend.json"

sed -e "s|AUTHORITY_PLACEHOLDER|$AUTHORITY|g" \
    -e "s|RECIPIENT_PLACEHOLDER|$VALIDATOR_ADDR|g" \
    "$FIXTURE_LARGE" > "$PROPOSAL_LARGE"

log_step "Submitting large community pool spend proposal"
PID_LARGE=$(submit_gov_proposal "$VALIDATOR_KEY" "$PROPOSAL_LARGE")
log_info "Large spend proposal ID: $PID_LARGE"

vote_proposal "$VALIDATOR_KEY" "$PID_LARGE" "yes"

log_step "Waiting for large spend proposal to pass"
poll_until 360 "$BLOCK_TIME" \
    "query_proposal_status $PID_LARGE" \
    '.proposal.status // .status' \
    'PROPOSAL_STATUS_PASSED' || \
    log_fail "Large spend proposal did not pass"

wait_blocks 3 60

log_step "Querying risk report for large spend"
RISK_LARGE=$(poll_until 60 "$BLOCK_TIME" \
    "query_risk_report $PID_LARGE" \
    '.risk_report.proposal_id // .proposal_id' \
    "$PID_LARGE") || log_fail "No risk report for large spend proposal"

TIER_LARGE=$(jq_get "$RISK_LARGE" '.risk_report.tier // .tier')
SCORE_LARGE=$(jq_get "$RISK_LARGE" '.risk_report.score // .score')
DELAY_LARGE=$(jq_get "$RISK_LARGE" '.risk_report.computed_delay_blocks // .computed_delay_blocks')
REASON_LARGE=$(jq_get "$RISK_LARGE" '.risk_report.reason_codes // .reason_codes')

log_info "Large spend tier: $TIER_LARGE"
log_info "Large spend score: $SCORE_LARGE"
log_info "Large spend delay: $DELAY_LARGE"
log_info "Large spend reasons: $REASON_LARGE"

# Large spend should be HIGH or CRITICAL
assert_in "$TIER_LARGE" "RISK_TIER_HIGH,RISK_TIER_CRITICAL,3,4" \
    "Large community pool spend is HIGH or CRITICAL"

# Reason codes should contain treasury spend code
assert_contains "$REASON_LARGE" "TREASURY_SPEND" \
    "Large spend reason includes TREASURY_SPEND"

# ==========================================================================
# Test C: Verify tier ordering — large >= small
# ==========================================================================

log_section "Test C: Tier Ordering Check"

# Map tiers to numeric for comparison
tier_to_num() {
    case "$1" in
        RISK_TIER_LOW|1)       echo 1 ;;
        RISK_TIER_MED|2)       echo 2 ;;
        RISK_TIER_HIGH|3)      echo 3 ;;
        RISK_TIER_CRITICAL|4)  echo 4 ;;
        *)                     echo 0 ;;
    esac
}

NUM_SMALL=$(tier_to_num "$TIER_SMALL")
NUM_LARGE=$(tier_to_num "$TIER_LARGE")

assert_gte "$NUM_LARGE" "$NUM_SMALL" \
    "Large spend tier ($TIER_LARGE) >= small spend tier ($TIER_SMALL)"

# ---- Cleanup -------------------------------------------------------

rm -f "$PROPOSAL_SMALL" "$PROPOSAL_LARGE"

log_pass "Scenario 02: Community Pool Spend Treasury BPS"
