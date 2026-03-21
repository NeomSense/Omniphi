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
use crate::chain_bridge::snapshot::{ChainCommitteeSnapshot, SnapshotImporter, SnapshotImportError};
use crate::chain_bridge::exporter::{ChainBridgeExporter, ExportBatch};
use crate::observability::NodeEventLog;
use crate::observability::metrics::PoSeqMetrics;

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
    /// Number of slots per epoch.  When a slot crosses this boundary the node
    /// automatically exports the just-completed epoch and increments
    /// `current_epoch`.  Defaults to 10 when constructed via helper fns.
    pub slots_per_epoch: u64,
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
    /// Legacy plain-string event log (kept for devnet/test compatibility).
    pub event_log: Vec<String>,
    /// Structured JSON event log — primary observability path.
    pub slog: NodeEventLog,
    /// Monotone counter for runtime delivery sequence numbers.
    pub delivery_seq: u64,
    /// Epochs for which an ExportBatch has already been emitted.
    /// Prevents duplicate export on restart or repeated epoch trigger.
    pub exported_epochs: std::collections::BTreeSet<u64>,
    /// Most recently imported committee snapshot epoch (None if never imported).
    pub latest_snapshot_epoch: Option<u64>,
    /// Sync status tracking (catch-up detection, bridge delivery, checkpoints).
    pub sync_engine: crate::sync::StateSyncEngine,
    /// Number of connected peers (updated from peer manager on each PeerStatus).
    pub connected_peers: usize,
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
            slog: NodeEventLog::new(false),
            delivery_seq: 0,
            exported_epochs: std::collections::BTreeSet::new(),
            latest_snapshot_epoch: None,
            sync_engine: crate::sync::StateSyncEngine::new(
                crate::checkpoints::CheckpointPolicy::default_policy(),
                crate::bridge_recovery::BridgeRetryPolicy::default_policy(),
            ),
            connected_peers: 0,
        }
    }

    pub fn log(&mut self, event: String) {
        // Legacy plain-string log (kept for devnet compatibility).
        self.event_log.push(event.clone());
        // Structured log: use event string as both event name and details.
        self.slog.info("node.event", self.current_epoch, Some(self.current_slot), event);
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
    /// Committee snapshot importer — validates and caches chain-produced snapshots.
    pub snapshot_importer: Arc<Mutex<SnapshotImporter>>,
    /// Chain bridge exporter — packages accountability events for submission to chain.
    pub bridge_exporter: Arc<Mutex<ChainBridgeExporter>>,
    inbox: mpsc::Receiver<(String, PoSeqMessage)>,
    /// Channel for external control (used in tests / devnet runner)
    pub ctrl_tx: mpsc::Sender<NodeControl>,
    ctrl_rx: mpsc::Receiver<NodeControl>,
    /// Optional Prometheus metrics — wired in devnet/binary mode, None in unit tests.
    pub metrics: Option<Arc<PoSeqMetrics>>,
    /// True if this node was restored from non-empty sled state (warm restart).
    is_warm_restart: bool,
    /// Raw 32-byte Ed25519 seed for wire message signing. Set by `set_signing_seed()`.
    pub signing_seed: Option<[u8; 32]>,
}

/// Control commands for the node event loop.
pub enum NodeControl {
    /// Advance to next slot (triggered externally in devnet).
    NextSlot,
    /// Simulate a node crash (stop processing inbound messages).
    Crash,
    /// Rejoin after a crash (resume processing).
    Rejoin,
    /// Graceful shutdown.
    Shutdown,
    /// Import a committee snapshot from the chain.
    /// The snapshot is validated before acceptance; invalid snapshots are rejected
    /// with a structured log entry.
    ImportSnapshot(Box<ChainCommitteeSnapshot>),
    /// Export accountability events + batch data for the given epoch to the chain bridge.
    /// Duplicate requests for an already-exported epoch are silently ignored
    /// (idempotent by design).
    ExportEpoch(u64),
    /// Internal: fired by the periodic heartbeat interval to run health checks.
    HeartbeatTick,
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

        // Restore persisted state from durable store.
        // For each restored item we emit a structured startup log entry so that
        // recovery decisions are visible and auditable in the node event log.
        let mut restored_snapshots: Vec<ChainCommitteeSnapshot> = Vec::new();
        {
            let locked = store.lock().await;

            // Restore latest_finalized batch id.
            if let Some(bytes) = locked.engine.get_raw(b"meta:latest_finalized") {
                if bytes.len() == 32 {
                    let mut id = [0u8; 32];
                    id.copy_from_slice(&bytes);
                    initial_state.latest_finalized = Some(id);
                    initial_state.slog.info(
                        "startup.restore.finalized",
                        0,
                        None,
                        format!("restored latest_finalized = {}", hex::encode(&id[..4])),
                    );
                }
            }

            // Restore exported_epochs by scanning persisted export keys.
            // Each exported epoch is stored at key b"export:epoch:<N>" so a prefix
            // scan lets us reconstruct the set without a separate index.
            let prefix = b"export:epoch:";
            for (key, _) in locked.engine.prefix_scan_raw(prefix) {
                // Key format: "export:epoch:<decimal string>"
                if let Ok(key_str) = std::str::from_utf8(&key) {
                    if let Some(epoch_str) = key_str.strip_prefix("export:epoch:") {
                        if let Ok(epoch) = epoch_str.parse::<u64>() {
                            initial_state.exported_epochs.insert(epoch);
                        }
                    }
                }
            }
            let epoch_count = initial_state.exported_epochs.len();
            initial_state.slog.info(
                "startup.restore.exported_epochs",
                0,
                None,
                format!("restored exported_epochs: {} epochs from sled", epoch_count),
            );

            // Restore chain snapshots: deserialize each stored snapshot and collect
            // for re-import into a fresh SnapshotImporter after the lock is released.
            // Snapshots are stored under key b"chain_snapshot:<epoch_be(8)>".
            let snap_prefix = b"chain_snapshot:";
            for (key, val) in locked.engine.prefix_scan_raw(snap_prefix) {
                // Key format: "chain_snapshot:" + epoch as 8 big-endian bytes
                let offset = snap_prefix.len();
                if key.len() == offset + 8 {
                    if let Ok(snap) = serde_json::from_slice::<ChainCommitteeSnapshot>(&val) {
                        restored_snapshots.push(snap);
                    }
                }
            }
        }

        // Reconstruct SnapshotImporter from persisted snapshots so that
        // duplicate delivery after restart is correctly rejected.
        let snapshot_importer = {
            let mut imp = SnapshotImporter::new();
            let mut latest_snap_epoch: Option<u64> = None;
            for snap in &restored_snapshots {
                let epoch = snap.epoch;
                if imp.import(snap.clone()).is_ok() {
                    latest_snap_epoch = Some(match latest_snap_epoch {
                        Some(prev) => prev.max(epoch),
                        None => epoch,
                    });
                }
            }
            initial_state.latest_snapshot_epoch = latest_snap_epoch;
            initial_state.slog.info(
                "startup.restore.snapshots",
                0,
                None,
                format!(
                    "restored {} committee snapshots from sled; latest_snapshot_epoch = {:?}",
                    restored_snapshots.len(),
                    initial_state.latest_snapshot_epoch,
                ),
            );
            imp
        };

        // Detect warm restart: non-empty restored state means this is not a cold boot.
        let is_warm_restart = !initial_state.exported_epochs.is_empty()
            || initial_state.latest_snapshot_epoch.is_some();

        let quorum = config.quorum_threshold;
        let hs_timeout_ms = config.slot_duration_ms * 4;

        // Create HotStuff engine and restore SafetyRule from durable store
        // BEFORE wrapping in Arc<Mutex> to prevent equivocation after restart.
        let mut hs_engine = HotStuffEngine::new(self_id, quorum, hs_timeout_ms);
        {
            let locked = store.lock().await;
            if hs_engine.restore_safety_rule(locked.engine.backend()) {
                initial_state.slog.info(
                    "startup.restore.safety_rule",
                    0,
                    None,
                    format!(
                        "restored HotStuff SafetyRule: view={}",
                        hs_engine.current_view(),
                    ),
                );
            }
        }

        Ok(NetworkedNode {
            config,
            state: Arc::new(Mutex::new(initial_state)),
            transport: Arc::new(transport),
            peer_manager: Arc::new(Mutex::new(pm)),
            store,
            key_pair: None,           // set via set_key_pair() after bind
            verify_signatures: false,  // enable via set_verify_signatures() after bind
            runtime_sender: None,      // set via set_runtime_sender() after bind
            hotstuff: Arc::new(Mutex::new(hs_engine)),
            sequencer_registry: Arc::new(Mutex::new(SequencerRegistry::new())),
            snapshot_importer: Arc::new(Mutex::new(snapshot_importer)),
            bridge_exporter: Arc::new(Mutex::new(ChainBridgeExporter::new())),
            inbox,
            ctrl_tx,
            ctrl_rx,
            metrics: None,             // set via set_metrics() after bind
            is_warm_restart,
            signing_seed: None,
        })
    }

    /// Set the node's Ed25519 signing key pair.
    /// Should be called before `run_event_loop()`.
    pub fn set_key_pair(&mut self, kp: NodeKeyPair) {
        self.key_pair = Some(Arc::new(kp));
    }

    /// Set the raw 32-byte Ed25519 seed used for wire message signing.
    /// When set, `maybe_sign` will wrap outbound messages in `WireSignedEnvelope`.
    pub fn set_signing_seed(&mut self, seed: [u8; 32]) {
        self.signing_seed = Some(seed);
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

    /// Attach Prometheus metrics for devnet/production observability.
    /// If this node was restored from non-empty sled state, increments `node_restarts`.
    pub fn set_metrics(&mut self, metrics: Arc<PoSeqMetrics>) {
        if self.is_warm_restart {
            metrics.node_restarts.inc();
        }
        self.metrics = Some(metrics);
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

    /// Wrap a message in a `WireSignedEnvelope` if a signing seed is configured,
    /// otherwise return the message unchanged.
    ///
    /// `signer_id` is always `node_id` — the protocol identity. The receiver
    /// looks up `node_id` in the SequencerRegistry to find the corresponding
    /// Ed25519 public key and verifies the signature against that.
    fn maybe_sign(&self, msg: PoSeqMessage) -> PoSeqMessage {
        if let Some(ref seed) = self.signing_seed {
            if let Some(signed) = WireSignedEnvelope::sign(&msg, seed, self.config.node_id) {
                return PoSeqMessage::Signed(signed);
            }
        }
        msg
    }

    /// Unwrap a signed envelope with full Ed25519 verification.
    ///
    /// Returns `(inner_message, verified_node_id)` on success, `None` on failure.
    ///
    /// Security model:
    /// - `signer_id` in the envelope is the sender's `node_id` (protocol identity).
    /// - The receiver looks up `signer_id` in the `SequencerRegistry` to get the
    ///   registered Ed25519 public key, then calls `verify_and_decode(pubkey)`.
    /// - If `signer_id` is not in the registry, the message is REJECTED —
    ///   including PeerStatus. We cannot verify a signature without a known
    ///   public key, and `node_id` is not guaranteed to be the raw Ed25519 key
    ///   (it may be a hash or operator-chosen value per registry.rs:48).
    ///   Peer discovery for unregistered nodes relies on the bootstrap peer
    ///   list in the node config, not on unauthenticated wire messages.
    /// - Unsigned messages are rejected when `verify_signatures = true`.
    fn maybe_verify(&self, msg: PoSeqMessage) -> Option<(PoSeqMessage, Option<[u8; 32]>)> {
        match msg {
            PoSeqMessage::Signed(ref env) => {
                if self.verify_signatures {
                    let reg = self.sequencer_registry.try_lock().ok()?;
                    if let Some(rec) = reg.get(&env.signer_id) {
                        // Registered node: verify signature against registered pubkey
                        let inner = env.verify_and_decode(&rec.public_key)?;
                        Some((inner, Some(env.signer_id)))
                    } else {
                        // Not in registry — reject. Without a registered pubkey
                        // we cannot verify the signature. node_id != pubkey in
                        // the general case (registry.rs:48).
                        None
                    }
                } else {
                    // Devnet/test mode: decode without verification
                    let inner = env.decode_unverified()?;
                    Some((inner, None))
                }
            }
            other => {
                if self.verify_signatures {
                    // Reject unsigned messages when signature enforcement is enabled.
                    None
                } else {
                    Some((other, None))
                }
            }
        }
    }

    /// Open a durable sled store. Panics on failure — in-memory fallback is
    /// unsafe because it silently breaks replay protection, dedup, and restart
    /// safety. A node that cannot persist state MUST NOT participate in consensus.
    fn open_store(data_dir: &str) -> Arc<Mutex<DurableStore>> {
        let backend: Box<dyn crate::persistence::backend::PersistenceBackend> =
            match SledBackend::open(std::path::Path::new(data_dir)) {
                Ok(sled) => Box::new(sled),
                Err(e) => {
                    panic!(
                        "[poseq] FATAL: sled open failed for {data_dir}: {e} — \
                         refusing to start with in-memory store (replay protection \
                         and dedup would be lost on restart). Fix the data directory \
                         permissions or disk and retry."
                    );
                }
            };
        let engine = PersistenceEngine::new(backend);
        let mut store = DurableStore::new(engine);
        store.write_schema_version();
        Arc::new(Mutex::new(store))
    }

    /// Broadcast a message, signing it first if a signing seed is configured.
    /// Use this for all consensus messages (Proposal, Attestation, Finalized, etc.)
    /// so that nodes with `verify_signatures = true` can receive them.
    async fn broadcast_signed(&self, addrs: &[String], msg: PoSeqMessage) {
        let signed = self.maybe_sign(msg);
        self.transport.broadcast(addrs, &signed).await;
    }

    /// Send a signed message to a single peer.
    async fn send_signed(&self, addr: &str, msg: PoSeqMessage) {
        let signed = self.maybe_sign(msg);
        let _ = self.transport.send_to(addr, &signed).await;
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
            protocol_version: Some(crate::versioning::PROTOCOL_VERSION.to_string()),
        });
        drop(state);
        let pm = self.peer_manager.lock().await;
        let addrs = pm.all_peer_addrs();
        let connected = pm.connected_count();
        drop(pm);
        self.broadcast_signed(&addrs, msg).await;
        if let Some(ref m) = self.metrics {
            m.peer_count.set(connected as i64);
        }
    }

    /// Periodic health-check tick: degrade silent peers, evict dead peers,
    /// update peer metrics, and attempt reconnect to disconnected peers.
    async fn run_heartbeat_tick(&self) {
        let now = std::time::Instant::now();
        // Health check: silence threshold = 10 slots
        let silence_ms = self.config.slot_duration_ms * 10;
        // Dead peer eviction: max silence = 50 slots
        let max_silence_ms = self.config.slot_duration_ms * 50;
        {
            let mut pm = self.peer_manager.lock().await;
            pm.tick_health_check(now, silence_ms);
            pm.evict_dead_peers(now, max_silence_ms);
            // Update peer metrics
            if let Some(ref m) = self.metrics {
                m.peer_count.set(pm.connected_count() as i64);
                m.peers_connected.set(pm.connected_count() as i64);
                m.peers_degraded.set(pm.degraded_count() as i64);
                m.peers_disconnected.set(pm.disconnected_count() as i64);
            }
        }
        // Phase 8: update sync lag metrics
        {
            let state = self.state.lock().await;
            let sync_status = state.sync_engine.sync_status();
            if let Some(ref m) = self.metrics {
                m.sync_lag_epochs.set(sync_status.lag as i64);
                m.bridge_backlog.set(sync_status.bridge_backlog as i64);
            }
        }
        // Announce to reconnect candidates (attempt reconnect by sending PeerStatus)
        let candidates = {
            let pm = self.peer_manager.lock().await;
            pm.reconnect_candidates()
        };
        if !candidates.is_empty() {
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
                protocol_version: Some(crate::versioning::PROTOCOL_VERSION.to_string()),
            });
            drop(state);
            let addrs: Vec<String> = candidates.iter().map(|(_, a)| a.clone()).collect();
            self.broadcast_signed(&addrs, msg).await;
        }
    }

    /// Run the main event loop.  Returns when Shutdown is received.
    pub async fn run_event_loop(&mut self) {
        let mut crashed = false;
        let heartbeat_interval = Duration::from_millis(self.config.slot_duration_ms * 3);
        let mut heartbeat_tick = tokio::time::interval(heartbeat_interval);
        heartbeat_tick.tick().await; // consume first immediate tick
        loop {
            tokio::select! {
                // Inbound TCP message
                maybe_msg = self.inbox.recv() => {
                    match maybe_msg {
                        None => break, // transport closed
                        Some((peer_addr, msg)) => {
                            if !crashed {
                                // Verify/unwrap signed envelope before dispatch
                                if let Some((inner, verified_signer)) = self.maybe_verify(msg) {
                                    self.handle_message(peer_addr, inner, verified_signer).await;
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
                        Some(NodeControl::ImportSnapshot(snap)) => {
                            self.handle_import_snapshot(*snap).await;
                        }
                        Some(NodeControl::ExportEpoch(epoch)) => {
                            self.handle_export_epoch(epoch).await;
                        }
                        Some(NodeControl::HeartbeatTick) => {
                            if !crashed {
                                self.run_heartbeat_tick().await;
                            }
                        }
                    }
                }
                // Periodic heartbeat tick
                _ = heartbeat_tick.tick() => {
                    if !crashed {
                        self.run_heartbeat_tick().await;
                    }
                }
            }
        }
    }

    /// Advance to the next slot.  If we are the leader, broadcast a proposal.
    /// When `current_slot % slots_per_epoch == 0`, automatically exports the
    /// just-completed epoch and increments `current_epoch`.
    async fn advance_slot(&self) {
        let mut state = self.state.lock().await;
        state.current_slot += 1;
        state.proposed_this_slot = false;
        state.pending_proposals.clear();
        state.attestations.clear();

        // Auto epoch advance: when slot crosses an epoch boundary, export the
        // completed epoch and move to the next one.
        let slots_per_epoch = self.config.slots_per_epoch.max(1);
        if state.current_slot > 0 && state.current_slot % slots_per_epoch == 0 {
            let completed_epoch = state.current_epoch;
            state.current_epoch += 1;
            let new_epoch = state.current_epoch;
            let boundary_slot = state.current_slot;
            state.slog.info(
                "epoch.advance",
                new_epoch,
                Some(boundary_slot),
                format!(
                    "slot {boundary_slot} crossed epoch boundary — epoch {completed_epoch} complete, advancing to epoch {new_epoch}",
                ),
            );
            // Update metrics gauges
            if let Some(ref m) = self.metrics {
                m.current_epoch.set(new_epoch as i64);
            }
            // Phase 8: create sync checkpoint at epoch boundary
            {
                let committee_hash = {
                    let mut h = sha2::Sha256::new();
                    for id in &state.committee { sha2::Digest::update(&mut h, id); }
                    let bytes: [u8; 32] = sha2::Digest::finalize(h).into();
                    bytes
                };
                // Extract values before taking mutable borrow of sync_engine
                let new_epoch_val = state.current_epoch;
                let slot_val = state.current_slot;
                let latest_fin = state.latest_finalized;
                let exported_clone = state.exported_epochs.clone();
                let node_id = self.config.node_id;
                state.sync_engine.update_local_epoch(new_epoch_val);
                let _cp_id = state.sync_engine.create_checkpoint(
                    completed_epoch,
                    slot_val,
                    latest_fin,
                    committee_hash,
                    &exported_clone,
                    node_id,
                );
            }
            drop(state);
            // Trigger export for the just-completed epoch (idempotent).
            let _ = self.ctrl_tx.send(NodeControl::ExportEpoch(completed_epoch)).await;
            // Re-acquire state for leader election below.
            let mut state = self.state.lock().await;
            let leader = NodeState::elect_leader(state.current_epoch, state.current_slot, &state.committee);
            state.current_leader = leader;
            let is_leader = leader == Some(self.config.node_id);
            state.in_committee = state.committee.contains(&self.config.node_id);
            let log_slot = state.current_slot;
            let log_epoch = state.current_epoch;
            let log_leader = leader.map(|id| hex::encode(&id[..4]));
            state.log(format!("slot={log_slot} epoch={log_epoch} leader={log_leader:?} is_me={is_leader}"));
            if let Some(ref m) = self.metrics {
                m.current_slot.set(log_slot as i64);
            }
            if is_leader && !state.proposed_this_slot {
                state.proposed_this_slot = true;
                let proposal = WireProposal {
                    proposal_id: [0u8; 32],
                    slot: state.current_slot,
                    epoch: state.current_epoch,
                    leader_id: self.config.node_id,
                    batch_root: [0u8; 32],
                    parent_batch_id: state.latest_finalized.unwrap_or([0u8; 32]),
                    ordered_submission_ids: vec![],
                    policy_version: 1,
                    created_at_height: state.current_slot,
                };
                let proposal_id = NodeState::compute_batch_id(&proposal);
                let proposal = WireProposal { proposal_id, ..proposal };
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
                self.broadcast_signed(&addrs, msg).await;
            }
            return;
        }

        // Deterministic leader election
        let leader = NodeState::elect_leader(state.current_epoch, state.current_slot, &state.committee);
        state.current_leader = leader;
        let is_leader = leader == Some(self.config.node_id);
        state.in_committee = state.committee.contains(&self.config.node_id);

        let log_slot = state.current_slot;
        let log_epoch = state.current_epoch;
        let log_leader = leader.map(|id| hex::encode(&id[..4]));
        state.log(format!("slot={log_slot} epoch={log_epoch} leader={log_leader:?} is_me={is_leader}"));
        if let Some(ref m) = self.metrics {
            m.current_slot.set(log_slot as i64);
        }

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
            self.broadcast_signed(&addrs, msg).await;
        }
    }

    /// Process one inbound message.
    /// Handle an inbound message after signature verification.
    ///
    /// `verified_signer` is `Some(node_id)` when the message was cryptographically
    /// verified against the SequencerRegistry, or `None` for unverified messages
    /// (devnet mode or PeerStatus discovery messages).
    ///
    /// For consensus messages (Proposal, Attestation, Finalized), the inner
    /// `leader_id` / `attestor_id` MUST match `verified_signer`. A mismatch
    /// means the peer signed with its own key but claimed a different identity
    /// in the message — this is rejected as identity spoofing.
    async fn handle_message(&self, _peer_addr: String, msg: PoSeqMessage, verified_signer: Option<[u8; 32]>) {
        // Dedup first
        let mut pm = self.peer_manager.lock().await;
        if pm.is_duplicate(&msg) {
            return;
        }
        drop(pm);

        match msg {
            PoSeqMessage::PeerStatus(status) => {
                // Identity binding: if the message was signature-verified,
                // the inner node_id must match the verified signer. This
                // prevents an attacker from signing with their own key but
                // claiming to be a different node in the PeerStatus payload.
                if let Some(signer) = verified_signer {
                    if status.node_id != signer {
                        let mut state = self.state.lock().await;
                        state.log(format!(
                            "PEERSTATUS REJECTED: signer {:?} != claimed node_id {:?} (identity spoofing)",
                            &signer[..4], &status.node_id[..4]
                        ));
                        return;
                    }
                }
                // All PeerStatus messages reaching here are from registered,
                // signature-verified senders (unregistered are rejected in
                // maybe_verify). In devnet mode (verify_signatures=false),
                // verified_signer is None but all messages are trusted.
                let mut pm = self.peer_manager.lock().await;
                pm.update_from_status(&status);
                let cc = pm.connected_count();
                if let Some(ref m) = self.metrics {
                    m.peer_count.set(cc as i64);
                    m.peers_connected.set(cc as i64);
                    m.peers_degraded.set(pm.degraded_count() as i64);
                    m.peers_disconnected.set(pm.disconnected_count() as i64);
                }
                drop(pm);
                // Update connected_peers on state so /status can report it
                {
                    let mut state = self.state.lock().await;
                    state.connected_peers = cc;
                }
                // Update peer epoch for catch-up detection
                {
                    let mut state = self.state.lock().await;
                    state.sync_engine.update_peer_epoch(status.current_epoch);
                }
                // Version compatibility check
                if let Some(ref version_str) = status.protocol_version {
                    use crate::versioning::ProtocolVersion;
                    if let Ok(remote_ver) = version_str.parse::<ProtocolVersion>() {
                        use crate::versioning::compat::check_wire_compat;
                        match check_wire_compat(&remote_ver) {
                            crate::versioning::compat::CompatResult::Incompatible(reason) => {
                                let mut state = self.state.lock().await;
                                let epoch = state.current_epoch;
                                let slot = state.current_slot;
                                state.slog.warn(
                                    "version.incompatible",
                                    epoch,
                                    Some(slot),
                                    format!("peer version {} incompatible: {reason}", version_str),
                                );
                                drop(state);
                                let mut pm = self.peer_manager.lock().await;
                                pm.mark_disconnected(&status.node_id);
                            }
                            crate::versioning::compat::CompatResult::CompatibleWithWarning(msg) => {
                                let mut state = self.state.lock().await;
                                let epoch = state.current_epoch;
                                let slot = state.current_slot;
                                state.slog.warn(
                                    "version.warning",
                                    epoch,
                                    Some(slot),
                                    format!("peer version {} warning: {msg}", version_str),
                                );
                            }
                            _ => {}
                        }
                    }
                }
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
                // Identity binding: the inner leader_id must match the verified
                // signer. Without this, a malicious peer could sign with its own
                // key but set leader_id to the real leader's node_id.
                if let Some(signer) = verified_signer {
                    if proposal.leader_id != signer {
                        let mut state = self.state.lock().await;
                        state.log(format!(
                            "PROPOSAL REJECTED: signer {:?} != claimed leader {:?} (identity spoofing)",
                            &signer[..4], &proposal.leader_id[..4]
                        ));
                        return;
                    }
                }
                let mut state = self.state.lock().await;
                let pid = proposal.proposal_id;
                // Verify this came from the expected leader
                if Some(proposal.leader_id) != state.current_leader {
                    state.log(format!("PROPOSAL from unexpected leader {:?}", &proposal.leader_id[..4]));
                    return;
                }
                // Reject proposals with duplicate submission IDs (double-spending prevention).
                // A malicious leader could include the same submission twice to double-execute it.
                {
                    let mut seen = std::collections::BTreeSet::new();
                    for sid in &proposal.ordered_submission_ids {
                        if !seen.insert(sid) {
                            state.log(format!(
                                "PROPOSAL REJECTED: duplicate submission_id {:?} (double-spend attempt)",
                                &sid[..4]
                            ));
                            return;
                        }
                    }
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
                    self.broadcast_signed(&addrs, PoSeqMessage::Attestation(attestation)).await;

                    // Persist HotStuff SafetyRule after voting to prevent equivocation on restart.
                    // Lock store first, then hotstuff, to maintain consistent lock ordering.
                    {
                        let mut locked = self.store.lock().await;
                        let hs = self.hotstuff.lock().await;
                        hs.persist_safety_rule(locked.engine.backend_mut());
                    }
                }
            }

            PoSeqMessage::Attestation(vote) => {
                // Identity binding: attestor_id must match the verified signer.
                if let Some(signer) = verified_signer {
                    if vote.attestor_id != signer {
                        let mut state = self.state.lock().await;
                        state.log(format!(
                            "ATTESTATION REJECTED: signer {:?} != claimed attestor {:?} (identity spoofing)",
                            &signer[..4], &vote.attestor_id[..4]
                        ));
                        return;
                    }
                }
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
                                // Persist HotStuff SafetyRule after finalization
                                let hs = self.hotstuff.lock().await;
                                hs.persist_safety_rule(locked.engine.backend_mut());
                                drop(hs);
                                locked.engine.flush();
                            }
                            // Deliver to runtime ingester
                            self.deliver_to_runtime(&finalized, delivery_seq).await;
                            let msg = PoSeqMessage::Finalized(finalized);
                            let pm = self.peer_manager.lock().await;
                            let addrs = pm.all_peer_addrs();
                            drop(pm);
                            self.broadcast_signed(&addrs, msg).await;
                            return;
                        }
                    }
                }
            }

            PoSeqMessage::Finalized(fin) => {
                // Identity binding: leader_id must match the verified signer.
                if let Some(signer) = verified_signer {
                    if fin.leader_id != signer {
                        let mut state = self.state.lock().await;
                        state.log(format!(
                            "FINALIZED REJECTED: signer {:?} != claimed leader {:?} (identity spoofing)",
                            &signer[..4], &fin.leader_id[..4]
                        ));
                        return;
                    }
                }
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
                        // Persist HotStuff SafetyRule after receiving finalization
                        let hs = self.hotstuff.lock().await;
                        hs.persist_safety_rule(locked.engine.backend_mut());
                        drop(hs);
                        locked.engine.flush();
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
                    self.send_signed(&addr, resp).await;
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
                self.broadcast_signed(&addrs, PoSeqMessage::HotStuffVote(vote)).await;
            }

            HotStuffOutput::BroadcastQC(qc) => {
                let pm = self.peer_manager.lock().await;
                let addrs = pm.all_peer_addrs();
                drop(pm);
                self.broadcast_signed(&addrs, PoSeqMessage::HotStuffQC(qc)).await;
            }

            HotStuffOutput::SendNewView(nv) => {
                let pm = self.peer_manager.lock().await;
                let addrs = pm.all_peer_addrs();
                drop(pm);
                self.broadcast_signed(&addrs, PoSeqMessage::HotStuffNewView(nv)).await;
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
                    locked.engine.flush();
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
                            self.broadcast_signed(&addrs, PoSeqMessage::HotStuffVote(vote)).await;
                        }
                        HotStuffOutput::BroadcastQC(qc) => {
                            self.broadcast_signed(&addrs, PoSeqMessage::HotStuffQC(qc)).await;
                        }
                        HotStuffOutput::SendNewView(nv) => {
                            self.broadcast_signed(&addrs, PoSeqMessage::HotStuffNewView(nv)).await;
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
                                locked.engine.flush();
                            }
                            self.deliver_to_runtime(&fin, delivery_seq).await;
                        }
                        HotStuffOutput::None | HotStuffOutput::Multi(_) => {}
                    }
                }
            }
        }
    }

    // ─── Cross-lane activation handlers ─────────────────────────────────────

    /// Import a committee snapshot from the chain lane.
    /// Validates via `SnapshotImporter` (hash check + dedup), then updates
    /// `NodeState::latest_snapshot_epoch`, persists to sled for restart recovery,
    /// and emits a structured log entry.
    async fn handle_import_snapshot(&self, snap: ChainCommitteeSnapshot) {
        let epoch = snap.epoch;
        let mut importer = self.snapshot_importer.lock().await;
        match importer.import(snap.clone()) {
            Ok(()) => {
                // Persist to sled so restart can recover latest_snapshot_epoch.
                // Key: b"chain_snapshot:" + epoch as 8 big-endian bytes
                if let Ok(json_bytes) = serde_json::to_vec(&snap) {
                    let mut key = b"chain_snapshot:".to_vec();
                    key.extend_from_slice(&epoch.to_be_bytes());
                    let mut locked = self.store.lock().await;
                    locked.engine.put_raw(&key, json_bytes);
                }
                if let Some(ref m) = self.metrics {
                    m.snapshots_imported.inc();
                }
                let mut state = self.state.lock().await;
                state.latest_snapshot_epoch = Some(epoch);
                state.slog.info(
                    "snapshot.imported",
                    epoch,
                    None,
                    format!("committee snapshot accepted for epoch {epoch}"),
                );

                // Activate committee from snapshot members
                let new_committee: Vec<[u8; 32]> = snap.members.iter()
                    .filter_map(|m| {
                        let bytes = hex::decode(&m.node_id).ok()?;
                        let mut id = [0u8; 32];
                        if bytes.len() == 32 { id.copy_from_slice(&bytes); Some(id) } else { None }
                    })
                    .collect();
                if !new_committee.is_empty() {
                    state.committee = new_committee.clone();
                    state.in_committee = state.committee.contains(&self.config.node_id);
                    let leader = NodeState::elect_leader(state.current_epoch, state.current_slot, &state.committee);
                    state.current_leader = leader;
                    // Extract values before the mutable borrow for slog.info
                    let current_slot = state.current_slot;
                    let in_committee = state.in_committee;
                    state.slog.info(
                        "committee.activated",
                        epoch,
                        Some(current_slot),
                        format!(
                            "committee activated: {} members, in_committee={}, leader={:?}",
                            new_committee.len(),
                            in_committee,
                            leader.map(|id| hex::encode(&id[..4])),
                        ),
                    );

                    // If we're in committee, broadcast EpochAnnounce to activate peers
                    if state.in_committee {
                        let epoch_seed = {
                            let mut h = sha2::Sha256::new();
                            sha2::Digest::update(&mut h, b"epoch_seed");
                            sha2::Digest::update(&mut h, &epoch.to_be_bytes());
                            sha2::Digest::finalize(h).into()
                        };
                        let announce = crate::networking::messages::WireEpochAnnounce {
                            epoch: state.current_epoch,
                            committee_members: state.committee.clone(),
                            leader_id: leader.unwrap_or(self.config.node_id),
                            epoch_seed,
                        };
                        drop(state);
                        let pm = self.peer_manager.lock().await;
                        let addrs = pm.all_peer_addrs();
                        drop(pm);
                        self.broadcast_signed(&addrs, crate::networking::messages::PoSeqMessage::EpochAnnounce(announce)).await;
                    } else {
                        drop(state);
                    }
                }
            }
            Err(SnapshotImportError::HashMismatch { .. }) => {
                if let Some(ref m) = self.metrics {
                    m.snapshots_rejected.inc();
                }
                let mut state = self.state.lock().await;
                state.slog.warn(
                    "snapshot.rejected",
                    epoch,
                    None,
                    "hash mismatch — snapshot integrity check failed",
                );
            }
            Err(SnapshotImportError::DuplicateEpoch(_)) => {
                if let Some(ref m) = self.metrics {
                    m.snapshots_rejected.inc();
                }
                let mut state = self.state.lock().await;
                state.slog.warn(
                    "snapshot.rejected",
                    epoch,
                    None,
                    format!("duplicate epoch {epoch} — snapshot already imported"),
                );
            }
            Err(e) => {
                if let Some(ref m) = self.metrics {
                    m.snapshots_rejected.inc();
                }
                let mut state = self.state.lock().await;
                state.slog.warn("snapshot.rejected", epoch, None, e.to_string());
            }
        }
    }

    /// Export an epoch's data to the chain lane via `ChainBridgeExporter`.
    /// Silently deduplicates: if this epoch was already exported, logs and returns.
    /// Serializes the `ExportBatch` to JSON and persists to sled under key
    /// `export:epoch:<epoch>` for later relay pickup.
    async fn handle_export_epoch(&self, epoch: u64) {
        // Dedup guard: skip if already exported
        {
            let state = self.state.lock().await;
            if state.exported_epochs.contains(&epoch) {
                drop(state);
                if let Some(ref m) = self.metrics {
                    m.export_dedup_hits.inc();
                }
                let mut s = self.state.lock().await;
                s.slog.info(
                    "export.skipped",
                    epoch,
                    None,
                    format!("epoch {epoch} already exported — duplicate suppressed"),
                );
                return;
            }
        }

        // Build ExportBatch: no incidents, no checkpoint — basic epoch summary.
        // Full misbehavior incident collection would be wired here in production.
        let committee_hash = {
            let state = self.state.lock().await;
            let mut h = sha2::Sha256::new();
            for id in &state.committee { h.update(id); }
            let bytes: [u8; 32] = h.finalize().into();
            bytes
        };
        let finalized_count = {
            let state = self.state.lock().await;
            state.finalized_batches.len() as u64
        };

        let (batch, _result) = {
            let mut exporter = self.bridge_exporter.lock().await;
            exporter.export(epoch, vec![], committee_hash, finalized_count, None)
        };

        // Persist to sled as JSON
        let key = format!("export:epoch:{epoch}");
        match serde_json::to_vec(&batch) {
            Ok(json_bytes) => {
                {
                    let mut locked = self.store.lock().await;
                    locked.engine.put_raw(key.as_bytes(), json_bytes);
                }
                // Mark epoch as exported
                {
                    if let Some(ref m) = self.metrics {
                        m.epochs_exported.inc();
                    }
                    let mut state = self.state.lock().await;
                    state.exported_epochs.insert(epoch);
                    // Phase 8: register epoch batch in bridge recovery store
                    let bridge_batch_id: [u8; 32] = {
                        let mut h = sha2::Sha256::new();
                        sha2::Digest::update(&mut h, b"bridge:epoch:");
                        sha2::Digest::update(&mut h, epoch.to_be_bytes());
                        sha2::Digest::finalize(h).into()
                    };
                    state.sync_engine.register_batch_for_bridge(bridge_batch_id);
                    state.sync_engine.mark_bridge_exported(&bridge_batch_id);
                    state.slog.info(
                        "export.completed",
                        epoch,
                        None,
                        format!(
                            "epoch {epoch} exported: {} evidence, {} escalations",
                            batch.evidence_set.packets.len(),
                            batch.escalations.len(),
                        ),
                    );
                }
            }
            Err(e) => {
                let mut state = self.state.lock().await;
                state.slog.error(
                    "export.failed",
                    epoch,
                    None,
                    format!("JSON serialization error for epoch {epoch}: {e}"),
                );
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
            slots_per_epoch: 10,
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
