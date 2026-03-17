package keeper

import (
	"context"
	"fmt"

	"pos/x/poseq/types"
)

// MsgServer implements the x/poseq message handlers.
type MsgServer struct {
	Keeper
}

func NewMsgServer(k Keeper) MsgServer {
	return MsgServer{Keeper: k}
}

// SubmitExportBatch ingests a full epoch ExportBatch from the PoSeq relayer.
func (m MsgServer) SubmitExportBatch(ctx context.Context, msg *types.MsgSubmitExportBatch) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	return m.Keeper.IngestExportBatch(ctx, msg.Sender, msg.Batch)
}

// SubmitEvidencePacket stores a single EvidencePacket directly.
func (m MsgServer) SubmitEvidencePacket(ctx context.Context, msg *types.MsgSubmitEvidencePacket) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	params := m.Keeper.GetParams(ctx)
	if params.AuthorizedSubmitter != "" && params.AuthorizedSubmitter != msg.Sender {
		return types.ErrUnauthorized.Wrapf(
			"sender %s is not the authorized submitter (%s)",
			msg.Sender, params.AuthorizedSubmitter,
		)
	}
	if err := m.Keeper.StoreEvidencePacket(ctx, msg.Packet); err != nil {
		// Duplicate is OK — idempotent
		if err == types.ErrDuplicateEvidencePacket {
			return nil
		}
		return fmt.Errorf("storing evidence packet: %w", err)
	}
	return nil
}

// SubmitCheckpointAnchor anchors a PoSeq checkpoint on-chain (write-once).
func (m MsgServer) SubmitCheckpointAnchor(ctx context.Context, msg *types.MsgSubmitCheckpointAnchor) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	params := m.Keeper.GetParams(ctx)
	if params.AuthorizedSubmitter != "" && params.AuthorizedSubmitter != msg.Sender {
		return types.ErrUnauthorized.Wrapf(
			"sender %s is not the authorized submitter",
			msg.Sender,
		)
	}
	return m.Keeper.StoreCheckpointAnchor(ctx, msg.Anchor)
}

// UpdateParams updates x/poseq governance params. Must be called by the
// governance authority address.
func (m MsgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if msg.Authority != m.Keeper.Authority() {
		return types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			m.Keeper.Authority(), msg.Authority,
		)
	}
	return m.Keeper.SetParams(ctx, msg.Params)
}
