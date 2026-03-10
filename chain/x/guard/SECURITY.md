# x/guard Security Model

## Overview

The `x/guard` module is a governance safeguard layer that intercepts all passed governance proposals and enforces a multi-phase execution pipeline. No proposal executes immediately; every proposal must traverse the gate state machine before its messages are dispatched.

## Threat Model

### T1: Governance Execution Bypass

**Attack:** An attacker somehow routes proposal execution around x/guard, executing proposal messages directly without passing through the gate state machine.

**Mitigations:**
1. **Pre-dispatch assertion:** `ExecuteProposal()` checks `exec.GateState == EXECUTION_GATE_READY` before dispatching any messages. Proposals in VISIBILITY, SHOCK_ABSORBER, CONDITIONAL_EXECUTION, EXECUTED, or ABORTED states are rejected.
2. **Governance status check:** `ExecuteProposal()` verifies the proposal still has `StatusPassed` in the gov module before execution.
3. **Execution markers:** After successful execution, an execution marker is written to a dedicated KV prefix (`0x0B`). The bypass invariant verifies that every EXECUTED queue entry has a matching marker.
4. **Registered invariant:** `ExecutionBypassInvariant` runs via `RegisterInvariants` and can be triggered by `invariant-check` to detect any proposal that reached EXECUTED state without going through the guard pipeline.

### T2: Double Execution

**Attack:** An attacker replays or re-triggers execution of an already-executed proposal.

**Mitigations:**
1. **Execution marker check:** `ExecuteProposal()` checks `HasExecutionMarker(proposalID)` before dispatching. If a marker exists, the call fails with `ErrDoubleExecution`.
2. **Terminal state check:** `ProcessQueue` skips proposals in terminal states (EXECUTED, ABORTED).
3. **Gate state machine:** The state machine only transitions READY -> EXECUTED once; there is no path back to READY from EXECUTED.

### T3: EndBlocker DOS

**Attack:** An attacker floods governance with proposals to make the EndBlocker consume excessive gas, potentially stalling the chain.

**Mitigations:**
1. **Bounded proposal polling:** `PollGovernanceProposals` processes at most `MaxProposalsPerBlock` proposals per block (default: 10, governance-configurable, max: 1000).
2. **Bounded queue scanning:** `ProcessQueue` scans at most `MaxQueueScanDepth` entries per block (default: 100, governance-configurable, max: 10000).
3. **Resume cursor:** The poller tracks `LastProcessedProposalID` so unprocessed proposals carry over to the next block rather than being dropped.
4. **No panics:** All EndBlocker errors are logged and swallowed; the chain never halts due to guard processing errors.

### T4: Gate State Machine Corruption

**Attack:** An attacker manipulates state to skip gate phases or corrupt transition logic.

**Mitigations:**
1. **Deterministic transitions:** Gate transitions are driven by block height and elapsed blocks in current gate, both derived from deterministic on-chain state.
2. **Queue consistency invariant:** Detects UNSPECIFIED gate states, EXECUTED-without-confirmation violations, and rejected/failed proposals still active in the queue.
3. **Monotonic gate progression:** The state machine only moves forward: VISIBILITY -> SHOCK_ABSORBER -> CONDITIONAL_EXECUTION -> READY -> EXECUTED. The only "reset" path is confirmation expiry (back to VISIBILITY) or abort (to ABORTED).

### T5: Validator Power Manipulation During Stability Checks

**Attack:** An attacker manipulates validator power distribution to bypass stability checks.

**Mitigations:**
1. **Snapshot-at-entry:** Validator power is snapshotted when a proposal enters CONDITIONAL_EXECUTION. Churn is measured against this snapshot, not a moving target.
2. **Re-snapshot on extension:** If stability checks fail and the delay is extended, a new snapshot is taken for the next check.
3. **Configurable threshold:** `MaxValidatorChurnBps` is governance-configurable (default: 2000 bps = 20%).

## Security Parameters

| Parameter | Default | Description |
|---|---|---|
| `max_proposals_per_block` | 10 | Max proposals polled from gov per EndBlocker |
| `max_queue_scan_depth` | 100 | Max queue entries scanned per EndBlocker |
| `max_validator_churn_bps` | 2000 | Max validator power change before extending delay |
| `critical_requires_second_confirm` | true | CRITICAL proposals need explicit 2nd confirmation |
| `critical_second_confirm_window_blocks` | 51840 | Blocks to provide 2nd confirmation before reset |

## Invariants

Run invariants with:
```bash
posd invariant-check guard execution-bypass
posd invariant-check guard queue-consistency
```

### execution-bypass
Verifies that every `QueuedExecution` with `GateState == EXECUTED` has a corresponding execution marker in the `0x0B` KV prefix. A broken invariant means a proposal was executed outside the guard pipeline.

### queue-consistency
Verifies:
- No `QueuedExecution` has `GateState == UNSPECIFIED`
- EXECUTED proposals that required 2nd confirmation have `SecondConfirmReceived == true`
- Proposals rejected/failed in governance are not still active (non-terminal) in the guard queue

## Events

All guard events include structured attributes for monitoring:

| Event | Key Attributes |
|---|---|
| `guard_proposal_queued` | proposal_id, tier, score, delay_blocks, threshold_bps, queued_height, earliest_exec_height, requires_confirmation, reason_codes |
| `guard_gate_transition` | proposal_id, from, to, tier, current_height, earliest_exec_height |
| `guard_proposal_executed` | proposal_id, tier, current_height, queued_height |
| `guard_proposal_aborted` | proposal_id, reason, error (if applicable) |
| `guard_execution_extended` | proposal_id, reason, churn_bps (if applicable), extension_blocks (if applicable) |
| `guard_execution_confirm_required` | proposal_id, blocks_remaining |

## Store Key Prefixes

| Prefix | Description |
|---|---|
| `0x01` | Module parameters |
| `0x02` | Risk reports by proposal ID |
| `0x03` | Queued executions by proposal ID |
| `0x04` | Queue height index (height + proposal ID) |
| `0x05` | Last processed proposal ID cursor |
| `0x06` | AI model metadata |
| `0x07` | Logistic model weights |
| `0x08` | AI evaluations by proposal ID |
| `0x09` | Advisory links by proposal ID |
| `0x0A` | Validator power snapshots by proposal ID |
| `0x0B` | Execution markers by proposal ID (bypass detection) |
