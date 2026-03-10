package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/rewardmult/types"
)

// RegisterInvariants registers module invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "multiplier-bounds", MultiplierBoundsInvariant(k))
	ir.RegisterRoute(types.ModuleName, "budget-neutral", BudgetNeutralInvariant(k))
	ir.RegisterRoute(types.ModuleName, "no-nan-inf", NoNaNInvariant(k))
}

// AllInvariants returns a combined invariant check
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		msg, broken := MultiplierBoundsInvariant(k)(ctx)
		if broken {
			return msg, broken
		}
		msg, broken = BudgetNeutralInvariant(k)(ctx)
		if broken {
			return msg, broken
		}
		return NoNaNInvariant(k)(ctx)
	}
}

// MultiplierBoundsInvariant checks that all effective multipliers are within [min, max]
func MultiplierBoundsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		params := k.GetParams(ctx)
		multipliers := k.GetAllValidatorMultipliers(ctx)

		for _, vm := range multipliers {
			if vm.MEffective.LT(params.MinMultiplier) {
				msg := fmt.Sprintf("validator %s effective multiplier %s below minimum %s",
					vm.ValidatorAddress, vm.MEffective, params.MinMultiplier)
				return sdk.FormatInvariant(types.ModuleName, "multiplier-bounds", msg), true
			}
			if vm.MEffective.GT(params.MaxMultiplier) {
				msg := fmt.Sprintf("validator %s effective multiplier %s above maximum %s",
					vm.ValidatorAddress, vm.MEffective, params.MaxMultiplier)
				return sdk.FormatInvariant(types.ModuleName, "multiplier-bounds", msg), true
			}
		}

		return sdk.FormatInvariant(types.ModuleName, "multiplier-bounds", "all multipliers within bounds"), false
	}
}

// BudgetNeutralInvariant checks that Σ(stakeWeight × MEffective) ≈ Σ(stakeWeight).
//
// V2.2: Tightened from 1% to 1e-6 relative error bound. This is safe because:
//   - LegacyDec uses 18 decimal places, so rounding noise is ~1e-18 per operation.
//   - With V2.2 iterative normalization, the only budget drift comes from validators
//     clamped at min/max whose "excess" cannot be fully redistributed because all
//     remaining validators are also near a clamp boundary.
//   - With default bounds [0.85, 1.15], the worst-case clamp-break residual across
//     100 validators is well under 1e-6.
//   - We use 1e-6 (not 1e-18) to provide margin for accumulated rounding across
//     many multiplications. This is strict enough to detect real inflation drift
//     but loose enough to never false-positive from harmless LegacyDec rounding.
func BudgetNeutralInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		multipliers := k.GetAllValidatorMultipliers(ctx)
		if len(multipliers) == 0 {
			return sdk.FormatInvariant(types.ModuleName, "budget-neutral", "no multipliers to check"), false
		}

		validators, err := k.stakingKeeper.GetAllValidators(ctx)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "budget-neutral",
				fmt.Sprintf("failed to get validators: %v", err)), false
		}

		totalBonded, err := k.stakingKeeper.TotalBondedTokens(ctx)
		if err != nil || totalBonded.IsZero() {
			return sdk.FormatInvariant(types.ModuleName, "budget-neutral", "no bonded tokens"), false
		}

		// Build map of multipliers
		multMap := make(map[string]types.ValidatorMultiplier)
		for _, vm := range multipliers {
			multMap[vm.ValidatorAddress] = vm
		}

		weightedMultSum := math.LegacyZeroDec()
		totalWeight := math.LegacyZeroDec()

		for _, val := range validators {
			if !val.IsBonded() {
				continue
			}
			stakeWeight := val.Tokens.ToLegacyDec()
			totalWeight = totalWeight.Add(stakeWeight)

			valAddr := val.GetOperator()
			vm, found := multMap[valAddr]
			mEff := math.LegacyOneDec()
			if found {
				mEff = vm.MEffective
			}
			weightedMultSum = weightedMultSum.Add(stakeWeight.Mul(mEff))
		}

		if totalWeight.IsZero() {
			return sdk.FormatInvariant(types.ModuleName, "budget-neutral", "no bonded weight"), false
		}

		// Check: |weightedMultSum - totalWeight| / totalWeight < epsilon
		diff := weightedMultSum.Sub(totalWeight).Abs()
		ratio := diff.Quo(totalWeight)

		// V2.2: Tight epsilon. 1e-6 = 0.0001% relative error.
		// Safe with 18-decimal LegacyDec and iterative normalization.
		epsilon := math.LegacyNewDecWithPrec(1, 6) // 0.000001

		if ratio.GT(epsilon) {
			msg := fmt.Sprintf("budget neutrality violated: weighted_sum=%s, total_weight=%s, ratio_diff=%s (epsilon=1e-6)",
				weightedMultSum, totalWeight, ratio)
			return sdk.FormatInvariant(types.ModuleName, "budget-neutral", msg), true
		}

		return sdk.FormatInvariant(types.ModuleName, "budget-neutral",
			fmt.Sprintf("budget neutral within tolerance (error=%s)", ratio)), false
	}
}

// ============================================================================
// V2.1 Safety Invariants
// ============================================================================

// NoNaNInvariant checks that no multiplier values contain NaN, Inf, or nil values.
// Any such value in the multiplier math would corrupt reward distribution.
func NoNaNInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		multipliers := k.GetAllValidatorMultipliers(ctx)

		for _, vm := range multipliers {
			// Check each decimal field for nil, NaN-like, or unreasonable values
			for _, check := range []struct {
				name string
				val  math.LegacyDec
			}{
				{"MRaw", vm.MRaw},
				{"MEMA", vm.MEMA},
				{"MEffective", vm.MEffective},
				{"UptimeBonus", vm.UptimeBonus},
				{"ParticipationBonus", vm.ParticipationBonus},
				{"SlashPenalty", vm.SlashPenalty},
				{"FraudPenalty", vm.FraudPenalty},
			} {
				if check.val.IsNil() {
					msg := fmt.Sprintf("validator %s has nil %s", vm.ValidatorAddress, check.name)
					return sdk.FormatInvariant(types.ModuleName, "no-nan-inf", msg), true
				}
				// Detect overflow/unreasonable values: no multiplier field should exceed 100
				absVal := check.val.Abs()
				if absVal.GT(math.LegacyNewDec(100)) {
					msg := fmt.Sprintf("validator %s has unreasonable %s value: %s (possible overflow)",
						vm.ValidatorAddress, check.name, check.val)
					return sdk.FormatInvariant(types.ModuleName, "no-nan-inf", msg), true
				}
			}
		}

		return sdk.FormatInvariant(types.ModuleName, "no-nan-inf", "all multiplier values are valid"), false
	}
}
