package comprehensive

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// ========================================
// TC-034 to TC-049: PoC Merit Engine Tests
// ========================================

// TC-034: Contribution Submission
// Priority: P0
// Purpose: Verify contributions can be submitted and are recorded correctly
func TestTC034_ContributionSubmission(t *testing.T) {
	tc := SetupTestContext(t)

	// Contributor account
	contributor := sdk.AccAddress("contributor_______")
	tc.BankKeeper.SetBalance(tc.Ctx, contributor, sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1000000))))

	// Submit contribution
	contributionID := "contrib-001"
	contributionData := "QmHash123456789"

	// Record contribution in mock state
	tc.PoCKeeper.SubmitContribution(tc.Ctx, contributionID, contributor, contributionData)

	// Verify contribution recorded
	contrib := tc.PoCKeeper.GetContribution(tc.Ctx, contributionID)
	require.NotNil(t, contrib, "Contribution should be recorded")
	require.Equal(t, "pending", contrib.Status, "Status should be pending")
	require.Equal(t, contributor.String(), contrib.Contributor, "Contributor should match")
}

// TC-035: Endorsement - Mixed Votes (67% Yes)
// Priority: P0
// Purpose: Verify contribution verified when ≥66.7% endorsements
func TestTC035_EndorsementMixedVotes(t *testing.T) {
	tc := SetupTestContext(t)

	contributionID := "contrib-002"
	contributor := sdk.AccAddress("contributor_______")

	// Submit contribution
	tc.PoCKeeper.SubmitContribution(tc.Ctx, contributionID, contributor, "QmHash")

	// 5 validators endorse: 3 yes, 2 no (60% - should fail to reach threshold)
	// But let's test 4 yes, 1 no (80% - should pass)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val1____________"), true)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val2____________"), true)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val3____________"), true)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val4____________"), true)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val5____________"), false)

	// Process endorsements
	tc.PoCKeeper.ProcessEndorsements(tc.Ctx, contributionID)

	// Verify contribution verified (4/5 = 80% > 66.7%)
	contrib := tc.PoCKeeper.GetContribution(tc.Ctx, contributionID)
	require.Equal(t, "verified", contrib.Status, "Contribution should be verified with 80% yes votes")
}

// TC-036: Endorsement - Rejection (40% Yes)
// Priority: P0
// Purpose: Verify contribution rejected when <66.7% endorsements
func TestTC036_EndorsementRejection(t *testing.T) {
	tc := SetupTestContext(t)

	contributionID := "contrib-003"
	contributor := sdk.AccAddress("contributor_______")

	// Submit contribution
	tc.PoCKeeper.SubmitContribution(tc.Ctx, contributionID, contributor, "QmHash")

	// 5 validators endorse: 2 yes, 3 no (40% - below threshold)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val1____________"), true)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val2____________"), true)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val3____________"), false)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val4____________"), false)
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, sdk.ValAddress("val5____________"), false)

	// Process endorsements
	tc.PoCKeeper.ProcessEndorsements(tc.Ctx, contributionID)

	// Verify contribution rejected (2/5 = 40% < 66.7%)
	contrib := tc.PoCKeeper.GetContribution(tc.Ctx, contributionID)
	require.Equal(t, "rejected", contrib.Status, "Contribution should be rejected with 40% yes votes")
}

// TC-037: Credit Minting
// Priority: P0
// Purpose: Verify credits minted for verified contributions
func TestTC037_CreditMinting(t *testing.T) {
	tc := SetupTestContext(t)

	contributionID := "contrib-004"
	contributor := sdk.AccAddress("contributor_______")

	// Record initial credits
	initialCredits := tc.PoCKeeper.GetCredits(tc.Ctx, contributor)

	// Submit and verify contribution
	tc.PoCKeeper.SubmitContribution(tc.Ctx, contributionID, contributor, "QmHash")

	// Unanimous endorsement (5/5 yes)
	for i := 1; i <= 5; i++ {
		valAddr := sdk.ValAddress([]byte("validator" + string(rune('0'+i))))
		tc.PoCKeeper.Endorse(tc.Ctx, contributionID, valAddr, true)
	}

	// Process and mint credits
	tc.PoCKeeper.ProcessEndorsements(tc.Ctx, contributionID)
	tc.PoCKeeper.MintCredits(tc.Ctx, contributionID, contributor, 10) // 10 credits per contribution

	// Verify credits increased
	finalCredits := tc.PoCKeeper.GetCredits(tc.Ctx, contributor)
	require.Equal(t, initialCredits+10, finalCredits, "Credits should increase by 10")
}

// TC-038: No Credit for Rejected
// Priority: P0
// Purpose: Verify rejected contributions don't mint credits
func TestTC038_NoCreditForRejected(t *testing.T) {
	tc := SetupTestContext(t)

	contributionID := "contrib-005"
	contributor := sdk.AccAddress("contributor_______")

	// Record initial credits
	initialCredits := tc.PoCKeeper.GetCredits(tc.Ctx, contributor)

	// Submit contribution
	tc.PoCKeeper.SubmitContribution(tc.Ctx, contributionID, contributor, "QmHash")

	// All validators reject (0/5 yes)
	for i := 1; i <= 5; i++ {
		valAddr := sdk.ValAddress([]byte("validator" + string(rune('0'+i))))
		tc.PoCKeeper.Endorse(tc.Ctx, contributionID, valAddr, false)
	}

	// Process endorsements (should reject)
	tc.PoCKeeper.ProcessEndorsements(tc.Ctx, contributionID)

	// Verify contribution rejected
	contrib := tc.PoCKeeper.GetContribution(tc.Ctx, contributionID)
	require.Equal(t, "rejected", contrib.Status)

	// Verify credits unchanged
	finalCredits := tc.PoCKeeper.GetCredits(tc.Ctx, contributor)
	require.Equal(t, initialCredits, finalCredits, "Credits should not change for rejected contribution")
}

// TC-039: Effective Power - Base (No Credits)
// Priority: P0
// Purpose: Verify effective power equals stake when validator has no credits
func TestTC039_EffectivePowerBase(t *testing.T) {
	tc := SetupTestContext(t)

	_ = sdk.ValAddress("validator1_________") // Not used in mock implementation
	stake := math.NewInt(100_000_000_000)     // 100k OMNI
	credits := int64(0)
	alpha := math.LegacyMustNewDecFromStr("0.1")

	// Calculate effective power: Power = Stake × (1 + α × Credits)
	effectivePower := tc.PoCKeeper.CalculateEffectivePower(stake, credits, alpha)

	// With 0 credits: Power = 100k × (1 + 0.1 × 0) = 100k
	require.Equal(t, stake.Int64(), effectivePower.Int64(),
		"Effective power should equal stake when credits = 0")
}

// TC-040: Effective Power - With Credits
// Priority: P0
// Purpose: Verify effective power increases with credits
func TestTC040_EffectivePowerWithCredits(t *testing.T) {
	tc := SetupTestContext(t)

	stake := math.NewInt(100_000_000_000) // 100k OMNI
	credits := int64(50)
	alpha := math.LegacyMustNewDecFromStr("0.1")

	// Calculate effective power: Power = Stake × (1 + α × Credits)
	// Power = 100k × (1 + 0.1 × 50) = 100k × 6 = 600k
	effectivePower := tc.PoCKeeper.CalculateEffectivePower(stake, credits, alpha)

	expected := stake.MulRaw(6) // 100k × 6
	require.Equal(t, expected.Int64(), effectivePower.Int64(),
		"Effective power should be 6x stake with 50 credits and α=0.1")
}

// TC-041: Alpha Bounds - Minimum (α = 0)
// Priority: P0
// Purpose: Verify credits have no effect when α = 0
func TestTC041_AlphaBoundsMinimum(t *testing.T) {
	tc := SetupTestContext(t)

	stake := math.NewInt(100_000_000_000) // 100k OMNI
	credits := int64(100)
	alpha := math.LegacyZeroDec() // α = 0

	// Calculate effective power with α = 0
	effectivePower := tc.PoCKeeper.CalculateEffectivePower(stake, credits, alpha)

	// With α = 0: Power = Stake × (1 + 0 × Credits) = Stake
	require.Equal(t, stake.Int64(), effectivePower.Int64(),
		"Effective power should equal stake when α = 0")
}

// TC-042: Alpha Bounds - Maximum (α = 1.0)
// Priority: P0
// Purpose: Verify alpha respects maximum bound
func TestTC042_AlphaBoundsMaximum(t *testing.T) {
	tc := SetupTestContext(t)

	stake := math.NewInt(100_000_000_000) // 100k OMNI
	credits := int64(50)
	alphaMax := math.LegacyOneDec() // α = 1.0 (maximum)

	// Calculate effective power with α = 1.0
	effectivePower := tc.PoCKeeper.CalculateEffectivePower(stake, credits, alphaMax)

	// With α = 1.0: Power = Stake × (1 + 1.0 × 50) = Stake × 51 = 5.1M
	expected := stake.MulRaw(51)
	require.Equal(t, expected.Int64(), effectivePower.Int64(),
		"Effective power should be 51x stake with 50 credits and α=1.0")

	// TODO: Add PoCAlpha field to TokenomicsParams proto definition
	// This validation would verify α is bounded (cannot exceed 1.0)
	// alphaTooHigh := math.LegacyMustNewDecFromStr("1.5")
	// params := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	// params.PoCAlpha = alphaTooHigh
	// err := params.Validate()
	// require.Error(t, err, "Alpha > 1.0 should fail validation")
}

// TC-043: Alpha Update Causality
// Priority: P0
// Purpose: Verify alpha updates only affect future blocks
func TestTC043_AlphaUpdateCausality(t *testing.T) {
	// NOTE: This test requires full proto regeneration with `ignite generate proto-go`
	// to properly support marshaling/unmarshaling of the new PoCAlpha field.
	// The proto definition has been updated in proto/pos/tokenomics/v1/params.proto
	// and the struct has been manually updated in params.pb.go, but full code generation
	// is needed for complete marshal/unmarshal support.
	t.Skip("Skipping until proto files are regenerated with ignite (marshaling support needed)")

	// When enabled, this test will verify:
	// 1. PoC alpha can be updated via governance
	// 2. Updates take effect after param_change_delay
	// 3. Effective power calculations use the updated alpha for future blocks only
	t.Log("Alpha update causality should be tested after full proto regeneration")
}

// TC-044: Fraud - False Endorsement
// Priority: P0
// Purpose: Verify validators are slashed for endorsing invalid contributions
func TestTC044_FraudFalseEndorsement(t *testing.T) {
	tc := SetupTestContext(t)

	contributionID := "contrib-fraud-001"
	contributor := sdk.AccAddress("contributor_______")
	fraudValidator := sdk.ValAddress("fraud_validator___")

	// Submit invalid contribution (marked as fraudulent)
	tc.PoCKeeper.SubmitContribution(tc.Ctx, contributionID, contributor, "InvalidHash")

	// Fraud validator endorses it
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, fraudValidator, true)

	// Fraud proof submitted
	tc.PoCKeeper.SubmitFraudProof(tc.Ctx, contributionID, fraudValidator)

	// Verify validator slashed
	slashEvent := tc.PoCKeeper.GetSlashEvent(tc.Ctx, fraudValidator, contributionID)
	require.NotNil(t, slashEvent, "Slash event should be recorded")
	require.Equal(t, "fraud_endorsement", slashEvent.Reason)
	require.True(t, slashEvent.Amount.GT(math.ZeroInt()), "Slash amount should be positive")
}

// TC-045: Fraud - Contradictory Votes
// Priority: P0
// Purpose: Verify validators cannot vote both yes and no
func TestTC045_FraudContradictoryVotes(t *testing.T) {
	tc := SetupTestContext(t)

	contributionID := "contrib-006"
	contributor := sdk.AccAddress("contributor_______")
	validator := sdk.ValAddress("validator1_________")

	// Submit contribution
	tc.PoCKeeper.SubmitContribution(tc.Ctx, contributionID, contributor, "QmHash")

	// Validator votes yes
	tc.PoCKeeper.Endorse(tc.Ctx, contributionID, validator, true)

	// Attempt to vote no (should fail)
	err := tc.PoCKeeper.EndorseWithError(tc.Ctx, contributionID, validator, false)
	require.Error(t, err, "Validator cannot vote twice")
	require.Contains(t, err.Error(), "already endorsed", "Error should mention duplicate endorsement")
}

// TC-046: Rate Limiting - Burst
// Priority: P0
// Purpose: Verify per-block contribution quota is enforced
func TestTC046_RateLimitingBurst(t *testing.T) {
	tc := SetupTestContext(t)

	perBlockQuota := 10 // Max 10 contributions per block
	contributor := sdk.AccAddress("contributor_______")

	// Submit quota + 1 contributions in same block
	for i := 0; i < perBlockQuota+1; i++ {
		contributionID := "contrib-burst-" + string(rune('0'+i))
		err := tc.PoCKeeper.SubmitContributionWithQuota(tc.Ctx, contributionID, contributor, "QmHash", perBlockQuota)

		if i < perBlockQuota {
			require.NoError(t, err, "First %d contributions should succeed", perBlockQuota)
		} else {
			require.Error(t, err, "Contribution beyond quota should fail")
			require.Contains(t, err.Error(), "quota exceeded")
		}
	}
}

// TC-047: Rate Limiting - Spam
// Priority: P0
// Purpose: Verify fee throttling prevents spam
func TestTC047_RateLimitingSpam(t *testing.T) {
	tc := SetupTestContext(t)

	contributor := sdk.AccAddress("contributor_______")
	initialBalance := math.NewInt(500_000_000) // 500 OMNI (enough for ~5000 submissions)
	tc.BankKeeper.SetBalance(tc.Ctx, contributor, sdk.NewCoins(sdk.NewCoin(TestDenom, initialBalance)))

	submissionFee := math.NewInt(100_000) // 0.1 OMNI per submission

	// Attempt 10,000 submissions (would cost 1,000 OMNI in fees, but we only have 500 OMNI)
	successCount := 0
	for i := 0; i < 10000; i++ {
		contributionID := "contrib-spam-" + string(rune(i))
		err := tc.PoCKeeper.SubmitContributionWithFee(tc.Ctx, contributionID, contributor, "QmHash", submissionFee)
		if err == nil {
			successCount++
		} else {
			// Fee exhaustion - balance too low
			break
		}
	}

	// Verify fees throttled spam (contributor ran out of funds)
	finalBalance := tc.BankKeeper.GetBalance(tc.Ctx, contributor, TestDenom).Amount
	feesSpent := initialBalance.Sub(finalBalance)

	t.Logf("Spam attack: %d submissions succeeded; %s OMNI spent in fees", successCount, feesSpent.String())
	require.Less(t, successCount, 10000, "Spam should be throttled by fees")
	require.True(t, feesSpent.GT(math.ZeroInt()), "Fees should have been collected")
}

// TC-048: Credit Decay (if enabled)
// Priority: P0
// Purpose: Verify credit decay applied per policy
func TestTC048_CreditDecay(t *testing.T) {
	tc := SetupTestContext(t)

	contributor := sdk.AccAddress("contributor_______")
	initialCredits := int64(100)
	tc.PoCKeeper.SetCredits(tc.Ctx, contributor, initialCredits)

	// Simulate 365 days (1 year) of decay at 10% annual rate
	decayRate := math.LegacyMustNewDecFromStr("0.1") // 10% per year
	daysElapsed := int64(365)

	// Apply decay
	tc.PoCKeeper.ApplyDecay(tc.Ctx, contributor, decayRate, daysElapsed)

	// Calculate expected: 100 × (1 - 0.1) = 90 credits
	expectedCredits := int64(90)
	finalCredits := tc.PoCKeeper.GetCredits(tc.Ctx, contributor)

	require.Equal(t, expectedCredits, finalCredits,
		"Credits should decay by 10% after 1 year")
}

// TC-049: Credit Non-Negative
// Priority: P0
// Purpose: Verify credits never go negative even with maximum decay
func TestTC049_CreditNonNegative(t *testing.T) {
	tc := SetupTestContext(t)

	contributor := sdk.AccAddress("contributor_______")
	initialCredits := int64(10)
	tc.PoCKeeper.SetCredits(tc.Ctx, contributor, initialCredits)

	// Apply extreme decay (100% decay over 10 years)
	decayRate := math.LegacyOneDec() // 100% decay
	daysElapsed := int64(3650) // 10 years

	// Apply decay
	tc.PoCKeeper.ApplyDecay(tc.Ctx, contributor, decayRate, daysElapsed)

	// Verify credits are 0 (not negative)
	finalCredits := tc.PoCKeeper.GetCredits(tc.Ctx, contributor)
	require.GreaterOrEqual(t, finalCredits, int64(0), "Credits should never be negative")
	require.Equal(t, int64(0), finalCredits, "Credits should be 0 after maximum decay")
}
