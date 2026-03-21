use crate::errors::RuntimeError;
use crate::objects::base::Object;
use std::collections::BTreeSet;

/// All capabilities that can be held by a principal.
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum Capability {
    ReadObject,
    WriteObject,
    TransferAsset,
    SwapAsset,
    ProvideLiquidity,
    WithdrawLiquidity,
    MintAsset,
    BurnAsset,
    ModifyGovernance,
    UpdateIdentity,
    /// Permission to call intents on a specific contract schema.
    ContractCall([u8; 32]),
    /// Admin permission for a specific contract (migrate, upgrade).
    ContractAdmin([u8; 32]),
    /// Permission to deploy new contract schemas.
    ContractDeploy,
}

/// An ordered set of capabilities (BTreeSet for deterministic ordering).
#[derive(Debug, Clone, Default)]
pub struct CapabilitySet(pub BTreeSet<Capability>);

impl CapabilitySet {
    /// Returns an empty capability set.
    pub fn empty() -> Self {
        CapabilitySet(BTreeSet::new())
    }

    /// Returns the full admin capability set (all capabilities).
    pub fn all() -> Self {
        let mut set = BTreeSet::new();
        set.insert(Capability::ReadObject);
        set.insert(Capability::WriteObject);
        set.insert(Capability::TransferAsset);
        set.insert(Capability::SwapAsset);
        set.insert(Capability::ProvideLiquidity);
        set.insert(Capability::WithdrawLiquidity);
        set.insert(Capability::MintAsset);
        set.insert(Capability::BurnAsset);
        set.insert(Capability::ModifyGovernance);
        set.insert(Capability::UpdateIdentity);
        CapabilitySet(set)
    }

    /// Returns the default capability set for a standard user.
    pub fn user_default() -> Self {
        let mut set = BTreeSet::new();
        set.insert(Capability::ReadObject);
        set.insert(Capability::WriteObject);
        set.insert(Capability::TransferAsset);
        set.insert(Capability::SwapAsset);
        CapabilitySet(set)
    }

    pub fn contains(&self, cap: &Capability) -> bool {
        self.0.contains(cap)
    }

    pub fn add(&mut self, cap: Capability) {
        self.0.insert(cap);
    }

    pub fn remove(&mut self, cap: Capability) {
        self.0.remove(&cap);
    }

    pub fn iter(&self) -> impl Iterator<Item = &Capability> {
        self.0.iter()
    }
}

/// Stateless helper for capability checks.
pub struct CapabilityChecker;

impl CapabilityChecker {
    /// Checks that `held` contains all capabilities in `required`.
    /// Returns the first missing capability as an error.
    pub fn check(held: &CapabilitySet, required: &[Capability]) -> Result<(), RuntimeError> {
        for cap in required {
            if !held.contains(cap) {
                return Err(RuntimeError::UnauthorizedCapability {
                    required: cap.clone(),
                    held: held.clone(),
                });
            }
        }
        Ok(())
    }

    /// Checks that `held` satisfies all write capabilities required by `obj`.
    pub fn check_object_write(held: &CapabilitySet, obj: &dyn Object) -> Result<(), RuntimeError> {
        let required = obj.required_capabilities_for_write();
        // For write operations we also always require WriteObject + the object-specific caps.
        // We check WriteObject separately so objects don't all need to list it.
        if !held.contains(&Capability::WriteObject) {
            return Err(RuntimeError::UnauthorizedCapability {
                required: Capability::WriteObject,
                held: held.clone(),
            });
        }
        Self::check(held, &required)
    }
}
