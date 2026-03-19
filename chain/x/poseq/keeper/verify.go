package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"

	"pos/x/poseq/types"
)

// verifyCheckpointAnchorHash recomputes the anchor hash and checks it matches.
// This mirrors the Rust CheckpointAnchorRecord::verify() method.
//
// anchor_hash = SHA256("ckpt" | checkpoint_id | epoch_be | epoch_state_hash | bridge_state_hash)
func verifyCheckpointAnchorHash(anchor types.CheckpointAnchorRecord) bool {
	if len(anchor.CheckpointID) != 32 ||
		len(anchor.EpochStateHash) != 32 ||
		len(anchor.BridgeStateHash) != 32 ||
		len(anchor.AnchorHash) != 32 {
		return false
	}

	h := sha256.New()
	h.Write([]byte("ckpt"))
	h.Write(anchor.CheckpointID)
	h.Write(uint64BE(anchor.Epoch))
	h.Write(anchor.EpochStateHash)
	h.Write(anchor.BridgeStateHash)
	computed := h.Sum(nil)

	for i := range computed {
		if computed[i] != anchor.AnchorHash[i] {
			return false
		}
	}
	return true
}

// verifyEvidencePacketHash recomputes the packet hash and checks it matches.
// This mirrors the Rust EvidencePacket::compute_packet_hash() method.
//
// packet_hash = SHA256(kind_tag | node_id | epoch_be | sorted_evidence_hashes)
func verifyEvidencePacketHash(pkt types.EvidencePacket) bool {
	if len(pkt.PacketHash) != 32 || len(pkt.OffenderNodeID) != 32 {
		return false
	}

	h := sha256.New()
	h.Write([]byte(evidenceKindTag(pkt.Kind)))
	h.Write(pkt.OffenderNodeID)
	h.Write(uint64BE(pkt.Epoch))

	// Sort evidence hashes (same as Rust BTreeSet ordering — lexicographic)
	sorted := sortedHashes(pkt.EvidenceHashes)
	for _, eh := range sorted {
		h.Write(eh)
	}
	computed := h.Sum(nil)

	for i := range computed {
		if computed[i] != pkt.PacketHash[i] {
			return false
		}
	}
	return true
}

func evidenceKindTag(k types.EvidenceKind) string {
	switch k {
	case types.EvidenceKindEquivocation:
		return "equivocation"
	case types.EvidenceKindUnauthorizedProposal:
		return "unauthorized_proposal"
	case types.EvidenceKindUnfairSequencing:
		return "unfair_sequencing"
	case types.EvidenceKindReplayAbuse:
		return "replay_abuse"
	case types.EvidenceKindBridgeAbuse:
		return "bridge_abuse"
	case types.EvidenceKindPersistentAbsence:
		return "persistent_absence"
	case types.EvidenceKindStaleAuthority:
		return "stale_authority"
	case types.EvidenceKindInvalidProposalSpam:
		return "invalid_proposal_spam"
	default:
		return "unknown"
	}
}

func uint64BE(v uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}

// ─── Committee Snapshot Hash ──────────────────────────────────────────────────

// computeCommitteeSnapshotHash recomputes the canonical snapshot hash.
//
// hash = SHA256("committee_snapshot" | epoch_be(8) | member_count_be(4) | sorted_node_id_bytes...)
//
// Node IDs are sorted lexicographically over their raw 32-byte form,
// matching the Rust SnapshotImporter::compute_hash() function.
func computeCommitteeSnapshotHash(snap types.CommitteeSnapshot) ([]byte, error) {
	// Decode and sort member node IDs
	nodeIDs := make([][]byte, 0, len(snap.Members))
	for _, m := range snap.Members {
		b, err := hex.DecodeString(m.NodeID)
		if err != nil || len(b) != 32 {
			return nil, types.ErrInvalidNodeID.Wrapf("member node_id %q is invalid", m.NodeID)
		}
		nodeIDs = append(nodeIDs, b)
	}
	sort.Slice(nodeIDs, func(i, j int) bool {
		return lessBytes(nodeIDs[i], nodeIDs[j])
	})

	h := sha256.New()
	h.Write([]byte("committee_snapshot"))
	h.Write(uint64BE(snap.Epoch))
	countBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(countBuf, uint32(len(nodeIDs)))
	h.Write(countBuf)
	for _, id := range nodeIDs {
		h.Write(id)
	}
	return h.Sum(nil), nil
}

// verifyCommitteeSnapshotHash checks that snap.SnapshotHash matches the recomputed hash.
func verifyCommitteeSnapshotHash(snap types.CommitteeSnapshot) bool {
	computed, err := computeCommitteeSnapshotHash(snap)
	if err != nil {
		return false
	}
	if len(computed) != len(snap.SnapshotHash) {
		return false
	}
	for i := range computed {
		if computed[i] != snap.SnapshotHash[i] {
			return false
		}
	}
	return true
}

// sortedHashes returns a lexicographically sorted copy of the hash slice.
func sortedHashes(hashes [][]byte) [][]byte {
	sorted := make([][]byte, len(hashes))
	copy(sorted, hashes)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && lessBytes(sorted[j], sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

func lessBytes(a, b []byte) bool {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
