# Omniphi Intent-Based Execution Architecture

**Version**: 1.0
**Date**: 2026-03-15
**Status**: Phase 1 Specification

---

## Section 1 — Core System Architecture

### 1.1 System Overview

Omniphi is an intent-native blockchain where users express desired outcomes rather than low-level transaction instructions. A decentralized solver market competes to fulfill those intents. The PoSeq sequencing layer orders solver bundles fairly. HotStuff BFT finalizes the ordering. An object-based runtime verifies solver results against user constraints before committing state.

The system guarantees: users get what they asked for (or nothing), solvers compete on execution quality (not speed of copying), and sequencing is deterministic and auditable.

### 1.2 Architectural Layers

```
┌─────────────────────────────────────────────────┐
│           Intent-Native User Layer               │
│   Users submit intents describing desired outcomes│
├─────────────────────────────────────────────────┤
│           Solver Auction Market                   │
│   Solvers commit/reveal execution bundles         │
├─────────────────────────────────────────────────┤
│           PoSeq Fair Sequencing                   │
│   Deterministic ordering of valid bundles         │
├─────────────────────────────────────────────────┤
│           HotStuff BFT Finality                   │
│   Consensus on canonical bundle sequence          │
├─────────────────────────────────────────────────┤
│      Object-Based Parallel Settlement             │
│   Verify solver results, commit state transitions │
└─────────────────────────────────────────────────┘
```

### 1.3 Component Responsibilities

**Intent-Native User Layer**
- Accepts user intents (structured outcome descriptions)
- Validates intent schema and signatures
- Manages intent lifecycle (admission → expiry)
- Gossips intents to the solver network
- Provides intent pool queries for solvers

**Solver Auction Market**
- Registers and bonds solvers
- Runs commit-reveal auctions per intent batch
- Validates bundle commitments and reveals
- Scores and ranks competing solver bundles
- Tracks solver reputation and enforces penalties

**PoSeq Fair Sequencing**
- Collects revealed bundles from the auction phase
- Filters out invalid or expired bundles
- Applies deterministic ordering rules
- Produces canonical batch ordering
- Emits FinalizationEnvelope to HotStuff

**HotStuff BFT Finality**
- Proposes blocks containing ordered bundle sequences
- Validates sequence correctness, commit-reveal integrity, fairness
- Runs pipelined 3-phase BFT voting (Prepare/PreCommit/Commit/Decide)
- Produces QuorumCertificates
- Finalizes batches with 2f+1 signatures

**Object-Based Parallel Settlement**
- Receives finalized batch from HotStuff
- Replays solver execution plans against object state
- Verifies every intent constraint is satisfied
- Detects read/write conflicts and schedules parallel groups
- Produces execution receipts with state proofs
- Commits state root on success; rejects on violation

### 1.4 Data Flow Between Layers

```
User → Intent-Native Layer:
    IntentTransaction { intent_id, type, constraints, signature }

Intent-Native Layer → Solver Market:
    AdmittedIntent { intent_id, constraints, deadline, nonce }

Solver Market (Commit Phase) → PoSeq:
    BundleCommitment { bundle_id, solver_id, commitment_hash, valid_until }

Solver Market (Reveal Phase) → PoSeq:
    BundleReveal { bundle_id, execution_steps, predicted_outputs, proof_data }

PoSeq → HotStuff:
    SequencedBatch { batch_id, ordered_bundle_ids, sequence_root, fairness_meta }

HotStuff → Settlement Runtime:
    FinalizationEnvelope { batch_id, ordered_bundles, quorum_certificate, commitment }

Settlement Runtime → Chain:
    SettlementResult { state_root, receipts[], dispute_window_start }
```

### 1.5 What Each Layer Verifies

| Layer | Verifies |
|-------|----------|
| Intent-Native | Signature validity, schema correctness, nonce uniqueness, deadline not expired |
| Solver Market | Solver bond exists, commitment hash matches reveal, no duplicate fills |
| PoSeq | Bundle validity, ordering fairness, no censored intents, deterministic sequence |
| HotStuff BFT | Sequence correctness, commit-reveal integrity, data availability, quorum threshold |
| Settlement | Solver output satisfies intent constraints, state transitions valid, gas within bounds |

### 1.6 Execution Pipeline (Concrete)

```
1. User signs and submits IntentTransaction
2. Intent pool validates and admits intent
3. Intent pool gossips intent to all connected solvers
4. COMMIT PHASE opens for current batch window
   - Solvers evaluate intents, produce execution plans
   - Solvers submit BundleCommitment (hash of plan)
5. COMMIT PHASE closes
6. REVEAL PHASE opens
   - Solvers submit BundleReveal (actual plan matching commitment)
   - Late or non-matching reveals are penalized
7. REVEAL PHASE closes
8. PoSeq leader collects valid reveals
   - Filters invalid/expired bundles
   - Groups bundles by target intent
   - Applies ordering rules (quality → deterministic tiebreak)
   - Produces canonical SequencedBatch
9. HotStuff BFT proposes block with SequencedBatch
   - Validators verify sequence, vote through 3 phases
   - QuorumCertificate produced with 2f+1 signatures
10. Finalized batch delivered to Settlement Runtime
    - Runtime replays each bundle's execution plan
    - Verifies intent constraints satisfied
    - Produces ExecutionReceipts
    - Computes post-execution state root
11. Receipts and state root committed on-chain
12. Dispute window opens
    - Fraud proofs can challenge any receipt
13. After dispute window: settlement is final
```

---

## Section 2 — Intent System

### 2.1 How Intents Differ from Transactions

Traditional transactions are imperative: "transfer 100 USDC from A to B" or "call swap(tokenA, tokenB, amount)". The user specifies the exact operations.

Intents are declarative: "I want to swap at least 95 USDC worth of ETH for USDC, with max fee 0.3%, by block 500000." The user specifies the desired outcome and constraints. The protocol finds the best way to achieve it.

| Property | Transaction | Intent |
|----------|------------|--------|
| Execution logic | Specified by user | Determined by solver |
| Optimization | None (user picks path) | Competitive (best solver wins) |
| MEV exposure | High (path visible) | Low (solver competes on outcome) |
| Composability | Manual (user chains calls) | Automatic (solver finds routes) |
| Failure mode | Reverts on-chain | Never submitted if unsolvable |

### 2.2 Intent Schema

```
IntentTransaction {
    // Identity
    intent_id:       [u8; 32]       // SHA256(user ‖ nonce ‖ type ‖ constraints)
    intent_type:     IntentType      // SwapIntent | PaymentIntent | RouteLiquidityIntent
    version:         u16             // Schema version (1 for Phase 1)

    // User
    user:            [u8; 32]        // User public key
    nonce:           u64             // Monotonic, prevents replay

    // Assets (type-specific, present for Swap/Payment)
    asset_in:        Option<AssetId>       // Asset user provides
    asset_out:       Option<AssetId>       // Asset user wants
    amount_in:       Option<u128>          // Exact amount user provides
    min_amount_out:  Option<u128>          // Minimum acceptable output

    // Routing
    recipient:       Option<[u8; 32]>      // Output destination (default: user)

    // Time
    deadline:        u64                   // Block height or unix timestamp
    valid_from:      Option<u64>           // Earliest valid block (default: now)

    // Fees
    max_fee:         u64                   // Maximum fee in basis points (0-10000)
    tip:             Option<u64>           // Optional solver tip (incentivizes priority)

    // Execution preferences
    partial_fill_allowed:    bool          // Can solver fill partially?
    solver_permissions:      SolverPermissions  // Whitelist/blacklist solvers
    execution_preferences:   ExecutionPrefs     // Priority, latency tolerance

    // Integrity
    witness_hash:    [u8; 32]              // Hash of off-chain context (optional)
    signature:       Vec<u8>               // Ed25519 signature over all fields
}
```

### 2.3 Supporting Types

```
AssetId {
    chain_id:    u32         // Chain identifier (0 = native)
    asset_type:  AssetType   // Native | Token | Object
    identifier:  [u8; 32]    // Token address or object ID
}

SolverPermissions {
    mode:           PermissionMode   // AllowAll | Whitelist | Blacklist
    solver_ids:     Vec<[u8; 32]>    // Applicable solver public keys
}

ExecutionPrefs {
    priority:           Priority        // Normal | High | Urgent
    max_solver_count:   Option<u8>      // Max solvers that can attempt (default: unlimited)
    require_full_fill:  bool            // Must fill 100% or nothing
}
```

### 2.4 Phase 1 Intent Types

**SwapIntent**

User wants to exchange one asset for another at acceptable terms.

```
SwapIntent {
    asset_in:        AssetId
    asset_out:       AssetId
    amount_in:       u128
    min_amount_out:  u128
    max_slippage_bps: u16    // Additional slippage tolerance (0-1000)
    route_hint:      Option<Vec<AssetId>>  // Suggested intermediate assets
}
```

Solver must prove: user receives >= min_amount_out of asset_out, pays <= amount_in of asset_in, fee <= max_fee.

**PaymentIntent**

User wants to send a specific amount to a recipient.

```
PaymentIntent {
    asset:       AssetId
    amount:      u128
    recipient:   [u8; 32]
    memo:        Option<Vec<u8>>   // Up to 256 bytes
}
```

Solver must prove: recipient balance increased by exactly `amount`, sender balance decreased by `amount + fee`, fee <= max_fee.

**RouteLiquidityIntent**

User or protocol wants to move liquidity between pools/vaults.

```
RouteLiquidityIntent {
    source_pool:      [u8; 32]
    target_pool:      [u8; 32]
    asset:            AssetId
    amount:           u128
    min_received:     u128
    rebalance_params: RebalanceParams
}

RebalanceParams {
    max_hops:          u8       // Maximum intermediate pools (1-5)
    max_price_impact:  u16      // Basis points (0-500)
}
```

Solver must prove: liquidity moved through valid pools, target pool received >= min_received, price impact <= max_price_impact per hop.

### 2.5 Intent Lifecycle

```
                  ┌──────────┐
                  │ Created  │ User constructs and signs intent
                  └────┬─────┘
                       │ validate_basic()
                  ┌────▼─────┐
                  │ Admitted │ Intent pool accepts, gossips to network
                  └────┬─────┘
                       │ batch window opens
                  ┌────▼─────┐
                  │   Open   │ Solvers can see and evaluate intent
                  └────┬─────┘
                       │ solver submits matching bundle
                  ┌────▼─────┐
                  │ Matched  │ At least one valid bundle targets this intent
                  └────┬─────┘
                       │ PoSeq includes in batch
                  ┌────▼──────┐
                  │ Sequenced │ Intent's winning bundle is in canonical order
                  └────┬──────┘
                       │ settlement runtime executes
                  ┌────▼──────┐
                  │ Executed  │ Solver plan verified against constraints
                  └────┬──────┘
                       │ dispute window passes
                  ┌────▼──────┐
                  │ Settled  │ Final, irreversible
                  └──────────┘
```

**Terminal states (from any active state):**

```
Open/Matched ──deadline expires──→ Expired
Any ──user cancels before Sequenced──→ Cancelled
Executed ──fraud proof accepted──→ Disputed
Disputed ──slash confirmed──→ Slashed
```

### 2.6 State Transitions

| From | To | Trigger | Condition |
|------|----|---------|-----------|
| Created | Admitted | `admit_intent()` | Valid schema, valid sig, nonce unused, deadline in future |
| Admitted | Open | Batch window opens | Automatic when commit phase begins |
| Open | Matched | `match_bundle()` | Valid revealed bundle targets this intent |
| Open | Expired | Deadline check | `current_block >= deadline` and no match |
| Matched | Sequenced | PoSeq ordering | Winning bundle included in SequencedBatch |
| Sequenced | Executed | Settlement | Runtime verifies constraints satisfied |
| Executed | Settled | Dispute window close | No valid fraud proof submitted |
| Executed | Disputed | `submit_fraud_proof()` | Valid fraud proof within dispute window |
| Disputed | Slashed | Slash execution | Governance or automatic slash applied |
| Open | Cancelled | `cancel_intent()` | User signs cancellation before Sequenced |
| Matched | Cancelled | `cancel_intent()` | User signs cancellation before Sequenced |

### 2.7 Intent ID Derivation

Intent IDs are deterministic and collision-resistant:

```
intent_id = SHA256(
    user_pubkey ‖
    nonce.to_be_bytes() ‖
    intent_type_tag ‖    // 1 byte: 0x01=Swap, 0x02=Payment, 0x03=Route
    canonical_constraints_bytes ‖
    deadline.to_be_bytes()
)
```

This ensures the same user cannot submit two intents with identical parameters (replay protection via nonce), and intent_id can be recomputed by any node for verification.

---

## Section 3 — Intent Pool (Intent Mempool)

### 3.1 Overview

The intent pool is a local, per-node data structure that holds admitted intents awaiting solver matching. It is analogous to a transaction mempool but operates on declarative intents rather than imperative transactions.

### 3.2 How Intents Propagate

```
1. User submits IntentTransaction to any full node via RPC
2. Receiving node validates:
   - Schema correctness
   - Signature validity (Ed25519 over canonical bytes)
   - Nonce > last known nonce for this user
   - Deadline > current_block + MIN_INTENT_LIFETIME (e.g., 10 blocks)
   - Fee budget >= MIN_INTENT_FEE
3. If valid, node adds to local intent pool
4. Node gossips IntentAnnouncement to peers:
   IntentAnnouncement {
       intent_id:    [u8; 32]
       intent_type:  u8          // type tag only (not full intent)
       user:         [u8; 32]
       deadline:     u64
       tip:          u64         // solvers see priority signal
       size_bytes:   u32         // for bandwidth budgeting
   }
5. Peers receiving announcement:
   - If intent_id unknown → request full IntentTransaction
   - If intent_id known → ignore (dedup)
6. Solvers subscribe to intent pool via streaming RPC
   - Filter by intent_type, asset_in, asset_out, min_tip
```

### 3.3 Solver Subscription

Solvers maintain persistent connections to one or more full nodes and receive intent stream updates:

```
SolverSubscription {
    solver_id:          [u8; 32]
    intent_type_filter: Vec<IntentType>   // empty = all types
    asset_filter:       Vec<AssetId>      // empty = all assets
    min_tip:            u64               // filter low-value intents
    min_amount:         u128              // filter dust
}
```

Nodes push new intents matching the subscription filter. This is pull-based from the solver's perspective (the solver initiates the subscription), push-based from the node's perspective (the node sends matching intents as they arrive).

### 3.4 Spam Prevention

**Fee Floor**: Every intent must specify `max_fee >= MIN_INTENT_FEE`. If the intent expires without execution, no fee is charged — but the fee floor prevents zero-cost spam submission.

**Rate Limiting**: Per-user rate limit of MAX_INTENTS_PER_BLOCK (e.g., 10). Exceeding this causes rejection at the pool level.

**Nonce Gaps**: If a user submits nonce 5 but nonce 4 has not been seen, the intent is held in a pending buffer (up to MAX_NONCE_GAP = 3). If the gap exceeds this, the intent is dropped.

**Proof of Stake**: Future Phase 2 enhancement — users must hold a minimum balance to submit intents. Phase 1 relies on fee floor + rate limiting.

**Size Limits**: `MAX_INTENT_SIZE = 4096 bytes`. Intents exceeding this are rejected.

### 3.5 Deduplication Rules

```
1. Primary key: intent_id (derived from user + nonce + type + constraints)
2. If intent_id already in pool → reject silently (idempotent)
3. If same user + nonce but different intent_id → reject (nonce conflict)
4. Gossip dedup: IntentAnnouncement includes intent_id;
   peers track seen IDs in a bloom filter (rotated every epoch)
```

### 3.6 Expiration Logic

```
Every EXPIRY_CHECK_INTERVAL blocks (e.g., 5):
    for each intent in pool:
        if current_block >= intent.deadline:
            remove from pool
            emit IntentExpired { intent_id, reason: DeadlineReached }

        if current_block >= intent.admitted_at + MAX_POOL_RESIDENCE:
            remove from pool
            emit IntentExpired { intent_id, reason: MaxResidenceExceeded }
```

`MAX_POOL_RESIDENCE` (e.g., 1000 blocks) prevents indefinite pool occupancy even if the deadline is far in the future. This is a local node policy, not a consensus rule.

### 3.7 Pool Types

**Public Intent Pool (Phase 1)**

All intents are visible to all solvers. This is the simplest model and is implemented first.

Properties:
- Full transparency: any solver can see any intent
- Simple gossip: standard flood-fill propagation
- No privacy: intent parameters are public before execution
- Vulnerable to: sophisticated solver frontrunning of other solvers (not user frontrunning — solvers protect users)

**Protected Intent Pool (Future)**

Intents are encrypted with a threshold key. Only the committee can decrypt after the commit phase closes. This prevents solver-on-solver frontrunning.

Properties:
- Encrypted intents: `encrypted_intent = Encrypt(threshold_pubkey, intent_bytes)`
- Decryption after commit: committee releases decryption shares after commit phase closes
- Solvers see intent metadata (type, assets, approximate size) but not exact amounts
- Requires threshold key management infrastructure

Phase 1 implements public pool only. Protected pool is deferred to Phase 2.

### 3.8 Pool Capacity

```
MAX_POOL_SIZE = 50_000 intents

Eviction policy (when pool is full):
1. Remove expired intents first
2. Remove lowest-tip intents next
3. If still full, reject new intents with tip < pool minimum tip
```

---

## Section 4 — Solver Market

### 4.1 Overview

Solvers are economic agents that compete to fill user intents. They evaluate intents, find optimal execution routes, and submit execution bundles. The protocol selects winning bundles based on user outcome quality.

### 4.2 Solver Registration

Solvers must register on-chain before participating:

```
MsgRegisterSolver {
    solver_id:              [u8; 32]    // Solver public key
    moniker:                String      // Human-readable name (3-32 chars)
    operator_address:       String      // Cosmos address for staking/rewards
    bond_amount:            u128        // Initial bond (>= MIN_SOLVER_BOND)
    supported_intent_types: Vec<u8>     // Intent type tags this solver handles
    endpoint_url:           Option<String>  // Optional RPC endpoint
    metadata:               BTreeMap<String, String>  // Arbitrary metadata
    signature:              Vec<u8>     // Ed25519 over registration fields
}
```

Registration creates an on-chain `SolverProfile`:

```
SolverProfile {
    solver_id:              [u8; 32]
    moniker:                String
    operator_address:       String
    stake:                  u128        // Current bond amount
    supported_intent_types: Vec<u8>
    is_active:              bool        // Can participate in auctions
    registered_epoch:       u64
    last_active_epoch:      u64

    // Performance metrics (updated each epoch)
    performance_score:      u64         // 0-10000 bps, based on fill rate
    violation_score:        u64         // 0-10000 bps, based on penalty history
    latency_score:          u64         // 0-10000 bps, based on reveal timeliness
    total_fills:            u64         // Lifetime successful fills
    total_violations:       u64         // Lifetime violations

    // Capital profile
    capital_profile: SolverCapitalProfile
}

SolverCapitalProfile {
    available_liquidity:    BTreeMap<AssetId, u128>  // Self-reported
    supported_pools:        Vec<[u8; 32]>            // Pool IDs solver can access
    max_single_fill:        u128                     // Largest single fill capacity
}
```

### 4.3 Bonding Requirements

```
MIN_SOLVER_BOND = 10_000 OMNI   // Minimum to register
ACTIVE_SOLVER_BOND = 50_000 OMNI // Minimum to participate in auctions

Bond is slashable for:
- Invalid settlement (100% of bond at risk)
- Commit without reveal (1% per occurrence)
- Fraudulent route (50-100% based on severity)
- Double fill (100%)

Bond can be increased:
- MsgIncreaseSolverBond { solver_id, amount }

Bond can be decreased (with unbonding period):
- MsgDecreaseSolverBond { solver_id, amount }
- UNBONDING_PERIOD = 7 days (7 * 24 * 60 * 60 / block_time blocks)
- During unbonding: bond is locked, solver can still be slashed
```

### 4.4 Solver Capabilities

Solvers declare which intent types they support. The intent-solver matcher uses this to filter candidates:

```
Capability Matrix (Phase 1):

SwapIntent:
  - Requires: access to liquidity pools
  - Solver must: find route, compute output, prove execution

PaymentIntent:
  - Requires: access to sender's asset
  - Solver must: route payment, minimize fees

RouteLiquidityIntent:
  - Requires: access to source and target pools
  - Solver must: compute multi-hop route, satisfy price impact limits
```

### 4.5 Solver Evaluation Workflow

```
1. Solver receives intent from pool subscription
2. Solver evaluates:
   a. Can I fill this intent? (capability check)
   b. What's my best execution route? (optimization)
   c. What's my expected output? (simulation)
   d. Is it profitable after gas/fees? (economics)
3. If viable:
   a. Solver constructs ExecutionPlan (read/write sets, operations)
   b. Solver computes commitment_hash = SHA256(execution_plan_bytes)
   c. Solver submits BundleCommitment during commit phase
4. If commit phase ends and solver committed:
   a. Solver submits BundleReveal with full execution plan
   b. Protocol verifies commitment_hash matches
```

### 4.6 Solver Responsibilities

1. **Honesty**: Reveal must match commitment. Mismatch = slash.
2. **Feasibility**: Execution plan must be executable against current state. Invalid plans = reputation penalty.
3. **Quality**: Maximize user output. Winning selection based on quality score.
4. **Timeliness**: Reveal before reveal phase closes. Late reveal = commit-without-reveal penalty.
5. **Uniqueness**: No double-filling an intent (submitting bundles that overlap in target intents with inconsistent state).

---

## Section 5 — Commit-Reveal Solver Auction

### 5.1 Auction Phases

Each batch window runs a two-phase auction:

```
┌──────────────────────────────────────────────────────────┐
│                    BATCH WINDOW N                         │
│                                                          │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐ │
│  │ COMMIT PHASE│→ │ REVEAL PHASE │→ │ SELECTION PHASE │ │
│  │ (C blocks)  │  │ (R blocks)   │  │ (instant)       │ │
│  └─────────────┘  └──────────────┘  └─────────────────┘ │
└──────────────────────────────────────────────────────────┘

Phase 1 defaults:
  C = 5 blocks (commit phase duration)
  R = 3 blocks (reveal phase duration)
  Batch window = C + R + 1 = 9 blocks
```

### 5.2 Commit Phase

During the commit phase, solvers submit sealed commitments:

```
BundleCommitment {
    bundle_id:            [u8; 32]    // Random, unique per bundle
    solver_id:            [u8; 32]    // Solver public key
    batch_window:         u64         // Which batch window this targets
    target_intent_count:  u16         // Number of intents this bundle fills
    commitment_hash:      [u8; 32]    // SHA256(reveal_preimage)
    expected_outputs_hash:[u8; 32]    // SHA256(serialized predicted_outputs)
    execution_plan_hash:  [u8; 32]    // SHA256(serialized execution_steps)
    valid_until:          u64         // Block height after which commitment expires
    bond_locked:          u128        // Bond amount locked for this commitment
    signature:            Vec<u8>     // Ed25519(solver_key, canonical_bytes)
}
```

**Commitment hash construction:**
```
reveal_preimage = bundle_id ‖ solver_id ‖ execution_steps_bytes ‖
                  predicted_outputs_bytes ‖ fee_breakdown_bytes ‖ nonce
commitment_hash = SHA256(reveal_preimage)
```

**Rules during commit phase:**
- A solver can submit multiple commitments for the same batch window (different bundles)
- A solver can target the same intent from multiple bundles (only one will be selected)
- Commitments are stored in a per-batch-window commitment pool
- Commitments are not gossipped to other solvers (only to validators/sequencers)
- MAX_COMMITMENTS_PER_SOLVER_PER_WINDOW = 10

### 5.3 Reveal Phase

After commit phase closes, reveal phase opens:

```
BundleReveal {
    bundle_id:          [u8; 32]     // Must match a commitment
    solver_id:          [u8; 32]     // Must match commitment's solver_id
    batch_window:       u64          // Must match commitment's batch_window

    // The actual execution plan
    target_intent_ids:  Vec<[u8; 32]>  // Intents this bundle fills
    execution_steps:    Vec<ExecutionStep>
    liquidity_sources:  Vec<LiquiditySource>
    predicted_outputs:  Vec<PredictedOutput>
    fee_breakdown:      FeeBreakdown

    // Proof data
    nonce:              [u8; 32]     // Used in commitment_hash
    proof_data:         Vec<u8>      // Optional: ZK proof or routing proof

    signature:          Vec<u8>      // Ed25519(solver_key, canonical_bytes)
}

ExecutionStep {
    step_index:     u16
    operation:      OperationType    // Debit | Credit | Swap | Lock | Unlock
    object_id:      [u8; 32]        // Target object
    asset:          AssetId
    amount:         u128
    read_set:       Vec<[u8; 32]>   // Objects read by this step
    write_set:      Vec<[u8; 32]>   // Objects modified by this step
}

PredictedOutput {
    intent_id:      [u8; 32]
    asset_out:      AssetId
    amount_out:     u128
    fee_charged:    u64             // Basis points
}

FeeBreakdown {
    solver_fee:     u64             // What solver takes
    protocol_fee:   u64             // What protocol takes
    total_fee_bps:  u64             // Total in basis points
}

LiquiditySource {
    source_type:    LiquidityType   // Pool | Vault | External
    source_id:      [u8; 32]
    asset:          AssetId
    amount_used:    u128
}
```

### 5.4 Reveal Validation

When a reveal is submitted, the protocol verifies:

```
1. COMMITMENT EXISTS
   commitment = lookup(bundle_id, solver_id, batch_window)
   if commitment is None → reject "no matching commitment"

2. COMMITMENT HASH MATCHES
   reveal_preimage = bundle_id ‖ solver_id ‖ execution_steps_bytes ‖
                     predicted_outputs_bytes ‖ fee_breakdown_bytes ‖ nonce
   computed_hash = SHA256(reveal_preimage)
   if computed_hash != commitment.commitment_hash → reject + SLASH

3. OUTPUTS HASH MATCHES
   if SHA256(predicted_outputs_bytes) != commitment.expected_outputs_hash → reject

4. PLAN HASH MATCHES
   if SHA256(execution_steps_bytes) != commitment.execution_plan_hash → reject

5. INTENT VALIDITY
   for each intent_id in target_intent_ids:
       intent = lookup(intent_id)
       if intent is None → reject "unknown intent"
       if intent.state != Open and intent.state != Matched → reject
       if intent.deadline < current_block → reject "expired intent"

6. SOLVER IS ACTIVE
   solver = lookup(solver_id)
   if !solver.is_active → reject
   if solver.stake < ACTIVE_SOLVER_BOND → reject

7. BASIC FEASIBILITY
   for each predicted_output:
       if output.fee_charged > intent.max_fee → reject "fee exceeds limit"
       if output.amount_out < intent.min_amount_out → reject "below minimum"
```

### 5.5 Why Commit-Reveal Prevents Copying

Without commit-reveal, a fast solver could:
1. See another solver's revealed execution plan
2. Copy the plan (possibly with minor improvements)
3. Submit the copied plan and win the auction

With commit-reveal:
1. Solver A commits hash H_A = SHA256(plan_A) during commit phase
2. Solver B commits hash H_B = SHA256(plan_B) during commit phase
3. Neither solver sees the other's plan during commit phase
4. During reveal phase, both reveal their plans
5. If Solver B's reveal doesn't match their commitment H_B, they are slashed
6. Solver B cannot change their plan after seeing Solver A's reveal

The commitment hash binds the solver to their execution plan before any plan is publicly visible.

### 5.6 Commit Without Reveal Penalty

If a solver commits but does not reveal during the reveal phase:

```
COMMIT_WITHOUT_REVEAL_PENALTY = 100 bps of locked bond (1%)

penalty_amount = commitment.bond_locked * 100 / 10000
solver.stake -= penalty_amount
solver.violation_score += 100

if solver.violation_score > MAX_VIOLATION_SCORE:
    solver.is_active = false  // auto-deactivate
```

This prevents griefing attacks where solvers flood the commitment pool with commitments they never intend to reveal, wasting other solvers' evaluation effort.

---

## Section 6 — PoSeq Sequencing Layer

### 6.1 Responsibilities

The PoSeq sequencing layer sits between the solver auction and HotStuff consensus. Its job is to take the set of valid revealed bundles and produce a canonical, deterministic, fair ordering.

PoSeq is already implemented in Omniphi (`poseq/src/`) with 50+ modules covering ordering, fairness, anti-MEV, and multi-node consensus. This section specifies how the existing PoSeq infrastructure integrates with the new intent/solver pipeline.

### 6.2 Sequencing Stages

```
Stage 1: BUNDLE INTAKE
    Input:  Set of BundleReveals from the reveal phase
    Action: Normalize and validate each reveal
    Output: Set of ValidatedBundles

Stage 2: REVEAL VALIDATION
    Input:  ValidatedBundles
    Action: Verify commitment hashes, check solver bonds, validate outputs
    Output: Set of VerifiedBundles (invalid bundles discarded + flagged)

Stage 3: CANDIDATE FILTERING
    Input:  VerifiedBundles
    Action: Remove expired, remove bundles for cancelled intents,
            remove bundles from deactivated solvers
    Output: Set of CandidateBundles

Stage 4: ORDERING COMPUTATION
    Input:  CandidateBundles
    Action: Group by intent, rank within groups, produce canonical order
    Output: OrderedBundleList

Stage 5: SEQUENCE COMMITMENT
    Input:  OrderedBundleList
    Action: Compute sequence root, produce SequencedBatch
    Output: SequencedBatch ready for HotStuff proposal
```

### 6.3 Ordering Rules

The ordering algorithm runs per batch window:

```
STEP 1: DISCARD INVALID BUNDLES
  Remove any bundle where:
  - commitment_hash mismatch (should already be caught in Stage 2)
  - solver deactivated or bond insufficient
  - any target intent expired or cancelled

STEP 2: GROUP BY INTENT
  For each intent_id in the current batch:
    bundles_for_intent[intent_id] = all bundles targeting this intent

STEP 3: RANK WITHIN EACH INTENT GROUP
  For each intent group:
    Sort bundles by user_outcome_score descending:
      user_outcome_score = predicted_amount_out * (10000 - fee_bps) / 10000

    This means: highest effective output for the user wins

STEP 4: BREAK TIES DETERMINISTICALLY
  If two bundles have identical user_outcome_score:
    tiebreak_key = SHA256(bundle_id ‖ intent_id ‖ batch_window.to_be_bytes())
    Sort by tiebreak_key ascending (lexicographic)

  This is:
  - Deterministic (all nodes compute the same order)
  - Unpredictable (depends on bundle_id which is random)
  - Unbiasable (neither solver can influence the tiebreak)

STEP 5: SELECT WINNER PER INTENT
  For each intent group:
    winning_bundle = bundles_for_intent[intent_id][0]  // highest ranked

  Losing bundles are discarded (no penalty — losing is normal)

STEP 6: PRODUCE CANONICAL ORDERED BUNDLE LIST
  ordered_bundles = winning bundles sorted by:
    1. Intent deadline ascending (most urgent first)
    2. Intent tip descending (highest priority)
    3. SHA256(intent_id ‖ batch_window) ascending (deterministic tiebreak)

  This ordering determines settlement execution order.

STEP 7: COMPUTE SEQUENCE ROOT
  sequence_root = SHA256(
      ordered_bundles[0].bundle_id ‖
      ordered_bundles[1].bundle_id ‖
      ... ‖
      ordered_bundles[n].bundle_id
  )
```

### 6.4 Integration with Existing PoSeq

The existing `PoSeqNode` pipeline (SubmissionReceiver → SubmissionQueue → OrderingEngine → BatchBuilder) is extended:

```
Current PoSeq pipeline (generic submissions):
  SubmissionReceiver → Validator → Queue → OrderingEngine → BatchBuilder

Extended pipeline (intent-aware):
  BundleReveal → BundleIntakeAdapter → SubmissionReceiver → Validator →
  IntentAwarePriorityQueue → BundleOrderingEngine → BatchBuilder

BundleIntakeAdapter:
  - Converts BundleReveal into SequencingSubmission
  - Attaches intent metadata (deadline, tip, user_outcome_score)
  - Sets priority based on intent urgency and solver quality

IntentAwarePriorityQueue:
  - Extends SubmissionQueue with intent grouping
  - Maintains per-intent bundle lists
  - Applies ranking rules from Section 6.3

BundleOrderingEngine:
  - Extends OrderingEngine with intent-aware ordering
  - Implements the 7-step ordering algorithm
  - Produces canonical order deterministically
```

### 6.5 Fairness Protections

Leveraging the existing PoSeq fairness modules:

**Anti-Censorship** (`poseq/src/inclusion/`)
- If an intent has been Open for > FORCED_INCLUSION_THRESHOLD blocks and has valid bundles, it MUST be included in the next batch
- Sequencer that censors forced-inclusion intents is slashable

**Anti-MEV** (`poseq/src/anti_mev/`)
- Bundle commitments are opaque during commit phase
- Sequencer cannot see execution plans until reveal phase
- Ordering rules use user_outcome_score (not solver profit), preventing profit-maximizing reordering

**Fairness Audit** (`poseq/src/fairness_audit/`)
- Every batch produces a FairnessReport
- Reports are included in FinalizationEnvelope
- Validators can challenge unfair orderings

**Determinism**
- All ordering rules use deterministic tiebreakers (SHA256-based)
- No timestamp-dependent ordering (uses block height only)
- No randomness that could be influenced by sequencer

### 6.6 Sequenced Batch Output

```
SequencedBatch {
    batch_id:               [u8; 32]    // SHA256(sequence_root ‖ batch_window)
    batch_window:           u64
    epoch:                  u64
    slot:                   u64
    leader_id:              [u8; 32]    // PoSeq leader for this slot

    // Canonical order
    ordered_bundles:        Vec<SequencedBundle>
    sequence_root:          [u8; 32]    // SHA256(bundle_id[0] ‖ ... ‖ bundle_id[n])

    // Metadata
    total_intents_filled:   u32
    total_bundles_received: u32         // Including discarded
    total_bundles_valid:    u32
    fairness_meta:          FairnessMeta

    // Commitment
    batch_commitment:       [u8; 32]    // SHA256(batch_id ‖ sequence_root ‖ epoch ‖ slot)
}

SequencedBundle {
    bundle_id:          [u8; 32]
    solver_id:          [u8; 32]
    target_intent_ids:  Vec<[u8; 32]>
    execution_steps:    Vec<ExecutionStep>
    predicted_outputs:  Vec<PredictedOutput>
    sequence_index:     u32             // Position in canonical order
}
```

---

## Section 7 — HotStuff BFT Finality

### 7.1 Integration with PoSeq

HotStuff BFT is already implemented in Omniphi (`poseq/src/hotstuff/`). The integration point is:

```
PoSeq produces SequencedBatch
    → PoSeq leader wraps as HotStuffBlock
    → HotStuff runs pipelined consensus
    → On finalization: FinalizationEnvelope emitted to Settlement Runtime
```

### 7.2 Block Proposal Process

The PoSeq leader for the current view constructs a block proposal:

```
HotStuffBlock {
    view:           u64
    parent_hash:    [u8; 32]
    payload_hash:   [u8; 32]    // SHA256(SequencedBatch bytes)
    proposer_id:    [u8; 32]

    // PoSeq-specific payload
    sequenced_batch: SequencedBatch
}
```

The leader broadcasts this proposal to all validators. Validators must verify the block before voting.

### 7.3 Validator Verification Checklist

Before casting a vote, each validator MUST verify:

```
1. SEQUENCE CORRECTNESS
   ☐ Recompute sequence_root from ordered_bundles
   ☐ Verify sequence_root matches claimed value
   ☐ Verify batch_commitment = SHA256(batch_id ‖ sequence_root ‖ epoch ‖ slot)
   ☐ Verify ordering follows deterministic rules (Section 6.3)
   ☐ Verify no forced-inclusion intents were censored

2. BUNDLE VALIDITY
   ☐ Every bundle has a matching BundleCommitment on record
   ☐ Every bundle's solver_id is registered and active
   ☐ Every bundle's target intents exist and are not expired
   ☐ No duplicate intent fills (each intent filled at most once per batch)

3. COMMIT-REVEAL INTEGRITY
   ☐ For each bundle: SHA256(reveal_preimage) == commitment_hash
   ☐ For each bundle: expected_outputs_hash matches
   ☐ For each bundle: execution_plan_hash matches
   ☐ No bundle from a solver with commit-without-reveal this window

4. DATA AVAILABILITY
   ☐ Full SequencedBatch payload is available (not just hash)
   ☐ All referenced BundleCommitments are available
   ☐ All referenced intents are available (in pool or archive)

5. ORDERING FAIRNESS
   ☐ FairnessMeta included and non-empty
   ☐ No evidence of sequencer manipulation (reordering for profit)
   ☐ Forced-inclusion rules respected
```

If any check fails, the validator does NOT vote and may submit a challenge.

### 7.4 Voting Phases

HotStuff runs pipelined 3-phase consensus (already implemented):

```
Phase 1: PREPARE
  Leader broadcasts: Proposal(block, highQC)
  Validators verify block → send PrepareVote
  Leader collects 2f+1 PrepareVotes → forms prepareQC

Phase 2: PRE-COMMIT
  Leader broadcasts: PreCommit(prepareQC)
  Validators verify prepareQC → send PreCommitVote
  Leader collects 2f+1 PreCommitVotes → forms precommitQC

Phase 3: COMMIT
  Leader broadcasts: Commit(precommitQC)
  Validators verify precommitQC → send CommitVote
  Leader collects 2f+1 CommitVotes → forms commitQC

Phase 4: DECIDE
  Leader broadcasts: Decide(commitQC)
  Validators apply finality → block is FINAL
```

A block is final when the Decide phase completes. At this point, the FinalizationEnvelope is emitted.

### 7.5 Slashable Validator Faults

| Fault | Severity | Penalty |
|-------|----------|---------|
| Equivocation (vote for conflicting blocks in same view) | CRITICAL | 100% bond slash + permanent ban |
| Unauthorized proposal (propose when not leader) | HIGH | 50% bond slash + jail |
| Unfair sequencing (proven ordering manipulation) | HIGH | 50% bond slash + jail |
| Censorship (proven forced-inclusion violation) | HIGH | 25% bond slash + jail |
| Liveness failure (offline for > LIVENESS_THRESHOLD views) | MEDIUM | 1% bond slash per view + temporary jail |
| Invalid block proposal (fails verification) | MEDIUM | 5% bond slash + jail for 1 epoch |

### 7.6 Finalization Output

```
FinalizationEnvelope {
    batch_id:                [u8; 32]
    delivery_id:             [u8; 32]    // Unique per delivery attempt
    attempt_count:           u32
    slot:                    u64
    epoch:                   u64
    sequence_number:         u64
    leader_id:               [u8; 32]
    parent_batch_id:         [u8; 32]

    // Canonical content
    ordered_submission_ids:  Vec<[u8; 32]>  // Bundle IDs in canonical order
    batch_root:              [u8; 32]
    finalization_hash:       [u8; 32]
    quorum_approvals:        usize
    committee_size:          usize

    // Metadata
    fairness:                FairnessMeta
    commitment:              BatchCommitment

    // Full bundle data (for settlement)
    bundles:                 Vec<SequencedBundle>
}
```

This envelope is delivered to the Settlement Runtime via the existing PoSeq→Runtime bridge (`poseq/src/bridge/`).

---

## Section 8 — Object-Based Settlement Runtime

### 8.1 Why Object-Based

Traditional account-based models (like Ethereum) use a global key-value store where every transaction can potentially read/write any account. This creates:
- Serialization bottlenecks (transactions must be ordered globally)
- State contention (concurrent transactions conflict on same accounts)
- Opaque dependencies (hard to know what a transaction touches until execution)

Omniphi uses an object-based model where:
- Every piece of state is an explicit Object with an owner and permissions
- Every execution step declares its read_set and write_set upfront
- Conflict detection is exact (no false positives)
- Non-conflicting operations can execute in parallel

### 8.2 Object Model

```
Object {
    object_id:      ObjectId        // Globally unique identifier
    object_type:    ObjectType      // Balance | Token | Pool | Vault | Custom
    owner:          [u8; 32]        // Current owner public key
    permissions:    PermissionSet   // Who can read/write/transfer
    state_data:     Vec<u8>         // Type-specific serialized state
    version:        u64             // Monotonically increasing on each mutation
    created_at:     u64             // Block height of creation
    last_modified:  u64             // Block height of last mutation
}

ObjectId = [u8; 32]   // SHA256(object_type ‖ creation_params ‖ creator ‖ nonce)

ObjectType:
    Balance {
        asset: AssetId
        amount: u128
    }
    Token {
        asset: AssetId
        total_supply: u128
        metadata: TokenMetadata
    }
    Pool {
        asset_a: AssetId
        asset_b: AssetId
        reserve_a: u128
        reserve_b: u128
        lp_supply: u128
        fee_bps: u16
    }
    Vault {
        asset: AssetId
        total_deposited: u128
        total_shares: u128
        strategy_id: [u8; 32]
    }
    Custom {
        type_tag: String
        data: Vec<u8>
    }

PermissionSet {
    owner_can:      CapabilitySet   // What the owner can do
    delegated:      Vec<Delegation> // Third-party permissions
}

Delegation {
    delegate:       [u8; 32]        // Delegated to this public key
    capabilities:   CapabilitySet
    expires_at:     Option<u64>     // Block height expiry
}

CapabilitySet = BitFlags {
    READ            = 0b00000001
    WRITE           = 0b00000010
    TRANSFER        = 0b00000100
    LOCK            = 0b00001000
    BURN            = 0b00010000
    ADMIN           = 0b10000000    // Can modify permissions
}
```

### 8.3 Execution Steps

Each bundle's execution plan consists of ordered steps. Each step explicitly declares what objects it reads and writes:

```
ExecutionStep {
    step_index:     u16
    operation:      OperationType

    // Object access declaration
    read_set:       Vec<ObjectId>   // Objects this step reads (version-pinned)
    write_set:      Vec<ObjectId>   // Objects this step modifies
    consumed:       Vec<ObjectId>   // Objects destroyed by this step
    created:        Vec<ObjectSpec> // Objects created by this step
    locked:         Vec<ObjectId>   // Objects locked during this step

    // Operation details
    params:         OperationParams
}

OperationType:
    Debit       // Reduce balance object
    Credit      // Increase balance object
    Swap        // Pool swap (read pool, write pool + balances)
    Lock        // Lock object for duration
    Unlock      // Unlock previously locked object
    Create      // Create new object
    Destroy     // Destroy object (must own)
    Transfer    // Change object ownership

OperationParams {
    asset:          Option<AssetId>
    amount:         Option<u128>
    recipient:      Option<[u8; 32]>
    pool_id:        Option<ObjectId>
    custom_data:    Option<Vec<u8>>
}

ObjectSpec {
    object_type:    ObjectType
    owner:          [u8; 32]
    initial_state:  Vec<u8>
    permissions:    PermissionSet
}
```

### 8.4 Conflict Detection

Two execution steps conflict if they access the same object and at least one access is a write:

```
conflicts(step_a, step_b) =
    (step_a.write_set ∩ step_b.write_set ≠ ∅) OR     // Write-Write
    (step_a.write_set ∩ step_b.read_set  ≠ ∅) OR     // Write-Read
    (step_a.read_set  ∩ step_b.write_set ≠ ∅)         // Read-Write

Read-Read is NOT a conflict (safe to parallelize).
```

### 8.5 Parallel Execution Scheduling

The scheduler (already implemented in `runtime/src/scheduler/`) builds a conflict graph and groups non-conflicting bundles:

```
1. Build conflict graph:
   Nodes = bundles in the sequenced batch
   Edges = conflict(bundle_a, bundle_b)

2. Graph coloring:
   Assign each bundle a group_index such that no two
   conflicting bundles share the same group_index.
   Use greedy coloring on the canonical ordering.

3. Execute groups in order:
   Group 0: execute all bundles (no conflicts within group)
   Group 1: execute all bundles (no conflicts within group)
   ...

   Groups execute SEQUENTIALLY.
   Bundles WITHIN a group can execute in PARALLEL.
```

This is already implemented in `ParallelScheduler` with `ExecutionGroup` structures.

### 8.6 State Transitions

```
Before execution:
  ObjectStore contains current state (all objects at their current versions)

For each execution group (in order):
  For each bundle in group (parallelizable):
    1. Read objects from read_set (version must match expected)
    2. If version mismatch → bundle fails (stale state)
    3. Apply operations:
       - Debit: balance.amount -= params.amount
       - Credit: balance.amount += params.amount
       - Swap: apply constant-product formula to pool
       - Lock: set object.locked = true
       - etc.
    4. Increment version on all written objects
    5. Record state changes in execution receipt

After all groups:
  Compute post-execution state root = SHA256(all_object_hashes)
```

---

## Section 9 — Settlement Verification

### 9.1 Overview

The settlement runtime does NOT trust solver execution plans. It replays each plan against the object store and verifies that the result satisfies all intent constraints. If any constraint is violated, the bundle is rejected and the solver is penalized.

### 9.2 SwapIntent Verification

```
verify_swap(intent: SwapIntent, receipt: ExecutionReceipt) → Result<(), VerificationError>

CHECKS:

1. ASSET_IN CORRECT
   ☐ User's asset_in balance decreased by exactly intent.amount_in
   ☐ No other asset_in debits from user's account in this bundle

2. ASSET_OUT CORRECT
   ☐ Recipient received asset_out (not a different asset)
   ☐ Recipient is intent.recipient (or intent.user if recipient is None)

3. MIN_OUTPUT SATISFIED
   ☐ actual_amount_out >= intent.min_amount_out
   ☐ If partial_fill_allowed:
       actual_amount_out >= intent.min_amount_out * fill_fraction
       fill_fraction must be reported in receipt

4. FEE WITHIN LIMIT
   ☐ actual_fee_bps <= intent.max_fee
   ☐ actual_fee_bps = (1 - actual_amount_out / fair_market_value) * 10000
   ☐ Alternative: fee_bps = solver_declared_fee + protocol_fee
   ☐ Both must be <= max_fee

5. RECIPIENT CORRECT
   ☐ Output credited to intent.recipient (or intent.user)
   ☐ No other parties received output from this intent

6. DEADLINE NOT EXCEEDED
   ☐ Execution block <= intent.deadline

7. STATE CONSISTENCY
   ☐ All pool states changed consistently (constant product invariant)
   ☐ No negative balances created
   ☐ Total asset conservation (sum of all debits == sum of all credits per asset)
```

### 9.3 PaymentIntent Verification

```
verify_payment(intent: PaymentIntent, receipt: ExecutionReceipt) → Result<(), VerificationError>

CHECKS:

1. SENDER BALANCE
   ☐ Sender had sufficient balance of intent.asset
   ☐ Sender balance decreased by intent.amount + actual_fee

2. CORRECT RECIPIENT
   ☐ intent.recipient balance increased by exactly intent.amount
   ☐ No other party received funds

3. CORRECT AMOUNT
   ☐ Transfer amount exactly matches intent.amount (no partial for payments)

4. VALID NONCE
   ☐ intent.nonce > last_executed_nonce for this user
   ☐ No duplicate execution of same nonce

5. FEE WITHIN LIMIT
   ☐ actual_fee <= intent.max_fee (basis points of intent.amount)

6. MEMO INTEGRITY
   ☐ If intent.memo is Some, memo hash is recorded in receipt
```

### 9.4 RouteLiquidityIntent Verification

```
verify_route_liquidity(intent: RouteLiquidityIntent, receipt: ExecutionReceipt) → Result<(), VerificationError>

CHECKS:

1. VALID LIQUIDITY SOURCE
   ☐ Source pool exists and contains intent.asset
   ☐ Source pool balance decreased by routed amount

2. CORRECT ROUTING
   ☐ Each hop in the route uses a valid pool
   ☐ Number of hops <= intent.rebalance_params.max_hops
   ☐ Each intermediate pool's state changed consistently

3. PRICE IMPACT PER HOP
   ☐ For each hop:
       price_impact_bps = |price_before - price_after| / price_before * 10000
       price_impact_bps <= intent.rebalance_params.max_price_impact

4. TARGET POOL RECEIVED
   ☐ Target pool balance increased by actual_received
   ☐ actual_received >= intent.min_received

5. STATE TRANSITIONS VALID
   ☐ All pool invariants maintained (constant product, reserve ratios)
   ☐ LP token supply changed correctly (if applicable)
   ☐ No assets created or destroyed (conservation)

6. ASSET CONSERVATION
   ☐ sum(amount removed from source + intermediaries) ==
     sum(amount added to target + intermediaries) + fees
```

### 9.5 Verification Failure Handling

```
If verification fails for a bundle:
    1. Bundle execution is ROLLED BACK (state changes undone)
    2. Intent returns to OPEN state (can be filled by another solver)
    3. ExecutionReceipt records failure:
       receipt.execution_status = Failed
       receipt.failure_reason = specific VerificationError
    4. Solver penalty applied:
       - First failure in epoch: warning (reputation -100)
       - Repeated failures: escalating bond slash (1% → 5% → 25%)
    5. FraudProofRecord created automatically (no dispute needed)
```

---

## Section 10 — Execution Receipts

### 10.1 Receipt Structure

Every executed bundle produces an ExecutionReceipt:

```
ExecutionReceipt {
    // Identity
    receipt_id:         [u8; 32]    // SHA256(intent_id ‖ bundle_id ‖ batch_id)
    intent_id:          [u8; 32]
    bundle_id:          [u8; 32]
    solver_id:          [u8; 32]
    batch_id:           [u8; 32]
    sequence_slot:      u64         // Position in the batch

    // Execution results
    amount_in:          u128        // Actual input consumed
    amount_out:         u128        // Actual output produced
    fee_paid_bps:       u64         // Fee charged in basis points
    recipient:          [u8; 32]    // Who received the output
    fill_fraction:      u64         // 0-10000 bps (10000 = full fill)

    // Execution status
    execution_status:   ExecutionStatus   // Succeeded | Failed | PartialFill
    failure_reason:     Option<String>    // If failed, why
    gas_used:           u64               // Gas consumed by this bundle

    // State proofs
    state_root_before:  [u8; 32]    // Object store root before execution
    state_root_after:   [u8; 32]    // Object store root after execution
    objects_read:       Vec<(ObjectId, u64)>   // (object_id, version_read)
    objects_written:    Vec<(ObjectId, u64)>   // (object_id, version_written)
    objects_created:    Vec<ObjectId>
    objects_consumed:   Vec<ObjectId>

    // Proof
    proof_hash:         [u8; 32]    // SHA256(execution trace)

    // Timing
    timestamp:          u64         // Block height of execution
    batch_window:       u64         // Which batch window
    epoch:              u64
}

ExecutionStatus:
    Succeeded       // All constraints verified, state committed
    Failed          // Constraint violation, state rolled back
    PartialFill     // partial_fill_allowed was true, filled < 100%
```

### 10.2 Receipt Computation

```
receipt_id = SHA256(intent_id ‖ bundle_id ‖ batch_id)

proof_hash = SHA256(
    for each step in execution_steps:
        step.operation_type ‖
        step.read_set_hashes ‖
        step.write_set_hashes ‖
        step.params_bytes
)

state_root_before = ObjectStore.root_hash() before bundle execution
state_root_after  = ObjectStore.root_hash() after bundle execution
```

### 10.3 Why Receipts Are Critical

**Auditing**: Any observer can verify that a solver's execution matched the intent constraints by checking the receipt. The state_root_before and state_root_after provide a Merkle proof that state transitions are correct.

**Disputes**: Fraud proofs reference receipts. A challenger can point to a specific receipt and prove that the execution violated a constraint. Without receipts, disputes would require re-executing entire batches.

**Analytics**: Receipts provide a complete record of solver performance — fill rates, fees charged, execution quality. This feeds into solver reputation scoring.

**Solver Reputation**: Each receipt updates the solver's performance metrics:
```
if receipt.execution_status == Succeeded:
    solver.total_fills += 1
    solver.performance_score = update_ema(solver.performance_score, quality_score)
    quality_score = (amount_out * 10000 / predicted_amount_out)  // actual vs predicted

if receipt.execution_status == Failed:
    solver.total_violations += 1
    solver.violation_score = update_ema(solver.violation_score, 10000)
```

### 10.4 Receipt Storage

Receipts are stored on-chain with the following key scheme:

```
Prefix 0x10: receipt by receipt_id
    Key:   0x10 ‖ receipt_id[32]
    Value: ExecutionReceipt (JSON serialized)

Prefix 0x11: receipts by intent_id (index)
    Key:   0x11 ‖ intent_id[32] ‖ receipt_id[32]
    Value: [] (presence key only)

Prefix 0x12: receipts by solver_id (index)
    Key:   0x12 ‖ solver_id[32] ‖ epoch[8] ‖ receipt_id[32]
    Value: [] (presence key only)

Prefix 0x13: receipts by batch_id (index)
    Key:   0x13 ‖ batch_id[32] ‖ sequence_slot[8]
    Value: receipt_id[32]
```

---

## Section 11 — Dispute System

### 11.1 Overview

The dispute system provides post-execution fraud proofs. Even though the settlement runtime verifies constraints at execution time, disputes handle edge cases where:
- The runtime itself had a bug
- State was corrupted before execution
- A validator colluded with a solver
- Off-chain evidence reveals fraud not detectable on-chain

### 11.2 Dispute Windows

```
FAST DISPUTE WINDOW
  Duration: 100 blocks (~10 minutes at 6s blocks)
  Who can submit: anyone with a valid fraud proof
  Bond required: FAST_DISPUTE_BOND = 1000 OMNI
  Resolution: automatic (on-chain verification of proof)

  During this window:
  - Settlement is TENTATIVE (not yet final)
  - Receipts are visible but state changes are pending
  - If a valid fraud proof is submitted, settlement is REVERTED

EXTENDED DISPUTE WINDOW
  Duration: 50400 blocks (~7 days at 12s blocks)
  Who can submit: anyone with a valid fraud proof
  Bond required: EXTENDED_DISPUTE_BOND = 10000 OMNI
  Resolution: governance vote (complex disputes)

  During this window:
  - Settlement is SOFT-FINAL (state changes applied but reversible)
  - Only disputes requiring off-chain evidence are accepted
  - Resolution requires governance vote with 2/3 quorum

After both windows close:
  - Settlement is HARD-FINAL (irreversible)
  - No further disputes accepted
```

### 11.3 Fraud Proof Types

```
FraudProof {
    proof_id:       [u8; 32]    // SHA256(challenger ‖ receipt_id ‖ proof_type)
    proof_type:     FraudProofType
    challenger:     [u8; 32]    // Public key of challenger
    receipt_id:     [u8; 32]    // Receipt being challenged
    evidence:       Vec<u8>     // Proof-type-specific evidence
    bond_amount:    u128        // Bond deposited
    submitted_at:   u64         // Block height
    signature:      Vec<u8>
}

FraudProofType:

1. INVALID_REVEAL
   Solver's reveal did not match their commitment hash.
   Evidence: BundleCommitment + BundleReveal showing hash mismatch.
   Verification: recompute SHA256(reveal_preimage) != commitment_hash
   Penalty: 100% solver bond slash

2. FEE_VIOLATION
   Solver charged more than intent.max_fee.
   Evidence: receipt showing actual_fee_bps > intent.max_fee
   Verification: check receipt.fee_paid_bps > intent.max_fee
   Penalty: 5% solver bond slash + refund excess fee to user

3. MIN_OUTPUT_VIOLATION
   User received less than intent.min_amount_out.
   Evidence: receipt showing amount_out < min_amount_out
   Verification: check receipt.amount_out < intent.min_amount_out
   Penalty: 10% solver bond slash + make user whole

4. DOUBLE_FILL
   Same intent filled by two different bundles in same or different batches.
   Evidence: two receipts with same intent_id, both Succeeded
   Verification: query receipts by intent_id, check count > 1
   Penalty: 100% solver bond slash (both solvers if colluding)

5. INVALID_ROUTE
   Solver used a pool that doesn't exist or routed through invalid intermediary.
   Evidence: execution step referencing non-existent ObjectId
   Verification: check ObjectStore for referenced objects
   Penalty: 25% solver bond slash

6. STATE_CORRUPTION
   Post-execution state root is incorrect.
   Evidence: Merkle proof showing state_root_after doesn't match actual state
   Verification: recompute state root from objects_written
   Penalty: depends on cause — solver slash or validator slash

7. CONSERVATION_VIOLATION
   Assets were created or destroyed (total supply changed incorrectly).
   Evidence: sum of debits != sum of credits for an asset
   Verification: replay execution trace, check conservation
   Penalty: 50% solver bond slash
```

### 11.4 Dispute Resolution Process

```
1. SUBMISSION
   Challenger submits FraudProof with bond
   Dispute enters PENDING state

2. VALIDATION (automatic, within same block)
   For fast-window disputes:
     - Verify proof format
     - Verify challenger has sufficient bond
     - Verify receipt exists and is within dispute window
     - Verify evidence is well-formed

3. VERIFICATION
   For auto-verifiable proofs (types 1-5):
     - On-chain contract recomputes and checks
     - If proof valid: dispute ACCEPTED
     - If proof invalid: dispute REJECTED, challenger bond slashed

   For complex proofs (types 6-7):
     - Enters governance queue
     - Committee reviews evidence
     - Vote with 2/3 quorum required

4. RESOLUTION
   If ACCEPTED:
     - Solver bond slashed (amount depends on proof type)
     - Challenger receives reward (portion of slashed bond)
     - Affected intent returns to OPEN state (if possible)
     - User made whole from slashed funds (if applicable)
     - Receipt updated: execution_status = Disputed

   If REJECTED:
     - Challenger bond slashed (FRIVOLOUS_DISPUTE_PENALTY = 50%)
     - No changes to solver or receipt

5. APPEAL (extended window only)
   - Losing party can appeal within 3 days
   - Requires 2x the original bond
   - Full governance vote required
```

### 11.5 Dispute Incentives

```
CHALLENGER_REWARD = 30% of slashed solver bond
PROTOCOL_CUT     = 20% of slashed solver bond
USER_REFUND      = 50% of slashed solver bond (if user was harmed)

If no user harm (e.g., invalid reveal that didn't affect output):
  CHALLENGER_REWARD = 50% of slashed solver bond
  PROTOCOL_CUT     = 50% of slashed solver bond
```

---

## Section 12 — Slashing & Rewards

### 12.1 Solver Penalties

**Minor Violations**

| Violation | Penalty | Reputation Impact |
|-----------|---------|-------------------|
| Commit without reveal | 1% bond slash | violation_score +100 |
| Spam bundles (> MAX_COMMITMENTS_PER_WINDOW) | 0.5% bond slash | violation_score +50 |
| Low-quality submission (predicted far from actual) | No slash | performance_score -200 |
| Late reveal (within grace period) | No slash | latency_score -100 |

**Major Violations**

| Violation | Penalty | Reputation Impact |
|-----------|---------|-------------------|
| Invalid settlement (constraints violated) | 10-25% bond slash | violation_score +2000 |
| Fraudulent route (fake pools/sources) | 50% bond slash | violation_score +5000, deactivation |
| Double fill (colluding or negligent) | 100% bond slash | permanent ban |
| Invalid reveal (hash mismatch) | 100% bond slash | permanent ban |

**Escalation**

```
violation_score thresholds:
  0-2000:      No action (normal operation)
  2001-5000:   Warning (reduced priority in auctions)
  5001-8000:   Probation (limited to 3 commits per window)
  8001-9500:   Suspension (cannot participate for 1 epoch)
  9501-10000:  Auto-deactivation (must re-register and re-bond)
```

### 12.2 Validator Penalties

Validators (PoSeq committee members) are penalized for consensus faults:

| Fault | Penalty | Recovery |
|-------|---------|----------|
| Equivocation | 100% slash + permanent ban | None |
| Unauthorized proposal | 50% slash + jail 10 epochs | Unjail after jail period |
| Unfair sequencing | 50% slash + jail 10 epochs | Unjail after jail period |
| Censorship | 25% slash + jail 5 epochs | Unjail after jail period |
| Liveness failure | 1% slash per view + jail | Unjail after 1 epoch |
| Invalid proposal | 5% slash + jail 1 epoch | Unjail after jail period |

### 12.3 Reward Distribution

Revenue sources:
```
1. Protocol fees from successful settlements
2. Solver tips from intent submitters
3. Slashed bonds from misbehaving participants
```

Distribution per epoch:

```
TOTAL_EPOCH_REVENUE = sum(protocol_fees) + sum(tips) + sum(slashed_bonds * PROTOCOL_CUT)

Distribution:
  50% → Validator rewards (proportional to stake + uptime)
  30% → Solver rewards (proportional to fills + quality)
  10% → Treasury (protocol development fund)
  10% → Insurance fund (covers user losses from bugs)

Validator reward per validator:
  base_reward = TOTAL_EPOCH_REVENUE * 0.50 / active_validator_count
  uptime_multiplier = blocks_signed / total_blocks  // 0.0 to 1.0
  validator_reward = base_reward * uptime_multiplier

Solver reward per solver:
  fill_share = solver.total_fills / sum(all_solver_fills)
  quality_bonus = solver.performance_score / 10000
  solver_reward = TOTAL_EPOCH_REVENUE * 0.30 * fill_share * (1 + quality_bonus * 0.5)
```

---

## Section 13 — Data Availability

### 13.1 What Must Be Available

For any batch to be considered valid, the following data must be retrievable by any full node:

```
1. BUNDLE COMMITMENTS
   All BundleCommitments for the batch window
   Required for: verifying commit-reveal integrity
   Stored: on each PoSeq validator node during commit phase
   Availability: gossipped to all validators, stored in commitment pool

2. BUNDLE REVEALS
   All BundleReveals for committed bundles
   Required for: verifying execution plans, replaying settlements
   Stored: on each PoSeq validator node during reveal phase
   Availability: included in SequencedBatch payload

3. SEQUENCE METADATA
   SequencedBatch including ordering, fairness_meta, sequence_root
   Required for: verifying ordering correctness and fairness
   Stored: in HotStuff block payload
   Availability: all validators store full blocks

4. EXECUTION PROOFS
   ExecutionReceipts with state roots and object access lists
   Required for: dispute resolution, auditing
   Stored: on-chain in x/poseq store
   Availability: queryable via chain RPC

5. INTENT DATA
   Full IntentTransaction for each filled intent
   Required for: verifying constraint satisfaction
   Stored: intent pool (ephemeral) + receipt references (permanent)
   Availability: archived by full nodes, referenced by intent_id in receipts
```

### 13.2 DA Validation Rules

Before a validator votes on a HotStuff block, they must verify data availability:

```
DA_CHECK_1: PAYLOAD COMPLETE
  The block's SequencedBatch payload is fully present (not just a hash).
  If a validator only received a payload hash, they must request the full
  payload before voting. If they cannot retrieve it within DA_TIMEOUT (5s),
  they do NOT vote.

DA_CHECK_2: COMMITMENTS ACCESSIBLE
  For each bundle in the batch, the corresponding BundleCommitment
  is either:
  a. In the validator's local commitment pool, OR
  b. Retrievable from >= f+1 peers within DA_TIMEOUT

  If any commitment is unavailable, the validator does NOT vote.

DA_CHECK_3: INTENT REFERENCES VALID
  For each intent_id referenced by a bundle, the validator can verify
  the intent exists (either in local pool or by querying peers).
  Intent data itself is not required in the block (referenced by hash).

DA_CHECK_4: HISTORICAL AVAILABILITY
  After block finalization, the full block data (including SequencedBatch,
  all bundles, and all commitments) must be stored by >= 2f+1 validators
  for at least ARCHIVE_RETENTION_EPOCHS (e.g., 100 epochs).
```

### 13.3 DA Failure Handling

```
If a proposed block fails DA checks for a validator:
  1. Validator does NOT vote (treated as abstention, not fault)
  2. If < 2f+1 validators can verify DA, block does not finalize
  3. Pacemaker advances to next view
  4. New leader proposes (may include same batch if DA issue was transient)

If DA degrades across multiple views:
  5. After DA_FAILURE_THRESHOLD (3 consecutive views):
     Validators flag potential leader censorship
  6. Leader rotation occurs
  7. If DA persists across leaders: epoch transition forced
```

---

## Section 14 — Storage Model

### 14.1 On-Chain Storage Layout

All on-chain state is stored in the Cosmos SDK KV store under the `poseq` module. Key prefixes are organized by data type:

```
PREFIX  DESCRIPTION                          KEY STRUCTURE                              VALUE
──────  ─────────────────────────────────────  ─────────────────────────────────────────  ──────────────
0x01    EvidencePacket                       0x01 ‖ packet_hash[32]                     EvidencePacket
0x02    GovernanceEscalationRecord           0x02 ‖ escalation_id[32]                   GovernanceEscalationRecord
0x03    CheckpointAnchorRecord               0x03 ‖ epoch[8] ‖ slot[8]                 CheckpointAnchorRecord
0x04    EpochStateReference                  0x04 ‖ epoch[8]                            EpochStateReference
0x05    CommitteeSuspensionRecommendation    0x05 ‖ node_id[N]                          CommitteeSuspensionRecommendation
0x06    ExportBatch                          0x06 ‖ epoch[8]                            ExportBatch
0x07    SequencerRecord                      0x07 ‖ node_id[32]                         SequencerRecord
0x08    CommittedBatch                       0x08 ‖ batch_id[32]                        CommittedBatchRecord
0x09    SlashRecord                          0x09 ‖ node_id[32] ‖ packet_hash[32]      SlashRecord

--- NEW (Intent-Based Execution) ---

0x10    ExecutionReceipt                     0x10 ‖ receipt_id[32]                      ExecutionReceipt
0x11    Receipt-by-Intent index              0x11 ‖ intent_id[32] ‖ receipt_id[32]     (empty value, presence key)
0x12    Receipt-by-Solver index              0x12 ‖ solver_id[32] ‖ epoch[8] ‖ receipt_id[32]  (empty value)
0x13    Receipt-by-Batch index               0x13 ‖ batch_id[32] ‖ sequence_slot[8]    receipt_id[32]

0x20    IntentRecord                         0x20 ‖ intent_id[32]                      IntentRecord
0x21    Intent-by-User index                 0x21 ‖ user[32] ‖ nonce[8]               intent_id[32]
0x22    Intent-by-Status index               0x22 ‖ status[1] ‖ deadline[8] ‖ intent_id[32]  (empty value)

0x30    SolverProfile                        0x30 ‖ solver_id[32]                      SolverProfile
0x31    Solver-by-Operator index             0x31 ‖ operator_address_bytes ‖ solver_id[32]   (empty value)
0x32    SolverUnbonding                      0x32 ‖ completion_block[8] ‖ solver_id[32]  UnbondingEntry

0x40    BundleCommitment                     0x40 ‖ batch_window[8] ‖ bundle_id[32]    BundleCommitment
0x41    BundleReveal                         0x41 ‖ batch_window[8] ‖ bundle_id[32]    BundleReveal
0x42    AuctionResult                        0x42 ‖ batch_window[8]                     AuctionResult

0x50    SequenceCommitment                   0x50 ‖ batch_id[32]                       SequenceCommitment
0x51    Batch-by-Epoch index                 0x51 ‖ epoch[8] ‖ slot[8]                batch_id[32]

0x60    DisputeRecord                        0x60 ‖ proof_id[32]                       DisputeRecord
0x61    Dispute-by-Receipt index             0x61 ‖ receipt_id[32] ‖ proof_id[32]     (empty value)
0x62    Dispute-by-Challenger index          0x62 ‖ challenger[32] ‖ proof_id[32]     (empty value)

0x70    Params                               0x70                                       Params
0x71    EpochRewardRecord                    0x71 ‖ epoch[8]                            EpochRewardRecord
```

### 14.2 Key Encoding Rules

```
Integers:  BigEndian encoding (uint64 → 8 bytes, uint128 → 16 bytes)
Hashes:    Raw 32 bytes (no hex encoding on-chain)
Addresses: Raw bytes of Cosmos address (varies, typically 20 bytes)
Status:    Single byte enum (0x00=Created, 0x01=Admitted, ... 0x0A=Slashed)

Range iteration:
  All keys with prefix P and varying suffix:
    start_key = P
    end_key   = PrefixEndBytes(P)

  All intents for a user:
    start_key = 0x21 ‖ user[32]
    end_key   = PrefixEndBytes(0x21 ‖ user[32])
```

### 14.3 Storage Size Estimates

```
Per intent:     ~512 bytes (IntentRecord) + ~64 bytes (2 index entries)
Per bundle:     ~2048 bytes (commitment + reveal)
Per receipt:    ~1024 bytes (receipt) + ~96 bytes (3 index entries)
Per solver:     ~512 bytes (profile) + ~32 bytes (index)
Per dispute:    ~512 bytes (record) + ~64 bytes (2 index entries)
Per batch:      ~256 bytes (SequenceCommitment) + ~32 bytes (index)

At 100 intents/block, 12s blocks:
  ~720 intents/hour → ~17,280 intents/day
  Storage: ~10 MB/day (intents + receipts + indexes)
  Archival: old receipts can be pruned after ARCHIVE_RETENTION_EPOCHS
```

---

## Section 15 — Phase 1 Implementation Scope

### 15.1 What's Included

Phase 1 is the minimum viable protocol that demonstrates the full pipeline:

```
INTENT LAYER
  ✓ SwapIntent
  ✓ PaymentIntent
  ✓ RouteLiquidityIntent
  ✓ Public intent pool with gossip
  ✓ Intent lifecycle (Created → Settled)
  ✓ Nonce-based replay protection
  ✓ Deadline-based expiry
  ✓ Signature validation (Ed25519)

SOLVER MARKET
  ✓ Solver registration and bonding
  ✓ Solver capability declaration
  ✓ Solver reputation tracking
  ✓ Commit-reveal auction per batch window
  ✓ Bundle commitment and reveal validation
  ✓ Commitment hash verification
  ✓ Commit-without-reveal penalties

POSEQ SEQUENCING
  ✓ Bundle intake from reveal phase
  ✓ Deterministic ordering (quality → tiebreak)
  ✓ Forced-inclusion for censorship resistance
  ✓ Sequence root computation
  ✓ Fairness metadata
  ✓ Integration with existing PoSeq pipeline

HOTSTUFF BFT
  ✓ Block proposal with SequencedBatch payload
  ✓ 5-point validator verification
  ✓ Pipelined 3-phase consensus
  ✓ QuorumCertificate with 2f+1 signatures
  ✓ FinalizationEnvelope emission

SETTLEMENT RUNTIME
  ✓ Object-based state model (Balance, Pool, Vault)
  ✓ Execution step replay with read/write sets
  ✓ Conflict detection and parallel scheduling
  ✓ SwapIntent verification
  ✓ PaymentIntent verification
  ✓ RouteLiquidityIntent verification
  ✓ Gas metering

RECEIPTS
  ✓ Full ExecutionReceipt with state proofs
  ✓ Receipt indexing (by intent, solver, batch)
  ✓ Solver performance tracking from receipts

DISPUTES
  ✓ Fast dispute window (100 blocks)
  ✓ Extended dispute window (7 days)
  ✓ Auto-verifiable fraud proofs (types 1-5)
  ✓ Governance-resolved complex disputes (types 6-7)
  ✓ Challenger rewards

SLASHING
  ✓ Solver minor/major violation penalties
  ✓ Validator consensus fault penalties
  ✓ Escalating violation scores
  ✓ Auto-deactivation at threshold

DATA AVAILABILITY
  ✓ Payload completeness checks
  ✓ Commitment accessibility verification
  ✓ DA failure handling (view skip)

STORAGE
  ✓ Full on-chain storage layout (prefixes 0x01-0x71)
  ✓ Index keys for efficient queries
```

### 15.2 What's Excluded (Future Phases)

```
Phase 2:
  ✗ Cross-chain intents
  ✗ Protected intent pool (threshold-encrypted)
  ✗ Private orderflow auctions (PBS-style)
  ✗ Proof-of-stake intent submission gating

Phase 3:
  ✗ AI/ML solver agents
  ✗ Batch auctions (sealed-bid multi-intent)
  ✗ Solver delegation and sub-contracting
  ✗ Intent composition (chain intents together)

Phase 4:
  ✗ ZK-proof based settlement verification
  ✗ Cross-chain atomic settlement
  ✗ Solver insurance pools
  ✗ Dynamic fee markets for intent priority
```

### 15.3 Implementation Order

```
Step 1: Intent types and pool
  - Define IntentTransaction, SwapIntent, PaymentIntent, RouteLiquidityIntent
  - Implement intent pool with validation, gossip, expiry
  - Add MsgSubmitIntent to chain module

Step 2: Solver registration and profiles
  - Define SolverProfile, MsgRegisterSolver
  - Implement bonding and unbonding
  - Add solver store operations

Step 3: Commit-reveal auction
  - Define BundleCommitment, BundleReveal
  - Implement batch window timing (commit → reveal → select)
  - Implement commitment validation and reveal verification
  - Implement commit-without-reveal penalty

Step 4: PoSeq integration
  - Extend PoSeqNode to accept BundleReveals as submissions
  - Implement intent-aware ordering (Section 6.3)
  - Produce SequencedBatch with bundle data

Step 5: Settlement verification
  - Extend runtime to replay solver execution plans
  - Implement constraint verification per intent type
  - Produce ExecutionReceipts

Step 6: Dispute system
  - Implement fraud proof types
  - Implement dispute windows and resolution
  - Implement challenger rewards

Step 7: Rewards and analytics
  - Implement epoch reward distribution
  - Implement solver reputation updates
  - Implement receipt-based analytics queries
```

### 15.4 Existing Infrastructure Leveraged

The following already-implemented components are directly leveraged:

| Component | Location | How It's Used |
|-----------|----------|---------------|
| PoSeq pipeline | `poseq/src/` (50+ modules) | Ordering, batching, fairness, anti-MEV |
| HotStuff BFT | `poseq/src/hotstuff/` | Finality consensus |
| Networking | `poseq/src/networking/` | Peer gossip, message signing |
| Object store | `runtime/src/objects/`, `runtime/src/state/` | Object-based state model |
| Solver market | `runtime/src/solver_market/` | CandidatePlan, PlanAction types |
| Solver registry | `runtime/src/solver_registry/` | SolverProfile, reputation |
| Settlement engine | `runtime/src/settlement/` | ExecutionReceipt, state root computation |
| Parallel scheduler | `runtime/src/scheduler/` | Conflict detection, execution groups |
| Intent resolver | `runtime/src/resolution/` | Intent → ExecutionPlan conversion |
| Plan validation | `runtime/src/plan_validation/` | Solver plan verification |
| Chain bridge | `poseq/src/chain_bridge/` | Evidence export, governance escalation |
| x/poseq module | `chain/x/poseq/` | On-chain storage, governance integration |
| Runtime bridge | `poseq/src/bridge/` | FinalizationEnvelope delivery |
| CRX layer | `runtime/src/crx/` | Deterministic rights execution |
| Safety kernel | `runtime/src/safety/` | Safety rule enforcement |

---

## Appendix A — Constants

```
// Timing
COMMIT_PHASE_BLOCKS         = 5
REVEAL_PHASE_BLOCKS         = 3
BATCH_WINDOW_BLOCKS         = 9     // COMMIT + REVEAL + 1
MIN_INTENT_LIFETIME         = 10    // blocks
MAX_POOL_RESIDENCE          = 1000  // blocks
FAST_DISPUTE_WINDOW         = 100   // blocks
EXTENDED_DISPUTE_WINDOW     = 50400 // blocks (~7 days at 12s)
DA_TIMEOUT_MS               = 5000
DA_FAILURE_THRESHOLD        = 3     // consecutive views
ARCHIVE_RETENTION_EPOCHS    = 100
UNBONDING_PERIOD_BLOCKS     = 50400 // ~7 days

// Economic
MIN_INTENT_FEE              = 10    // basis points
MIN_SOLVER_BOND             = 10_000 OMNI
ACTIVE_SOLVER_BOND          = 50_000 OMNI
FAST_DISPUTE_BOND           = 1_000  OMNI
EXTENDED_DISPUTE_BOND       = 10_000 OMNI
COMMIT_WITHOUT_REVEAL_BPS   = 100   // 1% of locked bond
FRIVOLOUS_DISPUTE_PENALTY   = 5000  // 50% of dispute bond
CHALLENGER_REWARD_PCT       = 30
PROTOCOL_CUT_PCT            = 20
USER_REFUND_PCT             = 50

// Limits
MAX_INTENTS_PER_BLOCK       = 10    // per user
MAX_NONCE_GAP               = 3
MAX_INTENT_SIZE             = 4096  // bytes
MAX_POOL_SIZE               = 50_000
MAX_COMMITMENTS_PER_WINDOW  = 10    // per solver
MAX_VIOLATION_SCORE         = 9500  // auto-deactivation threshold
MAX_BUNDLE_STEPS            = 64    // execution steps per bundle
MAX_READ_SET_SIZE           = 32    // objects per step
MAX_WRITE_SET_SIZE          = 16    // objects per step

// Reputation
EMA_ALPHA                   = 0.125 // 2/(N+1), N=15
PERFORMANCE_SCORE_INIT      = 5000
VIOLATION_SCORE_INIT        = 0
LATENCY_SCORE_INIT          = 5000
```

## Appendix B — Message Types Summary

```
// Intent Layer
MsgSubmitIntent             { intent: IntentTransaction }
MsgCancelIntent             { intent_id, user, signature }

// Solver Market
MsgRegisterSolver           { solver_id, moniker, operator, bond, types, signature }
MsgDeactivateSolver         { solver_id, operator, signature }
MsgIncreaseSolverBond       { solver_id, amount }
MsgDecreaseSolverBond       { solver_id, amount }

// Commit-Reveal Auction
MsgSubmitBundleCommitment   { commitment: BundleCommitment }
MsgSubmitBundleReveal       { reveal: BundleReveal }

// Settlement
MsgCommitExecution          { batch_id, finalization_hash, solver_id, approvals, committee_size }

// Disputes
MsgSubmitFraudProof         { proof: FraudProof }
MsgAppealDispute            { proof_id, appellant, new_bond, evidence, signature }

// Slashing
MsgExecuteSlash             { node_id, evidence_packet_hash, slash_bps, reason, authority }

// Governance
MsgUpdateParams             { authority, params }
```

## Appendix C — Error Codes

```
// Intent errors (100-119)
100  ErrInvalidIntentSchema
101  ErrInvalidIntentSignature
102  ErrIntentExpired
103  ErrIntentNonceUsed
104  ErrIntentNonceGap
105  ErrIntentPoolFull
106  ErrIntentNotFound
107  ErrIntentNotCancellable
108  ErrInvalidAsset

// Solver errors (120-139)
120  ErrSolverAlreadyRegistered
121  ErrSolverNotFound
122  ErrSolverInactive
123  ErrInsufficientBond
124  ErrUnbondingInProgress
125  ErrMaxViolationsExceeded
126  ErrSolverBlacklisted

// Auction errors (140-159)
140  ErrCommitPhaseEnded
141  ErrRevealPhaseNotStarted
142  ErrRevealPhaseClosed
143  ErrNoMatchingCommitment
144  ErrCommitmentHashMismatch
145  ErrOutputsHashMismatch
146  ErrPlanHashMismatch
147  ErrMaxCommitmentsExceeded
148  ErrInvalidBundleSignature

// Settlement errors (160-179)
160  ErrConstraintViolation
161  ErrMinOutputNotMet
162  ErrFeeExceeded
163  ErrInvalidRecipient
164  ErrStateConflict
165  ErrGasExceeded
166  ErrObjectNotFound
167  ErrObjectVersionMismatch
168  ErrConservationViolation
169  ErrInvalidRoute

// Dispute errors (180-199)
180  ErrDisputeWindowClosed
181  ErrInsufficientDisputeBond
182  ErrInvalidFraudProof
183  ErrDisputeAlreadyExists
184  ErrDisputeNotFound
185  ErrAppealWindowClosed
186  ErrNotAppealable
```
