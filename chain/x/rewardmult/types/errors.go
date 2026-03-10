package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/rewardmult module sentinel errors
var (
	ErrInvalidAuthority       = errorsmod.Register(ModuleName, 1, "invalid authority")
	ErrInvalidMultiplierRange = errorsmod.Register(ModuleName, 2, "invalid multiplier range")
	ErrInvalidEMAWindow       = errorsmod.Register(ModuleName, 3, "invalid EMA window")
	ErrInvalidLookbackEpochs  = errorsmod.Register(ModuleName, 4, "invalid lookback epochs")
	ErrInvalidUptimeThreshold = errorsmod.Register(ModuleName, 5, "invalid uptime threshold")
	ErrInvalidBonusValue      = errorsmod.Register(ModuleName, 6, "invalid bonus value")
	ErrInvalidPenaltyValue    = errorsmod.Register(ModuleName, 7, "invalid penalty value")
	ErrValidatorNotFound      = errorsmod.Register(ModuleName, 8, "validator not found")
	ErrMultiplierNotFound     = errorsmod.Register(ModuleName, 9, "multiplier not found")
	ErrInvalidParams          = errorsmod.Register(ModuleName, 10, "invalid params")
	ErrNormalizationFailed    = errorsmod.Register(ModuleName, 11, "budget-neutral normalization failed")
	ErrMaxParticipationBonus  = errorsmod.Register(ModuleName, 12, "invalid max participation bonus")

	// V2.2 Errors
	ErrSnapshotMismatch    = errorsmod.Register(ModuleName, 13, "stake snapshot sum does not match total bonded")
	ErrIterativeNormFailed = errorsmod.Register(ModuleName, 14, "iterative normalization did not converge")
)
