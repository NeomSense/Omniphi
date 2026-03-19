package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// QueryServer implements query handlers for x/poseq.
type QueryServer struct {
	keeper *Keeper
}

func NewQueryServer(k *Keeper) *QueryServer {
	return &QueryServer{keeper: k}
}

// Params returns current module parameters.
func (q *QueryServer) Params(ctx context.Context) (*types.QueryParamsResponse, error) {
	params := q.keeper.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

// Sequencer returns a single sequencer by node_id hex.
func (q *QueryServer) Sequencer(ctx context.Context, req *types.QuerySequencerRequest) (*types.QuerySequencerResponse, error) {
	seq, err := q.keeper.GetSequencer(ctx, req.NodeId)
	if err != nil {
		return nil, err
	}
	return &types.QuerySequencerResponse{Sequencer: seq}, nil
}

// AllSequencers returns all registered sequencers.
func (q *QueryServer) AllSequencers(ctx context.Context) (*types.QueryAllSequencersResponse, error) {
	store := q.keeper.storeService.OpenKVStore(ctx)

	var sequencers []types.SequencerRecord

	// Scan all keys with sequencer prefix
	prefix := []byte{types.KeyPrefixSequencer}
	end := make([]byte, len(prefix)+1)
	copy(end, prefix)
	end[len(prefix)] = 0xFF
	iter, err := store.Iterator(prefix, end)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var rec types.SequencerRecord
		if err := json.Unmarshal(iter.Value(), &rec); err != nil {
			continue
		}
		sequencers = append(sequencers, rec)
	}

	return &types.QueryAllSequencersResponse{Sequencers: sequencers}, nil
}

// CommittedBatch returns a committed batch by batch_id hex.
func (q *QueryServer) CommittedBatch(ctx context.Context, req *types.QueryCommittedBatchRequest) (*types.QueryCommittedBatchResponse, error) {
	batch, err := q.keeper.GetCommittedBatch(ctx, req.BatchId)
	if err != nil {
		return nil, err
	}
	return &types.QueryCommittedBatchResponse{Batch: batch}, nil
}

// EvidencePacket returns an evidence packet by hash hex.
func (q *QueryServer) EvidencePacket(ctx context.Context, req *types.QueryEvidencePacketRequest) (*types.QueryEvidencePacketResponse, error) {
	hashBytes, err := hex.DecodeString(req.PacketHash)
	if err != nil || len(hashBytes) != 32 {
		return nil, fmt.Errorf("invalid packet_hash: must be 64-char hex")
	}
	pkt, err := q.keeper.GetEvidencePacket(ctx, hashBytes)
	if err != nil {
		return nil, err
	}
	return &types.QueryEvidencePacketResponse{Packet: pkt}, nil
}

// CheckpointAnchor returns a checkpoint anchor by epoch+slot.
func (q *QueryServer) CheckpointAnchor(ctx context.Context, req *types.QueryCheckpointAnchorRequest) (*types.QueryCheckpointAnchorResponse, error) {
	anchor, err := q.keeper.GetCheckpointAnchor(ctx, req.Epoch, req.Slot)
	if err != nil {
		return nil, err
	}
	return &types.QueryCheckpointAnchorResponse{Anchor: anchor}, nil
}

// SlashRecord returns a slash record by node_id + packet_hash.
func (q *QueryServer) SlashRecord(ctx context.Context, req *types.QuerySlashRecordRequest) (*types.QuerySlashRecordResponse, error) {
	rec, err := q.keeper.GetSlashRecord(ctx, req.NodeId, req.PacketHash)
	if err != nil {
		return nil, err
	}
	return &types.QuerySlashRecordResponse{Record: rec}, nil
}

// SettlementAnchor returns a settlement anchor by batch_hash hex.
func (q *QueryServer) SettlementAnchor(ctx context.Context, req *types.QuerySettlementAnchorRequest) (*types.QuerySettlementAnchorResponse, error) {
	anchor, err := q.keeper.GetSettlementAnchor(ctx, req.BatchHash)
	if err != nil {
		return nil, err
	}
	return &types.QuerySettlementAnchorResponse{Anchor: anchor}, nil
}

// EpochState returns epoch state by epoch number.
func (q *QueryServer) EpochState(ctx context.Context, req *types.QueryEpochStateRequest) (*types.QueryEpochStateResponse, error) {
	state, err := q.keeper.GetEpochState(ctx, req.Epoch)
	if err != nil {
		return nil, err
	}
	return &types.QueryEpochStateResponse{State: state}, nil
}

// ExportBatch returns the full export batch for an epoch.
func (q *QueryServer) ExportBatch(ctx context.Context, req *types.QueryExportBatchRequest) (*types.QueryExportBatchResponse, error) {
	batch, err := q.keeper.GetExportBatch(ctx, req.Epoch)
	if err != nil {
		return nil, err
	}
	return &types.QueryExportBatchResponse{Batch: batch}, nil
}

// CommitteeSnapshot returns the committee snapshot for an epoch.
func (q *QueryServer) CommitteeSnapshot(ctx context.Context, req *types.QueryCommitteeSnapshotRequest) (*types.QueryCommitteeSnapshotResponse, error) {
	snap, err := q.keeper.GetCommitteeSnapshot(ctx, req.Epoch)
	if err != nil {
		return nil, err
	}
	return &types.QueryCommitteeSnapshotResponse{Snapshot: snap}, nil
}

// LivenessEvent returns the liveness event for a (epoch, node_id) pair.
func (q *QueryServer) LivenessEvent(ctx context.Context, req *types.QueryLivenessEventRequest) (*types.QueryLivenessEventResponse, error) {
	ev, err := q.keeper.GetLivenessEventByHex(ctx, req.Epoch, req.NodeId)
	if err != nil {
		return nil, err
	}
	return &types.QueryLivenessEventResponse{Event: ev}, nil
}

// PerformanceRecord returns the performance record for a (epoch, node_id) pair.
func (q *QueryServer) PerformanceRecord(ctx context.Context, req *types.QueryPerformanceRecordRequest) (*types.QueryPerformanceRecordResponse, error) {
	rec, err := q.keeper.GetPerformanceRecordByHex(ctx, req.Epoch, req.NodeId)
	if err != nil {
		return nil, err
	}
	return &types.QueryPerformanceRecordResponse{Record: rec}, nil
}

// EpochPerformance returns all performance records for an epoch.
func (q *QueryServer) EpochPerformance(ctx context.Context, req *types.QueryEpochPerformanceRequest) (*types.QueryEpochPerformanceResponse, error) {
	records, err := q.keeper.AllPerformanceRecordsForEpoch(ctx, req.Epoch)
	if err != nil {
		return nil, err
	}
	return &types.QueryEpochPerformanceResponse{Records: records}, nil
}

// BuildCommitteeSnapshot builds (but does not store) a snapshot from current Active sequencers.
// Used by governance tooling before submitting MsgSubmitCommitteeSnapshot.
func (q *QueryServer) BuildCommitteeSnapshot(ctx context.Context, req *types.QueryCommitteeSnapshotRequest) (*types.QueryCommitteeSnapshotResponse, error) {
	sdkCtx, ok := ctx.(interface{ BlockHeight() int64 })
	var blockHeight int64
	if ok {
		blockHeight = sdkCtx.BlockHeight()
	}
	snap, err := q.keeper.BuildCommitteeSnapshot(ctx, req.Epoch, blockHeight)
	if err != nil {
		return nil, fmt.Errorf("building committee snapshot: %w", err)
	}
	return &types.QueryCommitteeSnapshotResponse{Snapshot: &snap}, nil
}

// ─── Phase 5: Economic Enforcement Layer queries ──────────────────────────────

// OperatorBond returns the operator bond for a (operatorAddress, nodeID) pair.
func (q *QueryServer) OperatorBond(ctx context.Context, req *types.QueryOperatorBondRequest) (*types.QueryOperatorBondResponse, error) {
	if req.NodeId == "" {
		return nil, fmt.Errorf("node_id must not be empty")
	}
	if req.OperatorAddress == "" {
		return nil, fmt.Errorf("operator_address must not be empty")
	}
	bond, err := q.keeper.GetOperatorBond(ctx, req.OperatorAddress, req.NodeId)
	if err != nil {
		return nil, err
	}
	return &types.QueryOperatorBondResponse{Bond: bond}, nil
}

// RewardScore returns the reward score for a (epoch, nodeID) pair.
func (q *QueryServer) RewardScore(ctx context.Context, req *types.QueryRewardScoreRequest) (*types.QueryRewardScoreResponse, error) {
	if req.NodeId == "" {
		return nil, fmt.Errorf("node_id must not be empty")
	}
	score, err := q.keeper.GetRewardScore(ctx, req.Epoch, req.NodeId)
	if err != nil {
		return nil, err
	}
	return &types.QueryRewardScoreResponse{Score: score}, nil
}

// EpochRewardScores returns all reward scores for a given epoch.
func (q *QueryServer) EpochRewardScores(ctx context.Context, req *types.QueryEpochRewardScoresRequest) (*types.QueryEpochRewardScoresResponse, error) {
	scores, err := q.keeper.AllRewardScoresForEpoch(ctx, req.Epoch)
	if err != nil {
		return nil, err
	}
	return &types.QueryEpochRewardScoresResponse{Scores: scores}, nil
}

// SlashQueue returns all pending (not yet executed) slash queue entries.
func (q *QueryServer) SlashQueue(ctx context.Context, _ *types.QuerySlashQueueRequest) (*types.QuerySlashQueueResponse, error) {
	entries, err := q.keeper.AllPendingSlashEntries(ctx)
	if err != nil {
		return nil, err
	}
	return &types.QuerySlashQueueResponse{Entries: entries}, nil
}

// OperatorProfile assembles the full economic profile for a node: sequencer record,
// active bond, latest reward score, and pending slash entries.
func (q *QueryServer) OperatorProfile(ctx context.Context, req *types.QueryOperatorProfileRequest) (*types.QueryOperatorProfileResponse, error) {
	if req.NodeId == "" {
		return nil, fmt.Errorf("node_id must not be empty")
	}

	resp := &types.QueryOperatorProfileResponse{}

	// Sequencer record
	seq, err := q.keeper.GetSequencer(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("fetching sequencer: %w", err)
	}
	resp.Sequencer = seq

	// Active bond (best-effort)
	bond, err := q.keeper.GetActiveBondForNode(ctx, req.NodeId)
	if err == nil {
		resp.Bond = bond
	}

	// Reward score for requested epoch (best-effort)
	if req.Epoch > 0 {
		score, err := q.keeper.GetRewardScore(ctx, req.Epoch, req.NodeId)
		if err == nil {
			resp.RewardScore = score
		}
	}

	// Pending slash entries for this node
	slashEntries, err := q.keeper.AllSlashEntriesForNode(ctx, req.NodeId)
	if err == nil {
		var pending []types.SlashQueueEntry
		for _, e := range slashEntries {
			if !e.Executed {
				pending = append(pending, e)
			}
		}
		resp.PendingSlashEntries = pending
	}

	return resp, nil
}

// PoCMultiplier returns the PoC multiplier record for a (epoch, operatorAddress) pair.
func (q *QueryServer) PoCMultiplier(ctx context.Context, req *types.QueryPoCMultiplierRequest) (*types.QueryPoCMultiplierResponse, error) {
	if req.OperatorAddress == "" {
		return nil, fmt.Errorf("operator_address must not be empty")
	}
	rec, err := q.keeper.GetPoCMultiplier(ctx, req.Epoch, req.OperatorAddress)
	if err != nil {
		return nil, err
	}
	return &types.QueryPoCMultiplierResponse{Record: rec}, nil
}
