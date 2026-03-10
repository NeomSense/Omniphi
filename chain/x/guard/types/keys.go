package types

import (
	"encoding/binary"
)

const (
	// ModuleName defines the module name
	ModuleName = "guard"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// KVStore prefixes
var (
	// ParamsKey is the key for module parameters
	ParamsKey = []byte{0x01}

	// RiskReportPrefix is the prefix for risk reports
	// Key: 0x02 | proposalID (big endian uint64)
	RiskReportPrefix = []byte{0x02}

	// QueuedExecutionPrefix is the prefix for queued executions
	// Key: 0x03 | proposalID (big endian uint64)
	QueuedExecutionPrefix = []byte{0x03}

	// QueueIndexByHeightPrefix is the prefix for height-based queue index
	// Key: 0x04 | height (big endian uint64) | proposalID (big endian uint64)
	QueueIndexByHeightPrefix = []byte{0x04}

	// LastProcessedProposalIDKey stores the last processed gov proposal ID
	LastProcessedProposalIDKey = []byte{0x05}

	// AIModelMetadataKey is the key for AI model metadata (Layer 2)
	AIModelMetadataKey = []byte{0x06}

	// LogisticModelKey is the key for logistic regression model weights (Layer 2)
	LogisticModelKey = []byte{0x07}

	// AIEvaluationPrefix is the prefix for AI evaluation results by proposal ID (Layer 2)
	// Key: 0x08 | proposalID (big endian uint64)
	AIEvaluationPrefix = []byte{0x08}

	// AdvisoryLinkPrefix is the prefix for advisory links by proposal ID (Layer 3)
	// Key: 0x09 | proposalID (big endian uint64)
	AdvisoryLinkPrefix = []byte{0x09}

	// ValidatorPowerSnapshotPrefix stores validator power snapshots for stability checks
	// Key: 0x0A | proposalID (big endian uint64)
	ValidatorPowerSnapshotPrefix = []byte{0x0A}

	// ExecutionMarkerPrefix marks proposals that were executed through x/guard.
	// Used by the bypass-detection invariant to verify no proposal was executed
	// outside the guard pipeline.
	// Key: 0x0B | proposalID (big endian uint64)
	ExecutionMarkerPrefix = []byte{0x0B}

	// ── Dynamic Deterministic Guard (DDG) v2 keys ────────────────────────

	// ReevaluationRecordPrefix stores the latest reevaluation result per proposal.
	// Key: 0x0C | proposalID (big endian uint64)
	ReevaluationRecordPrefix = []byte{0x0C}

	// AggregateRiskWindowKey stores the aggregate risk window state.
	// Single-key: rolling window of active proposals.
	AggregateRiskWindowKey = []byte{0x0D}

	// EmergencyHardeningKey stores the EmergencyHardeningMode flag.
	// Single-key: 1 byte (0x00 or 0x01).
	EmergencyHardeningKey = []byte{0x0E}

	// ThresholdEscalationPrefix stores per-proposal threshold escalation records.
	// Key: 0x0F | proposalID (big endian uint64)
	ThresholdEscalationPrefix = []byte{0x0F}

	// ── Layer 3 v2: Advisory Intelligence + Attack Memory keys ──────────

	// AdvisoryEntryV2Prefix stores versioned advisory entries.
	// Key: 0x10 | proposalID (8 bytes) | advisoryID (8 bytes)
	AdvisoryEntryV2Prefix = []byte{0x10}

	// AdvisoryCorrelationPrefix stores advisory-to-risk correlations.
	// Key: 0x11 | proposalID (8 bytes)
	AdvisoryCorrelationPrefix = []byte{0x11}

	// AttackMemoryPrefix stores attack memory dataset entries.
	// Key: 0x12 | featureHash (32 bytes, raw SHA256)
	AttackMemoryPrefix = []byte{0x12}

	// AdvisoryIndexByTierPrefix is a secondary index for querying advisories by tier.
	// Key: 0x13 | tier (1 byte) | proposalID (8 bytes) | advisoryID (8 bytes)
	AdvisoryIndexByTierPrefix = []byte{0x13}

	// AdvisoryIndexByTrackPrefix is a secondary index for querying advisories by track.
	// Key: 0x14 | trackNameLen (1 byte) | trackName (var) | proposalID (8 bytes) | advisoryID (8 bytes)
	AdvisoryIndexByTrackPrefix = []byte{0x14}

	// AdvisoryIndexByOutcomePrefix is a secondary index for querying advisories by outcome.
	// Key: 0x15 | outcomeLen (1 byte) | outcome (var) | proposalID (8 bytes) | advisoryID (8 bytes)
	AdvisoryIndexByOutcomePrefix = []byte{0x15}

	// AdvisoryCounterPrefix stores the advisory count per proposal for auto-incrementing.
	// Key: 0x16 | proposalID (8 bytes) → uint64 big-endian counter
	AdvisoryCounterPrefix = []byte{0x16}

	// AttackMemoryByProposalPrefix is a secondary index for attack memory by proposal ID.
	// Key: 0x17 | proposalID (8 bytes)
	AttackMemoryByProposalPrefix = []byte{0x17}

	// TreasuryOutflowWindowKey stores the rolling 24h treasury outflow window.
	// Single-key: cumulative bps of community pool spends within the window.
	TreasuryOutflowWindowKey = []byte{0x18}

	// ── Guard/Timelock integration keys ──────────────────────────────────

	// TimelockHandoverPrefix marks proposals where timelock has authorized
	// guard to execute despite StatusFailed.
	// Key: 0x19 | proposalID (big endian uint64)
	TimelockHandoverPrefix = []byte{0x19}

	// ActiveExecutionIndexPrefix is the secondary index for active (non-terminal)
	// queued executions. Maintained by SetQueuedExecution to avoid full-table scans.
	// Key: 0x1A | proposalID (big endian uint64) -> []byte{1}
	ActiveExecutionIndexPrefix = []byte{0x1A}
)

// GetRiskReportKey returns the key for a risk report
func GetRiskReportKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = RiskReportPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetQueuedExecutionKey returns the key for a queued execution
func GetQueuedExecutionKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = QueuedExecutionPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetQueueIndexKey returns the key for the queue height index
func GetQueueIndexKey(height uint64, proposalID uint64) []byte {
	key := make([]byte, 1+8+8)
	key[0] = QueueIndexByHeightPrefix[0]
	binary.BigEndian.PutUint64(key[1:9], height)
	binary.BigEndian.PutUint64(key[9:], proposalID)
	return key
}

// GetQueueIndexPrefixByHeight returns the prefix for all queue entries at a height
func GetQueueIndexPrefixByHeight(height uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = QueueIndexByHeightPrefix[0]
	binary.BigEndian.PutUint64(key[1:], height)
	return key
}

// SdkUint64ToBigEndian converts uint64 to big endian bytes
func SdkUint64ToBigEndian(i uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, i)
	return b
}

// BigEndianToUint64 converts big endian bytes to uint64
func BigEndianToUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

// GetAIEvaluationKey returns the key for an AI evaluation result
func GetAIEvaluationKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = AIEvaluationPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetAdvisoryLinkKey returns the key for an advisory link
func GetAdvisoryLinkKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = AdvisoryLinkPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetValidatorPowerSnapshotKey returns the key for a validator power snapshot
func GetValidatorPowerSnapshotKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = ValidatorPowerSnapshotPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetExecutionMarkerKey returns the key for an execution marker
func GetExecutionMarkerKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = ExecutionMarkerPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetReevaluationRecordKey returns the key for a reevaluation record
func GetReevaluationRecordKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = ReevaluationRecordPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetThresholdEscalationKey returns the key for a threshold escalation record
func GetThresholdEscalationKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = ThresholdEscalationPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// ── Layer 3 v2 key helpers ──

// GetAdvisoryEntryV2Key returns the key for a versioned advisory entry.
func GetAdvisoryEntryV2Key(proposalID, advisoryID uint64) []byte {
	key := make([]byte, 1+8+8)
	key[0] = AdvisoryEntryV2Prefix[0]
	binary.BigEndian.PutUint64(key[1:9], proposalID)
	binary.BigEndian.PutUint64(key[9:17], advisoryID)
	return key
}

// GetAdvisoryEntryV2PrefixByProposal returns the prefix for all advisories of a proposal.
func GetAdvisoryEntryV2PrefixByProposal(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = AdvisoryEntryV2Prefix[0]
	binary.BigEndian.PutUint64(key[1:9], proposalID)
	return key
}

// GetAdvisoryCorrelationKey returns the key for an advisory correlation.
func GetAdvisoryCorrelationKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = AdvisoryCorrelationPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetAttackMemoryKey returns the key for an attack memory entry by feature hash.
// featureHashHex must be a 64-char hex string (SHA256).
func GetAttackMemoryKey(featureHashHex string) []byte {
	// Use raw hex bytes as key suffix (up to 32 bytes)
	hashBytes := []byte(featureHashHex)
	if len(hashBytes) > 64 {
		hashBytes = hashBytes[:64]
	}
	key := make([]byte, 1+len(hashBytes))
	key[0] = AttackMemoryPrefix[0]
	copy(key[1:], hashBytes)
	return key
}

// GetAttackMemoryByProposalKey returns the key for attack memory indexed by proposal.
func GetAttackMemoryByProposalKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = AttackMemoryByProposalPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetAdvisoryIndexByTierKey returns the secondary index key for tier-based queries.
func GetAdvisoryIndexByTierKey(tier RiskTier, proposalID, advisoryID uint64) []byte {
	key := make([]byte, 1+1+8+8)
	key[0] = AdvisoryIndexByTierPrefix[0]
	key[1] = byte(tier)
	binary.BigEndian.PutUint64(key[2:10], proposalID)
	binary.BigEndian.PutUint64(key[10:18], advisoryID)
	return key
}

// GetAdvisoryIndexByTierPrefix returns the prefix for all advisories of a given tier.
func GetAdvisoryIndexByTierPrefixForTier(tier RiskTier) []byte {
	key := make([]byte, 1+1)
	key[0] = AdvisoryIndexByTierPrefix[0]
	key[1] = byte(tier)
	return key
}

// GetAdvisoryIndexByTrackKey returns the secondary index key for track-based queries.
func GetAdvisoryIndexByTrackKey(trackName string, proposalID, advisoryID uint64) []byte {
	tn := []byte(trackName)
	if len(tn) > 255 {
		tn = tn[:255]
	}
	key := make([]byte, 1+1+len(tn)+8+8)
	key[0] = AdvisoryIndexByTrackPrefix[0]
	key[1] = byte(len(tn))
	copy(key[2:2+len(tn)], tn)
	binary.BigEndian.PutUint64(key[2+len(tn):], proposalID)
	binary.BigEndian.PutUint64(key[2+len(tn)+8:], advisoryID)
	return key
}

// GetAdvisoryIndexByTrackPrefixForTrack returns the prefix for all advisories of a given track.
func GetAdvisoryIndexByTrackPrefixForTrack(trackName string) []byte {
	tn := []byte(trackName)
	if len(tn) > 255 {
		tn = tn[:255]
	}
	key := make([]byte, 1+1+len(tn))
	key[0] = AdvisoryIndexByTrackPrefix[0]
	key[1] = byte(len(tn))
	copy(key[2:], tn)
	return key
}

// GetAdvisoryIndexByOutcomeKey returns the secondary index key for outcome-based queries.
func GetAdvisoryIndexByOutcomeKey(outcome string, proposalID, advisoryID uint64) []byte {
	oc := []byte(outcome)
	if len(oc) > 255 {
		oc = oc[:255]
	}
	key := make([]byte, 1+1+len(oc)+8+8)
	key[0] = AdvisoryIndexByOutcomePrefix[0]
	key[1] = byte(len(oc))
	copy(key[2:2+len(oc)], oc)
	binary.BigEndian.PutUint64(key[2+len(oc):], proposalID)
	binary.BigEndian.PutUint64(key[2+len(oc)+8:], advisoryID)
	return key
}

// GetAdvisoryIndexByOutcomePrefixForOutcome returns the prefix for all advisories of a given outcome.
func GetAdvisoryIndexByOutcomePrefixForOutcome(outcome string) []byte {
	oc := []byte(outcome)
	if len(oc) > 255 {
		oc = oc[:255]
	}
	key := make([]byte, 1+1+len(oc))
	key[0] = AdvisoryIndexByOutcomePrefix[0]
	key[1] = byte(len(oc))
	copy(key[2:], oc)
	return key
}

// GetAdvisoryCounterKey returns the key for the advisory counter of a proposal.
func GetAdvisoryCounterKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = AdvisoryCounterPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetTimelockHandoverKey returns the key for a timelock handover marker.
func GetTimelockHandoverKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = TimelockHandoverPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// GetActiveExecutionIndexKey returns the key for an active execution index entry.
func GetActiveExecutionIndexKey(proposalID uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = ActiveExecutionIndexPrefix[0]
	binary.BigEndian.PutUint64(key[1:], proposalID)
	return key
}

// PrefixEnd returns the end key for a given prefix (for iteration)
func PrefixEnd(prefix []byte) []byte {
	end := make([]byte, len(prefix))
	copy(end, prefix)
	end[len(end)-1]++
	return end
}
