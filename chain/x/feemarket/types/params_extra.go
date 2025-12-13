package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// ActivityType represents the type of blockchain activity for burn calculation
type ActivityType string

const (
	ActivityMessaging      ActivityType = "messaging"
	ActivityPosGas         ActivityType = "pos_gas"
	ActivityPocAnchoring   ActivityType = "poc_anchoring"
	ActivitySmartContracts ActivityType = "smart_contracts"
	ActivityAiQueries      ActivityType = "ai_queries"
	ActivitySequencer      ActivityType = "sequencer"
)

// DefaultParams returns the default feemarket parameters
func DefaultParams() FeeMarketParams {
	return FeeMarketParams{
		// Gas Pricing (EIP-1559)
		MinGasPrice:          math.LegacyMustNewDecFromStr("0.05"),
		BaseFeeEnabled:       true,
		BaseFeeInitial:       math.LegacyMustNewDecFromStr("0.05"),
		ElasticityMultiplier: math.LegacyMustNewDecFromStr("1.125"),
		MaxTipRatio:          math.LegacyMustNewDecFromStr("0.20"),

		// Block Utilization
		TargetBlockUtilization: math.LegacyMustNewDecFromStr("0.33"),
		MaxTxGas:               10_000_000,
		FreeTxQuota:            100,

		// Adaptive Burn Tiers
		BurnCool:          math.LegacyMustNewDecFromStr("0.10"),
		BurnNormal:        math.LegacyMustNewDecFromStr("0.20"),
		BurnHot:           math.LegacyMustNewDecFromStr("0.40"),
		UtilCoolThreshold: math.LegacyMustNewDecFromStr("0.16"),
		UtilHotThreshold:  math.LegacyMustNewDecFromStr("0.33"),

		// Fee Distribution (Post-Burn)
		ValidatorFeeRatio: math.LegacyMustNewDecFromStr("0.70"),
		TreasuryFeeRatio:  math.LegacyMustNewDecFromStr("0.30"),

		// Safety Limits
		MaxBurnRatio:     math.LegacyMustNewDecFromStr("0.50"),
		MinGasPriceFloor: math.LegacyMustNewDecFromStr("0.025"),

		// Activity Multipliers (Single-Pass Burn Model)
		MultiplierMessaging:      math.LegacyMustNewDecFromStr("0.50"),
		MultiplierPosGas:         math.LegacyMustNewDecFromStr("1.00"),
		MultiplierPocAnchoring:   math.LegacyMustNewDecFromStr("0.75"),
		MultiplierSmartContracts: math.LegacyMustNewDecFromStr("1.50"),
		MultiplierAiQueries:      math.LegacyMustNewDecFromStr("1.25"),
		MultiplierSequencer:      math.LegacyMustNewDecFromStr("1.25"),
		MinMultiplier:            math.LegacyMustNewDecFromStr("0.25"),
		MaxMultiplier:            math.LegacyMustNewDecFromStr("2.00"),
	}
}

// GetActivityMultiplier returns the burn multiplier for a given activity type
func (p FeeMarketParams) GetActivityMultiplier(activity ActivityType) math.LegacyDec {
	switch activity {
	case ActivityMessaging:
		return p.MultiplierMessaging
	case ActivityPosGas:
		return p.MultiplierPosGas
	case ActivityPocAnchoring:
		return p.MultiplierPocAnchoring
	case ActivitySmartContracts:
		return p.MultiplierSmartContracts
	case ActivityAiQueries:
		return p.MultiplierAiQueries
	case ActivitySequencer:
		return p.MultiplierSequencer
	default:
		return p.MultiplierPosGas
	}
}

// Validate validates the feemarket parameters
func (p FeeMarketParams) Validate() error {
	// Gas Pricing
	if p.MinGasPrice.IsNegative() {
		return fmt.Errorf("min gas price cannot be negative: %s", p.MinGasPrice)
	}
	if p.MinGasPrice.LT(p.MinGasPriceFloor) {
		return fmt.Errorf("min gas price (%s) cannot be below floor (%s)", p.MinGasPrice, p.MinGasPriceFloor)
	}
	if p.MinGasPrice.GT(math.LegacyMustNewDecFromStr("1.0")) {
		return fmt.Errorf("min gas price too high: %s", p.MinGasPrice)
	}
	if p.BaseFeeInitial.IsNegative() {
		return fmt.Errorf("base fee initial cannot be negative: %s", p.BaseFeeInitial)
	}
	if p.ElasticityMultiplier.LT(math.LegacyMustNewDecFromStr("1.01")) ||
		p.ElasticityMultiplier.GT(math.LegacyMustNewDecFromStr("1.50")) {
		return fmt.Errorf("elasticity multiplier must be between 1.01 and 1.50, got: %s", p.ElasticityMultiplier)
	}
	if p.MaxTipRatio.IsNegative() || p.MaxTipRatio.GT(math.LegacyMustNewDecFromStr("0.50")) {
		return fmt.Errorf("max tip ratio must be between 0 and 0.50, got: %s", p.MaxTipRatio)
	}

	// Block Utilization
	if p.TargetBlockUtilization.LT(math.LegacyMustNewDecFromStr("0.20")) ||
		p.TargetBlockUtilization.GT(math.LegacyMustNewDecFromStr("0.80")) {
		return fmt.Errorf("target block utilization must be between 0.20 and 0.80, got: %s", p.TargetBlockUtilization)
	}
	if p.MaxTxGas < 100_000 || p.MaxTxGas > 100_000_000 {
		return fmt.Errorf("max tx gas must be between 100k and 100M, got: %d", p.MaxTxGas)
	}
	if p.FreeTxQuota < 0 || p.FreeTxQuota > 1000 {
		return fmt.Errorf("free tx quota must be between 0 and 1000, got: %d", p.FreeTxQuota)
	}

	// Burn Tiers
	if p.BurnCool.IsNegative() || p.BurnCool.GT(math.LegacyMustNewDecFromStr("0.20")) {
		return fmt.Errorf("burn cool must be between 0 and 0.20, got: %s", p.BurnCool)
	}
	if p.BurnNormal.IsNegative() || p.BurnNormal.GT(math.LegacyMustNewDecFromStr("0.30")) {
		return fmt.Errorf("burn normal must be between 0 and 0.30, got: %s", p.BurnNormal)
	}
	if p.BurnHot.IsNegative() || p.BurnHot.GT(math.LegacyMustNewDecFromStr("0.50")) {
		return fmt.Errorf("burn hot must be between 0 and 0.50, got: %s", p.BurnHot)
	}
	if p.BurnCool.GT(p.BurnNormal) || p.BurnNormal.GT(p.BurnHot) {
		return fmt.Errorf("burn tiers must be ordered: cool <= normal <= hot")
	}
	if p.UtilCoolThreshold.GTE(p.UtilHotThreshold) {
		return fmt.Errorf("util cool threshold must be < util hot threshold")
	}

	// Fee Distribution
	sum := p.ValidatorFeeRatio.Add(p.TreasuryFeeRatio)
	if !sum.Equal(math.LegacyOneDec()) {
		return fmt.Errorf("validator + treasury fee ratios must equal 1.0, got: %s", sum)
	}

	// Safety Limits (PROTOCOL ENFORCED: Max 50%)
	if p.MaxBurnRatio.GT(math.LegacyMustNewDecFromStr("0.50")) {
		return fmt.Errorf("max burn ratio cannot exceed 0.50, got: %s", p.MaxBurnRatio)
	}
	if p.BurnHot.GT(p.MaxBurnRatio) {
		return fmt.Errorf("burn hot cannot exceed max burn ratio")
	}
	if p.MinGasPriceFloor.LT(math.LegacyMustNewDecFromStr("0.01")) {
		return fmt.Errorf("min gas price floor cannot be below 0.01")
	}

	// Activity Multipliers
	if p.MinMultiplier.GTE(p.MaxMultiplier) {
		return fmt.Errorf("min multiplier must be < max multiplier")
	}
	multipliers := map[string]math.LegacyDec{
		"messaging":       p.MultiplierMessaging,
		"pos_gas":         p.MultiplierPosGas,
		"poc_anchoring":   p.MultiplierPocAnchoring,
		"smart_contracts": p.MultiplierSmartContracts,
		"ai_queries":      p.MultiplierAiQueries,
		"sequencer":       p.MultiplierSequencer,
	}
	for name, val := range multipliers {
		if val.LT(p.MinMultiplier) || val.GT(p.MaxMultiplier) {
			return fmt.Errorf("%s multiplier (%s) out of bounds [%s, %s]",
				name, val, p.MinMultiplier, p.MaxMultiplier)
		}
	}

	return nil
}
