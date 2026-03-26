use crate::errors::RuntimeError;
use crate::intents::types::{
    RouteLiquidityIntent, SwapIntent, TransferIntent, TreasuryRebalanceIntent,
    YieldAllocateIntent,
};
use crate::objects::base::ObjectId;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// Constraints specific to a contract call intent.
#[derive(Debug, Clone)]
pub struct ContractConstraints {
    /// Maximum bytes of state delta the solver may propose.
    pub max_state_delta_bytes: u64,
    /// If set, the intent is only valid against these exact object versions.
    /// Prevents stale-state execution.
    pub required_object_versions: Vec<(ObjectId, u64)>,
    /// Contract-specific constraint parameters (opaque, passed to validator).
    pub custom_constraints: Vec<u8>,
}

/// An intent to invoke a deployed Intent Contract.
#[derive(Debug, Clone)]
pub struct ContractCallIntent {
    /// The schema ID of the target contract.
    pub schema_id: [u8; 32],
    /// The method selector within the contract (matches an intent schema name).
    pub method_selector: String,
    /// Arbitrary parameters for the contract method.
    pub params: BTreeMap<String, Vec<u8>>,
    /// Constraints on the execution.
    pub constraints: ContractConstraints,
}

/// The variant of intent contained in this transaction.
#[derive(Debug, Clone)]
pub enum IntentType {
    Transfer(TransferIntent),
    Swap(SwapIntent),
    YieldAllocate(YieldAllocateIntent),
    TreasuryRebalance(TreasuryRebalanceIntent),
    RouteLiquidity(RouteLiquidityIntent),
    /// A call to a deployed Intent Contract.
    ContractCall(ContractCallIntent),
}

/// How the intent should be executed by solvers.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ExecutionMode {
    /// Best effort — solver picks optimal plan (default).
    BestEffort,
    /// Exact — solver must match constraints precisely or reject.
    Exact,
    /// Fill-or-kill — execute fully or not at all (no partial fills).
    FillOrKill,
    /// Partial — allow partial execution if constraints allow.
    Partial,
}

impl Default for ExecutionMode {
    fn default() -> Self { ExecutionMode::BestEffort }
}

/// Universal constraints applicable to any intent type.
#[derive(Debug, Clone, Default)]
pub struct IntentConstraints {
    /// Maximum amount the sender is willing to spend (any asset).
    pub max_spend: Option<u128>,
    /// Minimum amount the sender expects to receive (any asset).
    pub min_received: Option<u128>,
    /// Maximum slippage in basis points (0-10000).
    pub max_slippage_bps: Option<u32>,
    /// Restrict execution to these specific object IDs only.
    pub allowed_objects: Vec<ObjectId>,
    /// Require these exact object versions (stale-state protection).
    pub required_versions: Vec<(ObjectId, u64)>,
    /// Execution path hints for solvers (informational, not enforced).
    pub path_hints: Vec<String>,
}

/// A fully described, signed intent transaction.
#[derive(Debug, Clone)]
pub struct IntentTransaction {
    pub tx_id: [u8; 32],
    pub sender: [u8; 32],
    pub intent: IntentType,
    pub max_fee: u64,
    pub deadline_epoch: u64,
    pub nonce: u64,
    pub signature: [u8; 64],
    pub metadata: BTreeMap<String, String>,
    /// Objects this intent will touch (explicit declaration for access control).
    pub target_objects: Vec<ObjectId>,
    /// Universal constraints checked during admission.
    pub constraints: IntentConstraints,
    /// How the solver should execute this intent.
    pub execution_mode: ExecutionMode,
}

impl IntentTransaction {
    /// Structural validation only — no state access required.
    pub fn validate(&self) -> Result<(), RuntimeError> {
        // Sender must be non-zero
        if self.sender == [0u8; 32] {
            return Err(RuntimeError::InvalidIntent(
                "sender address must be non-zero".to_string(),
            ));
        }
        // tx_id must be non-zero
        if self.tx_id == [0u8; 32] {
            return Err(RuntimeError::InvalidIntent(
                "tx_id must be non-zero".to_string(),
            ));
        }
        // max_fee must be positive
        if self.max_fee == 0 {
            return Err(RuntimeError::InvalidIntent(
                "max_fee must be greater than zero".to_string(),
            ));
        }

        // Intent-specific structural checks
        match &self.intent {
            IntentType::Transfer(t) => {
                if t.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "transfer amount must be greater than zero".to_string(),
                    ));
                }
                if t.recipient == [0u8; 32] {
                    return Err(RuntimeError::InvalidIntent(
                        "recipient address must be non-zero".to_string(),
                    ));
                }
                if t.asset_id == [0u8; 32] {
                    return Err(RuntimeError::InvalidIntent(
                        "asset_id must be non-zero".to_string(),
                    ));
                }
            }
            IntentType::Swap(s) => {
                if s.input_amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "swap input_amount must be greater than zero".to_string(),
                    ));
                }
                if s.input_asset == s.output_asset {
                    return Err(RuntimeError::InvalidIntent(
                        "swap input_asset and output_asset must differ".to_string(),
                    ));
                }
                if s.max_slippage_bps > 10000 {
                    return Err(RuntimeError::InvalidIntent(
                        "max_slippage_bps must not exceed 10000 (100%)".to_string(),
                    ));
                }
            }
            IntentType::YieldAllocate(y) => {
                if y.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "yield allocate amount must be greater than zero".to_string(),
                    ));
                }
            }
            IntentType::TreasuryRebalance(r) => {
                if r.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "treasury rebalance amount must be greater than zero".to_string(),
                    ));
                }
                if r.authorized_by.is_empty() {
                    return Err(RuntimeError::InvalidIntent(
                        "treasury rebalance requires at least one authority".to_string(),
                    ));
                }
                if r.from_asset == r.to_asset {
                    return Err(RuntimeError::InvalidIntent(
                        "treasury rebalance from_asset and to_asset must differ".to_string(),
                    ));
                }
            }
            IntentType::RouteLiquidity(rl) => {
                if rl.amount == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity amount must be greater than zero".to_string(),
                    ));
                }
                if rl.min_received == 0 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity min_received must be greater than zero".to_string(),
                    ));
                }
                if rl.source_pool == rl.target_pool {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity source and target pools must differ".to_string(),
                    ));
                }
                if rl.max_hops == 0 || rl.max_hops > 5 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity max_hops must be 1-5".to_string(),
                    ));
                }
                if rl.max_price_impact_bps > 500 {
                    return Err(RuntimeError::InvalidIntent(
                        "route liquidity max_price_impact_bps must not exceed 500".to_string(),
                    ));
                }
            }
            IntentType::ContractCall(c) => {
                if c.schema_id == [0u8; 32] {
                    return Err(RuntimeError::InvalidIntent(
                        "contract call schema_id must be non-zero".to_string(),
                    ));
                }
                if c.method_selector.is_empty() {
                    return Err(RuntimeError::InvalidIntent(
                        "contract call method_selector must not be empty".to_string(),
                    ));
                }
            }
        }

        Ok(())
    }

    /// Compute the deterministic canonical byte representation of the intent payload.
    ///
    /// Used as input to signature verification. Each intent variant is prefixed
    /// with a single discriminant byte for domain separation.
    pub fn intent_canonical_bytes(&self) -> Vec<u8> {
        let mut buf = Vec::new();
        match &self.intent {
            IntentType::Transfer(t) => {
                buf.push(0x01);
                buf.extend_from_slice(&t.asset_id);
                buf.extend_from_slice(&t.amount.to_be_bytes());
                buf.extend_from_slice(&t.recipient);
            }
            IntentType::Swap(s) => {
                buf.push(0x02);
                buf.extend_from_slice(&s.input_asset);
                buf.extend_from_slice(&s.output_asset);
                buf.extend_from_slice(&s.input_amount.to_be_bytes());
                buf.extend_from_slice(&s.min_output_amount.to_be_bytes());
                buf.extend_from_slice(&s.max_slippage_bps.to_be_bytes());
            }
            IntentType::YieldAllocate(y) => {
                buf.push(0x03);
                buf.extend_from_slice(&y.asset_id);
                buf.extend_from_slice(&y.amount.to_be_bytes());
                buf.extend_from_slice(&(y.target_vault_id.0));
                buf.extend_from_slice(&y.min_apy_bps.to_be_bytes());
            }
            IntentType::TreasuryRebalance(r) => {
                buf.push(0x04);
                buf.extend_from_slice(&r.from_asset);
                buf.extend_from_slice(&r.to_asset);
                buf.extend_from_slice(&r.amount.to_be_bytes());
                buf.extend_from_slice(&(r.authorized_by.len() as u32).to_be_bytes());
                for auth in &r.authorized_by {
                    buf.extend_from_slice(auth);
                }
            }
            IntentType::RouteLiquidity(rl) => {
                buf.push(0x05);
                buf.extend_from_slice(&(rl.source_pool.0));
                buf.extend_from_slice(&(rl.target_pool.0));
                buf.extend_from_slice(&rl.asset_id);
                buf.extend_from_slice(&rl.amount.to_be_bytes());
                buf.extend_from_slice(&rl.min_received.to_be_bytes());
                buf.push(rl.max_hops);
                buf.extend_from_slice(&rl.max_price_impact_bps.to_be_bytes());
            }
            IntentType::ContractCall(c) => {
                buf.push(0x06);
                buf.extend_from_slice(&c.schema_id);
                buf.extend_from_slice(c.method_selector.as_bytes());
                buf.extend_from_slice(&c.constraints.max_state_delta_bytes.to_be_bytes());
                buf.extend_from_slice(&c.constraints.custom_constraints);
            }
        }
        buf
    }

    /// Compute the signing payload for this transaction.
    ///
    /// payload = SHA256("OMNIPHI_INTENT_V1" || tx_id || sender || nonce_be
    ///                   || intent_bytes || max_fee_be || deadline_epoch_be)
    pub fn signing_payload(&self) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_INTENT_V1");
        h.update(&self.tx_id);
        h.update(&self.sender);
        h.update(&self.nonce.to_be_bytes());
        h.update(&self.intent_canonical_bytes());
        h.update(&self.max_fee.to_be_bytes());
        h.update(&self.deadline_epoch.to_be_bytes());
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Verify the transaction signature using Ed25519.
    ///
    /// The `sender` field is the 32-byte Ed25519 public key. The `signature`
    /// field is the 64-byte Ed25519 signature over `signing_payload()`.
    pub fn verify_signature(&self) -> bool {
        use ed25519_dalek::{Signature, Verifier, VerifyingKey};

        let pubkey = match VerifyingKey::from_bytes(&self.sender) {
            Ok(k) => k,
            Err(_) => return false, // invalid public key
        };

        let sig = Signature::from_slice(&self.signature);
        let sig = match sig {
            Ok(s) => s,
            Err(_) => return false,
        };

        let payload = self.signing_payload();
        pubkey.verify(&payload, &sig).is_ok()
    }

    /// Sign this transaction with an Ed25519 secret key.
    ///
    /// The `signing_key` is a 32-byte Ed25519 seed. Returns the 64-byte
    /// signature. The caller should set `self.signature` to the result.
    pub fn sign(&self, signing_key: &[u8; 32]) -> [u8; 64] {
        use ed25519_dalek::{Signer, SigningKey};

        let key = SigningKey::from_bytes(signing_key);
        let payload = self.signing_payload();
        let sig = key.sign(&payload);
        sig.to_bytes()
    }

    /// Compute a test-only placeholder signature (for backward compatibility
    /// with tests that don't have real keypairs). Uses SHA256, NOT Ed25519.
    /// This will NOT pass `verify_signature()` — use only in scaffold tests
    /// that skip signature verification.
    #[cfg(test)]
    pub fn compute_placeholder_signature(&self) -> [u8; 64] {
        let payload = self.signing_payload();
        let mut h = Sha256::new();
        h.update(&payload);
        h.update(&self.sender);
        let r = h.finalize();
        let mut sig = [0u8; 64];
        sig[0..32].copy_from_slice(&r);
        sig
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// NONCE TRACKER — Replay Protection
// ─────────────────────────────────────────────────────────────────────────────

/// Tracks per-sender nonces to prevent replay attacks.
#[derive(Debug, Clone, Default)]
pub struct NonceTracker {
    nonces: BTreeMap<[u8; 32], u64>,
}

impl NonceTracker {
    pub fn new() -> Self { NonceTracker::default() }

    pub fn expected_nonce(&self, sender: &[u8; 32]) -> u64 {
        self.nonces.get(sender).copied().unwrap_or(0)
    }

    pub fn is_valid(&self, sender: &[u8; 32], nonce: u64) -> bool {
        nonce == self.expected_nonce(sender)
    }

    pub fn advance(&mut self, sender: &[u8; 32], nonce: u64) -> Result<(), String> {
        let expected = self.expected_nonce(sender);
        if nonce != expected {
            return Err(format!("nonce mismatch: expected {}, got {}", expected, nonce));
        }
        self.nonces.insert(*sender, nonce + 1);
        Ok(())
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// INTENT ADMISSION PIPELINE
// ─────────────────────────────────────────────────────────────────────────────

#[derive(Debug, Clone)]
pub enum AdmissionResult {
    Accepted,
    Rejected(String),
}

/// Full intent admission pipeline: structural → signature → expiry → nonce → objects → constraints.
pub struct IntentAdmissionPipeline {
    pub nonce_tracker: NonceTracker,
    pub current_epoch: u64,
    pub verify_signatures: bool,
}

impl IntentAdmissionPipeline {
    pub fn new(current_epoch: u64) -> Self {
        IntentAdmissionPipeline {
            nonce_tracker: NonceTracker::new(),
            current_epoch,
            verify_signatures: true,
        }
    }

    pub fn admit(
        &mut self,
        tx: &IntentTransaction,
        store: &crate::state::store::ObjectStore,
    ) -> AdmissionResult {
        // 1. Structural
        if let Err(e) = tx.validate() {
            return AdmissionResult::Rejected(format!("structural: {}", e));
        }

        // 2. Signature
        if self.verify_signatures && !tx.verify_signature() {
            return AdmissionResult::Rejected("invalid Ed25519 signature".to_string());
        }

        // 3. Expiration
        if tx.deadline_epoch < self.current_epoch {
            return AdmissionResult::Rejected(format!(
                "expired: deadline {} < current {}", tx.deadline_epoch, self.current_epoch
            ));
        }

        // 4. Nonce
        if !self.nonce_tracker.is_valid(&tx.sender, tx.nonce) {
            let expected = self.nonce_tracker.expected_nonce(&tx.sender);
            return AdmissionResult::Rejected(format!(
                "nonce: expected {}, got {}", expected, tx.nonce
            ));
        }

        // 5. Object access
        for obj_id in &tx.target_objects {
            if store.get(obj_id).is_none() {
                return AdmissionResult::Rejected(format!("target object {} not found", obj_id));
            }
        }

        // 6. Constraints
        if let Some(max_slippage) = tx.constraints.max_slippage_bps {
            if max_slippage > 10000 {
                return AdmissionResult::Rejected("constraint: max_slippage_bps > 10000".to_string());
            }
        }
        for (obj_id, required_ver) in &tx.constraints.required_versions {
            match store.get(obj_id) {
                Some(obj) if obj.meta().version != *required_ver => {
                    return AdmissionResult::Rejected(format!(
                        "constraint: object {} version {} != required {}",
                        obj_id, obj.meta().version, required_ver
                    ));
                }
                None => {
                    return AdmissionResult::Rejected(format!(
                        "constraint: required_version object {} not found", obj_id
                    ));
                }
                _ => {}
            }
        }
        if !tx.constraints.allowed_objects.is_empty() {
            for obj_id in &tx.target_objects {
                if !tx.constraints.allowed_objects.contains(obj_id) {
                    return AdmissionResult::Rejected(format!(
                        "constraint: target {} not in allowed_objects", obj_id
                    ));
                }
            }
        }

        // All passed — advance nonce
        let _ = self.nonce_tracker.advance(&tx.sender, tx.nonce);
        AdmissionResult::Accepted
    }
}
