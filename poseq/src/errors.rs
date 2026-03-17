#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ValidationError {
    ZeroSubmissionId,
    ZeroSender,
    EmptyPayloadHash,
    PayloadTooLarge { size: usize, max: usize },
    InvalidPayloadKind(String),
    MissingNonce,
    MalformedEnvelope(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum OrderingError {
    EmptyInput,
    PolicyViolation(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BatchingError {
    EmptyOrderedSet,
    BatchSizeExceeded { count: usize, max: usize },
    InvalidBatchTransition(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BridgeError {
    EmptyBatch,
    SerializationFailure(String),
    RuntimeDeliveryFailed(String),
    // Phase 2 additions
    AlreadyDelivered,
    AckReplay,
    RejectedByRuntime,
    DeliveryFailed,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ProposalError {
    NotLeader,
    AlreadyProposed,
    InvalidSlot,
    InvalidParent,
    InvalidBatchRoot,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AttestationError {
    NotEligible,
    DuplicateVote,
    ConflictingVote,
    ProposalNotFound,
    AlreadyFinalized,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FinalizationError {
    InsufficientQuorum,
    ConflictDetected,
    AlreadyFinalized,
    ProposalNotFound,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ReplayError {
    DuplicateSubmission,
    DuplicateProposal,
    DuplicateAck,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StateRecoveryError {
    CorruptState,
    MissingData,
    VersionMismatch,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum NetworkError {
    Timeout,
    Disconnected,
    Rejected,
    InvalidPayload,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PoSeqError {
    Validation(ValidationError),
    Duplicate([u8; 32]),       // submission_id
    ReplayDetected([u8; 32]), // submission_id
    QueueFull { capacity: usize },
    Ordering(OrderingError),
    Batching(BatchingError),
    Bridge(BridgeError),
    PolicyViolation(String),
    AttestationError(String),
    // Phase 6: Intent-Based Execution
    IntentValidation(crate::intent_pool::types::IntentValidationError),
    Auction(crate::auction::state::AuctionError),
}

impl From<ValidationError> for PoSeqError { fn from(e: ValidationError) -> Self { PoSeqError::Validation(e) } }
impl From<OrderingError>  for PoSeqError { fn from(e: OrderingError)  -> Self { PoSeqError::Ordering(e) } }
impl From<BatchingError>  for PoSeqError { fn from(e: BatchingError)  -> Self { PoSeqError::Batching(e) } }
impl From<BridgeError>    for PoSeqError { fn from(e: BridgeError)    -> Self { PoSeqError::Bridge(e) } }
impl From<crate::intent_pool::types::IntentValidationError> for PoSeqError {
    fn from(e: crate::intent_pool::types::IntentValidationError) -> Self { PoSeqError::IntentValidation(e) }
}
impl From<crate::auction::state::AuctionError> for PoSeqError {
    fn from(e: crate::auction::state::AuctionError) -> Self { PoSeqError::Auction(e) }
}

// ─── Phase 3 Error Types ──────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FairnessError {
    PolicyVersionMismatch { expected: u32, got: u32 },
    InvalidFairnessClass,
    ClassificationFailed(String),
    UnknownRule(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InclusionError {
    BatchCapacityExceeded,
    SnapshotExpired,
    NoEligibleSubmissions,
    ForcedInclusionConflict,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AntiMevError {
    ReorderBoundViolated,
    ProtectedFlowViolated,
    SnapshotCommitmentMissing,
    LeaderDiscretionExceeded,
    OrderingCommitmentRequired,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SnapshotError {
    SnapshotNotFound,
    StaleSnapshot,
    CommitmentMismatch,
    EmptySnapshot,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AuditError {
    AuditNotFound,
    AuditHashMismatch,
    RecordCorrupted,
}

// ─── Phase 4 Error Types ──────────────────────────────────────────────────────

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Phase4Error {
    /// Signer not found in registry.
    UnknownSigner([u8; 32]),
    /// Duplicate signature from same signer.
    DuplicateSignature([u8; 32]),
    /// Signature verification failed.
    InvalidSignature([u8; 32]),
    /// Committee rotation config invalid (e.g., min > max).
    InvalidRotationConfig(String),
    /// Not enough candidates for committee.
    InsufficientCandidates { required: usize, available: usize },
    /// Epoch not found in store.
    EpochNotFound(u64),
    /// Node is jailed.
    NodeJailed([u8; 32]),
    /// Node already jailed.
    AlreadyJailed([u8; 32]),
    /// Node not jailed (cannot unjail).
    NotJailed([u8; 32]),
    /// Unjail cooldown not elapsed.
    UnjailCooldownActive { eligible_at: u64, current: u64 },
    /// Serialization error.
    SerializationError(String),
    /// Deserialization error.
    DeserializationError(String),
    /// Key not found in persistence backend.
    KeyNotFound(String),
    /// Simulation scenario error.
    SimulationError(String),
    /// General offense-related error.
    OffenseError(String),
}
