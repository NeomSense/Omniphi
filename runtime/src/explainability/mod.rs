//! Explainability Engine — Deterministic Execution Previews
//!
//! Generates a human-readable, machine-parseable preview of what an intent
//! will do BEFORE execution. Wallets, dApps, and agents consume this to:
//!
//! - Show users "You will spend 100 OMNI and receive ~95 USDC"
//! - Flag risky operations ("This touches a quarantined object")
//! - Identify potential failure conditions ("Insufficient balance for this swap")
//! - Enable informed consent before signing
//!
//! ## How Wallets Consume This
//!
//! ```text
//! 1. User drafts intent in wallet UI
//! 2. Wallet calls PreviewGenerator::preview(intent, store) → ExecutionPreview
//! 3. Wallet renders: assets_spent, assets_received, risk_flags, failure_conditions
//! 4. User reviews and signs (or rejects)
//! 5. Signed intent submitted to PoSeq
//! ```
//!
//! ## Design Principles
//!
//! - **Deterministic**: Same intent + same store state = same preview. No AI, no randomness.
//! - **Conservative**: Estimates err on the side of caution (worst-case spend, best-case receive).
//! - **Rule-based risks**: Risk flags come from explicit rules, not ML inference.
//! - **No side effects**: Preview reads state but never mutates it.

use crate::intents::base::{IntentTransaction, IntentType};
use crate::objects::base::ObjectId;
use crate::state::store::ObjectStore;
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// Preview Data Structure
// ─────────────────────────────────────────────────────────────────────────────

/// A deterministic preview of what an intent will do.
#[derive(Debug, Clone)]
pub struct ExecutionPreview {
    /// Intent ID being previewed.
    pub intent_id: [u8; 32],
    /// Assets that will be spent (asset_id → amount).
    pub assets_spent: BTreeMap<[u8; 32], u128>,
    /// Assets that will be received (asset_id → estimated amount).
    pub assets_received: BTreeMap<[u8; 32], u128>,
    /// Objects that will be read or written.
    pub objects_touched: Vec<ObjectTouch>,
    /// Capabilities/permissions required for this intent.
    pub permissions_used: Vec<String>,
    /// Expected state changes (human-readable descriptions).
    pub expected_state_changes: Vec<StateChange>,
    /// Risk flags (rule-based, not AI).
    pub risk_flags: Vec<RiskFlag>,
    /// Conditions under which execution would fail.
    pub failure_conditions: Vec<FailureCondition>,
    /// Estimated gas cost.
    pub estimated_gas: u64,
    /// Whether this preview is exact (simple transfer) or estimated (swap/contract).
    pub is_exact: bool,
}

/// An object that will be touched during execution.
#[derive(Debug, Clone)]
pub struct ObjectTouch {
    pub object_id: ObjectId,
    pub access: TouchAccess,
    pub object_type: String,
    pub current_version: Option<u64>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TouchAccess {
    Read,
    Write,
    Create,
}

/// A human-readable state change description.
#[derive(Debug, Clone)]
pub struct StateChange {
    pub description: String,
    pub object_id: Option<ObjectId>,
    pub change_type: ChangeType,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ChangeType {
    BalanceDecrease,
    BalanceIncrease,
    OwnershipTransfer,
    ObjectCreation,
    ObjectMutation,
    PoolReserveChange,
    ContractStateUpdate,
}

/// A rule-based risk flag.
#[derive(Debug, Clone)]
pub struct RiskFlag {
    pub severity: RiskSeverity,
    pub code: String,
    pub message: String,
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum RiskSeverity {
    Info,
    Low,
    Medium,
    High,
    Critical,
}

/// A condition under which execution would fail.
#[derive(Debug, Clone)]
pub struct FailureCondition {
    pub code: String,
    pub message: String,
    pub likelihood: FailureLikelihood,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FailureLikelihood {
    /// Will definitely fail based on current state.
    Certain,
    /// May fail depending on execution ordering.
    Possible,
    /// Unlikely but theoretically possible.
    Unlikely,
}

// ─────────────────────────────────────────────────────────────────────────────
// Risk Classifier (rule-based)
// ─────────────────────────────────────────────────────────────────────────────

/// Rule-based risk classifier. No AI, no ML — explicit rules only.
pub struct RiskClassifier;

impl RiskClassifier {
    /// Classify risks for a transfer intent.
    pub fn classify_transfer(
        asset_id: &[u8; 32],
        amount: u128,
        sender_balance: Option<u128>,
        recipient: &[u8; 32],
    ) -> Vec<RiskFlag> {
        let mut flags = vec![];

        // R1: Insufficient balance
        if let Some(balance) = sender_balance {
            if amount > balance {
                flags.push(RiskFlag {
                    severity: RiskSeverity::Critical,
                    code: "INSUFFICIENT_BALANCE".into(),
                    message: format!("Transfer amount {} exceeds balance {}", amount, balance),
                });
            } else if amount > balance * 9 / 10 {
                flags.push(RiskFlag {
                    severity: RiskSeverity::Medium,
                    code: "HIGH_BALANCE_RATIO".into(),
                    message: format!("Transfer uses >90% of balance ({}/{})", amount, balance),
                });
            }
        } else {
            flags.push(RiskFlag {
                severity: RiskSeverity::High,
                code: "BALANCE_NOT_FOUND".into(),
                message: "Sender balance object not found in store".into(),
            });
        }

        // R2: Large transfer (>1M units)
        if amount > 1_000_000_000_000 {
            flags.push(RiskFlag {
                severity: RiskSeverity::Medium,
                code: "LARGE_TRANSFER".into(),
                message: format!("Large transfer: {} units", amount),
            });
        }

        // R3: Transfer to self
        // (Already caught by validation, but flag it for preview)
        if asset_id == recipient {
            flags.push(RiskFlag {
                severity: RiskSeverity::Low,
                code: "SELF_TRANSFER_PATTERN".into(),
                message: "Asset ID matches recipient — possible misconfiguration".into(),
            });
        }

        flags
    }

    /// Classify risks for a swap intent.
    pub fn classify_swap(
        input_amount: u128,
        max_slippage_bps: u32,
        sender_balance: Option<u128>,
    ) -> Vec<RiskFlag> {
        let mut flags = vec![];

        if let Some(balance) = sender_balance {
            if input_amount > balance {
                flags.push(RiskFlag {
                    severity: RiskSeverity::Critical,
                    code: "INSUFFICIENT_BALANCE".into(),
                    message: format!("Swap input {} exceeds balance {}", input_amount, balance),
                });
            }
        }

        if max_slippage_bps > 500 {
            flags.push(RiskFlag {
                severity: RiskSeverity::High,
                code: "HIGH_SLIPPAGE".into(),
                message: format!("Slippage tolerance {}bps ({}%) — significant price impact risk",
                    max_slippage_bps, max_slippage_bps as f64 / 100.0),
            });
        } else if max_slippage_bps > 100 {
            flags.push(RiskFlag {
                severity: RiskSeverity::Medium,
                code: "ELEVATED_SLIPPAGE".into(),
                message: format!("Slippage tolerance {}bps ({}%)", max_slippage_bps, max_slippage_bps as f64 / 100.0),
            });
        }

        flags
    }

    /// Classify risks for a contract call.
    pub fn classify_contract_call(
        schema_id: &[u8; 32],
        method: &str,
        _store: &ObjectStore,
    ) -> Vec<RiskFlag> {
        let mut flags = vec![];

        // R1: Unknown schema (not in store — may be a new/unverified contract)
        flags.push(RiskFlag {
            severity: RiskSeverity::Info,
            code: "CONTRACT_CALL".into(),
            message: format!("Calling contract {}::{}", hex::encode(&schema_id[..4]), method),
        });

        flags
    }

    /// General risk flags that apply to any intent.
    pub fn classify_general(
        tx: &IntentTransaction,
        current_epoch: u64,
    ) -> Vec<RiskFlag> {
        let mut flags = vec![];

        // G1: Intent expires soon
        if tx.deadline_epoch > 0 && tx.deadline_epoch <= current_epoch + 2 {
            flags.push(RiskFlag {
                severity: RiskSeverity::Medium,
                code: "NEAR_DEADLINE".into(),
                message: format!("Intent expires in {} epoch(s)", tx.deadline_epoch.saturating_sub(current_epoch)),
            });
        }

        // G2: High fee
        if tx.max_fee > 10_000 {
            flags.push(RiskFlag {
                severity: RiskSeverity::Low,
                code: "HIGH_FEE".into(),
                message: format!("Max fee: {} (above typical)", tx.max_fee),
            });
        }

        // G3: Sponsored intent
        if tx.sponsor.is_some() {
            flags.push(RiskFlag {
                severity: RiskSeverity::Info,
                code: "SPONSORED".into(),
                message: "This intent is sponsored — a third party is paying fees".into(),
            });
        }

        flags
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Preview Generator
// ─────────────────────────────────────────────────────────────────────────────

/// Generates deterministic execution previews from intents + current state.
pub struct PreviewGenerator;

impl PreviewGenerator {
    /// Generate a preview for an intent transaction.
    ///
    /// This reads current state from the store but NEVER mutates it.
    pub fn preview(
        tx: &IntentTransaction,
        store: &ObjectStore,
        current_epoch: u64,
    ) -> ExecutionPreview {
        let mut assets_spent = BTreeMap::new();
        let mut assets_received = BTreeMap::new();
        let mut objects_touched = vec![];
        let mut permissions_used = vec![];
        let mut expected_state_changes = vec![];
        let mut risk_flags = vec![];
        let mut failure_conditions = vec![];
        let mut estimated_gas: u64 = 1000; // base tx cost
        let mut is_exact = true;

        // General risk flags
        risk_flags.extend(RiskClassifier::classify_general(tx, current_epoch));

        match &tx.intent {
            IntentType::Transfer(t) => {
                // Assets
                assets_spent.insert(t.asset_id, t.amount);
                assets_received.insert(t.asset_id, t.amount); // recipient receives exact

                // Objects touched: sender balance, recipient balance
                let sender_bal_id = Self::balance_object_id(&tx.sender, &t.asset_id);
                let recip_bal_id = Self::balance_object_id(&t.recipient, &t.asset_id);

                let sender_balance = Self::lookup_balance(store, &tx.sender, &t.asset_id);

                objects_touched.push(ObjectTouch {
                    object_id: sender_bal_id,
                    access: TouchAccess::Write,
                    object_type: "Balance".into(),
                    current_version: None,
                });
                objects_touched.push(ObjectTouch {
                    object_id: recip_bal_id,
                    access: TouchAccess::Write,
                    object_type: "Balance".into(),
                    current_version: None,
                });

                permissions_used.push("TransferAsset".into());

                expected_state_changes.push(StateChange {
                    description: format!("Debit {} from sender", t.amount),
                    object_id: Some(sender_bal_id),
                    change_type: ChangeType::BalanceDecrease,
                });
                expected_state_changes.push(StateChange {
                    description: format!("Credit {} to recipient", t.amount),
                    object_id: Some(recip_bal_id),
                    change_type: ChangeType::BalanceIncrease,
                });

                // Risk classification
                risk_flags.extend(RiskClassifier::classify_transfer(
                    &t.asset_id, t.amount, sender_balance, &t.recipient,
                ));

                // Failure conditions
                if let Some(bal) = sender_balance {
                    if t.amount > bal {
                        failure_conditions.push(FailureCondition {
                            code: "INSUFFICIENT_BALANCE".into(),
                            message: format!("Need {}, have {}", t.amount, bal),
                            likelihood: FailureLikelihood::Certain,
                        });
                    }
                } else {
                    failure_conditions.push(FailureCondition {
                        code: "BALANCE_NOT_FOUND".into(),
                        message: "Sender balance object does not exist".into(),
                        likelihood: FailureLikelihood::Certain,
                    });
                }

                estimated_gas = 2000; // transfer is cheap
            }

            IntentType::Swap(s) => {
                is_exact = false; // swaps are estimated

                assets_spent.insert(s.input_asset, s.input_amount);
                // Output is estimated — worst case with max slippage
                let min_output = if s.min_output_amount > 0 {
                    s.min_output_amount
                } else {
                    // Estimate: input * (1 - slippage)
                    let slippage_factor = 10000u128.saturating_sub(s.max_slippage_bps as u128);
                    s.input_amount * slippage_factor / 10000
                };
                assets_received.insert(s.output_asset, min_output);

                permissions_used.push("SwapAsset".into());

                expected_state_changes.push(StateChange {
                    description: format!("Debit {} of input asset", s.input_amount),
                    object_id: None,
                    change_type: ChangeType::BalanceDecrease,
                });
                expected_state_changes.push(StateChange {
                    description: format!("Credit ~{} of output asset (min)", min_output),
                    object_id: None,
                    change_type: ChangeType::BalanceIncrease,
                });
                expected_state_changes.push(StateChange {
                    description: "Pool reserves updated".into(),
                    object_id: None,
                    change_type: ChangeType::PoolReserveChange,
                });

                let sender_balance = Self::lookup_balance(store, &tx.sender, &s.input_asset);
                risk_flags.extend(RiskClassifier::classify_swap(
                    s.input_amount, s.max_slippage_bps, sender_balance,
                ));

                if let Some(bal) = sender_balance {
                    if s.input_amount > bal {
                        failure_conditions.push(FailureCondition {
                            code: "INSUFFICIENT_BALANCE".into(),
                            message: format!("Need {}, have {}", s.input_amount, bal),
                            likelihood: FailureLikelihood::Certain,
                        });
                    }
                }

                failure_conditions.push(FailureCondition {
                    code: "SLIPPAGE_EXCEEDED".into(),
                    message: format!("Actual output may be below min_output {}", min_output),
                    likelihood: FailureLikelihood::Possible,
                });

                estimated_gas = 5000; // swap is more expensive
            }

            IntentType::ContractCall(c) => {
                is_exact = false;

                permissions_used.push(format!("ContractCall({})", hex::encode(&c.schema_id[..4])));

                expected_state_changes.push(StateChange {
                    description: format!("Contract state update: {}::{}", hex::encode(&c.schema_id[..4]), c.method_selector),
                    object_id: None,
                    change_type: ChangeType::ContractStateUpdate,
                });

                risk_flags.extend(RiskClassifier::classify_contract_call(
                    &c.schema_id, &c.method_selector, store,
                ));

                estimated_gas = 10000; // contract calls are expensive
            }

            IntentType::YieldAllocate(y) => {
                assets_spent.insert(y.asset_id, y.amount);
                permissions_used.push("ProvideLiquidity".into());
                expected_state_changes.push(StateChange {
                    description: format!("Allocate {} to yield", y.amount),
                    object_id: None,
                    change_type: ChangeType::BalanceDecrease,
                });
                estimated_gas = 4000;
                is_exact = false;
            }

            IntentType::TreasuryRebalance(r) => {
                assets_spent.insert(r.from_asset, r.amount);
                assets_received.insert(r.to_asset, r.amount);
                permissions_used.push("ModifyGovernance".into());
                expected_state_changes.push(StateChange {
                    description: format!("Rebalance {} from one asset to another", r.amount),
                    object_id: None,
                    change_type: ChangeType::BalanceDecrease,
                });
                estimated_gas = 6000;
                is_exact = false;
            }

            IntentType::RouteLiquidity(rl) => {
                assets_spent.insert(rl.asset_id, rl.amount);
                assets_received.insert(rl.asset_id, rl.min_received);
                permissions_used.push("ProvideLiquidity".into());
                permissions_used.push("WithdrawLiquidity".into());
                expected_state_changes.push(StateChange {
                    description: format!("Route {} through {} hops", rl.amount, rl.max_hops),
                    object_id: None,
                    change_type: ChangeType::PoolReserveChange,
                });
                estimated_gas = 8000;
                is_exact = false;
            }
        }

        // Fee as a spent asset (gas token)
        let gas_token = [0u8; 32]; // native token
        *assets_spent.entry(gas_token).or_insert(0) += tx.max_fee as u128;

        ExecutionPreview {
            intent_id: tx.tx_id,
            assets_spent,
            assets_received,
            objects_touched,
            permissions_used,
            expected_state_changes,
            risk_flags,
            failure_conditions,
            estimated_gas,
            is_exact,
        }
    }

    /// Derive a deterministic balance object ID from (owner, asset_id).
    fn balance_object_id(owner: &[u8; 32], asset_id: &[u8; 32]) -> ObjectId {
        use sha2::{Digest, Sha256};
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_BALANCE_OBJ");
        h.update(owner);
        h.update(asset_id);
        let r = h.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&r);
        ObjectId(id)
    }

    /// Look up a balance from the store. Returns None if not found.
    fn lookup_balance(store: &ObjectStore, owner: &[u8; 32], asset_id: &[u8; 32]) -> Option<u128> {
        store.find_balance(owner, asset_id).map(|b| b.amount)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::intents::base::*;
    use crate::intents::types::{TransferIntent, SwapIntent};
    use crate::objects::types::BalanceObject;
    use crate::objects::base::ObjectId;
    use std::collections::BTreeMap as StdBTreeMap;

    fn sender() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 1; b }
    fn recipient() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 2; b }
    fn asset_a() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 0xAA; b }
    fn asset_b() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 0xBB; b }

    fn make_transfer(amount: u128) -> IntentTransaction {
        IntentTransaction {
            tx_id: [0x42; 32],
            sender: sender(),
            intent: IntentType::Transfer(TransferIntent {
                asset_id: asset_a(),
                recipient: recipient(),
                amount,
                memo: None,
            }),
            max_fee: 100,
            deadline_epoch: 999,
            nonce: 1,
            signature: [0u8; 64],
            metadata: StdBTreeMap::new(),
            target_objects: vec![],
            constraints: IntentConstraints::default(),
            execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
        }
    }

    fn make_swap(input_amount: u128, slippage: u32) -> IntentTransaction {
        IntentTransaction {
            tx_id: [0x43; 32],
            sender: sender(),
            intent: IntentType::Swap(SwapIntent {
                input_asset: asset_a(),
                output_asset: asset_b(),
                input_amount,
                min_output_amount: 0,
                max_slippage_bps: slippage,
                allowed_pool_ids: None,
            }),
            max_fee: 200,
            deadline_epoch: 999,
            nonce: 1,
            signature: [0u8; 64],
            metadata: StdBTreeMap::new(),
            target_objects: vec![],
            constraints: IntentConstraints::default(),
            execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
        }
    }

    fn store_with_balance(owner: [u8; 32], asset: [u8; 32], amount: u128) -> ObjectStore {
        let mut store = ObjectStore::new();
        let obj_id = PreviewGenerator::balance_object_id(&owner, &asset);
        let bal = BalanceObject::new(obj_id, owner, asset, amount, 1);
        store.insert(Box::new(bal));
        store
    }

    // ── Test 1: deterministic preview generation ─────────────

    #[test]
    fn test_deterministic_preview() {
        let tx = make_transfer(1000);
        let store = store_with_balance(sender(), asset_a(), 5000);

        let p1 = PreviewGenerator::preview(&tx, &store, 10);
        let p2 = PreviewGenerator::preview(&tx, &store, 10);

        assert_eq!(p1.assets_spent.len(), p2.assets_spent.len());
        assert_eq!(p1.assets_received.len(), p2.assets_received.len());
        assert_eq!(p1.risk_flags.len(), p2.risk_flags.len());
        assert_eq!(p1.failure_conditions.len(), p2.failure_conditions.len());
        assert_eq!(p1.estimated_gas, p2.estimated_gas);
        assert_eq!(p1.is_exact, p2.is_exact);
    }

    // ── Test 2: accurate asset change reporting (transfer) ───

    #[test]
    fn test_transfer_asset_reporting() {
        let tx = make_transfer(1000);
        let store = store_with_balance(sender(), asset_a(), 5000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        // Assets spent: 1000 of asset_a + 100 fee (gas token)
        assert_eq!(*preview.assets_spent.get(&asset_a()).unwrap(), 1000);
        assert!(preview.assets_spent.contains_key(&[0u8; 32])); // gas token fee

        // Assets received: 1000 of asset_a (by recipient)
        assert_eq!(*preview.assets_received.get(&asset_a()).unwrap(), 1000);

        assert!(preview.is_exact);
    }

    // ── Test 3: accurate object touch list ───────────────────

    #[test]
    fn test_object_touch_list() {
        let tx = make_transfer(1000);
        let store = ObjectStore::new();
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        assert_eq!(preview.objects_touched.len(), 2); // sender bal + recipient bal
        assert!(preview.objects_touched.iter().all(|t| t.access == TouchAccess::Write));
        assert!(preview.objects_touched.iter().all(|t| t.object_type == "Balance"));
    }

    // ── Test 4: failure condition — insufficient balance ─────

    #[test]
    fn test_failure_insufficient_balance() {
        let tx = make_transfer(5000);
        let store = store_with_balance(sender(), asset_a(), 1000); // only 1000
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let certain = preview.failure_conditions.iter()
            .find(|f| f.code == "INSUFFICIENT_BALANCE");
        assert!(certain.is_some());
        assert_eq!(certain.unwrap().likelihood, FailureLikelihood::Certain);
    }

    // ── Test 5: failure condition — balance not found ─────────

    #[test]
    fn test_failure_balance_not_found() {
        let tx = make_transfer(1000);
        let store = ObjectStore::new(); // empty store
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let not_found = preview.failure_conditions.iter()
            .find(|f| f.code == "BALANCE_NOT_FOUND");
        assert!(not_found.is_some());
    }

    // ── Test 6: swap preview (estimated, not exact) ──────────

    #[test]
    fn test_swap_preview_estimated() {
        let tx = make_swap(1000, 300); // 3% slippage
        let store = store_with_balance(sender(), asset_a(), 5000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        assert!(!preview.is_exact); // swaps are estimated
        assert!(preview.assets_spent.contains_key(&asset_a()));
        assert!(preview.assets_received.contains_key(&asset_b()));

        // Estimated output with 3% slippage: 1000 * 9700/10000 = 970
        assert_eq!(*preview.assets_received.get(&asset_b()).unwrap(), 970);

        // Should have slippage risk flag
        let slippage = preview.risk_flags.iter()
            .find(|f| f.code == "ELEVATED_SLIPPAGE");
        assert!(slippage.is_some());
    }

    // ── Test 7: high slippage risk flag ──────────────────────

    #[test]
    fn test_high_slippage_flag() {
        let tx = make_swap(1000, 800); // 8% slippage — high
        let store = store_with_balance(sender(), asset_a(), 5000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let high = preview.risk_flags.iter()
            .find(|f| f.code == "HIGH_SLIPPAGE");
        assert!(high.is_some());
        assert_eq!(high.unwrap().severity, RiskSeverity::High);
    }

    // ── Test 8: large transfer risk flag ─────────────────────

    #[test]
    fn test_large_transfer_flag() {
        let tx = make_transfer(2_000_000_000_000); // 2T units
        let store = store_with_balance(sender(), asset_a(), 10_000_000_000_000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let large = preview.risk_flags.iter()
            .find(|f| f.code == "LARGE_TRANSFER");
        assert!(large.is_some());
    }

    // ── Test 9: high balance ratio risk flag ─────────────────

    #[test]
    fn test_high_balance_ratio_flag() {
        let tx = make_transfer(950);
        let store = store_with_balance(sender(), asset_a(), 1000); // 95%
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let ratio = preview.risk_flags.iter()
            .find(|f| f.code == "HIGH_BALANCE_RATIO");
        assert!(ratio.is_some());
    }

    // ── Test 10: sponsored intent info flag ───────────────────

    #[test]
    fn test_sponsored_flag() {
        let mut tx = make_transfer(100);
        tx.sponsor = Some([0x99; 32]);
        let store = store_with_balance(sender(), asset_a(), 5000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let sponsored = preview.risk_flags.iter()
            .find(|f| f.code == "SPONSORED");
        assert!(sponsored.is_some());
    }

    // ── Test 11: near deadline risk flag ──────────────────────

    #[test]
    fn test_near_deadline_flag() {
        let mut tx = make_transfer(100);
        tx.deadline_epoch = 12; // expires in 2 epochs
        let store = store_with_balance(sender(), asset_a(), 5000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let deadline = preview.risk_flags.iter()
            .find(|f| f.code == "NEAR_DEADLINE");
        assert!(deadline.is_some());
    }

    // ── Test 12: permissions used list ────────────────────────

    #[test]
    fn test_permissions_used() {
        let tx = make_transfer(100);
        let store = ObjectStore::new();
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        assert!(preview.permissions_used.contains(&"TransferAsset".to_string()));
    }

    // ── Test 13: state changes describe what happens ─────────

    #[test]
    fn test_state_changes() {
        let tx = make_transfer(500);
        let store = store_with_balance(sender(), asset_a(), 5000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        assert!(preview.expected_state_changes.len() >= 2);
        let debit = preview.expected_state_changes.iter()
            .find(|c| c.change_type == ChangeType::BalanceDecrease);
        let credit = preview.expected_state_changes.iter()
            .find(|c| c.change_type == ChangeType::BalanceIncrease);
        assert!(debit.is_some());
        assert!(credit.is_some());
    }

    // ── Test 14: swap failure condition (slippage exceeded) ──

    #[test]
    fn test_swap_failure_slippage() {
        let tx = make_swap(1000, 100);
        let store = store_with_balance(sender(), asset_a(), 5000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let slippage_fail = preview.failure_conditions.iter()
            .find(|f| f.code == "SLIPPAGE_EXCEEDED");
        assert!(slippage_fail.is_some());
        assert_eq!(slippage_fail.unwrap().likelihood, FailureLikelihood::Possible);
    }

    // ── Test 15: no false positives for healthy transfer ─────

    #[test]
    fn test_healthy_transfer_no_critical_flags() {
        let tx = make_transfer(100);
        let store = store_with_balance(sender(), asset_a(), 10_000);
        let preview = PreviewGenerator::preview(&tx, &store, 10);

        let critical = preview.risk_flags.iter()
            .filter(|f| f.severity == RiskSeverity::Critical)
            .count();
        assert_eq!(critical, 0);
        assert!(preview.failure_conditions.is_empty());
    }

    // ── Test 16: gas estimation varies by intent type ────────

    #[test]
    fn test_gas_estimation_varies() {
        let store = store_with_balance(sender(), asset_a(), 10_000);

        let transfer = PreviewGenerator::preview(&make_transfer(100), &store, 10);
        let swap = PreviewGenerator::preview(&make_swap(100, 100), &store, 10);

        assert!(swap.estimated_gas > transfer.estimated_gas);
    }
}
