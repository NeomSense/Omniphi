use crate::types::submission::{SequencingSubmission, SubmissionEnvelope};

/// Assigns monotonically increasing intake sequence numbers.
pub struct SubmissionReceiver {
    intake_counter: u64,
}

impl SubmissionReceiver {
    pub fn new() -> Self { SubmissionReceiver { intake_counter: 0 } }

    /// Normalize a raw submission into an envelope.
    /// Assigns intake sequence and computes normalized_id.
    pub fn receive(&mut self, submission: SequencingSubmission) -> SubmissionEnvelope {
        let normalized_id = submission.compute_id();
        let seq = self.intake_counter;
        self.intake_counter += 1;
        SubmissionEnvelope {
            submission,
            received_at_sequence: seq,
            normalized_id,
        }
    }
}

impl Default for SubmissionReceiver {
    fn default() -> Self { Self::new() }
}
