package types

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── CommittedBatchRecord ────────────────────────────────────────────────────

// CommittedBatchRecord is the on-chain record for a committed PoSeq batch.
// Stored by batch_id; represents the chain's acknowledgment of PoSeq finalization.
type CommittedBatchRecord struct {
	// BatchID is the PoSeq batch ID (hex-encoded 32 bytes).
	BatchID string `json:"batch_id"`

	// FinalizationHash is the HotStuff finalization hash (hex-encoded 32 bytes).
	FinalizationHash string `json:"finalization_hash"`

	// Epoch at which this batch was sequenced.
	Epoch uint64 `json:"epoch"`

	// Slot (view) at which this batch was finalized.
	Slot uint64 `json:"slot"`

	// OrderedSubmissionIDs are the hex-encoded 32-byte IDs of submissions in
	// the batch, in their final ordering.
	OrderedSubmissionIDs []string `json:"ordered_submission_ids"`

	// LeaderID is the hex-encoded node ID of the sequencer who proposed this batch.
	LeaderID string `json:"leader_id"`

	// Approvals is the number of HotStuff QC signatures that formed the quorum.
	Approvals uint64 `json:"approvals"`

	// CommitteeSize is the total committee size at finalization.
	CommitteeSize uint64 `json:"committee_size"`

	// SubmitterAddress is the bech32 address of the relayer who submitted this tx.
	SubmitterAddress string `json:"submitter_address"`
}

// BatchIDBytes decodes the hex BatchID into raw bytes.
func (r CommittedBatchRecord) BatchIDBytes() ([]byte, error) {
	return hex.DecodeString(r.BatchID)
}

// ─── MsgCommitExecution ──────────────────────────────────────────────────────

// MsgCommitExecution commits a PoSeq-finalized batch on-chain.
//
// This is submitted by the PoSeq relayer after a batch reaches HotStuff
// quorum. It anchors the batch's finalization hash on-chain, enabling the
// execution layer to reference it for settlement.
//
// Write-once: if the batch_id is already committed, the tx is rejected with
// ErrBatchAlreadyCommitted.
type MsgCommitExecution struct {
	// Sender is the relayer's bech32 operator address.
	Sender string `json:"sender"`

	// BatchID is the hex-encoded 32-byte PoSeq batch ID.
	BatchID string `json:"batch_id"`

	// FinalizationHash is the HotStuff QC finalization hash (hex-encoded, 32 bytes).
	FinalizationHash string `json:"finalization_hash"`

	// Epoch at which this batch was sequenced.
	Epoch uint64 `json:"epoch"`

	// Slot (HotStuff view) at which this batch was finalized.
	Slot uint64 `json:"slot"`

	// OrderedSubmissionIDs are the hex-encoded 32-byte submission IDs in order.
	OrderedSubmissionIDs []string `json:"ordered_submission_ids"`

	// LeaderID is the hex-encoded 32-byte node ID of the proposing sequencer.
	LeaderID string `json:"leader_id"`

	// Approvals is the number of QC signatures.
	Approvals uint64 `json:"approvals"`

	// CommitteeSize is the total committee size.
	CommitteeSize uint64 `json:"committee_size"`

	// ProposalID is the hex-encoded 32-byte HotStuff proposal ID.
	// Used to recompute and verify the finalization hash on-chain.
	ProposalID string `json:"proposal_id"`

	// BatchRoot is the hex-encoded 32-byte Merkle root of the batch contents.
	// Used to recompute and verify the finalization hash on-chain.
	BatchRoot string `json:"batch_root"`

	// QCSignatures are the Ed25519 signatures from the Quorum Certificate.
	// Each entry is {NodeID (hex, 64 chars), Signature (hex, 128 chars)}.
	// These are verified against the committee snapshot's registered public keys.
	// When empty (devnet/bootstrap), QC verification is skipped with a warning log.
	QCSignatures []QCSignatureEntry `json:"qc_signatures,omitempty"`
}

// QCSignatureEntry is a single Ed25519 signature in a Quorum Certificate.
type QCSignatureEntry struct {
	NodeID    string `json:"node_id"`    // hex-encoded 32-byte node ID
	Signature string `json:"signature"`  // hex-encoded 64-byte Ed25519 signature
}

func (m *MsgCommitExecution) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return ErrUnauthorized.Wrapf("invalid sender address: %s", err)
	}
	batchIDBytes, err := hex.DecodeString(m.BatchID)
	if err != nil || len(batchIDBytes) != 32 {
		return ErrInvalidBatchID.Wrap("batch_id must be 64 hex chars (32 bytes)")
	}
	finHashBytes, err := hex.DecodeString(m.FinalizationHash)
	if err != nil || len(finHashBytes) != 32 {
		return ErrInvalidFinalizationHash.Wrap("finalization_hash must be 64 hex chars (32 bytes)")
	}
	proposalIDBytes, err := hex.DecodeString(m.ProposalID)
	if err != nil || len(proposalIDBytes) != 32 {
		return ErrInvalidExportBatch.Wrap("proposal_id must be 64 hex chars (32 bytes)")
	}
	batchRootBytes, err := hex.DecodeString(m.BatchRoot)
	if err != nil || len(batchRootBytes) != 32 {
		return ErrInvalidExportBatch.Wrap("batch_root must be 64 hex chars (32 bytes)")
	}
	if m.Epoch == 0 {
		return ErrInvalidEpoch
	}
	if m.Approvals == 0 {
		return ErrInvalidExportBatch.Wrap("approvals must be > 0")
	}
	if m.CommitteeSize == 0 {
		return ErrInvalidExportBatch.Wrap("committee_size must be > 0")
	}
	// Verify quorum is met: approvals > 2/3 of committee
	if m.Approvals*3 <= m.CommitteeSize*2 {
		return ErrInvalidExportBatch.Wrapf(
			"insufficient quorum: %d approvals out of %d committee members",
			m.Approvals, m.CommitteeSize,
		)
	}
	// Validate QC signature entries if present
	for i, entry := range m.QCSignatures {
		nodeBytes, err := hex.DecodeString(entry.NodeID)
		if err != nil || len(nodeBytes) != 32 {
			return ErrInvalidExportBatch.Wrapf("qc_signatures[%d]: invalid node_id hex", i)
		}
		sigBytes, err := hex.DecodeString(entry.Signature)
		if err != nil || len(sigBytes) != 64 {
			return ErrInvalidExportBatch.Wrapf("qc_signatures[%d]: invalid signature hex (need 128 chars)", i)
		}
	}
	return nil
}

// ComputeFinalizationHash recomputes the deterministic finalization hash.
// Must match Rust-side NodeState::compute_finalization_hash:
//
//	SHA256(proposal_id || slot_be || epoch_be || leader_id || batch_root || approvals_be)
func ComputeFinalizationHash(proposalID []byte, slot, epoch uint64, leaderID, batchRoot []byte, approvals uint64) [32]byte {
	h := sha256.New()
	h.Write(proposalID)
	// slot (big-endian u64)
	slotBytes := make([]byte, 8)
	slotBytes[0] = byte(slot >> 56)
	slotBytes[1] = byte(slot >> 48)
	slotBytes[2] = byte(slot >> 40)
	slotBytes[3] = byte(slot >> 32)
	slotBytes[4] = byte(slot >> 24)
	slotBytes[5] = byte(slot >> 16)
	slotBytes[6] = byte(slot >> 8)
	slotBytes[7] = byte(slot)
	h.Write(slotBytes)
	// epoch (big-endian u64)
	epochBytes := make([]byte, 8)
	epochBytes[0] = byte(epoch >> 56)
	epochBytes[1] = byte(epoch >> 48)
	epochBytes[2] = byte(epoch >> 40)
	epochBytes[3] = byte(epoch >> 32)
	epochBytes[4] = byte(epoch >> 24)
	epochBytes[5] = byte(epoch >> 16)
	epochBytes[6] = byte(epoch >> 8)
	epochBytes[7] = byte(epoch)
	h.Write(epochBytes)
	h.Write(leaderID)
	h.Write(batchRoot)
	// approvals (big-endian u64)
	approvalsBytes := make([]byte, 8)
	approvalsBytes[0] = byte(approvals >> 56)
	approvalsBytes[1] = byte(approvals >> 48)
	approvalsBytes[2] = byte(approvals >> 40)
	approvalsBytes[3] = byte(approvals >> 32)
	approvalsBytes[4] = byte(approvals >> 24)
	approvalsBytes[5] = byte(approvals >> 16)
	approvalsBytes[6] = byte(approvals >> 8)
	approvalsBytes[7] = byte(approvals)
	h.Write(approvalsBytes)
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// ComputeVoteHash computes the HotStuff vote hash for QC verification.
// Must match the Rust-side: SHA256("HOTSTUFF_VOTE_V1" || view_be || block_id || phase_byte).
// For CommitExecution, phase = Commit (2).
func ComputeVoteHash(slot uint64, batchID []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte("HOTSTUFF_VOTE_V1"))
	// view = slot (big-endian u64)
	viewBytes := make([]byte, 8)
	viewBytes[0] = byte(slot >> 56)
	viewBytes[1] = byte(slot >> 48)
	viewBytes[2] = byte(slot >> 40)
	viewBytes[3] = byte(slot >> 32)
	viewBytes[4] = byte(slot >> 24)
	viewBytes[5] = byte(slot >> 16)
	viewBytes[6] = byte(slot >> 8)
	viewBytes[7] = byte(slot)
	h.Write(viewBytes)
	h.Write(batchID)
	h.Write([]byte{2}) // Phase::Commit = 2
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// VerifyQC verifies the Ed25519 signatures in QCSignatures against the
// committee snapshot's registered public keys.
//
// Returns the number of valid unique signatures. The caller checks 2f+1 threshold.
//
// pubkeyLookup maps hex node_id → 32-byte Ed25519 public key.
func VerifyQC(
	qcSigs []QCSignatureEntry,
	voteHash [32]byte,
	pubkeyLookup map[string][]byte,
) (validCount int) {
	seen := make(map[string]bool)
	for _, entry := range qcSigs {
		// Dedup by node_id
		if seen[entry.NodeID] {
			continue
		}
		seen[entry.NodeID] = true

		pubkey, ok := pubkeyLookup[entry.NodeID]
		if !ok || len(pubkey) != ed25519.PublicKeySize {
			continue // unknown signer
		}
		sigBytes, err := hex.DecodeString(entry.Signature)
		if err != nil || len(sigBytes) != ed25519.SignatureSize {
			continue
		}
		if ed25519.Verify(pubkey, voteHash[:], sigBytes) {
			validCount++
		}
	}
	return
}
