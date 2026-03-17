package types

import sdk "github.com/cosmos/cosmos-sdk/types"

// ─── MsgSubmitExportBatch ─────────────────────────────────────────────────────

// MsgSubmitExportBatch submits a PoSeq epoch-end ExportBatch to the chain.
// The sender must be the AuthorizedSubmitter (or any address if unconfigured).
type MsgSubmitExportBatch struct {
	// Sender is the relayer account submitting the batch.
	Sender string `json:"sender"`
	// Batch is the JSON-encoded ExportBatch from Rust ChainBridgeExporter.
	Batch ExportBatch `json:"batch"`
}

func (m *MsgSubmitExportBatch) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return ErrUnauthorized.Wrapf("invalid sender address: %s", err)
	}
	if m.Batch.Epoch == 0 {
		return ErrInvalidEpoch
	}
	return nil
}

// ─── MsgSubmitEvidencePacket ──────────────────────────────────────────────────

// MsgSubmitEvidencePacket submits a single EvidencePacket directly (for
// lightweight or incremental submission without a full ExportBatch).
type MsgSubmitEvidencePacket struct {
	Sender string        `json:"sender"`
	Packet EvidencePacket `json:"packet"`
}

func (m *MsgSubmitEvidencePacket) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return ErrUnauthorized.Wrapf("invalid sender address: %s", err)
	}
	if len(m.Packet.PacketHash) != 32 {
		return ErrInvalidPacketHash.Wrap("packet_hash must be 32 bytes")
	}
	return nil
}

// ─── MsgSubmitCheckpointAnchor ────────────────────────────────────────────────

// MsgSubmitCheckpointAnchor anchors a PoSeq checkpoint on-chain.
// This is a write-once operation — anchors are immutable once stored.
type MsgSubmitCheckpointAnchor struct {
	Sender string                `json:"sender"`
	Anchor CheckpointAnchorRecord `json:"anchor"`
}

func (m *MsgSubmitCheckpointAnchor) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return ErrUnauthorized.Wrapf("invalid sender address: %s", err)
	}
	if len(m.Anchor.CheckpointID) != 32 {
		return ErrInvalidCheckpointID.Wrap("checkpoint_id must be 32 bytes")
	}
	if m.Anchor.Epoch == 0 {
		return ErrInvalidEpoch
	}
	return nil
}

// ─── MsgUpdateParams ──────────────────────────────────────────────────────────

// MsgUpdateParams is a governance-gated message to update x/poseq params.
type MsgUpdateParams struct {
	// Authority is the governance module address.
	Authority string `json:"authority"`
	Params    Params `json:"params"`
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrUnauthorized.Wrapf("invalid authority address: %s", err)
	}
	return m.Params.Validate()
}
