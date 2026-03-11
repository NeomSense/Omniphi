# Phase 3: Causal Rights Execution (CRX)

## Overview

The Causal Rights Execution (CRX) module is the Phase 3 extension to the Omniphi Object + Intent Runtime. It provides a principled framework for executing intent-driven goal packets with formal rights validation, causal graph construction, branch-aware execution, and deterministic finality classification.

CRX sits above the Phase 2 Solver Market and provides:
- **GoalPackets**: A structured expression of a user's desired outcome with policy envelopes
- **RightsCapsules**: Fine-grained authorization scopes synthesized from causal graphs
- **CausalGraphs**: Deterministic DAGs representing the execution plan's data dependencies
- **Rights Validation**: Per-node authorization checking against a RightsCapsule
- **Causal Validity**: Structural correctness verification of the execution DAG
- **Branch-Aware Execution**: Policy-driven partial failure handling
- **Finality Classification**: Settlement outcome classification with domain policy escalation
- **Simulation**: Dry-run execution with full result preview

---

## Module Structure

```
src/crx/
├── mod.rs               — Module declarations and re-exports
├── goal_packet.rs       — GoalPacket, GoalConstraints, GoalPolicyEnvelope
├── rights_capsule.rs    — RightsCapsule, AllowedActionSet, RightsScope
├── causal_graph.rs      — CausalGraph, CausalNode, CausalEdge, NodeId
├── plan_builder.rs      — CausalPlanBuilder, GraphAssembler, RightsSynthesizer
├── rights_validation.rs — RightsValidationEngine
├── causal_validity.rs   — CausalValidityEngine, CausalAuditRecord
├── branch_execution.rs  — BranchAwareExecutor
├── finality.rs          — FinalityClassifier, DomainFinalityPolicy
├── settlement.rs        — CRXSettlementEngine, CRXSettlementRecord
├── simulation.rs        — CRXSimulator, GraphExplainabilityRecord
└── poseq_bridge.rs      — PoSeqCRXBridge, OrderedGoalBatch
```

---

## Key Design Principles

### 1. Determinism
All operations are deterministic: same inputs always produce the same outputs.
- `BTreeMap`/`BTreeSet` everywhere (no `HashMap`)
- Kahn's algorithm with `BTreeSet` queue for topological sort
- SHA256 of bincode serialization for all hashes

### 2. No Floating Point
All amounts, scores, and fees are `u64`/`u128`/`i128`. No `f64` or `f32`.

### 3. Zero-Panic Guarantee
No `unwrap()` or `expect()` in production code paths that could fail at runtime.

### 4. Snapshot-Based Revert
Before execution, the settlement engine snapshots balance objects. If finality class is `Reverted`, the snapshot is restored.

---

## Lifecycle Pipeline

```
GoalPacket
    │
    ▼
CausalPlanBuilder::build(plan, goal, store, epoch)
    ├──► GraphAssembler::assemble(plan, goal) → CausalGraph
    └──► RightsSynthesizer::synthesize(graph, goal, solver_id, epoch) → RightsCapsule
    │
    ▼
RightsValidationEngine::validate(graph, capsule, store, epoch)
    │  → RightsValidationResult { all_passed, violations, total_spend }
    │
    ▼
CausalValidityEngine::validate(graph, capsule, goal)
    │  → CausalValidityResult { is_causally_valid, violations, audit }
    │
    ▼ (if both pass)
BranchAwareExecutor::execute(graph, capsule, goal, store)
    │  → (ExecutionSettlementClass, Vec<BranchExecutionResult>)
    │
    ▼
FinalityClassifier::classify(settlement_class, goal, domain_policies)
    │  → SettlementDisposition { finality_class, escalation_required }
    │
    ▼
CRXSettlementRecord
```

---

## GoalPacket

A `GoalPacket` is the top-level expression of user intent for CRX:

```rust
struct GoalPacket {
    packet_id:       [u8; 32],
    intent_id:       [u8; 32],
    sender:          [u8; 32],
    desired_outcome: String,
    constraints:     GoalConstraints,
    policy:          GoalPolicyEnvelope,
    max_fee:         u64,
    deadline_epoch:  u64,
    nonce:           u64,
    metadata:        BTreeMap<String, String>,
}
```

Validation rules:
- `packet_id` must be non-zero
- `max_fee > 0`
- `deadline_epoch > 0`
- At least one `allowed_object_type` or `allowed_domain`

### PartialFailurePolicy

| Policy | Behavior on branch failure |
|---|---|
| `StrictAllOrNothing` | Revert entire execution |
| `AllowBranchDowngrade` | Downgrade (if capsule allows); continue |
| `QuarantineOnFailure` | Quarantine affected objects; continue |
| `DowngradeAndContinue` | Downgrade and proceed with rest |

---

## RightsCapsule

Synthesized from a `CausalGraph` + `GoalPacket`, a `RightsCapsule` is the authorization token for a single plan execution:

```rust
struct RightsCapsule {
    capsule_id:            [u8; 32],
    goal_packet_id:        [u8; 32],
    authorized_solver_id:  [u8; 32],
    scope:                 RightsScope,
    valid_from_epoch:      u64,
    valid_until_epoch:     u64,
    capsule_hash:          [u8; 32],
    metadata:              BTreeMap<String, String>,
}
```

The `RightsScope` constrains:
- Which object IDs/types may be accessed
- Which actions may be performed
- Which domains are allowed/forbidden
- Cumulative spend limit
- Maximum number of objects touched

### Preset Action Sets

| Set | Actions |
|---|---|
| `full()` | All actions |
| `read_only()` | `Read` |
| `transfer_only()` | `Read, DebitBalance, CreditBalance, UpdateVersion, EmitReceipt` |
| `swap_only()` | `Read, DebitBalance, CreditBalance, SwapPoolAmounts, UpdateVersion, EmitReceipt` |

---

## CausalGraph

A `CausalGraph` is a directed acyclic graph (DAG) representing the data dependencies in an execution plan:

```rust
struct CausalGraph {
    graph_id:          [u8; 32],
    goal_packet_id:    [u8; 32],
    nodes:             BTreeMap<NodeId, CausalNode>,
    edges:             Vec<CausalEdge>,
    branches:          Vec<ExecutionBranch>,
    topological_order: Vec<NodeId>,
    graph_hash:        [u8; 32],
}
```

### Node Execution Classes

| Class | Description |
|---|---|
| `ReadObject` | Verify existence |
| `CheckBalance` | Validate available balance |
| `CheckPolicy` | Policy pre-check |
| `MutateBalance` | Debit or credit balance |
| `SwapPoolAmounts` | Apply pool reserve deltas |
| `LockObject` / `UnlockObject` | Adjust locked_amount |
| `FinalizeSettlement` | Increment object versions |
| `BranchGate` | Conditional node for branch control |
| `EmitReceipt` | Extension point |
| `InvokeSafetyHook` | Extension point |

### Topological Sort

Kahn's algorithm with `BTreeSet` queue ensures stable, deterministic ordering. If `validate_acyclic()` produces a shorter order than the number of nodes, a cycle exists.

### Action → Node Mapping

| PlanAction | Nodes Created |
|---|---|
| `DebitBalance` | `CheckBalance` + `MutateBalance(debit)` with `PolicyDependent` edge |
| `CreditBalance` | `MutateBalance(credit)` with `StateDependent` edge from prior debit |
| `SwapPoolAmounts` | `CheckBalance(reserves)` + `SwapPoolAmounts` with `StateDependent` edge |
| `LockBalance` | `LockObject` |
| `UnlockBalance` | `UnlockObject` |
| `Custom("update_version")` | `FinalizeSettlement` |

A root `FinalizeSettlement` node is always appended at the end with `OrderingOnly` edges from all mutation nodes.

---

## Rights Validation

`RightsValidationEngine::validate` checks each node in topological order:

1. Capsule epoch validity (`is_valid_at_epoch`)
2. Action authorization (`can_perform_action`)
3. Object scope (`can_access_object` — checks both `allowed_object_ids` and `allowed_object_types`)
4. Domain authorization (`domain_envelope.is_domain_allowed`)
5. Spend limit (cumulative debit vs `max_spend`)
6. Object count (`objects_touched` vs `max_objects_touched`)

If the capsule is expired, all nodes fail with `CapsuleExpired` immediately.

### ScopeBreachReason Values

- `ObjectNotInScope` — object type/id not in capsule scope
- `ActionNotAllowed` — action not in `allowed_actions`
- `DomainForbidden` — domain in `forbidden_domains`
- `SpendLimitExceeded` — cumulative debit exceeds `max_spend`
- `CapsuleExpired` — epoch outside `[valid_from, valid_until]`
- `SolverMismatch` — (reserved)
- `ObjectCountExceeded` — too many write objects touched

---

## Causal Validity

`CausalValidityEngine::validate` checks:

1. **Acyclicity** — `validate_acyclic()` via Kahn's algorithm
2. **Dependency ordering** — critical edges must be satisfied in topological order
3. **Branch bypass** — write nodes cannot precede their branch's `BranchGate`
4. **Forbidden domains** — no node may access a domain in `goal.constraints.forbidden_domains`
5. **Validation skipping** — debit `MutateBalance` and `SwapPoolAmounts` nodes must have a preceding `CheckBalance`/`CheckPolicy` node via a `PolicyDependent` edge

Each check produces typed `CausalViolation` variants for audit trail purposes.

### CausalAuditRecord

A `CausalAuditRecord` is always generated, even for valid graphs. Its `audit_hash` is the SHA256 of `(graph_id, total_nodes, violation_count, flags)` and can be committed on-chain for audit trail integrity.

---

## Branch-Aware Execution

`BranchAwareExecutor::execute` executes each branch in topological order of its nodes.

On node failure:

| PartialFailurePolicy | capsule allows | BranchFailureMode |
|---|---|---|
| `StrictAllOrNothing` | any | `Revert` → returns `FullRevert` immediately |
| `AllowBranchDowngrade` | `allow_downgrade=true` | `Downgrade` |
| `AllowBranchDowngrade` | `allow_downgrade=false` | `Revert` |
| `QuarantineOnFailure` | `quarantine_eligible=true` | `Quarantine` |
| `QuarantineOnFailure` | `quarantine_eligible=false` | `Revert` |
| `DowngradeAndContinue` | `allow_downgrade=true` | `Downgrade` |

### Node Execution Semantics

- **`ReadObject`**: `store.get(id)?`
- **`CheckBalance/CheckPolicy`**: verify `available >= node.amount`
- **`MutateBalance(debit)`**: `amount - locked >= requested`; subtract from `amount`
- **`MutateBalance(credit)`**: `amount += node.amount` (saturating)
- **`SwapPoolAmounts`**: apply `delta_a`/`delta_b` from node metadata to pool reserves
- **`LockObject`**: `locked_amount += amount` (after checking available)
- **`UnlockObject`**: `locked_amount -= amount` (saturating)
- **`FinalizeSettlement`**: increment `meta.version` on target object
- **`BranchGate`, `EmitReceipt`, `InvokeSafetyHook`**: no-op (extension points)

---

## Finality Classification

`FinalityClassifier::classify` maps `ExecutionSettlementClass` + policy to `FinalityClass`:

| ExecutionSettlementClass | Policy | FinalityClass |
|---|---|---|
| `FullSuccess` | any | `Finalized` |
| `SuccessWithDowngrade` | `AllowBranchDowngrade` | `FinalizedWithDowngrade` |
| `SuccessWithDowngrade` | `Strict` | `Reverted` |
| `SuccessWithQuarantine` | `QuarantineOnFailure` | `FinalizedWithQuarantine` |
| `SuccessWithQuarantine` | `Strict` | `Reverted` |
| `PartialSuccess` | `AllowBranchDowngrade` | `FinalizedWithDowngrade` |
| `PartialSuccess` | `Strict` | `Reverted` |
| `FullRevert` | any | `Reverted` |
| `Rejected` | any | `Rejected` |

If `SettlementStrictness::Provisional`, any non-Rejected/non-Reverted outcome is overridden to `Provisional`.

Domain policies can trigger `escalation_required = true` if the achieved finality rank is below the domain's minimum.

**Finality rank** (higher = better): `Finalized(5) > FinalizedWithDowngrade(4) > FinalizedWithQuarantine(3) > Provisional(2) > Reverted(1) > Rejected(0)`.

---

## Settlement Engine

`CRXSettlementEngine::settle` is the top-level pipeline:

```
1. CausalPlanBuilder::build(plan, goal, store, epoch)
2. RightsValidationEngine::validate(graph, capsule, store, epoch)
3. CausalValidityEngine::validate(graph, capsule, goal)
4. If validation fails → return Rejected record (no store mutation)
5. Snapshot balance objects for possible revert
6. BranchAwareExecutor::execute(graph, capsule, goal, store)
7. store.sync_typed_to_canonical()
8. FinalityClassifier::classify(settlement_class, goal, domain_policies)
9. If Reverted → restore snapshot, re-sync
10. Collect version_transitions, build CRXSettlementRecord
```

The `CRXSettlementRecord` contains:
- `capsule_receipt` — capsule_id, capsule_hash, scope_summary, was_respected
- `graph_receipt` — graph_id, graph_hash, node counts, branch_results
- `causal_summary` — causal/rights validity flags, violations_count, audit_hash
- `affected_objects` — object IDs that were mutated
- `version_transitions` — `(object_id, old_version, new_version)` tuples
- `gas_used` — estimated gas cost

---

## Simulation

`CRXSimulator::simulate` runs the full pipeline on a **clone** of the store, discarding all mutations:

```rust
let result = CRXSimulator::simulate(&plan, &goal, &store, &domain_policies, epoch);
// store is unchanged
```

Returns `CRXSimulationResult` including:
- `would_pass_rights_validation` / `would_pass_causal_validation`
- `predicted_finality` / `predicted_settlement_class`
- `preview_graph_hash` / `preview_capsule_hash`
- `preview_balance_changes: Vec<([u8; 32], i128)>` — net delta per object
- `rights_violations` / `causal_violations`
- `estimated_gas`

Additional helpers:
- `CRXSimulator::inspect_rights(capsule)` → `RightsInspectionView`
- `CRXSimulator::explain_graph(graph)` → `GraphExplainabilityRecord` with critical path

---

## PoSeq Bridge

`PoSeqCRXBridge` integrates CRX with the PoSeq ordered batch pipeline:

```rust
let mut bridge = PoSeqCRXBridge::new(SelectionPolicy::BestScore, Box::new(PermissivePolicy));
bridge.register_solver(profile)?;
bridge.seed_object(Box::new(balance_object));

let results = bridge.process_batch(OrderedGoalBatch { ... });
```

For each goal in canonical order:
1. Validate goal
2. Build fake `IntentTransaction` from `GoalPacket` for solver matching
3. Validate all candidate plans via `PlanValidator`
4. Select winner via `WinningPlanSelector`
5. Settle via `CRXSettlementEngine`
6. Collect `CRXExecutionResult`

If no valid candidates exist, a minimal synthetic fallback plan is used.

---

## Error Variants (Phase 3)

New `RuntimeError` variants added in Phase 3:

| Variant | When raised |
|---|---|
| `GoalPacketInvalid(String)` | `GoalPacket::validate()` fails |
| `RightsCapsuleViolation { capsule_id, reason }` | Rights scope breach (reserved for future use) |
| `CausalViolation(String)` | Cycle detected in causal graph |
| `BranchExecutionFailed { branch_id, reason }` | Reserved for caller use |
| `FinalityEscalationRequired(String)` | Reserved for caller use |
| `CRXSettlementFailed(String)` | Reserved for caller use |

---

## Implementation Decisions

### Why BTreeSet for Kahn's Queue?
Using a `BTreeSet` as the zero-in-degree queue ensures that when multiple nodes have the same in-degree, they are always processed in `NodeId` ascending order. This produces a stable, deterministic topological ordering independent of insertion order.

### Why Not Clone ObjectStore?
`ObjectStore` contains `Box<dyn Object>` entries which don't implement `Clone` (trait objects). The simulation engine instead uses a pre-execution snapshot of only the balance and pool objects (which do implement `Clone`) and restores them on revert. Full store cloning would require a `Clone` bound on the `Object` trait.

### Spend Limit Tracking
The rights validation engine tracks `cumulative_spend` incrementally as it processes debit nodes in topological order. This means early-graph debits consume the budget first, which is intentional (topological order = causal order).

### Domain Tag Propagation
When `GraphAssembler` processes a `PlanAction`, it reads the `"domain_tag"` key from the action's metadata and propagates it to the generated `CausalNode`. This allows plan submitters to tag specific operations for domain-level policy enforcement without modifying the graph schema.
