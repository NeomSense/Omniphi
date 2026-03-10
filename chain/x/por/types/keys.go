package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "por"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// TStoreKey defines the transient store key (per-block state)
	TStoreKey = "transient_por"

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_por"

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// Store key prefixes for all module state
var (
	// 0x01 - Registered applications
	KeyPrefixApp = []byte{0x01}

	// 0x02 - Verifier sets
	KeyPrefixVerifierSet = []byte{0x02}

	// 0x03 - Batch commitments
	KeyPrefixBatch = []byte{0x03}

	// 0x04 - Attestations (composite key: batch_id + verifier_addr)
	KeyPrefixAttestation = []byte{0x04}

	// 0x05 - Challenges
	KeyPrefixChallenge = []byte{0x05}

	// 0x06 - Verifier reputation scores
	KeyPrefixVerifierReputation = []byte{0x06}

	// 0x07 - Auto-increment ID counters
	KeyNextAppID         = []byte{0x07, 0x01}
	KeyNextBatchID       = []byte{0x07, 0x02}
	KeyNextChallengeID   = []byte{0x07, 0x03}
	KeyNextVerifierSetID = []byte{0x07, 0x04}

	// 0x08 - Parameters (using string key for consistency with poc module)
	KeyParams = []byte("params")

	// 0x09 - Index: batches by epoch (for BatchesByEpoch query)
	KeyPrefixBatchByEpoch = []byte{0x09}

	// 0x0A - Index: batches by app_id (for filtering)
	KeyPrefixBatchByApp = []byte{0x0A}

	// 0x0B - Index: batches by status (critical for EndBlocker iteration)
	KeyPrefixBatchByStatus = []byte{0x0B}

	// 0x0C - Index: challenges by batch_id
	KeyPrefixChallengeByBatch = []byte{0x0C}

	// 0x0D - Transient store: per-block batch submission counter
	KeyPrefixBlockSubmissionCount = []byte{0x0D}

	// 0x0E - Slash records (audit trail)
	KeyPrefixSlashRecord = []byte{0x0E}

	// 0x0F - Per-epoch cumulative credit tracker (F2/F6)
	KeyPrefixEpochCreditTracker = []byte{0x0F}

	// 0x10 - Per-batch record leaf hashes (F3)
	KeyPrefixBatchLeafHash = []byte{0x10}

	// 0x11 - Reverse index: leaf_hash -> batch_ids (F3)
	KeyPrefixLeafHashToBatch = []byte{0x11}

	// 0x12 - Challenge rate limiter per address/epoch (F4)
	KeyPrefixChallengeRateLimit = []byte{0x12}

	// 0x13 - PoSeq registered commitments (F8)
	KeyPrefixPoSeqCommitment = []byte{0x13}

	// 0x14 - PoSeq sequencer set (F8)
	KeyPoSeqSequencerSet = []byte{0x14}
)

// GetAppKey returns the store key for an app by ID
func GetAppKey(appID uint64) []byte {
	return append(KeyPrefixApp, sdk.Uint64ToBigEndian(appID)...)
}

// GetVerifierSetKey returns the store key for a verifier set by ID
func GetVerifierSetKey(verifierSetID uint64) []byte {
	return append(KeyPrefixVerifierSet, sdk.Uint64ToBigEndian(verifierSetID)...)
}

// GetBatchKey returns the store key for a batch by ID
func GetBatchKey(batchID uint64) []byte {
	return append(KeyPrefixBatch, sdk.Uint64ToBigEndian(batchID)...)
}

// GetAttestationKey returns the store key for an attestation (composite: batch_id + verifier address)
func GetAttestationKey(batchID uint64, verifierAddr string) []byte {
	key := append(KeyPrefixAttestation, sdk.Uint64ToBigEndian(batchID)...)
	return append(key, []byte(verifierAddr)...)
}

// GetAttestationsByBatchPrefix returns the prefix for iterating all attestations for a batch
func GetAttestationsByBatchPrefix(batchID uint64) []byte {
	return append(KeyPrefixAttestation, sdk.Uint64ToBigEndian(batchID)...)
}

// GetChallengeKey returns the store key for a challenge by ID
func GetChallengeKey(challengeID uint64) []byte {
	return append(KeyPrefixChallenge, sdk.Uint64ToBigEndian(challengeID)...)
}

// GetVerifierReputationKey returns the store key for verifier reputation
func GetVerifierReputationKey(addr string) []byte {
	return append(KeyPrefixVerifierReputation, []byte(addr)...)
}

// GetBatchByEpochKey returns the index key for batch-by-epoch lookups
func GetBatchByEpochKey(epoch uint64, batchID uint64) []byte {
	key := append(KeyPrefixBatchByEpoch, sdk.Uint64ToBigEndian(epoch)...)
	return append(key, sdk.Uint64ToBigEndian(batchID)...)
}

// GetBatchByEpochPrefix returns the prefix for iterating all batches in an epoch
func GetBatchByEpochPrefix(epoch uint64) []byte {
	return append(KeyPrefixBatchByEpoch, sdk.Uint64ToBigEndian(epoch)...)
}

// GetBatchByAppKey returns the index key for batch-by-app lookups
func GetBatchByAppKey(appID uint64, batchID uint64) []byte {
	key := append(KeyPrefixBatchByApp, sdk.Uint64ToBigEndian(appID)...)
	return append(key, sdk.Uint64ToBigEndian(batchID)...)
}

// GetBatchByStatusKey returns the index key for batch-by-status lookups
func GetBatchByStatusKey(status BatchStatus, batchID uint64) []byte {
	key := append(KeyPrefixBatchByStatus, byte(status))
	return append(key, sdk.Uint64ToBigEndian(batchID)...)
}

// GetBatchByStatusPrefix returns the prefix for iterating all batches with a given status
func GetBatchByStatusPrefix(status BatchStatus) []byte {
	return append(KeyPrefixBatchByStatus, byte(status))
}

// GetChallengeByBatchKey returns the index key for challenge-by-batch lookups
func GetChallengeByBatchKey(batchID uint64, challengeID uint64) []byte {
	key := append(KeyPrefixChallengeByBatch, sdk.Uint64ToBigEndian(batchID)...)
	return append(key, sdk.Uint64ToBigEndian(challengeID)...)
}

// GetChallengeByBatchPrefix returns the prefix for iterating all challenges for a batch
func GetChallengeByBatchPrefix(batchID uint64) []byte {
	return append(KeyPrefixChallengeByBatch, sdk.Uint64ToBigEndian(batchID)...)
}

// GetSlashRecordKey returns the store key for a slash record
func GetSlashRecordKey(batchID uint64, addr string) []byte {
	key := append(KeyPrefixSlashRecord, sdk.Uint64ToBigEndian(batchID)...)
	return append(key, []byte(addr)...)
}

// GetEpochCreditTrackerKey returns the key for tracking cumulative credits per epoch (F2/F6)
func GetEpochCreditTrackerKey(epoch uint64) []byte {
	return append(KeyPrefixEpochCreditTracker, sdk.Uint64ToBigEndian(epoch)...)
}

// GetBatchLeafHashKey returns the key for a specific leaf hash in a batch (F3)
func GetBatchLeafHashKey(batchID uint64, leafIndex uint64) []byte {
	key := append(KeyPrefixBatchLeafHash, sdk.Uint64ToBigEndian(batchID)...)
	return append(key, sdk.Uint64ToBigEndian(leafIndex)...)
}

// GetBatchLeafHashPrefix returns the prefix for iterating all leaf hashes in a batch (F3)
func GetBatchLeafHashPrefix(batchID uint64) []byte {
	return append(KeyPrefixBatchLeafHash, sdk.Uint64ToBigEndian(batchID)...)
}

// GetLeafHashToBatchKey returns the reverse index key: leaf_hash + batch_id (F3)
func GetLeafHashToBatchKey(leafHash []byte, batchID uint64) []byte {
	key := append(KeyPrefixLeafHashToBatch, leafHash...)
	return append(key, sdk.Uint64ToBigEndian(batchID)...)
}

// GetLeafHashToBatchPrefix returns the prefix for finding all batches containing a leaf hash (F3)
func GetLeafHashToBatchPrefix(leafHash []byte) []byte {
	return append(KeyPrefixLeafHashToBatch, leafHash...)
}

// GetChallengeRateLimitKey returns the key for challenge rate limiting per address/epoch (F4)
func GetChallengeRateLimitKey(addr string, epoch uint64) []byte {
	key := append(KeyPrefixChallengeRateLimit, []byte(addr)...)
	return append(key, sdk.Uint64ToBigEndian(epoch)...)
}

// GetPoSeqCommitmentKey returns the key for a registered PoSeq commitment (F8)
func GetPoSeqCommitmentKey(commitmentHash []byte) []byte {
	return append(KeyPrefixPoSeqCommitment, commitmentHash...)
}
