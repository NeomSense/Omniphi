# Phase 6: HotStuff Consensus Integration

## Overview

PoSeq uses a pipelined HotStuff BFT consensus protocol for batch finalization. The implementation is in `poseq/src/hotstuff/` with five modules:

| Module | Purpose |
|--------|---------|
| `types.rs` | Core data types: Block, QC, Vote, NewView, SafetyRule |
| `pacemaker.rs` | View timer, exponential backoff, NewView quorum |
| `engine.rs` | Main state machine: on_block, on_vote, on_qc, on_timeout |
| `persistence.rs` | Consensus state snapshot for crash-safe recovery |
| `verification.rs` | Committee membership and message authentication checks |

## Consensus Flow

```
View v:
  Leader proposes HotStuffBlock(view=v, parent_id, batch_root, justify_qc=HighQC)

  Validators check:
    1. Block extends safe ancestor (SafetyRule)
    2. Leader is expected proposer (verification)
    3. Fairness commitment is valid
    4. DA artifacts available

  If valid → vote PREPARE

  Leader collects 2f+1 PREPARE votes → forms PREPARE-QC
  Broadcasts PREPARE-QC → validators vote PRE-COMMIT

  Leader collects 2f+1 PRE-COMMIT votes → forms PRE-COMMIT-QC
  Validators LOCK on PRE-COMMIT-QC → vote COMMIT

  Leader collects 2f+1 COMMIT votes → forms COMMIT-QC
  All nodes DECIDE: block is FINALIZED
```

## Safety Rules

A node votes on a proposal only if:
1. The block extends the locked QC directly (parent chain), OR
2. The block's justify_qc has a higher view than the locked QC

This prevents conflicting commits: once a node locks on a PRE-COMMIT-QC, it will not vote for a conflicting block unless a higher-view QC proves the network has moved past the lock.

## Pacemaker / View Change

When a view times out (leader offline/slow):
1. Node emits `TimeoutVote` (currently `NewViewMessage` with HighQC)
2. Timeout increases exponentially (base × 2^n, capped at 32× base)
3. When 2f+1 NewView messages arrive, next leader advances to the new view
4. New leader proposes extending the highest QC from the NewView set

## Authenticated Transport

All consensus messages must be verified before processing:

| Check | Module |
|-------|--------|
| Vote is from committee member | `verify_vote_membership()` |
| Proposal is from expected leader | `verify_proposal_leader()` |
| QC has 2f+1 valid committee sigs | `verify_qc_committee()` |
| NewView is from committee member | `verify_new_view_membership()` |
| View is not stale (within 2 of current) | All verification functions |

Signing payload computation:
- Vote: `SHA256("HOTSTUFF_VOTE_V1" ‖ view ‖ block_id ‖ phase_byte)`
- Proposal: `SHA256("HOTSTUFF_PROPOSAL_V1" ‖ block_id ‖ view ‖ leader_id ‖ batch_root)`

## Crash Recovery

On restart, node loads `HotStuffConsensusState` from sled:
- `current_view` — resume at this view
- `locked_qc` — safety constraint
- `high_qc` — liveness (extend from here)
- `voted_views` — prevent double-voting
- `last_finalized_view` — last committed state

Recovery procedure:
1. Load consensus state from sled
2. Restore SafetyRule (locked_qc, high_qc, current_view)
3. Check voted_views before accepting any vote request
4. Wait for NewView messages to sync with current network view
5. Resume normal consensus participation

## Migration from Old Quorum Model

The old `Phase2PoSeqNode` used a 2-phase proposal/attestation flow. HotStuff replaces this with:
- 3-phase pipelined voting (PREPARE → PRE-COMMIT → COMMIT)
- Explicit lock tracking (PRE-COMMIT QC locks)
- Pacemaker-driven view changes (timeout + exponential backoff)
- Deterministic leader rotation via SHA256

The old flow in `node_runner.rs` runs in parallel with HotStuff during transition. To complete migration:
1. Feature-gate old proposal/attestation handlers
2. Route all consensus through HotStuffEngine
3. Remove old FinalizationEngine path

## Fairness Integration

HotStuff decides consensus on batch proposals, but fairness validation is mandatory before voting:
- Ordering commitment must match deterministic PoSeq ordering
- Fairness root must match the fairness log
- Anti-starvation rules must be satisfied
- DA artifacts must be available

A proposal that is consensus-valid but fairness-invalid must be REJECTED by honest validators.
