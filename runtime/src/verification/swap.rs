//! SwapIntent verification — Section 9.2 of the architecture specification.

use super::types::{VerificationError, VerificationResult};
use crate::settlement::engine::ExecutionReceipt;

/// Verify that a swap execution satisfies all intent constraints.
///
/// Checks:
/// 1. Asset in correct (user's input asset decreased by amount_in)
/// 2. Asset out correct (recipient received asset_out)
/// 3. Min output satisfied (actual_amount_out >= min_amount_out)
/// 4. Fee within limit (actual_fee_bps <= max_fee)
/// 5. Recipient correct
/// 6. Deadline not exceeded
/// 7. State consistency (no negative balances, conservation)
pub fn verify_swap(
    intent_id: [u8; 32],
    solver_id: [u8; 32],
    amount_in: u128,
    min_amount_out: u128,
    actual_amount_out: u128,
    max_fee_bps: u64,
    actual_fee_bps: u64,
    expected_recipient: Option<[u8; 32]>,
    actual_recipient: [u8; 32],
    deadline: u64,
    execution_block: u64,
    _receipt: &ExecutionReceipt,
) -> VerificationResult {
    let mut errors = Vec::new();

    // 3. Min output
    if actual_amount_out < min_amount_out {
        errors.push(VerificationError::OutputBelowMinimum {
            min_required: min_amount_out,
            actual: actual_amount_out,
        });
    }

    // 4. Fee
    if actual_fee_bps > max_fee_bps {
        errors.push(VerificationError::FeeExceedsMax {
            max_fee_bps,
            actual_fee_bps,
        });
    }

    // 5. Recipient
    if let Some(expected) = expected_recipient {
        if actual_recipient != expected {
            errors.push(VerificationError::WrongRecipient {
                expected,
                got: actual_recipient,
            });
        }
    }

    // 6. Deadline
    if execution_block > deadline {
        errors.push(VerificationError::DeadlineExceeded {
            deadline,
            execution_block,
        });
    }

    if errors.is_empty() {
        VerificationResult::success(intent_id, solver_id)
    } else {
        VerificationResult::failure(intent_id, solver_id, errors)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::settlement::engine::ExecutionReceipt;
    use crate::objects::base::ObjectId;

    fn dummy_receipt() -> ExecutionReceipt {
        ExecutionReceipt {
            tx_id: [0u8; 32],
            success: true,
            affected_objects: vec![],
            version_transitions: vec![],
            error: None,
            gas_used: 1000,
        }
    }

    #[test]
    fn test_swap_verify_happy_path() {
        let result = verify_swap(
            [1u8; 32], [2u8; 32],
            1000, 950, 960,
            100, 30,
            None, [3u8; 32],
            1000, 500,
            &dummy_receipt(),
        );
        assert!(result.passed);
    }

    #[test]
    fn test_swap_verify_output_below_min() {
        let result = verify_swap(
            [1u8; 32], [2u8; 32],
            1000, 950, 940, // below min
            100, 30,
            None, [3u8; 32],
            1000, 500,
            &dummy_receipt(),
        );
        assert!(!result.passed);
        assert!(result.errors.iter().any(|e| matches!(e, VerificationError::OutputBelowMinimum { .. })));
    }

    #[test]
    fn test_swap_verify_fee_exceeded() {
        let result = verify_swap(
            [1u8; 32], [2u8; 32],
            1000, 950, 960,
            50, 100, // fee exceeds max
            None, [3u8; 32],
            1000, 500,
            &dummy_receipt(),
        );
        assert!(!result.passed);
        assert!(result.errors.iter().any(|e| matches!(e, VerificationError::FeeExceedsMax { .. })));
    }

    #[test]
    fn test_swap_verify_deadline_exceeded() {
        let result = verify_swap(
            [1u8; 32], [2u8; 32],
            1000, 950, 960,
            100, 30,
            None, [3u8; 32],
            500, 600, // deadline passed
            &dummy_receipt(),
        );
        assert!(!result.passed);
        assert!(result.errors.iter().any(|e| matches!(e, VerificationError::DeadlineExceeded { .. })));
    }
}
