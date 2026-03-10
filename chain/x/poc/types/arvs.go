package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// ============================================================================
// Adaptive Reward Vesting System (ARVS)
//
// Architecture:
//   1. RiskScore calculation — weighted combination of 5 risk signals
//   2. VestingProfile selection — maps RiskScore range → unlock schedule
//   3. UnlockStage — (percent, delay_epochs) tuple defining tranched release
//   4. BountyDistribution — how slashed rewards are allocated on fraud confirm
//   5. CategoryRiskLevel — per-category risk weight for the pipeline
//
// The ARVS replaces the fixed ImmediateRewardRatio + VestingEpochs split with a
// fully dynamic, risk-scored profile system. Trust score, similarity, category
// risk, verifier confidence, and dispute probability all drive vesting speed.
// ============================================================================

// ============================================================================
// 1. Category Risk Levels
// ============================================================================

// CategoryRisk defines how risky a contribution category is.
// Higher risk → slower vesting.
type CategoryRisk uint32

const (
	CategoryRiskLow    CategoryRisk = 1 // documentation, translations, UI
	CategoryRiskMedium CategoryRisk = 2 // standard code, integrations
	CategoryRiskHigh   CategoryRisk = 3 // security modules, consensus, economic params
)

// DefaultCategoryRiskMap returns the default risk level per contribution type.
func DefaultCategoryRiskMap() map[string]CategoryRisk {
	return map[string]CategoryRisk{
		"documentation": CategoryRiskLow,
		"translation":   CategoryRiskLow,
		"ui":            CategoryRiskLow,
		"analytics":     CategoryRiskMedium,
		"integration":   CategoryRiskMedium,
		"code":          CategoryRiskMedium,
		"security":      CategoryRiskHigh,
		"consensus":     CategoryRiskHigh,
		"economic":      CategoryRiskHigh,
	}
}

// CategoryRiskLevel returns the risk level for a category string.
// Defaults to Medium if not found.
func CategoryRiskLevel(category string, riskMap map[string]CategoryRisk) CategoryRisk {
	if r, ok := riskMap[category]; ok {
		return r
	}
	return CategoryRiskMedium
}

// ============================================================================
// 2. Unlock Stages
// ============================================================================

// UnlockStage defines a single tranche release in a vesting profile.
type UnlockStage struct {
	// UnlockPercent is the fraction of the vested reward unlocked at this stage.
	// Stored as basis points (0-10000). Sum across all stages in a profile MUST be 10000.
	UnlockBps uint32 `json:"unlock_bps"`

	// DelayEpochs is the number of epochs after vesting start before this tranche unlocks.
	// Stage 0 (immediate) has DelayEpochs == 0.
	DelayEpochs int64 `json:"delay_epochs"`
}

// Validate checks that an UnlockStage is well-formed.
func (s UnlockStage) Validate() error {
	if s.UnlockBps == 0 || s.UnlockBps > 10000 {
		return fmt.Errorf("unlock_bps must be 1-10000, got %d", s.UnlockBps)
	}
	if s.DelayEpochs < 0 {
		return fmt.Errorf("delay_epochs cannot be negative, got %d", s.DelayEpochs)
	}
	return nil
}

// ============================================================================
// 3. Vesting Profiles
// ============================================================================

// VestingProfileID identifies a vesting profile.
type VestingProfileID uint32

const (
	VestingProfileLowRisk        VestingProfileID = 1
	VestingProfileMediumRisk     VestingProfileID = 2
	VestingProfileHighRisk       VestingProfileID = 3
	VestingProfileDerivative     VestingProfileID = 4
	VestingProfileRepeatOffender VestingProfileID = 5 // extra-slow, set by repeat-offender logic
)

// VestingProfile defines the multi-stage unlock schedule for a risk tier.
type VestingProfile struct {
	// ProfileID uniquely identifies this profile.
	ProfileID VestingProfileID `json:"profile_id"`

	// Name is a human-readable label (e.g., "low_risk", "high_risk").
	Name string `json:"name"`

	// Stages defines the sequence of unlock tranches. Must sum to 10000 bps (100%).
	Stages []UnlockStage `json:"stages"`
}

// Validate checks that all stages sum to 10000 bps.
func (p VestingProfile) Validate() error {
	if len(p.Stages) == 0 {
		return fmt.Errorf("vesting profile %d has no stages", p.ProfileID)
	}
	var total uint32
	for i, s := range p.Stages {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("stage %d: %w", i, err)
		}
		total += s.UnlockBps
	}
	if total != 10000 {
		return fmt.Errorf("vesting profile %d stages sum to %d bps, must be 10000", p.ProfileID, total)
	}
	return nil
}

// DefaultVestingProfiles returns the four built-in ARVS profiles.
// These can be overridden by governance.
//
//   Low Risk:    60% immediate, 25% @ 30 epochs, 15% @ 60 epochs
//   Medium Risk: 30% immediate, 40% @ 30 epochs, 30% @ 90 epochs
//   High Risk:   10% immediate, 40% @ 60 epochs, 50% @ 120 epochs
//   Derivative:  15% immediate, 35% @ 60 epochs, 50% @ 120 epochs
//   Repeat Off:   5% immediate, 30% @ 90 epochs, 65% @ 180 epochs
func DefaultVestingProfiles() []VestingProfile {
	return []VestingProfile{
		{
			ProfileID: VestingProfileLowRisk,
			Name:      "low_risk",
			Stages: []UnlockStage{
				{UnlockBps: 6000, DelayEpochs: 0},  // 60% immediate
				{UnlockBps: 2500, DelayEpochs: 30}, // 25% @ 30 epochs
				{UnlockBps: 1500, DelayEpochs: 60}, // 15% @ 60 epochs
			},
		},
		{
			ProfileID: VestingProfileMediumRisk,
			Name:      "medium_risk",
			Stages: []UnlockStage{
				{UnlockBps: 3000, DelayEpochs: 0},  // 30% immediate
				{UnlockBps: 4000, DelayEpochs: 30}, // 40% @ 30 epochs
				{UnlockBps: 3000, DelayEpochs: 90}, // 30% @ 90 epochs
			},
		},
		{
			ProfileID: VestingProfileHighRisk,
			Name:      "high_risk",
			Stages: []UnlockStage{
				{UnlockBps: 1000, DelayEpochs: 0},   // 10% immediate
				{UnlockBps: 4000, DelayEpochs: 60},  // 40% @ 60 epochs
				{UnlockBps: 5000, DelayEpochs: 120}, // 50% @ 120 epochs
			},
		},
		{
			ProfileID: VestingProfileDerivative,
			Name:      "derivative",
			Stages: []UnlockStage{
				{UnlockBps: 1500, DelayEpochs: 0},   // 15% immediate
				{UnlockBps: 3500, DelayEpochs: 60},  // 35% @ 60 epochs
				{UnlockBps: 5000, DelayEpochs: 120}, // 50% @ 120 epochs
			},
		},
		{
			ProfileID: VestingProfileRepeatOffender,
			Name:      "repeat_offender",
			Stages: []UnlockStage{
				{UnlockBps: 500,  DelayEpochs: 0},   // 5% immediate
				{UnlockBps: 3000, DelayEpochs: 90},  // 30% @ 90 epochs
				{UnlockBps: 6500, DelayEpochs: 180}, // 65% @ 180 epochs
			},
		},
	}
}

// ============================================================================
// 4. Risk Score Calculation
// ============================================================================

// ARVSWeights defines the weights used in the RiskScore formula.
// All weights are in basis points (sum should equal 10000).
type ARVSWeights struct {
	// CategoryWeight is the weight applied to CategoryRiskLevel (1-3).
	CategoryWeight uint32 `json:"category_weight_bps"` // default: 2500

	// ReputationWeight is applied to (1 - trust_score), so lower trust → higher risk.
	ReputationWeight uint32 `json:"reputation_weight_bps"` // default: 3000

	// OriginalityWeight is applied to the similarity score (higher sim → more risk).
	OriginalityWeight uint32 `json:"originality_weight_bps"` // default: 2500

	// ConfidenceWeight is applied to (1 - verifier_confidence).
	ConfidenceWeight uint32 `json:"confidence_weight_bps"` // default: 1500

	// DisputeWeight is applied to the historical dispute rate for this contributor.
	DisputeWeight uint32 `json:"dispute_weight_bps"` // default: 500
}

// DefaultARVSWeights returns the default ARVS weight configuration.
func DefaultARVSWeights() ARVSWeights {
	return ARVSWeights{
		CategoryWeight:    2500,
		ReputationWeight:  3000,
		OriginalityWeight: 2500,
		ConfidenceWeight:  1500,
		DisputeWeight:     500,
	}
}

// Validate ensures weights sum to 10000.
func (w ARVSWeights) Validate() error {
	total := w.CategoryWeight + w.ReputationWeight + w.OriginalityWeight + w.ConfidenceWeight + w.DisputeWeight
	if total != 10000 {
		return fmt.Errorf("ARVS weights must sum to 10000 bps, got %d", total)
	}
	return nil
}

// RiskScoreInput contains all signals used for RiskScore computation.
type RiskScoreInput struct {
	// CategoryRisk is 1 (low), 2 (medium), or 3 (high). Normalised to [0, 1] by dividing by 3.
	CategoryRisk CategoryRisk

	// TrustScore is the contributor's reputation in [0.0, 1.0]. High trust → low risk.
	TrustScore math.LegacyDec

	// SimilarityScore is the AI originality score in [0.0, 1.0]. High sim → high risk.
	SimilarityScore math.LegacyDec

	// VerifierConfidence is the average verifier confidence in [0.0, 1.0].
	// Low confidence → high risk. Defaults to 1.0 if not available.
	VerifierConfidence math.LegacyDec

	// DisputeRate is the historical dispute rate for this contributor in [0.0, 1.0].
	// (overturnedReviews / totalSubmissions). Defaults to 0.0 for new contributors.
	DisputeRate math.LegacyDec

	// IsDerivative forces the derivative profile if true, bypassing risk score.
	IsDerivative bool

	// IsRepeatOffender bypasses normal profile and assigns the repeat offender schedule.
	IsRepeatOffender bool
}

// ============================================================================
// 5. Bounty Distribution
// ============================================================================

// BountyDistribution defines how slashed rewards are allocated when fraud is confirmed.
type BountyDistribution struct {
	// ChallengerBps is the fraction paid to the challenger who filed the dispute.
	ChallengerBps uint32 `json:"challenger_bps"` // default: 4000 (40%)

	// BurnBps is the fraction burned permanently.
	BurnBps uint32 `json:"burn_bps"` // default: 3000 (30%)

	// TreasuryBps is the fraction sent to the treasury/community pool.
	TreasuryBps uint32 `json:"treasury_bps"` // default: 2000 (20%)

	// ReviewerPenaltyPoolBps is the fraction added to the reviewer penalty pool.
	// Used to compensate reviewers who correctly flagged fraud in later epochs.
	ReviewerPenaltyPoolBps uint32 `json:"reviewer_penalty_pool_bps"` // default: 1000 (10%)
}

// DefaultBountyDistribution returns the default 40/30/20/10 split.
func DefaultBountyDistribution() BountyDistribution {
	return BountyDistribution{
		ChallengerBps:          4000,
		BurnBps:                3000,
		TreasuryBps:            2000,
		ReviewerPenaltyPoolBps: 1000,
	}
}

// Validate ensures fractions sum to 10000 bps.
func (b BountyDistribution) Validate() error {
	total := b.ChallengerBps + b.BurnBps + b.TreasuryBps + b.ReviewerPenaltyPoolBps
	if total != 10000 {
		return fmt.Errorf("bounty distribution must sum to 10000 bps, got %d", total)
	}
	return nil
}

// ============================================================================
// 6. ARVS Stage Vesting Schedule
// ============================================================================

// ARVSStageEntry tracks the state of a single unlock tranche.
type ARVSStageEntry struct {
	// StageIndex within the profile (0-based).
	StageIndex uint32 `json:"stage_index"`

	// UnlockAtEpoch is the absolute epoch when this tranche becomes unlockable.
	UnlockAtEpoch uint64 `json:"unlock_at_epoch"`

	// Amount is the token amount for this tranche.
	Amount math.Int `json:"amount"`

	// Released tracks whether this stage has been paid out.
	Released bool `json:"released"`
}

// ARVSVestingSchedule is the rich replacement for the simple VestingSchedule.
// It stores per-stage entries instead of linear vesting.
type ARVSVestingSchedule struct {
	Contributor string `json:"contributor"`
	ClaimID     uint64 `json:"claim_id"`

	// ProfileID records which vesting profile was applied.
	ProfileID VestingProfileID `json:"profile_id"`

	// RiskScore in [0, 10000] bps. Stored for observability/governance.
	RiskScoreBps uint32 `json:"risk_score_bps"`

	// TotalAmount is the total reward covered by this schedule (sum of all stages).
	TotalAmount math.Int `json:"total_amount"`

	// ReleasedAmount tracks cumulative paid-out tokens.
	ReleasedAmount math.Int `json:"released_amount"`

	// Stages holds each tranche entry.
	Stages []ARVSStageEntry `json:"stages"`

	// Status mirrors VestingStatus for consistency with existing code.
	Status VestingStatus `json:"status"`

	// StartEpoch when this schedule was created.
	StartEpoch uint64 `json:"start_epoch"`
}
