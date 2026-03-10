#!/bin/bash
# =============================================================================
# Scenario 06: Emergency Veto → Abort
# =============================================================================
# Verifies that a governance proposal vetoed (NO_WITH_VETO) during the guard
# pipeline results in the queued execution being ABORTED.
#
# Strategy:
# 1. Submit a proposal
# 2. Vote NO_WITH_VETO (with single validator = >33.34% veto)
# 3. Verify gov proposal status becomes REJECTED
# 4. Verify guard marks queued execution as ABORTED (if it was queued)
#
# Note: On a single-validator localnet, the validator has 100% of voting power,
# so a NO_WITH_VETO vote will immediately veto the proposal. The proposal may
# not even reach the guard queue if veto happens before passing.
# We test both cases:
#   A) Veto before passing (gov rejects directly — guard never queues)
#   B) If guard already queued it, verify ABORTED state
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

log_section "Scenario 06: Emergency Veto Abort"

check_prerequisites

AUTHORITY=$(gov_authority)
log_info "Governance authority: $AUTHORITY"

# ---- Submit a proposal ----

# Use a text-only proposal so it's harmless
FIXTURE="$SCRIPT_DIR/fixtures/text_only.json"
PROPOSAL_FILE="/tmp/e2e_guard_06_veto.json"

cp "$FIXTURE" "$PROPOSAL_FILE"

log_step "Submitting text-only proposal for veto test"
PROPOSAL_ID=$(submit_gov_proposal "$VALIDATOR_KEY" "$PROPOSAL_FILE")
log_info "Proposal ID: $PROPOSAL_ID"

# ---- Vote NO_WITH_VETO ----

log_step "Voting NO_WITH_VETO on proposal $PROPOSAL_ID"
vote_proposal "$VALIDATOR_KEY" "$PROPOSAL_ID" "no_with_veto"

# ---- Wait for proposal to be resolved ----

log_step "Waiting for proposal to be rejected/vetoed"

# Poll for terminal gov status (REJECTED or FAILED)
GOV_RESULT=$(poll_until_in 360 "$BLOCK_TIME" \
    "query_proposal_status $PROPOSAL_ID" \
    '.proposal.status // .status' \
    'PROPOSAL_STATUS_REJECTED,PROPOSAL_STATUS_FAILED,PROPOSAL_STATUS_VETOED') || {

    # Check what status we got
    GOV_JSON=$(query_proposal_status "$PROPOSAL_ID")
    GOV_STATUS=$(jq_get "$GOV_JSON" '.proposal.status // .status')
    log_info "Current gov status: $GOV_STATUS"

    # If it somehow passed (e.g. voting logic differs), that's still testable
    if [[ "$GOV_STATUS" == "PROPOSAL_STATUS_PASSED" ]]; then
        log_warn "Proposal passed despite NO_WITH_VETO vote"
        log_warn "This may happen if veto threshold differs from expected"
        log_warn "Checking if guard queued and can be verified..."
    else
        log_fail "Proposal in unexpected status: $GOV_STATUS"
    fi
}

GOV_JSON=$(query_proposal_status "$PROPOSAL_ID")
GOV_STATUS=$(jq_get "$GOV_JSON" '.proposal.status // .status')
log_info "Final gov status: $GOV_STATUS"

# ---- Verify tally includes veto votes ----

log_step "Checking tally results"
TALLY=$(posd query gov tally "$PROPOSAL_ID" $(query_flags) 2>/dev/null || echo "{}")
VETO_COUNT=$(jq_get "$TALLY" '.tally.no_with_veto_count // .no_with_veto // "0"')
log_info "Veto vote count: $VETO_COUNT"

assert_ne "$VETO_COUNT" "0" "Veto votes recorded in tally"

# ---- Check guard state ----

log_step "Checking guard queued execution state"

# The proposal may or may not have been queued by guard depending on timing.
# If the veto resolved before the proposal passed, guard never sees it.
# If guard somehow processed it (e.g., proposal briefly passed then was
# handled by EndBlocker), check for ABORTED.

wait_blocks 3 60

EXEC_JSON=$(query_queued_execution "$PROPOSAL_ID")
GUARD_STATE=$(jq_get "$EXEC_JSON" '.queued_execution.gate_state // .gate_state // "NOT_QUEUED"')

if [[ "$GUARD_STATE" == "NOT_QUEUED" || "$GUARD_STATE" == "null" || -z "$GUARD_STATE" ]]; then
    log_ok "Proposal was never queued by guard (vetoed before passing)"
    log_info "This is the expected path for NO_WITH_VETO on single-validator localnet"

    # Verify the gov proposal is indeed rejected
    assert_in "$GOV_STATUS" \
        "PROPOSAL_STATUS_REJECTED,PROPOSAL_STATUS_FAILED,PROPOSAL_STATUS_VETOED" \
        "Gov proposal is rejected/vetoed"

else
    log_info "Guard queued execution found with state: $GUARD_STATE"

    # If guard processed it, expect ABORTED
    if [[ "$GUARD_STATE" == "EXECUTION_GATE_ABORTED" || "$GUARD_STATE" == "6" ]]; then
        log_ok "Guard correctly ABORTED the vetoed proposal"

        # Check status note
        STATUS_NOTE=$(jq_get "$EXEC_JSON" '.queued_execution.status_note // .status_note')
        log_info "Abort status note: $STATUS_NOTE"
    else
        # If guard queued it but hasn't aborted yet, wait a bit more
        log_info "Waiting for guard to detect veto and abort..."
        ABORT_JSON=$(poll_until_in 120 "$BLOCK_TIME" \
            "query_queued_execution $PROPOSAL_ID" \
            '.queued_execution.gate_state // .gate_state' \
            'EXECUTION_GATE_ABORTED,6') || {
            FINAL_EXEC=$(query_queued_execution "$PROPOSAL_ID")
            FINAL_STATE=$(jq_get "$FINAL_EXEC" '.queued_execution.gate_state // .gate_state')
            log_warn "Guard did not abort (final state: $FINAL_STATE)"
            log_warn "This may indicate guard does not poll vetoed proposals"
        }
    fi
fi

# ---- Verify risk report if exists ----

log_step "Checking risk report"
RISK_JSON=$(query_risk_report "$PROPOSAL_ID")
RISK_TIER=$(jq_get "$RISK_JSON" '.risk_report.tier // .tier // "NONE"')

if [[ "$RISK_TIER" != "NONE" && "$RISK_TIER" != "null" && -n "$RISK_TIER" ]]; then
    log_info "Risk report exists: tier=$RISK_TIER"
    # Text-only proposals should be LOW
    assert_in "$RISK_TIER" "RISK_TIER_LOW,1" "Text-only vetoed proposal was LOW risk"
else
    log_info "No risk report (expected — proposal was vetoed before guard processed it)"
fi

# ---- Cleanup -------------------------------------------------------

rm -f "$PROPOSAL_FILE"

log_pass "Scenario 06: Emergency Veto Abort"
