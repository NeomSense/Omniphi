package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// CommitExecution commits a PoSeq-finalized batch on-chain.
// Write-once: returns ErrBatchAlreadyCommitted if the batch_id is already stored.
//
// Validation pipeline:
// 1. ValidateBasic — structural checks (hex formats, quorum ratio)
// 2. Authorization — sender must be AuthorizedSubmitter (if configured)
// 3. Committee binding — LeaderID must be a member of the epoch's committee snapshot
// 4. Committee size consistency — msg.CommitteeSize must match the snapshot
// 5. QC verification — Ed25519 multi-sig check (enforced when RequireQCSignatures=true)
// 6. Finalization hash verification — recomputed and compared to msg.FinalizationHash
// 7. Dedup — batch_id must not already exist
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

	// Committee binding: verify the claimed leader and committee size against
	// the on-chain committee snapshot for this epoch. Without this check, a
	// relayer could fabricate approval counts or claim an arbitrary leader.
	snap, err := m.Keeper.GetCommitteeSnapshot(ctx, msg.Epoch)
	if err != nil {
		return types.ErrInvalidExportBatch.Wrapf("failed to load committee snapshot for epoch %d: %v", msg.Epoch, err)
	}
	if snap == nil {
		// No committee snapshot for this epoch — reject. Without a snapshot,
		// we cannot verify committee membership, leader eligibility, or committee
		// size. Allowing commits without a snapshot reverts the system to pure
		// relayer trust, which is unacceptable for consensus binding.
		return types.ErrInvalidExportBatch.Wrapf(
			"no committee snapshot registered for epoch %d — cannot verify committee binding",
			msg.Epoch,
		)
	}
	// Verify committee size matches
	if uint64(len(snap.Members)) != msg.CommitteeSize {
		return types.ErrInvalidExportBatch.Wrapf(
			"committee_size mismatch: msg claims %d, snapshot has %d members",
			msg.CommitteeSize, len(snap.Members),
		)
	}
	// Verify leader is a committee member
	leaderFound := false
	for _, mem := range snap.Members {
		if mem.NodeID == msg.LeaderID {
			leaderFound = true
			break
		}
	}
	if !leaderFound {
		return types.ErrUnauthorized.Wrapf(
			"leader %s is not a member of the epoch %d committee",
			msg.LeaderID, msg.Epoch,
		)
	}

	// QC verification: if QCSignatures are provided, verify Ed25519 multi-sig
	// against the committee snapshot's registered public keys.
	if len(msg.QCSignatures) > 0 {
		batchIDForHash, _ := hex.DecodeString(msg.BatchID)
		voteHash := types.ComputeVoteHash(msg.Slot, batchIDForHash)

		// Build pubkey lookup from snapshot members
		pubkeyLookup := make(map[string][]byte)
		for _, mem := range snap.Members {
			if pk, err := hex.DecodeString(mem.PublicKey); err == nil && len(pk) == 32 {
				pubkeyLookup[mem.NodeID] = pk
			}
		}

		validSigs := types.VerifyQC(msg.QCSignatures, voteHash, pubkeyLookup)
		// Require 2f+1: validSigs * 3 > committeeSize * 2
		committeeSize := uint64(len(snap.Members))
		if uint64(validSigs)*3 <= committeeSize*2 {
			return types.ErrInvalidExportBatch.Wrapf(
				"QC verification failed: %d valid signatures out of %d committee (need >2/3)",
				validSigs, committeeSize,
			)
		}

		m.Keeper.Logger().Info("QC verification passed",
			"valid_sigs", validSigs,
			"committee_size", committeeSize,
			"batch_id", msg.BatchID,
		)
	} else {
		// No QC signatures provided. When RequireQCSignatures is enabled
		// (mainnet mode), reject the transaction outright.
		if params.RequireQCSignatures {
			return types.ErrInvalidExportBatch.Wrapf(
				"QC signatures required (require_qc_signatures=true) but none provided for batch %s",
				msg.BatchID,
			)
		}
		m.Keeper.Logger().Warn("CommitExecution without QC signatures (require_qc_signatures=false)",
			"batch_id", msg.BatchID,
			"epoch", msg.Epoch,
		)
	}

	// Finalization hash verification: recompute from constituent fields and
	// compare to the submitted value. This prevents fabricated finalization
	// hashes that don't correspond to actual PoSeq finalized proposals.
	// Matches Rust-side NodeState::compute_finalization_hash.
	proposalIDBytes, _ := hex.DecodeString(msg.ProposalID)
	leaderIDBytes, _ := hex.DecodeString(msg.LeaderID)
	batchRootBytes, _ := hex.DecodeString(msg.BatchRoot)
	expectedFinHash := types.ComputeFinalizationHash(
		proposalIDBytes, msg.Slot, msg.Epoch, leaderIDBytes, batchRootBytes, msg.Approvals,
	)
	expectedFinHashHex := hex.EncodeToString(expectedFinHash[:])
	if expectedFinHashHex != msg.FinalizationHash {
		return types.ErrInvalidFinalizationHash.Wrapf(
			"finalization_hash mismatch: expected %s, got %s",
			expectedFinHashHex, msg.FinalizationHash,
		)
	}

	// Verify batch_id is consistent with proposal_id and batch_root.
	// batch_id = SHA256("batch:" || proposal_id || batch_root)
	batchIDCheck := sha256.New()
	batchIDCheck.Write([]byte("batch:"))
	batchIDCheck.Write(proposalIDBytes)
	batchIDCheck.Write(batchRootBytes)
	expectedBatchID := hex.EncodeToString(batchIDCheck.Sum(nil))
	if expectedBatchID != msg.BatchID {
		return types.ErrInvalidBatchID.Wrapf(
			"batch_id mismatch: expected SHA256(batch:||proposal_id||batch_root)=%s, got %s",
			expectedBatchID, msg.BatchID,
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
