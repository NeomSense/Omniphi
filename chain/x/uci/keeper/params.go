package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"pos/x/uci/types"
)

func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal uci params: %w", err)
	}
	return kvStore.Set(types.KeyParams, bz)
}

func (k Keeper) GetParams(ctx context.Context) types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyParams)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := json.Unmarshal(bz, &params); err != nil {
		k.logger.Error("failed to unmarshal uci params", "error", err)
		return types.DefaultParams()
	}
	return params
}
