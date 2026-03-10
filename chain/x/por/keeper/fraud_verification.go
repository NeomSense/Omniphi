package keeper

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// ============================================================================
// Automatic Fraud Proof Verification
//
// Resolves open challenges on-chain when possible. Called by EndBlocker for
// PENDING batches past their challenge window that still have open challenges.
//
// Verifiable on-chain:
//   - ChallengeTypeInvalidRoot (0): Recompute merkle root from provided leaves
//   - ChallengeTypeDoubleInclusion (1): Verify other batch exists and is finalized
//
// Not verifiable on-chain (auto-reject after ChallengeResolutionTimeout):
//   - ChallengeTypeMissingRecord (2)
//   - ChallengeTypeInvalidSchema (3)
// ============================================================================

// InvalidRootProof is the expected JSON format of ProofData for ChallengeTypeInvalidRoot.
type InvalidRootProof struct {
	LeafHashes [][]byte `json:"leaf_hashes"` // SHA256 hashes of all records in order
}

// DoubleInclusionProof is the expected JSON format of ProofData for ChallengeTypeDoubleInclusion.
type DoubleInclusionProof struct {
	RecordHash   []byte `json:"record_hash"`    // Hash of the duplicated record
	OtherBatchId uint64 `json:"other_batch_id"` // ID of the batch containing the duplicate
}

// resolveOpenChallenges attempts to automatically verify and resolve all open
// challenges for a given batch. Called by EndBlocker when a batch is past its
// challenge window but still has open challenges.
//
// SAFETY: Never panics. All errors are logged and processing continues.
func (k Keeper) resolveOpenChallenges(ctx sdk.Context, batchID uint64) error {
	batch, found := k.GetBatch(ctx, batchID)
	if !found {
		return fmt.Errorf("batch %d not found during challenge resolution", batchID)
	}

	challenges := k.GetChallengesForBatch(ctx, batchID)
	params := k.GetParams(ctx)
	now := ctx.BlockTime().Unix()

	for _, challenge := range challenges {
		if challenge.Status != types.ChallengeStatusOpen {
			continue
		}

		valid, conclusive, err := k.verifyChallenge(ctx, challenge, batch)
		if err != nil {
			k.Logger().Error("challenge verification error",
				"challenge_id", challenge.ChallengeId,
				"batch_id", batchID,
				"error", err,
			)
			// Check timeout for inconclusive verification
			if k.isChallengeTimedOut(now, batch.ChallengeEndTime, params.ChallengeResolutionTimeout) {
				k.Logger().Info("auto-rejecting timed-out challenge",
					"challenge_id", challenge.ChallengeId,
					"batch_id", batchID,
				)
				if pErr := k.ProcessInvalidChallenge(ctx, challenge.ChallengeId); pErr != nil {
					k.Logger().Error("failed to auto-reject timed-out challenge",
						"challenge_id", challenge.ChallengeId, "error", pErr,
					)
				}
			}
			continue
		}

		if conclusive {
			if valid {
				// Fraud proven — reject batch and slash attesters
				if pErr := k.ProcessValidChallenge(ctx, challenge.ChallengeId); pErr != nil {
					k.Logger().Error("failed to process valid challenge",
						"challenge_id", challenge.ChallengeId, "error", pErr,
					)
				} else {
					ctx.EventManager().EmitEvent(sdk.NewEvent(
						"por_fraud_auto_verified",
						sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challenge.ChallengeId)),
						sdk.NewAttribute("batch_id", fmt.Sprintf("%d", batchID)),
						sdk.NewAttribute("challenge_type", fmt.Sprintf("%d", challenge.ChallengeType)),
					))
					// Batch is now REJECTED — stop processing remaining challenges
					return nil
				}
			} else {
				// Challenge disproven — reject the challenge
				if pErr := k.ProcessInvalidChallenge(ctx, challenge.ChallengeId); pErr != nil {
					k.Logger().Error("failed to process invalid challenge",
						"challenge_id", challenge.ChallengeId, "error", pErr,
					)
				}
			}
		} else {
			// Inconclusive — check if timed out
			if k.isChallengeTimedOut(now, batch.ChallengeEndTime, params.ChallengeResolutionTimeout) {
				k.Logger().Info("auto-rejecting inconclusive timed-out challenge",
					"challenge_id", challenge.ChallengeId,
					"batch_id", batchID,
					"challenge_type", challenge.ChallengeType,
				)
				if pErr := k.ProcessInvalidChallenge(ctx, challenge.ChallengeId); pErr != nil {
					k.Logger().Error("failed to auto-reject timed-out challenge",
						"challenge_id", challenge.ChallengeId, "error", pErr,
					)
				}
			}
		}
	}

	return nil
}

// verifyChallenge dispatches verification based on ChallengeType.
// Returns (valid, conclusive, error).
//   - valid=true, conclusive=true: fraud proven
//   - valid=false, conclusive=true: challenge disproven
//   - conclusive=false: cannot determine on-chain (will timeout)
func (k Keeper) verifyChallenge(ctx sdk.Context, challenge types.Challenge, batch types.BatchCommitment) (bool, bool, error) {
	switch challenge.ChallengeType {
	case types.ChallengeTypeInvalidRoot:
		return verifyInvalidRoot(challenge, batch)
	case types.ChallengeTypeDoubleInclusion:
		return k.verifyDoubleInclusion(ctx, challenge, batch)
	case types.ChallengeTypeMissingRecord, types.ChallengeTypeInvalidSchema:
		// Cannot verify on-chain — return inconclusive
		return false, false, nil
	default:
		return false, false, fmt.Errorf("unknown challenge type: %d", challenge.ChallengeType)
	}
}

// verifyInvalidRoot checks whether the challenger's provided leaf hashes
// produce a different merkle root than the batch's RecordMerkleRoot.
//
// If the computed root from the challenger's leaves does NOT match the batch root,
// AND the leaf count matches RecordCount, the challenge is valid (batch has wrong root).
// If the computed root matches, the challenge is invalid.
func verifyInvalidRoot(challenge types.Challenge, batch types.BatchCommitment) (bool, bool, error) {
	var proof InvalidRootProof
	if err := json.Unmarshal(challenge.ProofData, &proof); err != nil {
		return false, false, fmt.Errorf("failed to parse InvalidRootProof: %w", err)
	}

	if len(proof.LeafHashes) == 0 {
		return false, false, fmt.Errorf("empty leaf hashes in proof")
	}

	// The leaf count must match the batch's RecordCount for valid comparison
	if uint64(len(proof.LeafHashes)) != batch.RecordCount {
		return false, false, fmt.Errorf("leaf count %d does not match batch record count %d",
			len(proof.LeafHashes), batch.RecordCount)
	}

	// Validate each leaf hash is 32 bytes
	for i, h := range proof.LeafHashes {
		if len(h) != 32 {
			return false, false, fmt.Errorf("leaf hash %d has invalid length %d (expected 32)", i, len(h))
		}
	}

	// Compute merkle root from provided leaves
	computedRoot := ComputeMerkleRoot(proof.LeafHashes)

	// Compare with batch's stored root
	if !bytesEqual(computedRoot, batch.RecordMerkleRoot) {
		// Roots differ — the challenger has demonstrated the correct records
		// produce a different root than what the batch claims. Fraud proven.
		return true, true, nil
	}

	// Roots match — the challenger's leaves are consistent with the batch root.
	// Challenge is invalid.
	return false, true, nil
}

// verifyDoubleInclusion checks whether a record hash exists in another batch.
// SECURITY (F3): When both batches have stored leaf hashes, verification is
// conclusive via the reverse index. Otherwise falls back to inconclusive.
func (k Keeper) verifyDoubleInclusion(ctx sdk.Context, challenge types.Challenge, batch types.BatchCommitment) (bool, bool, error) {
	var proof DoubleInclusionProof
	if err := json.Unmarshal(challenge.ProofData, &proof); err != nil {
		return false, false, fmt.Errorf("failed to parse DoubleInclusionProof: %w", err)
	}

	if len(proof.RecordHash) != 32 {
		return false, false, fmt.Errorf("record hash has invalid length %d (expected 32)", len(proof.RecordHash))
	}

	if proof.OtherBatchId == batch.BatchId {
		// Cannot be a duplicate within the same batch via this proof type
		return false, true, nil
	}

	// Check that the other batch exists
	otherBatch, found := k.GetBatch(ctx, proof.OtherBatchId)
	if !found {
		// Other batch doesn't exist — challenge is invalid
		return false, true, nil
	}

	// F3: Conclusive verification via leaf hash reverse index
	// If the challenged batch has stored leaf hashes, we can do a definitive lookup
	inThisBatch := k.HasLeafHashInBatch(ctx, batch.BatchId, proof.RecordHash)
	inOtherBatch := k.HasLeafHashInBatch(ctx, proof.OtherBatchId, proof.RecordHash)

	if inThisBatch && inOtherBatch {
		// Both batches contain this leaf hash — conclusive double-inclusion proven
		return true, true, nil
	}

	if inThisBatch && !inOtherBatch {
		// Leaf exists in challenged batch but NOT in the other — challenge is invalid
		return false, true, nil
	}

	// If leaf hashes were not stored for one or both batches, check credibility
	if otherBatch.Status == types.BatchStatusFinalized || otherBatch.Status == types.BatchStatusPending {
		// The other batch exists and is live — credible but inconclusive
		// (no leaf hashes stored to do definitive check)
		return false, false, nil
	}

	// Other batch is rejected or submitted — not a credible duplicate source
	return false, true, nil
}

// isChallengeTimedOut returns true if the challenge resolution timeout has elapsed.
func (k Keeper) isChallengeTimedOut(now, challengeEndTime, timeout int64) bool {
	return now > challengeEndTime+timeout
}

// ============================================================================
// Merkle Tree Helpers
// ============================================================================

// ComputeMerkleRoot computes a SHA256 binary merkle tree root from sorted leaf hashes.
// Leaves are sorted lexicographically before building the tree to ensure determinism.
// This matches the standard Cosmos SDK merkle tree construction.
func ComputeMerkleRoot(leaves [][]byte) []byte {
	if len(leaves) == 0 {
		return nil
	}

	// Sort leaves for determinism
	sorted := make([][]byte, len(leaves))
	copy(sorted, leaves)
	sort.Slice(sorted, func(i, j int) bool {
		return bytesLess(sorted[i], sorted[j])
	})

	// Build tree bottom-up
	level := sorted
	for len(level) > 1 {
		var nextLevel [][]byte
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				nextLevel = append(nextLevel, hashPair(level[i], level[i+1]))
			} else {
				// Odd node — promote to next level
				nextLevel = append(nextLevel, level[i])
			}
		}
		level = nextLevel
	}

	return level[0]
}

// hashPair computes SHA256(left || right) for merkle tree internal nodes.
func hashPair(left, right []byte) []byte {
	h := sha256.New()
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// bytesEqual compares two byte slices for equality.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// bytesLess returns true if a < b lexicographically.
func bytesLess(a, b []byte) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return len(a) < len(b)
}
