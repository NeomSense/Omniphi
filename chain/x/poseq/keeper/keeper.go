package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"

	"pos/x/poseq/types"
)

// Keeper manages x/poseq state.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger

	// authority is the governance module address (for MsgUpdateParams).
	authority string
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		logger:       logger.With("module", fmt.Sprintf("x/%s", types.ModuleName)),
		authority:    authority,
	}
}

func (k Keeper) Logger() log.Logger { return k.logger }
func (k Keeper) Authority() string  { return k.authority }

// ─── Params ───────────────────────────────────────────────────────────────────

func (k Keeper) GetParams(ctx context.Context) types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get([]byte(types.ParamsKey))
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var p types.Params
	if err := json.Unmarshal(bz, &p); err != nil {
		return types.DefaultParams()
	}
	return p
}

func (k Keeper) SetParams(ctx context.Context, p types.Params) error {
	bz, err := json.Marshal(p)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set([]byte(types.ParamsKey), bz)
}

// ─── EvidencePacket ───────────────────────────────────────────────────────────

// StoreEvidencePacket stores an evidence packet. Returns ErrDuplicateEvidencePacket
// if a packet with the same hash already exists (idempotent safety).
func (k Keeper) StoreEvidencePacket(ctx context.Context, packet types.EvidencePacket) error {
	if len(packet.PacketHash) != 32 {
		return types.ErrInvalidPacketHash
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetEvidencePacketKey(packet.PacketHash)
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		return types.ErrDuplicateEvidencePacket
	}
	bz, err := json.Marshal(packet)
	if err != nil {
		return err
	}
	return kvStore.Set(key, bz)
}

// GetEvidencePacket retrieves an evidence packet by its hash. Returns (nil, nil) if not found.
func (k Keeper) GetEvidencePacket(ctx context.Context, packetHash []byte) (*types.EvidencePacket, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetEvidencePacketKey(packetHash))
	if err != nil || bz == nil {
		return nil, err
	}
	var p types.EvidencePacket
	if err := json.Unmarshal(bz, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ─── GovernanceEscalationRecord ───────────────────────────────────────────────

// StoreEscalationRecord stores a governance escalation. Write-once — returns
// ErrDuplicateEscalation if the escalation_id already exists.
func (k Keeper) StoreEscalationRecord(ctx context.Context, rec types.GovernanceEscalationRecord) error {
	if len(rec.EscalationID) != 32 {
		return types.ErrInvalidEscalationID
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetEscalationRecordKey(rec.EscalationID)
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		return types.ErrDuplicateEscalation
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return kvStore.Set(key, bz)
}

// GetEscalationRecord retrieves a governance escalation record by its ID.
func (k Keeper) GetEscalationRecord(ctx context.Context, escalationID []byte) (*types.GovernanceEscalationRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetEscalationRecordKey(escalationID))
	if err != nil || bz == nil {
		return nil, err
	}
	var r types.GovernanceEscalationRecord
	if err := json.Unmarshal(bz, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ─── CheckpointAnchorRecord ───────────────────────────────────────────────────

// StoreCheckpointAnchor stores a checkpoint anchor. Write-once — returns
// ErrDuplicateCheckpointAnchor if an anchor for this (epoch, slot) already exists.
// Also verifies the anchor_hash before storing.
func (k Keeper) StoreCheckpointAnchor(ctx context.Context, anchor types.CheckpointAnchorRecord) error {
	if len(anchor.CheckpointID) != 32 {
		return types.ErrInvalidCheckpointID
	}
	if !verifyCheckpointAnchorHash(anchor) {
		return types.ErrCheckpointAnchorTampered
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetCheckpointAnchorKey(anchor.Epoch, anchor.Slot)
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		return types.ErrDuplicateCheckpointAnchor
	}
	bz, err := json.Marshal(anchor)
	if err != nil {
		return err
	}
	return kvStore.Set(key, bz)
}

// GetCheckpointAnchor retrieves a checkpoint anchor by (epoch, slot).
func (k Keeper) GetCheckpointAnchor(ctx context.Context, epoch, slot uint64) (*types.CheckpointAnchorRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetCheckpointAnchorKey(epoch, slot))
	if err != nil || bz == nil {
		return nil, err
	}
	var a types.CheckpointAnchorRecord
	if err := json.Unmarshal(bz, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// ─── EpochStateReference ──────────────────────────────────────────────────────

// StoreEpochState stores the epoch state reference. Overwrites any existing
// record for this epoch (last-write-wins for operator visibility).
func (k Keeper) StoreEpochState(ctx context.Context, state types.EpochStateReference) error {
	if state.Epoch == 0 {
		return types.ErrInvalidEpoch
	}
	bz, err := json.Marshal(state)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetEpochStateKey(state.Epoch), bz)
}

// GetEpochState retrieves epoch state for the given epoch.
func (k Keeper) GetEpochState(ctx context.Context, epoch uint64) (*types.EpochStateReference, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetEpochStateKey(epoch))
	if err != nil || bz == nil {
		return nil, err
	}
	var s types.EpochStateReference
	if err := json.Unmarshal(bz, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ─── CommitteeSuspensionRecommendation ────────────────────────────────────────

// StoreSuspension stores a suspension recommendation keyed by node_id.
// Overwrites existing — the latest suspension always wins.
func (k Keeper) StoreSuspension(ctx context.Context, rec types.CommitteeSuspensionRecommendation) error {
	if len(rec.NodeID) == 0 {
		return fmt.Errorf("node_id must not be empty")
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetSuspensionKey(rec.NodeID), bz)
}

// GetSuspension retrieves the active suspension for a node. Returns (nil, nil)
// if no suspension exists.
func (k Keeper) GetSuspension(ctx context.Context, nodeID []byte) (*types.CommitteeSuspensionRecommendation, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetSuspensionKey(nodeID))
	if err != nil || bz == nil {
		return nil, err
	}
	var r types.CommitteeSuspensionRecommendation
	if err := json.Unmarshal(bz, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// IsNodeSuspended returns true if the node has an active suspension covering the given epoch.
func (k Keeper) IsNodeSuspended(ctx context.Context, nodeID []byte, epoch uint64) (bool, error) {
	rec, err := k.GetSuspension(ctx, nodeID)
	if err != nil || rec == nil {
		return false, err
	}
	return epoch >= rec.SuspendFromEpoch && epoch < rec.SuspendUntilEpoch, nil
}

// ─── ExportBatch ─────────────────────────────────────────────────────────────

// IngestExportBatch ingests a full ExportBatch from the PoSeq relayer.
//
// Processing order:
//  1. Validate params (sender authorization, evidence count caps)
//  2. Store all EvidencePackets (skip duplicates, log)
//  3. Store all GovernanceEscalationRecords (skip duplicates, log)
//  4. Store CommitteeSuspensionRecommendations
//  5. Store CheckpointAnchorRecord if present (verify hash before storing)
//  6. Store EpochStateReference
//  7. Store the full ExportBatch for operator queries
func (k Keeper) IngestExportBatch(ctx context.Context, sender string, batch types.ExportBatch) error {
	params := k.GetParams(ctx)

	// Authorization check
	if params.AuthorizedSubmitter != "" && params.AuthorizedSubmitter != sender {
		return types.ErrUnauthorized.Wrapf(
			"sender %s is not the authorized submitter (%s)",
			sender, params.AuthorizedSubmitter,
		)
	}

	// Evidence cap
	if uint32(len(batch.EvidenceSet.Packets)) > params.MaxEvidencePerEpoch {
		return types.ErrInvalidExportBatch.Wrapf(
			"evidence count %d exceeds max %d",
			len(batch.EvidenceSet.Packets), params.MaxEvidencePerEpoch,
		)
	}

	// Escalation cap
	if uint32(len(batch.Escalations)) > params.MaxEscalationsPerEpoch {
		return types.ErrInvalidExportBatch.Wrapf(
			"escalation count %d exceeds max %d",
			len(batch.Escalations), params.MaxEscalationsPerEpoch,
		)
	}

	// 1. Evidence packets
	for _, pkt := range batch.EvidenceSet.Packets {
		if err := k.StoreEvidencePacket(ctx, pkt); err != nil {
			if err == types.ErrDuplicateEvidencePacket {
				k.logger.Info("skipping duplicate evidence packet",
					"epoch", batch.Epoch,
					"packet_hash", fmt.Sprintf("%x", pkt.PacketHash),
				)
				continue
			}
			return fmt.Errorf("storing evidence packet: %w", err)
		}
	}

	// 2. Governance escalations
	for _, esc := range batch.Escalations {
		if err := k.StoreEscalationRecord(ctx, esc); err != nil {
			if err == types.ErrDuplicateEscalation {
				k.logger.Info("skipping duplicate escalation",
					"epoch", batch.Epoch,
					"escalation_id", fmt.Sprintf("%x", esc.EscalationID),
				)
				continue
			}
			return fmt.Errorf("storing escalation record: %w", err)
		}
	}

	// 3. Suspensions (stored regardless of auto_apply_suspensions —
	// governance tooling reads the same records either way)
	for _, susp := range batch.Suspensions {
		if err := k.StoreSuspension(ctx, susp); err != nil {
			k.logger.Error("failed to store suspension",
				"node_id", fmt.Sprintf("%x", susp.NodeID),
				"error", err,
			)
		}
	}

	// 4. Checkpoint anchor (write-once)
	if batch.CheckpointAnchor != nil {
		if err := k.StoreCheckpointAnchor(ctx, *batch.CheckpointAnchor); err != nil {
			if err == types.ErrDuplicateCheckpointAnchor {
				k.logger.Info("checkpoint anchor already exists, skipping", "epoch", batch.Epoch)
			} else {
				return fmt.Errorf("storing checkpoint anchor: %w", err)
			}
		}
	}

	// 5. Epoch state (last-write-wins)
	if err := k.StoreEpochState(ctx, batch.EpochState); err != nil {
		return fmt.Errorf("storing epoch state: %w", err)
	}

	// 6. Full export batch (operator visibility)
	if err := k.storeExportBatch(ctx, batch); err != nil {
		k.logger.Error("failed to store full export batch", "epoch", batch.Epoch, "error", err)
		// Non-fatal: individual records already stored above
	}

	k.logger.Info("ingested PoSeq export batch",
		"epoch", batch.Epoch,
		"evidence_count", len(batch.EvidenceSet.Packets),
		"escalation_count", len(batch.Escalations),
		"suspension_count", len(batch.Suspensions),
	)
	return nil
}

// storeExportBatch stores the full ExportBatch for operator-facing epoch queries.
func (k Keeper) storeExportBatch(ctx context.Context, batch types.ExportBatch) error {
	bz, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetExportBatchKey(batch.Epoch), bz)
}

// GetExportBatch retrieves the full ExportBatch for an epoch.
func (k Keeper) GetExportBatch(ctx context.Context, epoch uint64) (*types.ExportBatch, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetExportBatchKey(epoch))
	if err != nil || bz == nil {
		return nil, err
	}
	var b types.ExportBatch
	if err := json.Unmarshal(bz, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
