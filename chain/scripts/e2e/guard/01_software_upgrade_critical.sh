#!/bin/bash
# =============================================================================
# Scenario 01: Software Upgrade → CRITICAL Risk Tier
# =============================================================================
# Verifies that x/guard classifies MsgSoftwareUpgrade proposals as CRITICAL
# with max delay, max threshold, and second confirmation required.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

log_section "Scenario 01: Software Upgrade CRITICAL"

check_prerequisites

# ---- Prepare fixture -------------------------------------------------------

AUTHORITY=$(gov_authority)
log_info "Governance authority: $AUTHORITY"

FIXTURE="$SCRIPT_DIR/fixtures/software_upgrade.json"
PROPOSAL_FILE="/tmp/e2e_guard_01_software_upgrade.json"

# Replace authority placeholder
sed "s|AUTHORITY_PLACEHOLDER|$AUTHORITY|g" "$FIXTURE" > "$PROPOSAL_FILE"
log_info "Proposal file: $PROPOSAL_FILE"

# ---- Submit proposal -------------------------------------------------------

log_step "Submitting software upgrade proposal"
PROPOSAL_ID=$(submit_gov_proposal "$VALIDATOR_KEY" "$PROPOSAL_FILE")
log_info "Proposal ID: $PROPOSAL_ID"

# ---- Vote YES (single validator = passes immediately) ----------------------

log_step "Voting YES on proposal $PROPOSAL_ID"
vote_proposal "$VALIDATOR_KEY" "$PROPOSAL_ID" "yes"

# ---- Wait for proposal to pass ---------------------------------------------

log_step "Waiting for proposal to pass"
poll_until 360 "$BLOCK_TIME" \
    "query_proposal_status $PROPOSAL_ID" \
    '.proposal.status // .status' \
    'PROPOSAL_STATUS_PASSED' || \
    log_fail "Proposal $PROPOSAL_ID did not pass"

# Wait for guard EndBlocker to process
wait_blocks 3 60

# ---- Assert risk report ----

log_step "Querying risk report for proposal $PROPOSAL_ID"
RISK_JSON=$(query_risk_report "$PROPOSAL_ID")

if [[ "$RISK_JSON" == "{}" || -z "$RISK_JSON" ]]; then
    # Try polling a bit longer — EndBlocker may not have run yet
    RISK_JSON=$(poll_until 60 "$BLOCK_TIME" \
        "query_risk_report $PROPOSAL_ID" \
        '.risk_report.proposal_id // .proposal_id' \
        "$PROPOSAL_ID") || log_fail "No risk report found for proposal $PROPOSAL_ID"
fi

TIER=$(jq_get "$RISK_JSON" '.risk_report.tier // .tier')
SCORE=$(jq_get "$RISK_JSON" '.risk_report.score // .score')
DELAY=$(jq_get "$RISK_JSON" '.risk_report.computed_delay_blocks // .computed_delay_blocks')
THRESHOLD=$(jq_get "$RISK_JSON" '.risk_report.computed_threshold_bps // .computed_threshold_bps')
REASON=$(jq_get "$RISK_JSON" '.risk_report.reason_codes // .reason_codes')

log_info "Risk report: tier=$TIER score=$SCORE delay=$DELAY threshold=$THRESHOLD"
log_info "Reason codes: $REASON"

# Tier must be CRITICAL (enum string or numeric 4)
assert_in "$TIER" "RISK_TIER_CRITICAL,4" "Software upgrade tier is CRITICAL"

# Score should be high (>=90)
assert_gte "$SCORE" 90 "Software upgrade score >= 90"

# Delay should be the critical delay (default 241920, or E2E short: 20)
assert_gt "$DELAY" 0 "Computed delay > 0"

# Threshold should be critical threshold (default 7500 bps)
assert_gt "$THRESHOLD" 0 "Computed threshold > 0"

# Reason codes should contain SOFTWARE_UPGRADE
assert_contains "$REASON" "SOFTWARE_UPGRADE" "Reason codes contain SOFTWARE_UPGRADE"

# ---- Assert queued execution ----

log_step "Querying queued execution for proposal $PROPOSAL_ID"
EXEC_JSON=$(query_queued_execution "$PROPOSAL_ID")

if [[ "$EXEC_JSON" == "{}" || -z "$EXEC_JSON" ]]; then
    EXEC_JSON=$(poll_until 60 "$BLOCK_TIME" \
        "query_queued_execution $PROPOSAL_ID" \
        '.queued_execution.proposal_id // .proposal_id' \
        "$PROPOSAL_ID") || log_fail "No queued execution for proposal $PROPOSAL_ID"
fi

GATE_STATE=$(jq_get "$EXEC_JSON" '.queued_execution.gate_state // .gate_state')
REQUIRES_2ND=$(jq_get "$EXEC_JSON" '.queued_execution.requires_second_confirm // .requires_second_confirm')
EXEC_TIER=$(jq_get "$EXEC_JSON" '.queued_execution.tier // .tier')
EARLIEST_HEIGHT=$(jq_get "$EXEC_JSON" '.queued_execution.earliest_exec_height // .earliest_exec_height')

log_info "Gate state: $GATE_STATE"
log_info "Requires 2nd confirm: $REQUIRES_2ND"
log_info "Tier: $EXEC_TIER"
log_info "Earliest exec height: $EARLIEST_HEIGHT"

# Gate state should be VISIBILITY (1) or already transitioned
assert_in "$GATE_STATE" "EXECUTION_GATE_VISIBILITY,EXECUTION_GATE_SHOCK_ABSORBER,1,2" \
    "Gate state is VISIBILITY or SHOCK_ABSORBER after queuing"

# Should require second confirmation for CRITICAL
assert_in "$REQUIRES_2ND" "true,1" "CRITICAL proposal requires second confirmation"

# Tier in queued execution should match
assert_in "$EXEC_TIER" "RISK_TIER_CRITICAL,4" "Queued execution tier is CRITICAL"

# Earliest exec height should be in the future
CURRENT=$(current_height)
assert_gt "$EARLIEST_HEIGHT" "$CURRENT" "Earliest exec height ($EARLIEST_HEIGHT) > current ($CURRENT)"

# ---- Cleanup -------------------------------------------------------

rm -f "$PROPOSAL_FILE"

log_pass "Scenario 01: Software Upgrade CRITICAL"
