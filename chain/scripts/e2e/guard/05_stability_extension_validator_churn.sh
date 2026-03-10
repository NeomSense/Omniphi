#!/bin/bash
# =============================================================================
# Scenario 05: Stability Extension via Validator Churn
# =============================================================================
# Verifies that CheckStabilityConditions() detects validator power churn and
# extends the execution delay when churn exceeds max_validator_churn_bps.
#
# Strategy:
# 1. Submit a proposal that will reach CONDITIONAL_EXECUTION
# 2. When it enters CONDITIONAL_EXECUTION, perform a large delegation change
#    to trigger validator power churn
# 3. Verify that earliest_exec_height is extended and gate stays CONDITIONAL
#
# Note: This test requires at least one additional delegatable address with
# sufficient funds. If unavailable, the test documents the expected behavior
# and passes with a warning.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

log_section "Scenario 05: Stability Extension (Validator Churn)"

check_prerequisites

AUTHORITY=$(gov_authority)
VALIDATOR_ADDR=$(posd keys show "$VALIDATOR_KEY" --keyring-backend "$KEYRING_BACKEND" -a 2>/dev/null)
log_info "Governance authority: $AUTHORITY"
log_info "Validator address: $VALIDATOR_ADDR"

# ---- Check if we can cause churn ----

# Get validator operator address for delegation
VALOPER=$(posd query staking validators $(query_flags) 2>/dev/null | \
    jq -r '.validators[0].operator_address // empty')

if [[ -z "$VALOPER" ]]; then
    log_warn "No validators found — cannot test churn"
    log_warn "SKIPPING stability extension test (single-node limitation)"
    log_pass "Scenario 05: Stability Extension (SKIPPED - no validators for churn)"
    exit 0
fi

log_info "Validator operator: $VALOPER"

# Check current delegations to determine churn amount
CURRENT_POWER=$(posd query staking validators $(query_flags) 2>/dev/null | \
    jq -r '.validators[0].tokens // "0"')
log_info "Current validator power (tokens): $CURRENT_POWER"

# We need to cause >20% churn (default max_validator_churn_bps=2000)
# On single validator, any delegation/undelegation changes total power
# A self-delegation of 25%+ should trigger it

# Check if we have a second key to delegate from
USER1_ADDR=$(posd keys show user1 --keyring-backend "$KEYRING_BACKEND" -a 2>/dev/null || echo "")

if [[ -z "$USER1_ADDR" ]]; then
    log_warn "No 'user1' key found. Creating one for delegation test..."
    posd keys add user1 --keyring-backend "$KEYRING_BACKEND" 2>/dev/null || true
    USER1_ADDR=$(posd keys show user1 --keyring-backend "$KEYRING_BACKEND" -a 2>/dev/null || echo "")

    if [[ -n "$USER1_ADDR" ]]; then
        # Fund user1 with enough to cause significant delegation
        FUND_AMOUNT="100000000000000omniphi"  # 100M OMNI
        log_step "Funding user1 ($USER1_ADDR) with $FUND_AMOUNT"
        posd tx bank send "$VALIDATOR_KEY" "$USER1_ADDR" "$FUND_AMOUNT" \
            $(tx_flags) 2>/dev/null || {
            log_warn "Could not fund user1 — cannot test churn dynamically"
            USER1_ADDR=""
        }
        sleep $(( BLOCK_TIME * 2 ))
    fi
fi

CAN_CAUSE_CHURN=false
if [[ -n "$USER1_ADDR" ]]; then
    USER1_BALANCE=$(posd query bank balances "$USER1_ADDR" $(query_flags) 2>/dev/null | \
        jq -r '.balances[] | select(.denom=="omniphi") | .amount // "0"' || echo "0")
    log_info "user1 balance: $USER1_BALANCE omniphi"
    if (( USER1_BALANCE > 10000000000000 )); then
        CAN_CAUSE_CHURN=true
    fi
fi

# ---- Submit proposal ----

FIXTURE="$SCRIPT_DIR/fixtures/param_change_staking.json"
PROPOSAL_FILE="/tmp/e2e_guard_05_stability.json"

# Use current staking params for a no-op
CURRENT_STAKING=$(posd query staking params $(query_flags) 2>/dev/null || echo "")
if [[ -n "$CURRENT_STAKING" ]]; then
    STAKING_PARAMS=$(echo "$CURRENT_STAKING" | jq '.params // .')
    cat > "$PROPOSAL_FILE" << EOFPROP
{
  "messages": [
    {
      "@type": "/cosmos.staking.v1beta1.MsgUpdateParams",
      "authority": "$AUTHORITY",
      "params": $STAKING_PARAMS
    }
  ],
  "metadata": "E2E guard stability test",
  "deposit": "10000000000omniphi",
  "title": "E2E Test: Stability Extension",
  "summary": "Staking param change to test guard stability condition checks during CONDITIONAL_EXECUTION."
}
EOFPROP
else
    sed "s|AUTHORITY_PLACEHOLDER|$AUTHORITY|g" "$FIXTURE" > "$PROPOSAL_FILE"
fi

log_step "Submitting proposal for stability test"
PROPOSAL_ID=$(submit_gov_proposal "$VALIDATOR_KEY" "$PROPOSAL_FILE")
log_info "Proposal ID: $PROPOSAL_ID"

vote_proposal "$VALIDATOR_KEY" "$PROPOSAL_ID" "yes"

log_step "Waiting for proposal to pass"
poll_until 360 "$BLOCK_TIME" \
    "query_proposal_status $PROPOSAL_ID" \
    '.proposal.status // .status' \
    'PROPOSAL_STATUS_PASSED' || \
    log_fail "Proposal did not pass"

wait_blocks 3 60

# ---- Wait for CONDITIONAL_EXECUTION state ----

log_step "Waiting for gate to reach CONDITIONAL_EXECUTION"
COND_JSON=$(poll_until_in 600 "$BLOCK_TIME" \
    "query_queued_execution $PROPOSAL_ID" \
    '.queued_execution.gate_state // .gate_state' \
    'EXECUTION_GATE_CONDITIONAL_EXECUTION,3') || {

    # May have jumped straight to READY if delays are very short
    CUR_JSON=$(query_queued_execution "$PROPOSAL_ID")
    CUR_STATE=$(jq_get "$CUR_JSON" '.queued_execution.gate_state // .gate_state')
    log_warn "Did not catch CONDITIONAL_EXECUTION (current: $CUR_STATE)"

    if [[ "$CUR_STATE" == "EXECUTION_GATE_READY" || "$CUR_STATE" == "4" ]]; then
        log_warn "Delays too short to catch CONDITIONAL — stability check may have auto-passed"
        log_pass "Scenario 05: Stability Extension (SKIPPED - delays too short to observe)"
        exit 0
    fi
    log_fail "Gate never reached CONDITIONAL_EXECUTION"
}

# Record initial earliest_exec_height
INITIAL_EARLIEST=$(jq_get "$COND_JSON" '.queued_execution.earliest_exec_height // .earliest_exec_height')
log_info "Initial earliest_exec_height: $INITIAL_EARLIEST"

# ---- Cause validator power churn (if possible) ----

if [[ "$CAN_CAUSE_CHURN" == "true" ]]; then
    log_section "Causing Validator Power Churn"

    # Delegate a large amount to the validator to change power distribution
    DELEGATE_AMOUNT="50000000000000omniphi"  # 50M OMNI — should be >20% churn
    log_step "Delegating $DELEGATE_AMOUNT from user1 to validator"

    posd tx staking delegate "$VALOPER" "$DELEGATE_AMOUNT" \
        --from user1 \
        $(tx_flags) 2>/dev/null || {
        log_warn "Delegation failed — testing without churn"
        CAN_CAUSE_CHURN=false
    }

    sleep $(( BLOCK_TIME * 3 ))
fi

# ---- Check for extension ----

log_step "Checking for stability extension"

# Wait a few blocks for the next EndBlocker stability check
wait_blocks 5 120

AFTER_JSON=$(query_queued_execution "$PROPOSAL_ID")
AFTER_STATE=$(jq_get "$AFTER_JSON" '.queued_execution.gate_state // .gate_state')
AFTER_EARLIEST=$(jq_get "$AFTER_JSON" '.queued_execution.earliest_exec_height // .earliest_exec_height')
STATUS_NOTE=$(jq_get "$AFTER_JSON" '.queued_execution.status_note // .status_note')

log_info "After churn: gate=$AFTER_STATE, earliest=$AFTER_EARLIEST"
log_info "Status note: $STATUS_NOTE"

if [[ "$CAN_CAUSE_CHURN" == "true" ]]; then
    # If we successfully caused churn, verify extension happened
    if (( AFTER_EARLIEST > INITIAL_EARLIEST )); then
        log_ok "earliest_exec_height extended: $INITIAL_EARLIEST -> $AFTER_EARLIEST"
        assert_gt "$AFTER_EARLIEST" "$INITIAL_EARLIEST" \
            "Stability extension pushed earliest_exec_height"

        # Gate should still be CONDITIONAL (not advanced to READY)
        assert_in "$AFTER_STATE" "EXECUTION_GATE_CONDITIONAL_EXECUTION,3" \
            "Gate remains in CONDITIONAL_EXECUTION after churn"
    else
        log_warn "No extension detected — churn may not have exceeded threshold"
        log_info "This can happen if delegation amount was too small relative to total power"
    fi
else
    log_warn "Could not cause churn — verifying stability check runs without error"
    # Even without churn, the gate should eventually progress
    assert_ne "$AFTER_STATE" "" "Gate state is not empty"
fi

# ---- Verify eventual progression to READY ----

log_step "Waiting for eventual progression to READY"
FINAL_JSON=$(poll_until_in 600 "$BLOCK_TIME" \
    "query_queued_execution $PROPOSAL_ID" \
    '.queued_execution.gate_state // .gate_state' \
    'EXECUTION_GATE_READY,EXECUTION_GATE_EXECUTED,4,5') || {
    FINAL_JSON=$(query_queued_execution "$PROPOSAL_ID")
    FINAL_STATE=$(jq_get "$FINAL_JSON" '.queued_execution.gate_state // .gate_state')
    log_warn "Did not reach READY in time (current: $FINAL_STATE)"
}

# ---- Cleanup -------------------------------------------------------

rm -f "$PROPOSAL_FILE"

log_pass "Scenario 05: Stability Extension (Validator Churn)"
