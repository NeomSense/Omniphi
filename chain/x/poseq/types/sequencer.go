package types

import (
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── SequencerRecord ──────────────────────────────────────────────────────────

// SequencerRecord is the on-chain record for a registered PoSeq sequencer.
// Keyed by NodeID (hex-encoded 32-byte Ed25519 public key hash).
type SequencerRecord struct {
	// NodeID is the 32-byte node identity (hex-encoded).
	NodeID string `json:"node_id"`

	// PublicKey is the raw Ed25519 public key (hex-encoded, 32 bytes).
	PublicKey string `json:"public_key"`

	// Moniker is a human-readable operator label (≤ 64 chars).
	Moniker string `json:"moniker"`

	// OperatorAddress is the Cosmos bech32 address of the operator.
	OperatorAddress string `json:"operator_address"`

	// RegisteredEpoch is the PoSeq epoch at time of registration.
	RegisteredEpoch uint64 `json:"registered_epoch"`

	// IsActive indicates the sequencer is eligible to participate in consensus.
	// Set to true by governance after bonding requirements are met.
	IsActive bool `json:"is_active"`
}

// NodeIDBytes decodes the hex NodeID into raw bytes.
func (r SequencerRecord) NodeIDBytes() ([]byte, error) {
	return hex.DecodeString(r.NodeID)
}

// ─── MsgRegisterSequencer ─────────────────────────────────────────────────────

// MsgRegisterSequencer registers a new PoSeq sequencer node on-chain.
// The sender must be the operator who will control this sequencer.
//
// Registration is permissionless — any operator can register. Activation
// (setting IsActive = true) requires a governance proposal or the governance
// authority to call MsgActivateSequencer.
type MsgRegisterSequencer struct {
	// Sender is the operator's bech32 address. Must match OperatorAddress.
	Sender string `json:"sender"`

	// NodeID is the 32-byte node identity, hex-encoded.
	// Must be unique in the registry.
	NodeID string `json:"node_id"`

	// PublicKey is the Ed25519 public key the sequencer will use for signing
	// PoSeq wire messages, hex-encoded (32 bytes).
	PublicKey string `json:"public_key"`

	// Moniker is a human-readable label for this sequencer (≤ 64 chars).
	Moniker string `json:"moniker"`

	// Epoch is the current PoSeq epoch at registration time (informational;
	// used to populate RegisteredEpoch in the stored record).
	Epoch uint64 `json:"epoch"`
}

func (m *MsgRegisterSequencer) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return ErrUnauthorized.Wrapf("invalid sender address: %s", err)
	}
	nodeIDBytes, err := hex.DecodeString(m.NodeID)
	if err != nil || len(nodeIDBytes) != 32 {
		return ErrInvalidNodeID.Wrap("node_id must be 64 hex chars (32 bytes)")
	}
	pkBytes, err := hex.DecodeString(m.PublicKey)
	if err != nil || len(pkBytes) != 32 {
		return ErrInvalidPublicKey.Wrap("public_key must be 64 hex chars (32 bytes)")
	}
	if len(m.Moniker) == 0 || len(m.Moniker) > 64 {
		return ErrInvalidMoniker.Wrap("moniker must be 1–64 characters")
	}
	return nil
}

// ─── MsgActivateSequencer ─────────────────────────────────────────────────────

// MsgActivateSequencer sets IsActive = true for an already-registered sequencer.
// Must be sent by the governance authority address.
type MsgActivateSequencer struct {
	// Authority is the governance module address.
	Authority string `json:"authority"`
	// NodeID identifies the sequencer to activate (64 hex chars).
	NodeID string `json:"node_id"`
}

func (m *MsgActivateSequencer) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrUnauthorized.Wrapf("invalid authority address: %s", err)
	}
	nodeIDBytes, err := hex.DecodeString(m.NodeID)
	if err != nil || len(nodeIDBytes) != 32 {
		return ErrInvalidNodeID.Wrap("node_id must be 64 hex chars (32 bytes)")
	}
	return nil
}

// ─── MsgDeactivateSequencer ───────────────────────────────────────────────────

// MsgDeactivateSequencer sets IsActive = false for a registered sequencer.
// Can be sent by the operator (self-deactivation) or the governance authority.
type MsgDeactivateSequencer struct {
	// Sender is the operator or governance authority address.
	Sender string `json:"sender"`
	// NodeID identifies the sequencer to deactivate (64 hex chars).
	NodeID string `json:"node_id"`
	// Reason is a human-readable explanation (for audit log).
	Reason string `json:"reason"`
}

func (m *MsgDeactivateSequencer) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return ErrUnauthorized.Wrapf("invalid sender address: %s", err)
	}
	nodeIDBytes, err := hex.DecodeString(m.NodeID)
	if err != nil || len(nodeIDBytes) != 32 {
		return ErrInvalidNodeID.Wrap("node_id must be 64 hex chars (32 bytes)")
	}
	return nil
}
