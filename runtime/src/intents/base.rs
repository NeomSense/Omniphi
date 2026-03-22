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

/// A fully described, signed intent transaction.
#[derive(Debug, Clone)]
pub struct IntentTransaction {
    pub tx_id: [u8; 32],
    pub sender: [u8; 32],
    pub intent: IntentType,
    pub max_fee: u64,
    pub deadline_epoch: u64,
    pub nonce: u64,
    /// Placeholder — signature verification is out of scope for the runtime engine.
    pub signature: [u8; 64],
    pub metadata: BTreeMap<String, String>,
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
