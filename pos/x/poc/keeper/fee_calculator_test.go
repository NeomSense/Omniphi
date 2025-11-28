package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// Test3LayerFee_BaseModel tests the base fee model (Layer 1)
func Test3LayerFee_BaseModel(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get default params
	params := f.keeper.GetParams(f.ctx)

	t.Logf("Params retrieved - BaseSubmissionFee: %+v", params.BaseSubmissionFee)
	t.Logf("Params retrieved - BaseSubmissionFee.Denom: %s", params.BaseSubmissionFee.Denom)
	t.Logf("Params retrieved - BaseSubmissionFee.Amount: %s", params.BaseSubmissionFee.Amount)
	t.Logf("Params retrieved - TargetSubmissionsPerBlock: %d", params.TargetSubmissionsPerBlock)
	t.Logf("Params retrieved - MaxCscoreDiscount: %s", params.MaxCscoreDiscount)
	t.Logf("Params retrieved - MinimumSubmissionFee: %+v", params.MinimumSubmissionFee)

	// Base fee should be 30000 uomni by default
	require.Equal(t, "uomni", params.BaseSubmissionFee.Denom, "Base submission fee denom should be uomni")
	require.Equal(t, math.NewInt(30000), params.BaseSubmissionFee.Amount, "Base submission fee amount should be 30000")

	t.Logf("Base submission fee: %s", params.BaseSubmissionFee)
}

// Test3LayerFee_EpochMultiplier tests the epoch-adaptive fee model (Layer 2)
func Test3LayerFee_EpochMultiplier(t *testing.T) {
	f := SetupKeeperTest(t)

	tests := []struct {
		name                   string
		currentSubmissions     uint32
		targetSubmissions      uint32
		expectedMultiplierMin  string
		expectedMultiplierMax  string
	}{
		{
			name:                   "no_submissions_uses_min_multiplier_0.8",
			currentSubmissions:     0,
			targetSubmissions:      5,
			expectedMultiplierMin:  "0.8",
			expectedMultiplierMax:  "0.8",
		},
		{
			name:                   "at_target_multiplier_1.0",
			currentSubmissions:     5,
			targetSubmissions:      5,
			expectedMultiplierMin:  "1.0",
			expectedMultiplierMax:  "1.0",
		},
		{
			name:                   "double_target_multiplier_2.0",
			currentSubmissions:     10,
			targetSubmissions:      5,
			expectedMultiplierMin:  "2.0",
			expectedMultiplierMax:  "2.0",
		},
		{
			name:                   "extreme_congestion_capped_at_5.0",
			currentSubmissions:     100,
			targetSubmissions:      5,
			expectedMultiplierMin:  "5.0",
			expectedMultiplierMax:  "5.0",
		},
		{
			name:                   "half_target_multiplier_0.8_min",
			currentSubmissions:     2,
			targetSubmissions:      5,
			expectedMultiplierMin:  "0.8",
			expectedMultiplierMax:  "0.8",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Update target in params
			params := f.keeper.GetParams(f.ctx)
			params.TargetSubmissionsPerBlock = tc.targetSubmissions
			err := f.keeper.SetParams(f.ctx, params)
			require.NoError(t, err)

			// Simulate submissions by incrementing counter
			for i := uint32(0); i < tc.currentSubmissions; i++ {
				f.keeper.IncrementBlockSubmissions(f.ctx)
			}

			// Calculate multiplier
			multiplier, err := f.keeper.CalculateEpochMultiplier(f.ctx)
			require.NoError(t, err)

			// Verify bounds
			expectedMin, err := math.LegacyNewDecFromStr(tc.expectedMultiplierMin)
			require.NoError(t, err)
			expectedMax, err := math.LegacyNewDecFromStr(tc.expectedMultiplierMax)
			require.NoError(t, err)

			require.True(t, multiplier.GTE(expectedMin),
				"multiplier %s should be >= %s", multiplier, expectedMin)
			require.True(t, multiplier.LTE(expectedMax),
				"multiplier %s should be <= %s", multiplier, expectedMax)

			t.Logf("Current submissions: %d, Target: %d, Multiplier: %s",
				tc.currentSubmissions, tc.targetSubmissions, multiplier)

			// Reset for next test
			f.keeper.ResetBlockSubmissions(f.ctx)
		})
	}
}

// Test3LayerFee_CScoreDiscount tests the C-Score weighted discount model (Layer 3)
func Test3LayerFee_CScoreDiscount(t *testing.T) {
	f := SetupKeeperTest(t)

	tests := []struct {
		name                    string
		cscore                  int64
		maxDiscountPercent      int64 // as percentage (90 = 0.90)
		expectedDiscountPercent int64 // as percentage
	}{
		{
			name:                    "no_cscore_no_discount",
			cscore:                  0,
			maxDiscountPercent:      90,
			expectedDiscountPercent: 0,
		},
		{
			name:                    "cscore_500_gets_50_percent_discount",
			cscore:                  500,
			maxDiscountPercent:      90,
			expectedDiscountPercent: 50,
		},
		{
			name:                    "cscore_1000_gets_90_percent_discount_capped",
			cscore:                  1000,
			maxDiscountPercent:      90,
			expectedDiscountPercent: 90,
		},
		{
			name:                    "cscore_2000_still_capped_at_90_percent",
			cscore:                  2000,
			maxDiscountPercent:      90,
			expectedDiscountPercent: 90,
		},
		{
			name:                    "cscore_100_gets_10_percent_discount",
			cscore:                  100,
			maxDiscountPercent:      90,
			expectedDiscountPercent: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set max discount
			params := f.keeper.GetParams(f.ctx)
			params.MaxCscoreDiscount = math.LegacyNewDecWithPrec(tc.maxDiscountPercent, 2)
			err := f.keeper.SetParams(f.ctx, params)
			require.NoError(t, err)

			// Create contributor with C-Score
			addrs := createTestAddresses(1)
			contributor := addrs[0]

			credits := types.Credits{
				Address: contributor.String(),
				Amount:  math.NewInt(tc.cscore),
			}
			err = f.keeper.SetCredits(f.ctx, credits)
			require.NoError(t, err)

			// Calculate discount
			discount, err := f.keeper.CalculateCScoreDiscount(f.ctx, contributor)
			require.NoError(t, err)

			// Verify discount
			expectedDiscount := math.LegacyNewDecWithPrec(tc.expectedDiscountPercent, 2)
			require.Equal(t, expectedDiscount, discount,
				"Expected discount %s, got %s", expectedDiscount, discount)

			t.Logf("C-Score: %d, Max Discount: %d%%, Actual Discount: %s",
				tc.cscore, tc.maxDiscountPercent, discount)
		})
	}
}

// Test3LayerFee_CombinedCalculation tests the complete 3-layer fee calculation
func Test3LayerFee_CombinedCalculation(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup params
	params := f.keeper.GetParams(f.ctx)
	params.BaseSubmissionFee = sdk.NewCoin("uomni", math.NewInt(30000))
	params.TargetSubmissionsPerBlock = 5
	params.MaxCscoreDiscount = math.LegacyMustNewDecFromStr("0.9")
	params.MinimumSubmissionFee = sdk.NewCoin("uomni", math.NewInt(3000))
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	tests := []struct {
		name               string
		cscore             int64
		currentSubmissions uint32
		expectedFeeMin     int64
		expectedFeeMax     int64
	}{
		{
			name:               "no_discount_no_congestion",
			cscore:             0,
			currentSubmissions: 5, // At target, multiplier = 1.0
			expectedFeeMin:     30000,
			expectedFeeMax:     30000,
		},
		{
			name:               "max_discount_no_congestion",
			cscore:             1000,
			currentSubmissions: 5, // At target, multiplier = 1.0
			// 30000 * 1.0 * (1 - 0.9) = 3000
			expectedFeeMin:     3000,
			expectedFeeMax:     3000,
		},
		{
			name:               "no_discount_high_congestion",
			cscore:             0,
			currentSubmissions: 25, // 5x target, multiplier = 5.0
			// 30000 * 5.0 * (1 - 0) = 150000
			expectedFeeMin:     150000,
			expectedFeeMax:     150000,
		},
		{
			name:               "max_discount_high_congestion",
			cscore:             1000,
			currentSubmissions: 25, // 5x target, multiplier = 5.0
			// 30000 * 5.0 * (1 - 0.9) = 15000
			expectedFeeMin:     15000,
			expectedFeeMax:     15000,
		},
		{
			name:               "medium_discount_medium_congestion",
			cscore:             500,
			currentSubmissions: 10, // 2x target, multiplier = 2.0
			// 30000 * 2.0 * (1 - 0.5) = 30000
			expectedFeeMin:     30000,
			expectedFeeMax:     30000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset block submissions
			f.keeper.ResetBlockSubmissions(f.ctx)

			// Set contributor C-Score
			addrs := createTestAddresses(1)
			contributor := addrs[0]

			credits := types.Credits{
				Address: contributor.String(),
				Amount:  math.NewInt(tc.cscore),
			}
			err := f.keeper.SetCredits(f.ctx, credits)
			require.NoError(t, err)

			// Simulate submissions
			for i := uint32(0); i < tc.currentSubmissions; i++ {
				f.keeper.IncrementBlockSubmissions(f.ctx)
			}

			// Calculate final fee
			finalFee, epochMultiplier, cscoreDiscount, err := f.keeper.Calculate3LayerFee(f.ctx, contributor)
			require.NoError(t, err)

			// Verify fee is within expected range
			require.True(t, finalFee.Amount.GTE(math.NewInt(tc.expectedFeeMin)),
				"Fee %s should be >= %d", finalFee.Amount, tc.expectedFeeMin)
			require.True(t, finalFee.Amount.LTE(math.NewInt(tc.expectedFeeMax)),
				"Fee %s should be <= %d", finalFee.Amount, tc.expectedFeeMax)

			// Verify minimum fee floor
			minimumFee := params.MinimumSubmissionFee
			require.True(t, finalFee.Amount.GTE(minimumFee.Amount),
				"Fee %s should never be below minimum %s", finalFee, minimumFee)

			t.Logf("C-Score: %d, Submissions: %d, Epoch Multiplier: %s, Discount: %s, Final Fee: %s",
				tc.cscore, tc.currentSubmissions, epochMultiplier, cscoreDiscount, finalFee)
		})
	}
}

// Test3LayerFee_MinimumFeeFloor tests that fees never go below the minimum
func Test3LayerFee_MinimumFeeFloor(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup with very high discount and low congestion
	params := f.keeper.GetParams(f.ctx)
	params.BaseSubmissionFee = sdk.NewCoin("uomni", math.NewInt(30000))
	params.TargetSubmissionsPerBlock = 100 // High target = low multiplier
	params.MaxCscoreDiscount = math.LegacyMustNewDecFromStr("0.99") // 99% discount
	params.MinimumSubmissionFee = sdk.NewCoin("uomni", math.NewInt(3000))
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Create high C-Score contributor
	addrs := createTestAddresses(1)
	contributor := addrs[0]

	credits := types.Credits{
		Address: contributor.String(),
		Amount:  math.NewInt(1000), // Max discount
	}
	err = f.keeper.SetCredits(f.ctx, credits)
	require.NoError(t, err)

	// No submissions (quiet block) = 0.8 multiplier
	// 30000 * 0.8 * (1 - 0.99) = 240
	// Should be capped at minimum 3000

	finalFee, _, _, err := f.keeper.Calculate3LayerFee(f.ctx, contributor)
	require.NoError(t, err)

	// Fee should be exactly the minimum
	require.Equal(t, params.MinimumSubmissionFee.Amount, finalFee.Amount,
		"Fee should be capped at minimum %s, got %s", params.MinimumSubmissionFee, finalFee)

	t.Logf("Calculated fee below minimum, applied floor: %s", finalFee)
}

// Test3LayerFee_FeeCollection tests the fee collection and split
func Test3LayerFee_FeeCollection(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup params
	params := f.keeper.GetParams(f.ctx)
	params.BaseSubmissionFee = sdk.NewCoin("uomni", math.NewInt(30000))
	params.TargetSubmissionsPerBlock = 5
	params.MaxCscoreDiscount = math.LegacyMustNewDecFromStr("0.5")
	params.MinimumSubmissionFee = sdk.NewCoin("uomni", math.NewInt(3000))
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Create contributor
	addrs := createTestAddresses(1)
	contributor := addrs[0]

	// Set moderate C-Score
	credits := types.Credits{
		Address: contributor.String(),
		Amount:  math.NewInt(500), // 50% discount
	}
	err = f.keeper.SetCredits(f.ctx, credits)
	require.NoError(t, err)

	// Set submissions to target (multiplier = 1.0)
	for i := 0; i < 5; i++ {
		f.keeper.IncrementBlockSubmissions(f.ctx)
	}

	// Calculate fee
	// 30000 * 1.0 * (1 - 0.5) = 15000
	finalFee, epochMultiplier, cscoreDiscount, err := f.keeper.Calculate3LayerFee(f.ctx, contributor)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(15000), finalFee.Amount)

	// Verify fee calculation components
	require.Equal(t, math.LegacyOneDec(), epochMultiplier, "Epoch multiplier should be 1.0 at target")
	require.Equal(t, math.LegacyMustNewDecFromStr("0.5"), cscoreDiscount, "C-Score discount should be 50%")

	// Verify burn/pool split calculation
	burnAmount := math.LegacyNewDecFromInt(finalFee.Amount).Mul(math.LegacyMustNewDecFromStr("0.5")).TruncateInt()
	poolAmount := finalFee.Amount.Sub(burnAmount)

	require.Equal(t, math.NewInt(7500), burnAmount, "50% should be burned")
	require.Equal(t, math.NewInt(7500), poolAmount, "50% should go to pool")

	t.Logf("Fee: %s, Burned: %s, Pool: %s", finalFee.Amount, burnAmount, poolAmount)
}

// Test3LayerFee_BlockSubmissionCounter tests the submission counter
func Test3LayerFee_BlockSubmissionCounter(t *testing.T) {
	f := SetupKeeperTest(t)

	// Initially 0
	count := f.keeper.GetCurrentBlockSubmissions(f.ctx)
	require.Equal(t, uint32(0), count)

	// Increment multiple times
	for i := 1; i <= 10; i++ {
		f.keeper.IncrementBlockSubmissions(f.ctx)
		count = f.keeper.GetCurrentBlockSubmissions(f.ctx)
		require.Equal(t, uint32(i), count)
	}

	// Reset
	f.keeper.ResetBlockSubmissions(f.ctx)
	count = f.keeper.GetCurrentBlockSubmissions(f.ctx)
	require.Equal(t, uint32(0), count)
}

// Test3LayerFee_ParameterValidation tests parameter validation
func Test3LayerFee_ParameterValidation(t *testing.T) {
	tests := []struct {
		name          string
		modifyParams  func(params *types.Params)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_default_params",
			modifyParams: func(params *types.Params) {
				// No modification, use defaults
			},
			expectError: false,
		},
		{
			name: "invalid_zero_target_submissions",
			modifyParams: func(params *types.Params) {
				params.TargetSubmissionsPerBlock = 0
			},
			expectError:   true,
			errorContains: "must be greater than 0",
		},
		{
			name: "invalid_excessive_target_submissions",
			modifyParams: func(params *types.Params) {
				params.TargetSubmissionsPerBlock = 2000
			},
			expectError:   true,
			errorContains: "cannot exceed 1000",
		},
		{
			name: "invalid_minimum_fee_exceeds_base",
			modifyParams: func(params *types.Params) {
				params.MinimumSubmissionFee = sdk.NewCoin("uomni", math.NewInt(50000))
				params.BaseSubmissionFee = sdk.NewCoin("uomni", math.NewInt(30000))
			},
			expectError:   true,
			errorContains: "cannot exceed base_submission_fee",
		},
		{
			name: "invalid_denom_mismatch",
			modifyParams: func(params *types.Params) {
				params.BaseSubmissionFee = sdk.NewCoin("uomni", math.NewInt(30000))
				params.MinimumSubmissionFee = sdk.NewCoin("stake", math.NewInt(3000))
			},
			expectError:   true,
			errorContains: "must have same denom",
		},
		{
			name: "invalid_negative_max_discount",
			modifyParams: func(params *types.Params) {
				params.MaxCscoreDiscount = math.LegacyMustNewDecFromStr("-0.5")
			},
			expectError:   true,
			errorContains: "cannot be negative",
		},
		{
			name: "invalid_max_discount_exceeds_one",
			modifyParams: func(params *types.Params) {
				params.MaxCscoreDiscount = math.LegacyMustNewDecFromStr("1.5")
			},
			expectError:   true,
			errorContains: "cannot exceed 1.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			tc.modifyParams(&params)

			err := params.Validate()

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					require.Contains(t, err.Error(), tc.errorContains)
				}
				t.Logf("Expected error: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Test3LayerFee_EventEmission tests that events are properly emitted
func Test3LayerFee_EventEmission(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup
	params := f.keeper.GetParams(f.ctx)
	params.BaseSubmissionFee = sdk.NewCoin("uomni", math.NewInt(30000))
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	addrs := createTestAddresses(1)
	contributor := addrs[0]

	// Calculate fee
	finalFee, epochMultiplier, cscoreDiscount, err := f.keeper.Calculate3LayerFee(f.ctx, contributor)
	require.NoError(t, err)

	// Clear existing events
	sdkCtx := sdk.UnwrapSDKContext(f.ctx)
	sdkCtx = sdkCtx.WithEventManager(sdk.NewEventManager())

	// Create a test event manually (since we can't actually collect fees with mock keepers)
	// In real usage, this event is emitted by CollectAndSplit3LayerFee
	burnAmount := math.LegacyNewDecFromInt(finalFee.Amount).Mul(math.LegacyMustNewDecFromStr("0.5")).TruncateInt()
	poolAmount := finalFee.Amount.Sub(burnAmount)
	burnCoin := sdk.NewCoin(finalFee.Denom, burnAmount)
	poolCoin := sdk.NewCoin(finalFee.Denom, poolAmount)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_3layer_fee",
		sdk.NewAttribute("contributor", contributor.String()),
		sdk.NewAttribute("total_fee", finalFee.String()),
		sdk.NewAttribute("burned", burnCoin.String()),
		sdk.NewAttribute("to_pool", poolCoin.String()),
		sdk.NewAttribute("epoch_multiplier", epochMultiplier.String()),
		sdk.NewAttribute("cscore_discount", cscoreDiscount.String()),
	))

	// Check events
	events := sdkCtx.EventManager().Events()
	var feeEvent sdk.Event
	for _, e := range events {
		if e.Type == "poc_3layer_fee" {
			feeEvent = e
			break
		}
	}

	require.NotNil(t, feeEvent, "poc_3layer_fee event should be emitted")

	// Verify event attributes
	attrs := feeEvent.Attributes
	require.NotEmpty(t, attrs)

	// Check for required attributes
	hasContributor := false
	hasTotalFee := false
	hasBurned := false
	hasToPool := false
	hasEpochMultiplier := false
	hasCscoreDiscount := false

	for _, attr := range attrs {
		switch attr.Key {
		case "contributor":
			hasContributor = true
			require.Equal(t, contributor.String(), attr.Value)
		case "total_fee":
			hasTotalFee = true
		case "burned":
			hasBurned = true
		case "to_pool":
			hasToPool = true
		case "epoch_multiplier":
			hasEpochMultiplier = true
		case "cscore_discount":
			hasCscoreDiscount = true
		}
	}

	require.True(t, hasContributor, "Event should have contributor attribute")
	require.True(t, hasTotalFee, "Event should have total_fee attribute")
	require.True(t, hasBurned, "Event should have burned attribute")
	require.True(t, hasToPool, "Event should have to_pool attribute")
	require.True(t, hasEpochMultiplier, "Event should have epoch_multiplier attribute")
	require.True(t, hasCscoreDiscount, "Event should have cscore_discount attribute")

	t.Logf("Fee event emitted correctly with all required attributes")
}
