package keeper

import (
	"context"

	"pos/x/rewardmult/types"
)

type msgServer struct {
	Keeper
}

var _ types.MsgServer = msgServer{}

// NewMsgServerImpl returns an implementation of the MsgServer interface
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// UpdateParams handles governance-gated parameter updates
func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != ms.GetAuthority() {
		return nil, types.ErrInvalidAuthority
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, types.ErrInvalidParams.Wrap(err.Error())
	}

	if err := ms.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
