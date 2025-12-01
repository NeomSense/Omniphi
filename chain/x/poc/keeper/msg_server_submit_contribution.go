package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// SubmitContribution handles the submission of a new contribution
func (ms msgServer) SubmitContribution(goCtx context.Context, msg *types.MsgSubmitContribution) (*types.MsgSubmitContributionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Convert contributor address
	contributor, err := sdk.AccAddressFromBech32(msg.Contributor)
	if err != nil {
		return nil, fmt.Errorf("invalid contributor address: %w", err)
	}

	// THREE-LAYER VERIFICATION PIPELINE
	// Layer 1: PoE (Proof of Existence) - Already validated in msg.ValidateBasic()
	// Layer 2: PoA (Proof of Authority) - Check C-Score and identity requirements
	// Layer 3: PoV (Proof of Value) - Validator endorsements (happens later)

	// LAYER 2: PoA - Check access control requirements
	if err := ms.CheckProofOfAuthority(goCtx, contributor, msg.Ctype); err != nil {
		return nil, err
	}

	// Check rate limit
	if err := ms.CheckRateLimit(goCtx); err != nil {
		return nil, err
	}

	// CALCULATE 3-LAYER FEE
	// This combines:
	// 1. Base Fee Model (static base)
	// 2. Epoch-Adaptive Fee (dynamic congestion multiplier)
	// 3. C-Score Weighted Discount (reputation-based reduction)
	finalFee, epochMultiplier, cscoreDiscount, err := ms.Calculate3LayerFee(goCtx, contributor)
	if err != nil {
		return nil, fmt.Errorf("fee calculation failed: %w", err)
	}

	// INCREMENT BLOCK SUBMISSION COUNTER (for next submission's epoch multiplier)
	ms.IncrementBlockSubmissions(goCtx)

	// COLLECT AND SPLIT FEE BEFORE creating contribution
	// This ensures atomicity - if fee payment fails, contribution is not created
	// Split: 50% burned, 50% to reward pool
	if err := ms.CollectAndSplit3LayerFee(goCtx, contributor, finalFee, epochMultiplier, cscoreDiscount); err != nil {
		return nil, fmt.Errorf("fee collection failed: %w", err)
	}

	// Get next ID
	id, err := ms.GetNextContributionID(goCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next contribution ID: %w", err)
	}

	// Create contribution
	contribution := types.NewContribution(
		id,
		msg.Contributor,
		msg.Ctype,
		msg.Uri,
		msg.Hash,
		ctx.BlockHeight(),
		ctx.BlockTime().Unix(),
	)

	// Store contribution
	if err := ms.SetContribution(goCtx, contribution); err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_submit",
			sdk.NewAttribute("id", fmt.Sprintf("%d", id)),
			sdk.NewAttribute("contributor", msg.Contributor),
			sdk.NewAttribute("ctype", msg.Ctype),
			sdk.NewAttribute("uri", msg.Uri),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Contributor),
		),
	})

	return &types.MsgSubmitContributionResponse{
		Id: id,
	}, nil
}
