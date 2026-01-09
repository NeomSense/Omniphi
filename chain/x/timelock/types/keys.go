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
