package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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

	// 7. Liveness events (non-fatal; update LastLivenessEpoch on sequencer record)
	for _, le := range batch.LivenessEvents {
		if err := k.StoreLivenessEvent(ctx, le); err != nil {
			k.logger.Error("failed to store liveness event",
				"epoch", batch.Epoch,
				"node_id", fmt.Sprintf("%x", le.NodeID),
				"error", err,
			)
			continue
		}
		// Update LastLivenessEpoch on the sequencer record if newer
		nodeIDHex := fmt.Sprintf("%x", le.NodeID)
		rec, err := k.GetSequencer(ctx, nodeIDHex)
		if err == nil && rec != nil && le.Epoch > rec.LastLivenessEpoch {
			rec.LastLivenessEpoch = le.Epoch
			bz, err := json.Marshal(rec)
			if err == nil {
				nodeIDBytes, _ := hex.DecodeString(rec.NodeID)
				kvStore := k.storeService.OpenKVStore(ctx)
				_ = kvStore.Set(types.GetSequencerKey(nodeIDBytes), bz)
			}
		}
	}

	// 8. Performance records (non-fatal)
	for _, pr := range batch.PerformanceRecords {
		if err := k.StorePerformanceRecord(ctx, pr); err != nil {
			k.logger.Error("failed to store performance record",
				"epoch", batch.Epoch,
				"node_id", fmt.Sprintf("%x", pr.NodeID),
				"error", err,
			)
		}
	}

	// 9. Status recommendations (conditional on AutoApplySuspensions)
	if params.AutoApplySuspensions {
		for _, rec := range batch.StatusRecommendations {
			var targetStatus types.SequencerStatus
			switch rec.RecommendedStatus {
			case "Suspended":
				targetStatus = types.SequencerStatusSuspended
			case "Jailed":
				targetStatus = types.SequencerStatusJailed
			default:
				k.logger.Error("ignoring unknown status recommendation",
					"status", rec.RecommendedStatus,
					"node_id", fmt.Sprintf("%x", rec.NodeID),
				)
				continue
			}
			nodeIDHex := fmt.Sprintf("%x", rec.NodeID)
			if err := k.SetSequencerStatus(ctx, nodeIDHex, targetStatus, rec.Epoch); err != nil {
				k.logger.Error("failed to apply status recommendation",
					"node_id", nodeIDHex,
					"target_status", targetStatus,
					"error", err,
				)
			} else {
				k.logger.Info("auto-applied status recommendation",
					"node_id", nodeIDHex,
					"status", targetStatus,
					"epoch", rec.Epoch,
					"reason", rec.Reason,
				)
			}
		}
	}

	// 10. Compute reward scores for all nodes with performance data (non-fatal)
	for _, pr := range batch.PerformanceRecords {
		nodeIDHex := fmt.Sprintf("%x", pr.NodeID)
		if _, err := k.ComputeAndStoreRewardScore(ctx, pr.Epoch, nodeIDHex); err != nil {
			k.logger.Error("failed to compute reward score",
				"node_id", nodeIDHex,
				"epoch", pr.Epoch,
				"error", err,
			)
		}
	}

	// 11. Auto-enforcement: inactivity → auto-suspend
	if params.InactivitySuspendEpochs > 0 && params.AutoApplySuspensions {
		for _, ie := range batch.InactivityEvents {
			if ie.MissedEpochs <= uint64(params.InactivitySuspendEpochs) {
				continue
			}
			nodeIDHex := fmt.Sprintf("%x", ie.NodeID)
			rec, err := k.GetSequencer(ctx, nodeIDHex)
			if err != nil || rec == nil || rec.Status != types.SequencerStatusActive {
				continue
			}
			if err := k.SetSequencerStatus(ctx, nodeIDHex, types.SequencerStatusSuspended, batch.Epoch); err != nil {
				k.logger.Error("auto-suspend failed", "node_id", nodeIDHex, "error", err)
			} else {
				k.logger.Info("auto-suspended inactive node",
					"node_id", nodeIDHex, "missed_epochs", ie.MissedEpochs)
			}
		}
	}

	// 12. Auto-enforcement: fault threshold → auto-jail
	if params.FaultJailThreshold > 0 && params.AutoApplySuspensions {
		for _, pr := range batch.PerformanceRecords {
			if pr.FaultEvents < uint64(params.FaultJailThreshold) {
				continue
			}
			nodeIDHex := fmt.Sprintf("%x", pr.NodeID)
			rec, err := k.GetSequencer(ctx, nodeIDHex)
			if err != nil || rec == nil {
				continue
			}
			if rec.Status == types.SequencerStatusJailed || rec.Status == types.SequencerStatusRetired {
				continue
			}
			if err := k.SetSequencerStatus(ctx, nodeIDHex, types.SequencerStatusJailed, batch.Epoch); err != nil {
				k.logger.Error("auto-jail failed", "node_id", nodeIDHex, "error", err)
			} else {
				k.logger.Info("auto-jailed node for fault threshold",
					"node_id", nodeIDHex, "fault_events", pr.FaultEvents)
			}
		}
	}

	// 13. Slash queue: enqueue Critical evidence packets
	for _, pkt := range batch.EvidenceSet.Packets {
		if pkt.Severity != types.SeverityCritical {
			continue
		}
		if pkt.ProposedSlashBps == 0 {
			continue
		}
		nodeIDHex := fmt.Sprintf("%x", pkt.OffenderNodeID)

		// Look up operator address from bond index (best-effort)
		operatorAddr := ""
		if bond, bondErr := k.GetActiveBondForNode(ctx, nodeIDHex); bondErr == nil && bond != nil {
			operatorAddr = bond.OperatorAddress
		}

		entryID := computeSlashEntryID(operatorAddr, nodeIDHex, batch.Epoch)
		entry := types.SlashQueueEntry{
			EntryID:         entryID,
			OperatorAddress: operatorAddr,
			NodeID:          nodeIDHex,
			EvidenceRef:     pkt.PacketHash,
			Severity:        string(pkt.Severity),
			SlashBps:        pkt.ProposedSlashBps,
			Epoch:           batch.Epoch,
			Reason:          fmt.Sprintf("Critical misbehavior: %s", pkt.Kind),
			Executed:        false,
		}
		if err := k.EnqueueSlashEntry(ctx, entry); err != nil {
			k.logger.Error("failed to enqueue slash entry",
				"node_id", nodeIDHex,
				"epoch", batch.Epoch,
				"error", err,
			)
		}
	}

	k.logger.Info("ingested PoSeq export batch",
		"epoch", batch.Epoch,
		"evidence_count", len(batch.EvidenceSet.Packets),
		"escalation_count", len(batch.Escalations),
		"suspension_count", len(batch.Suspensions),
		"liveness_events", len(batch.LivenessEvents),
		"performance_records", len(batch.PerformanceRecords),
		"status_recommendations", len(batch.StatusRecommendations),
		"inactivity_events", len(batch.InactivityEvents),
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

// ─── Export Batch Dedup ───────────────────────────────────────────────────────

// getExportBatchDedup returns the dedup record for an epoch, or nil if unseen.
func (k Keeper) getExportBatchDedup(ctx context.Context, epoch uint64) (*types.ExportBatchDedupRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetExportBatchDedupKey(epoch))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.ExportBatchDedupRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// setExportBatchDedup persists a dedup record for an epoch.
func (k Keeper) setExportBatchDedup(ctx context.Context, rec types.ExportBatchDedupRecord) error {
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetExportBatchDedupKey(rec.Epoch), bz)
}

// ─── ACK Computation ──────────────────────────────────────────────────────────

// ComputeBridgeBatchID returns SHA256("bridge:epoch:" | epoch_be).
// This matches the Rust-side batch_id derivation in node_runner.rs.
func ComputeBridgeBatchID(epoch uint64) []byte {
	h := sha256.New()
	h.Write([]byte("bridge:epoch:"))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], epoch)
	h.Write(buf[:])
	return h.Sum(nil)
}

// computeAckHash returns SHA256("ack" | epoch_be | status_bytes).
func computeAckHash(epoch uint64, status types.AckStatus) []byte {
	h := sha256.New()
	h.Write([]byte("ack"))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], epoch)
	h.Write(buf[:])
	h.Write([]byte(status))
	return h.Sum(nil)
}

// BuildExportBatchAck constructs a fully populated ACK artifact.
func BuildExportBatchAck(epoch uint64, status types.AckStatus, reason string, blockHeight int64) types.ExportBatchAck {
	return types.ExportBatchAck{
		SchemaVersion: 1,
		Epoch:         epoch,
		BatchID:       ComputeBridgeBatchID(epoch),
		AckHash:       computeAckHash(epoch, status),
		Status:        status,
		Reason:        reason,
		BlockHeight:   blockHeight,
	}
}

// ─── Idempotent Ingestion with ACK ────────────────────────────────────────────

// IngestExportBatchWithAck wraps IngestExportBatch with dedup and ACK generation.
//
// Returns the ACK to be written to the bridge return path. If the epoch was
// already ingested, returns a "duplicate" ACK without re-processing. If ingestion
// fails validation, returns a "rejected" ACK. Otherwise processes the batch and
// returns an "accepted" ACK.
func (k Keeper) IngestExportBatchWithAck(ctx context.Context, sender string, batch types.ExportBatch, blockHeight int64) (types.ExportBatchAck, error) {
	// Epoch validation
	if batch.Epoch == 0 {
		ack := BuildExportBatchAck(0, types.AckStatusRejected, "epoch must be > 0", blockHeight)
		k.logger.Error("bridge.export.rejected", "epoch", batch.Epoch, "reason", "invalid epoch")
		return ack, types.ErrInvalidEpoch
	}

	// Dedup check
	existing, err := k.getExportBatchDedup(ctx, batch.Epoch)
	if err != nil {
		return types.ExportBatchAck{}, fmt.Errorf("checking dedup: %w", err)
	}
	if existing != nil {
		ack := BuildExportBatchAck(batch.Epoch, types.AckStatusDuplicate, "already ingested", blockHeight)
		k.logger.Info("bridge.export.duplicate",
			"epoch", batch.Epoch,
			"original_status", existing.Status,
		)
		return ack, nil
	}

	// Ingest
	if err := k.IngestExportBatch(ctx, sender, batch); err != nil {
		ack := BuildExportBatchAck(batch.Epoch, types.AckStatusRejected, err.Error(), blockHeight)

		// Persist rejected dedup record so we don't retry a bad batch
		_ = k.setExportBatchDedup(ctx, types.ExportBatchDedupRecord{
			Epoch:      batch.Epoch,
			Status:     types.AckStatusRejected,
			AckHash:    ack.AckHash,
			IngestedAt: blockHeight,
		})

		k.logger.Error("bridge.export.rejected",
			"epoch", batch.Epoch,
			"error", err,
		)
		return ack, err
	}

	// Success — persist accepted dedup record
	ack := BuildExportBatchAck(batch.Epoch, types.AckStatusAccepted, "", blockHeight)
	if err := k.setExportBatchDedup(ctx, types.ExportBatchDedupRecord{
		Epoch:      batch.Epoch,
		Status:     types.AckStatusAccepted,
		AckHash:    ack.AckHash,
		IngestedAt: blockHeight,
	}); err != nil {
		k.logger.Error("bridge.dedup.persist_failed", "epoch", batch.Epoch, "error", err)
	}

	k.logger.Info("bridge.export.accepted", "epoch", batch.Epoch)
	return ack, nil
}
