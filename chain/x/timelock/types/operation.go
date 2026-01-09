package types

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoprotoany "github.com/cosmos/gogoproto/types/any"
)

// NewQueuedOperation creates a new queued operation
func NewQueuedOperation(
	id uint64,
	proposalID uint64,
	messages []sdk.Msg,
	executor string,
	queuedAt time.Time,
	minDelaySeconds uint64,
	gracePeriodSeconds uint64,
	cdc codec.Codec,
) (*QueuedOperation, error) {
	// Pack messages into Any
	anyMsgs := make([]*gogoprotoany.Any, len(messages))
	for i, msg := range messages {
		anyMsg, err := codectypes.NewAnyWithValue(msg)
		if err != nil {
			return nil, err
		}
		// Convert from codectypes.Any to gogoproto any.Any
		anyMsgs[i] = &gogoprotoany.Any{
			TypeUrl: anyMsg.TypeUrl,
			Value:   anyMsg.Value,
		}
	}

	queuedAtUnix := queuedAt.Unix()
	executableAtUnix := queuedAtUnix + int64(minDelaySeconds)
	expiresAtUnix := executableAtUnix + int64(gracePeriodSeconds)

	op := &QueuedOperation{
		Id:              id,
		ProposalId:     proposalID,
		Messages:        anyMsgs,
		QueuedAtUnix:    queuedAtUnix,
		ExecutableAtUnix: executableAtUnix,
		ExpiresAtUnix:   expiresAtUnix,
		Status:          OperationStatusQueued,
		Executor:        executor,
	}

	// Compute operation hash
	op.OperationHash = op.ComputeHash()

	return op, nil
}

// ComputeHash computes the cryptographic hash of the operation
// Hash includes: proposalID + operationID + all message type URLs and content + queued time
func (op *QueuedOperation) ComputeHash() []byte {
	h := sha256.New()

	// Include proposal ID
	proposalIDBz := make([]byte, 8)
	binary.BigEndian.PutUint64(proposalIDBz, op.ProposalId)
	h.Write(proposalIDBz)

	// Include operation ID (prevents replay)
	opIDBz := make([]byte, 8)
	binary.BigEndian.PutUint64(opIDBz, op.Id)
	h.Write(opIDBz)

	// Include each message
	for _, anyMsg := range op.Messages {
		h.Write([]byte(anyMsg.TypeUrl))
		h.Write(anyMsg.Value)
	}

	// Include queued timestamp (prevents replay with same messages)
	timeBz := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBz, uint64(op.QueuedAtUnix))
	h.Write(timeBz)

	return h.Sum(nil)
}

// VerifyHash verifies the operation hash matches the computed hash
func (op *QueuedOperation) VerifyHash() bool {
	computed := op.ComputeHash()
	if len(computed) != len(op.OperationHash) {
		return false
	}
	for i := range computed {
		if computed[i] != op.OperationHash[i] {
			return false
		}
	}
	return true
}

// GetSDKMessages unpacks and returns the SDK messages
func (op *QueuedOperation) GetSDKMessages(cdc codec.Codec) ([]sdk.Msg, error) {
	msgs := make([]sdk.Msg, len(op.Messages))
	for i, anyMsg := range op.Messages {
		// Convert gogoproto any.Any to codectypes.Any for unpacking
		codecAny := &codectypes.Any{
			TypeUrl: anyMsg.TypeUrl,
			Value:   anyMsg.Value,
		}
		var msg sdk.Msg
		if err := cdc.UnpackAny(codecAny, &msg); err != nil {
			return nil, err
		}
		msgs[i] = msg
	}
	return msgs, nil
}

// QueuedTime returns the queued time as time.Time
func (op *QueuedOperation) QueuedTime() time.Time {
	return time.Unix(op.QueuedAtUnix, 0)
}

// ExecutableTime returns the executable time as time.Time
func (op *QueuedOperation) ExecutableTime() time.Time {
	return time.Unix(op.ExecutableAtUnix, 0)
}

// ExpiresTime returns the expiration time as time.Time
func (op *QueuedOperation) ExpiresTime() time.Time {
	return time.Unix(op.ExpiresAtUnix, 0)
}

// ExecutedTime returns the executed time as time.Time (nil-safe)
func (op *QueuedOperation) ExecutedTime() *time.Time {
	if op.ExecutedAtUnix == 0 {
		return nil
	}
	t := time.Unix(op.ExecutedAtUnix, 0)
	return &t
}

// CancelledTime returns the cancelled time as time.Time (nil-safe)
func (op *QueuedOperation) CancelledTime() *time.Time {
	if op.CancelledAtUnix == 0 {
		return nil
	}
	t := time.Unix(op.CancelledAtUnix, 0)
	return &t
}

// IsQueued returns true if operation is in QUEUED status
func (op *QueuedOperation) IsQueued() bool {
	return op.Status == OperationStatusQueued
}

// IsExecutable returns true if the operation can be executed now
func (op *QueuedOperation) IsExecutable(now time.Time) bool {
	nowUnix := now.Unix()
	return op.Status == OperationStatusQueued &&
		nowUnix >= op.ExecutableAtUnix &&
		nowUnix < op.ExpiresAtUnix
}

// IsExpired returns true if the operation has expired
func (op *QueuedOperation) IsExpired(now time.Time) bool {
	return now.Unix() >= op.ExpiresAtUnix
}

// CanEmergencyExecute returns true if operation is eligible for emergency execution
func (op *QueuedOperation) CanEmergencyExecute(now time.Time, emergencyDelaySeconds uint64) bool {
	emergencyTimeUnix := op.QueuedAtUnix + int64(emergencyDelaySeconds)
	nowUnix := now.Unix()
	return op.Status == OperationStatusQueued &&
		nowUnix >= emergencyTimeUnix &&
		nowUnix < op.ExpiresAtUnix
}

// TimeUntilExecutable returns the duration until the operation becomes executable
func (op *QueuedOperation) TimeUntilExecutable(now time.Time) time.Duration {
	executableTime := time.Unix(op.ExecutableAtUnix, 0)
	if now.After(executableTime) {
		return 0
	}
	return executableTime.Sub(now)
}

// TimeUntilExpiry returns the duration until the operation expires
func (op *QueuedOperation) TimeUntilExpiry(now time.Time) time.Duration {
	expiresTime := time.Unix(op.ExpiresAtUnix, 0)
	if now.After(expiresTime) {
		return 0
	}
	return expiresTime.Sub(now)
}

// MarkExecuted marks the operation as executed
func (op *QueuedOperation) MarkExecuted(executedAt time.Time) {
	op.Status = OperationStatusExecuted
	op.ExecutedAtUnix = executedAt.Unix()
}

// MarkCancelled marks the operation as cancelled
func (op *QueuedOperation) MarkCancelled(cancelledAt time.Time, reason string) {
	op.Status = OperationStatusCancelled
	op.CancelledAtUnix = cancelledAt.Unix()
	op.CancelReason = reason
}

// MarkExpired marks the operation as expired
func (op *QueuedOperation) MarkExpired() {
	op.Status = OperationStatusExpired
}

// MarkFailed marks the operation as failed with error
func (op *QueuedOperation) MarkFailed(executedAt time.Time, err error) {
	op.Status = OperationStatusFailed
	op.ExecutedAtUnix = executedAt.Unix()
	if err != nil {
		op.ExecutionError = err.Error()
	}
}

// Validate validates the operation
func (op *QueuedOperation) Validate() error {
	if op.Id == 0 {
		return ErrInvalidOperationHash
	}

	if len(op.Messages) == 0 {
		return ErrNoMessages
	}

	if op.Executor == "" {
		return ErrInvalidExecutor
	}

	if !op.VerifyHash() {
		return ErrOperationHashMismatch
	}

	if op.ExecutableAtUnix < op.QueuedAtUnix {
		return ErrInvalidDelay
	}

	if op.ExpiresAtUnix < op.ExecutableAtUnix {
		return ErrGracePeriodInvalid
	}

	return nil
}

// UnpackInterfaces implements UnpackInterfacesMessage
func (op *QueuedOperation) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	for _, anyMsg := range op.Messages {
		// Convert gogoproto any.Any to codectypes.Any for unpacking
		codecAny := &codectypes.Any{
			TypeUrl: anyMsg.TypeUrl,
			Value:   anyMsg.Value,
		}
		var msg sdk.Msg
		if err := unpacker.UnpackAny(codecAny, &msg); err != nil {
			return err
		}
	}
	return nil
}
