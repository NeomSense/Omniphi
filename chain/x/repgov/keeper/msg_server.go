package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/repgov/types"
)

type msgServer struct {
	k Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{k: k}
}

// UpdateParams updates the module parameters (governance-gated)
func (ms *msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != ms.k.authority {
		return nil, types.ErrInvalidAuthority
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}

	if err := ms.k.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"repgov_params_updated",
			sdk.NewAttribute("authority", msg.Authority),
			sdk.NewAttribute("enabled", boolStr(msg.Params.Enabled)),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// DelegateReputation delegates a portion of governance reputation to another address
func (ms *msgServer) DelegateReputation(ctx context.Context, msg *types.MsgDelegateReputation) (*types.MsgDelegateReputationResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.DelegationEnabled {
		return nil, types.ErrModuleDisabled
	}

	// Check delegation amount doesn't exceed max delegable ratio
	if msg.Amount.GT(params.MaxDelegableRatio) {
		return nil, types.ErrDelegationOverflow
	}

	// Check max delegations limit
	existing := ms.k.GetDelegationsFrom(ctx, msg.Delegator)
	if int64(len(existing)) >= params.MaxDelegationsPerAddress {
		return nil, types.ErrMaxDelegationsExceeded
	}

	// Check total delegated doesn't exceed max
	totalDelegated := msg.Amount
	for _, d := range existing {
		if d.Delegatee == msg.Delegatee {
			// Updating existing delegation
			totalDelegated = msg.Amount
		} else {
			totalDelegated = totalDelegated.Add(d.Amount)
		}
	}
	if totalDelegated.GT(params.MaxDelegableRatio) {
		return nil, types.ErrDelegationOverflow
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	epoch := sdkCtx.BlockHeight() / params.RecomputeInterval

	delegation := types.DelegatedReputation{
		Delegator: msg.Delegator,
		Delegatee: msg.Delegatee,
		Amount:    msg.Amount,
		Epoch:     epoch,
	}

	if err := ms.k.SetDelegation(ctx, delegation); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"repgov_reputation_delegated",
			sdk.NewAttribute("delegator", msg.Delegator),
			sdk.NewAttribute("delegatee", msg.Delegatee),
			sdk.NewAttribute("amount", msg.Amount.String()),
		),
	)

	return &types.MsgDelegateReputationResponse{}, nil
}

// UndelegateReputation removes a reputation delegation
func (ms *msgServer) UndelegateReputation(ctx context.Context, msg *types.MsgUndelegateReputation) (*types.MsgUndelegateReputationResponse, error) {
	_, found := ms.k.GetDelegation(ctx, msg.Delegator, msg.Delegatee)
	if !found {
		return nil, types.ErrDelegationNotFound
	}

	if err := ms.k.DeleteDelegation(ctx, msg.Delegator, msg.Delegatee); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"repgov_reputation_undelegated",
			sdk.NewAttribute("delegator", msg.Delegator),
			sdk.NewAttribute("delegatee", msg.Delegatee),
		),
	)

	return &types.MsgUndelegateReputationResponse{}, nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
