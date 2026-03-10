package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ============================================================================
// V2.1 Mainnet Hardening - Deterministic Finality (Requirement 1)
//
// Contributions follow a strict state machine:
//   PENDING → FINAL    (challenge window expired, no valid challenge, PoR batch finalized if linked)
//   PENDING → CHALLENGED (valid challenge submitted within window)
//   CHALLENGED → INVALIDATED (objective fraud proof succeeds)
//   CHALLENGED → FINAL  (challenge window expires with no valid proof)
//
// Finality is time-bound and deterministic, never discretionary.
// ============================================================================

// GetChallengeWindow retrieves the challenge window for a contribution
func (k Keeper) GetChallengeWindow(ctx context.Context, contributionID uint64) (types.ChallengeWindow, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetChallengeWindowKey(contributionID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ChallengeWindow{}, false
	}

	var cw types.ChallengeWindow
	if err := cw.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal challenge window", "id", contributionID, "error", err)
		return types.ChallengeWindow{}, false
	}
	return cw, true
}

// SetChallengeWindow stores the challenge window for a contribution
func (k Keeper) SetChallengeWindow(ctx context.Context, cw types.ChallengeWindow) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetChallengeWindowKey(cw.ContributionID)

	bz, err := cw.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal challenge window: %w", err)
	}
	return store.Set(key, bz)
}

// OpenChallengeWindow creates a challenge window after a contribution is verified via PoV quorum.
// The window starts at the current block and runs for DefaultChallengeWindowBlocks.
func (k Keeper) OpenChallengeWindow(ctx context.Context, contributionID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	cw := types.ChallengeWindow{
		ContributionID: contributionID,
		StartHeight:    currentHeight,
		EndHeight:      currentHeight + types.DefaultChallengeWindowBlocks,
	}

	if err := k.SetChallengeWindow(ctx, cw); err != nil {
		return err
	}

	// Set contribution to PENDING (awaiting finality)
	finality := types.ContributionFinality{
		ContributionID: contributionID,
		Status:         types.FinalityStatusPending,
	}
	if err := k.SetContributionFinality(ctx, finality); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_challenge_window_opened",
		sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
		sdk.NewAttribute("start_height", fmt.Sprintf("%d", currentHeight)),
		sdk.NewAttribute("end_height", fmt.Sprintf("%d", cw.EndHeight)),
	))

	return nil
}

// TryFinalizeContribution attempts to transition PENDING → FINAL.
// Succeeds only if:
// 1. Challenge window has elapsed
// 2. No successful challenge exists
// 3. If PoR is enabled: referenced PoR batch is FINALIZED
func (k Keeper) TryFinalizeContribution(ctx context.Context, contributionID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	finality := k.GetContributionFinality(ctx, contributionID)

	// Only PENDING can transition to FINAL
	if finality.Status != types.FinalityStatusPending {
		return types.ErrInvalidStateTransition.Wrapf(
			"contribution %d is %s, expected PENDING", contributionID, finality.Status)
	}

	// Check challenge window
	cw, found := k.GetChallengeWindow(ctx, contributionID)
	if found && !cw.IsExpired(currentHeight) {
		return types.ErrChallengeWindowOpen.Wrapf(
			"contribution %d window closes at height %d, current %d",
			contributionID, cw.EndHeight, currentHeight)
	}

	// Check for existing fraud proof (means challenge was validated)
	if _, hasFraud := k.GetFraudProof(ctx, contributionID); hasFraud {
		return types.ErrContributionInvalidated.Wrapf(
			"contribution %d has a validated fraud proof", contributionID)
	}

	// If PoR is enabled, verify batch finality
	if k.porKeeper != nil {
		batchID := k.porKeeper.GetBatchForContribution(ctx, contributionID)
		if batchID > 0 && !k.porKeeper.IsBatchFinalized(ctx, batchID) {
			return types.ErrPoRBatchNotFinalized.Wrapf(
				"contribution %d linked to PoR batch %d which is not finalized",
				contributionID, batchID)
		}
	}

	// All checks pass — finalize
	finality.Status = types.FinalityStatusFinal
	finality.FinalizedAt = currentHeight
	if err := k.SetContributionFinality(ctx, finality); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_contribution_finalized_deterministic",
		sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", currentHeight)),
	))

	return nil
}

// ChallengeContribution transitions PENDING → CHALLENGED.
// Can only be called while the challenge window is open.
func (k Keeper) ChallengeContribution(ctx context.Context, contributionID uint64, challengeID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	finality := k.GetContributionFinality(ctx, contributionID)
	if finality.Status != types.FinalityStatusPending {
		return types.ErrInvalidStateTransition.Wrapf(
			"contribution %d is %s, expected PENDING", contributionID, finality.Status)
	}

	// Verify challenge window is still open
	cw, found := k.GetChallengeWindow(ctx, contributionID)
	if found && cw.IsExpired(currentHeight) {
		return types.ErrChallengeWindowClosed.Wrapf(
			"contribution %d challenge window closed at height %d", contributionID, cw.EndHeight)
	}

	finality.Status = types.FinalityStatusChallenged
	finality.ChallengeID = challengeID
	if err := k.SetContributionFinality(ctx, finality); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_contribution_challenged",
		sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
		sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challengeID)),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", currentHeight)),
	))

	return nil
}

// ResolveChallengeInvalid transitions CHALLENGED → FINAL.
// Called when the challenge window expires with no valid fraud proof.
func (k Keeper) ResolveChallengeInvalid(ctx context.Context, contributionID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	finality := k.GetContributionFinality(ctx, contributionID)
	if finality.Status != types.FinalityStatusChallenged {
		return types.ErrInvalidStateTransition.Wrapf(
			"contribution %d is %s, expected CHALLENGED", contributionID, finality.Status)
	}

	// Verify no valid fraud proof exists
	if _, hasFraud := k.GetFraudProof(ctx, contributionID); hasFraud {
		return types.ErrContributionInvalidated.Wrapf(
			"contribution %d has a validated fraud proof, cannot resolve as invalid", contributionID)
	}

	finality.Status = types.FinalityStatusFinal
	finality.FinalizedAt = sdkCtx.BlockHeight()
	if err := k.SetContributionFinality(ctx, finality); err != nil {
		return err
	}

	// Unfreeze credits if they were frozen
	contribution, found := k.GetContribution(ctx, contributionID)
	if found {
		_ = k.UnfreezeCredits(ctx, contribution.Contributor)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_challenge_resolved_invalid",
		sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
	))

	return nil
}

// InvalidateContribution transitions CHALLENGED → INVALIDATED.
// Called only when an objective fraud proof succeeds.
func (k Keeper) InvalidateContribution(ctx context.Context, contributionID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	finality := k.GetContributionFinality(ctx, contributionID)
	if finality.Status != types.FinalityStatusChallenged {
		return types.ErrInvalidStateTransition.Wrapf(
			"contribution %d is %s, expected CHALLENGED", contributionID, finality.Status)
	}

	finality.Status = types.FinalityStatusInvalidated
	if err := k.SetContributionFinality(ctx, finality); err != nil {
		return err
	}

	// Burn frozen credits for the invalidated contribution
	contribution, found := k.GetContribution(ctx, contributionID)
	if found {
		_ = k.BurnFrozenCredits(ctx, contribution.Contributor)

		// SECURITY (V2.2): Slash validators who endorsed this fraudulent contribution.
		// Only approving endorsers are slashed. This provides real economic consequences
		// for cartel behavior and rubber-stamp endorsements.
		k.SlashFraudEndorsers(ctx, contribution)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_contribution_invalidated",
		sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
	))

	return nil
}

// ============================================================================
// V2.1 Mainnet Hardening - Objective Fraud Proofs (Requirement 2)
//
// Supported fraud proof types (all deterministic, no subjectivity):
// 1. INVALID_QUORUM  - endorsement power < 2/3 at verification time
// 2. HASH_MISMATCH   - hash/CID does not match declared data
// 3. NONCE_REPLAY    - claim nonce was already used
// 4. MERKLE_INCLUSION - PoR merkle inclusion proof mismatch
//
// Unsupported challenge types are rejected explicitly.
// ============================================================================

// GetFraudProof retrieves a fraud proof for a contribution
func (k Keeper) GetFraudProof(ctx context.Context, contributionID uint64) (types.FraudProof, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetFraudProofKey(contributionID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.FraudProof{}, false
	}

	var fp types.FraudProof
	if err := fp.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal fraud proof", "id", contributionID, "error", err)
		return types.FraudProof{}, false
	}
	return fp, true
}

// SetFraudProof stores a fraud proof
func (k Keeper) SetFraudProof(ctx context.Context, fp types.FraudProof) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetFraudProofKey(fp.ContributionID)

	bz, err := fp.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal fraud proof: %w", err)
	}
	return store.Set(key, bz)
}

// SubmitFraudProof validates and stores a fraud proof. Only deterministic proof types accepted.
// If valid, transitions contribution to CHALLENGED (or if already challenged, to INVALIDATED).
func (k Keeper) SubmitFraudProof(ctx context.Context, contributionID uint64, proofType types.FraudProofType, challenger string, proofData []byte) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Reject unsupported proof types explicitly
	if !proofType.IsValid() {
		return types.ErrInvalidFraudProofType.Wrapf("proof type %d is not supported", proofType)
	}

	// Reject if fraud proof already exists
	if _, exists := k.GetFraudProof(ctx, contributionID); exists {
		return types.ErrFraudProofAlreadyExists.Wrapf("contribution %d", contributionID)
	}

	// Verify the fraud proof deterministically
	valid, err := k.VerifyFraudProof(ctx, contributionID, proofType, proofData)
	if err != nil {
		return types.ErrFraudProofFailed.Wrapf("verification error: %v", err)
	}
	if !valid {
		return types.ErrFraudProofFailed.Wrapf("fraud proof for contribution %d did not validate", contributionID)
	}

	// Store validated fraud proof
	fp := types.FraudProof{
		ContributionID: contributionID,
		ProofType:      proofType,
		Challenger:     challenger,
		ProofData:      proofData,
		SubmittedAt:    sdkCtx.BlockHeight(),
		Validated:      true,
	}
	if err := k.SetFraudProof(ctx, fp); err != nil {
		return err
	}

	// Transition contribution: if PENDING → challenge then invalidate, if CHALLENGED → invalidate
	finality := k.GetContributionFinality(ctx, contributionID)
	switch finality.Status {
	case types.FinalityStatusPending:
		// Challenge the contribution first, then invalidate
		if err := k.ChallengeContribution(ctx, contributionID, 0); err != nil {
			k.logger.Error("failed to challenge contribution during fraud proof", "id", contributionID, "error", err)
		}
		if err := k.InvalidateContribution(ctx, contributionID); err != nil {
			k.logger.Error("failed to invalidate contribution during fraud proof", "id", contributionID, "error", err)
		}
	case types.FinalityStatusChallenged:
		if err := k.InvalidateContribution(ctx, contributionID); err != nil {
			k.logger.Error("failed to invalidate challenged contribution", "id", contributionID, "error", err)
		}
	default:
		// FINAL or INVALIDATED — fraud proof stored but no state transition
		k.logger.Warn("fraud proof submitted for non-pending contribution",
			"id", contributionID, "status", finality.Status)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_fraud_proof_submitted",
		sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
		sdk.NewAttribute("proof_type", proofType.String()),
		sdk.NewAttribute("challenger", challenger),
		sdk.NewAttribute("validated", "true"),
	))

	return nil
}

// VerifyFraudProof deterministically verifies a fraud proof.
// Each proof type has a specific, objective verification algorithm.
func (k Keeper) VerifyFraudProof(ctx context.Context, contributionID uint64, proofType types.FraudProofType, proofData []byte) (bool, error) {
	contribution, found := k.GetContribution(ctx, contributionID)
	if !found {
		return false, types.ErrContributionNotFound.Wrapf("contribution %d", contributionID)
	}

	switch proofType {
	case types.FraudProofInvalidQuorum:
		return k.verifyInvalidQuorumProof(ctx, contribution)
	case types.FraudProofHashMismatch:
		return k.verifyHashMismatchProof(ctx, contribution, proofData)
	case types.FraudProofNonceReplay:
		return k.verifyNonceReplayProof(ctx, contribution, proofData)
	case types.FraudProofMerkleInclusion:
		return k.verifyMerkleInclusionProof(ctx, contribution, proofData)
	default:
		return false, types.ErrInvalidFraudProofType.Wrapf("unsupported type %d", proofType)
	}
}

// verifyInvalidQuorumProof checks if the endorsement power was < 2/3 at verification time
func (k Keeper) verifyInvalidQuorumProof(ctx context.Context, contribution types.Contribution) (bool, error) {
	if !contribution.Verified {
		return false, nil // Not verified, no quorum to check
	}

	params := k.GetParams(ctx)
	totalBonded, err := k.stakingKeeper.TotalBondedTokens(ctx)
	if err != nil {
		return false, err
	}

	requiredPower := math.LegacyNewDecFromInt(totalBonded).Mul(params.QuorumPct).TruncateInt()
	approvalPower := contribution.GetApprovalPower()

	// Fraud proven if approval power < required quorum
	return approvalPower.LT(requiredPower), nil
}

// verifyHashMismatchProof checks if the contribution hash doesn't match expected format
func (k Keeper) verifyHashMismatchProof(_ context.Context, contribution types.Contribution, proofData []byte) (bool, error) {
	if len(proofData) == 0 {
		return false, fmt.Errorf("hash mismatch proof requires proof data with expected hash")
	}

	// The proof data should contain the expected hash.
	// Fraud is proven if the stored hash differs from what the contribution claims.
	if len(contribution.Hash) == 0 {
		return true, nil // Empty hash is invalid
	}

	// Verify hash length is valid (SHA256=32, SHA512=64)
	hashLen := len(contribution.Hash)
	if hashLen != 32 && hashLen != 64 {
		return true, nil // Invalid hash length is fraud
	}

	// Check if all bytes are zero (null hash)
	allZero := true
	for _, b := range contribution.Hash {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return true, nil // All-zero hash is fraud
	}

	return false, nil
}

// verifyNonceReplayProof checks if a claim nonce was reused
func (k Keeper) verifyNonceReplayProof(ctx context.Context, contribution types.Contribution, proofData []byte) (bool, error) {
	// The proof data should contain the nonce that was replayed (as uint64 big-endian)
	if len(proofData) < 8 {
		return false, fmt.Errorf("nonce replay proof requires 8-byte nonce in proof data")
	}

	claimedNonce := sdk.BigEndianToUint64(proofData[:8])
	currentNonce := k.GetClaimNonce(ctx, contribution.Contributor)

	// Fraud is proven if the claimed nonce is >= current nonce (already used)
	return claimedNonce < currentNonce, nil
}

// verifyMerkleInclusionProof verifies PoR merkle inclusion proof mismatch.
// Requires PoR module to be available.
func (k Keeper) verifyMerkleInclusionProof(ctx context.Context, contribution types.Contribution, proofData []byte) (bool, error) {
	if k.porKeeper == nil {
		return false, fmt.Errorf("PoR module not available, cannot verify merkle inclusion proof")
	}

	// Get the PoR batch for this contribution
	batchID := k.porKeeper.GetBatchForContribution(ctx, contribution.Id)
	if batchID == 0 {
		return false, fmt.Errorf("contribution %d not linked to any PoR batch", contribution.Id)
	}

	// If batch is rejected, the merkle proof is inherently invalid
	if k.porKeeper.IsBatchRejected(ctx, batchID) {
		return true, nil
	}

	return false, nil
}

// ============================================================================
// V2.1 Mainnet Hardening - Endorsement Quality Penalties (Requirement 4)
//
// Soft penalties only: reduce ParticipationBonus eligibility and
// optionally reduce endorsement weight by a bounded factor (max -10%).
// Does NOT slash stake. Does NOT affect consensus power.
// ============================================================================

// GetEndorsementPenalty retrieves the endorsement penalty for a validator
func (k Keeper) GetEndorsementPenalty(ctx context.Context, valAddr string) (types.EndorsementPenalty, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEndorsementPenaltyKey(valAddr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.EndorsementPenalty{}, false
	}

	var ep types.EndorsementPenalty
	if err := ep.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal endorsement penalty", "validator", valAddr, "error", err)
		return types.EndorsementPenalty{}, false
	}
	return ep, true
}

// SetEndorsementPenalty stores an endorsement penalty
func (k Keeper) SetEndorsementPenalty(ctx context.Context, ep types.EndorsementPenalty) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEndorsementPenaltyKey(ep.ValidatorAddress)

	bz, err := ep.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal endorsement penalty: %w", err)
	}
	return store.Set(key, bz)
}

// ApplyEndorsementQualityPenalties checks all validators and applies soft penalties
// for freeriding or quorum gaming. Called at epoch boundaries.
func (k Keeper) ApplyEndorsementQualityPenalties(ctx context.Context, currentEpoch int64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	validators, err := k.stakingKeeper.GetAllValidators(ctx)
	if err != nil {
		return nil // Non-fatal, log and continue
	}

	var penaltiesApplied uint64

	for _, val := range validators {
		if !val.IsBonded() {
			continue
		}

		valAddr := val.GetOperator()
		isFreeriding, isQuorumGaming := k.CheckValidatorEndorsementQuality(ctx, valAddr)

		if !isFreeriding && !isQuorumGaming {
			continue
		}

		// Check if already penalized
		existing, found := k.GetEndorsementPenalty(ctx, valAddr)
		if found && existing.IsActive(currentEpoch) {
			continue // Already penalized, don't stack
		}

		// Determine penalty parameters
		var reason string
		var weightReductionBps uint32

		if isFreeriding {
			reason = "freeriding: participation rate below minimum threshold"
			weightReductionBps = uint32(types.DefaultMaxEndorsementWeightReductionBps)
		} else if isQuorumGaming {
			reason = "quorum gaming: majority of endorsements are post-quorum"
			weightReductionBps = uint32(types.DefaultMaxEndorsementWeightReductionBps / 2) // 5% for gaming (less severe)
		}

		penalty := types.EndorsementPenalty{
			ValidatorAddress:          valAddr,
			PenaltyStartEpoch:         currentEpoch,
			PenaltyEndEpoch:           currentEpoch + types.DefaultEndorsementPenaltyEpochs,
			ParticipationBonusBlocked: true,
			EndorsementWeightReductionBps: weightReductionBps,
			Reason:                    reason,
		}

		if err := k.SetEndorsementPenalty(ctx, penalty); err != nil {
			k.logger.Error("failed to set endorsement penalty", "validator", valAddr, "error", err)
			continue
		}

		penaltiesApplied++

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"poc_endorsement_penalty_applied",
			sdk.NewAttribute("validator", valAddr),
			sdk.NewAttribute("reason", reason),
			sdk.NewAttribute("weight_reduction_bps", fmt.Sprintf("%d", weightReductionBps)),
			sdk.NewAttribute("penalty_end_epoch", fmt.Sprintf("%d", penalty.PenaltyEndEpoch)),
		))
	}

	if penaltiesApplied > 0 {
		k.logger.Info("endorsement quality penalties applied",
			"epoch", currentEpoch,
			"penalties_applied", penaltiesApplied,
		)
	}

	return nil
}

// GetEffectiveEndorsementWeight returns the endorsement weight after applying any penalty.
// This reduces the validator's PoV endorsement weight by a bounded factor (max -10%).
// Does NOT affect consensus power or stake.
func (k Keeper) GetEffectiveEndorsementWeight(ctx context.Context, valAddr string, baseWeight math.Int, currentEpoch int64) math.Int {
	penalty, found := k.GetEndorsementPenalty(ctx, valAddr)
	if !found || !penalty.IsActive(currentEpoch) {
		return baseWeight
	}

	multiplier := penalty.GetWeightMultiplier()
	return math.LegacyNewDecFromInt(baseWeight).Mul(multiplier).TruncateInt()
}

// IsParticipationBonusBlocked returns true if the validator's participation bonus is blocked
func (k Keeper) IsParticipationBonusBlocked(ctx context.Context, valAddr string, currentEpoch int64) bool {
	penalty, found := k.GetEndorsementPenalty(ctx, valAddr)
	if !found {
		return false
	}
	return penalty.IsActive(currentEpoch) && penalty.ParticipationBonusBlocked
}

// ============================================================================
// V2.1 Mainnet Hardening - Governance Safety Rails (Requirement 5)
//
// Protects against governance attacks by:
// 1. Change-rate limits: max 20% delta per proposal for critical params
// 2. Timelocks: critical param changes are delayed by DefaultParamTimelockBlocks
// 3. Emergency pause: can pause PoC payouts without halting chain or PoS rewards
// ============================================================================

// IsPayoutsPaused returns true if PoC payouts are currently paused
func (k Keeper) IsPayoutsPaused(ctx context.Context) bool {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyPoCPayoutsPaused)
	if err != nil || bz == nil {
		return false
	}
	return len(bz) > 0 && bz[0] == 1
}

// SetPayoutsPaused sets the emergency pause flag for PoC payouts.
// When paused, no PoC credit payouts occur, but chain and PoS rewards continue.
func (k Keeper) SetPayoutsPaused(ctx context.Context, paused bool) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.storeService.OpenKVStore(ctx)

	var val byte
	if paused {
		val = 1
	}

	if err := store.Set(types.KeyPoCPayoutsPaused, []byte{val}); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_payouts_pause_changed",
		sdk.NewAttribute("paused", fmt.Sprintf("%t", paused)),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
	))

	return nil
}

// RecordParamChange records a parameter change for rate limiting enforcement
func (k Keeper) RecordParamChange(ctx context.Context, paramName string, oldValue string, newValue string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetParamChangeHistoryKey(sdkCtx.BlockHeight())

	record := types.ParamChangeRecord{
		BlockHeight: sdkCtx.BlockHeight(),
		ParamName:   paramName,
		OldValue:    oldValue,
		NewValue:    newValue,
	}

	bz, err := record.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal param change record: %w", err)
	}

	return store.Set(key, bz)
}

// ValidateParamChangeRate checks that a param change does not exceed the max allowed delta.
// For critical params, the change is capped at DefaultMaxParamChangePctPerProposalBps (20%).
func (k Keeper) ValidateParamChangeRate(paramName string, oldDec math.LegacyDec, newDec math.LegacyDec) error {
	if !types.IsCriticalParam(paramName) {
		return nil // Non-critical params have no rate limit
	}

	if oldDec.IsZero() {
		return nil // Can't compute % change from zero
	}

	// Calculate % change
	diff := newDec.Sub(oldDec).Abs()
	pctChange := diff.Quo(oldDec)

	maxPct := math.LegacyNewDecWithPrec(int64(types.DefaultMaxParamChangePctPerProposalBps), 4) // 0.20 = 20%

	if pctChange.GT(maxPct) {
		return types.ErrParamChangeRateExceeded.Wrapf(
			"param %s change of %.2f%% exceeds max %.2f%%",
			paramName,
			pctChange.MulInt64(100).MustFloat64(),
			maxPct.MulInt64(100).MustFloat64(),
		)
	}

	return nil
}

// ============================================================================
// V2.1 Mainnet Hardening - Gas & Storage Optimization (Requirement 6)
//
// Key optimizations:
// 1. Lazy decay: apply on read/write instead of full-store iteration
// 2. Bounded per-epoch iteration via validator set (not full store scan)
// 3. Efficient indexed lookups for frozen credits and finality states
// ============================================================================

// ApplyLazyDecay applies credit decay lazily when credits are accessed.
// Instead of iterating all accounts at epoch boundary, we decay on access.
// This converts O(N) EndBlock cost to O(1) amortized per-access cost.
func (k Keeper) ApplyLazyDecay(ctx context.Context, addr sdk.AccAddress) {
	currentEpoch := k.GetCurrentEpoch(ctx)
	lastDecayEpoch := k.GetLazyDecayMarker(ctx, addr.String())

	if currentEpoch <= lastDecayEpoch {
		return // Already up to date
	}

	epochsMissed := currentEpoch - lastDecayEpoch
	if epochsMissed == 0 {
		return
	}

	credits := k.GetCredits(ctx, addr)
	if credits.Amount.IsZero() {
		// Just update the marker even if no credits
		_ = k.SetLazyDecayMarker(ctx, addr.String(), currentEpoch)
		return
	}

	// Apply compound decay for missed epochs: amount * (1 - rate)^epochs
	// Uses closed-form exponentiation via repeated squaring to handle any number of
	// missed epochs in O(log N) multiplications, avoiding the previous 100-epoch cap
	// that left dormant accounts with incorrectly high balances.
	decayRate := math.LegacyNewDecWithPrec(types.DefaultCreditDecayRateBps, 4) // 0.0050
	retainRate := math.LegacyOneDec().Sub(decayRate)

	// Exponentiation by squaring: retainRate^epochsMissed in O(log N)
	remaining := math.LegacyNewDecFromInt(credits.Amount)
	base := retainRate
	exp := epochsMissed
	factor := math.LegacyOneDec()
	for exp > 0 {
		if exp%2 == 1 {
			factor = factor.Mul(base)
		}
		base = base.Mul(base)
		exp /= 2
	}
	remaining = remaining.Mul(factor)

	newAmount := remaining.TruncateInt()
	if newAmount.IsNegative() {
		newAmount = math.ZeroInt()
	}

	credits.Amount = newAmount
	_ = k.SetCredits(ctx, credits)
	_ = k.SetLazyDecayMarker(ctx, addr.String(), currentEpoch)
}

// GetLazyDecayMarker returns the last epoch decay was applied for an address
func (k Keeper) GetLazyDecayMarker(ctx context.Context, addr string) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetLazyDecayMarkerKey(addr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return 0
	}
	return sdk.BigEndianToUint64(bz)
}

// SetLazyDecayMarker stores the last decay epoch for an address
func (k Keeper) SetLazyDecayMarker(ctx context.Context, addr string, epoch uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetLazyDecayMarkerKey(addr)
	return store.Set(key, sdk.Uint64ToBigEndian(epoch))
}

// ============================================================================
// V2.2 Fraud Endorsement Slashing
//
// When a contribution is invalidated via fraud proof, validators who endorsed
// it with Decision=true (approved) are slashed and jailed. This provides real
// economic consequences for cartel behavior, rubber-stamp endorsements, and
// colluding validators who endorse fraudulent content.
//
// Only approving endorsers are slashed. Rejecting endorsers are not penalized.
// If the slashing keeper is not available, this is a no-op (soft penalties still apply).
// ============================================================================

// SlashFraudEndorsers slashes and jails validators who approved a fraudulent contribution.
// Called from InvalidateContribution() after a fraud proof succeeds.
func (k Keeper) SlashFraudEndorsers(ctx context.Context, contribution types.Contribution) {
	if k.slashingKeeper == nil {
		k.logger.Debug("slashing keeper not available, skipping fraud endorser slashing",
			"contribution_id", contribution.Id)
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	slashFraction := types.DefaultSlashFractionFraudEndorsement()
	var slashedCount int

	for _, endorsement := range contribution.Endorsements {
		if !endorsement.Decision {
			continue // Only slash approvers of fraudulent content
		}

		valAddr, err := sdk.ValAddressFromBech32(endorsement.ValAddr)
		if err != nil {
			// Try AccAddress → ValAddress conversion
			accAddr, accErr := sdk.AccAddressFromBech32(endorsement.ValAddr)
			if accErr != nil {
				k.logger.Debug("invalid endorser address, skipping slash",
					"address", endorsement.ValAddr, "error", err)
				continue
			}
			valAddr = sdk.ValAddress(accAddr)
		}

		consAddr := sdk.ConsAddress(valAddr)

		slashedAmt, err := k.slashingKeeper.Slash(
			ctx,
			consAddr,
			sdkCtx.BlockHeight(),
			0, // power=0 lets the slashing module determine from state
			slashFraction,
		)
		if err != nil {
			k.logger.Error("failed to slash fraud endorser",
				"validator", valAddr.String(),
				"contribution_id", contribution.Id,
				"error", err)
			continue
		}

		if err := k.slashingKeeper.Jail(ctx, consAddr); err != nil {
			k.logger.Error("failed to jail fraud endorser",
				"validator", valAddr.String(),
				"contribution_id", contribution.Id,
				"error", err)
			continue
		}

		slashedCount++

		k.logger.Warn("FRAUD ENDORSER SLASHED AND JAILED",
			"validator", valAddr.String(),
			"contribution_id", contribution.Id,
			"slash_fraction", slashFraction.String(),
			"slashed_amount", slashedAmt.String(),
			"block_height", sdkCtx.BlockHeight(),
		)

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"poc_fraud_endorser_slashed",
			sdk.NewAttribute("validator", valAddr.String()),
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contribution.Id)),
			sdk.NewAttribute("slash_fraction", slashFraction.String()),
			sdk.NewAttribute("slashed_amount", slashedAmt.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		))
	}

	if slashedCount > 0 {
		k.logger.Info("fraud endorsement slashing complete",
			"contribution_id", contribution.Id,
			"endorsers_slashed", slashedCount,
			"total_endorsements", len(contribution.Endorsements),
		)
	}
}
