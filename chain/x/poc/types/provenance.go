package types

import (
	"cosmossdk.io/math"
)

// DerivationReason indicates why a contribution was classified as derivative or original.
type DerivationReason uint32

const (
	// DerivationNone means the contribution is original work.
	DerivationNone DerivationReason = 0
	// DerivationAI means the AI similarity engine flagged it as derivative.
	DerivationAI DerivationReason = 1
	// DerivationHuman means human reviewers flagged it as derivative.
	DerivationHuman DerivationReason = 2
	// DerivationExplicit means the submitter self-declared it as derivative.
	DerivationExplicit DerivationReason = 3
)

// ProvenanceEntry is the immutable registry record for an accepted contribution.
// Once written, entries are never updated or deleted.
type ProvenanceEntry struct {
	ClaimID               uint64           `json:"claim_id"`
	CanonicalHash         []byte           `json:"canonical_hash"`
	Category              string           `json:"category"`
	Submitter             string           `json:"submitter"`
	ParentClaimID         uint64           `json:"parent_claim_id"`
	IsDerivative          bool             `json:"is_derivative"`
	DerivationReason      DerivationReason `json:"derivation_reason"`
	OriginalityMultiplier math.LegacyDec   `json:"originality_multiplier"`
	QualityScore          uint32           `json:"quality_score"`
	Depth                 uint32           `json:"depth"`
	AcceptedAtHeight      int64            `json:"accepted_at_height"`
	AcceptedAtTime        int64            `json:"accepted_at_time"`
	Epoch                 uint64           `json:"epoch"`
	SchemaVersion         uint32           `json:"schema_version"`
}

// ProvenanceStats tracks aggregate registry metrics.
type ProvenanceStats struct {
	TotalEntries    uint64            `json:"total_entries"`
	RootEntries     uint64            `json:"root_entries"`
	DerivativeCount uint64            `json:"derivative_count"`
	MaxDepthSeen    uint32            `json:"max_depth_seen"`
	CategoryCounts  map[string]uint64 `json:"category_counts"`
	SchemaVersion   uint32            `json:"schema_version"`
}

// NewProvenanceStats returns an initialized ProvenanceStats with zero counters.
func NewProvenanceStats() ProvenanceStats {
	return ProvenanceStats{
		CategoryCounts: make(map[string]uint64),
		SchemaVersion:  1,
	}
}
