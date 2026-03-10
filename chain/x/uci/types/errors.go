package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/uci module sentinel errors
var (
	ErrInvalidAuthority        = errorsmod.Register(ModuleName, 1, "invalid authority")
	ErrInvalidParams           = errorsmod.Register(ModuleName, 2, "invalid params")
	ErrAdapterNotFound         = errorsmod.Register(ModuleName, 3, "adapter not found")
	ErrAdapterAlreadyExists    = errorsmod.Register(ModuleName, 4, "adapter already exists")
	ErrInvalidAdapterConfig    = errorsmod.Register(ModuleName, 5, "invalid adapter config")
	ErrAdapterSuspended        = errorsmod.Register(ModuleName, 6, "adapter is suspended")
	ErrOracleNotAuthorized     = errorsmod.Register(ModuleName, 7, "oracle not authorized")
	ErrInvalidAttestation      = errorsmod.Register(ModuleName, 8, "invalid attestation")
	ErrExternalIDAlreadyMapped = errorsmod.Register(ModuleName, 9, "external id already mapped")
	ErrMappingNotFound         = errorsmod.Register(ModuleName, 10, "contribution mapping not found")
	ErrSchemaValidationFailed  = errorsmod.Register(ModuleName, 11, "schema validation failed")
	ErrAdapterRateLimited      = errorsmod.Register(ModuleName, 12, "adapter rate limit exceeded")
	ErrInvalidOracleSignature  = errorsmod.Register(ModuleName, 13, "invalid oracle signature")
	ErrBatchNotFound           = errorsmod.Register(ModuleName, 14, "batch not found")
	ErrAdapterOwnerMismatch    = errorsmod.Register(ModuleName, 15, "adapter owner mismatch")
	ErrMaxAdaptersExceeded     = errorsmod.Register(ModuleName, 16, "max adapters exceeded")
	ErrInvalidExternalID       = errorsmod.Register(ModuleName, 17, "invalid external id")
	ErrModuleDisabled          = errorsmod.Register(ModuleName, 18, "module disabled")
	ErrInvalidSchema           = errorsmod.Register(ModuleName, 19, "invalid schema")
	ErrAdapterDeregistered     = errorsmod.Register(ModuleName, 20, "adapter deregistered")
)
