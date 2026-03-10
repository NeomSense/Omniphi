package types

import (
	errorsmod "cosmossdk.io/errors"
)

var (
	ErrInvalidAuthority         = errorsmod.Register(ModuleName, 1, "invalid authority")
	ErrInvalidParams            = errorsmod.Register(ModuleName, 2, "invalid params")
	ErrTokenNotFound            = errorsmod.Register(ModuleName, 3, "royalty token not found")
	ErrNotTokenOwner            = errorsmod.Register(ModuleName, 4, "sender is not token owner")
	ErrTokenNotTransferable     = errorsmod.Register(ModuleName, 5, "token is not transferable")
	ErrClaimNotFound            = errorsmod.Register(ModuleName, 6, "contribution claim not found")
	ErrTokenAlreadyExists       = errorsmod.Register(ModuleName, 7, "royalty token already exists for this claim")
	ErrInvalidFractionCount     = errorsmod.Register(ModuleName, 8, "invalid fraction count")
	ErrFractionNotFound         = errorsmod.Register(ModuleName, 9, "fraction not found")
	ErrTokenFrozen              = errorsmod.Register(ModuleName, 10, "token is frozen due to clawback")
	ErrInvalidRoyaltyShare      = errorsmod.Register(ModuleName, 11, "invalid royalty share")
	ErrNoAccumulatedRoyalties   = errorsmod.Register(ModuleName, 12, "no accumulated royalties to claim")
	ErrListingNotFound          = errorsmod.Register(ModuleName, 13, "listing not found")
	ErrListingAlreadyExists     = errorsmod.Register(ModuleName, 14, "listing already exists for this token")
	ErrInsufficientFunds        = errorsmod.Register(ModuleName, 15, "insufficient funds")
	ErrModuleDisabled           = errorsmod.Register(ModuleName, 16, "royalty tokenization is disabled")
	ErrMaxFractionsExceeded     = errorsmod.Register(ModuleName, 17, "maximum fractions exceeded")
	ErrInvalidAddress           = errorsmod.Register(ModuleName, 18, "invalid address")
	ErrTokenAlreadyFractionalized = errorsmod.Register(ModuleName, 19, "token is already fractionalized")
)
