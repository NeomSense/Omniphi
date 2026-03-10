package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// ============================================================================
// Helper: configure fixture for human review tests.
// Sets EnableHumanReview=true, zero bonds (unless overridden), and seeds
// eligible reviewer credits.
// ============================================================================

func setupReviewFixture(t *testing.T) (*KeeperTestFixture, sdk.Context) {
	t.Helper()
	fixture := SetupKeeperTest(t)

	// Start from DefaultParams to ensure all sidecar fields are properly initialised
	// (proto marshal/unmarshal drops the sidecar-only JSON fields like RoyaltyShare).
	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	params.BaseRewardUnit = math.NewInt(100)
	params.MaxPerBlock = 100

	// Human Review layer overrides
	params.EnableHumanReview = true
	params.VerifiersPerClaim = 3
	params.ReviewVotePeriod = 100
	params.MinReviewerReputation = 1000
	params.MinReviewerBond = "0omniphi"
	params.ReviewQuorumPct = 67
	params.AppealBond = "0omniphi"
	params.AppealVotePeriod = 2400
	params.CollusionThresholdBps = 9000

	err := fixture.keeper.SetParams(fixture.ctx, params)
	require.NoError(t, err)

	// Use a context with block height and header hash (needed by SelectReviewers).
	ctx := fixture.ctx.WithBlockHeight(100).WithHeaderHash([]byte("deterministic_hash__value_32byte"))
	return fixture, ctx
}

// freshReviewParams returns a full Params object built from DefaultParams with
// the review-layer overrides applied.  This avoids the nil-sidecar-field problem
// that occurs when GetParams round-trips through proto (RoyaltyShare etc. are
// sidecar-only JSON fields that get lost in proto marshal/unmarshal).
func freshReviewParams() types.Params {
	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	params.BaseRewardUnit = math.NewInt(100)
	params.MaxPerBlock = 100
	params.EnableHumanReview = true
	params.VerifiersPerClaim = 3
	params.ReviewVotePeriod = 100
	params.MinReviewerReputation = 1000
	params.MinReviewerBond = "0omniphi"
	params.ReviewQuorumPct = 67
	params.AppealBond = "0omniphi"
	params.AppealVotePeriod = 2400
	params.CollusionThresholdBps = 9000
	return params
}

// Address helpers — all 20-byte padded so sdk.AccAddress(x).String() is valid bech32.
var (
	addrContributor = sdk.AccAddress("contributor_________").String()
	addrReviewer1   = sdk.AccAddress("reviewer1___________").String()
	addrReviewer2   = sdk.AccAddress("reviewer2___________").String()
	addrReviewer3   = sdk.AccAddress("reviewer3___________").String()
	addrReviewer4   = sdk.AccAddress("reviewer4___________").String()
	addrReviewer5   = sdk.AccAddress("reviewer5___________").String()
	addrAuthority   = sdk.AccAddress("gov_________________").String()
	addrAppellant   = sdk.AccAddress("appellant___________").String()
)

// seedReviewers stores enough Credits entries so that SelectReviewers can
// build an eligible set.  n addresses are seeded starting from reviewer1.
func seedReviewers(t *testing.T, fixture *KeeperTestFixture, ctx sdk.Context, addrs []string, amount int64) {
	t.Helper()
	for _, addr := range addrs {
		err := fixture.keeper.SetCredits(ctx, types.Credits{
			Address: addr,
			Amount:  math.NewInt(amount),
		})
		require.NoError(t, err)
	}
}

// seedContribution creates a contribution in the store.
func seedContribution(t *testing.T, fixture *KeeperTestFixture, ctx sdk.Context, id uint64, contributor string) {
	t.Helper()
	contrib := types.Contribution{
		Id:          id,
		Contributor: contributor,
		Ctype:       "code",
		Uri:         "ipfs://test",
	}
	err := fixture.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)
}

// ============================================================================
// A. CRUD Tests
// ============================================================================

func TestReviewSessionCRUD_Roundtrip(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	session := types.ReviewSession{
		ContributionID:    42,
		Status:            types.ReviewStatusInReview,
		AssignedReviewers: []string{addrReviewer1, addrReviewer2},
		Votes:             []types.ReviewVote{},
		StartHeight:       100,
		EndHeight:         200,
		RandomSeed:        []byte("seed_bytes"),
	}

	// Set
	err := fixture.keeper.SetReviewSession(ctx, session)
	require.NoError(t, err)

	// Get
	got, found := fixture.keeper.GetReviewSession(ctx, 42)
	require.True(t, found)
	require.Equal(t, uint64(42), got.ContributionID)
	require.Equal(t, types.ReviewStatusInReview, got.Status)
	require.Equal(t, 2, len(got.AssignedReviewers))
	require.Equal(t, int64(100), got.StartHeight)
	require.Equal(t, int64(200), got.EndHeight)
	require.Equal(t, []byte("seed_bytes"), got.RandomSeed)

	// Not found for different ID
	_, found = fixture.keeper.GetReviewSession(ctx, 99)
	require.False(t, found)

	// Delete
	err = fixture.keeper.DeleteReviewSession(ctx, 42)
	require.NoError(t, err)

	_, found = fixture.keeper.GetReviewSession(ctx, 42)
	require.False(t, found)
}

func TestReviewerProfileCRUD_Roundtrip(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	profile := types.ReviewerProfile{
		Address:          addrReviewer1,
		TotalReviews:     10,
		AcceptedReviews:  7,
		RejectedReviews:  3,
		AvgQualityScore:  82,
		ReputationScore:  5000,
		LastReviewHeight: 99,
		BondedAmount:     "1000omniphi",
		SlashedCount:     1,
		Suspended:        false,
	}

	err := fixture.keeper.SetReviewerProfile(ctx, profile)
	require.NoError(t, err)

	got, found := fixture.keeper.GetReviewerProfile(ctx, addrReviewer1)
	require.True(t, found)
	require.Equal(t, addrReviewer1, got.Address)
	require.Equal(t, uint64(10), got.TotalReviews)
	require.Equal(t, uint64(7), got.AcceptedReviews)
	require.Equal(t, uint64(3), got.RejectedReviews)
	require.Equal(t, uint32(82), got.AvgQualityScore)
	require.Equal(t, uint64(5000), got.ReputationScore)
	require.Equal(t, int64(99), got.LastReviewHeight)
	require.Equal(t, "1000omniphi", got.BondedAmount)
	require.Equal(t, uint64(1), got.SlashedCount)
	require.False(t, got.Suspended)

	// Not found for different address
	_, found = fixture.keeper.GetReviewerProfile(ctx, addrReviewer2)
	require.False(t, found)
}

func TestReviewAppealCRUD_Roundtrip(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	appeal := types.ReviewAppeal{
		AppealID:         1,
		ContributionID:   42,
		Appellant:        addrAppellant,
		Reason:           "unfair rejection",
		AppealBond:       "50000omniphi",
		FiledAtHeight:    200,
		ResolvedAtHeight: 0,
		Resolved:         false,
		Upheld:           false,
		ResolverNotes:    "",
	}

	err := fixture.keeper.SetReviewAppeal(ctx, appeal)
	require.NoError(t, err)

	got, found := fixture.keeper.GetReviewAppeal(ctx, 1)
	require.True(t, found)
	require.Equal(t, uint64(1), got.AppealID)
	require.Equal(t, uint64(42), got.ContributionID)
	require.Equal(t, addrAppellant, got.Appellant)
	require.Equal(t, "unfair rejection", got.Reason)
	require.Equal(t, "50000omniphi", got.AppealBond)
	require.Equal(t, int64(200), got.FiledAtHeight)
	require.False(t, got.Resolved)

	// Not found
	_, found = fixture.keeper.GetReviewAppeal(ctx, 999)
	require.False(t, found)
}

func TestCoVotingRecordCRUD_Roundtrip(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	// Store with addrB > addrA lexicographically — key function should canonicalize.
	record := types.CoVotingRecord{
		ReviewerA:      addrReviewer1,
		ReviewerB:      addrReviewer2,
		SameVoteCount:  3,
		TotalPairCount: 5,
		LastUpdated:    100,
	}

	err := fixture.keeper.SetCoVotingRecord(ctx, record)
	require.NoError(t, err)

	// Retrieve with same order
	got, found := fixture.keeper.GetCoVotingRecord(ctx, addrReviewer1, addrReviewer2)
	require.True(t, found)
	require.Equal(t, uint64(3), got.SameVoteCount)
	require.Equal(t, uint64(5), got.TotalPairCount)
	require.Equal(t, int64(100), got.LastUpdated)

	// Retrieve with reversed order — the key function canonicalizes, so this should find the same record.
	got2, found2 := fixture.keeper.GetCoVotingRecord(ctx, addrReviewer2, addrReviewer1)
	require.True(t, found2)
	require.Equal(t, got.SameVoteCount, got2.SameVoteCount)
	require.Equal(t, got.TotalPairCount, got2.TotalPairCount)
}

func TestNextAppealID_Increment(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	// Initial value is 1 (see review.go: GetNextAppealID returns 1 when not set)
	id := fixture.keeper.GetNextAppealID(ctx)
	require.Equal(t, uint64(1), id)

	// SetReviewAppeal does NOT auto-increment the counter
	appeal := types.ReviewAppeal{
		AppealID:       5,
		ContributionID: 1,
		Appellant:      addrAppellant,
	}
	err := fixture.keeper.SetReviewAppeal(ctx, appeal)
	require.NoError(t, err)

	// Counter unchanged
	id = fixture.keeper.GetNextAppealID(ctx)
	require.Equal(t, uint64(1), id)
}

// ============================================================================
// B. Reviewer Selection Tests
// ============================================================================

func TestSelectReviewers_Determinism(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3, addrReviewer4, addrReviewer5}
	seedReviewers(t, fixture, ctx, reviewers, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// Two calls with the same context (same block hash) should produce the same result.
	selected1, seed1, err := fixture.keeper.SelectReviewers(ctx, 1, addrContributor, 3)
	require.NoError(t, err)
	require.Len(t, selected1, 3)

	selected2, seed2, err := fixture.keeper.SelectReviewers(ctx, 1, addrContributor, 3)
	require.NoError(t, err)
	require.Equal(t, selected1, selected2)
	require.Equal(t, seed1, seed2)
}

func TestSelectReviewers_NoSelfReview(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	// Seed the contributor with high credits so they would otherwise be eligible.
	allAddrs := []string{addrContributor, addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, allAddrs, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	selected, _, err := fixture.keeper.SelectReviewers(ctx, 1, addrContributor, 3)
	require.NoError(t, err)
	require.Len(t, selected, 3)

	// Contributor must not appear in selected reviewers.
	for _, r := range selected {
		require.NotEqual(t, addrContributor, r, "contributor should not be selected as reviewer")
	}
}

func TestSelectReviewers_InsufficientEligible(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	// Only 2 eligible reviewers when we need 3
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	_, _, err := fixture.keeper.SelectReviewers(ctx, 1, addrContributor, 3)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInsufficientEligibleReviewers)
}

func TestSelectReviewers_DifferentSeeds(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3, addrReviewer4, addrReviewer5}
	seedReviewers(t, fixture, ctx, reviewers, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// Context with different header hash
	ctx2 := fixture.ctx.WithBlockHeight(100).WithHeaderHash([]byte("another_hash_value__that_is_32b!"))

	selected1, _, err := fixture.keeper.SelectReviewers(ctx, 1, addrContributor, 3)
	require.NoError(t, err)

	selected2, _, err := fixture.keeper.SelectReviewers(ctx2, 1, addrContributor, 3)
	require.NoError(t, err)

	// With different seeds, the selected set should differ (with very high probability
	// given 5 eligible candidates and 3 slots).
	differ := false
	for i := range selected1 {
		if selected1[i] != selected2[i] {
			differ = true
			break
		}
	}
	require.True(t, differ, "different header hashes should produce different reviewer sets")
}

func TestSelectReviewers_ExcludeSuspended(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	// 4 reviewers, but one is suspended — need 3
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3, addrReviewer4}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// Suspend reviewer1
	err := fixture.keeper.SetReviewerProfile(ctx, types.ReviewerProfile{
		Address:   addrReviewer1,
		Suspended: true,
	})
	require.NoError(t, err)

	selected, _, err := fixture.keeper.SelectReviewers(ctx, 1, addrContributor, 3)
	require.NoError(t, err)
	require.Len(t, selected, 3)

	// Suspended reviewer must not appear.
	for _, r := range selected {
		require.NotEqual(t, addrReviewer1, r, "suspended reviewer should not be selected")
	}
}

// ============================================================================
// C. State Machine Tests
// ============================================================================

func TestStartReview_HappyPath(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.ReviewersAssigned, 3)

	// Verify session was created
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusInReview, session.Status)
	require.Equal(t, int64(100), session.StartHeight)
	require.Equal(t, int64(200), session.EndHeight) // 100 + 100 vote period
	require.Len(t, session.AssignedReviewers, 3)
	require.NotEmpty(t, session.RandomSeed)
}

func TestStartReview_DisabledReturnsError(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	// Disable human review — rebuild params from defaults to avoid nil sidecar fields
	params := freshReviewParams()
	params.EnableHumanReview = false
	err := fixture.keeper.SetParams(ctx, params)
	require.NoError(t, err)

	seedContribution(t, fixture, ctx, 1, addrContributor)

	_, err = fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrReviewDisabled)
}

func TestStartReview_ContributionNotFound(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)

	_, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 999,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrContributionNotFound)
}

func TestStartReview_DoubleStart(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// First start succeeds
	_, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// Second start fails
	_, err = fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrReviewNotActive)
}

func TestCastVote_HappyPath(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)
	reviewer := resp.ReviewersAssigned[0]

	_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		Reviewer:            reviewer,
		ContributionId:      1,
		Decision:            uint32(types.ReviewVoteAccept),
		OriginalityOverride: uint32(types.OverrideKeepAI),
		QualityScore:        85,
		NotesPointer:        "ipfs://notes",
	})
	require.NoError(t, err)

	// Verify vote was recorded
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Len(t, session.Votes, 1)
	require.Equal(t, reviewer, session.Votes[0].Reviewer)
	require.Equal(t, types.ReviewVoteAccept, session.Votes[0].Decision)
	require.Equal(t, uint32(85), session.Votes[0].QualityScore)
}

func TestCastVote_NotAssigned(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	_, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// A random unassigned address tries to vote
	unassigned := sdk.AccAddress("unassigned__________").String()
	_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		Reviewer:       unassigned,
		ContributionId: 1,
		Decision:       uint32(types.ReviewVoteAccept),
		QualityScore:   80,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrNotAssignedReviewer)
}

func TestCastVote_AlreadyVoted(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)
	reviewer := resp.ReviewersAssigned[0]

	// First vote succeeds
	_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		Reviewer:       reviewer,
		ContributionId: 1,
		Decision:       uint32(types.ReviewVoteAccept),
		QualityScore:   80,
	})
	require.NoError(t, err)

	// Second vote from the same reviewer fails
	_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		Reviewer:       reviewer,
		ContributionId: 1,
		Decision:       uint32(types.ReviewVoteReject),
		QualityScore:   50,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrReviewAlreadyVoted)
}

func TestCastVote_PeriodExpired(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)
	reviewer := resp.ReviewersAssigned[0]

	// Advance past end height (StartHeight=100, EndHeight=200, so height 201 is past)
	expiredCtx := ctx.WithBlockHeight(201)

	_, err = fixture.keeper.ProcessCastReviewVote(expiredCtx, &types.MsgCastReviewVote{
		Reviewer:       reviewer,
		ContributionId: 1,
		Decision:       uint32(types.ReviewVoteAccept),
		QualityScore:   80,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrReviewPeriodExpired)
}

func TestFinalizeReview_HappyPath_Accepted(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// All 3 reviewers vote ACCEPT
	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → session is auto-finalized immediately
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusAccepted, session.Status)
	require.Equal(t, types.ReviewVoteAccept, session.FinalDecision)
	require.Equal(t, uint32(80), session.FinalQuality) // all voted 80
}

func TestFinalizeReview_HappyPath_Rejected(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// 2 reject, 1 accept -> majority reject
	for i, reviewer := range resp.ReviewersAssigned {
		decision := uint32(types.ReviewVoteReject)
		if i == 0 {
			decision = uint32(types.ReviewVoteAccept)
		}
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       decision,
			QualityScore:   60,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → session is auto-finalized immediately
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusRejected, session.Status)
	require.Equal(t, types.ReviewVoteReject, session.FinalDecision)
}

// ============================================================================
// D. Override Tests
// ============================================================================

func TestFinalizeReview_Override_DerivativeFalsePositive(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// Mark contribution as derivative (AI says derivative)
	contrib, _ := fixture.keeper.GetContribution(ctx, 1)
	contrib.IsDerivative = true
	err := fixture.keeper.SetContribution(ctx, contrib)
	require.NoError(t, err)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// All reviewers vote ACCEPT with OverrideDerivativeFalsePositive (AI was wrong)
	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:            reviewer,
			ContributionId:      1,
			Decision:            uint32(types.ReviewVoteAccept),
			OriginalityOverride: uint32(types.OverrideDerivativeFalsePositive),
			QualityScore:        90,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → session is auto-finalized immediately
	// Contribution should no longer be marked derivative
	contrib, found := fixture.keeper.GetContribution(ctx, 1)
	require.True(t, found)
	require.False(t, contrib.IsDerivative, "override should clear IsDerivative")

	session, _ := fixture.keeper.GetReviewSession(ctx, 1)
	require.Equal(t, types.OverrideDerivativeFalsePositive, session.OverrideApplied)
}

func TestFinalizeReview_Override_NotDerivativeFalseNegative(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// Contribution is NOT marked derivative (AI missed it)
	contrib, _ := fixture.keeper.GetContribution(ctx, 1)
	require.False(t, contrib.IsDerivative)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// Majority vote with OverrideNotDerivativeFalseNegative
	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:            reviewer,
			ContributionId:      1,
			Decision:            uint32(types.ReviewVoteReject),
			OriginalityOverride: uint32(types.OverrideNotDerivativeFalseNegative),
			QualityScore:        30,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → session is auto-finalized immediately
	// Contribution should now be marked derivative
	contrib, found := fixture.keeper.GetContribution(ctx, 1)
	require.True(t, found)
	require.True(t, contrib.IsDerivative, "override should set IsDerivative")
}

func TestFinalizeReview_Override_KeepAI(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// All vote with KeepAI override (the default)
	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:            reviewer,
			ContributionId:      1,
			Decision:            uint32(types.ReviewVoteAccept),
			OriginalityOverride: uint32(types.OverrideKeepAI),
			QualityScore:        75,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → session is auto-finalized immediately
	session, _ := fixture.keeper.GetReviewSession(ctx, 1)
	require.Equal(t, types.OverrideKeepAI, session.OverrideApplied)
}

func TestFinalizeReview_ParentClaimOverride(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// Also create a "parent" contribution
	seedContribution(t, fixture, ctx, 42, addrContributor)

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// Majority sets ParentClaimOverride = 42
	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:            reviewer,
			ContributionId:      1,
			Decision:            uint32(types.ReviewVoteAccept),
			QualityScore:        85,
			ParentClaimOverride: 42,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → session is auto-finalized immediately
	// Contribution should have ParentClaimId = 42
	contrib, found := fixture.keeper.GetContribution(ctx, 1)
	require.True(t, found)
	require.Equal(t, uint64(42), contrib.ParentClaimId, "majority parent claim override should be applied")
}

// ============================================================================
// E. Collusion Detection Tests
// ============================================================================

func TestCollusionDetection_BelowThreshold(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	// 6 total reviews, 4 same votes => 4*10000/6 = 6666 bps < 9000 threshold
	record := types.CoVotingRecord{
		ReviewerA:      addrReviewer1,
		ReviewerB:      addrReviewer2,
		SameVoteCount:  4,
		TotalPairCount: 6,
		LastUpdated:    50,
	}
	err := fixture.keeper.SetCoVotingRecord(ctx, record)
	require.NoError(t, err)

	collusionDetected, err := fixture.keeper.CheckCollusionRisk(ctx, []string{addrReviewer1, addrReviewer2})
	require.NoError(t, err)
	require.False(t, collusionDetected, "6666 bps should be below 9000 threshold")
}

func TestCollusionDetection_AboveThreshold(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	// 10 total, 10 same votes => 10000 bps >= 9000 threshold
	record := types.CoVotingRecord{
		ReviewerA:      addrReviewer1,
		ReviewerB:      addrReviewer2,
		SameVoteCount:  10,
		TotalPairCount: 10,
		LastUpdated:    50,
	}
	err := fixture.keeper.SetCoVotingRecord(ctx, record)
	require.NoError(t, err)

	collusionDetected, err := fixture.keeper.CheckCollusionRisk(ctx, []string{addrReviewer1, addrReviewer2})
	require.NoError(t, err)
	require.True(t, collusionDetected, "10000 bps should exceed 9000 threshold")
}

func TestCollusionDetection_InsufficientSample(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	// Only 4 pair reviews (< 5 minimum for flagging)
	record := types.CoVotingRecord{
		ReviewerA:      addrReviewer1,
		ReviewerB:      addrReviewer2,
		SameVoteCount:  4,
		TotalPairCount: 4,
		LastUpdated:    50,
	}
	err := fixture.keeper.SetCoVotingRecord(ctx, record)
	require.NoError(t, err)

	collusionDetected, err := fixture.keeper.CheckCollusionRisk(ctx, []string{addrReviewer1, addrReviewer2})
	require.NoError(t, err)
	require.False(t, collusionDetected, "< 5 pair reviews should not trigger collusion flag")
}

func TestUpdateCoVotingRecords_Accumulation(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	session := &types.ReviewSession{
		ContributionID:    1,
		AssignedReviewers: []string{addrReviewer1, addrReviewer2, addrReviewer3},
		Votes: []types.ReviewVote{
			{Reviewer: addrReviewer1, Decision: types.ReviewVoteAccept},
			{Reviewer: addrReviewer2, Decision: types.ReviewVoteAccept},
			{Reviewer: addrReviewer3, Decision: types.ReviewVoteReject},
		},
	}

	err := fixture.keeper.UpdateCoVotingRecords(ctx, session)
	require.NoError(t, err)

	// reviewer1 + reviewer2: same vote (both Accept)
	rec12, found := fixture.keeper.GetCoVotingRecord(ctx, addrReviewer1, addrReviewer2)
	require.True(t, found)
	require.Equal(t, uint64(1), rec12.SameVoteCount)
	require.Equal(t, uint64(1), rec12.TotalPairCount)

	// reviewer1 + reviewer3: different vote (Accept vs Reject)
	rec13, found := fixture.keeper.GetCoVotingRecord(ctx, addrReviewer1, addrReviewer3)
	require.True(t, found)
	require.Equal(t, uint64(0), rec13.SameVoteCount)
	require.Equal(t, uint64(1), rec13.TotalPairCount)

	// Run another session with same voters to verify accumulation
	session2 := &types.ReviewSession{
		ContributionID:    2,
		AssignedReviewers: []string{addrReviewer1, addrReviewer2},
		Votes: []types.ReviewVote{
			{Reviewer: addrReviewer1, Decision: types.ReviewVoteReject},
			{Reviewer: addrReviewer2, Decision: types.ReviewVoteReject},
		},
	}

	err = fixture.keeper.UpdateCoVotingRecords(ctx, session2)
	require.NoError(t, err)

	rec12after, found := fixture.keeper.GetCoVotingRecord(ctx, addrReviewer1, addrReviewer2)
	require.True(t, found)
	require.Equal(t, uint64(2), rec12after.SameVoteCount, "should accumulate same votes")
	require.Equal(t, uint64(2), rec12after.TotalPairCount, "should accumulate total pair count")
}

// ============================================================================
// F. Params Tests
// ============================================================================

func TestHumanReviewParams_SidecarRoundtrip(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)

	params := fixture.keeper.GetParams(ctx)

	// Verify the human review params we set in setupReviewFixture
	require.True(t, params.EnableHumanReview)
	require.Equal(t, uint32(3), params.VerifiersPerClaim)
	require.Equal(t, int64(100), params.ReviewVotePeriod)
	require.Equal(t, uint64(1000), params.MinReviewerReputation)
	require.Equal(t, "0omniphi", params.MinReviewerBond)
	require.Equal(t, uint32(67), params.ReviewQuorumPct)
	require.Equal(t, "0omniphi", params.AppealBond)
	require.Equal(t, int64(2400), params.AppealVotePeriod)
	require.Equal(t, uint32(9000), params.CollusionThresholdBps)

	// Modify and re-read — use freshReviewParams() to avoid nil sidecar fields
	updated := freshReviewParams()
	updated.VerifiersPerClaim = 5
	updated.ReviewVotePeriod = 500
	updated.MinReviewerReputation = 2000
	updated.ReviewQuorumPct = 80
	updated.CollusionThresholdBps = 8000
	err := fixture.keeper.SetParams(ctx, updated)
	require.NoError(t, err)

	got := fixture.keeper.GetParams(ctx)
	require.Equal(t, uint32(5), got.VerifiersPerClaim)
	require.Equal(t, int64(500), got.ReviewVotePeriod)
	require.Equal(t, uint64(2000), got.MinReviewerReputation)
	require.Equal(t, uint32(80), got.ReviewQuorumPct)
	require.Equal(t, uint32(8000), got.CollusionThresholdBps)
}

func TestHumanReviewParams_Validation(t *testing.T) {
	// Use freshReviewParams() throughout to avoid nil sidecar fields from proto roundtrip.
	_ = SetupKeeperTest(t) // ensure test infrastructure works

	// VerifiersPerClaim = 0 is invalid
	params := freshReviewParams()
	params.VerifiersPerClaim = 0
	err := params.Validate()
	require.Error(t, err, "verifiers_per_claim=0 should fail validation")

	// VerifiersPerClaim > MaxReviewersPerClaim is invalid
	params = freshReviewParams()
	params.VerifiersPerClaim = types.MaxReviewersPerClaim + 1
	err = params.Validate()
	require.Error(t, err, "verifiers_per_claim > max should fail validation")

	// ReviewVotePeriod <= 0 is invalid
	params = freshReviewParams()
	params.ReviewVotePeriod = 0
	err = params.Validate()
	require.Error(t, err, "review_vote_period=0 should fail validation")

	// ReviewVotePeriod > MaxReviewVotePeriod is invalid
	params = freshReviewParams()
	params.ReviewVotePeriod = types.MaxReviewVotePeriod + 1
	err = params.Validate()
	require.Error(t, err, "review_vote_period > max should fail validation")

	// ReviewQuorumPct > 100 is invalid
	params = freshReviewParams()
	params.ReviewQuorumPct = 101
	err = params.Validate()
	require.Error(t, err, "review_quorum_pct > 100 should fail validation")

	// CollusionThresholdBps > 10000 is invalid
	params = freshReviewParams()
	params.CollusionThresholdBps = 10001
	err = params.Validate()
	require.Error(t, err, "collusion_threshold_bps > 10000 should fail validation")

	// ReviewQuorumPct = 0 when EnableHumanReview is true
	params = freshReviewParams()
	params.EnableHumanReview = true
	params.ReviewQuorumPct = 0
	err = params.Validate()
	require.Error(t, err, "review_quorum_pct=0 with enable_human_review=true should fail")

	// Valid params should pass
	params = freshReviewParams()
	err = params.Validate()
	require.NoError(t, err, "default review fixture params should be valid")
}

// ============================================================================
// G. Appeal & EndBlocker Tests
// ============================================================================

// helper: run a full review cycle to completion (accepted)
func runFullReviewAccepted(t *testing.T, fixture *KeeperTestFixture, ctx sdk.Context, contribID uint64) []string {
	t.Helper()

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: contribID,
	})
	require.NoError(t, err)

	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: contribID,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → auto-finalized
	session, found := fixture.keeper.GetReviewSession(ctx, contribID)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusAccepted, session.Status)

	return resp.ReviewersAssigned
}

// helper: run a full review cycle to completion (rejected)
func runFullReviewRejected(t *testing.T, fixture *KeeperTestFixture, ctx sdk.Context, contribID uint64) []string {
	t.Helper()

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: contribID,
	})
	require.NoError(t, err)

	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: contribID,
			Decision:       uint32(types.ReviewVoteReject),
			QualityScore:   20,
		})
		require.NoError(t, err)
	}

	// Fast-path: all votes cast → auto-finalized
	session, found := fixture.keeper.GetReviewSession(ctx, contribID)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusRejected, session.Status)

	return resp.ReviewersAssigned
}

func TestAppealReview_HappyPath(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	runFullReviewRejected(t, fixture, ctx, 1)

	// File appeal
	appealResp, err := fixture.keeper.ProcessAppealReview(ctx, &types.MsgAppealReview{
		Appellant:      addrAppellant,
		ContributionId: 1,
		Reason:         "unfair rejection, content is original",
	})
	require.NoError(t, err)
	require.NotNil(t, appealResp)
	require.Equal(t, uint64(1), appealResp.AppealId)

	// Verify session status is APPEALED
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusAppealed, session.Status)
	require.Equal(t, uint64(1), session.AppealID)

	// Verify appeal was stored
	appeal, found := fixture.keeper.GetReviewAppeal(ctx, 1)
	require.True(t, found)
	require.Equal(t, addrAppellant, appeal.Appellant)
	require.Equal(t, "unfair rejection, content is original", appeal.Reason)
	require.False(t, appeal.Resolved)
}

func TestResolveAppeal_Upheld(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	runFullReviewRejected(t, fixture, ctx, 1)

	// File appeal
	appealResp, err := fixture.keeper.ProcessAppealReview(ctx, &types.MsgAppealReview{
		Appellant:      addrAppellant,
		ContributionId: 1,
		Reason:         "test appeal",
	})
	require.NoError(t, err)

	// Resolve: upheld (original verdict stands)
	_, err = fixture.keeper.ProcessResolveAppeal(ctx, &types.MsgResolveAppeal{
		Authority:     addrAuthority,
		AppealId:      appealResp.AppealId,
		Upheld:        true,
		ResolverNotes: "after review, original rejection stands",
	})
	require.NoError(t, err)

	// Verify appeal is resolved and upheld
	appeal, found := fixture.keeper.GetReviewAppeal(ctx, appealResp.AppealId)
	require.True(t, found)
	require.True(t, appeal.Resolved)
	require.True(t, appeal.Upheld)
	require.Equal(t, "after review, original rejection stands", appeal.ResolverNotes)

	// Session reverts to original status (REJECTED)
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusRejected, session.Status)
}

func TestResolveAppeal_Overturned(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	runFullReviewRejected(t, fixture, ctx, 1)

	// File appeal
	appealResp, err := fixture.keeper.ProcessAppealReview(ctx, &types.MsgAppealReview{
		Appellant:      addrAppellant,
		ContributionId: 1,
		Reason:         "content is original",
	})
	require.NoError(t, err)

	// Resolve: NOT upheld (overturn)
	_, err = fixture.keeper.ProcessResolveAppeal(ctx, &types.MsgResolveAppeal{
		Authority:     addrAuthority,
		AppealId:      appealResp.AppealId,
		Upheld:        false,
		ResolverNotes: "upon re-examination, content is original",
	})
	require.NoError(t, err)

	// Verify appeal is resolved and NOT upheld
	appeal, found := fixture.keeper.GetReviewAppeal(ctx, appealResp.AppealId)
	require.True(t, found)
	require.True(t, appeal.Resolved)
	require.False(t, appeal.Upheld)

	// Session should now be ACCEPTED (reversed from REJECTED)
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusAccepted, session.Status)
	require.Equal(t, types.ReviewVoteAccept, session.FinalDecision)

	// Contribution review status should also be updated
	contrib, found := fixture.keeper.GetContribution(ctx, 1)
	require.True(t, found)
	require.Equal(t, uint32(types.ReviewStatusAccepted), contrib.ReviewStatus)
}

func TestFinalizeExpiredReviews_AutoFinalize(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	seedReviewers(t, fixture, ctx, []string{addrReviewer1, addrReviewer2, addrReviewer3}, 5000)
	seedContribution(t, fixture, ctx, 1, addrContributor)

	// Start a review
	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// Only one reviewer votes (out of 3 required by quorum 67%)
	_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
		Reviewer:       resp.ReviewersAssigned[0],
		ContributionId: 1,
		Decision:       uint32(types.ReviewVoteAccept),
		QualityScore:   80,
	})
	require.NoError(t, err)

	// Verify session is still IN_REVIEW
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusInReview, session.Status)

	// Advance block height past end height (EndHeight = 200)
	expiredCtx := ctx.WithBlockHeight(201)

	// Run EndBlocker auto-finalization
	err = fixture.keeper.FinalizeExpiredReviews(expiredCtx)
	require.NoError(t, err)

	// Session should now be finalized (REJECTED because quorum not met: 1/3 < ceil(3*67/100)=3)
	// Actually with quorum 67% and 3 assigned: ceil(3*67/100) = ceil(2.01) = 3 required
	// Only 1 vote cast, so quorum not met -> default to REJECTED
	session, found = fixture.keeper.GetReviewSession(expiredCtx, 1)
	require.True(t, found)
	require.NotEqual(t, types.ReviewStatusInReview, session.Status, "session should no longer be IN_REVIEW after auto-finalize")
	require.Equal(t, types.ReviewStatusRejected, session.Status, "quorum not met should default to REJECTED")
}
