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
//  4. Deactivate sequencer in registry (sets IsActive = false)
//  5. Store SlashRecord for audit trail
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

	// 2. Deactivate the sequencer
	if err := m.Keeper.SetSequencerActive(ctx, msg.NodeID, false); err != nil {
		// If the sequencer isn't registered, log a warning but continue.
		// (It may have been registered off-chain or the registry is out of sync.)
		if err != types.ErrSequencerNotFound {
			return fmt.Errorf("deactivating sequencer: %w", err)
		}
		m.Keeper.Logger().Warn("sequencer not found in registry during slash — evidence recorded anyway",
			"node_id", msg.NodeID,
		)
	}

	// 3. Record the slash
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
	)
	return nil
}
