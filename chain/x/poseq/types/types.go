// Package types defines the Go-side representations of PoSeq chain bridge records.
//
// These types mirror the Rust chain_bridge structs serialized as JSON by the
// ChainBridgeExporter. The chain's x/poseq keeper deserializes them and stores
// them for governance, slashing, and operator visibility queries.
package types

import "encoding/json"

// ─── EvidenceKind ─────────────────────────────────────────────────────────────

type EvidenceKind string

const (
	EvidenceKindEquivocation         EvidenceKind = "Equivocation"
	EvidenceKindUnauthorizedProposal EvidenceKind = "UnauthorizedProposal"
	EvidenceKindUnfairSequencing     EvidenceKind = "UnfairSequencing"
	EvidenceKindReplayAbuse          EvidenceKind = "ReplayAbuse"
	EvidenceKindBridgeAbuse          EvidenceKind = "BridgeAbuse"
	EvidenceKindPersistentAbsence    EvidenceKind = "PersistentAbsence"
	EvidenceKindStaleAuthority       EvidenceKind = "StaleAuthority"
	EvidenceKindInvalidProposalSpam  EvidenceKind = "InvalidProposalSpam"
)

// ─── MisbehaviorSeverity ──────────────────────────────────────────────────────

type MisbehaviorSeverity string

const (
	SeverityMinor    MisbehaviorSeverity = "Minor"
	SeverityModerate MisbehaviorSeverity = "Moderate"
	SeveritySevere   MisbehaviorSeverity = "Severe"
	SeverityCritical MisbehaviorSeverity = "Critical"
)

// ─── EvidencePacket ───────────────────────────────────────────────────────────

// EvidencePacket is the Go representation of the Rust EvidencePacket from
// poseq/src/chain_bridge/evidence.rs. The chain stores these by packet_hash
// and exposes them for slashing and governance queries.
type EvidencePacket struct {
	// PacketHash is SHA256("kind_tag" | node_id | epoch_be | sorted_evidence_hashes).
	PacketHash []byte `json:"packet_hash"`

	Kind             EvidenceKind        `json:"kind"`
	OffenderNodeID   []byte              `json:"offender_node_id"`
	Epoch            uint64              `json:"epoch"`
	Slot             uint64              `json:"slot"`
	Severity         MisbehaviorSeverity `json:"severity"`
	ProposedSlashBps uint32              `json:"proposed_slash_bps"`
	EvidenceHashes   [][]byte            `json:"evidence_hashes"`
	RequiresGovernance    bool           `json:"requires_governance"`
	RecommendSuspension   bool           `json:"recommend_suspension"`
	LinkedBatchID    []byte              `json:"linked_batch_id,omitempty"`
}

func (p *EvidencePacket) MarshalJSON() ([]byte, error) {
	type Alias EvidencePacket
	return json.Marshal((*Alias)(p))
}

// ─── PenaltyRecommendationRecord ─────────────────────────────────────────────

// PenaltyRecommendationRecord carries a non-binding slash recommendation from
// PoSeq. Governance must ratify before any on-chain tokens are slashed.
type PenaltyRecommendationRecord struct {
	NodeID       []byte `json:"node_id"`
	Epoch        uint64 `json:"epoch"`
	SlashBps     uint32 `json:"slash_bps"`
	Reason       string `json:"reason"`
	PacketHash   []byte `json:"packet_hash"`
}

// ─── EscalationSeverity / EscalationAction ───────────────────────────────────

type EscalationSeverity string

const (
	EscalationSeveritySevere   EscalationSeverity = "Severe"
	EscalationSeverityCritical EscalationSeverity = "Critical"
)

// EscalationAction is the governance action PoSeq recommends.
// The chain does NOT auto-execute — it becomes governance proposal content.
type EscalationAction struct {
	// Tag identifies the variant: "SuspendFromCommittee", "PermanentBan",
	// "CommitteeFreeze", or "GovernanceReview".
	Tag string `json:"tag"`

	// For SuspendFromCommittee
	Epochs *uint64 `json:"epochs,omitempty"`

	// For CommitteeFreeze
	CommitteeEpoch *uint64 `json:"committee_epoch,omitempty"`
}

// ─── GovernanceEscalationRecord ───────────────────────────────────────────────

// GovernanceEscalationRecord is the Go representation of the Rust
// GovernanceEscalationRecord from poseq/src/chain_bridge/escalation.rs.
//
// The chain's x/poseq keeper stores these by escalation_id and exposes them
// for governance tooling to draft corresponding proposals.
type GovernanceEscalationRecord struct {
	// EscalationID is SHA256("esc" | offender | epoch_be | evidence_hash).
	EscalationID []byte `json:"escalation_id"`

	OffenderNodeID      []byte             `json:"offender_node_id"`
	EvidencePacketHash  []byte             `json:"evidence_packet_hash"`
	Epoch               uint64             `json:"epoch"`
	Severity            EscalationSeverity `json:"severity"`
	RecommendedAction   EscalationAction   `json:"recommended_action"`
	Rationale           string             `json:"rationale"`
	BlockPendingGovernance bool            `json:"block_pending_governance"`
}

// ─── CommitteeSuspensionRecommendation ───────────────────────────────────────

// CommitteeSuspensionRecommendation recommends blocking a node from committee
// participation. The x/poseq keeper can auto-apply this if the evidence is
// sufficient (configured by governance params).
type CommitteeSuspensionRecommendation struct {
	NodeID              []byte `json:"node_id"`
	SuspendFromEpoch    uint64 `json:"suspend_from_epoch"`
	SuspendUntilEpoch   uint64 `json:"suspend_until_epoch"`
	EvidencePacketHash  []byte `json:"evidence_packet_hash"`
	Reason              string `json:"reason"`
}

// ─── BatchFinalityReference ───────────────────────────────────────────────────

// BatchFinalityReference is a chain-side reference to a finalized PoSeq batch.
type BatchFinalityReference struct {
	BatchID          []byte `json:"batch_id"`
	Slot             uint64 `json:"slot"`
	Epoch            uint64 `json:"epoch"`
	FinalizationHash []byte `json:"finalization_hash"`
	SubmissionCount  uint64 `json:"submission_count"`
	QuorumApprovals  uint64 `json:"quorum_approvals"`
	CommitteeSize    uint64 `json:"committee_size"`
}

// ─── EpochStateReference ─────────────────────────────────────────────────────

// EpochStateReference is the chain-side epoch summary from PoSeq.
type EpochStateReference struct {
	Epoch                 uint64 `json:"epoch"`
	CommitteeHash         []byte `json:"committee_hash"`
	FinalizedBatchCount   uint64 `json:"finalized_batch_count"`
	MisbehaviorCount      uint32 `json:"misbehavior_count"`
	EvidencePacketCount   uint32 `json:"evidence_packet_count"`
	GovernanceEscalations uint32 `json:"governance_escalations"`
	// EpochStateHash is SHA256("epoch" | epoch_be | committee_hash | finalized_batch_count_be).
	EpochStateHash []byte `json:"epoch_state_hash"`
}

// ─── CheckpointAnchorRecord ──────────────────────────────────────────────────

// CheckpointAnchorRecord is the chain-side on-chain anchor for a PoSeq checkpoint.
// Write-once: once stored, it is immutable. Operators can query this to verify
// PoSeq's internal state after any dispute.
type CheckpointAnchorRecord struct {
	// CheckpointID is the PoSeq checkpoint's canonical ID.
	CheckpointID []byte `json:"checkpoint_id"`

	Epoch             uint64                 `json:"epoch"`
	Slot              uint64                 `json:"slot"`
	EpochStateHash    []byte                 `json:"epoch_state_hash"`
	BridgeStateHash   []byte                 `json:"bridge_state_hash"`
	MisbehaviorCount  uint32                 `json:"misbehavior_count"`
	FinalitySummary   BatchFinalityReference `json:"finality_summary"`
	// AnchorHash is SHA256("ckpt" | checkpoint_id | epoch_be | epoch_state_hash | bridge_state_hash).
	AnchorHash []byte `json:"anchor_hash"`
}

// ─── EvidencePacketSet ───────────────────────────────────────────────────────

// EvidencePacketSet is the full set of evidence for an epoch, as exported by
// the Rust ChainBridgeExporter.
type EvidencePacketSet struct {
	Epoch          uint64                        `json:"epoch"`
	Packets        []EvidencePacket              `json:"packets"`
	PenaltyRecords []PenaltyRecommendationRecord `json:"penalty_records"`
	// SetHash is SHA256(epoch_be | sorted_packet_hashes).
	SetHash []byte `json:"set_hash"`
}

// ─── ExportBatch ─────────────────────────────────────────────────────────────

// ExportBatch is the top-level payload produced by the Rust ChainBridgeExporter
// at the end of each epoch. The x/poseq keeper ingests this in a single tx.
type ExportBatch struct {
	Epoch            uint64                              `json:"epoch"`
	EvidenceSet      EvidencePacketSet                   `json:"evidence_set"`
	Escalations      []GovernanceEscalationRecord        `json:"escalations"`
	Suspensions      []CommitteeSuspensionRecommendation `json:"suspensions"`
	CheckpointAnchor *CheckpointAnchorRecord             `json:"checkpoint_anchor,omitempty"`
	EpochState       EpochStateReference                 `json:"epoch_state"`
	// LivenessEvents contains per-node liveness observations for this epoch.
	LivenessEvents []LivenessEvent `json:"liveness_events,omitempty"`
	// PerformanceRecords contains per-node performance attribution for this epoch.
	PerformanceRecords []NodePerformanceRecord `json:"performance_records,omitempty"`
	// StatusRecommendations are lifecycle changes recommended by PoSeq.
	// Applied if AutoApplySuspensions is true; else stored for governance review.
	StatusRecommendations []StatusRecommendation `json:"status_recommendations,omitempty"`
	// InactivityEvents contains per-node inactivity observations for this epoch.
	// Each entry records the number of consecutive missed epochs for a node.
	// Used by Phase 5 auto-enforcement to trigger suspension.
	InactivityEvents []InactivityEvent `json:"inactivity_events,omitempty"`
}

// ─── ExportBatchAck ──────────────────────────────────────────────────────────

// AckStatus describes the result of ingesting an ExportBatch on the Go chain.
type AckStatus string

const (
	AckStatusAccepted  AckStatus = "accepted"
	AckStatusDuplicate AckStatus = "duplicate"
	AckStatusRejected  AckStatus = "rejected"
)

// ExportBatchAck is the acknowledgment artifact written back to the bridge
// directory for PoSeq to consume. It tells PoSeq whether the chain accepted,
// duplicated, or rejected the export for a given epoch.
type ExportBatchAck struct {
	// SchemaVersion allows forward-compatible evolution of the ACK format.
	SchemaVersion uint32 `json:"schema_version"`
	// Epoch identifies which ExportBatch this ACK is for.
	Epoch uint64 `json:"epoch"`
	// BatchID is SHA256("bridge:epoch:" | epoch_be) — matches the Rust bridge batch_id.
	BatchID []byte `json:"batch_id"`
	// AckHash is SHA256("ack" | epoch_be | status_bytes) — used by PoSeq to
	// validate the ACK and transition the bridge lifecycle state.
	AckHash []byte `json:"ack_hash"`
	// Status indicates the ingestion outcome.
	Status AckStatus `json:"status"`
	// Reason provides human-readable context for rejected ACKs.
	Reason string `json:"reason,omitempty"`
	// BlockHeight is the Cosmos block at which the ingestion occurred.
	BlockHeight int64 `json:"block_height,omitempty"`
}

// ─── ExportBatchDedupRecord ──────────────────────────────────────────────────

// ExportBatchDedupRecord is persisted in the KV store to prevent double-ingestion
// of the same epoch's ExportBatch. Keyed by epoch.
type ExportBatchDedupRecord struct {
	Epoch     uint64    `json:"epoch"`
	Status    AckStatus `json:"status"`
	AckHash   []byte    `json:"ack_hash"`
	IngestedAt int64    `json:"ingested_at"` // block height
}
