package types

import (
	"encoding/binary"
	"time"
)

const (
	// ModuleName defines the module name
	ModuleName = "timelock"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName
)

// Store key prefixes
var (
	// ParamsKey is the key for module parameters
	ParamsKey = []byte{0x01}

	// OperationKeyPrefix is the prefix for operation storage
	OperationKeyPrefix = []byte{0x02}

	// OperationByHashKeyPrefix is the prefix for operation lookup by hash
	OperationByHashKeyPrefix = []byte{0x03}

	// OperationByProposalKeyPrefix is the prefix for operation lookup by proposal
	OperationByProposalKeyPrefix = []byte{0x04}

	// OperationByStatusKeyPrefix is the prefix for operation lookup by status
	OperationByStatusKeyPrefix = []byte{0x05}

	// OperationByExecutableTimeKeyPrefix is for time-based lookups
	OperationByExecutableTimeKeyPrefix = []byte{0x06}

	// NextOperationIDKey is the key for the next operation ID counter
	NextOperationIDKey = []byte{0x07}

	// KeyGuardianCancelCount tracks the number of cancellations by the guardian in the current window
	KeyGuardianCancelCount = []byte{0x10}

	// KeyGuardianCancelWindowStart tracks the block height when the guardian cancel window started
	KeyGuardianCancelWindowStart = []byte{0x11}

	// --- AST v2: Track system ---

	// TrackKeyPrefix is the prefix for per-track configuration (Track structs).
	// Key: TrackKeyPrefix | track_name_bytes
	TrackKeyPrefix = []byte{0x20}

	// OperationTrackKeyPrefix maps operation ID → track name + computed delay.
	// Key: OperationTrackKeyPrefix | BigEndian(operationID)
	OperationTrackKeyPrefix = []byte{0x21}

	// TreasuryWindowKey stores the rolling 24-hour treasury outflow window.
	// Single entry (one rolling window at a time).
	TreasuryWindowKey = []byte{0x22}

	// ParamChangeFreqKeyPrefix counts governance parameter mutation events.
	// Key: ParamChangeFreqKeyPrefix | BigEndian(windowStartBlock)
	ParamChangeFreqKeyPrefix = []byte{0x23}
)

// GetOperationKey returns the store key for an operation
func GetOperationKey(operationID uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, operationID)
	return append(OperationKeyPrefix, bz...)
}

// GetOperationByHashKey returns the store key for operation lookup by hash
func GetOperationByHashKey(hash []byte) []byte {
	return append(OperationByHashKeyPrefix, hash...)
}

// GetOperationByProposalKey returns the store key for operation lookup by proposal
func GetOperationByProposalKey(proposalID uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, proposalID)
	return append(OperationByProposalKeyPrefix, bz...)
}

// GetOperationByStatusKey returns the store key prefix for status lookup
func GetOperationByStatusKey(status OperationStatus, operationID uint64) []byte {
	statusBz := []byte{byte(status)}
	idBz := make([]byte, 8)
	binary.BigEndian.PutUint64(idBz, operationID)
	return append(append(OperationByStatusKeyPrefix, statusBz...), idBz...)
}

// GetOperationByStatusPrefix returns the prefix for iterating operations by status
func GetOperationByStatusPrefix(status OperationStatus) []byte {
	return append(OperationByStatusKeyPrefix, byte(status))
}

// GetOperationByExecutableTimeKey returns the key for time-based lookup
func GetOperationByExecutableTimeKey(executableAt time.Time, operationID uint64) []byte {
	timeBz := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBz, uint64(executableAt.Unix()))
	idBz := make([]byte, 8)
	binary.BigEndian.PutUint64(idBz, operationID)
	return append(append(OperationByExecutableTimeKeyPrefix, timeBz...), idBz...)
}

// GetOperationByExecutableTimePrefix returns the prefix for time-based iteration
func GetOperationByExecutableTimePrefix(beforeTime time.Time) []byte {
	timeBz := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBz, uint64(beforeTime.Unix()))
	return append(OperationByExecutableTimeKeyPrefix, timeBz...)
}

// GetTrackKey returns the store key for a track by name.
func GetTrackKey(name string) []byte {
	return append(TrackKeyPrefix, []byte(name)...)
}

// GetOperationTrackKey returns the store key mapping an operation to its track record.
func GetOperationTrackKey(operationID uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, operationID)
	return append(OperationTrackKeyPrefix, bz...)
}
