# Omniphi Runtime Security & Correctness Audit Report

**Date**: 2026-03-11
**Auditor**: Internal Security Review
**Codebase**: `runtime/src/` — Phases 1–4
**Commit basis**: HEAD of main branch

---

## 1. Executive Summary

| Readiness Level | Status |
|---|---|
| Safe to build on | **Conditional** — Phase 1 core is solid; Phases 3–4 have blocking bugs |
| Devnet ready | **No** — 3 Critical and 5 High findings must be resolved first |
| Testnet ready | **No** — Requires all Critical/High findings fixed and invariant test suite green |
| Mainnet ready | **No** — Many systemic issues; estimated 6–8 months of hardening required |

The Phase 1 object store and settlement engine are architecturally sound and show good engineering discipline (U256 overflow protection, all-or-nothing atomicity via dry-run validation, deterministic BTreeMap ordering). However, Phases 3 and 4 introduce several serious correctness issues: the CRX branch executor does not sync typed overlays to the canonical store before state root computation, the safety kernel has no emergency mode reset path, the snapshot/restore mechanism in CRX settlement only covers balance objects (silently leaving pool and vault mutations unreverted), and the solver reputation double-counting inflates winner submission counts. These findings are not theoretical — they are directly observable from the code paths that execute in every settlement cycle.

---

## 2. Critical and High Findings

---

### FIND-001

**Title**: CRX Branch Execution Does Not Sync Typed Overlays Before State Root
**Phase/Module**: Phase 3 / `crx/branch_execution.rs`, `crx/settlement.rs`
**Severity**: Critical
**Category**: correctness / determinism

**Description**:
`BranchAwareExecutor::execute_node()` in `crx/branch_execution.rs` mutates `BalanceObject` and `LiquidityPoolObject` through the typed overlay accessors (`get_balance_by_id_mut`, `get_pool_by_id_mut`) but does NOT call `store.sync_typed_to_canonical()` after mutations. `CRXSettlementEngine::settle()` (`crx/settlement.rs:206`) does call `store.sync_typed_to_canonical()` after `BranchAwareExecutor::execute()` returns — however, `FinalizeSettlement` nodes in branch execution (`branch_execution.rs:460–472`) also call `store.get_balance_by_id_mut()` / `store.get_pool_by_id_mut()` and increment versions in the typed overlay only. The canonical `objects` BTreeMap used by `state_root()` does not see these version bumps until sync is called.

More critically: if a revert is triggered, `snapshot.restore()` (`crx/settlement.rs:92–98`) restores only `BalanceObject` values. The `objects` canonical map is synced afterward (`crx/settlement.rs:217`), but the pool typed overlay is never snapshot-taken or restored (the `take_all_balances()` helper at line 86–89 returns an empty snapshot). This means a reverted CRX settlement that touched a pool leaves the pool mutations in the typed overlay un-reverted, while the canonical map may differ from the typed overlay.

**Why it matters**:
The state root can be computed over stale or corrupted data. Nodes that replay state would disagree on root hashes. This breaks the primary determinism guarantee.

**Exploit scenario**:
A solver submits a plan that debits a balance (reverted) and swaps a pool (not reverted due to incomplete snapshot). On revert, the balance is restored but the pool reserve mutation persists. State root is computed over the inconsistent canonical store. Validators diverge.

**Recommended fix**:
`StoreSnapshot::take()` must also snapshot pool and vault objects. `StoreSnapshot::restore()` must restore all three types. Add `sync_typed_to_canonical()` call inside `BranchAwareExecutor::execute_node()` immediately after any typed overlay mutation, or alternatively call it once at the very end of `BranchAwareExecutor::execute()` before returning.

**Test to add**: `test_revert_restores_pool_mutation` — verify pool reserves are unchanged after a reverted settlement.

---

### FIND-002

**Title**: Safety Kernel Emergency Mode Has No Recovery Path
**Phase/Module**: Phase 4 / `safety/kernel.rs`
**Severity**: Critical
**Category**: security / correctness

**Description**:
`SafetyKernel::apply_action()` at `kernel.rs:438–440` sets `self.emergency_mode = true` when a `SafetyAction::EmergencyMode` is triggered. There is no code path anywhere in `kernel.rs`, `poseq_bridge.rs`, or any other module that sets `self.emergency_mode = false`. Once triggered by the `CrossDomainBlastRadius` policy (requires 3+ affected domains — a realistic event), every subsequent call to `PoSeqSafetyBridge::process_batch()` will set `final_allowed = false` for all settlements (`poseq_bridge.rs:70`), permanently blocking the chain.

**Why it matters**:
A single cross-domain incident permanently halts all settlement. There is no governance-gated reset. The chain cannot recover without a software patch and restart.

**Exploit scenario**:
An attacker crafts a single CRX settlement record with `affected_domains` containing 3 domain strings. The `CrossDomainBlastRadius` policy triggers, sets `emergency_mode = true`. All subsequent settlements are rejected with `final_allowed = false`. Chain is halted.

**Recommended fix**:
Add a `reset_emergency_mode(&mut self, governance_proof: GovernanceProof)` method that verifies a multi-sig governance approval before setting `emergency_mode = false`. Expose it via the PoSeq bridge. The recovery path must be gated by a governance threshold, not by a single operator.

**Test to add**: `test_emergency_mode_persists_after_kernel_evaluate` — verify `emergency_mode` cannot be reset without explicit call.

---

### FIND-003

**Title**: Solver Reputation Double-Counted for Winner
**Phase/Module**: Phase 2 / `poseq/interface.rs`
**Severity**: High
**Category**: correctness

**Description**:
In `SolverMarketRuntime::process_solver_batch()` (`interface.rs:238`), `self.registry.record_submission(&plan.solver_id, accepted, false)` is called for every plan including the eventual winner. Then at line 267–268, `self.registry.record_submission(&winner_solver_id, true, true)` is called again for the winner. This means the winner's `total_plans_submitted` is incremented twice: once in the validation loop (line 238) and once after winner selection (line 267). The winner's `plans_accepted` is also incremented twice, and `consecutive_valid_plans` is incremented twice. Only `plans_won` receives a single increment (via `won: true`).

**Why it matters**:
Reputation scores computed from `plans_accepted / total_plans_submitted` are inflated for winners. A solver who wins 50% of plans will appear to have a much higher acceptance rate than non-winners. Over time, winners accumulate false reputation that biases future selection. The `is_flagged` auto-flag logic (triggered by 3 consecutive invalid plans) uses `consecutive_invalid_plans` which is reset by the double-call when the solver wins, masking actual consecutive invalid plan history.

**Exploit scenario**:
A solver submits slightly-invalid plans repeatedly, winning occasionally. Each win resets their consecutive invalid counter via the double call to `record_submission`. They avoid the auto-flag threshold even though they should have triggered it.

**Recommended fix**:
Remove the second call to `record_submission` (lines 267–268). Instead, after selecting the winner, call only `registry.record_win(&winner_solver_id)` which increments only `plans_won`. The first call in the validation loop already handles `total_plans_submitted` and `plans_accepted`.

**Test to add**: `test_solver_submission_count_not_double_incremented`.

---

### FIND-004

**Title**: CRX Snapshot/Restore Only Covers Balances — Pools and Vaults Not Restored on Revert
**Phase/Module**: Phase 3 / `crx/settlement.rs`
**Severity**: Critical
**Category**: correctness

**Description**:
`StoreSnapshot::take_all_balances()` at `crx/settlement.rs:86–89` returns `StoreSnapshot { balances: BTreeMap::new() }` — an empty snapshot. The comment says "We can't directly iterate balances since it's private." This method is not actually called anywhere; `take()` is used instead. However `take()` only snapshots `BalanceObject`s by the provided `object_ids`. If the plan includes `SwapPoolAmounts` or vault mutations, the pool and vault typed overlays are mutated by `execute_node()` but are NOT included in the snapshot. When `snapshot.restore()` is called on revert, pool reserves remain mutated. The canonical store is synced (line 217) and then reflects the un-reverted pool state.

**Why it matters**:
A FullRevert is supposed to be a no-op. If pool reserves are mutated and not reverted, the AMM price is permanently altered by failed settlements — a critical economic invariant violation.

**Recommended fix**:
Extend `StoreSnapshot` to include `pools: BTreeMap<ObjectId, LiquidityPoolObject>` and `vaults: BTreeMap<ObjectId, VaultObject>`. Expose `ObjectStore::all_pool_ids()` and `ObjectStore::all_vault_ids()` iterators, or make `StoreSnapshot::take()` accept the store's full typed collections.

**Test to add**: `test_revert_does_not_mutate_pool`.

---

### FIND-005

**Title**: Rights Capsule Solver Identity Not Verified at Execution Time
**Phase/Module**: Phase 3 / `crx/rights_validation.rs`, `crx/settlement.rs`
**Severity**: High
**Category**: security

**Description**:
`RightsCapsule` contains `authorized_solver_id: [u8; 32]`. The `RightsValidationEngine::validate()` function checks object scope, action types, domains, and spend limits — but never checks that `capsule.authorized_solver_id == plan.solver_id`. The `CRXSettlementEngine::settle()` at `crx/settlement.rs:117` builds the capsule via `CausalPlanBuilder::build()` which sets `authorized_solver_id = plan.solver_id` (plan_builder.rs:575). So the synthesized capsule always matches. However, there is no external enforcement preventing a solver from reusing a synthesized capsule from a different plan (if the system were extended to accept pre-submitted capsules). More critically, the `can_access_object()` check at `rights_capsule.rs:165–169` does NOT check `authorized_solver_id`. A capsule with a broad `allowed_object_ids` scope could be passed to a different solver's plan.

**Why it matters**:
In a future API where capsules are submitted separately from plans, a solver could steal another solver's approved capsule and use it to access objects they should not be able to access.

**Recommended fix**:
Add a `solver_id: [u8; 32]` parameter to `RightsValidationEngine::validate()` and verify `capsule.authorized_solver_id == solver_id` as the first check. Return a `SolverMismatch` violation if they differ.

**Test to add**: `test_rights_capsule_wrong_solver_rejected`.

---

### FIND-006

**Title**: `allowed_object_ids` Empty in Synthesized Capsule Means Unrestricted Object Access
**Phase/Module**: Phase 3 / `crx/plan_builder.rs`, `crx/rights_capsule.rs`
**Severity**: High
**Category**: security

**Description**:
`RightsSynthesizer::synthesize()` in `plan_builder.rs:432–497` builds `allowed_object_ids` by collecting `node.target_object` for Write, SettlementOnly, and ValidationOnly nodes. For a graph with no nodes that match these access types (e.g., a graph with only `BranchGate` or `EmitReceipt` nodes — or a minimal graph), `allowed_object_ids` remains empty. `RightsCapsule::can_access_object()` at `rights_capsule.rs:166` implements the check as: `self.scope.allowed_object_ids.is_empty() || self.scope.allowed_object_ids.contains(object_id)`. An empty `allowed_object_ids` means the capsule grants access to **any** object id — unrestricted.

**Why it matters**:
A carefully crafted minimal CausalGraph (with no Write/ValidationOnly/SettlementOnly nodes) synthesizes a capsule with empty `allowed_object_ids`, which grants the capsule access to every object in the store. Since the capsule is synthesized from the plan itself, a solver could craft a plan that achieves this.

**Exploit scenario**:
Attacker submits a plan whose actions are all `Custom("noop")` type (mapped to `EmitReceipt` execution class, which has `SettlementOnly` access). `allowed_object_ids` may be populated depending on action target objects, but if `target_object` is set to a system object ID, the capsule's `allowed_object_ids` includes it. Attacker can use this capsule path to later access unrelated objects.

**Recommended fix**:
Change the semantics: empty `allowed_object_ids` should mean DENY ALL, not allow all. Add an explicit `unrestricted: bool` flag to `RightsScope` and only use the "empty = unrestricted" shorthand when `unrestricted = true`, which must be set explicitly by a privileged path.

**Test to add**: `test_rights_capsule_empty_allowed_object_ids_is_unrestricted`.

---

### FIND-007

**Title**: Gas Limit Computed with `saturating_mul(1000)` — Effectively Uncapped for Large `max_fee`
**Phase/Module**: Phase 1 / `resolution/planner.rs`, `poseq/interface.rs`
**Severity**: Medium
**Category**: DoS

**Description**:
At `planner.rs:197`, `gas_limit: max_fee.saturating_mul(1_000)`. Since `max_fee` is `u64` and `saturating_mul` returns `u64::MAX` on overflow, a `max_fee` of `u64::MAX / 1000 + 1 = 18_446_744_073_709_552` causes `gas_limit = u64::MAX`. The `GasMeter::new(u64::MAX)` is then effectively unlimited — the gas ceiling cannot be breached because `consumed.checked_add(amount)` will overflow and return an error only at `u64::MAX`, but with a limit of `u64::MAX` the check `consumed > limit` can never be true before the overflow error fires. The net effect is that a transaction with a very large `max_fee` gets unlimited execution gas.

**Why it matters**:
The gas limit is the primary DoS protection for runaway plan execution. Bypassing it allows a single intent to consume unbounded computation.

**Recommended fix**:
Cap gas_limit at a protocol-defined constant (e.g., `MAX_GAS_PER_TX = 10_000_000`). The formula should be `gas_limit = max_fee.saturating_mul(1_000).min(MAX_GAS_PER_TX)`. Document the protocol cap.

**Test to add**: `test_gas_limit_no_overflow_from_large_max_fee`.

---

### FIND-008

**Title**: Scheduler Conflict Detection Misses Mixed Read/Write Duplicate Entries
**Phase/Module**: Phase 1 / `scheduler/parallel.rs`
**Severity**: Medium
**Category**: correctness / determinism

**Description**:
`ParallelScheduler::conflicts()` at `parallel.rs:114–148` builds `a_writes` and `a_reads` as `BTreeSet` from `plan.object_access`. If `object_access` contains the same `object_id` listed as both `ReadOnly` AND `ReadWrite` (which can happen when `candidate_plan_to_execution_plan()` in `interface.rs:451–462` adds an object to both `object_reads` and `object_writes`), it appears in both `a_reads` and `a_writes`. A plan B that only reads the same object would be detected as conflicting via `rw` (B writes what A reads), which is correct. However, a plan A with an object in both sets would have the object in `a_reads` AND `a_writes`, but since the conflict check is symmetric, this does not cause false negatives — it only causes potential false positives (unnecessary serialization). The actual risk is that the scheduler groups two plans as non-conflicting when one uses a "write-only" declaration but the object was also added as read, leading to semantic ambiguity.

**Why it matters**:
Plans that should conflict can be placed in the same parallel group, allowing concurrent mutation of the same object without coordination.

**Recommended fix**:
Enforce that `object_access` is deduplicated before scheduling. If an object appears as both ReadOnly and ReadWrite, it must be treated as ReadWrite. Add a normalization pass in `ParallelScheduler::schedule()` that deduplicates and promotes any object with any ReadWrite mode.

**Test to add**: `test_scheduler_conflict_with_duplicate_object_entries`.

---

### FIND-009

**Title**: Safety Kernel Context Built Before `evaluate()` — Batch Intents Don't See Quarantine State Set by Prior Intents
**Phase/Module**: Phase 4 / `safety/poseq_bridge.rs`
**Severity**: High
**Category**: correctness

**Description**:
In `PoSeqSafetyBridge::process_batch()` (`poseq_bridge.rs:56–84`), for each `crx_result` in the batch, the context is built from `crx_result.record` BEFORE `evaluate()` is called. After `evaluate()` returns, `apply_action()` may have quarantined objects from this result. The next iteration builds its context from `crx_result` data (which was captured before evaluation) — this is fine. However, the `final_allowed` check at lines 70–80 reads `self.kernel.quarantined_objects` and `self.kernel.is_solver_allowed()` which DO reflect the post-evaluate state. So intent N+1 correctly reads quarantine state set by intent N. This part is correct.

However, the `SafetyEvaluationContext::from_crx_record()` at `kernel.rs:48–80` sets `affected_domains: vec![]` unconditionally (line 58, comment: "would be populated from domain map in real integration"). This means domain-based blocking (`is_domain_paused()`) never fires during batch processing regardless of paused domain state, because the domains list in the context is always empty.

**Why it matters**:
Domain pause protection is entirely bypassed in batch processing. A paused domain's settlements are never blocked by the domain check.

**Recommended fix**:
Populate `affected_domains` in `SafetyEvaluationContext::from_crx_record()` by passing a domain map and looking up the graph's domains from the CRX record.

**Test to add**: `test_poseq_bridge_domain_pause_check_reads_updated_state`.

---

### FIND-010

**Title**: Domain Pause Check in PoSeq Bridge Uses Empty `affected_domains` — Pause Never Fires
**Phase/Module**: Phase 4 / `safety/poseq_bridge.rs`, `safety/kernel.rs`
**Severity**: High
**Category**: security / correctness

**Description**:
`SafetyEvaluationContext::from_crx_record()` at `kernel.rs:58` sets `affected_domains: vec![]`. The domain pause check at `poseq_bridge.rs:83–84` is: `!ctx.affected_domains.iter().any(|d| self.kernel.is_domain_paused(d))`. Since `ctx.affected_domains` is always empty, this expression always evaluates to `true`, meaning domain-paused settlements are always `final_allowed = true` from the domain perspective. The domain pause set is populated correctly in `apply_action()` (`kernel.rs:421–424`) and `is_domain_paused()` works correctly — but the domain ids used to check are never populated.

**Why it matters**:
The Pause(Domain) action in the safety kernel has no effect on settlement filtering. Any settlement in a paused domain will proceed.

**Recommended fix**:
Pass the domain tags from the graph through the `CRXSettlementRecord` and populate `affected_domains` in `from_crx_record()`. Alternatively, pass domains as a parameter to `from_crx_record()`.

**Test to add**: `test_domain_pause_blocks_settlement`.

---

### FIND-011

**Title**: Branch Quarantine Failure Mode Does Not Restore Pre-Failure Mutations
**Phase/Module**: Phase 3 / `crx/branch_execution.rs`
**Severity**: High
**Category**: correctness

**Description**:
In `BranchAwareExecutor::execute()` (`branch_execution.rs:152–240`), when the `Quarantine` failure mode is selected, the code quarantines only `failed_node.target_object` (the object that was being mutated when the branch failed). However, `mutated_objects` at line 98 tracks all objects that were successfully mutated BEFORE the failure point. These mutations remain in the typed overlay and are NOT reverted. The `BranchExecutionResult` returned with `success: false` includes `mutated_objects` containing these pre-failure mutations. The `CRXSettlementEngine::settle()` snapshot/restore mechanism only fires for `FullRevert` settlement class; `SuccessWithQuarantine` does not trigger restore.

**Why it matters**:
In quarantine mode, a branch can partially execute — debiting a balance, then failing on a later node — and the debit persists in state while the branch reports failure. Funds are lost: debited from the sender but never credited to the recipient.

**Exploit scenario**:
Attacker crafts a plan where: node 1 debits user's balance (succeeds), node 2 swaps pool with a bad delta that will fail (fails). Quarantine mode is selected. User's balance is debited. Pool mutation fails and is quarantined. Funds disappear from user with no corresponding credit.

**Recommended fix**:
For `Quarantine` failure mode, revert all pre-failure mutations in the branch. This requires taking a per-branch snapshot before execution and restoring it on quarantine failure.

**Test to add**: `test_quarantine_mode_reverts_partial_debit`.

---

### FIND-012

**Title**: `FinalizeSettlement` Node Increments Version Without Syncing to Canonical Store
**Phase/Module**: Phase 3 / `crx/branch_execution.rs`
**Severity**: Medium
**Category**: correctness

**Description**:
`BranchAwareExecutor::execute_node()` for `NodeExecutionClass::FinalizeSettlement` at `branch_execution.rs:460–472` increments the version in the typed overlay (`bal.meta.version += 1` or `pool.meta.version += 1`) but does NOT call `store.sync_typed_to_canonical()`. Since `state_root()` reads from the canonical `objects` BTreeMap, the version increment is invisible until sync is called externally. The external sync call happens at `crx/settlement.rs:206` only after ALL branches have executed. If a state root is computed between branches (e.g., in multi-branch graphs), it will reflect stale versions.

**Why it matters**:
Multi-branch version tracking is broken. Finality receipts include incorrect version transitions when multiple branches execute.

**Recommended fix**:
Call `store.sync_typed_to_canonical()` inside `execute_node()` after the version increment in `FinalizeSettlement` handling, or defer all version increments to a single post-execution sync pass.

**Test to add**: `test_finalize_settlement_version_visible_in_state_root`.

---

### FIND-013

**Title**: `CandidatePlan::compute_hash()` Excludes `explanation` and `metadata` — Hash Collision Possible
**Phase/Module**: Phase 2 / `solver_market/market.rs`
**Severity**: Medium
**Category**: security / correctness

**Description**:
Per the audit prompt, `compute_hash()` excludes `explanation` and `metadata` fields. Two plans with identical functional fields but different `metadata` entries produce identical hashes. The validator checks hash integrity via `plan.validate_hash()`, but since metadata is excluded, a solver can submit a plan with metadata `{"attack": "true"}` that has the same hash as a benign plan. The selection audit trail (`SelectionAuditRecord`) records `plan_hash` but the hash cannot distinguish these plans.

**Why it matters**:
Audit non-repudiation is broken. A solver can claim they submitted a different plan than what was executed by pointing to hash collisions. Post-incident forensics cannot reconstruct the exact plan that was run.

**Recommended fix**:
Include all fields in `compute_hash()`. If `explanation` is large and variable, hash it separately and include the sub-hash.

**Test to add**: `test_plan_hash_covers_metadata`.

---

### FIND-014

**Title**: Provisional Finality Still Applies Mutations Without Confirmation Mechanism
**Phase/Module**: Phase 3 / `crx/finality.rs`, `crx/settlement.rs`
**Severity**: Medium
**Category**: correctness

**Description**:
When `SettlementStrictness::Provisional` is set in the goal policy, `FinalityClassifier::classify()` returns `FinalityClass::Provisional`. `CRXSettlementEngine::settle()` checks `is_reverted` at line 212–213 as `finality.finality_class == FinalityClass::Reverted || settlement_class == ExecutionSettlementClass::FullRevert`. `Provisional` does NOT trigger revert. So state mutations are applied and the settlement returns with `Provisional` class — but there is no mechanism to confirm or finalize provisionally settled state later. Funds are moved, but the settlement is marked as unconfirmed with no follow-up path.

**Why it matters**:
Funds are in a limbo state: moved in state but marked provisional. If the provisional settlement is later determined to be invalid (e.g., by a governance challenge), there is no revert mechanism. The provisional marker becomes a pure documentation artifact with no enforcement.

**Recommended fix**:
Provisional settlements should either: (a) not apply mutations until confirmed (hold in a pending state), or (b) apply mutations but record a reversibility marker that the governance module can use to trigger a revert within `provisional_until_epoch`.

**Test to add**: `test_provisional_finality_mutation_behavior`.

---

### FIND-015

**Title**: `SwapPoolAmounts` `delta_b` Lost in `plan_action_to_operation()` Conversion
**Phase/Module**: Phase 2 / `poseq/interface.rs`
**Severity**: High
**Category**: correctness

**Description**:
`plan_action_to_operation()` at `interface.rs:492–496` converts `PlanActionType::SwapPoolAmounts` as:
```rust
ObjectOperation::SwapPoolAmounts {
    pool_id: action.target_object,
    delta_a: action.amount.unwrap_or(0) as i128,
    delta_b: 0,
}
```
`delta_b` is hardcoded to `0`. In `PlanAction`, only a single `amount` field exists. When a solver submits a swap plan through the solver market, the `delta_b` for the pool swap is always zero — meaning the pool's `reserve_b` is never updated. The settlement will debit the sender's input asset balance and attempt to credit the output asset, but the pool reserves will be partially updated (only `reserve_a`), breaking the constant-product invariant.

**Why it matters**:
Every solver-market-routed swap execution corrupts the AMM pool state. The pool reserve_b is never debited on output, making the pool appear to have more reserves than it does. Subsequent swaps compute incorrect output amounts.

**Recommended fix**:
`PlanAction` needs a `delta_b: Option<i128>` field, or the conversion must extract both deltas from `metadata`. The `operation_to_plan_action()` conversion at lines 532–536 already loses `delta_b` by setting `amount: Some(*delta_a as u128)` and ignoring `delta_b`.

**Test to add**: `test_swap_pool_delta_b_not_zero_after_conversion`.

---

### FIND-016

**Title**: Causal Validity Does Not Detect Hidden Object Access in Branch Execution
**Phase/Module**: Phase 3 / `crx/causal_validity.rs`, `crx/branch_execution.rs`
**Severity**: Medium
**Category**: security

**Description**:
`CausalValidityEngine::validate()` checks graph structure (acyclicity, dependency order, branch bypass, forbidden domains, skipped validation nodes) but there is no `HiddenObjectAccess` check implemented. The `CausalViolation::HiddenObjectAccess` enum variant is defined but never pushed to `all_violations`. `BranchAwareExecutor::execute_node()` for `MutateBalance` at lines 341–371 operates on any object ID in `node.target_object` regardless of whether that object is declared in the capsule's `allowed_object_ids`. A solver could insert a node with a `target_object` not in the causal graph's declared scope but not in the capsule's allowed list — and `rights_validation.rs` only checks objects that are in the STORE (line 196–199: `obj_type = store.get(&obj_id).map(|o| o.object_type())`). If the store doesn't have the object (e.g., it was quarantined or removed), the check at line 214 falls through and if `allowed_object_ids` is empty (FIND-006), access is granted.

**Why it matters**:
Objects outside the causal graph's declared scope can be accessed silently.

**Recommended fix**:
Implement the `HiddenObjectAccess` check: after execution, compare the set of actually-mutated objects against the capsule's `allowed_object_ids`. Any mutation on an object not in the allowed set should be a fatal violation.

**Test to add**: `test_branch_execution_cannot_touch_undeclared_object`.

---

### FIND-017

**Title**: Receipt Ordering Nondeterministic When Execution Errors Occur Within Groups
**Phase/Module**: Phase 1 / `settlement/engine.rs`
**Severity**: Low
**Category**: determinism

**Description**:
`SettlementEngine::execute_groups()` at `engine.rs:54–67` processes plans sequentially within each group (despite the comment mentioning rayon). Receipts are pushed in plan order within each group, and groups are sorted by `group_index`. This is deterministic for the success path. However, the comment at line 37–38 says "rayon is available" — if rayon parallel iteration were used in the future, receipt ordering would not be guaranteed within a group. More concretely: `settlement_result.receipts` is not sorted by `tx_id` before being returned. Callers that compare `receipts` vectors for two runs will get identical results, but callers that sort by receipt position rather than tx_id may get inconsistent behavior.

**Why it matters**:
Any downstream consumer that assumes `receipts[i]` corresponds to the ith submitted plan would break if the group execution order changes. Explicit sorting by tx_id should be enforced.

**Recommended fix**:
Sort `receipts` by `tx_id` (lexicographic) before returning `SettlementResult`. This makes the contract explicit and stable across potential future parallelism refactors.

**Test to add**: `test_settlement_receipt_order_deterministic`.

---

## 3. Medium and Low Findings

---

### FIND-018

**Title**: `CreditBalance` Uses `saturating_add` — Overflow Silently Caps Balance
**Severity**: Medium | **Category**: correctness
**Module**: `settlement/engine.rs:284–286`

`balance.amount = balance.amount.saturating_add(*amount)` silently caps at `u128::MAX` on overflow. A credit that would overflow produces a balance less than expected without error. The sender's debit succeeds, the recipient's credit saturates, and the difference disappears. Use `checked_add` and return an error on overflow.

---

### FIND-019

**Title**: `UnlockBalance` in `execute_node()` Uses `saturating_sub` — Can Unlock More Than Locked
**Severity**: Medium | **Category**: correctness
**Module**: `crx/branch_execution.rs:456–458`

`bal.locked_amount = bal.locked_amount.saturating_sub(amount)` will silently succeed even if `amount > locked_amount`. The engine-level `validate_op()` checks this for Phase 1 operations, but `BranchAwareExecutor::execute_node()` for `UnlockObject` does not validate before applying. This allows an unlock that brings `locked_amount` to 0 even when the unlock amount is larger than the locked amount.

---

### FIND-020

**Title**: `SolverInvalidPlanRateExceeds` Condition Always Returns False
**Severity**: Medium | **Category**: correctness
**Module**: `safety/policies.rs:281–296`

The condition computes `let total = invalid + 1; let rate = (invalid * 10_000) / total;`. When `invalid = 0`, `rate = 0`. When `invalid = 1`, `rate = (1 * 10000) / 2 = 5000`. This logic is inverted — it treats `invalid` count as the numerator but uses `invalid + 1` as the denominator instead of the actual total plan count. The effective rate for any non-zero invalid count is always less than 10000 bps (100%), making the condition never exceed a `threshold_bps` of 5000 for small counts. More importantly, this condition does not use actual plan submission history from the registry.

---

### FIND-021

**Title**: `check_balance_sufficiency` in Validator Uses Pre-Lock Balance, Not Available Balance
**Severity**: Medium | **Category**: correctness
**Module**: `plan_validation/validator.rs:186–196`

`balance.available()` is called at line 190. If `available()` correctly subtracts `locked_amount` (which it does per `types.rs`), this is correct. However, validation runs BEFORE execution, so if two plans in the same batch both attempt to debit the same balance, the second plan passes validation (balance looks sufficient) but fails execution. This is a known TOCTOU issue but worth documenting — the validator result is not a guarantee of execution success.

---

### FIND-022

**Title**: `RepeatedRightsViolations` Condition Uses Wrong Solver ID — Wildcard `[0u8; 32]`
**Severity**: Medium | **Category**: correctness
**Module**: `safety/policies.rs:196–213`

`RuleCondition::RepeatedRightsViolations { solver_id: [0u8; 32], ... }` uses all-zeros as the solver_id in the default policy. The `check_condition()` at line 201 looks up: `let key = (*solver_id, "SolverMisconduct".to_string())` where `solver_id` is the all-zeros from the policy. The violation count is accumulated under the zero key regardless of the actual solver. This means violation counts aggregate across all solvers under the same key, causing false positives (one bad solver poisons counts for all) and false negatives (a specific solver's count is never checked per-solver).

---

### FIND-023

**Title**: Plan Score `fee_component` Silently Truncates Fees Above 10,000
**Severity**: Low | **Category**: correctness
**Module**: `plan_validation/validator.rs:238`

`let fee_normalized = plan.fee_quote.min(10_000)`. Any fee above 10,000 is treated the same as exactly 10,000, resulting in a `fee_component = 0`. A solver quoting 10,001 and one quoting 1,000,000 get identical scores. Since higher fees represent worse outcomes for the intent submitter, this silent truncation prevents proper discrimination between high-fee plans.

---

### FIND-024

**Title**: `BranchAwareExecutor` Does Not Validate Node Has Target Object Before Balance Ops
**Severity**: Low | **Category**: correctness
**Module**: `crx/branch_execution.rs:340–370`

`MutateBalance` execution at line 341 errors if `obj_id` is `None` (correct). However, `LockObject` and `UnlockObject` nodes both use `obj_id.ok_or_else(...)` (lines 426, 447) — this is correct. But `CheckBalance`/`CheckPolicy` at lines 324–337 silently return `Ok(())` if `obj_id` is `None`. A node with no target object passes the check unconditionally, which could allow a branch to skip a balance check.

---

### FIND-025

**Title**: Domain Envelope `is_domain_allowed()` Treats Empty `allowed_domains` as Unrestricted
**Severity**: Low | **Category**: security
**Module**: `crx/rights_capsule.rs:90–94`

Same pattern as FIND-006 but for domains. `self.allowed_domains.is_empty() || self.allowed_domains.contains(domain)` — empty allowed_domains means all domains are allowed. This is by design (commented as "unrestricted") but should be explicitly documented and tested, as a capsule synthesized with no domain restrictions grants access to `governance` and `treasury` domains.

---

### FIND-026

**Title**: `IncidentLedger::append()` Return Value Ignored — Ledger Overflow Silently Discards Records
**Severity**: Low | **Category**: correctness
**Module**: `safety/kernel.rs:383`

`let _ = self.ledger.append(receipt, ctx.batch_id)` — the return value is discarded. If the ledger is full (max_entries exceeded), `append()` returns an error that is silently ignored. Safety incidents may not be recorded in the ledger without any notification.

---

### FIND-027

**Title**: `ObjectStore::sync_typed_to_canonical()` Skips Objects Not in Canonical Map
**Severity**: Low | **Category**: correctness
**Module**: `state/store.rs:197–202`

The sync checks `if self.objects.contains_key(&id)` before updating the canonical map. If a typed overlay object was inserted directly without going through `insert()` (which maintains both maps), it would be in `balances` but not in `objects`, and sync would silently skip it. The invariant is documented but not enforced at runtime.

---

## 4. Threat Model

### Malicious Solvers

Solvers can currently:
- Submit plans with `metadata` that changes behavior without affecting plan hash (FIND-013)
- Craft plans that pass validation but fail mid-execution in quarantine mode, causing fund loss (FIND-011)
- Submit duplicate plans where the second call to `record_submission` for the winner inflates their reputation (FIND-003)
- Exploit the empty `allowed_object_ids = unrestricted` capsule synthesis via minimal-action plans (FIND-006)
- Submit swap plans through the solver market where `delta_b = 0` always corrupts the AMM (FIND-015)

**Risk**: HIGH. Solvers have significant power to corrupt state and manipulate reputation without detection.

### Malicious Users

Users can currently:
- Submit intents with `max_fee` near `u64::MAX` to get effectively unlimited gas (FIND-007)
- Craft intents whose CRX plans include `FinalizeSettlement` nodes targeting arbitrary objects for version bumping (no scope enforcement per FIND-016)

**Risk**: MEDIUM. Intent validation provides some protection but gas limits are bypassable.

### Economic Attackers

- AMM pool invariants can be broken by all solver-market-routed swaps due to `delta_b = 0` bug (FIND-015)
- Quarantine mode allows partial debit without credit (FIND-011), enabling fund extraction
- Provisional settlements apply mutations without confirmation, creating unresolvable limbo states (FIND-014)

**Risk**: CRITICAL. Multiple paths to permanent fund loss or AMM corruption.

### Griefing Attacks

- Triggering emergency mode with 3 affected domains halts all settlement permanently (FIND-002)
- An attacker who knows the `affected_domains` list is always empty can trigger domain pause with no effect, wasting governance cycles
- Submitting high-fee intents with `max_fee = u64::MAX` bypasses gas limits (FIND-007)

**Risk**: HIGH. Chain halt is achievable with a single well-crafted CRX record.

### Safety Kernel Abuse

- The `CrossDomainBlastRadius` policy fires on 3+ affected domains. Since `affected_domains` is always empty (FIND-009/010), this policy never fires — which means the emergency mode trigger is paradoxically protected by the same bug that breaks domain pausing. If `affected_domains` is fixed, the emergency mode becomes a chain-halt vector.
- Solver suspension uses `[0u8; 32]` wildcard ID, suspending all solvers under one key (FIND-022)

**Risk**: HIGH. The safety kernel has bugs that both prevent intended protections and enable unintended halts.

---

## 5. Invariant Checklist

| # | Invariant | Status | Code Reference |
|---|---|---|---|
| I-01 | State root computed over synced canonical store | **NEEDS VERIFICATION** | `store.rs:236`, `settlement.rs:70–71` — engine syncs; CRX branch does not per FIND-001 |
| I-02 | All-or-nothing plan atomicity in Phase 1 settlement | **HOLDS** | `engine.rs:112–117` dry-run before apply |
| I-03 | Revert restores all mutated objects | **FAILS** | `crx/settlement.rs:86–89` — pools and vaults not snapshot/restored (FIND-004) |
| I-04 | Gas meter prevents unbounded computation | **NEEDS VERIFICATION** | `planner.rs:197` saturating_mul allows u64::MAX (FIND-007) |
| I-05 | Solver identity verified against capsule | **FAILS** | `rights_validation.rs` — no `authorized_solver_id` check (FIND-005) |
| I-06 | Rights capsule restricts object access | **CONDITIONALLY FAILS** | `rights_capsule.rs:166` — empty allowed_object_ids = unrestricted (FIND-006) |
| I-07 | Spend accumulator uses updated cumulative value for each debit | **HOLDS** | `rights_validation.rs:106–116` — cumulative_spend updated after check, passed to next check |
| I-08 | Causal graph is acyclic before execution | **HOLDS** | `plan_builder.rs:573`, `causal_graph.rs:validate_acyclic()` |
| I-09 | Emergency mode requires explicit governance reset | **FAILS** | `kernel.rs:438–440` — no reset path (FIND-002) |
| I-10 | AMM pool constant-product invariant maintained after solver-market swaps | **FAILS** | `interface.rs:492–496` delta_b=0 (FIND-015) |
| I-11 | Solver reputation accurately reflects submission history | **FAILS** | `interface.rs:238, 267–268` — double count (FIND-003) |
| I-12 | Domain pause blocks settlements in that domain | **FAILS** | `kernel.rs:58` — affected_domains always empty (FIND-009/010) |
| I-13 | Branch failure in quarantine mode does not persist pre-failure debits | **FAILS** | `branch_execution.rs:199–226` — no per-branch revert (FIND-011) |
| I-14 | CandidatePlan hash uniquely identifies plan | **FAILS** | metadata excluded from hash (FIND-013) |
| I-15 | Scheduler conflict detection is sound and complete | **HOLDS** | `parallel.rs:114–148` — BTreeSet intersection logic is correct |

---

## 6. Test Matrix

| Test Name | Phase | Category | Priority | Status |
|---|---|---|---|---|
| test_state_root_reflects_typed_overlay_mutations | 1 | correctness | P1 | Missing |
| test_rights_capsule_empty_allowed_object_ids_is_unrestricted | 3 | security | P1 | Missing |
| test_spend_limit_accumulates_correctly_across_two_debits | 3 | correctness | P1 | Missing |
| test_gas_limit_no_overflow_from_large_max_fee | 1 | DoS | P1 | Missing |
| test_scheduler_conflict_with_duplicate_object_entries | 1 | correctness | P2 | Missing |
| test_emergency_mode_persists_after_kernel_evaluate | 4 | security | P1 | Missing |
| test_poseq_bridge_domain_pause_check_reads_updated_state | 4 | security | P1 | Missing |
| test_settlement_receipt_order_deterministic | 1 | determinism | P2 | Missing |
| test_rights_capsule_wrong_solver_rejected | 3 | security | P1 | Missing |
| test_provisional_finality_mutation_behavior | 3 | correctness | P2 | Missing |
| test_repeated_batch_execution_deterministic_state_root | 1 | determinism | P1 | Missing |
| test_solver_selection_fully_deterministic | 2 | determinism | P1 | Missing |
| test_branch_execution_cannot_touch_undeclared_object | 3 | security | P1 | Missing |
| test_zero_amount_debit_operation | 1 | correctness | P2 | Missing |
| test_causal_graph_cycle_is_detected | 3 | correctness | P1 | Exists (test_crx_causal_graph.rs) |
| test_revert_does_not_mutate_pool | 3 | correctness | P1 | Missing |
| test_quarantine_mode_reverts_partial_debit | 3 | correctness | P1 | Missing |
| test_solver_submission_count_not_double_incremented | 2 | correctness | P1 | Missing |
| test_swap_delta_b_not_zero | 2 | correctness | P1 | Missing |
| test_settle_reverted_does_not_mutate_store | 3 | correctness | P1 | Exists (test_crx_settlement.rs) |

---

## 7. Hardening Recommendations

### Priority 1 — Must Fix Before Devnet

1. **FIND-001/004**: Fix `StoreSnapshot` to cover pools and vaults. Ensure sync is called before any state root computation in CRX paths.
2. **FIND-002**: Add governance-gated `reset_emergency_mode()`. Document the recovery procedure.
3. **FIND-011**: Implement per-branch snapshot/restore for quarantine mode.
4. **FIND-015**: Fix `plan_action_to_operation()` to carry `delta_b`. Add `delta_b: Option<i128>` to `PlanAction` or encode in metadata.
5. **FIND-006**: Change empty `allowed_object_ids` semantics to DENY ALL. Add explicit `unrestricted` flag.
6. **FIND-009/010**: Populate `affected_domains` in `SafetyEvaluationContext::from_crx_record()`.

### Priority 2 — Must Fix Before Internal Testnet

7. **FIND-003**: Remove double-call to `record_submission` for winners.
8. **FIND-005**: Enforce `authorized_solver_id` check in `RightsValidationEngine::validate()`.
9. **FIND-007**: Cap gas_limit at `MAX_GAS_PER_TX` constant.
10. **FIND-013**: Include all fields in `CandidatePlan::compute_hash()`.
11. **FIND-014**: Define provisional settlement confirmation/revert flow.
12. **FIND-018**: Use `checked_add` in `CreditBalance` apply_op.
13. **FIND-022**: Fix `RepeatedRightsViolations` to use per-solver counters.

### Priority 3 — Must Fix Before Mainnet Hardening

14. **FIND-008**: Normalize `object_access` deduplication in scheduler.
15. **FIND-012**: Sync typed-to-canonical after `FinalizeSettlement` version increments.
16. **FIND-016**: Implement `HiddenObjectAccess` check in causal validity engine.
17. **FIND-017**: Sort receipts by `tx_id` before returning `SettlementResult`.
18. **FIND-019**: Replace `saturating_sub` with `checked_sub` in `UnlockObject` execution.
19. **FIND-020**: Fix `SolverInvalidPlanRateExceeds` condition logic.
20. **FIND-026**: Propagate ledger overflow error rather than ignoring it.

---

## 8. Code Quality Assessment

| Phase / Module | Score | Notes |
|---|---|---|
| Phase 1 — ObjectStore / Scheduler / Settlement | **7/10** | Good: U256 AMM, dry-run atomicity, BTreeMap determinism. Issues: saturating_add on credit, gas cap via saturating_mul |
| Phase 1 — IntentResolver / GasMeter | **7/10** | Clean and well-structured. Gas meter correctly uses checked_add. |
| Phase 2 — Solver Market / Plan Validation / Selection | **6/10** | Good selection policy determinism. Issues: double reputation counting, hash excluding metadata, delta_b loss |
| Phase 3 — CRX Goal Packets / Rights Capsule | **6/10** | Well-designed abstraction. Issues: empty=unrestricted semantics, solver identity not checked |
| Phase 3 — CRX Causal Validity / Branch Execution | **5/10** | Good graph structure checking. Issues: HiddenObjectAccess unimplemented, quarantine no per-branch revert, sync missing |
| Phase 3 — CRX Settlement / Finality | **5/10** | Finality classification is clean. Issues: snapshot incomplete, provisional has no follow-up, pool revert missing |
| Phase 4 — Safety Kernel / Policies | **5/10** | Good rule engine architecture. Issues: emergency_mode no reset, affected_domains always empty, rule conditions buggy |
| Phase 4 — PoSeq Safety Bridge | **5/10** | Clean integration pattern. Issues: domain pause check bypassed, context building incomplete |

---

## 9. Final Verdict

**Early Prototype**

The codebase demonstrates solid architectural thinking and good Rust idioms throughout. Phase 1 (object store, scheduler, gas metering) is close to devnet-ready quality. However, Phases 3 and 4 have multiple Critical and High severity bugs that cause incorrect state transitions, fund loss, and chain halt vectors. Several of these bugs are in core safety infrastructure (the very code intended to prevent exploits). The invariant gaps — particularly around snapshot/restore correctness, emergency mode recovery, and domain pause enforcement — would be exploited immediately in any adversarial environment.

Recommended next steps: resolve all Priority 1 items and run the full invariant test suite before any external devnet deployment.
