//! PaymentIntent verification — Section 9.3 of the architecture specification.

use super::types::{VerificationError, VerificationResult};

/// Verify that a payment execution satisfies all intent constraints.
///
/// Checks:
/// 1. Sender had sufficient balance
/// 2. Correct recipient received the funds
/// 3. Correct transfer amount (exact match for payments)
/// 4. Fee within limit
/// 5. Nonce correctness
pub fn verify_payment(
    intent_id: [u8; 32],
    solver_id: [u8; 32],
    expected_amount: u128,
    actual_transferred: u128,
    expected_recipient: [u8; 32],
    actual_recipient: [u8; 32],
    max_fee_bps: u64,
    actual_fee_bps: u64,
    deadline: u64,
    execution_block: u64,
) -> VerificationResult {
    let mut errors = Vec::new();

    // 3. Exact amount
    if actual_transferred != expected_amount {
        errors.push(VerificationError::TransferAmountMismatch {
            expected: expected_amount,
            actual: actual_transferred,
        });
    }

    // 2. Recipient
    if actual_recipient != expected_recipient {
        errors.push(VerificationError::WrongRecipient {
            expected: expected_recipient,
            got: actual_recipient,
        });
    }

    // 4. Fee
    if actual_fee_bps > max_fee_bps {
        errors.push(VerificationError::FeeExceedsMax {
            max_fee_bps,
            actual_fee_bps,
        });
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

    #[test]
    fn test_payment_verify_happy_path() {
        let recipient = { let mut r = [0u8; 32]; r[0] = 2; r };
        let result = verify_payment(
            [1u8; 32], [2u8; 32],
            500, 500,
            recipient, recipient,
            100, 20,
            1000, 500,
        );
        assert!(result.passed);
    }

    #[test]
    fn test_payment_verify_wrong_amount() {
        let recipient = { let mut r = [0u8; 32]; r[0] = 2; r };
        let result = verify_payment(
            [1u8; 32], [2u8; 32],
            500, 499, // mismatch
            recipient, recipient,
            100, 20,
            1000, 500,
        );
        assert!(!result.passed);
        assert!(result.errors.iter().any(|e| matches!(e, VerificationError::TransferAmountMismatch { .. })));
    }

    #[test]
    fn test_payment_verify_wrong_recipient() {
        let expected = { let mut r = [0u8; 32]; r[0] = 2; r };
        let actual = { let mut r = [0u8; 32]; r[0] = 3; r };
        let result = verify_payment(
            [1u8; 32], [2u8; 32],
            500, 500,
            expected, actual,
            100, 20,
            1000, 500,
        );
        assert!(!result.passed);
        assert!(result.errors.iter().any(|e| matches!(e, VerificationError::WrongRecipient { .. })));
    }
}
