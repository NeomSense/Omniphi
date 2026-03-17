//! Adversarial economic simulation harness for the PoSeq fee model.
//!
//! Tests economic behavior under various load, congestion, fairness class,
//! and adversarial conditions using deterministic simulation inputs.

use super::calculator::{CongestionState, PoSeqFeeCalculator};
use super::parameters::PoSeqFeeParameters;
use super::priority::{compute_priority, PriorityConfig, PriorityInput};
use super::routing::distribute_fee;
use super::types::FeeEnvelope;
use super::lifecycle::{FeeLifecycle, FeeLedger};
use serde::{Deserialize, Serialize};

/// Simulation configuration.
#[derive(Debug, Clone)]
pub struct SimConfig {
    pub params: PoSeqFeeParameters,
    pub priority_config: PriorityConfig,
    pub num_slots: u64,
    pub intents_per_slot: u64,
}

/// A simulated intent for fee testing.
#[derive(Debug, Clone)]
pub struct SimIntent {
    pub id: [u8; 32],
    pub payer: [u8; 32],
    pub fairness_class_weight: u64,
    pub max_poseq_fee: u128,
    pub max_runtime_fee: u128,
    pub tip: u128,
    pub size_bytes: u64,
    pub deadline_offset: u64,
    pub runtime_gas: u64,
    pub is_protected: bool,
    pub label: String,
}

/// Result of a single simulation slot.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SlotResult {
    pub slot: u64,
    pub intents_submitted: u64,
    pub intents_admitted: u64,
    pub intents_rejected: u64,
    pub total_fees_charged: u128,
    pub total_refunds: u128,
    pub avg_fee: u128,
    pub congestion_bps: u64,
    pub max_priority_score: u64,
    pub min_priority_score: u64,
}

/// Aggregate simulation results.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SimResults {
    pub total_slots: u64,
    pub total_intents_submitted: u64,
    pub total_intents_admitted: u64,
    pub total_intents_rejected: u64,
    pub total_fees_collected: u128,
    pub total_refunds: u128,
    pub total_burned: u128,
    pub avg_fee_per_intent: u128,
    pub avg_congestion_bps: u64,
    pub fairness_class_inclusion: std::collections::BTreeMap<String, (u64, u64)>, // (admitted, total)
    pub tip_effectiveness: TipEffectiveness,
    pub budget_consistent: bool,
    pub slot_results: Vec<SlotResult>,
}

/// Measures how much tips influenced ordering.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TipEffectiveness {
    pub intents_with_tip: u64,
    pub intents_without_tip: u64,
    pub avg_priority_with_tip: u64,
    pub avg_priority_without_tip: u64,
    pub tip_priority_advantage_bps: u64,
}

/// Run a deterministic fee simulation.
pub fn run_simulation(config: &SimConfig, intents: &[SimIntent]) -> SimResults {
    let mut slot_results = Vec::new();
    let mut total_fees = 0u128;
    let mut total_refunds = 0u128;
    let mut total_admitted = 0u64;
    let mut total_rejected = 0u64;
    let mut total_burned = 0u128;
    let mut class_stats: std::collections::BTreeMap<String, (u64, u64)> = std::collections::BTreeMap::new();
    let mut tip_priorities = Vec::new();
    let mut no_tip_priorities = Vec::new();
    let mut budget_consistent = true;
    let mut ledger = FeeLedger::new();

    let intents_per_slot = config.intents_per_slot as usize;

    for slot in 0..config.num_slots {
        let slot_start = (slot as usize) * intents_per_slot;
        let slot_end = std::cmp::min(slot_start + intents_per_slot, intents.len());
        if slot_start >= intents.len() { break; }

        let slot_intents = &intents[slot_start..slot_end];
        let block = slot * 10 + 100; // deterministic block heights

        let congestion = CongestionState::new(slot_intents.len() as u64, intents_per_slot as u64);
        let congestion_bps = PoSeqFeeCalculator::compute_congestion_multiplier(&config.params, &congestion);

        let mut slot_fees = 0u128;
        let mut slot_refunds = 0u128;
        let mut slot_admitted = 0u64;
        let mut slot_rejected = 0u64;
        let mut max_priority = 0u64;
        let mut min_priority = u64::MAX;

        for intent in slot_intents {
            let entry = class_stats.entry(intent.label.clone()).or_insert((0, 0));
            entry.1 += 1;

            let envelope = FeeEnvelope {
                payer: intent.payer,
                max_poseq_fee: intent.max_poseq_fee,
                max_runtime_fee: intent.max_runtime_fee,
                priority_tip: intent.tip,
                expiry: block + intent.deadline_offset,
            };

            let fee_result = PoSeqFeeCalculator::compute(
                &config.params, &envelope, intent.size_bytes, &congestion, block,
            );

            match fee_result {
                Ok(result) => {
                    slot_admitted += 1;
                    slot_fees += result.charged_fee;
                    slot_refunds += result.refund;

                    entry.0 += 1;

                    // Track full lifecycle
                    if let Ok(lc) = ledger.begin(intent.id) {
                        let _ = lc.reserve(
                            intent.payer,
                            intent.max_poseq_fee,
                            intent.max_runtime_fee,
                            block,
                        );
                        let _ = lc.charge_sequencing(result.charged_fee, block + 1);
                        let runtime_fee = (intent.runtime_gas as u128).saturating_mul(1);
                        let _ = lc.charge_runtime(runtime_fee, block + 2);
                        let _ = lc.issue_refund(block + 3);
                        let _ = lc.close(block + 4);

                        if !lc.verify_budget_consistency() {
                            budget_consistent = false;
                        }
                    }

                    // Distribution check
                    let dist = distribute_fee(&config.params, result.charged_fee);
                    if !dist.verify_budget_neutral() {
                        budget_consistent = false;
                    }
                    total_burned += dist.burn;

                    // Priority scoring
                    let priority_input = PriorityInput {
                        fairness_class_weight: intent.fairness_class_weight,
                        admitted_at_block: block,
                        deadline: block + intent.deadline_offset,
                        current_block: block,
                        effective_tip: result.effective_tip,
                        tip_cap: config.params.priority_tip_cap,
                        is_protected: intent.is_protected,
                    };
                    let score = compute_priority(&config.priority_config, &priority_input);

                    if score.composite > max_priority { max_priority = score.composite; }
                    if score.composite < min_priority { min_priority = score.composite; }

                    if intent.tip > 0 {
                        tip_priorities.push(score.composite);
                    } else {
                        no_tip_priorities.push(score.composite);
                    }
                }
                Err(_) => {
                    slot_rejected += 1;
                }
            }
        }

        total_fees += slot_fees;
        total_refunds += slot_refunds;
        total_admitted += slot_admitted;
        total_rejected += slot_rejected;

        slot_results.push(SlotResult {
            slot,
            intents_submitted: slot_intents.len() as u64,
            intents_admitted: slot_admitted,
            intents_rejected: slot_rejected,
            total_fees_charged: slot_fees,
            total_refunds: slot_refunds,
            avg_fee: if slot_admitted > 0 { slot_fees / slot_admitted as u128 } else { 0 },
            congestion_bps,
            max_priority_score: max_priority,
            min_priority_score: if min_priority == u64::MAX { 0 } else { min_priority },
        });
    }

    let avg_tip_priority = if tip_priorities.is_empty() { 0 } else {
        tip_priorities.iter().sum::<u64>() / tip_priorities.len() as u64
    };
    let avg_no_tip_priority = if no_tip_priorities.is_empty() { 0 } else {
        no_tip_priorities.iter().sum::<u64>() / no_tip_priorities.len() as u64
    };
    let tip_advantage = if avg_no_tip_priority > 0 {
        ((avg_tip_priority as u128).saturating_sub(avg_no_tip_priority as u128))
            .saturating_mul(10_000)
            / avg_no_tip_priority as u128
    } else {
        0
    };

    SimResults {
        total_slots: config.num_slots,
        total_intents_submitted: total_admitted + total_rejected,
        total_intents_admitted: total_admitted,
        total_intents_rejected: total_rejected,
        total_fees_collected: total_fees,
        total_refunds,
        total_burned,
        avg_fee_per_intent: if total_admitted > 0 { total_fees / total_admitted as u128 } else { 0 },
        avg_congestion_bps: if slot_results.is_empty() { 10_000 } else {
            slot_results.iter().map(|s| s.congestion_bps).sum::<u64>() / slot_results.len() as u64
        },
        fairness_class_inclusion: class_stats,
        tip_effectiveness: TipEffectiveness {
            intents_with_tip: tip_priorities.len() as u64,
            intents_without_tip: no_tip_priorities.len() as u64,
            avg_priority_with_tip: avg_tip_priority,
            avg_priority_without_tip: avg_no_tip_priority,
            tip_priority_advantage_bps: tip_advantage as u64,
        },
        budget_consistent,
        slot_results,
    }
}

/// Generate a deterministic batch of simulated intents.
pub fn generate_intents(count: usize, seed: u8) -> Vec<SimIntent> {
    let mut intents = Vec::with_capacity(count);
    for i in 0..count {
        let mut id = [0u8; 32];
        id[0] = seed;
        // Use full 4 bytes to avoid collisions for counts up to 2^32
        let i_bytes = (i as u32).to_be_bytes();
        id[1] = i_bytes[0];
        id[2] = i_bytes[1];
        id[3] = i_bytes[2];
        id[4] = i_bytes[3];
        let mut payer = [0u8; 32];
        payer[0] = (i as u8).wrapping_add(1);
        payer[1] = ((i >> 8) as u8).wrapping_add(1);

        // Deterministic variety
        let class = match i % 5 {
            0 => (9_000u64, true, "safety_critical"),
            1 => (8_000, true, "protected_user"),
            2 => (5_000, false, "bridge_adjacent"),
            3 => (3_000, false, "normal"),
            _ => (3_000, false, "normal_low_tip"),
        };

        let tip = match i % 5 {
            0 => 0,          // safety critical: no tip
            1 => 2_000,      // protected: modest tip
            2 => 10_000,     // bridge: medium tip
            3 => 50_000,     // normal: max tip (whale)
            _ => 0,          // normal: no tip
        };

        intents.push(SimIntent {
            id,
            payer,
            fairness_class_weight: class.0,
            max_poseq_fee: 1_000_000,
            max_runtime_fee: 5_000_000,
            tip: tip as u128,
            size_bytes: 200 + (i as u64 % 200),
            deadline_offset: 100 + (i as u64 % 50),
            runtime_gas: 2_000 + (i as u64 % 3_000),
            is_protected: class.1,
            label: class.2.to_string(),
        });
    }
    intents
}

// ─── Tests ─────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn sim_config() -> SimConfig {
        SimConfig {
            params: PoSeqFeeParameters::testnet_defaults(),
            priority_config: PriorityConfig::default(),
            num_slots: 10,
            intents_per_slot: 50,
        }
    }

    #[test]
    fn test_simulation_runs() {
        let config = sim_config();
        let intents = generate_intents(500, 1);
        let results = run_simulation(&config, &intents);

        assert_eq!(results.total_slots, 10);
        assert!(results.total_intents_admitted > 0);
        assert!(results.total_fees_collected > 0);
        assert!(results.budget_consistent);
    }

    #[test]
    fn test_simulation_deterministic() {
        let config = sim_config();
        let intents = generate_intents(200, 42);
        let r1 = run_simulation(&config, &intents);
        let r2 = run_simulation(&config, &intents);

        assert_eq!(r1.total_fees_collected, r2.total_fees_collected);
        assert_eq!(r1.total_intents_admitted, r2.total_intents_admitted);
        assert_eq!(r1.avg_fee_per_intent, r2.avg_fee_per_intent);
    }

    #[test]
    fn test_budget_consistency_under_load() {
        let config = SimConfig {
            num_slots: 20,
            intents_per_slot: 100,
            ..sim_config()
        };
        let intents = generate_intents(2000, 99);
        let results = run_simulation(&config, &intents);

        assert!(results.budget_consistent,
            "budget consistency violated under load");
    }

    #[test]
    fn test_fairness_class_inclusion_tracked() {
        let config = sim_config();
        let intents = generate_intents(500, 1);
        let results = run_simulation(&config, &intents);

        // All classes should have entries
        assert!(results.fairness_class_inclusion.contains_key("safety_critical"));
        assert!(results.fairness_class_inclusion.contains_key("normal"));

        // Safety critical should have 100% admission rate
        let (admitted, total) = results.fairness_class_inclusion.get("safety_critical").unwrap();
        assert_eq!(admitted, total, "safety_critical should always be admitted");
    }

    #[test]
    fn test_tip_bounded_advantage() {
        let config = sim_config();
        let intents = generate_intents(500, 1);
        let results = run_simulation(&config, &intents);

        // Tip advantage should exist but be bounded
        // Since tip weight is only 15%, advantage should be < 100% (10000 bps)
        assert!(results.tip_effectiveness.tip_priority_advantage_bps < 10_000,
            "tip advantage {} bps should be < 100%",
            results.tip_effectiveness.tip_priority_advantage_bps);
    }

    #[test]
    fn test_congestion_reflected_in_fees() {
        let low_config = SimConfig {
            intents_per_slot: 10, // well below target of 64
            num_slots: 5,
            ..sim_config()
        };
        let high_config = SimConfig {
            intents_per_slot: 200, // well above target
            num_slots: 5,
            ..sim_config()
        };

        let intents = generate_intents(1000, 1);
        let low_results = run_simulation(&low_config, &intents);
        let high_results = run_simulation(&high_config, &intents);

        assert!(high_results.avg_fee_per_intent >= low_results.avg_fee_per_intent,
            "high congestion fee {} should >= low congestion fee {}",
            high_results.avg_fee_per_intent, low_results.avg_fee_per_intent);
    }

    #[test]
    fn test_high_class_beats_whale_tip() {
        let config = sim_config();
        let priority_config = PriorityConfig::default();

        // Safety critical with no tip
        let sc_input = PriorityInput {
            fairness_class_weight: 9_000,
            admitted_at_block: 100,
            deadline: 200,
            current_block: 100,
            effective_tip: 0,
            tip_cap: 50_000,
            is_protected: true,
        };

        // Normal class with maximum tip
        let whale_input = PriorityInput {
            fairness_class_weight: 3_000,
            admitted_at_block: 100,
            deadline: 200,
            current_block: 100,
            effective_tip: 50_000,
            tip_cap: 50_000,
            is_protected: false,
        };

        let sc_score = compute_priority(&priority_config, &sc_input);
        let whale_score = compute_priority(&priority_config, &whale_input);

        assert!(sc_score.composite > whale_score.composite,
            "SafetyCritical (score={}) must beat whale-tip Normal (score={})",
            sc_score.composite, whale_score.composite);
    }

    #[test]
    fn test_starvation_eventually_dominates() {
        let config = PriorityConfig::default();

        // Normal class, no tip, waiting 200 blocks (100 beyond threshold)
        let starving = PriorityInput {
            fairness_class_weight: 3_000,
            admitted_at_block: 10,
            deadline: 500,
            current_block: 210,
            effective_tip: 0,
            tip_cap: 50_000,
            is_protected: false,
        };

        // Normal class, max tip, just admitted
        let whale = PriorityInput {
            fairness_class_weight: 3_000,
            admitted_at_block: 210,
            deadline: 500,
            current_block: 210,
            effective_tip: 50_000,
            tip_cap: 50_000,
            is_protected: false,
        };

        let starving_score = compute_priority(&config, &starving);
        let whale_score = compute_priority(&config, &whale);

        assert!(starving_score.composite > whale_score.composite,
            "starving intent (score={}) should beat fresh whale (score={})",
            starving_score.composite, whale_score.composite);
    }

    #[test]
    fn test_spam_cost_curve() {
        let params = PoSeqFeeParameters::testnet_defaults();

        // Cost of 1000 minimum-fee spam intents
        let spam_cost: u128 = (0..1000).map(|_| {
            let envelope = FeeEnvelope {
                payer: [1u8; 32],
                max_poseq_fee: 1_000_000,
                max_runtime_fee: 0,
                priority_tip: 0,
                expiry: 10_000,
            };
            let cong = CongestionState::new(100, 100); // moderate congestion
            PoSeqFeeCalculator::compute(&params, &envelope, 200, &cong, 100)
                .map(|r| r.charged_fee)
                .unwrap_or(0)
        }).sum();

        // At moderate congestion (100/64 = ~1.56x), base+bytes = 1000+2000=3000
        // multiplied by ~1.56x = ~4680 per intent, 1000 intents ≈ 4.68M OMNI units
        assert!(spam_cost > 3_000_000,
            "1000 spam intents should cost > 3M units, got {}", spam_cost);
    }
}
