use serde::{Deserialize, Serialize};

/// BondState tracks the lifecycle of an operator bond.
///
/// Mirrors the Go `BondState` enum in `chain/x/poseq/types/bond.go`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum BondState {
    #[default]
    Active,
    PartiallySlashed,
    Jailed,
    Exhausted,
    Retired,
}

impl BondState {
    /// Returns true if the bond can still be slashed.
    pub fn is_slashable(&self) -> bool {
        matches!(self, BondState::Active | BondState::PartiallySlashed | BondState::Jailed)
    }
}

impl std::fmt::Display for BondState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            BondState::Active => write!(f, "Active"),
            BondState::PartiallySlashed => write!(f, "PartiallySlashed"),
            BondState::Jailed => write!(f, "Jailed"),
            BondState::Exhausted => write!(f, "Exhausted"),
            BondState::Retired => write!(f, "Retired"),
        }
    }
}

/// A PoSeq operator's bond declaration on the slow lane.
///
/// Phase 5: this is a declaration record only — no token movement occurs here.
/// The associated tokens are expected to be locked on-chain (Phase 6 integration).
///
/// Keyed by `(operator_address, node_id)`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OperatorBond {
    /// Cosmos bech32 operator address.
    pub operator_address: String,
    /// 32-byte node identity (hex or raw bytes).
    pub node_id: [u8; 32],
    /// Declared bond amount in the chain's base denom (e.g. uomni).
    pub bond_amount: u64,
    /// Token denomination.
    pub bond_denom: String,
    /// Epoch when the bond was first declared.
    pub bonded_since_epoch: u64,
    /// Whether the bond is currently active (not withdrawn).
    pub is_active: bool,
    /// Epoch when the bond was withdrawn. 0 = not withdrawn.
    pub withdrawn_at_epoch: u64,

    /// Bond lifecycle state. Defaults to Active.
    #[serde(default)]
    pub state: BondState,
    /// Cumulative amount slashed from this bond.
    #[serde(default)]
    pub slashed_amount: u64,
    /// Available bond = bond_amount - slashed_amount.
    #[serde(default)]
    pub available_bond: u64,
    /// Epoch of the most recent slash. 0 = never slashed.
    #[serde(default)]
    pub last_slash_epoch: u64,
    /// Number of slash executions against this bond.
    #[serde(default)]
    pub slash_count: u32,
}

impl OperatorBond {
    pub fn new(
        operator_address: String,
        node_id: [u8; 32],
        bond_amount: u64,
        bond_denom: String,
        epoch: u64,
    ) -> Self {
        OperatorBond {
            operator_address,
            node_id,
            bond_amount,
            bond_denom,
            bonded_since_epoch: epoch,
            is_active: true,
            withdrawn_at_epoch: 0,
            state: BondState::Active,
            slashed_amount: 0,
            available_bond: bond_amount,
            last_slash_epoch: 0,
            slash_count: 0,
        }
    }

    /// Apply a slash of `slash_bps` basis points against the original `bond_amount`.
    ///
    /// Returns the amount slashed. Returns 0 if the bond is already exhausted.
    ///
    /// Formula: `slash_amount = min(bond_amount * slash_bps / 10000, available_bond)`.
    /// Uses bond_amount (not available_bond) as base so fractions are consistent across
    /// multiple slashes.
    pub fn apply_slash(&mut self, slash_bps: u32, epoch: u64) -> u64 {
        if self.available_bond == 0 {
            return 0;
        }
        // Initialize available_bond if it was never set (backwards compat)
        if self.available_bond == 0 && self.slashed_amount == 0 {
            self.available_bond = self.bond_amount;
        }
        let mut slash_amount = (self.bond_amount as u128 * slash_bps as u128 / 10_000) as u64;
        if slash_amount == 0 {
            slash_amount = 1;
        }
        if slash_amount > self.available_bond {
            slash_amount = self.available_bond;
        }
        self.available_bond -= slash_amount;
        self.slashed_amount += slash_amount;
        self.last_slash_epoch = epoch;
        self.slash_count += 1;

        if self.available_bond == 0 {
            self.state = BondState::Exhausted;
        } else if self.state == BondState::Active {
            self.state = BondState::PartiallySlashed;
        }
        slash_amount
    }

    /// Withdraw the bond. Sets is_active = false, records epoch.
    pub fn withdraw(&mut self, epoch: u64) {
        self.is_active = false;
        self.withdrawn_at_epoch = epoch;
    }
}

/// Validation errors for bond operations.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BondError {
    BondAlreadyActive,
    BondNotFound,
    BondAlreadyWithdrawn,
    InvalidBondAmount,
    InvalidDenom,
    BondExhausted,
}

impl std::fmt::Display for BondError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::BondAlreadyActive => write!(f, "bond already active for this operator/node"),
            Self::BondNotFound => write!(f, "no bond found for this operator/node"),
            Self::BondAlreadyWithdrawn => write!(f, "bond already withdrawn"),
            Self::InvalidBondAmount => write!(f, "bond amount must be > 0"),
            Self::InvalidDenom => write!(f, "bond denom must not be empty"),
            Self::BondExhausted => write!(f, "bond is exhausted — no remaining amount"),
        }
    }
}

impl std::error::Error for BondError {}
