package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// WithdrawPOCRewards handles the withdrawal of PoC rewards
func (ms msgServer) WithdrawPOCRewards(goCtx context.Context, msg *types.MsgWithdrawPOCRewards) (*types.MsgWithdrawPOCRewardsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Convert address
	addr, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return nil, err
	}

	// Withdraw credits
	amount, err := ms.WithdrawCredits(goCtx, addr)
	if err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_withdraw",
			sdk.NewAttribute("address", msg.Address),
			sdk.NewAttribute("amount", amount.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Address),
		),
	})

	return &types.MsgWithdrawPOCRewardsResponse{
		Amount: amount,
	}, nil
}
