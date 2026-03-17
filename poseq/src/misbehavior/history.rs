#![allow(dead_code)]

use std::collections::BTreeMap;

use super::types::{MisbehaviorCase, MisbehaviorSeverity};

// ---------------------------------------------------------------------------
// NodeMisbehaviorHistory
// ---------------------------------------------------------------------------

pub struct NodeMisbehaviorHistory {
    records: BTreeMap<[u8; 32], Vec<MisbehaviorCase>>,
}

impl NodeMisbehaviorHistory {
    pub fn new() -> Self {
        NodeMisbehaviorHistory {
            records: BTreeMap::new(),
        }
    }

    pub fn record(&mut self, case: MisbehaviorCase) {
        self.records
            .entry(case.evidence.node_id)
            .or_default()
            .push(case);
    }

    pub fn cases_for(&self, node_id: &[u8; 32]) -> &[MisbehaviorCase] {
        self.records
            .get(node_id)
            .map(|v| v.as_slice())
            .unwrap_or(&[])
    }

    pub fn critical_count(&self, node_id: &[u8; 32]) -> usize {
        self.cases_for(node_id)
            .iter()
            .filter(|c| c.severity == MisbehaviorSeverity::Critical)
            .count()
    }

    pub fn has_unresolved_slashable(&self, node_id: &[u8; 32]) -> bool {
        self.cases_for(node_id)
            .iter()
            .any(|c| !c.resolved && c.evidence.evidence_type.is_slashable())
    }

    pub fn resolve_case(&mut self, node_id: &[u8; 32], case_id: &[u8; 32]) -> bool {
        if let Some(cases) = self.records.get_mut(node_id) {
            for case in cases.iter_mut() {
                if &case.case_id == case_id {
                    case.resolved = true;
                    return true;
                }
            }
        }
        false
    }

    pub fn all_unresolved(&self) -> Vec<&MisbehaviorCase> {
        self.records
            .values()
            .flat_map(|v| v.iter())
            .filter(|c| !c.resolved)
            .collect()
    }
}

impl Default for NodeMisbehaviorHistory {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// MisbehaviorError
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum MisbehaviorError {
    CaseAlreadyExists([u8; 32]),
    NodeNotFound([u8; 32]),
    CaseNotFound([u8; 32]),
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::misbehavior::types::{MisbehaviorEvidence, MisbehaviorType};

    fn node(n: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = n;
        id
    }

    fn make_case(node_id: [u8; 32], mtype: MisbehaviorType, epoch: u64) -> MisbehaviorCase {
        let evidence = MisbehaviorEvidence::new(node_id, mtype, epoch, None, "details".into());
        MisbehaviorCase::new(evidence)
    }

    #[test]
    fn test_record_and_retrieve() {
        let mut history = NodeMisbehaviorHistory::new();
        let case = make_case(node(1), MisbehaviorType::Equivocation, 1);
        history.record(case);
        assert_eq!(history.cases_for(&node(1)).len(), 1);
    }

    #[test]
    fn test_critical_count() {
        let mut history = NodeMisbehaviorHistory::new();
        history.record(make_case(node(2), MisbehaviorType::Equivocation, 1));
        history.record(make_case(node(2), MisbehaviorType::ReplayAttack, 2));
        history.record(make_case(node(2), MisbehaviorType::AbsentFromDuty, 3));
        assert_eq!(history.critical_count(&node(2)), 2);
    }

    #[test]
    fn test_has_unresolved_slashable() {
        let mut history = NodeMisbehaviorHistory::new();
        history.record(make_case(node(3), MisbehaviorType::SlotHijackingAttempt, 1));
        assert!(history.has_unresolved_slashable(&node(3)));
    }

    #[test]
    fn test_resolve_case() {
        let mut history = NodeMisbehaviorHistory::new();
        let case = make_case(node(4), MisbehaviorType::Equivocation, 1);
        let case_id = case.case_id;
        history.record(case);
        assert!(history.resolve_case(&node(4), &case_id));
        assert!(!history.cases_for(&node(4))[0].resolved == false); // it's true now
        assert!(history.cases_for(&node(4))[0].resolved);
    }

    #[test]
    fn test_resolve_case_not_found() {
        let mut history = NodeMisbehaviorHistory::new();
        history.record(make_case(node(5), MisbehaviorType::Equivocation, 1));
        let result = history.resolve_case(&node(5), &[0xFFu8; 32]);
        assert!(!result);
    }

    #[test]
    fn test_all_unresolved() {
        let mut history = NodeMisbehaviorHistory::new();
        let case1 = make_case(node(6), MisbehaviorType::Equivocation, 1);
        let case2 = make_case(node(7), MisbehaviorType::ReplayAttack, 2);
        history.record(case1);
        history.record(case2);
        assert_eq!(history.all_unresolved().len(), 2);
    }

    #[test]
    fn test_resolved_excluded_from_unresolved_list() {
        let mut history = NodeMisbehaviorHistory::new();
        let case = make_case(node(8), MisbehaviorType::Equivocation, 1);
        let case_id = case.case_id;
        history.record(case);
        history.resolve_case(&node(8), &case_id);
        assert_eq!(history.all_unresolved().len(), 0);
    }

    #[test]
    fn test_empty_node_returns_empty_slice() {
        let history = NodeMisbehaviorHistory::new();
        assert_eq!(history.cases_for(&node(99)).len(), 0);
        assert_eq!(history.critical_count(&node(99)), 0);
        assert!(!history.has_unresolved_slashable(&node(99)));
    }

    #[test]
    fn test_slashable_types() {
        assert!(MisbehaviorType::Equivocation.is_slashable());
        assert!(MisbehaviorType::SlotHijackingAttempt.is_slashable());
        assert!(MisbehaviorType::RuntimeBridgeAbuse.is_slashable());
        assert!(MisbehaviorType::ReplayAttack.is_slashable());
        assert!(!MisbehaviorType::AbsentFromDuty.is_slashable());
        assert!(!MisbehaviorType::FairnessViolation.is_slashable());
    }

    #[test]
    fn test_evidence_hash_deterministic() {
        let h1 = MisbehaviorEvidence::compute_hash(&node(1), &MisbehaviorType::Equivocation, 5, Some(10));
        let h2 = MisbehaviorEvidence::compute_hash(&node(1), &MisbehaviorType::Equivocation, 5, Some(10));
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_evidence_hash_differs_by_slot() {
        let h1 = MisbehaviorEvidence::compute_hash(&node(1), &MisbehaviorType::Equivocation, 5, Some(10));
        let h2 = MisbehaviorEvidence::compute_hash(&node(1), &MisbehaviorType::Equivocation, 5, Some(11));
        assert_ne!(h1, h2);
    }

    #[test]
    fn test_multiple_cases_same_node() {
        let mut history = NodeMisbehaviorHistory::new();
        history.record(make_case(node(10), MisbehaviorType::AbsentFromDuty, 1));
        history.record(make_case(node(10), MisbehaviorType::AbsentFromDuty, 2));
        history.record(make_case(node(10), MisbehaviorType::Equivocation, 3));
        assert_eq!(history.cases_for(&node(10)).len(), 3);
        assert_eq!(history.critical_count(&node(10)), 1);
    }
}
