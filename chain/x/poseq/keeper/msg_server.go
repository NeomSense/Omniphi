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

// ─── Phase 5: Bond message handlers ──────────────────────────────────────────

// DeclareOperatorBond processes MsgDeclareOperatorBond.
// Permissionless — any operator may declare a bond for a registered sequencer.
// The sender's address is used as the OperatorAddress; it must match the
// OperatorAddress on the registered SequencerRecord.
func (m MsgServer) DeclareOperatorBond(ctx context.Context, msg *types.MsgDeclareOperatorBond) error {
	if msg.BondAmount == 0 {
		return types.ErrInvalidBondAmount
	}
	if msg.NodeID == "" {
		return types.ErrInvalidNodeID.Wrap("node_id must not be empty")
	}

	// Verify sender is the registered operator for this node
	seq, err := m.Keeper.GetSequencer(ctx, msg.NodeID)
	if err != nil {
		return fmt.Errorf("fetching sequencer: %w", err)
	}
	if seq == nil {
		return types.ErrSequencerNotFound
	}
	if seq.OperatorAddress != msg.OperatorAddress {
		return types.ErrOperatorMismatch.Wrapf(
			"sender %s is not the registered operator %s for node %s",
			msg.OperatorAddress, seq.OperatorAddress, msg.NodeID,
		)
	}

	bond := types.OperatorBond{
		OperatorAddress:  msg.OperatorAddress,
		NodeID:           msg.NodeID,
		BondAmount:       msg.BondAmount,
		BondDenom:        msg.BondDenom,
		BondedSinceEpoch: msg.Epoch,
		IsActive:         true,
	}
	if err := m.Keeper.DeclareOperatorBond(ctx, bond); err != nil {
		return fmt.Errorf("declaring operator bond: %w", err)
	}

	m.Keeper.Logger().Info("operator bond declared",
		"operator", msg.OperatorAddress,
		"node_id", msg.NodeID,
		"amount", msg.BondAmount,
		"denom", msg.BondDenom,
		"epoch", msg.Epoch,
	)
	return nil
}

// WithdrawOperatorBond processes MsgWithdrawOperatorBond.
// Only the registered operator may withdraw their bond declaration.
func (m MsgServer) WithdrawOperatorBond(ctx context.Context, msg *types.MsgWithdrawOperatorBond) error {
	if msg.NodeID == "" {
		return types.ErrInvalidNodeID.Wrap("node_id must not be empty")
	}

	// Verify sender is the registered operator
	seq, err := m.Keeper.GetSequencer(ctx, msg.NodeID)
	if err != nil {
		return fmt.Errorf("fetching sequencer: %w", err)
	}
	if seq == nil {
		return types.ErrSequencerNotFound
	}
	if seq.OperatorAddress != msg.OperatorAddress {
		return types.ErrOperatorMismatch.Wrapf(
			"sender %s is not the registered operator %s for node %s",
			msg.OperatorAddress, seq.OperatorAddress, msg.NodeID,
		)
	}

	if err := m.Keeper.WithdrawOperatorBond(ctx, msg.OperatorAddress, msg.NodeID, msg.Epoch); err != nil {
		return fmt.Errorf("withdrawing operator bond: %w", err)
	}

	m.Keeper.Logger().Info("operator bond withdrawn",
		"operator", msg.OperatorAddress,
		"node_id", msg.NodeID,
		"epoch", msg.Epoch,
	)
	return nil
}
