//! Full-stack contract integration tests.
//!
//! Tests the complete pipeline: deploy → solver PlanAction → conversion →
//! settlement → state verification. Also tests Ed25519 signature roundtrip.

use omniphi_runtime::objects::base::{AccessMode, ObjectAccess, ObjectId, SchemaId};
use omniphi_runtime::objects::types::ContractObject;
use omniphi_runtime::resolution::planner::{ExecutionPlan, ObjectOperation};
use omniphi_runtime::scheduler::parallel::ParallelScheduler;
use omniphi_runtime::settlement::engine::SettlementEngine;
use omniphi_runtime::state::store::ObjectStore;
use omniphi_runtime::capabilities::checker::Capability;
use omniphi_runtime::solver_market::market::{PlanAction, PlanActionType};
use omniphi_runtime::intents::base::{IntentTransaction, IntentType, IntentConstraints, ExecutionMode};
use std::collections::BTreeMap;

fn sid(name: &str) -> SchemaId {
    use sha2::{Digest, Sha256};
    let h = Sha256::digest(format!("FULLSTACK_{}", name).as_bytes());
    let mut id = [0u8; 32];
    id.copy_from_slice(&h);
    id
}

fn cid(byte: u8) -> ObjectId {
    let mut id = [0u8; 32];
    id[0] = byte;
    ObjectId(id)
}

fn make_plan(contract_id: ObjectId, schema_id: SchemaId, state: &str) -> ExecutionPlan {
    ExecutionPlan {
        tx_id: [0x01; 32],
        operations: vec![ObjectOperation::ContractStateTransition {
            contract_id,
            schema_id,
            proposed_state: state.as_bytes().to_vec(),
        }],
        required_capabilities: vec![Capability::ContractCall(schema_id)],
        object_access: vec![ObjectAccess { object_id: contract_id, mode: AccessMode::ReadWrite }],
        gas_estimate: 10_000,
        gas_limit: 100_000,
    }
}

fn settle(store: &mut ObjectStore, plan: ExecutionPlan, epoch: u64) -> usize {
    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, store, epoch);
    result.succeeded
}

fn read_contract_state(store: &ObjectStore, id: &ObjectId) -> Vec<u8> {
    let obj = store.get(id).expect("contract should exist");
    let encoded = obj.encode();
    let contract: ContractObject = bincode::deserialize(&encoded).expect("should deserialize");
    contract.state
}

// ─── Tests ──────────────────────────────────────────────────────────────────

#[test]
fn test_full_lifecycle() {
    let mut store = ObjectStore::new();
    let schema = sid("escrow");
    let contract = cid(0xCC);

    let obj = ContractObject::new(contract, [1u8; 32], schema, b"idle:0".to_vec(), 65536, 0);
    store.insert(Box::new(obj));

    // idle:0 → funded:1000
    let succeeded = settle(&mut store, make_plan(contract, schema, "funded:1000"), 1);
    assert!(succeeded > 0, "should succeed");
    assert_eq!(read_contract_state(&store, &contract), b"funded:1000");
}

#[test]
fn test_sequential_transitions() {
    let mut store = ObjectStore::new();
    let schema = sid("counter");
    let contract = cid(0xAA);

    let obj = ContractObject::new(contract, [1u8; 32], schema, b"0".to_vec(), 65536, 0);
    store.insert(Box::new(obj));

    assert!(settle(&mut store, make_plan(contract, schema, "1"), 1) > 0);
    let mut p2 = make_plan(contract, schema, "2");
    p2.tx_id = [0x02; 32];
    assert!(settle(&mut store, p2, 2) > 0);

    assert_eq!(read_contract_state(&store, &contract), b"2");
}

#[test]
fn test_wrong_schema_rejected() {
    let mut store = ObjectStore::new();
    let real = sid("real");
    let wrong = sid("wrong");
    let contract = cid(0xBB);

    let obj = ContractObject::new(contract, [1u8; 32], real, b"init".to_vec(), 65536, 0);
    store.insert(Box::new(obj));

    let succeeded = settle(&mut store, make_plan(contract, wrong, "hacked"), 1);
    assert_eq!(succeeded, 0, "wrong schema should fail");
    assert_eq!(read_contract_state(&store, &contract), b"init");
}

#[test]
fn test_oversized_state_rejected() {
    let mut store = ObjectStore::new();
    let schema = sid("tiny");
    let contract = cid(0xDD);

    let obj = ContractObject::new(contract, [1u8; 32], schema, b"ok".to_vec(), 10, 0);
    store.insert(Box::new(obj));

    let big = "A".repeat(100);
    let succeeded = settle(&mut store, make_plan(contract, schema, &big), 1);
    assert_eq!(succeeded, 0, "oversized state should fail");
}

#[test]
fn test_solver_plan_action_to_settlement() {
    let mut store = ObjectStore::new();
    let schema = sid("escrow_v2");
    let contract = cid(0xEE);

    let obj = ContractObject::new(contract, [1u8; 32], schema, b"empty".to_vec(), 65536, 0);
    store.insert(Box::new(obj));

    // Solver builds PlanAction
    let mut metadata = BTreeMap::new();
    metadata.insert("schema_id".to_string(), hex::encode(schema));
    metadata.insert("proposed_state".to_string(), hex::encode(b"deposited:500"));

    let action = PlanAction {
        action_type: PlanActionType::Custom("contract_state_transition".to_string()),
        target_object: contract,
        amount: None,
        metadata,
    };

    // Convert → ObjectOperation
    let op = omniphi_runtime::poseq::interface::plan_action_to_operation(&action);

    match &op {
        ObjectOperation::ContractStateTransition { contract_id, schema_id, proposed_state } => {
            assert_eq!(*contract_id, contract);
            assert_eq!(*schema_id, schema);
            assert_eq!(proposed_state, b"deposited:500");
        }
        _ => panic!("expected ContractStateTransition"),
    }

    // Execute
    let plan = ExecutionPlan {
        tx_id: [0xFF; 32],
        operations: vec![op],
        required_capabilities: vec![Capability::ContractCall(schema)],
        object_access: vec![ObjectAccess { object_id: contract, mode: AccessMode::ReadWrite }],
        gas_estimate: 10_000,
        gas_limit: 100_000,
    };

    assert!(settle(&mut store, plan, 1) > 0);
    assert_eq!(read_contract_state(&store, &contract), b"deposited:500");
}

#[test]
fn test_ed25519_signature_roundtrip() {
    use ed25519_dalek::{SigningKey, VerifyingKey};

    let seed = [42u8; 32];
    let signing_key = SigningKey::from_bytes(&seed);
    let pubkey = VerifyingKey::from(&signing_key);
    let sender = pubkey.to_bytes();

    let transfer_intent = omniphi_runtime::intents::types::TransferIntent {
        recipient: [2u8; 32],
        asset_id: [0xAA; 32],
        amount: 1000,
        memo: None,
    };

    let mut tx = IntentTransaction {
        tx_id: [0xAA; 32],
        sender,
        intent: IntentType::Transfer(transfer_intent),
        nonce: 1,
        max_fee: 1000,
        deadline_epoch: 100,
        signature: [0u8; 64],
        metadata: BTreeMap::new(),
        target_objects: vec![],
        constraints: IntentConstraints::default(),
        execution_mode: ExecutionMode::BestEffort,
    };

    tx.signature = tx.sign(&seed);
    assert!(tx.verify_signature(), "valid sig should verify");

    let mut tampered = tx.clone();
    tampered.nonce = 999;
    assert!(!tampered.verify_signature(), "tampered should fail");
}
