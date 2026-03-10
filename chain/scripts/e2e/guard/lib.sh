#!/bin/bash
# =============================================================================
# Guard E2E Test Library
# =============================================================================
# Shared helpers for all x/guard E2E scenario scripts.
# Source this file at the top of each scenario script.
# =============================================================================

# Strict mode
set -euo pipefail

# ---- Colors ----------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# ---- Configuration ---------------------------------------------------------
export POSD_NODE="${POSD_NODE:-tcp://localhost:26657}"
export POSD_HOME="${POSD_HOME:-$HOME/.pos}"
export POSD_CHAIN_ID="${POSD_CHAIN_ID:-omniphi-testnet-2}"
export KEYRING_BACKEND="${KEYRING_BACKEND:-test}"
export FEES="${FEES:-100000omniphi}"
export GAS="${GAS:-500000}"
export BOND_DENOM="${BOND_DENOM:-omniphi}"
export VALIDATOR_KEY="${VALIDATOR_KEY:-validator}"
export BLOCK_TIME="${BLOCK_TIME:-4}"  # seconds per block

# Test-mode guard params: short delays for E2E (set via gov before running)
# If GUARD_E2E_SHORT_DELAYS=1, the test scripts will submit a guard param
# update proposal first to reduce delays to a few blocks.
export GUARD_E2E_SHORT_DELAYS="${GUARD_E2E_SHORT_DELAYS:-1}"

# ---- Prerequisite checks ---------------------------------------------------

require_cmd() {
    if ! command -v "$1" &>/dev/null; then
        echo -e "${RED}FATAL: required command '$1' not found${NC}" >&2
        exit 1
    fi
}

check_prerequisites() {
    require_cmd posd
    require_cmd jq
    require_cmd bash

    # Verify node is running
    if ! posd status --node "$POSD_NODE" &>/dev/null 2>&1; then
        echo -e "${RED}FATAL: node not reachable at $POSD_NODE${NC}" >&2
        echo -e "${YELLOW}Start localnet first: posd start${NC}" >&2
        exit 1
    fi
    log_info "Node reachable at $POSD_NODE"
}

# ---- Logging ----------------------------------------------------------------

log_info()    { echo -e "${BLUE}[INFO]${NC}  $*"; }
log_ok()      { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_err()     { echo -e "${RED}[ERR]${NC}   $*"; }
log_step()    { echo -e "${CYAN}[STEP]${NC}  $*"; }
log_section() {
    echo ""
    echo -e "${MAGENTA}======================================${NC}"
    echo -e "${MAGENTA}  $*${NC}"
    echo -e "${MAGENTA}======================================${NC}"
    echo ""
}
log_pass() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  PASS: $*${NC}"
    echo -e "${GREEN}========================================${NC}"
}
log_fail() {
    echo ""
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}  FAIL: $*${NC}"
    echo -e "${RED}========================================${NC}"
    exit 1
}

# ---- CLI flag builders ------------------------------------------------------

tx_flags() {
    echo "--node $POSD_NODE --chain-id $POSD_CHAIN_ID --keyring-backend $KEYRING_BACKEND --fees $FEES --gas $GAS --broadcast-mode sync -y -o json"
}

query_flags() {
    echo "--node $POSD_NODE -o json"
}

# ---- JSON helpers -----------------------------------------------------------

# Extract a value from JSON using a jq path.
# Usage: jq_get '{"a":1}' '.a'  => 1
jq_get() {
    local json="$1"
    local path="$2"
    echo "$json" | jq -r "$path"
}

# Assert two values are equal.
assert_eq() {
    local actual="$1"
    local expected="$2"
    local message="${3:-assertion failed}"
    if [[ "$actual" != "$expected" ]]; then
        log_err "ASSERT_EQ FAILED: $message"
        log_err "  expected: $expected"
        log_err "  actual:   $actual"
        exit 1
    fi
    log_ok "ASSERT: $message (got: $actual)"
}

# Assert two values are NOT equal.
assert_ne() {
    local actual="$1"
    local not_expected="$2"
    local message="${3:-assertion failed}"
    if [[ "$actual" == "$not_expected" ]]; then
        log_err "ASSERT_NE FAILED: $message"
        log_err "  should not be: $not_expected"
        log_err "  actual:        $actual"
        exit 1
    fi
    log_ok "ASSERT: $message (got: $actual, not $not_expected)"
}

# Assert value is one of a comma-separated set.
# Usage: assert_in "HIGH" "HIGH,CRITICAL" "tier is elevated"
assert_in() {
    local actual="$1"
    local csv_expected="$2"
    local message="${3:-assertion failed}"
    IFS=',' read -ra vals <<< "$csv_expected"
    for v in "${vals[@]}"; do
        if [[ "$actual" == "$v" ]]; then
            log_ok "ASSERT: $message (got: $actual)"
            return 0
        fi
    done
    log_err "ASSERT_IN FAILED: $message"
    log_err "  actual:   $actual"
    log_err "  expected one of: $csv_expected"
    exit 1
}

# Assert numeric value >= threshold.
assert_gte() {
    local actual="$1"
    local threshold="$2"
    local message="${3:-assertion failed}"
    if (( actual < threshold )); then
        log_err "ASSERT_GTE FAILED: $message"
        log_err "  actual:    $actual"
        log_err "  threshold: $threshold"
        exit 1
    fi
    log_ok "ASSERT: $message ($actual >= $threshold)"
}

# Assert numeric value > threshold.
assert_gt() {
    local actual="$1"
    local threshold="$2"
    local message="${3:-assertion failed}"
    if (( actual <= threshold )); then
        log_err "ASSERT_GT FAILED: $message"
        log_err "  actual:    $actual"
        log_err "  threshold: $threshold"
        exit 1
    fi
    log_ok "ASSERT: $message ($actual > $threshold)"
}

# Assert a string contains a substring.
assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="${3:-assertion failed}"
    if [[ "$haystack" != *"$needle"* ]]; then
        log_err "ASSERT_CONTAINS FAILED: $message"
        log_err "  looking for: $needle"
        log_err "  in:          $haystack"
        exit 1
    fi
    log_ok "ASSERT: $message (contains '$needle')"
}

# ---- Chain helpers ----------------------------------------------------------

# Get the current block height.
current_height() {
    posd status --node "$POSD_NODE" 2>/dev/null | jq -r '.sync_info.latest_block_height // .SyncInfo.latest_block_height' 2>/dev/null || echo "0"
}

# Wait until chain reaches target height. Times out after $2 seconds.
# Usage: wait_for_height 150 120
wait_for_height() {
    local target="$1"
    local timeout="${2:-300}"
    local start=$(date +%s)
    log_info "Waiting for height $target (timeout: ${timeout}s)..."
    while true; do
        local h
        h=$(current_height)
        if (( h >= target )); then
            log_ok "Reached height $h (target: $target)"
            return 0
        fi
        local elapsed=$(( $(date +%s) - start ))
        if (( elapsed >= timeout )); then
            log_err "Timed out waiting for height $target (current: $h, elapsed: ${elapsed}s)"
            return 1
        fi
        sleep "$BLOCK_TIME"
    done
}

# Wait N blocks from now.
wait_blocks() {
    local n="$1"
    local timeout="${2:-300}"
    local h
    h=$(current_height)
    local target=$(( h + n ))
    wait_for_height "$target" "$timeout"
}

# ---- Polling helpers --------------------------------------------------------

# Poll until a command's JSON output has a field matching expected value.
# Usage: poll_until 120 4 "posd query guard risk-report 1 $(query_flags)" '.risk_report.tier' 'RISK_TIER_CRITICAL'
poll_until() {
    local timeout="$1"
    local sleep_sec="$2"
    local cmd="$3"
    local jq_path="$4"
    local expected="$5"
    local start=$(date +%s)
    log_info "Polling: $jq_path == $expected (timeout: ${timeout}s)"
    while true; do
        local result
        result=$(eval "$cmd" 2>/dev/null || echo "{}")
        local val
        val=$(echo "$result" | jq -r "$jq_path" 2>/dev/null || echo "null")
        if [[ "$val" == "$expected" ]]; then
            log_ok "Poll matched: $jq_path == $expected"
            echo "$result"  # return full JSON for callers
            return 0
        fi
        local elapsed=$(( $(date +%s) - start ))
        if (( elapsed >= timeout )); then
            log_err "Poll timed out after ${elapsed}s: $jq_path != $expected (last: $val)"
            return 1
        fi
        sleep "$sleep_sec"
    done
}

# Poll until field value is one of CSV options.
poll_until_in() {
    local timeout="$1"
    local sleep_sec="$2"
    local cmd="$3"
    local jq_path="$4"
    local csv_expected="$5"
    local start=$(date +%s)
    log_info "Polling: $jq_path in [$csv_expected] (timeout: ${timeout}s)"
    while true; do
        local result
        result=$(eval "$cmd" 2>/dev/null || echo "{}")
        local val
        val=$(echo "$result" | jq -r "$jq_path" 2>/dev/null || echo "null")
        IFS=',' read -ra vals <<< "$csv_expected"
        for v in "${vals[@]}"; do
            if [[ "$val" == "$v" ]]; then
                log_ok "Poll matched: $jq_path == $val"
                echo "$result"
                return 0
            fi
        done
        local elapsed=$(( $(date +%s) - start ))
        if (( elapsed >= timeout )); then
            log_err "Poll timed out after ${elapsed}s: $jq_path not in [$csv_expected] (last: $val)"
            return 1
        fi
        sleep "$sleep_sec"
    done
}

# Poll until field value changes from a known previous value.
poll_until_changes() {
    local timeout="$1"
    local sleep_sec="$2"
    local cmd="$3"
    local jq_path="$4"
    local previous="$5"
    local start=$(date +%s)
    log_info "Polling: $jq_path != $previous (timeout: ${timeout}s)"
    while true; do
        local result
        result=$(eval "$cmd" 2>/dev/null || echo "{}")
        local val
        val=$(echo "$result" | jq -r "$jq_path" 2>/dev/null || echo "null")
        if [[ "$val" != "$previous" && "$val" != "null" ]]; then
            log_ok "Poll detected change: $jq_path: $previous -> $val"
            echo "$result"
            return 0
        fi
        local elapsed=$(( $(date +%s) - start ))
        if (( elapsed >= timeout )); then
            log_err "Poll timed out: $jq_path still == $previous after ${elapsed}s"
            return 1
        fi
        sleep "$sleep_sec"
    done
}

# ---- Governance helpers -----------------------------------------------------

# Get the governance module account address.
gov_authority() {
    posd query auth module-account gov --node "$POSD_NODE" 2>&1 | grep "address:" | awk '{print $2}'
}

# Submit a governance proposal from a JSON file.
# Returns the proposal ID on stdout.
submit_gov_proposal() {
    local from_key="$1"
    local proposal_file="$2"
    log_step "Submitting governance proposal from $proposal_file (key: $from_key)"

    local tx_result
    tx_result=$(posd tx gov submit-proposal "$proposal_file" \
        --from "$from_key" \
        $(tx_flags) 2>&1)

    local tx_code
    tx_code=$(echo "$tx_result" | jq -r '.code // 0' 2>/dev/null || echo "unknown")
    if [[ "$tx_code" != "0" ]]; then
        log_err "Proposal submission failed (code=$tx_code)"
        echo "$tx_result" | jq '.' 2>/dev/null || echo "$tx_result"
        return 1
    fi

    local tx_hash
    tx_hash=$(echo "$tx_result" | jq -r '.txhash')
    log_info "Tx hash: $tx_hash"

    # Wait for tx inclusion
    sleep $(( BLOCK_TIME * 2 ))

    # Extract proposal ID from tx events
    local tx_detail
    tx_detail=$(posd query tx "$tx_hash" $(query_flags) 2>/dev/null || echo "{}")

    local proposal_id
    proposal_id=$(echo "$tx_detail" | jq -r '
        .events[]? |
        select(.type == "submit_proposal") |
        .attributes[]? |
        select(.key == "proposal_id") |
        .value' 2>/dev/null | head -1)

    # Fallback: check logs for proposal_id
    if [[ -z "$proposal_id" || "$proposal_id" == "null" ]]; then
        proposal_id=$(echo "$tx_detail" | jq -r '
            .logs[0].events[]? |
            select(.type == "submit_proposal") |
            .attributes[]? |
            select(.key == "proposal_id") |
            .value' 2>/dev/null | head -1)
    fi

    # Fallback: query latest proposals
    if [[ -z "$proposal_id" || "$proposal_id" == "null" ]]; then
        log_warn "Could not extract proposal_id from tx events, querying latest proposals"
        proposal_id=$(posd query gov proposals --status voting_period $(query_flags) 2>/dev/null | \
            jq -r '.proposals[-1].id // empty' 2>/dev/null)
    fi

    if [[ -z "$proposal_id" || "$proposal_id" == "null" ]]; then
        log_err "Failed to determine proposal ID"
        return 1
    fi

    log_ok "Proposal submitted: ID=$proposal_id"
    echo "$proposal_id"
}

# Vote on a proposal.
vote_proposal() {
    local from_key="$1"
    local proposal_id="$2"
    local vote_option="$3"  # yes, no, no_with_veto, abstain
    log_step "Voting $vote_option on proposal $proposal_id (key: $from_key)"

    local tx_result
    tx_result=$(posd tx gov vote "$proposal_id" "$vote_option" \
        --from "$from_key" \
        $(tx_flags) 2>&1)

    local tx_code
    tx_code=$(echo "$tx_result" | jq -r '.code // 0' 2>/dev/null || echo "unknown")
    if [[ "$tx_code" != "0" ]]; then
        log_err "Vote failed (code=$tx_code)"
        echo "$tx_result" | jq '.' 2>/dev/null || echo "$tx_result"
        return 1
    fi

    log_ok "Vote $vote_option cast on proposal $proposal_id"
    sleep "$BLOCK_TIME"
}

# ---- Guard helpers ----------------------------------------------------------

# Send MsgConfirmExecution for a CRITICAL proposal.
confirm_execution() {
    local from_key="$1"
    local proposal_id="$2"
    local justification="${3:-E2E test confirmation}"
    log_step "Confirming execution for proposal $proposal_id"

    local authority
    authority=$(gov_authority)

    local tx_result
    tx_result=$(posd tx guard confirm-execution \
        --proposal-id "$proposal_id" \
        --justification "$justification" \
        --authority "$authority" \
        --from "$from_key" \
        $(tx_flags) 2>&1)

    local tx_code
    tx_code=$(echo "$tx_result" | jq -r '.code // 0' 2>/dev/null || echo "unknown")
    if [[ "$tx_code" != "0" ]]; then
        # autocli may use positional args — try alternate syntax
        log_warn "Trying positional confirm-execution syntax..."
        tx_result=$(posd tx guard confirm-execution "$authority" "$proposal_id" "$justification" \
            --from "$from_key" \
            $(tx_flags) 2>&1)
        tx_code=$(echo "$tx_result" | jq -r '.code // 0' 2>/dev/null || echo "unknown")
        if [[ "$tx_code" != "0" ]]; then
            log_err "Confirm execution failed (code=$tx_code)"
            echo "$tx_result" | jq '.' 2>/dev/null || echo "$tx_result"
            return 1
        fi
    fi

    log_ok "Execution confirmed for proposal $proposal_id"
    sleep "$BLOCK_TIME"
}

# Query a risk report for a proposal.
query_risk_report() {
    local proposal_id="$1"
    posd query guard risk-report "$proposal_id" $(query_flags) 2>/dev/null || echo "{}"
}

# Query queued execution state for a proposal.
query_queued_execution() {
    local proposal_id="$1"
    posd query guard queued-execution "$proposal_id" $(query_flags) 2>/dev/null || echo "{}"
}

# Query governance proposal status.
query_proposal_status() {
    local proposal_id="$1"
    posd query gov proposal "$proposal_id" $(query_flags) 2>/dev/null || echo "{}"
}

# Query guard module params.
query_guard_params() {
    posd query guard params $(query_flags) 2>/dev/null || echo "{}"
}

# ---- Guard param override for E2E ------------------------------------------
# Sets guard delays to small values (a few blocks) so tests complete quickly.
# This submits a gov proposal to update guard params and votes YES.
# Only runs if GUARD_E2E_SHORT_DELAYS=1.

setup_short_guard_delays() {
    if [[ "${GUARD_E2E_SHORT_DELAYS}" != "1" ]]; then
        log_info "GUARD_E2E_SHORT_DELAYS != 1, using default guard params"
        return 0
    fi

    log_section "Setting short guard delays for E2E testing"

    local authority
    authority=$(gov_authority)

    local fixture_dir
    fixture_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fixtures"

    # Create temporary param-update proposal
    local proposal_file="/tmp/guard_e2e_param_update.json"
    cat > "$proposal_file" << EOFPROP
{
  "messages": [
    {
      "@type": "/pos.guard.v1.MsgUpdateParams",
      "authority": "$authority",
      "params": {
        "delay_low_blocks": "5",
        "delay_med_blocks": "8",
        "delay_high_blocks": "12",
        "delay_critical_blocks": "20",
        "visibility_window_blocks": "3",
        "shock_absorber_window_blocks": "4",
        "threshold_default_bps": "5000",
        "threshold_high_bps": "6667",
        "threshold_critical_bps": "7500",
        "treasury_throttle_enabled": true,
        "treasury_max_outflow_bps_per_day": "1000",
        "enable_stability_checks": true,
        "max_validator_churn_bps": "2000",
        "advisory_ai_enabled": false,
        "binding_ai_enabled": false,
        "ai_shadow_mode": false,
        "critical_requires_second_confirm": true,
        "critical_second_confirm_window_blocks": "15",
        "extension_high_blocks": "5",
        "extension_critical_blocks": "8"
      }
    }
  ],
  "metadata": "Guard E2E short delays",
  "deposit": "10000000000omniphi",
  "title": "Guard E2E: Set Short Delays",
  "summary": "Reduce guard delay blocks so E2E tests complete within minutes."
}
EOFPROP

    local pid
    pid=$(submit_gov_proposal "$VALIDATOR_KEY" "$proposal_file") || {
        log_warn "Could not submit guard param update — tests will use default (long) delays"
        return 0
    }

    vote_proposal "$VALIDATOR_KEY" "$pid" "yes"

    # Wait for voting period to end (testnet = 5 min, but we poll)
    log_info "Waiting for guard param proposal $pid to pass..."
    poll_until 360 "$BLOCK_TIME" \
        "query_proposal_status $pid" \
        '.proposal.status // .status' \
        'PROPOSAL_STATUS_PASSED' || {
        log_warn "Guard param proposal did not pass in time — using default params"
        return 0
    }

    # Wait a few blocks for EndBlocker to process
    wait_blocks 3 60

    log_ok "Guard delays set to short values for E2E"
}

# ---- Fixture path helper ----------------------------------------------------

fixture_dir() {
    echo "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fixtures"
}

script_dir() {
    echo "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
}
