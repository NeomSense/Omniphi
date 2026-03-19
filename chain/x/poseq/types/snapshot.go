package types

import (
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── CommitteeSnapshot ────────────────────────────────────────────────────────

// CommitteeSnapshotMember is one entry in a deterministic committee snapshot.
type CommitteeSnapshotMember struct {
	// NodeID is the 32-byte node identity (hex-encoded, 64 chars).
	NodeID string `json:"node_id"`
	// PublicKey is the Ed25519 public key (hex-encoded, 32 bytes = 64 chars).
	PublicKey string `json:"public_key"`
	// Moniker is the human-readable operator label.
	Moniker string `json:"moniker"`
	// Role is "Sequencer" (propose+attest) or "Validator" (attest only).
	Role string `json:"role"`
}

// CommitteeSnapshot is the canonical, deterministic record of the eligible
// committee for a PoSeq epoch. Produced by the chain from the Active sequencer
// registry and imported by PoSeq nodes before each epoch transition.
//
// Members are sorted by NodeID (lexicographic on the raw 32-byte value).
//
// SnapshotHash = SHA256("committee_snapshot" | epoch_be(8) | member_count_be(4) | sorted_node_id_bytes...)
type CommitteeSnapshot struct {
	// Epoch is the PoSeq epoch this snapshot covers.
	Epoch uint64 `json:"epoch"`
	// Members is the sorted list of eligible committee participants.
	Members []CommitteeSnapshotMember `json:"members"`
	// SnapshotHash is the canonical integrity hash (32 bytes).
	SnapshotHash []byte `json:"snapshot_hash"`
	// ProducedAtBlock is the Cosmos block height when this snapshot was built.
	ProducedAtBlock int64 `json:"produced_at_block"`
}

// ─── MsgSubmitCommitteeSnapshot ───────────────────────────────────────────────

// MsgSubmitCommitteeSnapshot registers a committee snapshot on-chain.
// Must be sent by the governance authority address.
// Once stored for an epoch, the snapshot is immutable (write-once).
type MsgSubmitCommitteeSnapshot struct {
	// Authority is the governance module address.
	Authority string `json:"authority"`
	// Snapshot is the committee snapshot to register.
	Snapshot CommitteeSnapshot `json:"snapshot"`
}

func (m *MsgSubmitCommitteeSnapshot) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return ErrUnauthorized.Wrapf("invalid authority address: %s", err)
	}
	if m.Snapshot.Epoch == 0 {
		return ErrInvalidSnapshotEpoch.Wrap("snapshot epoch must be > 0")
	}
	if len(m.Snapshot.SnapshotHash) != 32 {
		return ErrSnapshotHashMismatch.Wrap("snapshot_hash must be 32 bytes")
	}
	for _, mem := range m.Snapshot.Members {
		b, err := hex.DecodeString(mem.NodeID)
		if err != nil || len(b) != 32 {
			return ErrInvalidNodeID.Wrapf("member node_id %q is not 64 hex chars", mem.NodeID)
		}
		if mem.Role != "Sequencer" && mem.Role != "Validator" {
			return ErrInvalidExportBatch.Wrapf("member role %q must be Sequencer or Validator", mem.Role)
		}
	}
	return nil
}
