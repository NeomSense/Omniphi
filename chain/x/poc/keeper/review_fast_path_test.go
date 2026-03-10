package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

func TestReview_FastPathFinalization(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	// 1. Setup: Enable Human Review
	params := f.keeper.GetParams(ctx)
	params.EnableHumanReview = true
	params.VerifiersPerClaim = 3
	params.ReviewVotePeriod = 100
	params.MinReviewerBond = "1000omniphi"
	err := f.keeper.SetParams(ctx, params)
	require.NoError(t, err)

	// 2. Create Contribution
	contrib := types.NewContribution(1, "contributor", "code", "ipfs://test", []byte{1}, 10, 1000)
	err = f.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)

	// 3. Setup 3 Reviewers with credits and balances
	reviewers := []string{}
	for i := 0; i < 3; i++ {
		addr := sdk.AccAddress([]byte{byte(i), 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
		reviewers = append(reviewers, addr.String())

		// Give credits (reputation)
		err := f.keeper.AddCreditsWithOverflowCheck(ctx, addr, math.NewInt(2000))
		require.NoError(t, err)

		// Give funds for bond
		coins := sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(10000)))
		err = f.bankKeeper.MintCoins(ctx, types.ModuleName, coins)
		require.NoError(t, err)
		err = f.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, coins)
		require.NoError(t, err)
	}

	// 4. Start Review
	// We mock the selection by manually creating the session to ensure our specific reviewers are used
	// (In integration tests we'd rely on SelectReviewers, but here we want control)
	session := types.ReviewSession{
		ContributionID:    1,
		Status:            types.ReviewStatusInReview,
		AssignedReviewers: reviewers,
		StartHeight:       ctx.BlockHeight(),
		EndHeight:         ctx.BlockHeight() + 100,
	}
	err = f.keeper.SetReviewSession(ctx, session)
	require.NoError(t, err)

	// 5. Cast Votes (2 Accept, 1 Reject)
	// Vote 1
	_, err = f.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		ContributionId: 1,
		Reviewer:       reviewers[0],
		Decision:       uint32(types.ReviewVoteAccept),
		QualityScore:   90,
	})
	require.NoError(t, err)

	// Check: Still In Review
	s, _ := f.keeper.GetReviewSession(ctx, 1)
	require.Equal(t, types.ReviewStatusInReview, s.Status)

	// Vote 2
	_, err = f.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		ContributionId: 1,
		Reviewer:       reviewers[1],
		Decision:       uint32(types.ReviewVoteAccept),
		QualityScore:   95,
	})
	require.NoError(t, err)

	// Check: Still In Review
	s, _ = f.keeper.GetReviewSession(ctx, 1)
	require.Equal(t, types.ReviewStatusInReview, s.Status)

	// Vote 3 (Final Vote)
	_, err = f.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		ContributionId: 1,
		Reviewer:       reviewers[2],
		Decision:       uint32(types.ReviewVoteReject),
		QualityScore:   50,
	})
	require.NoError(t, err)

	// 6. Verify Fast Path Finalization
	// The session should now be ACCEPTED immediately, without waiting for EndBlocker
	finalSession, found := f.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusAccepted, finalSession.Status, "Session should be auto-finalized")
	require.Equal(t, types.ReviewVoteAccept, finalSession.FinalDecision)
}
