package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// ============================================================================
// Configurable Originality Bands
// ============================================================================

func TestRewardLogic_DefaultBands_HighSimilarity(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	input := types.RewardContext{
		ClaimID:         1,
		Contributor:     sdk.AccAddress("bandtest1___________").String(),
		QualityScore:    math.LegacyNewDec(10), // max quality
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(90, 2), // 0.90 → should get 0.4x
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	// 1000 * (10/10) * 0.4 * 1.0 (rep) = 400
	// Immediate = 20% of 400 = 80, Vested = 320
	require.True(t, output.OriginalityMultiplier.Equal(math.LegacyNewDecWithPrec(4, 1)),
		"expected 0.4, got %s", output.OriginalityMultiplier)
}

func TestRewardLogic_DefaultBands_LowSimilarity(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	input := types.RewardContext{
		ClaimID:         2,
		Contributor:     sdk.AccAddress("bandtest2___________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(30, 2), // 0.30 → should get 1.2x bonus
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	require.True(t, output.OriginalityMultiplier.Equal(math.LegacyNewDecWithPrec(12, 1)),
		"expected 1.2, got %s", output.OriginalityMultiplier)
}

func TestRewardLogic_ConfigurableBands_Custom(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	// Set custom bands via params
	params := f.keeper.GetParams(ctx)
	params.EnableConfigurableBands = true
	params.OriginalityBands = []types.OriginalityBand{
		{MinSimilarity: math.LegacyNewDecWithPrec(80, 2), MaxSimilarity: math.LegacyNewDecWithPrec(101, 2), Multiplier: math.LegacyNewDecWithPrec(2, 1)},  // >= 0.80: 0.2x
		{MinSimilarity: math.LegacyZeroDec(), MaxSimilarity: math.LegacyNewDecWithPrec(80, 2), Multiplier: math.LegacyNewDecWithPrec(15, 1)},               // < 0.80: 1.5x
	}
	err := f.keeper.SetParams(ctx, params)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         3,
		Contributor:     sdk.AccAddress("bandtest3___________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(85, 2), // 0.85 → custom band gives 0.2x
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	require.True(t, output.OriginalityMultiplier.Equal(math.LegacyNewDecWithPrec(2, 1)),
		"expected 0.2 from custom band, got %s", output.OriginalityMultiplier)
}

func TestRewardLogic_DefaultBands_BoundaryMatching(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	// Exactly at 0.75 boundary (should match 0.75-0.85 band → 0.7x, not the 0.50-0.75 band)
	input := types.RewardContext{
		ClaimID:         4,
		Contributor:     sdk.AccAddress("bandtest4___________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(75, 2), // exactly 0.75
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	// 0.75 is >= 0.75 and < 0.85, so should get 0.7x band
	require.True(t, output.OriginalityMultiplier.Equal(math.LegacyNewDecWithPrec(7, 1)),
		"expected 0.7 at boundary, got %s", output.OriginalityMultiplier)
}

// ============================================================================
// Reputation Integration
// ============================================================================

func TestRewardLogic_ReputationFloor(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("repfloor____________").String()

	// Set reputation to floor (0.1)
	stats := types.NewContributorStats(contributor, 0)
	stats.ReputationScore = math.LegacyNewDecWithPrec(1, 1) // 0.1
	err := f.keeper.SetContributorStats(ctx, stats)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         10,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(30, 2), // 1.2x band
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	// 1000 * 1.0 * 1.2 * 0.1 = 120
	require.True(t, output.FinalRewardAmount.Add(output.TotalRoyaltyPaid).Equal(math.NewInt(120)),
		"expected gross 120 with 0.1 reputation, got %s + %s",
		output.FinalRewardAmount, output.TotalRoyaltyPaid)
}

func TestRewardLogic_ReputationDecay(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("repdecay____________").String()

	// Submit duplicate → should decay reputation by 10%
	input := types.RewardContext{
		ClaimID:      11,
		Contributor:  contributor,
		IsDuplicate:  true,
		QualityScore: math.LegacyNewDec(5),
		BaseReward:   math.NewInt(1000),
	}

	err := f.keeper.UpdateContributorStats(ctx, contributor, input)
	require.NoError(t, err)

	stats := f.keeper.GetContributorStats(ctx, contributor)
	require.Equal(t, uint64(1), stats.DuplicateCount)
	// 1.0 * 0.90 = 0.90
	require.True(t, stats.ReputationScore.Equal(math.LegacyNewDecWithPrec(90, 2)),
		"expected 0.90, got %s", stats.ReputationScore)
}

func TestRewardLogic_ReputationCap(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("repcap______________").String()

	// Submit good contribution → should increase reputation by 1%
	input := types.RewardContext{
		ClaimID:         12,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(8),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(30, 2), // low similarity = good
	}

	err := f.keeper.UpdateContributorStats(ctx, contributor, input)
	require.NoError(t, err)

	stats := f.keeper.GetContributorStats(ctx, contributor)
	// 1.0 * 1.01 = 1.01, but capped at 1.0
	require.True(t, stats.ReputationScore.Equal(math.LegacyOneDec()),
		"expected capped at 1.0, got %s", stats.ReputationScore)
}

// ============================================================================
// Repeat Offender Penalties
// ============================================================================

func TestRewardLogic_RepeatOffender_RewardCap(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("offender1___________").String()

	// Set up stats with enough offenses to trigger threshold (default=3)
	stats := types.NewContributorStats(contributor, 0)
	stats.DuplicateCount = 3
	err := f.keeper.SetContributorStats(ctx, stats)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         20,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(30, 2), // 1.2x = 1200 gross normally
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	// RepeatOffenderRewardCap = 0.50 * 1000 = 500 max
	// But also reputation is still 1.0, so gross = 1000 * 1.0 * 1.2 * 1.0 = 1200 → capped to 500
	totalGross := output.FinalRewardAmount.Add(output.TotalRoyaltyPaid)
	require.True(t, totalGross.LTE(math.NewInt(500)),
		"expected capped at 500, got %s", totalGross)
}

func TestRewardLogic_RepeatOffender_VestingExtension(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("offender2___________").String()

	f.bankKeeper.setBalance(
		sdk.AccAddress("module_address______").String(),
		"omniphi",
		math.NewInt(1000000),
	)

	// Set up stats with offenses at threshold
	stats := types.NewContributorStats(contributor, 0)
	stats.DuplicateCount = 3
	err := f.keeper.SetContributorStats(ctx, stats)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         21,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(60, 2), // 1.0x band
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)

	// Distribute rewards (creates vesting with extended epochs)
	if output.FinalRewardAmount.IsPositive() {
		err = f.keeper.DistributeRewards(ctx, output, contributor)
		require.NoError(t, err)

		vs, found := f.keeper.GetVestingSchedule(ctx, contributor, 21)
		if found && vs.TotalAmount.IsPositive() {
			// Default vesting = 10 epochs, RepeatOffenderVestingMultiplier = 2.0
			// So effective = 10 * 2.0 = 20 epochs
			require.True(t, vs.VestingEpochs >= 20,
				"expected extended vesting >= 20, got %d", vs.VestingEpochs)
		}
	}
}

func TestRewardLogic_RepeatOffender_BelowThreshold(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("offender3___________").String()

	// Set up stats below threshold (2 < 3)
	stats := types.NewContributorStats(contributor, 0)
	stats.DuplicateCount = 2
	err := f.keeper.SetContributorStats(ctx, stats)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         22,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(30, 2), // 1.2x
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	// No cap: gross = 1000 * 1.0 * 1.2 * 1.0 = 1200
	totalGross := output.FinalRewardAmount.Add(output.TotalRoyaltyPaid)
	require.True(t, totalGross.Equal(math.NewInt(1200)),
		"expected uncapped 1200, got %s", totalGross)
}

// ============================================================================
// Multi-Level Royalty Routing
// ============================================================================

func TestRewardLogic_Royalty_ParentOnly(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("royalty1____________").String()
	parent := sdk.AccAddress("parent1_____________").String()

	// Create parent contribution (no grandparent)
	parentContrib := types.NewContribution(100, parent, "code", "ipfs://parent", []byte{100}, 10, 1000)
	parentContrib.ParentClaimId = 0 // no grandparent
	err := f.keeper.SetContribution(ctx, parentContrib)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         30,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(80, 2), // 0.7x (derivative range)
		ParentClaimID:   100,
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	require.Len(t, output.RoyaltyRoutes, 1)
	require.Equal(t, parent, output.RoyaltyRoutes[0].Recipient)
	require.Equal(t, uint32(1), output.RoyaltyRoutes[0].Depth)
	// Parent royalty = 10% of gross (1000 * 1.0 * 0.7 = 700) = 70
	require.True(t, output.RoyaltyRoutes[0].Amount.Equal(math.NewInt(70)),
		"expected parent royalty 70, got %s", output.RoyaltyRoutes[0].Amount)
}

func TestRewardLogic_Royalty_ParentAndGrandparent(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("royalty2____________").String()
	parent := sdk.AccAddress("parent2_____________").String()
	grandparent := sdk.AccAddress("grandparent2________").String()

	// Create grandparent contribution
	gpContrib := types.NewContribution(200, grandparent, "code", "ipfs://gp", []byte{200}, 10, 1000)
	gpContrib.ParentClaimId = 0
	err := f.keeper.SetContribution(ctx, gpContrib)
	require.NoError(t, err)

	// Create parent contribution pointing to grandparent
	parentContrib := types.NewContribution(201, parent, "code", "ipfs://parent2", []byte{201}, 10, 1000)
	parentContrib.ParentClaimId = 200
	err = f.keeper.SetContribution(ctx, parentContrib)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         31,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(80, 2), // 0.7x
		ParentClaimID:   201,
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	require.Len(t, output.RoyaltyRoutes, 2, "expected 2 routes (parent + grandparent)")

	// Route 1: parent gets 10% of 700 = 70
	require.Equal(t, parent, output.RoyaltyRoutes[0].Recipient)
	require.True(t, output.RoyaltyRoutes[0].Amount.Equal(math.NewInt(70)))

	// Route 2: grandparent gets 5% of 700 = 35
	require.Equal(t, grandparent, output.RoyaltyRoutes[1].Recipient)
	require.True(t, output.RoyaltyRoutes[1].Amount.Equal(math.NewInt(35)))

	// Total royalty = 105
	require.True(t, output.TotalRoyaltyPaid.Equal(math.NewInt(105)))
}

func TestRewardLogic_Royalty_MaxTotalCap(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("royalty3____________").String()
	parent := sdk.AccAddress("parent3_____________").String()
	grandparent := sdk.AccAddress("grandparent3________").String()

	// Set aggressive royalty shares that would exceed cap
	params := f.keeper.GetParams(ctx)
	params.RoyaltyShare = math.LegacyNewDecWithPrec(20, 2)            // 20%
	params.GrandparentRoyaltyShare = math.LegacyNewDecWithPrec(15, 2) // 15%
	params.MaxTotalRoyaltyShare = math.LegacyNewDecWithPrec(25, 2)    // 25% cap
	err := f.keeper.SetParams(ctx, params)
	require.NoError(t, err)

	gpContrib := types.NewContribution(300, grandparent, "code", "ipfs://gp3", []byte{200}, 10, 1000)
	err = f.keeper.SetContribution(ctx, gpContrib)
	require.NoError(t, err)

	parentContrib := types.NewContribution(301, parent, "code", "ipfs://p3", []byte{201}, 10, 1000)
	parentContrib.ParentClaimId = 300
	err = f.keeper.SetContribution(ctx, parentContrib)
	require.NoError(t, err)

	input := types.RewardContext{
		ClaimID:         32,
		Contributor:     contributor,
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(80, 2), // 0.7x
		ParentClaimID:   301,
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)

	// Gross = 700. Max royalty = 25% of 700 = 175
	// Parent wants 20% = 140 (under cap)
	// Grandparent wants 15% = 105, but total would be 245 > 175
	// So grandparent gets capped to 175 - 140 = 35
	require.True(t, output.TotalRoyaltyPaid.LTE(math.NewInt(175)),
		"total royalty %s should not exceed 175", output.TotalRoyaltyPaid)
}

func TestRewardLogic_Royalty_NoRoyaltyForOriginal(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	// Original content (multiplier = 1.0 or higher) → no royalty routing
	input := types.RewardContext{
		ClaimID:         33,
		Contributor:     sdk.AccAddress("royalty4____________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(60, 2), // 1.0x (original range)
		ParentClaimID:   100,                               // even with parent set
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	require.Len(t, output.RoyaltyRoutes, 0, "original content should not pay royalties")
	require.True(t, output.TotalRoyaltyPaid.IsZero())
}

// ============================================================================
// Vesting Split
// ============================================================================

func TestRewardLogic_VestingSplit(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	input := types.RewardContext{
		ClaimID:         40,
		Contributor:     sdk.AccAddress("vestsplit___________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(60, 2), // 1.0x
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)

	// gross = 1000, immediate = 20% = 200, vested = 800
	require.True(t, output.ImmediateAmount.Equal(math.NewInt(200)),
		"expected immediate 200, got %s", output.ImmediateAmount)
	require.True(t, output.VestedAmount.Equal(math.NewInt(800)),
		"expected vested 800, got %s", output.VestedAmount)
}

// ============================================================================
// Override Logic
// ============================================================================

func TestRewardLogic_DuplicateZeroReward(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	input := types.RewardContext{
		ClaimID:         50,
		Contributor:     sdk.AccAddress("duptest_____________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		IsDuplicate:     true,
		SimilarityScore: math.LegacyNewDecWithPrec(99, 2),
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	require.True(t, output.FinalRewardAmount.IsZero(), "duplicates should get zero reward")
	require.True(t, output.OriginalityMultiplier.IsZero())
}

func TestRewardLogic_ReviewOverride_FalsePositive(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	input := types.RewardContext{
		ClaimID:         51,
		Contributor:     sdk.AccAddress("overridetest________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(95, 2), // Very high similarity
		ReviewOverride:  types.Override_DERIVATIVE_FALSE_POSITIVE,
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	// Override should give 1.0x despite high similarity
	require.True(t, output.OriginalityMultiplier.Equal(math.LegacyOneDec()),
		"false positive override should give 1.0x, got %s", output.OriginalityMultiplier)
}

// ============================================================================
// RewardMult Integration (nil keeper fallback)
// ============================================================================

func TestRewardLogic_NilRewardMultKeeper(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	// No rewardmult keeper set — should still work with 1.0x multiplier
	input := types.RewardContext{
		ClaimID:         60,
		Contributor:     sdk.AccAddress("nilrmtest___________").String(),
		QualityScore:    math.LegacyNewDec(10),
		BaseReward:      math.NewInt(1000),
		SimilarityScore: math.LegacyNewDecWithPrec(60, 2), // 1.0x
	}

	output, err := f.keeper.CalculateReward(ctx, input)
	require.NoError(t, err)
	// Should work normally without panicking
	require.True(t, output.FinalRewardAmount.IsPositive())
}
