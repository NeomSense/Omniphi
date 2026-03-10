package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "rewardmult"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// KVStore key prefixes
var (
	// KeyPrefixValidatorMultiplier stores the current multiplier state per validator
	KeyPrefixValidatorMultiplier = []byte{0x01}

	// KeyPrefixEMAHistory stores the EMA ring buffer per validator
	KeyPrefixEMAHistory = []byte{0x02}

	// KeyParams stores the module parameters
	KeyParams = []byte{0x03}

	// KeyPrefixSlashEvent stores recent slash events per validator for penalty decay
	KeyPrefixSlashEvent = []byte{0x04}

	// KeyPrefixFraudEvent stores recent fraud events per validator (future PoR integration)
	KeyPrefixFraudEvent = []byte{0x05}

	// KeyLastProcessedEpoch stores the last epoch that was processed
	KeyLastProcessedEpoch = []byte{0x06}

	// V2.2 Store Keys

	// KeyPrefixEpochStakeSnapshot stores per-validator stake snapshots at epoch boundaries.
	// Used to ensure normalization and distribution use identical stake weights.
	KeyPrefixEpochStakeSnapshot = []byte{0x07}
)

// GetValidatorMultiplierKey returns the store key for a validator's multiplier
func GetValidatorMultiplierKey(valAddr string) []byte {
	return append(KeyPrefixValidatorMultiplier, []byte(valAddr)...)
}

// GetEMAHistoryKey returns the store key for a validator's EMA history
func GetEMAHistoryKey(valAddr string) []byte {
	return append(KeyPrefixEMAHistory, []byte(valAddr)...)
}

// GetSlashEventKey returns the store key for a validator's slash event at a given epoch
func GetSlashEventKey(valAddr string, epoch int64) []byte {
	key := append(KeyPrefixSlashEvent, []byte(valAddr)...)
	return append(key, sdk.Uint64ToBigEndian(uint64(epoch))...)
}

// GetSlashEventPrefixKey returns the prefix key for all slash events of a validator
func GetSlashEventPrefixKey(valAddr string) []byte {
	return append(KeyPrefixSlashEvent, []byte(valAddr)...)
}

// GetFraudEventKey returns the store key for a validator's fraud event at a given epoch
func GetFraudEventKey(valAddr string, epoch int64) []byte {
	key := append(KeyPrefixFraudEvent, []byte(valAddr)...)
	return append(key, sdk.Uint64ToBigEndian(uint64(epoch))...)
}

// GetFraudEventPrefixKey returns the prefix key for all fraud events of a validator
func GetFraudEventPrefixKey(valAddr string) []byte {
	return append(KeyPrefixFraudEvent, []byte(valAddr)...)
}

// GetEpochStakeSnapshotKey returns the store key for a validator's stake snapshot at an epoch
func GetEpochStakeSnapshotKey(epoch int64, valAddr string) []byte {
	key := append(KeyPrefixEpochStakeSnapshot, sdk.Uint64ToBigEndian(uint64(epoch))...)
	return append(key, []byte(valAddr)...)
}

// GetEpochStakeSnapshotPrefixKey returns the prefix key for all stake snapshots at an epoch
func GetEpochStakeSnapshotPrefixKey(epoch int64) []byte {
	return append(KeyPrefixEpochStakeSnapshot, sdk.Uint64ToBigEndian(uint64(epoch))...)
}
