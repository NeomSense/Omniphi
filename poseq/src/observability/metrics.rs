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
}
