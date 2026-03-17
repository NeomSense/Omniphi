package keeper_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poseq/keeper"
	"pos/x/poseq/types"
)

// ─── Test helpers ─────────────────────────────────────────────────────────────

func makeKeeper(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	ir := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(ir)

	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(cdc, storeService, log.NewNopLogger(), sdk.AccAddress("authority___________").String())

	ctx := sdk.NewContext(stateStore, tmproto.Header{Height: 1, Time: time.Now()}, false, log.NewNopLogger())
	return k, ctx
}

func nodeID(b byte) []byte {
	id := make([]byte, 32)
	id[0] = b
	return id
}

func hash32(b byte) []byte {
	h := make([]byte, 32)
	h[0] = b
	return h
}

// makeEvidencePacket builds a packet with a valid packet_hash.
// Mirrors Rust: SHA256("equivocation" | node_id | epoch_be | sorted_evidence_hashes)
func makeEvidencePacket(nodeIDFirst byte, epoch uint64) types.EvidencePacket {
	nid := nodeID(nodeIDFirst)
	evidenceHash := hash32(0xAB)

	h := sha256.New()
	h.Write([]byte("equivocation"))
	h.Write(nid)
	epochBE := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBE, epoch)
	h.Write(epochBE)
	h.Write(evidenceHash)
	packetHash := h.Sum(nil)

	return types.EvidencePacket{
		PacketHash:          packetHash,
		Kind:                types.EvidenceKindEquivocation,
		OffenderNodeID:      nid,
		Epoch:               epoch,
		Slot:                5,
		Severity:            types.SeverityCritical,
		ProposedSlashBps:    500,
		EvidenceHashes:      [][]byte{evidenceHash},
		RequiresGovernance:  true,
		RecommendSuspension: true,
	}
}

// makeCheckpointAnchor builds an anchor with a valid anchor_hash.
// Mirrors Rust: SHA256("ckpt" | checkpoint_id | epoch_be | epoch_state_hash | bridge_state_hash)
func makeCheckpointAnchor(epoch, slot uint64) types.CheckpointAnchorRecord {
	checkpointID := hash32(byte(epoch))
	epochStateHash := hash32(0xDD)
	bridgeStateHash := hash32(0xEE)

	h := sha256.New()
	h.Write([]byte("ckpt"))
	h.Write(checkpointID)
	epochBE := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBE, epoch)
	h.Write(epochBE)
	h.Write(epochStateHash)
	h.Write(bridgeStateHash)
	anchorHash := h.Sum(nil)

	return types.CheckpointAnchorRecord{
		CheckpointID:    checkpointID,
		Epoch:           epoch,
		Slot:            slot,
		EpochStateHash:  epochStateHash,
		BridgeStateHash: bridgeStateHash,
		MisbehaviorCount: 2,
		FinalitySummary: types.BatchFinalityReference{
			BatchID:          hash32(0x01),
			Slot:             slot,
			Epoch:            epoch,
			FinalizationHash: hash32(0x02),
			SubmissionCount:  5,
			QuorumApprovals:  3,
			CommitteeSize:    4,
		},
		AnchorHash: anchorHash,
	}
}

// ─── Params tests ─────────────────────────────────────────────────────────────

func TestGetSetParams(t *testing.T) {
	k, ctx := makeKeeper(t)

	// Default params
	p := k.GetParams(ctx)
	require.Equal(t, types.DefaultParams(), p)

	// Update
	p.AuthorizedSubmitter = sdk.AccAddress("relayer______________").String()
	p.AutoApplySuspensions = true
	require.NoError(t, k.SetParams(ctx, p))

	got := k.GetParams(ctx)
	require.Equal(t, p.AuthorizedSubmitter, got.AuthorizedSubmitter)
	require.True(t, got.AutoApplySuspensions)
}

// ─── EvidencePacket tests ─────────────────────────────────────────────────────

func TestStoreAndGetEvidencePacket(t *testing.T) {
	k, ctx := makeKeeper(t)
	pkt := makeEvidencePacket(0x01, 5)

	require.NoError(t, k.StoreEvidencePacket(ctx, pkt))

	got, err := k.GetEvidencePacket(ctx, pkt.PacketHash)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, pkt.PacketHash, got.PacketHash)
	require.Equal(t, types.EvidenceKindEquivocation, got.Kind)
}

func TestStoreEvidencePacket_Duplicate(t *testing.T) {
	k, ctx := makeKeeper(t)
	pkt := makeEvidencePacket(0x01, 5)

	require.NoError(t, k.StoreEvidencePacket(ctx, pkt))
	err := k.StoreEvidencePacket(ctx, pkt)
	require.ErrorIs(t, err, types.ErrDuplicateEvidencePacket)
}

func TestGetEvidencePacket_NotFound(t *testing.T) {
	k, ctx := makeKeeper(t)
	got, err := k.GetEvidencePacket(ctx, hash32(0xFF))
	require.NoError(t, err)
	require.Nil(t, got)
}

// ─── EscalationRecord tests ───────────────────────────────────────────────────

func makeEscalationRecord(epoch uint64) types.GovernanceEscalationRecord {
	pkt := makeEvidencePacket(0x02, epoch)

	h := sha256.New()
	h.Write([]byte("esc"))
	h.Write(pkt.OffenderNodeID)
	epochBE := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBE, epoch)
	h.Write(epochBE)
	h.Write(pkt.PacketHash)
	escalationID := h.Sum(nil)

	return types.GovernanceEscalationRecord{
		EscalationID:           escalationID,
		OffenderNodeID:         pkt.OffenderNodeID,
		EvidencePacketHash:     pkt.PacketHash,
		Epoch:                  epoch,
		Severity:               types.EscalationSeverityCritical,
		RecommendedAction:      types.EscalationAction{Tag: "SuspendFromCommittee", Epochs: ptr(uint64(5))},
		Rationale:              "test escalation",
		BlockPendingGovernance: true,
	}
}

func ptr[T any](v T) *T { return &v }

func TestStoreAndGetEscalationRecord(t *testing.T) {
	k, ctx := makeKeeper(t)
	rec := makeEscalationRecord(3)

	require.NoError(t, k.StoreEscalationRecord(ctx, rec))

	got, err := k.GetEscalationRecord(ctx, rec.EscalationID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, rec.EscalationID, got.EscalationID)
	require.Equal(t, types.EscalationSeverityCritical, got.Severity)
}

func TestStoreEscalationRecord_Duplicate(t *testing.T) {
	k, ctx := makeKeeper(t)
	rec := makeEscalationRecord(3)

	require.NoError(t, k.StoreEscalationRecord(ctx, rec))
	err := k.StoreEscalationRecord(ctx, rec)
	require.ErrorIs(t, err, types.ErrDuplicateEscalation)
}

// ─── CheckpointAnchor tests ───────────────────────────────────────────────────

func TestStoreAndGetCheckpointAnchor(t *testing.T) {
	k, ctx := makeKeeper(t)
	anchor := makeCheckpointAnchor(5, 10)

	require.NoError(t, k.StoreCheckpointAnchor(ctx, anchor))

	got, err := k.GetCheckpointAnchor(ctx, 5, 10)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, anchor.CheckpointID, got.CheckpointID)
	require.Equal(t, uint64(5), got.Epoch)
}

func TestStoreCheckpointAnchor_Duplicate(t *testing.T) {
	k, ctx := makeKeeper(t)
	anchor := makeCheckpointAnchor(5, 10)

	require.NoError(t, k.StoreCheckpointAnchor(ctx, anchor))
	err := k.StoreCheckpointAnchor(ctx, anchor)
	require.ErrorIs(t, err, types.ErrDuplicateCheckpointAnchor)
}

func TestStoreCheckpointAnchor_TamperedHash(t *testing.T) {
	k, ctx := makeKeeper(t)
	anchor := makeCheckpointAnchor(5, 10)
	anchor.Epoch = 99 // tamper

	err := k.StoreCheckpointAnchor(ctx, anchor)
	require.ErrorIs(t, err, types.ErrCheckpointAnchorTampered)
}

// ─── EpochState tests ─────────────────────────────────────────────────────────

func TestStoreAndGetEpochState(t *testing.T) {
	k, ctx := makeKeeper(t)
	state := types.EpochStateReference{
		Epoch:               5,
		CommitteeHash:       hash32(0xCC),
		FinalizedBatchCount: 20,
		MisbehaviorCount:    3,
		EvidencePacketCount: 3,
		GovernanceEscalations: 1,
		EpochStateHash:      hash32(0xEE),
	}

	require.NoError(t, k.StoreEpochState(ctx, state))

	got, err := k.GetEpochState(ctx, 5)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, uint64(5), got.Epoch)
	require.Equal(t, uint64(20), got.FinalizedBatchCount)
}

// ─── Suspension tests ─────────────────────────────────────────────────────────

func TestStoreAndGetSuspension(t *testing.T) {
	k, ctx := makeKeeper(t)
	rec := types.CommitteeSuspensionRecommendation{
		NodeID:             nodeID(0x07),
		SuspendFromEpoch:   3,
		SuspendUntilEpoch:  8,
		EvidencePacketHash: hash32(0xAB),
		Reason:             "equivocation",
	}

	require.NoError(t, k.StoreSuspension(ctx, rec))

	got, err := k.GetSuspension(ctx, rec.NodeID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, uint64(3), got.SuspendFromEpoch)
	require.Equal(t, uint64(8), got.SuspendUntilEpoch)
}

func TestIsNodeSuspended(t *testing.T) {
	k, ctx := makeKeeper(t)
	rec := types.CommitteeSuspensionRecommendation{
		NodeID:            nodeID(0x07),
		SuspendFromEpoch:  3,
		SuspendUntilEpoch: 8,
	}
	require.NoError(t, k.StoreSuspension(ctx, rec))

	suspended, err := k.IsNodeSuspended(ctx, rec.NodeID, 5)
	require.NoError(t, err)
	require.True(t, suspended)

	suspended, err = k.IsNodeSuspended(ctx, rec.NodeID, 2)
	require.NoError(t, err)
	require.False(t, suspended)

	suspended, err = k.IsNodeSuspended(ctx, rec.NodeID, 8) // end is exclusive
	require.NoError(t, err)
	require.False(t, suspended)
}

// ─── IngestExportBatch tests ──────────────────────────────────────────────────

func makeExportBatch(epoch uint64) types.ExportBatch {
	pkt1 := makeEvidencePacket(0x01, epoch)
	pkt2 := makeEvidencePacket(0x02, epoch)
	anchor := makeCheckpointAnchor(epoch, 10)
	esc := makeEscalationRecord(epoch)

	return types.ExportBatch{
		Epoch: epoch,
		EvidenceSet: types.EvidencePacketSet{
			Epoch:   epoch,
			Packets: []types.EvidencePacket{pkt1, pkt2},
		},
		Escalations: []types.GovernanceEscalationRecord{esc},
		Suspensions: []types.CommitteeSuspensionRecommendation{
			{
				NodeID:             nodeID(0x01),
				SuspendFromEpoch:   epoch,
				SuspendUntilEpoch:  epoch + 5,
				EvidencePacketHash: pkt1.PacketHash,
				Reason:             "equivocation",
			},
		},
		CheckpointAnchor: &anchor,
		EpochState: types.EpochStateReference{
			Epoch:               epoch,
			CommitteeHash:       hash32(0xCC),
			FinalizedBatchCount: 10,
			MisbehaviorCount:    2,
			EvidencePacketCount: 2,
			GovernanceEscalations: 1,
			EpochStateHash:      hash32(0xEE),
		},
	}
}

func TestIngestExportBatch_Success(t *testing.T) {
	k, ctx := makeKeeper(t)
	batch := makeExportBatch(7)
	sender := sdk.AccAddress("relayer______________").String()

	require.NoError(t, k.IngestExportBatch(ctx, sender, batch))

	// Evidence stored
	pkt := batch.EvidenceSet.Packets[0]
	got, err := k.GetEvidencePacket(ctx, pkt.PacketHash)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Escalation stored
	esc := batch.Escalations[0]
	gotEsc, err := k.GetEscalationRecord(ctx, esc.EscalationID)
	require.NoError(t, err)
	require.NotNil(t, gotEsc)

	// Checkpoint stored
	gotAnchor, err := k.GetCheckpointAnchor(ctx, 7, 10)
	require.NoError(t, err)
	require.NotNil(t, gotAnchor)

	// Epoch state stored
	gotState, err := k.GetEpochState(ctx, 7)
	require.NoError(t, err)
	require.NotNil(t, gotState)
	require.Equal(t, uint64(10), gotState.FinalizedBatchCount)

	// Full export batch queryable
	gotBatch, err := k.GetExportBatch(ctx, 7)
	require.NoError(t, err)
	require.NotNil(t, gotBatch)
	require.Equal(t, uint64(7), gotBatch.Epoch)
}

func TestIngestExportBatch_Idempotent(t *testing.T) {
	k, ctx := makeKeeper(t)
	batch := makeExportBatch(8)
	sender := sdk.AccAddress("relayer______________").String()

	require.NoError(t, k.IngestExportBatch(ctx, sender, batch))
	// Second ingest should succeed (duplicates are skipped, not errors)
	require.NoError(t, k.IngestExportBatch(ctx, sender, batch))
}

func TestIngestExportBatch_AuthCheck(t *testing.T) {
	k, ctx := makeKeeper(t)

	// Configure authorized submitter
	p := types.DefaultParams()
	p.AuthorizedSubmitter = sdk.AccAddress("authorizedrelayer____").String()
	require.NoError(t, k.SetParams(ctx, p))

	batch := makeExportBatch(9)

	// Wrong sender
	err := k.IngestExportBatch(ctx, sdk.AccAddress("wrongsender__________").String(), batch)
	require.ErrorIs(t, err, types.ErrUnauthorized)

	// Correct sender
	require.NoError(t, k.IngestExportBatch(ctx, p.AuthorizedSubmitter, batch))
}

func TestIngestExportBatch_EvidenceCap(t *testing.T) {
	k, ctx := makeKeeper(t)

	p := types.DefaultParams()
	p.MaxEvidencePerEpoch = 1
	require.NoError(t, k.SetParams(ctx, p))

	batch := makeExportBatch(10)
	// batch has 2 packets, cap is 1

	err := k.IngestExportBatch(ctx, sdk.AccAddress("relayer______________").String(), batch)
	require.ErrorIs(t, err, types.ErrInvalidExportBatch)
}
