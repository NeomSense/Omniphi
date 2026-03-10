package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ============================================================================
// Human Review Layer (Layer 3: PoV Override)
//
// Architecture:
//   1. CRUD — ReviewSession, ReviewerProfile, ReviewAppeal, CoVotingRecord, Bond Escrow
//   2. Reviewer Selection — deterministic Fisher-Yates using block hash seed
//   3. Collusion Detection — co-voting pattern analysis
//   4. Bond Management — collect / refund / slash reviewer bonds
//   5. Processing Pipeline — StartReview, CastVote, Finalize, Appeal, Resolve
//   6. EndBlocker — auto-finalize expired reviews
// ============================================================================

// ============================================================================
// 1. CRUD Methods
// ============================================================================

// ---------- ReviewSession ----------

// GetReviewSession retrieves a review session by contribution ID.
func (k Keeper) GetReviewSession(ctx context.Context, contributionID uint64) (types.ReviewSession, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewSessionKey(contributionID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ReviewSession{}, false
	}

	var session types.ReviewSession
	if err := json.Unmarshal(bz, &session); err != nil {
		return types.ReviewSession{}, false
	}
	return session, true
}

// SetReviewSession stores a review session.
func (k Keeper) SetReviewSession(ctx context.Context, session types.ReviewSession) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewSessionKey(session.ContributionID)

	bz, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal review session: %w", err)
	}

	return store.Set(key, bz)
}

// DeleteReviewSession removes a review session from the store.
func (k Keeper) DeleteReviewSession(ctx context.Context, contributionID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewSessionKey(contributionID)
	return store.Delete(key)
}

// ---------- ReviewerProfile ----------

// GetReviewerProfile retrieves a reviewer profile by address.
func (k Keeper) GetReviewerProfile(ctx context.Context, addr string) (types.ReviewerProfile, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewerProfileKey(addr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ReviewerProfile{}, false
	}

	var profile types.ReviewerProfile
	if err := json.Unmarshal(bz, &profile); err != nil {
		return types.ReviewerProfile{}, false
	}
	return profile, true
}

// SetReviewerProfile stores a reviewer profile.
func (k Keeper) SetReviewerProfile(ctx context.Context, profile types.ReviewerProfile) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewerProfileKey(profile.Address)

	bz, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal reviewer profile: %w", err)
	}

	return store.Set(key, bz)
}

// ---------- ReviewAppeal ----------

// GetReviewAppeal retrieves a review appeal by appeal ID.
func (k Keeper) GetReviewAppeal(ctx context.Context, appealID uint64) (types.ReviewAppeal, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewAppealKey(appealID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ReviewAppeal{}, false
	}

	var appeal types.ReviewAppeal
	if err := json.Unmarshal(bz, &appeal); err != nil {
		return types.ReviewAppeal{}, false
	}
	return appeal, true
}

// SetReviewAppeal stores a review appeal.
func (k Keeper) SetReviewAppeal(ctx context.Context, appeal types.ReviewAppeal) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewAppealKey(appeal.AppealID)

	bz, err := json.Marshal(appeal)
	if err != nil {
		return fmt.Errorf("failed to marshal review appeal: %w", err)
	}

	return store.Set(key, bz)
}

// ---------- Appeal ID Counter ----------

// GetNextAppealID returns the next appeal ID and increments the counter.
func (k Keeper) GetNextAppealID(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyNextReviewAppealID)
	if err != nil || bz == nil {
		return 1
	}
	return sdk.BigEndianToUint64(bz)
}

// setNextAppealID sets the next appeal ID counter.
func (k Keeper) setNextAppealID(ctx context.Context, id uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyNextReviewAppealID, sdk.Uint64ToBigEndian(id))
}

// ---------- CoVotingRecord ----------

// GetCoVotingRecord retrieves a co-voting record for a reviewer pair.
func (k Keeper) GetCoVotingRecord(ctx context.Context, addrA, addrB string) (types.CoVotingRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetCoVotingRecordKey(addrA, addrB)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.CoVotingRecord{}, false
	}

	var record types.CoVotingRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.CoVotingRecord{}, false
	}
	return record, true
}

// SetCoVotingRecord stores a co-voting record.
func (k Keeper) SetCoVotingRecord(ctx context.Context, record types.CoVotingRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetCoVotingRecordKey(record.ReviewerA, record.ReviewerB)

	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal co-voting record: %w", err)
	}

	return store.Set(key, bz)
}

// ---------- Pending Review Index ----------

// SetPendingReviewIndex indexes a review session for EndBlocker auto-finalization.
func (k Keeper) SetPendingReviewIndex(ctx context.Context, endHeight int64, contributionID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetPendingReviewIndexKey(endHeight, contributionID)
	return store.Set(key, []byte{1}) // value is a sentinel byte
}

// DeletePendingReviewIndex removes a pending review index entry.
func (k Keeper) DeletePendingReviewIndex(ctx context.Context, endHeight int64, contributionID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetPendingReviewIndexKey(endHeight, contributionID)
	return store.Delete(key)
}

// ---------- Review Bond Escrow (private) ----------

func (k Keeper) setReviewBondEscrow(ctx context.Context, addr string, contributionID uint64, escrow types.ReviewBondEscrow) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewBondEscrowKey(addr, contributionID)

	bz, err := json.Marshal(escrow)
	if err != nil {
		return fmt.Errorf("failed to marshal review bond escrow: %w", err)
	}

	return store.Set(key, bz)
}

func (k Keeper) getReviewBondEscrow(ctx context.Context, addr string, contributionID uint64) (types.ReviewBondEscrow, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewBondEscrowKey(addr, contributionID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ReviewBondEscrow{}, false
	}

	var escrow types.ReviewBondEscrow
	if err := json.Unmarshal(bz, &escrow); err != nil {
		return types.ReviewBondEscrow{}, false
	}
	return escrow, true
}

func (k Keeper) deleteReviewBondEscrow(ctx context.Context, addr string, contributionID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReviewBondEscrowKey(addr, contributionID)
	return store.Delete(key)
}

// ============================================================================
// 2. Reviewer Selection Algorithm
// ============================================================================

// SelectReviewers builds an eligible set and deterministically selects N reviewers
// using a Fisher-Yates shuffle seeded by block hash and contribution ID.
// Returns the selected reviewer addresses and the seed used.
func (k Keeper) SelectReviewers(ctx context.Context, contributionID uint64, contributor string, n uint32) ([]string, []byte, error) {
	params := k.GetParams(ctx)
	minRep := math.NewIntFromUint64(params.MinReviewerReputation)

	// 1. Build eligible set
	var eligible []string
	err := k.IterateCredits(ctx, func(credits types.Credits) bool {
		addr := credits.Address

		// Exclude the contributor (no self-review)
		if addr == contributor {
			return false
		}

		// Check minimum reputation (credits amount)
		if credits.Amount.LT(minRep) {
			return false
		}

		// Check if reviewer is suspended
		profile, found := k.GetReviewerProfile(ctx, addr)
		if found && profile.Suspended {
			return false
		}

		eligible = append(eligible, addr)
		return false
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to iterate credits for reviewer selection: %w", err)
	}

	// Sort for determinism (iteration order may vary)
	sort.Strings(eligible)

	if uint32(len(eligible)) < n {
		return nil, nil, types.ErrInsufficientEligibleReviewers.Wrapf(
			"need %d reviewers but only %d eligible", n, len(eligible))
	}

	// 2. Compute seed: SHA256(block_hash || contribution_id_bytes)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHash := sdkCtx.HeaderHash()
	idBytes := sdk.Uint64ToBigEndian(contributionID)
	seedInput := append(blockHash, idBytes...)
	seed := sha256.Sum256(seedInput)

	// 3. Deterministic Fisher-Yates shuffle
	for i := len(eligible) - 1; i > 0; i-- {
		// hash = SHA256(seed || counter)
		counterBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(counterBytes, uint64(i))
		h := sha256.Sum256(append(seed[:], counterBytes...))

		// Take first 8 bytes as uint64, modulo (i+1)
		randVal := binary.BigEndian.Uint64(h[:8])
		j := int(randVal % uint64(i+1))

		eligible[i], eligible[j] = eligible[j], eligible[i]
	}

	// 4. Take first N
	selected := eligible[:n]

	return selected, seed[:], nil
}

// ============================================================================
// 3. Collusion Detection
// ============================================================================

// CheckCollusionRisk checks all reviewer pairs for suspicious co-voting patterns.
// Returns true if any pair exceeds the collusion threshold.
func (k Keeper) CheckCollusionRisk(ctx context.Context, reviewers []string) (bool, error) {
	params := k.GetParams(ctx)
	threshold := uint64(params.CollusionThresholdBps)

	for i := 0; i < len(reviewers); i++ {
		for j := i + 1; j < len(reviewers); j++ {
			record, found := k.GetCoVotingRecord(ctx, reviewers[i], reviewers[j])
			if !found {
				continue
			}

			// Require at least 5 co-reviews before flagging
			if record.TotalPairCount < 5 {
				continue
			}

			// Check if same-vote ratio exceeds threshold (basis points)
			// sameVoteCount * 10000 / totalPairCount >= threshold
			ratio := record.SameVoteCount * 10000 / record.TotalPairCount
			if ratio >= threshold {
				return true, nil
			}
		}
	}

	return false, nil
}

// UpdateCoVotingRecords updates co-voting records for all voter pairs in a finalized session.
func (k Keeper) UpdateCoVotingRecords(ctx context.Context, session *types.ReviewSession) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := sdkCtx.BlockHeight()

	// Build a map of reviewer -> decision for quick lookup
	voteMap := make(map[string]types.ReviewVoteDecision)
	for _, vote := range session.Votes {
		voteMap[vote.Reviewer] = vote.Decision
	}

	// For each pair of voters, update the co-voting record
	voters := make([]string, 0, len(voteMap))
	for addr := range voteMap {
		voters = append(voters, addr)
	}
	sort.Strings(voters) // deterministic ordering

	for i := 0; i < len(voters); i++ {
		for j := i + 1; j < len(voters); j++ {
			addrA, addrB := voters[i], voters[j]

			record, found := k.GetCoVotingRecord(ctx, addrA, addrB)
			if !found {
				// Canonical order: GetCoVotingRecordKey handles ordering
				record = types.CoVotingRecord{
					ReviewerA: addrA,
					ReviewerB: addrB,
				}
				// Ensure canonical order matches key function
				if addrA > addrB {
					record.ReviewerA = addrB
					record.ReviewerB = addrA
				}
			}

			record.TotalPairCount++
			if voteMap[addrA] == voteMap[addrB] {
				record.SameVoteCount++
			}
			record.LastUpdated = height

			if err := k.SetCoVotingRecord(ctx, record); err != nil {
				return fmt.Errorf("failed to update co-voting record for %s/%s: %w", addrA, addrB, err)
			}
		}
	}

	return nil
}

// ============================================================================
// 4. Bond Management
// ============================================================================

// CollectReviewerBond escrows a reviewer's bond for a specific review assignment.
func (k Keeper) CollectReviewerBond(ctx context.Context, reviewer sdk.AccAddress, contributionID uint64, bond sdk.Coin) error {
	if bond.IsZero() {
		return nil
	}

	// Check balance
	balance := k.bankKeeper.GetBalance(ctx, reviewer, bond.Denom)
	if balance.Amount.LT(bond.Amount) {
		return types.ErrInsufficientReviewerBond.Wrapf(
			"need %s but only have %s", bond, balance)
	}

	// Transfer to module
	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, reviewer, types.ModuleName, sdk.NewCoins(bond))
	if err != nil {
		return types.ErrReviewBondEscrowFailed.Wrapf("failed to escrow reviewer bond: %s", err)
	}

	// Record escrow
	escrow := types.ReviewBondEscrow{
		Reviewer:       reviewer.String(),
		ContributionID: contributionID,
		Amount:         bond.String(),
	}

	return k.setReviewBondEscrow(ctx, reviewer.String(), contributionID, escrow)
}

// RefundReviewerBond returns the escrowed bond to the reviewer.
func (k Keeper) RefundReviewerBond(ctx context.Context, reviewer sdk.AccAddress, contributionID uint64) error {
	escrow, found := k.getReviewBondEscrow(ctx, reviewer.String(), contributionID)
	if !found {
		return nil // No bond to refund
	}

	coin, err := types.ParseCoinFromString(escrow.Amount)
	if err != nil {
		return types.ErrReviewBondRefundFailed.Wrapf("failed to parse escrowed amount: %s", err)
	}

	if coin.IsZero() {
		return nil
	}

	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, reviewer, sdk.NewCoins(coin))
	if err != nil {
		return types.ErrReviewBondRefundFailed.Wrapf("failed to refund reviewer bond: %s", err)
	}

	return k.deleteReviewBondEscrow(ctx, reviewer.String(), contributionID)
}

// SlashReviewerBond burns the escrowed bond for a reviewer who voted against the majority.
func (k Keeper) SlashReviewerBond(ctx context.Context, reviewer sdk.AccAddress, contributionID uint64) error {
	escrow, found := k.getReviewBondEscrow(ctx, reviewer.String(), contributionID)
	if !found {
		return nil // No bond to slash
	}

	coin, err := types.ParseCoinFromString(escrow.Amount)
	if err != nil {
		return fmt.Errorf("failed to parse escrowed amount: %w", err)
	}

	if coin.IsZero() {
		return nil
	}

	err = k.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(coin))
	if err != nil {
		return fmt.Errorf("failed to burn slashed reviewer bond: %w", err)
	}

	return k.deleteReviewBondEscrow(ctx, reviewer.String(), contributionID)
}

// ============================================================================
// 5. Processing Pipeline
// ============================================================================

// ProcessStartReview initiates a human review session for a contribution.
func (k Keeper) ProcessStartReview(ctx context.Context, msg *types.MsgStartReview) (*types.MsgStartReviewResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// 0. Authority check — only governance can start reviews
	if msg.Authority != k.authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.authority, msg.Authority)
	}

	// 1. Check EnableHumanReview
	if !params.EnableHumanReview {
		return nil, types.ErrReviewDisabled.Wrap("human review layer is not enabled via governance")
	}

	// 2. Verify contribution exists
	contribution, found := k.GetContribution(ctx, msg.ContributionId)
	if !found {
		return nil, types.ErrContributionNotFound.Wrapf("contribution %d not found", msg.ContributionId)
	}

	// 3. Check no existing review session
	_, exists := k.GetReviewSession(ctx, msg.ContributionId)
	if exists {
		return nil, types.ErrReviewNotActive.Wrapf("review session already exists for contribution %d", msg.ContributionId)
	}

	// 4. Select reviewers
	reviewerCount := params.VerifiersPerClaim
	reviewers, seed, err := k.SelectReviewers(ctx, msg.ContributionId, contribution.Contributor, reviewerCount)
	if err != nil {
		return nil, err
	}

	// 5. Check collusion risk — if flagged, try to select +2 more reviewers
	collusionRisk, err := k.CheckCollusionRisk(ctx, reviewers)
	if err != nil {
		return nil, fmt.Errorf("collusion check failed: %w", err)
	}

	if collusionRisk {
		expandedCount := reviewerCount + 2
		expandedReviewers, expandedSeed, err := k.SelectReviewers(ctx, msg.ContributionId, contribution.Contributor, expandedCount)
		if err != nil {
			// If we cannot expand, proceed with original set but log the collusion flag
			k.Logger().Info("collusion risk detected but cannot expand reviewer set",
				"contribution_id", msg.ContributionId,
				"error", err.Error())
		} else {
			reviewers = expandedReviewers
			seed = expandedSeed
		}
	}

	// 6. Parse bond and collect from all assigned reviewers
	bond, err := types.ParseCoinFromString(params.MinReviewerBond)
	if err != nil {
		return nil, fmt.Errorf("failed to parse min_reviewer_bond: %w", err)
	}

	for _, reviewerAddr := range reviewers {
		addr, err := sdk.AccAddressFromBech32(reviewerAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid reviewer address %s: %w", reviewerAddr, err)
		}
		if err := k.CollectReviewerBond(ctx, addr, msg.ContributionId, bond); err != nil {
			return nil, err
		}
	}

	// 7. Create ReviewSession
	startHeight := sdkCtx.BlockHeight()
	endHeight := startHeight + params.ReviewVotePeriod

	session := types.ReviewSession{
		ContributionID:    msg.ContributionId,
		Status:            types.ReviewStatusInReview,
		AssignedReviewers: reviewers,
		Votes:             []types.ReviewVote{},
		StartHeight:       startHeight,
		EndHeight:         endHeight,
		RandomSeed:        seed,
	}

	if err := k.SetReviewSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to store review session: %w", err)
	}

	// 8. Update contribution review status
	contribution.ReviewStatus = uint32(types.ReviewStatusInReview)
	if err := k.SetContribution(ctx, contribution); err != nil {
		return nil, fmt.Errorf("failed to update contribution review status: %w", err)
	}

	// 9. Set pending review index for EndBlocker auto-finalization
	if err := k.SetPendingReviewIndex(ctx, endHeight, msg.ContributionId); err != nil {
		return nil, fmt.Errorf("failed to set pending review index: %w", err)
	}

	// 10. Emit event
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_review_started",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", msg.ContributionId)),
			sdk.NewAttribute("reviewers_count", fmt.Sprintf("%d", len(reviewers))),
			sdk.NewAttribute("end_height", fmt.Sprintf("%d", endHeight)),
			sdk.NewAttribute("collusion_flagged", fmt.Sprintf("%t", collusionRisk)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Authority),
		),
	})

	// 11. Update unified claim status
	k.TransitionClaimStatus(ctx, msg.ContributionId, types.ClaimStatusInReview)

	// 12. Return response
	return &types.MsgStartReviewResponse{
		ReviewersAssigned: reviewers,
	}, nil
}

// ProcessCastReviewVote handles a reviewer casting their vote on a contribution.
func (k Keeper) ProcessCastReviewVote(ctx context.Context, msg *types.MsgCastReviewVote) (*types.MsgCastReviewVoteResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// 1. Get review session
	session, found := k.GetReviewSession(ctx, msg.ContributionId)
	if !found || session.Status != types.ReviewStatusInReview {
		return nil, types.ErrReviewNotActive.Wrapf("no active review for contribution %d", msg.ContributionId)
	}

	// 2. Check voting period not expired
	blockHeight := sdkCtx.BlockHeight()
	if blockHeight > session.EndHeight {
		return nil, types.ErrReviewPeriodExpired.Wrapf(
			"review period ended at height %d, current height %d", session.EndHeight, blockHeight)
	}

	// 3. Check signer is assigned reviewer
	if !session.IsAssignedReviewer(msg.Reviewer) {
		return nil, types.ErrNotAssignedReviewer.Wrapf(
			"address %s is not assigned to review contribution %d", msg.Reviewer, msg.ContributionId)
	}

	// 4. Check not already voted
	if session.HasVoted(msg.Reviewer) {
		return nil, types.ErrReviewAlreadyVoted.Wrapf(
			"reviewer %s has already voted on contribution %d", msg.Reviewer, msg.ContributionId)
	}

	// 5. Create ReviewVote and append
	vote := types.ReviewVote{
		Reviewer:            msg.Reviewer,
		Decision:            types.ReviewVoteDecision(msg.Decision),
		OriginalityOverride: types.OriginalityOverride(msg.OriginalityOverride),
		QualityScore:        msg.QualityScore,
		NotesPointer:        msg.NotesPointer,
		ParentClaimOverride: msg.ParentClaimOverride,
		BlockHeight:         blockHeight,
	}
	session.Votes = append(session.Votes, vote)

	// 6. Save session
	if err := k.SetReviewSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save review session: %w", err)
	}

	// 7. Emit event
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_review_vote_cast",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", msg.ContributionId)),
			sdk.NewAttribute("reviewer", msg.Reviewer),
			sdk.NewAttribute("decision", types.ReviewVoteDecision(msg.Decision).String()),
			sdk.NewAttribute("quality_score", fmt.Sprintf("%d", msg.QualityScore)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Reviewer),
		),
	})

	// 8. Fast Path: If all reviewers have voted, finalize immediately
	if session.AllVotesCast() {
		// We ignore the return values/error here to ensure the vote itself isn't rolled back
		// if finalization fails (though it shouldn't). The session remains IN_REVIEW
		// and will be picked up by EndBlocker or manual finalization if this fails.
		// The finalize method handles its own state saving and event emission.
		_, _ = k.finalizeReviewSession(ctx, &session)
	}

	// 9. Return response
	return &types.MsgCastReviewVoteResponse{}, nil
}

// ProcessFinalizeReview tallies votes and finalizes a review session.
func (k Keeper) ProcessFinalizeReview(ctx context.Context, msg *types.MsgFinalizeReview) (*types.MsgFinalizeReviewResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	session, found := k.GetReviewSession(ctx, msg.ContributionId)
	if !found || session.Status != types.ReviewStatusInReview {
		return nil, types.ErrReviewNotActive.Wrapf("no active review for contribution %d", msg.ContributionId)
	}

	blockHeight := sdkCtx.BlockHeight()

	// Check period expired OR all votes cast
	if blockHeight <= session.EndHeight && !session.AllVotesCast() {
		return nil, fmt.Errorf("review period not expired and not all votes cast (height %d <= end %d, votes %d/%d)",
			blockHeight, session.EndHeight, len(session.Votes), len(session.AssignedReviewers))
	}

	accepted, err := k.finalizeReviewSession(ctx, &session)
	if err != nil {
		return nil, err
	}

	return &types.MsgFinalizeReviewResponse{
		FinalDecision: uint32(session.FinalDecision),
		Accepted:      accepted,
	}, nil
}

// finalizeReviewSession is the shared finalization logic used by both ProcessFinalizeReview
// and FinalizeExpiredReviews (EndBlocker). It tallies votes, applies overrides, settles bonds,
// and updates all related state.
func (k Keeper) finalizeReviewSession(ctx context.Context, session *types.ReviewSession) (bool, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Tally votes
	var acceptCount, rejectCount uint32
	var qualitySum uint32
	overrideCounts := make(map[types.OriginalityOverride]uint32)
	parentClaimCounts := make(map[uint64]uint32)

	for _, vote := range session.Votes {
		switch vote.Decision {
		case types.ReviewVoteAccept:
			acceptCount++
		case types.ReviewVoteReject:
			rejectCount++
			// ReviewVoteRequestInfo counts toward neither
		}
		qualitySum += vote.QualityScore
		overrideCounts[vote.OriginalityOverride]++
		if vote.ParentClaimOverride > 0 {
			parentClaimCounts[vote.ParentClaimOverride]++
		}
	}

	// Check quorum: len(votes) >= ceil(len(assigned) * ReviewQuorumPct / 100)
	assignedCount := uint32(len(session.AssignedReviewers))
	quorumRequired := (assignedCount*params.ReviewQuorumPct + 99) / 100 // ceil division
	quorumMet := uint32(len(session.Votes)) >= quorumRequired

	// Determine final decision
	var finalDecision types.ReviewVoteDecision
	accepted := false

	if !quorumMet {
		// Quorum not met defaults to REJECTED
		finalDecision = types.ReviewVoteReject
	} else if acceptCount > rejectCount {
		finalDecision = types.ReviewVoteAccept
		accepted = true
	} else {
		finalDecision = types.ReviewVoteReject
	}

	// Determine majority originality override
	overrideApplied := types.OverrideKeepAI
	var maxOverrideCount uint32
	for override, count := range overrideCounts {
		if override == types.OverrideKeepAI {
			continue
		}
		if count > maxOverrideCount {
			maxOverrideCount = count
			overrideApplied = override
		}
	}
	// If no non-KeepAI overrides, keep AI decision
	if maxOverrideCount == 0 {
		overrideApplied = types.OverrideKeepAI
	}

	// Compute average quality score
	var avgQuality uint32
	if len(session.Votes) > 0 {
		avgQuality = qualitySum / uint32(len(session.Votes))
	}

	// Determine majority parent claim override
	var majorityParentClaim uint64
	var maxParentCount uint32
	for parentID, count := range parentClaimCounts {
		if count > maxParentCount {
			maxParentCount = count
			majorityParentClaim = parentID
		}
	}

	// Update session fields
	session.Status = types.ReviewStatusAccepted
	if !accepted {
		session.Status = types.ReviewStatusRejected
	}
	session.FinalDecision = finalDecision
	session.OverrideApplied = overrideApplied
	session.FinalQuality = avgQuality

	// Apply override to contribution
	contribution, found := k.GetContribution(ctx, session.ContributionID)
	if found {
		// Apply originality override
		switch overrideApplied {
		case types.OverrideDerivativeFalsePositive:
			// AI said derivative, human says original
			contribution.IsDerivative = false
		case types.OverrideNotDerivativeFalseNegative:
			// AI missed derivative, human flags it
			contribution.IsDerivative = true
		}

		// Apply parent claim override if majority voted
		if majorityParentClaim > 0 {
			contribution.ParentClaimId = majorityParentClaim
		}

		// Update contribution review status
		if accepted {
			contribution.ReviewStatus = uint32(types.ReviewStatusAccepted)
		} else {
			contribution.ReviewStatus = uint32(types.ReviewStatusRejected)
		}

		if err := k.SetContribution(ctx, contribution); err != nil {
			return false, fmt.Errorf("failed to update contribution after review: %w", err)
		}

		// Register accepted contribution in provenance registry (best-effort, never blocks finalization)
		if accepted {
			if err := k.RegisterProvenance(ctx, contribution, session); err != nil {
				k.Logger().Error("provenance registration failed",
					"contribution_id", session.ContributionID,
					"error", err.Error())
			}
		}

		// Trigger reward pipeline for accepted contributions (best-effort)
		// SECURITY: Never reward duplicates even if they somehow pass review
		if accepted && contribution.Contributor != "" && contribution.DuplicateOf == 0 {
			rewardCtx := types.RewardContext{
				ClaimID:         session.ContributionID,
				Contributor:     contribution.Contributor,
				Category:        contribution.Ctype,
				QualityScore:    math.LegacyNewDec(int64(avgQuality)).Quo(math.LegacyNewDec(10)), // normalize 0-100 to 0-10
				BaseReward:      params.BaseRewardUnit,
				SimilarityScore: k.GetSimilarityScore(ctx, session.ContributionID),
				IsDuplicate:     contribution.DuplicateOf > 0,
				IsDerivative:    contribution.IsDerivative,
				ParentClaimID:   contribution.ParentClaimId,
				ReviewOverride:  overrideApplied,
			}

			output, calcErr := k.CalculateReward(ctx, rewardCtx)
			if calcErr != nil {
				k.Logger().Error("reward calculation failed",
					"contribution_id", session.ContributionID,
					"error", calcErr.Error())
			} else if output.FinalRewardAmount.IsPositive() {
				// Use ARVS if enabled, otherwise fall back to legacy distribution
				if params.EnableARVS {
					verifierConf := math.LegacyNewDec(int64(session.FinalQuality)).Quo(math.LegacyNewDec(100))
					if distErr := k.DistributeRewardsARVS(ctx, output, contribution.Contributor,
						rewardCtx.SimilarityScore, verifierConf,
						contribution.Ctype, contribution.IsDerivative); distErr != nil {
						k.Logger().Error("ARVS reward distribution failed",
							"contribution_id", session.ContributionID,
							"error", distErr.Error())
					}
				} else {
					if distErr := k.DistributeRewards(ctx, output, contribution.Contributor); distErr != nil {
						k.Logger().Error("reward distribution failed",
							"contribution_id", session.ContributionID,
							"error", distErr.Error())
					}
				}

				k.UpdateContributorStatsOnAccept(ctx, contribution.Contributor, contribution.IsDerivative, rewardCtx.SimilarityScore)
			}
		}
	}

	// Bond settlement: determine majority voters
	majorityVoters := make(map[string]bool)
	minorityVoters := make(map[string]bool)
	votedSet := make(map[string]bool)

	for _, vote := range session.Votes {
		votedSet[vote.Reviewer] = true
		if vote.Decision == finalDecision {
			majorityVoters[vote.Reviewer] = true
		} else {
			minorityVoters[vote.Reviewer] = true
		}
	}

	// Slash minority voters and non-voters, refund majority
	for _, reviewerAddr := range session.AssignedReviewers {
		addr, err := sdk.AccAddressFromBech32(reviewerAddr)
		if err != nil {
			continue
		}

		if majorityVoters[reviewerAddr] {
			// Refund majority voters
			if err := k.RefundReviewerBond(ctx, addr, session.ContributionID); err != nil {
				k.Logger().Error("failed to refund reviewer bond",
					"reviewer", reviewerAddr,
					"contribution_id", session.ContributionID,
					"error", err.Error())
			}
		} else {
			// Slash minority voters and non-voters
			if err := k.SlashReviewerBond(ctx, addr, session.ContributionID); err != nil {
				k.Logger().Error("failed to slash reviewer bond",
					"reviewer", reviewerAddr,
					"contribution_id", session.ContributionID,
					"error", err.Error())
			}
		}

		// Update reviewer profiles
		profile, profileFound := k.GetReviewerProfile(ctx, reviewerAddr)
		if !profileFound {
			profile = types.ReviewerProfile{
				Address: reviewerAddr,
			}
		}

		profile.TotalReviews++
		profile.LastReviewHeight = sdkCtx.BlockHeight()

		if majorityVoters[reviewerAddr] {
			profile.AcceptedReviews++
		} else if minorityVoters[reviewerAddr] || !votedSet[reviewerAddr] {
			profile.RejectedReviews++
			profile.SlashedCount++
			// Suspend if slashed too many times
			if profile.SlashedCount >= 3 {
				profile.Suspended = true
			}
		}

		if err := k.SetReviewerProfile(ctx, profile); err != nil {
			k.Logger().Error("failed to update reviewer profile",
				"reviewer", reviewerAddr,
				"error", err.Error())
		}
	}

	// Update co-voting records
	if err := k.UpdateCoVotingRecords(ctx, session); err != nil {
		k.Logger().Error("failed to update co-voting records",
			"contribution_id", session.ContributionID,
			"error", err.Error())
	}

	// Save session
	if err := k.SetReviewSession(ctx, *session); err != nil {
		return false, fmt.Errorf("failed to save finalized review session: %w", err)
	}

	// Delete pending review index
	if err := k.DeletePendingReviewIndex(ctx, session.EndHeight, session.ContributionID); err != nil {
		k.Logger().Error("failed to delete pending review index",
			"contribution_id", session.ContributionID,
			"error", err.Error())
	}

	// Update unified claim status
	if accepted {
		k.TransitionClaimStatus(ctx, session.ContributionID, types.ClaimStatusAccepted)
	} else {
		k.TransitionClaimStatus(ctx, session.ContributionID, types.ClaimStatusRejected)
	}

	// Emit event
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_review_finalized",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", session.ContributionID)),
			sdk.NewAttribute("final_decision", finalDecision.String()),
			sdk.NewAttribute("accepted", fmt.Sprintf("%t", accepted)),
			sdk.NewAttribute("quorum_met", fmt.Sprintf("%t", quorumMet)),
			sdk.NewAttribute("votes_cast", fmt.Sprintf("%d", len(session.Votes))),
			sdk.NewAttribute("override_applied", overrideApplied.String()),
			sdk.NewAttribute("avg_quality", fmt.Sprintf("%d", avgQuality)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		),
	})

	return accepted, nil
}

// ProcessAppealReview handles filing an appeal against a review outcome.
func (k Keeper) ProcessAppealReview(ctx context.Context, msg *types.MsgAppealReview) (*types.MsgAppealReviewResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// 1. Get review session (must be ACCEPTED or REJECTED)
	session, found := k.GetReviewSession(ctx, msg.ContributionId)
	if !found {
		return nil, types.ErrReviewNotActive.Wrapf("no review session for contribution %d", msg.ContributionId)
	}
	if session.Status != types.ReviewStatusAccepted && session.Status != types.ReviewStatusRejected {
		return nil, types.ErrReviewNotActive.Wrapf(
			"review session must be ACCEPTED or REJECTED to appeal, current status: %s", session.Status)
	}

	// 2. Check no existing appeal
	if session.AppealID != 0 {
		return nil, types.ErrAppealAlreadyFiled.Wrapf(
			"appeal already filed for contribution %d (appeal ID: %d)", msg.ContributionId, session.AppealID)
	}

	// 3. Parse and collect appeal bond
	appealBond, err := types.ParseCoinFromString(params.AppealBond)
	if err != nil {
		return nil, fmt.Errorf("failed to parse appeal_bond: %w", err)
	}

	appellantAddr, err := sdk.AccAddressFromBech32(msg.Appellant)
	if err != nil {
		return nil, fmt.Errorf("invalid appellant address: %w", err)
	}

	// Check balance
	balance := k.bankKeeper.GetBalance(ctx, appellantAddr, appealBond.Denom)
	if balance.Amount.LT(appealBond.Amount) {
		return nil, types.ErrInvalidAppealBond.Wrapf(
			"need %s but only have %s", appealBond, balance)
	}

	// Collect bond
	err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, appellantAddr, types.ModuleName, sdk.NewCoins(appealBond))
	if err != nil {
		return nil, fmt.Errorf("failed to collect appeal bond: %w", err)
	}

	// 4. Get next appeal ID and create appeal
	appealID := k.GetNextAppealID(ctx)
	if err := k.setNextAppealID(ctx, appealID+1); err != nil {
		return nil, fmt.Errorf("failed to increment appeal ID: %w", err)
	}

	appeal := types.ReviewAppeal{
		AppealID:       appealID,
		ContributionID: msg.ContributionId,
		Appellant:      msg.Appellant,
		Reason:         msg.Reason,
		AppealBond:     appealBond.String(),
		FiledAtHeight:  sdkCtx.BlockHeight(),
	}

	if err := k.SetReviewAppeal(ctx, appeal); err != nil {
		return nil, fmt.Errorf("failed to store appeal: %w", err)
	}

	// 5. Set session status = APPEALED, set AppealID
	session.Status = types.ReviewStatusAppealed
	session.AppealID = appealID

	if err := k.SetReviewSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update review session with appeal: %w", err)
	}

	// Update contribution review status and freeze vesting
	contribution, found := k.GetContribution(ctx, msg.ContributionId)
	if found {
		contribution.ReviewStatus = uint32(types.ReviewStatusAppealed)
		if err := k.SetContribution(ctx, contribution); err != nil {
			return nil, fmt.Errorf("failed to update contribution review status: %w", err)
		}

		// Freeze vesting during appeal (best-effort)
		if freezeErr := k.PauseVesting(ctx, contribution.Contributor, msg.ContributionId); freezeErr != nil {
			k.Logger().Error("failed to pause vesting during appeal",
				"contribution_id", msg.ContributionId,
				"error", freezeErr.Error())
		}
	}

	// 6. Update unified claim status
	k.TransitionClaimStatus(ctx, msg.ContributionId, types.ClaimStatusDisputed)

	// 7. Emit event
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_review_appealed",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", msg.ContributionId)),
			sdk.NewAttribute("appeal_id", fmt.Sprintf("%d", appealID)),
			sdk.NewAttribute("appellant", msg.Appellant),
			sdk.NewAttribute("appeal_bond", appealBond.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Appellant),
		),
	})

	return &types.MsgAppealReviewResponse{
		AppealId: appealID,
	}, nil
}

// ProcessResolveAppeal handles governance resolution of an appeal.
func (k Keeper) ProcessResolveAppeal(ctx context.Context, msg *types.MsgResolveAppeal) (*types.MsgResolveAppealResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// 1. Authority check
	if msg.Authority != k.authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.authority, msg.Authority)
	}

	// 2. Get appeal, verify not resolved
	appeal, found := k.GetReviewAppeal(ctx, msg.AppealId)
	if !found {
		return nil, types.ErrAppealNotFound.Wrapf("appeal %d not found", msg.AppealId)
	}
	if appeal.Resolved {
		return nil, types.ErrAppealAlreadyResolved.Wrapf("appeal %d already resolved", msg.AppealId)
	}

	// Get session
	session, found := k.GetReviewSession(ctx, appeal.ContributionID)
	if !found {
		return nil, types.ErrReviewNotActive.Wrapf("review session not found for contribution %d", appeal.ContributionID)
	}

	// Parse appeal bond
	appealBondCoin, err := types.ParseCoinFromString(appeal.AppealBond)
	if err != nil {
		return nil, fmt.Errorf("failed to parse appeal bond: %w", err)
	}

	appellantAddr, err := sdk.AccAddressFromBech32(appeal.Appellant)
	if err != nil {
		return nil, fmt.Errorf("invalid appellant address: %w", err)
	}

	// Record original decision before we potentially change it
	originalDecision := session.FinalDecision

	if msg.Upheld {
		// Appeal upheld = original verdict stands, burn appeal bond
		if !appealBondCoin.IsZero() {
			if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(appealBondCoin)); err != nil {
				return nil, fmt.Errorf("failed to burn appeal bond: %w", err)
			}
		}

		appeal.Resolved = true
		appeal.Upheld = true
		appeal.ResolvedAtHeight = sdkCtx.BlockHeight()
		appeal.ResolverNotes = msg.ResolverNotes

		// Revert session status to original final decision
		if originalDecision == types.ReviewVoteAccept {
			session.Status = types.ReviewStatusAccepted

			// Resume vesting — original acceptance stands
			if contrib, cFound := k.GetContribution(ctx, appeal.ContributionID); cFound {
				_ = k.ResumeVesting(ctx, contrib.Contributor, appeal.ContributionID)
			}
		} else {
			session.Status = types.ReviewStatusRejected
		}
	} else {
		// Appeal overturned = refund appeal bond, reverse decision
		if !appealBondCoin.IsZero() {
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, appellantAddr, sdk.NewCoins(appealBondCoin)); err != nil {
				return nil, fmt.Errorf("failed to refund appeal bond: %w", err)
			}
		}

		appeal.Resolved = true
		appeal.Upheld = false
		appeal.ResolvedAtHeight = sdkCtx.BlockHeight()
		appeal.ResolverNotes = msg.ResolverNotes

		// Reverse the session status
		if originalDecision == types.ReviewVoteAccept {
			session.Status = types.ReviewStatusRejected
			session.FinalDecision = types.ReviewVoteReject
		} else {
			session.Status = types.ReviewStatusAccepted
			session.FinalDecision = types.ReviewVoteAccept
		}

		// Update contribution and handle vesting
		contribution, found := k.GetContribution(ctx, appeal.ContributionID)
		if found {
			if session.Status == types.ReviewStatusAccepted {
				contribution.ReviewStatus = uint32(types.ReviewStatusAccepted)
				// Overturned from REJECTED to ACCEPTED — resume vesting if it exists
				_ = k.ResumeVesting(ctx, contribution.Contributor, appeal.ContributionID)
			} else {
				contribution.ReviewStatus = uint32(types.ReviewStatusRejected)
				// Overturned from ACCEPTED to REJECTED — full clawback (vesting + immediate)
				if clawErr := k.ExecuteClawback(ctx, appeal.ContributionID, "appeal_overturn_accepted_to_rejected", msg.Authority); clawErr != nil {
					// ExecuteClawback may fail if already clawed back; fall back to vesting-only
					if _, vestErr := k.ClawbackVesting(ctx, contribution.Contributor, appeal.ContributionID); vestErr != nil {
						k.Logger().Error("failed to clawback after appeal overturn",
							"contribution_id", appeal.ContributionID,
							"clawback_error", clawErr.Error(),
							"vesting_error", vestErr.Error())
					}
				}

				// ARVS clawback (if ARVS schedule exists)
				params := k.GetParams(ctx)
				if params.EnableARVS {
					unvested, arvsErr := k.ClawbackARVSVesting(ctx, contribution.Contributor, appeal.ContributionID)
					if arvsErr != nil {
						k.Logger().Error("failed to clawback ARVS vesting after appeal overturn",
							"contribution_id", appeal.ContributionID,
							"error", arvsErr.Error())
					} else if params.ARVSEnableBounty && unvested.IsPositive() {
						// Distribute bounty to appellant (challenger) + burn + treasury + reviewer pool
						if bountyErr := k.DistributeBounty(ctx, unvested, appeal.Appellant, appeal.ContributionID); bountyErr != nil {
							k.Logger().Error("bounty distribution failed",
								"contribution_id", appeal.ContributionID,
								"error", bountyErr.Error())
						}
					}
				}
			}
			if err := k.SetContribution(ctx, contribution); err != nil {
				return nil, fmt.Errorf("failed to update contribution after appeal: %w", err)
			}
		}

		// Slash original majority voters (those who voted with the now-reversed decision)
		for _, vote := range session.Votes {
			if vote.Decision == originalDecision {
				addr, err := sdk.AccAddressFromBech32(vote.Reviewer)
				if err != nil {
					continue
				}

				// Update profile: increment slash count, check suspension
				profile, profileFound := k.GetReviewerProfile(ctx, vote.Reviewer)
				if !profileFound {
					profile = types.ReviewerProfile{
						Address: vote.Reviewer,
					}
				}
				profile.SlashedCount++
				if profile.SlashedCount >= 3 {
					profile.Suspended = true
				}

				if err := k.SetReviewerProfile(ctx, profile); err != nil {
					k.Logger().Error("failed to update reviewer profile after appeal",
						"reviewer", vote.Reviewer,
						"error", err.Error())
				}

				// Note: original bonds were already settled during finalization.
				// The slash here is a profile penalty, not a bond slash.
				_ = addr // addr validated above
			}
		}
	}

	// Save appeal
	if err := k.SetReviewAppeal(ctx, appeal); err != nil {
		return nil, fmt.Errorf("failed to save resolved appeal: %w", err)
	}

	// Save session
	if err := k.SetReviewSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session after appeal resolution: %w", err)
	}

	// Emit event
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_appeal_resolved",
			sdk.NewAttribute("appeal_id", fmt.Sprintf("%d", msg.AppealId)),
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", appeal.ContributionID)),
			sdk.NewAttribute("upheld", fmt.Sprintf("%t", msg.Upheld)),
			sdk.NewAttribute("new_status", session.Status.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Authority),
		),
	})

	// Update unified claim status
	k.TransitionClaimStatus(ctx, appeal.ContributionID, types.ClaimStatusResolved)

	return &types.MsgResolveAppealResponse{}, nil
}

// ============================================================================
// 6. EndBlocker Integration
// ============================================================================

// FinalizeExpiredReviews iterates pending review index entries for reviews that have
// exceeded their voting period and auto-finalizes them. This is called from EndBlocker.
// It never panics — errors are logged and iteration continues.
func (k Keeper) FinalizeExpiredReviews(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	store := k.storeService.OpenKVStore(ctx)

	// Iterate from the start of the pending review index up to and including currentHeight.
	// Key format: 0x22 | end_height (big endian uint64) | contribution_id (big endian uint64)
	prefix := types.KeyPrefixPendingReviewIndex

	// End key: prefix + (currentHeight + 1) — exclusive upper bound
	endKey := append(prefix, sdk.Uint64ToBigEndian(uint64(currentHeight+1))...)

	iterator, err := store.Iterator(prefix, endKey)
	if err != nil {
		k.Logger().Error("failed to create pending review iterator", "error", err.Error())
		return nil // never panic in EndBlocker
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()

		// Parse contribution ID from key: prefix(1) + endHeight(8) + contributionID(8)
		if len(key) < len(prefix)+16 {
			k.Logger().Error("invalid pending review index key length", "key_len", len(key))
			continue
		}

		contributionIDBytes := key[len(prefix)+8:]
		contributionID := sdk.BigEndianToUint64(contributionIDBytes)

		// Load session
		session, found := k.GetReviewSession(ctx, contributionID)
		if !found {
			k.Logger().Info("pending review index points to missing session, cleaning up",
				"contribution_id", contributionID)
			// Clean up orphaned index entry
			endHeightBytes := key[len(prefix) : len(prefix)+8]
			endHeight := int64(sdk.BigEndianToUint64(endHeightBytes))
			if delErr := k.DeletePendingReviewIndex(ctx, endHeight, contributionID); delErr != nil {
				k.Logger().Error("failed to delete orphaned pending review index",
					"contribution_id", contributionID,
					"error", delErr.Error())
			}
			continue
		}

		// Only finalize sessions still in review
		if session.Status != types.ReviewStatusInReview {
			// Clean up stale index entry
			endHeightBytes := key[len(prefix) : len(prefix)+8]
			endHeight := int64(sdk.BigEndianToUint64(endHeightBytes))
			if delErr := k.DeletePendingReviewIndex(ctx, endHeight, contributionID); delErr != nil {
				k.Logger().Error("failed to delete stale pending review index",
					"contribution_id", contributionID,
					"error", delErr.Error())
			}
			continue
		}

		// Auto-finalize
		_, err := k.finalizeReviewSession(ctx, &session)
		if err != nil {
			k.Logger().Error("failed to auto-finalize review session",
				"contribution_id", contributionID,
				"error", err.Error())
			// Continue — do not panic in EndBlocker
			continue
		}

		k.Logger().Info("auto-finalized expired review session",
			"contribution_id", contributionID,
			"final_decision", session.FinalDecision.String(),
			"votes_cast", len(session.Votes))
	}

	return nil
}
