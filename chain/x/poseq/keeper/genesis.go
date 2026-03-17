package keeper

import (
	"context"

	"pos/x/poseq/types"
)

// InitGenesis initializes x/poseq state from a genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the current x/poseq genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	return &types.GenesisState{
		Params: k.GetParams(ctx),
	}
}
