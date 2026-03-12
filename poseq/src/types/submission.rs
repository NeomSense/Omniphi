use std::collections::BTreeMap;
use crate::config::policy::SubmissionClass;

/// Serde helper for [u8; 64] (serde doesn't implement Serialize/Deserialize for arrays > 32).
mod serde_array64 {
    use serde::{Deserialize, Deserializer, Serialize, Serializer};

    pub fn serialize<S: Serializer>(arr: &[u8; 64], s: S) -> Result<S::Ok, S::Error> {
        arr.as_ref().serialize(s)
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<[u8; 64], D::Error> {
        let v: Vec<u8> = Vec::deserialize(d)?;
        if v.len() != 64 {
            return Err(serde::de::Error::custom("expected 64 bytes"));
        }
        let mut arr = [0u8; 64];
        arr.copy_from_slice(&v);
        Ok(arr)
    }
}

/// The raw payload kind tag on a submission.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum SubmissionKind {
    IntentTransaction,
    GoalPacket,
    CandidatePlan,
    AgentSubmission,
    RawBytes,
}

impl SubmissionKind {
    pub fn to_class(&self) -> SubmissionClass {
        match self {
            SubmissionKind::IntentTransaction => SubmissionClass::Transfer, // caller should override
            SubmissionKind::GoalPacket        => SubmissionClass::GoalPacket,
            SubmissionKind::CandidatePlan     => SubmissionClass::Swap,
            SubmissionKind::AgentSubmission   => SubmissionClass::AgentSubmission,
            SubmissionKind::RawBytes          => SubmissionClass::Other("raw".into()),
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SubmissionMetadata {
    pub sequence_hint: u64,     // sender's locally-incremented nonce/sequence
    pub priority_hint: u32,     // 0-10000 bps, soft suggestion to sequencer
    pub solver_id: Option<[u8; 32]>,
    pub domain_tag: Option<String>,
    pub extra: BTreeMap<String, String>,
}

/// A raw submission from a client. May be invalid.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SequencingSubmission {
    pub submission_id: [u8; 32],
    pub sender: [u8; 32],
    pub kind: SubmissionKind,
    pub class: SubmissionClass,
    pub payload_hash: [u8; 32],    // SHA256 of payload_body
    pub payload_body: Vec<u8>,     // raw serialized payload
    pub nonce: u64,
    pub max_fee: u64,
    pub deadline_epoch: u64,
    pub metadata: SubmissionMetadata,
    #[serde(with = "serde_array64")]
    pub signature: [u8; 64],       // placeholder auth
}

impl SequencingSubmission {
    /// Compute canonical submission_id = SHA256(bincode of key fields, excluding signature).
    pub fn compute_id(&self) -> [u8; 32] {
        use sha2::{Sha256, Digest};
        // hash: sender || kind_tag || payload_hash || nonce || max_fee || deadline_epoch
        let mut hasher = Sha256::new();
        hasher.update(&self.sender);
        let kind_bytes = bincode::serialize(&self.kind).unwrap_or_default();
        hasher.update(&kind_bytes);
        hasher.update(&self.payload_hash);
        hasher.update(&self.nonce.to_be_bytes());
        hasher.update(&self.max_fee.to_be_bytes());
        hasher.update(&self.deadline_epoch.to_be_bytes());
        let hash = hasher.finalize();
        let mut id = [0u8; 32];
        id.copy_from_slice(&hash);
        id
    }

    pub fn validate_payload_hash(&self) -> bool {
        use sha2::{Sha256, Digest};
        let hash = Sha256::digest(&self.payload_body);
        let mut h = [0u8; 32];
        h.copy_from_slice(&hash);
        h == self.payload_hash
    }
}

/// An envelope wrapping a submission with receiver-assigned intake metadata.
#[derive(Debug, Clone)]
pub struct SubmissionEnvelope {
    pub submission: SequencingSubmission,
    pub received_at_sequence: u64,  // monotonic intake counter
    pub normalized_id: [u8; 32],    // = submission.compute_id()
}
