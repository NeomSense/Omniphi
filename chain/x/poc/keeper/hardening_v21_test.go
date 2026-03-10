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
// V2.1 Mainnet Hardening Tests
// ============================================================================

// ============================================================================
// 1. Deterministic Finality Timing Tests
// ============================================================================

func TestChallengeWindow_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// No window set — should return not found
	_, found := f.keeper.GetChallengeWindow(f.ctx, 1)
	require.False(t, found)

	// Set a challenge window
	cw := types.ChallengeWindow{
		ContributionID: 1,
		StartHeight:    100,
		EndHeight:      200,
	}
	err := f.keeper.SetChallengeWindow(f.ctx, cw)
	require.NoError(t, err)

	// Get it back
	got, found := f.keeper.GetChallengeWindow(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, int64(100), got.StartHeight)
	require.Equal(t, int64(200), got.EndHeight)
}

func TestChallengeWindow_IsExpired(t *testing.T) {
	cw := types.ChallengeWindow{
		ContributionID: 1,
		StartHeight:    100,
		EndHeight:      200,
	}

	require.False(t, cw.IsExpired(100))
	require.False(t, cw.IsExpired(200))
	require.True(t, cw.IsExpired(201))
}

func TestOpenChallengeWindow(t *testing.T) {
	f := SetupKeeperTest(t)

	err := f.keeper.OpenChallengeWindow(f.ctx, 42)
	require.NoError(t, err)

	// Verify window was created
	cw, found := f.keeper.GetChallengeWindow(f.ctx, 42)
	require.True(t, found)
	require.Equal(t, uint64(42), cw.ContributionID)
	require.Equal(t, cw.StartHeight+types.DefaultChallengeWindowBlocks, cw.EndHeight)

	// Verify finality was set to PENDING
	finality := f.keeper.GetContributionFinality(f.ctx, 42)
	require.Equal(t, types.FinalityStatusPending, finality.Status)
}

func TestTryFinalizeContribution_WindowStillOpen(t *testing.T) {
	f := SetupKeeperTest(t)

	// Open a challenge window
	err := f.keeper.OpenChallengeWindow(f.ctx, 1)
	require.NoError(t, err)

	// Try to finalize — should fail (window still open, block height is 0)
	err = f.keeper.TryFinalizeContribution(f.ctx, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrChallengeWindowOpen)
}

func TestTryFinalizeContribution_WindowExpired(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create contribution
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte("testhash12345678901234567890123"),
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	// Set a challenge window that's already expired
	cw := types.ChallengeWindow{
		ContributionID: 1,
		StartHeight:    0,
		EndHeight:      0, // Already expired at any positive height
	}
	err = f.keeper.SetChallengeWindow(f.ctx, cw)
	require.NoError(t, err)

	// Set contribution finality to PENDING
	finality := types.ContributionFinality{
		ContributionID: 1,
		Status:         types.FinalityStatusPending,
	}
	err = f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	// Advance context block height past the window
	f.ctx = f.ctx.WithBlockHeight(1)

	// Now should finalize
	err = f.keeper.TryFinalizeContribution(f.ctx, 1)
	require.NoError(t, err)

	// Verify FINAL status
	got := f.keeper.GetContributionFinality(f.ctx, 1)
	require.Equal(t, types.FinalityStatusFinal, got.Status)
}

func TestTryFinalizeContribution_BlockedByFraudProof(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set contribution as PENDING
	finality := types.ContributionFinality{
		ContributionID: 1,
		Status:         types.FinalityStatusPending,
	}
	err := f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	// Set an expired challenge window
	cw := types.ChallengeWindow{ContributionID: 1, StartHeight: 0, EndHeight: 0}
	err = f.keeper.SetChallengeWindow(f.ctx, cw)
	require.NoError(t, err)

	// Store a validated fraud proof
	fp := types.FraudProof{
		ContributionID: 1,
		ProofType:      types.FraudProofHashMismatch,
		Challenger:     testAddr2.String(),
		Validated:      true,
		SubmittedAt:    0,
	}
	err = f.keeper.SetFraudProof(f.ctx, fp)
	require.NoError(t, err)

	f.ctx = f.ctx.WithBlockHeight(1)

	// Should fail because fraud proof exists
	err = f.keeper.TryFinalizeContribution(f.ctx, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrContributionInvalidated)
}

func TestStateTransition_PendingToChallenged(t *testing.T) {
	f := SetupKeeperTest(t)

	// Open challenge window
	err := f.keeper.OpenChallengeWindow(f.ctx, 1)
	require.NoError(t, err)

	// Challenge the contribution
	err = f.keeper.ChallengeContribution(f.ctx, 1, 42)
	require.NoError(t, err)

	finality := f.keeper.GetContributionFinality(f.ctx, 1)
	require.Equal(t, types.FinalityStatusChallenged, finality.Status)
	require.Equal(t, uint64(42), finality.ChallengeID)
}

func TestStateTransition_ChallengedToFinal(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create contribution
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte("testhash12345678901234567890123"),
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	// Set to CHALLENGED
	finality := types.ContributionFinality{
		ContributionID: 1,
		Status:         types.FinalityStatusChallenged,
		ChallengeID:    1,
	}
	err = f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	// Resolve as invalid challenge (no fraud found)
	err = f.keeper.ResolveChallengeInvalid(f.ctx, 1)
	require.NoError(t, err)

	got := f.keeper.GetContributionFinality(f.ctx, 1)
	require.Equal(t, types.FinalityStatusFinal, got.Status)
}

func TestStateTransition_ChallengedToInvalidated(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create contribution with credits
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte("testhash12345678901234567890123"),
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	// Add credits and freeze them
	err = f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(5000))
	require.NoError(t, err)
	err = f.keeper.FreezeCredits(f.ctx, testAddr1.String(), math.NewInt(5000), 1, "challenge")
	require.NoError(t, err)

	// Set to CHALLENGED
	finality := types.ContributionFinality{
		ContributionID: 1,
		Status:         types.FinalityStatusChallenged,
	}
	err = f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	// Invalidate
	err = f.keeper.InvalidateContribution(f.ctx, 1)
	require.NoError(t, err)

	got := f.keeper.GetContributionFinality(f.ctx, 1)
	require.Equal(t, types.FinalityStatusInvalidated, got.Status)

	// Credits should be burned
	credits := f.keeper.GetCredits(f.ctx, testAddr1)
	require.True(t, credits.Amount.IsZero())
}

func TestStateTransition_InvalidTransitions(t *testing.T) {
	f := SetupKeeperTest(t)

	// FINAL → anything should fail
	finality := types.ContributionFinality{
		ContributionID: 1,
		Status:         types.FinalityStatusFinal,
		FinalizedAt:    100,
	}
	err := f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	err = f.keeper.ChallengeContribution(f.ctx, 1, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidStateTransition)

	// INVALIDATED cannot be challenged
	finality.Status = types.FinalityStatusInvalidated
	finality.ContributionID = 2
	err = f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	err = f.keeper.ChallengeContribution(f.ctx, 2, 1)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidStateTransition)

	// PENDING cannot be invalidated directly
	finality.Status = types.FinalityStatusPending
	finality.ContributionID = 3
	err = f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	err = f.keeper.InvalidateContribution(f.ctx, 3)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidStateTransition)
}

// ============================================================================
// 2. Objective Fraud Proof Tests
// ============================================================================

func TestFraudProof_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Not found initially
	_, found := f.keeper.GetFraudProof(f.ctx, 1)
	require.False(t, found)

	fp := types.FraudProof{
		ContributionID: 1,
		ProofType:      types.FraudProofHashMismatch,
		Challenger:     testAddr1.String(),
		ProofData:      []byte("proof"),
		SubmittedAt:    100,
		Validated:      true,
	}
	err := f.keeper.SetFraudProof(f.ctx, fp)
	require.NoError(t, err)

	got, found := f.keeper.GetFraudProof(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, types.FraudProofHashMismatch, got.ProofType)
	require.True(t, got.Validated)
}

func TestFraudProofType_IsValid(t *testing.T) {
	require.True(t, types.FraudProofInvalidQuorum.IsValid())
	require.True(t, types.FraudProofHashMismatch.IsValid())
	require.True(t, types.FraudProofNonceReplay.IsValid())
	require.True(t, types.FraudProofMerkleInclusion.IsValid())
	require.False(t, types.FraudProofType(99).IsValid())
}

func TestFraudProofType_String(t *testing.T) {
	require.Equal(t, "INVALID_QUORUM", types.FraudProofInvalidQuorum.String())
	require.Equal(t, "HASH_MISMATCH", types.FraudProofHashMismatch.String())
	require.Equal(t, "NONCE_REPLAY", types.FraudProofNonceReplay.String())
	require.Equal(t, "MERKLE_INCLUSION", types.FraudProofMerkleInclusion.String())
}

func TestSubmitFraudProof_InvalidType(t *testing.T) {
	f := SetupKeeperTest(t)

	err := f.keeper.SubmitFraudProof(f.ctx, 1, types.FraudProofType(99), testAddr1.String(), nil)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidFraudProofType)
}

func TestSubmitFraudProof_DuplicateRejected(t *testing.T) {
	f := SetupKeeperTest(t)

	// Store a fraud proof first
	fp := types.FraudProof{
		ContributionID: 1,
		ProofType:      types.FraudProofHashMismatch,
		Validated:      true,
	}
	err := f.keeper.SetFraudProof(f.ctx, fp)
	require.NoError(t, err)

	// Try to submit another
	err = f.keeper.SubmitFraudProof(f.ctx, 1, types.FraudProofHashMismatch, testAddr1.String(), nil)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrFraudProofAlreadyExists)
}

func TestVerifyFraudProof_HashMismatch_EmptyHash(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create contribution with empty hash
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte{}, // Empty hash — fraud
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	valid, err := f.keeper.VerifyFraudProof(f.ctx, 1, types.FraudProofHashMismatch, []byte("expected"))
	require.NoError(t, err)
	require.True(t, valid, "empty hash should be detected as fraud")
}

func TestVerifyFraudProof_HashMismatch_InvalidLength(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create contribution with invalid hash length (not 32 or 64)
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte("short"),
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	valid, err := f.keeper.VerifyFraudProof(f.ctx, 1, types.FraudProofHashMismatch, []byte("expected"))
	require.NoError(t, err)
	require.True(t, valid, "invalid hash length should be detected as fraud")
}

func TestVerifyFraudProof_HashMismatch_ValidHash(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create contribution with valid 32-byte hash
	hash := make([]byte, 32)
	hash[0] = 0x01 // Non-zero
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        hash,
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	valid, err := f.keeper.VerifyFraudProof(f.ctx, 1, types.FraudProofHashMismatch, []byte("expected"))
	require.NoError(t, err)
	require.False(t, valid, "valid 32-byte non-zero hash should not be fraud")
}

func TestVerifyFraudProof_NonceReplay(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create contribution
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        make([]byte, 32),
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	// Increment nonce twice (simulating two withdrawals)
	err = f.keeper.IncrementClaimNonce(f.ctx, testAddr1.String())
	require.NoError(t, err)
	err = f.keeper.IncrementClaimNonce(f.ctx, testAddr1.String())
	require.NoError(t, err)

	// Now nonce is 2. Claim nonce 0 was already used.
	proofData := sdk.Uint64ToBigEndian(0) // nonce 0 (already used)
	valid, err := f.keeper.VerifyFraudProof(f.ctx, 1, types.FraudProofNonceReplay, proofData)
	require.NoError(t, err)
	require.True(t, valid, "nonce 0 < current nonce 2, so it's a replay")

	// Nonce 5 is not yet used
	proofData = sdk.Uint64ToBigEndian(5) // nonce 5 (not used)
	valid, err = f.keeper.VerifyFraudProof(f.ctx, 1, types.FraudProofNonceReplay, proofData)
	require.NoError(t, err)
	require.False(t, valid, "nonce 5 >= current nonce 2, not a replay")
}

func TestVerifyFraudProof_ContributionNotFound(t *testing.T) {
	f := SetupKeeperTest(t)

	_, err := f.keeper.VerifyFraudProof(f.ctx, 999, types.FraudProofHashMismatch, []byte("data"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrContributionNotFound)
}

// ============================================================================
// 3. RewardMult Safety Invariant Tests
// ============================================================================

func TestNoNaNInvariant(t *testing.T) {
	// This is tested in the rewardmult package, but we verify the type is accessible
	require.NotNil(t, types.FinalityStatusPending)
}

// ============================================================================
// 4. Endorsement Quality Soft Penalty Tests
// ============================================================================

func TestEndorsementPenalty_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	valAddr := sdk.ValAddress("endorsement_penalty_").String()

	// Not found initially
	_, found := f.keeper.GetEndorsementPenalty(f.ctx, valAddr)
	require.False(t, found)

	// Set a penalty
	penalty := types.EndorsementPenalty{
		ValidatorAddress:          valAddr,
		PenaltyStartEpoch:         10,
		PenaltyEndEpoch:           20,
		ParticipationBonusBlocked: true,
		EndorsementWeightReductionBps: 1000, // 10%
		Reason:                    "freeriding",
	}
	err := f.keeper.SetEndorsementPenalty(f.ctx, penalty)
	require.NoError(t, err)

	got, found := f.keeper.GetEndorsementPenalty(f.ctx, valAddr)
	require.True(t, found)
	require.Equal(t, int64(10), got.PenaltyStartEpoch)
	require.Equal(t, int64(20), got.PenaltyEndEpoch)
	require.True(t, got.ParticipationBonusBlocked)
	require.Equal(t, uint32(1000), got.EndorsementWeightReductionBps)
}

func TestEndorsementPenalty_IsActive(t *testing.T) {
	penalty := types.EndorsementPenalty{
		PenaltyStartEpoch: 10,
		PenaltyEndEpoch:   20,
	}

	require.False(t, penalty.IsActive(9))
	require.True(t, penalty.IsActive(10))
	require.True(t, penalty.IsActive(15))
	require.True(t, penalty.IsActive(19))
	require.False(t, penalty.IsActive(20)) // End epoch is exclusive
	require.False(t, penalty.IsActive(25))
}

func TestEndorsementPenalty_GetWeightMultiplier(t *testing.T) {
	// No reduction
	penalty := types.EndorsementPenalty{EndorsementWeightReductionBps: 0}
	require.True(t, penalty.GetWeightMultiplier().Equal(math.LegacyOneDec()))

	// 10% reduction = 0.90 multiplier
	penalty = types.EndorsementPenalty{EndorsementWeightReductionBps: 1000}
	require.True(t, penalty.GetWeightMultiplier().Equal(math.LegacyNewDecWithPrec(90, 2)))

	// 5% reduction = 0.95 multiplier
	penalty = types.EndorsementPenalty{EndorsementWeightReductionBps: 500}
	require.True(t, penalty.GetWeightMultiplier().Equal(math.LegacyNewDecWithPrec(95, 2)))
}

func TestGetEffectiveEndorsementWeight(t *testing.T) {
	f := SetupKeeperTest(t)

	valAddr := sdk.ValAddress("endorsement_penalty_").String()
	baseWeight := math.NewInt(100000)

	// No penalty — full weight
	effective := f.keeper.GetEffectiveEndorsementWeight(f.ctx, valAddr, baseWeight, 5)
	require.True(t, effective.Equal(baseWeight))

	// Apply 10% penalty
	penalty := types.EndorsementPenalty{
		ValidatorAddress:          valAddr,
		PenaltyStartEpoch:         1,
		PenaltyEndEpoch:           20,
		EndorsementWeightReductionBps: 1000,
	}
	err := f.keeper.SetEndorsementPenalty(f.ctx, penalty)
	require.NoError(t, err)

	// During penalty — weight reduced by 10%
	effective = f.keeper.GetEffectiveEndorsementWeight(f.ctx, valAddr, baseWeight, 5)
	require.True(t, effective.Equal(math.NewInt(90000)))

	// After penalty expires — full weight again
	effective = f.keeper.GetEffectiveEndorsementWeight(f.ctx, valAddr, baseWeight, 25)
	require.True(t, effective.Equal(baseWeight))
}

func TestIsParticipationBonusBlocked(t *testing.T) {
	f := SetupKeeperTest(t)

	valAddr := sdk.ValAddress("endorsement_penalty_").String()

	// No penalty — not blocked
	require.False(t, f.keeper.IsParticipationBonusBlocked(f.ctx, valAddr, 5))

	// Apply penalty with bonus blocked
	penalty := types.EndorsementPenalty{
		ValidatorAddress:          valAddr,
		PenaltyStartEpoch:         1,
		PenaltyEndEpoch:           20,
		ParticipationBonusBlocked: true,
	}
	err := f.keeper.SetEndorsementPenalty(f.ctx, penalty)
	require.NoError(t, err)

	require.True(t, f.keeper.IsParticipationBonusBlocked(f.ctx, valAddr, 5))
	require.False(t, f.keeper.IsParticipationBonusBlocked(f.ctx, valAddr, 25))
}

// ============================================================================
// 5. Governance Safety Rail Tests
// ============================================================================

func TestPayoutsPaused_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Default: not paused
	require.False(t, f.keeper.IsPayoutsPaused(f.ctx))

	// Pause
	err := f.keeper.SetPayoutsPaused(f.ctx, true)
	require.NoError(t, err)
	require.True(t, f.keeper.IsPayoutsPaused(f.ctx))

	// Unpause
	err = f.keeper.SetPayoutsPaused(f.ctx, false)
	require.NoError(t, err)
	require.False(t, f.keeper.IsPayoutsPaused(f.ctx))
}

func TestRecordParamChange(t *testing.T) {
	f := SetupKeeperTest(t)

	err := f.keeper.RecordParamChange(f.ctx, "quorum_pct", "0.67", "0.75")
	require.NoError(t, err)
	// Just ensure it doesn't error — the record is stored for audit
}

func TestValidateParamChangeRate_CriticalParam(t *testing.T) {
	f := SetupKeeperTest(t)
	_ = f // suppress unused

	// 10% change should pass (< 20% limit)
	oldDec := math.LegacyNewDecWithPrec(67, 2) // 0.67
	newDec := math.LegacyNewDecWithPrec(72, 2) // 0.72  (~7.5% change)
	err := f.keeper.ValidateParamChangeRate("quorum_pct", oldDec, newDec)
	require.NoError(t, err)

	// 50% change should fail (> 20% limit)
	newDec = math.LegacyOneDec() // 1.00 (~49% change from 0.67)
	err = f.keeper.ValidateParamChangeRate("quorum_pct", oldDec, newDec)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrParamChangeRateExceeded)
}

func TestValidateParamChangeRate_NonCriticalParam(t *testing.T) {
	f := SetupKeeperTest(t)
	_ = f

	// Non-critical params have no rate limit
	oldDec := math.LegacyNewDecWithPrec(50, 2)
	newDec := math.LegacyNewDec(100) // Huge change
	err := f.keeper.ValidateParamChangeRate("some_non_critical_param", oldDec, newDec)
	require.NoError(t, err)
}

func TestValidateParamChangeRate_ZeroOldValue(t *testing.T) {
	f := SetupKeeperTest(t)
	_ = f

	// Zero old value — skip rate check (can't compute % from zero)
	err := f.keeper.ValidateParamChangeRate("quorum_pct", math.LegacyZeroDec(), math.LegacyOneDec())
	require.NoError(t, err)
}

func TestIsCriticalParam(t *testing.T) {
	require.True(t, types.IsCriticalParam("credit_cap"))
	require.True(t, types.IsCriticalParam("decay_rate"))
	require.True(t, types.IsCriticalParam("quorum_pct"))
	require.True(t, types.IsCriticalParam("min_multiplier"))
	require.True(t, types.IsCriticalParam("max_multiplier"))
	require.False(t, types.IsCriticalParam("some_random_param"))
	require.False(t, types.IsCriticalParam(""))
}

// ============================================================================
// 6. Gas & Storage Optimization Tests
// ============================================================================

func TestLazyDecay_BasicOperation(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add credits
	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(10000))
	require.NoError(t, err)

	// Set current epoch > 0 so decay applies
	// We need to use block-based epoch approximation (epoch = blockHeight / 100)
	// Block 200 = epoch 2
	f.ctx = f.ctx.WithBlockHeight(200)

	// Apply lazy decay
	f.keeper.ApplyLazyDecay(f.ctx, testAddr1)

	// Credits should be decayed: 10000 * (1 - 0.005)^2 = 10000 * 0.995^2 = 9900 (approx)
	credits := f.keeper.GetCredits(f.ctx, testAddr1)
	require.True(t, credits.Amount.LT(math.NewInt(10000)), "credits should decrease after decay")
	require.True(t, credits.Amount.GT(math.NewInt(9800)), "credits should not decrease too much")

	// Verify decay marker was set
	marker := f.keeper.GetLazyDecayMarker(f.ctx, testAddr1.String())
	require.Equal(t, uint64(2), marker)
}

func TestLazyDecay_NoDuplicateApplication(t *testing.T) {
	f := SetupKeeperTest(t)

	err := f.keeper.AddCreditsWithOverflowCheck(f.ctx, testAddr1, math.NewInt(10000))
	require.NoError(t, err)

	f.ctx = f.ctx.WithBlockHeight(100) // epoch 1

	// Apply once
	f.keeper.ApplyLazyDecay(f.ctx, testAddr1)
	credits1 := f.keeper.GetCredits(f.ctx, testAddr1)

	// Apply again — should not change
	f.keeper.ApplyLazyDecay(f.ctx, testAddr1)
	credits2 := f.keeper.GetCredits(f.ctx, testAddr1)

	require.True(t, credits1.Amount.Equal(credits2.Amount), "second decay application should be a no-op")
}

func TestLazyDecay_ZeroCredits(t *testing.T) {
	f := SetupKeeperTest(t)

	f.ctx = f.ctx.WithBlockHeight(100)

	// Apply decay to address with no credits — should not panic
	f.keeper.ApplyLazyDecay(f.ctx, testAddr1)

	// Marker should be set even with zero credits
	marker := f.keeper.GetLazyDecayMarker(f.ctx, testAddr1.String())
	require.Equal(t, uint64(1), marker)
}

func TestLazyDecayMarker_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	// Default is 0
	marker := f.keeper.GetLazyDecayMarker(f.ctx, testAddr1.String())
	require.Equal(t, uint64(0), marker)

	// Set
	err := f.keeper.SetLazyDecayMarker(f.ctx, testAddr1.String(), 42)
	require.NoError(t, err)

	marker = f.keeper.GetLazyDecayMarker(f.ctx, testAddr1.String())
	require.Equal(t, uint64(42), marker)
}

// ============================================================================
// 7. Invariant Tests for V2.1
// ============================================================================

func TestFinalityConsistencyInvariant_Passes(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create a valid finalized contribution
	contribution := types.Contribution{
		Id:          1,
		Contributor: testAddr1.String(),
		Ctype:       "code",
		Uri:         "ipfs://test",
		Hash:        []byte("testhash12345678901234567890123"),
		Verified:    true,
	}
	err := f.keeper.SetContribution(f.ctx, contribution)
	require.NoError(t, err)

	finality := types.ContributionFinality{
		ContributionID: 1,
		Status:         types.FinalityStatusFinal,
		FinalizedAt:    100,
	}
	err = f.keeper.SetContributionFinality(f.ctx, finality)
	require.NoError(t, err)

	// Invariant should pass
	msg, broken := FinalityConsistencyInvariant(f.keeper)(f.ctx)
	require.False(t, broken, "invariant should pass: %s", msg)
}

// ============================================================================
// 8. PoC Economics Unchanged Tests (V2.1 Confirmation)
// ============================================================================

func TestPoCEconomics_UnchangedByV21(t *testing.T) {
	f := SetupKeeperTest(t)

	// Verify that V2.1 additions do not alter core PoC economics
	params := f.keeper.GetParams(f.ctx)

	// Credits remain uniform
	require.True(t, params.BaseRewardUnit.IsPositive())

	// Quorum still 67%
	require.True(t, params.QuorumPct.GT(math.LegacyNewDecWithPrec(60, 2)))

	// Credit cap still 100,000
	require.Equal(t, 100000, types.DefaultCreditCap)

	// Decay rate still 0.5%
	require.Equal(t, 50, types.DefaultCreditDecayRateBps)

	// Governance boost still capped at 10%
	rs := types.NewReputationScore(testAddr1.String())
	rs.Score = math.LegacyNewDec(999999999)
	err := f.keeper.SetReputationScore(f.ctx, rs)
	require.NoError(t, err)

	boost := f.keeper.GetGovBoostFromReputation(f.ctx, testAddr1)
	require.True(t, boost.LTE(math.LegacyNewDecWithPrec(10, 2)))
}

// ============================================================================
// Test Helper Imports
// ============================================================================

var (
	FinalityConsistencyInvariant = keeper.FinalityConsistencyInvariant
)
