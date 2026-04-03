package types

import (
	"fmt"

	"cosmossdk.io/math"
)

var (
	DefaultEnabled             = true  // Enabled at mainnet (passive — no risk)
	DefaultTransferEnabled     = true  // Transferable royalty tokens
	DefaultFractionalizationEnabled = false // Phase 2: fractional tokens (needs audit)
	DefaultMaxFractions        int64 = 100  // max fractions per token
	DefaultMinRoyaltyShare     = math.LegacyNewDecWithPrec(1, 2)  // 1% minimum royalty share
	DefaultMaxRoyaltyShare     = math.LegacyNewDecWithPrec(50, 2) // 50% maximum royalty share
	DefaultClawbackGracePeriod int64 = 1000 // blocks after fraud detection before clawback
	DefaultMarketplaceFee      = math.LegacyNewDecWithPrec(2, 2) // 2% marketplace fee
	DefaultRewardDenom         = "omniphi"
)

type Params struct {
	// Enabled controls whether royalty tokenization is active
	Enabled bool `json:"enabled"`

	// TransferEnabled controls whether tokens can be transferred (Phase 2)
	TransferEnabled bool `json:"transfer_enabled"`

	// FractionalizationEnabled controls whether tokens can be fractionalized
	FractionalizationEnabled bool `json:"fractionalization_enabled"`

	// MaxFractions is the maximum number of fractions per token
	MaxFractions int64 `json:"max_fractions"`

	// MinRoyaltyShare is the minimum royalty percentage a token can represent
	MinRoyaltyShare math.LegacyDec `json:"min_royalty_share"`

	// MaxRoyaltyShare is the maximum royalty percentage a token can represent
	MaxRoyaltyShare math.LegacyDec `json:"max_royalty_share"`

	// ClawbackGracePeriod is the number of blocks to wait after fraud detection
	ClawbackGracePeriod int64 `json:"clawback_grace_period"`

	// MarketplaceFee is the fee percentage on secondary market sales
	MarketplaceFee math.LegacyDec `json:"marketplace_fee"`

	// RewardDenom is the denomination for royalty payments
	RewardDenom string `json:"reward_denom"`
}

func DefaultParams() Params {
	return Params{
		Enabled:                    DefaultEnabled,
		TransferEnabled:            DefaultTransferEnabled,
		FractionalizationEnabled:   DefaultFractionalizationEnabled,
		MaxFractions:               DefaultMaxFractions,
		MinRoyaltyShare:            DefaultMinRoyaltyShare,
		MaxRoyaltyShare:            DefaultMaxRoyaltyShare,
		ClawbackGracePeriod:        DefaultClawbackGracePeriod,
		MarketplaceFee:             DefaultMarketplaceFee,
		RewardDenom:                DefaultRewardDenom,
	}
}

func (p Params) Validate() error {
	if p.MaxFractions < 1 || p.MaxFractions > 10000 {
		return fmt.Errorf("%w: max_fractions must be in [1, 10000]", ErrInvalidParams)
	}

	if p.MinRoyaltyShare.IsNil() || p.MinRoyaltyShare.IsNegative() {
		return fmt.Errorf("%w: min_royalty_share must be non-negative", ErrInvalidParams)
	}
	if p.MaxRoyaltyShare.IsNil() || p.MaxRoyaltyShare.IsNegative() {
		return fmt.Errorf("%w: max_royalty_share must be non-negative", ErrInvalidParams)
	}
	if p.MinRoyaltyShare.GT(p.MaxRoyaltyShare) {
		return fmt.Errorf("%w: min_royalty_share cannot exceed max_royalty_share", ErrInvalidParams)
	}
	if p.MaxRoyaltyShare.GT(math.LegacyOneDec()) {
		return fmt.Errorf("%w: max_royalty_share cannot exceed 1.0", ErrInvalidParams)
	}

	if p.ClawbackGracePeriod < 0 {
		return fmt.Errorf("%w: clawback_grace_period cannot be negative", ErrInvalidParams)
	}

	if !p.MarketplaceFee.IsNil() && (p.MarketplaceFee.IsNegative() || p.MarketplaceFee.GT(math.LegacyNewDecWithPrec(10, 2))) {
		return fmt.Errorf("%w: marketplace_fee must be in [0, 0.10]", ErrInvalidParams)
	}

	if p.RewardDenom == "" {
		return fmt.Errorf("%w: reward_denom cannot be empty", ErrInvalidParams)
	}

	return nil
}
