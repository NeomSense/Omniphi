use std::collections::{BTreeMap, BTreeSet};
use crate::errors::Phase4Error;
use crate::slashing::offenses::{SlashRecord, SlashableOffense, SlashingConfig, SlashingEngine};
use crate::slashing::jail::{JailRecord, JailStore};

/// Unified slashing store: slash records + jail state.
pub struct SlashingStore {
    /// node_id → Vec<SlashRecord> (all historical offenses)
    records: BTreeMap<[u8; 32], Vec<SlashRecord>>,
    pub jail_store: JailStore,
    pub engine: SlashingEngine,
}

impl SlashingStore {
    pub fn new(config: SlashingConfig) -> Self {
        SlashingStore {
            records: BTreeMap::new(),
            jail_store: JailStore::new(),
            engine: SlashingEngine::new(config),
        }
    }

    /// Process an offense, store the slash record, and jail the node if threshold met.
    pub fn process(
        &mut self,
        offense: SlashableOffense,
        node_id: [u8; 32],
        epoch: u64,
        evidence: &[u8],
    ) -> Result<ProcessResult, Phase4Error> {
        let record = self.engine.process_offense(offense, node_id, epoch, evidence)?;
        self.records.entry(node_id).or_insert_with(Vec::new).push(record.clone());

        let jailed = if self.engine.should_jail(&node_id) && !self.jail_store.is_jailed(&node_id) {
            let cooldown = self.engine.config.unjail_cooldown_epochs;
            self.jail_store.jail(node_id, epoch, cooldown)?;
            true
        } else {
            false
        };

        Ok(ProcessResult { record, jailed })
    }

    /// Unjail a node after cooldown. Also resets cumulative slash.
    pub fn unjail(&mut self, node_id: [u8; 32], current_epoch: u64) -> Result<(), Phase4Error> {
        self.jail_store.unjail(node_id, current_epoch)?;
        self.engine.reset_slash(&node_id);
        Ok(())
    }

    /// Get all slash records for a node.
    pub fn get_records(&self, node_id: &[u8; 32]) -> &[SlashRecord] {
        self.records.get(node_id).map(|v| v.as_slice()).unwrap_or(&[])
    }

    /// Get the jail record for a node if jailed.
    pub fn get_jail_record(&self, node_id: &[u8; 32]) -> Option<&JailRecord> {
        self.jail_store.get_record(node_id)
    }

    /// Is the node currently jailed?
    pub fn is_jailed(&self, node_id: &[u8; 32]) -> bool {
        self.jail_store.is_jailed(node_id)
    }

    /// All currently jailed nodes as a BTreeSet (for exclusion from rotation).
    pub fn jailed_set(&self) -> BTreeSet<[u8; 32]> {
        self.jail_store.jailed_nodes().into_iter().collect()
    }

    /// Total slash records across all nodes.
    pub fn total_offense_count(&self) -> usize {
        self.records.values().map(|v| v.len()).sum()
    }

    /// Offense count for a specific node.
    pub fn offense_count(&self, node_id: &[u8; 32]) -> usize {
        self.records.get(node_id).map(|v| v.len()).unwrap_or(0)
    }
}

/// Result of processing an offense through the store.
#[derive(Debug)]
pub struct ProcessResult {
    pub record: SlashRecord,
    pub jailed: bool,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::slashing::offenses::SlashingConfig;

    fn make_id(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    fn default_store() -> SlashingStore {
        SlashingStore::new(SlashingConfig::default_config())
    }

    #[test]
    fn test_process_single_offense() {
        let mut store = default_store();
        let result = store.process(
            SlashableOffense::AbsentFromDuty, make_id(1), 1, b"slot1"
        ).unwrap();
        assert_eq!(result.record.slash_amount_bps, 50);
        assert!(!result.jailed);
        assert_eq!(store.offense_count(&make_id(1)), 1);
    }

    #[test]
    fn test_jail_triggered_on_threshold() {
        let mut store = default_store();
        let node = make_id(1);
        // DoubleProposal = 500 bps, threshold = 1000 bps → need 2
        store.process(SlashableOffense::DoubleProposal, node, 1, b"e1").unwrap();
        let result = store.process(SlashableOffense::DoubleProposal, node, 2, b"e2").unwrap();
        assert!(result.jailed);
        assert!(store.is_jailed(&node));
    }

    #[test]
    fn test_unjail_after_cooldown() {
        let mut store = default_store();
        let node = make_id(1);
        store.process(SlashableOffense::DoubleProposal, node, 1, b"e1").unwrap();
        store.process(SlashableOffense::DoubleProposal, node, 2, b"e2").unwrap();
        assert!(store.is_jailed(&node));
        // cooldown = 10 epochs, jailed at epoch 2 → eligible at 12
        store.unjail(node, 12).unwrap();
        assert!(!store.is_jailed(&node));
        // Cumulative slash reset
        assert_eq!(store.engine.cumulative_slash_bps(&node), 0);
    }

    #[test]
    fn test_unjail_before_cooldown_fails() {
        let mut store = default_store();
        let node = make_id(1);
        store.process(SlashableOffense::DoubleProposal, node, 1, b"e1").unwrap();
        store.process(SlashableOffense::DoubleProposal, node, 2, b"e2").unwrap();
        let err = store.unjail(node, 5);
        assert!(matches!(err, Err(Phase4Error::UnjailCooldownActive { .. })));
    }

    #[test]
    fn test_get_records_returns_all() {
        let mut store = default_store();
        let node = make_id(1);
        store.process(SlashableOffense::AbsentFromDuty, node, 1, b"a").unwrap();
        store.process(SlashableOffense::ReplayAttack, node, 2, b"b").unwrap();
        let records = store.get_records(&node);
        assert_eq!(records.len(), 2);
    }

    #[test]
    fn test_jailed_set_contains_jailed_nodes() {
        let mut store = default_store();
        let n1 = make_id(1);
        let n2 = make_id(2);
        // Jail n1
        store.process(SlashableOffense::DoubleProposal, n1, 1, b"e").unwrap();
        store.process(SlashableOffense::DoubleProposal, n1, 2, b"e").unwrap();
        let jailed = store.jailed_set();
        assert!(jailed.contains(&n1));
        assert!(!jailed.contains(&n2));
    }

    #[test]
    fn test_total_offense_count() {
        let mut store = default_store();
        store.process(SlashableOffense::AbsentFromDuty, make_id(1), 1, b"a").unwrap();
        store.process(SlashableOffense::AbsentFromDuty, make_id(2), 1, b"b").unwrap();
        store.process(SlashableOffense::AbsentFromDuty, make_id(1), 2, b"c").unwrap();
        assert_eq!(store.total_offense_count(), 3);
    }

    #[test]
    fn test_get_jail_record() {
        let mut store = default_store();
        let node = make_id(1);
        store.process(SlashableOffense::DoubleProposal, node, 5, b"e").unwrap();
        store.process(SlashableOffense::DoubleProposal, node, 5, b"e2").unwrap();
        let rec = store.get_jail_record(&node).unwrap();
        assert_eq!(rec.jailed_at_epoch, 5);
    }

    #[test]
    fn test_multiple_nodes_independent() {
        let mut store = default_store();
        let n1 = make_id(1);
        let n2 = make_id(2);
        store.process(SlashableOffense::FairnessViolation, n1, 1, b"a").unwrap();
        store.process(SlashableOffense::InvalidAttestation, n2, 1, b"b").unwrap();
        assert_eq!(store.offense_count(&n1), 1);
        assert_eq!(store.offense_count(&n2), 1);
        assert_eq!(store.engine.cumulative_slash_bps(&n1), 200);
        assert_eq!(store.engine.cumulative_slash_bps(&n2), 100);
    }

    #[test]
    fn test_no_double_jail() {
        let mut store = default_store();
        let node = make_id(1);
        // Jail on 2nd offense
        store.process(SlashableOffense::DoubleProposal, node, 1, b"e1").unwrap();
        store.process(SlashableOffense::DoubleProposal, node, 2, b"e2").unwrap();
        assert!(store.is_jailed(&node));
        // 3rd offense — should not try to re-jail (already jailed)
        let result = store.process(SlashableOffense::DoubleProposal, node, 3, b"e3");
        // Should succeed but not double-jail
        assert!(result.is_ok());
        assert!(!result.unwrap().jailed); // already jailed, not a new jail event
    }

    #[test]
    fn test_records_empty_for_unknown_node() {
        let store = default_store();
        assert!(store.get_records(&make_id(99)).is_empty());
        assert!(store.get_jail_record(&make_id(99)).is_none());
    }
}
