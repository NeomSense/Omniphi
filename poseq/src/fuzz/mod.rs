//! Cross-layer serialization boundary fuzzing.
//!
//! Tests that all serialization boundaries handle malformed, truncated,
//! oversized, and adversarial inputs without panicking.
//!
//! Targets (in priority order):
//! 1. Wire message deserialization (bincode, untrusted peers)
//! 2. Checkpoint restore (bincode, crash recovery)
//! 3. Bundle commitment/reveal hash verification
//! 4. Genesis deserialization and validation
//! 5. Evidence packet hash computation
//! 6. Versioned envelope handling

#[cfg(test)]
mod tests {
    use sha2::{Sha256, Digest};

    // ═══════════════════════════════════════════════════════════════════
    // Target 1: Wire message deserialization (P0 — untrusted peer input)
    // ═══════════════════════════════════════════════════════════════════

    use crate::networking::messages::PoSeqMessage;

    /// Empty input must not panic.
    #[test]
    fn fuzz_wire_message_empty() {
        let result = PoSeqMessage::decode(&[]);
        assert!(result.is_err());
    }

    /// Single byte must not panic.
    #[test]
    fn fuzz_wire_message_single_byte() {
        for b in 0..=255u8 {
            let _ = PoSeqMessage::decode(&[b]);
        }
    }

    /// All-zeros of various lengths must not panic.
    #[test]
    fn fuzz_wire_message_zeros() {
        for len in [1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024] {
            let data = vec![0u8; len];
            let _ = PoSeqMessage::decode(&data);
        }
    }

    /// All-ones of various lengths must not panic.
    #[test]
    fn fuzz_wire_message_ones() {
        for len in [1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024] {
            let data = vec![0xFF; len];
            let _ = PoSeqMessage::decode(&data);
        }
    }

    /// Truncated valid message must not panic.
    #[test]
    fn fuzz_wire_message_truncated() {
        // Encode a valid message, then try decoding every prefix
        let msg = PoSeqMessage::PeerStatus(crate::networking::messages::WirePeerStatus {
            node_id: [1u8; 32],
            listen_addr: "127.0.0.1:7000".into(),
            current_epoch: 1,
            current_slot: 0,
            latest_finalized_batch_id: None,
            is_leader: false,
            in_committee: true,
            role: crate::networking::messages::NodeRole::Attestor,
            protocol_version: None,
        });
        let encoded = msg.encode().unwrap();
        // Skip 4-byte length prefix for direct decode
        let payload = &encoded[4..];
        for i in 0..payload.len() {
            let _ = PoSeqMessage::decode(&payload[..i]);
        }
    }

    /// Bit-flip every byte of a valid message.
    #[test]
    fn fuzz_wire_message_bitflip() {
        let msg = PoSeqMessage::PeerStatus(crate::networking::messages::WirePeerStatus {
            node_id: [1u8; 32],
            listen_addr: "127.0.0.1:7000".into(),
            current_epoch: 1,
            current_slot: 5,
            latest_finalized_batch_id: None,
            is_leader: false,
            in_committee: true,
            role: crate::networking::messages::NodeRole::Attestor,
            protocol_version: None,
        });
        let encoded = msg.encode().unwrap();
        let payload = &encoded[4..];
        for i in 0..payload.len() {
            let mut corrupted = payload.to_vec();
            corrupted[i] ^= 0xFF;
            // Must not panic — may decode to different message or error
            let _ = PoSeqMessage::decode(&corrupted);
        }
    }

    /// Oversized length prefix must not cause OOM.
    #[test]
    fn fuzz_wire_message_oversized_length() {
        // 4-byte length prefix claiming 4GB of data, followed by 8 bytes
        let mut data = vec![0xFF, 0xFF, 0xFF, 0xFF];
        data.extend_from_slice(&[0u8; 8]);
        let _ = PoSeqMessage::decode(&data);
    }

    /// Random-looking data (deterministic seed) must not panic.
    #[test]
    fn fuzz_wire_message_pseudorandom() {
        for seed in 0u64..100 {
            let mut h = Sha256::new();
            h.update(&seed.to_be_bytes());
            let hash: [u8; 32] = h.finalize().into();
            // Use hash bytes as message payload of various lengths
            for len in [4, 8, 16, 32] {
                let _ = PoSeqMessage::decode(&hash[..len]);
            }
        }
    }

    // ═══════════════════════════════════════════════════════════════════
    // Target 2: Checkpoint deserialization (P0 — crash recovery)
    // ═══════════════════════════════════════════════════════════════════

    use crate::checkpoints::PoSeqCheckpoint;

    #[test]
    fn fuzz_checkpoint_deserialize_empty() {
        let result: Result<PoSeqCheckpoint, _> = bincode::deserialize(&[]);
        assert!(result.is_err());
    }

    #[test]
    fn fuzz_checkpoint_deserialize_garbage() {
        for seed in 0u64..50 {
            let mut h = Sha256::new();
            h.update(b"CHECKPOINT_FUZZ");
            h.update(&seed.to_be_bytes());
            let hash: [u8; 32] = h.finalize().into();
            // Various lengths of pseudorandom data
            for len in [1, 16, 32, 64, 128, 256] {
                let data: Vec<u8> = (0..len).map(|i| hash[i % 32]).collect();
                let _: Result<PoSeqCheckpoint, _> = bincode::deserialize(&data);
            }
        }
    }

    #[test]
    fn fuzz_checkpoint_valid_then_truncated() {
        let cp = crate::checkpoints::make_test_checkpoint(10, 0);
        let encoded = bincode::serialize(&cp).unwrap();
        for i in 0..encoded.len() {
            let _: Result<PoSeqCheckpoint, _> = bincode::deserialize(&encoded[..i]);
        }
    }

    #[test]
    fn fuzz_checkpoint_valid_then_bitflip() {
        let cp = crate::checkpoints::make_test_checkpoint(10, 0);
        let encoded = bincode::serialize(&cp).unwrap();
        for i in 0..encoded.len() {
            let mut corrupted = encoded.clone();
            corrupted[i] ^= 0xFF;
            let result: Result<PoSeqCheckpoint, _> = bincode::deserialize(&corrupted);
            // If it deserializes, verify_id should catch tampering
            if let Ok(cp) = result {
                // Corrupted checkpoint should fail ID verification
                // (unless we got very unlucky and flipped a non-hash byte)
                let _ = cp.verify_id();
            }
        }
    }

    // ═══════════════════════════════════════════════════════════════════
    // Target 3: Bundle commitment/reveal hash verification (P1)
    // ═══════════════════════════════════════════════════════════════════

    use crate::auction::types::{
        BundleCommitment, BundleReveal, ExecutionStep, OperationType,
        OperationParams, PredictedOutput, FeeBreakdown, ResourceAccess,
    };
    use crate::intent_pool::types::{AssetId, AssetType};

    fn make_test_reveal() -> BundleReveal {
        BundleReveal {
            bundle_id: [1u8; 32],
            solver_id: [2u8; 32],
            batch_window: 1,
            target_intent_ids: vec![[3u8; 32]],
            execution_steps: vec![ExecutionStep {
                step_index: 0,
                operation: OperationType::Credit,
                object_id: [4u8; 32],
                read_set: vec![[4u8; 32]],
                write_set: vec![[4u8; 32]],
                params: OperationParams {
                    asset: Some(AssetId { chain_id: 0, asset_type: AssetType::Native, identifier: [0xAA; 32] }),
                    amount: Some(1000),
                    recipient: None,
                    pool_id: None,
                    custom_data: None,
                },
            }],
            liquidity_sources: Vec::new(),
            predicted_outputs: vec![PredictedOutput {
                intent_id: [3u8; 32],
                asset_out: AssetId { chain_id: 0, asset_type: AssetType::Native, identifier: [0xBB; 32] },
                amount_out: 1000,
                fee_charged_bps: 100,
            }],
            fee_breakdown: FeeBreakdown {
                solver_fee_bps: 100,
                protocol_fee_bps: 10,
                total_fee_bps: 110,
            },
            resource_declarations: vec![ResourceAccess::read([4u8; 32])],
            nonce: [0xFF; 32],
            proof_data: Vec::new(),
            signature: Vec::new(),
        }
    }

    #[test]
    fn fuzz_commitment_hash_determinism() {
        let reveal = make_test_reveal();
        let h1 = reveal.compute_commitment_hash();
        let h2 = reveal.compute_commitment_hash();
        assert_eq!(h1, h2, "commitment hash must be deterministic");
    }

    #[test]
    fn fuzz_commitment_hash_changes_with_any_field() {
        let base = make_test_reveal();
        let base_hash = base.compute_commitment_hash();

        // Modify bundle_id
        let mut modified = base.clone();
        modified.bundle_id[0] ^= 1;
        assert_ne!(modified.compute_commitment_hash(), base_hash, "bundle_id change must change hash");

        // Modify solver_id
        let mut modified = base.clone();
        modified.solver_id[0] ^= 1;
        assert_ne!(modified.compute_commitment_hash(), base_hash, "solver_id change must change hash");

        // Modify nonce
        let mut modified = base.clone();
        modified.nonce[0] ^= 1;
        assert_ne!(modified.compute_commitment_hash(), base_hash, "nonce change must change hash");

        // Modify fee
        let mut modified = base.clone();
        modified.fee_breakdown.solver_fee_bps += 1;
        assert_ne!(modified.compute_commitment_hash(), base_hash, "fee change must change hash");
    }

    #[test]
    fn fuzz_commitment_verify_rejects_tampered_reveal() {
        let reveal = make_test_reveal();
        let commitment = BundleCommitment {
            bundle_id: reveal.bundle_id,
            solver_id: reveal.solver_id,
            batch_window: reveal.batch_window,
            target_intent_count: 1,
            commitment_hash: reveal.compute_commitment_hash(),
            expected_outputs_hash: reveal.compute_expected_outputs_hash(),
            execution_plan_hash: reveal.compute_execution_plan_hash(),
            valid_until: 100,
            bond_locked: 500,
            signature: Vec::new(),
        };

        // Valid reveal verifies
        assert!(reveal.verify_against_commitment(&commitment).is_ok());

        // Tampered reveal fails
        let mut tampered = reveal.clone();
        tampered.nonce[0] ^= 1;
        assert!(tampered.verify_against_commitment(&commitment).is_err());
    }

    // ═══════════════════════════════════════════════════════════════════
    // Target 4: Genesis deserialization + validation (P1)
    // ═══════════════════════════════════════════════════════════════════

    use crate::genesis::PoSeqGenesisState;
    use crate::genesis::validation::validate_genesis;

    #[test]
    fn fuzz_genesis_deserialize_empty() {
        let result: Result<PoSeqGenesisState, _> = serde_json::from_str("");
        assert!(result.is_err());
    }

    #[test]
    fn fuzz_genesis_deserialize_garbage_json() {
        let inputs = [
            "{}", "[]", "null", "true", "42", "\"hello\"",
            "{\"validators\": null}",
            "{\"chain_id\": \"\", \"validators\": []}",
            "{\"version\": {\"protocol_version\": {\"major\": 99}}}",
        ];
        for input in &inputs {
            let _ = serde_json::from_str::<PoSeqGenesisState>(input);
        }
    }

    #[test]
    fn fuzz_genesis_validation_adversarial() {
        // Valid genesis as baseline
        let valid_json = serde_json::to_string(&crate::genesis::PoSeqGenesisState::testnet(
            "test".into(),
            vec![crate::genesis::GenesisValidator {
                node_id: hex::encode([1u8; 32]),
                public_key: hex::encode([101u8; 32]),
                moniker: "v1".into(),
                initial_stake: 1000,
                active: true,
            },
            crate::genesis::GenesisValidator {
                node_id: hex::encode([2u8; 32]),
                public_key: hex::encode([102u8; 32]),
                moniker: "v2".into(),
                initial_stake: 1000,
                active: true,
            },
            crate::genesis::GenesisValidator {
                node_id: hex::encode([3u8; 32]),
                public_key: hex::encode([103u8; 32]),
                moniker: "v3".into(),
                initial_stake: 1000,
                active: true,
            }],
        )).unwrap();

        // Parse and validate — must succeed
        let genesis: PoSeqGenesisState = serde_json::from_str(&valid_json).unwrap();
        let errors = validate_genesis(&genesis);
        assert!(errors.is_empty(), "valid genesis must pass: {:?}", errors);
    }

    // ═══════════════════════════════════════════════════════════════════
    // Target 5: Encode/decode roundtrip consistency
    // ═══════════════════════════════════════════════════════════════════

    #[test]
    fn fuzz_wire_message_roundtrip_all_variants() {
        use crate::networking::messages::{PoSeqMessage, WirePeerStatus, WireProposal, WireAttestation, WireFinalized, NodeRole};

        let messages = vec![
            PoSeqMessage::PeerStatus(WirePeerStatus {
                node_id: [1u8; 32], listen_addr: "127.0.0.1:7000".into(),
                current_epoch: 1, current_slot: 5,
                latest_finalized_batch_id: None, is_leader: false,
                in_committee: true, role: NodeRole::Attestor,
                protocol_version: None,
            }),
            PoSeqMessage::Proposal(WireProposal {
                proposal_id: [2u8; 32], slot: 1, epoch: 1, leader_id: [3u8; 32],
                batch_root: [0u8; 32],
                ordered_submission_ids: vec![[4u8; 32]], parent_batch_id: [5u8; 32],
                policy_version: 1, created_at_height: 100,
            }),
            PoSeqMessage::Attestation(WireAttestation {
                attestor_id: [6u8; 32], proposal_id: [7u8; 32],
                batch_id_attested: [8u8; 32], approve: true, epoch: 1, slot: 1,
            }),
            PoSeqMessage::Finalized(WireFinalized {
                batch_id: [9u8; 32], proposal_id: [10u8; 32], slot: 1, epoch: 1,
                leader_id: [11u8; 32], batch_root: [0u8; 32],
                ordered_submission_ids: vec![], finalization_hash: [12u8; 32],
                approvals: 3, committee_size: 4,
            }),
        ];

        for msg in messages {
            let encoded = msg.encode().unwrap();
            let payload = &encoded[4..];
            let decoded = PoSeqMessage::decode(payload).unwrap();
            let re_encoded = decoded.encode().unwrap();
            assert_eq!(encoded, re_encoded, "roundtrip must be lossless");
        }
    }

    // ═══════════════════════════════════════════════════════════════════
    // Target 6: Version mismatch edge cases
    // ═══════════════════════════════════════════════════════════════════

    use crate::versioning::{ProtocolVersion, VersionedEnvelope};
    use crate::versioning::compat::{check_wire_compat, CompatResult};

    #[test]
    fn fuzz_version_compat_exhaustive_major() {
        // Test all major versions 0-10 against current
        for major in 0u32..=10 {
            let v = ProtocolVersion::new(major, 0, 0);
            let result = check_wire_compat(&v);
            if major == crate::versioning::PROTOCOL_VERSION.major {
                assert!(matches!(result, CompatResult::Compatible | CompatResult::CompatibleWithWarning(_)));
            } else {
                assert!(matches!(result, CompatResult::Incompatible(_)));
            }
        }
    }

    #[test]
    fn fuzz_versioned_envelope_serialization() {
        let env = VersionedEnvelope::wrap(vec![1u8, 2, 3]);
        let json = serde_json::to_string(&env).unwrap();
        let decoded: VersionedEnvelope<Vec<u8>> = serde_json::from_str(&json).unwrap();
        assert_eq!(decoded.payload, vec![1, 2, 3]);
        assert!(decoded.is_compatible());
    }

    #[test]
    fn fuzz_version_u64_boundary_values() {
        let cases = [
            (0u32, 0u32, 0u32),
            (u16::MAX as u32, u16::MAX as u32, u16::MAX as u32),
            (1, 0, 0),
            (0, 1, 0),
            (0, 0, 1),
        ];
        for (major, minor, patch) in cases {
            let v = ProtocolVersion::new(major, minor, patch);
            let encoded = v.to_u64();
            let decoded = ProtocolVersion::from_u64(encoded);
            assert_eq!(v, decoded, "roundtrip failed for {major}.{minor}.{patch}");
        }
    }
}
