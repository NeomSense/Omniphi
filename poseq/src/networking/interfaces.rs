use crate::proposals::batch::ProposedBatch;
use crate::finalization::engine::FinalizedBatch;
use crate::attestations::collector::BatchAttestationVote;
use crate::errors::NetworkError;

/// Trait for broadcasting a new batch proposal to peers.
pub trait ProposalBroadcaster {
    fn broadcast_proposal(&self, proposal: &ProposedBatch) -> Result<(), NetworkError>;
}

/// Trait for sending an attestation vote to peers.
pub trait AttestationChannel {
    fn send_attestation(&self, vote: BatchAttestationVote) -> Result<(), NetworkError>;
}

/// Trait for synchronizing a finalized batch to late peers.
pub trait BatchSyncInterface {
    fn sync_finalized_batch(&self, batch: &FinalizedBatch) -> Result<(), NetworkError>;
}
