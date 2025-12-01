package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// Helper to create test addresses
func createTestAddresses(count int) []sdk.AccAddress {
	addrs := make([]sdk.AccAddress, count)
	for i := 0; i < count; i++ {
		addrs[i] = sdk.AccAddress([]byte{byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1), byte(i + 1)})
	}
	return addrs
}

// TestCheckProofOfAuthority_Disabled tests that PoA checks are bypassed when disabled
func TestCheckProofOfAuthority_Disabled(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// By default, both C-Score and identity gating are disabled
	params := f.keeper.GetParams(f.ctx)
	require.False(t, params.EnableCscoreGating)
	require.False(t, params.EnableIdentityGating)

	// Should allow any contribution type regardless of C-Score
	err := f.keeper.CheckProofOfAuthority(f.ctx, contributor, "security")
	require.NoError(t, err, "should allow when gating disabled")
}

// TestCheckMinimumCScore_NoRequirement tests C-Score check when no requirement exists
func TestCheckMinimumCScore_NoRequirement(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Enable C-Score gating but don't set any requirements
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = make(map[string]math.Int)
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Should allow contribution when no requirement exists
	err := f.keeper.CheckMinimumCScore(f.ctx, contributor, "code")
	require.NoError(t, err, "should allow when no requirement exists")
}

// TestCheckMinimumCScore_SufficientCScore tests successful C-Score validation
func TestCheckMinimumCScore_SufficientCScore(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Give contributor 5000 C-Score
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(5000))
	require.NoError(t, err)

	// Enable C-Score gating and set requirement of 1000 for "code"
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(1000),
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Should allow contribution (5000 >= 1000)
	err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "code")
	require.NoError(t, err, "should allow when C-Score sufficient")
}

// TestCheckMinimumCScore_InsufficientCScore tests C-Score rejection
func TestCheckMinimumCScore_InsufficientCScore(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Give contributor only 500 C-Score
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(500))
	require.NoError(t, err)

	// Enable C-Score gating and set requirement of 1000 for "code"
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(1000),
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Should reject contribution (500 < 1000)
	err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "code")
	require.Error(t, err, "should reject when C-Score insufficient")
	require.ErrorIs(t, err, types.ErrInsufficientCScore)
	require.Contains(t, err.Error(), "need 500 more")
}

// TestCheckMinimumCScore_ExactRequirement tests edge case of exact C-Score match
func TestCheckMinimumCScore_ExactRequirement(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Give contributor exactly 1000 C-Score
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(1000))
	require.NoError(t, err)

	// Enable C-Score gating and set requirement of 1000 for "code"
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(1000),
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Should allow contribution (1000 >= 1000)
	err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "code")
	require.NoError(t, err, "should allow when C-Score exactly matches requirement")
}

// TestGetRequiredCScore tests C-Score requirement lookup
func TestGetRequiredCScore(t *testing.T) {
	f := SetupKeeperTest(t)

	// Test when gating disabled
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = false
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	score, hasReq := f.keeper.GetRequiredCScore(f.ctx, "code")
	require.False(t, hasReq, "should have no requirement when gating disabled")
	require.True(t, score.IsZero())

	// Test when gating enabled but no requirement for type
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"governance": math.NewInt(10000),
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	score, hasReq = f.keeper.GetRequiredCScore(f.ctx, "code")
	require.False(t, hasReq, "should have no requirement for unlisted type")

	// Test when requirement exists
	score, hasReq = f.keeper.GetRequiredCScore(f.ctx, "governance")
	require.True(t, hasReq, "should have requirement for listed type")
	require.Equal(t, math.NewInt(10000), score)
}

// TestIsExemptAddress tests address exemption logic
func TestIsExemptAddress(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	addr1 := addrs[0]
	addr2 := addrs[1]

	// Initially no exempt addresses
	require.False(t, f.keeper.IsExemptAddress(f.ctx, addr1))

	// Add addr1 to exempt list
	params := f.keeper.GetParams(f.ctx)
	params.ExemptAddresses = []string{addr1.String()}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// addr1 should be exempt, addr2 should not
	require.True(t, f.keeper.IsExemptAddress(f.ctx, addr1))
	require.False(t, f.keeper.IsExemptAddress(f.ctx, addr2))
}

// TestCheckProofOfAuthority_ExemptAddress tests that exempt addresses bypass all checks
func TestCheckProofOfAuthority_ExemptAddress(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Enable both C-Score and identity gating with strict requirements
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(100000), // Very high requirement
	}
	params.EnableIdentityGating = true
	params.RequireIdentityForCtype = map[string]bool{
		"code": true,
	}
	params.ExemptAddresses = []string{contributor.String()}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Even with 0 C-Score and no identity, should allow (exempt)
	err := f.keeper.CheckProofOfAuthority(f.ctx, contributor, "code")
	require.NoError(t, err, "exempt address should bypass all checks")
}

// TestCheckIdentityRequirement_IdentityModuleUnavailable tests fail-safe behavior
func TestCheckIdentityRequirement_IdentityModuleUnavailable(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Enable identity gating and require identity for "treasury"
	params := f.keeper.GetParams(f.ctx)
	params.EnableIdentityGating = true
	params.RequireIdentityForCtype = map[string]bool{
		"treasury": true,
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Since identity keeper is nil, should fail-safe and reject
	err := f.keeper.CheckIdentityRequirement(f.ctx, contributor, "treasury")
	require.Error(t, err, "should reject when identity required but module unavailable")
	require.ErrorIs(t, err, types.ErrIdentityCheckFailed)
	require.Contains(t, err.Error(), "identity module is not available")
}

// TestCanSubmitContribution_ReadOnlyCheck tests the read-only submission check
func TestCanSubmitContribution_ReadOnlyCheck(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Test 1: No restrictions
	canSubmit, reason := f.keeper.CanSubmitContribution(f.ctx, contributor, "code")
	require.True(t, canSubmit)
	require.Contains(t, reason, "all requirements met")

	// Test 2: Insufficient C-Score
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(1000),
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	canSubmit, reason = f.keeper.CanSubmitContribution(f.ctx, contributor, "code")
	require.False(t, canSubmit)
	require.Contains(t, reason, "insufficient C-Score")
	require.Contains(t, reason, "need 1000 more")

	// Test 3: Give sufficient C-Score
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(2000))
	require.NoError(t, err)

	canSubmit, reason = f.keeper.CanSubmitContribution(f.ctx, contributor, "code")
	require.True(t, canSubmit)
	require.Contains(t, reason, "all requirements met")

	// Test 4: Identity required but not available
	params.EnableIdentityGating = true
	params.RequireIdentityForCtype = map[string]bool{
		"code": true,
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	canSubmit, reason = f.keeper.CanSubmitContribution(f.ctx, contributor, "code")
	require.False(t, canSubmit)
	require.Contains(t, reason, "verified identity")
}

// TestGetCScoreRequirements tests the query helper function
func TestGetCScoreRequirements(t *testing.T) {
	f := SetupKeeperTest(t)

	// Test when gating disabled
	reqs := f.keeper.GetCScoreRequirements(f.ctx)
	require.Empty(t, reqs)

	// Test when gating enabled with requirements
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code":       math.NewInt(1000),
		"governance": math.NewInt(10000),
		"security":   math.NewInt(100000),
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reqs = f.keeper.GetCScoreRequirements(f.ctx)
	require.Len(t, reqs, 3)
	require.Equal(t, math.NewInt(1000), reqs["code"])
	require.Equal(t, math.NewInt(10000), reqs["governance"])
	require.Equal(t, math.NewInt(100000), reqs["security"])
}

// TestGetIdentityRequirements tests the query helper function
func TestGetIdentityRequirements(t *testing.T) {
	f := SetupKeeperTest(t)

	// Test when gating disabled
	reqs := f.keeper.GetIdentityRequirements(f.ctx)
	require.Empty(t, reqs)

	// Test when gating enabled with requirements
	params := f.keeper.GetParams(f.ctx)
	params.EnableIdentityGating = true
	params.RequireIdentityForCtype = map[string]bool{
		"treasury": true,
		"upgrade":  true,
		"code":     false,
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	reqs = f.keeper.GetIdentityRequirements(f.ctx)
	require.Len(t, reqs, 3)
	require.True(t, reqs["treasury"])
	require.True(t, reqs["upgrade"])
	require.False(t, reqs["code"])
}

// TestLayeredAccessControl tests the complete 5-tier system
func TestLayeredAccessControl(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Setup layered access control
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code":       math.NewInt(1000),   // Level 1: Bronze
		"governance": math.NewInt(10000),  // Level 2: Silver
		"security":   math.NewInt(100000), // Level 3: Gold
	}
	params.EnableIdentityGating = true
	params.RequireIdentityForCtype = map[string]bool{
		"treasury": true, // Level 4: Verified Identity
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Level 0: Public (no C-Score, no identity) - "data" type
	canSubmit, _ := f.keeper.CanSubmitContribution(f.ctx, contributor, "data")
	require.True(t, canSubmit, "Level 0: should allow public contributions")

	// Level 1: Bronze (1000 C-Score) - "code" type
	canSubmit, _ = f.keeper.CanSubmitContribution(f.ctx, contributor, "code")
	require.False(t, canSubmit, "Level 1: should reject without Bronze tier")

	// Grant Bronze tier
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(1500))
	require.NoError(t, err)

	canSubmit, _ = f.keeper.CanSubmitContribution(f.ctx, contributor, "code")
	require.True(t, canSubmit, "Level 1: should allow with Bronze tier")

	// Level 2: Silver (10000 C-Score) - "governance" type
	canSubmit, _ = f.keeper.CanSubmitContribution(f.ctx, contributor, "governance")
	require.False(t, canSubmit, "Level 2: should reject without Silver tier")

	// Grant Silver tier
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(10000))
	require.NoError(t, err)

	canSubmit, _ = f.keeper.CanSubmitContribution(f.ctx, contributor, "governance")
	require.True(t, canSubmit, "Level 2: should allow with Silver tier")

	// Level 3: Gold (100000 C-Score) - "security" type
	canSubmit, _ = f.keeper.CanSubmitContribution(f.ctx, contributor, "security")
	require.False(t, canSubmit, "Level 3: should reject without Gold tier")

	// Grant Gold tier
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(90000))
	require.NoError(t, err)

	canSubmit, _ = f.keeper.CanSubmitContribution(f.ctx, contributor, "security")
	require.True(t, canSubmit, "Level 3: should allow with Gold tier")

	// Level 4: Verified Identity - "treasury" type
	// Should fail since identity module not available
	canSubmit, _ = f.keeper.CanSubmitContribution(f.ctx, contributor, "treasury")
	require.False(t, canSubmit, "Level 4: should reject without identity verification")
}

// TestMultipleContributionTypes tests different C-Score requirements per type
func TestMultipleContributionTypes(t *testing.T) {
	f := SetupKeeperTest(t)
	addrs := createTestAddresses(5)

	contributor := addrs[0]

	// Setup different requirements for different types
	params := f.keeper.GetParams(f.ctx)
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"data":       math.NewInt(0),      // Free
		"code":       math.NewInt(1000),   // Bronze
		"governance": math.NewInt(10000),  // Silver
		"security":   math.NewInt(100000), // Gold
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Contributor with 5000 C-Score
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(5000))
	require.NoError(t, err)

	// Should allow data (0 required)
	err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "data")
	require.NoError(t, err)

	// Should allow code (1000 required, has 5000)
	err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "code")
	require.NoError(t, err)

	// Should reject governance (10000 required, has 5000)
	err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "governance")
	require.Error(t, err)

	// Should reject security (100000 required, has 5000)
	err = f.keeper.CheckMinimumCScore(f.ctx, contributor, "security")
	require.Error(t, err)
}

// TestParamValidation_AccessControl tests parameter validation for access control
func TestParamValidation_AccessControl(t *testing.T) {
	tests := []struct {
		name      string
		params    types.Params
		expectErr bool
	}{
		{
			name: "valid - no access control",
			params: func() types.Params {
				p := types.DefaultParams()
				return p
			}(),
			expectErr: false,
		},
		{
			name: "valid - C-Score requirements",
			params: func() types.Params {
				p := types.DefaultParams()
				p.EnableCscoreGating = true
				p.MinCscoreForCtype = map[string]math.Int{
					"code": math.NewInt(1000),
				}
				return p
			}(),
			expectErr: false,
		},
		{
			name: "invalid - negative C-Score requirement",
			params: func() types.Params {
				p := types.DefaultParams()
				p.EnableCscoreGating = true
				p.MinCscoreForCtype = map[string]math.Int{
					"code": math.NewInt(-1000),
				}
				return p
			}(),
			expectErr: true,
		},
		{
			name: "invalid - empty contribution type in C-Score map",
			params: func() types.Params {
				p := types.DefaultParams()
				p.EnableCscoreGating = true
				p.MinCscoreForCtype = map[string]math.Int{
					"": math.NewInt(1000),
				}
				return p
			}(),
			expectErr: true,
		},
		{
			name: "valid - exempt addresses",
			params: func() types.Params {
				p := types.DefaultParams()
				p.ExemptAddresses = []string{
					sdk.AccAddress([]byte("addr1_______________")).String(),
				}
				return p
			}(),
			expectErr: false,
		},
		{
			name: "invalid - duplicate exempt addresses",
			params: func() types.Params {
				p := types.DefaultParams()
				addr := sdk.AccAddress([]byte("addr1_______________")).String()
				p.ExemptAddresses = []string{addr, addr}
				return p
			}(),
			expectErr: true,
		},
		{
			name: "invalid - empty exempt address",
			params: func() types.Params {
				p := types.DefaultParams()
				p.ExemptAddresses = []string{""}
				return p
			}(),
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
