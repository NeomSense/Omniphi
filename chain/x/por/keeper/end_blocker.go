package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// EndBlocker processes pending batches for finalization.
// SAFETY: This method MUST NEVER panic or halt the chain.
// All errors are logged and processing continues.
func (k Keeper) EndBlocker(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	now := ctx.BlockTime().Unix()

	// Step 1: Resolve open challenges on batches past their challenge window.
	// This must happen BEFORE finalization so that resolved batches can
	// proceed to finalization (or rejection) in the same block.
	pendingForResolution := k.GetBatchesByStatus(ctx, types.BatchStatusPending)
	for _, batch := range pendingForResolution {
		if now <= batch.ChallengeEndTime {
			continue
		}
		if k.HasOpenChallenges(ctx, batch.BatchId) {
			if err := k.resolveOpenChallenges(ctx, batch.BatchId); err != nil {
				k.Logger().Error("EndBlocker: challenge resolution failed",
					"batch_id", batch.BatchId,
					"error", err,
				)
			}
		}
	}

	// Step 2: Re-fetch pending batches — resolutions may have changed statuses
	pendingBatches := k.GetBatchesByStatus(ctx, types.BatchStatusPending)

	finalized := uint32(0)
	for _, batch := range pendingBatches {
		// Cap processing to prevent gas exhaustion
		if finalized >= params.MaxFinalizationsPerBlock {
			k.Logger().Info("EndBlocker finalization cap reached",
				"finalized", finalized,
				"remaining", len(pendingBatches)-int(finalized),
				"cap", params.MaxFinalizationsPerBlock,
			)
			break
		}

		// Skip if challenge window hasn't expired yet
		if now <= batch.ChallengeEndTime {
			continue
		}

		// Skip if there are open challenges
		if k.HasOpenChallenges(ctx, batch.BatchId) {
			k.Logger().Debug("skipping batch with open challenges",
				"batch_id", batch.BatchId,
			)
			continue
		}

		// Finalize the batch (all errors logged, never panicked)
		if err := k.finalizeBatchInEndBlocker(ctx, &batch, now, params); err != nil {
			k.Logger().Error("EndBlocker: failed to finalize batch",
				"batch_id", batch.BatchId,
				"error", err,
			)
			continue // never halt the chain
		}

		finalized++
	}

	if finalized > 0 {
		k.Logger().Info("EndBlocker: finalized batches",
			"count", finalized,
			"block_height", ctx.BlockHeight(),
		)
	}

	return nil
}

// finalizeBatchInEndBlocker performs the finalization logic for a single batch.
// This is called by EndBlocker and must be safe (no panics).
func (k Keeper) finalizeBatchInEndBlocker(ctx sdk.Context, batch *types.BatchCommitment, now int64, params types.Params) error {
	// Set finalization timestamp
	batch.FinalizedAt = now

	// Update status to FINALIZED
	if err := k.UpdateBatchStatus(ctx, batch, types.BatchStatusFinalized); err != nil {
		return fmt.Errorf("failed to update batch status: %w", err)
	}

	// Award PoC credits for the finalized batch (if PoC keeper is available)
	if k.pocKeeper != nil {
		credits := params.BaseRecordReward.Mul(math.NewIntFromUint64(batch.RecordCount))

		// SECURITY (F2/F6): Enforce per-batch credit cap
		if params.MaxCreditsPerBatch.IsPositive() && credits.GT(params.MaxCreditsPerBatch) {
			k.Logger().Info("EndBlocker: capping batch credits",
				"batch_id", batch.BatchId,
				"computed", credits,
				"cap", params.MaxCreditsPerBatch,
			)
			credits = params.MaxCreditsPerBatch
		}

		// SECURITY (F2/F6): Enforce per-epoch credit cap
		if params.MaxCreditsPerEpoch.IsPositive() && credits.IsPositive() {
			epochUsed := k.GetEpochCreditsUsed(ctx, batch.Epoch)
			remaining := params.MaxCreditsPerEpoch.Sub(epochUsed)
			if remaining.IsZero() || remaining.IsNegative() {
				k.Logger().Info("EndBlocker: epoch credit cap exhausted, skipping credit award",
					"batch_id", batch.BatchId,
					"epoch", batch.Epoch,
					"epoch_used", epochUsed,
					"cap", params.MaxCreditsPerEpoch,
				)
				credits = math.ZeroInt()
			} else if credits.GT(remaining) {
				k.Logger().Info("EndBlocker: capping credits to remaining epoch budget",
					"batch_id", batch.BatchId,
					"computed", credits,
					"remaining", remaining,
				)
				credits = remaining
			}
		}

		submitterAddr, err := sdk.AccAddressFromBech32(batch.Submitter)
		if err == nil && credits.IsPositive() {
			if err := k.pocKeeper.AddCreditsWithOverflowCheck(ctx, submitterAddr, credits); err != nil {
				// Non-critical: log but don't fail finalization
				k.Logger().Error("EndBlocker: failed to award PoC credits",
					"batch_id", batch.BatchId,
					"submitter", batch.Submitter,
					"credits", credits,
					"error", err,
				)
			} else {
				// Track epoch credit usage
				if _, err := k.IncrementEpochCredits(ctx, batch.Epoch, credits); err != nil {
					k.Logger().Error("EndBlocker: failed to track epoch credits",
						"batch_id", batch.BatchId,
						"epoch", batch.Epoch,
						"error", err,
					)
				}
			}
		}
	}

	// Update verifier reputations for all attesters
	attestations := k.GetAttestationsForBatch(ctx, batch.BatchId)
	for _, att := range attestations {
		rep := k.GetOrCreateVerifierReputation(ctx, att.VerifierAddress)
		rep.CorrectAttestations++
		rep.ReputationScore = rep.ReputationScore.Add(math.OneInt())
		if err := k.SetVerifierReputation(ctx, rep); err != nil {
			k.Logger().Error("EndBlocker: failed to update verifier reputation",
				"verifier", att.VerifierAddress, "error", err,
			)
		}
	}

	// Emit finalization event
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"por_batch_auto_finalized",
		sdk.NewAttribute("batch_id", fmt.Sprintf("%d", batch.BatchId)),
		sdk.NewAttribute("app_id", fmt.Sprintf("%d", batch.AppId)),
		sdk.NewAttribute("record_count", fmt.Sprintf("%d", batch.RecordCount)),
		sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", len(attestations))),
		sdk.NewAttribute("finalized_at", fmt.Sprintf("%d", now)),
	))

	return nil
}
