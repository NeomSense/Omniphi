# Safety Kernel — Phase 4: Deterministic Safety Kernel + Scoped Kill-Switch System

## Overview

The Safety Kernel is a deterministic, composable safety layer for the Omniphi Object + Intent Runtime. It intercepts CRX settlement results and applies a 9-step lifecycle to detect, classify, contain, and record safety incidents.

## Architecture

```
runtime/src/safety/
├── mod.rs                — public re-exports
├── incidents.rs          — IncidentType, IncidentSeverity, IncidentScope, SafetyIncident
├── actions.rs            — SafetyAction (Quarantine, Pause, RateLimit, SuspendSolver, EmergencyMode, ...)
├── policies.rs           — SafetyPolicy, RuleCondition, SafetyRuleEngine, default_policies()
├── blast_radius.rs       — BlastRadiusEngine, ContainmentScope, ScopeResolutionPolicy
├── domain_profiles.rs    — DomainRiskProfile, DomainSafetyPolicy, DomainCriticality
├── solver_controls.rs    — SolverSafetyController, SolverSafetyStatus, SolverRestrictionProfile
├── receipts.rs           — SafetyReceipt, IncidentLedger, IncidentLedgerEntry
├── recovery_hooks.rs     — RecoveryHook trait, NoOpRecoveryHook, CriticalEscalationHook
├── kernel.rs             — SafetyKernel (9-step lifecycle), SafetyEvaluationContext, SafetyDecision
├── poseq_bridge.rs       — PoSeqSafetyBridge, SafetyAnnotatedSettlement, SafetyConstrainedExecutionState
└── simulation.rs         — SafetySimulator, IncidentScenario, SafetySimulationResult
```

## Core Types

### `IncidentSeverity` (ordered)
```
Info < Low < Medium < High < Critical < Emergency
```

### `IncidentType` (15 variants)
AbnormalOutflow, SolverMisconduct, GovernanceSensitiveObjectMisuse, LiquidityPoolInstability,
AbnormalMutationVelocity, CrossDomainBlastRadiusEscalation, RepeatedBranchQuarantine, and more.

### `SafetyAction` (8 variants)
- `NoAction` — pass-through
- `LogOnly(String)` — audit log only
- `Quarantine(QuarantineAction)` — quarantine specific object IDs
- `Pause(PauseAction)` — pause a domain, execution class, or liquidity venue
- `RateLimit(RateLimitAction)` — throttle mutations or value flow
- `SuspendSolver([u8;32], String)` — bar solver from submission
- `EmergencyMode(EmergencyModeAction)` — placeholder for full-chain halt
- `MultiAction(Vec<SafetyAction>)` — stacked actions

## 9-Step Safety Lifecycle (SafetyKernel::evaluate)

1. **Rule evaluation** — `SafetyRuleEngine::evaluate()` runs all enabled policies against the `SafetyEvaluationContext`
2. **Highest severity** — select the top-triggered rule (sorted by severity desc)
3. **Incident classification** — build a `SafetyIncident` with type, scope, affected entities, metadata
4. **Blast radius assessment** — `BlastRadiusEngine::assess()` determines containment scope (Minimal → Global)
5. **Action determination** — use the rule's `default_action` or `escalated_action`
6. **State updates** — apply quarantines, pauses, suspensions; build `ConstrainedStateUpdate` list
7. **Recovery hooks** — for Critical+ incidents, call all registered `RecoveryHook`s
8. **Ledger append** — write `SafetyReceipt` to `IncidentLedger`
9. **Return `SafetyDecision`** — incident, action, blast_radius, governance_escalation, state_updates

## Default Policies

| Policy | Condition | Severity | Action |
|---|---|---|---|
| AbnormalOutflow | outflow > 1,000,000 | High | Quarantine affected objects |
| SolverMisconduct | 3+ rights violations | Medium | SuspendSolver |
| RepeatedBranchQuarantine | >2 quarantines in window | Low | RateLimit |
| GovernanceSensitiveObjectMisuse | is_governance_sensitive flag | High | Pause(governance domain) |
| LiquidityPoolInstability | pool reserve deviation > 5000 bps | High | Pause(LiquidityVenue) |
| AbnormalMutationVelocity | >100 mutations/epoch (global) | Medium | RateLimit |
| CrossDomainBlastRadius | 3+ affected domains | Critical | EmergencyMode (placeholder) |

## Blast Radius Scope Resolution

```
Severity / Domain Count → ContainmentScope
Info/Low (any)          → Minimal
Medium (single domain)  → Scoped
High (with domain)      → Domain
Critical                → MultiDomain
Emergency               → Global
>= emergency_threshold  → Global (override)
>= cross_domain_threshold → MultiDomain (override)
```

Defaults: `cross_domain_threshold = 2`, `emergency_threshold = 4`.

## Domain Risk Profiles

Pre-configured for: `treasury`, `governance`, `dex_liquidity`, `lending`, `bridge`, `identity`, `rewards`, `oracle`.

- `treasury`, `bridge`, `governance` → `DomainCriticality::Critical`, governance escalation required
- `governance` → `pause_sensitivity = false` (cannot be auto-paused)
- `oracle`, `lending` → `oracle_dependent = true`

## Solver Safety States (ordered)

```
Normal < Monitored < RateRestricted < DowngradeOnly < Suspended < Banned
```

Only `Normal`, `Monitored`, `RateRestricted`, `DowngradeOnly` → solver is allowed.
`Suspended` and `Banned` → solver is blocked.

## PoSeq Integration

`PoSeqSafetyBridge::process_batch()` evaluates each `CRXExecutionResult` through the kernel and returns:
- `Vec<SafetyAnnotatedSettlement>` — each result with `safety_decision` and `final_allowed` flag
- `SafetyConstrainedExecutionState` — current quarantined objects, paused domains, suspended solvers, emergency_mode flag

## Simulation

`SafetySimulator::simulate()` clones the kernel and runs evaluation without mutating the original state — safe for dry-run impact assessment.

`SafetySimulator::run_all_scenarios()` runs 6 built-in scenarios:
1. `abnormal_outflow` — High severity, Quarantine
2. `solver_misconduct` — Medium severity, SuspendSolver
3. `governance_misuse` — High severity, Pause(governance)
4. `oracle_inconsistency` — Provisional mode
5. `liquidity_instability` — High severity, Pause(LiquidityVenue)
6. `cross_domain_emergency` — Critical severity, EmergencyMode

## Design Invariants

- No `HashMap` — all maps use `BTreeMap`/`BTreeSet` for determinism
- No floating point — thresholds are `u128`/`u64`; rates are basis points (`u32`/`u64`)
- `SafetyKernel::evaluate()` is the sole entry point; all state updates flow through `apply_action()`
- Emergency mode is a placeholder — full-chain halt is a governance workflow
- `IncidentLedger` with `max_entries = 0` is unbounded; production use should set a cap
- Governance-required actions produce a `GovernanceEscalationMarker` via recovery hooks

## Runtime Error Variants (Phase 4)

```rust
SafetyViolation { incident_type: String, severity: String }
ContainmentFailed { scope: String, reason: String }
KernelEvaluationError(String)
IncidentLedgerFull
RecoveryRequiresGovernance(String)
```
