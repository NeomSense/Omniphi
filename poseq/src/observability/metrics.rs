//! Prometheus metrics for the PoSeq node.
//!
//! # Metrics exported
//!
//! | Metric                           | Type      | Description |
//! |----------------------------------|-----------|-------------|
//! | `poseq_batches_finalized_total`  | Counter   | Total finalized batches |
//! | `poseq_view_changes_total`       | Counter   | Total HotStuff view changes |
//! | `poseq_attestation_latency_ms`   | Histogram | Time from proposal to quorum |
//! | `poseq_fairness_violations_total`| Counter   | Detected fairness violations |
//! | `poseq_runtime_rejections_total` | Counter   | Runtime ingester rejections |
//! | `poseq_peer_count`               | Gauge     | Current connected peer count |
//! | `poseq_current_epoch`            | Gauge     | Current consensus epoch |
//! | `poseq_current_slot`             | Gauge     | Current slot within epoch |
//! | `poseq_store_finalized_count`    | Counter   | Batches persisted to sled |
//! | `poseq_snapshots_imported_total` | Counter   | Committee snapshots accepted |
//! | `poseq_snapshots_rejected_total` | Counter   | Committee snapshots rejected (hash/dup) |
//! | `poseq_epochs_exported_total`    | Counter   | Epoch exports completed |
//! | `poseq_export_dedup_hits_total`  | Counter   | Export dedup blocks (replay suppressed) |
//! | `poseq_node_restarts_total`      | Counter   | Warm restarts detected via sled load |
//! | `poseq_bridge_acked_total`       | Counter   | Bridge ACKs consumed from Go chain |
//! | `poseq_bridge_ack_duplicates_total`| Counter | Duplicate/stale bridge ACKs received |

use prometheus::{
    self, Counter, Gauge, Histogram, HistogramOpts, IntCounter, IntGauge, Opts, Registry,
};
use std::sync::Arc;

/// All PoSeq Prometheus metrics.
#[derive(Clone)]
pub struct PoSeqMetrics {
    /// Total finalized batches.
    pub batches_finalized: IntCounter,
    /// Total HotStuff view changes (leader timeouts).
    pub view_changes: IntCounter,
    /// Attestation round-trip latency in milliseconds.
    pub attestation_latency_ms: Histogram,
    /// Detected fairness violations.
    pub fairness_violations: IntCounter,
    /// Runtime ingester rejections (idempotency or validation failures).
    pub runtime_rejections: IntCounter,
    /// Current connected peer count.
    pub peer_count: IntGauge,
    /// Current consensus epoch.
    pub current_epoch: IntGauge,
    /// Current slot.
    pub current_slot: IntGauge,
    /// Batches written to durable store.
    pub store_writes: IntCounter,

    // ── Cross-lane / devnet metrics ──────────────────────────────────────────
    /// Committee snapshots accepted from chain lane.
    pub snapshots_imported: IntCounter,
    /// Committee snapshots rejected (hash mismatch or duplicate epoch).
    pub snapshots_rejected: IntCounter,
    /// Epoch exports completed and persisted to sled.
    pub epochs_exported: IntCounter,
    /// Export dedup hits — re-trigger of already-exported epoch was suppressed.
    pub export_dedup_hits: IntCounter,
    /// Warm restarts: boot with existing sled state (non-zero restored epochs or snapshots).
    pub node_restarts: IntCounter,

    // ── Per-state peer counts ─────────────────────────────────────────────────
    /// Number of peers in Connected state.
    pub peers_connected: IntGauge,
    /// Number of peers in Degraded state.
    pub peers_degraded: IntGauge,
    /// Number of peers in Disconnected state.
    pub peers_disconnected: IntGauge,

    // ── Phase 8: State Sync metrics ───────────────────────────────────────────
    /// Epoch lag behind peer_max_epoch (0 = in sync).
    pub sync_lag_epochs: IntGauge,
    /// Bridge batches currently pending delivery.
    pub bridge_backlog: IntGauge,
    /// Total bridge batches that reached Failed state.
    pub bridge_failed_total: IntCounter,
    /// Total bridge ACKs successfully consumed from Go chain.
    pub bridge_acked_total: IntCounter,
    /// Bridge ACKs received for already-acked or unknown batches (duplicates).
    pub bridge_ack_duplicates: IntCounter,

    /// The registry holding all metrics (used by the HTTP exporter).
    pub registry: Arc<Registry>,
}

impl PoSeqMetrics {
    /// Create and register all metrics in a new `Registry`.
    pub fn new() -> Result<Self, prometheus::Error> {
        let registry = Registry::new();

        let batches_finalized = IntCounter::with_opts(
            Opts::new("poseq_batches_finalized_total", "Total batches finalized by this node"),
        )?;
        registry.register(Box::new(batches_finalized.clone()))?;

        let view_changes = IntCounter::with_opts(
            Opts::new("poseq_view_changes_total", "Total HotStuff view changes (timeouts)"),
        )?;
        registry.register(Box::new(view_changes.clone()))?;

        let attestation_latency_ms = Histogram::with_opts(
            HistogramOpts::new(
                "poseq_attestation_latency_ms",
                "Latency from proposal broadcast to quorum reached, in milliseconds",
            )
            .buckets(vec![1.0, 5.0, 10.0, 25.0, 50.0, 100.0, 250.0, 500.0, 1000.0, 2500.0, 5000.0]),
        )?;
        registry.register(Box::new(attestation_latency_ms.clone()))?;

        let fairness_violations = IntCounter::with_opts(
            Opts::new("poseq_fairness_violations_total", "Detected ordering fairness violations"),
        )?;
        registry.register(Box::new(fairness_violations.clone()))?;

        let runtime_rejections = IntCounter::with_opts(
            Opts::new("poseq_runtime_rejections_total", "Runtime ingester batch rejections"),
        )?;
        registry.register(Box::new(runtime_rejections.clone()))?;

        let peer_count = IntGauge::with_opts(
            Opts::new("poseq_peer_count", "Number of currently connected peers"),
        )?;
        registry.register(Box::new(peer_count.clone()))?;

        let current_epoch = IntGauge::with_opts(
            Opts::new("poseq_current_epoch", "Current consensus epoch"),
        )?;
        registry.register(Box::new(current_epoch.clone()))?;

        let current_slot = IntGauge::with_opts(
            Opts::new("poseq_current_slot", "Current slot within epoch"),
        )?;
        registry.register(Box::new(current_slot.clone()))?;

        let store_writes = IntCounter::with_opts(
            Opts::new("poseq_store_finalized_count", "Finalized batches written to durable store"),
        )?;
        registry.register(Box::new(store_writes.clone()))?;

        let snapshots_imported = IntCounter::with_opts(
            Opts::new("poseq_snapshots_imported_total", "Committee snapshots accepted from chain"),
        )?;
        registry.register(Box::new(snapshots_imported.clone()))?;

        let snapshots_rejected = IntCounter::with_opts(
            Opts::new("poseq_snapshots_rejected_total", "Committee snapshots rejected (hash/dup)"),
        )?;
        registry.register(Box::new(snapshots_rejected.clone()))?;

        let epochs_exported = IntCounter::with_opts(
            Opts::new("poseq_epochs_exported_total", "Epoch exports completed to chain lane"),
        )?;
        registry.register(Box::new(epochs_exported.clone()))?;

        let export_dedup_hits = IntCounter::with_opts(
            Opts::new("poseq_export_dedup_hits_total", "Export dedup: already-exported epoch suppressed"),
        )?;
        registry.register(Box::new(export_dedup_hits.clone()))?;

        let node_restarts = IntCounter::with_opts(
            Opts::new("poseq_node_restarts_total", "Warm restarts (non-zero sled state restored)"),
        )?;
        registry.register(Box::new(node_restarts.clone()))?;

        let peers_connected = IntGauge::with_opts(
            Opts::new("poseq_peers_connected", "Number of peers in Connected state"),
        )?;
        registry.register(Box::new(peers_connected.clone()))?;

        let peers_degraded = IntGauge::with_opts(
            Opts::new("poseq_peers_degraded", "Number of peers in Degraded state"),
        )?;
        registry.register(Box::new(peers_degraded.clone()))?;

        let peers_disconnected = IntGauge::with_opts(
            Opts::new("poseq_peers_disconnected", "Number of peers in Disconnected state"),
        )?;
        registry.register(Box::new(peers_disconnected.clone()))?;

        let sync_lag_epochs = IntGauge::with_opts(
            Opts::new("poseq_sync_lag_epochs", "Epoch lag behind peer_max_epoch (0 = in sync)"),
        )?;
        registry.register(Box::new(sync_lag_epochs.clone()))?;

        let bridge_backlog = IntGauge::with_opts(
            Opts::new("poseq_bridge_backlog", "Bridge batches currently pending delivery"),
        )?;
        registry.register(Box::new(bridge_backlog.clone()))?;

        let bridge_failed_total = IntCounter::with_opts(
            Opts::new("poseq_bridge_failed_total", "Total bridge batches that reached Failed state"),
        )?;
        registry.register(Box::new(bridge_failed_total.clone()))?;

        let bridge_acked_total = IntCounter::with_opts(
            Opts::new("poseq_bridge_acked_total", "Total bridge ACKs consumed from Go chain"),
        )?;
        registry.register(Box::new(bridge_acked_total.clone()))?;

        let bridge_ack_duplicates = IntCounter::with_opts(
            Opts::new("poseq_bridge_ack_duplicates_total", "Duplicate or stale bridge ACKs received"),
        )?;
        registry.register(Box::new(bridge_ack_duplicates.clone()))?;

        Ok(PoSeqMetrics {
            batches_finalized,
            view_changes,
            attestation_latency_ms,
            fairness_violations,
            runtime_rejections,
            peer_count,
            current_epoch,
            current_slot,
            store_writes,
            snapshots_imported,
            snapshots_rejected,
            epochs_exported,
            export_dedup_hits,
            node_restarts,
            peers_connected,
            peers_degraded,
            peers_disconnected,
            sync_lag_epochs,
            bridge_backlog,
            bridge_failed_total,
            bridge_acked_total,
            bridge_ack_duplicates,
            registry: Arc::new(registry),
        })
    }

    /// Render all metrics in Prometheus text format.
    pub fn render(&self) -> String {
        use prometheus::Encoder;
        let encoder = prometheus::TextEncoder::new();
        let mut buffer = Vec::new();
        let mf = self.registry.gather();
        encoder.encode(&mf, &mut buffer).unwrap_or_default();
        String::from_utf8(buffer).unwrap_or_default()
    }
}

impl Default for PoSeqMetrics {
    fn default() -> Self {
        Self::new().expect("metrics registration failed")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_metrics_create_and_render() {
        let m = PoSeqMetrics::new().unwrap();
        m.batches_finalized.inc();
        m.batches_finalized.inc();
        m.view_changes.inc();
        m.peer_count.set(3);
        m.current_epoch.set(5);
        m.current_slot.set(42);
        let rendered = m.render();
        assert!(rendered.contains("poseq_batches_finalized_total 2"));
        assert!(rendered.contains("poseq_view_changes_total 1"));
        assert!(rendered.contains("poseq_peer_count 3"));
        assert!(rendered.contains("poseq_current_epoch 5"));
    }

    #[test]
    fn test_attestation_latency_histogram() {
        let m = PoSeqMetrics::new().unwrap();
        m.attestation_latency_ms.observe(45.0);
        m.attestation_latency_ms.observe(150.0);
        let rendered = m.render();
        assert!(rendered.contains("poseq_attestation_latency_ms_count 2"));
    }

    #[test]
    fn test_fairness_and_runtime_counters() {
        let m = PoSeqMetrics::new().unwrap();
        m.fairness_violations.inc_by(3);
        m.runtime_rejections.inc();
        let rendered = m.render();
        assert!(rendered.contains("poseq_fairness_violations_total 3"));
        assert!(rendered.contains("poseq_runtime_rejections_total 1"));
    }

    #[test]
    fn test_peer_state_gauges() {
        let m = PoSeqMetrics::new().unwrap();
        m.peers_connected.set(5);
        m.peers_degraded.set(2);
        m.peers_disconnected.set(1);
        let rendered = m.render();
        assert!(rendered.contains("poseq_peers_connected 5"));
        assert!(rendered.contains("poseq_peers_degraded 2"));
        assert!(rendered.contains("poseq_peers_disconnected 1"));
    }

    #[test]
    fn test_peer_state_gauges_initial_zero() {
        let m = PoSeqMetrics::new().unwrap();
        let rendered = m.render();
        assert!(rendered.contains("poseq_peers_connected 0"));
        assert!(rendered.contains("poseq_peers_degraded 0"));
        assert!(rendered.contains("poseq_peers_disconnected 0"));
    }
}
