package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/repgov module sentinel errors
var (
	ErrInvalidAuthority        = errorsmod.Register(ModuleName, 1, "invalid authority")
	ErrInvalidParams           = errorsmod.Register(ModuleName, 2, "invalid params")
	ErrInvalidAddress          = errorsmod.Register(ModuleName, 3, "invalid address")
	ErrVoterWeightNotFound     = errorsmod.Register(ModuleName, 4, "voter weight not found")
	ErrSelfDelegation          = errorsmod.Register(ModuleName, 5, "cannot delegate reputation to self")
	ErrDelegationOverflow      = errorsmod.Register(ModuleName, 6, "total delegated reputation exceeds maximum")
	ErrDelegationNotFound      = errorsmod.Register(ModuleName, 7, "delegation not found")
	ErrMaxDelegationsExceeded  = errorsmod.Register(ModuleName, 8, "max delegations per address exceeded")
	ErrInvalidWeight           = errorsmod.Register(ModuleName, 9, "invalid weight value")
	ErrInvalidProposalID       = errorsmod.Register(ModuleName, 10, "invalid proposal id")
	ErrReputationTooLow        = errorsmod.Register(ModuleName, 11, "reputation too low for governance action")
	ErrSybilDetected           = errorsmod.Register(ModuleName, 12, "sybil resistance check failed")
	ErrWeightCapExceeded       = errorsmod.Register(ModuleName, 13, "individual weight cap exceeded")
	ErrModuleDisabled          = errorsmod.Register(ModuleName, 14, "reputation-weighted governance is disabled")
	ErrInvalidReputationSource = errorsmod.Register(ModuleName, 15, "invalid reputation source weight")
)
