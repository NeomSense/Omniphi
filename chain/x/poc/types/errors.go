package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/poc module sentinel errors
var (
	ErrInvalidCType          = errorsmod.Register(ModuleName, 1, "invalid contribution type")
	ErrInvalidURI            = errorsmod.Register(ModuleName, 2, "invalid URI")
	ErrInvalidHash           = errorsmod.Register(ModuleName, 3, "invalid hash")
	ErrContributionNotFound  = errorsmod.Register(ModuleName, 4, "contribution not found")
	ErrNotValidator          = errorsmod.Register(ModuleName, 5, "signer is not a validator")
	ErrZeroPower             = errorsmod.Register(ModuleName, 6, "validator has zero power")
	ErrAlreadyEndorsed       = errorsmod.Register(ModuleName, 7, "validator already endorsed this contribution")
	ErrNoCredits             = errorsmod.Register(ModuleName, 8, "no credits available to withdraw")
	ErrRateLimitExceeded     = errorsmod.Register(ModuleName, 9, "submission rate limit exceeded")
	ErrInvalidQuorumPct      = errorsmod.Register(ModuleName, 10, "invalid quorum percentage")
	ErrInvalidRewardUnit     = errorsmod.Register(ModuleName, 11, "invalid reward unit")
	ErrInvalidInflationShare = errorsmod.Register(ModuleName, 12, "invalid inflation share")
	ErrInvalidContributor    = errorsmod.Register(ModuleName, 13, "invalid contributor address")
	ErrInvalidSubmissionFee  = errorsmod.Register(ModuleName, 14, "invalid submission fee")
	ErrInvalidBurnRatio      = errorsmod.Register(ModuleName, 15, "invalid burn ratio")
	ErrFeeBelowMinimum       = errorsmod.Register(ModuleName, 16, "submission fee below minimum")
	ErrFeeAboveMaximum       = errorsmod.Register(ModuleName, 17, "submission fee above maximum")
	ErrBurnRatioBelowMinimum  = errorsmod.Register(ModuleName, 18, "burn ratio below minimum")
	ErrBurnRatioAboveMaximum  = errorsmod.Register(ModuleName, 19, "burn ratio above maximum")
	ErrInsufficientFee        = errorsmod.Register(ModuleName, 20, "insufficient balance to pay submission fee")
	ErrInsufficientCScore     = errorsmod.Register(ModuleName, 21, "insufficient C-Score for contribution type")
	ErrIdentityNotVerified    = errorsmod.Register(ModuleName, 22, "identity verification required for contribution type")
	ErrIdentityCheckFailed    = errorsmod.Register(ModuleName, 23, "identity verification check failed")
	ErrCTypeNotAllowed        = errorsmod.Register(ModuleName, 24, "contribution type not allowed for this contributor")
)
