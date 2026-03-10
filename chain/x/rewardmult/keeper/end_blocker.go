package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EndBlocker runs at the end of each block.
// It uses block height as a proxy for epochs (1 epoch = 100 blocks by default).
// When actual x/epochs hooks are wired, ProcessEpoch will be called from there instead.
// SAFETY: Never panics, all errors are logged.
func (k Keeper) EndBlocker(ctx sdk.Context) error {
	// Use block height as epoch proxy: epoch = blockHeight / 100
	// This provides a simple epoch mechanism without requiring x/epochs integration.
	// In production, this should be replaced with AfterEpochEnd hook.
	blockHeight := ctx.BlockHeight()
	epochLength := int64(100) // blocks per epoch
	if blockHeight%epochLength != 0 {
		return nil // not an epoch boundary
	}

	epoch := blockHeight / epochLength

	// Process the epoch - this computes all multipliers
	if err := k.ProcessEpoch(ctx, epoch); err != nil {
		k.logger.Error("epoch processing failed",
			"epoch", epoch,
			"block_height", blockHeight,
			"error", err,
		)
		// Never halt the chain
	}

	return nil
}
