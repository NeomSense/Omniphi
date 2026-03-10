package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/repgov/types"
)

// RegisterInvariants registers all invariants for the repgov module
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "weight-bounds", WeightBoundsInvariant(k))
}

// WeightBoundsInvariant checks that all voter weights are within [1.0, MaxVotingWeightCap]
func WeightBoundsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		params := k.GetParams(ctx)
		weights := k.GetAllVoterWeights(ctx)

		for _, w := range weights {
			if w.EffectiveWeight.LT(math.LegacyOneDec()) {
				return sdk.FormatInvariant(
					types.ModuleName, "weight-bounds",
					fmt.Sprintf("voter %s has effective weight %s < 1.0", w.Address, w.EffectiveWeight),
				), true
			}
			if w.EffectiveWeight.GT(params.MaxVotingWeightCap) {
				return sdk.FormatInvariant(
					types.ModuleName, "weight-bounds",
					fmt.Sprintf("voter %s has effective weight %s > cap %s", w.Address, w.EffectiveWeight, params.MaxVotingWeightCap),
				), true
			}
		}

		return sdk.FormatInvariant(types.ModuleName, "weight-bounds", "all weights within bounds"), false
	}
}
