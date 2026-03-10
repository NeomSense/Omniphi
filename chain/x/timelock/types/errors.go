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
	ErrMinDelayTooShort = errors.Register(ModuleName, 3013, "minimum delay is below absolute minimum (6 hours)")

	// ErrMaxDelayTooLong is returned when maximum delay exceeds limit
	ErrMaxDelayTooLong = errors.Register(ModuleName, 3014, "maximum delay exceeds limit (30 days)")

	// ErrGracePeriodInvalid is returned when grace period is invalid
	ErrGracePeriodInvalid = errors.Register(ModuleName, 3015, "grace period must be at least 1 hour")

	// ErrEmergencyDelayInvalid is returned when emergency delay is invalid
	ErrEmergencyDelayInvalid = errors.Register(ModuleName, 3016, "emergency delay must be at least 6 hours")

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

	// ErrGuardianCannotCancelProtected is returned when guardian tries to cancel
	// an operation containing MsgUpdateGuardian or MsgUpdateParams (timelock).
	// This prevents the guardian from making themselves irremovable.
	ErrGuardianCannotCancelProtected = errors.Register(ModuleName, 3030, "guardian cannot cancel operations that modify guardian role or timelock params")

	// ErrGuardianAutoRevoked is returned after guardian is auto-revoked due to
	// exceeding the maximum number of cancellations within a rolling window
	ErrGuardianAutoRevoked = errors.Register(ModuleName, 3031, "guardian auto-revoked due to excessive cancellations")

	// ErrProtectedOperationEmergency is returned when guardian tries to emergency-execute
	// an operation containing MsgUpdateGuardian or MsgUpdateParams (timelock).
	// Protected operations must go through the full governance delay.
	ErrProtectedOperationEmergency = errors.Register(ModuleName, 3032, "protected operations cannot be emergency-executed; must wait for full governance delay")

	// ErrExecutionDisabled is returned when timelock execution is disabled
	// because guard is the sole executor.
	ErrExecutionDisabled = errors.Register(ModuleName, 3033, "timelock execution disabled; guard is authoritative executor")

	// --- AST v2: Track system errors (range 3033-3049) ---

	// ErrTrackNotFound is returned when a track name is not in the store.
	ErrTrackNotFound = errors.Register(ModuleName, 3034, "track not found")

	// ErrTrackPaused is returned when a proposal attempts to queue on a paused track.
	ErrTrackPaused = errors.Register(ModuleName, 3035, "track is paused: new operations cannot be queued")

	// ErrTrackFrozen is returned when execution is attempted on a frozen track.
	ErrTrackFrozen = errors.Register(ModuleName, 3036, "track is frozen until the specified height")

	// ErrInvalidTrackName is returned when an unrecognised track name is supplied.
	ErrInvalidTrackName = errors.Register(ModuleName, 3037, "invalid or unknown track name")

	// ErrInvalidTrackMultiplier is returned when a multiplier is out of [1000,5000].
	ErrInvalidTrackMultiplier = errors.Register(ModuleName, 3038, "track multiplier must be in range [1000, 5000]")

	// ErrCumulativeTreasuryExceeded is returned when queuing would push
	// 24-hour cumulative treasury outflow above the configured threshold.
	ErrCumulativeTreasuryExceeded = errors.Register(ModuleName, 3039, "cumulative 24-hour treasury outflow would exceed threshold")

	// ErrAdaptiveDelayOverflow is returned if the multi-factor delay calculation
	// would exceed AbsoluteMaxDelaySeconds before clamping logic runs.
	// (Informational; clamping always applies — this error is never user-visible
	// in normal operation but is registered so it can appear in logs.)
	ErrAdaptiveDelayOverflow = errors.Register(ModuleName, 3040, "adaptive delay overflow clamped to absolute maximum")

	// ErrFreezeTooLong is returned when freeze_until_height is too far in the future.
	ErrFreezeTooLong = errors.Register(ModuleName, 3041, "freeze_until_height exceeds maximum allowed freeze duration")
)
