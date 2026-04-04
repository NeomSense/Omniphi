use omniphi_runtime::{
    crx::goal_packet::{
        goal_packet_from_intent, GoalConstraints, GoalPacket, GoalPolicyEnvelope,
        PartialFailurePolicy, RiskTier, SettlementStrictness,
    },
    IntentTransaction, IntentType, TransferIntent,
    IntentConstraints, ExecutionMode, FeePolicy, SponsorshipLimits,
};
use std::collections::BTreeMap;

fn make_valid_packet() -> GoalPacket {
    GoalPacket {
        packet_id: [1u8; 32],
        intent_id: [2u8; 32],
        sender: [3u8; 32],
        desired_outcome: "test transfer".to_string(),
        constraints: GoalConstraints {
            min_output_amount: 0,
            max_slippage_bps: 300,
            allowed_object_ids: None,
            allowed_object_types: vec![omniphi_runtime::ObjectType::Balance],
            allowed_domains: vec![],
            forbidden_domains: vec![],
            max_objects_touched: 8,
        },
        policy: GoalPolicyEnvelope {
            risk_tier: RiskTier::Standard,
            partial_failure_policy: PartialFailurePolicy::StrictAllOrNothing,
            require_rights_capsule: true,
            require_causal_audit: true,
            settlement_strictness: SettlementStrictness::Strict,
        },
        max_fee: 1000,
        deadline_epoch: 500,
        nonce: 1,
        metadata: BTreeMap::new(),
    }
}

#[test]
fn test_valid_goal_packet() {
    let packet = make_valid_packet();
    assert!(packet.validate().is_ok());
}

#[test]
fn test_zero_packet_id_rejected() {
    let mut packet = make_valid_packet();
    packet.packet_id = [0u8; 32];
    let result = packet.validate();
    assert!(result.is_err());
    let msg = result.unwrap_err().to_string();
    assert!(msg.contains("packet_id"), "Expected packet_id error, got: {}", msg);
}

#[test]
fn test_zero_fee_rejected() {
    let mut packet = make_valid_packet();
    packet.max_fee = 0;
    let result = packet.validate();
    assert!(result.is_err());
    let msg = result.unwrap_err().to_string();
    assert!(msg.contains("max_fee"), "Expected max_fee error, got: {}", msg);
}

#[test]
fn test_zero_deadline_rejected() {
    let mut packet = make_valid_packet();
    packet.deadline_epoch = 0;
    let result = packet.validate();
    assert!(result.is_err());
    let msg = result.unwrap_err().to_string();
    assert!(msg.contains("deadline_epoch"), "Expected deadline_epoch error, got: {}", msg);
}

#[test]
fn test_no_object_types_or_domains_rejected() {
    let mut packet = make_valid_packet();
    packet.constraints.allowed_object_types.clear();
    packet.constraints.allowed_domains.clear();
    let result = packet.validate();
    assert!(result.is_err());
    let msg = result.unwrap_err().to_string();
    assert!(
        msg.contains("allowed_object_type") || msg.contains("allowed_domain"),
        "Expected constraint error, got: {}",
        msg
    );
}

#[test]
fn test_no_object_types_but_has_domain() {
    let mut packet = make_valid_packet();
    packet.constraints.allowed_object_types.clear();
    packet.constraints.allowed_domains = vec!["defi".to_string()];
    // Should pass: has at least one allowed_domain
    assert!(packet.validate().is_ok());
}

#[test]
fn test_hash_deterministic() {
    let packet = make_valid_packet();
    let h1 = packet.compute_hash();
    let h2 = packet.compute_hash();
    assert_eq!(h1, h2);
}

#[test]
fn test_hash_changes_with_content() {
    let p1 = make_valid_packet();
    let mut p2 = make_valid_packet();
    p2.nonce = 999;
    assert_ne!(p1.compute_hash(), p2.compute_hash());
}

#[test]
fn test_goal_packet_from_intent() {
    let intent = IntentTransaction {
        tx_id: [10u8; 32],
        sender: [20u8; 32],
        intent: IntentType::Transfer(TransferIntent {
            asset_id: [5u8; 32],
            amount: 1000,
            recipient: [30u8; 32],
            memo: None,
        }),
        max_fee: 500,
        deadline_epoch: 200,
        nonce: 7,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
            fee_envelope: None,
    };

    let packet = goal_packet_from_intent(&intent, 100);

    assert_eq!(packet.intent_id, intent.tx_id);
    assert_eq!(packet.sender, intent.sender);
    assert_eq!(packet.max_fee, intent.max_fee);
    assert_eq!(packet.deadline_epoch, intent.deadline_epoch);
    assert_eq!(packet.nonce, intent.nonce);
    assert!(!packet.constraints.allowed_object_types.is_empty());
    // Packet id must differ from intent tx_id (it's derived but different)
    assert_ne!(packet.packet_id, intent.tx_id);
    // Should be valid
    assert!(packet.validate().is_ok(), "goal_packet_from_intent should produce valid packet");
}

#[test]
fn test_goal_packet_from_intent_swap() {
    use omniphi_runtime::SwapIntent;
    let intent = IntentTransaction {
        tx_id: [11u8; 32],
        sender: [21u8; 32],
        intent: IntentType::Swap(SwapIntent {
            input_asset: [5u8; 32],
            output_asset: [6u8; 32],
            input_amount: 500,
            min_output_amount: 450,
            max_slippage_bps: 100,
            allowed_pool_ids: None,
        }),
        max_fee: 600,
        deadline_epoch: 300,
        nonce: 8,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
            sponsor: None,
            sponsor_signature: None,
            sponsorship_limits: SponsorshipLimits::default(),
            fee_policy: FeePolicy::SenderPays,
            fee_envelope: None,
    };

    let packet = goal_packet_from_intent(&intent, 100);
    assert_eq!(packet.constraints.min_output_amount, 450);
    assert_eq!(packet.constraints.max_slippage_bps, 100);
    assert!(packet.validate().is_ok());
}
