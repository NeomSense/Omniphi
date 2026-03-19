//! Compatibility checking for protocol version transitions.
//!
//! Defines which version pairs are compatible and provides upgrade/downgrade
//! rejection logic for wire messages, snapshots, and genesis state.

use super::{ProtocolVersion, PROTOCOL_VERSION, MIN_COMPATIBLE_VERSION};

/// Result of a compatibility check between two protocol versions.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CompatResult {
    /// Fully compatible — proceed normally.
    Compatible,
    /// Compatible but peer is older — warn operator.
    CompatibleWithWarning(String),
    /// Incompatible — reject connection/message.
    Incompatible(String),
}

/// Check wire protocol compatibility between local and remote versions.
pub fn check_wire_compat(remote: &ProtocolVersion) -> CompatResult {
    if remote.major != PROTOCOL_VERSION.major {
        return CompatResult::Incompatible(format!(
            "major version mismatch: local={}, remote={}",
            PROTOCOL_VERSION, remote
        ));
    }

    if !remote.meets_minimum(&MIN_COMPATIBLE_VERSION) {
        return CompatResult::Incompatible(format!(
            "remote version {} below minimum {}",
            remote, MIN_COMPATIBLE_VERSION
        ));
    }

    if remote.minor < PROTOCOL_VERSION.minor {
        return CompatResult::CompatibleWithWarning(format!(
            "remote version {} is older than local {}",
            remote, PROTOCOL_VERSION
        ));
    }

    CompatResult::Compatible
}

/// Check snapshot compatibility before loading.
pub fn check_snapshot_compat(
    snapshot_version: &ProtocolVersion,
    snapshot_format: u32,
) -> CompatResult {
    if snapshot_version.major != PROTOCOL_VERSION.major {
        return CompatResult::Incompatible(format!(
            "snapshot major version {} != local major {}",
            snapshot_version.major, PROTOCOL_VERSION.major
        ));
    }

    if snapshot_format != 1 {
        return CompatResult::Incompatible(format!(
            "unsupported snapshot format version {}",
            snapshot_format
        ));
    }

    if snapshot_version > &PROTOCOL_VERSION {
        return CompatResult::Incompatible(format!(
            "snapshot version {} is newer than local {}; upgrade required",
            snapshot_version, PROTOCOL_VERSION
        ));
    }

    CompatResult::Compatible
}

/// Check genesis compatibility before loading.
pub fn check_genesis_compat(
    genesis_version: &ProtocolVersion,
    genesis_format: u32,
) -> CompatResult {
    if genesis_version.major != PROTOCOL_VERSION.major {
        return CompatResult::Incompatible(format!(
            "genesis major version {} != local major {}",
            genesis_version.major, PROTOCOL_VERSION.major
        ));
    }

    if genesis_format != 1 {
        return CompatResult::Incompatible(format!(
            "unsupported genesis format version {}",
            genesis_format
        ));
    }

    CompatResult::Compatible
}

/// Version negotiation handshake data sent on peer connect.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct VersionHandshake {
    pub protocol_version: ProtocolVersion,
    pub min_compatible: ProtocolVersion,
    pub node_id: [u8; 32],
    pub chain_id: String,
}

impl VersionHandshake {
    pub fn new(node_id: [u8; 32], chain_id: String) -> Self {
        VersionHandshake {
            protocol_version: PROTOCOL_VERSION,
            min_compatible: MIN_COMPATIBLE_VERSION,
            node_id,
            chain_id,
        }
    }

    /// Validate a remote handshake against our requirements.
    pub fn validate_remote(&self, remote: &VersionHandshake) -> CompatResult {
        // Chain ID must match
        if self.chain_id != remote.chain_id {
            return CompatResult::Incompatible(format!(
                "chain_id mismatch: local={}, remote={}",
                self.chain_id, remote.chain_id
            ));
        }

        // Check bidirectional compatibility
        if !remote.protocol_version.meets_minimum(&self.min_compatible) {
            return CompatResult::Incompatible(format!(
                "remote version {} below our minimum {}",
                remote.protocol_version, self.min_compatible
            ));
        }

        if !self.protocol_version.meets_minimum(&remote.min_compatible) {
            return CompatResult::Incompatible(format!(
                "our version {} below remote's minimum {}",
                self.protocol_version, remote.min_compatible
            ));
        }

        check_wire_compat(&remote.protocol_version)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_wire_compat_same_version() {
        assert_eq!(check_wire_compat(&PROTOCOL_VERSION), CompatResult::Compatible);
    }

    #[test]
    fn test_wire_compat_different_major() {
        let remote = ProtocolVersion::new(99, 0, 0);
        assert!(matches!(check_wire_compat(&remote), CompatResult::Incompatible(_)));
    }

    #[test]
    fn test_wire_compat_older_minor() {
        // Only triggers warning if MIN_COMPATIBLE allows it and remote.minor < local.minor
        let remote = ProtocolVersion::new(PROTOCOL_VERSION.major, 0, 0);
        let result = check_wire_compat(&remote);
        // With current PROTOCOL_VERSION 1.0.0, same minor → Compatible
        assert_eq!(result, CompatResult::Compatible);
    }

    #[test]
    fn test_snapshot_compat_ok() {
        assert_eq!(
            check_snapshot_compat(&PROTOCOL_VERSION, 1),
            CompatResult::Compatible,
        );
    }

    #[test]
    fn test_snapshot_compat_wrong_major() {
        let v = ProtocolVersion::new(99, 0, 0);
        assert!(matches!(check_snapshot_compat(&v, 1), CompatResult::Incompatible(_)));
    }

    #[test]
    fn test_snapshot_compat_future_version() {
        let v = ProtocolVersion::new(PROTOCOL_VERSION.major, 99, 0);
        assert!(matches!(check_snapshot_compat(&v, 1), CompatResult::Incompatible(_)));
    }

    #[test]
    fn test_snapshot_compat_bad_format() {
        assert!(matches!(
            check_snapshot_compat(&PROTOCOL_VERSION, 99),
            CompatResult::Incompatible(_),
        ));
    }

    #[test]
    fn test_genesis_compat_ok() {
        assert_eq!(
            check_genesis_compat(&PROTOCOL_VERSION, 1),
            CompatResult::Compatible,
        );
    }

    #[test]
    fn test_genesis_compat_wrong_major() {
        let v = ProtocolVersion::new(2, 0, 0);
        assert!(matches!(check_genesis_compat(&v, 1), CompatResult::Incompatible(_)));
    }

    #[test]
    fn test_handshake_same_version() {
        let h1 = VersionHandshake::new([1u8; 32], "omniphi-testnet-1".into());
        let h2 = VersionHandshake::new([2u8; 32], "omniphi-testnet-1".into());
        assert_eq!(h1.validate_remote(&h2), CompatResult::Compatible);
    }

    #[test]
    fn test_handshake_chain_id_mismatch() {
        let h1 = VersionHandshake::new([1u8; 32], "omniphi-testnet-1".into());
        let h2 = VersionHandshake::new([2u8; 32], "omniphi-mainnet".into());
        assert!(matches!(h1.validate_remote(&h2), CompatResult::Incompatible(_)));
    }

    #[test]
    fn test_handshake_remote_too_old() {
        let h1 = VersionHandshake::new([1u8; 32], "test".into());
        let mut h2 = VersionHandshake::new([2u8; 32], "test".into());
        h2.protocol_version = ProtocolVersion::new(0, 1, 0);
        assert!(matches!(h1.validate_remote(&h2), CompatResult::Incompatible(_)));
    }

    #[test]
    fn test_handshake_serialization_roundtrip() {
        let h = VersionHandshake::new([1u8; 32], "test".into());
        let json = serde_json::to_string(&h).unwrap();
        let decoded: VersionHandshake = serde_json::from_str(&json).unwrap();
        assert_eq!(decoded.protocol_version, h.protocol_version);
        assert_eq!(decoded.chain_id, h.chain_id);
    }
}
