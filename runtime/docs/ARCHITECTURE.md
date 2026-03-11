# Omniphi Object + Intent Runtime — Architecture

## 1. Object Model

Every piece of on-chain state is an **Object**: a typed, versioned, owner-tagged value stored in the `ObjectStore`.

| Concrete Type              | Purpose                                              |
|----------------------------|------------------------------------------------------|
| `WalletObject`             | Account with authorized signing keys and a nonce     |
| `BalanceObject`            | Fungible asset holding for one owner / asset pair    |
| `TokenObject`              | Asset metadata: symbol, decimals, supply, authority  |
| `LiquidityPoolObject`      | AMM pool: reserves, fee bps, LP token id             |
| `VaultObject`              | Yield vault: deposited amount, strategy, lock-until  |
| `GovernanceProposalObject` | On-chain proposal with yes/no/abstain vote tallies   |
| `IdentityObject`           | KYC/reputation record for an address                 |
| `ExecutionReceiptObject`   | Immutable audit trail of a settled transaction       |

Every object carries an `ObjectMeta` with:
- `id: ObjectId` — 32-byte unique identifier (displayed as hex)
- `object_type: ObjectType` — discriminant for store lookups
- `owner: [u8; 32]` — the controlling address
- `version: u64` — incremented on every successful mutation
- `created_at / updated_at: u64` — epoch timestamps

Objects implement the `Object` trait which requires:
- `meta() / meta_mut()` — metadata access
- `required_capabilities_for_write()` — capability guard
- `encode()` — deterministic serde_json serialisation (used for state root)

## 2. Intent Model

Transactions are expressed as **intents** — semantic descriptions of desired state transitions rather than raw opcodes.

```
IntentTransaction {
    tx_id, sender, intent: IntentType, max_fee,
    deadline_epoch, nonce, signature, metadata
}
```

`IntentType` variants:
- `Transfer` — debit sender balance, credit recipient balance
- `Swap` — constant-product AMM swap through a `LiquidityPoolObject`
- `YieldAllocate` — lock sender balance and record deposit into a vault
- `TreasuryRebalance` — governed multi-authority cross-asset rebalance

Structural validation (`IntentTransaction::validate`) checks:
- Non-zero sender, tx_id, max_fee
- Zero-amount and self-swap guards
- Slippage bps ≤ 10 000

## 3. Capability System

Access to mutating operations is gated by a **capability set** — a `BTreeSet<Capability>` for deterministic serialisation.

| Capability          | Grants                                             |
|---------------------|----------------------------------------------------|
| `ReadObject`        | Read-only access to any object                     |
| `WriteObject`       | Base write gate (required for all mutations)       |
| `TransferAsset`     | Debit / credit `BalanceObject`                     |
| `SwapAsset`         | Interact with `LiquidityPoolObject`                |
| `ProvideLiquidity`  | Modify pool reserves                               |
| `WithdrawLiquidity` | Remove liquidity from a pool                       |
| `MintAsset`         | Increase `TokenObject.total_supply`                |
| `BurnAsset`         | Decrease `TokenObject.total_supply`                |
| `ModifyGovernance`  | Create or vote on `GovernanceProposalObject`       |
| `UpdateIdentity`    | Mutate `IdentityObject`                            |

`CapabilitySet::all()` = admin set; `CapabilitySet::user_default()` = `{ReadObject, WriteObject, TransferAsset, SwapAsset}`.

`CapabilityChecker::check_object_write` first verifies `WriteObject` is held, then verifies the per-type required capabilities returned by `required_capabilities_for_write()`.

## 4. Resolution Engine

`IntentResolver::resolve` converts an `IntentTransaction` into an `ExecutionPlan`:

```
ExecutionPlan {
    tx_id,
    operations: Vec<ObjectOperation>,
    required_capabilities: Vec<Capability>,
    object_access: Vec<ObjectAccess>,   // id + ReadOnly|ReadWrite
}
```

`ObjectOperation` variants:
- `DebitBalance { balance_id, amount }`
- `CreditBalance { balance_id, amount }`
- `SwapPoolAmounts { pool_id, delta_a: i128, delta_b: i128 }`
- `LockBalance { balance_id, amount }`
- `UnlockBalance { balance_id, amount }`
- `UpdateVersion { object_id }`

**Swap formula** (constant product AMM, integer only, no floats):
```
output = (input_amount * (10000 - fee_bps) * reserve_out)
       / (reserve_in * 10000 + input_amount * (10000 - fee_bps))
```
Slippage is enforced in basis points against the ideal (fee-free) output.

## 5. Parallel Scheduler

`ParallelScheduler::schedule` takes plans in PoSeq canonical order and returns `Vec<ExecutionGroup>` using **greedy graph coloring**:

1. Build a `ConflictGraph`: two plans conflict when their `object_access` sets share an `ObjectId` with at least one `ReadWrite` side.
2. Iterate plans in PoSeq order; assign each to the first group with no conflict.
3. Plans within the same group have no mutual conflicts → safe to execute in parallel.
4. Groups must be executed in strictly ascending `group_index` order.

This preserves PoSeq ordering (serialisability) while maximising intra-epoch parallelism.

## 6. PoSeq Integration

`PoSeqRuntime` is the top-level engine. It:
- Holds an `ObjectStore` (single source of truth)
- Accepts `OrderedBatch` values — pre-ordered by the PoSeq sequencer
- Advances `current_epoch` to match each batch

`OrderedBatch` fields:
```
batch_id: [u8; 32]
epoch: u64
sequence_number: u64
transactions: Vec<IntentTransaction>   // already PoSeq-ordered
```

The scheduler's group output is deterministic given the same ordered input, so any validator re-executing the batch produces the same state root.

## 7. Execution Lifecycle (9 Steps)

```
process_batch(OrderedBatch) → SettlementResult
```

1. **Structural validation** — `IntentTransaction::validate()` on every tx; invalid txs are skipped.
2. **Resolution** — `IntentResolver::resolve` converts each valid tx to an `ExecutionPlan`. Failed resolutions are skipped; they will appear in `SettlementResult.failed`.
3. **Access map** — embedded in each `ExecutionPlan.object_access`.
4. **Scheduling** — `ParallelScheduler::schedule(plans)` groups plans by conflict-free sets.
5. **Settlement** — `SettlementEngine::execute_groups` iterates groups in order.
   - Per plan: snapshot old versions → validate all ops (dry run) → apply ops → bump versions → emit receipt.
   - Atomicity: if any validation step fails, no state is mutated for that plan.
6. **Epoch advance** — `current_epoch = batch.epoch`.
7. **Typed overlay sync** — `ObjectStore::sync_typed_to_canonical()` flushes mutated typed objects back to the canonical `BTreeMap`.
8. **State root** — SHA256 of deterministically serialised (sorted by `ObjectId`) store.
9. **Return** — `SettlementResult { epoch, total_plans, succeeded, failed, receipts, state_root }`.

## 8. Extension Points

| Area | Hook |
|------|------|
| Object quarantine | `RuntimeError::ObjectQuarantined(ObjectId)` — extend `SettlementEngine::validate_op` to check a quarantine registry |
| Domain pause | `RuntimeError::DomainPaused(String)` — add a domain pause registry queried at the start of resolution |
| Per-sender capability lookup | Replace `CapabilitySet::all()` in `PoSeqRuntime::process_batch` with a lookup keyed on `tx.sender` |
| Signature verification | `IntentTransaction::validate()` currently skips sig checks; add Ed25519 verification against `sender` + `authorized_keys` |
| Rayon parallelism | Within each `ExecutionGroup`, plans are conflict-free; replace sequential loop with `rayon::par_iter()` over a snapshot-copy approach or per-object locks |
| Persistent state | Replace `ObjectStore`'s in-memory `BTreeMap` with an IAVL-backed store (mirroring the Go `chain/` module pattern) |
| Receipt objects | After settlement, materialise `ExecutionReceiptObject` values and insert them into the store for auditability |
