package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/repgov/types"
)

// InitGenesis initializes the module's state from a provided genesis state
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}

	for _, vw := range gs.VoterWeights {
		if err := k.SetVoterWeight(ctx, vw); err != nil {
			return err
		}
	}

	for _, d := range gs.Delegations {
		if err := k.SetDelegation(ctx, d); err != nil {
			return err
		}
	}

	return nil
}

// ExportGenesis returns the module's exported genesis state
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:       k.GetParams(ctx),
		VoterWeights: k.GetAllVoterWeights(ctx),
		Delegations:  k.exportAllDelegations(ctx),
	}
}

// exportAllDelegations returns all delegations (for genesis export)
func (k Keeper) exportAllDelegations(ctx sdk.Context) []types.DelegatedReputation {
	var delegations []types.DelegatedReputation
	weights := k.GetAllVoterWeights(ctx)
	for _, w := range weights {
		delegations = append(delegations, k.GetDelegationsFrom(ctx, w.Address)...)
	}
	return delegations
}
