//! Parameter stress tests and fairness preservation proofs.
//!
//! Tests dangerous parameter settings, validates safe ranges,
//! and proves fairness guarantees hold under fee pressure.

#[cfg(test)]
mod tests {
    use crate::gas::calculator::{CongestionState, PoSeqFeeCalculator};
    use crate::gas::parameters::PoSeqFeeParameters;
    use crate::gas::priority::{compute_priority, PriorityConfig, PriorityInput};
    use crate::gas::routing::distribute_fee;
    use crate::gas::types::FeeEnvelope;
    use crate::gas::lifecycle::{FeeLifecycle, FeeLedger};

    fn payer() -> [u8; 32] { [1u8; 32] }

    // ═══════════════════════════════════════════════════════════════════
    // PARAMETER STRESS TESTS
    // ═══════════════════════════════════════════════════════════════════

    #[test]
    fn test_admission_fee_too_low_detected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.base_admission_fee = 0;
        assert!(params.validate().is_err());
    }

    #[test]
    fn test_tip_cap_too_high_still_bounded() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.priority_tip_cap = u128::MAX / 2;
        // Should still pass validation (tip cap is user-side only)
        assert!(params.validate().is_ok());

        // But the priority model must still bound the advantage
        let config = PriorityConfig::default();
        let max_tip_input = PriorityInput {
            fairness_class_weight: 3_000,
            admitted_at_block: 100,
            deadline: 200,
            current_block: 100,
            effective_tip: u128::MAX / 2,
            tip_cap: u128::MAX / 2,
            is_protected: false,
        };
        let score = compute_priority(&config, &max_tip_input);
        // Tip component at max is 10000 * 1500 / 10000 = 1500
        assert_eq!(score.tip_component, 1_500);
    }

    #[test]
    fn test_congestion_max_too_high_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.max_congestion_multiplier_bps = 2_000_000; // 200x
        assert!(params.validate().is_err());
    }

    #[test]
    fn test_congestion_min_zero_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.min_congestion_multiplier_bps = 0;
        assert!(params.validate().is_err());
    }

    #[test]
    fn test_storage_growth_cost_zero_still_works() {
        // Zero storage growth cost shouldn't crash, just no storage charge
        let params = PoSeqFeeParameters::testnet_defaults();
        let envelope = FeeEnvelope {
            payer: payer(),
            max_poseq_fee: 100_000,
            max_runtime_fee: 100_000,
            priority_tip: 0,
            expiry: 1000,
        };
        let cong = CongestionState::new(64, 64);
        let result = PoSeqFeeCalculator::compute(&params, &envelope, 0, &cong, 500);
        assert!(result.is_ok());
    }

    #[test]
    fn test_fee_splits_malformed_rejected() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.sequencer_reward_bps = 0;
        params.shared_security_bps = 0;
        params.treasury_bps = 0;
        params.burn_bps = 0;
        assert!(params.validate().is_err());
    }

    #[test]
    fn test_expiry_penalty_100_percent() {
        let mut params = PoSeqFeeParameters::testnet_defaults();
        params.expiry_penalty_bps = 10_000; // 100% penalty
        assert!(params.validate().is_ok()); // valid but aggressive

        let penalty = PoSeqFeeCalculator::compute_expiry_penalty(&params, 5_000);
        assert_eq!(penalty, 5_000); // full fee retained
    }

    #[test]
    fn test_extreme_congestion_doesnt_overflow() {
        let params = PoSeqFeeParameters::testnet_defaults();
        let envelope = FeeEnvelope {
            payer: payer(),
            max_poseq_fee: u128::MAX / 2,
            max_runtime_fee: u128::MAX / 4,
            priority_tip: 0,
            expiry: u64::MAX / 2,
        };
        let cong = CongestionState::new(u64::MAX / 2, u64::MAX / 2);
        // Should not panic, just clamp
        let _ = PoSeqFeeCalculator::compute(&params, &envelope, u64::MAX / 2, &cong, 100);
    }

    #[test]
    fn test_all_default_presets_validate() {
        assert!(PoSeqFeeParameters::testnet_defaults().validate().is_ok());
    }

    // ─── Parameter sweep: verify distribution budget-neutral across range ──

    #[test]
    fn test_distribution_sweep() {
        let params = PoSeqFeeParameters::testnet_defaults();
        // Sweep fees from 0 to 100,000 in steps
        for fee in (0..100_001).step_by(17) {
            let dist = distribute_fee(&params, fee);
            assert!(dist.verify_budget_neutral(),
                "budget neutrality failed at fee={}", fee);
        }
    }

    // ═══════════════════════════════════════════════════════════════════
    // FAIRNESS PRESERVATION PROOFS
    // ═══════════════════════════════════════════════════════════════════

    /// PROOF 1: SafetyCritical always outranks Normal, regardless of tip.
    #[test]
    fn test_fairness_proof_1_class_dominance() {
        let config = PriorityConfig::default();

        for tip in [0u128, 10_000, 25_000, 50_000, 100_000] {
            let effective_tip = std::cmp::min(tip, 50_000); // cap at 50000

            let safety = PriorityInput {
                fairness_class_weight: 9_000,
                admitted_at_block: 100,
                deadline: 200,
                current_block: 100,
                effective_tip: 0, // SC with NO tip
                tip_cap: 50_000,
                is_protected: true,
            };

            let normal = PriorityInput {
                fairness_class_weight: 3_000,
                admitted_at_block: 100,
                deadline: 200,
                current_block: 100,
                effective_tip,  // Normal with MAXIMUM tip
                tip_cap: 50_000,
                is_protected: false,
            };

            let sc = compute_priority(&config, &safety);
            let nm = compute_priority(&config, &normal);

            assert!(sc.composite > nm.composite,
                "FAIRNESS VIOLATION: SC(tip=0, score={}) <= Normal(tip={}, score={})",
                sc.composite, effective_tip, nm.composite);
        }
    }

    /// PROOF 2: Starvation boost eventually overcomes any tip advantage.
    #[test]
    fn test_fairness_proof_2_starvation_beats_tip() {
        let config = PriorityConfig::default();

        // Same fairness class, same deadline, but starving vs whale
        let starving = PriorityInput {
            fairness_class_weight: 3_000,
            admitted_at_block: 10,
            deadline: 500,
            current_block: 200, // 190 blocks waiting → 90 beyond threshold
            effective_tip: 0,
            tip_cap: 50_000,
            is_protected: false,
        };

        let whale = PriorityInput {
            fairness_class_weight: 3_000,
            admitted_at_block: 200,
            deadline: 500,
            current_block: 200, // just admitted
            effective_tip: 50_000, // max tip
            tip_cap: 50_000,
            is_protected: false,
        };

        let starving_score = compute_priority(&config, &starving);
        let whale_score = compute_priority(&config, &whale);

        assert!(starving_score.starvation_active);
        assert!(!whale_score.starvation_active);
        assert!(starving_score.composite > whale_score.composite,
            "STARVATION VIOLATION: starving(score={}) <= whale(score={})",
            starving_score.composite, whale_score.composite);
    }

    /// PROOF 3: Urgency matters — near-deadline intent beats far-deadline.
    #[test]
    fn test_fairness_proof_3_urgency_works() {
        let config = PriorityConfig::default();

        let near = PriorityInput {
            fairness_class_weight: 5_000,
            admitted_at_block: 100,
            deadline: 130, // 30 blocks remaining (within 50-block horizon)
            current_block: 100,
            effective_tip: 1_000,
            tip_cap: 50_000,
            is_protected: false,
        };

        let far = PriorityInput {
            fairness_class_weight: 5_000,
            admitted_at_block: 100,
            deadline: 300, // 200 blocks remaining (beyond horizon)
            current_block: 100,
            effective_tip: 5_000, // higher tip but farther deadline
            tip_cap: 50_000,
            is_protected: false,
        };

        let near_score = compute_priority(&config, &near);
        let far_score = compute_priority(&config, &far);

        assert!(near_score.urgency_component > far_score.urgency_component);
    }

    /// PROOF 4: Zero-tip intents always have nonzero priority.
    #[test]
    fn test_fairness_proof_4_zero_tip_viable() {
        let config = PriorityConfig::default();

        for class in [3_000u64, 5_000, 8_000, 9_000] {
            let input = PriorityInput {
                fairness_class_weight: class,
                admitted_at_block: 100,
                deadline: 200,
                current_block: 100,
                effective_tip: 0,
                tip_cap: 50_000,
                is_protected: false,
            };
            let score = compute_priority(&config, &input);
            assert!(score.composite > 0,
                "zero-tip intent with class {} should have nonzero priority", class);
        }
    }

    /// PROOF 5: Tip advantage is bounded — max advantage is < 100%.
    #[test]
    fn test_fairness_proof_5_tip_advantage_bounded() {
        let config = PriorityConfig::default();

        let no_tip = PriorityInput {
            fairness_class_weight: 5_000,
            admitted_at_block: 100,
            deadline: 200,
            current_block: 100,
            effective_tip: 0,
            tip_cap: 50_000,
            is_protected: false,
        };

        let max_tip = PriorityInput {
            fairness_class_weight: 5_000,
            admitted_at_block: 100,
            deadline: 200,
            current_block: 100,
            effective_tip: 50_000,
            tip_cap: 50_000,
            is_protected: false,
        };

        let no_score = compute_priority(&config, &no_tip);
        let max_score = compute_priority(&config, &max_tip);

        let advantage_bps = if no_score.composite > 0 {
            ((max_score.composite as u128) - (no_score.composite as u128))
                * 10_000
                / (no_score.composite as u128)
        } else {
            0
        };

        assert!(advantage_bps < 10_000,
            "tip advantage {} bps exceeds 100%", advantage_bps);

        // Tip weight is 15% (1500 bps). For an intent with only fairness component
        // (no urgency, no starvation), the no-tip score equals the fairness component
        // alone. Adding max tip (1500) to a fairness base of ~2000 gives ~75% advantage.
        // This is by design: tips matter within a class, but cannot override class ordering.
        // The critical invariant (Proof 1) is that class dominance holds regardless.
        assert!(advantage_bps < 8_000,
            "tip advantage {} bps exceeds 80% bound", advantage_bps);
    }

    // ═══════════════════════════════════════════════════════════════════
    // LIFECYCLE STRESS TESTS
    // ═══════════════════════════════════════════════════════════════════

    #[test]
    fn test_lifecycle_100_intents_budget_consistent() {
        let mut ledger = FeeLedger::new();
        let params = PoSeqFeeParameters::testnet_defaults();
        let cong = CongestionState::new(64, 64);

        for i in 0u8..100 {
            let mut id = [0u8; 32]; id[0] = i;
            let lc = ledger.begin(id).unwrap();

            let envelope = FeeEnvelope {
                payer: payer(),
                max_poseq_fee: 50_000 + (i as u128 * 1_000),
                max_runtime_fee: 100_000,
                priority_tip: (i as u128) * 500,
                expiry: 10_000,
            };

            let fee = PoSeqFeeCalculator::compute(&params, &envelope, 200, &cong, 500);

            match fee {
                Ok(result) => {
                    lc.reserve(payer(), envelope.max_poseq_fee, envelope.max_runtime_fee, 500).unwrap();
                    lc.charge_sequencing(result.charged_fee, 501).unwrap();
                    let runtime_fee = (2000 + i as u128 * 100).min(envelope.max_runtime_fee);
                    lc.charge_runtime(runtime_fee, 502).unwrap();
                    lc.issue_refund(503).unwrap();
                    lc.close(504).unwrap();

                    assert!(lc.verify_budget_consistency(),
                        "budget consistency failed for intent {}", i);
                }
                Err(_) => {
                    lc.reject("insufficient fee").unwrap();
                }
            }
        }

        assert_eq!(ledger.active_count(), 0, "all lifecycles should be closed");
    }

    #[test]
    fn test_lifecycle_mixed_outcomes() {
        let mut ledger = FeeLedger::new();
        let params = PoSeqFeeParameters::testnet_defaults();

        // Intent 1: happy path
        let mut id1 = [0u8; 32]; id1[0] = 1;
        let lc1 = ledger.begin(id1).unwrap();
        lc1.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc1.charge_sequencing(3_000, 105).unwrap();
        lc1.charge_runtime(20_000, 110).unwrap();
        lc1.issue_refund(115).unwrap();
        lc1.close(120).unwrap();

        // Intent 2: expires
        let mut id2 = [0u8; 32]; id2[0] = 2;
        let lc2 = ledger.begin(id2).unwrap();
        lc2.reserve(payer(), 10_000, 50_000, 100).unwrap();
        let penalty = PoSeqFeeCalculator::compute_expiry_penalty(&params, 3_000);
        lc2.expire(penalty, 200).unwrap();
        lc2.issue_refund(201).unwrap();
        lc2.close(202).unwrap();

        // Intent 3: cancelled
        let mut id3 = [0u8; 32]; id3[0] = 3;
        let lc3 = ledger.begin(id3).unwrap();
        lc3.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc3.cancel(150).unwrap();
        lc3.issue_refund(151).unwrap();
        lc3.close(152).unwrap();

        // Intent 4: rejected
        let mut id4 = [0u8; 32]; id4[0] = 4;
        let lc4 = ledger.begin(id4).unwrap();
        lc4.reject("spam").unwrap();

        assert_eq!(ledger.active_count(), 0);

        // Verify all closed/rejected
        for id in [id1, id2, id3, id4] {
            let lc = ledger.get(&id).unwrap();
            assert!(lc.state.is_terminal());
        }
    }

    #[test]
    fn test_lifecycle_no_double_charge_after_multiple_transitions() {
        let mut id = [0u8; 32]; id[0] = 99;
        let mut lc = FeeLifecycle::new(id);
        lc.reserve(payer(), 10_000, 50_000, 100).unwrap();
        lc.charge_sequencing(3_000, 105).unwrap();

        // Try double sequencing charge
        let result = lc.charge_sequencing(3_000, 106);
        assert!(result.is_err());

        lc.charge_runtime(20_000, 110).unwrap();

        // Try double runtime charge
        let result = lc.charge_runtime(5_000, 111);
        assert!(result.is_err());
    }
}
