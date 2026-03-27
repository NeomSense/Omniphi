use omniphi_runtime::objects::base::{AccessMode, ObjectAccess, ObjectId};
use omniphi_runtime::resolution::planner::ExecutionPlan;
use omniphi_runtime::scheduler::parallel::ParallelScheduler;

fn id(n: u8) -> ObjectId {
    let mut b = [0u8; 32];
    b[0] = n;
    ObjectId::new(b)
}

fn txid(n: u8) -> [u8; 32] {
    let mut b = [0u8; 32];
    b[31] = n;
    b
}

fn make_plan(tx: u8, accesses: Vec<ObjectAccess>) -> ExecutionPlan {
    ExecutionPlan {
        tx_id: txid(tx),
        operations: vec![],
        required_capabilities: vec![],
        object_access: accesses,
        gas_estimate: 1_000,
        gas_limit: u64::MAX,
    }
}

// ──────────────────────────────────────────────
// Conflict detection
// ──────────────────────────────────────────────

#[test]
fn test_no_conflict_different_objects() {
    let a = make_plan(1, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite }]);
    let b = make_plan(2, vec![ObjectAccess { object_id: id(2), mode: AccessMode::ReadWrite }]);
    assert!(!ParallelScheduler::conflicts(&a, &b));
}

#[test]
fn test_conflict_same_object_both_readwrite() {
    let a = make_plan(1, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite }]);
    let b = make_plan(2, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite }]);
    assert!(ParallelScheduler::conflicts(&a, &b));
}

#[test]
fn test_conflict_same_object_one_readwrite_one_readonly() {
    let a = make_plan(1, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite }]);
    let b = make_plan(2, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadOnly }]);
    assert!(ParallelScheduler::conflicts(&a, &b));
}

#[test]
fn test_no_conflict_same_object_both_readonly() {
    let a = make_plan(1, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadOnly }]);
    let b = make_plan(2, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadOnly }]);
    assert!(!ParallelScheduler::conflicts(&a, &b));
}

// ──────────────────────────────────────────────
// Scheduling: 2 non-conflicting transfers → 1 group
// ──────────────────────────────────────────────

#[test]
fn test_two_non_conflicting_transfers_schedule_in_one_group() {
    // Plan A touches objects 1 and 2
    let plan_a = make_plan(
        1,
        vec![
            ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: id(2), mode: AccessMode::ReadWrite },
        ],
    );
    // Plan B touches objects 3 and 4 (completely disjoint)
    let plan_b = make_plan(
        2,
        vec![
            ObjectAccess { object_id: id(3), mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: id(4), mode: AccessMode::ReadWrite },
        ],
    );

    let groups = ParallelScheduler::schedule(vec![plan_a, plan_b]);
    assert_eq!(groups.len(), 1, "non-conflicting plans should be in 1 group");
    assert_eq!(groups[0].plans.len(), 2);
}

// ──────────────────────────────────────────────
// 2 conflicting transfers → 2 groups of 1
// ──────────────────────────────────────────────

#[test]
fn test_two_conflicting_transfers_schedule_in_two_groups() {
    // Both plans touch object 1 (write-write conflict)
    let plan_a = make_plan(
        1,
        vec![
            ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: id(2), mode: AccessMode::ReadWrite },
        ],
    );
    let plan_b = make_plan(
        2,
        vec![
            ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: id(5), mode: AccessMode::ReadWrite },
        ],
    );

    let groups = ParallelScheduler::schedule(vec![plan_a, plan_b]);
    assert_eq!(groups.len(), 2, "conflicting plans should be in 2 groups");
    assert_eq!(groups[0].plans.len(), 1);
    assert_eq!(groups[1].plans.len(), 1);
}

// ──────────────────────────────────────────────
// A→B, B→C, C→D: B is shared between A and B's tx, C shared between B and C's tx
// ──────────────────────────────────────────────

#[test]
fn test_chain_abc_serialized_correctly() {
    // Transfer A→B: touches balance(A) and balance(B)
    // Transfer B→C: touches balance(B) and balance(C)  -- conflicts with above
    // Transfer C→D: touches balance(C) and balance(D)  -- conflicts with B→C

    let balance_a = id(1);
    let balance_b = id(2);
    let balance_c = id(3);
    let balance_d = id(4);

    let plan_ab = make_plan(
        1,
        vec![
            ObjectAccess { object_id: balance_a, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: balance_b, mode: AccessMode::ReadWrite },
        ],
    );
    let plan_bc = make_plan(
        2,
        vec![
            ObjectAccess { object_id: balance_b, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: balance_c, mode: AccessMode::ReadWrite },
        ],
    );
    let plan_cd = make_plan(
        3,
        vec![
            ObjectAccess { object_id: balance_c, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: balance_d, mode: AccessMode::ReadWrite },
        ],
    );

    let groups = ParallelScheduler::schedule(vec![plan_ab, plan_bc, plan_cd]);

    // A→B and C→D don't share any objects, so they can be in the same group.
    // B→C conflicts with both, so it must be in a separate group.
    // Expected: group 0 = [A→B, C→D], group 1 = [B→C]
    // OR:       group 0 = [A→B], group 1 = [B→C, ...], etc.
    // What matters: B→C must not be in the same group as A→B or C→D.

    // Verify total plans = 3
    let total: usize = groups.iter().map(|g| g.plans.len()).sum();
    assert_eq!(total, 3, "all 3 plans must appear in some group");

    // B→C (tx_id = txid(2)) must not share a group with A→B (txid(1)) or C→D (txid(3))
    let find_group = |target: [u8; 32]| -> usize {
        for g in &groups {
            if g.plans.iter().any(|p| p.tx_id == target) {
                return g.group_index;
            }
        }
        panic!("tx not found in any group");
    };

    let g_ab = find_group(txid(1));
    let g_bc = find_group(txid(2));
    let g_cd = find_group(txid(3));

    assert_ne!(g_ab, g_bc, "A→B and B→C must be in different groups (share balance_b)");
    assert_ne!(g_bc, g_cd, "B→C and C→D must be in different groups (share balance_c)");
}

// ──────────────────────────────────────────────
// Group index ordering
// ──────────────────────────────────────────────

#[test]
fn test_group_indices_are_zero_based_and_sequential() {
    let plan_a = make_plan(1, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite }]);
    let plan_b = make_plan(2, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite }]);

    let groups = ParallelScheduler::schedule(vec![plan_a, plan_b]);
    let indices: Vec<usize> = groups.iter().map(|g| g.group_index).collect();
    assert_eq!(indices, vec![0, 1]);
}

// ──────────────────────────────────────────────
// Empty input
// ──────────────────────────────────────────────

#[test]
fn test_empty_plans_returns_empty_groups() {
    let groups = ParallelScheduler::schedule(vec![]);
    assert!(groups.is_empty());
}

// ──────────────────────────────────────────────
// Fix D: Exhaustive conflict detection verification
// ──────────────────────────────────────────────

/// Exhaustively verifies all conflict combinations:
///   WW (write-write)   → conflict = true
///   WR (write-read)    → conflict = true   (a writes what b reads)
///   RW (read-write)    → conflict = true   (b writes what a reads)
///   RR (read-read)     → conflict = false  (safe to parallelize)
///   disjoint objects   → conflict = false  (no shared state)
#[test]
fn test_conflict_detection_exhaustive() {
    let obj1 = id(1);
    let obj2 = id(2);
    let obj3 = id(3);

    // ── WW: both write obj1 → conflict ──────────────────────────────────────
    {
        let a = make_plan(1, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadWrite }]);
        let b = make_plan(2, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadWrite }]);
        assert!(
            ParallelScheduler::conflicts(&a, &b),
            "WW conflict on same object must be detected"
        );
    }

    // ── WR: a writes obj1, b reads obj1 → conflict ──────────────────────────
    {
        let a = make_plan(1, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadWrite }]);
        let b = make_plan(2, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadOnly }]);
        assert!(
            ParallelScheduler::conflicts(&a, &b),
            "WR conflict (a writes, b reads) must be detected"
        );
    }

    // ── RW: a reads obj1, b writes obj1 → conflict ──────────────────────────
    {
        let a = make_plan(1, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadOnly }]);
        let b = make_plan(2, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadWrite }]);
        assert!(
            ParallelScheduler::conflicts(&a, &b),
            "RW conflict (a reads, b writes) must be detected"
        );
    }

    // ── RR: both read obj1 → NO conflict ────────────────────────────────────
    {
        let a = make_plan(1, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadOnly }]);
        let b = make_plan(2, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadOnly }]);
        assert!(
            !ParallelScheduler::conflicts(&a, &b),
            "RR (both read same object) must NOT be a conflict"
        );
    }

    // ── Disjoint objects: a writes obj1, b writes obj2 → NO conflict ────────
    {
        let a = make_plan(1, vec![ObjectAccess { object_id: obj1, mode: AccessMode::ReadWrite }]);
        let b = make_plan(2, vec![ObjectAccess { object_id: obj2, mode: AccessMode::ReadWrite }]);
        assert!(
            !ParallelScheduler::conflicts(&a, &b),
            "writes to different objects must NOT be a conflict"
        );
    }

    // ── Multi-object: a writes obj1 & obj2, b reads obj2 & writes obj3 ──────
    // Conflict because a writes obj2 and b reads obj2 (WR)
    {
        let a = make_plan(1, vec![
            ObjectAccess { object_id: obj1, mode: AccessMode::ReadWrite },
            ObjectAccess { object_id: obj2, mode: AccessMode::ReadWrite },
        ]);
        let b = make_plan(2, vec![
            ObjectAccess { object_id: obj2, mode: AccessMode::ReadOnly },
            ObjectAccess { object_id: obj3, mode: AccessMode::ReadWrite },
        ]);
        assert!(
            ParallelScheduler::conflicts(&a, &b),
            "WR conflict across multi-object plans must be detected"
        );
    }

    // ── RR on multiple shared objects → NO conflict ──────────────────────────
    {
        let a = make_plan(1, vec![
            ObjectAccess { object_id: obj1, mode: AccessMode::ReadOnly },
            ObjectAccess { object_id: obj2, mode: AccessMode::ReadOnly },
        ]);
        let b = make_plan(2, vec![
            ObjectAccess { object_id: obj1, mode: AccessMode::ReadOnly },
            ObjectAccess { object_id: obj2, mode: AccessMode::ReadOnly },
        ]);
        assert!(
            !ParallelScheduler::conflicts(&a, &b),
            "multiple shared reads must NOT be a conflict"
        );
    }
}

// ──────────────────────────────────────────────
// Deterministic schedule reproducibility
// ──────────────────────────────────────────────

#[test]
fn test_deterministic_schedule_reproducibility() {
    // Same plans in same order must always produce identical grouping.
    let make_plans = || {
        vec![
            make_plan(1, vec![
                ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite },
                ObjectAccess { object_id: id(2), mode: AccessMode::ReadWrite },
            ]),
            make_plan(2, vec![
                ObjectAccess { object_id: id(2), mode: AccessMode::ReadOnly },
                ObjectAccess { object_id: id(3), mode: AccessMode::ReadWrite },
            ]),
            make_plan(3, vec![
                ObjectAccess { object_id: id(4), mode: AccessMode::ReadWrite },
                ObjectAccess { object_id: id(5), mode: AccessMode::ReadWrite },
            ]),
            make_plan(4, vec![
                ObjectAccess { object_id: id(3), mode: AccessMode::ReadWrite },
                ObjectAccess { object_id: id(6), mode: AccessMode::ReadWrite },
            ]),
        ]
    };

    let groups_a = ParallelScheduler::schedule(make_plans());
    let groups_b = ParallelScheduler::schedule(make_plans());

    assert_eq!(groups_a.len(), groups_b.len(), "same number of groups");
    for (ga, gb) in groups_a.iter().zip(groups_b.iter()) {
        assert_eq!(ga.group_index, gb.group_index);
        assert_eq!(ga.plans.len(), gb.plans.len());
        for (pa, pb) in ga.plans.iter().zip(gb.plans.iter()) {
            assert_eq!(pa.tx_id, pb.tx_id, "plans within each group must be in same order");
        }
    }
}

// ──────────────────────────────────────────────
// Large-scale: 10 plans with mixed conflicts
// ──────────────────────────────────────────────

#[test]
fn test_large_scale_mixed_conflicts() {
    // 10 plans:
    // Plans 1-5 each write a unique object (no mutual conflicts) → 1 group
    // Plans 6-10 all write object 100 (pairwise WW conflicts) → 5 groups
    let mut plans = vec![];
    for i in 1..=5u8 {
        plans.push(make_plan(i, vec![
            ObjectAccess { object_id: id(i), mode: AccessMode::ReadWrite },
        ]));
    }
    let shared = id(100);
    for i in 6..=10u8 {
        plans.push(make_plan(i, vec![
            ObjectAccess { object_id: shared, mode: AccessMode::ReadWrite },
        ]));
    }

    let groups = ParallelScheduler::schedule(plans);

    // All 10 plans must be present
    let total: usize = groups.iter().map(|g| g.plans.len()).sum();
    assert_eq!(total, 10);

    // Plans 6-10 must each be in a different group (they all conflict via object 100)
    let find_group = |target: [u8; 32]| -> usize {
        for g in &groups {
            if g.plans.iter().any(|p| p.tx_id == target) {
                return g.group_index;
            }
        }
        panic!("tx not found");
    };

    let g6 = find_group(txid(6));
    let g7 = find_group(txid(7));
    let g8 = find_group(txid(8));
    let g9 = find_group(txid(9));
    let g10 = find_group(txid(10));

    // All must be in different groups
    let mut group_set = std::collections::BTreeSet::new();
    group_set.insert(g6);
    group_set.insert(g7);
    group_set.insert(g8);
    group_set.insert(g9);
    group_set.insert(g10);
    assert_eq!(group_set.len(), 5, "5 mutually conflicting plans must be in 5 different groups");

    // Plans 1-5 can share groups with non-conflicting plans
    // (they don't touch object 100, so they can go in any group)
}

// ──────────────────────────────────────────────
// Read-write asymmetry: a reads, b writes same object
// ──────────────────────────────────────────────

#[test]
fn test_rw_asymmetry_serialized() {
    // Plan A reads object 1, Plan B writes object 1
    // This is a RW conflict — B must come after A (or in separate group)
    let a = make_plan(1, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadOnly }]);
    let b = make_plan(2, vec![ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite }]);

    let groups = ParallelScheduler::schedule(vec![a, b]);
    assert_eq!(groups.len(), 2, "read-then-write must be serialized into 2 groups");
}

// ──────────────────────────────────────────────
// Single plan always produces exactly 1 group
// ──────────────────────────────────────────────

#[test]
fn test_single_plan_one_group() {
    let plan = make_plan(1, vec![
        ObjectAccess { object_id: id(1), mode: AccessMode::ReadWrite },
    ]);
    let groups = ParallelScheduler::schedule(vec![plan]);
    assert_eq!(groups.len(), 1);
    assert_eq!(groups[0].plans.len(), 1);
    assert_eq!(groups[0].group_index, 0);
}
