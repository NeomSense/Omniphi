package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

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
