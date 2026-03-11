use crate::capabilities::checker::Capability;
use std::fmt;

/// A 32-byte unique identifier for an object.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct ObjectId(pub [u8; 32]);

impl ObjectId {
    pub fn new(bytes: [u8; 32]) -> Self {
        ObjectId(bytes)
    }

    pub fn zero() -> Self {
        ObjectId([0u8; 32])
    }

    pub fn as_bytes(&self) -> &[u8; 32] {
        &self.0
    }
}

impl fmt::Display for ObjectId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", hex::encode(self.0))
    }
}

impl From<[u8; 32]> for ObjectId {
    fn from(bytes: [u8; 32]) -> Self {
        ObjectId(bytes)
    }
}

impl serde::Serialize for ObjectId {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&hex::encode(self.0))
    }
}

impl<'de> serde::Deserialize<'de> for ObjectId {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let s = String::deserialize(deserializer)?;
        let bytes = hex::decode(&s).map_err(serde::de::Error::custom)?;
        if bytes.len() != 32 {
            return Err(serde::de::Error::custom("ObjectId must be 32 bytes"));
        }
        let mut arr = [0u8; 32];
        arr.copy_from_slice(&bytes);
        Ok(ObjectId(arr))
    }
}

/// The type of a blockchain object.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, serde::Serialize, serde::Deserialize)]
pub enum ObjectType {
    Wallet,
    Balance,
    Token,
    LiquidityPool,
    Vault,
    GovernanceProposal,
    Identity,
    ExecutionReceipt,
}

/// Describes how an intent accesses a particular object.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum AccessMode {
    ReadOnly,
    ReadWrite,
}

/// Pairs an ObjectId with the access mode required for it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ObjectAccess {
    pub object_id: ObjectId,
    pub mode: AccessMode,
}

/// Monotonically increasing version counter on every object.
pub type ObjectVersion = u64;

/// Metadata common to all objects.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ObjectMeta {
    pub id: ObjectId,
    pub object_type: ObjectType,
    pub owner: [u8; 32],
    pub version: ObjectVersion,
    pub created_at: u64,
    pub updated_at: u64,
}

impl ObjectMeta {
    pub fn new(id: ObjectId, object_type: ObjectType, owner: [u8; 32], now: u64) -> Self {
        ObjectMeta {
            id,
            object_type,
            owner,
            version: 0,
            created_at: now,
            updated_at: now,
        }
    }
}

/// Core trait that every object type must implement.
pub trait Object: Send + Sync {
    fn meta(&self) -> &ObjectMeta;
    fn meta_mut(&mut self) -> &mut ObjectMeta;
    fn object_type(&self) -> ObjectType;
    fn required_capabilities_for_write(&self) -> Vec<Capability>;
    /// Deterministic serialization for state root computation.
    fn encode(&self) -> Vec<u8>;
}

/// Type-erased boxed object.
pub type BoxedObject = Box<dyn Object>;
