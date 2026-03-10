package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EndBlocker runs at the end of each block.
// Recomputes governance weights at epoch boundaries.
// SAFETY: Never panics, all errors are logged.
func (k Keeper) EndBlocker(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return nil
	}

	blockHeight := ctx.BlockHeight()
	if blockHeight%params.RecomputeInterval != 0 {
		return nil // not a recompute boundary
	}

	epoch := blockHeight / params.RecomputeInterval

	if err := k.RecomputeAllWeights(ctx, epoch); err != nil {
		k.logger.Error("governance weight recomputation failed",
			"epoch", epoch,
			"block_height", blockHeight,
			"error", err,
		)
		// Never halt the chain
	}

	return nil
}
