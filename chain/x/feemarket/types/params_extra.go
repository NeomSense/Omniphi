package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// DefaultParams returns the default feemarket parameters
func DefaultParams() FeeMarketParams {
	return FeeMarketParams{
		// Gas Pricing (EIP-1559)
		MinGasPrice:          math.LegacyMustNewDecFromStr("0.05"),  // 0.05 uomni/gas
		BaseFeeEnabled:       true,
		BaseFeeInitial:       math.LegacyMustNewDecFromStr("0.05"),  // 0.05 uomni/gas
		ElasticityMultiplier: math.LegacyMustNewDecFromStr("1.125"), // 12.5% change per block
		MaxTipRatio:          math.LegacyMustNewDecFromStr("0.20"),  // 20% of base fee

		// Block Utilization
		TargetBlockUtilization: math.LegacyMustNewDecFromStr("0.33"), // 33%
		MaxTxGas:               10_000_000,                           // 10M gas
		FreeTxQuota:            100,                                  // 100 free msgs/block

		// Adaptive Burn Tiers
		BurnCool:           math.LegacyMustNewDecFromStr("0.10"), // 10%
		BurnNormal:         math.LegacyMustNewDecFromStr("0.20"), // 20%
		BurnHot:            math.LegacyMustNewDecFromStr("0.40"), // 40%
		UtilCoolThreshold:  math.LegacyMustNewDecFromStr("0.16"), // 16%
		UtilHotThreshold:   math.LegacyMustNewDecFromStr("0.33"), // 33%

		// Fee Distribution (Post-Burn)
		ValidatorFeeRatio: math.LegacyMustNewDecFromStr("0.70"), // 70%
		TreasuryFeeRatio:  math.LegacyMustNewDecFromStr("0.30"), // 30%

		// Safety Limits
		MaxBurnRatio:      math.LegacyMustNewDecFromStr("0.50"),   // 50% cap
		MinGasPriceFloor:  math.LegacyMustNewDecFromStr("0.025"),  // 0.025 hard floor
	}
}

// Validate validates the feemarket parameters
func (p FeeMarketParams) Validate() error {
	// Validate min gas price
	if p.MinGasPrice.IsNegative() {
		return fmt.Errorf("min gas price cannot be negative: %s", p.MinGasPrice)
	}
	if p.MinGasPrice.LT(p.MinGasPriceFloor) {
		return fmt.Errorf("min gas price (%s) cannot be below floor (%s)", p.MinGasPrice, p.MinGasPriceFloor)
	}
	if p.MinGasPrice.GT(math.LegacyMustNewDecFromStr("1.0")) {
		return fmt.Errorf("min gas price too high: %s", p.MinGasPrice)
	}

	// Validate base fee initial
	if p.BaseFeeInitial.IsNegative() {
		return fmt.Errorf("base fee initial cannot be negative: %s", p.BaseFeeInitial)
	}

	// Validate elasticity multiplier (1.01 - 1.50)
	if p.ElasticityMultiplier.LT(math.LegacyMustNewDecFromStr("1.01")) ||
		p.ElasticityMultiplier.GT(math.LegacyMustNewDecFromStr("1.50")) {
		return fmt.Errorf("elasticity multiplier must be between 1.01 and 1.50, got: %s", p.ElasticityMultiplier)
	}

	// Validate max tip ratio (0.0 - 0.50)
	if p.MaxTipRatio.IsNegative() || p.MaxTipRatio.GT(math.LegacyMustNewDecFromStr("0.50")) {
		return fmt.Errorf("max tip ratio must be between 0 and 0.50, got: %s", p.MaxTipRatio)
	}

	// Validate target block utilization (0.20 - 0.80)
	if p.TargetBlockUtilization.LT(math.LegacyMustNewDecFromStr("0.20")) ||
		p.TargetBlockUtilization.GT(math.LegacyMustNewDecFromStr("0.80")) {
		return fmt.Errorf("target block utilization must be between 0.20 and 0.80, got: %s", p.TargetBlockUtilization)
	}

	// Validate max tx gas
	if p.MaxTxGas <= 0 {
		return fmt.Errorf("max tx gas must be positive, got: %d", p.MaxTxGas)
	}
	if p.MaxTxGas < 100_000 || p.MaxTxGas > 100_000_000 {
		return fmt.Errorf("max tx gas must be between 100k and 100M, got: %d", p.MaxTxGas)
	}

	// Validate free tx quota
	if p.FreeTxQuota < 0 || p.FreeTxQuota > 1000 {
		return fmt.Errorf("free tx quota must be between 0 and 1000, got: %d", p.FreeTxQuota)
	}

	// Validate burn tiers
	if p.BurnCool.IsNegative() || p.BurnCool.GT(math.LegacyMustNewDecFromStr("0.20")) {
		return fmt.Errorf("burn cool must be between 0 and 0.20, got: %s", p.BurnCool)
	}
	if p.BurnNormal.IsNegative() || p.BurnNormal.GT(math.LegacyMustNewDecFromStr("0.30")) {
		return fmt.Errorf("burn normal must be between 0 and 0.30, got: %s", p.BurnNormal)
	}
	if p.BurnHot.IsNegative() || p.BurnHot.GT(math.LegacyMustNewDecFromStr("0.50")) {
		return fmt.Errorf("burn hot must be between 0 and 0.50, got: %s", p.BurnHot)
	}

	// Validate burn tiers are ordered: cool <= normal <= hot
	if p.BurnCool.GT(p.BurnNormal) {
		return fmt.Errorf("burn cool (%s) must be <= burn normal (%s)", p.BurnCool, p.BurnNormal)
	}
	if p.BurnNormal.GT(p.BurnHot) {
		return fmt.Errorf("burn normal (%s) must be <= burn hot (%s)", p.BurnNormal, p.BurnHot)
	}

	// Validate utilization thresholds
	if p.UtilCoolThreshold.IsNegative() || p.UtilCoolThreshold.GT(math.LegacyOneDec()) {
		return fmt.Errorf("util cool threshold must be between 0 and 1, got: %s", p.UtilCoolThreshold)
	}
	if p.UtilHotThreshold.IsNegative() || p.UtilHotThreshold.GT(math.LegacyOneDec()) {
		return fmt.Errorf("util hot threshold must be between 0 and 1, got: %s", p.UtilHotThreshold)
	}
	if p.UtilCoolThreshold.GTE(p.UtilHotThreshold) {
		return fmt.Errorf("util cool threshold (%s) must be < util hot threshold (%s)",
			p.UtilCoolThreshold, p.UtilHotThreshold)
	}

	// Validate fee distribution ratios
	if p.ValidatorFeeRatio.IsNegative() || p.ValidatorFeeRatio.GT(math.LegacyOneDec()) {
		return fmt.Errorf("validator fee ratio must be between 0 and 1, got: %s", p.ValidatorFeeRatio)
	}
	if p.TreasuryFeeRatio.IsNegative() || p.TreasuryFeeRatio.GT(math.LegacyOneDec()) {
		return fmt.Errorf("treasury fee ratio must be between 0 and 1, got: %s", p.TreasuryFeeRatio)
	}

	// Validate ratios sum to 1.0
	sum := p.ValidatorFeeRatio.Add(p.TreasuryFeeRatio)
	if !sum.Equal(math.LegacyOneDec()) {
		return fmt.Errorf("validator fee ratio + treasury fee ratio must equal 1.0, got: %s", sum)
	}

	// Validate max burn ratio
	if p.MaxBurnRatio.IsNegative() || p.MaxBurnRatio.GT(math.LegacyMustNewDecFromStr("0.50")) {
		return fmt.Errorf("max burn ratio must be between 0 and 0.50, got: %s", p.MaxBurnRatio)
	}

	// Ensure burn hot doesn't exceed max burn
	if p.BurnHot.GT(p.MaxBurnRatio) {
		return fmt.Errorf("burn hot (%s) cannot exceed max burn ratio (%s)", p.BurnHot, p.MaxBurnRatio)
	}

	// Validate min gas price floor
	if p.MinGasPriceFloor.IsNegative() {
		return fmt.Errorf("min gas price floor cannot be negative: %s", p.MinGasPriceFloor)
	}
	if p.MinGasPriceFloor.LT(math.LegacyMustNewDecFromStr("0.01")) {
		return fmt.Errorf("min gas price floor cannot be below 0.01, got: %s", p.MinGasPriceFloor)
	}

	return nil
}
