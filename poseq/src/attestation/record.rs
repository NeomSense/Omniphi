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

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SequencerIdentity {
    pub sequencer_id: [u8; 32],
    pub display_name: String,
    pub public_key: [u8; 32],
}

/// Placeholder for future validator/multi-sig sequencer attestation.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SequencingProofPlaceholder {
    pub batch_id: [u8; 32],
    pub proof_type: String,     // "placeholder" for now
    pub proof_data: Vec<u8>,    // empty for now
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct AttestationRecord {
    pub batch_id: [u8; 32],
    pub sequencer: Option<SequencerIdentity>,
    pub proof: SequencingProofPlaceholder,
    pub finalized: bool,
}

/// Batch-level attestation attached to every OrderedBatch.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BatchAttestation {
    pub batch_id: [u8; 32],
    #[serde(with = "serde_array64")]
    pub sequencer_signature: [u8; 64],  // all-zero placeholder
    pub sequencer_id: [u8; 32],         // all-zero placeholder
    pub proof_placeholder: SequencingProofPlaceholder,
    pub is_finalized: bool,
}

impl BatchAttestation {
    pub fn placeholder(batch_id: [u8; 32]) -> Self {
        BatchAttestation {
            batch_id,
            sequencer_signature: [0u8; 64],
            sequencer_id: [0u8; 32],
            proof_placeholder: SequencingProofPlaceholder {
                batch_id,
                proof_type: "placeholder_v1".into(),
                proof_data: vec![],
            },
            is_finalized: false,
        }
    }
}
