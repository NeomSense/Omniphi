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

// DefaultSubmissionFee is the default fee for submitting a contribution (2000 uomni = 0.002 OMNI)
// Updated from 1000 to 2000 uomni as per Adaptive Fee Market v2 specification
var DefaultSubmissionFee = sdk.NewCoin("uomni", math.NewInt(2000))

// DefaultSubmissionBurnRatio is the default percentage of submission fee to burn (50%)
// Updated from 75% to 50% as per Adaptive Fee Market v2 specification
// This aligns with the new fee distribution model: 50% burn, 50% to pool
var DefaultSubmissionBurnRatio = math.LegacyNewDecWithPrec(50, 2) // 0.50

// DefaultMinSubmissionFee is the minimum allowed submission fee (100 uomni = 0.0001 OMNI)
var DefaultMinSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100))

// DefaultMaxSubmissionFee is the maximum allowed submission fee (100000 uomni = 0.1 OMNI)
var DefaultMaxSubmissionFee = sdk.NewCoin("uomni", math.NewInt(100000))

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
// Default: 30000 uomni (0.03 OMNI)
var DefaultBaseSubmissionFee = sdk.NewCoin("uomni", math.NewInt(30000))

// DefaultTargetSubmissionsPerBlock is the target number of submissions per block
// Used for dynamic congestion fee calculation
const DefaultTargetSubmissionsPerBlock uint32 = 5

// DefaultMaxCscoreDiscount is the maximum discount available based on C-Score
// Default: 0.90 (90% discount cap for C-Score 1000)
var DefaultMaxCscoreDiscount = math.LegacyNewDecWithPrec(90, 2) // 0.90

// DefaultMinimumSubmissionFee is the absolute floor for fees after all discounts
// Default: 3000 uomni (0.003 OMNI)
var DefaultMinimumSubmissionFee = sdk.NewCoin("uomni", math.NewInt(3000))

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
) Params {
	return Params{
		QuorumPct:      quorumPct,
		BaseRewardUnit: baseRewardUnit,
		InflationShare: inflationShare,
		MaxPerBlock:    maxPerBlock,
		Tiers:          tiers,
		RewardDenom:    rewardDenom,
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
		EnableCscoreGating:       DefaultEnableCscoreGating,
		MinCscoreForCtype:        DefaultMinCscoreForCtype(),
		EnableIdentityGating:     DefaultEnableIdentityGating,
		RequireIdentityForCtype:  DefaultRequireIdentityForCtype(),
		ExemptAddresses:          DefaultExemptAddresses(),
		// 3-Layer Fee System defaults
		BaseSubmissionFee:         DefaultBaseSubmissionFee,
		TargetSubmissionsPerBlock: DefaultTargetSubmissionsPerBlock,
		MaxCscoreDiscount:         DefaultMaxCscoreDiscount,
		MinimumSubmissionFee:      DefaultMinimumSubmissionFee,
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

// ParseCoinFromString parses a coin from string format (e.g. "30000uomni")
func ParseCoinFromString(s string) (sdk.Coin, error) {
	coin, err := sdk.ParseCoinNormalized(s)
	if err != nil {
		return sdk.Coin{}, err
	}
	return coin, nil
}

// Note: String() method is auto-generated in params.pb.go
// Note: Equal() method for Tier is defined in helpers.go
