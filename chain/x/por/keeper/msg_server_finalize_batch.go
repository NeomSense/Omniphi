package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// FinalizeBatch handles MsgFinalizeBatch - finalizes a batch after the challenge window expires
// This can be called by governance (authority) or triggered by EndBlocker
func (ms msgServer) FinalizeBatch(goCtx context.Context, msg *types.MsgFinalizeBatch) (*types.MsgFinalizeBatchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate the message
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	// Verify authority is the governance module
	if msg.Authority != ms.GetAuthority() {
		return nil, types.ErrInvalidAuthority.Wrapf(
			"expected %s, got %s", ms.GetAuthority(), msg.Authority,
		)
	}

	// Get the batch
	batch, found := ms.GetBatch(goCtx, msg.BatchId)
	if !found {
		return nil, types.ErrBatchNotFound.Wrapf("batch_id: %d", msg.BatchId)
	}

	// Batch must be in PENDING status to finalize
	if batch.Status == types.BatchStatusFinalized {
		return nil, types.ErrBatchAlreadyFinalized.Wrapf("batch_id: %d", msg.BatchId)
	}
	if batch.Status == types.BatchStatusRejected {
		return nil, types.ErrBatchRejected.Wrapf("batch_id: %d", msg.BatchId)
	}
	if batch.Status != types.BatchStatusPending {
		return nil, types.ErrBatchNotPending.Wrapf(
			"batch %d is in status %s, expected PENDING", msg.BatchId, batch.Status,
		)
	}

	// Verify challenge window has expired
	now := ctx.BlockTime().Unix()
	if now <= batch.ChallengeEndTime {
		return nil, types.ErrChallengeWindowOpen.Wrapf(
			"batch %d challenge window closes at %d, current time: %d",
			msg.BatchId, batch.ChallengeEndTime, now,
		)
	}

	// Verify no open challenges
	if ms.HasOpenChallenges(goCtx, msg.BatchId) {
		return nil, types.ErrOpenChallengesExist.Wrapf("batch_id: %d", msg.BatchId)
	}

	// Finalize the batch
	batch.FinalizedAt = now
	if err := ms.UpdateBatchStatus(goCtx, &batch, types.BatchStatusFinalized); err != nil {
		return nil, fmt.Errorf("failed to finalize batch: %w", err)
	}

	// Award PoC credits for the finalized batch (if PoC keeper is available)
	if ms.pocKeeper != nil {
		params := ms.GetParams(goCtx)
		// Credits = record_count * base_record_reward
		credits := params.BaseRecordReward.Mul(math.NewIntFromUint64(batch.RecordCount))

		// SECURITY (re-audit): Enforce per-batch credit cap (same as EndBlocker)
		if params.MaxCreditsPerBatch.IsPositive() && credits.GT(params.MaxCreditsPerBatch) {
			ms.Logger().Info("FinalizeBatch: capping batch credits",
				"batch_id", msg.BatchId,
				"computed", credits,
				"cap", params.MaxCreditsPerBatch,
			)
			credits = params.MaxCreditsPerBatch
		}

		// SECURITY (re-audit): Enforce per-epoch credit cap (same as EndBlocker)
		if params.MaxCreditsPerEpoch.IsPositive() && credits.IsPositive() {
			epochUsed := ms.GetEpochCreditsUsed(goCtx, batch.Epoch)
			remaining := params.MaxCreditsPerEpoch.Sub(epochUsed)
			if remaining.IsZero() || remaining.IsNegative() {
				ms.Logger().Info("FinalizeBatch: epoch credit cap exhausted, skipping credit award",
					"batch_id", msg.BatchId,
					"epoch", batch.Epoch,
					"epoch_used", epochUsed,
					"cap", params.MaxCreditsPerEpoch,
				)
				credits = math.ZeroInt()
			} else if credits.GT(remaining) {
				ms.Logger().Info("FinalizeBatch: capping credits to remaining epoch budget",
					"batch_id", msg.BatchId,
					"computed", credits,
					"remaining", remaining,
				)
				credits = remaining
			}
		}

		submitterAddr, err := sdk.AccAddressFromBech32(batch.Submitter)
		if err == nil && credits.IsPositive() {
			if err := ms.pocKeeper.AddCreditsWithOverflowCheck(goCtx, submitterAddr, credits); err != nil {
				// Non-critical: log but don't fail finalization
				ms.Logger().Error("failed to award PoC credits",
					"batch_id", msg.BatchId,
					"submitter", batch.Submitter,
					"credits", credits,
					"error", err,
				)
			} else {
				// Track epoch credit usage
				if _, err := ms.IncrementEpochCredits(goCtx, batch.Epoch, credits); err != nil {
					ms.Logger().Error("FinalizeBatch: failed to track epoch credits",
						"batch_id", msg.BatchId,
						"epoch", batch.Epoch,
						"error", err,
					)
				}

				ms.Logger().Info("PoC credits awarded",
					"batch_id", msg.BatchId,
					"submitter", batch.Submitter,
					"credits", credits,
				)

				ctx.EventManager().EmitEvent(sdk.NewEvent(
					"por_poc_credits_awarded",
					sdk.NewAttribute("batch_id", fmt.Sprintf("%d", msg.BatchId)),
					sdk.NewAttribute("submitter", batch.Submitter),
					sdk.NewAttribute("credits", credits.String()),
				))
			}
		}
	}

	// Update verifier reputations for all attesters (mark as correct)
	attestations := ms.GetAttestationsForBatch(goCtx, msg.BatchId)
	for _, att := range attestations {
		rep := ms.GetOrCreateVerifierReputation(goCtx, att.VerifierAddress)
		rep.CorrectAttestations++
		// Increase reputation score
		rep.ReputationScore = rep.ReputationScore.Add(math.OneInt())
		if err := ms.SetVerifierReputation(goCtx, rep); err != nil {
			ms.Logger().Error("failed to update verifier reputation on finalization",
				"verifier", att.VerifierAddress, "error", err,
			)
		}
	}

	ms.Logger().Info("batch finalized",
		"batch_id", msg.BatchId,
		"record_count", batch.RecordCount,
		"attestation_count", len(attestations),
	)

	// Emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"por_finalize_batch",
			sdk.NewAttribute("batch_id", fmt.Sprintf("%d", msg.BatchId)),
			sdk.NewAttribute("app_id", fmt.Sprintf("%d", batch.AppId)),
			sdk.NewAttribute("record_count", fmt.Sprintf("%d", batch.RecordCount)),
			sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", len(attestations))),
			sdk.NewAttribute("finalized_at", fmt.Sprintf("%d", now)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Authority),
		),
	})

	return &types.MsgFinalizeBatchResponse{}, nil
}
