package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ============================================================================
// Similarity Engine Keeper Methods (Layer 2)
//
// Architecture:
//   1. VerifyOracleSignature — iterate allowlist, verify secp256k1 sig
//   2. ProcessSimilarityCommitment — full pipeline from msg to stored record
//   3. GetSimilarityCommitment / SetSimilarityCommitment — KV CRUD
//   4. GetSimilarityEpoch — epoch calc using SimilarityEpochBlocks param
// ============================================================================

// GetSimilarityEpoch returns the current similarity epoch based on block height
// and the SimilarityEpochBlocks param. This is independent of the main epoch.
func (k Keeper) GetSimilarityEpoch(ctx context.Context) uint64 {
	params := k.GetParams(ctx)
	if params.SimilarityEpochBlocks <= 0 {
		return 0
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return uint64(sdkCtx.BlockHeight()) / uint64(params.SimilarityEpochBlocks)
}

// GetSimilarityCommitment retrieves a similarity commitment record by contribution ID.
func (k Keeper) GetSimilarityCommitment(ctx context.Context, contributionID uint64) (types.SimilarityCommitmentRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetSimilarityCommitmentKey(contributionID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.SimilarityCommitmentRecord{}, false
	}

	var record types.SimilarityCommitmentRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.SimilarityCommitmentRecord{}, false
	}
	return record, true
}

// SetSimilarityCommitment stores a similarity commitment record.
func (k Keeper) SetSimilarityCommitment(ctx context.Context, record types.SimilarityCommitmentRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetSimilarityCommitmentKey(record.ContributionID)

	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal similarity commitment record: %w", err)
	}

	return store.Set(key, bz)
}

// GetSimilarityScore retrieves the normalized similarity score for a contribution.
// Returns a decimal in [0.0, 1.0]. Returns 0.0 if no similarity data exists.
func (k Keeper) GetSimilarityScore(ctx context.Context, contributionID uint64) math.LegacyDec {
	record, found := k.GetSimilarityCommitment(ctx, contributionID)
	if !found {
		return math.LegacyZeroDec()
	}
	// Convert basis points (0-10000) to decimal (0.0-1.0)
	return math.LegacyNewDec(int64(record.CompactData.OverallSimilarity)).Quo(math.LegacyNewDec(10000))
}

// VerifyOracleSignature verifies that the given data was signed by one of the
// allowlisted oracle addresses. Returns the matching oracle's bech32 address on success.
//
// Signature verification uses secp256k1 (same as Cosmos SDK accounts).
// The signature must be a 64-byte compact secp256k1 signature.
func (k Keeper) VerifyOracleSignature(ctx context.Context, data []byte, signature []byte) (string, error) {
	params := k.GetParams(ctx)

	if len(params.SimilarityOracleAllowlist) == 0 {
		return "", types.ErrOracleNotAllowlisted.Wrap("oracle allowlist is empty")
	}

	// Try each allowlisted oracle address
	for _, oracleAddr := range params.SimilarityOracleAllowlist {
		accAddr, err := sdk.AccAddressFromBech32(oracleAddr)
		if err != nil {
			continue // skip malformed addresses in allowlist
		}

		// Retrieve the oracle's account to get their public key
		account := k.accountKeeper.GetAccount(ctx, accAddr)
		if account == nil {
			continue // oracle account doesn't exist on-chain yet
		}

		pubKey := account.GetPubKey()
		if pubKey == nil {
			continue // no public key registered for this account
		}

		// Verify signature using the account's public key (supports secp256k1, ed25519, etc.)
		if pubKey.VerifySignature(data, signature) {
			return oracleAddr, nil
		}
	}

	return "", types.ErrInvalidOracleSignature.Wrap("signature does not match any allowlisted oracle")
}

// ProcessSimilarityCommitment is the full pipeline for handling a similarity commitment.
//
// Pipeline:
//  1. Check similarity engine is enabled
//  2. Verify contribution exists
//  3. Check no existing commitment for this contribution
//  4. Verify dual signatures (compact data + full commitment hash)
//  5. Anti-replay: verify epoch matches current similarity epoch
//  6. Apply derivative threshold → flag contribution if above threshold
//  7. Store SimilarityCommitmentRecord
//  8. Emit event
func (k Keeper) ProcessSimilarityCommitment(ctx context.Context, msg *types.MsgSubmitSimilarityCommitment) (*types.MsgSubmitSimilarityCommitmentResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// 1. Check similarity engine is enabled
	if !params.EnableSimilarityCheck {
		return nil, types.ErrSimilarityDisabled.Wrap("similarity engine is not enabled via governance")
	}

	// 2. Verify contribution exists
	contribution, found := k.GetContribution(ctx, msg.ContributionID)
	if !found {
		return nil, types.ErrContributionNotFound.Wrapf("contribution %d not found", msg.ContributionID)
	}

	// 3. Check no existing commitment for this contribution
	_, exists := k.GetSimilarityCommitment(ctx, msg.ContributionID)
	if exists {
		return nil, types.ErrSimilarityCommitmentExists.Wrapf(
			"contribution %d already has a similarity commitment", msg.ContributionID)
	}

	// 4a. Parse compact data
	var compactData types.SimilarityCompactData
	if err := json.Unmarshal(msg.CompactDataJson, &compactData); err != nil {
		return nil, types.ErrInvalidCompactData.Wrapf("failed to parse compact data: %s", err)
	}

	// DEFENSE-IN-DEPTH: Ensure the signed data belongs to the contribution being updated
	if compactData.ContributionID != msg.ContributionID {
		return nil, types.ErrInvalidCompactData.Wrapf(
			"compact data contribution ID (%d) does not match message contribution ID (%d)",
			compactData.ContributionID, msg.ContributionID)
	}

	// 4b. Verify compact data signature
	compactOracleAddr, err := k.VerifyOracleSignature(ctx, msg.CompactDataJson, msg.OracleSignatureCompact)
	if err != nil {
		return nil, fmt.Errorf("compact data signature verification failed: %w", err)
	}

	// 4c. Verify full commitment hash signature (must be signed by the SAME oracle)
	fullOracleAddr, err := k.VerifyOracleSignature(ctx, msg.CommitmentHashFull, msg.OracleSignatureFull)
	if err != nil {
		return nil, fmt.Errorf("full commitment hash signature verification failed: %w", err)
	}

	// 4d. Ensure both signatures are from the same oracle
	if compactOracleAddr != fullOracleAddr {
		return nil, types.ErrInvalidOracleSignature.Wrapf(
			"compact signature from %s but full signature from %s — must be same oracle",
			compactOracleAddr, fullOracleAddr)
	}

	// 5. Anti-replay: verify epoch matches current similarity epoch
	currentEpoch := k.GetSimilarityEpoch(ctx)
	if compactData.Epoch != currentEpoch {
		return nil, types.ErrSimilarityEpochMismatch.Wrapf(
			"compact_data.epoch=%d but current similarity epoch=%d",
			compactData.Epoch, currentEpoch)
	}

	// 6. Apply derivative threshold
	isDerivative := compactData.OverallSimilarity >= params.DerivativeThreshold

	// Flag the contribution as derivative if threshold exceeded
	if isDerivative {
		contribution.IsDerivative = true
		if err := k.SetContribution(ctx, contribution); err != nil {
			return nil, fmt.Errorf("failed to update contribution with derivative flag: %w", err)
		}
	}

	// 7. Store SimilarityCommitmentRecord
	record := types.SimilarityCommitmentRecord{
		ContributionID:     msg.ContributionID,
		CompactData:        compactData,
		CommitmentHashFull: msg.CommitmentHashFull,
		OracleAddress:      compactOracleAddr,
		BlockHeight:        sdkCtx.BlockHeight(),
		IsDerivative:       isDerivative,
	}

	if err := k.SetSimilarityCommitment(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to store similarity commitment: %w", err)
	}

	// 8. Emit event
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_similarity_commitment",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", msg.ContributionID)),
			sdk.NewAttribute("oracle", compactOracleAddr),
			sdk.NewAttribute("overall_similarity", fmt.Sprintf("%d", compactData.OverallSimilarity)),
			sdk.NewAttribute("confidence", fmt.Sprintf("%d", compactData.Confidence)),
			sdk.NewAttribute("nearest_parent", fmt.Sprintf("%d", compactData.NearestParentClaimID)),
			sdk.NewAttribute("is_derivative", fmt.Sprintf("%t", isDerivative)),
			sdk.NewAttribute("epoch", fmt.Sprintf("%d", compactData.Epoch)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Submitter),
		),
	})

	// Update unified claim status
	if isDerivative {
		k.TransitionClaimStatus(ctx, msg.ContributionID, types.ClaimStatusFlaggedDerivative)
	}
	// Non-derivative: stay at AWAITING_SIMILARITY — ProcessStartReview will transition to IN_REVIEW

	return &types.MsgSubmitSimilarityCommitmentResponse{
		IsDerivative: isDerivative,
	}, nil
}
