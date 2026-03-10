package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// ChallengeBatch handles MsgChallengeBatch - submits a fraud proof against a pending batch
func (ms msgServer) ChallengeBatch(goCtx context.Context, msg *types.MsgChallengeBatch) (*types.MsgChallengeBatchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate the message
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	// Get the batch
	batch, found := ms.GetBatch(goCtx, msg.BatchId)
	if !found {
		return nil, types.ErrBatchNotFound.Wrapf("batch_id: %d", msg.BatchId)
	}

	// Batch must be in SUBMITTED or PENDING status to accept challenges
	if batch.Status != types.BatchStatusSubmitted && batch.Status != types.BatchStatusPending {
		if batch.Status == types.BatchStatusFinalized {
			return nil, types.ErrBatchAlreadyFinalized.Wrapf("batch_id: %d", msg.BatchId)
		}
		if batch.Status == types.BatchStatusRejected {
			return nil, types.ErrBatchRejected.Wrapf("batch_id: %d", msg.BatchId)
		}
		return nil, types.ErrBatchNotSubmitted.Wrapf(
			"batch %d is in status %s", msg.BatchId, batch.Status,
		)
	}

	// Verify challenge window is still open
	now := ctx.BlockTime().Unix()
	if now > batch.ChallengeEndTime {
		return nil, types.ErrChallengeWindowClosed.Wrapf(
			"batch %d challenge window closed at %d, current time: %d",
			msg.BatchId, batch.ChallengeEndTime, now,
		)
	}

	params := ms.GetParams(goCtx)

	// SECURITY (F4): Challenge rate limiting — cap per-address challenges per epoch
	if params.MaxChallengesPerAddress > 0 {
		count := ms.GetAddressChallengeCount(goCtx, msg.Challenger, batch.Epoch)
		if count >= params.MaxChallengesPerAddress {
			return nil, types.ErrChallengeRateLimitExceeded.Wrapf(
				"address %s has submitted %d challenges in epoch %d (max %d)",
				msg.Challenger, count, batch.Epoch, params.MaxChallengesPerAddress,
			)
		}
	}

	// SECURITY (F4): Collect challenge bond from challenger
	bondAmount := params.ChallengeBondAmount
	if bondAmount.IsPositive() {
		challengerAddr, err := sdk.AccAddressFromBech32(msg.Challenger)
		if err != nil {
			return nil, fmt.Errorf("invalid challenger address: %w", err)
		}
		bondCoins := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, bondAmount))
		if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, challengerAddr, types.ModuleName, bondCoins); err != nil {
			return nil, types.ErrInsufficientChallengeBond.Wrapf(
				"challenger %s cannot pay bond %s: %s", msg.Challenger, bondAmount, err,
			)
		}
	}

	// Increment challenge rate limiter
	if params.MaxChallengesPerAddress > 0 {
		if _, err := ms.IncrementAddressChallengeCount(goCtx, msg.Challenger, batch.Epoch); err != nil {
			ms.Logger().Error("failed to increment challenge count", "challenger", msg.Challenger, "error", err)
		}
	}

	// Get next challenge ID
	challengeID, err := ms.GetNextChallengeID(goCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next challenge ID: %w", err)
	}

	// Create and store the challenge with bond amount
	challenge := types.NewChallengeWithBond(
		challengeID,
		msg.BatchId,
		msg.Challenger,
		msg.ChallengeType,
		msg.ProofData,
		now,
		bondAmount,
	)

	if err := ms.SetChallenge(goCtx, challenge); err != nil {
		return nil, fmt.Errorf("failed to store challenge: %w", err)
	}

	ms.Logger().Info("challenge submitted",
		"challenge_id", challengeID,
		"batch_id", msg.BatchId,
		"challenger", msg.Challenger,
		"challenge_type", msg.ChallengeType.String(),
		"bond_amount", bondAmount,
	)

	// Emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"por_challenge_batch",
			sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challengeID)),
			sdk.NewAttribute("batch_id", fmt.Sprintf("%d", msg.BatchId)),
			sdk.NewAttribute("challenger", msg.Challenger),
			sdk.NewAttribute("challenge_type", msg.ChallengeType.String()),
			sdk.NewAttribute("bond_amount", bondAmount.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Challenger),
		),
	})

	return &types.MsgChallengeBatchResponse{ChallengeId: challengeID}, nil
}
