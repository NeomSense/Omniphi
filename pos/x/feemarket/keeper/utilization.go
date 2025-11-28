package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetBlockUtilization returns the current block utilization as a decimal (0.0 - 1.0)
// Utilization = gas_used / max_block_gas
func (k Keeper) GetBlockUtilization(ctx context.Context) math.LegacyDec {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// During queries (non-BeginBlock/EndBlock), BlockGasMeter might be nil
	// In that case, return the stored previous utilization
	defer func() {
		if r := recover(); r != nil {
			k.Logger(ctx).Debug("recovered from panic in GetBlockUtilization, returning zero", "panic", r)
		}
	}()

	// Check if we're in a query context (no block gas meter)
	if sdkCtx.BlockGasMeter() == nil {
		// Return stored previous utilization or zero
		return k.GetPreviousBlockUtilization(ctx)
	}

	// Get gas consumed in current block
	blockGasUsed := sdkCtx.BlockGasMeter().GasConsumed()

	// Get max block gas from consensus params
	consParams := sdkCtx.ConsensusParams()
	if consParams.Block == nil {
		k.Logger(ctx).Debug("consensus params block not available, returning previous utilization")
		return k.GetPreviousBlockUtilization(ctx)
	}

	maxBlockGas := consParams.Block.MaxGas
	if maxBlockGas <= 0 {
		k.Logger(ctx).Debug("max block gas is zero or negative, returning previous utilization", "maxBlockGas", maxBlockGas)
		return k.GetPreviousBlockUtilization(ctx)
	}

	// Calculate utilization: used / max
	utilization := math.LegacyNewDec(int64(blockGasUsed)).
		QuoInt64(maxBlockGas)

	// Cap at 1.0 (100%)
	if utilization.GT(math.LegacyOneDec()) {
		utilization = math.LegacyOneDec()
	}

	return utilization
}
