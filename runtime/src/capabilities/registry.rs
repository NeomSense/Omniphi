//! Capability registry mapping sender addresses to their permitted capabilities.
//!
//! Populated from chain state (governance-controlled). The ingestion and
//! execution layers resolve per-sender capabilities through this registry
//! instead of granting `CapabilitySet::all()`.

use super::checker::CapabilitySet;
use std::collections::BTreeMap;

/// Maps sender public keys to their allowed capabilities.
/// Populated from chain state (governance-controlled).
pub struct CapabilityRegistry {
    /// sender_pubkey -> CapabilitySet
    entries: BTreeMap<[u8; 32], CapabilitySet>,
    /// Default capabilities for unknown senders (basic user permissions)
    default_caps: CapabilitySet,
}

impl CapabilityRegistry {
    pub fn new() -> Self {
        CapabilityRegistry {
            entries: BTreeMap::new(),
            // user_default grants: ReadObject, WriteObject, TransferAsset, SwapAsset
            default_caps: CapabilitySet::user_default(),
        }
    }

    /// Create a registry with a custom default capability set.
    pub fn with_default(default_caps: CapabilitySet) -> Self {
        CapabilityRegistry {
            entries: BTreeMap::new(),
            default_caps,
        }
    }

    /// Register capabilities for a sender.
    pub fn register(&mut self, sender: [u8; 32], caps: CapabilitySet) {
        self.entries.insert(sender, caps);
    }

    /// Remove a sender's registered capabilities (reverts to default).
    pub fn revoke(&mut self, sender: &[u8; 32]) {
        self.entries.remove(sender);
    }

    /// Resolve capabilities for a sender. Returns default if not registered.
    pub fn resolve(&self, sender: &[u8; 32]) -> &CapabilitySet {
        self.entries.get(sender).unwrap_or(&self.default_caps)
    }

    /// Number of explicitly registered senders.
    pub fn registered_count(&self) -> usize {
        self.entries.len()
    }

    /// Check whether a sender has explicit (non-default) capabilities.
    pub fn is_registered(&self, sender: &[u8; 32]) -> bool {
        self.entries.contains_key(sender)
    }
}

impl Default for CapabilityRegistry {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::capabilities::checker::Capability;

    fn sender(b: u8) -> [u8; 32] {
        let mut s = [0u8; 32];
        s[0] = b;
        s
    }

    #[test]
    fn test_default_caps_for_unknown_sender() {
        let reg = CapabilityRegistry::new();
        let caps = reg.resolve(&sender(1));
        // user_default includes TransferAsset but not ModifyGovernance
        assert!(caps.contains(&Capability::TransferAsset));
        assert!(!caps.contains(&Capability::ModifyGovernance));
    }

    #[test]
    fn test_registered_sender_gets_custom_caps() {
        let mut reg = CapabilityRegistry::new();
        let mut custom = CapabilitySet::all();
        custom.remove(Capability::BurnAsset);
        reg.register(sender(2), custom);

        let caps = reg.resolve(&sender(2));
        assert!(caps.contains(&Capability::ModifyGovernance));
        assert!(!caps.contains(&Capability::BurnAsset));
    }

    #[test]
    fn test_revoke_returns_to_default() {
        let mut reg = CapabilityRegistry::new();
        reg.register(sender(3), CapabilitySet::all());
        assert!(reg.is_registered(&sender(3)));

        reg.revoke(&sender(3));
        assert!(!reg.is_registered(&sender(3)));

        let caps = reg.resolve(&sender(3));
        assert!(!caps.contains(&Capability::ModifyGovernance));
    }

    #[test]
    fn test_registered_count() {
        let mut reg = CapabilityRegistry::new();
        assert_eq!(reg.registered_count(), 0);
        reg.register(sender(1), CapabilitySet::all());
        reg.register(sender(2), CapabilitySet::all());
        assert_eq!(reg.registered_count(), 2);
    }
}
