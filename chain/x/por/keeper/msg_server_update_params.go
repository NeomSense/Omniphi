package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// UpdateParams handles MsgUpdateParams - governance-gated parameter update
func (ms msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
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

	// SetParams validates internally before storing
	if err := ms.SetParams(goCtx, msg.Params); err != nil {
		return nil, err
	}

	ms.Logger().Info("params updated",
		"authority", msg.Authority,
		"max_batches_per_block", msg.Params.MaxBatchesPerBlock,
		"max_finalizations_per_block", msg.Params.MaxFinalizationsPerBlock,
	)

	// Emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"por_update_params",
			sdk.NewAttribute("authority", msg.Authority),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Authority),
		),
	})

	return &types.MsgUpdateParamsResponse{}, nil
}
