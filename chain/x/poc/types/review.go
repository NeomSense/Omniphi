package types

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// Human Review Layer Types (Layer 3: PoV Override)
//
// Architecture:
//   1. ReviewSession tracks the full lifecycle of a contribution review
//   2. ReviewVote is a single reviewer's vote
//   3. ReviewerProfile tracks reviewer eligibility and performance
//   4. ReviewAppeal handles disputes against review outcomes
//   5. CoVotingRecord tracks co-voting patterns for collusion detection
// ============================================================================

// ReviewStatus represents the lifecycle state of a review session.
type ReviewStatus uint32

const (
	ReviewStatusPending  ReviewStatus = 0 // Created, awaiting reviewer assignment
	ReviewStatusInReview ReviewStatus = 1 // Reviewers assigned, voting open
	ReviewStatusAccepted ReviewStatus = 2 // Quorum accepted
	ReviewStatusRejected ReviewStatus = 3 // Quorum rejected
	ReviewStatusAppealed ReviewStatus = 4 // Appeal filed, awaiting resolution
)

func (rs ReviewStatus) String() string {
	switch rs {
	case ReviewStatusPending:
		return "PENDING"
	case ReviewStatusInReview:
		return "IN_REVIEW"
	case ReviewStatusAccepted:
		return "ACCEPTED"
	case ReviewStatusRejected:
		return "REJECTED"
	case ReviewStatusAppealed:
		return "APPEALED"
	default:
		return "UNKNOWN"
	}
}

// ReviewVoteDecision represents a reviewer's vote choice.
type ReviewVoteDecision uint32

const (
	ReviewVoteAccept      ReviewVoteDecision = 0
	ReviewVoteReject      ReviewVoteDecision = 1
	ReviewVoteRequestInfo ReviewVoteDecision = 2
)

func (d ReviewVoteDecision) String() string {
	switch d {
	case ReviewVoteAccept:
		return "ACCEPT"
	case ReviewVoteReject:
		return "REJECT"
	case ReviewVoteRequestInfo:
		return "REQUEST_INFO"
	default:
		return "UNKNOWN"
	}
}

func (d ReviewVoteDecision) IsValid() bool {
	return d <= ReviewVoteRequestInfo
}

// OriginalityOverride represents a reviewer's override of the AI similarity decision.
type OriginalityOverride uint32

const (
	OverrideKeepAI                     OriginalityOverride = 0 // Accept AI decision
	OverrideDerivativeFalsePositive    OriginalityOverride = 1 // AI said derivative, human says original
	OverrideDerivativeTruePositive     OriginalityOverride = 2 // Confirm AI derivative flag
	OverrideNotDerivativeFalseNegative OriginalityOverride = 3 // AI missed derivative, human flags it
)

func (o OriginalityOverride) String() string {
	switch o {
	case OverrideKeepAI:
		return "KEEP_AI"
	case OverrideDerivativeFalsePositive:
		return "OVERRIDE_DERIVATIVE_FALSE_POSITIVE"
	case OverrideDerivativeTruePositive:
		return "OVERRIDE_DERIVATIVE_TRUE_POSITIVE"
	case OverrideNotDerivativeFalseNegative:
		return "OVERRIDE_NOT_DERIVATIVE_FALSE_NEGATIVE"
	default:
		return "UNKNOWN"
	}
}

func (o OriginalityOverride) IsValid() bool {
	return o <= OverrideNotDerivativeFalseNegative
}

// Limits
const (
	MaxNotesPointerLength = 256
	MaxAppealReasonLength = 512
	MaxQualityScore       = 100
	MaxReviewersPerClaim  = 15
	MinReviewersPerClaim  = 1
	MaxReviewVotePeriod   = 86400 // ~24h at 1s blocks
	MinReviewVotePeriod   = 100
)

// ReviewVote is a single reviewer's vote on a contribution.
type ReviewVote struct {
	Reviewer            string              `json:"reviewer"`              // bech32 address
	Decision            ReviewVoteDecision  `json:"decision"`              // accept/reject/request-info
	OriginalityOverride OriginalityOverride `json:"originality_override"`  // override AI similarity
	QualityScore        uint32              `json:"quality_score"`         // 0-100
	NotesPointer        string              `json:"notes_pointer"`         // IPFS URI (optional)
	ParentClaimOverride uint64              `json:"parent_claim_override"` // 0 = no override
	BlockHeight         int64               `json:"block_height"`          // when vote was cast
}

// ReviewSession is the full review lifecycle record for a contribution.
type ReviewSession struct {
	ContributionID    uint64              `json:"contribution_id"`
	Status            ReviewStatus        `json:"status"`
	AssignedReviewers []string            `json:"assigned_reviewers"` // bech32 addresses
	Votes             []ReviewVote        `json:"votes"`
	StartHeight       int64               `json:"start_height"`   // block when review started
	EndHeight         int64               `json:"end_height"`     // start_height + ReviewVotePeriod
	FinalDecision     ReviewVoteDecision  `json:"final_decision"` // majority vote result
	OverrideApplied   OriginalityOverride `json:"override_applied"`
	FinalQuality      uint32              `json:"final_quality"` // average quality score
	AppealID          uint64              `json:"appeal_id"`     // 0 = no appeal
	RandomSeed        []byte              `json:"random_seed"`   // seed used for assignment
}

func (rs *ReviewSession) Marshal() ([]byte, error) { return json.Marshal(rs) }
func (rs *ReviewSession) Unmarshal(bz []byte) error { return json.Unmarshal(bz, rs) }

// HasVoted returns true if the given reviewer has already cast a vote.
func (rs *ReviewSession) HasVoted(reviewer string) bool {
	for _, v := range rs.Votes {
		if v.Reviewer == reviewer {
			return true
		}
	}
	return false
}

// IsAssignedReviewer returns true if the given address is an assigned reviewer.
func (rs *ReviewSession) IsAssignedReviewer(addr string) bool {
	for _, r := range rs.AssignedReviewers {
		if r == addr {
			return true
		}
	}
	return false
}

// AllVotesCast returns true if all assigned reviewers have voted.
func (rs *ReviewSession) AllVotesCast() bool {
	return len(rs.Votes) >= len(rs.AssignedReviewers)
}

// ReviewerProfile tracks a reviewer's eligibility and performance.
type ReviewerProfile struct {
	Address          string `json:"address"`
	TotalReviews     uint64 `json:"total_reviews"`
	AcceptedReviews  uint64 `json:"accepted_reviews"`  // votes matching final outcome
	RejectedReviews  uint64 `json:"rejected_reviews"`  // votes opposing final outcome
	AvgQualityScore  uint32 `json:"avg_quality_score"`
	ReputationScore  uint64 `json:"reputation_score"` // from PoC credits
	LastReviewHeight int64  `json:"last_review_height"`
	BondedAmount     string `json:"bonded_amount"` // currently locked bond
	SlashedCount     uint64 `json:"slashed_count"` // number of times slashed
	Suspended        bool   `json:"suspended"`     // if true, ineligible
}

func (rp *ReviewerProfile) Marshal() ([]byte, error) { return json.Marshal(rp) }
func (rp *ReviewerProfile) Unmarshal(bz []byte) error { return json.Unmarshal(bz, rp) }

// ReviewAppeal is an appeal against a review outcome.
type ReviewAppeal struct {
	AppealID         uint64 `json:"appeal_id"`
	ContributionID   uint64 `json:"contribution_id"`
	Appellant        string `json:"appellant"`     // bech32
	Reason           string `json:"reason"`        // text (max 512 chars)
	AppealBond       string `json:"appeal_bond"`   // coin string
	FiledAtHeight    int64  `json:"filed_at_height"`
	ResolvedAtHeight int64  `json:"resolved_at_height"`
	Resolved         bool   `json:"resolved"`
	Upheld           bool   `json:"upheld"`         // true = original verdict stands
	ResolverNotes    string `json:"resolver_notes"` // IPFS URI
}

func (ra *ReviewAppeal) Marshal() ([]byte, error) { return json.Marshal(ra) }
func (ra *ReviewAppeal) Unmarshal(bz []byte) error { return json.Unmarshal(bz, ra) }

// CoVotingRecord tracks co-voting patterns between a reviewer pair.
type CoVotingRecord struct {
	ReviewerA      string `json:"reviewer_a"`
	ReviewerB      string `json:"reviewer_b"`
	SameVoteCount  uint64 `json:"same_vote_count"`
	TotalPairCount uint64 `json:"total_pair_count"`
	LastUpdated    int64  `json:"last_updated"` // block height
}

func (cvr *CoVotingRecord) Marshal() ([]byte, error) { return json.Marshal(cvr) }
func (cvr *CoVotingRecord) Unmarshal(bz []byte) error { return json.Unmarshal(bz, cvr) }

// ReviewBondEscrow tracks a reviewer's escrowed bond for a specific review.
type ReviewBondEscrow struct {
	Reviewer       string `json:"reviewer"`
	ContributionID uint64 `json:"contribution_id"`
	Amount         string `json:"amount"` // coin string
}

func (rbe *ReviewBondEscrow) Marshal() ([]byte, error) { return json.Marshal(rbe) }
func (rbe *ReviewBondEscrow) Unmarshal(bz []byte) error { return json.Unmarshal(bz, rbe) }

// ============================================================================
// Unified ClaimStatus (Pipeline State Machine)
//
// Tracks a contribution's progress across all 5 layers.
// ============================================================================

// ClaimStatus represents the unified pipeline status of a contribution.
type ClaimStatus uint32

const (
	ClaimStatusSubmitted          ClaimStatus = 0 // Layer 1: just submitted
	ClaimStatusDuplicate          ClaimStatus = 1 // Layer 1: exact duplicate detected (terminal)
	ClaimStatusAwaitingSimilarity ClaimStatus = 2 // Layer 2: pending oracle analysis
	ClaimStatusFlaggedDerivative  ClaimStatus = 3 // Layer 2: flagged as derivative
	ClaimStatusInReview           ClaimStatus = 4 // Layer 3: human review active
	ClaimStatusAccepted           ClaimStatus = 5 // Layer 3: accepted
	ClaimStatusRejected           ClaimStatus = 6 // Layer 3: rejected
	ClaimStatusDisputed           ClaimStatus = 7 // Layer 3: appeal filed
	ClaimStatusResolved           ClaimStatus = 8 // Layer 3: appeal resolved (terminal)
)

func (cs ClaimStatus) String() string {
	switch cs {
	case ClaimStatusSubmitted:
		return "SUBMITTED"
	case ClaimStatusDuplicate:
		return "DUPLICATE"
	case ClaimStatusAwaitingSimilarity:
		return "AWAITING_SIMILARITY"
	case ClaimStatusFlaggedDerivative:
		return "FLAGGED_DERIVATIVE"
	case ClaimStatusInReview:
		return "IN_REVIEW"
	case ClaimStatusAccepted:
		return "ACCEPTED"
	case ClaimStatusRejected:
		return "REJECTED"
	case ClaimStatusDisputed:
		return "DISPUTED"
	case ClaimStatusResolved:
		return "RESOLVED"
	default:
		return "UNKNOWN"
	}
}

// validTransitions defines the allowed state transitions in the pipeline.
var validTransitions = map[ClaimStatus][]ClaimStatus{
	ClaimStatusSubmitted:          {ClaimStatusDuplicate, ClaimStatusAwaitingSimilarity},
	ClaimStatusAwaitingSimilarity: {ClaimStatusFlaggedDerivative, ClaimStatusInReview},
	ClaimStatusFlaggedDerivative:  {ClaimStatusInReview},
	ClaimStatusInReview:           {ClaimStatusAccepted, ClaimStatusRejected},
	ClaimStatusAccepted:           {ClaimStatusDisputed},
	ClaimStatusRejected:           {ClaimStatusDisputed},
	ClaimStatusDisputed:           {ClaimStatusResolved},
	// ClaimStatusDuplicate and ClaimStatusResolved are terminal — no further transitions
}

// ValidateClaimTransition checks that a status transition is allowed.
func ValidateClaimTransition(from, to ClaimStatus) error {
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("no transitions allowed from status %s (%d)", from, from)
	}
	for _, a := range allowed {
		if a == to {
			return nil
		}
	}
	return fmt.Errorf("invalid claim status transition: %s -> %s", from, to)
}
