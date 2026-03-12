use std::collections::BTreeMap;
use crate::validation::validator::ValidatedSubmission;
use crate::config::policy::{OrderingPolicyConfig, TieBreakRule};
use crate::errors::OrderingError;

/// The key used for deterministic ordering.
/// Ordering is: (priority_weight DESC, sender_nonce if enforce, tie_break_secondary ASC, submission_id)
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct OrderingKey {
    /// Negated priority weight so higher priority sorts first in BTreeMap ascending.
    pub neg_priority: i64,
    /// Sender — used when enforce_sender_nonce_order is true to group same-sender submissions.
    pub sender: [u8; 32],
    /// Nonce — used as secondary key when enforce_sender_nonce_order is true.
    pub nonce: u64,
    /// Tie-break value (semantics depend on TieBreakRule).
    pub tie_break_bytes: [u8; 32],
    /// Final deterministic fallback: submission_id
    pub submission_id: [u8; 32],
}

impl PartialOrd for OrderingKey {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for OrderingKey {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        // 1. priority descending (higher priority → smaller neg_priority value)
        self.neg_priority.cmp(&other.neg_priority)
            // 2. same sender → nonce ascending (same-sender nonce ordering within priority group)
            .then_with(|| {
                if self.sender == other.sender {
                    self.nonce.cmp(&other.nonce)
                } else {
                    std::cmp::Ordering::Equal
                }
            })
            // 3. tie_break_bytes ascending
            .then(self.tie_break_bytes.cmp(&other.tie_break_bytes))
            // 4. nonce ascending (cross-sender fallback)
            .then(self.nonce.cmp(&other.nonce))
            // 5. submission_id ascending (final deterministic fallback)
            .then(self.submission_id.cmp(&other.submission_id))
    }
}

pub struct OrderingEngine {
    pub config: OrderingPolicyConfig,
}

impl OrderingEngine {
    pub fn new(config: OrderingPolicyConfig) -> Self {
        OrderingEngine { config }
    }

    /// Sort validated submissions into deterministic canonical order.
    /// Returns ordered Vec (highest priority first).
    pub fn order(&self, submissions: Vec<ValidatedSubmission>) -> Result<Vec<ValidatedSubmission>, OrderingError> {
        if submissions.is_empty() {
            return Err(OrderingError::EmptyInput);
        }

        // Build BTreeMap keyed by OrderingKey for automatic deterministic sort
        let mut map: BTreeMap<OrderingKey, ValidatedSubmission> = BTreeMap::new();

        for sub in submissions {
            let key = self.build_key(&sub);
            // On key collision (should not happen with submission_id in key), last wins
            map.insert(key, sub);
        }

        Ok(map.into_values().collect())
    }

    fn build_key(&self, sub: &ValidatedSubmission) -> OrderingKey {
        let priority = sub.envelope.submission.class.priority_weight() as i64;
        // Add metadata priority_hint (0-10000 bps scaled to 0-1000) as tiebreaker bonus
        let hint_bonus = (sub.envelope.submission.metadata.priority_hint.min(10000) as i64) / 10;
        let neg_priority = -(priority + hint_bonus);

        let tie_break_bytes = match self.config.tie_break {
            TieBreakRule::LexicographicAscending => sub.envelope.normalized_id,
            TieBreakRule::HigherNonce => {
                // Invert nonce bytes so higher nonce sorts first
                let n = u64::MAX.wrapping_sub(sub.envelope.submission.nonce);
                let mut b = [0u8; 32];
                b[..8].copy_from_slice(&n.to_be_bytes());
                b
            }
            TieBreakRule::LowerNonce => {
                let mut b = [0u8; 32];
                b[..8].copy_from_slice(&sub.envelope.submission.nonce.to_be_bytes());
                b
            }
            TieBreakRule::SenderAscending => {
                let mut b = [0u8; 32];
                b.copy_from_slice(&sub.envelope.submission.sender);
                b
            }
        };

        // When enforce_sender_nonce_order is false, use a zero sender so nonce grouping never fires
        let sender = if self.config.enforce_sender_nonce_order {
            sub.envelope.submission.sender
        } else {
            [0u8; 32]
        };

        OrderingKey {
            neg_priority,
            sender,
            nonce: sub.envelope.submission.nonce,
            tie_break_bytes,
            submission_id: sub.envelope.normalized_id,
        }
    }
}
