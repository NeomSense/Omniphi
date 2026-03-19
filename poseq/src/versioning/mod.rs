//! Protocol versioning and compatibility for Omniphi PoSeq.
//!
//! Every compatibility boundary (wire messages, genesis, snapshots, batch
//! handoff, gRPC API) carries an explicit version. Nodes reject messages
//! from incompatible protocol versions at the wire level.

pub mod compat;

/// Current protocol version. Bump on breaking changes.
pub const PROTOCOL_VERSION: ProtocolVersion = ProtocolVersion {
    major: 1,
    minor: 0,
    patch: 0,
};

/// Minimum compatible protocol version for peer connections.
pub const MIN_COMPATIBLE_VERSION: ProtocolVersion = ProtocolVersion {
    major: 1,
    minor: 0,
    patch: 0,
};

/// Structured protocol version with semantic versioning.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, serde::Serialize, serde::Deserialize)]
pub struct ProtocolVersion {
    pub major: u32,
    pub minor: u32,
    pub patch: u32,
}

impl ProtocolVersion {
    pub const fn new(major: u32, minor: u32, patch: u32) -> Self {
        ProtocolVersion { major, minor, patch }
    }

    /// Encode as a single u64 for compact storage: major(16) | minor(16) | patch(16).
    pub fn to_u64(&self) -> u64 {
        ((self.major as u64) << 32) | ((self.minor as u64) << 16) | (self.patch as u64)
    }

    /// Decode from compact u64 representation.
    pub fn from_u64(v: u64) -> Self {
        ProtocolVersion {
            major: ((v >> 32) & 0xFFFF) as u32,
            minor: ((v >> 16) & 0xFFFF) as u32,
            patch: (v & 0xFFFF) as u32,
        }
    }

    /// Check if this version is compatible with another.
    /// Compatible = same major version AND our minor >= their minor.
    pub fn is_compatible_with(&self, other: &ProtocolVersion) -> bool {
        self.major == other.major && self.minor >= other.minor
    }

    /// Check if this version meets the minimum requirement.
    pub fn meets_minimum(&self, min: &ProtocolVersion) -> bool {
        self >= min
    }
}

impl std::fmt::Display for ProtocolVersion {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}.{}.{}", self.major, self.minor, self.patch)
    }
}

impl std::str::FromStr for ProtocolVersion {
    type Err = ();

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let parts: Vec<&str> = s.split('.').collect();
        if parts.len() != 3 {
            return Err(());
        }
        let major = parts[0].parse::<u32>().map_err(|_| ())?;
        let minor = parts[1].parse::<u32>().map_err(|_| ())?;
        let patch = parts[2].parse::<u32>().map_err(|_| ())?;
        Ok(ProtocolVersion { major, minor, patch })
    }
}

/// Version metadata attached to wire messages.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct VersionedEnvelope<T> {
    pub protocol_version: ProtocolVersion,
    pub payload: T,
}

impl<T> VersionedEnvelope<T> {
    pub fn wrap(payload: T) -> Self {
        VersionedEnvelope {
            protocol_version: PROTOCOL_VERSION,
            payload,
        }
    }

    pub fn is_compatible(&self) -> bool {
        self.protocol_version.is_compatible_with(&MIN_COMPATIBLE_VERSION)
    }
}

/// Genesis version metadata.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct GenesisVersion {
    pub protocol_version: ProtocolVersion,
    pub genesis_format: u32,
    pub chain_id: String,
}

impl GenesisVersion {
    pub fn current(chain_id: String) -> Self {
        GenesisVersion {
            protocol_version: PROTOCOL_VERSION,
            genesis_format: 1,
            chain_id,
        }
    }
}

/// Snapshot version metadata.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SnapshotVersion {
    pub protocol_version: ProtocolVersion,
    pub snapshot_format: u32,
}

impl SnapshotVersion {
    pub fn current() -> Self {
        SnapshotVersion {
            protocol_version: PROTOCOL_VERSION,
            snapshot_format: 1,
        }
    }

    pub fn is_loadable(&self) -> bool {
        self.protocol_version.major == PROTOCOL_VERSION.major
            && self.snapshot_format == 1
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_version_display() {
        let v = ProtocolVersion::new(1, 2, 3);
        assert_eq!(v.to_string(), "1.2.3");
    }

    #[test]
    fn test_version_u64_roundtrip() {
        let v = ProtocolVersion::new(1, 5, 12);
        let encoded = v.to_u64();
        let decoded = ProtocolVersion::from_u64(encoded);
        assert_eq!(v, decoded);
    }

    #[test]
    fn test_compatible_same_major() {
        let v1 = ProtocolVersion::new(1, 2, 0);
        let v2 = ProtocolVersion::new(1, 1, 0);
        assert!(v1.is_compatible_with(&v2));
    }

    #[test]
    fn test_incompatible_different_major() {
        let v1 = ProtocolVersion::new(2, 0, 0);
        let v2 = ProtocolVersion::new(1, 0, 0);
        assert!(!v1.is_compatible_with(&v2));
    }

    #[test]
    fn test_incompatible_lower_minor() {
        let v1 = ProtocolVersion::new(1, 0, 0);
        let v2 = ProtocolVersion::new(1, 1, 0);
        assert!(!v1.is_compatible_with(&v2));
    }

    #[test]
    fn test_meets_minimum() {
        let v = ProtocolVersion::new(1, 1, 0);
        let min = ProtocolVersion::new(1, 0, 0);
        assert!(v.meets_minimum(&min));
    }

    #[test]
    fn test_does_not_meet_minimum() {
        let v = ProtocolVersion::new(0, 9, 0);
        let min = ProtocolVersion::new(1, 0, 0);
        assert!(!v.meets_minimum(&min));
    }

    #[test]
    fn test_versioned_envelope_compatible() {
        let env = VersionedEnvelope::wrap("hello");
        assert!(env.is_compatible());
    }

    #[test]
    fn test_versioned_envelope_incompatible() {
        let env = VersionedEnvelope {
            protocol_version: ProtocolVersion::new(99, 0, 0),
            payload: "hello",
        };
        assert!(!env.is_compatible());
    }

    #[test]
    fn test_genesis_version() {
        let gv = GenesisVersion::current("omniphi-testnet-1".into());
        assert_eq!(gv.protocol_version, PROTOCOL_VERSION);
        assert_eq!(gv.genesis_format, 1);
    }

    #[test]
    fn test_snapshot_version_loadable() {
        let sv = SnapshotVersion::current();
        assert!(sv.is_loadable());
    }

    #[test]
    fn test_snapshot_version_not_loadable_wrong_major() {
        let sv = SnapshotVersion {
            protocol_version: ProtocolVersion::new(99, 0, 0),
            snapshot_format: 1,
        };
        assert!(!sv.is_loadable());
    }

    #[test]
    fn test_serialization_roundtrip() {
        let v = PROTOCOL_VERSION;
        let json = serde_json::to_string(&v).unwrap();
        let decoded: ProtocolVersion = serde_json::from_str(&json).unwrap();
        assert_eq!(v, decoded);
    }

    #[test]
    fn test_version_ordering() {
        let v1 = ProtocolVersion::new(1, 0, 0);
        let v2 = ProtocolVersion::new(1, 1, 0);
        let v3 = ProtocolVersion::new(2, 0, 0);
        assert!(v1 < v2);
        assert!(v2 < v3);
    }
}
