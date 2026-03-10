package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultQuorumPct is the default quorum percentage (67% or 2/3)
var DefaultQuorumPct = math.LegacyNewDecWithPrec(67, 2) // 0.67

// DefaultBaseRewardUnit is the default base reward credits
var DefaultBaseRewardUnit = math.NewInt(1000)

// DefaultInflationShare is the default inflation share (0% for now)
var DefaultInflationShare = math.LegacyZeroDec()

// DefaultMaxPerBlock is the default max submissions per block
const DefaultMaxPerBlock uint32 = 10

// DefaultRewardDenom is the default denomination for rewards
const DefaultRewardDenom = "omniphi"

// Fee Burn Parameter Defaults

// DefaultSubmissionFee is the default fee for submitting a contribution (2000 omniphi = 0.002 OMNI)
// Updated from 1000 to 2000 omniphi as per Adaptive Fee Market v2 specification
var DefaultSubmissionFee = sdk.NewCoin("omniphi", math.NewInt(2000))

// DefaultSubmissionBurnRatio is the default percentage of submission fee to burn (50%)
// Updated from 75% to 50% as per Adaptive Fee Market v2 specification
// This aligns with the new fee distribution model: 50% burn, 50% to pool
var DefaultSubmissionBurnRatio = math.LegacyNewDecWithPrec(50, 2) // 0.50

// DefaultMinSubmissionFee is the minimum allowed submission fee (100 omniphi = 0.0001 OMNI)
var DefaultMinSubmissionFee = sdk.NewCoin("omniphi", math.NewInt(100))

// DefaultMaxSubmissionFee is the maximum allowed submission fee (100000 omniphi = 0.1 OMNI)
var DefaultMaxSubmissionFee = sdk.NewCoin("omniphi", math.NewInt(100000))

// DefaultMinBurnRatio is the minimum allowed burn ratio (50%)
var DefaultMinBurnRatio = math.LegacyNewDecWithPrec(50, 2) // 0.50

// DefaultMaxBurnRatio is the maximum allowed burn ratio (90%)
var DefaultMaxBurnRatio = math.LegacyNewDecWithPrec(90, 2) // 0.90

// Access Control Parameter Defaults (PoA Layer Enhancement)

// DefaultEnableCscoreGating - C-Score gating disabled by default for backwards compatibility
const DefaultEnableCscoreGating = false

// DefaultEnableIdentityGating - Identity gating disabled by default for backwards compatibility
const DefaultEnableIdentityGating = false

// 3-Layer Fee System Defaults

// DefaultBaseSubmissionFee is the base fee for all submissions before multipliers
// Default: 30000 omniphi (0.03 OMNI)
var DefaultBaseSubmissionFee = sdk.NewCoin("omniphi", math.NewInt(30000))

// DefaultTargetSubmissionsPerBlock is the target number of submissions per block
// Used for dynamic congestion fee calculation
const DefaultTargetSubmissionsPerBlock uint32 = 5

// DefaultMaxCscoreDiscount is the maximum discount available based on C-Score
// Default: 0.90 (90% discount cap for C-Score 1000)
var DefaultMaxCscoreDiscount = math.LegacyNewDecWithPrec(90, 2) // 0.90

// DefaultMinimumSubmissionFee is the absolute floor for fees after all discounts
// Default: 3000 omniphi (0.003 OMNI)
var DefaultMinimumSubmissionFee = sdk.NewCoin("omniphi", math.NewInt(3000))

// DefaultTreasuryAddress is the default treasury address for governance-controlled funds
// SECURITY: Empty by default - MUST be set via governance before enabling treasury features
// Format: bech32 address (e.g., "omni1...")
const DefaultTreasuryAddress = ""

// DefaultTreasuryShareRatio is the percentage of pool rewards sent to treasury (0% by default)
// This allows protocol-level revenue sharing when configured via governance
var DefaultTreasuryShareRatio = math.LegacyZeroDec()

// Canonical Hash Layer Defaults

// DefaultDuplicateBond is the refundable bond required per submission (anti-spam)
// Default: 10000 omniphi (0.01 OMNI)
var DefaultDuplicateBond = sdk.NewCoin("omniphi", math.NewInt(10000))

// DefaultEnableCanonicalHashCheck controls whether canonical hash deduplication is active
// Disabled by default for backwards compatibility — enable via governance
const DefaultEnableCanonicalHashCheck = false

// DefaultMaxDuplicatesPerEpoch is the max duplicate submissions per address per epoch
// before rate limiting kicks in (rejects further submissions)
const DefaultMaxDuplicatesPerEpoch uint32 = 3

// DefaultDuplicateBondEscalationBps is the bond escalation per detected duplicate in basis points
// 5000 = 50% increase per duplicate (bond = base * (1 + N*escalation/10000))
const DefaultDuplicateBondEscalationBps uint32 = 5000

// Similarity Engine Defaults

// DefaultSimilarityOracleAllowlist is empty by default — must be set via governance
func DefaultSimilarityOracleAllowlist() []string {
	return []string{}
}

// DefaultDerivativeThreshold is the similarity score above which a contribution is flagged as derivative
// 8500 = 85.00% similarity
const DefaultDerivativeThreshold uint32 = 8500

// DefaultSimilarityEpochBlocks is the number of blocks per similarity epoch (anti-replay)
// Default: 100 blocks
const DefaultSimilarityEpochBlocks int64 = 100

// DefaultEnableSimilarityCheck controls whether similarity checking is active
// Disabled by default — enable via governance after configuring oracle allowlist
const DefaultEnableSimilarityCheck = false

// Human Review Layer Defaults

// DefaultVerifiersPerClaim is the number of reviewers assigned to each claim
const DefaultVerifiersPerClaim uint32 = 3

// DefaultEnableHumanReview controls whether the human review layer is active
// Disabled by default — enable via governance after configuring reviewer params
const DefaultEnableHumanReview = false

// DefaultReviewQuorumPct is the percentage of assigned reviewers that must vote (67% = 2/3)
const DefaultReviewQuorumPct uint32 = 67

// DefaultMinReviewerBond is the bond required per review assignment (5000 omniphi)
const DefaultMinReviewerBond = "5000omniphi"

// DefaultMinReviewerReputation is the minimum PoC credits required for reviewer eligibility
const DefaultMinReviewerReputation uint64 = 1000

// DefaultAppealBond is the bond required to file an appeal (50000 omniphi)
const DefaultAppealBond = "50000omniphi"

// DefaultAppealVotePeriod is the number of blocks for appeal resolution (~4 hours)
const DefaultAppealVotePeriod int64 = 2400

// DefaultCollusionThresholdBps is the co-voting overlap threshold in basis points (9000 = 90%)
const DefaultCollusionThresholdBps uint32 = 9000

// Layer 4: Economic Adjustment Defaults

// DefaultRoyaltyShare is the percentage of rewards sent to the parent claim (10%)
var DefaultRoyaltyShare = math.LegacyNewDecWithPrec(10, 2) // 0.10

// DefaultImmediateRewardRatio is the percentage of rewards paid immediately (20%)
var DefaultImmediateRewardRatio = math.LegacyNewDecWithPrec(20, 2) // 0.20

// DefaultVestingEpochs is the number of epochs over which the remainder vests
const DefaultVestingEpochs int64 = 10

// DefaultEnableConfigurableBands controls whether governance-configurable bands are active
const DefaultEnableConfigurableBands = false

// DefaultGrandparentRoyaltyShare is the royalty share for grandparent claims (5%)
var DefaultGrandparentRoyaltyShare = math.LegacyNewDecWithPrec(5, 2) // 0.05

// DefaultMaxRoyaltyDepth is the maximum lineage depth for royalty routing
const DefaultMaxRoyaltyDepth uint32 = 2

// DefaultMaxTotalRoyaltyShare caps the total royalty deduction (25%)
var DefaultMaxTotalRoyaltyShare = math.LegacyNewDecWithPrec(25, 2) // 0.25

// DefaultRepeatOffenderThreshold is the offense count before penalties apply
const DefaultRepeatOffenderThreshold uint64 = 3

// DefaultRepeatOffenderBondEscalationBps is bond increase per offense in bps (50%)
const DefaultRepeatOffenderBondEscalationBps uint32 = 5000

// DefaultRepeatOffenderRewardCap is the max reward fraction for offenders (50%)
var DefaultRepeatOffenderRewardCap = math.LegacyNewDecWithPrec(50, 2) // 0.50

// DefaultRepeatOffenderVestingMultiplier extends vesting for offenders (2x)
var DefaultRepeatOffenderVestingMultiplier = math.LegacyNewDec(2) // 2.0x

// DefaultEnableAutoClawback controls whether automatic clawback is enabled
const DefaultEnableAutoClawback = false

// Layer 5: Global Provenance Registry Defaults

// DefaultMaxProvenanceDepth is the maximum depth of the provenance lineage DAG
const DefaultMaxProvenanceDepth uint32 = 10

// DefaultEnableProvenanceRegistry controls whether provenance registration is active
const DefaultEnableProvenanceRegistry = false

// DefaultProvenanceSchemaVersion is the initial schema version for provenance entries
const DefaultProvenanceSchemaVersion uint32 = 1

// DefaultReviewVotePeriod is the number of blocks reviewers have to vote
const DefaultReviewVotePeriod int64 = 1200 // ~1-2 hours

// Adaptive Reward Vesting System (ARVS) Defaults

// DefaultEnableARVS — ARVS disabled by default; enable via governance after configuring profiles
const DefaultEnableARVS = false

// DefaultARVSCategoryRiskMapJSON — empty JSON object; uses in-code DefaultCategoryRiskMap() at runtime
const DefaultARVSCategoryRiskMapJSON = "{}"

// DefaultARVSEnableBounty — bounty distribution disabled by default
const DefaultARVSEnableBounty = false

// DefaultARVSTreasuryAddress — empty; must be set via governance before enabling bounty treasury share
const DefaultARVSTreasuryAddress = ""

// DefaultARVSRiskScoreLowThreshold is the max risk score (bps) for Low Risk profile (30%)
const DefaultARVSRiskScoreLowThreshold uint32 = 3000

// DefaultARVSRiskScoreHighThreshold is the min risk score (bps) for High Risk profile (65%)
const DefaultARVSRiskScoreHighThreshold uint32 = 6500

// DefaultMinCscoreForCtype returns the default C-Score requirements for contribution types
// Empty map by default = no restrictions (backwards compatible)
// Governance can set requirements like:
// - "code": 1000 (Bronze tier)
// - "governance": 10000 (Silver tier)
// - "security": 100000 (Gold tier)
func DefaultMinCscoreForCtype() map[string]math.Int {
	return make(map[string]math.Int)
}

// DefaultRequireIdentityForCtype returns the default identity requirements for contribution types
// Empty map by default = no requirements (backwards compatible)
// Governance can require identity for types like:
// - "treasury": true
// - "upgrade": true
// - "emergency": true
func DefaultRequireIdentityForCtype() map[string]bool {
	return make(map[string]bool)
}

// DefaultExemptAddresses returns the default exempt addresses list
// Empty by default - no exemptions
func DefaultExemptAddresses() []string {
	return []string{}
}

// NewParams creates a new Params instance
func NewParams(
	quorumPct math.LegacyDec,
	baseRewardUnit math.Int,
	inflationShare math.LegacyDec,
	maxPerBlock uint32,
	tiers []Tier,
	rewardDenom string,
	verifiersPerClaim uint32,
	reviewVotePeriod int64,
	royaltyShare math.LegacyDec,
	immediateRewardRatio math.LegacyDec,
	vestingEpochs int64,
) Params {
	return Params{
		QuorumPct:            quorumPct,
		BaseRewardUnit:       baseRewardUnit,
		InflationShare:       inflationShare,
		MaxPerBlock:          maxPerBlock,
		Tiers:                tiers,
		RewardDenom:          rewardDenom,
		VerifiersPerClaim:    verifiersPerClaim,
		ReviewVotePeriod:     reviewVotePeriod,
		RoyaltyShare:         royaltyShare,
		ImmediateRewardRatio: immediateRewardRatio,
		VestingEpochs:        vestingEpochs,
	}
}

// DefaultParams returns default module parameters
func DefaultParams() Params {
	return Params{
		QuorumPct:              DefaultQuorumPct,
		BaseRewardUnit:         DefaultBaseRewardUnit,
		InflationShare:         DefaultInflationShare,
		MaxPerBlock:            DefaultMaxPerBlock,
		Tiers:                  DefaultTiers(),
		RewardDenom:            DefaultRewardDenom,
		MaxContributionsToKeep: 100000,
		SubmissionFee:          DefaultSubmissionFee,
		SubmissionBurnRatio:    DefaultSubmissionBurnRatio,
		MinSubmissionFee:       DefaultMinSubmissionFee,
		MaxSubmissionFee:       DefaultMaxSubmissionFee,
		MinBurnRatio:           DefaultMinBurnRatio,
		MaxBurnRatio:           DefaultMaxBurnRatio,
		// Access control defaults (backwards compatible - all disabled)
		EnableCscoreGating:      DefaultEnableCscoreGating,
		MinCscoreForCtype:       DefaultMinCscoreForCtype(),
		EnableIdentityGating:    DefaultEnableIdentityGating,
		RequireIdentityForCtype: DefaultRequireIdentityForCtype(),
		ExemptAddresses:         DefaultExemptAddresses(),
		// 3-Layer Fee System defaults
		BaseSubmissionFee:         DefaultBaseSubmissionFee,
		TargetSubmissionsPerBlock: DefaultTargetSubmissionsPerBlock,
		MaxCscoreDiscount:         DefaultMaxCscoreDiscount,
		MinimumSubmissionFee:      DefaultMinimumSubmissionFee,
		// Canonical Hash Layer defaults
		DuplicateBond:              DefaultDuplicateBond,
		EnableCanonicalHashCheck:   DefaultEnableCanonicalHashCheck,
		MaxDuplicatesPerEpoch:      DefaultMaxDuplicatesPerEpoch,
		DuplicateBondEscalationBps: DefaultDuplicateBondEscalationBps,
		// Similarity Engine defaults
		SimilarityOracleAllowlist: DefaultSimilarityOracleAllowlist(),
		DerivativeThreshold:       DefaultDerivativeThreshold,
		SimilarityEpochBlocks:     DefaultSimilarityEpochBlocks,
		EnableSimilarityCheck:     DefaultEnableSimilarityCheck,
		// Human Review defaults
		VerifiersPerClaim:       DefaultVerifiersPerClaim,
		ReviewVotePeriod:        DefaultReviewVotePeriod,
		EnableHumanReview:       DefaultEnableHumanReview,
		ReviewQuorumPct:         DefaultReviewQuorumPct,
		MinReviewerBond:         DefaultMinReviewerBond,
		MinReviewerReputation:   DefaultMinReviewerReputation,
		AppealBond:              DefaultAppealBond,
		AppealVotePeriod:        DefaultAppealVotePeriod,
		CollusionThresholdBps:   DefaultCollusionThresholdBps,
		// Economic Adjustment defaults
		RoyaltyShare:                    DefaultRoyaltyShare,
		ImmediateRewardRatio:            DefaultImmediateRewardRatio,
		VestingEpochs:                   DefaultVestingEpochs,
		EnableConfigurableBands:         DefaultEnableConfigurableBands,
		OriginalityBands:                DefaultOriginalityBands(),
		GrandparentRoyaltyShare:         DefaultGrandparentRoyaltyShare,
		MaxRoyaltyDepth:                 DefaultMaxRoyaltyDepth,
		MaxTotalRoyaltyShare:            DefaultMaxTotalRoyaltyShare,
		RepeatOffenderThreshold:         DefaultRepeatOffenderThreshold,
		RepeatOffenderBondEscalationBps: DefaultRepeatOffenderBondEscalationBps,
		RepeatOffenderRewardCap:         DefaultRepeatOffenderRewardCap,
		RepeatOffenderVestingMultiplier: DefaultRepeatOffenderVestingMultiplier,
		EnableAutoClawback:              DefaultEnableAutoClawback,
		// Provenance Registry defaults
		MaxProvenanceDepth:       DefaultMaxProvenanceDepth,
		EnableProvenanceRegistry: DefaultEnableProvenanceRegistry,
		ProvenanceSchemaVersion:  DefaultProvenanceSchemaVersion,
		// Adaptive Reward Vesting System (ARVS) defaults
		EnableARVS:                 DefaultEnableARVS,
		ARVSWeights:                DefaultARVSWeights(),
		ARVSVestingProfiles:        DefaultVestingProfiles(),
		ARVSCategoryRiskMapJSON:    DefaultARVSCategoryRiskMapJSON,
		ARVSBountyDistribution:     DefaultBountyDistribution(),
		ARVSEnableBounty:           DefaultARVSEnableBounty,
		ARVSTreasuryAddress:        DefaultARVSTreasuryAddress,
		ARVSRiskScoreLowThreshold:  DefaultARVSRiskScoreLowThreshold,
		ARVSRiskScoreHighThreshold: DefaultARVSRiskScoreHighThreshold,
	}
}

// DefaultTiers returns default contribution tiers
func DefaultTiers() []Tier {
	return []Tier{
		{
			Name:   "bronze",
			Cutoff: math.NewInt(1000),
		},
		{
			Name:   "silver",
			Cutoff: math.NewInt(10000),
		},
		{
			Name:   "gold",
			Cutoff: math.NewInt(100000),
		},
	}
}

// Validate performs basic validation of module parameters
func (p Params) Validate() error {
	if p.QuorumPct.IsNil() || p.QuorumPct.IsNegative() || p.QuorumPct.GT(math.LegacyOneDec()) {
		return ErrInvalidQuorumPct
	}

	if p.BaseRewardUnit.IsNil() || p.BaseRewardUnit.IsNegative() {
		return ErrInvalidRewardUnit
	}

	if p.InflationShare.IsNil() || p.InflationShare.IsNegative() || p.InflationShare.GT(math.LegacyOneDec()) {
		return ErrInvalidInflationShare
	}

	if p.MaxPerBlock == 0 {
		return ErrRateLimitExceeded
	}

	if p.RewardDenom == "" {
		return ErrInvalidCType
	}

	// Validate tiers
	for i, tier := range p.Tiers {
		if tier.Name == "" {
			return ErrInvalidCType
		}
		if tier.Cutoff.IsNil() || tier.Cutoff.IsNegative() {
			return ErrInvalidRewardUnit
		}
		// Ensure tiers are in ascending order
		if i > 0 && tier.Cutoff.LTE(p.Tiers[i-1].Cutoff) {
			return ErrInvalidRewardUnit
		}
	}

	// Validate fee parameters
	if err := validateSubmissionFee(p.SubmissionFee); err != nil {
		return err
	}
	if err := validateBurnRatio(p.SubmissionBurnRatio); err != nil {
		return err
	}
	if err := validateSubmissionFee(p.MinSubmissionFee); err != nil {
		return err
	}
	if err := validateSubmissionFee(p.MaxSubmissionFee); err != nil {
		return err
	}
	if err := validateBurnRatio(p.MinBurnRatio); err != nil {
		return err
	}
	if err := validateBurnRatio(p.MaxBurnRatio); err != nil {
		return err
	}
	if err := validateFeeWithinBounds(p.SubmissionFee, p.MinSubmissionFee, p.MaxSubmissionFee); err != nil {
		return err
	}
	if err := validateRatioWithinBounds(p.SubmissionBurnRatio, p.MinBurnRatio, p.MaxBurnRatio); err != nil {
		return err
	}

	// Validate access control parameters
	if err := validateCScoreRequirements(p.MinCscoreForCtype); err != nil {
		return err
	}
	if err := validateExemptAddresses(p.ExemptAddresses); err != nil {
		return err
	}

	// Validate 3-layer fee system parameters
	if err := validateSubmissionFee(p.BaseSubmissionFee); err != nil {
		return fmt.Errorf("invalid base_submission_fee: %w", err)
	}
	if p.TargetSubmissionsPerBlock == 0 {
		return fmt.Errorf("target_submissions_per_block must be greater than 0")
	}
	if p.TargetSubmissionsPerBlock > 1000 {
		return fmt.Errorf("target_submissions_per_block cannot exceed 1000 (got %d)", p.TargetSubmissionsPerBlock)
	}
	if err := validateBurnRatio(p.MaxCscoreDiscount); err != nil {
		return fmt.Errorf("invalid max_cscore_discount: %w", err)
	}
	if err := validateSubmissionFee(p.MinimumSubmissionFee); err != nil {
		return fmt.Errorf("invalid minimum_submission_fee: %w", err)
	}
	// Ensure minimum fee is less than base fee
	if p.MinimumSubmissionFee.Amount.GT(p.BaseSubmissionFee.Amount) {
		return fmt.Errorf("minimum_submission_fee (%s) cannot exceed base_submission_fee (%s)",
			p.MinimumSubmissionFee, p.BaseSubmissionFee)
	}
	// Ensure same denom
	if p.BaseSubmissionFee.Denom != p.MinimumSubmissionFee.Denom {
		return fmt.Errorf("base_submission_fee and minimum_submission_fee must have same denom (got %s and %s)",
			p.BaseSubmissionFee.Denom, p.MinimumSubmissionFee.Denom)
	}

	// Validate canonical hash layer parameters
	if p.DuplicateBond.IsValid() {
		if p.DuplicateBond.IsNegative() {
			return fmt.Errorf("duplicate_bond cannot be negative: %s", p.DuplicateBond)
		}
	}
	if p.MaxDuplicatesPerEpoch == 0 && p.EnableCanonicalHashCheck {
		return fmt.Errorf("max_duplicates_per_epoch must be > 0 when canonical hash check is enabled")
	}
	if p.DuplicateBondEscalationBps > 10000 {
		return fmt.Errorf("duplicate_bond_escalation_bps cannot exceed 10000 (100%%): got %d", p.DuplicateBondEscalationBps)
	}

	// Validate similarity engine parameters
	if p.EnableSimilarityCheck {
		if len(p.SimilarityOracleAllowlist) == 0 {
			return fmt.Errorf("similarity_oracle_allowlist must not be empty when similarity check is enabled")
		}
	}
	if len(p.SimilarityOracleAllowlist) > MaxOracleAllowlistSize {
		return fmt.Errorf("similarity_oracle_allowlist too large: %d > max %d",
			len(p.SimilarityOracleAllowlist), MaxOracleAllowlistSize)
	}
	oracleSeen := make(map[string]bool)
	for _, addr := range p.SimilarityOracleAllowlist {
		if addr == "" {
			return fmt.Errorf("similarity_oracle_allowlist contains empty address")
		}
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return fmt.Errorf("invalid oracle address in allowlist '%s': %w", addr, err)
		}
		if oracleSeen[addr] {
			return fmt.Errorf("duplicate oracle address in allowlist: %s", addr)
		}
		oracleSeen[addr] = true
	}
	if p.DerivativeThreshold > SimilarityScaleMax {
		return fmt.Errorf("derivative_threshold cannot exceed %d (100.00%%): got %d",
			SimilarityScaleMax, p.DerivativeThreshold)
	}
	if p.SimilarityEpochBlocks < 0 {
		return fmt.Errorf("similarity_epoch_blocks cannot be negative: got %d", p.SimilarityEpochBlocks)
	}
	if p.EnableSimilarityCheck && p.SimilarityEpochBlocks == 0 {
		return fmt.Errorf("similarity_epoch_blocks must be > 0 when similarity check is enabled")
	}

	// Validate Human Review parameters
	if p.VerifiersPerClaim == 0 {
		return fmt.Errorf("verifiers_per_claim must be greater than 0")
	}
	if p.VerifiersPerClaim > MaxReviewersPerClaim {
		return fmt.Errorf("verifiers_per_claim cannot exceed %d (got %d)", MaxReviewersPerClaim, p.VerifiersPerClaim)
	}
	if p.VerifiersPerClaim < MinReviewersPerClaim {
		return fmt.Errorf("verifiers_per_claim must be at least %d (got %d)", MinReviewersPerClaim, p.VerifiersPerClaim)
	}
	if p.ReviewVotePeriod <= 0 {
		return fmt.Errorf("review_vote_period must be positive")
	}
	if p.ReviewVotePeriod < MinReviewVotePeriod {
		return fmt.Errorf("review_vote_period must be at least %d (got %d)", MinReviewVotePeriod, p.ReviewVotePeriod)
	}
	if p.ReviewVotePeriod > MaxReviewVotePeriod {
		return fmt.Errorf("review_vote_period cannot exceed %d (got %d)", MaxReviewVotePeriod, p.ReviewVotePeriod)
	}
	if p.ReviewQuorumPct > 100 {
		return fmt.Errorf("review_quorum_pct cannot exceed 100 (got %d)", p.ReviewQuorumPct)
	}
	if p.EnableHumanReview && p.ReviewQuorumPct == 0 {
		return fmt.Errorf("review_quorum_pct must be > 0 when human review is enabled")
	}
	if p.MinReviewerBond != "" {
		if _, err := sdk.ParseCoinNormalized(p.MinReviewerBond); err != nil {
			return fmt.Errorf("invalid min_reviewer_bond '%s': %w", p.MinReviewerBond, err)
		}
	}
	if p.AppealBond != "" {
		if _, err := sdk.ParseCoinNormalized(p.AppealBond); err != nil {
			return fmt.Errorf("invalid appeal_bond '%s': %w", p.AppealBond, err)
		}
	}
	if p.AppealVotePeriod < 0 {
		return fmt.Errorf("appeal_vote_period cannot be negative (got %d)", p.AppealVotePeriod)
	}
	if p.EnableHumanReview && p.AppealVotePeriod == 0 {
		return fmt.Errorf("appeal_vote_period must be > 0 when human review is enabled")
	}
	if p.CollusionThresholdBps > 10000 {
		return fmt.Errorf("collusion_threshold_bps cannot exceed 10000 (got %d)", p.CollusionThresholdBps)
	}

	// Validate Economic Adjustment parameters
	if err := validateBurnRatio(p.RoyaltyShare); err != nil {
		return fmt.Errorf("invalid royalty_share: %w", err)
	}
	if err := validateBurnRatio(p.ImmediateRewardRatio); err != nil {
		return fmt.Errorf("invalid immediate_reward_ratio: %w", err)
	}
	if p.VestingEpochs < 0 {
		return fmt.Errorf("vesting_epochs cannot be negative")
	}

	// Validate Layer 4 parameters
	if err := validateOriginalityBands(p.OriginalityBands); err != nil {
		return fmt.Errorf("invalid originality_bands: %w", err)
	}
	if !p.GrandparentRoyaltyShare.IsNil() {
		if p.GrandparentRoyaltyShare.IsNegative() {
			return fmt.Errorf("grandparent_royalty_share cannot be negative")
		}
		if p.GrandparentRoyaltyShare.GT(math.LegacyNewDecWithPrec(25, 2)) {
			return fmt.Errorf("grandparent_royalty_share cannot exceed 0.25 (25%%)")
		}
	}
	if p.MaxRoyaltyDepth > 5 {
		return fmt.Errorf("max_royalty_depth cannot exceed 5 (got %d)", p.MaxRoyaltyDepth)
	}
	if !p.MaxTotalRoyaltyShare.IsNil() {
		if p.MaxTotalRoyaltyShare.IsNegative() {
			return fmt.Errorf("max_total_royalty_share cannot be negative")
		}
		if p.MaxTotalRoyaltyShare.GT(math.LegacyNewDecWithPrec(50, 2)) {
			return fmt.Errorf("max_total_royalty_share cannot exceed 0.50 (50%%)")
		}
	}
	if p.RepeatOffenderBondEscalationBps > 10000 {
		return fmt.Errorf("repeat_offender_bond_escalation_bps cannot exceed 10000 (got %d)", p.RepeatOffenderBondEscalationBps)
	}
	if !p.RepeatOffenderRewardCap.IsNil() {
		if p.RepeatOffenderRewardCap.LT(math.LegacyNewDecWithPrec(1, 1)) {
			return fmt.Errorf("repeat_offender_reward_cap cannot be less than 0.1")
		}
		if p.RepeatOffenderRewardCap.GT(math.LegacyOneDec()) {
			return fmt.Errorf("repeat_offender_reward_cap cannot exceed 1.0")
		}
	}
	if !p.RepeatOffenderVestingMultiplier.IsNil() {
		if p.RepeatOffenderVestingMultiplier.LT(math.LegacyOneDec()) {
			return fmt.Errorf("repeat_offender_vesting_multiplier cannot be less than 1.0")
		}
		if p.RepeatOffenderVestingMultiplier.GT(math.LegacyNewDec(10)) {
			return fmt.Errorf("repeat_offender_vesting_multiplier cannot exceed 10.0")
		}
	}

	// Validate ARVS parameters
	if p.EnableARVS {
		if err := p.ARVSWeights.Validate(); err != nil {
			return fmt.Errorf("invalid arvs_weights: %w", err)
		}
		if len(p.ARVSVestingProfiles) == 0 {
			return fmt.Errorf("arvs_vesting_profiles must not be empty when ARVS is enabled")
		}
		for _, profile := range p.ARVSVestingProfiles {
			if err := profile.Validate(); err != nil {
				return fmt.Errorf("invalid arvs_vesting_profile %d: %w", profile.ProfileID, err)
			}
		}
		if err := p.ARVSBountyDistribution.Validate(); err != nil {
			return fmt.Errorf("invalid arvs_bounty_distribution: %w", err)
		}
		if p.ARVSRiskScoreLowThreshold >= p.ARVSRiskScoreHighThreshold {
			return fmt.Errorf("arvs_risk_score_low_threshold (%d) must be less than high_threshold (%d)",
				p.ARVSRiskScoreLowThreshold, p.ARVSRiskScoreHighThreshold)
		}
		if p.ARVSRiskScoreHighThreshold > 10000 {
			return fmt.Errorf("arvs_risk_score_high_threshold cannot exceed 10000 (got %d)", p.ARVSRiskScoreHighThreshold)
		}
		if p.ARVSEnableBounty && p.ARVSTreasuryAddress == "" && p.ARVSBountyDistribution.TreasuryBps > 0 {
			return fmt.Errorf("arvs_treasury_address must be set when bounty treasury share > 0")
		}
	}

	// Validate Provenance Registry parameters
	if p.MaxProvenanceDepth > 20 {
		return fmt.Errorf("max_provenance_depth cannot exceed 20 (got %d)", p.MaxProvenanceDepth)
	}
	if p.EnableProvenanceRegistry && p.MaxProvenanceDepth == 0 {
		return fmt.Errorf("max_provenance_depth must be > 0 when provenance registry is enabled")
	}
	if p.ProvenanceSchemaVersion > 10 {
		return fmt.Errorf("provenance_schema_version cannot exceed 10 (got %d)", p.ProvenanceSchemaVersion)
	}

	return nil
}

// validateOriginalityBands validates the originality bands configuration
func validateOriginalityBands(bands []OriginalityBand) error {
	if len(bands) == 0 {
		return nil // empty is valid (use defaults)
	}
	if len(bands) > 10 {
		return fmt.Errorf("too many bands: %d > max 10", len(bands))
	}
	for i, band := range bands {
		if band.MinSimilarity.IsNil() || band.MaxSimilarity.IsNil() || band.Multiplier.IsNil() {
			return fmt.Errorf("band %d has nil fields", i)
		}
		if band.MinSimilarity.IsNegative() || band.MinSimilarity.GT(math.LegacyOneDec()) {
			return fmt.Errorf("band %d min_similarity out of range [0,1]: %s", i, band.MinSimilarity)
		}
		if band.MaxSimilarity.IsNegative() || band.MaxSimilarity.GT(math.LegacyNewDecWithPrec(101, 2)) {
			return fmt.Errorf("band %d max_similarity out of range [0,1.01]: %s", i, band.MaxSimilarity)
		}
		if band.MinSimilarity.GTE(band.MaxSimilarity) {
			return fmt.Errorf("band %d min >= max: %s >= %s", i, band.MinSimilarity, band.MaxSimilarity)
		}
		if band.Multiplier.IsNegative() || band.Multiplier.GT(math.LegacyNewDec(2)) {
			return fmt.Errorf("band %d multiplier out of range [0,2]: %s", i, band.Multiplier)
		}
	}
	return nil
}

// validateSubmissionFee validates a submission fee coin
func validateSubmissionFee(fee sdk.Coin) error {
	if !fee.IsValid() {
		return fmt.Errorf("%w: %s", ErrInvalidSubmissionFee, fee)
	}
	if fee.IsNegative() {
		return fmt.Errorf("%w: cannot be negative %s", ErrInvalidSubmissionFee, fee)
	}
	if fee.Denom == "" {
		return fmt.Errorf("%w: denom cannot be empty", ErrInvalidSubmissionFee)
	}
	return nil
}

// validateBurnRatio validates a burn ratio decimal
func validateBurnRatio(ratio math.LegacyDec) error {
	if ratio.IsNil() {
		return fmt.Errorf("%w: cannot be nil", ErrInvalidBurnRatio)
	}
	if ratio.IsNegative() {
		return fmt.Errorf("%w: cannot be negative %s", ErrInvalidBurnRatio, ratio)
	}
	if ratio.GT(math.LegacyOneDec()) {
		return fmt.Errorf("%w: cannot exceed 1.0 (100%%), got %s", ErrInvalidBurnRatio, ratio)
	}
	return nil
}

// validateFeeWithinBounds ensures fee is within min/max bounds
func validateFeeWithinBounds(fee, min, max sdk.Coin) error {
	if fee.Denom != min.Denom || fee.Denom != max.Denom {
		return fmt.Errorf("%w: denom mismatch (fee=%s, min=%s, max=%s)", ErrInvalidSubmissionFee, fee.Denom, min.Denom, max.Denom)
	}
	if fee.Amount.LT(min.Amount) {
		return fmt.Errorf("%w: %s < minimum %s", ErrFeeBelowMinimum, fee, min)
	}
	if fee.Amount.GT(max.Amount) {
		return fmt.Errorf("%w: %s > maximum %s", ErrFeeAboveMaximum, fee, max)
	}
	return nil
}

// validateRatioWithinBounds ensures burn ratio is within min/max bounds
func validateRatioWithinBounds(ratio, min, max math.LegacyDec) error {
	if ratio.LT(min) {
		return fmt.Errorf("%w: %s < minimum %s", ErrBurnRatioBelowMinimum, ratio, min)
	}
	if ratio.GT(max) {
		return fmt.Errorf("%w: %s > maximum %s", ErrBurnRatioAboveMaximum, ratio, max)
	}
	return nil
}

// validateCScoreRequirements validates C-Score requirement map
func validateCScoreRequirements(requirements map[string]math.Int) error {
	if requirements == nil {
		return nil // nil map is valid (no requirements)
	}

	for ctype, requiredScore := range requirements {
		// Validate contribution type name
		if ctype == "" {
			return fmt.Errorf("C-Score requirement has empty contribution type")
		}

		// Validate required score is non-negative
		if requiredScore.IsNegative() {
			return fmt.Errorf("C-Score requirement for type '%s' cannot be negative: %s", ctype, requiredScore)
		}

		// Validate required score is reasonable (< 2^63)
		const maxSafeUint64 = uint64(1<<63 - 1)
		maxSafeCredits := math.NewIntFromUint64(maxSafeUint64)
		if requiredScore.GT(maxSafeCredits) {
			return fmt.Errorf("C-Score requirement for type '%s' exceeds maximum safe value: %s > %s",
				ctype, requiredScore, maxSafeCredits)
		}
	}

	return nil
}

// validateExemptAddresses validates the exempt addresses list
func validateExemptAddresses(addresses []string) error {
	if addresses == nil {
		return nil // nil list is valid (no exemptions)
	}

	seen := make(map[string]bool)
	for _, addr := range addresses {
		// Validate address format
		if addr == "" {
			return fmt.Errorf("exempt address list contains empty address")
		}

		// Validate bech32 format
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return fmt.Errorf("invalid exempt address '%s': %w", addr, err)
		}

		// Check for duplicates
		if seen[addr] {
			return fmt.Errorf("duplicate exempt address: %s", addr)
		}
		seen[addr] = true
	}

	return nil
}

// validateTreasuryConfig validates treasury address and share ratio
// SECURITY: Ensures treasury configuration is consistent and safe
func validateTreasuryConfig(treasuryAddress string, shareRatio math.LegacyDec) error {
	// Validate share ratio bounds
	if shareRatio.IsNil() {
		return fmt.Errorf("treasury_share_ratio cannot be nil")
	}
	if shareRatio.IsNegative() {
		return fmt.Errorf("treasury_share_ratio cannot be negative: %s", shareRatio)
	}
	if shareRatio.GT(math.LegacyOneDec()) {
		return fmt.Errorf("treasury_share_ratio cannot exceed 1.0 (100%%): %s", shareRatio)
	}

	// SECURITY: If share ratio is > 0, treasury address MUST be set
	// This prevents funds being sent to a zero/invalid address
	if shareRatio.IsPositive() && treasuryAddress == "" {
		return fmt.Errorf("treasury_address must be set when treasury_share_ratio > 0")
	}

	// If treasury address is set, validate it
	if treasuryAddress != "" {
		// Validate bech32 format
		if _, err := sdk.AccAddressFromBech32(treasuryAddress); err != nil {
			return fmt.Errorf("invalid treasury_address '%s': %w", treasuryAddress, err)
		}

		// SECURITY: Warn if using common test/burn addresses
		// These checks help prevent accidental misconfiguration
		if treasuryAddress == "omni1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqnrql8a" {
			return fmt.Errorf("treasury_address cannot be the zero address")
		}
	}

	// SECURITY: Cap treasury share at 50% to prevent governance attacks
	// that could drain all rewards to a compromised treasury
	maxTreasuryShare := math.LegacyNewDecWithPrec(50, 2) // 0.50 (50%)
	if shareRatio.GT(maxTreasuryShare) {
		return fmt.Errorf("treasury_share_ratio cannot exceed 50%% for safety (got %s)", shareRatio)
	}

	return nil
}

// ParseCoinFromString parses a coin from string format (e.g., "30000omniphi")
func ParseCoinFromString(s string) (sdk.Coin, error) {
	coin, err := sdk.ParseCoinNormalized(s)
	if err != nil {
		return sdk.Coin{}, err
	}
	return coin, nil
}

// Note: String() method is auto-generated in params.pb.go
// Note: Equal() method for Tier is defined in helpers.go
