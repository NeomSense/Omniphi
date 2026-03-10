package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"pos/x/rewardmult/types"
)

// GetParams returns the module parameters
func (k Keeper) GetParams(ctx context.Context) types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyParams)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}

	var params types.Params
	if err := json.Unmarshal(bz, &params); err != nil {
		k.logger.Error("failed to unmarshal params, returning defaults", "error", err)
		return types.DefaultParams()
	}
	return params
}

// SetParams sets the module parameters
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}
	return kvStore.Set(types.KeyParams, bz)
}
