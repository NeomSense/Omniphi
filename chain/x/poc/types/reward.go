package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// OriginalityOverride is defined in review.go
// Legacy aliases for backward compatibility
const (
	Override_KEEP_AI                   = OverrideKeepAI
	Override_DERIVATIVE_FALSE_POSITIVE = OverrideDerivativeFalsePositive
	Override_DERIVATIVE_TRUE_POSITIVE  = OverrideDerivativeTruePositive
)

// RewardContext contains all inputs required to calculate the final reward
type RewardContext struct {
	ClaimID         uint64
	Contributor     string         // contributor address for reputation lookup
	Category        string
	QualityScore    math.LegacyDec // 0.0 to 10.0 from Layer 3
	BaseReward      math.Int       // From Params based on Category
	SimilarityScore math.LegacyDec // From Layer 2 (0.0 to 1.0)
	IsDuplicate     bool           // From Layer 1
	IsDerivative    bool           // From Layer 2
	ParentClaimID   uint64         // From Layer 2/3
	ReviewOverride  OriginalityOverride
}

// ContributorStats tracks long-term behavior for repeat offender penalties
type ContributorStats struct {
	Address               string         `json:"address"`
	TotalSubmissions      uint64         `json:"total_submissions"`
	HighSimilarityCount   uint64         `json:"high_similarity_count"` // > 85%
	DuplicateCount        uint64         `json:"duplicate_count"`
	OverturnedReviews     uint64         `json:"overturned_reviews"` // Times they contested and lost
	ReputationScore       math.LegacyDec `json:"reputation_score"`   // 0.0 to 1.0
	LastUpdatedBlock      int64          `json:"last_updated_block"`
	FraudCount            uint64         `json:"fraud_count"`              // confirmed fraud/clawback count
	TotalClawedBack       math.Int       `json:"total_clawed_back"`        // cumulative clawback amount
	CurrentBondMultiplier math.LegacyDec `json:"current_bond_multiplier"` // 1.0 = standard, increases with offenses
}

// NewContributorStats creates a new stats object with default reputation
func NewContributorStats(addr string, blockHeight int64) ContributorStats {
	return ContributorStats{
		Address:               addr,
		ReputationScore:       math.LegacyOneDec(),  // Start with 1.0 (100%)
		LastUpdatedBlock:      blockHeight,
		TotalClawedBack:       math.ZeroInt(),
		CurrentBondMultiplier: math.LegacyOneDec(), // 1.0x standard
	}
}

// RewardOutput contains the calculated results
type RewardOutput struct {
	FinalRewardAmount     math.Int
	OriginalityMultiplier math.LegacyDec
	RoyaltyAmount         math.Int
	RoyaltyRecipient      string // Address of parent claim owner (legacy single-parent)
	VestedAmount          math.Int
	ImmediateAmount       math.Int
	RoyaltyRoutes         []RoyaltyRoute `json:"royalty_routes"`      // multi-level royalty routes
	TotalRoyaltyPaid      math.Int       `json:"total_royalty_paid"`  // sum of all routes
	ClaimID               uint64         `json:"claim_id"`            // for vesting linkage
}

// ============================================================================
// Layer 4: Economic Adjustment Types
// ============================================================================

// OriginalityBand defines a configurable similarity-to-multiplier mapping.
type OriginalityBand struct {
	MinSimilarity math.LegacyDec `json:"min_similarity"` // inclusive lower bound
	MaxSimilarity math.LegacyDec `json:"max_similarity"` // exclusive upper bound
	Multiplier    math.LegacyDec `json:"multiplier"`
}

// DefaultOriginalityBands returns the default bands (matching prior hardcoded values).
func DefaultOriginalityBands() []OriginalityBand {
	return []OriginalityBand{
		{MinSimilarity: math.LegacyNewDecWithPrec(85, 2), MaxSimilarity: math.LegacyNewDecWithPrec(101, 2), Multiplier: math.LegacyNewDecWithPrec(4, 1)},  // >= 0.85: 0.4x
		{MinSimilarity: math.LegacyNewDecWithPrec(75, 2), MaxSimilarity: math.LegacyNewDecWithPrec(85, 2), Multiplier: math.LegacyNewDecWithPrec(7, 1)},   // 0.75-0.85: 0.7x
		{MinSimilarity: math.LegacyNewDecWithPrec(50, 2), MaxSimilarity: math.LegacyNewDecWithPrec(75, 2), Multiplier: math.LegacyOneDec()},               // 0.50-0.75: 1.0x
		{MinSimilarity: math.LegacyZeroDec(), MaxSimilarity: math.LegacyNewDecWithPrec(50, 2), Multiplier: math.LegacyNewDecWithPrec(12, 1)},              // < 0.50: 1.2x
	}
}

// RoyaltyRoute represents a single royalty payment in a multi-level lineage.
type RoyaltyRoute struct {
	Recipient string   `json:"recipient"` // bech32 address
	Amount    math.Int `json:"amount"`
	ClaimID   uint64   `json:"claim_id"` // ancestor claim
	Depth     uint32   `json:"depth"`    // 1 = parent, 2 = grandparent
}

// VestingStatus represents the state of a vesting schedule.
type VestingStatus uint32

const (
	VestingStatusActive     VestingStatus = 0
	VestingStatusCompleted  VestingStatus = 1
	VestingStatusClawedBack VestingStatus = 2
	VestingStatusPaused     VestingStatus = 3 // Frozen during appeal/dispute
)

// VestingSchedule tracks a per-claim vesting schedule for a contributor.
type VestingSchedule struct {
	Contributor    string        `json:"contributor"`
	ClaimID        uint64        `json:"claim_id"`
	TotalAmount    math.Int      `json:"total_amount"`
	ReleasedAmount math.Int      `json:"released_amount"`
	StartEpoch     uint64        `json:"start_epoch"`
	VestingEpochs  int64         `json:"vesting_epochs"`
	Status         VestingStatus `json:"status"`
}

// ClawbackRecord stores the result of a fraud clawback.
type ClawbackRecord struct {
	Contributor       string   `json:"contributor"`
	ClaimID           uint64   `json:"claim_id"`
	Reason            string   `json:"reason"`
	AmountClawedBack  math.Int `json:"amount_clawed_back"`
	VestingClawedBack math.Int `json:"vesting_clawed_back"`
	BondSlashed       math.Int `json:"bond_slashed"`
	ExecutedAtBlock   int64    `json:"executed_at_block"`
	Authority         string   `json:"authority"`
}

// Key Prefixes
var (
	ContributorStatsKeyPrefix = []byte{0x40} // Key for storing ContributorStats
)

// GetContributorStatsKey returns the store key for a contributor's stats
func GetContributorStatsKey(addr sdk.AccAddress) []byte {
	return append(ContributorStatsKeyPrefix, addr.Bytes()...)
}
