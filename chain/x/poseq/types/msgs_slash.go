package types

import (
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── SlashRecord ──────────────────────────────────────────────────────────────

// SlashRecord is stored on-chain after a governance-ratified slash is executed.
type SlashRecord struct {
	// NodeID is the hex-encoded 32-byte ID of the slashed sequencer.
	NodeID string `json:"node_id"`

	// PacketHash is the hex-encoded 32-byte EvidencePacket hash that triggered this slash.
	PacketHash string `json:"packet_hash"`

	// SlashBps is the actual slash in basis points (1 bps = 0.01%) that was applied.
	SlashBps uint32 `json:"slash_bps"`

	// Epoch at which the misbehavior occurred.
	Epoch uint64 `json:"epoch"`

	// ExecutedByAddress is the bech32 address of the governance executor.
	ExecutedByAddress string `json:"executed_by_address"`

	// Reason is a human-readable audit trail entry.
	Reason string `json:"reason"`
}

// ─── MsgExecuteSlash ──────────────────────────────────────────────────────────

// MsgExecuteSlash executes a governance-ratified slash against a registered sequencer.
//
// This message is gated: only the governance authority address may submit it.
// It does NOT call into x/staking slashing directly (PoSeq nodes are not
// necessarily Cosmos validators). Instead it:
//   1. Verifies the EvidencePacket exists in the store.
//   2. Deactivates the sequencer in the registry.
//   3. Records the slash in the SlashRecord store.
//   4. Emits a SlashExecuted event for off-chain listeners.
//
// Actual token slashing (if desired) must be implemented as a separate
// governance proposal that calls x/staking's Slash or through a custom
// bank.SendCoins from the sequencer's bond module account.
type MsgExecuteSlash struct {
	// Authority is the governance module address.
	Authority string `json:"authority"`

	// NodeID is the hex-encoded 32-byte node ID of the sequencer to slash.
	NodeID string `json:"node_id"`

	// PacketHash is the hex-encoded 32-byte hash of the EvidencePacket
	// that provides the justification for this slash.
	PacketHash string `json:"packet_hash"`

	// SlashBps is the slash in basis points to record (1–10000).
	// The governance proposal sets this after reviewing the EvidencePacket.
	SlashBps uint32 `json:"slash_bps"`

	// Reason is a human-readable audit trail entry (≤ 256 chars).
	Reason string `json:"reason"`

	// CurrentEpoch is the PoSeq epoch at time of execution.
	// Used for staleness checks and AdjudicationRecord stamping.
	// Zero means unknown (staleness check is skipped).
	CurrentEpoch uint64 `json:"current_epoch,omitempty"`
}

func (m *MsgExecuteSlash) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrUnauthorized.Wrapf("invalid authority address: %s", err)
	}
	nodeIDBytes, err := hex.DecodeString(m.NodeID)
	if err != nil || len(nodeIDBytes) != 32 {
		return ErrInvalidNodeID.Wrap("node_id must be 64 hex chars (32 bytes)")
	}
	pktHashBytes, err := hex.DecodeString(m.PacketHash)
	if err != nil || len(pktHashBytes) != 32 {
		return ErrInvalidPacketHash.Wrap("packet_hash must be 64 hex chars (32 bytes)")
	}
	if m.SlashBps == 0 || m.SlashBps > 10000 {
		return ErrInvalidExportBatch.Wrap("slash_bps must be 1–10000")
	}
	if len(m.Reason) > 256 {
		return ErrInvalidMoniker.Wrap("reason exceeds 256 characters")
	}
	return nil
}
