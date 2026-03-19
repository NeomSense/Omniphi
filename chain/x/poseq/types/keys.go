package types

const (
	ModuleName = "poseq"
	StoreKey   = ModuleName
	RouterKey  = ModuleName

	// Store key prefixes
	KeyPrefixEvidencePacket    = 0x01
	KeyPrefixEscalationRecord  = 0x02
	KeyPrefixCheckpointAnchor  = 0x03
	KeyPrefixEpochState        = 0x04
	KeyPrefixSuspension        = 0x05
	KeyPrefixExportBatch       = 0x06

	// Sequencer registry prefix
	KeyPrefixSequencer = 0x07

	// Committed batches prefix (Sprint 4A)
	KeyPrefixCommittedBatch = 0x08

	// Settlement anchor prefix (Phase 7)
	KeyPrefixSettlementAnchor = 0x0A

	// Committee snapshot prefix — keyed by epoch_be(8)
	KeyPrefixCommitteeSnapshot = 0x0B

	// Liveness event prefix — keyed by epoch_be(8) | node_id(32)
	KeyPrefixLivenessEvent = 0x0C

	// Performance record prefix — keyed by epoch_be(8) | node_id(32)
	KeyPrefixPerformanceRecord = 0x0D

	// Bond record prefix — keyed by operator_address bytes | 0x00 | node_id_hex_bytes
	KeyPrefixOperatorBond = 0x0E

	// Slash queue prefix — keyed by entry_id (32 bytes)
	KeyPrefixSlashQueue = 0x0F

	// Epoch reward score prefix — keyed by epoch_be(8) | node_id_hex_bytes
	KeyPrefixRewardScore = 0x10

	// PoC multiplier prefix — keyed by epoch_be(8) | operator_address_bytes
	KeyPrefixPoCMultiplier = 0x11

	// Node bond index prefix — secondary index: node_id_hex_bytes → operator_address string
	KeyPrefixNodeBondIndex = 0x12

	// Adjudication record prefix — keyed by evidence_packet_hash (32 bytes)
	KeyPrefixAdjudication = 0x13

	// Epoch settlement record prefix — keyed by epoch_be(8) | node_id_hex_bytes
	KeyPrefixEpochSettlement = 0x14

	// Sequencer ranking profile prefix — keyed by node_id_hex_bytes
	KeyPrefixRankingProfile = 0x15

	// Export batch ingestion dedup prefix — keyed by epoch_be(8)
	// Stores the ingestion result for each epoch to prevent double processing.
	KeyPrefixExportBatchDedup = 0x16

	// Params key
	ParamsKey = "params"
)

// GetEvidencePacketKey returns the store key for an evidence packet by its hash.
func GetEvidencePacketKey(packetHash []byte) []byte {
	key := make([]byte, 1+len(packetHash))
	key[0] = KeyPrefixEvidencePacket
	copy(key[1:], packetHash)
	return key
}

// GetEscalationRecordKey returns the store key for a governance escalation by its ID.
func GetEscalationRecordKey(escalationID []byte) []byte {
	key := make([]byte, 1+len(escalationID))
	key[0] = KeyPrefixEscalationRecord
	copy(key[1:], escalationID)
	return key
}

// GetCheckpointAnchorKey returns the store key for a checkpoint anchor by (epoch, slot).
func GetCheckpointAnchorKey(epoch, slot uint64) []byte {
	key := make([]byte, 1+8+8)
	key[0] = KeyPrefixCheckpointAnchor
	putUint64BE(key[1:], epoch)
	putUint64BE(key[9:], slot)
	return key
}

// GetEpochStateKey returns the store key for an epoch state reference.
func GetEpochStateKey(epoch uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = KeyPrefixEpochState
	putUint64BE(key[1:], epoch)
	return key
}

// GetSuspensionKey returns the store key for a suspension record by node ID.
func GetSuspensionKey(nodeID []byte) []byte {
	key := make([]byte, 1+len(nodeID))
	key[0] = KeyPrefixSuspension
	copy(key[1:], nodeID)
	return key
}

// GetExportBatchKey returns the store key for an export batch by epoch.
func GetExportBatchKey(epoch uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = KeyPrefixExportBatch
	putUint64BE(key[1:], epoch)
	return key
}

// GetSequencerKey returns the store key for a sequencer record by node_id (raw bytes).
func GetSequencerKey(nodeID []byte) []byte {
	key := make([]byte, 1+len(nodeID))
	key[0] = KeyPrefixSequencer
	copy(key[1:], nodeID)
	return key
}

// GetCommittedBatchKey returns the store key for a committed batch by its batch_id.
func GetCommittedBatchKey(batchID []byte) []byte {
	key := make([]byte, 1+len(batchID))
	key[0] = KeyPrefixCommittedBatch
	copy(key[1:], batchID)
	return key
}

// GetSettlementAnchorKey returns the store key for a settlement anchor by batch hash.
func GetSettlementAnchorKey(batchHash []byte) []byte {
	key := make([]byte, 1+len(batchHash))
	key[0] = KeyPrefixSettlementAnchor
	copy(key[1:], batchHash)
	return key
}

// GetCommitteeSnapshotKey returns the store key for a committee snapshot by epoch.
func GetCommitteeSnapshotKey(epoch uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = KeyPrefixCommitteeSnapshot
	putUint64BE(key[1:], epoch)
	return key
}

// GetLivenessEventKey returns the store key for a liveness event by (epoch, node_id).
func GetLivenessEventKey(epoch uint64, nodeID []byte) []byte {
	key := make([]byte, 1+8+len(nodeID))
	key[0] = KeyPrefixLivenessEvent
	putUint64BE(key[1:], epoch)
	copy(key[9:], nodeID)
	return key
}

// GetPerformanceRecordKey returns the store key for a performance record by (epoch, node_id).
func GetPerformanceRecordKey(epoch uint64, nodeID []byte) []byte {
	key := make([]byte, 1+8+len(nodeID))
	key[0] = KeyPrefixPerformanceRecord
	putUint64BE(key[1:], epoch)
	copy(key[9:], nodeID)
	return key
}

// GetOperatorBondKey returns the store key for an operator bond by (operatorAddress, nodeIDHex).
// Layout: [0x0E | operator_address_bytes | 0x00 | node_id_hex_bytes]
// The 0x00 separator prevents collisions when operator_address is a variable-length string.
func GetOperatorBondKey(operatorAddress string, nodeIDHex string) []byte {
	opBytes := []byte(operatorAddress)
	nodeBytes := []byte(nodeIDHex)
	key := make([]byte, 1+len(opBytes)+1+len(nodeBytes))
	key[0] = KeyPrefixOperatorBond
	copy(key[1:], opBytes)
	key[1+len(opBytes)] = 0x00
	copy(key[2+len(opBytes):], nodeBytes)
	return key
}

// GetOperatorBondPrefixKey returns the prefix key for range-scanning all bonds
// for a given operator address.
// Layout: [0x0E | operator_address_bytes | 0x00]
func GetOperatorBondPrefixKey(operatorAddress string) []byte {
	opBytes := []byte(operatorAddress)
	key := make([]byte, 1+len(opBytes)+1)
	key[0] = KeyPrefixOperatorBond
	copy(key[1:], opBytes)
	key[1+len(opBytes)] = 0x00
	return key
}

// GetSlashQueueKey returns the store key for a slash queue entry by its entryID.
func GetSlashQueueKey(entryID []byte) []byte {
	key := make([]byte, 1+len(entryID))
	key[0] = KeyPrefixSlashQueue
	copy(key[1:], entryID)
	return key
}

// GetRewardScoreKey returns the store key for an epoch reward score by (epoch, nodeID).
// Layout: [0x10 | epoch_be(8) | node_id_hex_bytes]
func GetRewardScoreKey(epoch uint64, nodeID string) []byte {
	nodeBytes := []byte(nodeID)
	key := make([]byte, 1+8+len(nodeBytes))
	key[0] = KeyPrefixRewardScore
	putUint64BE(key[1:], epoch)
	copy(key[9:], nodeBytes)
	return key
}

// GetRewardScoreEpochPrefix returns the prefix for scanning all reward scores
// for a given epoch.
// Layout: [0x10 | epoch_be(8)]
func GetRewardScoreEpochPrefix(epoch uint64) []byte {
	key := make([]byte, 9)
	key[0] = KeyPrefixRewardScore
	putUint64BE(key[1:], epoch)
	return key
}

// GetPoCMultiplierKey returns the store key for a PoC multiplier record by (epoch, operatorAddress).
// Layout: [0x11 | epoch_be(8) | operator_address_bytes]
func GetPoCMultiplierKey(epoch uint64, operatorAddress string) []byte {
	opBytes := []byte(operatorAddress)
	key := make([]byte, 1+8+len(opBytes))
	key[0] = KeyPrefixPoCMultiplier
	putUint64BE(key[1:], epoch)
	copy(key[9:], opBytes)
	return key
}

// GetNodeBondIndexKey returns the secondary index key that maps nodeIDHex → operator address.
// Layout: [0x12 | node_id_hex_bytes]
func GetNodeBondIndexKey(nodeIDHex string) []byte {
	nodeBytes := []byte(nodeIDHex)
	key := make([]byte, 1+len(nodeBytes))
	key[0] = KeyPrefixNodeBondIndex
	copy(key[1:], nodeBytes)
	return key
}

// GetAdjudicationKey returns the store key for an adjudication record by evidence packet hash.
func GetAdjudicationKey(packetHash []byte) []byte {
	key := make([]byte, 1+len(packetHash))
	key[0] = KeyPrefixAdjudication
	copy(key[1:], packetHash)
	return key
}

// GetEpochSettlementKey returns the store key for an epoch settlement record by (epoch, nodeID).
// Layout: [0x14 | epoch_be(8) | node_id_hex_bytes]
func GetEpochSettlementKey(epoch uint64, nodeIDHex string) []byte {
	nodeBytes := []byte(nodeIDHex)
	key := make([]byte, 1+8+len(nodeBytes))
	key[0] = KeyPrefixEpochSettlement
	putUint64BE(key[1:], epoch)
	copy(key[9:], nodeBytes)
	return key
}

// GetEpochSettlementPrefix returns the prefix for scanning all settlements for an epoch.
func GetEpochSettlementPrefix(epoch uint64) []byte {
	key := make([]byte, 9)
	key[0] = KeyPrefixEpochSettlement
	putUint64BE(key[1:], epoch)
	return key
}

// GetRankingProfileKey returns the store key for a sequencer ranking profile by nodeIDHex.
func GetRankingProfileKey(nodeIDHex string) []byte {
	nodeBytes := []byte(nodeIDHex)
	key := make([]byte, 1+len(nodeBytes))
	key[0] = KeyPrefixRankingProfile
	copy(key[1:], nodeBytes)
	return key
}

// GetExportBatchDedupKey returns the store key for batch ingestion dedup by epoch.
func GetExportBatchDedupKey(epoch uint64) []byte {
	key := make([]byte, 1+8)
	key[0] = KeyPrefixExportBatchDedup
	putUint64BE(key[1:], epoch)
	return key
}

func putUint64BE(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}
