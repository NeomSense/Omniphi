package types

import (
	"cosmossdk.io/errors"
)

// Guard module error codes (range: 5000-5099)
var (
	// ErrInvalidProposalID is returned when proposal ID is invalid
	ErrInvalidProposalID = errors.Register(ModuleName, 5001, "invalid proposal ID")

	// ErrProposalNotFound is returned when proposal doesn't exist
	ErrProposalNotFound = errors.Register(ModuleName, 5002, "proposal not found")

	// ErrRiskReportNotFound is returned when risk report doesn't exist
	ErrRiskReportNotFound = errors.Register(ModuleName, 5003, "risk report not found for proposal")

	// ErrQueuedExecutionNotFound is returned when queued execution doesn't exist
	ErrQueuedExecutionNotFound = errors.Register(ModuleName, 5004, "queued execution not found")

	// ErrInvalidGateState is returned when gate state transition is invalid
	ErrInvalidGateState = errors.Register(ModuleName, 5005, "invalid gate state")

	// ErrNotReady is returned when proposal is not ready for execution
	ErrNotReady = errors.Register(ModuleName, 5006, "proposal not ready for execution")

	// ErrAlreadyExecuted is returned when proposal was already executed
	ErrAlreadyExecuted = errors.Register(ModuleName, 5007, "proposal already executed")

	// ErrAlreadyAborted is returned when proposal was already aborted
	ErrAlreadyAborted = errors.Register(ModuleName, 5008, "proposal already aborted")

	// ErrSecondConfirmRequired is returned when CRITICAL proposal requires confirmation
	ErrSecondConfirmRequired = errors.Register(ModuleName, 5009, "CRITICAL proposal requires second confirmation")

	// ErrConfirmationExpired is returned when confirmation window expired
	ErrConfirmationExpired = errors.Register(ModuleName, 5010, "confirmation window expired")

	// ErrThresholdNotMet is returned when voting threshold not met
	ErrThresholdNotMet = errors.Register(ModuleName, 5011, "required voting threshold not met")

	// ErrStabilityChecksFailed is returned when stability checks fail
	ErrStabilityChecksFailed = errors.Register(ModuleName, 5012, "stability checks failed")

	// ErrTreasuryThrottled is returned when treasury outflow exceeds limit
	ErrTreasuryThrottled = errors.Register(ModuleName, 5013, "treasury outflow throttled")

	// ErrUnauthorized is returned when caller lacks permission
	ErrUnauthorized = errors.Register(ModuleName, 5014, "unauthorized")

	// ErrInvalidParams is returned when module parameters are invalid
	ErrInvalidParams = errors.Register(ModuleName, 5015, "invalid module parameters")

	// ErrProposalNotPassed is returned when proposal did not pass voting
	ErrProposalNotPassed = errors.Register(ModuleName, 5016, "proposal did not pass governance vote")

	// ErrExecutionFailed is returned when proposal execution fails
	ErrExecutionFailed = errors.Register(ModuleName, 5017, "proposal execution failed")

	// ErrBypassDetected is returned by invariant when a proposal was executed outside guard
	ErrBypassDetected = errors.Register(ModuleName, 5018, "governance execution bypass detected")

	// ErrDoubleExecution is returned when attempting to execute a proposal twice
	ErrDoubleExecution = errors.Register(ModuleName, 5019, "proposal already executed through guard")

	// ── Dynamic Deterministic Guard (DDG) v2 error codes ────────────────

	// ErrTierEscalated is emitted when a proposal's risk tier is escalated
	ErrTierEscalated = errors.Register(ModuleName, 5020, "risk tier escalated")

	// ErrThresholdEscalated is emitted when voting threshold is raised dynamically
	ErrThresholdEscalated = errors.Register(ModuleName, 5021, "voting threshold escalated")

	// ErrAggregateRiskExceeded is returned when cross-proposal risk coupling detects danger
	ErrAggregateRiskExceeded = errors.Register(ModuleName, 5022, "aggregate risk exceeded")

	// ErrEmergencyHardeningActive is returned when hardening mode blocks an action
	ErrEmergencyHardeningActive = errors.Register(ModuleName, 5023, "emergency hardening mode active")

	// ErrMonotonicViolation is logged when an attempt to lower tier/threshold is detected
	ErrMonotonicViolation = errors.Register(ModuleName, 5024, "monotonic constraint violated: tier/threshold may never decrease")

	// ── Layer 3 v2: Advisory Intelligence error codes ────────────────────

	// ErrInvalidAdvisoryType is returned when an unknown advisory type is submitted
	ErrInvalidAdvisoryType = errors.Register(ModuleName, 5030, "invalid advisory type")

	// ErrInvalidSchemaVersion is returned when the schema version is wrong
	ErrInvalidSchemaVersion = errors.Register(ModuleName, 5031, "invalid advisory schema version")

	// ErrAdvisoryNotFound is returned when an advisory entry is not found
	ErrAdvisoryNotFound = errors.Register(ModuleName, 5032, "advisory entry not found")

	// ErrAttackMemoryNotFound is returned when an attack memory entry is not found
	ErrAttackMemoryNotFound = errors.Register(ModuleName, 5033, "attack memory entry not found")

	// ── Execution control error codes ────────────────────────────────────

	// ErrExecutionDeferred is returned when execution is deferred (e.g. track frozen).
	// This allows the queue processor to reschedule rather than abort.
	ErrExecutionDeferred = errors.Register(ModuleName, 5040, "execution deferred")
)
