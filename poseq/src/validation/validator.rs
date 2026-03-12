use crate::types::submission::SubmissionEnvelope;
use crate::config::policy::PoSeqPolicy;
use crate::errors::ValidationError;

#[derive(Debug, Clone)]
pub struct ValidatedSubmission {
    pub envelope: SubmissionEnvelope,
    pub normalized_class: crate::config::policy::SubmissionClass,
    pub payload_size: usize,
}

#[derive(Debug, Clone)]
pub struct SubmissionValidationResult {
    pub normalized_id: [u8; 32],
    pub passed: bool,
    pub error: Option<ValidationError>,
}

pub struct SubmissionValidator;

impl SubmissionValidator {
    pub fn validate(
        envelope: SubmissionEnvelope,
        policy: &PoSeqPolicy,
    ) -> Result<ValidatedSubmission, ValidationError> {
        let s = &envelope.submission;

        // 1. submission_id must not be all zeros
        if s.submission_id == [0u8; 32] {
            return Err(ValidationError::ZeroSubmissionId);
        }
        // 2. sender must not be all zeros
        if s.sender == [0u8; 32] {
            return Err(ValidationError::ZeroSender);
        }
        // 3. payload_hash must not be all zeros
        if s.payload_hash == [0u8; 32] {
            return Err(ValidationError::EmptyPayloadHash);
        }
        // 4. payload size check
        let payload_size = s.payload_body.len();
        if payload_size > policy.batch.max_payload_bytes_per_submission {
            return Err(ValidationError::PayloadTooLarge {
                size: payload_size,
                max: policy.batch.max_payload_bytes_per_submission,
            });
        }
        // 5. class must be allowed
        if !policy.is_class_allowed(&s.class) {
            return Err(ValidationError::InvalidPayloadKind(format!("{:?}", s.class)));
        }
        // 6. payload hash must match body
        if !s.validate_payload_hash() {
            return Err(ValidationError::MalformedEnvelope("payload_hash does not match payload_body".into()));
        }

        Ok(ValidatedSubmission {
            normalized_class: s.class.clone(),
            payload_size,
            envelope,
        })
    }
}
