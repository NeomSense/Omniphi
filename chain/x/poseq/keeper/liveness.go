package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"pos/x/poseq/types"
)

// ─── LivenessEvent store ──────────────────────────────────────────────────────

// StoreLivenessEvent stores a liveness event keyed by (epoch, node_id).
// Last-write-wins — subsequent submissions overwrite.
func (k Keeper) StoreLivenessEvent(ctx context.Context, event types.LivenessEvent) error {
	if len(event.NodeID) != 32 {
		return types.ErrLivenessEventInvalid.Wrap("node_id must be 32 bytes")
	}
	if event.Epoch == 0 {
		return types.ErrLivenessEventInvalid.Wrap("epoch must be > 0")
	}
	bz, err := json.Marshal(event)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetLivenessEventKey(event.Epoch, event.NodeID), bz)
}

// GetLivenessEvent retrieves a liveness event for (epoch, nodeID).
// Returns (nil, nil) if not found.
func (k Keeper) GetLivenessEvent(ctx context.Context, epoch uint64, nodeID []byte) (*types.LivenessEvent, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetLivenessEventKey(epoch, nodeID))
	if err != nil || bz == nil {
		return nil, err
	}
	var ev types.LivenessEvent
	if err := json.Unmarshal(bz, &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

// ─── NodePerformanceRecord store ─────────────────────────────────────────────

// StorePerformanceRecord stores a performance record keyed by (epoch, node_id).
// Last-write-wins.
func (k Keeper) StorePerformanceRecord(ctx context.Context, rec types.NodePerformanceRecord) error {
	if len(rec.NodeID) != 32 {
		return types.ErrPerformanceRecordInvalid.Wrap("node_id must be 32 bytes")
	}
	if rec.Epoch == 0 {
		return types.ErrPerformanceRecordInvalid.Wrap("epoch must be > 0")
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetPerformanceRecordKey(rec.Epoch, rec.NodeID), bz)
}

// GetPerformanceRecord retrieves a performance record for (epoch, nodeID).
// Returns (nil, nil) if not found.
func (k Keeper) GetPerformanceRecord(ctx context.Context, epoch uint64, nodeID []byte) (*types.NodePerformanceRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetPerformanceRecordKey(epoch, nodeID))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.NodePerformanceRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// AllPerformanceRecordsForEpoch returns all performance records for a given epoch
// via a prefix scan over [KeyPrefixPerformanceRecord | epoch_be(8)].
func (k Keeper) AllPerformanceRecordsForEpoch(ctx context.Context, epoch uint64) ([]types.NodePerformanceRecord, error) {
	// Build prefix: [0x0D | epoch_be(8)]
	prefix := types.GetPerformanceRecordKey(epoch, make([]byte, 0))
	end := make([]byte, len(prefix))
	copy(end, prefix)
	// Increment last byte to form exclusive end bound
	end[len(end)-1] = 0xFF

	// Use a 9-byte prefix for the scan: [0x0D | epoch_be(8)]
	scanPrefix := make([]byte, 9)
	scanPrefix[0] = types.KeyPrefixPerformanceRecord
	scanPrefix[1] = byte(epoch >> 56)
	scanPrefix[2] = byte(epoch >> 48)
	scanPrefix[3] = byte(epoch >> 40)
	scanPrefix[4] = byte(epoch >> 32)
	scanPrefix[5] = byte(epoch >> 24)
	scanPrefix[6] = byte(epoch >> 16)
	scanPrefix[7] = byte(epoch >> 8)
	scanPrefix[8] = byte(epoch)

	scanEnd := make([]byte, 9)
	copy(scanEnd, scanPrefix)
	// Increment the epoch bytes to get the next epoch prefix as end bound
	for i := 8; i >= 1; i-- {
		scanEnd[i]++
		if scanEnd[i] != 0 {
			break
		}
	}

	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(scanPrefix, scanEnd)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var records []types.NodePerformanceRecord
	for ; iter.Valid(); iter.Next() {
		var rec types.NodePerformanceRecord
		if err := json.Unmarshal(iter.Value(), &rec); err != nil {
			k.logger.Error("failed to unmarshal performance record", "error", err)
			continue
		}
		records = append(records, rec)
	}
	return records, iter.Close()
}

// ─── gRPC query helpers ───────────────────────────────────────────────────────

// GetLivenessEventByHex retrieves a liveness event using a hex-encoded node_id.
func (k Keeper) GetLivenessEventByHex(ctx context.Context, epoch uint64, nodeIDHex string) (*types.LivenessEvent, error) {
	nodeID, err := hex.DecodeString(nodeIDHex)
	if err != nil || len(nodeID) != 32 {
		return nil, types.ErrInvalidNodeID
	}
	return k.GetLivenessEvent(ctx, epoch, nodeID)
}

// GetPerformanceRecordByHex retrieves a performance record using a hex-encoded node_id.
func (k Keeper) GetPerformanceRecordByHex(ctx context.Context, epoch uint64, nodeIDHex string) (*types.NodePerformanceRecord, error) {
	nodeID, err := hex.DecodeString(nodeIDHex)
	if err != nil || len(nodeID) != 32 {
		return nil, types.ErrInvalidNodeID
	}
	return k.GetPerformanceRecord(ctx, epoch, nodeID)
}
