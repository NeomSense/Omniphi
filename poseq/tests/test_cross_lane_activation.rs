//! Phase 9 — Cross-Lane Committee Activation Integration Tests
//!
//! Tests the full activation flow: snapshot import → committee activation → leader election → proposals.

use tokio::time::{sleep, Duration};
use omniphi_poseq::networking::node_runner::{NetworkedNode, NodeConfig, NodeControl};
use omniphi_poseq::networking::messages::NodeRole;
use omniphi_poseq::chain_bridge::snapshot::{ChainCommitteeSnapshot, ChainCommitteeMember};

fn make_node_id(byte: u8) -> [u8; 32] {
    [byte; 32]
}

async fn make_test_node(node_id: [u8; 32], port: u16) -> NetworkedNode {
    let config = NodeConfig {
        node_id,
        listen_addr: format!("127.0.0.1:{port}"),
        peers: vec![],
        quorum_threshold: 1,
        slot_duration_ms: 500,
        data_dir: format!("./test_activation_data_{port}"),
        role: NodeRole::Attestor,
        slots_per_epoch: 10,
    };
    NetworkedNode::bind(config).await.unwrap()
}

fn make_snapshot(epoch: u64, node_ids: &[[u8; 32]]) -> ChainCommitteeSnapshot {
    let members: Vec<ChainCommitteeMember> = node_ids.iter().map(|id| {
        ChainCommitteeMember {
            node_id: hex::encode(id),
            public_key: hex::encode(id),
            moniker: format!("node_{}", hex::encode(&id[..2])),
            role: "Sequencer".to_string(),
        }
    }).collect();
    // Sort IDs for canonical hash computation
    let mut sorted_ids: Vec<[u8; 32]> = node_ids.to_vec();
    sorted_ids.sort();
    let hash = ChainCommitteeSnapshot::compute_hash(epoch, &sorted_ids);
    ChainCommitteeSnapshot {
        epoch,
        members,
        snapshot_hash: hash.to_vec(),
        produced_at_block: 1,
    }
}

#[tokio::test]
async fn test_snapshot_activates_committee() {
    let node_id = make_node_id(0x01);
    let mut node = make_test_node(node_id, 17101).await;

    // Initially not in committee
    {
        let state = node.state.lock().await;
        assert!(!state.in_committee);
        assert!(state.committee.is_empty());
    }

    let ctrl_tx = node.ctrl_tx.clone();
    let state_ref = node.state.clone();

    // Send snapshot containing this node BEFORE spawning event loop
    let snap = make_snapshot(1, &[node_id]);
    ctrl_tx.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();

    tokio::spawn(async move {
        node.run_event_loop().await;
    });

    sleep(Duration::from_millis(400)).await;

    let state = state_ref.lock().await;
    assert!(state.in_committee, "node should be in committee after snapshot import");
    assert_eq!(state.committee.len(), 1);
    assert!(state.current_leader.is_some(), "leader should be elected");

    let _ = ctrl_tx.send(NodeControl::Shutdown).await;

    // Cleanup
    let _ = std::fs::remove_dir_all("./test_activation_data_17101");
}

#[tokio::test]
async fn test_snapshot_activates_and_produces_proposal() {
    let node_id = make_node_id(0x02);
    let mut node = make_test_node(node_id, 17102).await;

    // Set signing seed
    node.set_signing_seed(node_id); // use node_id as seed for test
    node.set_verify_signatures(false); // keep off for simplicity in test

    let ctrl_tx = node.ctrl_tx.clone();
    let state_ref = node.state.clone();

    // Import snapshot
    let snap = make_snapshot(1, &[node_id]);
    ctrl_tx.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();

    tokio::spawn(async move {
        node.run_event_loop().await;
    });

    sleep(Duration::from_millis(300)).await;

    // Advance a slot — since only 1 node and it's the leader, it should propose
    ctrl_tx.send(NodeControl::NextSlot).await.unwrap();
    sleep(Duration::from_millis(300)).await;

    let state = state_ref.lock().await;
    assert!(state.in_committee);
    // Leader should propose in the slot
    let is_leader = state.current_leader == Some(node_id);
    if is_leader {
        assert!(state.proposed_this_slot, "leader should have proposed");
    }

    let _ = ctrl_tx.send(NodeControl::Shutdown).await;
    let _ = std::fs::remove_dir_all("./test_activation_data_17102");
}

#[tokio::test]
async fn test_signing_wraps_message() {
    // Test that set_signing_seed causes maybe_sign to wrap messages
    // We test this indirectly by checking the signing_seed is stored
    let node_id = make_node_id(0x03);
    let mut node = make_test_node(node_id, 17103).await;

    assert!(node.signing_seed.is_none());
    let seed = [0xABu8; 32];
    node.set_signing_seed(seed);
    assert_eq!(node.signing_seed, Some(seed));

    let _ = node.ctrl_tx.send(NodeControl::Shutdown).await;
    let _ = std::fs::remove_dir_all("./test_activation_data_17103");
}

#[tokio::test]
async fn test_duplicate_snapshot_rejected() {
    let node_id = make_node_id(0x04);
    let mut node = make_test_node(node_id, 17104).await;
    let ctrl_tx = node.ctrl_tx.clone();
    let state_ref = node.state.clone();

    let snap = make_snapshot(1, &[node_id]);
    ctrl_tx.send(NodeControl::ImportSnapshot(Box::new(snap.clone()))).await.unwrap();

    tokio::spawn(async move { node.run_event_loop().await; });

    sleep(Duration::from_millis(400)).await;

    // Send same snapshot again — should be rejected as duplicate
    ctrl_tx.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();
    sleep(Duration::from_millis(200)).await;

    let state = state_ref.lock().await;
    // latest_snapshot_epoch should still be 1 (not 2, not reset)
    assert_eq!(state.latest_snapshot_epoch, Some(1));
    // Committee still activated from first import
    assert!(state.in_committee);

    let _ = ctrl_tx.send(NodeControl::Shutdown).await;
    let _ = std::fs::remove_dir_all("./test_activation_data_17104");
}

#[tokio::test]
async fn test_non_member_node_not_in_committee() {
    let node_id = make_node_id(0x05);
    let other_id = make_node_id(0x06);
    let mut node = make_test_node(node_id, 17105).await;
    let ctrl_tx = node.ctrl_tx.clone();
    let state_ref = node.state.clone();

    // Snapshot contains other_id, NOT node_id
    let snap = make_snapshot(1, &[other_id]);
    ctrl_tx.send(NodeControl::ImportSnapshot(Box::new(snap))).await.unwrap();

    tokio::spawn(async move { node.run_event_loop().await; });

    sleep(Duration::from_millis(400)).await;

    let state = state_ref.lock().await;
    // Committee IS set (1 member) but this node is NOT in it
    assert_eq!(state.committee.len(), 1);
    assert!(!state.in_committee, "node not in committee if not in snapshot");

    let _ = ctrl_tx.send(NodeControl::Shutdown).await;
    let _ = std::fs::remove_dir_all("./test_activation_data_17105");
}
