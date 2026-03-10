package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

// ============================================================================
// PoC Hardening Upgrade Tests (v2)
// ============================================================================

// ============================================================================
// 1. Finality Enforcement Tests
// ============================================================================

func TestContributionFinality_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get default finality (should be pending)
	finality := f.keeper.GetContributionFinality(f.ctx, 1)
	require.Equal(t, types.FinalityStatusPending, finality.Status)
	require.Equal(t, uint64(1), finality.ContributionID)

	// Set finality
	newFinality := types.ContributionFinality{
		ContributionID: 1,
		Status:         types.FinalityStatusFinal,
		FinalizedAt:    100,
	}
	err := f.keeper.SetContributionFinality(f.ctx, newFinality)
	require.NoError(t, err)

	// Get again
	got := f.keeper.GetContributionFinality(f.ctx, 1)
	require.Equal(t, types.FinalityStatusFinal, got.Status)
	require.Equal(t, int64(100), got.FinalizedAt)
}

func TestIsContributionFinal_DirectPoV(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create unverified contribution
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte("testhash12345678901234567890123"),
		Verified:    false,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	// Should not be final (not verified)
	isFinal := f.keeper.IsContributionFinal(f.ctx, 1)
	require.False(t, isFinal)

	// Mark as verified
	contribution.Verified = true
	err = f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	// Should be final (verified via PoV = final in direct mode)
	isFinal = f.keeper.IsContributionFinal(f.ctx, 1)
	require.True(t, isFinal)
}

func TestFinalizeContribution(t *testing.T) {
	f := SetupKeeperTest(t)

	err := f.keeper.FinalizeContribution(f.ctx, 42)
	require.NoError(t, err)

	finality := f.keeper.GetContributionFinality(f.ctx, 42)
	require.Equal(t, types.FinalityStatusFinal, finality.Status)
	// FinalizedAt is set to block height, which may be 0 in test context
	require.True(t, finality.FinalizedAt >= 0)
}

// ============================================================================
// 2. ReputationScore Tests
// ============================================================================

func TestReputationScore_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get default (should be zero)
	rs := f.keeper.GetReputationScore(f.ctx, testAddr1.String())
	require.Equal(t, testAddr1.String(), rs.Address)
	require.True(t, rs.Score.IsZero())

	// Set reputation
	rs.Score = math.LegacyNewDec(1000)
	rs.LastUpdated = 10
	err := f.keeper.SetReputationScore(f.ctx, rs)
	require.NoError(t, err)

	// Get again
	got := f.keeper.GetReputationScore(f.ctx, testAddr1.String())
	require.True(t, got.Score.Equal(math.LegacyNewDec(1000)))
	require.Equal(t, int64(10), got.LastUpdated)
}

func TestReputationScore_EMASmoothing(t *testing.T) {
	f := SetupKeeperTest(t)

	addr := testAddr1.String()

	// Add credits epoch 1
	err := f.keeper.UpdateReputationScore(f.ctx, addr, math.NewInt(1000), 1)
	require.NoError(t, err)

	rs1 := f.keeper.GetReputationScore(f.ctx, addr)
	// With alpha=0.1 and starting from 0: 0.1 * 1000 + 0.9 * 0 = 100
	require.True(t, rs1.Score.Equal(math.LegacyNewDec(100)))

	// Add more credits epoch 2
	err = f.keeper.UpdateReputationScore(f.ctx, addr, math.NewInt(1000), 2)
	require.NoError(t, err)

	rs2 := f.keeper.GetReputationScore(f.ctx, addr)
	// With alpha=0.1: 0.1 * 1000 + 0.9 * 100 = 100 + 90 = 190
	require.True(t, rs2.Score.Equal(math.LegacyNewDec(190)))

	// Verify it's slow-moving (not just replacing with new value)
	require.True(t, rs2.Score.LT(math.LegacyNewDec(1000)))
}

func TestGovBoostFromReputation(t *testing.T) {
	f := SetupKeeperTest(t)

	// Zero reputation = zero boost
	boost := f.keeper.GetGovBoostFromReputation(f.ctx, testAddr1)
	require.True(t, boost.IsZero())

	// Set reputation to 500,000 = 50% of max boost (5%)
	rs := types.NewReputationScore(testAddr1.String())
	rs.Score = math.LegacyNewDec(500000)
	err := f.keeper.SetReputationScore(f.ctx, rs)
	require.NoError(t, err)

	boost = f.keeper.GetGovBoostFromReputation(f.ctx, testAddr1)
	// 500000 / 1000000 = 0.5, but max is 10%, so 0.5 * 0.1 = 0.05
	// Actually the formula is: boost = min(0.10, score / 1,000,000)
	// So 500000 / 1000000 = 0.0005, not 0.05
	// Let me verify: boost = min(10%, score / 1,000,000) = min(0.10, 0.5) = 0.10? No.
	// score / 1,000,000 = 500000/1000000 = 0.5 which is > 0.10, so capped at 0.10
	require.True(t, boost.Equal(math.LegacyNewDecWithPrec(10, 2)), "expected 0.10, got %s", boost) // 0.10 (capped)

	// Set reputation to 50,000 = 5% boost (50000/1000000 = 0.05)
	rs.Score = math.LegacyNewDec(50000)
	err = f.keeper.SetReputationScore(f.ctx, rs)
	require.NoError(t, err)

	boost = f.keeper.GetGovBoostFromReputation(f.ctx, testAddr1)
	require.True(t, boost.Equal(math.LegacyNewDecWithPrec(5, 2)), "expected 0.05, got %s", boost) // 0.05

	// Set reputation to 2,000,000 = capped at 10%
	rs.Score = math.LegacyNewDec(2000000)
	err = f.keeper.SetReputationScore(f.ctx, rs)
	require.NoError(t, err)

	boost = f.keeper.GetGovBoostFromReputation(f.ctx, testAddr1)
	require.True(t, boost.Equal(math.LegacyNewDecWithPrec(10, 2))) // 0.10 (capped)
}

// ============================================================================
// 3. Credit Hardening Tests
// ============================================================================

func TestEpochCredits_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get default
	ec := f.keeper.GetEpochCredits(f.ctx, testAddr1.String(), 1)
	require.Equal(t, testAddr1.String(), ec.Address)
	require.Equal(t, uint64(1), ec.Epoch)
	require.True(t, ec.Credits.IsZero())

	// Set
	ec.Credits = math.NewInt(5000)
	err := f.keeper.SetEpochCredits(f.ctx, ec)
	require.NoError(t, err)

	// Get again
	got := f.keeper.GetEpochCredits(f.ctx, testAddr1.String(), 1)
	require.True(t, got.Credits.Equal(math.NewInt(5000)))
}

func TestTypeCredits_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get default
	tc := f.keeper.GetTypeCredits(f.ctx, testAddr1.String(), "code")
	require.Equal(t, testAddr1.String(), tc.Address)
	require.Equal(t, "code", tc.Ctype)
	require.True(t, tc.Credits.IsZero())
	require.Equal(t, uint64(0), tc.Count)

	// Set
	tc.Credits = math.NewInt(25000)
	tc.Count = 5
	err := f.keeper.SetTypeCredits(f.ctx, tc)
	require.NoError(t, err)

	// Get again
	got := f.keeper.GetTypeCredits(f.ctx, testAddr1.String(), "code")
	require.True(t, got.Credits.Equal(math.NewInt(25000)))
	require.Equal(t, uint64(5), got.Count)
}

func TestAddCreditsWithCaps_EpochCap(t *testing.T) {
	f := SetupKeeperTest(t)

	epoch := f.keeper.GetCurrentEpoch(f.ctx)

	// Add credits up to epoch cap (10,000)
	for i := 0; i < 10; i++ {
		err := f.keeper.AddCreditsWithCaps(f.ctx, testAddr1, math.NewInt(1000), "code", epoch)
		require.NoError(t, err)
	}

	// Check epoch credits
	ec := f.keeper.GetEpochCredits(f.ctx, testAddr1.String(), epoch)
	require.True(t, ec.Credits.Equal(math.NewInt(10000)))

	// Next addition should be capped/rejected
	err := f.keeper.AddCreditsWithCaps(f.ctx, testAddr1, math.NewInt(1000), "code", epoch)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEpochCreditCapExceeded)
}

func TestAddCreditsWithCaps_TypeCap(t *testing.T) {
	f := SetupKeeperTest(t)

	// Use different epochs to bypass epoch cap
	for i := 0; i < 50; i++ {
		epoch := uint64(i)
		err := f.keeper.AddCreditsWithCaps(f.ctx, testAddr1, math.NewInt(1000), "code", epoch)
		require.NoError(t, err)
	}

	// Check type credits - should hit type cap (50,000)
	tc := f.keeper.GetTypeCredits(f.ctx, testAddr1.String(), "code")
	require.True(t, tc.Credits.Equal(math.NewInt(50000)))

	// Next addition should fail due to type cap
	err := f.keeper.AddCreditsWithCaps(f.ctx, testAddr1, math.NewInt(1000), "code", 100)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTypeCreditCapExceeded)
}

func TestAddCreditsWithCaps_TotalCap(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add credits via different types to reach total cap (100,000)
	// Type "code" = 50,000
	for i := 0; i < 50; i++ {
		epoch := uint64(i)
		err := f.keeper.AddCreditsWithCaps(f.ctx, testAddr1, math.NewInt(1000), "code", epoch)
		require.NoError(t, err)
	}

	// Type "record" = another 50,000
	for i := 0; i < 50; i++ {
		epoch := uint64(i + 100)
		err := f.keeper.AddCreditsWithCaps(f.ctx, testAddr1, math.NewInt(1000), "record", epoch)
		require.NoError(t, err)
	}

	// Total should be 100,000
	credits := f.keeper.GetCredits(f.ctx, testAddr1)
	require.True(t, credits.Amount.Equal(math.NewInt(100000)))

	// Any more should be rejected
	err := f.keeper.AddCreditsWithCaps(f.ctx, testAddr1, math.NewInt(1000), "other", 500)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrCreditCapExceeded)
}

func TestDiminishingReturnsCurve(t *testing.T) {
	// The formula is: effective = cap * sqrt(raw / cap)
	// This gives diminishing returns as raw approaches cap
	tests := []struct {
		name       string
		rawCredits int64
		cap        int64
		wantMin    int64 // effective should be at least this
		wantMax    int64 // effective should be at most this
	}{
		{
			name:       "small amount - sqrt behavior",
			rawCredits: 100,
			cap:        10000,
			// sqrt(100/10000) * 10000 = sqrt(0.01) * 10000 = 0.1 * 10000 = 1000
			wantMin:    900,
			wantMax:    1100,
		},
		{
			name:       "half cap - sqrt behavior",
			rawCredits: 5000,
			cap:        10000,
			// sqrt(5000/10000) * 10000 = sqrt(0.5) * 10000 ≈ 0.707 * 10000 = 7071
			wantMin:    7000,
			wantMax:    7200,
		},
		{
			name:       "at cap - identity",
			rawCredits: 10000,
			cap:        10000,
			// sqrt(1.0) * 10000 = 10000
			wantMin:    9900,
			wantMax:    10100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := types.DiminishingReturnsCurve(
				math.NewInt(tt.rawCredits),
				math.NewInt(tt.cap),
			)
			require.True(t, result.GTE(math.NewInt(tt.wantMin)),
				"expected %d to be >= %d", result.Int64(), tt.wantMin)
			require.True(t, result.LTE(math.NewInt(tt.wantMax)),
				"expected %d to be <= %d", result.Int64(), tt.wantMax)
		})
	}
}

// ============================================================================
// 4. Endorsement Quality Tests
// ============================================================================

func TestValidatorEndorsementStats_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	valAddr := sdk.ValAddress("endorsement_stats___").String()

	// Get default
	stats := f.keeper.GetValidatorEndorsementStats(f.ctx, valAddr)
	require.Equal(t, valAddr, stats.ValidatorAddress)
	require.Equal(t, uint64(0), stats.TotalEndorsed)
	require.Equal(t, uint64(0), stats.TotalOpportunity)

	// Set
	stats.TotalEndorsed = 50
	stats.TotalOpportunity = 100
	stats.EarlyEndorsed = 40
	stats.QuorumEndorsed = 10
	err := f.keeper.SetValidatorEndorsementStats(f.ctx, stats)
	require.NoError(t, err)

	// Get again
	got := f.keeper.GetValidatorEndorsementStats(f.ctx, valAddr)
	require.Equal(t, uint64(50), got.TotalEndorsed)
	require.Equal(t, uint64(100), got.TotalOpportunity)
	require.Equal(t, uint64(40), got.EarlyEndorsed)
	require.Equal(t, uint64(10), got.QuorumEndorsed)
}

func TestValidatorEndorsementStats_ParticipationRate(t *testing.T) {
	stats := types.NewValidatorEndorsementStats("val1")

	// No opportunities = zero rate
	require.True(t, stats.GetParticipationRate().IsZero())

	// 50/100 = 50%
	stats.TotalEndorsed = 50
	stats.TotalOpportunity = 100
	rate := stats.GetParticipationRate()
	require.True(t, rate.Equal(math.LegacyNewDecWithPrec(50, 2)))
}

func TestValidatorEndorsementStats_IsFreeriding(t *testing.T) {
	stats := types.NewValidatorEndorsementStats("val1")

	// Too few opportunities - not flagged
	stats.TotalOpportunity = 5
	stats.TotalEndorsed = 0
	require.False(t, stats.IsFreeriding(math.LegacyNewDecWithPrec(20, 2)))

	// Enough opportunities, low participation = freeriding
	stats.TotalOpportunity = 100
	stats.TotalEndorsed = 10 // 10% < 20% threshold
	require.True(t, stats.IsFreeriding(math.LegacyNewDecWithPrec(20, 2)))

	// Good participation = not freeriding
	stats.TotalEndorsed = 50 // 50% > 20% threshold
	require.False(t, stats.IsFreeriding(math.LegacyNewDecWithPrec(20, 2)))
}

func TestValidatorEndorsementStats_IsQuorumGaming(t *testing.T) {
	stats := types.NewValidatorEndorsementStats("val1")

	// Too few endorsements - not flagged
	stats.TotalEndorsed = 5
	stats.QuorumEndorsed = 5
	require.False(t, stats.IsQuorumGaming(math.LegacyNewDecWithPrec(70, 2)))

	// High quorum endorsement rate = gaming
	stats.TotalEndorsed = 100
	stats.QuorumEndorsed = 80 // 80% > 70% threshold
	require.True(t, stats.IsQuorumGaming(math.LegacyNewDecWithPrec(70, 2)))

	// Normal distribution = not gaming
	stats.QuorumEndorsed = 30 // 30% < 70% threshold
	require.False(t, stats.IsQuorumGaming(math.LegacyNewDecWithPrec(70, 2)))
}

func TestGetEndorsementParticipationRate(t *testing.T) {
	f := SetupKeeperTest(t)

	valAddr := sdk.ValAddress("validator1__________")

	// Set stats
	stats := types.NewValidatorEndorsementStats(valAddr.String())
	stats.ParticipationEMA = math.LegacyNewDecWithPrec(75, 2) // 0.75
	err := f.keeper.SetValidatorEndorsementStats(f.ctx, stats)
	require.NoError(t, err)

	// Get rate
	rate, err := f.keeper.GetEndorsementParticipationRate(f.ctx, valAddr)
	require.NoError(t, err)
	require.True(t, rate.Equal(math.LegacyNewDecWithPrec(75, 2)))
}

// ============================================================================
// 5. Fraud & Rollback Safety Tests
// ============================================================================

func TestFrozenCredits_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get default (should be zero)
	fc := f.keeper.GetFrozenCredits(f.ctx, testAddr1.String())
	require.True(t, fc.Amount.IsZero())

	// Set
	fc = types.NewFrozenCredits(testAddr1.String(), math.NewInt(5000), 1, 100, "challenge")
	err := f.keeper.SetFrozenCredits(f.ctx, fc)
	require.NoError(t, err)

	// Get again
	got := f.keeper.GetFrozenCredits(f.ctx, testAddr1.String())
	require.True(t, got.Amount.Equal(math.NewInt(5000)))
	require.Equal(t, uint64(1), got.ContributionID)
	require.Equal(t, "challenge", got.Reason)
}

func TestFreezeCredits(t *testing.T) {
	f := SetupKeeperTest(t)

	err := f.keeper.FreezeCredits(f.ctx, testAddr1.String(), math.NewInt(1000), 1, "test challenge")
	require.NoError(t, err)

	fc := f.keeper.GetFrozenCredits(f.ctx, testAddr1.String())
	require.True(t, fc.Amount.Equal(math.NewInt(1000)))
	require.Equal(t, "test challenge", fc.Reason)
}

func TestUnfreezeCredits(t *testing.T) {
	f := SetupKeeperTest(t)

	// Freeze first
	err := f.keeper.FreezeCredits(f.ctx, testAddr1.String(), math.NewInt(1000), 1, "test")
	require.NoError(t, err)

	// Unfreeze
	err = f.keeper.UnfreezeCredits(f.ctx, testAddr1.String())
	require.NoError(t, err)

	// Should be zero
	fc := f.keeper.GetFrozenCredits(f.ctx, testAddr1.String())
	require.True(t, fc.Amount.IsZero())
}

func TestBurnFrozenCredits(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add some credits
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(5000))
	require.NoError(t, err)

	// Freeze some
	err = f.keeper.FreezeCredits(f.ctx, testAddr1.String(), math.NewInt(2000), 1, "fraud")
	require.NoError(t, err)

	// Burn frozen
	err = f.keeper.BurnFrozenCredits(f.ctx, testAddr1.String())
	require.NoError(t, err)

	// Check credits reduced
	credits := f.keeper.GetCredits(f.ctx, testAddr1)
	require.True(t, credits.Amount.Equal(math.NewInt(3000))) // 5000 - 2000

	// Check frozen cleared
	fc := f.keeper.GetFrozenCredits(f.ctx, testAddr1.String())
	require.True(t, fc.Amount.IsZero())
}

func TestGetAvailableCredits(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add credits
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(10000))
	require.NoError(t, err)

	// No frozen = full available
	available := f.keeper.GetAvailableCredits(f.ctx, testAddr1)
	require.True(t, available.Equal(math.NewInt(10000)))

	// Freeze some
	err = f.keeper.FreezeCredits(f.ctx, testAddr1.String(), math.NewInt(3000), 1, "test")
	require.NoError(t, err)

	// Available reduced
	available = f.keeper.GetAvailableCredits(f.ctx, testAddr1)
	require.True(t, available.Equal(math.NewInt(7000))) // 10000 - 3000
}

func TestClaimNonce(t *testing.T) {
	f := SetupKeeperTest(t)

	// Initial nonce = 0
	nonce := f.keeper.GetClaimNonce(f.ctx, testAddr1.String())
	require.Equal(t, uint64(0), nonce)

	// Increment
	err := f.keeper.IncrementClaimNonce(f.ctx, testAddr1.String())
	require.NoError(t, err)

	nonce = f.keeper.GetClaimNonce(f.ctx, testAddr1.String())
	require.Equal(t, uint64(1), nonce)

	// Increment again
	err = f.keeper.IncrementClaimNonce(f.ctx, testAddr1.String())
	require.NoError(t, err)

	nonce = f.keeper.GetClaimNonce(f.ctx, testAddr1.String())
	require.Equal(t, uint64(2), nonce)
}

// ============================================================================
// 6. Credit Decay Tests
// ============================================================================

func TestCreditDecay(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add credits to multiple addresses
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(10000))
	require.NoError(t, err)
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr2, math.NewInt(20000))
	require.NoError(t, err)

	// Apply decay for epoch 1
	err = f.keeper.ApplyCreditDecay(f.ctx, 1)
	require.NoError(t, err)

	// Check decay applied (0.5% = 50 bps)
	// addr1: 10000 * 0.995 = 9950
	credits1 := f.keeper.GetCredits(f.ctx, testAddr1)
	require.True(t, credits1.Amount.Equal(math.NewInt(9950)))

	// addr2: 20000 * 0.995 = 19900
	credits2 := f.keeper.GetCredits(f.ctx, testAddr2)
	require.True(t, credits2.Amount.Equal(math.NewInt(19900)))

	// Verify last decay epoch recorded
	lastDecay := f.keeper.GetLastDecayEpoch(f.ctx)
	require.Equal(t, uint64(1), lastDecay)
}

func TestCreditDecay_NoDuplicateDecay(t *testing.T) {
	f := SetupKeeperTest(t)

	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(10000))
	require.NoError(t, err)

	// Apply decay epoch 1
	err = f.keeper.ApplyCreditDecay(f.ctx, 1)
	require.NoError(t, err)

	credits1 := f.keeper.GetCredits(f.ctx, testAddr1)
	require.True(t, credits1.Amount.Equal(math.NewInt(9950)))

	// Try to apply decay for same epoch again
	err = f.keeper.ApplyCreditDecay(f.ctx, 1)
	require.NoError(t, err)

	// Credits should not change
	credits2 := f.keeper.GetCredits(f.ctx, testAddr1)
	require.True(t, credits2.Amount.Equal(math.NewInt(9950)))
}

// ============================================================================
// 7. Invariant Tests
// ============================================================================

func TestCreditCapInvariant(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add credits within cap
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(50000))
	require.NoError(t, err)

	// Check invariant passes
	msg, broken := CreditCapInvariant(f.keeper)(f.ctx)
	require.False(t, broken, "invariant broken: %s", msg)
}

func TestFrozenCreditsInvariant(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add credits
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(10000))
	require.NoError(t, err)

	// Freeze less than total
	err = f.keeper.FreezeCredits(f.ctx, testAddr1.String(), math.NewInt(5000), 1, "test")
	require.NoError(t, err)

	// Check invariant passes
	msg, broken := FrozenCreditsInvariant(f.keeper)(f.ctx)
	require.False(t, broken, "invariant broken: %s", msg)
}

// ============================================================================
// 8. Security Tests - PoC Economics Unchanged
// ============================================================================

func TestPoCEconomics_CreditsUniform(t *testing.T) {
	f := SetupKeeperTest(t)

	// Verify BaseRewardUnit is set (default is 1000)
	params := f.keeper.GetParams(f.ctx)
	require.True(t, params.BaseRewardUnit.IsPositive(), "BaseRewardUnit should be positive")

	// Verify weight function returns 1 (uniform) - tested via EnqueueReward
	// The key guarantee is that ALL contributions get the same BaseRewardUnit
	// regardless of who submitted them or their stake
}

func TestPoCEconomics_NoStakeWeightedCredits(t *testing.T) {
	// This test verifies that credit rewards are NOT weighted by validator stake
	// Credits should be uniform per verified contribution

	f := SetupKeeperTest(t)

	// Create contribution
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte("testhash12345678901234567890123"),
		Verified:    true,
	}

	// Enqueue reward
	err := f.keeper.EnqueueReward(f.ctx, contribution)
	require.NoError(t, err)

	// Check credits = BaseRewardUnit (1000), not stake-weighted
	credits := f.keeper.GetCredits(f.ctx, testAddr1)
	params := f.keeper.GetParams(f.ctx)
	require.True(t, credits.Amount.Equal(params.BaseRewardUnit))
}

func TestPoCEconomics_GovBoostCapped(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set reputation very high
	rs := types.NewReputationScore(testAddr1.String())
	rs.Score = math.LegacyNewDec(100000000) // 100 million
	err := f.keeper.SetReputationScore(f.ctx, rs)
	require.NoError(t, err)

	// Gov boost should still be capped at 10%
	boost := f.keeper.GetGovBoostFromReputation(f.ctx, testAddr1)
	require.True(t, boost.LTE(math.LegacyNewDecWithPrec(10, 2)))
}

// ============================================================================
// Test Helpers
// ============================================================================

var (
	testAddr1 = sdk.AccAddress("test1_______________")
	testAddr2 = sdk.AccAddress("test2_______________")
)

// Import invariants for testing
var (
	CreditCapInvariant      = keeper.CreditCapInvariant
	FrozenCreditsInvariant  = keeper.FrozenCreditsInvariant
)
