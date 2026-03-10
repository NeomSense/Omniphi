package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

// ============================================================================
// Pipeline Integration Tests
//
// These tests verify the end-to-end wiring across all 5 PoC layers:
//   Layer 1: Canonical Hash (deduplication)
//   Layer 2: Similarity Engine (derivative detection)
//   Layer 3: Human Review (PoV override)
//   Layer 4: Economic Adjustment (rewards/vesting/clawback)
//   Layer 5: Provenance Registry (DAG lineage)
// ============================================================================

// TestPipeline_OriginalAccepted tests the full happy-path:
// Submit -> Review (accept) -> Reward distributed -> ClaimStatus = ACCEPTED
func TestPipeline_OriginalAccepted(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)

	// Fund the module account so rewards can be distributed
	moduleAddr := sdk.AccAddress("module_address______").String()
	fixture.bankKeeper.setBalance(moduleAddr, "omniphi", math.NewInt(1000000))

	// --- STEP 1: Create contribution (simulates post-submission state) ---
	contrib := types.Contribution{
		Id:          1,
		Contributor: addrContributor,
		Ctype:       "code",
		Uri:         "ipfs://original-work",
		Hash:        make([]byte, 32),
		ClaimStatus: uint32(types.ClaimStatusAwaitingSimilarity),
	}
	contrib.Hash[0] = 0x01
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))

	// Transition to IN_REVIEW (simulates similarity check passed, non-derivative)
	fixture.keeper.TransitionClaimStatus(ctx, 1, types.ClaimStatusInReview)

	// --- STEP 2: Start review ---
	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)
	require.Len(t, resp.ReviewersAssigned, 3)

	// --- STEP 3: All reviewers vote ACCEPT with quality 80 ---
	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// --- VERIFY: Review accepted ---
	session, found := fixture.keeper.GetReviewSession(ctx, 1)
	require.True(t, found)
	require.Equal(t, types.ReviewStatusAccepted, session.Status)

	// --- VERIFY: ClaimStatus updated to ACCEPTED ---
	updatedContrib, found := fixture.keeper.GetContribution(ctx, 1)
	require.True(t, found)
	require.Equal(t, uint32(types.ClaimStatusAccepted), updatedContrib.ClaimStatus)
	require.Equal(t, uint32(types.ReviewStatusAccepted), updatedContrib.ReviewStatus)

	// --- VERIFY: Contributor stats updated ---
	stats := fixture.keeper.GetContributorStats(ctx, addrContributor)
	require.Equal(t, uint64(1), stats.TotalSubmissions)
}

// TestPipeline_DuplicateRejected tests: Submit -> Canonical dup detected -> ClaimStatus = DUPLICATE
func TestPipeline_DuplicateRejected(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Enable canonical hash check
	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	params.BaseRewardUnit = math.NewInt(100)
	params.MaxPerBlock = 100
	params.EnableCanonicalHashCheck = true
	params.DuplicateBond = sdk.NewCoin("omniphi", math.ZeroInt())
	require.NoError(t, fixture.keeper.SetParams(ctx, params))

	// Fund contributor
	contributor := sdk.AccAddress("contributor_________")
	fixture.bankKeeper.setBalance(contributor.String(), "omniphi", math.NewInt(1000000))

	msgSrv := keeper.NewMsgServerImpl(fixture.keeper)

	// Submit first contribution
	hash := make([]byte, 32)
	hash[0] = 0xAA
	canonicalHash := make([]byte, 32)
	canonicalHash[0] = 0xBB

	msg1 := &types.MsgSubmitContribution{
		Contributor:          contributor.String(),
		Ctype:                "code",
		Uri:                  "ipfs://original",
		Hash:                 hash,
		CanonicalHash:        canonicalHash,
		CanonicalSpecVersion: 1,
	}
	resp1, err := msgSrv.SubmitContribution(ctx, msg1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp1.Id)

	// First contribution should be AWAITING_SIMILARITY
	c1, _ := fixture.keeper.GetContribution(ctx, 1)
	require.Equal(t, uint32(types.ClaimStatusAwaitingSimilarity), c1.ClaimStatus)

	// Submit duplicate with same canonical hash
	hash2 := make([]byte, 32)
	hash2[0] = 0xCC
	msg2 := &types.MsgSubmitContribution{
		Contributor:          contributor.String(),
		Ctype:                "code",
		Uri:                  "ipfs://duplicate",
		Hash:                 hash2,
		CanonicalHash:        canonicalHash, // same canonical hash
		CanonicalSpecVersion: 1,
	}
	resp2, err := msgSrv.SubmitContribution(ctx, msg2)
	require.NoError(t, err)

	// Second contribution should be DUPLICATE
	c2, found := fixture.keeper.GetContribution(ctx, resp2.Id)
	require.True(t, found)
	require.Equal(t, uint64(1), c2.DuplicateOf)
	require.Equal(t, uint32(types.ClaimStatusDuplicate), c2.ClaimStatus)

	// No vesting schedule should exist for the duplicate
	_, vestingFound := fixture.keeper.GetVestingSchedule(ctx, contributor.String(), resp2.Id)
	require.False(t, vestingFound, "duplicate should not have vesting schedule")
}

// TestPipeline_AcceptedThenAppealed tests the full dispute flow:
// Submit -> Review (accept) -> Appeal filed -> Vesting frozen -> Appeal resolved (overturned) -> Vesting clawed back
func TestPipeline_AcceptedThenAppealed(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)

	// Fund the module account for rewards
	moduleAddr := sdk.AccAddress("module_address______").String()
	fixture.bankKeeper.setBalance(moduleAddr, "omniphi", math.NewInt(1000000))

	// --- STEP 1: Create and accept contribution ---
	contrib := types.Contribution{
		Id:          1,
		Contributor: addrContributor,
		Ctype:       "code",
		Uri:         "ipfs://original-work",
		Hash:        make([]byte, 32),
		ClaimStatus: uint32(types.ClaimStatusInReview),
	}
	contrib.Hash[0] = 0x01
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// Verify accepted
	session, _ := fixture.keeper.GetReviewSession(ctx, 1)
	require.Equal(t, types.ReviewStatusAccepted, session.Status)
	require.Equal(t, uint32(types.ClaimStatusAccepted), func() uint32 {
		c, _ := fixture.keeper.GetContribution(ctx, 1)
		return c.ClaimStatus
	}())

	// --- STEP 2: Manually create a vesting schedule (simulates DistributeRewards creating one) ---
	vestingSchedule := types.VestingSchedule{
		Contributor:    addrContributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(100),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     0,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}
	require.NoError(t, fixture.keeper.CreateVestingSchedule(ctx, vestingSchedule))

	// --- STEP 3: File appeal ---
	_, err = fixture.keeper.ProcessAppealReview(ctx, &types.MsgAppealReview{
		ContributionId: 1,
		Appellant:      addrAppellant,
		Reason:         "This is plagiarized content",
	})
	require.NoError(t, err)

	// Verify vesting is PAUSED
	schedule, found := fixture.keeper.GetVestingSchedule(ctx, addrContributor, 1)
	require.True(t, found)
	require.Equal(t, types.VestingStatusPaused, schedule.Status, "vesting should be paused during appeal")

	// Verify ClaimStatus = DISPUTED
	c, _ := fixture.keeper.GetContribution(ctx, 1)
	require.Equal(t, uint32(types.ClaimStatusDisputed), c.ClaimStatus)

	// --- STEP 4: Resolve appeal — overturn to REJECTED ---
	_, err = fixture.keeper.ProcessResolveAppeal(ctx, &types.MsgResolveAppeal{
		Authority:     addrAuthority,
		AppealId:      1,
		Upheld:        false, // overturn
		ResolverNotes: "ipfs://proof-of-plagiarism",
	})
	require.NoError(t, err)

	// Verify vesting is CLAWED BACK
	// Clawed-back schedule is deleted from the store (terminal state cleanup).
	_, found = fixture.keeper.GetVestingSchedule(ctx, addrContributor, 1)
	require.False(t, found, "clawed-back vesting schedule should be deleted from store after overturn")

	// Verify ClaimStatus = RESOLVED
	c, _ = fixture.keeper.GetContribution(ctx, 1)
	require.Equal(t, uint32(types.ClaimStatusResolved), c.ClaimStatus)
}

// TestPipeline_VestingResumedOnUpheldAppeal tests: Accept -> Appeal -> Upheld -> Vesting resumes
func TestPipeline_VestingResumedOnUpheldAppeal(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)

	moduleAddr := sdk.AccAddress("module_address______").String()
	fixture.bankKeeper.setBalance(moduleAddr, "omniphi", math.NewInt(1000000))

	// Create and accept contribution
	contrib := types.Contribution{
		Id:          1,
		Contributor: addrContributor,
		Ctype:       "code",
		Uri:         "ipfs://original-work",
		Hash:        make([]byte, 32),
		ClaimStatus: uint32(types.ClaimStatusInReview),
	}
	contrib.Hash[0] = 0x01
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// Create vesting
	vestingSchedule := types.VestingSchedule{
		Contributor:    addrContributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(100),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     0,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}
	require.NoError(t, fixture.keeper.CreateVestingSchedule(ctx, vestingSchedule))

	// File appeal
	_, err = fixture.keeper.ProcessAppealReview(ctx, &types.MsgAppealReview{
		ContributionId: 1,
		Appellant:      addrAppellant,
		Reason:         "Suspect plagiarism",
	})
	require.NoError(t, err)

	// Verify paused
	schedule, _ := fixture.keeper.GetVestingSchedule(ctx, addrContributor, 1)
	require.Equal(t, types.VestingStatusPaused, schedule.Status)

	// Resolve appeal — UPHELD (original acceptance stands)
	_, err = fixture.keeper.ProcessResolveAppeal(ctx, &types.MsgResolveAppeal{
		Authority:     addrAuthority,
		AppealId:      1,
		Upheld:        true,
		ResolverNotes: "Appeal dismissed — work is original",
	})
	require.NoError(t, err)

	// Verify vesting resumed
	schedule, found := fixture.keeper.GetVestingSchedule(ctx, addrContributor, 1)
	require.True(t, found)
	require.Equal(t, types.VestingStatusActive, schedule.Status, "vesting should be active after upheld appeal")

	// Verify ClaimStatus = RESOLVED
	c, _ := fixture.keeper.GetContribution(ctx, 1)
	require.Equal(t, uint32(types.ClaimStatusResolved), c.ClaimStatus)
}

// ============================================================================
// ClaimStatus Transition Tests
// ============================================================================

func TestClaimStatusTransition_ValidPath(t *testing.T) {
	tests := []struct {
		from types.ClaimStatus
		to   types.ClaimStatus
	}{
		{types.ClaimStatusSubmitted, types.ClaimStatusDuplicate},
		{types.ClaimStatusSubmitted, types.ClaimStatusAwaitingSimilarity},
		{types.ClaimStatusAwaitingSimilarity, types.ClaimStatusFlaggedDerivative},
		{types.ClaimStatusAwaitingSimilarity, types.ClaimStatusInReview},
		{types.ClaimStatusFlaggedDerivative, types.ClaimStatusInReview},
		{types.ClaimStatusInReview, types.ClaimStatusAccepted},
		{types.ClaimStatusInReview, types.ClaimStatusRejected},
		{types.ClaimStatusAccepted, types.ClaimStatusDisputed},
		{types.ClaimStatusRejected, types.ClaimStatusDisputed},
		{types.ClaimStatusDisputed, types.ClaimStatusResolved},
	}

	for _, tt := range tests {
		t.Run(tt.from.String()+"->"+tt.to.String(), func(t *testing.T) {
			err := types.ValidateClaimTransition(tt.from, tt.to)
			require.NoError(t, err)
		})
	}
}

func TestClaimStatusTransition_InvalidPath(t *testing.T) {
	tests := []struct {
		from types.ClaimStatus
		to   types.ClaimStatus
	}{
		{types.ClaimStatusDuplicate, types.ClaimStatusInReview},     // terminal
		{types.ClaimStatusResolved, types.ClaimStatusAccepted},      // terminal
		{types.ClaimStatusSubmitted, types.ClaimStatusAccepted},     // skip layers
		{types.ClaimStatusAccepted, types.ClaimStatusRejected},      // no direct reversal
		{types.ClaimStatusInReview, types.ClaimStatusSubmitted},     // backward
		{types.ClaimStatusFlaggedDerivative, types.ClaimStatusAccepted}, // skip review
	}

	for _, tt := range tests {
		t.Run(tt.from.String()+"->"+tt.to.String(), func(t *testing.T) {
			err := types.ValidateClaimTransition(tt.from, tt.to)
			require.Error(t, err)
		})
	}
}

// TestPipeline_PauseResumeVesting tests PauseVesting and ResumeVesting directly
func TestPipeline_PauseResumeVesting(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	contributor := addrContributor

	// Create an active vesting schedule
	schedule := types.VestingSchedule{
		Contributor:    contributor,
		ClaimID:        42,
		TotalAmount:    math.NewInt(500),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     0,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}
	require.NoError(t, fixture.keeper.CreateVestingSchedule(ctx, schedule))

	// Pause
	require.NoError(t, fixture.keeper.PauseVesting(ctx, contributor, 42))
	s, found := fixture.keeper.GetVestingSchedule(ctx, contributor, 42)
	require.True(t, found)
	require.Equal(t, types.VestingStatusPaused, s.Status)

	// Resume
	require.NoError(t, fixture.keeper.ResumeVesting(ctx, contributor, 42))
	s, _ = fixture.keeper.GetVestingSchedule(ctx, contributor, 42)
	require.Equal(t, types.VestingStatusActive, s.Status)

	// Pause then clawback (paused schedule should also be clawbackable)
	require.NoError(t, fixture.keeper.PauseVesting(ctx, contributor, 42))
	unvested, err := fixture.keeper.ClawbackVesting(ctx, contributor, 42)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(500), unvested)
	// Clawed-back schedule is deleted from the store.
	_, clawedFound := fixture.keeper.GetVestingSchedule(ctx, contributor, 42)
	require.False(t, clawedFound, "clawed-back schedule should be deleted from store")
}

// TestPipeline_PauseVesting_NoOp tests that PauseVesting is a no-op on non-active schedules
func TestPipeline_PauseVesting_NoOp(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Pause on non-existent schedule — no error
	require.NoError(t, fixture.keeper.PauseVesting(ctx, addrContributor, 999))

	// Resume on non-existent schedule — no error
	require.NoError(t, fixture.keeper.ResumeVesting(ctx, addrContributor, 999))
}

// TestPipeline_GetSimilarityScore tests the similarity score retrieval helper
func TestPipeline_GetSimilarityScore(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// No similarity data → returns 0
	score := fixture.keeper.GetSimilarityScore(ctx, 1)
	require.True(t, score.IsZero())

	// Store a similarity commitment record
	record := types.SimilarityCommitmentRecord{
		ContributionID: 1,
		CompactData: types.SimilarityCompactData{
			ContributionID:    1,
			OverallSimilarity: 8500, // 85%
			Confidence:        9000,
			ModelVersion:      "v1",
			Epoch:             1,
		},
		CommitmentHashFull: make([]byte, 32),
		OracleAddress:      "oracle",
		BlockHeight:        100,
		IsDerivative:       true,
	}
	record.CommitmentHashFull[0] = 0x01
	require.NoError(t, fixture.keeper.SetSimilarityCommitment(ctx, record))

	// Now should return 0.85
	score = fixture.keeper.GetSimilarityScore(ctx, 1)
	expected := math.LegacyNewDecWithPrec(85, 2)
	require.True(t, score.Equal(expected), "expected 0.85, got %s", score)
}

// ============================================================================
// Security Tests
// ============================================================================

// TestSecurity_StartReview_AuthorityRequired verifies that only the governance
// authority can start reviews, preventing griefing attacks.
func TestSecurity_StartReview_AuthorityRequired(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)

	// Create contribution
	contrib := types.Contribution{
		Id:          1,
		Contributor: addrContributor,
		Ctype:       "code",
		Uri:         "ipfs://work",
		Hash:        make([]byte, 32),
		ClaimStatus: uint32(types.ClaimStatusAwaitingSimilarity),
	}
	contrib.Hash[0] = 0x01
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))
	fixture.keeper.TransitionClaimStatus(ctx, 1, types.ClaimStatusInReview)

	// Attempt with unauthorized address — should fail
	_, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrContributor, // NOT the authority
		ContributionId: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")

	// Attempt with correct authority — should succeed
	_, err = fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)
}

// TestSecurity_DuplicateNeverRewarded verifies that a contribution marked as
// duplicate never receives rewards even if it somehow passes review.
func TestSecurity_DuplicateNeverRewarded(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)

	moduleAddr := sdk.AccAddress("module_address______").String()
	fixture.bankKeeper.setBalance(moduleAddr, "omniphi", math.NewInt(1000000))

	// Create a contribution that is a duplicate but somehow enters review
	contrib := types.Contribution{
		Id:          1,
		Contributor: addrContributor,
		Ctype:       "code",
		Uri:         "ipfs://dup-work",
		Hash:        make([]byte, 32),
		DuplicateOf: 99, // marked as duplicate!
		ClaimStatus: uint32(types.ClaimStatusInReview),
	}
	contrib.Hash[0] = 0x01
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))

	// Start and accept review
	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// Review should be accepted
	session, _ := fixture.keeper.GetReviewSession(ctx, 1)
	require.Equal(t, types.ReviewStatusAccepted, session.Status)

	// But NO vesting schedule should exist — duplicate must not be rewarded
	_, vestingFound := fixture.keeper.GetVestingSchedule(ctx, addrContributor, 1)
	require.False(t, vestingFound, "duplicate contribution should never get vesting even if review accepts")

	// Contributor stats should NOT be updated
	stats := fixture.keeper.GetContributorStats(ctx, addrContributor)
	require.Equal(t, uint64(0), stats.TotalSubmissions, "duplicate should not increment stats")
}

// TestSecurity_AppealOverturn_FullClawback verifies that when an appeal overturns
// an ACCEPTED contribution to REJECTED, both vesting AND immediate rewards are clawed back.
func TestSecurity_AppealOverturn_FullClawback(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)

	moduleAddr := sdk.AccAddress("module_address______").String()
	fixture.bankKeeper.setBalance(moduleAddr, "omniphi", math.NewInt(1000000))

	// Create and accept contribution
	contrib := types.Contribution{
		Id:          1,
		Contributor: addrContributor,
		Ctype:       "code",
		Uri:         "ipfs://original-work",
		Hash:        make([]byte, 32),
		ClaimStatus: uint32(types.ClaimStatusInReview),
	}
	contrib.Hash[0] = 0x01
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// Create vesting schedule
	vestingSchedule := types.VestingSchedule{
		Contributor:    addrContributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(100),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     0,
		VestingEpochs:  10,
		Status:         types.VestingStatusActive,
	}
	require.NoError(t, fixture.keeper.CreateVestingSchedule(ctx, vestingSchedule))

	// Give contributor some balance (simulates immediate reward already sent)
	fixture.bankKeeper.setBalance(addrContributor, "omniphi", math.NewInt(50))

	// File appeal
	_, err = fixture.keeper.ProcessAppealReview(ctx, &types.MsgAppealReview{
		ContributionId: 1,
		Appellant:      addrAppellant,
		Reason:         "Plagiarized",
	})
	require.NoError(t, err)

	// Overturn appeal (ACCEPTED -> REJECTED)
	_, err = fixture.keeper.ProcessResolveAppeal(ctx, &types.MsgResolveAppeal{
		Authority:     addrAuthority,
		AppealId:      1,
		Upheld:        false,
		ResolverNotes: "Confirmed plagiarism",
	})
	require.NoError(t, err)

	// Clawed-back schedule is deleted from the store (terminal state cleanup prevents iterator bloat).
	_, found := fixture.keeper.GetVestingSchedule(ctx, addrContributor, 1)
	require.False(t, found, "clawed-back vesting schedule should be deleted from store")

	// Verify a clawback record exists (ExecuteClawback was called)
	_, clawbackFound := fixture.keeper.GetClawbackRecord(ctx, 1)
	require.True(t, clawbackFound, "ExecuteClawback should create a clawback record on appeal overturn")
}

// TestSecurity_DoubleFinalization_Prevented verifies that a review cannot be
// finalized twice to get double rewards.
func TestSecurity_DoubleFinalization_Prevented(t *testing.T) {
	fixture, ctx := setupReviewFixture(t)
	reviewers := []string{addrReviewer1, addrReviewer2, addrReviewer3}
	seedReviewers(t, fixture, ctx, reviewers, 5000)

	moduleAddr := sdk.AccAddress("module_address______").String()
	fixture.bankKeeper.setBalance(moduleAddr, "omniphi", math.NewInt(1000000))

	// Create contribution
	contrib := types.Contribution{
		Id:          1,
		Contributor: addrContributor,
		Ctype:       "code",
		Uri:         "ipfs://work",
		Hash:        make([]byte, 32),
		ClaimStatus: uint32(types.ClaimStatusInReview),
	}
	contrib.Hash[0] = 0x01
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))

	resp, err := fixture.keeper.ProcessStartReview(ctx, &types.MsgStartReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.NoError(t, err)

	// Cast all votes (triggers fast-path finalization)
	for _, reviewer := range resp.ReviewersAssigned {
		_, err = fixture.keeper.ProcessCastReviewVote(ctx, &types.MsgCastReviewVote{
			Reviewer:       reviewer,
			ContributionId: 1,
			Decision:       uint32(types.ReviewVoteAccept),
			QualityScore:   80,
		})
		require.NoError(t, err)
	}

	// Already finalized via fast path
	session, _ := fixture.keeper.GetReviewSession(ctx, 1)
	require.Equal(t, types.ReviewStatusAccepted, session.Status)

	// Attempt manual finalization — should fail because status is no longer IN_REVIEW
	_, err = fixture.keeper.ProcessFinalizeReview(ctx, &types.MsgFinalizeReview{
		Authority:      addrAuthority,
		ContributionId: 1,
	})
	require.Error(t, err, "double finalization should be rejected")
	require.Contains(t, err.Error(), "no active review")
}

// TestSecurity_VestingPauseBypass_Prevented verifies that ProcessVestingReleases
// does NOT release funds from a Paused schedule.
func TestSecurity_VestingPauseBypass_Prevented(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Set up params with reward denom
	params := types.DefaultParams()
	params.RewardDenom = "omniphi"
	require.NoError(t, fixture.keeper.SetParams(ctx, params))

	// Create a paused vesting schedule
	schedule := types.VestingSchedule{
		Contributor:    addrContributor,
		ClaimID:        1,
		TotalAmount:    math.NewInt(1000),
		ReleasedAmount: math.ZeroInt(),
		StartEpoch:     0,
		VestingEpochs:  4,
		Status:         types.VestingStatusPaused,
	}
	require.NoError(t, fixture.keeper.CreateVestingSchedule(ctx, schedule))

	// Process vesting releases (simulates EndBlocker)
	require.NoError(t, fixture.keeper.ProcessVestingReleases(ctx))

	// Verify NO funds were released
	s, found := fixture.keeper.GetVestingSchedule(ctx, addrContributor, 1)
	require.True(t, found)
	require.Equal(t, types.VestingStatusPaused, s.Status, "paused schedule should remain paused")
	require.True(t, s.ReleasedAmount.IsZero(), "paused schedule should not release any funds")
}
