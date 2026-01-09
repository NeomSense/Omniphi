package types

import (
	"fmt"
	"time"
)

// Security constants - absolute minimums that cannot be overridden
const (
	// AbsoluteMinDelay is the minimum delay that cannot be reduced (1 hour)
	// This ensures even emergency operations have a review window
	AbsoluteMinDelay = 1 * time.Hour

	// AbsoluteMaxDelay is the maximum delay allowed (30 days)
	// Prevents indefinite queueing that could lock governance
	AbsoluteMaxDelay = 30 * 24 * time.Hour

	// AbsoluteMinGracePeriod is the minimum grace period (1 hour)
	AbsoluteMinGracePeriod = 1 * time.Hour

	// DefaultMinDelay is the default minimum delay (24 hours)
	DefaultMinDelay = 24 * time.Hour

	// DefaultMaxDelay is the default maximum delay (14 days)
	DefaultMaxDelay = 14 * 24 * time.Hour

	// DefaultGracePeriod is the default grace period (7 days)
	DefaultGracePeriod = 7 * 24 * time.Hour

	// DefaultEmergencyDelay is the default emergency delay (1 hour)
	DefaultEmergencyDelay = 1 * time.Hour

	// MinCancelReasonLength is the minimum length for cancellation reason
	MinCancelReasonLength = 10

	// MaxCancelReasonLength is the maximum length for cancellation reason
	MaxCancelReasonLength = 500

	// MinJustificationLength is the minimum length for emergency justification
	MinJustificationLength = 20

	// MaxJustificationLength is the maximum length for emergency justification
	MaxJustificationLength = 1000
)

// OperationStatus represents the state of a queued operation
type OperationStatus int32

const (
	OperationStatusUnspecified OperationStatus = 0
	OperationStatusQueued      OperationStatus = 1
	OperationStatusExecuted    OperationStatus = 2
	OperationStatusCancelled   OperationStatus = 3
	OperationStatusExpired     OperationStatus = 4
	OperationStatusFailed      OperationStatus = 5
)

// String returns the string representation of the status
func (s OperationStatus) String() string {
	switch s {
	case OperationStatusQueued:
		return "QUEUED"
	case OperationStatusExecuted:
		return "EXECUTED"
	case OperationStatusCancelled:
		return "CANCELLED"
	case OperationStatusExpired:
		return "EXPIRED"
	case OperationStatusFailed:
		return "FAILED"
	default:
		return "UNSPECIFIED"
	}
}

// IsTerminal returns true if the status is a terminal state
func (s OperationStatus) IsTerminal() bool {
	return s == OperationStatusExecuted ||
		s == OperationStatusCancelled ||
		s == OperationStatusExpired ||
		s == OperationStatusFailed
}

// Params defines the timelock module parameters
type Params struct {
	MinDelay       time.Duration
	MaxDelay       time.Duration
	GracePeriod    time.Duration
	EmergencyDelay time.Duration
	Guardian       string
}

// DefaultParams returns the default module parameters
func DefaultParams() Params {
	return Params{
		MinDelay:       DefaultMinDelay,
		MaxDelay:       DefaultMaxDelay,
		GracePeriod:    DefaultGracePeriod,
		EmergencyDelay: DefaultEmergencyDelay,
		Guardian:       "", // Must be set during genesis or via governance
	}
}

// Validate validates the module parameters
func (p Params) Validate() error {
	if err := p.validateDelays(); err != nil {
		return err
	}

	if err := p.validateGracePeriod(); err != nil {
		return err
	}

	if err := p.validateEmergencyDelay(); err != nil {
		return err
	}

	return nil
}

// validateDelays validates the delay parameters
func (p Params) validateDelays() error {
	// Check absolute minimum
	if p.MinDelay < AbsoluteMinDelay {
		return fmt.Errorf("%w: got %v, minimum is %v",
			ErrMinDelayTooShort, p.MinDelay, AbsoluteMinDelay)
	}

	// Check absolute maximum
	if p.MaxDelay > AbsoluteMaxDelay {
		return fmt.Errorf("%w: got %v, maximum is %v",
			ErrMaxDelayTooLong, p.MaxDelay, AbsoluteMaxDelay)
	}

	// Check ordering
	if p.MinDelay > p.MaxDelay {
		return fmt.Errorf("%w: min_delay (%v) > max_delay (%v)",
			ErrDelayOrderInvalid, p.MinDelay, p.MaxDelay)
	}

	return nil
}

// validateGracePeriod validates the grace period
func (p Params) validateGracePeriod() error {
	if p.GracePeriod < AbsoluteMinGracePeriod {
		return fmt.Errorf("%w: got %v, minimum is %v",
			ErrGracePeriodInvalid, p.GracePeriod, AbsoluteMinGracePeriod)
	}

	return nil
}

// validateEmergencyDelay validates the emergency delay
func (p Params) validateEmergencyDelay() error {
	// Emergency delay must be at least the absolute minimum
	if p.EmergencyDelay < AbsoluteMinDelay {
		return fmt.Errorf("%w: got %v, minimum is %v",
			ErrEmergencyDelayInvalid, p.EmergencyDelay, AbsoluteMinDelay)
	}

	// Emergency delay must be less than regular min delay
	if p.EmergencyDelay >= p.MinDelay {
		return fmt.Errorf("%w: emergency_delay (%v) >= min_delay (%v)",
			ErrEmergencyExceedsMin, p.EmergencyDelay, p.MinDelay)
	}

	return nil
}

// ValidateCancelReason validates the cancellation reason
func ValidateCancelReason(reason string) error {
	if len(reason) < MinCancelReasonLength {
		return fmt.Errorf("%w: reason must be at least %d characters",
			ErrCancelReasonRequired, MinCancelReasonLength)
	}
	if len(reason) > MaxCancelReasonLength {
		return fmt.Errorf("%w: reason exceeds maximum length of %d characters",
			ErrCancelReasonRequired, MaxCancelReasonLength)
	}
	return nil
}

// ValidateJustification validates the emergency justification
func ValidateJustification(justification string) error {
	if len(justification) < MinJustificationLength {
		return fmt.Errorf("%w: justification must be at least %d characters",
			ErrJustificationRequired, MinJustificationLength)
	}
	if len(justification) > MaxJustificationLength {
		return fmt.Errorf("%w: justification exceeds maximum length of %d characters",
			ErrJustificationRequired, MaxJustificationLength)
	}
	return nil
}
