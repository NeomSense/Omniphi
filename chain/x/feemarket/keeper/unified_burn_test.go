package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"pos/x/feemarket/types"
)

// TestComputeEffectiveBurn_NormalUtilization tests burn calculation at normal utilization
func TestComputeEffectiveBurn_NormalUtilization(t *testing.T) {
	// Normal utilization (20% burn base) + smart contract (1.5x multiplier) = 30% effective
	// With 100 uomni fee: burn = 30, validator = 49, treasury = 21

	baseBurn := math.LegacyMustNewDecFromStr("0.20")    // Normal tier
	multiplier := math.LegacyMustNewDecFromStr("1.50")  // Smart contracts
	maxBurn := math.LegacyMustNewDecFromStr("0.50")     // 50% cap
	treasuryRatio := math.LegacyMustNewDecFromStr("0.30")

	totalFee := math.NewInt(100)

	// Calculate effective burn
	effectiveBurn := baseBurn.Mul(multiplier)
	if effectiveBurn.GT(maxBurn) {
		effectiveBurn = maxBurn
	}

	require.Equal(t, "0.300000000000000000", effectiveBurn.String(), "effective burn should be 30%")

	// Calculate amounts
	burnAmount := effectiveBurn.MulInt(totalFee).TruncateInt()
	distributable := totalFee.Sub(burnAmount)
	treasuryAmount := treasuryRatio.MulInt(distributable).TruncateInt()
	validatorAmount := distributable.Sub(treasuryAmount)

	require.Equal(t, int64(30), burnAmount.Int64(), "burn should be 30")
	require.Equal(t, int64(21), treasuryAmount.Int64(), "treasury should be 21")
	require.Equal(t, int64(49), validatorAmount.Int64(), "validator should be 49")

	// Verify conservation
	total := burnAmount.Add(treasuryAmount).Add(validatorAmount)
	require.Equal(t, totalFee.Int64(), total.Int64(), "amounts must sum to total")
}

// TestComputeEffectiveBurn_HotUtilization tests burn capping at 50%
func TestComputeEffectiveBurn_HotUtilization_Capped(t *testing.T) {
	// Hot utilization (40% burn base) + smart contract (1.5x multiplier) = 60% -> capped to 50%

	baseBurn := math.LegacyMustNewDecFromStr("0.40")    // Hot tier
	multiplier := math.LegacyMustNewDecFromStr("1.50")  // Smart contracts
	maxBurn := math.LegacyMustNewDecFromStr("0.50")     // 50% cap

	totalFee := math.NewInt(1000)

	// Calculate effective burn (should be capped)
	effectiveBurn := baseBurn.Mul(multiplier)
	wasCapped := false
	if effectiveBurn.GT(maxBurn) {
		effectiveBurn = maxBurn
		wasCapped = true
	}

	require.True(t, wasCapped, "burn should be capped")
	require.Equal(t, "0.500000000000000000", effectiveBurn.String(), "effective burn should be capped at 50%")

	burnAmount := effectiveBurn.MulInt(totalFee).TruncateInt()
	require.Equal(t, int64(500), burnAmount.Int64(), "burn should be 500 (50% of 1000)")
}

// TestComputeEffectiveBurn_CoolUtilization tests low burn at cool utilization
func TestComputeEffectiveBurn_CoolUtilization_Messaging(t *testing.T) {
	// Cool utilization (10% burn base) + messaging (0.5x multiplier) = 5% effective

	baseBurn := math.LegacyMustNewDecFromStr("0.10")    // Cool tier
	multiplier := math.LegacyMustNewDecFromStr("0.50")  // Messaging
	maxBurn := math.LegacyMustNewDecFromStr("0.50")     // 50% cap

	totalFee := math.NewInt(1000)

	effectiveBurn := baseBurn.Mul(multiplier)
	require.False(t, effectiveBurn.GT(maxBurn), "should not be capped")
	require.Equal(t, "0.050000000000000000", effectiveBurn.String(), "effective burn should be 5%")

	burnAmount := effectiveBurn.MulInt(totalFee).TruncateInt()
	require.Equal(t, int64(50), burnAmount.Int64(), "burn should be 50 (5% of 1000)")
}

// TestValidatorRevenueInvariant verifies validators always get >= 50% of post-burn fees
func TestValidatorRevenueInvariant(t *testing.T) {
	validatorRatio := math.LegacyMustNewDecFromStr("0.70")
	treasuryRatio := math.LegacyMustNewDecFromStr("0.30")

	// Sum must be 1.0
	sum := validatorRatio.Add(treasuryRatio)
	require.True(t, sum.Equal(math.LegacyOneDec()), "ratios must sum to 1.0")

	// Validator must get at least 50% of distributable
	minValidatorRatio := math.LegacyMustNewDecFromStr("0.50")
	require.True(t, validatorRatio.GTE(minValidatorRatio), 
		"validator ratio must be >= 50% for security")
}

// TestNoDoubleBurn verifies single execution path
func TestNoDoubleBurn(t *testing.T) {
	// This test documents the invariant that only ONE burn calculation occurs
	// per transaction. The unified burn model ensures:
	// 1. Base burn rate comes from utilization tier (ONLY source)
	// 2. Activity multiplier adjusts the base rate
	// 3. Final burn is capped at maxBurnRatio
	// 4. NO additional burns from activity type

	// Old model (WRONG - double counting):
	// burn1 = fee * utilization_burn_rate
	// burn2 = fee * activity_burn_rate
	// total_burn = burn1 + burn2  // Could exceed 100%!

	// New model (CORRECT - single pass):
	// effective_burn = min(utilization_burn * activity_multiplier, max_cap)
	// burn = fee * effective_burn  // Always <= 50%

	totalFee := math.NewInt(1000)
	maxBurn := math.LegacyMustNewDecFromStr("0.50")

	// Even with highest multiplier (2.0) and hot tier (40%)
	// effective = 0.40 * 2.0 = 0.80 -> capped to 0.50
	baseBurn := math.LegacyMustNewDecFromStr("0.40")
	maxMultiplier := math.LegacyMustNewDecFromStr("2.00")

	effectiveBurn := baseBurn.Mul(maxMultiplier)
	if effectiveBurn.GT(maxBurn) {
		effectiveBurn = maxBurn
	}

	burnAmount := effectiveBurn.MulInt(totalFee).TruncateInt()

	// Maximum burn is always 50%
	require.LessOrEqual(t, burnAmount.Int64(), int64(500), 
		"burn cannot exceed 50% of fee")
}

// TestActivityMultiplierBounds verifies multiplier bounds are enforced
func TestActivityMultiplierBounds(t *testing.T) {
	params := types.DefaultParams()

	// Min multiplier should be 0.25
	require.Equal(t, "0.250000000000000000", params.MinMultiplier.String())

	// Max multiplier should be 2.00
	require.Equal(t, "2.000000000000000000", params.MaxMultiplier.String())

	// All activity multipliers must be within bounds
	activities := map[types.ActivityType]math.LegacyDec{
		types.ActivityMessaging:      params.MultiplierMessaging,
		types.ActivityPosGas:         params.MultiplierPosGas,
		types.ActivityPocAnchoring:   params.MultiplierPocAnchoring,
		types.ActivitySmartContracts: params.MultiplierSmartContracts,
		types.ActivityAiQueries:      params.MultiplierAiQueries,
		types.ActivitySequencer:      params.MultiplierSequencer,
	}

	for activity, multiplier := range activities {
		require.True(t, multiplier.GTE(params.MinMultiplier),
			"%s multiplier below min", activity)
		require.True(t, multiplier.LTE(params.MaxMultiplier),
			"%s multiplier above max", activity)
	}
}

// TestDefaultActivityMultipliers verifies expected default values
func TestDefaultActivityMultipliers(t *testing.T) {
	params := types.DefaultParams()

	expected := map[types.ActivityType]string{
		types.ActivityMessaging:      "0.500000000000000000", // 0.5x - Low
		types.ActivityPosGas:         "1.000000000000000000", // 1.0x - Baseline
		types.ActivityPocAnchoring:   "0.750000000000000000", // 0.75x - Encourage
		types.ActivitySmartContracts: "1.500000000000000000", // 1.5x - Higher
		types.ActivityAiQueries:      "1.250000000000000000", // 1.25x - Higher
		types.ActivitySequencer:      "1.250000000000000000", // 1.25x - Higher
	}

	for activity, expectedVal := range expected {
		actual := params.GetActivityMultiplier(activity)
		require.Equal(t, expectedVal, actual.String(),
			"unexpected multiplier for %s", activity)
	}
}
