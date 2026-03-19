/// Dual-lane E2E simulation tests.
///
/// Each scenario exercises a realistic end-to-end path through PoSeq's
/// core subsystems: liveness tracking, adjudication, bonding/slashing,
/// settlement, committee formation, and snapshot verification.
///
/// These are integration tests (not unit tests) — they compose multiple
/// subsystems to verify that the combined behavior is correct.
use omniphi_poseq::adjudication::engine::{AdjudicationDecision, AdjudicationError, AdjudicationEngine, AdjudicationPath};
use omniphi_poseq::bonding::record::{BondState, OperatorBond};
use omniphi_poseq::chain_bridge::snapshot::{
    ChainCommitteeMember, ChainCommitteeSnapshot, SnapshotImportError, SnapshotImporter,
};
use omniphi_poseq::committee::membership::{CommitteeFormationError, PoSeqCommittee};
use omniphi_poseq::identities::registry::{ChainSequencerStatus, SequencerRecord, SequencerRegistry};
use omniphi_poseq::liveness::tracker::LivenessTracker;
use omniphi_poseq::membership::MembershipStore;
use omniphi_poseq::reward::score::EpochRewardScore;
use omniphi_poseq::settlement::epoch::EpochSettlement;

// ─── Helpers ─────────────────────────────────────────────────────────────────

fn node_id(b: u8) -> [u8; 32] {
    let mut arr = [0u8; 32];
    arr[0] = b;
    arr
}

fn pkt_hash(b: u8) -> [u8; 32] {
    let mut arr = [0u8; 32];
    arr[0] = b;
    arr[1] = 0xEE; // distinguish from node ids
    arr
}

fn make_bond(n: u8, amount: u64) -> OperatorBond {
    OperatorBond::new(
        format!("omni1operator{n}"),
        node_id(n),
        amount,
        "uomni".to_string(),
        1,
    )
}

fn make_reward_score(nid: [u8; 32], gross: u32, fault_bps: u32) -> EpochRewardScore {
    EpochRewardScore {
        node_id: nid,
        operator_address: Some("omni1test".to_string()),
        epoch: 10,
        base_score_bps: gross,
        uptime_score_bps: 10_000,
        poc_multiplier_bps: 10_000,
        fault_penalty_bps: fault_bps,
        final_score_bps: gross,
        is_bonded: true,
    }
}

/// Build a valid `ChainCommitteeSnapshot` for the given epoch and node IDs.
fn make_snapshot(epoch: u64, members: Vec<(u8, &str)>) -> ChainCommitteeSnapshot {
    let mut node_ids: Vec<[u8; 32]> = members.iter().map(|(b, _)| node_id(*b)).collect();
    node_ids.sort();

    let hash = ChainCommitteeSnapshot::compute_hash(epoch, &node_ids);

    ChainCommitteeSnapshot {
        epoch,
        members: members
            .iter()
            .map(|(b, role)| ChainCommitteeMember {
                node_id: hex::encode(node_id(*b)),
                public_key: hex::encode([*b; 32]),
                moniker: format!("node-{b}"),
                role: role.to_string(),
            })
            .collect(),
        snapshot_hash: hash.to_vec(),
        produced_at_block: 100,
    }
}

/// Register a sequencer as Active in the given registry.
fn register_active(registry: &mut SequencerRegistry, n: u8) {
    registry.apply_registration(SequencerRecord {
        node_id: node_id(n),
        public_key: [n; 32],
        moniker: format!("node-{n}"),
        operator_address: format!("omni1operator{n}"),
        cosmos_validator_address: None,
        registered_epoch: 1,
        is_active: true,
        chain_status: ChainSequencerStatus::Active,
    });
}

// ─── Scenario 1: Normal Participation ────────────────────────────────────────

/// Ten slots of full participation → no inactivity events, participation = 10/10.
#[test]
fn test_dual_lane_normal_participation() {
    let nid = node_id(1);
    let mut tracker = LivenessTracker::new(4);

    // Simulate 10 slots of activity in epoch 1
    for slot in 1..=10 {
        tracker.record_active(nid, slot, slot == 1, true); // proposer once
    }

    let export = tracker.finalize_epoch(1, &[nid]);

    assert_eq!(export.epoch, 1);
    assert_eq!(export.active_events.len(), 1, "one active event for the node");
    assert!(
        export.inactivity_events.is_empty(),
        "no inactivity events for active node"
    );

    let event = &export.active_events[0];
    assert_eq!(event.node_id, nid);
    assert!(event.was_proposer || event.was_attestor);
}

// ─── Scenario 2: Inactivity Suspension ───────────────────────────────────────

/// Five consecutive epochs of zero activity → inactivity event emitted on epoch 5.
/// Threshold = 4; consecutive_missed > 4 triggers on epoch 5 (missed = 5).
#[test]
fn test_dual_lane_inactivity_suspension() {
    let nid = node_id(2);
    let mut tracker = LivenessTracker::new(4);
    let registered = [nid];

    // 5 epochs of complete absence
    for epoch in 1..=5u64 {
        let export = tracker.finalize_epoch(epoch, &registered);
        if epoch < 5 {
            // missed = 1..4 — threshold not exceeded yet
            assert!(
                export.inactivity_events.is_empty(),
                "epoch {epoch}: no inactivity event yet (missed={epoch})"
            );
        } else {
            // missed = 5 > threshold = 4 → event emitted
            assert_eq!(
                export.inactivity_events.len(),
                1,
                "epoch {epoch}: inactivity event expected (missed={epoch})"
            );
            let ev = &export.inactivity_events[0];
            assert_eq!(ev.node_id, nid);
            assert_eq!(ev.detected_at_epoch, 5);
            assert_eq!(ev.missed_epochs, 5);
            assert_eq!(ev.last_active_epoch, 0); // never seen
        }
    }
}

// ─── Scenario 3: Moderate Slash ──────────────────────────────────────────────

/// Moderate evidence → adjudicated as Penalized (300 bps) → bond partially slashed
/// → settlement net reduced accordingly.
#[test]
fn test_dual_lane_moderate_slash() {
    let nid = node_id(3);
    let epoch = 5u64;

    // Adjudicate the evidence
    let mut engine = AdjudicationEngine::new();
    let rec = engine
        .adjudicate(pkt_hash(3), nid, "UnfairSequencing", "Moderate", epoch)
        .expect("adjudication should succeed");

    assert_eq!(rec.path, AdjudicationPath::Automatic);
    assert_eq!(rec.decision, AdjudicationDecision::Penalized);
    assert_eq!(rec.slash_bps, 300);
    assert!(rec.auto_applied);

    // Apply the slash to the bond
    let mut bond = make_bond(3, 1_000_000);
    let slashed = bond.apply_slash(rec.slash_bps, epoch);

    assert_eq!(slashed, 30_000, "3% of 1_000_000");
    assert_eq!(bond.available_bond, 970_000);
    assert_eq!(bond.state, BondState::PartiallySlashed);

    // Compute settlement with the slash applied
    let reward = make_reward_score(nid, 8_000, 0);
    let settlement = EpochSettlement::compute(
        nid,
        "omni1operator3".to_string(),
        epoch,
        &reward,
        rec.slash_bps,
        1,
        Some("omni1operator3".to_string()),
    );

    // gross=8000, slash_penalty=min(300, 8000)=300 → net = 8000 - 300 = 7700
    assert_eq!(settlement.gross_reward_score_bps, 8_000);
    assert_eq!(settlement.slash_penalty_bps, 300);
    assert_eq!(settlement.net_reward_score_bps, 7_700);
    assert_eq!(settlement.slash_count, 1);
    assert!(settlement.is_bonded);
}

// ─── Scenario 4: Bond Exhaustion ─────────────────────────────────────────────

/// Successive Critical slashes against a small bond → exhaustion.
/// After exhaustion, `apply_slash` returns 0.
#[test]
fn test_dual_lane_bond_exhaustion() {
    let nid = node_id(4);
    let mut bond = make_bond(4, 1_000); // small bond: 1000 uomni

    // 5 Critical slashes (2000 bps = 20% of bond_amount = 200 per slash)
    // 1: available=1000 → slash=200 → available=800
    // 2: available=800 → slash=200 → available=600
    // 3: available=600 → slash=200 → available=400
    // 4: available=400 → slash=200 → available=200
    // 5: available=200 → slash=200 → available=0 → Exhausted
    for i in 1..=5u64 {
        let slashed = bond.apply_slash(2000, i);
        assert!(slashed > 0, "slash {i} should remove some bond");
    }

    assert_eq!(bond.available_bond, 0);
    assert_eq!(bond.state, BondState::Exhausted);
    assert_eq!(bond.slash_count, 5);

    // Further slash returns 0 — bond already exhausted
    let further = bond.apply_slash(2000, 6);
    assert_eq!(further, 0, "exhausted bond yields 0 on further slash");
    // slash_count does NOT increment on an already-exhausted bond (apply_slash returns early)
    assert_eq!(bond.slash_count, 5, "slash_count unchanged after exhaustion");
}

// ─── Scenario 5: Jailed Operator Excluded from Committee ─────────────────────

/// Severe evidence → Escalated → registry status set to Jailed → excluded from committee.
#[test]
fn test_dual_lane_jailed_operator_excluded() {
    let nid1 = node_id(5);
    let nid2 = node_id(6);
    let epoch = 7u64;

    // Adjudicate severe evidence for node 5 → Escalated (governance review)
    let mut engine = AdjudicationEngine::new();
    let rec = engine
        .adjudicate(pkt_hash(5), nid1, "Equivocation", "Severe", epoch)
        .expect("adjudication should succeed");
    assert_eq!(rec.decision, AdjudicationDecision::Escalated);
    assert_eq!(rec.path, AdjudicationPath::GovernanceReview);

    // Simulate governance applying the Jailed status
    let mut registry = SequencerRegistry::new();
    register_active(&mut registry, 5);
    register_active(&mut registry, 6);

    registry.apply_status_update(&nid1, ChainSequencerStatus::Jailed);

    // Build a committee snapshot with both nodes
    let snap = make_snapshot(epoch, vec![(5, "Sequencer"), (6, "Sequencer")]);

    // Build in-memory MembershipStore (all active)
    let mut membership = MembershipStore::new();
    membership.register(nid1, epoch).unwrap();
    membership.register(nid2, epoch).unwrap();

    let committee = PoSeqCommittee::from_chain_snapshot(&snap, &registry, &membership, epoch, 1)
        .expect("committee formation should succeed with 1 eligible member");

    // Node 5 (jailed) must be excluded
    assert!(
        !committee.is_member(&nid1),
        "jailed node must not be in committee"
    );
    // Node 6 (active) must be included
    assert!(
        committee.is_member(&nid2),
        "active node must be in committee"
    );
}

// ─── Scenario 6: Double Slash Prevented ──────────────────────────────────────

/// The same evidence packet adjudicated twice returns AlreadyFinalized.
#[test]
fn test_dual_lane_double_slash_prevented() {
    let nid = node_id(7);
    let hash = pkt_hash(7);

    let mut engine = AdjudicationEngine::new();

    // First adjudication succeeds
    engine
        .adjudicate(hash, nid, "Equivocation", "Minor", 3)
        .expect("first adjudication should succeed");

    // Second adjudication of the same packet_hash must be rejected
    let result = engine.adjudicate(hash, nid, "Equivocation", "Minor", 3);
    match result {
        Err(AdjudicationError::AlreadyFinalized { packet_hash }) => {
            assert_eq!(packet_hash, hash, "error must carry the offending hash");
        }
        other => panic!("expected AlreadyFinalized, got {:?}", other),
    }

    // Engine still has exactly one record for that hash
    assert_eq!(engine.len(), 1);
}

// ─── Scenario 7: Committee Snapshot Consistency ──────────────────────────────

/// A valid snapshot passes `verify_hash` and is accepted by `SnapshotImporter`.
/// A tampered snapshot (one byte of a node_id flipped) is rejected.
#[test]
fn test_dual_lane_committee_snapshot_consistency() {
    let epoch = 10u64;
    let valid = make_snapshot(epoch, vec![(1, "Sequencer"), (2, "Sequencer"), (3, "Sequencer")]);

    // Valid snapshot passes hash verification
    assert!(
        valid.verify_hash(),
        "valid snapshot must pass verify_hash"
    );

    // SnapshotImporter accepts the valid snapshot
    let mut importer = SnapshotImporter::new();
    importer.import(valid.clone()).expect("valid snapshot should be imported");
    assert!(importer.has_epoch(epoch));
    assert_eq!(importer.get(epoch).unwrap().members.len(), 3);

    // Tamper: flip one byte of the first member's node_id hex string
    let mut tampered = valid.clone();
    {
        let first = &mut tampered.members[0];
        // The original hex starts with "05..." (node_id(1) = [1, 0, 0, ...])
        // Change first char from '0' to 'f' to break the hash
        let original = first.node_id.clone();
        first.node_id = format!("ff{}", &original[2..]);
    }

    // Tampered snapshot fails verify_hash
    assert!(
        !tampered.verify_hash(),
        "tampered snapshot must fail verify_hash"
    );

    // SnapshotImporter rejects the tampered snapshot
    let mut importer2 = SnapshotImporter::new();
    let result = importer2.import(tampered.clone());
    assert!(
        matches!(result, Err(SnapshotImportError::HashMismatch { .. })),
        "tampered snapshot must produce HashMismatch error, got: {:?}",
        result
    );

    // Duplicate import of valid snapshot is also rejected
    let mut importer3 = SnapshotImporter::new();
    importer3.import(valid.clone()).unwrap();
    let dup = importer3.import(valid);
    assert!(
        matches!(dup, Err(SnapshotImportError::DuplicateEpoch(_))),
        "duplicate epoch must produce DuplicateEpoch error"
    );
}
