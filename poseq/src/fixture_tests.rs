/// Cross-language parity tests driven by shared JSON fixtures.
///
/// Each test loads a fixture from `tests/fixtures/` (relative to the workspace root),
/// runs the actual Rust logic, and asserts that every `expected_*` field matches exactly.
/// The same fixtures are consumed by Go tests in `chain/x/poseq/keeper/keeper_test.go`.
#[cfg(test)]
mod tests {
    use crate::adjudication::engine::{AdjudicationEngine, AdjudicationDecision, AdjudicationPath, slash_bps_for_severity, is_auto_adjudicable};
    use crate::bonding::record::{BondState, OperatorBond};
    use crate::ranking::profile::{SequencerTier, SequencerRankingProfile};
    use crate::settlement::epoch::EpochSettlement;
    use crate::reward::score::EpochRewardScore;

    use serde::Deserialize;
    use hex;

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    fn hex_to_32(s: &str) -> [u8; 32] {
        let bytes = hex::decode(s).expect("invalid hex in fixture");
        let mut out = [0u8; 32];
        out.copy_from_slice(&bytes[..32]);
        out
    }

    // -------------------------------------------------------------------------
    // Adjudication fixtures
    // -------------------------------------------------------------------------

    #[derive(Deserialize)]
    struct AdjudicationFixture {
        packet_hash: String,
        node_id: String,
        misbehavior_type: String,
        severity: String,
        epoch: u64,
        expected_path: String,
        expected_decision: String,
        expected_slash_bps: u32,
    }

    fn load_adjudication(name: &str) -> AdjudicationFixture {
        let json = match name {
            "minor" => include_str!("../../tests/fixtures/adjudication/minor.json"),
            "moderate" => include_str!("../../tests/fixtures/adjudication/moderate.json"),
            "severe" => include_str!("../../tests/fixtures/adjudication/severe.json"),
            "critical" => include_str!("../../tests/fixtures/adjudication/critical.json"),
            _ => panic!("unknown adjudication fixture: {}", name),
        };
        serde_json::from_str(json).expect("failed to parse adjudication fixture")
    }

    fn run_adjudication_fixture(name: &str) {
        let f = load_adjudication(name);
        let packet_hash = hex_to_32(&f.packet_hash);
        let node_id = hex_to_32(&f.node_id);

        // Verify slash_bps function matches fixture
        let bps = slash_bps_for_severity(&f.severity);
        assert_eq!(bps, f.expected_slash_bps,
            "[{}] slash_bps_for_severity mismatch: got {}, want {}", name, bps, f.expected_slash_bps);

        // Verify adjudication engine produces expected path + decision
        let mut engine = AdjudicationEngine::new();
        let rec = engine.adjudicate(
            packet_hash,
            node_id,
            &f.misbehavior_type,
            &f.severity,
            f.epoch,
        ).expect("adjudication failed");

        let expected_path = match f.expected_path.as_str() {
            "Automatic" => AdjudicationPath::Automatic,
            "GovernanceReview" => AdjudicationPath::GovernanceReview,
            s => panic!("unknown path: {}", s),
        };
        let expected_decision = match f.expected_decision.as_str() {
            "Dismissed" => AdjudicationDecision::Dismissed,
            "Penalized" => AdjudicationDecision::Penalized,
            "Escalated" => AdjudicationDecision::Escalated,
            s => panic!("unknown decision: {}", s),
        };

        assert_eq!(rec.path, expected_path,
            "[{}] path mismatch: got {:?}, want {:?}", name, rec.path, expected_path);
        assert_eq!(rec.decision, expected_decision,
            "[{}] decision mismatch: got {:?}, want {:?}", name, rec.decision, expected_decision);
        assert_eq!(rec.slash_bps, f.expected_slash_bps,
            "[{}] slash_bps mismatch: got {}, want {}", name, rec.slash_bps, f.expected_slash_bps);
    }

    #[test]
    fn fixture_adjudication_minor() {
        run_adjudication_fixture("minor");
    }

    #[test]
    fn fixture_adjudication_moderate() {
        run_adjudication_fixture("moderate");
    }

    #[test]
    fn fixture_adjudication_severe() {
        run_adjudication_fixture("severe");
    }

    #[test]
    fn fixture_adjudication_critical() {
        run_adjudication_fixture("critical");
    }

    // -------------------------------------------------------------------------
    // Slashing fixtures
    // -------------------------------------------------------------------------

    #[derive(Deserialize)]
    struct SlashFixture {
        bond_amount: u64,
        available_bond: u64,
        slash_bps: u32,
        epoch: u64,
        expected_slash_amount: u64,
        expected_available_after: u64,
        expected_slashed_total: u64,
        expected_state_after: String,
    }

    fn load_slash(name: &str) -> SlashFixture {
        let json = match name {
            "standard" => include_str!("../../tests/fixtures/slashing/standard.json"),
            "exhaustion" => include_str!("../../tests/fixtures/slashing/exhaustion.json"),
            "minimum" => include_str!("../../tests/fixtures/slashing/minimum.json"),
            _ => panic!("unknown slashing fixture: {}", name),
        };
        serde_json::from_str(json).expect("failed to parse slashing fixture")
    }

    fn run_slash_fixture(name: &str) {
        let f = load_slash(name);
        let mut bond = OperatorBond {
            operator_address: "omni1test".into(),
            node_id: [0u8; 32],
            bond_amount: f.bond_amount,
            bond_denom: "uomni".into(),
            bonded_since_epoch: 1,
            is_active: true,
            withdrawn_at_epoch: 0,
            state: BondState::Active,
            slashed_amount: f.bond_amount - f.available_bond, // pre-existing slashes
            available_bond: f.available_bond,
            last_slash_epoch: 0,
            slash_count: 0,
        };

        let slashed = bond.apply_slash(f.slash_bps, f.epoch);

        assert_eq!(slashed, f.expected_slash_amount,
            "[{}] slash_amount mismatch: got {}, want {}", name, slashed, f.expected_slash_amount);
        assert_eq!(bond.available_bond, f.expected_available_after,
            "[{}] available_bond mismatch: got {}, want {}", name, bond.available_bond, f.expected_available_after);
        assert_eq!(bond.slashed_amount, f.expected_slashed_total + (f.bond_amount - f.available_bond),
            "[{}] slashed_total mismatch", name);

        let expected_state = match f.expected_state_after.as_str() {
            "Active" => BondState::Active,
            "PartiallySlashed" => BondState::PartiallySlashed,
            "Jailed" => BondState::Jailed,
            "Exhausted" => BondState::Exhausted,
            "Retired" => BondState::Retired,
            s => panic!("unknown bond state: {}", s),
        };
        assert_eq!(bond.state, expected_state,
            "[{}] bond_state mismatch: got {:?}, want {:?}", name, bond.state, expected_state);
    }

    #[test]
    fn fixture_slash_standard() {
        run_slash_fixture("standard");
    }

    #[test]
    fn fixture_slash_exhaustion() {
        run_slash_fixture("exhaustion");
    }

    #[test]
    fn fixture_slash_minimum() {
        run_slash_fixture("minimum");
    }

    // -------------------------------------------------------------------------
    // Settlement fixtures
    // -------------------------------------------------------------------------

    #[derive(Deserialize)]
    struct SettlementFixture {
        node_id: String,
        operator_address: String,
        epoch: u64,
        gross_reward_score_bps: u32,
        poc_multiplier_bps: u32,
        fault_penalty_bps: u32,
        slash_penalty_bps: u32,
        is_bonded: bool,
        slash_count: u32,
        expected_net: u32,
    }

    fn load_settlement(name: &str) -> SettlementFixture {
        let json = match name {
            "no_slash" => include_str!("../../tests/fixtures/settlement/no_slash.json"),
            "with_slash" => include_str!("../../tests/fixtures/settlement/with_slash.json"),
            "slash_exceeds_gross" => include_str!("../../tests/fixtures/settlement/slash_exceeds_gross.json"),
            _ => panic!("unknown settlement fixture: {}", name),
        };
        serde_json::from_str(json).expect("failed to parse settlement fixture")
    }

    fn run_settlement_fixture(name: &str) {
        let f = load_settlement(name);
        let node_id = hex_to_32(&f.node_id);

        // Build a reward score that yields the gross value
        let reward_score = EpochRewardScore {
            node_id,
            operator_address: Some(f.operator_address.clone()),
            epoch: f.epoch,
            base_score_bps: f.gross_reward_score_bps,
            uptime_score_bps: 10_000,
            poc_multiplier_bps: f.poc_multiplier_bps,
            fault_penalty_bps: f.fault_penalty_bps,
            final_score_bps: f.gross_reward_score_bps, // gross = final before slash
            is_bonded: f.is_bonded,
        };

        let s = EpochSettlement::compute(
            node_id,
            f.operator_address.clone(),
            f.epoch,
            &reward_score,
            f.slash_penalty_bps,
            f.slash_count,
            Some(f.operator_address.clone()),
        );

        assert_eq!(s.gross_reward_score_bps, f.gross_reward_score_bps,
            "[{}] gross mismatch: got {}, want {}", name, s.gross_reward_score_bps, f.gross_reward_score_bps);
        assert_eq!(s.net_reward_score_bps, f.expected_net,
            "[{}] net mismatch: got {}, want {}", name, s.net_reward_score_bps, f.expected_net);
        assert_eq!(s.slash_count, f.slash_count,
            "[{}] slash_count mismatch", name);
        assert_eq!(s.is_bonded, f.is_bonded,
            "[{}] is_bonded mismatch", name);
    }

    #[test]
    fn fixture_settlement_no_slash() {
        run_settlement_fixture("no_slash");
    }

    #[test]
    fn fixture_settlement_with_slash() {
        run_settlement_fixture("with_slash");
    }

    #[test]
    fn fixture_settlement_slash_exceeds_gross() {
        run_settlement_fixture("slash_exceeds_gross");
    }

    // -------------------------------------------------------------------------
    // Ranking fixtures
    // -------------------------------------------------------------------------

    #[derive(Deserialize)]
    struct RankingFixture {
        node_id: String,
        poc_multiplier_bps: u32,
        participation_rate_bps: u32,
        fault_events_recent: u64,
        epochs_since_activation: u64,
        is_bonded: bool,
        expected_tier: String,
        expected_rank_score: u32,
    }

    fn load_ranking(name: &str) -> RankingFixture {
        let json = match name {
            "elite" => include_str!("../../tests/fixtures/ranking/elite.json"),
            "probationary" => include_str!("../../tests/fixtures/ranking/probationary.json"),
            "underperforming" => include_str!("../../tests/fixtures/ranking/underperforming.json"),
            _ => panic!("unknown ranking fixture: {}", name),
        };
        serde_json::from_str(json).expect("failed to parse ranking fixture")
    }

    fn run_ranking_fixture(name: &str) {
        let f = load_ranking(name);
        let node_id = hex_to_32(&f.node_id);

        let profile = SequencerRankingProfile::build(
            node_id,
            "omni1test".into(),
            1,
            1_000_000,
            f.poc_multiplier_bps,
            f.participation_rate_bps,
            f.fault_events_recent,
            f.epochs_since_activation,
            f.is_bonded,
        );

        let expected_tier = match f.expected_tier.as_str() {
            "Elite" => SequencerTier::Elite,
            "Established" => SequencerTier::Established,
            "Standard" => SequencerTier::Standard,
            "Probationary" => SequencerTier::Probationary,
            "Underperforming" => SequencerTier::Underperforming,
            s => panic!("unknown tier: {}", s),
        };

        assert_eq!(profile.tier, expected_tier,
            "[{}] tier mismatch: got {:?}, want {:?}", name, profile.tier, expected_tier);
        assert_eq!(profile.rank_score, f.expected_rank_score,
            "[{}] rank_score mismatch: got {}, want {}", name, profile.rank_score, f.expected_rank_score);
    }

    #[test]
    fn fixture_ranking_elite() {
        run_ranking_fixture("elite");
    }

    #[test]
    fn fixture_ranking_probationary() {
        run_ranking_fixture("probationary");
    }

    #[test]
    fn fixture_ranking_underperforming() {
        run_ranking_fixture("underperforming");
    }
}
