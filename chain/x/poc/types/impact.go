package types

import "cosmossdk.io/math"

// ============================================================================
// Layer 5: Utility & Impact Scoring Types
// ============================================================================

// ContributionImpactRecord tracks the long-term ecosystem value of an accepted
// contribution. Updated incrementally each epoch by the impact scoring engine.
type ContributionImpactRecord struct {
	// ClaimID is the contribution's unique ID.
	ClaimID uint64 `json:"claim_id"`

	// UtilityScore is a [0, 100] score reflecting observed downstream usage:
	// reuse count, invocations, validator adoption weighted by reputation.
	UtilityScore uint32 `json:"utility_score"`

	// ImpactScore is a [0, 100] score reflecting long-term ecosystem value:
	// dependency depth, cross-category references, historical trajectory.
	ImpactScore uint32 `json:"impact_score"`

	// ImpactMultiplierBps is the derived multiplier applied to future rewards
	// for the contributor. Bounds: [0.8, 1.5] (stored as basis points: 8000–15000).
	ImpactMultiplierBps uint32 `json:"impact_multiplier_bps"`

	// ReuseCount is the number of distinct contributions that reference this claim
	// as a parent (via provenance DAG). Self-references are excluded.
	ReuseCount uint32 `json:"reuse_count"`

	// DependencyCount is the count of unique contributors (distinct addresses)
	// whose accepted contributions depend on this claim.
	DependencyCount uint32 `json:"dependency_count"`

	// InvocationCount is the number of times this contribution's artifact has been
	// referenced by on-chain transactions (e.g., UCI calls). Starts at 0.
	InvocationCount uint64 `json:"invocation_count"`

	// ValidatorAdoptionCount is the number of distinct validators that have endorsed
	// or invoked this contribution in attestations.
	ValidatorAdoptionCount uint32 `json:"validator_adoption_count"`

	// LastUpdatedEpoch is the epoch number when this record was last recalculated.
	LastUpdatedEpoch uint64 `json:"last_updated_epoch"`

	// AnomalyFlag is set when graph anomaly detection identifies suspicious
	// reference patterns (e.g., circular loops, coordinated self-boosting).
	AnomalyFlag bool `json:"anomaly_flag"`
}

// ContributorImpactProfile aggregates impact across all of a contributor's claims.
// Updated whenever any of the contributor's ContributionImpactRecords change.
type ContributorImpactProfile struct {
	// Address is the contributor's bech32 address.
	Address string `json:"address"`

	// AggregateImpactScore is the sum of ImpactScore across all accepted claims,
	// capped at 10000 to prevent unbounded accumulation.
	AggregateImpactScore uint32 `json:"aggregate_impact_score"`

	// AverageUtilityScore is the rolling average UtilityScore across accepted claims.
	AverageUtilityScore uint32 `json:"average_utility_score"`

	// HighImpactCount is the number of contributions with ImpactScore >= 70.
	HighImpactCount uint32 `json:"high_impact_count"`

	// LowImpactPatternCount counts consecutive low-impact submissions (score < 20).
	// Triggers TrustAdjustmentFactor reduction after threshold.
	LowImpactPatternCount uint32 `json:"low_impact_pattern_count"`

	// TrustAdjustmentFactor modifies the ImpactMultiplier for this contributor.
	// Stored as basis points: 10000 = 1.0 (neutral). Range: [5000, 15000].
	TrustAdjustmentFactor uint32 `json:"trust_adjustment_factor_bps"`

	// LastUpdatedEpoch is the epoch when this profile was last recalculated.
	LastUpdatedEpoch uint64 `json:"last_updated_epoch"`

	// TotalTrackedClaims is the count of accepted claims with impact records.
	TotalTrackedClaims uint32 `json:"total_tracked_claims"`
}

// ContributionUsageEdge records a single reference in the usage graph.
// Each edge links a parent claim (being depended on) to a child claim (the referrer).
// This is a superset of the provenance DAG — it captures runtime invocations too.
type ContributionUsageEdge struct {
	// ParentClaimID is the claim being referenced.
	ParentClaimID uint64 `json:"parent_claim_id"`

	// ChildClaimID is the claim that references the parent.
	ChildClaimID uint64 `json:"child_claim_id"`

	// ReferenceType distinguishes how the reference was created.
	// "provenance" = provenance DAG link, "invocation" = runtime UCI call,
	// "endorsement" = validator attestation reference.
	ReferenceType string `json:"reference_type"`

	// ChildContributor is the address of the contributor of the child claim.
	ChildContributor string `json:"child_contributor"`

	// Timestamp is the block time (Unix seconds) when the edge was recorded.
	Timestamp int64 `json:"timestamp"`

	// Epoch is the epoch when the edge was recorded.
	Epoch uint64 `json:"epoch"`
}

// ImpactParams holds all Layer 5 governance parameters.
// Stored as a JSON sidecar to avoid proto field descriptor regeneration.
type ImpactParams struct {
	// MultiplierMinBps is the lower bound of the impact multiplier (default: 8000 = 0.8x).
	MultiplierMinBps uint32 `json:"multiplier_min_bps"`

	// MultiplierMaxBps is the upper bound of the impact multiplier (default: 15000 = 1.5x).
	MultiplierMaxBps uint32 `json:"multiplier_max_bps"`

	// UtilityReuseWeight is the basis-point weight for ReuseCount in utility scoring (default: 4000).
	UtilityReuseWeight uint32 `json:"utility_reuse_weight_bps"`

	// UtilityInvocationWeight is the weight for InvocationCount (default: 3000).
	UtilityInvocationWeight uint32 `json:"utility_invocation_weight_bps"`

	// UtilityValidatorWeight is the weight for ValidatorAdoptionCount (default: 3000).
	UtilityValidatorWeight uint32 `json:"utility_validator_weight_bps"`

	// ImpactUtilityWeight is the weight for UtilityScore in impact scoring (default: 5000).
	ImpactUtilityWeight uint32 `json:"impact_utility_weight_bps"`

	// ImpactDependencyWeight is the weight for DependencyCount in impact scoring (default: 3000).
	ImpactDependencyWeight uint32 `json:"impact_dependency_weight_bps"`

	// ImpactTrajectoryWeight is the weight for historical trend in impact scoring (default: 2000).
	ImpactTrajectoryWeight uint32 `json:"impact_trajectory_weight_bps"`

	// LowImpactThreshold is the ImpactScore below which a contribution is "low impact" (default: 20).
	LowImpactThreshold uint32 `json:"low_impact_threshold"`

	// HighImpactThreshold is the ImpactScore at or above which a contribution is "high impact" (default: 70).
	HighImpactThreshold uint32 `json:"high_impact_threshold"`

	// LowImpactPatternLimit is consecutive low-impact submissions before trust penalty (default: 3).
	LowImpactPatternLimit uint32 `json:"low_impact_pattern_limit"`

	// TrustPenaltyBps is the basis-point reduction to TrustAdjustmentFactor per excess
	// low-impact submission beyond LowImpactPatternLimit (default: 500 = 5%).
	TrustPenaltyBps uint32 `json:"trust_penalty_bps"`

	// TrustRecoveryBps is the basis-point increase to TrustAdjustmentFactor per
	// high-impact submission (default: 200 = 2%).
	TrustRecoveryBps uint32 `json:"trust_recovery_bps"`

	// TrustMinBps is the floor for TrustAdjustmentFactor (default: 5000 = 0.5x).
	TrustMinBps uint32 `json:"trust_min_bps"`

	// TrustMaxBps is the ceiling for TrustAdjustmentFactor (default: 15000 = 1.5x).
	TrustMaxBps uint32 `json:"trust_max_bps"`

	// SelfReferenceFilterEnabled controls whether self-references are excluded from scoring (default: true).
	SelfReferenceFilterEnabled bool `json:"self_reference_filter_enabled"`

	// DiminishingReturnDepth is the max provenance depth at which reuse signals are counted (default: 5).
	DiminishingReturnDepth uint32 `json:"diminishing_return_depth"`

	// MaxEdgesPerClaim is the max usage graph edges tracked per claim before deduplication (default: 1000).
	MaxEdgesPerClaim uint32 `json:"max_edges_per_claim"`

	// EnableImpactScoring enables the entire Layer 5 system (default: false for rollout safety).
	EnableImpactScoring bool `json:"enable_impact_scoring"`

	// EpochBatchSize is the max number of impact records updated per EndBlocker epoch pass (default: 50).
	EpochBatchSize uint32 `json:"epoch_batch_size"`
}

// DefaultImpactParams returns safe production defaults for Layer 5.
func DefaultImpactParams() ImpactParams {
	return ImpactParams{
		MultiplierMinBps:           8000,
		MultiplierMaxBps:           15000,
		UtilityReuseWeight:         4000,
		UtilityInvocationWeight:    3000,
		UtilityValidatorWeight:     3000,
		ImpactUtilityWeight:        5000,
		ImpactDependencyWeight:     3000,
		ImpactTrajectoryWeight:     2000,
		LowImpactThreshold:         20,
		HighImpactThreshold:        70,
		LowImpactPatternLimit:      3,
		TrustPenaltyBps:            500,
		TrustRecoveryBps:           200,
		TrustMinBps:                5000,
		TrustMaxBps:                15000,
		SelfReferenceFilterEnabled: true,
		DiminishingReturnDepth:     5,
		MaxEdgesPerClaim:           1000,
		EnableImpactScoring:        false, // off by default; enable via governance
		EpochBatchSize:             50,
	}
}

// ImpactMultiplierDec converts basis-point impact multiplier to LegacyDec.
func ImpactMultiplierDec(bps uint32) math.LegacyDec {
	return math.LegacyNewDec(int64(bps)).Quo(math.LegacyNewDec(10000))
}

// ClampImpactMultiplier clamps a basis-point value within governance bounds.
func ClampImpactMultiplier(bps, minBps, maxBps uint32) uint32 {
	if bps < minBps {
		return minBps
	}
	if bps > maxBps {
		return maxBps
	}
	return bps
}
