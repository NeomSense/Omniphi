use crate::errors::RuntimeError;
use crate::safety::actions::SafetyAction;
use crate::safety::incidents::{IncidentSeverity, SafetyIncident};

#[derive(Debug, Clone)]
pub struct ContainmentActionRecord {
    pub action: SafetyAction,
    pub applied_at_epoch: u64,
    pub applied_to: String, // human-readable scope description
    pub success: bool,
    pub error: Option<String>,
}

#[derive(Debug, Clone)]
pub struct SafetyRecoveryPlaceholder {
    pub incident_id: [u8; 32],
    pub recovery_status: RecoveryStatus,
    pub governance_proposal_id: Option<u64>,
    pub recovery_epoch: Option<u64>,
    pub notes: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RecoveryStatus {
    Pending,
    InGovernanceReview,
    Resolved,
    Unresolved,
}

#[derive(Debug, Clone)]
pub struct SafetyReceipt {
    pub receipt_id: [u8; 32],
    pub incident: SafetyIncident,
    pub containment_actions: Vec<ContainmentActionRecord>,
    pub recovery: SafetyRecoveryPlaceholder,
    pub epoch: u64,
    pub receipt_hash: [u8; 32],
}

impl SafetyReceipt {
    pub fn compute_hash(&self) -> [u8; 32] {
        use sha2::{Digest, Sha256};
        let input = format!("{:?}:{}", self.incident.incident_id, self.epoch);
        let hash = Sha256::digest(input.as_bytes());
        let mut h = [0u8; 32];
        h.copy_from_slice(&hash);
        h
    }
}

#[derive(Debug, Clone)]
pub struct IncidentLedgerEntry {
    pub sequence: u64,
    pub receipt: SafetyReceipt,
    pub batch_id: Option<[u8; 32]>,
}

pub struct IncidentLedger {
    pub entries: Vec<IncidentLedgerEntry>,
    pub max_entries: usize, // 0 = unlimited
    pub(crate) next_sequence: u64,
}

impl IncidentLedger {
    pub fn new(max_entries: usize) -> Self {
        IncidentLedger {
            entries: Vec::new(),
            max_entries,
            next_sequence: 0,
        }
    }

    /// Returns sequence number; Err(IncidentLedgerFull) if at capacity and max > 0
    pub fn append(
        &mut self,
        mut receipt: SafetyReceipt,
        batch_id: Option<[u8; 32]>,
    ) -> Result<u64, RuntimeError> {
        if self.max_entries > 0 && self.entries.len() >= self.max_entries {
            return Err(RuntimeError::IncidentLedgerFull);
        }
        // Compute receipt hash before storing
        let hash = receipt.compute_hash();
        receipt.receipt_hash = hash;

        let seq = self.next_sequence;
        self.next_sequence += 1;

        self.entries.push(IncidentLedgerEntry {
            sequence: seq,
            receipt,
            batch_id,
        });

        Ok(seq)
    }

    pub fn get_by_severity(&self, min_severity: &IncidentSeverity) -> Vec<&IncidentLedgerEntry> {
        self.entries
            .iter()
            .filter(|e| e.receipt.incident.severity >= *min_severity)
            .collect()
    }

    pub fn get_by_solver(&self, solver_id: &[u8; 32]) -> Vec<&IncidentLedgerEntry> {
        self.entries
            .iter()
            .filter(|e| {
                e.receipt
                    .incident
                    .solver_id
                    .map(|s| s == *solver_id)
                    .unwrap_or(false)
            })
            .collect()
    }

    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn latest(&self) -> Option<&IncidentLedgerEntry> {
        self.entries.last()
    }
}
