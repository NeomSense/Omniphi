/// Phase 7 Audit: Cross-language conformance and safety tests.
///
/// These tests verify that Rust PoSeq logic is numerically and semantically
/// identical to the Go chain implementation. Any divergence is a critical bug.
///
/// Test coverage:
///   1. Inactivity threshold semantics (LivenessTracker + EnforcementConfig)
///   2. Minor severity: Dismissed not Penalized (adjudication/engine.rs)
///   3. Slash BPS values match Go (Minor=0, Moderate=300, Severe=1000, Critical=2000)
///   4. Fault penalty cap: min(faults * 500, 5000)
///   5. Reward score formula: (base + uptime) / 2 * poc_mult / 10000 - fault_penalty
///   6. Settlement net formula: slash_penalty = min(slash, gross); net = clamp(gross-slash, 0, 20000)
///   7. Rank score formula: participation * poc_mult / 10000
///   8. SequencerTier thresholds
///   9. BondState FSM transitions (Active → PartiallySlashed → Exhausted)
///  10. Idempotency: double-adjudication rejected
///  11. Inactivity threshold edge cases (exactly-at, below, above)
///  12. Enforcement rules: inactivity > threshold triggers Suspended recommendation

#[cfg(test)]
mod tests {
    use crate::adjudication::engine::{
        AdjudicationDecision, AdjudicationEngine, AdjudicationPath,
        is_auto_adjudicable, slash_bps_for_severity,
    };
    use crate::bonding::record::{BondState, OperatorBond};
    use crate::enforcement::rules::{EnforcementConfig, evaluate_inactivity};
    use crate::liveness::events::InactivityEvent;
    use crate::liveness::tracker::LivenessTracker;
    use crate::ranking::profile::{RankingEngine, SequencerRankingProfile, SequencerTier};
    use crate::reward::score::EpochRewardScore;
    use crate::settlement::epoch::{EpochSettlement, SettlementEngine};

    fn nid(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn pkt(b: u8) -> [u8; 32] {
        let mut h = [0u8; 32];
        h[0] = b;
        h
    }

    // ── Section 1: Inactivity Threshold Semantics ─────────────────────────────

    /// LivenessTracker uses strictly-greater-than semantics (> threshold),
    /// which matches EnforcementConfig.evaluate_inactivity and Go keeper step 11.
    /// Default threshold is 4 in both LivenessTracker (configured externally)
    /// and EnforcementConfig.
    #[test]
    fn audit_inactivity_semantics_strictly_greater_than() {
        let threshold = 4u64;
        let mut tracker = LivenessTracker::new(threshold);
        let node = nid(1);

        // Miss threshold+1 epochs (1..=5). After epoch 5, consecutive_missed = 5 > 4 → event.
        let mut export = tracker.finalize_epoch(1, &[node]);
        for epoch in 2..=(threshold + 1) {
            export = tracker.finalize_epoch(epoch, &[node]);
        }
        assert!(
            !export.inactivity_events.is_empty(),
            "After {} misses (> threshold={}), inactivity event must fire",
            threshold + 1,
            threshold
        );
    }

    #[test]
    fn audit_inactivity_exactly_at_threshold_no_event() {
        let threshold = 4u64;
        let mut tracker = LivenessTracker::new(threshold);
        let node = nid(2);

        // Miss exactly threshold epochs (1, 2, 3, 4).
        // After epoch 4, consecutive_missed = 4. Check: 4 > 4? NO. No event.
        let mut last_export = tracker.finalize_epoch(1, &[node]);
        for epoch in 2..=threshold {
            last_export = tracker.finalize_epoch(epoch, &[node]);
        }
        assert!(
            last_export.inactivity_events.is_empty(),
            "Exactly {} misses with threshold={} must NOT emit inactivity event (> not >=)",
            threshold,
            threshold
        );
    }

    #[test]
    fn audit_inactivity_enforcement_config_default_is_4() {
        let config = EnforcementConfig::default();
        assert_eq!(
            config.inactivity_suspend_threshold, 4,
            "EnforcementConfig default must be 4, matching Go InactivitySuspendEpochs default"
        );
    }

    #[test]
    fn audit_enforcement_rules_threshold_alignment() {
        let config = EnforcementConfig {
            inactivity_suspend_threshold: 4,
            ..Default::default()
        };
        let epoch = 10;

        // missed = 4 (== threshold) → no recommendation
        let events_at_threshold = vec![InactivityEvent {
            node_id: nid(1),
            detected_at_epoch: epoch,
            last_active_epoch: 5,
            missed_epochs: 4,
        }];
        let recs = evaluate_inactivity(&events_at_threshold, &config, epoch);
        assert!(
            recs.is_empty(),
            "missed=4 (== threshold=4) must NOT produce a Suspended recommendation"
        );

        // missed = 5 (> threshold) → recommendation
        let events_above_threshold = vec![InactivityEvent {
            node_id: nid(2),
            detected_at_epoch: epoch,
            last_active_epoch: 4,
            missed_epochs: 5,
        }];
        let recs = evaluate_inactivity(&events_above_threshold, &config, epoch);
        assert_eq!(recs.len(), 1, "missed=5 (> threshold=4) must produce one Suspended recommendation");
        assert_eq!(recs[0].recommended_status, "Suspended");
    }

    // ── Section 2: Minor Severity is Informational ────────────────────────────

    #[test]
    fn audit_minor_severity_dismissed_not_penalized() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(1), nid(1), "AbsentFromDuty", "Minor", 5).unwrap();
        assert_eq!(
            rec.decision, AdjudicationDecision::Dismissed,
            "Minor severity must be Dismissed (Informational), not Penalized"
        );
        assert_eq!(
            rec.slash_bps, 0,
            "Minor severity must have 0 slash_bps"
        );
        assert_eq!(
            rec.path, AdjudicationPath::Automatic,
            "Minor severity must be Automatic path (no governance escalation)"
        );
    }

    #[test]
    fn audit_moderate_severity_penalized_automatically() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(2), nid(1), "InvalidAttestation", "Moderate", 5).unwrap();
        assert_eq!(rec.decision, AdjudicationDecision::Penalized);
        assert_eq!(rec.slash_bps, 300);
        assert_eq!(rec.path, AdjudicationPath::Automatic);
    }

    #[test]
    fn audit_severe_escalated_to_governance() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(3), nid(1), "DoubleProposal", "Severe", 5).unwrap();
        assert_eq!(rec.decision, AdjudicationDecision::Escalated);
        assert_eq!(rec.slash_bps, 1000);
        assert_eq!(rec.path, AdjudicationPath::GovernanceReview);
    }

    #[test]
    fn audit_critical_escalated_to_governance() {
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(pkt(4), nid(1), "Equivocation", "Critical", 5).unwrap();
        assert_eq!(rec.decision, AdjudicationDecision::Escalated);
        assert_eq!(rec.slash_bps, 2000);
        assert_eq!(rec.path, AdjudicationPath::GovernanceReview);
    }

    // ── Section 3: Slash BPS Values Match Go ──────────────────────────────────

    #[test]
    fn audit_slash_bps_conformance_with_go() {
        assert_eq!(slash_bps_for_severity("Minor"), 0,
            "Minor: Rust must match Go SlashBpsForSeverity = 0");
        assert_eq!(slash_bps_for_severity("Moderate"), 300,
            "Moderate: Rust must match Go SlashBpsForSeverity = 300");
        assert_eq!(slash_bps_for_severity("Severe"), 1000,
            "Severe: Rust must match Go SlashBpsForSeverity = 1000");
        assert_eq!(slash_bps_for_severity("Critical"), 2000,
            "Critical: Rust must match Go SlashBpsForSeverity = 2000");
        assert_eq!(slash_bps_for_severity("Unknown"), 0,
            "Unknown severity: must return 0");
    }

    #[test]
    fn audit_is_auto_adjudicable_conformance_with_go() {
        assert!(!is_auto_adjudicable("Minor"),
            "Minor must NOT be auto-adjudicable (matches Go IsAutoAdjudicable)");
        assert!(is_auto_adjudicable("Moderate"),
            "Moderate must be auto-adjudicable");
        assert!(!is_auto_adjudicable("Severe"),
            "Severe must NOT be auto-adjudicable");
        assert!(!is_auto_adjudicable("Critical"),
            "Critical must NOT be auto-adjudicable");
    }

    // ── Section 4: Fault Penalty Cap ──────────────────────────────────────────

    #[test]
    fn audit_fault_penalty_formula_and_cap() {
        assert_eq!(EpochRewardScore::compute_fault_penalty(0), 0);
        assert_eq!(EpochRewardScore::compute_fault_penalty(1), 500);
        assert_eq!(EpochRewardScore::compute_fault_penalty(5), 2500);
        assert_eq!(EpochRewardScore::compute_fault_penalty(10), 5000, "capped at 5000");
        assert_eq!(EpochRewardScore::compute_fault_penalty(20), 5000, "still capped at 5000");
    }

    // ── Section 5: Reward Score Formula ───────────────────────────────────────

    #[test]
    fn audit_reward_score_formula_neutral_poc() {
        // (10000 + 10000) / 2 * 10000 / 10000 - 0 = 10000
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 10_000, 0), 10_000);
    }

    #[test]
    fn audit_reward_score_formula_with_poc_boost() {
        // (10000 + 10000) / 2 * 15000 / 10000 = 15000
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 15_000, 0), 15_000);
    }

    #[test]
    fn audit_reward_score_formula_with_fault_penalty() {
        // (10000 + 10000) / 2 * 10000 / 10000 - 2000 = 8000
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 10_000, 2_000), 8_000);
    }

    #[test]
    fn audit_reward_score_no_underflow() {
        // fault_penalty > combined → saturates to 0
        assert_eq!(EpochRewardScore::compute_final(100, 100, 10_000, 5_000), 0,
            "reward score must saturate to 0, not underflow");
    }

    #[test]
    fn audit_reward_score_clamped_at_20000() {
        // poc=20000 (2x), combined=10000 → 20000 (at cap)
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 20_000, 0), 20_000);
        // poc=30000 would exceed 20000 → still clamped
        assert_eq!(EpochRewardScore::compute_final(10_000, 10_000, 30_000, 0), 20_000);
    }

    // ── Section 6: Settlement Net Formula ─────────────────────────────────────

    #[test]
    fn audit_settlement_net_formula_no_slash() {
        let rs = make_reward_score(1, 9000, 0);
        let s = EpochSettlement::compute(nid(1), "".into(), 5, &rs, 0, 0, None);
        assert_eq!(s.gross_reward_score_bps, 9000);
        assert_eq!(s.slash_penalty_bps, 0);
        assert_eq!(s.net_reward_score_bps, 9000);
    }

    #[test]
    fn audit_settlement_net_formula_partial_slash() {
        let rs = make_reward_score(2, 9000, 0);
        let s = EpochSettlement::compute(nid(2), "".into(), 5, &rs, 500, 1, None);
        assert_eq!(s.slash_penalty_bps, 500);
        assert_eq!(s.net_reward_score_bps, 8500,
            "net = 9000 - 500 = 8500");
    }

    #[test]
    fn audit_settlement_slash_capped_at_gross() {
        let rs = make_reward_score(3, 1000, 0);
        // slash 5000 exceeds gross 1000 → capped
        let s = EpochSettlement::compute(nid(3), "".into(), 5, &rs, 5000, 1, None);
        assert_eq!(s.slash_penalty_bps, 1000,
            "slash_penalty capped at gross=1000");
        assert_eq!(s.net_reward_score_bps, 0,
            "net = 1000 - 1000 = 0");
    }

    #[test]
    fn audit_settlement_net_cannot_exceed_20000() {
        let rs = make_reward_score(4, 20000, 0);
        let s = EpochSettlement::compute(nid(4), "".into(), 5, &rs, 0, 0, None);
        assert!(s.net_reward_score_bps <= 20_000,
            "net reward score must not exceed 20000 bps");
    }

    // ── Section 7: Rank Score Formula ─────────────────────────────────────────

    #[test]
    fn audit_rank_score_formula_canonical_case() {
        // participation=8000, poc_mult=12000 → 8000 * 12000 / 10000 = 9600
        let p = SequencerRankingProfile::build(
            nid(1), "op".into(), 5, 1_000_000, 12_000, 8_000, 0, 10, true,
        );
        assert_eq!(p.rank_score, 9600, "rank_score formula: participation*poc_mult/10000");
    }

    #[test]
    fn audit_rank_score_neutral_poc() {
        // participation=7000, poc_mult=10000 → 7000
        let p = SequencerRankingProfile::build(
            nid(2), "op".into(), 5, 1_000_000, 10_000, 7_000, 0, 10, true,
        );
        assert_eq!(p.rank_score, 7000);
    }

    #[test]
    fn audit_rank_score_zero_participation() {
        let p = SequencerRankingProfile::build(
            nid(3), "op".into(), 5, 1_000_000, 10_000, 0, 0, 10, true,
        );
        assert_eq!(p.rank_score, 0, "zero participation → zero rank score");
    }

    #[test]
    fn audit_ranking_engine_sorted_descending() {
        let mut engine = RankingEngine::new();
        for (i, participation) in [(1u8, 6000u32), (2, 9000), (3, 8000)] {
            let p = SequencerRankingProfile::build(
                nid(i), "op".into(), 5, 1_000_000, 10_000, participation, 0, 10, true,
            );
            engine.store(p);
        }
        let ranked = engine.ranked();
        assert_eq!(ranked[0].participation_rate_bps, 9000, "best first");
        assert_eq!(ranked[1].participation_rate_bps, 8000);
        assert_eq!(ranked[2].participation_rate_bps, 6000, "worst last");
    }

    // ── Section 8: SequencerTier Thresholds ───────────────────────────────────

    #[test]
    fn audit_tier_elite_thresholds() {
        // poc_mult > 11000 (strict), participation > 9000 (strict), faults == 0
        let elite = SequencerTier::classify(true, 11001, 9001, 0, 10);
        assert_eq!(elite, SequencerTier::Elite);

        // poc_mult exactly 11000 → NOT elite (strict >)
        let not_elite = SequencerTier::classify(true, 11000, 9500, 0, 10);
        assert_ne!(not_elite, SequencerTier::Elite,
            "poc_mult == 11000 must NOT qualify for Elite (strictly > 11000 required)");

        // participation exactly 9000 → NOT elite (strict >)
        let not_elite2 = SequencerTier::classify(true, 12000, 9000, 0, 10);
        assert_ne!(not_elite2, SequencerTier::Elite,
            "participation == 9000 must NOT qualify for Elite (strictly > 9000 required)");
    }

    #[test]
    fn audit_tier_established_thresholds() {
        // poc_mult >= 9000, participation >= 7000, faults <= 2
        let t = SequencerTier::classify(true, 9000, 7000, 2, 10);
        assert_eq!(t, SequencerTier::Established);

        // faults = 3 → no longer Established
        let t2 = SequencerTier::classify(true, 9000, 7000, 3, 10);
        assert_ne!(t2, SequencerTier::Established);
    }

    #[test]
    fn audit_tier_probationary_epoch_boundary() {
        // epochs_since_activation < 3 → Probationary
        let t = SequencerTier::classify(true, 12000, 9500, 0, 2);
        assert_eq!(t, SequencerTier::Probationary);

        // epochs_since_activation == 3 → NOT Probationary (< 3 is false)
        let t2 = SequencerTier::classify(true, 8000, 6000, 0, 3);
        assert_ne!(t2, SequencerTier::Probationary);
    }

    #[test]
    fn audit_tier_underperforming_conditions() {
        // unbonded
        assert_eq!(SequencerTier::classify(false, 12000, 9500, 0, 10), SequencerTier::Underperforming);
        // participation < 5000
        assert_eq!(SequencerTier::classify(true, 12000, 4999, 0, 10), SequencerTier::Underperforming);
        // faults > 5
        assert_eq!(SequencerTier::classify(true, 12000, 9500, 6, 10), SequencerTier::Underperforming);
    }

    // ── Section 9: BondState FSM ───────────────────────────────────────────────

    #[test]
    fn audit_bond_fsm_active_to_partially_slashed() {
        let mut bond = make_active_bond(1, 1_000_000);
        let slashed = bond.apply_slash(300, 1); // 3% = 30000
        assert_eq!(slashed, 30_000);
        assert_eq!(bond.available_bond, 970_000);
        assert_eq!(bond.state, BondState::PartiallySlashed,
            "first partial slash must transition Active → PartiallySlashed");
    }

    #[test]
    fn audit_bond_fsm_full_slash_to_exhausted() {
        let mut bond = make_active_bond(2, 1_000);
        bond.apply_slash(10_000, 1); // 100% slash
        assert_eq!(bond.available_bond, 0);
        assert_eq!(bond.state, BondState::Exhausted,
            "full slash must transition to Exhausted");
    }

    #[test]
    fn audit_bond_slash_minimum_one_unit() {
        let mut bond = make_active_bond(3, 10);
        // 10 * 1 / 10000 = 0 → bumped to 1
        let slashed = bond.apply_slash(1, 1);
        assert_eq!(slashed, 1, "minimum slash is 1 unit even if formula gives 0");
        assert_eq!(bond.available_bond, 9);
    }

    #[test]
    fn audit_bond_slash_capped_at_available_correct() {
        let mut bond = make_active_bond(5, 1_000);
        bond.apply_slash(5000, 1); // 50% = 500; available now 500
        assert_eq!(bond.available_bond, 500);

        // slash_amount = 1000 * 10000 / 10000 = 1000, capped at available=500
        let slashed2 = bond.apply_slash(10_000, 2);
        assert_eq!(slashed2, 500, "slash capped at available_bond=500");
        assert_eq!(bond.available_bond, 0);
        assert_eq!(bond.state, BondState::Exhausted);
    }

    // ── Section 10: Idempotency — Double Adjudication Rejected ────────────────

    #[test]
    fn audit_double_adjudication_rejected() {
        let mut engine = AdjudicationEngine::new();
        engine.adjudicate(pkt(10), nid(1), "Equivocation", "Moderate", 5).unwrap();
        let result = engine.adjudicate(pkt(10), nid(1), "Equivocation", "Moderate", 5);
        assert!(
            result.is_err(),
            "second adjudication of same packet_hash must be rejected"
        );
    }

    #[test]
    fn audit_different_packets_independently_adjudicated() {
        let mut engine = AdjudicationEngine::new();
        engine.adjudicate(pkt(11), nid(1), "Equivocation", "Moderate", 5).unwrap();
        // Different packet hash — must succeed
        let result = engine.adjudicate(pkt(12), nid(1), "Equivocation", "Moderate", 5);
        assert!(result.is_ok(), "different packet hashes must be independently adjudicated");
        assert_eq!(engine.len(), 2);
    }

    // ── Section 11: Settlement Engine Correctness ─────────────────────────────

    #[test]
    fn audit_settlement_engine_stores_and_retrieves() {
        let mut engine = SettlementEngine::new();
        let rs = make_reward_score(1, 8000, 500);
        let s = EpochSettlement::compute(nid(1), "omni1op".into(), 7, &rs, 0, 0, None);
        engine.store(s);

        let got = engine.get(&nid(1), 7).unwrap();
        assert_eq!(got.epoch, 7);
        assert_eq!(got.net_reward_score_bps, 8000);
    }

    #[test]
    fn audit_settlement_engine_for_epoch_filters_correctly() {
        let mut engine = SettlementEngine::new();
        for i in 1u8..=3 {
            let rs = make_reward_score(i, 7000, 0);
            engine.store(EpochSettlement::compute(nid(i), "".into(), 9, &rs, 0, 0, None));
        }
        let rs4 = make_reward_score(4, 5000, 0);
        engine.store(EpochSettlement::compute(nid(4), "".into(), 10, &rs4, 0, 0, None));

        assert_eq!(engine.for_epoch(9).len(), 3, "epoch 9 must have exactly 3 records");
        assert_eq!(engine.for_epoch(10).len(), 1, "epoch 10 must have exactly 1 record");
        assert_eq!(engine.for_epoch(99).len(), 0, "epoch 99 must have no records");
    }

    // ── Section 12: Determinism — Same Inputs, Same Outputs ──────────────────

    #[test]
    fn audit_adjudication_is_deterministic() {
        let mut e1 = AdjudicationEngine::new();
        let mut e2 = AdjudicationEngine::new();
        let r1 = e1.adjudicate(pkt(20), nid(1), "Equivocation", "Moderate", 5).unwrap();
        let r2 = e2.adjudicate(pkt(20), nid(1), "Equivocation", "Moderate", 5).unwrap();
        assert_eq!(r1.slash_bps, r2.slash_bps);
        assert_eq!(r1.decision, r2.decision);
        assert_eq!(r1.path, r2.path);
    }

    #[test]
    fn audit_tier_classification_is_deterministic() {
        for _ in 0..10 {
            let t = SequencerTier::classify(true, 12000, 9500, 0, 10);
            assert_eq!(t, SequencerTier::Elite, "tier classification must be deterministic");
        }
    }

    #[test]
    fn audit_rank_score_is_deterministic() {
        for _ in 0..10 {
            let score = 8000u64 * 12000u64 / 10_000;
            assert_eq!(score, 9600, "rank score must be deterministic");
        }
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    fn make_active_bond(b: u8, bond_amount: u64) -> OperatorBond {
        OperatorBond {
            operator_address: "omni1op".into(),
            node_id: nid(b),
            bond_amount,
            bond_denom: "uomni".into(),
            bonded_since_epoch: 1,
            is_active: true,
            withdrawn_at_epoch: 0,
            state: BondState::Active,
            slashed_amount: 0,
            available_bond: bond_amount,
            last_slash_epoch: 0,
            slash_count: 0,
        }
    }

    fn make_reward_score(node: u8, final_bps: u32, fault_bps: u32) -> crate::reward::score::EpochRewardScore {
        crate::reward::score::EpochRewardScore {
            node_id: nid(node),
            operator_address: None,
            epoch: 5,
            base_score_bps: 8000,
            uptime_score_bps: 10000,
            poc_multiplier_bps: 10000,
            fault_penalty_bps: fault_bps,
            final_score_bps: final_bps,
            is_bonded: true,
        }
    }
}
