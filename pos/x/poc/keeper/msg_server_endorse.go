package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// Endorse handles a validator's endorsement of a contribution
func (ms msgServer) Endorse(goCtx context.Context, msg *types.MsgEndorse) (*types.MsgEndorseResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Verify signer is a validator
	validatorAddr, err := sdk.AccAddressFromBech32(msg.Validator)
	if err != nil {
		return nil, err
	}

	valAddr := sdk.ValAddress(validatorAddr)
	validator, err := ms.stakingKeeper.GetValidator(goCtx, valAddr)
	if err != nil {
		return nil, types.ErrNotValidator
	}

	// CRITICAL FIX: Use bonded tokens (not consensus power) for quorum calculation
	// The quorum check compares against total bonded tokens, so we need actual token amounts
	tokens := validator.GetTokens()

	if tokens.IsZero() {
		return nil, types.ErrZeroPower
	}

	// Create endorsement with actual bonded tokens as power
	endorsement := types.NewEndorsement(
		valAddr.String(),
		msg.Decision,
		tokens,
		ctx.BlockTime().Unix(),
	)

	// Add endorsement and check for verification
	verified, err := ms.AddEndorsement(goCtx, msg.ContributionId, endorsement)
	if err != nil {
		return nil, err
	}

	// Emit events
	events := sdk.Events{
		sdk.NewEvent(
			"poc_endorse",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", msg.ContributionId)),
			sdk.NewAttribute("validator", valAddr.String()),
			sdk.NewAttribute("decision", fmt.Sprintf("%t", msg.Decision)),
			sdk.NewAttribute("power", tokens.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Validator),
		),
	}

	if verified {
		events = append(events, sdk.NewEvent(
			"poc_verified",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", msg.ContributionId)),
		))
	}

	ctx.EventManager().EmitEvents(events)

	return &types.MsgEndorseResponse{
		Verified: verified,
	}, nil
}
