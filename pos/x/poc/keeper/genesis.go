package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// InitGenesis initializes the module's state from a provided genesis state
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	// Set params
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}

	// Set next contribution ID
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(types.KeyNextContributionID, sdk.Uint64ToBigEndian(gs.NextContributionId)); err != nil {
		return err
	}

	// Import contributions
	for _, contribution := range gs.Contributions {
		if err := k.SetContribution(ctx, contribution); err != nil {
			return err
		}
	}

	// Import credits
	for _, credits := range gs.Credits {
		if err := k.SetCredits(ctx, credits); err != nil {
			return err
		}
	}

	// Import fee metrics
	k.SetFeeMetrics(ctx, gs.FeeMetrics)

	// Import contributor fee stats
	for _, stats := range gs.ContributorFeeStats {
		k.SetContributorFeeStats(ctx, stats)
	}

	return nil
}

// ExportGenesis returns the module's exported genesis
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	contributions := k.GetAllContributions(ctx)
	credits := k.GetAllCredits(ctx)

	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyNextContributionID)

	var nextID uint64
	if err != nil || bz == nil {
		// If there's an error or no value, start from 1
		// Log warning but don't fail genesis export
		k.logger.Warn("failed to get next contribution ID during genesis export, defaulting to 1", "error", err)
		nextID = 1
	} else {
		nextID = sdk.BigEndianToUint64(bz)
	}

	// Export fee metrics
	feeMetrics := k.GetFeeMetrics(ctx)

	// Export contributor fee stats
	contributorFeeStats := k.GetAllContributorFeeStats(ctx)

	return &types.GenesisState{
		Params:              params,
		Contributions:       contributions,
		Credits:             credits,
		NextContributionId:  nextID,
		FeeMetrics:          feeMetrics,
		ContributorFeeStats: contributorFeeStats,
	}
}
