# Omniphi Solver Market — Phase 2 Design Reference

## Overview

Phase 2 extends the Omniphi Object + Intent Runtime with a **Solver Market**: an
open, competitive market where external actors (solvers) submit execution plans for
intents.  The runtime validates, ranks, and selects the best plan, then executes it
via the existing Phase 1 settlement engine.

The design is deterministic, BTreeMap-based, and safe for use inside a PoSeq-ordered
consensus environment.

---

## Module Map

```
runtime/src/
├── solver_registry/        — Solver identity, status, capabilities, reputation
├── solver_market/          — CandidatePlan, PlanAction, PlanEvaluationResult
├── plan_validation/        — PlanValidator, ValidationReasonCode
├── selection/              — PlanRanker, WinningPlanSelector, SelectionPolicy
├── matching/               — IntentSolverMatcher
├── agent_interfaces/       — AgentSolverProfile, AgentPolicyEnvelope, DeterministicAgentSubmission
├── simulation/             — PlanSimulator, SimulationResult (dry-run)
├── policy/                 — PlanPolicyEvaluator trait + built-in policies
└── attribution/            — SolverAttributionRecord, SelectionAuditRecord, PlanOutcomeRecord
```

`poseq::interface` is updated with `SolverMarketRuntime` and `SolverMarketBatch`.

---

## Lifecycle: `SolverMarketRuntime::process_solver_batch`

```
SolverMarketBatch
    │
    ├── for each IntentTransaction (in PoSeq canonical order):
    │       │
    │       ├── IntentSolverMatcher::find_eligible_solvers
    │       │       (Active, not flagged, allowed intent class)
    │       │
    │       ├── for each CandidatePlan in batch.candidate_plans[intent_id]:
    │       │       PlanValidator::validate(plan, intent, store, registry, policy)
    │       │           → PlanEvaluationResult { passed, normalized_score, ... }
    │       │       registry.record_submission(solver_id, accepted, won=false)
    │       │
    │       ├── WinningPlanSelector::select(valid_plans, policy)
    │       │       → winner_index
    │       │
    │       ├── registry.record_submission(winner_id, true, won=true)
    │       │
    │       ├── Build SolverAttributionRecord + SelectionAuditRecord
    │       │
    │       └── (if no candidates or all invalid) → fallback_to_internal_resolver
    │
    ├── Convert winning CandidatePlans → ExecutionPlans
    ├── ParallelScheduler::schedule(execution_plans)
    └── SettlementEngine::execute_groups(groups, store, epoch)
            → FinalSettlement
```

---

## Plan Validation Checks (in order)

| # | Check | Reason Code |
|---|-------|-------------|
| 1 | `plan.validate_hash()` | `PlanHashMismatch` |
| 2 | Solver registered + status == Active | `SolverNotActive` |
| 3 | Solver capabilities ⊇ required_capabilities | `SolverCapabilityInsufficient` |
| 4 | Total objects ≤ solver.max_objects_per_plan | `MaxObjectsExceeded` |
| 5 | All object_reads + object_writes exist in store | `ObjectNotFound` |
| 6 | DebitBalance actions have sufficient available balance | `InsufficientBalance` |
| 7 | `policy.evaluate(plan, intent) == None` | `PolicyRejected` |
| 8 | `plan.fee_quote <= intent.max_fee` | `FeeTooHigh` |
| 9 | `plan.expected_output_amount > 0` | `ZeroOutput` |
| 10 | All checks passed → compute `normalized_score` | `Valid` |

---

## Scoring Formula

`normalized_score` (0–10000 bps) is computed only for plans that pass all checks:

```
quality_component  = quality_score × 60%          → 0–6000
fee_component      = (10000 - fee_quote) × 30%    → 0–3000  (lower fee = higher score)
constraint_cmp     = (satisfied / total) × 10%    → 0–1000

normalized_score   = min(quality + fee + constraint, 10000)
```

---

## Selection Policies

| Policy | Primary | Tie-break 1 | Tie-break 2 |
|--------|---------|-------------|-------------|
| `BestScore` | highest `normalized_score` | lowest `fee_quote` | lexical `solver_id` ascending |
| `LowestFee` | lowest `fee_quote` | highest `normalized_score` | lexical `solver_id` ascending |
| `BestOutput` | highest `expected_output_amount` | lowest `fee_quote` | lexical `solver_id` ascending |

All tie-breaking is deterministic: identical inputs always produce the same winner.

---

## Solver Reputation

```
SolverReputationRecord {
    reputation_score:           5000   // basis points, 0–10000; default neutral
    consecutive_invalid_plans:  u64    // resets to 0 on first valid submission
    is_flagged:                 bool   // set true when consecutive_invalid >= 3
}
```

Auto-flagging rules:
- 3 or more consecutive invalid plan submissions → `is_flagged = true`
- `is_flagged == true` → solver excluded from `find_eligible_solvers`
- Flag can be cleared by governance (direct `set_status` or `update_reputation_score`)

---

## Agent/AI Solver Interface

AI solvers register as normal `SolverProfile` entries with `is_agent = true`.

An `AgentSolverProfile` provides a supplemental `AgentPolicyEnvelope`:
- `allowed_object_types`: object types the agent may include in plans
- `max_value_per_intent`: hard cap on `expected_output_amount`
- `max_objects_per_plan`: hard cap on total objects in a plan
- `valid_until_epoch`: expiry epoch; submissions after this are rejected

`DeterministicAgentSubmission::validate_against_policy(current_epoch)` enforces
these constraints.  The runtime re-validates regardless of the agent's
`policy_check_passed` assertion.

`PlanExplanationMetadata` is stored in `CandidatePlan.explanation` (excluded from
plan hash) and carries:
- `reasoning_summary`: human-readable explanation
- `confidence_bps`: 0–10000 agent self-reported confidence
- `inputs_considered`, `alternatives_rejected`, `safety_checks_passed`

---

## Plan Hash

`CandidatePlan::compute_hash()` produces a SHA256 hash over:

- `plan_id`, `intent_id`, `solver_id`
- `actions` (action_type tag + target_object + amount)
- `required_capabilities` (stable byte tags)
- `object_reads` count + each id
- `object_writes` count + each id
- `expected_output_amount`, `fee_quote`, `quality_score`
- `constraint_proofs` (name + declared_satisfied + supporting_value)
- `submitted_at_sequence`

**Excluded from hash**: `plan_hash`, `explanation`, `metadata`.

The runtime rejects plans where the stored `plan_hash` does not match the recomputed
value.

---

## Built-in Policy Evaluators

| Type | Behaviour |
|------|-----------|
| `PermissivePolicy` | Accepts all plans (default for tests) |
| `MaxValuePolicy { max_value }` | Rejects plans with output > max_value |
| `ObjectBlocklistPolicy { blocked_objects }` | Rejects plans writing blocked objects |
| `SolverSafetyPolicy { blocked_solver_ids }` | Rejects plans from blocked solvers |
| `DomainRiskPolicy { quarantined_objects, … }` | Rejects plans writing quarantined objects |
| `CompositePolicyEvaluator { policies }` | All contained policies must pass |

---

## Simulation

`PlanSimulator::simulate(plan, intent, store, registry, policy)` dry-runs a plan
without mutating the real store.  It:

1. Runs the identical validation code path as production.
2. Applies mutations to an in-memory snapshot of referenced balance objects.
3. Returns `SimulationResult`:
   - `would_pass_validation`
   - `validation_reason_codes`
   - `preview_balance_changes`: `Vec<(ObjectId, i128)>` — negative = debit
   - `preview_new_versions`: `Vec<(ObjectId, u64)>`
   - `estimated_gas`, `estimated_output`, `normalized_score`

---

## Attribution Records

For every resolved intent the runtime produces:

**`SolverAttributionRecord`** — canonical winner record:
- Winning solver/plan id and hash
- Candidate counts (total/valid/rejected)
- Selection policy and basis string

**`SelectionAuditRecord`** — full audit trail:
- All plan_ids considered
- All `PlanEvaluationResult`s
- Ranking (indices best-first) and winner index

**`PlanOutcomeRecord`** — per-plan outcome for reputation accounting:
- `won`, `validation_passed`, `reason_codes`, `execution_success`

---

## Fallback to Phase 1

When an intent has no submitted candidate plans, or all candidates fail validation,
`SolverMarketRuntime` automatically falls back to the Phase 1 internal resolver
(`IntentResolver::resolve`).  The synthetic plan is assigned:
- `solver_id = [0xFF; 32]` (internal resolver marker)
- `attribution.selection_policy = "FallbackPhase1"`

---

## Determinism Guarantees

- `BTreeMap`/`BTreeSet` used everywhere — no `HashMap`.
- All numeric scores in `u64` basis points (no floating point).
- Tie-breaking is fully deterministic via lexical `solver_id` comparison.
- `CandidatePlan::compute_hash` excludes non-deterministic fields (`explanation`, `metadata`).
- `PlanRanker::rank` uses stable sort — equal elements preserve input order as secondary.
