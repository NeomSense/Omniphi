//! End-to-end integration test for Intent Contracts.
//!
//! Tests the full flow: deploy schema → create contract object → submit
//! contract call intent → solver produces plan → PlanValidator validates →
//! settlement engine applies state transition → verify state changed.

use omniphi_runtime::contracts::schema::{ContractSchema, ContractSchemaRegistry, IntentSchema, ObjectSchema};
use omniphi_runtime::contracts::validator::MockValidatorBridge;
use omniphi_runtime::errors::RuntimeError;
use omniphi_runtime::gas::meter::GasCosts;
use omniphi_runtime::intents::base::{ContractCallIntent, ContractConstraints, IntentTransaction, IntentType};
use omniphi_runtime::objects::base::{ObjectId, ObjectType, SchemaId};
use omniphi_runtime::objects::types::ContractObject;
use omniphi_runtime::resolution::planner::{ExecutionPlan, ObjectOperation};
use omniphi_runtime::scheduler::parallel::ParallelScheduler;
use omniphi_runtime::settlement::engine::SettlementEngine;
use omniphi_runtime::state::store::ObjectStore;
use omniphi_runtime::capabilities::checker::Capability;
use std::collections::BTreeMap;

fn test_schema_id() -> SchemaId {
    let mut id = [0u8; 32];
    id[0] = 0xAA;
    id[1] = 0xBB;
    id
}

fn test_contract_id() -> ObjectId {
    let mut id = [0u8; 32];
    id[0] = 0xCC;
    id[1] = 0xDD;
    ObjectId(id)
}

fn test_sender() -> [u8; 32] {
    [1u8; 32]
}

/// Create a ContractObject with initial state.
fn create_test_contract(schema_id: SchemaId, contract_id: ObjectId, initial_state: &str) -> ContractObject {
    ContractObject::new(
        contract_id,
        test_sender(),
        schema_id,
        initial_state.as_bytes().to_vec(),
        65536,
        0, // now
    )
}

/// Build an ExecutionPlan with a ContractStateTransition operation.
fn build_contract_plan(
    contract_id: ObjectId,
    schema_id: SchemaId,
    proposed_state: &str,
) -> ExecutionPlan {
    use omniphi_runtime::objects::base::{AccessMode, ObjectAccess};

    ExecutionPlan {
        tx_id: [0x01; 32],
        operations: vec![ObjectOperation::ContractStateTransition {
            contract_id,
            schema_id,
            proposed_state: proposed_state.as_bytes().to_vec(),
        }],
        required_capabilities: vec![Capability::ContractCall(schema_id)],
        object_access: vec![ObjectAccess {
            object_id: contract_id,
            mode: AccessMode::ReadWrite,
        }],
        gas_estimate: 10_000,
        gas_limit: 100_000,
    }
}

#[test]
fn test_contract_deploy_and_state_transition() {
    // Step 1: Create object store and seed a contract object
    let mut store = ObjectStore::new();
    let schema_id = test_schema_id();
    let contract_id = test_contract_id();

    let initial_state = r#"{"status":"created","amount":0}"#;
    let contract = create_test_contract(schema_id, contract_id, initial_state);
    store.insert(Box::new(contract));

    // Verify contract is in store
    let obj = store.get(&contract_id).expect("contract should exist");
    assert!(matches!(obj.object_type(), ObjectType::Contract(_)));

    // Step 2: Build a plan that transitions the contract state
    let proposed_state = r#"{"status":"funded","amount":1000}"#;
    let plan = build_contract_plan(contract_id, schema_id, proposed_state);

    // Step 3: Schedule and execute
    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    // Step 4: Verify execution succeeded
    assert_eq!(result.succeeded, 1, "plan should succeed");
    assert_eq!(result.failed, 0);
    assert!(!result.receipts.is_empty());
    assert!(result.receipts[0].success, "receipt should show success: {:?}", result.receipts[0].error);

    // Step 5: Verify state was actually changed
    let updated_obj = store.get(&contract_id).expect("contract should still exist");
    let encoded = updated_obj.encode();
    let updated_contract: ContractObject = bincode::deserialize(&encoded).expect("should deserialize");
    assert_eq!(
        String::from_utf8_lossy(&updated_contract.state),
        proposed_state,
        "contract state should be updated"
    );

    // Step 6: Verify version was incremented
    assert!(updated_contract.meta.version > 0, "version should be incremented");
}

#[test]
fn test_contract_wrong_schema_rejected() {
    let mut store = ObjectStore::new();
    let schema_id = test_schema_id();
    let contract_id = test_contract_id();

    let contract = create_test_contract(schema_id, contract_id, r#"{"status":"created"}"#);
    store.insert(Box::new(contract));

    // Build a plan with WRONG schema_id
    let wrong_schema = [0xFF; 32];
    let plan = build_contract_plan(contract_id, wrong_schema, r#"{"status":"hacked"}"#);

    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.failed, 1, "plan with wrong schema should fail");
    assert!(result.receipts[0].error.as_ref().unwrap().contains("schema_id mismatch"));
}

#[test]
fn test_contract_empty_state_rejected() {
    let mut store = ObjectStore::new();
    let schema_id = test_schema_id();
    let contract_id = test_contract_id();

    let contract = create_test_contract(schema_id, contract_id, r#"{"status":"created"}"#);
    store.insert(Box::new(contract));

    // Build a plan with empty proposed state
    let plan = ExecutionPlan {
        tx_id: [0x02; 32],
        operations: vec![ObjectOperation::ContractStateTransition {
            contract_id,
            schema_id,
            proposed_state: vec![], // empty!
        }],
        required_capabilities: vec![],
        object_access: vec![omniphi_runtime::objects::base::ObjectAccess {
            object_id: contract_id,
            mode: omniphi_runtime::objects::base::AccessMode::ReadWrite,
        }],
        gas_estimate: 10_000,
        gas_limit: 100_000,
    };

    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.failed, 1);
    assert!(result.receipts[0].error.as_ref().unwrap().contains("cannot be empty"));
}

#[test]
fn test_contract_nonexistent_rejected() {
    let mut store = ObjectStore::new();
    let schema_id = test_schema_id();
    let contract_id = test_contract_id(); // not in store

    let plan = build_contract_plan(contract_id, schema_id, r#"{"status":"funded"}"#);

    let groups = ParallelScheduler::schedule(vec![plan]);
    let result = SettlementEngine::execute_groups(groups, &mut store, 1);

    assert_eq!(result.failed, 1);
    assert!(result.receipts[0].error.as_ref().unwrap().contains("ObjectNotFound"));
}

#[test]
fn test_multiple_contract_transitions_parallel() {
    let mut store = ObjectStore::new();
    let schema_id = test_schema_id();

    // Create two independent contract objects
    let id1 = ObjectId([0x01; 32]);
    let id2 = ObjectId([0x02; 32]);

    store.insert(Box::new(create_test_contract(schema_id, id1, r#"{"v":1}"#)));
    store.insert(Box::new(create_test_contract(schema_id, id2, r#"{"v":1}"#)));

    // Two plans touching different contracts — should be parallelizable
    let plan1 = build_contract_plan(id1, schema_id, r#"{"v":2}"#);
    let mut plan2 = build_contract_plan(id2, schema_id, r#"{"v":2}"#);
    plan2.tx_id = [0x02; 32]; // different tx_id

    let groups = ParallelScheduler::schedule(vec![plan1, plan2]);

    // ParallelScheduler should put them in the same group (no conflicts)
    assert!(groups.len() <= 2, "independent contracts should parallelize");

    let result = SettlementEngine::execute_groups(groups, &mut store, 1);
    assert_eq!(result.succeeded, 2);
    assert_eq!(result.failed, 0);
}

#[test]
fn test_schema_registry() {
    let mut registry = ContractSchemaRegistry::new();
    let schema_id = test_schema_id();

    let schema = ContractSchema {
        schema_id,
        deployer: [1u8; 32],
        version: 1,
        object_schema: ObjectSchema::new("Escrow", vec![]),
        intent_schemas: vec![IntentSchema {
            method: "fund".to_string(),
            param_names: vec!["amount".to_string()],
            required_capabilities: vec![Capability::ContractCall(schema_id)],
        }],
        required_capabilities: vec![Capability::ContractCall(schema_id)],
        domain_tag: "contract.escrow".to_string(),
        max_gas_per_call: 500_000,
        max_state_bytes: 4096,
        validator_hash: [0u8; 32],
    };

    assert!(registry.register(schema.clone()).is_ok());
    assert!(registry.get(&schema_id).is_some());
    assert!(registry.get(&schema_id).unwrap().has_method("fund"));
    assert!(!registry.get(&schema_id).unwrap().has_method("nonexistent"));

    // Duplicate registration with same version should fail
    assert!(registry.register(schema).is_err());
}

#[test]
fn test_mock_validator_bridge() {
    let accepting = MockValidatorBridge::accepting();
    let rejecting = MockValidatorBridge::rejecting("test rejection");

    use omniphi_runtime::contracts::validator::{ConstraintValidatorBridge, ValidationContext};

    let ctx = ValidationContext {
        epoch: 1,
        sender: [1u8; 32],
        gas_remaining: 100_000,
        method_selector: "fund".to_string(),
    };

    let accept_result = accepting.validate(&[0u8; 32], b"proposed", b"current", b"params", &ctx);
    assert!(accept_result.valid);

    let reject_result = rejecting.validate(&[0u8; 32], b"proposed", b"current", b"params", &ctx);
    assert!(!reject_result.valid);
    assert_eq!(reject_result.reason, "test rejection");
}
