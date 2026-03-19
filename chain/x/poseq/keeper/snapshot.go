package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poseq/types"
)

// ─── CommitteeSnapshot store ──────────────────────────────────────────────────

// StoreCommitteeSnapshot stores a committee snapshot. Write-once per epoch —
// returns ErrDuplicateSnapshot if a snapshot already exists for this epoch.
// Verifies the snapshot hash before writing.
func (k Keeper) StoreCommitteeSnapshot(ctx context.Context, snap types.CommitteeSnapshot) error {
	if snap.Epoch == 0 {
		return types.ErrInvalidSnapshotEpoch
	}
	if len(snap.SnapshotHash) != 32 {
		return types.ErrSnapshotHashMismatch.Wrap("snapshot_hash must be 32 bytes")
	}
	if !verifyCommitteeSnapshotHash(snap) {
		return types.ErrSnapshotHashMismatch
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetCommitteeSnapshotKey(snap.Epoch)
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		return types.ErrDuplicateSnapshot.Wrapf("epoch %d", snap.Epoch)
	}
	bz, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return kvStore.Set(key, bz)
}

// GetCommitteeSnapshot retrieves a committee snapshot by epoch.
// Returns (nil, nil) if not found.
func (k Keeper) GetCommitteeSnapshot(ctx context.Context, epoch uint64) (*types.CommitteeSnapshot, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetCommitteeSnapshotKey(epoch))
	if err != nil || bz == nil {
		return nil, err
	}
	var snap types.CommitteeSnapshot
	if err := json.Unmarshal(bz, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// BuildCommitteeSnapshot constructs a deterministic CommitteeSnapshot from all
// currently Active sequencers that meet economic admission criteria.
//
// Admission filters (applied when the respective param is non-zero):
//   - MinBondForCommittee: AvailableBond must be >= threshold
//   - MinParticipationBps: participation_rate_bps from prior epoch must be >= threshold
//
// Members are sorted by NodeID bytes for determinism.
// Returns an error if there are no admissible active sequencers.
func (k Keeper) BuildCommitteeSnapshot(ctx context.Context, epoch uint64, blockHeight int64) (types.CommitteeSnapshot, error) {
	if epoch == 0 {
		return types.CommitteeSnapshot{}, types.ErrInvalidSnapshotEpoch
	}

	allSeqs, err := k.AllSequencers(ctx)
	if err != nil {
		return types.CommitteeSnapshot{}, fmt.Errorf("loading sequencers: %w", err)
	}

	var members []types.CommitteeSnapshotMember
	for _, rec := range allSeqs {
		if !rec.IsActive() {
			continue
		}
		// Apply economic admission filters
		if ok, reason := k.IsAdmissibleForCommittee(ctx, rec.NodeID, epoch); !ok {
			k.logger.Info("sequencer excluded from committee snapshot",
				"node_id", rec.NodeID,
				"epoch", epoch,
				"reason", reason,
			)
			continue
		}
		members = append(members, types.CommitteeSnapshotMember{
			NodeID:    rec.NodeID,
			PublicKey: rec.PublicKey,
			Moniker:   rec.Moniker,
			Role:      "Sequencer",
		})
	}
	if len(members) == 0 {
		return types.CommitteeSnapshot{}, fmt.Errorf("no admissible active sequencers for epoch %d", epoch)
	}

	// Sort by NodeID bytes for determinism
	sort.Slice(members, func(i, j int) bool {
		bi, _ := hex.DecodeString(members[i].NodeID)
		bj, _ := hex.DecodeString(members[j].NodeID)
		return lessBytes(bi, bj)
	})

	snap := types.CommitteeSnapshot{
		Epoch:           epoch,
		Members:         members,
		ProducedAtBlock: blockHeight,
	}

	hash, err := computeCommitteeSnapshotHash(snap)
	if err != nil {
		return types.CommitteeSnapshot{}, fmt.Errorf("computing snapshot hash: %w", err)
	}
	snap.SnapshotHash = hash
	return snap, nil
}

// AllSequencers returns all registered sequencer records via a prefix scan.
func (k Keeper) AllSequencers(ctx context.Context) ([]types.SequencerRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	startKey := []byte{types.KeyPrefixSequencer}
	endKey := []byte{types.KeyPrefixSequencer + 1}
	iter, err := kvStore.Iterator(startKey, endKey)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var records []types.SequencerRecord
	for ; iter.Valid(); iter.Next() {
		var rec types.SequencerRecord
		if err := json.Unmarshal(iter.Value(), &rec); err != nil {
			k.logger.Error("failed to unmarshal sequencer record", "error", err)
			continue
		}
		records = append(records, rec)
	}
	return records, iter.Close()
}

// ─── MsgServer handler ────────────────────────────────────────────────────────

// SubmitCommitteeSnapshot processes MsgSubmitCommitteeSnapshot.
// Governance authority only. Verifies hash, then stores write-once.
func (m MsgServer) SubmitCommitteeSnapshot(ctx context.Context, msg *types.MsgSubmitCommitteeSnapshot) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if msg.Authority != m.Keeper.Authority() {
		return types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			m.Keeper.Authority(), msg.Authority,
		)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if msg.Snapshot.ProducedAtBlock == 0 {
		msg.Snapshot.ProducedAtBlock = sdkCtx.BlockHeight()
	}

	if err := m.Keeper.StoreCommitteeSnapshot(ctx, msg.Snapshot); err != nil {
		return fmt.Errorf("storing committee snapshot: %w", err)
	}
	m.Keeper.Logger().Info("committee snapshot stored",
		"epoch", msg.Snapshot.Epoch,
		"members", len(msg.Snapshot.Members),
		"block", msg.Snapshot.ProducedAtBlock,
	)
	return nil
}
