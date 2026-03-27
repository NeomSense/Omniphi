//! Encrypted Intents — Protocol-Level Anti-Front-Running
//!
//! Users submit encrypted intents that are ordered by PoSeq BEFORE the content
//! is revealed. This prevents front-running, sandwich attacks, and information
//! leakage because sequencers and validators cannot see the intent details
//! during the ordering phase.
//!
//! ## Flow
//!
//! 1. User computes `commitment = SHA256("OMNIPHI_COMMIT_V1" || encrypted_payload || submitter || nonce)`
//! 2. User submits `EncryptedIntent { commitment, encrypted_payload, reveal_deadline, ... }`
//! 3. PoSeq orders the encrypted intent (sees only commitment + ciphertext blob)
//! 4. After ordering, the reveal phase begins
//! 5. User submits `IntentReveal { commitment, plaintext_intent, reveal_nonce }`
//! 6. Runtime verifies `SHA256(plaintext || reveal_nonce) == commitment`
//! 7. If valid, the plaintext intent enters the normal execution pipeline
//! 8. If reveal deadline passes without valid reveal, the encrypted intent expires
//!
//! ## Anti-MEV Benefits
//!
//! - Sequencers cannot front-run because they don't know the intent content during ordering
//! - Validators cannot sandwich because the execution details are hidden until after ordering
//! - Even if the encrypted payload is cracked, the commitment scheme ensures the revealed
//!   intent matches what was committed (no substitution attacks)
//!
//! ## Assumptions
//!
//! - The encryption is application-level (user encrypts with their own key or a threshold key)
//! - The runtime does NOT perform decryption — it only validates commitment ↔ reveal matching
//! - The reveal_nonce prevents commitment grinding (attacker cannot guess the plaintext)
//! - Determinism: all state transitions happen only after reveal, not during encrypted phase

use crate::intents::base::IntentTransaction;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

/// Status of an encrypted intent in the lifecycle.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EncryptedIntentStatus {
    /// Submitted and ordered, waiting for reveal.
    Pending,
    /// Successfully revealed and forwarded to execution pipeline.
    Revealed,
    /// Reveal deadline passed without valid reveal.
    Expired,
    /// Reveal was invalid (commitment mismatch, etc.).
    Rejected,
}

/// An encrypted intent submitted for ordering before reveal.
#[derive(Debug, Clone)]
pub struct EncryptedIntent {
    /// Commitment: SHA256 of the plaintext intent + reveal_nonce.
    /// This is the binding identity of the encrypted intent.
    pub commitment: [u8; 32],
    /// The encrypted payload (opaque to the runtime).
    /// The runtime does not decrypt this — it's carried for the user/relayer.
    pub encrypted_payload: Vec<u8>,
    /// Epoch after which the reveal window closes.
    /// If no valid reveal arrives by this epoch, the intent expires.
    pub reveal_deadline: u64,
    /// The submitter's Ed25519 public key (must match the revealed intent's sender).
    pub submitter: [u8; 32],
    /// Nonce for replay protection (unique per submitter).
    pub nonce: u64,
    /// Epoch when this encrypted intent was submitted.
    pub submitted_at_epoch: u64,
    /// Current lifecycle status.
    pub status: EncryptedIntentStatus,
}

/// A reveal submission that opens an encrypted intent.
#[derive(Debug, Clone)]
pub struct IntentReveal {
    /// The commitment this reveal corresponds to.
    pub commitment: [u8; 32],
    /// The plaintext intent transaction (fully formed, signed).
    pub plaintext_intent: IntentTransaction,
    /// Random nonce used in the commitment computation.
    /// Prevents commitment grinding attacks.
    pub reveal_nonce: [u8; 32],
}

impl EncryptedIntent {
    /// Compute the commitment for a given plaintext intent and reveal nonce.
    ///
    /// commitment = SHA256("OMNIPHI_COMMIT_V1" || serialized_intent || submitter || nonce || reveal_nonce)
    pub fn compute_commitment(
        intent: &IntentTransaction,
        reveal_nonce: &[u8; 32],
    ) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"OMNIPHI_COMMIT_V1");
        // Use the intent's signing payload as canonical representation
        h.update(&intent.signing_payload());
        h.update(&intent.sender);
        h.update(&intent.nonce.to_be_bytes());
        h.update(reveal_nonce);
        let r = h.finalize();
        let mut out = [0u8; 32];
        out.copy_from_slice(&r);
        out
    }

    /// Create a new encrypted intent from a plaintext intent.
    /// The caller provides the encrypted payload (runtime doesn't encrypt).
    pub fn create(
        intent: &IntentTransaction,
        reveal_nonce: &[u8; 32],
        encrypted_payload: Vec<u8>,
        reveal_deadline: u64,
        current_epoch: u64,
    ) -> Result<Self, String> {
        if encrypted_payload.is_empty() {
            return Err("encrypted_payload must not be empty".to_string());
        }
        if reveal_deadline <= current_epoch {
            return Err(format!(
                "reveal_deadline {} must be > current epoch {}",
                reveal_deadline, current_epoch
            ));
        }
        if intent.sender == [0u8; 32] {
            return Err("submitter must be non-zero".to_string());
        }

        let commitment = Self::compute_commitment(intent, reveal_nonce);

        Ok(EncryptedIntent {
            commitment,
            encrypted_payload,
            reveal_deadline,
            submitter: intent.sender,
            nonce: intent.nonce,
            submitted_at_epoch: current_epoch,
            status: EncryptedIntentStatus::Pending,
        })
    }

    /// Check if this encrypted intent can still be revealed.
    pub fn is_revealable(&self, current_epoch: u64) -> bool {
        self.status == EncryptedIntentStatus::Pending && current_epoch <= self.reveal_deadline
    }
}

/// Registry of encrypted intents, tracking lifecycle and enforcing rules.
#[derive(Debug, Clone, Default)]
pub struct EncryptedIntentRegistry {
    /// commitment → EncryptedIntent
    intents: BTreeMap<[u8; 32], EncryptedIntent>,
    /// (submitter, nonce) → commitment (for replay detection)
    by_submitter_nonce: BTreeMap<([u8; 32], u64), [u8; 32]>,
    /// O(1) counter of pending intents (maintained on every state transition).
    pending: usize,
}

impl EncryptedIntentRegistry {
    pub fn new() -> Self { EncryptedIntentRegistry::default() }

    /// Submit a new encrypted intent.
    pub fn submit(&mut self, ei: EncryptedIntent) -> Result<[u8; 32], String> {
        let commitment = ei.commitment;

        // Check for duplicate commitment
        if self.intents.contains_key(&commitment) {
            return Err("duplicate commitment".to_string());
        }

        // Check for submitter+nonce replay
        let key = (ei.submitter, ei.nonce);
        if self.by_submitter_nonce.contains_key(&key) {
            return Err("submitter nonce already used".to_string());
        }

        self.by_submitter_nonce.insert(key, commitment);
        self.intents.insert(commitment, ei);
        self.pending += 1;
        Ok(commitment)
    }

    /// Attempt to reveal an encrypted intent.
    ///
    /// Validates:
    /// 1. Commitment exists and is Pending
    /// 2. Reveal deadline not passed
    /// 3. Revealed intent's sender matches submitter
    /// 4. Commitment matches SHA256(plaintext || reveal_nonce)
    ///
    /// On success, transitions status to Revealed and returns the plaintext intent.
    pub fn reveal(
        &mut self,
        reveal: &IntentReveal,
        current_epoch: u64,
    ) -> Result<IntentTransaction, String> {
        // Step 1: Look up the encrypted intent
        let ei = self.intents.get_mut(&reveal.commitment)
            .ok_or_else(|| "commitment not found".to_string())?;

        // Step 2: Check status
        match ei.status {
            EncryptedIntentStatus::Pending => {}
            EncryptedIntentStatus::Revealed => {
                return Err("already revealed (duplicate reveal)".to_string());
            }
            EncryptedIntentStatus::Expired => {
                return Err("encrypted intent has expired".to_string());
            }
            EncryptedIntentStatus::Rejected => {
                return Err("encrypted intent was rejected".to_string());
            }
        }

        // Step 3: Check reveal deadline
        if current_epoch > ei.reveal_deadline {
            ei.status = EncryptedIntentStatus::Expired;
            self.pending = self.pending.saturating_sub(1);
            return Err(format!(
                "reveal deadline {} passed (current epoch: {})",
                ei.reveal_deadline, current_epoch
            ));
        }

        // Step 4: Submitter must match revealed sender
        if reveal.plaintext_intent.sender != ei.submitter {
            ei.status = EncryptedIntentStatus::Rejected;
            self.pending = self.pending.saturating_sub(1);
            return Err("revealed intent sender does not match submitter".to_string());
        }

        // Step 5: Verify commitment
        let expected_commitment = EncryptedIntent::compute_commitment(
            &reveal.plaintext_intent,
            &reveal.reveal_nonce,
        );
        if expected_commitment != reveal.commitment {
            ei.status = EncryptedIntentStatus::Rejected;
            self.pending = self.pending.saturating_sub(1);
            return Err("commitment mismatch: revealed intent does not match original commitment".to_string());
        }

        // Success: transition to Revealed
        ei.status = EncryptedIntentStatus::Revealed;
        self.pending = self.pending.saturating_sub(1);
        Ok(reveal.plaintext_intent.clone())
    }

    /// Expire all encrypted intents past their reveal deadline.
    /// Returns the number of intents expired.
    pub fn expire_stale(&mut self, current_epoch: u64) -> usize {
        let mut count = 0;
        for ei in self.intents.values_mut() {
            if ei.status == EncryptedIntentStatus::Pending && current_epoch > ei.reveal_deadline {
                ei.status = EncryptedIntentStatus::Expired;
                count += 1;
            }
        }
        self.pending = self.pending.saturating_sub(count);
        count
    }

    /// Prune non-pending intents older than `before_epoch`.
    pub fn prune(&mut self, before_epoch: u64) -> usize {
        let before = self.intents.len();
        self.intents.retain(|_, ei| {
            ei.status == EncryptedIntentStatus::Pending || ei.submitted_at_epoch >= before_epoch
        });
        // Recount pending after prune (pending entries may have been removed too)
        self.pending = self.intents.values()
            .filter(|ei| ei.status == EncryptedIntentStatus::Pending).count();
        before - self.intents.len()
    }

    /// Get an encrypted intent by commitment.
    pub fn get(&self, commitment: &[u8; 32]) -> Option<&EncryptedIntent> {
        self.intents.get(commitment)
    }

    /// Total number of encrypted intents (all statuses).
    pub fn count(&self) -> usize { self.intents.len() }

    /// Number of pending (unrevealed) intents. O(1).
    pub fn pending_count(&self) -> usize { self.pending }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::intents::base::*;
    use crate::intents::types::TransferIntent;
    use crate::objects::base::ObjectId;
    use std::collections::BTreeMap as StdBTreeMap;

    fn make_intent(sender: [u8; 32], nonce: u64) -> IntentTransaction {
        IntentTransaction {
            tx_id: {
                let mut id = [0u8; 32];
                id[0] = sender[0];
                id[1..9].copy_from_slice(&nonce.to_be_bytes());
                id
            },
            sender,
            intent: IntentType::Transfer(TransferIntent {
                asset_id: [0xAA; 32],
                recipient: [0xBB; 32],
                amount: 1000,
                memo: None,
            }),
            max_fee: 100,
            deadline_epoch: 999,
            nonce,
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

    fn sender() -> [u8; 32] { let mut b = [0u8; 32]; b[0] = 1; b }
    fn reveal_nonce() -> [u8; 32] { [0xCC; 32] }

    // ── Test 1: valid encrypted submission and reveal ────────

    #[test]
    fn test_valid_submit_and_reveal() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 100, 10).unwrap();
        let commitment = reg.submit(ei).unwrap();

        let reveal = IntentReveal {
            commitment,
            plaintext_intent: intent.clone(),
            reveal_nonce: nonce,
        };

        let result = reg.reveal(&reveal, 50);
        assert!(result.is_ok(), "Expected valid reveal, got: {:?}", result.err());

        let revealed = result.unwrap();
        assert_eq!(revealed.sender, sender());
        assert_eq!(revealed.nonce, 1);

        // Status should be Revealed
        let ei = reg.get(&commitment).unwrap();
        assert_eq!(ei.status, EncryptedIntentStatus::Revealed);
    }

    // ── Test 2: invalid reveal preimage ──────────────────────

    #[test]
    fn test_invalid_reveal_preimage() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 100, 10).unwrap();
        let commitment = reg.submit(ei).unwrap();

        // Use wrong reveal_nonce
        let wrong_nonce = [0xDD; 32];
        let reveal = IntentReveal {
            commitment,
            plaintext_intent: intent.clone(),
            reveal_nonce: wrong_nonce,
        };

        let result = reg.reveal(&reveal, 50);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("commitment mismatch"));

        // Status should be Rejected
        let ei = reg.get(&commitment).unwrap();
        assert_eq!(ei.status, EncryptedIntentStatus::Rejected);
    }

    // ── Test 3: reveal after deadline ────────────────────────

    #[test]
    fn test_reveal_after_deadline() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 50, 10).unwrap();
        let commitment = reg.submit(ei).unwrap();

        let reveal = IntentReveal {
            commitment,
            plaintext_intent: intent.clone(),
            reveal_nonce: nonce,
        };

        // Reveal at epoch 60, deadline was 50
        let result = reg.reveal(&reveal, 60);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("deadline"));

        let ei = reg.get(&commitment).unwrap();
        assert_eq!(ei.status, EncryptedIntentStatus::Expired);
    }

    // ── Test 4: commitment mismatch (different intent) ───────

    #[test]
    fn test_commitment_mismatch_different_intent() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 100, 10).unwrap();
        let commitment = reg.submit(ei).unwrap();

        // Create a DIFFERENT intent for the reveal
        let mut different_intent = make_intent(sender(), 1);
        different_intent.max_fee = 999; // changed

        let reveal = IntentReveal {
            commitment,
            plaintext_intent: different_intent,
            reveal_nonce: nonce,
        };

        let result = reg.reveal(&reveal, 50);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("commitment mismatch"));
    }

    // ── Test 5: duplicate reveal ─────────────────────────────

    #[test]
    fn test_duplicate_reveal() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 100, 10).unwrap();
        let commitment = reg.submit(ei).unwrap();

        let reveal = IntentReveal {
            commitment,
            plaintext_intent: intent.clone(),
            reveal_nonce: nonce,
        };

        // First reveal succeeds
        assert!(reg.reveal(&reveal, 50).is_ok());

        // Second reveal fails
        let result = reg.reveal(&reveal, 51);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("already revealed"));
    }

    // ── Test 6: expired encrypted intent cleanup ─────────────

    #[test]
    fn test_expire_stale_cleanup() {
        let mut reg = EncryptedIntentRegistry::new();

        let intent1 = make_intent(sender(), 1);
        let intent2 = make_intent(sender(), 2);
        let nonce = reveal_nonce();
        let nonce2 = [0xDD; 32];

        let ei1 = EncryptedIntent::create(&intent1, &nonce, vec![0x01], 30, 10).unwrap();
        let ei2 = EncryptedIntent::create(&intent2, &nonce2, vec![0x02], 100, 10).unwrap();

        reg.submit(ei1).unwrap();
        reg.submit(ei2).unwrap();

        assert_eq!(reg.pending_count(), 2);

        // Expire at epoch 50 — only intent1 (deadline 30) should expire
        let expired = reg.expire_stale(50);
        assert_eq!(expired, 1);
        assert_eq!(reg.pending_count(), 1);
    }

    // ── Test 7: duplicate commitment rejected ────────────────

    #[test]
    fn test_duplicate_commitment_rejected() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 100, 10).unwrap();
        reg.submit(ei.clone()).unwrap();

        let result = reg.submit(ei);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("duplicate commitment"));
    }

    // ── Test 8: submitter nonce replay rejected ──────────────

    #[test]
    fn test_submitter_nonce_replay() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent1 = make_intent(sender(), 1);
        let nonce1 = reveal_nonce();

        let ei1 = EncryptedIntent::create(&intent1, &nonce1, vec![0x01], 100, 10).unwrap();
        reg.submit(ei1).unwrap();

        // Same submitter + same nonce, different reveal_nonce (different commitment)
        let nonce2 = [0xEE; 32];
        let ei2 = EncryptedIntent::create(&intent1, &nonce2, vec![0x02], 100, 10).unwrap();
        let result = reg.submit(ei2);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("nonce already used"));
    }

    // ── Test 9: sender mismatch in reveal ────────────────────

    #[test]
    fn test_sender_mismatch_in_reveal() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 100, 10).unwrap();
        let commitment = reg.submit(ei).unwrap();

        // Reveal with a different sender
        let mut wrong_sender_intent = intent.clone();
        wrong_sender_intent.sender = { let mut b = [0u8; 32]; b[0] = 99; b };

        let reveal = IntentReveal {
            commitment,
            plaintext_intent: wrong_sender_intent,
            reveal_nonce: nonce,
        };

        let result = reg.reveal(&reveal, 50);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("sender does not match"));
    }

    // ── Test 10: commitment is deterministic ─────────────────

    #[test]
    fn test_commitment_deterministic() {
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let c1 = EncryptedIntent::compute_commitment(&intent, &nonce);
        let c2 = EncryptedIntent::compute_commitment(&intent, &nonce);
        assert_eq!(c1, c2);

        // Different nonce → different commitment
        let c3 = EncryptedIntent::compute_commitment(&intent, &[0xDD; 32]);
        assert_ne!(c1, c3);
    }

    // ── Test 11: empty encrypted payload rejected ────────────

    #[test]
    fn test_empty_payload_rejected() {
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let result = EncryptedIntent::create(&intent, &nonce, vec![], 100, 10);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("must not be empty"));
    }

    // ── Test 12: expired at creation rejected ────────────────

    #[test]
    fn test_expired_at_creation_rejected() {
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let result = EncryptedIntent::create(&intent, &nonce, vec![0x01], 5, 10);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("must be >"));
    }

    // ── Test 13: prune old entries ───────────────────────────

    #[test]
    fn test_prune_old_entries() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 100, 5).unwrap();
        let commitment = reg.submit(ei).unwrap();

        // Reveal it so it's no longer Pending
        let reveal = IntentReveal {
            commitment,
            plaintext_intent: intent.clone(),
            reveal_nonce: nonce,
        };
        reg.reveal(&reveal, 50).unwrap();

        // Prune entries submitted before epoch 10
        let pruned = reg.prune(10);
        assert_eq!(pruned, 1); // submitted at epoch 5, status Revealed → pruned
    }

    // ── Test 14: reveal at exact deadline epoch (boundary) ───

    #[test]
    fn test_reveal_at_exact_deadline() {
        let mut reg = EncryptedIntentRegistry::new();
        let intent = make_intent(sender(), 1);
        let nonce = reveal_nonce();

        let ei = EncryptedIntent::create(&intent, &nonce, vec![0xDE, 0xAD], 50, 10).unwrap();
        let commitment = reg.submit(ei).unwrap();

        let reveal = IntentReveal {
            commitment,
            plaintext_intent: intent.clone(),
            reveal_nonce: nonce,
        };

        // Reveal at exactly epoch 50 (deadline) — should succeed (<=)
        let result = reg.reveal(&reveal, 50);
        assert!(result.is_ok(), "Reveal at exact deadline should succeed");
    }
}
