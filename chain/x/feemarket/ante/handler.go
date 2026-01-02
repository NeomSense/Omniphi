package ante

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/feemarket/types"
)

// ===============================================================================
// ANCHOR LANE ANTE HANDLER INTEGRATION
// ===============================================================================
// This package provides ante decorators for the anchor lane.
// The primary decorator is MaxTxGasDecorator which enforces strict gas limits.
//
// INTEGRATION NOTES:
// - With depinject (SDK v0.50+), add this decorator to your ante handler chain
// - The decorator should be placed EARLY in the chain to reject oversized txs quickly
// - This is NOT optional for anchor lane operation
// ===============================================================================

// FeemarketKeeperI defines the minimal interface needed by ante decorators
// This allows the keeper to be injected via depinject
type FeemarketKeeperI interface {
	GetParams(ctx context.Context) types.FeeMarketParams
}

// GetMaxTxGasDecorator returns a MaxTxGasDecorator configured with the provided keeper.
// This is the preferred way to create the decorator for integration with depinject.
//
// Usage in app.go or ante handler setup:
//
//	maxTxGasDecorator := ante.GetMaxTxGasDecorator(feemarketKeeper)
//	anteDecorators := []sdk.AnteDecorator{
//	    maxTxGasDecorator,  // MUST be early in the chain
//	    // ... other decorators
//	}
func GetMaxTxGasDecorator(fmk FeemarketKeeperI) sdk.AnteDecorator {
	return NewMaxTxGasDecorator(fmk)
}

// ValidateTxGasLimit validates that a transaction's gas limit is within bounds.
// This can be called directly without going through the ante handler.
// Returns nil if valid, error if the gas limit exceeds MaxTxGas.
func ValidateTxGasLimit(ctx context.Context, fmk FeemarketKeeperI, gasLimit uint64) error {
	params := fmk.GetParams(ctx)
	return ValidateMaxTxGas(gasLimit, params.MaxTxGas)
}

// AnchorLaneLimits contains the gas limits for the anchor lane.
// Useful for clients to query limits before constructing transactions.
type AnchorLaneLimits struct {
	// MaxTxGas is the maximum gas per transaction (governance-adjustable within hard cap)
	MaxTxGas int64

	// ProtocolMaxTxGasHardCap is the absolute maximum - governance cannot exceed this
	ProtocolMaxTxGasHardCap int64

	// MaxBlockGas is the maximum gas per block
	MaxBlockGas int64

	// ProtocolMaxBlockGasHardCap is the absolute maximum block gas
	ProtocolMaxBlockGasHardCap int64

	// TargetBlockGas is the EIP-1559 target utilization
	TargetBlockGas int64
}

// GetAnchorLaneLimits returns the current anchor lane gas limits
func GetAnchorLaneLimits() AnchorLaneLimits {
	return AnchorLaneLimits{
		MaxTxGas:                   types.AnchorLaneMaxTxGas,
		ProtocolMaxTxGasHardCap:    types.ProtocolMaxTxGasHardCap,
		MaxBlockGas:                types.AnchorLaneMaxBlockGas,
		ProtocolMaxBlockGasHardCap: types.ProtocolMaxBlockGasHardCap,
		TargetBlockGas:             types.AnchorLaneTargetBlockGas,
	}
}
