use crate::objects::base::{AccessMode, ObjectId};
use crate::resolution::planner::ExecutionPlan;
use std::collections::{BTreeMap, BTreeSet};

/// A directed conflict graph: tx_id → set of conflicting tx_ids.
#[derive(Debug, Default)]
pub struct ConflictGraph {
    pub adjacency: BTreeMap<[u8; 32], BTreeSet<[u8; 32]>>,
}

impl ConflictGraph {
    pub fn new() -> Self {
        ConflictGraph {
            adjacency: BTreeMap::new(),
        }
    }

    pub fn add_conflict(&mut self, a: [u8; 32], b: [u8; 32]) {
        self.adjacency.entry(a).or_default().insert(b);
        self.adjacency.entry(b).or_default().insert(a);
    }

    pub fn conflicts_with(&self, tx_id: &[u8; 32], other: &[u8; 32]) -> bool {
        self.adjacency
            .get(tx_id)
            .map_or(false, |s| s.contains(other))
    }
}

/// A group of plans that can be executed in parallel within the group.
/// Groups must be executed in strictly ascending group_index order.
#[derive(Debug)]
pub struct ExecutionGroup {
    pub plans: Vec<ExecutionPlan>,
    pub group_index: usize,
}

pub struct ParallelScheduler;

impl ParallelScheduler {
    /// Takes an ordered list of plans (PoSeq canonical order) and returns
    /// groups that can be executed in parallel. Groups are ordered by
    /// `group_index`; plans within each group have no mutual conflicts.
    pub fn schedule(plans: Vec<ExecutionPlan>) -> Vec<ExecutionGroup> {
        if plans.is_empty() {
            return vec![];
        }

        // Build conflict graph using write-set index.
        // Instead of O(n^2) all-pairs, index which plans touch each object,
        // then only check pairs that share at least one object.
        let mut graph = ConflictGraph::new();

        // object_id → list of (plan_index, access_mode)
        let mut object_plans: BTreeMap<&ObjectId, Vec<(usize, AccessMode)>> = BTreeMap::new();
        for (idx, plan) in plans.iter().enumerate() {
            for acc in &plan.object_access {
                object_plans.entry(&acc.object_id).or_default().push((idx, acc.mode));
            }
        }

        // For each object touched by multiple plans, check conflict rules
        for (_obj_id, plan_refs) in &object_plans {
            if plan_refs.len() < 2 { continue; }
            for i in 0..plan_refs.len() {
                for j in (i + 1)..plan_refs.len() {
                    let (pi, mode_i) = &plan_refs[i];
                    let (pj, mode_j) = &plan_refs[j];
                    // RR = no conflict; any write involvement = conflict
                    if *mode_i == AccessMode::ReadWrite || *mode_j == AccessMode::ReadWrite {
                        graph.add_conflict(plans[*pi].tx_id, plans[*pj].tx_id);
                    }
                }
            }
        }

        // Greedy graph coloring: iterate plans in PoSeq order and assign each
        // plan to the first group it has no conflict with.
        //
        // group_assignments[group_idx] = vec of tx_ids already in that group.
        let mut group_assignments: Vec<Vec<[u8; 32]>> = vec![];

        // For each plan in canonical order, find the lowest-indexed group
        // that has no conflict with this plan.
        let mut plan_to_group: BTreeMap<[u8; 32], usize> = BTreeMap::new();

        for plan in &plans {
            let mut placed = false;
            for (group_idx, members) in group_assignments.iter_mut().enumerate() {
                // Check if this plan conflicts with any existing member of the group
                let conflict = members
                    .iter()
                    .any(|m| graph.conflicts_with(&plan.tx_id, m));
                if !conflict {
                    members.push(plan.tx_id);
                    plan_to_group.insert(plan.tx_id, group_idx);
                    placed = true;
                    break;
                }
            }
            if !placed {
                let new_idx = group_assignments.len();
                group_assignments.push(vec![plan.tx_id]);
                plan_to_group.insert(plan.tx_id, new_idx);
            }
        }

        // Assemble ExecutionGroups preserving original PoSeq order within each group
        let num_groups = group_assignments.len();
        let mut groups: Vec<Vec<ExecutionPlan>> = vec![vec![]; num_groups];

        for plan in plans {
            let group_idx = plan_to_group[&plan.tx_id];
            groups[group_idx].push(plan);
        }

        groups
            .into_iter()
            .enumerate()
            .map(|(group_index, plans)| ExecutionGroup { plans, group_index })
            .collect()
    }

    /// Returns true if two plans have a write-write, write-read, or read-write
    /// conflict on any shared ObjectId.
    ///
    /// Formally exhaustive conflict rules:
    ///   - write-write (ww): both plans write the same object        → conflict
    ///   - write-read  (wr): plan A writes what plan B reads         → conflict
    ///   - read-write  (rw): plan B writes what plan A reads         → conflict
    ///   - read-read   (rr): both plans only read the same object    → NO conflict
    pub fn conflicts(a: &ExecutionPlan, b: &ExecutionPlan) -> bool {
        // Build write and read sets for plan a
        let a_writes: BTreeSet<&ObjectId> = a
            .object_access
            .iter()
            .filter(|acc| acc.mode == AccessMode::ReadWrite)
            .map(|acc| &acc.object_id)
            .collect();
        let b_writes: BTreeSet<&ObjectId> = b
            .object_access
            .iter()
            .filter(|acc| acc.mode == AccessMode::ReadWrite)
            .map(|acc| &acc.object_id)
            .collect();
        let a_reads: BTreeSet<&ObjectId> = a
            .object_access
            .iter()
            .filter(|acc| acc.mode == AccessMode::ReadOnly)
            .map(|acc| &acc.object_id)
            .collect();
        let b_reads: BTreeSet<&ObjectId> = b
            .object_access
            .iter()
            .filter(|acc| acc.mode == AccessMode::ReadOnly)
            .map(|acc| &acc.object_id)
            .collect();

        // write-write conflict: both write the same object
        let ww = a_writes.intersection(&b_writes).next().is_some();
        // write-read conflict: a writes what b reads
        let wr = a_writes.intersection(&b_reads).next().is_some();
        // read-write conflict: b writes what a reads
        let rw = b_writes.intersection(&a_reads).next().is_some();

        ww || wr || rw
    }
}
