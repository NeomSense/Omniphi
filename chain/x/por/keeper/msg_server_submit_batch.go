package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// SubmitBatch handles MsgSubmitBatch - submits a merkle root commitment for a batch of off-chain records
func (ms msgServer) SubmitBatch(goCtx context.Context, msg *types.MsgSubmitBatch) (*types.MsgSubmitBatchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate the message
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	// Check rate limit (transient store counter)
	if err := ms.CheckRateLimit(goCtx); err != nil {
		return nil, err
	}

	// Verify app exists and is active
	app, found := ms.GetApp(goCtx, msg.AppId)
	if !found {
		return nil, types.ErrAppNotFound.Wrapf("app_id: %d", msg.AppId)
	}
	if app.Status != types.AppStatusActive {
		return nil, types.ErrAppNotActive.Wrapf("app_id: %d, status: %s", msg.AppId, app.Status)
	}

	// Verify verifier set exists and belongs to this app
	vs, found := ms.GetVerifierSet(goCtx, msg.VerifierSetId)
	if !found {
		return nil, types.ErrVerifierSetNotFound.Wrapf("verifier_set_id: %d", msg.VerifierSetId)
	}
	if vs.AppId != msg.AppId {
		return nil, types.ErrVerifierSetMismatch.Wrapf(
			"verifier set %d belongs to app %d, not app %d",
			msg.VerifierSetId, vs.AppId, msg.AppId,
		)
	}

	params := ms.GetParams(goCtx)

	// SECURITY (re-audit): Override user-supplied epoch with chain-derived epoch.
	// This prevents attackers from using arbitrary epoch values to bypass per-epoch
	// credit caps and challenge rate limits.
	chainEpoch := ms.GetCurrentEpoch(goCtx)
	if msg.Epoch != chainEpoch {
		ms.Logger().Debug("overriding user-supplied epoch with chain epoch",
			"user_epoch", msg.Epoch,
			"chain_epoch", chainEpoch,
		)
		msg.Epoch = chainEpoch
	}

	// SECURITY (F2/F6): Enforce DA commitment if required by governance
	if params.RequireDACommitment && len(msg.DACommitmentHash) == 0 {
		return nil, types.ErrDACommitmentRequired.Wrapf(
			"DA commitment hash is required for batch submission",
		)
	}

	// SECURITY (F2/F6): Enforce per-batch credit cap at submission time
	batchCredits := params.BaseRecordReward.Mul(math.NewIntFromUint64(msg.RecordCount))
	if params.MaxCreditsPerBatch.IsPositive() && batchCredits.GT(params.MaxCreditsPerBatch) {
		return nil, types.ErrEpochCreditCapExceeded.Wrapf(
			"batch credits %s exceed per-batch cap %s (record_count=%d, base_reward=%s)",
			batchCredits, params.MaxCreditsPerBatch, msg.RecordCount, params.BaseRecordReward,
		)
	}

	// SECURITY (F3): Enforce leaf hashes if required by governance
	if params.RequireLeafHashes && len(msg.LeafHashes) == 0 {
		return nil, types.ErrInvalidRecordCount.Wrapf(
			"leaf hashes are required for batch submission",
		)
	}

	// SECURITY (F8): Enforce PoSeq commitment if required by governance
	if params.RequirePoSeqCommitment {
		if len(msg.PoSeqCommitmentHash) == 0 {
			return nil, types.ErrPoSeqCommitmentNotFound.Wrapf(
				"PoSeq commitment hash is required for batch submission",
			)
		}
		// Verify the referenced PoSeq commitment exists on-chain
		_, poseqFound := ms.GetPoSeqCommitment(goCtx, msg.PoSeqCommitmentHash)
		if !poseqFound {
			return nil, types.ErrPoSeqCommitmentNotFound.Wrapf(
				"referenced PoSeq commitment hash is not registered on-chain",
			)
		}
	}

	// Get next batch ID
	batchID, err := ms.GetNextBatchID(goCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next batch ID: %w", err)
	}

	// Calculate challenge end time
	now := ctx.BlockTime().Unix()
	challengeEndTime := now + app.ChallengePeriod

	// Create the batch commitment with optional DA/PoSeq fields
	batch := types.NewBatchCommitmentWithOptions(
		batchID,
		msg.Epoch,
		msg.RecordMerkleRoot,
		msg.RecordCount,
		msg.AppId,
		msg.VerifierSetId,
		msg.Submitter,
		challengeEndTime,
		now,
		msg.DACommitmentHash,
		msg.PoSeqCommitmentHash,
	)

	// Store the batch (also maintains all indexes)
	if err := ms.SetBatch(goCtx, batch); err != nil {
		return nil, fmt.Errorf("failed to store batch: %w", err)
	}

	// SECURITY (F3): Store leaf hashes if provided (enables conclusive double-inclusion verification)
	if len(msg.LeafHashes) > 0 {
		if err := ms.StoreBatchLeafHashes(goCtx, batchID, msg.LeafHashes); err != nil {
			return nil, fmt.Errorf("failed to store leaf hashes: %w", err)
		}
	}

	ms.Logger().Info("batch submitted",
		"batch_id", batchID,
		"app_id", msg.AppId,
		"epoch", msg.Epoch,
		"record_count", msg.RecordCount,
		"submitter", msg.Submitter,
		"challenge_end_time", challengeEndTime,
		"has_da_commitment", len(msg.DACommitmentHash) > 0,
		"has_leaf_hashes", len(msg.LeafHashes) > 0,
		"has_poseq_commitment", len(msg.PoSeqCommitmentHash) > 0,
	)

	// Emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"por_submit_batch",
			sdk.NewAttribute("batch_id", fmt.Sprintf("%d", batchID)),
			sdk.NewAttribute("app_id", fmt.Sprintf("%d", msg.AppId)),
			sdk.NewAttribute("epoch", fmt.Sprintf("%d", msg.Epoch)),
			sdk.NewAttribute("record_count", fmt.Sprintf("%d", msg.RecordCount)),
			sdk.NewAttribute("submitter", msg.Submitter),
			sdk.NewAttribute("verifier_set_id", fmt.Sprintf("%d", msg.VerifierSetId)),
			sdk.NewAttribute("challenge_end_time", fmt.Sprintf("%d", challengeEndTime)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Submitter),
		),
	})

	return &types.MsgSubmitBatchResponse{BatchId: batchID}, nil
}
