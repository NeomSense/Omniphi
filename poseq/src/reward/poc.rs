use std::collections::BTreeMap;
use serde::{Deserialize, Serialize};

/// PoC-derived multiplier for an operator at a given epoch.
///
/// This is populated by the chain bridge after importing PoC data from
/// the slow lane. Default is 10000 (1.0x neutral) when no data is available.
///
/// Range: 5000 (0.5x) – 15000 (1.5x).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoCMultiplierRecord {
    pub operator_address: String,
    pub epoch: u64,
    /// Multiplier in basis points: 5000–15000.
    pub multiplier_bps: u32,
    /// Source: "governance", "poc_hook", or "default".
    pub source: String,
}

impl PoCMultiplierRecord {
    pub const DEFAULT_MULTIPLIER_BPS: u32 = 10_000;
    pub const MIN_MULTIPLIER_BPS: u32 = 5_000;
    pub const MAX_MULTIPLIER_BPS: u32 = 15_000;

    pub fn default_for(operator_address: String, epoch: u64) -> Self {
        PoCMultiplierRecord {
            operator_address,
            epoch,
            multiplier_bps: Self::DEFAULT_MULTIPLIER_BPS,
            source: "default".into(),
        }
    }

    /// Clamp multiplier to valid range.
    pub fn clamped_multiplier(&self) -> u32 {
        self.multiplier_bps
            .clamp(Self::MIN_MULTIPLIER_BPS, Self::MAX_MULTIPLIER_BPS)
    }
}

/// In-memory cache of PoC multipliers, keyed by (epoch, operator_address).
pub struct PoCMultiplierStore {
    records: BTreeMap<(u64, String), PoCMultiplierRecord>,
}

impl PoCMultiplierStore {
    pub fn new() -> Self {
        PoCMultiplierStore {
            records: BTreeMap::new(),
        }
    }

    pub fn set(&mut self, rec: PoCMultiplierRecord) {
        self.records.insert((rec.epoch, rec.operator_address.clone()), rec);
    }

    /// Get the PoC multiplier for (epoch, operator). Returns DEFAULT (10000) if not set.
    pub fn get(&self, epoch: u64, operator_address: &str) -> u32 {
        self.records
            .get(&(epoch, operator_address.to_string()))
            .map(|r| r.clamped_multiplier())
            .unwrap_or(PoCMultiplierRecord::DEFAULT_MULTIPLIER_BPS)
    }

    /// Get the full record, if present.
    pub fn get_record(&self, epoch: u64, operator_address: &str) -> Option<&PoCMultiplierRecord> {
        self.records.get(&(epoch, operator_address.to_string()))
    }
}

impl Default for PoCMultiplierStore {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_multiplier_when_missing() {
        let store = PoCMultiplierStore::new();
        assert_eq!(store.get(1, "omni1op"), 10_000);
    }

    #[test]
    fn test_set_and_get() {
        let mut store = PoCMultiplierStore::new();
        store.set(PoCMultiplierRecord {
            operator_address: "omni1op".into(),
            epoch: 5,
            multiplier_bps: 12_000,
            source: "poc_hook".into(),
        });
        assert_eq!(store.get(5, "omni1op"), 12_000);
        assert_eq!(store.get(4, "omni1op"), 10_000); // different epoch
    }

    #[test]
    fn test_clamp_below_min() {
        let rec = PoCMultiplierRecord {
            operator_address: "omni1op".into(),
            epoch: 1,
            multiplier_bps: 1_000, // below 5000
            source: "governance".into(),
        };
        assert_eq!(rec.clamped_multiplier(), 5_000);
    }

    #[test]
    fn test_clamp_above_max() {
        let rec = PoCMultiplierRecord {
            operator_address: "omni1op".into(),
            epoch: 1,
            multiplier_bps: 99_000, // above 15000
            source: "governance".into(),
        };
        assert_eq!(rec.clamped_multiplier(), 15_000);
    }
}
