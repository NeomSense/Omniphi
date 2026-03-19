//! Canonical wire-format message types for the PoSeq devnet transport.
//!
//! All messages are length-prefixed (4-byte big-endian u32 length header)
//! followed by `bincode`-serialized `PoSeqMessage`.
//!
//! # Message flow
//!
//! ```text
//! Leader          Attestors           All Nodes
//!   |                |                    |
//!   |-- Proposal --->|                    |
//!   |<-- Attestation-|                    |
//!   |-- Finalized ----------------------->|
//!   |-- BatchSync (on request) ---------->|
//!   |<-- PeerStatus ----------------------|
//!   |<-- SyncRequest --------------------|
//! ```

use serde::{Serialize, Deserialize};

// ─── Peer identity ────────────────────────────────────────────────────────────

/// 32-byte node identity (ed25519 public key hash).
pub type NodeId = [u8; 32];

// ─── Proposal wire type ───────────────────────────────────────────────────────

/// A leader's proposal broadcast to all committee members.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireProposal {
    pub proposal_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: NodeId,
    pub batch_root: [u8; 32],
    pub parent_batch_id: [u8; 32],
    pub ordered_submission_ids: Vec<[u8; 32]>,
    pub policy_version: u32,
    pub created_at_height: u64,
}

// ─── Attestation wire type ────────────────────────────────────────────────────

/// An attestation vote sent by a committee member.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireAttestation {
    pub attestor_id: NodeId,
    pub proposal_id: [u8; 32],
    pub batch_id_attested: [u8; 32],
    /// `true` = approve, `false` = reject.
    pub approve: bool,
    pub epoch: u64,
    pub slot: u64,
}

// ─── Finalization notice ──────────────────────────────────────────────────────

/// Broadcast when a batch reaches quorum and is finalized.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireFinalized {
    pub batch_id: [u8; 32],
    pub proposal_id: [u8; 32],
    pub slot: u64,
    pub epoch: u64,
    pub leader_id: NodeId,
    pub batch_root: [u8; 32],
    pub ordered_submission_ids: Vec<[u8; 32]>,
    /// Number of approvals that reached quorum.
    pub approvals: usize,
    /// Total committee size at finalization.
    pub committee_size: usize,
    pub finalization_hash: [u8; 32],
}

// ─── Batch sync ───────────────────────────────────────────────────────────────

/// Request a specific finalized batch (sent by a node that missed finalization).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireSyncRequest {
    pub requesting_node: NodeId,
    pub batch_id: [u8; 32],
    pub epoch: u64,
}

/// Response to a sync request.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireSyncResponse {
    pub responding_node: NodeId,
    pub batch_id: [u8; 32],
    /// `None` if the responder doesn't have the batch.
    pub batch: Option<WireFinalized>,
}

// ─── Checkpoint sync ──────────────────────────────────────────────────────────

/// Announce availability of a checkpoint (sent after creating one).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireCheckpointAnnounce {
    pub node_id: NodeId,
    pub checkpoint_id: [u8; 32],
    pub epoch: u64,
}

// ─── Peer status ──────────────────────────────────────────────────────────────

/// Sent periodically and on reconnect to advertise node state.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WirePeerStatus {
    pub node_id: NodeId,
    pub listen_addr: String,
    pub current_epoch: u64,
    pub current_slot: u64,
    pub latest_finalized_batch_id: Option<[u8; 32]>,
    pub is_leader: bool,
    pub in_committee: bool,
    pub role: NodeRole,
    /// Protocol version string of the sending node (e.g. "1.0.0").
    /// `None` means the peer is running an older version without this field.
    #[serde(default)]
    pub protocol_version: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum NodeRole {
    Leader,
    Attestor,
    Observer,
}

// ─── Epoch/committee metadata ─────────────────────────────────────────────────

/// Broadcast when committee rotates.  All nodes update their view.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireEpochAnnounce {
    pub epoch: u64,
    pub committee_members: Vec<NodeId>,
    pub leader_id: NodeId,
    pub epoch_seed: [u8; 32],
}

// ─── Bridge ack forwarding ────────────────────────────────────────────────────

/// Forward a runtime ack to all nodes so they update bridge state.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireBridgeAck {
    pub batch_id: [u8; 32],
    pub success: bool,
    pub ack_hash: [u8; 32],
}

// ─── Misbehavior evidence ─────────────────────────────────────────────────────

/// Broadcast when a node observes misbehavior from another.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireMisbehaviorReport {
    pub reporter_id: NodeId,
    pub accused_id: NodeId,
    pub kind: String,
    pub slot: u64,
    pub epoch: u64,
    pub evidence_hash: [u8; 32],
}

// ─── Wire-level authentication ────────────────────────────────────────────────

/// Outer authenticated wrapper for all PoSeq wire messages.
///
/// When wire security is enabled, the transport layer wraps every outbound
/// `PoSeqMessage` in a `WireSignedEnvelope` before sending, and verifies the
/// signature on every inbound message before dispatching to protocol handlers.
///
/// # Protocol
/// 1. Sender serializes the inner `PoSeqMessage` with `bincode` → `inner_bytes`.
/// 2. Sender computes `payload_hash = SHA256("POSEQ_WIRE_V1" ‖ inner_bytes)`.
/// 3. Sender signs `payload_hash` with its Ed25519 key → 64-byte `sig`.
/// 4. The full `WireSignedEnvelope { inner_bytes, sig, signer_id }` is sent.
///
/// # Verification
/// Receiver checks:
/// - `signer_id` is in the known validator set (or skip-verify mode).
/// - `Ed25519::verify(pubkey[signer_id], payload_hash, sig)` succeeds.
/// - Then deserialize `inner_bytes` to `PoSeqMessage` and dispatch.
///
/// # Backward compatibility
/// `PoSeqMessage` is still sent unwrapped in devnet / test mode
/// (`verify_wire_signatures = false` in node config).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WireSignedEnvelope {
    /// `bincode`-serialized `PoSeqMessage`.
    pub inner_bytes: Vec<u8>,
    /// 64-byte Ed25519 signature over SHA256("POSEQ_WIRE_V1" ‖ inner_bytes).
    /// Stored as `Vec<u8>` (exactly 64 bytes) because serde only auto-derives
    /// for arrays up to `[T; 32]`.
    pub sig: Vec<u8>,
    /// 32-byte Ed25519 public key of the sender (= node_id in production).
    pub signer_id: [u8; 32],
}

impl WireSignedEnvelope {
    /// Sign `msg` using `signing_key_bytes` (32-byte Ed25519 secret seed).
    ///
    /// Returns `None` if the key is invalid.
    pub fn sign(msg: &PoSeqMessage, signing_key_bytes: &[u8; 32], signer_id: [u8; 32]) -> Option<Self> {
        use sha2::{Sha256, Digest};
        use ed25519_dalek::SigningKey;
        use ed25519_dalek::Signer as _;

        let inner_bytes = bincode::serialize(msg).ok()?;
        let mut h = Sha256::new();
        h.update(b"POSEQ_WIRE_V1");
        h.update(&inner_bytes);
        let hash: [u8; 32] = h.finalize().into();

        let sk = SigningKey::from_bytes(signing_key_bytes);
        let signature = sk.sign(&hash);
        let sig = signature.to_bytes().to_vec();

        Some(WireSignedEnvelope { inner_bytes, sig, signer_id })
    }

    /// Verify the signature and deserialize the inner message.
    ///
    /// `pubkey_bytes` must be the 32-byte Ed25519 public key of `signer_id`.
    /// Returns `None` on signature failure or deserialization error.
    pub fn verify_and_decode(&self, pubkey_bytes: &[u8; 32]) -> Option<PoSeqMessage> {
        use sha2::{Sha256, Digest};
        use ed25519_dalek::{VerifyingKey, Signature, Verifier as _};

        let mut h = Sha256::new();
        h.update(b"POSEQ_WIRE_V1");
        h.update(&self.inner_bytes);
        let hash: [u8; 32] = h.finalize().into();

        let vk = VerifyingKey::from_bytes(pubkey_bytes).ok()?;
        let sig_bytes: [u8; 64] = self.sig.as_slice().try_into().ok()?;
        let signature = Signature::from_bytes(&sig_bytes);
        vk.verify(&hash, &signature).ok()?;

        bincode::deserialize(&self.inner_bytes).ok()
    }

    /// Decode the inner message WITHOUT signature verification.
    /// Use only in test mode or devnets.
    pub fn decode_unverified(&self) -> Option<PoSeqMessage> {
        bincode::deserialize(&self.inner_bytes).ok()
    }
}

// ─── Top-level message envelope ───────────────────────────────────────────────

/// All PoSeq wire messages.  Serialized to bytes with `bincode` for transport.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum PoSeqMessage {
    /// Leader → all: new batch proposal.
    Proposal(WireProposal),
    /// Attestor → all (or leader): attestation vote.
    Attestation(WireAttestation),
    /// Leader → all: quorum reached, batch finalized.
    Finalized(WireFinalized),
    /// Late node → peer: I need this batch.
    SyncRequest(WireSyncRequest),
    /// Peer → late node: here is the batch.
    SyncResponse(WireSyncResponse),
    /// Node → all: status heartbeat / rejoin announcement.
    PeerStatus(WirePeerStatus),
    /// Leader → all: new epoch and committee.
    EpochAnnounce(WireEpochAnnounce),
    /// Any → all: runtime acknowledged batch.
    BridgeAck(WireBridgeAck),
    /// Any → all: misbehavior evidence.
    MisbehaviorReport(WireMisbehaviorReport),
    /// Any → peer: checkpoint available.
    CheckpointAnnounce(WireCheckpointAnnounce),
    /// Authenticated wrapper around any other message.
    /// Used when `verify_signatures = true` in node config.
    Signed(WireSignedEnvelope),
    /// Peer discovery: list of known peers sent in response to a PeerStatus.
    PeerList(crate::networking::discovery::WirePeerList),

    // ── HotStuff BFT messages ──────────────────────────────────────────────

    /// HotStuff block proposal from the current leader.
    HotStuffBlock(crate::hotstuff::HotStuffBlock),
    /// HotStuff vote (PREPARE / PRE-COMMIT / COMMIT) from a validator.
    HotStuffVote(crate::hotstuff::HotStuffVote),
    /// HotStuff quorum certificate broadcast by the leader.
    HotStuffQC(crate::hotstuff::QuorumCertificate),
    /// Pacemaker NewView message (view timeout notification).
    HotStuffNewView(crate::hotstuff::NewViewMessage),

    // ── Sequencer registry messages ───────────────────────────────────────

    /// Broadcast by a newly registered sequencer after on-chain tx confirmed.
    /// Peers apply this to their local `SequencerRegistry`.
    SequencerRegistered(crate::identities::registry::WireSequencerRegistered),
}

impl PoSeqMessage {
    pub fn kind(&self) -> &'static str {
        match self {
            PoSeqMessage::Proposal(_) => "Proposal",
            PoSeqMessage::Attestation(_) => "Attestation",
            PoSeqMessage::Finalized(_) => "Finalized",
            PoSeqMessage::SyncRequest(_) => "SyncRequest",
            PoSeqMessage::SyncResponse(_) => "SyncResponse",
            PoSeqMessage::PeerStatus(_) => "PeerStatus",
            PoSeqMessage::EpochAnnounce(_) => "EpochAnnounce",
            PoSeqMessage::BridgeAck(_) => "BridgeAck",
            PoSeqMessage::MisbehaviorReport(_) => "MisbehaviorReport",
            PoSeqMessage::CheckpointAnnounce(_) => "CheckpointAnnounce",
            PoSeqMessage::Signed(_) => "Signed",
            PoSeqMessage::PeerList(_) => "PeerList",
            PoSeqMessage::HotStuffBlock(_) => "HotStuffBlock",
            PoSeqMessage::HotStuffVote(_) => "HotStuffVote",
            PoSeqMessage::HotStuffQC(_) => "HotStuffQC",
            PoSeqMessage::HotStuffNewView(_) => "HotStuffNewView",
            PoSeqMessage::SequencerRegistered(_) => "SequencerRegistered",
        }
    }

    /// Encode to length-prefixed bytes for TCP framing.
    pub fn encode(&self) -> Result<Vec<u8>, bincode::Error> {
        let payload = bincode::serialize(self)?;
        let len = payload.len() as u32;
        let mut out = len.to_be_bytes().to_vec();
        out.extend_from_slice(&payload);
        Ok(out)
    }

    /// Decode from a raw payload slice (without the 4-byte length prefix).
    pub fn decode(bytes: &[u8]) -> Result<Self, bincode::Error> {
        bincode::deserialize(bytes)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn node_id(b: u8) -> NodeId { let mut id = [0u8; 32]; id[0] = b; id }

    #[test]
    fn test_proposal_encode_decode_roundtrip() {
        let msg = PoSeqMessage::Proposal(WireProposal {
            proposal_id: node_id(1),
            slot: 5,
            epoch: 1,
            leader_id: node_id(2),
            batch_root: node_id(3),
            parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![node_id(10), node_id(11)],
            policy_version: 1,
            created_at_height: 100,
        });
        let encoded = msg.encode().unwrap();
        // First 4 bytes are length
        let len = u32::from_be_bytes(encoded[..4].try_into().unwrap()) as usize;
        assert_eq!(len, encoded.len() - 4);
        let decoded = PoSeqMessage::decode(&encoded[4..]).unwrap();
        assert_eq!(decoded.kind(), "Proposal");
    }

    #[test]
    fn test_attestation_encode_decode_roundtrip() {
        let msg = PoSeqMessage::Attestation(WireAttestation {
            attestor_id: node_id(5),
            proposal_id: node_id(1),
            batch_id_attested: node_id(2),
            approve: true,
            epoch: 1,
            slot: 3,
        });
        let encoded = msg.encode().unwrap();
        let decoded = PoSeqMessage::decode(&encoded[4..]).unwrap();
        assert_eq!(decoded.kind(), "Attestation");
    }

    #[test]
    fn test_finalized_encode_decode_roundtrip() {
        let msg = PoSeqMessage::Finalized(WireFinalized {
            batch_id: node_id(10),
            proposal_id: node_id(1),
            slot: 5,
            epoch: 2,
            leader_id: node_id(2),
            batch_root: node_id(3),
            ordered_submission_ids: vec![],
            approvals: 3,
            committee_size: 5,
            finalization_hash: node_id(99),
        });
        let encoded = msg.encode().unwrap();
        let decoded = PoSeqMessage::decode(&encoded[4..]).unwrap();
        assert_eq!(decoded.kind(), "Finalized");
    }

    #[test]
    fn test_peer_status_encode_decode() {
        let msg = PoSeqMessage::PeerStatus(WirePeerStatus {
            node_id: node_id(1),
            listen_addr: "127.0.0.1:7001".to_string(),
            current_epoch: 3,
            current_slot: 15,
            latest_finalized_batch_id: Some(node_id(99)),
            is_leader: false,
            in_committee: true,
            role: NodeRole::Attestor,
            protocol_version: None,
        });
        let encoded = msg.encode().unwrap();
        let decoded = PoSeqMessage::decode(&encoded[4..]).unwrap();
        assert_eq!(decoded.kind(), "PeerStatus");
    }

    #[test]
    fn test_all_message_kinds_round_trip() {
        let msgs: Vec<PoSeqMessage> = vec![
            PoSeqMessage::SyncRequest(WireSyncRequest { requesting_node: node_id(1), batch_id: node_id(2), epoch: 1 }),
            PoSeqMessage::SyncResponse(WireSyncResponse { responding_node: node_id(1), batch_id: node_id(2), batch: None }),
            PoSeqMessage::EpochAnnounce(WireEpochAnnounce { epoch: 1, committee_members: vec![], leader_id: node_id(1), epoch_seed: [0u8; 32] }),
            PoSeqMessage::BridgeAck(WireBridgeAck { batch_id: node_id(1), success: true, ack_hash: node_id(2) }),
            PoSeqMessage::MisbehaviorReport(WireMisbehaviorReport { reporter_id: node_id(1), accused_id: node_id(2), kind: "DualProposal".into(), slot: 1, epoch: 1, evidence_hash: node_id(3) }),
            PoSeqMessage::CheckpointAnnounce(WireCheckpointAnnounce { node_id: node_id(1), checkpoint_id: node_id(2), epoch: 5 }),
        ];
        for msg in msgs {
            let kind = msg.kind();
            let enc = msg.encode().unwrap();
            let dec = PoSeqMessage::decode(&enc[4..]).unwrap();
            assert_eq!(dec.kind(), kind, "roundtrip failed for {kind}");
        }
    }

    #[test]
    fn test_encode_has_4_byte_length_prefix() {
        let msg = PoSeqMessage::BridgeAck(WireBridgeAck { batch_id: [0u8; 32], success: true, ack_hash: [0u8; 32] });
        let enc = msg.encode().unwrap();
        assert!(enc.len() > 4);
        let declared_len = u32::from_be_bytes(enc[..4].try_into().unwrap()) as usize;
        assert_eq!(declared_len + 4, enc.len());
    }
}
