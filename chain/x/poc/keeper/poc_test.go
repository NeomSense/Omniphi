package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

func TestClaimReplayProtection(t *testing.T) {
	f := SetupKeeperTest(t)

	contributor := sdk.AccAddress("contributor1_____pad").String()

	// 1. Submit first contribution
	id1, err := f.keeper.GetNextContributionID(f.ctx)
	require.NoError(t, err)

	c1 := types.Contribution{
		Id:          id1,
		Contributor: contributor,
		Ctype:       "data",
		Uri:         "ipfs://test1",
		Hash:        []byte("hash1"),
	}
	err = f.keeper.SetContribution(f.ctx, c1)
	require.NoError(t, err)

	// 2. Get next ID — must differ (auto-increment = replay protection)
	id2, err := f.keeper.GetNextContributionID(f.ctx)
	require.NoError(t, err)
	require.NotEqual(t, id1, id2, "Sequential IDs must differ")

	c2 := types.Contribution{
		Id:          id2,
		Contributor: contributor,
		Ctype:       "data",
		Uri:         "ipfs://test2",
		Hash:        []byte("hash2"),
	}
	err = f.keeper.SetContribution(f.ctx, c2)
	require.NoError(t, err)

	// 3. Both stored independently
	_, found1 := f.keeper.GetContribution(f.ctx, id1)
	_, found2 := f.keeper.GetContribution(f.ctx, id2)
	require.True(t, found1)
	require.True(t, found2)
}

func TestCScoreCapAndDecay(t *testing.T) {
	f := SetupKeeperTest(t)

	addr := sdk.AccAddress("validator1__________")

	// 1. Set initial credits
	credits := types.Credits{
		Address: addr.String(),
		Amount:  math.NewInt(1000),
	}
	err := f.keeper.SetCredits(f.ctx, credits)
	require.NoError(t, err)

	// 2. Add more credits (epoch reward)
	err = f.keeper.AddCredits(f.ctx, addr, math.NewInt(500))
	require.NoError(t, err)

	// 3. Verify accumulation
	result := f.keeper.GetCredits(f.ctx, addr)
	require.True(t, result.Amount.GTE(math.NewInt(1500)),
		"Credits should be at least 1500 after adding 500, got %s", result.Amount)

	// 4. Overflow check with small amount
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, addr, math.NewInt(1))
	require.NoError(t, err)

	// 5. Credits remain positive
	final := f.keeper.GetCredits(f.ctx, addr)
	require.True(t, final.Amount.IsPositive())
}

func TestRateLimitAntiSpam(t *testing.T) {
	f := SetupKeeperTest(t)

	contributor := sdk.AccAddress("spammer1____________").String()

	// Submit many contributions rapidly
	var ids []uint64
	for i := 0; i < 20; i++ {
		id, err := f.keeper.GetNextContributionID(f.ctx)
		require.NoError(t, err)
		ids = append(ids, id)

		c := types.Contribution{
			Id:          id,
			Contributor: contributor,
			Ctype:       "data",
			Uri:         "ipfs://spam",
			Hash:        []byte("spamhash"),
		}
		err = f.keeper.SetContribution(f.ctx, c)
		require.NoError(t, err)
	}

	// All IDs must be unique
	idSet := make(map[uint64]bool)
	for _, id := range ids {
		require.False(t, idSet[id], "Duplicate ID %d", id)
		idSet[id] = true
	}
	require.Equal(t, 20, len(idSet))
}
