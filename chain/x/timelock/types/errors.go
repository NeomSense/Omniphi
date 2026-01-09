package types

import (
	"cosmossdk.io/errors"
)

// Timelock module error codes (range: 3000-3099)
var (
	// ErrOperationNotFound is returned when an operation doesn't exist
	ErrOperationNotFound = errors.Register(ModuleName, 3001, "operation not found")

	// ErrOperationNotQueued is returned when operation is not in QUEUED status
	ErrOperationNotQueued = errors.Register(ModuleName, 3002, "operation is not in queued status")

	// ErrOperationNotExecutable is returned when operation is not yet executable
	ErrOperationNotExecutable = errors.Register(ModuleName, 3003, "operation is not yet executable")

	// ErrOperationExpired is returned when operation has expired
	ErrOperationExpired = errors.Register(ModuleName, 3004, "operation has expired")

	// ErrOperationAlreadyExecuted is returned when operation was already executed
	ErrOperationAlreadyExecuted = errors.Register(ModuleName, 3005, "operation already executed")

	// ErrOperationCancelled is returned when operation was cancelled
	ErrOperationCancelled = errors.Register(ModuleName, 3006, "operation was cancelled")

	// ErrUnauthorized is returned when caller lacks permission
	ErrUnauthorized = errors.Register(ModuleName, 3007, "unauthorized")

	// ErrInvalidGuardian is returned when guardian address is invalid
	ErrInvalidGuardian = errors.Register(ModuleName, 3008, "invalid guardian address")

	// ErrNotGuardian is returned when caller is not the guardian
	ErrNotGuardian = errors.Register(ModuleName, 3009, "caller is not the guardian")

	// ErrInvalidExecutor is returned when executor address is invalid
	ErrInvalidExecutor = errors.Register(ModuleName, 3010, "invalid executor address")

	// ErrExecutorMismatch is returned when executor doesn't match queued executor
	ErrExecutorMismatch = errors.Register(ModuleName, 3011, "executor does not match queued executor")

	// ErrInvalidDelay is returned when delay parameters are invalid
	ErrInvalidDelay = errors.Register(ModuleName, 3012, "invalid delay parameters")

	// ErrMinDelayTooShort is returned when minimum delay is below absolute minimum
	ErrMinDelayTooShort = errors.Register(ModuleName, 3013, "minimum delay is below absolute minimum (1 hour)")

	// ErrMaxDelayTooLong is returned when maximum delay exceeds limit
	ErrMaxDelayTooLong = errors.Register(ModuleName, 3014, "maximum delay exceeds limit (30 days)")

	// ErrGracePeriodInvalid is returned when grace period is invalid
	ErrGracePeriodInvalid = errors.Register(ModuleName, 3015, "grace period must be at least 1 hour")

	// ErrEmergencyDelayInvalid is returned when emergency delay is invalid
	ErrEmergencyDelayInvalid = errors.Register(ModuleName, 3016, "emergency delay must be at least 1 hour")

	// ErrEmergencyNotEligible is returned when operation can't use emergency execution
	ErrEmergencyNotEligible = errors.Register(ModuleName, 3017, "operation not eligible for emergency execution")

	// ErrEmergencyAlreadyUsed is returned when operation already had emergency execution
	ErrEmergencyAlreadyUsed = errors.Register(ModuleName, 3018, "emergency execution already used for this operation")

	// ErrCancelReasonRequired is returned when cancel reason is missing
	ErrCancelReasonRequired = errors.Register(ModuleName, 3019, "cancellation reason is required")

	// ErrJustificationRequired is returned when emergency justification is missing
	ErrJustificationRequired = errors.Register(ModuleName, 3020, "emergency execution justification is required")

	// ErrNoMessages is returned when operation has no messages
	ErrNoMessages = errors.Register(ModuleName, 3021, "operation has no messages")

	// ErrInvalidOperationHash is returned when operation hash is invalid
	ErrInvalidOperationHash = errors.Register(ModuleName, 3022, "invalid operation hash")

	// ErrOperationHashMismatch is returned when computed hash doesn't match stored hash
	ErrOperationHashMismatch = errors.Register(ModuleName, 3023, "operation hash mismatch")

	// ErrMessageExecutionFailed is returned when message execution fails
	ErrMessageExecutionFailed = errors.Register(ModuleName, 3024, "message execution failed")

	// ErrInvalidProposalID is returned when proposal ID is invalid
	ErrInvalidProposalID = errors.Register(ModuleName, 3025, "invalid proposal ID")

	// ErrOperationAlreadyExists is returned when operation with same hash exists
	ErrOperationAlreadyExists = errors.Register(ModuleName, 3026, "operation with same hash already exists")

	// ErrInvalidParams is returned when module parameters are invalid
	ErrInvalidParams = errors.Register(ModuleName, 3027, "invalid module parameters")

	// ErrDelayOrderInvalid is returned when min_delay > max_delay
	ErrDelayOrderInvalid = errors.Register(ModuleName, 3028, "min_delay must be less than or equal to max_delay")

	// ErrEmergencyExceedsMin is returned when emergency_delay >= min_delay
	ErrEmergencyExceedsMin = errors.Register(ModuleName, 3029, "emergency_delay must be less than min_delay")
)
