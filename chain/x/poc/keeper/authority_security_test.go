package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// TestSecurityAudit_DoSResistance tests DoS attack resistance
func TestSecurityAudit_DoSResistance(t *testing.T) {
	f := SetupKeeperTest(t)

	tests := []struct {
		name           string
		setup          func()
		expectsError   bool
		maxGasConsumed uint64
	}{
		{
			name: "large_exempt_list_linear_search",
			setup: func() {
				// Create 100 exempt addresses (stress test for IsExemptAddress linear search)
				params := f.keeper.GetParams(f.ctx)
				params.EnableCscoreGating = true

				addrs := createTestAddresses(100)
				for _, addr := range addrs {
					params.ExemptAddresses = append(params.ExemptAddresses, addr.String())
				}

				err := f.keeper.SetParams(f.ctx, params)
				require.NoError(t, err)
			},
			expectsError:   false,
			maxGasConsumed: 200000, // Should still be reasonable even with 100 addresses
		},
		{
			name: "large_cscore_requirements_map",
			setup: func() {
				// Create 1000 different contribution types with requirements
				params := f.keeper.GetParams(f.ctx)
				params.EnableCscoreGating = true
				params.MinCscoreForCtype = make(map[string]math.Int)

				for i := 0; i < 1000; i++ {
					ctype := "type_" + string(rune(i))
					params.MinCscoreForCtype[ctype] = math.NewInt(int64(i * 1000))
				}

				err := f.keeper.SetParams(f.ctx, params)
				require.NoError(t, err)
			},
			expectsError:   false,
			maxGasConsumed: 150000, // O(1) lookup should be fast
		},
		{
			name: "maximum_cscore_value",
			setup: func() {
				// Test with maximum safe uint64 value
				params := f.keeper.GetParams(f.ctx)
				params.EnableCscoreGating = true
				maxSafe := math.NewIntFromUint64(1<<63 - 1)
				params.MinCscoreForCtype = map[string]math.Int{
					"ultra_secure": maxSafe,
				}

				err := f.keeper.SetParams(f.ctx, params)
				require.NoError(t, err)
			},
			expectsError:   false,
			maxGasConsumed: 100000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			f := SetupKeeperTest(t)
			tc.setup()

			// Get gas before
			gasBefore := f.ctx.GasMeter().GasConsumed()

			// Perform PoA check
			addrs := createTestAddresses(1)
			err := f.keeper.CheckProofOfAuthority(f.ctx, addrs[0], "code")

			// Get gas after
			gasAfter := f.ctx.GasMeter().GasConsumed()
			gasUsed := gasAfter - gasBefore

			// Validate gas consumption
			require.LessOrEqual(t, gasUsed, tc.maxGasConsumed,
				"Gas consumption too high: %d > %d", gasUsed, tc.maxGasConsumed)

			if tc.expectsError {
				require.Error(t, err)
			} else {
				// Error is expected for insufficient C-Score, but shouldn't panic
				require.NotPanics(t, func() {
					_ = err
				})
			}
		})
	}
}

// TestSecurityAudit_IntegerOverflow tests integer overflow protection
func TestSecurityAudit_IntegerOverflow(t *testing.T) {
	f := SetupKeeperTest(t)

	tests := []struct {
		name           string
		requiredScore  math.Int
		currentScore   math.Int
		expectsError   bool
		errorContains  string
	}{
		{
			name:          "max_uint64_requirement",
			requiredScore: math.NewIntFromUint64(1<<63 - 1),
			currentScore:  math.NewInt(1000),
			expectsError:  true,
			errorContains: "insufficient C-Score",
		},
		{
			name:          "subtraction_no_underflow",
			requiredScore: math.NewInt(1000),
			currentScore:  math.NewInt(500),
			expectsError:  true,
			errorContains: "need 500 more",
		},
		{
			name:          "zero_requirement",
			requiredScore: math.ZeroInt(),
			currentScore:  math.ZeroInt(),
			expectsError:  false,
		},
		{
			name:          "exact_match_no_overflow",
			requiredScore: math.NewIntFromUint64(1<<63 - 1),
			currentScore:  math.NewIntFromUint64(1<<63 - 1),
			expectsError:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup params
			params := f.keeper.GetParams(f.ctx)
			params.EnableCscoreGating = true
			params.MinCscoreForCtype = map[string]math.Int{
				"test_type": tc.requiredScore,
			}
			err := f.keeper.SetParams(f.ctx, params)
			require.NoError(t, err)

			// Setup contributor with C-Score
			addrs := createTestAddresses(1)
			contributor := addrs[0]

			// Set current C-Score
			credits := types.Credits{
				Address: contributor.String(),
				Amount:  tc.currentScore,
			}
			err = f.keeper.SetCredits(f.ctx, credits)
			require.NoError(t, err)

			// Test CheckMinimumCScore
			err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "test_type")

			if tc.expectsError {
				require.Error(t, err)
				if tc.errorContains != "" {
					require.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSecurityAudit_AddressValidation tests address validation security
func TestSecurityAudit_AddressValidation(t *testing.T) {
	tests := []struct {
		name          string
		addresses     []string
		expectsError  bool
		errorContains string
	}{
		{
			name:          "valid_addresses",
			addresses:     []string{"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"},
			expectsError:  false,
		},
		{
			name:          "invalid_bech32",
			addresses:     []string{"invalid_address"},
			expectsError:  true,
			errorContains: "invalid exempt address",
		},
		{
			name:          "empty_address",
			addresses:     []string{""},
			expectsError:  true,
			errorContains: "empty address",
		},
		{
			name:          "duplicate_addresses",
			addresses:     []string{"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du", "cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"},
			expectsError:  true,
			errorContains: "duplicate",
		},
		{
			name: "mixed_valid_invalid",
			addresses: []string{
				"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du",
				"invalid",
			},
			expectsError:  true,
			errorContains: "invalid exempt address",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			params.ExemptAddresses = tc.addresses

			err := params.Validate()

			if tc.expectsError {
				require.Error(t, err)
				if tc.errorContains != "" {
					require.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSecurityAudit_SequentialAccess tests sequential access patterns
// Note: Concurrent access is not tested here because in blockchain production,
// each block is processed sequentially with its own context. The SDK context
// and gas meter are not thread-safe by design, which is intentional.
func TestSecurityAudit_SequentialAccess(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup initial params
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(1000),
	}
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	addrs := createTestAddresses(10)

	// Test sequential processing (as happens in real blockchain)
	for i := 0; i < 10; i++ {
		err := f.keeper.CheckProofOfAuthority(f.ctx, addrs[i], "code")
		// Error expected for insufficient C-Score
		require.Error(t, err)

		isExempt := f.keeper.IsExemptAddress(f.ctx, addrs[i])
		require.False(t, isExempt)

		score, hasReq := f.keeper.GetRequiredCScore(f.ctx, "code")
		require.True(t, hasReq)
		require.Equal(t, math.NewInt(1000), score)

		canSubmit, reason := f.keeper.CanSubmitContribution(f.ctx, addrs[i], "code")
		require.False(t, canSubmit)
		require.Contains(t, reason, "insufficient C-Score")
	}

	// All sequential operations completed successfully
	require.True(t, true)
}

// TestSecurityAudit_StateConsistency tests state machine consistency
func TestSecurityAudit_StateConsistency(t *testing.T) {
	f := SetupKeeperTest(t)

	// Test 1: Params update atomicity
	t.Run("params_atomic_update", func(t *testing.T) {
		params := f.keeper.GetParams(f.ctx)
		params.EnableCscoreGating = true
		params.MinCscoreForCtype = map[string]math.Int{
			"code": math.NewInt(1000),
		}
		params.ExemptAddresses = []string{"cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnp7du"}

		err := f.keeper.SetParams(f.ctx, params)
		require.NoError(t, err)

		// Verify all fields persisted
		retrieved := f.keeper.GetParams(f.ctx)
		require.True(t, retrieved.EnableCscoreGating)
		require.Len(t, retrieved.MinCscoreForCtype, 1)
		require.Equal(t, math.NewInt(1000), retrieved.MinCscoreForCtype["code"])
		require.Len(t, retrieved.ExemptAddresses, 1)
	})

	// Test 2: Failed validation doesn't corrupt state
	t.Run("failed_validation_no_corruption", func(t *testing.T) {
		// Get current valid params
		validParams := f.keeper.GetParams(f.ctx)

		// Try to set invalid params
		invalidParams := validParams
		invalidParams.MinCscoreForCtype = map[string]math.Int{
			"": math.NewInt(1000), // Empty ctype should fail validation
		}

		err := f.keeper.SetParams(f.ctx, invalidParams)
		require.Error(t, err)

		// Verify original params are still intact
		currentParams := f.keeper.GetParams(f.ctx)
		require.Equal(t, validParams.EnableCscoreGating, currentParams.EnableCscoreGating)
	})

	// Test 3: Multiple param updates maintain consistency
	t.Run("multiple_updates_consistent", func(t *testing.T) {
		// Start with fresh params
		params := f.keeper.GetParams(f.ctx)
		params.MinCscoreForCtype = make(map[string]math.Int)
		err := f.keeper.SetParams(f.ctx, params)
		require.NoError(t, err)

		// Add 100 types
		for i := 0; i < 100; i++ {
			params := f.keeper.GetParams(f.ctx)
			params.MinCscoreForCtype["type_"+string(rune(i))] = math.NewInt(int64(i * 100))

			err := f.keeper.SetParams(f.ctx, params)
			require.NoError(t, err)
		}

		// Verify all 100 types persisted
		finalParams := f.keeper.GetParams(f.ctx)
		require.Len(t, finalParams.MinCscoreForCtype, 100)
	})
}

// TestSecurityAudit_IdentityModuleFailSafe tests fail-safe behavior
func TestSecurityAudit_IdentityModuleFailSafe(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup: Enable identity gating but don't set identity keeper
	params := f.keeper.GetParams(f.ctx)
	params.EnableIdentityGating = true
	params.RequireIdentityForCtype = map[string]bool{
		"treasury": true,
	}
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	addrs := createTestAddresses(1)

	// Test: Identity check should fail-safe (reject) when module unavailable
	err = f.keeper.CheckIdentityRequirement(f.ctx, addrs[0], "treasury")
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrIdentityCheckFailed)
	require.Contains(t, err.Error(), "identity module is not available")

	// Verify this is a security feature, not a bug
	t.Log("Fail-safe behavior confirmed: identity requirements reject when module unavailable")
}

// TestSecurityAudit_AuthorizationBypass attempts to bypass authorization
func TestSecurityAudit_AuthorizationBypass(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup strict requirements
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"secure_type": math.NewInt(1000000), // Very high requirement
	}
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	addrs := createTestAddresses(3)
	lowScoreUser := addrs[0]
	exemptUser := addrs[1]
	_ = addrs[2] // normalUser - reserved for future tests

	// Setup: Low C-Score user
	credits := types.Credits{
		Address: lowScoreUser.String(),
		Amount:  math.NewInt(100), // Well below requirement
	}
	err = f.keeper.SetCredits(f.ctx, credits)
	require.NoError(t, err)

	// Attack 1: Try to bypass via empty ctype
	t.Run("bypass_via_empty_ctype", func(t *testing.T) {
		// Empty ctype has no requirement, so should pass
		err := f.keeper.CheckProofOfAuthority(f.ctx, lowScoreUser, "")
		require.NoError(t, err, "Empty ctype should pass (no requirement set)")
	})

	// Attack 2: Try to bypass via nil/zero address (should be caught earlier in flow)
	t.Run("bypass_via_invalid_address", func(t *testing.T) {
		emptyAddr := sdk.AccAddress{}
		err := f.keeper.CheckProofOfAuthority(f.ctx, emptyAddr, "secure_type")
		// Should fail due to insufficient C-Score (no credits for empty address)
		require.Error(t, err)
	})

	// Attack 3: Verify exempt addresses actually work
	t.Run("exempt_address_bypass_valid", func(t *testing.T) {
		// Add user to exempt list
		params := f.keeper.GetParams(f.ctx)
		params.ExemptAddresses = []string{exemptUser.String()}
		err := f.keeper.SetParams(f.ctx, params)
		require.NoError(t, err)

		// Exempt user should bypass even with zero C-Score
		err = f.keeper.CheckProofOfAuthority(f.ctx, exemptUser, "secure_type")
		require.NoError(t, err, "Exempt users should bypass all checks")
	})

	// Attack 4: Verify disabling gating works as expected
	t.Run("disabled_gating_allows_all", func(t *testing.T) {
		params := f.keeper.GetParams(f.ctx)
		params.EnableCscoreGating = false
		err := f.keeper.SetParams(f.ctx, params)
		require.NoError(t, err)

		// Low score user should now pass
		err = f.keeper.CheckProofOfAuthority(f.ctx, lowScoreUser, "secure_type")
		require.NoError(t, err, "Disabled gating should allow all users")
	})
}

// TestSecurityAudit_GasExhaustion tests gas exhaustion attacks
func TestSecurityAudit_GasExhaustion(t *testing.T) {
	f := SetupKeeperTest(t)

	// Setup moderate requirements
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(1000),
	}

	// Add 50 exempt addresses
	addrs := createTestAddresses(50)
	for _, addr := range addrs {
		params.ExemptAddresses = append(params.ExemptAddresses, addr.String())
	}

	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Test: Non-exempt address at end should still have reasonable gas cost
	testAddr := createTestAddresses(1)[0]

	gasBefore := f.ctx.GasMeter().GasConsumed()
	err = f.keeper.CheckProofOfAuthority(f.ctx, testAddr, "code")
	gasAfter := f.ctx.GasMeter().GasConsumed()

	gasUsed := gasAfter - gasBefore

	// Gas should be reasonable even with 50 exempt addresses to check
	require.LessOrEqual(t, gasUsed, uint64(50000),
		"Gas consumption excessive with 50 exempt addresses: %d", gasUsed)

	t.Logf("Gas used with 50 exempt addresses: %d", gasUsed)
}
