package types

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

// QueuedOperation represents an operation waiting for execution
type QueuedOperation struct {
	ID             uint64
	ProposalID     uint64
	Messages       []*codectypes.Any
	OperationHash  []byte
	QueuedAt       time.Time
	ExecutableAt   time.Time
	ExpiresAt      time.Time
	Status         OperationStatus
	Executor       string
	ExecutedAt     *time.Time
	CancelledAt    *time.Time
	CancelReason   string
	ExecutionError string
}

// NewQueuedOperation creates a new queued operation
func NewQueuedOperation(
	id uint64,
	proposalID uint64,
	messages []sdk.Msg,
	executor string,
	queuedAt time.Time,
	minDelay time.Duration,
	gracePeriod time.Duration,
	cdc codec.Codec,
) (*QueuedOperation, error) {
	// Pack messages into Any
	anyMsgs := make([]*codectypes.Any, len(messages))
	for i, msg := range messages {
		anyMsg, err := codectypes.NewAnyWithValue(msg)
		if err != nil {
			return nil, err
		}
		anyMsgs[i] = anyMsg
	}

	executableAt := queuedAt.Add(minDelay)
	expiresAt := executableAt.Add(gracePeriod)

	op := &QueuedOperation{
		ID:           id,
		ProposalID:   proposalID,
		Messages:     anyMsgs,
		QueuedAt:     queuedAt,
		ExecutableAt: executableAt,
		ExpiresAt:    expiresAt,
		Status:       OperationStatusQueued,
		Executor:     executor,
	}

	// Compute operation hash
	op.OperationHash = op.ComputeHash()

	return op, nil
}

// ComputeHash computes the cryptographic hash of the operation
// Hash includes: proposalID + all message type URLs and content + queued time
func (op *QueuedOperation) ComputeHash() []byte {
	h := sha256.New()

	// Include proposal ID
	proposalIDBz := make([]byte, 8)
	binary.BigEndian.PutUint64(proposalIDBz, op.ProposalID)
	h.Write(proposalIDBz)

	// Include operation ID (prevents replay)
	opIDBz := make([]byte, 8)
	binary.BigEndian.PutUint64(opIDBz, op.ID)
	h.Write(opIDBz)

	// Include each message
	for _, anyMsg := range op.Messages {
		h.Write([]byte(anyMsg.TypeUrl))
		h.Write(anyMsg.Value)
	}

	// Include queued timestamp (prevents replay with same messages)
	timeBz := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBz, uint64(op.QueuedAt.Unix()))
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

// GetMessages unpacks and returns the SDK messages
func (op *QueuedOperation) GetMessages(cdc codec.Codec) ([]sdk.Msg, error) {
	msgs := make([]sdk.Msg, len(op.Messages))
	for i, anyMsg := range op.Messages {
		var msg sdk.Msg
		if err := cdc.UnpackAny(anyMsg, &msg); err != nil {
			return nil, err
		}
		msgs[i] = msg
	}
	return msgs, nil
}

// IsQueued returns true if operation is in QUEUED status
func (op *QueuedOperation) IsQueued() bool {
	return op.Status == OperationStatusQueued
}

// IsExecutable returns true if the operation can be executed now
func (op *QueuedOperation) IsExecutable(now time.Time) bool {
	return op.Status == OperationStatusQueued &&
		now.After(op.ExecutableAt) &&
		now.Before(op.ExpiresAt)
}

// IsExpired returns true if the operation has expired
func (op *QueuedOperation) IsExpired(now time.Time) bool {
	return now.After(op.ExpiresAt)
}

// CanEmergencyExecute returns true if operation is eligible for emergency execution
func (op *QueuedOperation) CanEmergencyExecute(now time.Time, emergencyDelay time.Duration) bool {
	emergencyTime := op.QueuedAt.Add(emergencyDelay)
	return op.Status == OperationStatusQueued &&
		now.After(emergencyTime) &&
		now.Before(op.ExpiresAt)
}

// TimeUntilExecutable returns the duration until the operation becomes executable
func (op *QueuedOperation) TimeUntilExecutable(now time.Time) time.Duration {
	if now.After(op.ExecutableAt) {
		return 0
	}
	return op.ExecutableAt.Sub(now)
}

// TimeUntilExpiry returns the duration until the operation expires
func (op *QueuedOperation) TimeUntilExpiry(now time.Time) time.Duration {
	if now.After(op.ExpiresAt) {
		return 0
	}
	return op.ExpiresAt.Sub(now)
}

// MarkExecuted marks the operation as executed
func (op *QueuedOperation) MarkExecuted(executedAt time.Time) {
	op.Status = OperationStatusExecuted
	op.ExecutedAt = &executedAt
}

// MarkCancelled marks the operation as cancelled
func (op *QueuedOperation) MarkCancelled(cancelledAt time.Time, reason string) {
	op.Status = OperationStatusCancelled
	op.CancelledAt = &cancelledAt
	op.CancelReason = reason
}

// MarkExpired marks the operation as expired
func (op *QueuedOperation) MarkExpired() {
	op.Status = OperationStatusExpired
}

// MarkFailed marks the operation as failed with error
func (op *QueuedOperation) MarkFailed(executedAt time.Time, err error) {
	op.Status = OperationStatusFailed
	op.ExecutedAt = &executedAt
	if err != nil {
		op.ExecutionError = err.Error()
	}
}

// Validate validates the operation
func (op *QueuedOperation) Validate() error {
	if op.ID == 0 {
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

	if op.ExecutableAt.Before(op.QueuedAt) {
		return ErrInvalidDelay
	}

	if op.ExpiresAt.Before(op.ExecutableAt) {
		return ErrGracePeriodInvalid
	}

	return nil
}

// UnpackInterfaces implements UnpackInterfacesMessage
func (op *QueuedOperation) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	for _, anyMsg := range op.Messages {
		var msg sdk.Msg
		if err := unpacker.UnpackAny(anyMsg, &msg); err != nil {
			return err
		}
	}
	return nil
}

// Marshal marshals the operation to bytes
func (op *QueuedOperation) Marshal() ([]byte, error) {
	return proto.Marshal(op)
}
