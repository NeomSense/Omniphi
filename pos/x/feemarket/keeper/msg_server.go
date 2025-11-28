package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"pos/x/feemarket/types"
)

var _ types.MsgServer = msgServer{}

// msgServer implements the Msg gRPC service
type msgServer struct {
	types.UnimplementedMsgServer
	k Keeper
}

// NewMsgServer creates a new gRPC message server
func NewMsgServer(keeper Keeper) types.MsgServer {
	return msgServer{k: keeper}
}

// UpdateParams updates the module parameters via governance
func (ms msgServer) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if req == nil {
		return nil, types.ErrInvalidParams.Wrap("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate authority
	if ms.k.authority != req.Authority {
		return nil, govtypes.ErrInvalidSigner.Wrapf(
			"invalid authority; expected %s, got %s",
			ms.k.authority,
			req.Authority,
		)
	}

	// Validate parameters
	if err := req.Params.Validate(); err != nil {
		return nil, types.ErrInvalidParams.Wrapf("validation failed: %v", err)
	}

	// Log old and new parameters for audit trail
	oldParams := ms.k.GetParams(goCtx)
	ms.k.Logger(goCtx).Info("updating feemarket parameters via governance",
		"authority", req.Authority,
		"old_min_gas_price", oldParams.MinGasPrice.String(),
		"new_min_gas_price", req.Params.MinGasPrice.String(),
		"old_burn_cool", oldParams.BurnCool.String(),
		"new_burn_cool", req.Params.BurnCool.String(),
		"old_burn_normal", oldParams.BurnNormal.String(),
		"new_burn_normal", req.Params.BurnNormal.String(),
		"old_burn_hot", oldParams.BurnHot.String(),
		"new_burn_hot", req.Params.BurnHot.String(),
	)

	// Set new parameters
	if err := ms.k.SetParams(goCtx, req.Params); err != nil {
		return nil, types.ErrInvalidParams.Wrapf("failed to set params: %v", err)
	}

	// Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"params_updated",
			sdk.NewAttribute("module", types.ModuleName),
			sdk.NewAttribute("authority", req.Authority),
		),
	)

	ms.k.Logger(goCtx).Info("feemarket parameters updated successfully")

	return &types.MsgUpdateParamsResponse{}, nil
}
