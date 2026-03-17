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
