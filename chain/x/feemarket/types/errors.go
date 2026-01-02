package types

import (
	"cosmossdk.io/errors"
)

// x/feemarket module sentinel errors
var (
	ErrInvalidParams            = errors.Register(ModuleName, 1, "invalid parameters")
	ErrInvalidBaseFee           = errors.Register(ModuleName, 2, "invalid base fee")
	ErrInvalidUtilization       = errors.Register(ModuleName, 3, "invalid block utilization")
	ErrInvalidBurnRatio         = errors.Register(ModuleName, 4, "invalid burn ratio")
	ErrInvalidFeeRatio          = errors.Register(ModuleName, 5, "invalid fee distribution ratio")
	ErrTreasuryAddressNotSet    = errors.Register(ModuleName, 6, "treasury address not set")
	ErrInsufficientFees         = errors.Register(ModuleName, 7, "insufficient fees collected")
	ErrBurnFailed               = errors.Register(ModuleName, 8, "failed to burn tokens")
	ErrTreasuryTransferFailed   = errors.Register(ModuleName, 9, "failed to transfer to treasury")
	ErrInvalidElasticity        = errors.Register(ModuleName, 10, "invalid elasticity multiplier")
	ErrInvalidGasPrice          = errors.Register(ModuleName, 11, "invalid gas price")
	ErrMaxBurnExceeded          = errors.Register(ModuleName, 12, "burn ratio exceeds maximum")
	ErrInvalidThreshold         = errors.Register(ModuleName, 13, "invalid utilization threshold")
	ErrInvalidMaxBlockGas       = errors.Register(ModuleName, 14, "invalid max block gas for anchor lane")
	ErrInvalidMaxTxGas          = errors.Register(ModuleName, 15, "invalid max transaction gas")
)
