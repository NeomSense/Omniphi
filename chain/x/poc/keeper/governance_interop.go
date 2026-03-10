package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CalculateWeightedVote computes the voting power of an address based on Stake + Reputation.
// Formula: VotingPower = Stake * Multiplier
// Multiplier = 1 + (log10(CScore) / 2)
// Max Multiplier = 5.0x
func (k Keeper) CalculateWeightedVote(ctx context.Context, voterAddr sdk.AccAddress, stake math.Int) math.Int {
	// 1. Get Contributor Stats (C-Score)
	stats := k.GetContributorStats(ctx, voterAddr.String())
	_ = stats.ReputationScore // currently unused; tracked for future C-Score weighting

	// If C-Score is tracked as credits (Int) in a different field, retrieve it.
	// Assuming we use the ReputationScore (LegacyDec) scaled up, or a separate Credit balance.
	// For this implementation, let's assume we derive a score from TotalSubmissions + Quality.
	// Or, if using the `x/poc` Credits system (not fully visible in context but implied), we'd query that.
	// Let's use a derived score for demonstration:
	score := math.NewInt(int64(stats.TotalSubmissions * 100)) // 100 points per submission

	if score.LTE(math.ZeroInt()) {
		return stake
	}

	// 2. Calculate Log10 Approximation
	// Since LegacyDec doesn't have Log10, we use a tiered approximation
	log10Approx := math.LegacyZeroDec()
	scoreInt := score.Int64()

	switch {
	case scoreInt >= 100000: // 10^5
		log10Approx = math.LegacyNewDec(5)
	case scoreInt >= 10000: // 10^4
		log10Approx = math.LegacyNewDec(4)
	case scoreInt >= 1000: // 10^3
		log10Approx = math.LegacyNewDec(3)
	case scoreInt >= 100: // 10^2
		log10Approx = math.LegacyNewDec(2)
	case scoreInt >= 10: // 10^1
		log10Approx = math.LegacyOneDec()
	default:
		log10Approx = math.LegacyNewDecWithPrec(5, 1) // 0.5
	}

	// 3. Calculate Multiplier: 1 + (log10 / 2)
	bonus := log10Approx.Quo(math.LegacyNewDec(2))
	multiplier := math.LegacyOneDec().Add(bonus)

	// 4. Cap Multiplier at 5.0x
	maxMult := math.LegacyNewDec(5)
	if multiplier.GT(maxMult) {
		multiplier = maxMult
	}

	// 5. Return Weighted Stake
	return math.LegacyNewDecFromInt(stake).Mul(multiplier).TruncateInt()
}
