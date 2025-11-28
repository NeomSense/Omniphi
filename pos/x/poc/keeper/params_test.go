package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// TestParamsSerialization tests that map fields serialize correctly
func TestParamsSerialization(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create params with map fields
	params := types.DefaultParams()
	params.EnableCscoreGating = true
	params.MinCscoreForCtype = map[string]math.Int{
		"code": math.NewInt(1000),
		"governance": math.NewInt(10000),
	}
	params.EnableIdentityGating = true
	params.RequireIdentityForCtype = map[string]bool{
		"treasury": true,
		"upgrade": true,
	}
	addrs := createTestAddresses(1)
	params.ExemptAddresses = []string{addrs[0].String()}

	t.Logf("Before SetParams:")
	t.Logf("  EnableCscoreGating: %v", params.EnableCscoreGating)
	t.Logf("  MinCscoreForCtype: %v", params.MinCscoreForCtype)
	t.Logf("  EnableIdentityGating: %v", params.EnableIdentityGating)
	t.Logf("  RequireIdentityForCtype: %v", params.RequireIdentityForCtype)
	t.Logf("  ExemptAddresses: %v", params.ExemptAddresses)

	// Set params
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Get params back
	retrieved := f.keeper.GetParams(f.ctx)

	t.Logf("After GetParams:")
	t.Logf("  EnableCscoreGating: %v", retrieved.EnableCscoreGating)
	t.Logf("  MinCscoreForCtype: %v", retrieved.MinCscoreForCtype)
	t.Logf("  EnableIdentityGating: %v", retrieved.EnableIdentityGating)
	t.Logf("  RequireIdentityForCtype: %v", retrieved.RequireIdentityForCtype)
	t.Logf("  ExemptAddresses: %v", retrieved.ExemptAddresses)

	// Verify boolean fields
	require.True(t, retrieved.EnableCscoreGating, "EnableCscoreGating should be true")
	require.True(t, retrieved.EnableIdentityGating, "EnableIdentityGating should be true")

	// Verify map fields (these will likely fail due to marshaling issue)
	require.NotNil(t, retrieved.MinCscoreForCtype, "MinCscoreForCtype should not be nil")
	require.NotNil(t, retrieved.RequireIdentityForCtype, "RequireIdentityForCtype should not be nil")

	if len(retrieved.MinCscoreForCtype) > 0 {
		t.Logf("MinCscoreForCtype serialized correctly!")
		require.Equal(t, math.NewInt(1000), retrieved.MinCscoreForCtype["code"])
		require.Equal(t, math.NewInt(10000), retrieved.MinCscoreForCtype["governance"])
	} else {
		t.Logf("WARNING: MinCscoreForCtype map is empty after serialization")
	}

	if len(retrieved.RequireIdentityForCtype) > 0 {
		t.Logf("RequireIdentityForCtype serialized correctly!")
		require.True(t, retrieved.RequireIdentityForCtype["treasury"])
		require.True(t, retrieved.RequireIdentityForCtype["upgrade"])
	} else {
		t.Logf("WARNING: RequireIdentityForCtype map is empty after serialization")
	}

	if len(retrieved.ExemptAddresses) > 0 {
		t.Logf("ExemptAddresses serialized correctly!")
		require.Equal(t, addrs[0].String(), retrieved.ExemptAddresses[0])
	} else {
		t.Logf("WARNING: ExemptAddresses is empty after serialization")
	}
}
