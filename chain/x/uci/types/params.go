package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// Default parameter values
var (
	DefaultEnabled                = false
	DefaultMaxAdapters            int64 = 50
	DefaultMaxContributionsPerBatch int64 = 100
	DefaultMinOracleAttestations  int64 = 2
	DefaultOracleTimeoutBlocks    int64 = 50
	DefaultDefaultRewardShare     = math.LegacyNewDecWithPrec(80, 2)                     // 0.80
	DefaultAdapterRegistrationFee = math.NewInt(100000)                                   // 100000 omniphi
	DefaultRewardDenom            = "omniphi"
)

// Params defines the module parameters for x/uci
type Params struct {
	// Enabled controls whether the UCI module is active (governance-enabled)
	Enabled bool `json:"enabled"`

	// MaxAdapters is the maximum number of registered DePIN adapters
	MaxAdapters int64 `json:"max_adapters"`

	// MaxContributionsPerBatch is the maximum DePIN contributions per batch submission
	MaxContributionsPerBatch int64 `json:"max_contributions_per_batch"`

	// MinOracleAttestations is the minimum number of oracle attestations needed to verify a batch
	MinOracleAttestations int64 `json:"min_oracle_attestations"`

	// OracleTimeoutBlocks is the number of blocks to wait for oracle attestations
	OracleTimeoutBlocks int64 `json:"oracle_timeout_blocks"`

	// DefaultRewardShare is the share of PoC reward that goes to the DePIN contributor
	// The remainder (1 - DefaultRewardShare) goes to the adapter operator
	DefaultRewardShare math.LegacyDec `json:"default_reward_share"`

	// AdapterRegistrationFee is the fee required to register a new adapter
	AdapterRegistrationFee math.Int `json:"adapter_registration_fee"`

	// RewardDenom is the denomination for reward payments
	RewardDenom string `json:"reward_denom"`
}

// DefaultParams returns the default module parameters
func DefaultParams() Params {
	return Params{
		Enabled:                  DefaultEnabled,
		MaxAdapters:              DefaultMaxAdapters,
		MaxContributionsPerBatch: DefaultMaxContributionsPerBatch,
		MinOracleAttestations:    DefaultMinOracleAttestations,
		OracleTimeoutBlocks:      DefaultOracleTimeoutBlocks,
		DefaultRewardShare:       DefaultDefaultRewardShare,
		AdapterRegistrationFee:   DefaultAdapterRegistrationFee,
		RewardDenom:              DefaultRewardDenom,
	}
}

// Validate performs parameter validation
func (p Params) Validate() error {
	if p.MaxAdapters < 1 || p.MaxAdapters > 10000 {
		return fmt.Errorf("%w: max_adapters must be in [1, 10000], got %d", ErrInvalidParams, p.MaxAdapters)
	}

	if p.MaxContributionsPerBatch < 1 || p.MaxContributionsPerBatch > 10000 {
		return fmt.Errorf("%w: max_contributions_per_batch must be in [1, 10000], got %d", ErrInvalidParams, p.MaxContributionsPerBatch)
	}

	if p.MinOracleAttestations < 1 || p.MinOracleAttestations > 100 {
		return fmt.Errorf("%w: min_oracle_attestations must be in [1, 100], got %d", ErrInvalidParams, p.MinOracleAttestations)
	}

	if p.OracleTimeoutBlocks < 1 || p.OracleTimeoutBlocks > 10000 {
		return fmt.Errorf("%w: oracle_timeout_blocks must be in [1, 10000], got %d", ErrInvalidParams, p.OracleTimeoutBlocks)
	}

	if p.DefaultRewardShare.IsNil() || p.DefaultRewardShare.IsNegative() {
		return fmt.Errorf("%w: default_reward_share must be non-negative", ErrInvalidParams)
	}
	if p.DefaultRewardShare.GT(math.LegacyOneDec()) {
		return fmt.Errorf("%w: default_reward_share cannot exceed 1.0", ErrInvalidParams)
	}

	if p.AdapterRegistrationFee.IsNil() || p.AdapterRegistrationFee.IsNegative() {
		return fmt.Errorf("%w: adapter_registration_fee must be non-negative", ErrInvalidParams)
	}

	if p.RewardDenom == "" {
		return fmt.Errorf("%w: reward_denom cannot be empty", ErrInvalidParams)
	}

	return nil
}
