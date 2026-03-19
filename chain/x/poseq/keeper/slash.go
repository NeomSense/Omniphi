package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// ─── Slash record store ───────────────────────────────────────────────────────

const (
	// KeyPrefixSlashRecord stores slash execution records keyed by node_id + packet_hash.
	KeyPrefixSlashRecord = 0x09
)

func getSlashRecordKey(nodeIDBytes, packetHashBytes []byte) []byte {
	key := make([]byte, 1+len(nodeIDBytes)+len(packetHashBytes))
	key[0] = KeyPrefixSlashRecord
	copy(key[1:], nodeIDBytes)
	copy(key[1+len(nodeIDBytes):], packetHashBytes)
	return key
}

// StoreSlashRecord stores a slash record. Returns an error if a record for
// the same (node_id, packet_hash) pair already exists (idempotent protection).
func (k Keeper) StoreSlashRecord(ctx context.Context, rec types.SlashRecord) error {
	nodeIDBytes, err := hex.DecodeString(rec.NodeID)
	if err != nil || len(nodeIDBytes) != 32 {
		return types.ErrInvalidNodeID
	}
	pktHashBytes, err := hex.DecodeString(rec.PacketHash)
	if err != nil || len(pktHashBytes) != 32 {
		return types.ErrInvalidPacketHash
	}

	kvStore := k.storeService.OpenKVStore(ctx)
	key := getSlashRecordKey(nodeIDBytes, pktHashBytes)
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		// Idempotent: same slash already recorded
		return nil
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return kvStore.Set(key, bz)
}

// GetSlashRecord retrieves a slash record by (nodeIDHex, packetHashHex).
// Returns (nil, nil) if not found.
func (k Keeper) GetSlashRecord(ctx context.Context, nodeIDHex, packetHashHex string) (*types.SlashRecord, error) {
	nodeIDBytes, err := hex.DecodeString(nodeIDHex)
	if err != nil || len(nodeIDBytes) != 32 {
		return nil, types.ErrInvalidNodeID
	}
	pktHashBytes, err := hex.DecodeString(packetHashHex)
	if err != nil || len(pktHashBytes) != 32 {
		return nil, types.ErrInvalidPacketHash
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(getSlashRecordKey(nodeIDBytes, pktHashBytes))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.SlashRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// ─── MsgExecuteSlash handler ──────────────────────────────────────────────────

// ExecuteSlash processes MsgExecuteSlash (governance-gated).
//
// Pipeline:
//  1. Validate msg
//  2. Check authority == governance authority
//  3. Verify EvidencePacket exists (justification must be on-chain)
//  4. Double-slash prevention: check no SlashRecord exists for (node_id, packet_hash)
//  5. Staleness check: evidence epoch must be within MaxEvidenceAgeEpochs
//  6. Transition sequencer to Jailed status
//  7. Reduce AvailableBond if SlashExecutionEnabled; update BondState
//  8. Store AdjudicationRecord (Penalized)
//  9. Store SlashRecord for audit trail
func (m MsgServer) ExecuteSlash(ctx context.Context, msg *types.MsgExecuteSlash) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if msg.Authority != m.Keeper.Authority() {
		return types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			m.Keeper.Authority(), msg.Authority,
		)
	}

	// 1. Verify EvidencePacket exists on-chain
	pktHashBytes, _ := hex.DecodeString(msg.PacketHash)
	pkt, err := m.Keeper.GetEvidencePacket(ctx, pktHashBytes)
	if err != nil {
		return fmt.Errorf("retrieving evidence packet: %w", err)
	}
	if pkt == nil {
		return types.ErrInvalidPacketHash.Wrapf(
			"evidence packet %s not found — must be submitted before executing slash",
			msg.PacketHash,
		)
	}

	// 2. Double-slash prevention
	existing, err := m.Keeper.GetSlashRecord(ctx, msg.NodeID, msg.PacketHash)
	if err != nil {
		return fmt.Errorf("checking for existing slash record: %w", err)
	}
	if existing != nil {
		return types.ErrDoubleSlash.Wrapf(
			"slash already executed for node %s, packet %s at epoch %d",
			msg.NodeID, msg.PacketHash, existing.Epoch,
		)
	}

	// 3. Staleness check
	params := m.Keeper.GetParams(ctx)
	if params.MaxEvidenceAgeEpochs > 0 && msg.CurrentEpoch > 0 {
		if pkt.Epoch+uint64(params.MaxEvidenceAgeEpochs) < msg.CurrentEpoch {
			return types.ErrStaleEvidence.Wrapf(
				"evidence from epoch %d is older than max age %d (current epoch %d)",
				pkt.Epoch, params.MaxEvidenceAgeEpochs, msg.CurrentEpoch,
			)
		}
	}

	// 4. Transition sequencer to Jailed status
	if err := m.Keeper.SetSequencerStatus(ctx, msg.NodeID, types.SequencerStatusJailed, msg.CurrentEpoch); err != nil {
		if err != types.ErrSequencerNotFound {
			return fmt.Errorf("jailing sequencer: %w", err)
		}
		m.Keeper.Logger().Warn("sequencer not found in registry during slash — evidence recorded anyway",
			"node_id", msg.NodeID,
		)
	}

	// 5. Bond reduction (only if SlashExecutionEnabled)
	var slashedAmount uint64
	if params.SlashExecutionEnabled {
		slashedAmount, err = m.Keeper.ApplyBondSlash(ctx, msg.NodeID, msg.SlashBps, pkt.Epoch)
		if err != nil {
			// Bond errors are non-fatal — node may not be bonded. Log and continue.
			m.Keeper.Logger().Warn("bond slash skipped",
				"node_id", msg.NodeID,
				"reason", err.Error(),
			)
		}
	}

	// 6. Store AdjudicationRecord
	adjRec := types.AdjudicationRecord{
		PacketHash:      msg.PacketHash,
		NodeID:          msg.NodeID,
		MisbehaviorType: string(pkt.Kind),
		Epoch:           pkt.Epoch,
		Path:            types.AdjudicationPathGovernanceReview,
		Decision:        types.AdjudicationDecisionPenalized,
		SlashBps:        msg.SlashBps,
		DecidedAtEpoch:  msg.CurrentEpoch,
		Reason:          msg.Reason,
		AutoApplied:     false,
	}
	if err := m.Keeper.StoreAdjudicationRecord(ctx, adjRec); err != nil {
		// Non-fatal: record failure in log but do not abort the slash
		m.Keeper.Logger().Warn("failed to store adjudication record",
			"node_id", msg.NodeID,
			"error", err.Error(),
		)
	}

	// 7. Record the slash
	rec := types.SlashRecord{
		NodeID:            msg.NodeID,
		PacketHash:        msg.PacketHash,
		SlashBps:          msg.SlashBps,
		Epoch:             pkt.Epoch,
		ExecutedByAddress: msg.Authority,
		Reason:            msg.Reason,
	}
	if err := m.Keeper.StoreSlashRecord(ctx, rec); err != nil {
		return fmt.Errorf("storing slash record: %w", err)
	}

	m.Keeper.Logger().Info("slash executed",
		"node_id", msg.NodeID,
		"packet_hash", msg.PacketHash,
		"slash_bps", msg.SlashBps,
		"epoch", pkt.Epoch,
		"slashed_amount", slashedAmount,
	)
	return nil
}
