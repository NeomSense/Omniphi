package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// ─── EpochRewardScore store ───────────────────────────────────────────────────

// StoreRewardScore stores an epoch reward score keyed by (epoch, nodeID).
// Last-write-wins.
func (k Keeper) StoreRewardScore(ctx context.Context, score types.EpochRewardScore) error {
	if score.NodeID == "" {
		return fmt.Errorf("node_id must not be empty")
	}
	if score.Epoch == 0 {
		return fmt.Errorf("epoch must be > 0")
	}
	bz, err := json.Marshal(score)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetRewardScoreKey(score.Epoch, score.NodeID), bz)
}

// GetRewardScore retrieves the reward score for a (epoch, nodeID) pair.
// Returns (nil, nil) if not found.
func (k Keeper) GetRewardScore(ctx context.Context, epoch uint64, nodeID string) (*types.EpochRewardScore, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetRewardScoreKey(epoch, nodeID))
	if err != nil || bz == nil {
		return nil, err
	}
	var score types.EpochRewardScore
	if err := json.Unmarshal(bz, &score); err != nil {
		return nil, err
	}
	return &score, nil
}

// AllRewardScoresForEpoch returns all reward scores for a given epoch via a
// prefix scan over [0x10 | epoch_be(8) | ...].
func (k Keeper) AllRewardScoresForEpoch(ctx context.Context, epoch uint64) ([]types.EpochRewardScore, error) {
	scanPrefix := types.GetRewardScoreEpochPrefix(epoch)

	// Build exclusive end: increment the epoch bytes.
	scanEnd := make([]byte, len(scanPrefix))
	copy(scanEnd, scanPrefix)
	for i := len(scanEnd) - 1; i >= 1; i-- {
		scanEnd[i]++
		if scanEnd[i] != 0 {
			break
		}
	}

	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(scanPrefix, scanEnd)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var scores []types.EpochRewardScore
	for ; iter.Valid(); iter.Next() {
		var score types.EpochRewardScore
		if err := json.Unmarshal(iter.Value(), &score); err != nil {
			k.logger.Error("failed to unmarshal reward score", "error", err)
			continue
		}
		scores = append(scores, score)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return scores, nil
}

// ComputeAndStoreRewardScore computes the reward score for (epoch, nodeID) using
// available performance and liveness data, then stores it.
//
// Algorithm:
//  1. Retrieve NodePerformanceRecord → BaseScoreBps = ParticipationRateBps
//  2. Retrieve LivenessEvent → UptimeScoreBps = 10000 if present, else 0
//  3. Retrieve PoCMultiplierRecord for the node's operator → default 10000
//  4. fault_penalty_bps = min(fault_events * 500, 5000)
//  5. FinalScoreBps = ComputeRewardScore(base, uptime, pocMult, fault_penalty)
//  6. Check active bond status
func (k Keeper) ComputeAndStoreRewardScore(ctx context.Context, epoch uint64, nodeIDHex string) (*types.EpochRewardScore, error) {
	score := types.EpochRewardScore{
		NodeID:           nodeIDHex,
		Epoch:            epoch,
		PoCMultiplierBps: 10000, // default: neutral
	}

	// 1. Performance record → base score
	pr, err := k.GetPerformanceRecordByHex(ctx, epoch, nodeIDHex)
	if err != nil {
		return nil, fmt.Errorf("fetching performance record: %w", err)
	}
	if pr != nil {
		score.BaseScoreBps = pr.ParticipationRateBps

		// fault_penalty_bps = min(fault_events * 500, 5000)
		penaltyBps := pr.FaultEvents * 500
		if penaltyBps > 5000 {
			penaltyBps = 5000
		}
		score.FaultPenaltyBps = uint32(penaltyBps)
	}

	// 2. Liveness event → uptime score
	nodeIDBytes, err := hexDecodeNodeID(nodeIDHex)
	if err != nil {
		return nil, err
	}
	le, err := k.GetLivenessEvent(ctx, epoch, nodeIDBytes)
	if err != nil {
		return nil, fmt.Errorf("fetching liveness event: %w", err)
	}
	if le != nil {
		score.UptimeScoreBps = 10000
	}

	// 3. Active bond → operator address + PoC multiplier
	bond, err := k.GetActiveBondForNode(ctx, nodeIDHex)
	if err != nil {
		// Non-fatal: proceed without bond data
		k.logger.Error("fetching active bond for reward score", "node_id", nodeIDHex, "error", err)
	}
	if bond != nil {
		score.OperatorAddress = bond.OperatorAddress
		score.IsBonded = true

		// 4. PoC multiplier for this operator
		pocRec, err := k.GetPoCMultiplier(ctx, epoch, bond.OperatorAddress)
		if err == nil && pocRec != nil {
			score.PoCMultiplierBps = pocRec.MultiplierBps
		}
	}

	// 5. Compute final score
	score.FinalScoreBps = types.ComputeRewardScore(
		score.BaseScoreBps,
		score.UptimeScoreBps,
		score.PoCMultiplierBps,
		score.FaultPenaltyBps,
	)

	// 6. Store
	if err := k.StoreRewardScore(ctx, score); err != nil {
		return nil, fmt.Errorf("storing reward score: %w", err)
	}
	return &score, nil
}

// ─── PoCMultiplierRecord store ────────────────────────────────────────────────

// StorePoCMultiplier stores a PoC multiplier record for (epoch, operatorAddress).
// Last-write-wins.
func (k Keeper) StorePoCMultiplier(ctx context.Context, rec types.PoCMultiplierRecord) error {
	if rec.OperatorAddress == "" {
		return fmt.Errorf("operator_address must not be empty")
	}
	if rec.Epoch == 0 {
		return fmt.Errorf("epoch must be > 0")
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetPoCMultiplierKey(rec.Epoch, rec.OperatorAddress), bz)
}

// GetPoCMultiplier retrieves a PoC multiplier record for (epoch, operatorAddress).
// Returns (nil, nil) if not found.
func (k Keeper) GetPoCMultiplier(ctx context.Context, epoch uint64, operatorAddress string) (*types.PoCMultiplierRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetPoCMultiplierKey(epoch, operatorAddress))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.PoCMultiplierRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// hexDecodeNodeID decodes a 64-char hex node ID into 32 bytes.
func hexDecodeNodeID(nodeIDHex string) ([]byte, error) {
	if len(nodeIDHex) != 64 {
		return nil, types.ErrInvalidNodeID.Wrap("node_id must be 64 hex chars")
	}
	b, err := hex.DecodeString(nodeIDHex)
	if err != nil {
		return nil, types.ErrInvalidNodeID.Wrap("node_id must be valid hex")
	}
	if len(b) != 32 {
		return nil, types.ErrInvalidNodeID.Wrap("node_id must decode to 32 bytes")
	}
	return b, nil
}
