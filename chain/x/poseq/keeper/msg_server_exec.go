package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// CommitExecution commits a PoSeq-finalized batch on-chain.
// Write-once: returns ErrBatchAlreadyCommitted if the batch_id is already stored.
func (m MsgServer) CommitExecution(ctx context.Context, msg *types.MsgCommitExecution) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// Authorization check (same as ExportBatch — must be authorized submitter)
	params := m.Keeper.GetParams(ctx)
	if params.AuthorizedSubmitter != "" && params.AuthorizedSubmitter != msg.Sender {
		return types.ErrUnauthorized.Wrapf(
			"sender %s is not the authorized submitter (%s)",
			msg.Sender, params.AuthorizedSubmitter,
		)
	}

	batchIDBytes, _ := hex.DecodeString(msg.BatchID)

	// Check for duplicate
	kvStore := m.Keeper.storeService.OpenKVStore(ctx)
	key := types.GetCommittedBatchKey(batchIDBytes)
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		return types.ErrBatchAlreadyCommitted.Wrapf("batch_id=%s", msg.BatchID)
	}

	// Build and store the committed batch record
	rec := types.CommittedBatchRecord{
		BatchID:              msg.BatchID,
		FinalizationHash:     msg.FinalizationHash,
		Epoch:                msg.Epoch,
		Slot:                 msg.Slot,
		OrderedSubmissionIDs: msg.OrderedSubmissionIDs,
		LeaderID:             msg.LeaderID,
		Approvals:            msg.Approvals,
		CommitteeSize:        msg.CommitteeSize,
		SubmitterAddress:     msg.Sender,
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshaling committed batch record: %w", err)
	}
	if err := kvStore.Set(key, bz); err != nil {
		return err
	}

	m.Keeper.Logger().Info("committed PoSeq batch",
		"batch_id", msg.BatchID,
		"epoch", msg.Epoch,
		"slot", msg.Slot,
		"approvals", msg.Approvals,
		"committee_size", msg.CommitteeSize,
		"submitter", msg.Sender,
	)
	return nil
}

// GetCommittedBatch retrieves a committed batch record by its hex batch_id.
// Returns (nil, nil) if not found.
func (k Keeper) GetCommittedBatch(ctx context.Context, batchIDHex string) (*types.CommittedBatchRecord, error) {
	batchIDBytes, err := hex.DecodeString(batchIDHex)
	if err != nil || len(batchIDBytes) != 32 {
		return nil, types.ErrInvalidBatchID
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetCommittedBatchKey(batchIDBytes))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.CommittedBatchRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
