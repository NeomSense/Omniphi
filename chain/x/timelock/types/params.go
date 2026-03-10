package types

import (
	"fmt"
	"time"
)

// Security constants - absolute minimums that cannot be overridden
const (
	// AbsoluteMinDelaySeconds is the minimum delay that cannot be reduced (6 hours = 21600 seconds)
	// SECURITY: Increased from 1 hour to 6 hours to prevent blitzkrieg governance takeover.
	// A 1-hour window is insufficient for community coordination in response to malicious proposals.
	AbsoluteMinDelaySeconds uint64 = 21600

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

	// DefaultEmergencyDelaySeconds is the default emergency delay (6 hours = 21600 seconds)
	// SECURITY: Matches AbsoluteMinDelaySeconds to ensure minimum community review window
	DefaultEmergencyDelaySeconds uint64 = 21600

	// Legacy time.Duration constants for backward compatibility in tests
	AbsoluteMinDelay       = 6 * time.Hour
	AbsoluteMaxDelay       = 30 * 24 * time.Hour
	AbsoluteMinGracePeriod = 1 * time.Hour
	DefaultMinDelay        = 24 * time.Hour
	DefaultMaxDelay        = 14 * 24 * time.Hour
	DefaultGracePeriod     = 7 * 24 * time.Hour
	DefaultEmergencyDelay  = 6 * time.Hour

	// MinCancelReasonLength is the minimum length for cancellation reason
	MinCancelReasonLength = 10

	// MaxCancelReasonLength is the maximum length for cancellation reason
	MaxCancelReasonLength = 500

	// MinJustificationLength is the minimum length for emergency justification
	MinJustificationLength = 20

	// MaxJustificationLength is the maximum length for emergency justification
	MaxJustificationLength = 1000

	// MaxGuardianCancelsPerWindow is the max cancellations a guardian can perform
	// within a rolling window before being auto-revoked. Prevents guardian DoS on governance.
	MaxGuardianCancelsPerWindow uint64 = 3

	// GuardianCancelWindowBlocks is the rolling window size in blocks (~24 hours at 6s/block)
	// for tracking guardian cancel frequency
	GuardianCancelWindowBlocks int64 = 50000

	// --- AST v2: Adaptive delay constants ---

	// DelayPrecision is the fixed-point denominator used in multiplier arithmetic.
	// All multipliers are integers with implied division by DelayPrecision.
	// e.g. RiskMultiplier = 1500 means 1.5×.
	DelayPrecision uint64 = 1000

	// DefaultRiskMultiplierLow maps RISK_TIER_LOW to a 1.0× delay.
	DefaultRiskMultiplierLow uint64 = 1000
	// DefaultRiskMultiplierMed maps RISK_TIER_MED to a 1.5× delay.
	DefaultRiskMultiplierMed uint64 = 1500
	// DefaultRiskMultiplierHigh maps RISK_TIER_HIGH to a 2.0× delay.
	DefaultRiskMultiplierHigh uint64 = 2000
	// DefaultRiskMultiplierCritical maps RISK_TIER_CRITICAL to a 3.0× delay.
	DefaultRiskMultiplierCritical uint64 = 3000

	// DefaultEconomicImpactMultiplierBase is applied when treasury spend < 5%.
	DefaultEconomicImpactMultiplierBase uint64 = 1000 // 1.0×
	// DefaultEconomicImpactMultiplierMed is applied for 5–25% treasury spend.
	DefaultEconomicImpactMultiplierMed uint64 = 1400 // 1.4×
	// DefaultEconomicImpactMultiplierHigh is applied for > 25% treasury spend.
	DefaultEconomicImpactMultiplierHigh uint64 = 2000 // 2.0×

	// CumulativeTreasuryWindow is the rolling window (seconds) for cumulative
	// treasury outflow detection. Default: 24 hours.
	CumulativeTreasuryWindow int64 = 86400

	// DefaultCumulativeTreasuryEscalateBps is the threshold at which cumulative
	// treasury outflow triggers delay escalation (in bps of community pool).
	DefaultCumulativeTreasuryEscalateBps uint64 = 3000 // 30%

	// MaxFreezeDurationBlocks is the maximum number of blocks a track can be
	// frozen in a single governance freeze (≈ 30 days at 5s/block).
	MaxFreezeDurationBlocks int64 = 518400

	// ParamChangeMutationWindowBlocks is the rolling window for tracking how
	// frequently governance param changes are made (for stability predicate).
	ParamChangeMutationWindowBlocks int64 = 50000

	// ParamChangeMutationThreshold is the number of param changes within
	// ParamChangeMutationWindowBlocks that triggers a stability-check delay.
	ParamChangeMutationThreshold uint64 = 5

	// MutationFreqMultiplier is the delay multiplier applied when param
	// mutation frequency exceeds ParamChangeMutationThreshold.  Fixed-point.
	MutationFreqMultiplier uint64 = 1500 // 1.5×
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
