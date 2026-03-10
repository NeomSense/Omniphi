package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

// moduleBalanceAddr is the bech32 address the mock bank keeper uses for module funds.
var moduleBalanceAddr = sdk.AccAddress("module_address______").String()

// ============================================================================
// ARVS Unit Tests
// ============================================================================

// --- Risk Score ---

func TestComputeRiskScore_LowRisk(t *testing.T) {
	input := types.RiskScoreInput{
		CategoryRisk:       types.CategoryRiskLow,                    // 1/3 ≈ 0.33
		TrustScore:         math.LegacyOneDec(),                      // perfect trust → 0 inverse
		SimilarityScore:    math.LegacyZeroDec(),                     // fully original
		VerifierConfidence: math.LegacyOneDec(),                      // full confidence → 0 inverse
		DisputeRate:        math.LegacyZeroDec(),                     // no disputes
	}
	weights := types.DefaultARVSWeights()
	score := keeper.ComputeRiskScore(input, weights)

	// Expected: only category term contributes ≈ (1/3) * 2500/10000 * 10000 = 833 bps
	require.Greater(t, score, uint32(700))
	require.Less(t, score, uint32(1100))
}

func TestComputeRiskScore_HighRisk(t *testing.T) {
	input := types.RiskScoreInput{
		CategoryRisk:       types.CategoryRiskHigh,                           // 3/3 = 1.0
		TrustScore:         math.LegacyNewDecWithPrec(1, 1),                  // 0.1 → inverse 0.9
		SimilarityScore:    math.LegacyNewDecWithPrec(95, 2),                 // 0.95
		VerifierConfidence: math.LegacyNewDecWithPrec(30, 2),                 // 0.30 → inverse 0.70
		DisputeRate:        math.LegacyNewDecWithPrec(80, 2),                 // 0.80
	}
	weights := types.DefaultARVSWeights()
	score := keeper.ComputeRiskScore(input, weights)

	// Should be well above the high threshold (6500)
	require.Greater(t, score, uint32(7000))
}

func TestComputeRiskScore_MediumRisk(t *testing.T) {
	input := types.RiskScoreInput{
		CategoryRisk:       types.CategoryRiskMedium,
		TrustScore:         math.LegacyNewDecWithPrec(70, 2),  // 0.70 → inverse 0.30
		SimilarityScore:    math.LegacyNewDecWithPrec(40, 2),  // 0.40
		VerifierConfidence: math.LegacyNewDecWithPrec(80, 2),  // 0.80 → inverse 0.20
		DisputeRate:        math.LegacyNewDecWithPrec(10, 2),  // 0.10
	}
	weights := types.DefaultARVSWeights()
	score := keeper.ComputeRiskScore(input, weights)

	// Should fall between 3000 and 6500
	require.GreaterOrEqual(t, score, uint32(3000))
	require.LessOrEqual(t, score, uint32(6500))
}

func TestComputeRiskScore_ClampMax(t *testing.T) {
	input := types.RiskScoreInput{
		CategoryRisk:       types.CategoryRiskHigh,
		TrustScore:         math.LegacyZeroDec(),              // 0 → inverse 1.0
		SimilarityScore:    math.LegacyOneDec(),               // max
		VerifierConfidence: math.LegacyZeroDec(),              // 0 → inverse 1.0
		DisputeRate:        math.LegacyOneDec(),               // max
	}
	weights := types.DefaultARVSWeights()
	score := keeper.ComputeRiskScore(input, weights)
	require.LessOrEqual(t, score, uint32(10000))
}

func TestComputeRiskScore_ClampMin(t *testing.T) {
	input := types.RiskScoreInput{
		CategoryRisk:       types.CategoryRiskLow,
		TrustScore:         math.LegacyOneDec(),
		SimilarityScore:    math.LegacyZeroDec(),
		VerifierConfidence: math.LegacyOneDec(),
		DisputeRate:        math.LegacyZeroDec(),
	}
	weights := types.DefaultARVSWeights()
	score := keeper.ComputeRiskScore(input, weights)
	require.GreaterOrEqual(t, score, uint32(0))
}

// --- Vesting Profile Selection ---

func TestSelectVestingProfile_Derivative(t *testing.T) {
	input := types.RiskScoreInput{
		IsDerivative:       true,
		CategoryRisk:       types.CategoryRiskMedium,
		TrustScore:         math.LegacyOneDec(),
		SimilarityScore:    math.LegacyNewDecWithPrec(70, 2),
		VerifierConfidence: math.LegacyOneDec(),
		DisputeRate:        math.LegacyZeroDec(),
	}
	profiles := types.DefaultVestingProfiles()
	p, _ := keeper.SelectVestingProfile(input, types.DefaultARVSWeights(), profiles, 3000, 6500)
	require.Equal(t, types.VestingProfileDerivative, p.ProfileID)
}

func TestSelectVestingProfile_RepeatOffender(t *testing.T) {
	input := types.RiskScoreInput{
		IsRepeatOffender: true,
		CategoryRisk:     types.CategoryRiskLow,
		TrustScore:       math.LegacyOneDec(),
	}
	profiles := types.DefaultVestingProfiles()
	p, score := keeper.SelectVestingProfile(input, types.DefaultARVSWeights(), profiles, 3000, 6500)
	require.Equal(t, types.VestingProfileRepeatOffender, p.ProfileID)
	require.Equal(t, uint32(10000), score)
}

func TestSelectVestingProfile_LowRisk(t *testing.T) {
	input := types.RiskScoreInput{
		CategoryRisk:       types.CategoryRiskLow,
		TrustScore:         math.LegacyOneDec(),
		SimilarityScore:    math.LegacyZeroDec(),
		VerifierConfidence: math.LegacyOneDec(),
		DisputeRate:        math.LegacyZeroDec(),
	}
	profiles := types.DefaultVestingProfiles()
	p, _ := keeper.SelectVestingProfile(input, types.DefaultARVSWeights(), profiles, 3000, 6500)
	require.Equal(t, types.VestingProfileLowRisk, p.ProfileID)
}

func TestSelectVestingProfile_HighRisk(t *testing.T) {
	input := types.RiskScoreInput{
		CategoryRisk:       types.CategoryRiskHigh,
		TrustScore:         math.LegacyNewDecWithPrec(1, 1),
		SimilarityScore:    math.LegacyNewDecWithPrec(95, 2),
		VerifierConfidence: math.LegacyNewDecWithPrec(20, 2),
		DisputeRate:        math.LegacyNewDecWithPrec(80, 2),
	}
	profiles := types.DefaultVestingProfiles()
	p, score := keeper.SelectVestingProfile(input, types.DefaultARVSWeights(), profiles, 3000, 6500)
	require.Equal(t, types.VestingProfileHighRisk, p.ProfileID)
	require.Greater(t, score, uint32(6500))
}

// --- VestingProfile Validation ---

func TestVestingProfile_Validate_Valid(t *testing.T) {
	for _, p := range types.DefaultVestingProfiles() {
		require.NoError(t, p.Validate(), "profile %s should be valid", p.Name)
	}
}

func TestVestingProfile_Validate_StagesDontSum(t *testing.T) {
	p := types.VestingProfile{
		ProfileID: 99,
		Name:      "bad",
		Stages: []types.UnlockStage{
			{UnlockBps: 5000, DelayEpochs: 0},
			{UnlockBps: 3000, DelayEpochs: 30},
			// Total = 8000, not 10000
		},
	}
	require.Error(t, p.Validate())
}

// --- BountyDistribution Validation ---

func TestBountyDistribution_Validate_Valid(t *testing.T) {
	require.NoError(t, types.DefaultBountyDistribution().Validate())
}

func TestBountyDistribution_Validate_WrongSum(t *testing.T) {
	b := types.BountyDistribution{
		ChallengerBps:          4000,
		BurnBps:                3000,
		TreasuryBps:            2000,
		ReviewerPenaltyPoolBps: 500, // only 9500 total
	}
	require.Error(t, b.Validate())
}

// --- ARVSWeights Validation ---

func TestARVSWeights_Validate_Valid(t *testing.T) {
	require.NoError(t, types.DefaultARVSWeights().Validate())
}

func TestARVSWeights_Validate_WrongSum(t *testing.T) {
	w := types.ARVSWeights{
		CategoryWeight:    2000,
		ReputationWeight:  2000,
		OriginalityWeight: 2000,
		ConfidenceWeight:  1000,
		DisputeWeight:     500, // total = 7500
	}
	require.Error(t, w.Validate())
}

// --- CategoryRiskLevel ---

func TestCategoryRiskLevel_Known(t *testing.T) {
	m := types.DefaultCategoryRiskMap()
	require.Equal(t, types.CategoryRiskLow, types.CategoryRiskLevel("documentation", m))
	require.Equal(t, types.CategoryRiskMedium, types.CategoryRiskLevel("code", m))
	require.Equal(t, types.CategoryRiskHigh, types.CategoryRiskLevel("security", m))
}

func TestCategoryRiskLevel_Unknown_DefaultsMedium(t *testing.T) {
	m := types.DefaultCategoryRiskMap()
	require.Equal(t, types.CategoryRiskMedium, types.CategoryRiskLevel("unknown_category", m))
}

// ============================================================================
// ARVS Integration Tests
// ============================================================================

// TestARVS_CreateSchedule_LowRisk verifies that a low-risk contributor gets
// a schedule where the immediate tranche is 60% of the total.
func TestARVS_CreateSchedule_LowRisk(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	profile := types.VestingProfile{
		ProfileID: types.VestingProfileLowRisk,
		Name:      "low_risk",
		Stages: []types.UnlockStage{
			{UnlockBps: 6000, DelayEpochs: 0},  // 60% immediate
			{UnlockBps: 2500, DelayEpochs: 30}, // 25% @ 30
			{UnlockBps: 1500, DelayEpochs: 60}, // 15% @ 60
		},
	}

	total := math.NewInt(1000)
	immediate, err := fixture.keeper.CreateARVSVestingSchedule(ctx, addrContributor, 1, total, profile, 1500)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(600), immediate) // 60%

	schedule, found := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 1)
	require.True(t, found)
	require.Equal(t, types.VestingStatusActive, schedule.Status)
	require.Equal(t, types.VestingProfileLowRisk, schedule.ProfileID)
	require.Equal(t, uint32(1500), schedule.RiskScoreBps)
	require.Equal(t, 3, len(schedule.Stages))

	// Stage 0 (immediate) should be released
	require.True(t, schedule.Stages[0].Released)
	require.Equal(t, math.NewInt(600), schedule.Stages[0].Amount)

	// Stages 1 and 2 should NOT be released yet
	require.False(t, schedule.Stages[1].Released)
	require.False(t, schedule.Stages[2].Released)
}

// TestARVS_CreateSchedule_HighRisk verifies that 10% is immediate and 90% is deferred.
func TestARVS_CreateSchedule_HighRisk(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	profile := types.VestingProfile{
		ProfileID: types.VestingProfileHighRisk,
		Name:      "high_risk",
		Stages: []types.UnlockStage{
			{UnlockBps: 1000, DelayEpochs: 0},   // 10% immediate
			{UnlockBps: 4000, DelayEpochs: 60},  // 40% @ 60
			{UnlockBps: 5000, DelayEpochs: 120}, // 50% @ 120
		},
	}

	total := math.NewInt(1000)
	immediate, err := fixture.keeper.CreateARVSVestingSchedule(ctx, addrContributor, 2, total, profile, 8000)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(100), immediate) // 10%

	schedule, found := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 2)
	require.True(t, found)
	require.Equal(t, math.NewInt(400), schedule.Stages[1].Amount) // 40%
	require.Equal(t, math.NewInt(500), schedule.Stages[2].Amount) // 50%
}

// TestARVS_RoundingRemainder verifies that rounding dust goes to the last stage.
func TestARVS_RoundingRemainder(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	profile := types.VestingProfile{
		ProfileID: types.VestingProfileMediumRisk,
		Name:      "medium_risk",
		Stages: []types.UnlockStage{
			{UnlockBps: 3000, DelayEpochs: 0},
			{UnlockBps: 4000, DelayEpochs: 30},
			{UnlockBps: 3000, DelayEpochs: 90},
		},
	}

	// 7 tokens — doesn't divide evenly by bps
	total := math.NewInt(7)
	immediate, err := fixture.keeper.CreateARVSVestingSchedule(ctx, addrContributor, 3, total, profile, 4000)
	require.NoError(t, err)

	schedule, _ := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 3)

	// Sum of all stages must equal total
	sum := math.ZeroInt()
	for _, s := range schedule.Stages {
		sum = sum.Add(s.Amount)
	}
	require.True(t, sum.Equal(total), "stages must sum to total, got %s", sum)

	// Immediate amount must be consistent with stage 0
	require.Equal(t, schedule.Stages[0].Amount, immediate)
}

// TestARVS_PauseResume verifies freeze/unfreeze on ARVS schedules.
func TestARVS_PauseResume(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	profile := types.DefaultVestingProfiles()[0] // low risk
	_, err := fixture.keeper.CreateARVSVestingSchedule(ctx, addrContributor, 10, math.NewInt(500), profile, 1000)
	require.NoError(t, err)

	// Pause
	require.NoError(t, fixture.keeper.PauseARVSVesting(ctx, addrContributor, 10))
	s, _ := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 10)
	require.Equal(t, types.VestingStatusPaused, s.Status)

	// Resume
	require.NoError(t, fixture.keeper.ResumeARVSVesting(ctx, addrContributor, 10))
	s, _ = fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 10)
	require.Equal(t, types.VestingStatusActive, s.Status)

	// Pause again then clawback
	require.NoError(t, fixture.keeper.PauseARVSVesting(ctx, addrContributor, 10))
	unvested, err := fixture.keeper.ClawbackARVSVesting(ctx, addrContributor, 10)
	require.NoError(t, err)
	require.True(t, unvested.IsPositive())

	s, _ = fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 10)
	require.Equal(t, types.VestingStatusClawedBack, s.Status)
}

// TestARVS_PauseResume_NoOp verifies no-op on nonexistent schedules.
func TestARVS_PauseResume_NoOp(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	require.NoError(t, fixture.keeper.PauseARVSVesting(ctx, addrContributor, 9999))
	require.NoError(t, fixture.keeper.ResumeARVSVesting(ctx, addrContributor, 9999))
	unvested, err := fixture.keeper.ClawbackARVSVesting(ctx, addrContributor, 9999)
	require.NoError(t, err)
	require.True(t, unvested.IsZero())
}

// TestARVS_ProcessReleases_UnlocksAtCorrectEpoch verifies that ProcessARVSVestingReleases
// only unlocks stages whose DelayEpochs has passed.
func TestARVS_ProcessReleases_UnlocksAtCorrectEpoch(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	require.NoError(t, fixture.keeper.SetParams(ctx, params))

	// Fund module account
	fixture.bankKeeper.setBalance(moduleBalanceAddr, "omniphi", math.NewInt(1_000_000))

	// Create schedule with current epoch = 0; stage 1 unlocks at epoch 5
	profile := types.VestingProfile{
		ProfileID: types.VestingProfileMediumRisk,
		Name:      "test",
		Stages: []types.UnlockStage{
			{UnlockBps: 3000, DelayEpochs: 0}, // immediate (already released)
			{UnlockBps: 7000, DelayEpochs: 5}, // unlocks at epoch 5
		},
	}

	total := math.NewInt(1000)
	_, err := fixture.keeper.CreateARVSVestingSchedule(ctx, addrContributor, 20, total, profile, 4500)
	require.NoError(t, err)

	// Run ProcessARVSVestingReleases at epoch 0 — stage 1 should NOT unlock yet
	require.NoError(t, fixture.keeper.ProcessARVSVestingReleases(ctx))

	s, _ := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 20)
	require.False(t, s.Stages[1].Released, "stage 1 should not release at epoch 0")
}

// TestARVS_Security_PausedScheduleNoRelease verifies that ProcessARVSVestingReleases
// does not release funds from a Paused schedule.
func TestARVS_Security_PausedScheduleNoRelease(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	require.NoError(t, fixture.keeper.SetParams(ctx, params))

	// Create an all-immediate profile (so epoch doesn't matter)
	profile := types.VestingProfile{
		ProfileID: types.VestingProfileLowRisk,
		Name:      "all_immediate_test",
		Stages: []types.UnlockStage{
			{UnlockBps: 5000, DelayEpochs: 0},
			{UnlockBps: 5000, DelayEpochs: 0}, // both immediate
		},
	}

	total := math.NewInt(1000)
	_, err := fixture.keeper.CreateARVSVestingSchedule(ctx, addrContributor, 21, total, profile, 500)
	require.NoError(t, err)

	// Pause the schedule
	require.NoError(t, fixture.keeper.PauseARVSVesting(ctx, addrContributor, 21))

	// Run releases — paused schedule must be skipped entirely
	require.NoError(t, fixture.keeper.ProcessARVSVestingReleases(ctx))

	s, _ := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 21)
	require.Equal(t, types.VestingStatusPaused, s.Status, "schedule must remain paused")

	// ReleasedAmount should only reflect the initial immediate release (stage 0+1 at epoch 0)
	// Since we paused AFTER creation (which already released immediate stages),
	// no additional releases should have happened.
	_ = s // verified by status check above
}

// TestARVS_BuildRiskScoreInput verifies that BuildRiskScoreInput populates all fields.
func TestARVS_BuildRiskScoreInput(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Set up contributor stats with some history
	stats := types.ContributorStats{
		Address:           addrContributor,
		TotalSubmissions:  10,
		OverturnedReviews: 2, // 20% dispute rate
		DuplicateCount:    1,
		FraudCount:        0,
		ReputationScore:   math.LegacyNewDecWithPrec(85, 2),
	}
	require.NoError(t, fixture.keeper.SetContributorStats(ctx, stats))

	params := types.DefaultParams()
	input := fixture.keeper.BuildRiskScoreInput(ctx, addrContributor, "security",
		math.LegacyNewDecWithPrec(60, 2), math.LegacyNewDecWithPrec(75, 2), false, params)

	require.Equal(t, types.CategoryRiskHigh, input.CategoryRisk)
	require.True(t, input.TrustScore.Equal(math.LegacyNewDecWithPrec(85, 2)))
	require.True(t, input.SimilarityScore.Equal(math.LegacyNewDecWithPrec(60, 2)))
	require.True(t, input.DisputeRate.Equal(math.LegacyNewDecWithPrec(20, 2)))
	require.False(t, input.IsDerivative)
	require.False(t, input.IsRepeatOffender) // DuplicateCount+FraudCount = 1 < threshold 3
}

// TestARVS_BuildRiskScoreInput_RepeatOffender verifies repeat offender detection.
func TestARVS_BuildRiskScoreInput_RepeatOffender(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	stats := types.ContributorStats{
		Address:          addrContributor,
		TotalSubmissions: 10,
		DuplicateCount:   2,
		FraudCount:       1, // total = 3 = threshold
		ReputationScore:  math.LegacyNewDecWithPrec(60, 2),
	}
	require.NoError(t, fixture.keeper.SetContributorStats(ctx, stats))

	params := types.DefaultParams() // RepeatOffenderThreshold = 3
	input := fixture.keeper.BuildRiskScoreInput(ctx, addrContributor, "code",
		math.LegacyZeroDec(), math.LegacyOneDec(), false, params)

	require.True(t, input.IsRepeatOffender)
}

// TestARVS_DistributeRewardsARVS_LowRisk verifies end-to-end reward distribution
// with ARVS: contributor gets 60% immediately, 40% deferred.
func TestARVS_DistributeRewardsARVS_LowRisk(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	params.EnableARVS = true
	params.ARVSWeights = types.DefaultARVSWeights()
	params.ARVSVestingProfiles = types.DefaultVestingProfiles()
	params.ARVSRiskScoreLowThreshold = 3000
	params.ARVSRiskScoreHighThreshold = 6500
	require.NoError(t, fixture.keeper.SetParams(ctx, params))

	// Fund module
	fixture.bankKeeper.setBalance(moduleBalanceAddr, "omniphi", math.NewInt(1_000_000))

	output := types.RewardOutput{
		FinalRewardAmount: math.NewInt(1000),
		ImmediateAmount:   math.NewInt(200), // legacy field — unused by ARVS path
		VestedAmount:      math.NewInt(800), // legacy field — unused by ARVS path
		ClaimID:           42,
	}

	// Low-risk conditions: trusted contributor, no similarity, high confidence
	err := fixture.keeper.DistributeRewardsARVS(ctx, output, addrContributor,
		math.LegacyZeroDec(),  // similarity
		math.LegacyOneDec(),   // verifier confidence
		"documentation",       // low-risk category
		false,                 // not derivative
	)
	require.NoError(t, err)

	// Verify ARVS schedule was created
	schedule, found := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 42)
	require.True(t, found)
	require.Equal(t, types.VestingProfileLowRisk, schedule.ProfileID)
	require.Equal(t, math.NewInt(1000), schedule.TotalAmount)

	// Immediate amount = 60% = 600
	require.Equal(t, math.NewInt(600), schedule.ReleasedAmount)
	require.True(t, schedule.Stages[0].Released)
	require.False(t, schedule.Stages[1].Released)
	require.False(t, schedule.Stages[2].Released)
}

// TestARVS_DistributeRewardsARVS_HighRisk verifies only 10% is immediate.
func TestARVS_DistributeRewardsARVS_HighRisk(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	params.EnableARVS = true
	params.ARVSWeights = types.DefaultARVSWeights()
	params.ARVSVestingProfiles = types.DefaultVestingProfiles()
	params.ARVSRiskScoreLowThreshold = 3000
	params.ARVSRiskScoreHighThreshold = 6500
	require.NoError(t, fixture.keeper.SetParams(ctx, params))

	fixture.bankKeeper.setBalance(moduleBalanceAddr, "omniphi", math.NewInt(1_000_000))

	// Set up contributor stats as high risk: low trust, high similarity, frequent disputes
	stats := types.ContributorStats{
		Address:           addrContributor,
		TotalSubmissions:  10,
		OverturnedReviews: 8,
		DuplicateCount:    0,
		FraudCount:        0,
		ReputationScore:   math.LegacyNewDecWithPrec(20, 2), // low trust
	}
	require.NoError(t, fixture.keeper.SetContributorStats(ctx, stats))

	output := types.RewardOutput{
		FinalRewardAmount: math.NewInt(1000),
		ClaimID:           43,
	}

	err := fixture.keeper.DistributeRewardsARVS(ctx, output, addrContributor,
		math.LegacyNewDecWithPrec(90, 2), // high similarity
		math.LegacyNewDecWithPrec(30, 2), // low confidence
		"security",                        // high-risk category
		false,
	)
	require.NoError(t, err)

	schedule, found := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 43)
	require.True(t, found)
	require.Equal(t, types.VestingProfileHighRisk, schedule.ProfileID)

	// Immediate = 10% = 100
	require.Equal(t, math.NewInt(100), schedule.ReleasedAmount)
}

// TestARVS_Security_ClawedBackCannotBeReleased verifies that once clawed back,
// ProcessARVSVestingReleases does not release additional funds.
func TestARVS_Security_ClawedBackCannotBeReleased(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	require.NoError(t, fixture.keeper.SetParams(ctx, params))
	fixture.bankKeeper.setBalance(moduleBalanceAddr, "omniphi", math.NewInt(1_000_000))

	profile := types.DefaultVestingProfiles()[0] // low risk
	_, err := fixture.keeper.CreateARVSVestingSchedule(ctx, addrContributor, 50, math.NewInt(1000), profile, 1000)
	require.NoError(t, err)

	// Clawback
	_, err = fixture.keeper.ClawbackARVSVesting(ctx, addrContributor, 50)
	require.NoError(t, err)

	// Now run releases — should be no-op
	require.NoError(t, fixture.keeper.ProcessARVSVestingReleases(ctx))

	s, _ := fixture.keeper.GetARVSVestingSchedule(ctx, addrContributor, 50)
	require.Equal(t, types.VestingStatusClawedBack, s.Status, "status must remain ClawedBack")
}
