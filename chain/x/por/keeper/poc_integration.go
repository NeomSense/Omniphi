package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// AwardPocCredits awards PoC credits for a finalized batch.
// This is the integration point between PoR and PoC modules.
// Credits = record_count * base_record_reward
func (k Keeper) AwardPocCredits(ctx sdk.Context, batch types.BatchCommitment) error {
	if k.pocKeeper == nil {
		k.Logger().Debug("PoC keeper not available, skipping credit award",
			"batch_id", batch.BatchId,
		)
		return nil
	}

	if batch.Status != types.BatchStatusFinalized {
		return fmt.Errorf("cannot award credits for non-finalized batch %d (status: %s)",
			batch.BatchId, batch.Status,
		)
	}

	params := k.GetParams(ctx)
	credits := params.BaseRecordReward.Mul(math.NewIntFromUint64(batch.RecordCount))

	if credits.IsZero() || credits.IsNegative() {
		k.Logger().Debug("no credits to award",
			"batch_id", batch.BatchId,
			"record_count", batch.RecordCount,
			"base_reward", params.BaseRecordReward,
		)
		return nil
	}

	submitterAddr, err := sdk.AccAddressFromBech32(batch.Submitter)
	if err != nil {
		return fmt.Errorf("invalid submitter address %s: %w", batch.Submitter, err)
	}

	if err := k.pocKeeper.AddCreditsWithOverflowCheck(ctx, submitterAddr, credits); err != nil {
		return fmt.Errorf("failed to add PoC credits: %w", err)
	}

	k.Logger().Info("PoC credits awarded via PoR integration",
		"batch_id", batch.BatchId,
		"submitter", batch.Submitter,
		"credits", credits,
		"record_count", batch.RecordCount,
	)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"por_poc_credits_awarded",
		sdk.NewAttribute("batch_id", fmt.Sprintf("%d", batch.BatchId)),
		sdk.NewAttribute("submitter", batch.Submitter),
		sdk.NewAttribute("credits", credits.String()),
		sdk.NewAttribute("record_count", fmt.Sprintf("%d", batch.RecordCount)),
	))

	return nil
}

// GetBatchRewardEstimate calculates the estimated PoC credits for a batch
// without actually awarding them. Useful for UI display.
func (k Keeper) GetBatchRewardEstimate(ctx sdk.Context, recordCount uint64) math.Int {
	params := k.GetParams(ctx)
	return params.BaseRecordReward.Mul(math.NewIntFromUint64(recordCount))
}
