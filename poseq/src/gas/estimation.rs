//! Fee estimation / quote API — helps wallets and solvers estimate costs.
//!
//! Provides deterministic, explainable fee approximations without committing
//! funds. The estimate reflects current congestion, fee parameters, and gas costs.

use serde::{Deserialize, Serialize};
use super::calculator::{CongestionState, PoSeqFeeCalculator};
use super::parameters::PoSeqFeeParameters;
use super::types::FeeEnvelope;

/// Structured fee quote returned by the estimation API.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FeeQuote {
    /// Estimated PoSeq sequencing fee.
    pub estimated_poseq_fee: u128,
    /// Estimated runtime execution fee (based on gas schedule estimate).
    pub estimated_runtime_fee: u128,
    /// Estimated total fee (poseq + runtime).
    pub estimated_total_fee: u128,
    /// Current congestion multiplier in basis points.
    pub congestion_multiplier_bps: u64,
    /// Effective priority tip that would be applied.
    pub priority_tip_used: u128,
    /// Whether the tip was capped by the protocol.
    pub tip_was_capped: bool,
    /// Breakdown of fee components.
    pub breakdown: FeeBreakdown,
    /// Warnings about the estimate.
    pub warnings: Vec<String>,
}

/// Detailed breakdown of estimated fee components.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FeeBreakdown {
    pub base_admission: u128,
    pub bandwidth_fee: u128,
    pub priority_tip: u128,
    pub congestion_adjustment: String,
    pub runtime_gas_estimate: u64,
    pub runtime_gas_price: u128,
}

/// Estimate PoSeq + runtime fees for an intent.
///
/// Arguments:
/// - `params`: current governance fee parameters
/// - `envelope`: the user's proposed fee envelope (used for tip)
/// - `intent_size_bytes`: estimated serialized intent size
/// - `congestion`: current pool/batch congestion state
/// - `current_block`: current block height
/// - `estimated_runtime_gas`: estimated gas units for execution
/// - `runtime_gas_price`: OMNI per gas unit (from runtime gas schedule)
pub fn estimate_fee(
    params: &PoSeqFeeParameters,
    envelope: &FeeEnvelope,
    intent_size_bytes: u64,
    congestion: &CongestionState,
    current_block: u64,
    estimated_runtime_gas: u64,
    runtime_gas_price: u128,
) -> FeeQuote {
    let mut warnings = Vec::new();

    // Compute PoSeq fee estimate
    let poseq_result = PoSeqFeeCalculator::compute(
        params, envelope, intent_size_bytes, congestion, current_block,
    );

    let (estimated_poseq_fee, congestion_multiplier_bps, tip_used, tip_capped, breakdown_base, breakdown_bytes) =
        match poseq_result {
            Ok(result) => {
                let tip_capped = envelope.priority_tip > params.priority_tip_cap;
                (
                    result.charged_fee,
                    result.congestion_multiplier_bps,
                    result.effective_tip,
                    tip_capped,
                    result.base_fee,
                    result.bytes_fee,
                )
            }
            Err(e) => {
                warnings.push(format!("poseq fee computation error: {}", e));
                // Fall back to floor estimate
                (
                    params.poseq_fee_floor,
                    10_000,
                    0,
                    false,
                    params.base_admission_fee,
                    (intent_size_bytes as u128).saturating_mul(params.fee_per_byte),
                )
            }
        };

    // Compute runtime fee estimate
    let estimated_runtime_fee = (estimated_runtime_gas as u128).saturating_mul(runtime_gas_price);

    // Generate warnings
    if congestion_multiplier_bps >= 30_000 {
        warnings.push("high congestion: fees are 3x+ normal".to_string());
    }
    if tip_capped {
        warnings.push(format!(
            "priority tip capped from {} to {}",
            envelope.priority_tip, params.priority_tip_cap
        ));
    }
    if envelope.max_poseq_fee < estimated_poseq_fee {
        warnings.push(format!(
            "max_poseq_fee ({}) may be insufficient (estimated {})",
            envelope.max_poseq_fee, estimated_poseq_fee
        ));
    }
    if envelope.max_runtime_fee < estimated_runtime_fee {
        warnings.push(format!(
            "max_runtime_fee ({}) may be insufficient (estimated {})",
            envelope.max_runtime_fee, estimated_runtime_fee
        ));
    }
    if envelope.is_expired(current_block) {
        warnings.push("fee envelope has already expired".to_string());
    }

    let congestion_desc = if congestion_multiplier_bps < 10_000 {
        format!("{}% below target (discount)", (10_000 - congestion_multiplier_bps) / 100)
    } else if congestion_multiplier_bps == 10_000 {
        "at target (1.0x)".to_string()
    } else {
        format!("{}% above target (surcharge)", (congestion_multiplier_bps - 10_000) / 100)
    };

    FeeQuote {
        estimated_poseq_fee,
        estimated_runtime_fee,
        estimated_total_fee: estimated_poseq_fee.saturating_add(estimated_runtime_fee),
        congestion_multiplier_bps,
        priority_tip_used: tip_used,
        tip_was_capped: tip_capped,
        breakdown: FeeBreakdown {
            base_admission: breakdown_base,
            bandwidth_fee: breakdown_bytes,
            priority_tip: tip_used,
            congestion_adjustment: congestion_desc,
            runtime_gas_estimate: estimated_runtime_gas,
            runtime_gas_price,
        },
        warnings,
    }
}

/// Suggest a fee envelope that should be sufficient given current conditions.
/// Adds a 20% safety margin over the estimate.
pub fn suggest_envelope(
    params: &PoSeqFeeParameters,
    intent_size_bytes: u64,
    congestion: &CongestionState,
    priority_tip: u128,
    estimated_runtime_gas: u64,
    runtime_gas_price: u128,
    expiry_blocks: u64,
    current_block: u64,
    payer: [u8; 32],
) -> FeeEnvelope {
    let effective_tip = std::cmp::min(priority_tip, params.priority_tip_cap);

    // Estimate base poseq fee
    let base = params.base_admission_fee;
    let bytes = (intent_size_bytes as u128).saturating_mul(params.fee_per_byte);
    let multiplier_bps = PoSeqFeeCalculator::compute_congestion_multiplier(params, congestion);
    let raw_poseq = (base + bytes + effective_tip)
        .saturating_mul(multiplier_bps as u128) / 10_000;
    let poseq_fee = std::cmp::max(raw_poseq, params.poseq_fee_floor);

    // Add 20% safety margin
    let margin_poseq = poseq_fee.saturating_mul(12) / 10;

    let estimated_runtime = (estimated_runtime_gas as u128).saturating_mul(runtime_gas_price);
    let margin_runtime = estimated_runtime.saturating_mul(12) / 10;

    FeeEnvelope {
        payer,
        max_poseq_fee: margin_poseq,
        max_runtime_fee: margin_runtime,
        priority_tip,
        expiry: current_block + expiry_blocks,
    }
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn default_params() -> PoSeqFeeParameters {
        PoSeqFeeParameters::testnet_defaults()
    }

    fn make_envelope() -> FeeEnvelope {
        FeeEnvelope {
            payer: [1u8; 32],
            max_poseq_fee: 100_000,
            max_runtime_fee: 500_000,
            priority_tip: 5_000,
            expiry: 1000,
        }
    }

    #[test]
    fn test_basic_estimate() {
        let params = default_params();
        let env = make_envelope();
        let cong = CongestionState::new(64, 64);
        let quote = estimate_fee(&params, &env, 200, &cong, 500, 5_000, 1);

        assert!(quote.estimated_poseq_fee > 0);
        assert_eq!(quote.estimated_runtime_fee, 5_000); // 5000 gas * 1 price
        assert_eq!(quote.estimated_total_fee,
            quote.estimated_poseq_fee + quote.estimated_runtime_fee);
        assert_eq!(quote.congestion_multiplier_bps, 10_000);
        assert!(!quote.tip_was_capped);
        assert!(quote.warnings.is_empty());
    }

    #[test]
    fn test_estimate_warns_on_high_congestion() {
        let params = default_params();
        let env = make_envelope();
        let cong = CongestionState::new(320, 320); // 5x target
        let quote = estimate_fee(&params, &env, 200, &cong, 500, 5_000, 1);

        assert!(quote.warnings.iter().any(|w| w.contains("high congestion")));
    }

    #[test]
    fn test_estimate_warns_on_tip_cap() {
        let params = default_params();
        let mut env = make_envelope();
        env.priority_tip = 100_000; // exceeds cap of 50,000
        let cong = CongestionState::new(64, 64);
        let quote = estimate_fee(&params, &env, 200, &cong, 500, 5_000, 1);

        assert!(quote.tip_was_capped);
        assert!(quote.warnings.iter().any(|w| w.contains("capped")));
    }

    #[test]
    fn test_estimate_warns_on_insufficient_budget() {
        let params = default_params();
        let mut env = make_envelope();
        env.max_runtime_fee = 100; // too low
        let cong = CongestionState::new(64, 64);
        let quote = estimate_fee(&params, &env, 200, &cong, 500, 50_000, 10);

        assert!(quote.warnings.iter().any(|w| w.contains("insufficient")));
    }

    #[test]
    fn test_estimate_deterministic() {
        let params = default_params();
        let env = make_envelope();
        let cong = CongestionState::new(80, 80);
        let q1 = estimate_fee(&params, &env, 200, &cong, 500, 5_000, 1);
        let q2 = estimate_fee(&params, &env, 200, &cong, 500, 5_000, 1);
        assert_eq!(q1, q2);
    }

    #[test]
    fn test_suggest_envelope() {
        let params = default_params();
        let cong = CongestionState::new(64, 64);
        let env = suggest_envelope(
            &params, 200, &cong, 5_000, 5_000, 1, 100, 500, [1u8; 32],
        );

        assert!(env.validate().is_ok());
        assert!(env.max_poseq_fee >= params.poseq_fee_floor);
        assert_eq!(env.expiry, 600);

        // Verify the suggested envelope can actually pay for the estimate
        let quote = estimate_fee(&params, &env, 200, &cong, 500, 5_000, 1);
        assert!(env.max_poseq_fee >= quote.estimated_poseq_fee);
        assert!(env.max_runtime_fee >= quote.estimated_runtime_fee);
    }

    #[test]
    fn test_estimate_with_expired_envelope() {
        let params = default_params();
        let env = make_envelope();
        let cong = CongestionState::new(64, 64);
        let quote = estimate_fee(&params, &env, 200, &cong, 1001, 5_000, 1);

        assert!(quote.warnings.iter().any(|w| w.contains("expired")));
    }
}
