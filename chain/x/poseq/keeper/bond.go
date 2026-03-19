package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// ─── OperatorBond store ───────────────────────────────────────────────────────

// DeclareOperatorBond stores a new operator bond declaration.
//
// Validation:
//   - node_id must be a registered sequencer
//   - bond_amount must be > 0
//   - no existing active bond for (operatorAddress, nodeIDHex)
//
// Two store keys are written:
//  1. Primary:   [0x0E | operator_address | 0x00 | node_id_hex] → bond JSON
//  2. Secondary: [0x12 | node_id_hex] → operator_address string (reverse lookup)
func (k Keeper) DeclareOperatorBond(ctx context.Context, bond types.OperatorBond) error {
	if bond.BondAmount == 0 {
		return types.ErrInvalidBondAmount
	}
	if len(bond.NodeID) != 64 {
		return types.ErrInvalidNodeID.Wrap("node_id must be 64 hex chars")
	}
	if _, err := hex.DecodeString(bond.NodeID); err != nil {
		return types.ErrInvalidNodeID.Wrap("node_id must be valid hex")
	}

	// Node must be registered
	seq, err := k.GetSequencer(ctx, bond.NodeID)
	if err != nil {
		return fmt.Errorf("checking sequencer: %w", err)
	}
	if seq == nil {
		return types.ErrSequencerNotFound
	}

	kvStore := k.storeService.OpenKVStore(ctx)

	// Check for existing active bond
	primaryKey := types.GetOperatorBondKey(bond.OperatorAddress, bond.NodeID)
	existing, err := kvStore.Get(primaryKey)
	if err != nil {
		return err
	}
	if existing != nil {
		var existingBond types.OperatorBond
		if jsonErr := json.Unmarshal(existing, &existingBond); jsonErr == nil && existingBond.IsActive {
			return types.ErrBondAlreadyExists
		}
	}

	bond.IsActive = true
	if bond.State == "" {
		bond.State = types.BondStateActive
	}
	if bond.AvailableBond == 0 {
		bond.AvailableBond = bond.BondAmount
	}

	bz, err := json.Marshal(bond)
	if err != nil {
		return err
	}

	// Write primary key
	if err := kvStore.Set(primaryKey, bz); err != nil {
		return err
	}

	// Write secondary index: node_id → operator_address
	indexKey := types.GetNodeBondIndexKey(bond.NodeID)
	return kvStore.Set(indexKey, []byte(bond.OperatorAddress))
}

// GetOperatorBond retrieves an operator bond by (operatorAddress, nodeIDHex).
// Returns (nil, nil) if not found.
func (k Keeper) GetOperatorBond(ctx context.Context, operatorAddress, nodeIDHex string) (*types.OperatorBond, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetOperatorBondKey(operatorAddress, nodeIDHex))
	if err != nil || bz == nil {
		return nil, err
	}
	var bond types.OperatorBond
	if err := json.Unmarshal(bz, &bond); err != nil {
		return nil, err
	}
	return &bond, nil
}

// WithdrawOperatorBond marks the operator bond as inactive and records
// the withdrawal epoch.
//
// Returns ErrBondNotFound if no bond exists, ErrBondAlreadyWithdrawn if already inactive.
func (k Keeper) WithdrawOperatorBond(ctx context.Context, operatorAddress, nodeIDHex string, epoch uint64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	primaryKey := types.GetOperatorBondKey(operatorAddress, nodeIDHex)

	bz, err := kvStore.Get(primaryKey)
	if err != nil {
		return err
	}
	if bz == nil {
		return types.ErrBondNotFound
	}

	var bond types.OperatorBond
	if err := json.Unmarshal(bz, &bond); err != nil {
		return err
	}
	if !bond.IsActive {
		return types.ErrBondAlreadyWithdrawn
	}

	bond.IsActive = false
	bond.WithdrawnAtEpoch = epoch

	updated, err := json.Marshal(bond)
	if err != nil {
		return err
	}

	if err := kvStore.Set(primaryKey, updated); err != nil {
		return err
	}

	// Remove secondary index since no active bond for this node
	indexKey := types.GetNodeBondIndexKey(nodeIDHex)
	return kvStore.Delete(indexKey)
}

// GetActiveBondForNode returns the active operator bond for the given node,
// using the secondary index to look up the operator address first.
//
// Returns (nil, nil) if no active bond exists for this node.
func (k Keeper) GetActiveBondForNode(ctx context.Context, nodeIDHex string) (*types.OperatorBond, error) {
	kvStore := k.storeService.OpenKVStore(ctx)

	// Use secondary index to find operator address
	indexKey := types.GetNodeBondIndexKey(nodeIDHex)
	operatorBytes, err := kvStore.Get(indexKey)
	if err != nil || operatorBytes == nil {
		return nil, err
	}
	operatorAddress := string(operatorBytes)

	// Fetch the bond
	bond, err := k.GetOperatorBond(ctx, operatorAddress, nodeIDHex)
	if err != nil {
		return nil, err
	}
	if bond == nil || !bond.IsActive {
		return nil, nil
	}
	return bond, nil
}

// HasActiveBond returns true if the given node has an active operator bond.
func (k Keeper) HasActiveBond(ctx context.Context, nodeIDHex string) (bool, error) {
	bond, err := k.GetActiveBondForNode(ctx, nodeIDHex)
	if err != nil {
		return false, err
	}
	return bond != nil, nil
}

// ─── Slash queue store ────────────────────────────────────────────────────────

// EnqueueSlashEntry adds a slash queue entry to the pending queue.
//
// Enforces MaxSlashQueueDepth from params. If the queue is full, returns ErrSlashQueueFull.
// Last-write-wins for duplicate entry IDs (idempotent for retries).
func (k Keeper) EnqueueSlashEntry(ctx context.Context, entry types.SlashQueueEntry) error {
	if len(entry.EntryID) != 32 {
		return fmt.Errorf("slash entry_id must be 32 bytes")
	}

	params := k.GetParams(ctx)
	if params.MaxSlashQueueDepth > 0 {
		pending, err := k.AllPendingSlashEntries(ctx)
		if err != nil {
			return fmt.Errorf("checking slash queue depth: %w", err)
		}
		if uint32(len(pending)) >= params.MaxSlashQueueDepth {
			return types.ErrSlashQueueFull
		}
	}

	bz, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetSlashQueueKey(entry.EntryID), bz)
}

// GetSlashQueueEntry retrieves a slash queue entry by its entryID.
// Returns (nil, nil) if not found.
func (k Keeper) GetSlashQueueEntry(ctx context.Context, entryID []byte) (*types.SlashQueueEntry, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetSlashQueueKey(entryID))
	if err != nil || bz == nil {
		return nil, err
	}
	var entry types.SlashQueueEntry
	if err := json.Unmarshal(bz, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// AllPendingSlashEntries returns all slash queue entries that have not been executed.
// Uses a prefix scan over [0x0F | ...].
func (k Keeper) AllPendingSlashEntries(ctx context.Context) ([]types.SlashQueueEntry, error) {
	prefix := []byte{types.KeyPrefixSlashQueue}
	end := []byte{types.KeyPrefixSlashQueue + 1}

	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(prefix, end)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var entries []types.SlashQueueEntry
	for ; iter.Valid(); iter.Next() {
		var entry types.SlashQueueEntry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			k.logger.Error("failed to unmarshal slash queue entry", "error", err)
			continue
		}
		if !entry.Executed {
			entries = append(entries, entry)
		}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return entries, nil
}

// AllSlashEntriesForNode returns all (including executed) slash entries for a given node.
// Scans all entries and filters by NodeID — acceptable since queue depth is bounded.
func (k Keeper) AllSlashEntriesForNode(ctx context.Context, nodeIDHex string) ([]types.SlashQueueEntry, error) {
	prefix := []byte{types.KeyPrefixSlashQueue}
	end := []byte{types.KeyPrefixSlashQueue + 1}

	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(prefix, end)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var entries []types.SlashQueueEntry
	for ; iter.Valid(); iter.Next() {
		var entry types.SlashQueueEntry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		if entry.NodeID == nodeIDHex {
			entries = append(entries, entry)
		}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return entries, nil
}

// ─── Slash entry ID helper ────────────────────────────────────────────────────

// computeSlashEntryID returns SHA256(operator_address | node_id_hex | epoch_be(8)).
// Used to deduplicate slash entries for the same (operator, node, epoch).
func computeSlashEntryID(operatorAddr, nodeIDHex string, epoch uint64) []byte {
	h := sha256.New()
	h.Write([]byte(operatorAddr))
	h.Write([]byte(nodeIDHex))
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, epoch)
	h.Write(epochBytes)
	return h.Sum(nil)
}
