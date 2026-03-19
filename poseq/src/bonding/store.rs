use std::collections::BTreeMap;

use super::record::{BondError, OperatorBond};

/// In-memory registry of operator bond declarations.
///
/// The authoritative bond state lives on-chain (x/poseq). This store
/// is the local fast-lane view — populated from chain snapshots and
/// used during committee formation and reward score computation.
pub struct BondingStore {
    /// Primary index: (operator_address, node_id) → bond.
    bonds: BTreeMap<(String, [u8; 32]), OperatorBond>,
    /// Secondary index: node_id → operator_address (for reverse lookup).
    node_index: BTreeMap<[u8; 32], String>,
}

impl BondingStore {
    pub fn new() -> Self {
        BondingStore {
            bonds: BTreeMap::new(),
            node_index: BTreeMap::new(),
        }
    }

    /// Declare a new bond. Returns `BondError::BondAlreadyActive` if an active
    /// bond already exists for this (operator, node_id) pair.
    pub fn declare(
        &mut self,
        operator_address: String,
        node_id: [u8; 32],
        bond_amount: u64,
        bond_denom: String,
        epoch: u64,
    ) -> Result<(), BondError> {
        if bond_amount == 0 {
            return Err(BondError::InvalidBondAmount);
        }
        if bond_denom.is_empty() {
            return Err(BondError::InvalidDenom);
        }
        let key = (operator_address.clone(), node_id);
        if let Some(existing) = self.bonds.get(&key) {
            if existing.is_active {
                return Err(BondError::BondAlreadyActive);
            }
        }
        let bond = OperatorBond::new(operator_address.clone(), node_id, bond_amount, bond_denom, epoch);
        self.bonds.insert(key, bond);
        self.node_index.insert(node_id, operator_address);
        Ok(())
    }

    /// Withdraw an existing active bond.
    pub fn withdraw(
        &mut self,
        operator_address: &str,
        node_id: &[u8; 32],
        epoch: u64,
    ) -> Result<(), BondError> {
        let key = (operator_address.to_string(), *node_id);
        let bond = self.bonds.get_mut(&key).ok_or(BondError::BondNotFound)?;
        if !bond.is_active {
            return Err(BondError::BondAlreadyWithdrawn);
        }
        bond.withdraw(epoch);
        self.node_index.remove(node_id);
        Ok(())
    }

    /// Get the bond for a specific (operator, node_id) pair.
    pub fn get(&self, operator_address: &str, node_id: &[u8; 32]) -> Option<&OperatorBond> {
        self.bonds.get(&(operator_address.to_string(), *node_id))
    }

    /// Look up the active bond for a node_id (reverse index lookup).
    pub fn get_by_node(&self, node_id: &[u8; 32]) -> Option<&OperatorBond> {
        let operator = self.node_index.get(node_id)?;
        self.bonds.get(&(operator.clone(), *node_id))
    }

    /// Returns true if node_id has an active bond.
    pub fn has_active_bond(&self, node_id: &[u8; 32]) -> bool {
        self.node_index
            .get(node_id)
            .and_then(|op| self.bonds.get(&(op.clone(), *node_id)))
            .map(|b| b.is_active)
            .unwrap_or(false)
    }

    /// Returns all active bonds.
    pub fn all_active(&self) -> Vec<&OperatorBond> {
        self.bonds.values().filter(|b| b.is_active).collect()
    }

    /// Returns the operator address for a node, if bonded.
    pub fn operator_for_node(&self, node_id: &[u8; 32]) -> Option<&str> {
        self.node_index.get(node_id).map(|s| s.as_str())
    }

    /// Load (bulk-import) bonds from chain state. Replaces all existing records.
    pub fn load_from_chain(&mut self, bonds: Vec<OperatorBond>) {
        self.bonds.clear();
        self.node_index.clear();
        for bond in bonds {
            if bond.is_active {
                self.node_index.insert(bond.node_id, bond.operator_address.clone());
            }
            self.bonds.insert((bond.operator_address.clone(), bond.node_id), bond);
        }
    }
}

impl Default for BondingStore {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn nid(b: u8) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[0] = b;
        id
    }

    #[test]
    fn test_declare_and_lookup() {
        let mut store = BondingStore::new();
        store.declare("omni1op1".into(), nid(1), 1_000_000, "uomni".into(), 1).unwrap();
        assert!(store.has_active_bond(&nid(1)));
        let bond = store.get("omni1op1", &nid(1)).unwrap();
        assert_eq!(bond.bond_amount, 1_000_000);
        assert_eq!(bond.operator_address, "omni1op1");
    }

    #[test]
    fn test_declare_zero_amount_rejected() {
        let mut store = BondingStore::new();
        let err = store.declare("omni1op1".into(), nid(1), 0, "uomni".into(), 1).unwrap_err();
        assert_eq!(err, BondError::InvalidBondAmount);
    }

    #[test]
    fn test_declare_empty_denom_rejected() {
        let mut store = BondingStore::new();
        let err = store.declare("omni1op1".into(), nid(1), 100, "".into(), 1).unwrap_err();
        assert_eq!(err, BondError::InvalidDenom);
    }

    #[test]
    fn test_declare_duplicate_active_rejected() {
        let mut store = BondingStore::new();
        store.declare("omni1op1".into(), nid(1), 100, "uomni".into(), 1).unwrap();
        let err = store.declare("omni1op1".into(), nid(1), 200, "uomni".into(), 2).unwrap_err();
        assert_eq!(err, BondError::BondAlreadyActive);
    }

    #[test]
    fn test_withdraw() {
        let mut store = BondingStore::new();
        store.declare("omni1op1".into(), nid(1), 100, "uomni".into(), 1).unwrap();
        store.withdraw("omni1op1", &nid(1), 5).unwrap();
        assert!(!store.has_active_bond(&nid(1)));
        let bond = store.get("omni1op1", &nid(1)).unwrap();
        assert!(!bond.is_active);
        assert_eq!(bond.withdrawn_at_epoch, 5);
    }

    #[test]
    fn test_withdraw_double_rejected() {
        let mut store = BondingStore::new();
        store.declare("omni1op1".into(), nid(2), 100, "uomni".into(), 1).unwrap();
        store.withdraw("omni1op1", &nid(2), 3).unwrap();
        let err = store.withdraw("omni1op1", &nid(2), 4).unwrap_err();
        assert_eq!(err, BondError::BondAlreadyWithdrawn);
    }

    #[test]
    fn test_get_by_node() {
        let mut store = BondingStore::new();
        store.declare("omni1op2".into(), nid(5), 500, "uomni".into(), 2).unwrap();
        let bond = store.get_by_node(&nid(5)).unwrap();
        assert_eq!(bond.operator_address, "omni1op2");
    }

    #[test]
    fn test_redeclare_after_withdraw() {
        let mut store = BondingStore::new();
        store.declare("omni1op1".into(), nid(1), 100, "uomni".into(), 1).unwrap();
        store.withdraw("omni1op1", &nid(1), 2).unwrap();
        // Re-declaring after withdrawal should succeed
        store.declare("omni1op1".into(), nid(1), 200, "uomni".into(), 3).unwrap();
        assert!(store.has_active_bond(&nid(1)));
    }

    #[test]
    fn test_load_from_chain() {
        let mut store = BondingStore::new();
        let bonds = vec![
            OperatorBond::new("omni1a".into(), nid(1), 100, "uomni".into(), 1),
            OperatorBond::new("omni1b".into(), nid(2), 200, "uomni".into(), 1),
        ];
        store.load_from_chain(bonds);
        assert!(store.has_active_bond(&nid(1)));
        assert!(store.has_active_bond(&nid(2)));
        assert_eq!(store.all_active().len(), 2);
    }

    #[test]
    fn test_operator_for_node() {
        let mut store = BondingStore::new();
        store.declare("omni1op9".into(), nid(9), 999, "uomni".into(), 1).unwrap();
        assert_eq!(store.operator_for_node(&nid(9)), Some("omni1op9"));
        assert_eq!(store.operator_for_node(&nid(0)), None);
    }
}
