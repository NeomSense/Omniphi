// Package types contains custom governance types and errors for Omniphi
package types

import (
	"cosmossdk.io/errors"
)

// Custom governance error codes
// Range 2000-2099 reserved for governance extension errors
const (
	errCodeProposalSimulationFailed = 2001
	errCodeInvalidProposalMessage   = 2002
	errCodeMessageRoutingFailed     = 2003
	errCodeProposalValidationFailed = 2004
)

var (
	// ErrProposalSimulationFailed is returned when a proposal message fails simulation
	ErrProposalSimulationFailed = errors.Register("govx", errCodeProposalSimulationFailed, "proposal message simulation failed")

	// ErrInvalidProposalMessage is returned when a proposal contains an invalid message
	ErrInvalidProposalMessage = errors.Register("govx", errCodeInvalidProposalMessage, "invalid proposal message")

	// ErrMessageRoutingFailed is returned when a proposal message cannot be routed
	ErrMessageRoutingFailed = errors.Register("govx", errCodeMessageRoutingFailed, "message routing failed")

	// ErrProposalValidationFailed is returned when proposal validation fails
	ErrProposalValidationFailed = errors.Register("govx", errCodeProposalValidationFailed, "proposal validation failed")
)
