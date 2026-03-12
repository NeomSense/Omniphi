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
}

impl From<ValidationError> for PoSeqError { fn from(e: ValidationError) -> Self { PoSeqError::Validation(e) } }
impl From<OrderingError>  for PoSeqError { fn from(e: OrderingError)  -> Self { PoSeqError::Ordering(e) } }
impl From<BatchingError>  for PoSeqError { fn from(e: BatchingError)  -> Self { PoSeqError::Batching(e) } }
impl From<BridgeError>    for PoSeqError { fn from(e: BridgeError)    -> Self { PoSeqError::Bridge(e) } }
