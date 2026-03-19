use serde::{Deserialize, Serialize};

/// Per-node performance summary for one epoch.
///
/// All rates are integer basis points (0–10000). No floating-point arithmetic.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodePerformanceRecord {
    pub node_id: [u8; 32],
    pub epoch: u64,
    pub proposals_count: u64,
    pub attestations_count: u64,
    pub missed_attestations: u64,
    pub fault_events: u64,
    /// Participation rate in basis points: `(attestations * 10_000) / eligible_slots`.
    /// 0 if `eligible_slots == 0`.
    pub participation_rate_bps: u32,
}

impl NodePerformanceRecord {
    /// Compute participation rate as integer basis points.
    /// Returns 0 if `eligible_slots == 0` (no divide-by-zero).
    pub fn compute_participation_rate(attestations: u64, eligible_slots: u64) -> u32 {
        if eligible_slots == 0 {
            return 0;
        }
        let rate = (attestations.saturating_mul(10_000)) / eligible_slots;
        // Clamp to u32 max (should never exceed 10_000 but saturate defensively)
        rate.min(10_000) as u32
    }
}
