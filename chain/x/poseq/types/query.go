package types

// Query request/response types for x/poseq module.
// These are used by the gRPC query server and CLI commands.

// ─── Params ─────────────────────────────────────────────────

type QueryParamsRequest struct{}

type QueryParamsResponse struct {
	Params Params `json:"params"`
}

// ─── Sequencer ──────────────────────────────────────────────

type QuerySequencerRequest struct {
	NodeId string `json:"node_id"`
}

type QuerySequencerResponse struct {
	Sequencer *SequencerRecord `json:"sequencer"`
}

type QueryAllSequencersRequest struct{}

type QueryAllSequencersResponse struct {
	Sequencers []SequencerRecord `json:"sequencers"`
}

// ─── Committed Batch ────────────────────────────────────────

type QueryCommittedBatchRequest struct {
	BatchId string `json:"batch_id"`
}

type QueryCommittedBatchResponse struct {
	Batch *CommittedBatchRecord `json:"batch"`
}

// ─── Evidence Packet ────────────────────────────────────────

type QueryEvidencePacketRequest struct {
	PacketHash string `json:"packet_hash"`
}

type QueryEvidencePacketResponse struct {
	Packet *EvidencePacket `json:"packet"`
}

// ─── Checkpoint Anchor ──────────────────────────────────────

type QueryCheckpointAnchorRequest struct {
	Epoch uint64 `json:"epoch"`
	Slot  uint64 `json:"slot"`
}

type QueryCheckpointAnchorResponse struct {
	Anchor *CheckpointAnchorRecord `json:"anchor"`
}

// ─── Slash Record ───────────────────────────────────────────

type QuerySlashRecordRequest struct {
	NodeId     string `json:"node_id"`
	PacketHash string `json:"packet_hash"`
}

type QuerySlashRecordResponse struct {
	Record *SlashRecord `json:"record"`
}

// ─── Settlement Anchor ──────────────────────────────────────

type QuerySettlementAnchorRequest struct {
	BatchHash string `json:"batch_hash"`
}

type QuerySettlementAnchorResponse struct {
	Anchor *SettlementAnchorRecord `json:"anchor"`
}

// ─── Epoch State ────────────────────────────────────────────

type QueryEpochStateRequest struct {
	Epoch uint64 `json:"epoch"`
}

type QueryEpochStateResponse struct {
	State *EpochStateReference `json:"state"`
}

// ─── Export Batch ───────────────────────────────────────────

type QueryExportBatchRequest struct {
	Epoch uint64 `json:"epoch"`
}

type QueryExportBatchResponse struct {
	Batch *ExportBatch `json:"batch"`
}

// ─── Committee Snapshot ─────────────────────────────────────

type QueryCommitteeSnapshotRequest struct {
	Epoch uint64 `json:"epoch"`
}

type QueryCommitteeSnapshotResponse struct {
	Snapshot *CommitteeSnapshot `json:"snapshot"`
}

// ─── Liveness Event ─────────────────────────────────────────

type QueryLivenessEventRequest struct {
	Epoch  uint64 `json:"epoch"`
	NodeId string `json:"node_id"` // 64 hex chars
}

type QueryLivenessEventResponse struct {
	Event *LivenessEvent `json:"event"`
}

// ─── Performance Record ─────────────────────────────────────

type QueryPerformanceRecordRequest struct {
	Epoch  uint64 `json:"epoch"`
	NodeId string `json:"node_id"` // 64 hex chars
}

type QueryPerformanceRecordResponse struct {
	Record *NodePerformanceRecord `json:"record"`
}

type QueryEpochPerformanceRequest struct {
	Epoch uint64 `json:"epoch"`
}

type QueryEpochPerformanceResponse struct {
	Records []NodePerformanceRecord `json:"records"`
}

// ─── Operator Bond ───────────────────────────────────────────

type QueryOperatorBondRequest struct {
	OperatorAddress string `json:"operator_address"`
	NodeId          string `json:"node_id"` // 64 hex chars
}

type QueryOperatorBondResponse struct {
	Bond *OperatorBond `json:"bond"`
}

// ─── Reward Score ────────────────────────────────────────────

type QueryRewardScoreRequest struct {
	Epoch  uint64 `json:"epoch"`
	NodeId string `json:"node_id"` // 64 hex chars
}

type QueryRewardScoreResponse struct {
	Score *EpochRewardScore `json:"score"`
}

type QueryEpochRewardScoresRequest struct {
	Epoch uint64 `json:"epoch"`
}

type QueryEpochRewardScoresResponse struct {
	Scores []EpochRewardScore `json:"scores"`
}

// ─── Slash Queue ─────────────────────────────────────────────

type QuerySlashQueueRequest struct{}

type QuerySlashQueueResponse struct {
	Entries []SlashQueueEntry `json:"entries"`
}

// ─── PoC Multiplier ──────────────────────────────────────────

type QueryPoCMultiplierRequest struct {
	Epoch           uint64 `json:"epoch"`
	OperatorAddress string `json:"operator_address"`
}

type QueryPoCMultiplierResponse struct {
	Record *PoCMultiplierRecord `json:"record"`
}

// ─── Operator Profile ────────────────────────────────────────

// QueryOperatorProfileRequest requests the full economic profile for an operator node.
type QueryOperatorProfileRequest struct {
	// NodeId is the 64-char hex node identity.
	NodeId string `json:"node_id"`
	// Epoch is the epoch for which to fetch the reward score.
	// If 0, the most recent stored score is returned (best-effort).
	Epoch uint64 `json:"epoch,omitempty"`
}

// QueryOperatorProfileResponse contains all available economic data for a node.
type QueryOperatorProfileResponse struct {
	// Sequencer is the lifecycle record for this node.
	Sequencer *SequencerRecord `json:"sequencer"`
	// Bond is the active operator bond, if any.
	Bond *OperatorBond `json:"bond,omitempty"`
	// RewardScore is the most recent epoch reward score, if available.
	RewardScore *EpochRewardScore `json:"reward_score,omitempty"`
	// PendingSlashEntries are slash queue entries for this node that have not
	// yet been executed.
	PendingSlashEntries []SlashQueueEntry `json:"pending_slash_entries,omitempty"`
}
