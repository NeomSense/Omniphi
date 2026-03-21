//! `chain_bridge` — PoSeq → Cosmos chain accountability interface.
//!
//! This module packages PoSeq's misbehavior, slashing, governance escalation,
//! and checkpoint outputs into self-contained records that the Cosmos chain
//! can ingest via its `x/poseq` module.
//!
//! # Design principles
//!
//! - PoSeq does **not** know about Cosmos SDK types, bank modules, or staking.
//! - The chain does **not** know about PoSeq committees, slots, or proposals.
//! - This module is the **boundary**: it translates PoSeq domain objects into
//!   chain-facing records that are fully self-contained and verifiable.
//!
//! # Record types
//!
//! | Record | Chain action |
//! |--------|-------------|
//! | `EvidencePacket` | Submit as evidence → triggers on-chain slashing |
//! | `PenaltyRecommendationRecord` | Advisory slash recommendation for governance |
//! | `GovernanceEscalationRecord` | Governance proposal reference (severe/critical only) |
//! | `CheckpointAnchorRecord` | On-chain finality anchor for operator visibility |
//! | `AccountabilityReport` | Epoch-end summary of all incidents for operator feeds |

pub mod evidence;
pub mod escalation;
pub mod anchor;
pub mod exporter;
pub mod snapshot;
pub mod contracts;

pub use evidence::{
    EvidencePacket, EvidenceKind, EvidencePacketSet,
    PenaltyRecommendationRecord, DuplicateEvidenceGuard,
};
pub use escalation::{
    GovernanceEscalationRecord, EscalationSeverity, EscalationAction,
    CommitteeSuspensionRecommendation,
};
pub use anchor::{
    CheckpointAnchorRecord, BatchFinalityReference, EpochStateReference,
};
pub use exporter::{
    ChainBridgeExporter, ExportBatch, ExportResult, StatusRecommendation,
};
pub use snapshot::{
    ChainCommitteeSnapshot, ChainCommitteeMember, SnapshotImporter, SnapshotImportError,
};
pub use contracts::{
    ChainContractSchema, ChainIntentSchema, ContractSchemaCache,
};
