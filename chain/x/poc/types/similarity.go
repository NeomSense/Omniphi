package types

import (
	"encoding/hex"
	"fmt"
)

// ============================================================================
// Similarity Engine Types (Layer 2: Oracle-Verified Content Similarity)
//
// Architecture:
//   Off-chain Oracle computes similarity → produces CompactData + Full Vector
//   Oracle signs both pieces separately (dual signature):
//     1. OracleSignatureCompact = Sign(CompactData)
//     2. OracleSignatureFull    = Sign(CommitmentHashFull)
//   On-chain: verify both signatures, apply thresholds, flag derivatives
// ============================================================================

// SimilarityCompactData is the small, public oracle output submitted on-chain.
// The oracle signs this struct directly.
type SimilarityCompactData struct {
	// ContributionID is the contribution being analyzed
	ContributionID uint64 `json:"contribution_id"`
	// OverallSimilarity is a scaled uint32 where 10000 = 100.00%
	// e.g., 8500 = 85.00% similarity to nearest parent
	OverallSimilarity uint32 `json:"overall_similarity"`
	// Confidence is the oracle's confidence in the similarity score (0-10000 scaled)
	Confidence uint32 `json:"confidence"`
	// NearestParentClaimID is the contribution ID most similar to this one (0 = none found)
	NearestParentClaimID uint64 `json:"nearest_parent_claim_id"`
	// ModelVersion identifies which similarity model produced this result
	ModelVersion string `json:"model_version"`
	// Epoch is the similarity epoch at the time of computation (anti-replay)
	Epoch uint64 `json:"epoch"`
}

// Validate performs basic validation of SimilarityCompactData.
func (d *SimilarityCompactData) Validate() error {
	if d.ContributionID == 0 {
		return fmt.Errorf("contribution_id must be > 0")
	}
	if d.OverallSimilarity > 10000 {
		return fmt.Errorf("overall_similarity must be <= 10000 (100.00%%), got %d", d.OverallSimilarity)
	}
	if d.Confidence > 10000 {
		return fmt.Errorf("confidence must be <= 10000, got %d", d.Confidence)
	}
	if d.ModelVersion == "" {
		return fmt.Errorf("model_version cannot be empty")
	}
	if len(d.ModelVersion) > 64 {
		return fmt.Errorf("model_version too long: max 64 chars, got %d", len(d.ModelVersion))
	}
	if d.Epoch == 0 {
		return fmt.Errorf("epoch must be > 0")
	}
	return nil
}

// SimilarityCommitmentRecord is the on-chain record stored after a successful similarity commitment.
type SimilarityCommitmentRecord struct {
	// ContributionID is the contribution this record belongs to
	ContributionID uint64 `json:"contribution_id"`
	// CompactData is the oracle's similarity analysis
	CompactData SimilarityCompactData `json:"compact_data"`
	// CommitmentHashFull is the hash of the full similarity vector (for future disputes)
	CommitmentHashFull []byte `json:"commitment_hash_full"`
	// OracleAddress is the bech32 address of the oracle that signed this commitment
	OracleAddress string `json:"oracle_address"`
	// BlockHeight is the block at which this commitment was stored
	BlockHeight int64 `json:"block_height"`
	// IsDerivative is true if OverallSimilarity >= DerivativeThreshold
	IsDerivative bool `json:"is_derivative"`
}

// MaxOracleSignatureLength is the maximum allowed length for oracle signatures.
// Ed25519 = 64 bytes, secp256k1 = 64-65 bytes. Allow up to 128 for future-proofing.
const MaxOracleSignatureLength = 128

// MaxCommitmentHashLength is the expected length for the full vector commitment hash (SHA-256).
const MaxCommitmentHashLength = 32

// MaxModelVersionLength is the maximum length for the model version string.
const MaxModelVersionLength = 64

// MaxOracleAllowlistSize is the maximum number of oracles in the allowlist.
const MaxOracleAllowlistSize = 20

// SimilarityScaleMax is the maximum value for scaled similarity/confidence (100.00%).
const SimilarityScaleMax uint32 = 10000

// ValidateCommitmentHash validates a full vector commitment hash.
func ValidateCommitmentHash(hash []byte) error {
	if len(hash) != MaxCommitmentHashLength {
		return fmt.Errorf("commitment hash must be %d bytes (SHA-256), got %d", MaxCommitmentHashLength, len(hash))
	}
	// Reject all-zeros
	allZeros := true
	for _, b := range hash {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		return fmt.Errorf("commitment hash cannot be all zeros")
	}
	return nil
}

// CommitmentHashHex returns the hex string representation of a commitment hash.
func CommitmentHashHex(hash []byte) string {
	return hex.EncodeToString(hash)
}
