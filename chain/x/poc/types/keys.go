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

	// ============================================================================
	// PoC Hardening Upgrade Keys (v2)
	// ============================================================================

	// KeyPrefixReputationScore stores slow-moving EMA reputation scores
	// Used for governance boost calculations (separate from RewardScore)
	KeyPrefixReputationScore = []byte{0x08}

	// KeyPrefixEpochCredits tracks credits earned per epoch per address
	// Used for per-epoch caps and diminishing returns
	KeyPrefixEpochCredits = []byte{0x09}

	// KeyPrefixTypeCredits tracks credits earned per contribution type per address
	// Used for per-type caps to prevent single-vector gaming
	KeyPrefixTypeCredits = []byte{0x0A}

	// KeyPrefixValidatorEndorsementStats tracks endorsement participation per validator
	// Used for endorsement quality tracking and PoS multiplier metrics
	KeyPrefixValidatorEndorsementStats = []byte{0x0B}

	// KeyPrefixFrozenCredits tracks credits frozen due to pending challenges
	// Used for fraud/rollback safety when PoR is integrated
	KeyPrefixFrozenCredits = []byte{0x0C}

	// KeyPrefixContributionFinality tracks finality status of contributions
	// Used to gate rewards until finality is confirmed
	KeyPrefixContributionFinality = []byte{0x0D}

	// KeyLastDecayEpoch tracks the last epoch when decay was applied
	KeyLastDecayEpoch = []byte{0x0E}

	// KeyCurrentEpoch tracks the current epoch number
	KeyCurrentEpoch = []byte{0x0F}

	// KeyPrefixClaimNonce tracks withdrawal nonces per address for replay protection
	KeyPrefixClaimNonce = []byte{0x10}

	// ============================================================================
	// V2.1 Mainnet Hardening Keys
	// ============================================================================

	// KeyPrefixChallengeWindow tracks per-contribution challenge window end height
	KeyPrefixChallengeWindow = []byte{0x11}

	// KeyPrefixFraudProof stores validated fraud proofs by contribution ID
	KeyPrefixFraudProof = []byte{0x12}

	// KeyPrefixEndorsementPenalty tracks soft penalty state per validator
	KeyPrefixEndorsementPenalty = []byte{0x13}

	// KeyPrefixParamChangeHistory tracks recent param changes for rate limiting
	KeyPrefixParamChangeHistory = []byte{0x14}

	// KeyPoCPayoutsPaused stores emergency pause flag for PoC payouts
	KeyPoCPayoutsPaused = []byte{0x15}

	// KeyPrefixLazyDecayMarker marks the last decay-applied epoch per address
	KeyPrefixLazyDecayMarker = []byte{0x16}

	// ============================================================================
	// Canonical Hash Layer Keys (Layer 1 Deduplication)
	// ============================================================================

	// KeyPrefixCanonicalRegistry maps canonical_hash → CanonicalRegistry (JSON).
	// Key: 0x17 | canonical_hash[32]
	KeyPrefixCanonicalRegistry = []byte{0x17}

	// KeyPrefixBondEscrow tracks escrowed duplicate bonds per contribution.
	// Key: 0x18 | contributor_addr | contribution_id (big endian uint64)
	KeyPrefixBondEscrow = []byte{0x18}

	// KeyPrefixDuplicateSubmission stores DuplicateRecord for flagged duplicates.
	// Key: 0x19 | contribution_id (big endian uint64)
	KeyPrefixDuplicateSubmission = []byte{0x19}

	// KeyPrefixEpochSubmissionRate tracks duplicate submissions per (address, epoch).
	// Key: 0x1A | contributor_addr | epoch (big endian uint64)
	KeyPrefixEpochSubmissionRate = []byte{0x1A}

	// ============================================================================
	// Similarity Engine Keys (Layer 2)
	// ============================================================================

	// KeyPrefixSimilarityCommitment stores SimilarityCommitmentRecord per contribution.
	// Key: 0x1B | contribution_id (big endian uint64)
	KeyPrefixSimilarityCommitment = []byte{0x1B}

	// ============================================================================
	// Human Review Layer Keys (Layer 3: PoV Override)
	// ============================================================================

	// KeyPrefixReviewSession stores ReviewSession per contribution.
	// Key: 0x1C | contribution_id (big endian uint64)
	KeyPrefixReviewSession = []byte{0x1C}

	// KeyPrefixReviewerProfile stores ReviewerProfile per address.
	// Key: 0x1D | reviewer_addr
	KeyPrefixReviewerProfile = []byte{0x1D}

	// KeyPrefixReviewBondEscrow stores review bond escrow per reviewer per contribution.
	// Key: 0x1E | reviewer_addr | contribution_id (big endian uint64)
	KeyPrefixReviewBondEscrow = []byte{0x1E}

	// KeyPrefixReviewAppeal stores ReviewAppeal per appeal ID.
	// Key: 0x1F | appeal_id (big endian uint64)
	KeyPrefixReviewAppeal = []byte{0x1F}

	// KeyNextReviewAppealID stores the next appeal ID counter.
	KeyNextReviewAppealID = []byte{0x20}

	// KeyPrefixCoVotingRecord stores CoVotingRecord per reviewer pair.
	// Key: 0x21 | min(addrA,addrB) | max(addrA,addrB)
	KeyPrefixCoVotingRecord = []byte{0x21}

	// KeyPrefixPendingReviewIndex indexes contributions needing finalization by end height.
	// Key: 0x22 | end_height (big endian uint64) | contribution_id (big endian uint64)
	KeyPrefixPendingReviewIndex = []byte{0x22}

	// ============================================================================
	// Economic Adjustment Keys (Layer 4)
	// ============================================================================

	// KeyPrefixVestingSchedule stores vesting schedules per contributor per claim.
	// Key: 0x23 | contributor_addr | claim_id (big endian uint64)
	KeyPrefixVestingSchedule = []byte{0x23}

	// KeyPrefixVestingEpochIndex indexes vesting schedules by release epoch.
	// Key: 0x24 | epoch (big endian uint64) | contributor_addr
	KeyPrefixVestingEpochIndex = []byte{0x24}

	// KeyPrefixVestingBalance stores aggregate unvested balance per contributor.
	// Key: 0x25 | contributor_addr
	KeyPrefixVestingBalance = []byte{0x25}

	// KeyPrefixClawbackRecord stores clawback records per claim.
	// Key: 0x26 | claim_id (big endian uint64)
	KeyPrefixClawbackRecord = []byte{0x26}

	// ============================================================================
	// Global Provenance Registry Keys (Layer 5)
	// ============================================================================

	// KeyPrefixProvenanceEntry stores ProvenanceEntry per accepted claim.
	// Key: 0x27 | claim_id (big endian uint64)
	KeyPrefixProvenanceEntry = []byte{0x27}

	// KeyPrefixProvenanceChildIndex indexes children by parent claim ID.
	// Key: 0x28 | parent_claim_id (big endian uint64) | child_claim_id (big endian uint64)
	KeyPrefixProvenanceChildIndex = []byte{0x28}

	// KeyPrefixProvenanceHashIndex indexes claims by canonical hash.
	// Key: 0x29 | canonical_hash (32 bytes) | claim_id (big endian uint64)
	KeyPrefixProvenanceHashIndex = []byte{0x29}

	// KeyPrefixProvenanceSubmitterIndex indexes claims by submitter address.
	// Key: 0x2A | submitter_addr | claim_id (big endian uint64)
	KeyPrefixProvenanceSubmitterIndex = []byte{0x2A}

	// KeyPrefixProvenanceCategoryIndex indexes claims by category (Ctype).
	// Key: 0x2B | category | claim_id (big endian uint64)
	KeyPrefixProvenanceCategoryIndex = []byte{0x2B}

	// KeyPrefixProvenanceEpochIndex indexes claims by epoch for time-range queries.
	// Key: 0x2C | epoch (big endian uint64) | claim_id (big endian uint64)
	KeyPrefixProvenanceEpochIndex = []byte{0x2C}

	// KeyPrefixProvenanceStats stores singleton aggregate provenance metrics.
	// Key: 0x2D
	KeyPrefixProvenanceStats = []byte{0x2D}

	// ============================================================================
	// Adaptive Reward Vesting System (ARVS) Keys (Layer 4 extension)
	// ============================================================================

	// KeyPrefixARVSVesting stores ARVSVestingSchedule per contributor per claim.
	// Key: 0x2E | contributor_addr | claim_id (big endian uint64)
	KeyPrefixARVSVesting = []byte{0x2E}

	// KeyMinQualityForEmission stores the minimum quality score (0-100) required
	// for a contribution to be eligible for emission rewards. Singleton.
	KeyMinQualityForEmission = []byte{0x2F}

	// KeyPrefixPendingRewardIndex is a set-index of contribution IDs that are
	// verified and not yet rewarded. Written when quorum is reached, deleted
	// when the reward is distributed. Enables O(pending) EndBlocker iteration
	// instead of O(all contributions).
	// Key: 0x30 | contribution_id (big endian uint64)
	KeyPrefixPendingRewardIndex = []byte{0x30}

	// KeyCtypeWeights stores a JSON map[string]uint32 of contribution-type weights
	// used by weightFor() to scale emission shares by contribution category.
	// Example: {"code":200, "record":100, "relay":80, "green":150}
	// Stored as JSON sidecar to avoid proto field descriptor issues.
	KeyCtypeWeights = []byte{0x31}

	// KeyMaxVestingReleasesPerEpoch stores a uint32 cap on how many vesting schedules
	// ProcessVestingReleases and ProcessARVSVestingReleases may release per epoch.
	// Prevents O(n) EndBlocker stalls when large numbers of schedules mature.
	KeyMaxVestingReleasesPerEpoch = []byte{0x32}

	// ============================================================================
	// Layer 5: Utility & Impact Scoring Keys
	// ============================================================================

	// KeyImpactParams stores the JSON-encoded ImpactParams governance sidecar.
	KeyImpactParams = []byte{0x33}

	// KeyPrefixImpactRecord stores ContributionImpactRecord per accepted claim.
	// Key: 0x34 | claim_id (big endian uint64)
	KeyPrefixImpactRecord = []byte{0x34}

	// KeyPrefixImpactProfile stores ContributorImpactProfile per contributor address.
	// Key: 0x35 | contributor_addr
	KeyPrefixImpactProfile = []byte{0x35}

	// KeyPrefixUsageEdge stores ContributionUsageEdge per (parent, child) pair.
	// Key: 0x36 | parent_claim_id (big endian uint64) | child_claim_id (big endian uint64)
	KeyPrefixUsageEdge = []byte{0x36}

	// KeyPrefixUsageEdgeByParent indexes usage edges by parent claim for fast fan-out queries.
	// Key: 0x37 | parent_claim_id (big endian uint64) | child_claim_id (big endian uint64)
	// Value: empty (presence-only index; actual edge data at KeyPrefixUsageEdge)
	KeyPrefixUsageEdgeByParent = []byte{0x37}

	// KeyPrefixImpactUpdateQueue is a set-index of claim IDs pending impact recalculation.
	// Written when a new usage edge is recorded; consumed by EndBlocker batch pass.
	// Key: 0x38 | claim_id (big endian uint64)
	KeyPrefixImpactUpdateQueue = []byte{0x38}
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

// ============================================================================
// PoC Hardening Upgrade Key Functions (v2)
// ============================================================================

// GetReputationScoreKey returns the store key for reputation score by address
func GetReputationScoreKey(addr string) []byte {
	return append(KeyPrefixReputationScore, []byte(addr)...)
}

// GetEpochCreditsKey returns the store key for epoch credits tracking
func GetEpochCreditsKey(addr string, epoch uint64) []byte {
	key := append(KeyPrefixEpochCredits, []byte(addr)...)
	return append(key, sdk.Uint64ToBigEndian(epoch)...)
}

// GetTypeCreditsKey returns the store key for type credits tracking
func GetTypeCreditsKey(addr string, ctype string) []byte {
	key := append(KeyPrefixTypeCredits, []byte(addr)...)
	return append(key, []byte(ctype)...)
}

// GetValidatorEndorsementStatsKey returns the store key for validator endorsement stats
func GetValidatorEndorsementStatsKey(valAddr string) []byte {
	return append(KeyPrefixValidatorEndorsementStats, []byte(valAddr)...)
}

// GetFrozenCreditsKey returns the store key for frozen credits by address
func GetFrozenCreditsKey(addr string) []byte {
	return append(KeyPrefixFrozenCredits, []byte(addr)...)
}

// GetContributionFinalityKey returns the store key for contribution finality status
func GetContributionFinalityKey(contributionID uint64) []byte {
	return append(KeyPrefixContributionFinality, sdk.Uint64ToBigEndian(contributionID)...)
}

// GetClaimNonceKey returns the store key for claim nonce by address
func GetClaimNonceKey(addr string) []byte {
	return append(KeyPrefixClaimNonce, []byte(addr)...)
}

// ============================================================================
// V2.1 Mainnet Hardening Key Functions
// ============================================================================

// GetChallengeWindowKey returns the store key for a contribution's challenge window
func GetChallengeWindowKey(contributionID uint64) []byte {
	return append(KeyPrefixChallengeWindow, sdk.Uint64ToBigEndian(contributionID)...)
}

// GetFraudProofKey returns the store key for a fraud proof by contribution ID
func GetFraudProofKey(contributionID uint64) []byte {
	return append(KeyPrefixFraudProof, sdk.Uint64ToBigEndian(contributionID)...)
}

// GetEndorsementPenaltyKey returns the store key for a validator's endorsement penalty
func GetEndorsementPenaltyKey(valAddr string) []byte {
	return append(KeyPrefixEndorsementPenalty, []byte(valAddr)...)
}

// GetParamChangeHistoryKey returns the store key for a param change record
func GetParamChangeHistoryKey(blockHeight int64) []byte {
	return append(KeyPrefixParamChangeHistory, sdk.Uint64ToBigEndian(uint64(blockHeight))...)
}

// GetLazyDecayMarkerKey returns the store key for an address's lazy decay marker
func GetLazyDecayMarkerKey(addr string) []byte {
	return append(KeyPrefixLazyDecayMarker, []byte(addr)...)
}

// ============================================================================
// Canonical Hash Layer Key Functions
// ============================================================================

// GetCanonicalRegistryKey returns the store key for a canonical hash registry entry.
// canonicalHash must be exactly 32 bytes (SHA-256).
func GetCanonicalRegistryKey(canonicalHash []byte) []byte {
	return append(KeyPrefixCanonicalRegistry, canonicalHash...)
}

// GetBondEscrowKey returns the store key for a bond escrow entry (per address).
func GetBondEscrowKey(addr string) []byte {
	return append(KeyPrefixBondEscrow, []byte(addr)...)
}

// GetDuplicateSubmissionKey returns the store key for a duplicate submission record.
func GetDuplicateSubmissionKey(contributionID uint64) []byte {
	return append(KeyPrefixDuplicateSubmission, sdk.Uint64ToBigEndian(contributionID)...)
}

// GetEpochSubmissionRateKey returns the store key for duplicate rate tracking.
func GetEpochSubmissionRateKey(addr string, epoch uint64) []byte {
	key := append(KeyPrefixEpochSubmissionRate, []byte(addr)...)
	return append(key, sdk.Uint64ToBigEndian(epoch)...)
}

// ============================================================================
// Similarity Engine Key Functions (Layer 2)
// ============================================================================

// GetSimilarityCommitmentKey returns the store key for a similarity commitment record.
func GetSimilarityCommitmentKey(contributionID uint64) []byte {
	return append(KeyPrefixSimilarityCommitment, sdk.Uint64ToBigEndian(contributionID)...)
}

// ============================================================================
// Human Review Layer Key Functions (Layer 3)
// ============================================================================

// GetReviewSessionKey returns the store key for a review session by contribution ID.
func GetReviewSessionKey(contributionID uint64) []byte {
	return append(KeyPrefixReviewSession, sdk.Uint64ToBigEndian(contributionID)...)
}

// GetReviewerProfileKey returns the store key for a reviewer profile by address.
func GetReviewerProfileKey(addr string) []byte {
	return append(KeyPrefixReviewerProfile, []byte(addr)...)
}

// GetReviewBondEscrowKey returns the store key for a review bond escrow.
func GetReviewBondEscrowKey(addr string, contributionID uint64) []byte {
	key := append(KeyPrefixReviewBondEscrow, []byte(addr)...)
	return append(key, sdk.Uint64ToBigEndian(contributionID)...)
}

// GetReviewAppealKey returns the store key for an appeal by ID.
func GetReviewAppealKey(appealID uint64) []byte {
	return append(KeyPrefixReviewAppeal, sdk.Uint64ToBigEndian(appealID)...)
}

// GetCoVotingRecordKey returns the store key for a co-voting record between two reviewers.
// Addresses are canonically ordered (lexicographically smaller first).
func GetCoVotingRecordKey(addrA, addrB string) []byte {
	if addrA > addrB {
		addrA, addrB = addrB, addrA
	}
	key := append(KeyPrefixCoVotingRecord, []byte(addrA)...)
	return append(key, []byte(addrB)...)
}

// GetPendingReviewIndexKey returns the store key for the pending review expiry index.
func GetPendingReviewIndexKey(endHeight int64, contributionID uint64) []byte {
	key := append(KeyPrefixPendingReviewIndex, sdk.Uint64ToBigEndian(uint64(endHeight))...)
	return append(key, sdk.Uint64ToBigEndian(contributionID)...)
}

// ============================================================================
// Economic Adjustment Key Functions (Layer 4)
// ============================================================================

// GetVestingScheduleKey returns the store key for a vesting schedule.
func GetVestingScheduleKey(contributor string, claimID uint64) []byte {
	key := append(KeyPrefixVestingSchedule, []byte(contributor)...)
	return append(key, sdk.Uint64ToBigEndian(claimID)...)
}

// GetVestingEpochIndexKey returns the store key for a vesting epoch index entry.
func GetVestingEpochIndexKey(epoch uint64, contributor string) []byte {
	key := append(KeyPrefixVestingEpochIndex, sdk.Uint64ToBigEndian(epoch)...)
	return append(key, []byte(contributor)...)
}

// GetVestingBalanceKey returns the store key for a contributor's aggregate vesting balance.
func GetVestingBalanceKey(contributor string) []byte {
	return append(KeyPrefixVestingBalance, []byte(contributor)...)
}

// GetClawbackRecordKey returns the store key for a clawback record.
func GetClawbackRecordKey(claimID uint64) []byte {
	return append(KeyPrefixClawbackRecord, sdk.Uint64ToBigEndian(claimID)...)
}

// GetARVSVestingKey returns the store key for an ARVS vesting schedule.
func GetARVSVestingKey(contributor string, claimID uint64) []byte {
	key := append(KeyPrefixARVSVesting, []byte(contributor)...)
	return append(key, sdk.Uint64ToBigEndian(claimID)...)
}

// ============================================================================
// Global Provenance Registry Key Functions (Layer 5)
// ============================================================================

// GetProvenanceEntryKey returns the store key for a provenance entry by claim ID.
func GetProvenanceEntryKey(claimID uint64) []byte {
	return append(KeyPrefixProvenanceEntry, sdk.Uint64ToBigEndian(claimID)...)
}

// GetProvenanceChildIndexKey returns the store key for a parent→child index entry.
func GetProvenanceChildIndexKey(parentClaimID, childClaimID uint64) []byte {
	key := append(KeyPrefixProvenanceChildIndex, sdk.Uint64ToBigEndian(parentClaimID)...)
	return append(key, sdk.Uint64ToBigEndian(childClaimID)...)
}

// GetProvenanceChildIndexPrefix returns the prefix for iterating all children of a parent.
func GetProvenanceChildIndexPrefix(parentClaimID uint64) []byte {
	return append(KeyPrefixProvenanceChildIndex, sdk.Uint64ToBigEndian(parentClaimID)...)
}

// GetProvenanceHashIndexKey returns the store key for a hash→claim index entry.
func GetProvenanceHashIndexKey(canonicalHash []byte, claimID uint64) []byte {
	key := append(KeyPrefixProvenanceHashIndex, canonicalHash...)
	return append(key, sdk.Uint64ToBigEndian(claimID)...)
}

// GetProvenanceHashIndexPrefix returns the prefix for iterating all claims with a given hash.
func GetProvenanceHashIndexPrefix(canonicalHash []byte) []byte {
	return append(KeyPrefixProvenanceHashIndex, canonicalHash...)
}

// GetProvenanceSubmitterIndexKey returns the store key for a submitter→claim index entry.
func GetProvenanceSubmitterIndexKey(addr string, claimID uint64) []byte {
	key := append(KeyPrefixProvenanceSubmitterIndex, []byte(addr)...)
	return append(key, sdk.Uint64ToBigEndian(claimID)...)
}

// GetProvenanceSubmitterIndexPrefix returns the prefix for iterating all claims by a submitter.
func GetProvenanceSubmitterIndexPrefix(addr string) []byte {
	return append(KeyPrefixProvenanceSubmitterIndex, []byte(addr)...)
}

// GetProvenanceCategoryIndexKey returns the store key for a category→claim index entry.
func GetProvenanceCategoryIndexKey(category string, claimID uint64) []byte {
	key := append(KeyPrefixProvenanceCategoryIndex, []byte(category)...)
	return append(key, sdk.Uint64ToBigEndian(claimID)...)
}

// GetProvenanceCategoryIndexPrefix returns the prefix for iterating all claims in a category.
func GetProvenanceCategoryIndexPrefix(category string) []byte {
	return append(KeyPrefixProvenanceCategoryIndex, []byte(category)...)
}

// GetProvenanceEpochIndexKey returns the store key for an epoch→claim index entry.
func GetProvenanceEpochIndexKey(epoch uint64, claimID uint64) []byte {
	key := append(KeyPrefixProvenanceEpochIndex, sdk.Uint64ToBigEndian(epoch)...)
	return append(key, sdk.Uint64ToBigEndian(claimID)...)
}

// GetProvenanceEpochIndexPrefix returns the prefix for iterating all claims in an epoch.
func GetProvenanceEpochIndexPrefix(epoch uint64) []byte {
	return append(KeyPrefixProvenanceEpochIndex, sdk.Uint64ToBigEndian(epoch)...)
}

// GetPendingRewardIndexKey returns the store key for a pending-reward index entry.
func GetPendingRewardIndexKey(contributionID uint64) []byte {
	return append(KeyPrefixPendingRewardIndex, sdk.Uint64ToBigEndian(contributionID)...)
}

// ============================================================================
// Layer 5: Utility & Impact Scoring Key Functions
// ============================================================================

// GetImpactRecordKey returns the store key for a ContributionImpactRecord by claim ID.
func GetImpactRecordKey(claimID uint64) []byte {
	return append(KeyPrefixImpactRecord, sdk.Uint64ToBigEndian(claimID)...)
}

// GetImpactProfileKey returns the store key for a ContributorImpactProfile by address.
func GetImpactProfileKey(addr string) []byte {
	return append(KeyPrefixImpactProfile, []byte(addr)...)
}

// GetUsageEdgeKey returns the store key for a ContributionUsageEdge by (parent, child).
func GetUsageEdgeKey(parentClaimID, childClaimID uint64) []byte {
	key := append(KeyPrefixUsageEdge, sdk.Uint64ToBigEndian(parentClaimID)...)
	return append(key, sdk.Uint64ToBigEndian(childClaimID)...)
}

// GetUsageEdgeByParentKey returns the parent-index key for a usage edge.
func GetUsageEdgeByParentKey(parentClaimID, childClaimID uint64) []byte {
	key := append(KeyPrefixUsageEdgeByParent, sdk.Uint64ToBigEndian(parentClaimID)...)
	return append(key, sdk.Uint64ToBigEndian(childClaimID)...)
}

// GetUsageEdgeByParentPrefix returns the prefix for iterating all edges from a parent.
func GetUsageEdgeByParentPrefix(parentClaimID uint64) []byte {
	return append(KeyPrefixUsageEdgeByParent, sdk.Uint64ToBigEndian(parentClaimID)...)
}

// GetImpactUpdateQueueKey returns the store key for a pending impact update by claim ID.
func GetImpactUpdateQueueKey(claimID uint64) []byte {
	return append(KeyPrefixImpactUpdateQueue, sdk.Uint64ToBigEndian(claimID)...)
}
