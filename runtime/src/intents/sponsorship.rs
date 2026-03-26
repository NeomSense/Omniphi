//! Sponsorship Validation Engine
//!
//! Validates sponsored intent transactions — where a third party (sponsor)
//! pays execution fees on behalf of the intent sender.

use crate::intents::base::{FeePolicy, IntentTransaction, IntentType};
use std::collections::BTreeSet;

/// Result of sponsorship validation.
#[derive(Debug, Clone)]
pub struct SponsorshipValidation {
    pub valid: bool,
    pub fee_payer: [u8; 32],
    pub sponsor_pays_amount: u64,
    pub sender_pays_amount: u64,
    pub reason: String,
}

/// Tracks sponsor nonces to prevent replay.
#[derive(Debug, Clone, Default)]
pub struct SponsorReplayTracker {
    /// Set of (sponsor, tx_id) pairs that have been seen.
    seen: BTreeSet<([u8; 32], [u8; 32])>,
}

impl SponsorReplayTracker {
    pub fn new() -> Self { SponsorReplayTracker::default() }

    /// Returns true if this (sponsor, tx_id) pair has NOT been seen before.
    /// Marks it as seen if new.
    pub fn check_and_mark(&mut self, sponsor: &[u8; 32], tx_id: &[u8; 32]) -> bool {
        self.seen.insert((*sponsor, *tx_id))
    }

    /// Check without marking (read-only).
    pub fn is_replay(&self, sponsor: &[u8; 32], tx_id: &[u8; 32]) -> bool {
        self.seen.contains(&(*sponsor, *tx_id))
    }

    /// Prune entries — in production, prune by epoch window.
    pub fn clear(&mut self) {
        self.seen.clear();
    }

    pub fn len(&self) -> usize { self.seen.len() }
    pub fn is_empty(&self) -> bool { self.seen.is_empty() }
}

/// Validates a sponsored intent transaction.
pub struct SponsorshipValidator;

impl SponsorshipValidator {
    /// Full sponsorship validation pipeline.
    ///
    /// Steps:
    /// 1. If no sponsor, validate as SenderPays (pass-through)
    /// 2. Verify sponsor address is valid (non-zero, different from sender)
    /// 3. Verify sponsor signature over sponsorship payload
    /// 4. Check replay protection
    /// 5. Check sponsorship limit: max_gas
    /// 6. Check sponsorship limit: max_fee_amount
    /// 7. Check sponsorship limit: allowed_intent_types
    /// 8. Check sponsorship limit: allowed_objects
    /// 9. Check sponsorship limit: expiration_epoch
    /// 10. Compute fee assignment (who pays what)
    pub fn validate(
        tx: &IntentTransaction,
        current_epoch: u64,
        replay_tracker: &mut SponsorReplayTracker,
    ) -> SponsorshipValidation {
        // Step 1: No sponsor → SenderPays
        if tx.fee_policy == FeePolicy::SenderPays || tx.sponsor.is_none() {
            return SponsorshipValidation {
                valid: true,
                fee_payer: tx.sender,
                sponsor_pays_amount: 0,
                sender_pays_amount: tx.max_fee,
                reason: String::new(),
            };
        }

        let sponsor = tx.sponsor.unwrap();

        // Step 2: Sponsor address checks
        if sponsor == [0u8; 32] {
            return Self::reject("sponsor address must be non-zero");
        }
        if sponsor == tx.sender {
            return Self::reject("sponsor and sender must be different addresses");
        }

        // Step 3: Verify sponsor signature
        if tx.sponsor_signature.is_none() {
            return Self::reject("sponsored intent requires sponsor_signature");
        }
        if !tx.verify_sponsor_signature() {
            return Self::reject("invalid sponsor signature");
        }

        // Step 4: Replay protection
        if !replay_tracker.check_and_mark(&sponsor, &tx.tx_id) {
            return Self::reject("sponsor replay: this (sponsor, tx_id) pair was already used");
        }

        // Step 5: Max gas limit
        if let Some(max_gas) = tx.sponsorship_limits.max_gas {
            if tx.max_fee > max_gas {
                return Self::reject(&format!(
                    "intent max_fee {} exceeds sponsor max_gas limit {}",
                    tx.max_fee, max_gas
                ));
            }
        }

        // Step 6: Max fee amount limit
        // For SponsorPays: enforce strictly (sponsor covers all, so must be within limit).
        // For SponsorThenSender: only enforce at fee-split time (sponsor pays up to cap,
        // sender pays remainder — rejection here would defeat the purpose).
        if let Some(max_fee) = tx.sponsorship_limits.max_fee_amount {
            if tx.fee_policy == FeePolicy::SponsorPays && tx.max_fee > max_fee {
                return Self::reject(&format!(
                    "intent max_fee {} exceeds sponsor max_fee_amount limit {}",
                    tx.max_fee, max_fee
                ));
            }
        }

        // Step 7: Allowed intent types
        if !tx.sponsorship_limits.allowed_intent_types.is_empty() {
            let intent_type_name = match &tx.intent {
                IntentType::Transfer(_) => "transfer",
                IntentType::Swap(_) => "swap",
                IntentType::YieldAllocate(_) => "yield_allocate",
                IntentType::TreasuryRebalance(_) => "treasury_rebalance",
                IntentType::RouteLiquidity(_) => "route_liquidity",
                IntentType::ContractCall(_) => "contract_call",
            };
            if !tx.sponsorship_limits.allowed_intent_types.iter().any(|t| t == intent_type_name) {
                return Self::reject(&format!(
                    "intent type '{}' not in sponsor's allowed types",
                    intent_type_name
                ));
            }
        }

        // Step 8: Allowed objects
        if !tx.sponsorship_limits.allowed_objects.is_empty() {
            for obj in &tx.target_objects {
                if !tx.sponsorship_limits.allowed_objects.contains(obj) {
                    return Self::reject(&format!(
                        "intent targets object {:?} not in sponsor's allowed objects",
                        obj
                    ));
                }
            }
        }

        // Step 9: Expiration
        if let Some(exp) = tx.sponsorship_limits.expiration_epoch {
            if current_epoch >= exp {
                return Self::reject(&format!(
                    "sponsorship expired at epoch {} (current: {})",
                    exp, current_epoch
                ));
            }
        }

        // Step 10: Fee assignment
        let (sponsor_pays, sender_pays) = match tx.fee_policy {
            FeePolicy::SponsorPays => (tx.max_fee, 0),
            FeePolicy::SponsorThenSender => {
                let sponsor_cap = tx.sponsorship_limits.max_fee_amount.unwrap_or(tx.max_fee);
                if tx.max_fee <= sponsor_cap {
                    (tx.max_fee, 0)
                } else {
                    (sponsor_cap, tx.max_fee - sponsor_cap)
                }
            }
            FeePolicy::SenderPays => (0, tx.max_fee), // shouldn't reach here
        };

        SponsorshipValidation {
            valid: true,
            fee_payer: sponsor,
            sponsor_pays_amount: sponsor_pays,
            sender_pays_amount: sender_pays,
            reason: String::new(),
        }
    }

    fn reject(reason: &str) -> SponsorshipValidation {
        SponsorshipValidation {
            valid: false,
            fee_payer: [0u8; 32],
            sponsor_pays_amount: 0,
            sender_pays_amount: 0,
            reason: reason.to_string(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::intents::base::*;
    use crate::objects::base::ObjectId;
    use ed25519_dalek::SigningKey;
    use std::collections::BTreeMap;

    fn make_keypair(seed: u8) -> (SigningKey, [u8; 32]) {
        let mut seed_bytes = [0u8; 32];
        seed_bytes[0] = seed;
        let sk = SigningKey::from_bytes(&seed_bytes);
        let pk: [u8; 32] = sk.verifying_key().to_bytes();
        (sk, pk)
    }

    fn make_basic_intent(sender_pk: [u8; 32]) -> IntentTransaction {
        IntentTransaction {
            tx_id: [0x42; 32],
            sender: sender_pk,
            intent: IntentType::Transfer(crate::intents::types::TransferIntent {
                asset_id: [0xAA; 32],
                recipient: [0xBB; 32],
                amount: 1000,
                memo: None,
            }),
            max_fee: 100,
            deadline_epoch: 999,
            nonce: 1,
            signature: [0u8; 64],
            metadata: BTreeMap::new(),
            target_objects: vec![],
            constraints: IntentConstraints::default(),
            execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
        }
    }

    fn sponsor_intent(
        tx: &mut IntentTransaction,
        sponsor_sk: &SigningKey,
        sponsor_pk: [u8; 32],
        policy: FeePolicy,
        limits: SponsorshipLimits,
    ) {
        tx.sponsor = Some(sponsor_pk);
        tx.sponsorship_limits = limits;
        tx.fee_policy = policy;
        let sig = tx.sign_sponsorship(&sponsor_sk.to_bytes());
        tx.sponsor_signature = Some(sig);
    }

    // ── Test 1: valid sponsored intent ───────────────────────

    #[test]
    fn test_valid_sponsored_intent() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, SponsorshipLimits::default());

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(result.valid, "Expected valid, got: {}", result.reason);
        assert_eq!(result.fee_payer, sponsor_pk);
        assert_eq!(result.sponsor_pays_amount, 100);
        assert_eq!(result.sender_pays_amount, 0);
    }

    // ── Test 2: invalid sponsor signature ────────────────────

    #[test]
    fn test_invalid_sponsor_signature() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (_sponsor_sk, sponsor_pk) = make_keypair(2);
        let (wrong_sk, _) = make_keypair(3);

        let mut tx = make_basic_intent(sender_pk);
        // Sign with wrong key
        sponsor_intent(&mut tx, &wrong_sk, sponsor_pk, FeePolicy::SponsorPays, SponsorshipLimits::default());

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result.valid);
        assert!(result.reason.contains("invalid sponsor signature"));
    }

    // ── Test 3: sponsor limit exceeded (max_fee_amount) ──────

    #[test]
    fn test_sponsor_limit_exceeded() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        tx.max_fee = 500; // Intent wants 500
        let limits = SponsorshipLimits {
            max_fee_amount: Some(100), // Sponsor only covers 100
            ..Default::default()
        };
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, limits);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result.valid);
        assert!(result.reason.contains("exceeds sponsor max_fee_amount"));
    }

    // ── Test 4: expired sponsorship ──────────────────────────

    #[test]
    fn test_expired_sponsorship() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        let limits = SponsorshipLimits {
            expiration_epoch: Some(50),
            ..Default::default()
        };
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, limits);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 60, &mut tracker); // epoch 60 > 50
        assert!(!result.valid);
        assert!(result.reason.contains("expired"));
    }

    // ── Test 5: user valid but sponsor invalid ───────────────

    #[test]
    fn test_user_valid_sponsor_invalid() {
        let (sender_sk, sender_pk) = make_keypair(1);
        let (_sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        tx.signature = tx.sign(&sender_sk.to_bytes()); // user signs correctly

        // Sponsor signature is missing
        tx.sponsor = Some(sponsor_pk);
        tx.fee_policy = FeePolicy::SponsorPays;
        // No sponsor_signature set

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result.valid);
        assert!(result.reason.contains("requires sponsor_signature"));
    }

    // ── Test 6: deterministic fee charging (SponsorThenSender) ─

    #[test]
    fn test_deterministic_fee_split() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        tx.max_fee = 300;
        let limits = SponsorshipLimits {
            max_fee_amount: Some(200), // Sponsor covers up to 200
            ..Default::default()
        };
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorThenSender, limits);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(result.valid, "Expected valid, got: {}", result.reason);
        assert_eq!(result.sponsor_pays_amount, 200); // sponsor pays up to cap
        assert_eq!(result.sender_pays_amount, 100);  // sender pays remainder
    }

    // ── Test 7: replay attempt rejection ─────────────────────

    #[test]
    fn test_replay_rejection() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, SponsorshipLimits::default());

        let mut tracker = SponsorReplayTracker::new();

        let result1 = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(result1.valid);

        // Same tx submitted again
        let result2 = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result2.valid);
        assert!(result2.reason.contains("replay"));
    }

    // ── Test 8: non-sponsored intent passes through ──────────

    #[test]
    fn test_non_sponsored_passes_through() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let tx = make_basic_intent(sender_pk);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(result.valid);
        assert_eq!(result.fee_payer, sender_pk);
        assert_eq!(result.sponsor_pays_amount, 0);
        assert_eq!(result.sender_pays_amount, 100);
    }

    // ── Test 9: sponsor = sender rejected ────────────────────

    #[test]
    fn test_sponsor_equals_sender_rejected() {
        let (sender_sk, sender_pk) = make_keypair(1);
        let mut tx = make_basic_intent(sender_pk);

        // Sponsor is same as sender
        tx.sponsor = Some(sender_pk);
        tx.fee_policy = FeePolicy::SponsorPays;
        let sig = tx.sign_sponsorship(&sender_sk.to_bytes());
        tx.sponsor_signature = Some(sig);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result.valid);
        assert!(result.reason.contains("different addresses"));
    }

    // ── Test 10: intent type restriction ─────────────────────

    #[test]
    fn test_intent_type_restriction() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk); // This is a Transfer
        let limits = SponsorshipLimits {
            allowed_intent_types: vec!["swap".to_string()], // Only swaps allowed
            ..Default::default()
        };
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, limits);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result.valid);
        assert!(result.reason.contains("not in sponsor's allowed types"));
    }

    // ── Test 11: max_gas restriction ─────────────────────────

    #[test]
    fn test_max_gas_restriction() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        tx.max_fee = 1000;
        let limits = SponsorshipLimits {
            max_gas: Some(500),
            ..Default::default()
        };
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, limits);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result.valid);
        assert!(result.reason.contains("exceeds sponsor max_gas"));
    }

    // ── Test 12: object restriction ──────────────────────────

    #[test]
    fn test_object_restriction() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        tx.target_objects = vec![ObjectId([0x11; 32])]; // Intent targets this object
        let limits = SponsorshipLimits {
            allowed_objects: vec![ObjectId([0x22; 32])], // Sponsor only allows this one
            ..Default::default()
        };
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, limits);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(!result.valid);
        assert!(result.reason.contains("not in sponsor's allowed objects"));
    }

    // ── Test 13: SponsorPays with fee fully covered ──────────

    #[test]
    fn test_sponsor_pays_full() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        tx.max_fee = 500;
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorPays, SponsorshipLimits::default());

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(result.valid);
        assert_eq!(result.sponsor_pays_amount, 500);
        assert_eq!(result.sender_pays_amount, 0);
    }

    // ── Test 14: SponsorThenSender with fee under cap ────────

    #[test]
    fn test_sponsor_then_sender_under_cap() {
        let (_sender_sk, sender_pk) = make_keypair(1);
        let (sponsor_sk, sponsor_pk) = make_keypair(2);

        let mut tx = make_basic_intent(sender_pk);
        tx.max_fee = 50;
        let limits = SponsorshipLimits {
            max_fee_amount: Some(200),
            ..Default::default()
        };
        sponsor_intent(&mut tx, &sponsor_sk, sponsor_pk, FeePolicy::SponsorThenSender, limits);

        let mut tracker = SponsorReplayTracker::new();
        let result = SponsorshipValidator::validate(&tx, 10, &mut tracker);
        assert!(result.valid);
        assert_eq!(result.sponsor_pays_amount, 50);  // Sponsor covers all (under cap)
        assert_eq!(result.sender_pays_amount, 0);
    }
}
