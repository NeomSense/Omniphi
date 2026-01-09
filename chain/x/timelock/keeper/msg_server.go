package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/timelock/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// ExecuteOperation executes a queued operation after the delay has passed
func (ms msgServer) ExecuteOperation(ctx context.Context, msg *types.MsgExecuteOperation) (*types.MsgExecuteOperationResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// Validate executor address
	if _, err := sdk.AccAddressFromBech32(msg.Executor); err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrInvalidExecutor, err)
	}

	// Execute the operation
	if err := ms.Keeper.ExecuteOperation(ctx, msg.OperationId, msg.Executor); err != nil {
		return nil, err
	}

	return &types.MsgExecuteOperationResponse{
		Success: true,
	}, nil
}

// CancelOperation cancels a queued operation (guardian only)
func (ms msgServer) CancelOperation(ctx context.Context, msg *types.MsgCancelOperation) (*types.MsgCancelOperationResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// Validate authority address
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return nil, fmt.Errorf("%w: invalid authority: %v", types.ErrUnauthorized, err)
	}

	// Validate reason
	if err := types.ValidateCancelReason(msg.Reason); err != nil {
		return nil, err
	}

	// Cancel the operation
	if err := ms.Keeper.CancelOperation(ctx, msg.OperationId, msg.Authority, msg.Reason); err != nil {
		return nil, err
	}

	return &types.MsgCancelOperationResponse{}, nil
}

// EmergencyExecute executes an operation with reduced delay (guardian only)
func (ms msgServer) EmergencyExecute(ctx context.Context, msg *types.MsgEmergencyExecute) (*types.MsgEmergencyExecuteResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// Validate authority address
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return nil, fmt.Errorf("%w: invalid authority: %v", types.ErrUnauthorized, err)
	}

	// Validate justification
	if err := types.ValidateJustification(msg.Justification); err != nil {
		return nil, err
	}

	// Emergency execute the operation
	if err := ms.Keeper.EmergencyExecute(ctx, msg.OperationId, msg.Authority, msg.Justification); err != nil {
		return nil, err
	}

	return &types.MsgEmergencyExecuteResponse{
		Success: true,
	}, nil
}

// UpdateParams updates the module parameters (governance only)
func (ms msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// Check authority
	if msg.Authority != ms.Keeper.GetAuthority() {
		return nil, fmt.Errorf("%w: expected %s, got %s",
			types.ErrUnauthorized, ms.Keeper.GetAuthority(), msg.Authority)
	}

	// Validate params
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrInvalidParams, err)
	}

	// Get current params for comparison
	oldParams, err := ms.Keeper.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// Additional security checks for parameter changes
	if err := ms.validateParamChanges(oldParams, msg.Params); err != nil {
		return nil, err
	}

	// Update params
	if err := ms.Keeper.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}

	// Emit event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"params_updated",
			sdk.NewAttribute("old_min_delay", oldParams.MinDelay.String()),
			sdk.NewAttribute("new_min_delay", msg.Params.MinDelay.String()),
			sdk.NewAttribute("old_guardian", oldParams.Guardian),
			sdk.NewAttribute("new_guardian", msg.Params.Guardian),
		),
	)

	ms.Keeper.Logger().Info("timelock params updated",
		"old_min_delay", oldParams.MinDelay,
		"new_min_delay", msg.Params.MinDelay,
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// UpdateGuardian updates the guardian address (governance only)
func (ms msgServer) UpdateGuardian(ctx context.Context, msg *types.MsgUpdateGuardian) (*types.MsgUpdateGuardianResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// Check authority
	if msg.Authority != ms.Keeper.GetAuthority() {
		return nil, fmt.Errorf("%w: expected %s, got %s",
			types.ErrUnauthorized, ms.Keeper.GetAuthority(), msg.Authority)
	}

	// Validate new guardian address
	if _, err := sdk.AccAddressFromBech32(msg.NewGuardian); err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrInvalidGuardian, err)
	}

	// Get current params
	params, err := ms.Keeper.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	oldGuardian := params.Guardian

	// Update guardian
	params.Guardian = msg.NewGuardian
	if err := ms.Keeper.SetParams(ctx, params); err != nil {
		return nil, err
	}

	// Emit event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"guardian_updated",
			sdk.NewAttribute("old_guardian", oldGuardian),
			sdk.NewAttribute("new_guardian", msg.NewGuardian),
		),
	)

	ms.Keeper.Logger().Warn("guardian updated",
		"old_guardian", oldGuardian,
		"new_guardian", msg.NewGuardian,
	)

	return &types.MsgUpdateGuardianResponse{}, nil
}

// validateParamChanges performs additional validation on parameter changes
func (ms msgServer) validateParamChanges(oldParams, newParams types.Params) error {
	// Security check: min_delay cannot be reduced by more than 50% in a single update
	// This prevents sudden dramatic reductions that could be exploited
	if newParams.MinDelay < oldParams.MinDelay/2 {
		return fmt.Errorf("min_delay cannot be reduced by more than 50%% in a single update: "+
			"old=%v, new=%v, minimum allowed=%v",
			oldParams.MinDelay, newParams.MinDelay, oldParams.MinDelay/2)
	}

	// Security check: emergency_delay cannot be reduced below 1 hour
	if newParams.EmergencyDelay < types.AbsoluteMinDelay {
		return fmt.Errorf("emergency_delay cannot be below %v", types.AbsoluteMinDelay)
	}

	return nil
}
