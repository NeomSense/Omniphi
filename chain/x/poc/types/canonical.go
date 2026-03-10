package types

import (
	"encoding/hex"
	"fmt"
)

// ============================================================================
// Canonical Hash Layer Types (Layer 1 Deduplication)
// ============================================================================

// ClaimRecord links a canonical hash to a specific contribution claim.
// Multiple ClaimRecords may exist per hash (collision handling).
type ClaimRecord struct {
	// ClaimID is the contribution ID that registered this canonical hash
	ClaimID uint64 `json:"claim_id"`
	// Submitter is the bech32 address of the original contributor
	Submitter string `json:"submitter"`
	// Category is the contribution type (e.g., "code", "docs", "dataset")
	Category string `json:"category"`
	// StoragePointer is the off-chain URI (IPFS/Arweave/Git)
	StoragePointer string `json:"storage_pointer"`
	// BlockHeight when this claim was registered
	BlockHeight int64 `json:"block_height"`
	// SpecVersion is the canonical normalization spec version used
	SpecVersion uint32 `json:"spec_version"`
}

// CanonicalRegistry stores all claims for a given canonical hash.
// Supports hash collisions by storing multiple claims per hash.
type CanonicalRegistry struct {
	// CanonicalHash is the SHA-256 hash of the canonicalized content (32 bytes)
	CanonicalHash []byte `json:"canonical_hash"`
	// Claims is the list of contributions that share this canonical hash
	Claims []ClaimRecord `json:"claims"`
}

// DuplicateRecord tracks a contribution flagged as a duplicate.
type DuplicateRecord struct {
	// ContributionID is the duplicate contribution
	ContributionID uint64 `json:"contribution_id"`
	// CanonicalHash is the matching canonical hash
	CanonicalHash []byte `json:"canonical_hash"`
	// OriginalClaimID is the first registered contribution with this hash
	OriginalClaimID uint64 `json:"original_claim_id"`
	// OriginalSubmitter is the original contributor's address
	OriginalSubmitter string `json:"original_submitter"`
}

// BondEscrowRecord tracks an escrowed bond for a submission.
type BondEscrowRecord struct {
	// ContributionID is the contribution this bond covers
	ContributionID uint64 `json:"contribution_id"`
	// Contributor is the bech32 address that posted the bond
	Contributor string `json:"contributor"`
	// Amount is the bond denomination and amount
	Amount string `json:"amount"`
}

// Submission status constants for canonical hash checks.
const (
	// StatusDuplicate marks a submission as an exact duplicate
	StatusDuplicate = "DUPLICATE"
	// StatusPendingSimilarity marks a new submission pending further similarity analysis
	StatusPendingSimilarity = "PENDING_SIMILARITY"
)

// Canonical specification versioning.
const (
	// CurrentCanonicalSpecVersion is the latest supported normalization spec version.
	// Increment when normalization rules change (breaking change).
	CurrentCanonicalSpecVersion uint32 = 1

	// CanonicalHashSize is the fixed size of canonical hashes (SHA-256 = 32 bytes).
	CanonicalHashSize = 32
)

// ValidateCanonicalHash checks that a canonical hash has valid format.
func ValidateCanonicalHash(hash []byte) error {
	if len(hash) != CanonicalHashSize {
		return fmt.Errorf("canonical hash must be %d bytes, got %d", CanonicalHashSize, len(hash))
	}

	// Reject all-zero hash
	allZero := true
	for _, b := range hash {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return fmt.Errorf("canonical hash cannot be all zeros")
	}

	// Reject all-ones hash
	allOnes := true
	for _, b := range hash {
		if b != 0xFF {
			allOnes = false
			break
		}
	}
	if allOnes {
		return fmt.Errorf("canonical hash cannot be all ones")
	}

	return nil
}

// CanonicalHashHex returns the hex-encoded string of a canonical hash.
func CanonicalHashHex(hash []byte) string {
	return hex.EncodeToString(hash)
}
