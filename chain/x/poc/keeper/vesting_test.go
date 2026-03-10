package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// ctxAtHeight returns a new context at the given block height (epoch = height/100).
func ctxAtHeight(ctx sdk.Context, height int64) sdk.Context {
	return ctx.WithBlockHeader(cmtproto.Header{Height: height})
}

func TestVesting_CreateAndRetrieve(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("contrib1____________").String()

	schedule := types.VestingSchedule{
		Contributor:    contributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(10000),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     5,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}

	err := f.keeper.CreateVestingSchedule(ctx, schedule)
	require.NoError(t, err)

	// Retrieve
	got, found := f.keeper.GetVestingSchedule(ctx, contributor, 1)
	require.True(t, found)
	require.Equal(t, schedule.TotalAmount, got.TotalAmount)
	require.Equal(t, schedule.VestingEpochs, got.VestingEpochs)
	require.Equal(t, types.VestingStatusActive, got.Status)

	// Aggregate balance should match
	bal := f.keeper.GetVestingBalance(ctx, contributor)
	require.True(t, bal.Equal(math.NewInt(10000)))
}

func TestVesting_LinearRelease(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("contrib2____________").String()

	// Fund module account so transfers work
	f.bankKeeper.setBalance(
		sdk.AccAddress("module_address______").String(),
		"omniphi",
		math.NewInt(1000000),
	)

	// Create schedule: 10000 over 10 epochs starting at epoch 1
	schedule := types.VestingSchedule{
		Contributor:    contributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(10000),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     1,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}
	err := f.keeper.CreateVestingSchedule(ctx, schedule)
	require.NoError(t, err)

	// Simulate epoch 4 (3 epochs elapsed → 30% released)
	// Epoch = height/100, so epoch 4 = height 400
	ctx4 := ctxAtHeight(ctx, 400)
	err = f.keeper.ProcessVestingReleases(ctx4)
	require.NoError(t, err)

	got, found := f.keeper.GetVestingSchedule(ctx4, contributor, 1)
	require.True(t, found)
	// 3/10 * 10000 = 3000
	require.True(t, got.ReleasedAmount.Equal(math.NewInt(3000)), "expected 3000, got %s", got.ReleasedAmount)
	require.Equal(t, types.VestingStatusActive, got.Status)

	// Check balance received
	recipientBal := f.bankKeeper.getOrInit(contributor, "omniphi")
	require.True(t, recipientBal.Equal(math.NewInt(3000)), "expected 3000, got %s", recipientBal)
}

func TestVesting_FullCompletion(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("contrib3____________").String()

	f.bankKeeper.setBalance(
		sdk.AccAddress("module_address______").String(),
		"omniphi",
		math.NewInt(1000000),
	)

	schedule := types.VestingSchedule{
		Contributor:    contributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(5000),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     1,
		VestingEpochs:  5,
		Status:         types.VestingStatusActive,
	}
	err := f.keeper.CreateVestingSchedule(ctx, schedule)
	require.NoError(t, err)

	// Advance past full vesting period (epoch 10 = height 1000)
	ctx10 := ctxAtHeight(ctx, 1000)
	err = f.keeper.ProcessVestingReleases(ctx10)
	require.NoError(t, err)

	got, found := f.keeper.GetVestingSchedule(ctx10, contributor, 1)
	require.True(t, found)
	require.True(t, got.ReleasedAmount.Equal(math.NewInt(5000)))
	require.Equal(t, types.VestingStatusCompleted, got.Status)
}

func TestVesting_ClawbackCancelsUnvested(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("contrib4____________").String()

	f.bankKeeper.setBalance(
		sdk.AccAddress("module_address______").String(),
		"omniphi",
		math.NewInt(1000000),
	)

	schedule := types.VestingSchedule{
		Contributor:    contributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(10000),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     1,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}
	err := f.keeper.CreateVestingSchedule(ctx, schedule)
	require.NoError(t, err)

	// Release partial (3 epochs elapsed: epoch 4 = height 400)
	ctx4 := ctxAtHeight(ctx, 400)
	err = f.keeper.ProcessVestingReleases(ctx4)
	require.NoError(t, err)

	// Clawback remaining
	unvested, err := f.keeper.ClawbackVesting(ctx4, contributor, 1)
	require.NoError(t, err)
	require.True(t, unvested.Equal(math.NewInt(7000)), "expected 7000 unvested, got %s", unvested)

	// Status should be ClawedBack
	got, found := f.keeper.GetVestingSchedule(ctx, contributor, 1)
	require.True(t, found)
	require.Equal(t, types.VestingStatusClawedBack, got.Status)

	// Subsequent process should skip clawed-back schedule
	ctx20 := ctxAtHeight(ctx, 2000)
	err = f.keeper.ProcessVestingReleases(ctx20)
	require.NoError(t, err)

	got, _ = f.keeper.GetVestingSchedule(ctx20, contributor, 1)
	require.True(t, got.ReleasedAmount.Equal(math.NewInt(3000)), "released amount should not change after clawback")
}

func TestVesting_NotFoundReturnsError(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	_, err := f.keeper.ClawbackVesting(ctx, "nonexistent", 999)
	require.ErrorIs(t, err, types.ErrVestingNotFound)
}
