package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"

	"pos/x/poseq/types"
)

// ─── Settlement Anchor Storage ───────────────────────────────────────────────

// StoreSettlementAnchor stores a settlement anchor record. Write-once per batch_hash.
func (k Keeper) StoreSettlementAnchor(ctx context.Context, anchor types.SettlementAnchorRecord) error {
	batchHashBytes, err := hex.DecodeString(anchor.BatchHash)
	if err != nil {
		return fmt.Errorf("invalid batch_hash hex: %w", err)
	}

	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetSettlementAnchorKey(batchHashBytes)

	// Write-once check
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		return types.ErrBatchAlreadyCommitted
	}

	data, err := json.Marshal(anchor)
	if err != nil {
		return err
	}

	return kvStore.Set(key, data)
}

// GetSettlementAnchor retrieves a settlement anchor by batch hash (hex).
func (k Keeper) GetSettlementAnchor(ctx context.Context, batchHashHex string) (*types.SettlementAnchorRecord, error) {
	batchHashBytes, err := hex.DecodeString(batchHashHex)
	if err != nil {
		return nil, fmt.Errorf("invalid batch_hash hex: %w", err)
	}

	kvStore := k.storeService.OpenKVStore(ctx)
	data, err := kvStore.Get(types.GetSettlementAnchorKey(batchHashBytes))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var anchor types.SettlementAnchorRecord
	if err := json.Unmarshal(data, &anchor); err != nil {
		return nil, err
	}

	return &anchor, nil
}

// ─── EpochSettlementRecord store ──────────────────────────────────────────────

// StoreEpochSettlement stores an epoch settlement record for (epoch, nodeID).
// Overwrites any prior record for the same (epoch, nodeID).
func (k Keeper) StoreEpochSettlement(ctx context.Context, rec types.EpochSettlementRecord) error {
	if len(rec.NodeID) != 64 {
		return types.ErrInvalidNodeID.Wrap("settlement: node_id must be 64 hex chars")
	}
	if rec.Epoch == 0 {
		return types.ErrInvalidEpoch
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetEpochSettlementKey(rec.Epoch, rec.NodeID), bz)
}

// GetEpochSettlement retrieves the settlement record for (epoch, nodeIDHex).
// Returns (nil, nil) if not found.
func (k Keeper) GetEpochSettlement(ctx context.Context, epoch uint64, nodeIDHex string) (*types.EpochSettlementRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetEpochSettlementKey(epoch, nodeIDHex))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.EpochSettlementRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// AllSettlementsForEpoch returns all epoch settlement records for the given epoch.
// Uses a prefix scan over [0x14 | epoch_be(8)].
func (k Keeper) AllSettlementsForEpoch(ctx context.Context, epoch uint64) ([]types.EpochSettlementRecord, error) {
	prefix := types.GetEpochSettlementPrefix(epoch)
	end := storetypes.PrefixEndBytes(prefix)

	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(prefix, end)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var records []types.EpochSettlementRecord
	for ; iter.Valid(); iter.Next() {
		var rec types.EpochSettlementRecord
		if err := json.Unmarshal(iter.Value(), &rec); err != nil {
			k.logger.Error("failed to unmarshal epoch settlement", "error", err)
			continue
		}
		records = append(records, rec)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return records, nil
}

// ComputeEpochSettlement builds an EpochSettlementRecord for a node from
// its reward score and any slash executions this epoch.
//
// Formula (all integer):
//
//	slash_penalty_bps = min(slashBpsThisEpoch, gross_reward_score)
//	net_reward_score  = clamp(gross - fault_penalty - slash_penalty, 0, 20000)
func (k Keeper) ComputeEpochSettlement(
	ctx context.Context,
	rewardScore types.EpochRewardScore,
	slashBpsThisEpoch uint32,
	slashCountThisEpoch uint32,
) (types.EpochSettlementRecord, error) {
	slashPenaltyBps := slashBpsThisEpoch
	if slashPenaltyBps > rewardScore.FinalScoreBps {
		slashPenaltyBps = rewardScore.FinalScoreBps
	}

	var netReward uint32
	if rewardScore.FinalScoreBps > slashPenaltyBps {
		netReward = rewardScore.FinalScoreBps - slashPenaltyBps
	}
	if netReward > 20_000 {
		netReward = 20_000
	}

	pocMult := rewardScore.PoCMultiplierBps
	if pocMult == 0 {
		pocMult = 10_000
	}

	operatorAddr := ""
	if bond, err := k.GetActiveBondForNode(ctx, rewardScore.NodeID); err == nil && bond != nil {
		operatorAddr = bond.OperatorAddress
	}

	return types.EpochSettlementRecord{
		NodeID:                 rewardScore.NodeID,
		OperatorAddress:        operatorAddr,
		Epoch:                  rewardScore.Epoch,
		GrossRewardScore:       rewardScore.FinalScoreBps,
		PoCMultiplierBps:       pocMult,
		FaultPenaltyBps:        rewardScore.FaultPenaltyBps,
		SlashPenaltyBps:        slashPenaltyBps,
		NetRewardScore:         netReward,
		IsBonded:               rewardScore.IsBonded,
		BondMultiplierApplied:  rewardScore.IsBonded,
		SlashExecutedThisEpoch: slashCountThisEpoch,
	}, nil
}

// SettleEpoch computes and stores settlement records for all nodes with reward
// scores in the given epoch. Best-effort: per-node failures are logged.
func (k Keeper) SettleEpoch(ctx context.Context, epoch uint64) error {
	rewardScores, err := k.AllRewardScoresForEpoch(ctx, epoch)
	if err != nil {
		return fmt.Errorf("fetching reward scores for epoch %d: %w", epoch, err)
	}

	for _, rs := range rewardScores {
		slashEntries, sErr := k.AllSlashEntriesForNode(ctx, rs.NodeID)
		var slashBpsSum uint32
		var slashCount uint32
		if sErr == nil {
			for _, e := range slashEntries {
				if e.Executed && e.Epoch == epoch {
					slashBpsSum += e.SlashBps
					slashCount++
				}
			}
		}

		settlement, cErr := k.ComputeEpochSettlement(ctx, rs, slashBpsSum, slashCount)
		if cErr != nil {
			k.logger.Error("settlement computation failed",
				"node_id", rs.NodeID,
				"epoch", epoch,
				"error", cErr,
			)
			continue
		}

		if storeErr := k.StoreEpochSettlement(ctx, settlement); storeErr != nil {
			k.logger.Error("failed to store epoch settlement",
				"node_id", rs.NodeID,
				"epoch", epoch,
				"error", storeErr,
			)
		}
	}

	return nil
}

// ─── MsgAnchorSettlement Handler ─────────────────────────────────────────────

// AnchorSettlement handles the MsgAnchorSettlement message.
func (m MsgServer) AnchorSettlement(ctx context.Context, msg *types.MsgAnchorSettlement) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// Authorization check: if AuthorizedSubmitter is set, sender must match
	params := m.GetParams(ctx)
	if params.AuthorizedSubmitter != "" && msg.Sender != params.AuthorizedSubmitter {
		return types.ErrUnauthorized
	}

	// Set submitter
	msg.Anchor.SubmitterAddress = msg.Sender

	// Store (write-once)
	if err := m.StoreSettlementAnchor(ctx, msg.Anchor); err != nil {
		return err
	}

	m.Logger().Info("settlement anchor stored",
		"batch_hash", msg.Anchor.BatchHash,
		"epoch", msg.Anchor.Epoch,
		"settled", msg.Anchor.SettledCount,
		"failed", msg.Anchor.FailedCount,
	)

	return nil
}
