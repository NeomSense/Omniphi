package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "repgov"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// KVStore key prefixes
var (
	// KeyParams stores the module parameters
	KeyParams = []byte{0x01}

	// KeyPrefixVoterWeight stores the computed governance weight per address
	// address -> VoterWeight (JSON)
	KeyPrefixVoterWeight = []byte{0x02}

	// KeyPrefixDelegatedReputation stores reputation delegation records
	// delegator_address | delegatee_address -> DelegatedReputation (JSON)
	KeyPrefixDelegatedReputation = []byte{0x03}

	// KeyPrefixEpochSnapshot stores per-epoch reputation snapshots for audit
	// epoch | address -> ReputationSnapshot (JSON)
	KeyPrefixEpochSnapshot = []byte{0x04}

	// KeyPrefixTallyOverride stores tally results that have been reputation-weighted
	// proposal_id -> TallyOverride (JSON)
	KeyPrefixTallyOverride = []byte{0x05}

	// KeyLastComputedEpoch tracks when voter weights were last recomputed
	KeyLastComputedEpoch = []byte{0x06}

	// KeyPrefixSybilScore stores sybil resistance scores per address
	// address -> SybilScore (JSON)
	KeyPrefixSybilScore = []byte{0x07}

	// KeyPrefixDelegatedReputationReverse is a reverse index: delegatee → delegators.
	// Enables O(k) lookup of all delegators TO a specific delegatee (k = delegators)
	// instead of O(N) full scan of all voters.
	// Key: 0x08 | delegatee_address | "/" | delegator_address → []byte{1}
	KeyPrefixDelegatedReputationReverse = []byte{0x08}
)

// GetVoterWeightKey returns the store key for an address's governance weight
func GetVoterWeightKey(addr string) []byte {
	return append(KeyPrefixVoterWeight, []byte(addr)...)
}

// GetDelegatedReputationKey returns the key for a delegation record
func GetDelegatedReputationKey(delegator, delegatee string) []byte {
	key := append(KeyPrefixDelegatedReputation, []byte(delegator)...)
	key = append(key, byte('/'))
	return append(key, []byte(delegatee)...)
}

// GetDelegatedReputationPrefixKey returns the prefix key for all delegations from a delegator
func GetDelegatedReputationPrefixKey(delegator string) []byte {
	key := append(KeyPrefixDelegatedReputation, []byte(delegator)...)
	return append(key, byte('/'))
}

// GetDelegatedReputationReverseKey returns the reverse index key: delegatee → delegator
func GetDelegatedReputationReverseKey(delegatee, delegator string) []byte {
	key := append(KeyPrefixDelegatedReputationReverse, []byte(delegatee)...)
	key = append(key, byte('/'))
	return append(key, []byte(delegator)...)
}

// GetDelegatedReputationReversePrefixKey returns the prefix for all delegators TO a delegatee
func GetDelegatedReputationReversePrefixKey(delegatee string) []byte {
	key := append(KeyPrefixDelegatedReputationReverse, []byte(delegatee)...)
	return append(key, byte('/'))
}

// GetEpochSnapshotKey returns the key for a reputation snapshot at an epoch
func GetEpochSnapshotKey(epoch int64, addr string) []byte {
	key := append(KeyPrefixEpochSnapshot, sdk.Uint64ToBigEndian(uint64(epoch))...)
	return append(key, []byte(addr)...)
}

// GetEpochSnapshotPrefixKey returns the prefix key for all snapshots at an epoch
func GetEpochSnapshotPrefixKey(epoch int64) []byte {
	return append(KeyPrefixEpochSnapshot, sdk.Uint64ToBigEndian(uint64(epoch))...)
}

// GetTallyOverrideKey returns the key for a proposal's tally override
func GetTallyOverrideKey(proposalID uint64) []byte {
	return append(KeyPrefixTallyOverride, sdk.Uint64ToBigEndian(proposalID)...)
}

// GetSybilScoreKey returns the key for an address's sybil score
func GetSybilScoreKey(addr string) []byte {
	return append(KeyPrefixSybilScore, []byte(addr)...)
}
