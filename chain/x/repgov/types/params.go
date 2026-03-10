package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// Default parameter values
var (
	// DefaultEnabled controls whether reputation weighting is active
	DefaultEnabled = false // governance-enabled

	// DefaultMaxVotingWeightCap is the maximum multiplier any single voter can have
	// Prevents whale concentration — no single voter can have more than 5x stake weight
	DefaultMaxVotingWeightCap = math.LegacyNewDec(5)

	// DefaultMinReputationThreshold is the minimum reputation score needed to receive
	// any governance weight bonus (below this, weight = 1.0x stake only)
	DefaultMinReputationThreshold = math.LegacyNewDecWithPrec(10, 2) // 0.10

	// Source weights — how much each reputation signal contributes to governance weight
	// Sum of weights determines the scaling. Each is [0, 1].

	// DefaultCScoreWeight is the weight given to PoC contribution score (C-Score)
	DefaultCScoreWeight = math.LegacyNewDecWithPrec(40, 2) // 0.40

	// DefaultEndorsementWeight is the weight given to validator endorsement participation
	DefaultEndorsementWeight = math.LegacyNewDecWithPrec(20, 2) // 0.20

	// DefaultOriginalityWeight is the weight given to originality quality (from review layer)
	DefaultOriginalityWeight = math.LegacyNewDecWithPrec(20, 2) // 0.20

	// DefaultUptimeWeight is the weight given to validator uptime record
	DefaultUptimeWeight = math.LegacyNewDecWithPrec(10, 2) // 0.10

	// DefaultLongevityWeight is the weight given to on-chain participation longevity
	DefaultLongevityWeight = math.LegacyNewDecWithPrec(10, 2) // 0.10

	// DefaultDelegationEnabled controls whether reputation delegation is allowed
	DefaultDelegationEnabled = false

	// DefaultMaxDelegationsPerAddress is the maximum number of delegations one address can receive
	DefaultMaxDelegationsPerAddress int64 = 10

	// DefaultMaxDelegableRatio is the maximum fraction of reputation that can be delegated
	DefaultMaxDelegableRatio = math.LegacyNewDecWithPrec(50, 2) // 0.50 = 50%

	// DefaultDecayRate is the per-epoch decay rate for unused governance weight
	// Encourages active participation: weight decays if you don't vote
	DefaultDecayRate = math.LegacyNewDecWithPrec(2, 2) // 0.02 = 2% per epoch

	// DefaultRecomputeInterval is how often (in blocks) voter weights are recalculated
	DefaultRecomputeInterval int64 = 100 // every epoch boundary
)

// Params defines the module parameters for x/repgov
type Params struct {
	// Enabled controls whether reputation weighting is active
	Enabled bool `json:"enabled"`

	// MaxVotingWeightCap is the maximum governance weight multiplier per voter
	MaxVotingWeightCap math.LegacyDec `json:"max_voting_weight_cap"`

	// MinReputationThreshold is the minimum reputation to receive weight bonus
	MinReputationThreshold math.LegacyDec `json:"min_reputation_threshold"`

	// Source weights
	CScoreWeight      math.LegacyDec `json:"c_score_weight"`
	EndorsementWeight math.LegacyDec `json:"endorsement_weight"`
	OriginalityWeight math.LegacyDec `json:"originality_weight"`
	UptimeWeight      math.LegacyDec `json:"uptime_weight"`
	LongevityWeight   math.LegacyDec `json:"longevity_weight"`

	// Delegation settings
	DelegationEnabled          bool           `json:"delegation_enabled"`
	MaxDelegationsPerAddress   int64          `json:"max_delegations_per_address"`
	MaxDelegableRatio          math.LegacyDec `json:"max_delegable_ratio"`

	// Decay
	DecayRate math.LegacyDec `json:"decay_rate"`

	// RecomputeInterval is how often (in blocks) to recompute voter weights
	RecomputeInterval int64 `json:"recompute_interval"`
}

// DefaultParams returns the default module parameters
func DefaultParams() Params {
	return Params{
		Enabled:                    DefaultEnabled,
		MaxVotingWeightCap:         DefaultMaxVotingWeightCap,
		MinReputationThreshold:     DefaultMinReputationThreshold,
		CScoreWeight:               DefaultCScoreWeight,
		EndorsementWeight:          DefaultEndorsementWeight,
		OriginalityWeight:          DefaultOriginalityWeight,
		UptimeWeight:               DefaultUptimeWeight,
		LongevityWeight:            DefaultLongevityWeight,
		DelegationEnabled:          DefaultDelegationEnabled,
		MaxDelegationsPerAddress:   DefaultMaxDelegationsPerAddress,
		MaxDelegableRatio:          DefaultMaxDelegableRatio,
		DecayRate:                  DefaultDecayRate,
		RecomputeInterval:          DefaultRecomputeInterval,
	}
}

// Validate performs parameter validation
func (p Params) Validate() error {
	if p.MaxVotingWeightCap.IsNil() || p.MaxVotingWeightCap.LT(math.LegacyOneDec()) {
		return fmt.Errorf("%w: max_voting_weight_cap must be >= 1.0, got %s", ErrInvalidParams, p.MaxVotingWeightCap)
	}
	if p.MaxVotingWeightCap.GT(math.LegacyNewDec(10)) {
		return fmt.Errorf("%w: max_voting_weight_cap must be <= 10.0, got %s", ErrInvalidParams, p.MaxVotingWeightCap)
	}

	if p.MinReputationThreshold.IsNil() || p.MinReputationThreshold.IsNegative() {
		return fmt.Errorf("%w: min_reputation_threshold cannot be negative", ErrInvalidParams)
	}
	if p.MinReputationThreshold.GT(math.LegacyOneDec()) {
		return fmt.Errorf("%w: min_reputation_threshold cannot exceed 1.0", ErrInvalidParams)
	}

	// Validate source weights are non-negative
	for name, w := range map[string]math.LegacyDec{
		"c_score_weight":      p.CScoreWeight,
		"endorsement_weight":  p.EndorsementWeight,
		"originality_weight":  p.OriginalityWeight,
		"uptime_weight":       p.UptimeWeight,
		"longevity_weight":    p.LongevityWeight,
	} {
		if w.IsNil() || w.IsNegative() {
			return fmt.Errorf("%w: %s must be non-negative", ErrInvalidReputationSource, name)
		}
		if w.GT(math.LegacyOneDec()) {
			return fmt.Errorf("%w: %s must be <= 1.0", ErrInvalidReputationSource, name)
		}
	}

	// Sum of weights must be > 0
	totalWeight := p.CScoreWeight.Add(p.EndorsementWeight).Add(p.OriginalityWeight).Add(p.UptimeWeight).Add(p.LongevityWeight)
	if totalWeight.IsZero() {
		return fmt.Errorf("%w: sum of source weights must be > 0", ErrInvalidReputationSource)
	}

	if p.MaxDelegationsPerAddress < 0 || p.MaxDelegationsPerAddress > 100 {
		return fmt.Errorf("%w: max_delegations_per_address must be in [0, 100]", ErrInvalidParams)
	}

	if !p.MaxDelegableRatio.IsNil() {
		if p.MaxDelegableRatio.IsNegative() || p.MaxDelegableRatio.GT(math.LegacyOneDec()) {
			return fmt.Errorf("%w: max_delegable_ratio must be in [0, 1.0]", ErrInvalidParams)
		}
	}

	if !p.DecayRate.IsNil() {
		if p.DecayRate.IsNegative() || p.DecayRate.GT(math.LegacyNewDecWithPrec(50, 2)) {
			return fmt.Errorf("%w: decay_rate must be in [0, 0.50]", ErrInvalidParams)
		}
	}

	if p.RecomputeInterval < 1 {
		return fmt.Errorf("%w: recompute_interval must be >= 1", ErrInvalidParams)
	}

	return nil
}
