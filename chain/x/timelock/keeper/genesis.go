package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/timelock/types"
)

// InitGenesis initializes the module state from genesis
func (k Keeper) InitGenesis(ctx context.Context, data *types.GenesisState) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate genesis state
	if err := data.Validate(); err != nil {
		return fmt.Errorf("invalid genesis state: %w", err)
	}

	// Set params
	if err := k.SetParams(ctx, data.Params); err != nil {
		return fmt.Errorf("failed to set params: %w", err)
	}

	// Set next operation ID
	if data.NextOperationId > 0 {
		// Initialize the sequence to the next operation ID
		for i := uint64(1); i < data.NextOperationId; i++ {
			if _, err := k.NextOperationID.Next(ctx); err != nil {
				return fmt.Errorf("failed to initialize operation ID sequence: %w", err)
			}
		}
	}

	// Import operations
	for _, op := range data.Operations {
		opCopy := op
		if err := k.SetOperation(ctx, &opCopy); err != nil {
			return fmt.Errorf("failed to set operation %d: %w", op.Id, err)
		}
	}

	k.logger.Info("timelock genesis initialized",
		"height", sdkCtx.BlockHeight(),
		"operations_count", len(data.Operations),
		"guardian", data.Params.Guardian,
	)

	return nil
}

// ExportGenesis exports the module state for genesis
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get params: %w", err)
	}

	// Export all operations
	var operations []types.QueuedOperation
	err = k.Operations.Walk(ctx, nil, func(id uint64, op types.QueuedOperation) (stop bool, err error) {
		operations = append(operations, op)
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to export operations: %w", err)
	}

	// Get next operation ID
	nextID, err := k.NextOperationID.Peek(ctx)
	if err != nil {
		nextID = 1 // Default if not set
	}

	return &types.GenesisState{
		Params:          params,
		Operations:      operations,
		NextOperationId: nextID,
	}, nil
}

// GenesisState represents the genesis state of the timelock module
type GenesisState = types.GenesisState

// DefaultGenesisState returns the default genesis state
func DefaultGenesisState() *types.GenesisState {
	return &types.GenesisState{
		Params:          types.DefaultParams(),
		Operations:      []types.QueuedOperation{},
		NextOperationId: 1,
	}
}
