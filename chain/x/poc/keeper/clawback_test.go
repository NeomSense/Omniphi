package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

func TestClawback_VestingClawback(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("clawcontrib1________").String()
	authority := sdk.AccAddress("gov_________________").String()

	f.bankKeeper.setBalance(
		sdk.AccAddress("module_address______").String(),
		"omniphi",
		math.NewInt(1000000),
	)

	// Create contribution
	contrib := types.NewContribution(1, contributor, "code", "ipfs://test", []byte{1}, 10, 1000)
	err := f.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)

	// Create vesting
	schedule := types.VestingSchedule{
		Contributor:    contributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(8000),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     1,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}
	err = f.keeper.CreateVestingSchedule(ctx, schedule)
	require.NoError(t, err)

	// Execute clawback
	err = f.keeper.ExecuteClawback(ctx, 1, "plagiarism detected", authority)
	require.NoError(t, err)

	// Verify clawback record
	record, found := f.keeper.GetClawbackRecord(ctx, 1)
	require.True(t, found)
	require.Equal(t, contributor, record.Contributor)
	require.Equal(t, "plagiarism detected", record.Reason)
	require.True(t, record.VestingClawedBack.Equal(math.NewInt(8000)))

	// Vesting status should be ClawedBack
	vs, found := f.keeper.GetVestingSchedule(ctx, contributor, 1)
	require.True(t, found)
	require.Equal(t, types.VestingStatusClawedBack, vs.Status)
}

func TestClawback_BalanceClawback(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("clawcontrib2________").String()
	authority := sdk.AccAddress("gov_________________").String()

	// Give contributor a balance
	f.bankKeeper.setBalance(contributor, "omniphi", math.NewInt(500))

	// Create contribution
	contrib := types.NewContribution(2, contributor, "code", "ipfs://test2", []byte{2}, 10, 1000)
	err := f.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)

	// Execute clawback (no vesting, but has balance)
	err = f.keeper.ExecuteClawback(ctx, 2, "fraud", authority)
	require.NoError(t, err)

	record, found := f.keeper.GetClawbackRecord(ctx, 2)
	require.True(t, found)
	// Should have clawed back the minimum of balance (500) and estimated immediate
	require.True(t, record.AmountClawedBack.IsPositive(), "should clawback some balance")
}

func TestClawback_DoubleClawbackPrevention(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("clawcontrib3________").String()
	authority := sdk.AccAddress("gov_________________").String()

	contrib := types.NewContribution(3, contributor, "code", "ipfs://test3", []byte{3}, 10, 1000)
	err := f.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)

	// First clawback succeeds
	err = f.keeper.ExecuteClawback(ctx, 3, "fraud", authority)
	require.NoError(t, err)

	// Second clawback fails
	err = f.keeper.ExecuteClawback(ctx, 3, "fraud again", authority)
	require.ErrorIs(t, err, types.ErrClawbackAlreadyApplied)
}

func TestClawback_StatsUpdate(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx
	contributor := sdk.AccAddress("clawcontrib4________").String()
	authority := sdk.AccAddress("gov_________________").String()

	contrib := types.NewContribution(4, contributor, "code", "ipfs://test4", []byte{4}, 10, 1000)
	err := f.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)

	// Check initial stats
	statsBefore := f.keeper.GetContributorStats(ctx, contributor)
	require.Equal(t, uint64(0), statsBefore.FraudCount)
	require.True(t, statsBefore.ReputationScore.Equal(math.LegacyOneDec()))

	// Execute clawback
	err = f.keeper.ExecuteClawback(ctx, 4, "duplicate content", authority)
	require.NoError(t, err)

	// Check updated stats
	statsAfter := f.keeper.GetContributorStats(ctx, contributor)
	require.Equal(t, uint64(1), statsAfter.FraudCount)
	// Reputation should decay by 25%: 1.0 * 0.75 = 0.75
	require.True(t, statsAfter.ReputationScore.Equal(math.LegacyNewDecWithPrec(75, 2)),
		"expected 0.75, got %s", statsAfter.ReputationScore)
	// Bond multiplier should increase by 0.50
	require.True(t, statsAfter.CurrentBondMultiplier.Equal(math.LegacyNewDecWithPrec(150, 2)),
		"expected 1.50, got %s", statsAfter.CurrentBondMultiplier)
}

func TestClawback_EmptyReasonRejected(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	contrib := types.NewContribution(5, "somecontrib", "code", "ipfs://x", []byte{5}, 10, 1000)
	err := f.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)

	err = f.keeper.ExecuteClawback(ctx, 5, "", "authority")
	require.ErrorIs(t, err, types.ErrInvalidClawbackReason)
}
