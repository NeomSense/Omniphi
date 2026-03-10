package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/rewardmult/types"
)

// InitGenesis initializes the module from genesis state
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := gs.Validate(); err != nil {
		return fmt.Errorf("invalid genesis state: %w", err)
	}

	if err := k.SetParams(ctx, gs.Params); err != nil {
		return fmt.Errorf("failed to set params: %w", err)
	}

	for _, vm := range gs.Multipliers {
		if err := k.SetValidatorMultiplier(ctx, vm); err != nil {
			return fmt.Errorf("failed to set multiplier for %s: %w", vm.ValidatorAddress, err)
		}
	}

	for _, h := range gs.EmaHistory {
		if err := k.SetEMAHistory(ctx, h); err != nil {
			return fmt.Errorf("failed to set EMA history for %s: %w", h.ValidatorAddress, err)
		}
	}

	return nil
}

// ExportGenesis exports the current state as genesis
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:      k.GetParams(ctx),
		Multipliers: k.GetAllValidatorMultipliers(ctx),
		EmaHistory:  k.GetAllEMAHistories(ctx),
	}
}
