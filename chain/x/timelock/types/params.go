package types

import (
	"fmt"
	"time"
)

// Security constants - absolute minimums that cannot be overridden
const (
	// AbsoluteMinDelaySeconds is the minimum delay that cannot be reduced (1 hour = 3600 seconds)
	// This ensures even emergency operations have a review window
	AbsoluteMinDelaySeconds uint64 = 3600

	// AbsoluteMaxDelaySeconds is the maximum delay allowed (30 days = 2592000 seconds)
	// Prevents indefinite queueing that could lock governance
	AbsoluteMaxDelaySeconds uint64 = 30 * 24 * 3600

	// AbsoluteMinGracePeriodSeconds is the minimum grace period (1 hour = 3600 seconds)
	AbsoluteMinGracePeriodSeconds uint64 = 3600

	// DefaultMinDelaySeconds is the default minimum delay (24 hours = 86400 seconds)
	DefaultMinDelaySeconds uint64 = 24 * 3600

	// DefaultMaxDelaySeconds is the default maximum delay (14 days = 1209600 seconds)
	DefaultMaxDelaySeconds uint64 = 14 * 24 * 3600

	// DefaultGracePeriodSeconds is the default grace period (7 days = 604800 seconds)
	DefaultGracePeriodSeconds uint64 = 7 * 24 * 3600

	// DefaultEmergencyDelaySeconds is the default emergency delay (1 hour = 3600 seconds)
	DefaultEmergencyDelaySeconds uint64 = 3600

	// Legacy time.Duration constants for backward compatibility in tests
	AbsoluteMinDelay       = 1 * time.Hour
	AbsoluteMaxDelay       = 30 * 24 * time.Hour
	AbsoluteMinGracePeriod = 1 * time.Hour
	DefaultMinDelay        = 24 * time.Hour
	DefaultMaxDelay        = 14 * 24 * time.Hour
	DefaultGracePeriod     = 7 * 24 * time.Hour
	DefaultEmergencyDelay  = 1 * time.Hour

	// MinCancelReasonLength is the minimum length for cancellation reason
	MinCancelReasonLength = 10

	// MaxCancelReasonLength is the maximum length for cancellation reason
	MaxCancelReasonLength = 500

	// MinJustificationLength is the minimum length for emergency justification
	MinJustificationLength = 20

	// MaxJustificationLength is the maximum length for emergency justification
	MaxJustificationLength = 1000
)

// Status constants that map to the proto-generated OperationStatus
const (
	OperationStatusUnspecified = OperationStatus_OPERATION_STATUS_UNSPECIFIED
	OperationStatusQueued      = OperationStatus_OPERATION_STATUS_QUEUED
	OperationStatusExecuted    = OperationStatus_OPERATION_STATUS_EXECUTED
	OperationStatusCancelled   = OperationStatus_OPERATION_STATUS_CANCELLED
	OperationStatusExpired     = OperationStatus_OPERATION_STATUS_EXPIRED
	OperationStatusFailed      = OperationStatus_OPERATION_STATUS_FAILED
)

// IsTerminal returns true if the status is a terminal state
func (s OperationStatus) IsTerminal() bool {
	return s == OperationStatusExecuted ||
		s == OperationStatusCancelled ||
		s == OperationStatusExpired ||
		s == OperationStatusFailed
}

// DefaultParams returns the default module parameters
func DefaultParams() Params {
	return Params{
		MinDelaySeconds:       DefaultMinDelaySeconds,
		MaxDelaySeconds:       DefaultMaxDelaySeconds,
		GracePeriodSeconds:    DefaultGracePeriodSeconds,
		EmergencyDelaySeconds: DefaultEmergencyDelaySeconds,
		Guardian:              "", // Must be set during genesis or via governance
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
	if p.MinDelaySeconds < AbsoluteMinDelaySeconds {
		return fmt.Errorf("%w: got %v seconds, minimum is %v seconds",
			ErrMinDelayTooShort, p.MinDelaySeconds, AbsoluteMinDelaySeconds)
	}

	// Check absolute maximum
	if p.MaxDelaySeconds > AbsoluteMaxDelaySeconds {
		return fmt.Errorf("%w: got %v seconds, maximum is %v seconds",
			ErrMaxDelayTooLong, p.MaxDelaySeconds, AbsoluteMaxDelaySeconds)
	}

	// Check ordering
	if p.MinDelaySeconds > p.MaxDelaySeconds {
		return fmt.Errorf("%w: min_delay (%v seconds) > max_delay (%v seconds)",
			ErrDelayOrderInvalid, p.MinDelaySeconds, p.MaxDelaySeconds)
	}

	return nil
}

// validateGracePeriod validates the grace period
func (p Params) validateGracePeriod() error {
	if p.GracePeriodSeconds < AbsoluteMinGracePeriodSeconds {
		return fmt.Errorf("%w: got %v seconds, minimum is %v seconds",
			ErrGracePeriodInvalid, p.GracePeriodSeconds, AbsoluteMinGracePeriodSeconds)
	}

	return nil
}

// validateEmergencyDelay validates the emergency delay
func (p Params) validateEmergencyDelay() error {
	// Emergency delay must be at least the absolute minimum
	if p.EmergencyDelaySeconds < AbsoluteMinDelaySeconds {
		return fmt.Errorf("%w: got %v seconds, minimum is %v seconds",
			ErrEmergencyDelayInvalid, p.EmergencyDelaySeconds, AbsoluteMinDelaySeconds)
	}

	// Emergency delay must be less than regular min delay
	if p.EmergencyDelaySeconds >= p.MinDelaySeconds {
		return fmt.Errorf("%w: emergency_delay (%v seconds) >= min_delay (%v seconds)",
			ErrEmergencyExceedsMin, p.EmergencyDelaySeconds, p.MinDelaySeconds)
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

// Helper methods for time conversion

// MinDelayDuration returns the minimum delay as a time.Duration
func (p Params) MinDelayDuration() time.Duration {
	return time.Duration(p.MinDelaySeconds) * time.Second
}

// MaxDelayDuration returns the maximum delay as a time.Duration
func (p Params) MaxDelayDuration() time.Duration {
	return time.Duration(p.MaxDelaySeconds) * time.Second
}

// GracePeriodDuration returns the grace period as a time.Duration
func (p Params) GracePeriodDuration() time.Duration {
	return time.Duration(p.GracePeriodSeconds) * time.Second
}

// EmergencyDelayDuration returns the emergency delay as a time.Duration
func (p Params) EmergencyDelayDuration() time.Duration {
	return time.Duration(p.EmergencyDelaySeconds) * time.Second
}
