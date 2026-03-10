#!/bin/bash
# =============================================================================
# Scenario 03: Gate State Transitions
# =============================================================================
# Verifies that x/guard transitions proposals through the full gate sequence:
#   VISIBILITY → SHOCK_ABSORBER → CONDITIONAL_EXECUTION → READY
#
# Uses a staking param-change proposal (CRITICAL via consensus classification)
# with short E2E delays so we can observe all transitions within minutes.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

log_section "Scenario 03: Gate State Transitions"

check_prerequisites

AUTHORITY=$(gov_authority)
log_info "Governance authority: $AUTHORITY"

# ---- Submit a proposal that triggers guard queuing ----

FIXTURE="$SCRIPT_DIR/fixtures/param_change_staking.json"
PROPOSAL_FILE="/tmp/e2e_guard_03_gate_transition.json"

# Try to use current staking params for a no-op change
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
  "metadata": "E2E guard gate transition test",
  "deposit": "10000000000omniphi",
  "title": "E2E Test: Gate Transition via Staking Params",
  "summary": "No-op staking param change to test guard gate state machine transitions."
}
EOFPROP
else
    # Fallback to fixture
    sed "s|AUTHORITY_PLACEHOLDER|$AUTHORITY|g" "$FIXTURE" > "$PROPOSAL_FILE"
fi

log_step "Submitting proposal for gate transition test"
PROPOSAL_ID=$(submit_gov_proposal "$VALIDATOR_KEY" "$PROPOSAL_FILE")
log_info "Proposal ID: $PROPOSAL_ID"

vote_proposal "$VALIDATOR_KEY" "$PROPOSAL_ID" "yes"

log_step "Waiting for proposal to pass"
poll_until 360 "$BLOCK_TIME" \
    "query_proposal_status $PROPOSAL_ID" \
    '.proposal.status // .status' \
    'PROPOSAL_STATUS_PASSED' || \
    log_fail "Proposal did not pass"

# Wait for guard to queue it
wait_blocks 3 60

# ---- Track gate transitions ----

log_section "Tracking Gate Transitions"

# Helper to get gate state and entered-height
get_gate_info() {
    local json
    json=$(query_queued_execution "$PROPOSAL_ID")
    local state
    state=$(jq_get "$json" '.queued_execution.gate_state // .gate_state')
    local entered
    entered=$(jq_get "$json" '.queued_execution.gate_entered_height // .gate_entered_height')
    local earliest
    earliest=$(jq_get "$json" '.queued_execution.earliest_exec_height // .earliest_exec_height')
    echo "$state|$entered|$earliest"
}

# Map gate string/num to sequential index for ordering checks
gate_to_idx() {
    case "$1" in
        EXECUTION_GATE_VISIBILITY|1)              echo 1 ;;
        EXECUTION_GATE_SHOCK_ABSORBER|2)          echo 2 ;;
        EXECUTION_GATE_CONDITIONAL_EXECUTION|3)   echo 3 ;;
        EXECUTION_GATE_READY|4)                   echo 4 ;;
        EXECUTION_GATE_EXECUTED|5)                echo 5 ;;
        EXECUTION_GATE_ABORTED|6)                 echo 6 ;;
        *)                                        echo 0 ;;
    esac
}

# ---- Step 1: Verify initial gate state is VISIBILITY ----

log_step "Step 1: Verifying initial VISIBILITY state"
EXEC_JSON=$(poll_until 60 "$BLOCK_TIME" \
    "query_queued_execution $PROPOSAL_ID" \
    '.queued_execution.proposal_id // .proposal_id' \
    "$PROPOSAL_ID") || log_fail "No queued execution found"

INITIAL_STATE=$(jq_get "$EXEC_JSON" '.queued_execution.gate_state // .gate_state')
INITIAL_HEIGHT=$(jq_get "$EXEC_JSON" '.queued_execution.gate_entered_height // .gate_entered_height')
log_info "Initial state: $INITIAL_STATE (entered at height: $INITIAL_HEIGHT)"

assert_in "$INITIAL_STATE" "EXECUTION_GATE_VISIBILITY,1" "Initial gate is VISIBILITY"

# Track seen states for ordering
SEEN_STATES=("$INITIAL_STATE")
LAST_HEIGHT="$INITIAL_HEIGHT"

# ---- Step 2: Poll through transitions ----

log_step "Step 2: Polling through gate transitions"

# Define the ordered states we expect to see
EXPECTED_SEQUENCE=("EXECUTION_GATE_SHOCK_ABSORBER" "EXECUTION_GATE_CONDITIONAL_EXECUTION" "EXECUTION_GATE_READY")
# Also accept numeric values
EXPECTED_NUMS=("2" "3" "4")

POLL_TIMEOUT=600  # 10 minutes max
POLL_START=$(date +%s)

PREV_STATE="$INITIAL_STATE"
TRANSITIONS_SEEN=0

while true; do
    sleep "$BLOCK_TIME"

    INFO=$(get_gate_info)
    IFS='|' read -r CUR_STATE CUR_HEIGHT CUR_EARLIEST <<< "$INFO"

    # Detect state change
    if [[ "$CUR_STATE" != "$PREV_STATE" ]]; then
        TRANSITIONS_SEEN=$((TRANSITIONS_SEEN + 1))
        log_ok "Transition $TRANSITIONS_SEEN: $PREV_STATE -> $CUR_STATE (height: $CUR_HEIGHT)"

        # Verify ordering: new state index must be > previous
        PREV_IDX=$(gate_to_idx "$PREV_STATE")
        CUR_IDX=$(gate_to_idx "$CUR_STATE")
        assert_gt "$CUR_IDX" "$PREV_IDX" "Gate state progressed forward ($PREV_IDX -> $CUR_IDX)"

        # Verify entered_height advances
        if [[ "$LAST_HEIGHT" != "null" && "$CUR_HEIGHT" != "null" ]]; then
            assert_gte "$CUR_HEIGHT" "$LAST_HEIGHT" \
                "gate_entered_height advances ($LAST_HEIGHT -> $CUR_HEIGHT)"
        fi

        SEEN_STATES+=("$CUR_STATE")
        LAST_HEIGHT="$CUR_HEIGHT"
        PREV_STATE="$CUR_STATE"
    fi

    # Check if we've reached READY (or EXECUTED)
    CUR_IDX=$(gate_to_idx "$CUR_STATE")
    if (( CUR_IDX >= 4 )); then
        log_ok "Reached gate state: $CUR_STATE"
        break
    fi

    # Timeout check
    ELAPSED=$(( $(date +%s) - POLL_START ))
    if (( ELAPSED >= POLL_TIMEOUT )); then
        log_err "Timed out after ${ELAPSED}s — last state: $CUR_STATE"
        log_err "States seen: ${SEEN_STATES[*]}"
        log_fail "Gate transition timeout"
    fi
done

# ---- Step 3: Verify we saw at least VISIBILITY → something → READY ----

log_section "Verifying Transition Ordering"

log_info "States observed: ${SEEN_STATES[*]}"
log_info "Total transitions: $TRANSITIONS_SEEN"

assert_gte "$TRANSITIONS_SEEN" 1 "At least 1 gate transition observed"

# Final state should be READY or later
FINAL_IDX=$(gate_to_idx "$PREV_STATE")
assert_gte "$FINAL_IDX" 4 "Final gate state is READY or later"

# ---- Step 4: Verify earliest_exec_height is respected ----

log_step "Verifying earliest_exec_height constraint"
FINAL_JSON=$(query_queued_execution "$PROPOSAL_ID")
FINAL_EARLIEST=$(jq_get "$FINAL_JSON" '.queued_execution.earliest_exec_height // .earliest_exec_height')
CURRENT_H=$(current_height)

log_info "Earliest exec height: $FINAL_EARLIEST, current height: $CURRENT_H"

# If gate reached READY, current height should be >= earliest
if [[ "$PREV_STATE" == "EXECUTION_GATE_READY" || "$PREV_STATE" == "4" ]]; then
    assert_gte "$CURRENT_H" "$FINAL_EARLIEST" \
        "Did not reach READY before earliest_exec_height"
fi

# ---- Cleanup -------------------------------------------------------

rm -f "$PROPOSAL_FILE"

log_pass "Scenario 03: Gate State Transitions"
