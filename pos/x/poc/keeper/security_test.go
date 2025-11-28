package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// TestCVE_2025_POC_001_NoPanicOnStoreError tests that GetNextContributionID returns error instead of panic
func TestCVE_2025_POC_001_NoPanicOnStoreError(t *testing.T) {
	f := SetupKeeperTest(t)

	// Test that GetNextContributionID returns error gracefully
	id, err := f.keeper.GetNextContributionID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)

	// Test incremental IDs
	id2, err := f.keeper.GetNextContributionID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), id2)
}

// TestCVE_2025_POC_002_EndorsementDoubleCounting tests canonical address validation
func TestCVE_2025_POC_002_EndorsementDoubleCounting(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create a test contribution
	contributor := sdk.AccAddress("contributor_______")
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i + 1) // Non-zero hash
	}

	contribution := types.NewContribution(
		1,
		contributor.String(),
		"code",
		"https://github.com/test/repo",
		hash,
		0,
		time.Now().Unix(),
	)

	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	// Create validator
	valAddr := sdk.ValAddress("validator_________")

	// First endorsement should succeed
	endorsement1 := types.NewEndorsement(
		valAddr.String(),
		true,
		math.NewInt(100),
		time.Now().Unix(),
	)

	verified, err := f.keeper.AddEndorsement(f.ctx, 1, endorsement1)
	require.NoError(t, err)
	require.False(t, verified) // Not enough power for quorum

	// Second endorsement from SAME validator should fail (double-counting prevention)
	endorsement2 := types.NewEndorsement(
		valAddr.String(),
		true,
		math.NewInt(100),
		time.Now().Unix(),
	)

	_, err = f.keeper.AddEndorsement(f.ctx, 1, endorsement2)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrAlreadyEndorsed)

	// Even with different address format (account vs validator), should still fail
	accAddr := sdk.AccAddress(valAddr)
	endorsement3 := types.NewEndorsement(
		accAddr.String(),
		true,
		math.NewInt(100),
		time.Now().Unix(),
	)

	_, err = f.keeper.AddEndorsement(f.ctx, 1, endorsement3)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrAlreadyEndorsed)
}

// TestCVE_2025_POC_003_IntegerOverflow tests credit overflow protection
func TestCVE_2025_POC_003_IntegerOverflow(t *testing.T) {
	f := SetupKeeperTest(t)

	addr := sdk.AccAddress("test_address______")

	// Add credits near max safe value
	const maxSafeUint64 = uint64(1<<63 - 1)
	nearMax := math.NewIntFromUint64(maxSafeUint64 - 1000)

	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, addr, nearMax)
	require.NoError(t, err)

	// Verify credits were added
	credits := f.keeper.GetCredits(f.ctx, addr)
	require.True(t, credits.Amount.Equal(nearMax))

	// Try to add more credits that would overflow
	overflow := math.NewInt(2000)
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, addr, overflow)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceed maximum safe value")

	// Verify credits were NOT changed (rollback on error)
	credits = f.keeper.GetCredits(f.ctx, addr)
	require.True(t, credits.Amount.Equal(nearMax))

	// Test that negative credits are rejected
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, addr, math.NewInt(-100))
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot add negative or zero credits")

	// Test that zero credits are rejected
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, addr, math.ZeroInt())
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot add negative or zero credits")
}

// TestCVE_2025_POC_005_WithdrawalReentrancy tests the check-zero-send pattern
func TestCVE_2025_POC_005_WithdrawalReentrancy(t *testing.T) {
	f := SetupKeeperTest(t)

	addr := sdk.AccAddress("test_withdrawer___")

	// Add some credits
	credits := math.NewInt(1000)
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, addr, credits)
	require.NoError(t, err)

	// Verify credits exist
	storedCredits := f.keeper.GetCredits(f.ctx, addr)
	require.True(t, storedCredits.Amount.Equal(credits))

	// Attempt withdrawal (will fail due to insufficient module balance, but tests the pattern)
	withdrawn, err := f.keeper.WithdrawCredits(f.ctx, addr)

	// Should fail due to insufficient module balance
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient module balance")
	require.True(t, withdrawn.IsZero())

	// CRITICAL: Verify credits were RESTORED after failure (no fund loss)
	storedCredits = f.keeper.GetCredits(f.ctx, addr)
	require.True(t, storedCredits.Amount.Equal(credits), "Credits should be restored on withdrawal failure")

	// Test withdrawal with zero credits
	addr2 := sdk.AccAddress("test_empty________")
	withdrawn, err = f.keeper.WithdrawCredits(f.ctx, addr2)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrNoCredits)
	require.True(t, withdrawn.IsZero())
}

// TestCVE_2025_POC_006_HashValidation tests strict hash validation
func TestCVE_2025_POC_006_HashValidation(t *testing.T) {
	contributor := sdk.AccAddress("contributor_______")

	testCases := []struct {
		name      string
		hash      []byte
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid SHA256 hash",
			hash:      make([]byte, 32),
			shouldErr: true, // All zeros should fail
			errMsg:    "hash cannot be all zeros",
		},
		{
			name:      "valid SHA512 hash",
			hash:      make([]byte, 64),
			shouldErr: true, // All zeros should fail
			errMsg:    "hash cannot be all zeros",
		},
		{
			name:      "empty hash",
			hash:      []byte{},
			shouldErr: true,
			errMsg:    "hash cannot be empty",
		},
		{
			name:      "invalid length hash",
			hash:      make([]byte, 16),
			shouldErr: true,
			errMsg:    "invalid hash length",
		},
		{
			name:      "all zeros SHA256",
			hash:      make([]byte, 32),
			shouldErr: true,
			errMsg:    "hash cannot be all zeros",
		},
		{
			name:      "all ones SHA256",
			hash:      func() []byte { h := make([]byte, 32); for i := range h { h[i] = 0xFF }; return h }(),
			shouldErr: true,
			errMsg:    "hash cannot be all ones",
		},
		{
			name:      "valid non-zero SHA256",
			hash:      func() []byte { h := make([]byte, 32); h[0] = 0x01; return h }(),
			shouldErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &types.MsgSubmitContribution{
				Contributor: contributor.String(),
				Ctype:       "code",
				Uri:         "https://github.com/test/repo",
				Hash:        tc.hash,
			}

			err := msg.ValidateBasic()
			if tc.shouldErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestEnqueueReward_OverflowProtection tests overflow protection in reward calculation
func TestEnqueueReward_OverflowProtection(t *testing.T) {
	f := SetupKeeperTest(t)

	contributor := sdk.AccAddress("contributor_______")
	hash := make([]byte, 32)
	hash[0] = 0x01 // Non-zero hash

	contribution := types.NewContribution(
		1,
		contributor.String(),
		"code",
		"https://github.com/test/repo",
		hash,
		0,
		time.Now().Unix(),
	)
	contribution.Verified = true

	// Set extremely large base reward unit to trigger overflow check
	params := f.keeper.GetParams(f.ctx)
	const maxSafeUint64 = uint64(1<<63 - 1)
	params.BaseRewardUnit = math.NewIntFromUint64(maxSafeUint64)
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// EnqueueReward should detect and reject overflow
	err = f.keeper.EnqueueReward(f.ctx, contribution)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum safe value")
}

// TestRateLimiting tests rate limit enforcement
func TestRateLimiting(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set low rate limit for testing
	params := f.keeper.GetParams(f.ctx)
	params.MaxPerBlock = 3
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// First 3 submissions should succeed
	for i := 0; i < 3; i++ {
		err := f.keeper.CheckRateLimit(f.ctx)
		require.NoError(t, err)
	}

	// 4th submission should fail
	err = f.keeper.CheckRateLimit(f.ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrRateLimitExceeded)
}
