//! Section 2 — Typed Object Storage tests
//!
//! Tests: schema registration, typed writes, validation, migration, determinism.

use omniphi_runtime::contracts::advanced::{ContractStorage, StorageValue};
use omniphi_runtime::contracts::schema::{
    ContractSchema, ContractSchemaRegistry, FieldDefinition, FieldType, IntentSchema, ObjectSchema,
};
use omniphi_runtime::objects::base::{Object, ObjectId};
use omniphi_runtime::objects::types::ContractObject;

fn schema_id(n: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[0] = n; b }
fn addr(n: u8) -> [u8; 32] { let mut b = [0u8; 32]; b[0] = n; b }

fn make_escrow_schema() -> ObjectSchema {
    ObjectSchema::new("Escrow", vec![
        FieldDefinition { name: "status".into(), field_type: FieldType::String, required: true, default: None },
        FieldDefinition { name: "amount".into(), field_type: FieldType::Uint128, required: true, default: None },
        FieldDefinition { name: "buyer".into(), field_type: FieldType::Bytes32, required: true, default: None },
        FieldDefinition { name: "seller".into(), field_type: FieldType::Bytes32, required: true, default: None },
        FieldDefinition { name: "memo".into(), field_type: FieldType::String, required: false, default: Some(StorageValue::String("".into())) },
    ])
}

fn make_contract_schema(sid: [u8; 32], obj_schema: ObjectSchema) -> ContractSchema {
    ContractSchema {
        schema_id: sid,
        deployer: addr(0x01),
        version: 1,
        object_schema: obj_schema,
        intent_schemas: vec![IntentSchema {
            method: "fund".into(),
            param_names: vec!["amount".into()],
            required_capabilities: vec![],
        }],
        required_capabilities: vec![],
        domain_tag: "escrow".into(),
        max_gas_per_call: 100_000,
        max_state_bytes: 4096,
        validator_hash: [0u8; 32],
    }
}

// ──────────────────────────────────────────────
// Schema Registration
// ──────────────────────────────────────────────

#[test]
fn test_valid_schema_registration() {
    let mut reg = ContractSchemaRegistry::new();
    let schema = make_contract_schema(schema_id(1), make_escrow_schema());
    assert!(reg.register(schema).is_ok());
    assert_eq!(reg.count(), 1);
    assert!(reg.get(&schema_id(1)).is_some());
}

#[test]
fn test_duplicate_schema_registration_rejected() {
    let mut reg = ContractSchemaRegistry::new();
    let s1 = make_contract_schema(schema_id(1), make_escrow_schema());
    let s2 = make_contract_schema(schema_id(1), make_escrow_schema());
    reg.register(s1).unwrap();
    let err = reg.register(s2).unwrap_err();
    assert!(err.contains("already registered"), "Expected already registered, got: {}", err);
}

#[test]
fn test_schema_lookup_by_deployer() {
    let mut reg = ContractSchemaRegistry::new();
    let schema = make_contract_schema(schema_id(1), make_escrow_schema());
    reg.register(schema).unwrap();
    let schemas = reg.by_deployer(&addr(0x01));
    assert_eq!(schemas.len(), 1);
    assert_eq!(schemas[0].object_schema.name, "Escrow");
}

// ──────────────────────────────────────────────
// Typed Writes — Valid
// ──────────────────────────────────────────────

#[test]
fn test_valid_typed_write() {
    let schema = make_escrow_schema();
    let mut storage = ContractStorage::new();
    storage.set("status", StorageValue::String("funded".into()));
    storage.set("amount", StorageValue::Uint128(1000));
    storage.set("buyer", StorageValue::Bytes32([1u8; 32]));
    storage.set("seller", StorageValue::Bytes32([2u8; 32]));

    assert!(schema.validate_state(&storage).is_ok());
}

#[test]
fn test_valid_typed_write_with_optional_field() {
    let schema = make_escrow_schema();
    let mut storage = ContractStorage::new();
    storage.set("status", StorageValue::String("funded".into()));
    storage.set("amount", StorageValue::Uint128(1000));
    storage.set("buyer", StorageValue::Bytes32([1u8; 32]));
    storage.set("seller", StorageValue::Bytes32([2u8; 32]));
    storage.set("memo", StorageValue::String("test transaction".into()));

    assert!(schema.validate_state(&storage).is_ok());
}

// ──────────────────────────────────────────────
// Typed Writes — Invalid
// ──────────────────────────────────────────────

#[test]
fn test_invalid_typed_write_wrong_type() {
    let schema = make_escrow_schema();
    let mut storage = ContractStorage::new();
    storage.set("status", StorageValue::Uint128(42)); // wrong: should be String
    storage.set("amount", StorageValue::Uint128(1000));
    storage.set("buyer", StorageValue::Bytes32([1u8; 32]));
    storage.set("seller", StorageValue::Bytes32([2u8; 32]));

    let err = schema.validate_state(&storage).unwrap_err();
    assert!(err.contains("status"), "Expected status type error, got: {}", err);
    assert!(err.contains("expected type String"), "Expected type name, got: {}", err);
}

#[test]
fn test_invalid_typed_write_missing_required_field() {
    let schema = make_escrow_schema();
    let mut storage = ContractStorage::new();
    storage.set("status", StorageValue::String("funded".into()));
    // missing: amount, buyer, seller

    let err = schema.validate_state(&storage).unwrap_err();
    assert!(err.contains("missing required field"), "Expected missing field, got: {}", err);
}

#[test]
fn test_missing_optional_field_is_ok() {
    let schema = make_escrow_schema();
    let mut storage = ContractStorage::new();
    storage.set("status", StorageValue::String("funded".into()));
    storage.set("amount", StorageValue::Uint128(1000));
    storage.set("buyer", StorageValue::Bytes32([1u8; 32]));
    storage.set("seller", StorageValue::Bytes32([2u8; 32]));
    // memo is optional, not provided

    assert!(schema.validate_state(&storage).is_ok());
}

// ──────────────────────────────────────────────
// Default Values
// ──────────────────────────────────────────────

#[test]
fn test_apply_defaults_fills_optional_fields() {
    let schema = make_escrow_schema();
    let mut storage = ContractStorage::new();
    storage.set("status", StorageValue::String("idle".into()));
    storage.set("amount", StorageValue::Uint128(0));
    storage.set("buyer", StorageValue::Bytes32([0u8; 32]));
    storage.set("seller", StorageValue::Bytes32([0u8; 32]));

    assert!(storage.get("memo").is_none());
    schema.apply_defaults(&mut storage);
    assert_eq!(storage.get_string("memo"), Some(""));
}

#[test]
fn test_apply_defaults_does_not_overwrite_existing() {
    let schema = make_escrow_schema();
    let mut storage = ContractStorage::new();
    storage.set("status", StorageValue::String("idle".into()));
    storage.set("amount", StorageValue::Uint128(0));
    storage.set("buyer", StorageValue::Bytes32([0u8; 32]));
    storage.set("seller", StorageValue::Bytes32([0u8; 32]));
    storage.set("memo", StorageValue::String("already set".into()));

    schema.apply_defaults(&mut storage);
    assert_eq!(storage.get_string("memo"), Some("already set"));
}

// ──────────────────────────────────────────────
// Deterministic Serialization and Hashing
// ──────────────────────────────────────────────

#[test]
fn test_deterministic_serialization() {
    let mut s1 = ContractStorage::new();
    s1.set("b_field", StorageValue::Uint128(2));
    s1.set("a_field", StorageValue::Uint128(1));

    let mut s2 = ContractStorage::new();
    s2.set("a_field", StorageValue::Uint128(1));
    s2.set("b_field", StorageValue::Uint128(2));

    // BTreeMap is sorted by key, so insertion order doesn't matter
    assert_eq!(s1.to_bytes(), s2.to_bytes());
}

#[test]
fn test_state_hash_changes_on_mutation() {
    let sid = schema_id(0xAA);
    let mut storage = ContractStorage::new();
    storage.set("counter", StorageValue::Uint128(0));

    let mut contract = ContractObject::new(
        ObjectId::new([1u8; 32]), addr(0x01), sid, storage.to_bytes(), 4096, 0,
    );
    let hash_before = contract.state_hash;

    storage.set("counter", StorageValue::Uint128(1));
    contract.set_state(storage.to_bytes());
    let hash_after = contract.state_hash;

    assert_ne!(hash_before, hash_after);
}

#[test]
fn test_same_state_same_hash() {
    let sid = schema_id(0xAA);
    let mut storage = ContractStorage::new();
    storage.set("value", StorageValue::Uint128(42));

    let c1 = ContractObject::new(
        ObjectId::new([1u8; 32]), addr(0x01), sid, storage.to_bytes(), 4096, 0,
    );
    let c2 = ContractObject::new(
        ObjectId::new([2u8; 32]), addr(0x01), sid, storage.to_bytes(), 4096, 0,
    );

    assert_eq!(c1.state_hash, c2.state_hash);
}

// ──────────────────────────────────────────────
// Schema-Validated Persistence
// ──────────────────────────────────────────────

#[test]
fn test_validate_and_set_state_success() {
    let obj_schema = make_escrow_schema();
    let sid = schema_id(0xBB);

    let initial = ContractStorage::new();
    let mut contract = ContractObject::new(
        ObjectId::new([1u8; 32]), addr(0x01), sid, initial.to_bytes(), 4096, 0,
    );

    let mut new_state = ContractStorage::new();
    new_state.set("status", StorageValue::String("funded".into()));
    new_state.set("amount", StorageValue::Uint128(500));
    new_state.set("buyer", StorageValue::Bytes32([1u8; 32]));
    new_state.set("seller", StorageValue::Bytes32([2u8; 32]));

    assert!(contract.validate_and_set_state(&new_state, &obj_schema).is_ok());

    // Verify persisted correctly
    let read_back = contract.read_typed_state().unwrap();
    assert_eq!(read_back.get_uint128("amount"), Some(500));
    assert_eq!(read_back.get_string("status"), Some("funded"));
}

#[test]
fn test_validate_and_set_state_rejects_invalid() {
    let obj_schema = make_escrow_schema();
    let sid = schema_id(0xBB);

    let initial = ContractStorage::new();
    let mut contract = ContractObject::new(
        ObjectId::new([1u8; 32]), addr(0x01), sid, initial.to_bytes(), 4096, 0,
    );

    let mut bad_state = ContractStorage::new();
    bad_state.set("status", StorageValue::Uint128(42)); // wrong type

    assert!(contract.validate_and_set_state(&bad_state, &obj_schema).is_err());
}

#[test]
fn test_validate_and_set_state_rejects_oversized() {
    let obj_schema = make_escrow_schema();
    let sid = schema_id(0xCC);

    let initial = ContractStorage::new();
    let mut contract = ContractObject::new(
        ObjectId::new([1u8; 32]), addr(0x01), sid, initial.to_bytes(), 50, 0, // tiny limit
    );

    let mut state = ContractStorage::new();
    state.set("status", StorageValue::String("funded".into()));
    state.set("amount", StorageValue::Uint128(500));
    state.set("buyer", StorageValue::Bytes32([1u8; 32]));
    state.set("seller", StorageValue::Bytes32([2u8; 32]));

    let err = contract.validate_and_set_state(&state, &obj_schema).unwrap_err();
    assert!(err.contains("exceeds max"), "Expected size error, got: {}", err);
}

// ──────────────────────────────────────────────
// Schema Version Migration / Backward Compatibility
// ──────────────────────────────────────────────

#[test]
fn test_schema_upgrade_add_optional_field() {
    let v1 = make_escrow_schema();
    let mut v2_fields = v1.fields.clone();
    v2_fields.push(FieldDefinition {
        name: "dispute_reason".into(),
        field_type: FieldType::String,
        required: false,
        default: Some(StorageValue::String("".into())),
    });
    let v2 = ObjectSchema::new("Escrow", v2_fields);

    assert!(v1.is_compatible_upgrade(&v2).is_ok());
}

#[test]
fn test_schema_upgrade_add_required_field_rejected() {
    let v1 = make_escrow_schema();
    let mut v2_fields = v1.fields.clone();
    v2_fields.push(FieldDefinition {
        name: "arbiter".into(),
        field_type: FieldType::Bytes32,
        required: true, // breaks existing state
        default: None,
    });
    let v2 = ObjectSchema::new("Escrow", v2_fields);

    let err = v1.is_compatible_upgrade(&v2).unwrap_err();
    assert!(err.contains("breaks backward compatibility"), "Expected compat error, got: {}", err);
}

#[test]
fn test_schema_upgrade_change_field_type_rejected() {
    let v1 = make_escrow_schema();
    let mut v2_fields: Vec<FieldDefinition> = v1.fields.iter().map(|f| {
        if f.name == "amount" {
            FieldDefinition { field_type: FieldType::String, ..f.clone() } // changed from Uint128
        } else {
            f.clone()
        }
    }).collect();
    let v2 = ObjectSchema::new("Escrow", v2_fields);

    let err = v1.is_compatible_upgrade(&v2).unwrap_err();
    assert!(err.contains("type changed"), "Expected type change error, got: {}", err);
}

#[test]
fn test_schema_upgrade_remove_required_field_rejected() {
    let v1 = make_escrow_schema();
    // v2 removes "amount" (required)
    let v2_fields: Vec<FieldDefinition> = v1.fields.iter()
        .filter(|f| f.name != "amount")
        .cloned()
        .collect();
    let v2 = ObjectSchema::new("Escrow", v2_fields);

    let err = v1.is_compatible_upgrade(&v2).unwrap_err();
    assert!(err.contains("required field"), "Expected required field error, got: {}", err);
}

#[test]
fn test_schema_upgrade_remove_optional_field_ok() {
    let v1 = make_escrow_schema();
    // v2 removes "memo" (optional)
    let v2_fields: Vec<FieldDefinition> = v1.fields.iter()
        .filter(|f| f.name != "memo")
        .cloned()
        .collect();
    let v2 = ObjectSchema::new("Escrow", v2_fields);

    assert!(v1.is_compatible_upgrade(&v2).is_ok());
}

#[test]
fn test_registry_upgrade_version() {
    let mut reg = ContractSchemaRegistry::new();
    let sid = schema_id(1);
    let v1_schema = make_contract_schema(sid, make_escrow_schema());
    reg.register(v1_schema).unwrap();

    // Upgrade to v2 with new optional field
    let mut v2_fields = make_escrow_schema().fields;
    v2_fields.push(FieldDefinition {
        name: "notes".into(), field_type: FieldType::String, required: false, default: None,
    });
    let v2_obj_schema = ObjectSchema::new("Escrow", v2_fields);
    let mut v2 = make_contract_schema(sid, v2_obj_schema);
    v2.version = 2;

    assert!(reg.upgrade(v2).is_ok());
    assert_eq!(reg.get(&sid).unwrap().version, 2);
}

#[test]
fn test_registry_upgrade_lower_version_rejected() {
    let mut reg = ContractSchemaRegistry::new();
    let sid = schema_id(1);
    let v1 = make_contract_schema(sid, make_escrow_schema());
    reg.register(v1).unwrap();

    let mut v0 = make_contract_schema(sid, make_escrow_schema());
    v0.version = 0;

    let err = reg.upgrade(v0).unwrap_err();
    assert!(err.contains("version"), "Expected version error, got: {}", err);
}

// ──────────────────────────────────────────────
// Round-trip: typed state through ContractObject
// ──────────────────────────────────────────────

#[test]
fn test_typed_state_roundtrip_through_contract_object() {
    let schema = make_escrow_schema();
    let sid = schema_id(0xDD);

    let mut state = ContractStorage::new();
    state.set("status", StorageValue::String("idle".into()));
    state.set("amount", StorageValue::Uint128(0));
    state.set("buyer", StorageValue::Bytes32([0u8; 32]));
    state.set("seller", StorageValue::Bytes32([0u8; 32]));

    let mut contract = ContractObject::new(
        ObjectId::new([1u8; 32]), addr(0x01), sid, state.to_bytes(), 4096, 0,
    );

    // Mutate via schema-validated path
    let mut new_state = ContractStorage::new();
    new_state.set("status", StorageValue::String("funded".into()));
    new_state.set("amount", StorageValue::Uint128(1000));
    new_state.set("buyer", StorageValue::Bytes32([1u8; 32]));
    new_state.set("seller", StorageValue::Bytes32([2u8; 32]));
    new_state.set("memo", StorageValue::String("payment for services".into()));

    contract.validate_and_set_state(&new_state, &schema).unwrap();

    // Read back
    let read = contract.read_typed_state().unwrap();
    assert_eq!(read.get_string("status"), Some("funded"));
    assert_eq!(read.get_uint128("amount"), Some(1000));
    assert_eq!(read.get_bytes32("buyer"), Some(&[1u8; 32]));
    assert_eq!(read.get_string("memo"), Some("payment for services"));
}

// ──────────────────────────────────────────────
// Mapping storage
// ──────────────────────────────────────────────

#[test]
fn test_mapping_storage_nested_read_write() {
    let mut storage = ContractStorage::new();
    storage.set_map_entry("balances", "alice", StorageValue::Uint128(100));
    storage.set_map_entry("balances", "bob", StorageValue::Uint128(200));

    assert_eq!(
        storage.get_map_entry("balances", "alice"),
        Some(&StorageValue::Uint128(100))
    );

    // Round-trip
    let bytes = storage.to_bytes();
    let restored = ContractStorage::from_bytes(&bytes).unwrap();
    assert_eq!(
        restored.get_map_entry("balances", "bob"),
        Some(&StorageValue::Uint128(200))
    );
}
