package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/uci/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}

	for _, adapter := range gs.Adapters {
		if err := k.SetAdapter(ctx, adapter); err != nil {
			return err
		}
	}

	if gs.NextAdapterID > 0 {
		if err := k.SetNextAdapterID(ctx, gs.NextAdapterID); err != nil {
			return err
		}
	}

	for _, cm := range gs.ContributionMappings {
		if err := k.SetContributionMapping(ctx, cm); err != nil {
			return err
		}
	}

	return nil
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:               k.GetParams(ctx),
		Adapters:             k.GetAllAdapters(ctx),
		NextAdapterID:        k.GetNextAdapterID(ctx),
		ContributionMappings: nil, // large set — skip for export efficiency
	}
}
