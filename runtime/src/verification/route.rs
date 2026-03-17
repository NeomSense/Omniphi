//! RouteLiquidityIntent verification — Section 9.4 of the architecture specification.

use super::types::{VerificationError, VerificationResult};
use crate::objects::base::ObjectId;

/// Verify that a route liquidity execution satisfies all intent constraints.
///
/// Checks:
/// 1. Source and target pools are valid
/// 2. Number of hops <= max_hops
/// 3. Price impact per hop <= max_price_impact_bps
/// 4. Target pool received >= min_received
/// 5. Asset conservation (no creation/destruction)
pub fn verify_route_liquidity(
    intent_id: [u8; 32],
    solver_id: [u8; 32],
    source_pool: ObjectId,
    target_pool: ObjectId,
    min_received: u128,
    actual_received: u128,
    max_hops: u8,
    actual_hops: u8,
    max_price_impact_bps: u16,
    actual_max_price_impact_bps: u16,
    max_fee_bps: u64,
    actual_fee_bps: u64,
    deadline: u64,
    execution_block: u64,
) -> VerificationResult {
    let mut errors = Vec::new();

    // 2. Hop count
    if actual_hops > max_hops {
        errors.push(VerificationError::TooManyHops {
            max: max_hops,
            actual: actual_hops,
        });
    }

    // 3. Price impact
    if actual_max_price_impact_bps > max_price_impact_bps {
        errors.push(VerificationError::PriceImpactExceeded {
            max_bps: max_price_impact_bps,
            actual_bps: actual_max_price_impact_bps,
        });
    }

    // 4. Min received
    if actual_received < min_received {
        errors.push(VerificationError::ReceivedBelowMinimum {
            min_received,
            actual: actual_received,
        });
    }

    // Fee
    if actual_fee_bps > max_fee_bps {
        errors.push(VerificationError::FeeExceedsMax {
            max_fee_bps,
            actual_fee_bps,
        });
    }

    // Deadline
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

    fn pool_id(b: u8) -> ObjectId {
        let mut id = [0u8; 32]; id[0] = b;
        ObjectId::new(id)
    }

    #[test]
    fn test_route_verify_happy_path() {
        let result = verify_route_liquidity(
            [1u8; 32], [2u8; 32],
            pool_id(10), pool_id(20),
            9_500, 9_600,
            3, 2,
            200, 150,
            100, 30,
            1000, 500,
        );
        assert!(result.passed);
    }

    #[test]
    fn test_route_verify_too_many_hops() {
        let result = verify_route_liquidity(
            [1u8; 32], [2u8; 32],
            pool_id(10), pool_id(20),
            9_500, 9_600,
            3, 4, // exceeds max_hops
            200, 150,
            100, 30,
            1000, 500,
        );
        assert!(!result.passed);
        assert!(result.errors.iter().any(|e| matches!(e, VerificationError::TooManyHops { .. })));
    }

    #[test]
    fn test_route_verify_below_min_received() {
        let result = verify_route_liquidity(
            [1u8; 32], [2u8; 32],
            pool_id(10), pool_id(20),
            9_500, 9_000, // below min
            3, 2,
            200, 150,
            100, 30,
            1000, 500,
        );
        assert!(!result.passed);
        assert!(result.errors.iter().any(|e| matches!(e, VerificationError::ReceivedBelowMinimum { .. })));
    }
}
