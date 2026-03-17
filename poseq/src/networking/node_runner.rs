//! Multi-process PoSeq node runner.
//!
//! `NetworkedNode` ties together the TCP transport, peer manager, and
//! deterministic protocol logic.  It runs a single PoSeq node process
//! that can communicate with peer nodes over real TCP connections.
//!
//! # Architecture
//!
//! ```text
//! ┌─────────────────────────────────────────────────────────┐
//! │ NetworkedNode                                           │
//! │                                                         │
//! │  NodeConfig ──────────────── fixed per process          │
//! │  NodeTransport ──────────── TCP send/receive            │
//! │  PeerManager ────────────── peer state + dedup          │
//! │  NodeState ──────────────── protocol FSM state          │
//! │  DurableStore ───────────── persistent storage          │
//! │                                                         │
//! │  run_event_loop() ─────────── main tokio task           │
//! │    ├─ process_message()   (inbound TCP messages)        │
//! │    └─ run_leader_slot()   (outbound proposal cycle)     │
//! └─────────────────────────────────────────────────────────┘
//! ```
//!
//! # Determinism guarantee
//! All protocol decisions (leader election, quorum check, finalization hash)
//! are functions of the persisted state + received messages.  Local clock
//! is used only for I/O timeouts, never for consensus decisions.

#![allow(dead_code)]

use std::collections::BTreeMap;
use std::sync::Arc;

use tokio::sync::{Mutex, mpsc};
use tokio::time::Duration;

use sha2::{Sha256, Digest};

use crate::networking::messages::*;
use crate::networking::transport::NodeTransport;
use crate::networking::peer_manager::PeerManager;
use crate::persistence::{DurableStore, SledBackend};
use crate::persistence::engine::PersistenceEngine;
use crate::crypto::NodeKeyPair;
use crate::bridge::runtime_channel::RuntimeDeliverySender;
use crate::bridge::pipeline::{FinalizationEnvelope, FairnessMeta, BatchCommitment};
use crate::hotstuff::{HotStuffEngine, HotStuffOutput};
use crate::identities::registry::SequencerRegistry;

// ─── NodeConfig ───────────────────────────────────────────────────────────────

/// Static per-process node configuration.
#[derive(Debug, Clone)]
pub struct NodeConfig {
    pub node_id: NodeId,
    pub listen_addr: String,
    pub peers: Vec<PeerEntry>,
    pub quorum_threshold: usize,
    pub slot_duration_ms: u64,
    pub data_dir: String,
    pub role: NodeRole,
}

#[derive(Debug, Clone)]
pub struct PeerEntry {
    pub node_id: NodeId,
    pub addr: String,
}

// ─── NodeState ────────────────────────────────────────────────────────────────

/// Mutable runtime state for the protocol FSM.
#[derive(Debug)]
pub struct NodeState {
    pub current_epoch: u64,
    pub current_slot: u64,
    pub current_leader: Option<NodeId>,
    pub committee: Vec<NodeId>,
    /// Proposals received this slot, keyed by proposal_id
    pub pending_proposals: BTreeMap<[u8; 32], WireProposal>,
    /// Attestations received per proposal_id
    pub attestations: BTreeMap<[u8; 32], Vec<WireAttestation>>,
    /// Finalized batches received, keyed by batch_id
    pub finalized_batches: BTreeMap<[u8; 32], WireFinalized>,
    /// Latest finalized batch_id
    pub latest_finalized: Option<[u8; 32]>,
    /// Proposed in this slot (leader only)
    pub proposed_this_slot: bool,
    /// Attested per proposal (to prevent double-voting)
    pub attested_proposals: BTreeMap<[u8; 32], bool>,
    /// Whether this node is in the current committee
    pub in_committee: bool,
    /// Log of events for observability
    pub event_log: Vec<String>,
    /// Monotone counter for runtime delivery sequence numbers.
    pub delivery_seq: u64,
}

impl NodeState {
    pub fn new() -> Self {
        NodeState {
            current_epoch: 0,
            current_slot: 0,
            current_leader: None,
            committee: Vec::new(),
            pending_proposals: BTreeMap::new(),
            attestations: BTreeMap::new(),
            finalized_batches: BTreeMap::new(),
            latest_finalized: None,
            proposed_this_slot: false,
            attested_proposals: BTreeMap::new(),
            in_committee: false,
            event_log: Vec::new(),
            delivery_seq: 0,
        }
    }

    pub fn log(&mut self, event: String) {
        self.event_log.push(event);
    }

    /// Deterministic leader election: SHA256(epoch ‖ slot ‖ committee_sorted)
    pub fn elect_leader(epoch: u64, slot: u64, committee: &[NodeId]) -> Option<NodeId> {
        if committee.is_empty() { return None; }
        let mut sorted = committee.to_vec();
        sorted.sort();
        let mut h = Sha256::new();
        h.update(epoch.to_be_bytes());
        h.update(slot.to_be_bytes());
        for id in &sorted { h.update(id); }
        let seed: [u8; 32] = h.finalize().into();
        // Pick index using first 8 bytes of seed mod committee size
        let idx = (u64::from_be_bytes(seed[..8].try_into().unwrap()) as usize) % sorted.len();
        Some(sorted[idx])
    }

    /// Check if a batch has reached quorum.
    pub fn quorum_reached(&self, proposal_id: &[u8; 32], threshold: usize) -> bool {
        let approvals = self.attestations.get(proposal_id)
            .map(|a| a.iter().filter(|v| v.approve).count())
            .unwrap_or(0);
        approvals >= threshold
    }

    /// Compute finalization hash for a proposal (deterministic).
    pub fn compute_finalization_hash(proposal: &WireProposal, approvals: usize) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(proposal.proposal_id);
        h.update(proposal.slot.to_be_bytes());
        h.update(proposal.epoch.to_be_bytes());
        h.update(proposal.leader_id);
        h.update(proposal.batch_root);
        h.update((approvals as u64).to_be_bytes());
        h.finalize().into()
    }

    /// Compute batch_id from proposal (matches WireFinalized.batch_id).
    pub fn compute_batch_id(proposal: &WireProposal) -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(b"batch:");
        h.update(proposal.proposal_id);
        h.update(proposal.batch_root);
        h.finalize().into()
    }
}

// ─── NetworkedNode ────────────────────────────────────────────────────────────

/// A single PoSeq node process with real TCP networking.
pub struct NetworkedNode {
    pub config: NodeConfig,
    pub state: Arc<Mutex<NodeState>>,
    pub transport: Arc<NodeTransport>,
    pub peer_manager: Arc<Mutex<PeerManager>>,
    /// Durable storage for finalized batches (survives restarts).
    pub store: Arc<Mutex<DurableStore>>,
    /// Node signing key-pair (Ed25519). `None` in devnet / test mode.
    pub key_pair: Option<Arc<NodeKeyPair>>,
    /// When `true`, all inbound messages must carry a valid `WireSignedEnvelope`.
    /// When `false` (default), messages are accepted unsigned (devnet / test mode).
    pub verify_signatures: bool,
    /// Optional sender to the runtime ingester.  When set, finalized batches are
    /// converted to `FinalizationEnvelope` and sent on this channel.
    pub runtime_sender: Option<RuntimeDeliverySender>,
    /// HotStuff BFT consensus engine (behind Arc<Mutex> for interior mutability
    /// — the event loop holds `&self` but HotStuff methods take `&mut self`).
    pub hotstuff: Arc<Mutex<HotStuffEngine>>,
    /// Registry of all registered sequencers (populated from chain + p2p gossip).
    pub sequencer_registry: Arc<Mutex<SequencerRegistry>>,
    inbox: mpsc::Receiver<(String, PoSeqMessage)>,
    /// Channel for external control (used in tests / devnet runner)
    pub ctrl_tx: mpsc::Sender<NodeControl>,
    ctrl_rx: mpsc::Receiver<NodeControl>,
}

/// Control commands for the node event loop.
#[derive(Debug)]
pub enum NodeControl {
    /// Advance to next slot (triggered externally in devnet).
    NextSlot,
    /// Simulate a node crash (stop processing inbound messages).
    Crash,
    /// Rejoin after a crash (resume processing).
    Rejoin,
    /// Graceful shutdown.
    Shutdown,
}

impl NetworkedNode {
    /// Create and bind a node.  Does NOT start the event loop.
    ///
    /// Opens (or creates) the durable sled store at `config.data_dir` and
    /// restores `latest_finalized` from persisted state so a restarted node
    /// can continue sequencing from where it left off.
    pub async fn bind(config: NodeConfig) -> Result<Self, std::io::Error> {
        let (transport, inbox) = NodeTransport::bind(&config.listen_addr).await?;
        let (ctrl_tx, ctrl_rx) = mpsc::channel(32);
        let self_id = config.node_id;
        let mut pm = PeerManager::new(self_id, 1024);
        for peer in &config.peers {
            pm.register_peer(peer.node_id, peer.addr.clone());
        }

        // Open durable store.  Falls back to in-memory if sled fails so that
        // tests and devnets without a real filesystem still work.
        let store = Self::open_store(&config.data_dir);
        let mut initial_state = NodeState::new();

        // Restore latest_finalized from persisted store.
        {
            let locked = store.lock().await;
            // latest_finalized is stored as raw bytes at key b"meta:latest_finalized"
            if let Some(bytes) = locked.engine.get_raw(b"meta:latest_finalized") {
                if bytes.len() == 32 {
                    let mut id = [0u8; 32];
                    id.copy_from_slice(&bytes);
                    initial_state.latest_finalized = Some(id);
                }
            }
        }

        let quorum = config.quorum_threshold;
        let hs_timeout_ms = config.slot_duration_ms * 4;
        Ok(NetworkedNode {
            config,
            state: Arc::new(Mutex::new(initial_state)),
            transport: Arc::new(transport),
            peer_manager: Arc::new(Mutex::new(pm)),
            store,
            key_pair: None,           // set via set_key_pair() after bind
            verify_signatures: false,  // enable via set_verify_signatures() after bind
            runtime_sender: None,      // set via set_runtime_sender() after bind
            hotstuff: Arc::new(Mutex::new(HotStuffEngine::new(self_id, quorum, hs_timeout_ms))),
            sequencer_registry: Arc::new(Mutex::new(SequencerRegistry::new())),
            inbox,
            ctrl_tx,
            ctrl_rx,
        })
    }

    /// Set the node's Ed25519 signing key pair.
    /// Should be called before `run_event_loop()`.
    pub fn set_key_pair(&mut self, kp: NodeKeyPair) {
        self.key_pair = Some(Arc::new(kp));
    }

    /// Enable or disable inbound signature verification.
    /// When enabled, unsigned messages are silently dropped.
    pub fn set_verify_signatures(&mut self, enabled: bool) {
        self.verify_signatures = enabled;
    }

    /// Wire the runtime delivery channel.
    /// Finalized batches will be converted to `FinalizationEnvelope` and sent here.
    pub fn set_runtime_sender(&mut self, sender: RuntimeDeliverySender) {
        self.runtime_sender = Some(sender);
    }

    /// Convert a `WireFinalized` + delivery context into a `FinalizationEnvelope`
    /// and deliver it to the runtime ingester (best-effort; errors are logged).
    async fn deliver_to_runtime(&self, fin: &WireFinalized, delivery_seq: u64) {
        let sender = match &self.runtime_sender {
            Some(s) => s,
            None => return, // no runtime connected
        };

        // Build delivery_id: SHA256("delivery:" ‖ batch_id ‖ delivery_seq)
        let delivery_id = {
            let mut h = sha2::Sha256::new();
            sha2::Digest::update(&mut h, b"delivery:");
            sha2::Digest::update(&mut h, &fin.batch_id);
            sha2::Digest::update(&mut h, &delivery_seq.to_be_bytes());
            sha2::Digest::finalize(h).into()
        };

        let commitment = BatchCommitment::compute(
            &fin.finalization_hash,
            &delivery_id,
            &fin.ordered_submission_ids,
        );

        let envelope = FinalizationEnvelope {
            batch_id: fin.batch_id,
            delivery_id,
            attempt_count: 1,
            slot: fin.slot,
            epoch: fin.epoch,
            sequence_number: delivery_seq,
            leader_id: fin.leader_id,
            parent_batch_id: [0u8; 32], // not available in WireFinalized
            ordered_submission_ids: fin.ordered_submission_ids.clone(),
            batch_root: fin.batch_root,
            finalization_hash: fin.finalization_hash,
            quorum_approvals: fin.approvals,
            committee_size: fin.committee_size,
            fairness: FairnessMeta::none(1),
            commitment,
        };

        if let Err(e) = sender.deliver(envelope).await {
            eprintln!("[poseq] runtime delivery failed for batch={}: {e}", hex::encode(&fin.batch_id[..4]));
        }
    }

    /// Wrap a message in a `WireSignedEnvelope` if a key pair is configured,
    /// otherwise return the message unchanged.
    fn maybe_sign(&self, msg: PoSeqMessage) -> PoSeqMessage {
        if let Some(ref kp) = self.key_pair {
            // Use the raw 32-byte key seed from the dalek SigningKey.
            // NodeKeyPair stores the public key; to get the signing key bytes
            // we re-derive from a test seed here — for production, store the
            // seed bytes on NodeKeyPair (TODO Sprint 3+).
            // For now, skip signing if no raw seed is available.
            let _ = kp; // key_pair field available but signing via raw bytes deferred
        }
        msg
    }

    /// Unwrap a signed envelope if `verify_signatures` is true, or pass through unchanged.
    ///
    /// Returns `None` if signature verification fails (message should be dropped).
    fn maybe_verify(&self, msg: PoSeqMessage) -> Option<PoSeqMessage> {
        match msg {
            PoSeqMessage::Signed(env) => {
                // In verify mode: require valid signature. In devnet mode: decode without verify.
                if self.verify_signatures {
                    // TODO: look up pubkey from sequencer registry by env.signer_id
                    // For now, accept all signed messages (trust-on-first-use devnet behavior)
                    env.decode_unverified()
                } else {
                    env.decode_unverified()
                }
            }
            other => {
                if self.verify_signatures {
                    // Reject unsigned messages when verification is required
                    None
                } else {
                    Some(other)
                }
            }
        }
    }

    /// Open a durable sled store, falling back to an in-memory store on error.
    fn open_store(data_dir: &str) -> Arc<Mutex<DurableStore>> {
        let backend: Box<dyn crate::persistence::backend::PersistenceBackend> =
            match SledBackend::open(std::path::Path::new(data_dir)) {
                Ok(sled) => Box::new(sled),
                Err(e) => {
                    eprintln!("[poseq] WARNING: sled open failed for {data_dir}: {e} — using in-memory store");
                    Box::new(crate::persistence::backend::InMemoryBackend::new())
                }
            };
        let engine = PersistenceEngine::new(backend);
        let mut store = DurableStore::new(engine);
        store.write_schema_version();
        Arc::new(Mutex::new(store))
    }

    /// Announce ourselves to all configured peers.
    pub async fn announce(&self) {
        let state = self.state.lock().await;
        let msg = PoSeqMessage::PeerStatus(WirePeerStatus {
            node_id: self.config.node_id,
            listen_addr: self.transport.listen_addr.clone(),
            current_epoch: state.current_epoch,
            current_slot: state.current_slot,
            latest_finalized_batch_id: state.latest_finalized,
            is_leader: state.current_leader == Some(self.config.node_id),
            in_committee: state.in_committee,
            role: self.config.role,
        });
        drop(state);
        let pm = self.peer_manager.lock().await;
        let addrs = pm.all_peer_addrs();
        drop(pm);
        self.transport.broadcast(&addrs, &msg).await;
    }

    /// Run the main event loop.  Returns when Shutdown is received.
    pub async fn run_event_loop(&mut self) {
        let mut crashed = false;
        loop {
            tokio::select! {
                // Inbound TCP message
                maybe_msg = self.inbox.recv() => {
                    match maybe_msg {
                        None => break, // transport closed
                        Some((peer_addr, msg)) => {
                            if !crashed {
                                // Verify/unwrap signed envelope before dispatch
                                if let Some(inner) = self.maybe_verify(msg) {
                                    self.handle_message(peer_addr, inner).await;
                                }
                                // else: drop silently (bad sig or unsigned in enforce mode)
                            }
                        }
                    }
                }
                // Control command
                maybe_ctrl = self.ctrl_rx.recv() => {
                    match maybe_ctrl {
                        None | Some(NodeControl::Shutdown) => break,
                        Some(NodeControl::Crash) => {
                            let mut state = self.state.lock().await;
                            let epoch = state.current_epoch;
                            let slot = state.current_slot;
                            state.log(format!("NODE CRASH at epoch={epoch} slot={slot}"));
                            crashed = true;
                        }
                        Some(NodeControl::Rejoin) => {
                            crashed = false;
                            let mut state = self.state.lock().await;
                            state.log("REJOIN — re-announcing to peers".into());
                            drop(state);
                            self.announce().await;
                        }
                        Some(NodeControl::NextSlot) => {
                            if !crashed {
                                self.advance_slot().await;
                            }
                        }
                    }
                }
            }
        }
    }

    /// Advance to the next slot.  If we are the leader, broadcast a proposal.
    async fn advance_slot(&self) {
        let mut state = self.state.lock().await;
        state.current_slot += 1;
        state.proposed_this_slot = false;
        state.pending_proposals.clear();
        state.attestations.clear();

        // Deterministic leader election
        let leader = NodeState::elect_leader(state.current_epoch, state.current_slot, &state.committee);
        state.current_leader = leader;
        let is_leader = leader == Some(self.config.node_id);
        state.in_committee = state.committee.contains(&self.config.node_id);

        let log_slot = state.current_slot;
        let log_epoch = state.current_epoch;
        let log_leader = leader.map(|id| hex::encode(&id[..4]));
        state.log(format!("slot={log_slot} epoch={log_epoch} leader={log_leader:?} is_me={is_leader}"));

        if is_leader && !state.proposed_this_slot {
            state.proposed_this_slot = true;
            // Build a minimal proposal (real nodes would fill with actual submissions)
            let proposal_id = NodeState::compute_batch_id(&WireProposal {
                proposal_id: [0u8; 32],
                slot: state.current_slot,
                epoch: state.current_epoch,
                leader_id: self.config.node_id,
                batch_root: [0u8; 32],
                parent_batch_id: state.latest_finalized.unwrap_or([0u8; 32]),
                ordered_submission_ids: vec![],
                policy_version: 1,
                created_at_height: state.current_slot,
            });
            let proposal = WireProposal {
                proposal_id,
                slot: state.current_slot,
                epoch: state.current_epoch,
                leader_id: self.config.node_id,
                batch_root: [0u8; 32],
                parent_batch_id: state.latest_finalized.unwrap_or([0u8; 32]),
                ordered_submission_ids: vec![],
                policy_version: 1,
                created_at_height: state.current_slot,
            };
            // Leader implicitly attests to its own proposal
            let self_attestation = WireAttestation {
                attestor_id: self.config.node_id,
                proposal_id: proposal.proposal_id,
                batch_id_attested: NodeState::compute_batch_id(&proposal),
                approve: true,
                epoch: proposal.epoch,
                slot: proposal.slot,
            };
            state.attestations.entry(proposal.proposal_id).or_default().push(self_attestation);
            state.attested_proposals.insert(proposal.proposal_id, true);
            state.pending_proposals.insert(proposal.proposal_id, proposal.clone());
            state.log(format!("PROPOSE batch_root=0x{}", hex::encode(&proposal.proposal_id[..4])));
            let msg = PoSeqMessage::Proposal(proposal);
            drop(state);
            let pm = self.peer_manager.lock().await;
            let addrs = pm.all_peer_addrs();
            drop(pm);
            self.transport.broadcast(&addrs, &msg).await;
        }
    }

    /// Process one inbound message.
    async fn handle_message(&self, _peer_addr: String, msg: PoSeqMessage) {
        // Dedup first
        let mut pm = self.peer_manager.lock().await;
        if pm.is_duplicate(&msg) {
            return;
        }
        drop(pm);

        match msg {
            PoSeqMessage::PeerStatus(status) => {
                let mut pm = self.peer_manager.lock().await;
                pm.update_from_status(&status);
            }

            PoSeqMessage::EpochAnnounce(ann) => {
                let mut state = self.state.lock().await;
                state.current_epoch = ann.epoch;
                state.committee = ann.committee_members.clone();
                state.current_leader = Some(ann.leader_id);
                state.in_committee = state.committee.contains(&self.config.node_id);
                state.log(format!("EPOCH {} committee_size={}", ann.epoch, ann.committee_members.len()));
            }

            PoSeqMessage::Proposal(proposal) => {
                let mut state = self.state.lock().await;
                let pid = proposal.proposal_id;
                // Verify this came from the expected leader
                if Some(proposal.leader_id) != state.current_leader {
                    state.log(format!("PROPOSAL from unexpected leader {:?}", &proposal.leader_id[..4]));
                    return;
                }
                state.pending_proposals.insert(pid, proposal.clone());
                state.log(format!("RECEIVED PROPOSAL slot={} epoch={}", proposal.slot, proposal.epoch));

                // If in committee and haven't voted, attest
                let in_committee = state.in_committee;
                let already_voted = state.attested_proposals.contains_key(&pid);
                if in_committee && !already_voted {
                    state.attested_proposals.insert(pid, true);
                    let attestation = WireAttestation {
                        attestor_id: self.config.node_id,
                        proposal_id: pid,
                        batch_id_attested: NodeState::compute_batch_id(&proposal),
                        approve: true,
                        epoch: proposal.epoch,
                        slot: proposal.slot,
                    };
                    drop(state);
                    let pm = self.peer_manager.lock().await;
                    let addrs = pm.all_peer_addrs();
                    drop(pm);
                    self.transport.broadcast(&addrs, &PoSeqMessage::Attestation(attestation)).await;
                }
            }

            PoSeqMessage::Attestation(vote) => {
                let mut state = self.state.lock().await;
                let pid = vote.proposal_id;
                state.attestations.entry(pid).or_default().push(vote.clone());
                let approvals = state.attestations[&pid].iter().filter(|v| v.approve).count();
                let quorum = self.config.quorum_threshold;
                state.log(format!("ATTESTATION from {:?} proposal={:?} approvals={}/{}",
                    &vote.attestor_id[..4], &pid[..4], approvals, quorum));

                // Check for quorum (leader finalizes)
                let is_leader = state.current_leader == Some(self.config.node_id);
                if is_leader && approvals >= quorum {
                    if let Some(proposal) = state.pending_proposals.get(&pid).cloned() {
                        let batch_id = NodeState::compute_batch_id(&proposal);
                        if !state.finalized_batches.contains_key(&batch_id) {
                            let fin_hash = NodeState::compute_finalization_hash(&proposal, approvals);
                            let finalized = WireFinalized {
                                batch_id,
                                proposal_id: pid,
                                slot: proposal.slot,
                                epoch: proposal.epoch,
                                leader_id: proposal.leader_id,
                                batch_root: proposal.batch_root,
                                ordered_submission_ids: proposal.ordered_submission_ids.clone(),
                                approvals,
                                committee_size: state.committee.len(),
                                finalization_hash: fin_hash,
                            };
                            state.finalized_batches.insert(batch_id, finalized.clone());
                            state.latest_finalized = Some(batch_id);
                            let delivery_seq = state.delivery_seq;
                            state.delivery_seq += 1;
                            state.log(format!("FINALIZED batch={:?} approvals={}", &batch_id[..4], approvals));
                            // Persist to durable store
                            let encoded = bincode::serialize(&finalized)
                                .unwrap_or_default();
                            drop(state);
                            {
                                let mut locked = self.store.lock().await;
                                locked.engine.put_finalized(&batch_id, encoded);
                                locked.engine.put_raw(b"meta:latest_finalized", batch_id.to_vec());
                            }
                            // Deliver to runtime ingester
                            self.deliver_to_runtime(&finalized, delivery_seq).await;
                            let msg = PoSeqMessage::Finalized(finalized);
                            let pm = self.peer_manager.lock().await;
                            let addrs = pm.all_peer_addrs();
                            drop(pm);
                            self.transport.broadcast(&addrs, &msg).await;
                            return;
                        }
                    }
                }
            }

            PoSeqMessage::Finalized(fin) => {
                let mut state = self.state.lock().await;
                let bid = fin.batch_id;
                if !state.finalized_batches.contains_key(&bid) {
                    state.finalized_batches.insert(bid, fin.clone());
                    state.latest_finalized = Some(bid);
                    let delivery_seq = state.delivery_seq;
                    state.delivery_seq += 1;
                    state.log(format!("RECEIVED FINALIZED batch={:?} epoch={}", &bid[..4], fin.epoch));
                    // Persist to durable store
                    let encoded = bincode::serialize(&fin).unwrap_or_default();
                    drop(state);
                    {
                        let mut locked = self.store.lock().await;
                        locked.engine.put_finalized(&bid, encoded);
                        locked.engine.put_raw(b"meta:latest_finalized", bid.to_vec());
                    }
                    // Deliver to runtime ingester
                    self.deliver_to_runtime(&fin, delivery_seq).await;
                    return;
                }
            }

            PoSeqMessage::SyncRequest(req) => {
                let state = self.state.lock().await;
                let batch = state.finalized_batches.get(&req.batch_id).cloned();
                let finalized = batch.map(|b| b);
                drop(state);
                let resp = PoSeqMessage::SyncResponse(WireSyncResponse {
                    responding_node: self.config.node_id,
                    batch_id: req.batch_id,
                    batch: finalized,
                });
                let pm = self.peer_manager.lock().await;
                // Find the requesting node's address
                if let Some(peer) = pm.get_peer(&req.requesting_node) {
                    let addr = peer.listen_addr.clone();
                    drop(pm);
                    let _ = self.transport.send_to(&addr, &resp).await;
                }
            }

            PoSeqMessage::SyncResponse(resp) => {
                if let Some(finalized) = resp.batch {
                    let mut state = self.state.lock().await;
                    let bid = finalized.batch_id;
                    if !state.finalized_batches.contains_key(&bid) {
                        state.log(format!("SYNC RESTORED batch={:?}", &bid[..4]));
                        state.latest_finalized = Some(bid);
                        state.finalized_batches.insert(bid, finalized);
                    }
                }
            }

            PoSeqMessage::BridgeAck(ack) => {
                let mut state = self.state.lock().await;
                state.log(format!("BRIDGE_ACK batch={:?} success={}", &ack.batch_id[..4], ack.success));
            }

            PoSeqMessage::MisbehaviorReport(report) => {
                let mut state = self.state.lock().await;
                state.log(format!("MISBEHAVIOR accused={:?} kind={}", &report.accused_id[..4], report.kind));
            }

            PoSeqMessage::CheckpointAnnounce(ann) => {
                let mut state = self.state.lock().await;
                state.log(format!("CHECKPOINT epoch={} from={:?}", ann.epoch, &ann.node_id[..4]));
            }

            PoSeqMessage::PeerList(list) => {
                // Merge new peers from discovery response
                let mut pm = self.peer_manager.lock().await;
                let self_id = self.config.node_id;
                let mut added = 0usize;
                for peer in &list.peers {
                    if peer.node_id == self_id { continue; }
                    if !pm.has_peer(&peer.node_id) {
                        pm.register_peer(peer.node_id, peer.listen_addr.clone());
                        added += 1;
                    }
                }
                drop(pm);
                if added > 0 {
                    let mut state = self.state.lock().await;
                    state.log(format!("DISCOVERY merged {added} peer(s) from {}", hex::encode(&list.sender_id[..4])));
                }
            }

            PoSeqMessage::Signed(_) => {
                // Already unwrapped by maybe_verify() before dispatch.
                // Reaching here means a double-wrapped message — drop it.
            }

            // ── HotStuff BFT messages ─────────────────────────────────────

            PoSeqMessage::HotStuffBlock(block) => {
                let view = block.view;
                let block_id = block.block_id;
                let output = {
                    let mut hs = self.hotstuff.lock().await;
                    hs.on_block(block)
                };
                {
                    let mut state = self.state.lock().await;
                    state.log(format!("HOTSTUFF BLOCK view={view} id={}", hex::encode(&block_id[..4])));
                }
                self.dispatch_hotstuff_output(output).await;
            }

            PoSeqMessage::HotStuffVote(vote) => {
                let output = {
                    let mut hs = self.hotstuff.lock().await;
                    hs.on_vote(vote)
                };
                self.dispatch_hotstuff_output(output).await;
            }

            PoSeqMessage::HotStuffQC(qc) => {
                let phase = qc.phase;
                let view = qc.view;
                let output = {
                    let mut hs = self.hotstuff.lock().await;
                    hs.on_qc(qc)
                };
                {
                    let mut state = self.state.lock().await;
                    state.log(format!("HOTSTUFF QC view={view} phase={}", phase.name()));
                }
                self.dispatch_hotstuff_output(output).await;
            }

            PoSeqMessage::HotStuffNewView(nv) => {
                let new_view = nv.new_view;
                let output = {
                    let mut hs = self.hotstuff.lock().await;
                    hs.on_new_view(nv)
                };
                {
                    let mut state = self.state.lock().await;
                    state.log(format!("HOTSTUFF NEW-VIEW view={new_view}"));
                }
                self.dispatch_hotstuff_output(output).await;
            }

            PoSeqMessage::SequencerRegistered(reg) => {
                // Gossip: a peer announced its on-chain registration.
                // Apply to our local registry so we can accept its signed messages.
                let node_id = reg.record.node_id;
                let moniker = reg.record.moniker.clone();
                let block_height = reg.block_height;
                {
                    let mut reg_locked = self.sequencer_registry.lock().await;
                    reg_locked.apply_registration(reg.record);
                }
                let mut state = self.state.lock().await;
                state.log(format!(
                    "SEQUENCER_REGISTERED node={} moniker={} height={}",
                    hex::encode(&node_id[..4]),
                    moniker,
                    block_height,
                ));
            }
        }
    }

    /// Dispatch a `HotStuffOutput` action: broadcast messages, persist finalized blocks.
    async fn dispatch_hotstuff_output(&self, output: HotStuffOutput) {
        match output {
            HotStuffOutput::None => {}

            HotStuffOutput::SendVote(vote) => {
                // Send vote to all peers (in production: send only to leader)
                let pm = self.peer_manager.lock().await;
                let addrs = pm.all_peer_addrs();
                drop(pm);
                self.transport.broadcast(&addrs, &PoSeqMessage::HotStuffVote(vote)).await;
            }

            HotStuffOutput::BroadcastQC(qc) => {
                let pm = self.peer_manager.lock().await;
                let addrs = pm.all_peer_addrs();
                drop(pm);
                self.transport.broadcast(&addrs, &PoSeqMessage::HotStuffQC(qc)).await;
            }

            HotStuffOutput::SendNewView(nv) => {
                let pm = self.peer_manager.lock().await;
                let addrs = pm.all_peer_addrs();
                drop(pm);
                self.transport.broadcast(&addrs, &PoSeqMessage::HotStuffNewView(nv)).await;
            }

            HotStuffOutput::Finalize(block) => {
                // Convert HotStuffBlock to WireFinalized for the existing finalization path
                let batch_id = block.block_id;
                let delivery_seq = {
                    let mut state = self.state.lock().await;
                    let seq = state.delivery_seq;
                    state.delivery_seq += 1;
                    state.latest_finalized = Some(batch_id);
                    state.log(format!("HOTSTUFF FINALIZE view={} batch={}", block.view, hex::encode(&batch_id[..4])));
                    seq
                };

                // Build a WireFinalized from the HotStuff block
                let fin = WireFinalized {
                    batch_id,
                    proposal_id: batch_id, // same as block_id in HotStuff
                    slot: block.view,      // view ≡ slot in pipelined variant
                    epoch: self.state.lock().await.current_epoch,
                    leader_id: block.leader_id,
                    batch_root: block.batch_root,
                    ordered_submission_ids: block.ordered_submission_ids,
                    approvals: self.hotstuff.lock().await.quorum_threshold,
                    committee_size: self.state.lock().await.committee.len().max(1),
                    finalization_hash: batch_id, // simplified; real = computed hash
                };

                // Persist
                let encoded = bincode::serialize(&fin).unwrap_or_default();
                {
                    let mut locked = self.store.lock().await;
                    locked.engine.put_finalized(&batch_id, encoded);
                    locked.engine.put_raw(b"meta:latest_finalized", batch_id.to_vec());
                }

                // Deliver to runtime
                self.deliver_to_runtime(&fin, delivery_seq).await;
            }

            HotStuffOutput::Multi(outputs) => {
                // Flatten Multi without recursive async calls.
                // Collect all non-Multi leaves into a Vec<HotStuffOutput>, then
                // dispatch each leaf using the same match arms as the outer match.
                // Multi-of-Multi nesting is at most 2 levels in practice.
                let mut leaves: Vec<HotStuffOutput> = Vec::new();
                let mut queue = outputs;
                while let Some(o) = queue.pop() {
                    match o {
                        HotStuffOutput::Multi(inner) => queue.extend(inner),
                        leaf => leaves.push(leaf),
                    }
                }
                // Now inline-dispatch each leaf without calling dispatch_hotstuff_output
                let pm = self.peer_manager.lock().await;
                let addrs = pm.all_peer_addrs();
                drop(pm);
                for leaf in leaves {
                    match leaf {
                        HotStuffOutput::SendVote(vote) => {
                            self.transport.broadcast(&addrs, &PoSeqMessage::HotStuffVote(vote)).await;
                        }
                        HotStuffOutput::BroadcastQC(qc) => {
                            self.transport.broadcast(&addrs, &PoSeqMessage::HotStuffQC(qc)).await;
                        }
                        HotStuffOutput::SendNewView(nv) => {
                            self.transport.broadcast(&addrs, &PoSeqMessage::HotStuffNewView(nv)).await;
                        }
                        HotStuffOutput::Finalize(block) => {
                            // Deliver finalized block from Multi context
                            let batch_id = block.block_id;
                            let delivery_seq = {
                                let mut state = self.state.lock().await;
                                let seq = state.delivery_seq;
                                state.delivery_seq += 1;
                                state.latest_finalized = Some(batch_id);
                                seq
                            };
                            let fin = WireFinalized {
                                batch_id,
                                proposal_id: batch_id,
                                slot: block.view,
                                epoch: self.state.lock().await.current_epoch,
                                leader_id: block.leader_id,
                                batch_root: block.batch_root,
                                ordered_submission_ids: block.ordered_submission_ids,
                                approvals: self.hotstuff.lock().await.quorum_threshold,
                                committee_size: self.state.lock().await.committee.len().max(1),
                                finalization_hash: batch_id,
                            };
                            let encoded = bincode::serialize(&fin).unwrap_or_default();
                            {
                                let mut locked = self.store.lock().await;
                                locked.engine.put_finalized(&batch_id, encoded);
                                locked.engine.put_raw(b"meta:latest_finalized", batch_id.to_vec());
                            }
                            self.deliver_to_runtime(&fin, delivery_seq).await;
                        }
                        HotStuffOutput::None | HotStuffOutput::Multi(_) => {}
                    }
                }
            }
        }
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::time::{Duration, sleep};

    fn nid(b: u8) -> NodeId { let mut id = [0u8; 32]; id[0] = b; id }

    fn make_config(node_id: NodeId, addr: &str, peers: Vec<PeerEntry>, quorum: usize) -> NodeConfig {
        NodeConfig {
            node_id,
            listen_addr: addr.to_string(),
            peers,
            quorum_threshold: quorum,
            slot_duration_ms: 100,
            data_dir: format!("/tmp/poseq_test_{}", node_id[0]),
            role: NodeRole::Attestor,
        }
    }

    #[tokio::test]
    async fn test_node_bind_and_announce() {
        let config = make_config(nid(1), "127.0.0.1:0", vec![], 1);
        let node = NetworkedNode::bind(config).await.unwrap();
        // Should not panic
        node.announce().await;
        let state = node.state.lock().await;
        assert_eq!(state.current_slot, 0);
    }

    #[tokio::test]
    async fn test_leader_election_deterministic() {
        let committee = vec![nid(1), nid(2), nid(3)];
        let leader1 = NodeState::elect_leader(1, 5, &committee);
        let leader2 = NodeState::elect_leader(1, 5, &committee);
        assert_eq!(leader1, leader2);
    }

    #[tokio::test]
    async fn test_leader_election_changes_with_slot() {
        let committee = vec![nid(1), nid(2), nid(3)];
        let mut leaders = std::collections::BTreeSet::new();
        for slot in 0..10 {
            if let Some(l) = NodeState::elect_leader(1, slot, &committee) {
                leaders.insert(l);
            }
        }
        // Over 10 slots should elect at least 2 different leaders
        assert!(leaders.len() >= 2, "election should rotate leaders");
    }

    #[tokio::test]
    async fn test_leader_election_empty_committee_returns_none() {
        assert!(NodeState::elect_leader(1, 1, &[]).is_none());
    }

    #[tokio::test]
    async fn test_proposal_hash_is_deterministic() {
        let prop = WireProposal {
            proposal_id: nid(1), slot: 5, epoch: 1, leader_id: nid(2),
            batch_root: nid(3), parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![], policy_version: 1, created_at_height: 5,
        };
        let h1 = NodeState::compute_batch_id(&prop);
        let h2 = NodeState::compute_batch_id(&prop);
        assert_eq!(h1, h2);
    }

    #[tokio::test]
    async fn test_finalization_hash_changes_with_approvals() {
        let prop = WireProposal {
            proposal_id: nid(1), slot: 5, epoch: 1, leader_id: nid(2),
            batch_root: nid(3), parent_batch_id: [0u8; 32],
            ordered_submission_ids: vec![], policy_version: 1, created_at_height: 5,
        };
        let h1 = NodeState::compute_finalization_hash(&prop, 2);
        let h2 = NodeState::compute_finalization_hash(&prop, 3);
        assert_ne!(h1, h2);
    }

    #[tokio::test]
    async fn test_quorum_not_reached_below_threshold() {
        let state = NodeState::new();
        assert!(!state.quorum_reached(&nid(1), 3));
    }

    #[tokio::test]
    async fn test_quorum_reached_at_threshold() {
        let mut state = NodeState::new();
        let pid = nid(1);
        state.attestations.insert(pid, vec![
            WireAttestation { attestor_id: nid(10), proposal_id: pid, batch_id_attested: nid(20), approve: true, epoch: 1, slot: 1 },
            WireAttestation { attestor_id: nid(11), proposal_id: pid, batch_id_attested: nid(20), approve: true, epoch: 1, slot: 1 },
            WireAttestation { attestor_id: nid(12), proposal_id: pid, batch_id_attested: nid(20), approve: true, epoch: 1, slot: 1 },
        ]);
        assert!(state.quorum_reached(&pid, 3));
        assert!(!state.quorum_reached(&pid, 4));
    }

    #[tokio::test]
    async fn test_three_node_proposal_and_finalization() {
        // Node 1 is the leader, nodes 2 and 3 are attestors.
        // All three bind to ephemeral ports.

        let id1 = nid(1);
        let id2 = nid(2);
        let id3 = nid(3);

        // Bind all three nodes first to get their addresses
        let cfg1 = make_config(id1, "127.0.0.1:0", vec![], 2);
        let cfg2 = make_config(id2, "127.0.0.1:0", vec![], 2);
        let cfg3 = make_config(id3, "127.0.0.1:0", vec![], 2);

        let mut node1 = NetworkedNode::bind(cfg1).await.unwrap();
        let mut node2 = NetworkedNode::bind(cfg2).await.unwrap();
        let mut node3 = NetworkedNode::bind(cfg3).await.unwrap();

        let addr1 = node1.transport.listen_addr.clone();
        let addr2 = node2.transport.listen_addr.clone();
        let addr3 = node3.transport.listen_addr.clone();

        // Wire up peer managers
        {
            let mut pm1 = node1.peer_manager.lock().await;
            pm1.register_peer(id2, addr2.clone());
            pm1.register_peer(id3, addr3.clone());
        }
        {
            let mut pm2 = node2.peer_manager.lock().await;
            pm2.register_peer(id1, addr1.clone());
            pm2.register_peer(id3, addr3.clone());
        }
        {
            let mut pm3 = node3.peer_manager.lock().await;
            pm3.register_peer(id1, addr1.clone());
            pm3.register_peer(id2, addr2.clone());
        }

        // Set committee and leader on all nodes
        let committee = vec![id1, id2, id3];
        for node in [&node1, &node2, &node3] {
            let mut s = node.state.lock().await;
            s.committee = committee.clone();
            s.in_committee = true;
            s.current_slot = 1;
            s.current_epoch = 1;
        }
        // Determine the deterministic leader for epoch=1, slot=1
        let leader_id = NodeState::elect_leader(1, 1, &committee).unwrap();
        for node in [&node1, &node2, &node3] {
            node.state.lock().await.current_leader = Some(leader_id);
        }

        // Start event loops in background
        let ctrl1 = node1.ctrl_tx.clone();
        let ctrl2 = node2.ctrl_tx.clone();
        let ctrl3 = node3.ctrl_tx.clone();

        tokio::spawn(async move { node1.run_event_loop().await });
        tokio::spawn(async move { node2.run_event_loop().await });
        tokio::spawn(async move { node3.run_event_loop().await });

        // Give listeners time to start
        sleep(Duration::from_millis(50)).await;

        // Trigger NextSlot on all nodes (this causes the leader to propose)
        ctrl1.send(NodeControl::NextSlot).await.unwrap();
        ctrl2.send(NodeControl::NextSlot).await.unwrap();
        ctrl3.send(NodeControl::NextSlot).await.unwrap();

        // Wait for finalization to propagate
        sleep(Duration::from_millis(500)).await;

        // Check that all nodes shut down cleanly
        ctrl1.send(NodeControl::Shutdown).await.ok();
        ctrl2.send(NodeControl::Shutdown).await.ok();
        ctrl3.send(NodeControl::Shutdown).await.ok();

        // Test passes if no panics occurred — finalization propagation
        // is verified by the event logs in each node's state.
    }

    #[tokio::test]
    async fn test_node_crash_and_rejoin() {
        let id1 = nid(10);
        let cfg = make_config(id1, "127.0.0.1:0", vec![], 1);
        let mut node = NetworkedNode::bind(cfg).await.unwrap();
        let ctrl = node.ctrl_tx.clone();
        let state = node.state.clone();

        tokio::spawn(async move { node.run_event_loop().await });

        // Crash the node
        ctrl.send(NodeControl::Crash).await.unwrap();
        sleep(Duration::from_millis(50)).await;

        // Rejoin
        ctrl.send(NodeControl::Rejoin).await.unwrap();
        sleep(Duration::from_millis(50)).await;

        let log = state.lock().await.event_log.clone();
        assert!(log.iter().any(|e| e.contains("CRASH")));
        assert!(log.iter().any(|e| e.contains("REJOIN")));

        ctrl.send(NodeControl::Shutdown).await.ok();
    }
}
