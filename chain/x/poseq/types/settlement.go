package types

import (
	"encoding/hex"
	"fmt"
)

// ─── SettlementAnchorRecord ──────────────────────────────────────────────────

// SettlementAnchorRecord anchors a runtime settlement result on-chain.
// Write-once per batch_hash — prevents double settlement.
type SettlementAnchorRecord struct {
	// SettlementHash is the SHA256 hash of the full settlement result.
	SettlementHash string `json:"settlement_hash"`

	// BatchHash links this settlement to the PoSeq finalized batch.
	BatchHash string `json:"batch_hash"`

	// PostStateRoot is the runtime state root after execution.
	PostStateRoot string `json:"post_state_root"`

	// ExecutionReceiptHash is the hash of all execution receipts.
	ExecutionReceiptHash string `json:"execution_receipt_hash"`

	// Epoch is the epoch of this settlement.
	Epoch uint64 `json:"epoch"`

	// SequenceNumber is the sequence within the epoch.
	SequenceNumber uint64 `json:"sequence_number"`

	// SettledCount is the number of intents that settled successfully.
	SettledCount uint32 `json:"settled_count"`

	// FailedCount is the number of intents that failed settlement.
	FailedCount uint32 `json:"failed_count"`

	// SubmitterAddress is the Cosmos address that submitted this anchor.
	SubmitterAddress string `json:"submitter_address"`
}

// BatchHashBytes returns the decoded batch hash.
func (r *SettlementAnchorRecord) BatchHashBytes() ([]byte, error) {
	return hex.DecodeString(r.BatchHash)
}

// ─── MsgAnchorSettlement ─────────────────────────────────────────────────────

// MsgAnchorSettlement anchors a runtime settlement result on-chain.
// Write-once per batch_hash.
type MsgAnchorSettlement struct {
	// Sender is the relayer account submitting this anchor.
	Sender string `json:"sender"`

	// Anchor is the settlement anchor data.
	Anchor SettlementAnchorRecord `json:"anchor"`
}

func (m *MsgAnchorSettlement) ValidateBasic() error {
	if m.Sender == "" {
		return fmt.Errorf("sender cannot be empty")
	}
	if m.Anchor.BatchHash == "" {
		return fmt.Errorf("batch_hash cannot be empty")
	}
	if m.Anchor.SettlementHash == "" {
		return fmt.Errorf("settlement_hash cannot be empty")
	}
	if m.Anchor.PostStateRoot == "" {
		return fmt.Errorf("post_state_root cannot be empty")
	}

	// Validate hex encoding (32 bytes = 64 hex chars)
	if len(m.Anchor.BatchHash) != 64 {
		return fmt.Errorf("batch_hash must be 64 hex chars (32 bytes)")
	}
	if _, err := hex.DecodeString(m.Anchor.BatchHash); err != nil {
		return fmt.Errorf("batch_hash is not valid hex: %w", err)
	}
	if len(m.Anchor.SettlementHash) != 64 {
		return fmt.Errorf("settlement_hash must be 64 hex chars (32 bytes)")
	}
	if _, err := hex.DecodeString(m.Anchor.SettlementHash); err != nil {
		return fmt.Errorf("settlement_hash is not valid hex: %w", err)
	}
	if len(m.Anchor.PostStateRoot) != 64 {
		return fmt.Errorf("post_state_root must be 64 hex chars (32 bytes)")
	}
	if _, err := hex.DecodeString(m.Anchor.PostStateRoot); err != nil {
		return fmt.Errorf("post_state_root is not valid hex: %w", err)
	}

	if m.Anchor.Epoch == 0 {
		return fmt.Errorf("epoch must be > 0")
	}

	return nil
}
