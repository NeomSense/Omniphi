package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "poc"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// TStoreKey defines the transient store key for per-block state
	TStoreKey = "transient_poc"

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_poc"

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// KVStore key prefixes
var (
	// KeyPrefixContribution is the prefix for contribution storage
	KeyPrefixContribution = []byte{0x01}

	// KeyPrefixCredits is the prefix for credits storage
	KeyPrefixCredits = []byte{0x02}

	// KeyNextContributionID is the key for the next contribution ID counter
	KeyNextContributionID = []byte{0x03}

	// KeyPrefixSubmissionCount is the prefix for tracking submissions per block
	KeyPrefixSubmissionCount = []byte{0x04}

	// KeyPrefixContributorIndex is the prefix for contributor index
	KeyPrefixContributorIndex = []byte{0x05}

	// KeyFeeMetrics is the key for global fee metrics
	KeyFeeMetrics = []byte{0x06}

	// KeyPrefixContributorFeeStats is the prefix for per-contributor fee statistics
	KeyPrefixContributorFeeStats = []byte{0x07}
)

// GetContributionKey returns the store key for a contribution by ID
func GetContributionKey(id uint64) []byte {
	return append(KeyPrefixContribution, sdk.Uint64ToBigEndian(id)...)
}

// GetCreditsKey returns the store key for credits by address
func GetCreditsKey(addr string) []byte {
	return append(KeyPrefixCredits, []byte(addr)...)
}

// GetSubmissionCountKey returns the store key for submission count per block
func GetSubmissionCountKey(blockHeight int64) []byte {
	return append(KeyPrefixSubmissionCount, sdk.Uint64ToBigEndian(uint64(blockHeight))...)
}

// GetContributorIndexKey returns the store key for indexing contributions by contributor
func GetContributorIndexKey(contributor string, id uint64) []byte {
	key := append(KeyPrefixContributorIndex, []byte(contributor)...)
	return append(key, sdk.Uint64ToBigEndian(id)...)
}

// GetContributorFeeStatsKey returns the store key for contributor fee statistics
func GetContributorFeeStatsKey(addr sdk.AccAddress) []byte {
	return append(KeyPrefixContributorFeeStats, addr.Bytes()...)
}
