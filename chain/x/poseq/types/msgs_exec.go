package types

import (
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
	return nil
}
