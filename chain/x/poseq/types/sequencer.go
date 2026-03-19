package types

import (
	"encoding/hex"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── SequencerStatus ──────────────────────────────────────────────────────────

// SequencerStatus represents the full lifecycle state of a PoSeq sequencer.
type SequencerStatus string

const (
	// SequencerStatusPending: registered, awaiting governance activation.
	SequencerStatusPending SequencerStatus = "Pending"
	// SequencerStatusActive: eligible to participate in PoSeq committee.
	SequencerStatusActive SequencerStatus = "Active"
	// SequencerStatusSuspended: temporarily barred; can recover via governance.
	SequencerStatusSuspended SequencerStatus = "Suspended"
	// SequencerStatusJailed: evidence-based bar; requires governance to unjail.
	SequencerStatusJailed SequencerStatus = "Jailed"
	// SequencerStatusRetired: voluntary exit; terminal state.
	SequencerStatusRetired SequencerStatus = "Retired"
)

// ValidateSequencerTransition returns nil if transitioning from → to is permitted.
//
// Permitted transitions:
//
//	Pending   → Active
//	Active    → Suspended, Jailed, Retired
//	Suspended → Active, Jailed, Retired
//	Jailed    → Active, Retired
//	Retired   → (none — terminal)
func ValidateSequencerTransition(from, to SequencerStatus) error {
	switch from {
	case SequencerStatusPending:
		if to == SequencerStatusActive {
			return nil
		}
	case SequencerStatusActive:
		if to == SequencerStatusSuspended || to == SequencerStatusJailed || to == SequencerStatusRetired {
			return nil
		}
	case SequencerStatusSuspended:
		if to == SequencerStatusActive || to == SequencerStatusJailed || to == SequencerStatusRetired {
			return nil
		}
	case SequencerStatusJailed:
		if to == SequencerStatusActive || to == SequencerStatusRetired {
			return nil
		}
	case SequencerStatusRetired:
		// terminal — no transitions allowed
	}
	return ErrInvalidLifecycleTransition.Wrapf("cannot transition from %s to %s", from, to)
}

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

	// CosmosValidatorAddress is the optional bech32 valoper address if the
	// operator also runs a Cosmos PoS validator (slow lane linkage).
	// Empty string means no explicit linkage declared.
	CosmosValidatorAddress string `json:"cosmos_validator_address,omitempty"`

	// RegisteredEpoch is the PoSeq epoch at time of registration.
	RegisteredEpoch uint64 `json:"registered_epoch"`

	// Status is the current lifecycle state of this sequencer.
	Status SequencerStatus `json:"status"`

	// StatusSince is the PoSeq epoch at which the current Status was set.
	StatusSince uint64 `json:"status_since"`

	// LastLivenessEpoch is the last epoch for which a liveness event was
	// received from the PoSeq network. Zero if never observed.
	LastLivenessEpoch uint64 `json:"last_liveness_epoch"`
}

// IsActive returns true only when Status == Active.
// This replaces the old IsActive bool field for backward compatibility.
func (r SequencerRecord) IsActive() bool {
	return r.Status == SequencerStatusActive
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

// ─── MsgTransitionSequencer ───────────────────────────────────────────────────

// MsgTransitionSequencer performs an explicit FSM lifecycle transition on a
// registered sequencer. Only the governance authority may execute this.
// Use this for Jailed, Retired, or governance-ordered status changes.
type MsgTransitionSequencer struct {
	// Authority is the governance module address.
	Authority string `json:"authority"`
	// NodeID identifies the sequencer (64 hex chars).
	NodeID string `json:"node_id"`
	// ToStatus is the target lifecycle state.
	ToStatus SequencerStatus `json:"to_status"`
	// Reason is a human-readable explanation for audit logging.
	Reason string `json:"reason"`
	// Epoch is the current PoSeq epoch (stored as StatusSince).
	Epoch uint64 `json:"epoch"`
}

func (m *MsgTransitionSequencer) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrUnauthorized.Wrapf("invalid authority address: %s", err)
	}
	nodeIDBytes, err := hex.DecodeString(m.NodeID)
	if err != nil || len(nodeIDBytes) != 32 {
		return ErrInvalidNodeID.Wrap("node_id must be 64 hex chars (32 bytes)")
	}
	switch m.ToStatus {
	case SequencerStatusActive, SequencerStatusSuspended, SequencerStatusJailed, SequencerStatusRetired:
		// valid targets
	default:
		return fmt.Errorf("invalid to_status %q: must be Active, Suspended, Jailed, or Retired", m.ToStatus)
	}
	return nil
}
