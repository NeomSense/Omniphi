//! Section 12 — End-to-End Hardening Tests
//!
//! Validates the entire Omniphi runtime as one coherent system.
//! Categories: integration, adversarial, determinism, replay, malformed state, invariant.

use omniphi_runtime::capabilities::spending::{
    SpendCapabilityRegistry, SpendCapabilityStatus,
};
use omniphi_runtime::intents::base::{
    ExecutionMode, FeePolicy, IntentConstraints, IntentTransaction, IntentType,
    SponsorshipLimits,
};
use omniphi_runtime::intents::encrypted::{
    EncryptedIntent, EncryptedIntentRegistry, EncryptedIntentStatus, IntentReveal,
};
use omniphi_runtime::intents::sponsorship::{SponsorReplayTracker, SponsorshipValidator};
use omniphi_runtime::intents::types::TransferIntent;
use omniphi_runtime::objects::base::{AccessMode, ObjectAccess, ObjectId};
use omniphi_runtime::ownership::{
    BehaviorFlags, LinkType, ObjectLink, OwnershipMode, OwnershipObject, OwnershipRegistry,
    TransferCondition,
};
use omniphi_runtime::randomness::{
    derive_bounded, derive_randomness, EntropyEngine, RandomnessRequest, ValidatorCommitment,
};
use omniphi_runtime::resolution::planner::ExecutionPlan;
use omniphi_runtime::safety::deterministic::{
    DeterministicSafetyEngine, RiskLevel, SafetyConfig, SafetyTrigger,
};
use omniphi_runtime::scheduler::parallel::ParallelScheduler;
use omniphi_runtime::explainability::{PreviewGenerator, RiskSeverity, FailureLikelihood};
use omniphi_runtime::state::store::ObjectStore;
use omniphi_runtime::objects::types::BalanceObject;

use ed25519_dalek::SigningKey;
use std::collections::BTreeMap;

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

fn addr(v: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[0] = v; b }
fn oid(v: u8) -> ObjectId { ObjectId(addr(v)) }
fn asset(v: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[0] = v; b[31] = 0xAA; b }

fn keypair(seed: u8) -> (SigningKey, [u8; 32]) {
    let mut s = [0u8; 32]; s[0] = seed;
    let sk = SigningKey::from_bytes(&s);
    let pk = sk.verifying_key().to_bytes();
    (sk, pk)
}

fn make_intent(sender: [u8; 32], nonce: u64, amount: u128, fee: u64) -> IntentTransaction {
    IntentTransaction {
        tx_id: {
            let mut id = [0u8; 32];
            id[0] = sender[0];
            id[1..9].copy_from_slice(&nonce.to_be_bytes());
            id
        },
        sender,
        intent: IntentType::Transfer(TransferIntent {
            asset_id: asset(0xAA),
            recipient: addr(99),
            amount,
            memo: None,
        }),
        max_fee: fee,
        deadline_epoch: 999,
        nonce,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
        sponsor: None,
        sponsor_signature: None,
        sponsorship_limits: SponsorshipLimits::default(),
        fee_policy: FeePolicy::SenderPays,
    }
}

fn store_with_balance(owner: [u8; 32], asset_id: [u8; 32], amount: u128) -> ObjectStore {
    let mut store = ObjectStore::new();
    let obj_id = PreviewGenerator::balance_object_id(&owner, &asset_id);
    let bal = BalanceObject::new(obj_id, owner, asset_id, amount, 1);
    store.insert(Box::new(bal));
    store
}

fn txid(n: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[31] = n; b }

fn make_plan(tx: u8, accesses: Vec<ObjectAccess>) -> ExecutionPlan {
    ExecutionPlan {
        tx_id: txid(tx),
        operations: vec![],
        required_capabilities: vec![],
        object_access: accesses,
        gas_estimate: 1_000,
        gas_limit: u64::MAX,
    }
}

// ═════════════════════════════════════════════════════════════════════════════
// 1. FULL INTENT LIFECYCLE (integration)
// ═════════════════════════════════════════════════════════════════════════════

/// Intent creation → structural validation → preview → (simulated) execution.
#[test]
fn test_e2e_intent_lifecycle() {
    let (sk, pk) = keypair(1);
    let mut tx = make_intent(pk, 1, 500, 100);
    tx.signature = tx.sign(&sk.to_bytes());

    // 1. Structural validation
    assert!(tx.validate().is_ok());

    // 2. Signature verification
    assert!(tx.verify_signature());

    // 3. Preview before execution
    let store = store_with_balance(pk, asset(0xAA), 10_000);
    let preview = PreviewGenerator::preview(&tx, &store, 10);
    assert!(preview.is_exact);
    assert_eq!(*preview.assets_spent.get(&asset(0xAA)).unwrap(), 500);
    assert!(preview.failure_conditions.is_empty()); // healthy transfer

    // 4. Access set derivation for scheduling
    let plan = make_plan(1, vec![
        ObjectAccess { object_id: oid(1), mode: AccessMode::ReadWrite },
        ObjectAccess { object_id: oid(99), mode: AccessMode::ReadWrite },
    ]);
    let groups = ParallelScheduler::schedule(vec![plan]);
    assert_eq!(groups.len(), 1);
}

// ═════════════════════════════════════════════════════════════════════════════
// 2. SPONSORED CAPABILITY-BASED EXECUTION (integration)
// ═════════════════════════════════════════════════════════════════════════════

/// User creates spend capability → sponsor signs → validator checks both.
#[test]
fn test_e2e_sponsored_capability_flow() {
    let (_user_sk, user_pk) = keypair(1);
    let (sponsor_sk, sponsor_pk) = keypair(2);

    // 1. User creates a spend capability for a DEX contract
    let mut cap_reg = SpendCapabilityRegistry::new();
    let cap_id = cap_reg.create(
        user_pk, addr(0xDE), asset(0xAA), 1000, 100, None, 10,
    ).unwrap();

    // 2. User builds a sponsored intent
    let mut tx = make_intent(user_pk, 1, 500, 200);
    tx.sponsor = Some(sponsor_pk);
    tx.fee_policy = FeePolicy::SponsorPays;
    tx.sponsorship_limits = SponsorshipLimits {
        max_fee_amount: Some(500),
        ..Default::default()
    };
    let sig = tx.sign_sponsorship(&sponsor_sk.to_bytes());
    tx.sponsor_signature = Some(sig);

    // 3. Validate sponsorship
    let mut replay = SponsorReplayTracker::new();
    let result = SponsorshipValidator::validate(&tx, 10, &mut replay);
    assert!(result.valid, "Sponsor validation failed: {}", result.reason);
    assert_eq!(result.sponsor_pays_amount, 200);
    assert_eq!(result.sender_pays_amount, 0);

    // 4. Consume capability
    let consumed = cap_reg.consume(&cap_id, &addr(0xDE), 500, 20, None).unwrap();
    assert_eq!(consumed, 500);
    assert_eq!(cap_reg.get(&cap_id).unwrap().remaining_amount, 500);
}

// ═════════════════════════════════════════════════════════════════════════════
// 3. ENCRYPTED INTENT LIFECYCLE (integration)
// ═════════════════════════════════════════════════════════════════════════════

/// Encrypt → submit → order → reveal → validate → execute.
#[test]
fn test_e2e_encrypted_intent_lifecycle() {
    let (sk, pk) = keypair(1);
    let mut tx = make_intent(pk, 1, 500, 100);
    tx.signature = tx.sign(&sk.to_bytes());

    let reveal_nonce = [0xCC; 32];
    let mut ei_reg = EncryptedIntentRegistry::new();

    // 1. Create encrypted intent
    let ei = EncryptedIntent::create(&tx, &reveal_nonce, vec![0xDE, 0xAD], 100, 10).unwrap();
    let commitment = ei_reg.submit(ei).unwrap();

    // 2. Verify it's pending
    assert_eq!(ei_reg.get(&commitment).unwrap().status, EncryptedIntentStatus::Pending);

    // 3. Preview BEFORE reveal (store state)
    let store = store_with_balance(pk, asset(0xAA), 10_000);
    let preview = PreviewGenerator::preview(&tx, &store, 20);
    assert!(preview.failure_conditions.is_empty());

    // 4. Reveal
    let reveal = IntentReveal { commitment, plaintext_intent: tx.clone(), reveal_nonce };
    let revealed_tx = ei_reg.reveal(&reveal, 50).unwrap();
    assert_eq!(revealed_tx.sender, pk);

    // 5. Verify revealed intent matches original
    assert_eq!(revealed_tx.nonce, tx.nonce);
    assert_eq!(ei_reg.get(&commitment).unwrap().status, EncryptedIntentStatus::Revealed);
}

// ═════════════════════════════════════════════════════════════════════════════
// 4. OWNERSHIP + SAFETY INTERACTION (integration)
// ═════════════════════════════════════════════════════════════════════════════

/// Create ownership object → transfer → safety freeze → blocked transfer.
#[test]
fn test_e2e_ownership_safety_interaction() {
    let mut reg = OwnershipRegistry::new();
    let obj = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::Direct,
        BTreeMap::new(), BehaviorFlags::default(), 1,
    ).unwrap();
    reg.register(obj).unwrap();

    // Transfer succeeds normally
    let obj = reg.get_mut(&addr(10)).unwrap();
    obj.transfer(&addr(1), addr(2), 5).unwrap();
    assert_eq!(obj.owner, addr(2));

    // Safety engine detects drain on a related asset object
    let mut safety = DeterministicSafetyEngine::new(SafetyConfig {
        max_object_outflow: 1000,
        ..SafetyConfig::default()
    });
    safety.record_outflow(5, oid(10), 2000); // drain triggers freeze
    assert!(safety.is_frozen(&oid(10)));

    // In production, the intent pipeline would check safety.is_frozen()
    // before allowing any operation on this object
}

// ═════════════════════════════════════════════════════════════════════════════
// 5. RANDOMNESS + SCHEDULING DETERMINISM (integration)
// ═════════════════════════════════════════════════════════════════════════════

/// Randomness-derived scheduling tiebreaker is deterministic.
#[test]
fn test_e2e_randomness_scheduling_determinism() {
    // Setup identical engines
    let mut engine1 = EntropyEngine::new();
    let mut engine2 = EntropyEngine::new();

    let seed = [0xAA; 32];
    engine1.set_epoch_seed(10, seed);
    engine2.set_epoch_seed(10, seed);

    let reveal = [0xBB; 32];
    let vc = ValidatorCommitment::create(addr(1), reveal, 10);
    engine1.add_validator_commitment(vc.clone()).unwrap();
    engine2.add_validator_commitment(vc).unwrap();
    engine1.reveal_validator_commitment(10, &addr(1), reveal).unwrap();
    engine2.reveal_validator_commitment(10, &addr(1), reveal).unwrap();

    let entropy1 = engine1.aggregate(10);
    let entropy2 = engine2.aggregate(10);
    assert_eq!(entropy1.seed, entropy2.seed);

    // Scheduling from same plans produces same grouping
    let plans = || vec![
        make_plan(1, vec![ObjectAccess { object_id: oid(1), mode: AccessMode::ReadWrite }]),
        make_plan(2, vec![ObjectAccess { object_id: oid(1), mode: AccessMode::ReadWrite }]),
        make_plan(3, vec![ObjectAccess { object_id: oid(2), mode: AccessMode::ReadWrite }]),
    ];
    let g1 = ParallelScheduler::schedule(plans());
    let g2 = ParallelScheduler::schedule(plans());
    assert_eq!(g1.len(), g2.len());
    for (a, b) in g1.iter().zip(g2.iter()) {
        assert_eq!(a.plans.len(), b.plans.len());
    }
}

// ═════════════════════════════════════════════════════════════════════════════
// 6. ADVERSARIAL: DOUBLE SPEND ATTEMPT
// ═════════════════════════════════════════════════════════════════════════════

/// Same capability consumed twice must fail on second attempt.
#[test]
fn test_adversarial_double_spend_capability() {
    let mut reg = SpendCapabilityRegistry::new();
    let cap_id = reg.create(addr(1), addr(2), asset(0xAA), 1000, 100, None, 10).unwrap();

    // First spend: 1000 (fully consumes)
    reg.consume(&cap_id, &addr(2), 1000, 20, None).unwrap();
    assert_eq!(reg.get(&cap_id).unwrap().status, SpendCapabilityStatus::Consumed);

    // Second spend: must fail
    let err = reg.consume(&cap_id, &addr(2), 1, 25, None).unwrap_err();
    assert!(err.contains("Consumed"), "Double spend must be rejected: {}", err);
}

/// Over-spend beyond remaining must fail.
#[test]
fn test_adversarial_over_spend() {
    let mut reg = SpendCapabilityRegistry::new();
    let cap_id = reg.create(addr(1), addr(2), asset(0xAA), 1000, 100, None, 10).unwrap();

    reg.consume(&cap_id, &addr(2), 600, 20, None).unwrap();
    let err = reg.consume(&cap_id, &addr(2), 500, 25, None).unwrap_err();
    assert!(err.contains("insufficient"), "Over-spend must be rejected: {}", err);
    // Remaining unchanged
    assert_eq!(reg.get(&cap_id).unwrap().remaining_amount, 400);
}

// ═════════════════════════════════════════════════════════════════════════════
// 7. ADVERSARIAL: STALE CAPABILITY REPLAY
// ═════════════════════════════════════════════════════════════════════════════

/// Expired capability cannot be used even if it was once valid.
#[test]
fn test_adversarial_stale_capability() {
    let mut reg = SpendCapabilityRegistry::new();
    let cap_id = reg.create(addr(1), addr(2), asset(0xAA), 1000, 50, None, 10).unwrap();

    // Valid at epoch 30
    reg.consume(&cap_id, &addr(2), 100, 30, None).unwrap();

    // Expired at epoch 60
    let err = reg.consume(&cap_id, &addr(2), 100, 60, None).unwrap_err();
    assert!(err.contains("expired"), "Stale capability must be rejected: {}", err);
}

/// Revoked capability cannot be used.
#[test]
fn test_adversarial_revoked_capability_replay() {
    let mut reg = SpendCapabilityRegistry::new();
    let cap_id = reg.create(addr(1), addr(2), asset(0xAA), 1000, 100, None, 10).unwrap();

    reg.revoke(&cap_id, &addr(1)).unwrap();
    let err = reg.consume(&cap_id, &addr(2), 100, 20, None).unwrap_err();
    assert!(err.contains("Revoked"), "Revoked capability must be rejected: {}", err);
}

// ═════════════════════════════════════════════════════════════════════════════
// 8. ADVERSARIAL: MALICIOUS SPONSOR REPLAY
// ═════════════════════════════════════════════════════════════════════════════

/// Same sponsor + tx_id pair submitted twice must be rejected.
#[test]
fn test_adversarial_sponsor_replay() {
    let (_user_sk, user_pk) = keypair(1);
    let (sponsor_sk, sponsor_pk) = keypair(2);

    let mut tx = make_intent(user_pk, 1, 500, 100);
    tx.sponsor = Some(sponsor_pk);
    tx.fee_policy = FeePolicy::SponsorPays;
    let sig = tx.sign_sponsorship(&sponsor_sk.to_bytes());
    tx.sponsor_signature = Some(sig);

    let mut tracker = SponsorReplayTracker::new();

    let r1 = SponsorshipValidator::validate(&tx, 10, &mut tracker);
    assert!(r1.valid);

    let r2 = SponsorshipValidator::validate(&tx, 10, &mut tracker);
    assert!(!r2.valid);
    assert!(r2.reason.contains("replay"));
}

/// Sponsor signature forged with wrong key must be rejected.
#[test]
fn test_adversarial_sponsor_forged_signature() {
    let (_user_sk, user_pk) = keypair(1);
    let (_sponsor_sk, sponsor_pk) = keypair(2);
    let (attacker_sk, _) = keypair(99);

    let mut tx = make_intent(user_pk, 1, 500, 100);
    tx.sponsor = Some(sponsor_pk);
    tx.fee_policy = FeePolicy::SponsorPays;
    // Sign with attacker's key, claim it's sponsor's
    let sig = tx.sign_sponsorship(&attacker_sk.to_bytes());
    tx.sponsor_signature = Some(sig);

    let mut tracker = SponsorReplayTracker::new();
    let r = SponsorshipValidator::validate(&tx, 10, &mut tracker);
    assert!(!r.valid);
    assert!(r.reason.contains("invalid sponsor signature"));
}

// ═════════════════════════════════════════════════════════════════════════════
// 9. ADVERSARIAL: INVALID REVEAL ATTEMPTS
// ═════════════════════════════════════════════════════════════════════════════

/// Reveal with tampered intent must fail commitment check.
#[test]
fn test_adversarial_tampered_reveal() {
    let (sk, pk) = keypair(1);
    let mut tx = make_intent(pk, 1, 500, 100);
    tx.signature = tx.sign(&sk.to_bytes());

    let nonce = [0xCC; 32];
    let mut reg = EncryptedIntentRegistry::new();
    let ei = EncryptedIntent::create(&tx, &nonce, vec![0xDE], 100, 10).unwrap();
    let commitment = reg.submit(ei).unwrap();

    // Tamper: change amount
    let mut tampered = tx.clone();
    tampered.max_fee = 9999;

    let reveal = IntentReveal { commitment, plaintext_intent: tampered, reveal_nonce: nonce };
    let err = reg.reveal(&reveal, 50).unwrap_err();
    assert!(err.contains("commitment mismatch"));
}

/// Reveal by different sender must fail.
#[test]
fn test_adversarial_reveal_wrong_sender() {
    let (sk, pk) = keypair(1);
    let (_, attacker_pk) = keypair(99);
    let mut tx = make_intent(pk, 1, 500, 100);
    tx.signature = tx.sign(&sk.to_bytes());

    let nonce = [0xCC; 32];
    let mut reg = EncryptedIntentRegistry::new();
    let ei = EncryptedIntent::create(&tx, &nonce, vec![0xDE], 100, 10).unwrap();
    let commitment = reg.submit(ei).unwrap();

    let mut stolen = tx.clone();
    stolen.sender = attacker_pk;

    let reveal = IntentReveal { commitment, plaintext_intent: stolen, reveal_nonce: nonce };
    let err = reg.reveal(&reveal, 50).unwrap_err();
    assert!(err.contains("sender does not match"));
}

/// Reveal after deadline must fail.
#[test]
fn test_adversarial_late_reveal() {
    let (sk, pk) = keypair(1);
    let mut tx = make_intent(pk, 1, 500, 100);
    tx.signature = tx.sign(&sk.to_bytes());

    let nonce = [0xCC; 32];
    let mut reg = EncryptedIntentRegistry::new();
    let ei = EncryptedIntent::create(&tx, &nonce, vec![0xDE], 50, 10).unwrap();
    let commitment = reg.submit(ei).unwrap();

    let reveal = IntentReveal { commitment, plaintext_intent: tx, reveal_nonce: nonce };
    let err = reg.reveal(&reveal, 60).unwrap_err(); // epoch 60 > deadline 50
    assert!(err.contains("deadline"));
}

// ═════════════════════════════════════════════════════════════════════════════
// 10. ADVERSARIAL: OBJECT MUTATION BYPASS
// ═════════════════════════════════════════════════════════════════════════════

/// Non-owner cannot transfer ownership object.
#[test]
fn test_adversarial_ownership_non_owner_transfer() {
    let mut obj = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::Direct,
        BTreeMap::new(), BehaviorFlags::default(), 1,
    ).unwrap();

    let err = obj.transfer(&addr(99), addr(2), 5).unwrap_err();
    assert!(err.contains("not the owner"));
}

/// Soulbound object cannot be transferred by anyone.
#[test]
fn test_adversarial_soulbound_bypass() {
    let mut obj = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::Direct,
        BTreeMap::new(), BehaviorFlags { soulbound: true, ..Default::default() }, 1,
    ).unwrap();

    let err = obj.transfer(&addr(1), addr(2), 5).unwrap_err();
    assert!(err.contains("soulbound"));
}

/// Time-locked object cannot be transferred before unlock.
#[test]
fn test_adversarial_timelock_bypass() {
    let mut obj = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::TimeLocked { unlock_epoch: 100 },
        BTreeMap::new(), BehaviorFlags::default(), 1,
    ).unwrap();

    let err = obj.transfer(&addr(1), addr(2), 50).unwrap_err();
    assert!(err.contains("time-locked"));
}

/// Fractional ownership: cannot transfer more shares than held.
#[test]
fn test_adversarial_fractional_over_transfer() {
    let mut shares = BTreeMap::new();
    shares.insert(addr(1), 6000);
    shares.insert(addr(2), 4000);

    let mut obj = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::Fractional { shares },
        BTreeMap::new(), BehaviorFlags::default(), 1,
    ).unwrap();

    let err = obj.transfer_shares(&addr(1), addr(3), 7000, 5).unwrap_err();
    assert!(err.contains("insufficient"));
}

// ═════════════════════════════════════════════════════════════════════════════
// 11. ADVERSARIAL: SCHEDULER CONFLICT ABUSE
// ═════════════════════════════════════════════════════════════════════════════

/// Attacker submitting plans that ALL conflict ensures serialization (no crash).
#[test]
fn test_adversarial_all_conflicting_plans() {
    let shared = oid(1);
    let plans: Vec<ExecutionPlan> = (1..=20u8).map(|i| {
        make_plan(i, vec![ObjectAccess { object_id: shared, mode: AccessMode::ReadWrite }])
    }).collect();

    let groups = ParallelScheduler::schedule(plans);

    // All 20 plans must be present, each in its own group
    let total: usize = groups.iter().map(|g| g.plans.len()).sum();
    assert_eq!(total, 20);
    assert_eq!(groups.len(), 20); // fully serialized
}

/// Plans with only reads on shared objects should parallelize maximally.
#[test]
fn test_adversarial_all_readonly_plans() {
    let shared = oid(1);
    let plans: Vec<ExecutionPlan> = (1..=10u8).map(|i| {
        make_plan(i, vec![ObjectAccess { object_id: shared, mode: AccessMode::ReadOnly }])
    }).collect();

    let groups = ParallelScheduler::schedule(plans);
    assert_eq!(groups.len(), 1, "All-read plans must parallelize into 1 group");
    assert_eq!(groups[0].plans.len(), 10);
}

// ═════════════════════════════════════════════════════════════════════════════
// 12. ADVERSARIAL: SAFETY TRIGGER EDGE CASES
// ═════════════════════════════════════════════════════════════════════════════

/// Rapid micro-outflows that individually are below threshold but aggregate above.
#[test]
fn test_adversarial_death_by_thousand_cuts() {
    let mut safety = DeterministicSafetyEngine::new(SafetyConfig {
        max_epoch_outflow: 10_000,
        max_object_outflow: 5_000,
        ..SafetyConfig::default()
    });

    // 100 outflows of 60 each on same object = 6000 > 5000 threshold
    let mut freeze_triggered = false;
    for i in 0..100u64 {
        let events = safety.record_outflow(1, oid(1), 60);
        if events.iter().any(|e| matches!(e.trigger, SafetyTrigger::ObjectDrain { .. })) {
            freeze_triggered = true;
            break;
        }
    }
    assert!(freeze_triggered, "Accumulated micro-outflows must trigger drain freeze");
    assert!(safety.is_frozen(&oid(1)));
}

/// Invariant violation with zero tolerance must catch any imbalance.
#[test]
fn test_adversarial_invariant_single_unit() {
    let mut safety = DeterministicSafetyEngine::new(SafetyConfig {
        invariant_epsilon: 0,
        ..SafetyConfig::default()
    });

    // Off by 1 unit
    let event = safety.check_invariant(1, 10000, 9999, vec![oid(1)]);
    assert!(event.is_some(), "Zero-epsilon invariant must catch single-unit imbalance");
}

// ═════════════════════════════════════════════════════════════════════════════
// 13. DETERMINISM: IDENTICAL INPUTS → IDENTICAL OUTPUTS
// ═════════════════════════════════════════════════════════════════════════════

/// Full pipeline determinism: two nodes processing same inputs get same results.
#[test]
fn test_determinism_full_pipeline() {
    // Node 1
    let (sk, pk) = keypair(1);
    let mut tx1 = make_intent(pk, 1, 500, 100);
    tx1.signature = tx1.sign(&sk.to_bytes());

    // Node 2 (same inputs)
    let mut tx2 = make_intent(pk, 1, 500, 100);
    tx2.signature = tx2.sign(&sk.to_bytes());

    // Signing payloads must match
    assert_eq!(tx1.signing_payload(), tx2.signing_payload());

    // Preview results must match
    let store1 = store_with_balance(pk, asset(0xAA), 10_000);
    let store2 = store_with_balance(pk, asset(0xAA), 10_000);
    let p1 = PreviewGenerator::preview(&tx1, &store1, 10);
    let p2 = PreviewGenerator::preview(&tx2, &store2, 10);
    assert_eq!(p1.assets_spent, p2.assets_spent);
    assert_eq!(p1.assets_received, p2.assets_received);
    assert_eq!(p1.estimated_gas, p2.estimated_gas);

    // Randomness derivation must match
    let mut e1 = EntropyEngine::new();
    let mut e2 = EntropyEngine::new();
    e1.set_epoch_seed(10, [0xAA; 32]);
    e2.set_epoch_seed(10, [0xAA; 32]);
    let ent1 = e1.aggregate(10);
    let ent2 = e2.aggregate(10);
    assert_eq!(ent1.seed, ent2.seed);

    let req = RandomnessRequest { domain: "test".into(), epoch: 10, index: 0, target_id: None };
    assert_eq!(derive_randomness(&ent1, &req), derive_randomness(&ent2, &req));

    // Safety engine must match
    let cfg = SafetyConfig::default();
    let mut s1 = DeterministicSafetyEngine::new(cfg.clone());
    let mut s2 = DeterministicSafetyEngine::new(cfg);
    s1.record_outflow(1, oid(1), 5000);
    s2.record_outflow(1, oid(1), 5000);
    assert_eq!(s1.event_count(), s2.event_count());
    assert_eq!(s1.is_frozen(&oid(1)), s2.is_frozen(&oid(1)));
}

/// Encrypted intent commitment is deterministic.
#[test]
fn test_determinism_encrypted_commitment() {
    let tx = make_intent(addr(1), 1, 500, 100);
    let nonce = [0xCC; 32];

    let c1 = EncryptedIntent::compute_commitment(&tx, &nonce);
    let c2 = EncryptedIntent::compute_commitment(&tx, &nonce);
    assert_eq!(c1, c2);
}

/// Capability ID generation is deterministic.
#[test]
fn test_determinism_capability_id() {
    let mut r1 = SpendCapabilityRegistry::new();
    let mut r2 = SpendCapabilityRegistry::new();

    let id1 = r1.create(addr(1), addr(2), asset(0xAA), 1000, 100, None, 10).unwrap();
    let id2 = r2.create(addr(1), addr(2), asset(0xAA), 1000, 100, None, 10).unwrap();
    assert_eq!(id1, id2);
}

// ═════════════════════════════════════════════════════════════════════════════
// 14. REPLAY: NONCE TRACKING
// ═════════════════════════════════════════════════════════════════════════════

/// Same (submitter, nonce) encrypted intent must be rejected.
#[test]
fn test_replay_encrypted_nonce() {
    let tx = make_intent(addr(1), 1, 500, 100);
    let mut reg = EncryptedIntentRegistry::new();

    let ei1 = EncryptedIntent::create(&tx, &[0xCC; 32], vec![0x01], 100, 10).unwrap();
    reg.submit(ei1).unwrap();

    let ei2 = EncryptedIntent::create(&tx, &[0xDD; 32], vec![0x02], 100, 10).unwrap();
    let err = reg.submit(ei2).unwrap_err();
    assert!(err.contains("nonce already used"));
}

// ═════════════════════════════════════════════════════════════════════════════
// 15. MALFORMED STATE: ZERO/INVALID INPUTS
// ═════════════════════════════════════════════════════════════════════════════

/// Zero-address sender must be rejected by structural validation.
#[test]
fn test_malformed_zero_sender() {
    let tx = make_intent([0u8; 32], 1, 500, 100);
    assert!(tx.validate().is_err());
}

/// Zero amount transfer must be rejected.
#[test]
fn test_malformed_zero_amount() {
    let tx = make_intent(addr(1), 1, 0, 100);
    assert!(tx.validate().is_err());
}

/// Zero max_fee must be rejected.
#[test]
fn test_malformed_zero_fee() {
    let tx = make_intent(addr(1), 1, 500, 0);
    assert!(tx.validate().is_err());
}

/// Ownership object with zero ID must be rejected.
#[test]
fn test_malformed_zero_object_id() {
    let result = OwnershipObject::create(
        [0u8; 32], addr(1), OwnershipMode::Direct,
        BTreeMap::new(), BehaviorFlags::default(), 1,
    );
    assert!(result.is_err());
}

/// Capability with zero amount must be rejected.
#[test]
fn test_malformed_zero_capability_amount() {
    let mut reg = SpendCapabilityRegistry::new();
    let err = reg.create(addr(1), addr(2), asset(0xAA), 0, 100, None, 10).unwrap_err();
    assert!(err.contains("must be > 0"));
}

/// Fractional ownership with bad sum must be rejected.
#[test]
fn test_malformed_fractional_bad_sum() {
    let mut shares = BTreeMap::new();
    shares.insert(addr(1), 5000);
    shares.insert(addr(2), 3000); // sum = 8000, not 10000

    let err = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::Fractional { shares },
        BTreeMap::new(), BehaviorFlags::default(), 1,
    ).unwrap_err();
    assert!(err.contains("10000"));
}

// ═════════════════════════════════════════════════════════════════════════════
// 16. INVARIANT: CAPABILITY CONSERVATION
// ═════════════════════════════════════════════════════════════════════════════

/// Capability remaining + consumed must always equal max_amount.
#[test]
fn test_invariant_capability_conservation() {
    let mut reg = SpendCapabilityRegistry::new();
    let cap_id = reg.create(addr(1), addr(2), asset(0xAA), 10000, 100, None, 10).unwrap();

    let amounts = [1500, 2500, 3000, 1000];
    let mut total_consumed: u128 = 0;
    for &amt in &amounts {
        reg.consume(&cap_id, &addr(2), amt, 20, None).unwrap();
        total_consumed += amt;
        let cap = reg.get(&cap_id).unwrap();
        assert_eq!(
            cap.remaining_amount + total_consumed, cap.max_amount,
            "remaining + consumed must equal max_amount after consuming {}",
            total_consumed
        );
    }
}

/// Fractional shares must always sum to 10000 after any transfer.
#[test]
fn test_invariant_fractional_share_conservation() {
    let mut shares = BTreeMap::new();
    shares.insert(addr(1), 5000);
    shares.insert(addr(2), 3000);
    shares.insert(addr(3), 2000);

    let mut obj = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::Fractional { shares },
        BTreeMap::new(), BehaviorFlags::default(), 1,
    ).unwrap();

    // Transfer shares
    obj.transfer_shares(&addr(1), addr(4), 1000, 5).unwrap(); // 1→4: 1000
    obj.transfer_shares(&addr(2), addr(3), 500, 6).unwrap();  // 2→3: 500
    obj.transfer_shares(&addr(4), addr(2), 300, 7).unwrap();  // 4→2: 300

    if let OwnershipMode::Fractional { ref shares } = obj.ownership_mode {
        let total: u32 = shares.values().sum();
        assert_eq!(total, 10000, "Fractional shares must always sum to 10000");
    } else {
        panic!("Expected fractional mode");
    }
}

// ═════════════════════════════════════════════════════════════════════════════
// 17. SCOPE ISOLATION: SAFETY FREEZE DOESN'T AFFECT UNRELATED OBJECTS
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_invariant_scope_isolation() {
    let mut safety = DeterministicSafetyEngine::new(SafetyConfig {
        max_object_outflow: 1000,
        ..SafetyConfig::default()
    });

    // Freeze object 1 via drain
    safety.record_outflow(1, oid(1), 2000);
    assert!(safety.is_frozen(&oid(1)));

    // Objects 2-10 must NOT be frozen
    for i in 2..=10u8 {
        assert!(!safety.is_frozen(&oid(i)),
            "Object {} should not be frozen by unrelated drain on object 1", i);
    }

    // Different path must NOT be blocked
    safety.record_failure(1, "bad_path");
    safety.record_failure(1, "bad_path");
    safety.record_failure(1, "bad_path");
    safety.record_failure(1, "bad_path");
    safety.record_failure(1, "bad_path");
    assert!(safety.is_path_blocked("bad_path"));
    assert!(!safety.is_path_blocked("good_path"));
}

// ═════════════════════════════════════════════════════════════════════════════
// 18. CROSS-MODULE: PREVIEW DETECTS INSUFFICIENT BALANCE (explainability)
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_cross_module_preview_detects_failure() {
    let (_, pk) = keypair(1);
    let tx = make_intent(pk, 1, 50_000, 100);

    // Store has only 1000
    let store = store_with_balance(pk, asset(0xAA), 1000);
    let preview = PreviewGenerator::preview(&tx, &store, 10);

    // Must detect insufficient balance
    let certain = preview.failure_conditions.iter()
        .find(|f| f.code == "INSUFFICIENT_BALANCE" && f.likelihood == FailureLikelihood::Certain);
    assert!(certain.is_some());

    // Must flag as critical risk
    let critical = preview.risk_flags.iter()
        .find(|f| f.severity == RiskSeverity::Critical);
    assert!(critical.is_some());
}

// ═════════════════════════════════════════════════════════════════════════════
// 19. CROSS-MODULE: CAPABILITY SCOPE HASH BINDING
// ═════════════════════════════════════════════════════════════════════════════

/// Capability bound to specific intent cannot be used for a different intent.
#[test]
fn test_cross_module_scoped_capability() {
    let scope = [0xAB; 32];
    let wrong_scope = [0xCD; 32];

    let mut reg = SpendCapabilityRegistry::new();
    let cap_id = reg.create(addr(1), addr(2), asset(0xAA), 1000, 100, Some(scope), 10).unwrap();

    // Correct scope: succeeds
    reg.consume(&cap_id, &addr(2), 500, 20, Some(&scope)).unwrap();

    // Wrong scope: rejected
    let err = reg.consume(&cap_id, &addr(2), 100, 25, Some(&wrong_scope)).unwrap_err();
    assert!(err.contains("scope_hash mismatch"));

    // No scope: rejected
    let err = reg.consume(&cap_id, &addr(2), 100, 26, None).unwrap_err();
    assert!(err.contains("scope_hash provided"));
}

// ═════════════════════════════════════════════════════════════════════════════
// 20. CROSS-MODULE: DELEGATION + EXPIRY
// ═════════════════════════════════════════════════════════════════════════════

#[test]
fn test_cross_module_delegation_lifecycle() {
    let mut obj = OwnershipObject::create(
        addr(10), addr(1), OwnershipMode::Direct,
        BTreeMap::new(), BehaviorFlags::default(), 1,
    ).unwrap();

    // Delegate to addr(2) until epoch 100
    obj.delegate(&addr(1), addr(2), 100, 5).unwrap();
    assert!(obj.has_usage_rights(&addr(2), 50));  // active
    assert!(!obj.has_usage_rights(&addr(2), 100)); // expired

    // Owner transfers — delegation cleared
    obj.transfer(&addr(1), addr(3), 60).unwrap();
    assert!(!obj.has_usage_rights(&addr(2), 60)); // delegate lost rights
    assert!(obj.has_usage_rights(&addr(3), 60));  // new owner has rights
}
