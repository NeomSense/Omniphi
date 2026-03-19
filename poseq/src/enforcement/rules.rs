use crate::chain_bridge::exporter::StatusRecommendation;
use crate::liveness::events::InactivityEvent;
use crate::performance::record::NodePerformanceRecord;

/// Configuration for automatic enforcement rules.
///
/// All thresholds are integers. Setting a threshold to 0 disables that rule.
#[derive(Debug, Clone)]
pub struct EnforcementConfig {
    /// Number of consecutive missed epochs before emitting a Suspended recommendation.
    /// 0 = disabled. Default: 4.
    pub inactivity_suspend_threshold: u64,

    /// Number of fault events in a single epoch before emitting a Jailed recommendation.
    /// 0 = disabled. Default: 5.
    pub fault_jail_threshold: u64,

    /// Minimum participation rate in basis points below which a Suspended recommendation
    /// is emitted (even if not yet inactivity-threshold). 0 = disabled.
    /// Default: 1000 (10% participation).
    pub min_participation_bps: u32,
}

impl Default for EnforcementConfig {
    fn default() -> Self {
        EnforcementConfig {
            inactivity_suspend_threshold: 4,
            fault_jail_threshold: 5,
            min_participation_bps: 1_000,
        }
    }
}

/// Evaluate inactivity events against enforcement rules.
///
/// Returns `StatusRecommendation`s for any node that exceeds the inactivity threshold.
/// Deterministic: same inputs always produce the same outputs.
pub fn evaluate_inactivity(
    events: &[InactivityEvent],
    config: &EnforcementConfig,
    epoch: u64,
) -> Vec<StatusRecommendation> {
    if config.inactivity_suspend_threshold == 0 {
        return Vec::new();
    }

    events
        .iter()
        .filter(|e| e.missed_epochs > config.inactivity_suspend_threshold)
        .map(|e| StatusRecommendation {
            node_id: e.node_id,
            recommended_status: "Suspended".to_string(),
            reason: format!(
                "node missed {} consecutive epochs (threshold: {})",
                e.missed_epochs, config.inactivity_suspend_threshold
            ),
            epoch,
        })
        .collect()
}

/// Evaluate performance records against enforcement rules.
///
/// Returns `StatusRecommendation`s for nodes that:
/// - exceed the fault jail threshold, OR
/// - have participation below the minimum (if threshold > 0)
pub fn evaluate_performance(
    records: &[NodePerformanceRecord],
    config: &EnforcementConfig,
    epoch: u64,
) -> Vec<StatusRecommendation> {
    let mut recs = Vec::new();

    for pr in records {
        // Fault threshold → Jail
        if config.fault_jail_threshold > 0
            && pr.fault_events >= config.fault_jail_threshold
        {
            recs.push(StatusRecommendation {
                node_id: pr.node_id,
                recommended_status: "Jailed".to_string(),
                reason: format!(
                    "node recorded {} fault events (threshold: {})",
                    pr.fault_events, config.fault_jail_threshold
                ),
                epoch,
            });
            continue; // Jail takes precedence over suspend for same node
        }

        // Low participation → Suspend
        if config.min_participation_bps > 0
            && pr.participation_rate_bps < config.min_participation_bps
            && pr.participation_rate_bps > 0 // zero may mean node wasn't eligible
        {
            recs.push(StatusRecommendation {
                node_id: pr.node_id,
                recommended_status: "Suspended".to_string(),
                reason: format!(
                    "participation rate {}bps below minimum {}bps",
                    pr.participation_rate_bps, config.min_participation_bps
                ),
                epoch,
            });
        }
    }

    recs
}

/// Run all enforcement rules against a set of epoch accountability events.
///
/// Returns the combined list of status recommendations, deduplicated by node_id
/// (Jailed takes precedence over Suspended for the same node).
pub fn evaluate_epoch(
    inactivity_events: &[InactivityEvent],
    performance_records: &[NodePerformanceRecord],
    config: &EnforcementConfig,
    epoch: u64,
) -> Vec<StatusRecommendation> {
    let inactivity_recs = evaluate_inactivity(inactivity_events, config, epoch);
    let perf_recs = evaluate_performance(performance_records, config, epoch);

    // Merge: Jailed takes precedence over Suspended for the same node.
    let mut by_node: std::collections::BTreeMap<[u8; 32], StatusRecommendation> =
        std::collections::BTreeMap::new();

    for rec in inactivity_recs.into_iter().chain(perf_recs.into_iter()) {
        let entry = by_node.entry(rec.node_id).or_insert_with(|| rec.clone());
        // Upgrade to Jailed if a Jailed rec comes in for same node
        if rec.recommended_status == "Jailed" && entry.recommended_status == "Suspended" {
            *entry = rec;
        }
    }

    by_node.into_values().collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::liveness::events::InactivityEvent;
    use crate::performance::record::NodePerformanceRecord;

    fn nid(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn inactivity(node: u8, missed: u64) -> InactivityEvent {
        InactivityEvent {
            node_id: nid(node),
            detected_at_epoch: 10,
            last_active_epoch: 10u64.saturating_sub(missed),
            missed_epochs: missed,
        }
    }

    fn perf(node: u8, participation_bps: u32, faults: u64) -> NodePerformanceRecord {
        NodePerformanceRecord {
            node_id: nid(node),
            epoch: 10,
            proposals_count: 1,
            attestations_count: participation_bps as u64 / 100,
            missed_attestations: 0,
            fault_events: faults,
            participation_rate_bps: participation_bps,
        }
    }

    #[test]
    fn test_inactivity_above_threshold_suspended() {
        let config = EnforcementConfig { inactivity_suspend_threshold: 3, ..Default::default() };
        let events = vec![inactivity(1, 5)]; // 5 > 3
        let recs = evaluate_inactivity(&events, &config, 10);
        assert_eq!(recs.len(), 1);
        assert_eq!(recs[0].recommended_status, "Suspended");
        assert_eq!(recs[0].node_id, nid(1));
    }

    #[test]
    fn test_inactivity_at_threshold_not_triggered() {
        let config = EnforcementConfig { inactivity_suspend_threshold: 4, ..Default::default() };
        let events = vec![inactivity(1, 4)]; // 4 == threshold → NOT triggered (must exceed)
        let recs = evaluate_inactivity(&events, &config, 10);
        assert_eq!(recs.len(), 0);
    }

    #[test]
    fn test_inactivity_disabled() {
        let config = EnforcementConfig { inactivity_suspend_threshold: 0, ..Default::default() };
        let events = vec![inactivity(1, 100)];
        let recs = evaluate_inactivity(&events, &config, 10);
        assert!(recs.is_empty());
    }

    #[test]
    fn test_fault_threshold_jailed() {
        let config = EnforcementConfig { fault_jail_threshold: 5, ..Default::default() };
        let records = vec![perf(2, 5_000, 5)]; // 5 >= 5
        let recs = evaluate_performance(&records, &config, 10);
        assert_eq!(recs.len(), 1);
        assert_eq!(recs[0].recommended_status, "Jailed");
    }

    #[test]
    fn test_fault_below_threshold_no_jail() {
        let config = EnforcementConfig { fault_jail_threshold: 5, ..Default::default() };
        let records = vec![perf(2, 5_000, 4)]; // 4 < 5
        let recs = evaluate_performance(&records, &config, 10);
        // May still trigger min_participation check — check only for jail
        assert!(!recs.iter().any(|r| r.recommended_status == "Jailed"));
    }

    #[test]
    fn test_low_participation_suspended() {
        let config = EnforcementConfig {
            min_participation_bps: 1_000,
            fault_jail_threshold: 10,
            ..Default::default()
        };
        let records = vec![perf(3, 500, 0)]; // 500 < 1000
        let recs = evaluate_performance(&records, &config, 10);
        assert_eq!(recs.len(), 1);
        assert_eq!(recs[0].recommended_status, "Suspended");
    }

    #[test]
    fn test_zero_participation_not_suspended() {
        // participation_rate_bps == 0 means the node wasn't eligible — skip
        let config = EnforcementConfig { min_participation_bps: 1_000, ..Default::default() };
        let records = vec![perf(4, 0, 0)];
        let recs = evaluate_performance(&records, &config, 10);
        assert!(recs.is_empty());
    }

    #[test]
    fn test_jail_takes_precedence_over_suspend() {
        let config = EnforcementConfig {
            inactivity_suspend_threshold: 3,
            fault_jail_threshold: 2,
            min_participation_bps: 0,
        };
        let inactivity_events = vec![inactivity(1, 5)]; // → Suspend
        let perf_records = vec![perf(1, 5_000, 3)];    // → Jail (3 >= 2)
        let recs = evaluate_epoch(&inactivity_events, &perf_records, &config, 10);
        // Same node — should only appear once with Jailed
        assert_eq!(recs.len(), 1);
        assert_eq!(recs[0].recommended_status, "Jailed");
    }

    #[test]
    fn test_evaluate_epoch_multiple_nodes() {
        let config = EnforcementConfig::default();
        let inactivity_events = vec![inactivity(1, 6)]; // node 1: inactivity → Suspend
        let perf_records = vec![
            perf(2, 5_000, 5),  // node 2: faults → Jail
            perf(3, 9_000, 0),  // node 3: healthy → no rec
        ];
        let recs = evaluate_epoch(&inactivity_events, &perf_records, &config, 10);
        assert_eq!(recs.len(), 2);
        let statuses: std::collections::BTreeMap<[u8; 32], &str> = recs
            .iter()
            .map(|r| (r.node_id, r.recommended_status.as_str()))
            .collect();
        assert_eq!(statuses[&nid(1)], "Suspended");
        assert_eq!(statuses[&nid(2)], "Jailed");
    }
}
