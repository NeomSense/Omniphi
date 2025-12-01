package keeper

import (
	"context"

	"pos/x/poc/types"
)

// UpdateParams updates the module parameters (governance only)
func (ms msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if ms.GetAuthority() != msg.Authority {
		return nil, types.ErrNotValidator
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}

	if err := ms.SetParams(goCtx, msg.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
