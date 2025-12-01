package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// DefaultParams returns the default tokenomics parameters
// These are the mainnet launch parameters
func DefaultParams() TokenomicsParams {
	return TokenomicsParams{
		// Supply caps (IMMUTABLE - protocol enforced)
		TotalSupplyCap:     math.NewInt(1_500_000_000_000_000), // 1.5B OMNI (6 decimals)
		CurrentTotalSupply: math.NewInt(375_000_000_000_000),   // 375M OMNI (genesis)
		TotalMinted:        math.NewInt(375_000_000_000_000),   // 375M OMNI (genesis)
		TotalBurned:        math.ZeroInt(),

		// Inflation policy (DAO-adjustable 1-5%, default 3%)
		InflationRate: math.LegacyNewDecWithPrec(3, 2),  // 0.03 = 3%
		InflationMin:  math.LegacyNewDecWithPrec(1, 2),  // 0.01 = 1%
		InflationMax:  math.LegacyNewDecWithPrec(5, 2),  // 0.05 = 5% PROTOCOL CAP

		// Emission splits (must sum to 1.0 = 100%)
		EmissionSplitStaking:   math.LegacyNewDecWithPrec(40, 2), // 0.40 = 40%
		EmissionSplitPoc:       math.LegacyNewDecWithPrec(30, 2), // 0.30 = 30%
		EmissionSplitSequencer: math.LegacyNewDecWithPrec(20, 2), // 0.20 = 20%
		EmissionSplitTreasury:  math.LegacyNewDecWithPrec(10, 2), // 0.10 = 10%

		// Burn rates by module (0-50% per module)
		BurnRatePosGas:         math.LegacyNewDecWithPrec(20, 2), // 20%
		BurnRatePocAnchoring:   math.LegacyNewDecWithPrec(25, 2), // 25%
		BurnRateSequencerGas:   math.LegacyNewDecWithPrec(15, 2), // 15%
		BurnRateSmartContracts: math.LegacyNewDecWithPrec(12, 2), // 12%
		BurnRateAiQueries:      math.LegacyNewDecWithPrec(10, 2), // 10%
		BurnRateMessaging:      math.LegacyNewDecWithPrec(8, 2),  // 8%

		// Treasury policy
		TreasuryBurnRedirect: math.LegacyNewDecWithPrec(10, 2), // 10% of burns to treasury

		// Fee Burn & Treasury Split (90/10)
		FeeBurnEnabled:    true,                                     // Enable fee burning by default
		FeeBurnRatio:      math.LegacyNewDecWithPrec(90, 2),        // 0.90 = 90% of fees burned
		TreasuryFeeRatio:  math.LegacyNewDecWithPrec(10, 2),        // 0.10 = 10% of fees to treasury

		// Gas & Fee policy
		MinGasPrice:                  math.LegacyNewDecWithPrec(1, 2), // 0.01 = 0.01 OMNI minimum (changed from 0.001)
		GasConversionRatioContinuity: math.LegacyNewDecWithPrec(50, 2), // 0.50 = 50% of Core price
		GasConversionRatioSequencer:  math.LegacyNewDecWithPrec(10, 2), // 0.10 = 10% of Core price

		// PoC Merit Engine
		PocAlpha: math.LegacyNewDecWithPrec(10, 2), // 0.10 = 10% credit weight

		// IBC parameters
		RewardStreamInterval: 100, // blocks (every 100 blocks = ~12 minutes with 7s blocks)
		ContinuityIbcChannel: "channel-0",
		SequencerIbcChannel:  "channel-1",

		// Governance safety (time locks and quorums)
		ParamChangeDelay:   172800,                                // 48 hours in seconds
		MinProposalDeposit: math.NewInt(10_000_000_000),           // 10,000 OMNI
		QuorumPercentage:   math.LegacyNewDecWithPrec(40, 2),      // 40%
		PassPercentage:     math.LegacyNewDecWithPrec(50, 2),      // 50%
		VotingPeriod:       604800,                                // 7 days in seconds

		// Adaptive Burn Controller (Dynamic burn scaling)
		AdaptiveBurnEnabled:          false,                                // Disabled by default, DAO can enable
		MinBurnRatio:                 math.LegacyNewDecWithPrec(80, 2),    // 0.80 = 80% minimum
		MaxBurnRatio:                 math.LegacyNewDecWithPrec(95, 2),    // 0.95 = 95% maximum
		DefaultBurnRatio:             math.LegacyNewDecWithPrec(90, 2),    // 0.90 = 90% baseline
		BlockCongestionThreshold:     math.LegacyNewDecWithPrec(75, 2),    // 0.75 = 75% gas usage
		TxPerDayTarget:               10000,                               // 10,000 tx/day target
		TreasuryFloorPct:             math.LegacyNewDecWithPrec(5, 2),     // 0.05 = 5% of supply
		BurnAdjustmentSmoothing:      100,                                 // 100 blocks smoothing
		LastAppliedBurnRatio:         math.LegacyNewDecWithPrec(90, 2),    // Initial 90%
		LastBurnTrigger:              "normal",                            // Initial state
		EmergencyBurnOverride:        false,                               // No emergency override
	}
}

// Validate validates the tokenomics parameters
// Enforces protocol caps and business logic constraints
func (p TokenomicsParams) Validate() error {
	// ========================================
	// P0-GEN-004: Validate supply caps
	// ========================================

	if !p.TotalSupplyCap.IsPositive() {
		return fmt.Errorf("total supply cap must be positive, got %s", p.TotalSupplyCap.String())
	}

	// Enforce protocol hard cap of 1.5B OMNI (immutable)
	protocolHardCap := math.NewInt(1_500_000_000_000_000) // 1.5B OMNI with 6 decimals
	if p.TotalSupplyCap.GT(protocolHardCap) {
		return fmt.Errorf("total supply cap (%s) exceeds protocol hard cap (%s)", p.TotalSupplyCap.String(), protocolHardCap.String())
	}

	if p.CurrentTotalSupply.GT(p.TotalSupplyCap) {
		return fmt.Errorf("current supply (%s) exceeds cap (%s)", p.CurrentTotalSupply.String(), p.TotalSupplyCap.String())
	}

	if p.TotalMinted.LT(p.TotalBurned) {
		return fmt.Errorf("total minted (%s) cannot be less than total burned (%s)", p.TotalMinted.String(), p.TotalBurned.String())
	}

	// Verify conservation law: current = minted - burned
	expected := p.TotalMinted.Sub(p.TotalBurned)
	if !p.CurrentTotalSupply.Equal(expected) {
		return fmt.Errorf("supply accounting error: current (%s) != minted (%s) - burned (%s)",
			p.CurrentTotalSupply.String(), p.TotalMinted.String(), p.TotalBurned.String())
	}

	// ========================================
	// P0-INF-003, P0-INF-004: Validate inflation bounds
	// ========================================

	if p.InflationRate.IsNegative() {
		return fmt.Errorf("inflation rate cannot be negative, got %s", p.InflationRate.String())
	}

	if p.InflationRate.LT(p.InflationMin) {
		return ErrInflationBelowMin
	}

	if p.InflationRate.GT(p.InflationMax) {
		return ErrInflationAboveMax
	}

	// P0-INF-005: Enforce protocol cap (inflation_max cannot exceed 5%)
	protocolInflationCap := math.LegacyNewDecWithPrec(5, 2) // 0.05 = 5%
	if p.InflationMax.GT(protocolInflationCap) {
		return ErrProtocolCapViolation
	}

	// Inflation min must be less than max
	if p.InflationMin.GTE(p.InflationMax) {
		return fmt.Errorf("inflation min (%s) must be less than max (%s)", p.InflationMin.String(), p.InflationMax.String())
	}

	// ========================================
	// P0-DIST-001: Validate emission splits sum to 100%
	// ========================================

	emissionSum := p.EmissionSplitStaking.
		Add(p.EmissionSplitPoc).
		Add(p.EmissionSplitSequencer).
		Add(p.EmissionSplitTreasury)

	if !emissionSum.Equal(math.LegacyOneDec()) {
		return ErrEmissionSplitInvalid
	}

	// Each split must be non-negative
	emissionSplits := []struct {
		name  string
		value math.LegacyDec
	}{
		{"staking", p.EmissionSplitStaking},
		{"poc", p.EmissionSplitPoc},
		{"sequencer", p.EmissionSplitSequencer},
		{"treasury", p.EmissionSplitTreasury},
	}

	for _, split := range emissionSplits {
		if split.value.IsNegative() {
			return fmt.Errorf("emission split %s cannot be negative, got %s", split.name, split.value.String())
		}
		if split.value.GT(math.LegacyOneDec()) {
			return fmt.Errorf("emission split %s cannot exceed 100%%, got %s", split.name, split.value.String())
		}
	}

	// ========================================
	// P1-GAME-001: Validate burn rates (0-50%)
	// ========================================

	maxBurnRate := math.LegacyNewDecWithPrec(50, 2) // 0.50 = 50%
	burnRates := []struct {
		name  string
		value math.LegacyDec
	}{
		{"pos_gas", p.BurnRatePosGas},
		{"poc_anchoring", p.BurnRatePocAnchoring},
		{"sequencer_gas", p.BurnRateSequencerGas},
		{"smart_contracts", p.BurnRateSmartContracts},
		{"ai_queries", p.BurnRateAiQueries},
		{"messaging", p.BurnRateMessaging},
	}

	for _, rate := range burnRates {
		if rate.value.IsNegative() {
			return fmt.Errorf("burn rate %s cannot be negative, got %s", rate.name, rate.value.String())
		}
		if rate.value.GT(maxBurnRate) {
			return fmt.Errorf("burn rate %s cannot exceed 50%%, got %s", rate.name, rate.value.String())
		}
	}

	// ========================================
	// P1-REDIRECT-004: Validate treasury redirect (0-20%)
	// ========================================

	maxTreasuryRedirect := math.LegacyNewDecWithPrec(20, 2) // 0.20 = 20%
	if p.TreasuryBurnRedirect.IsNegative() {
		return fmt.Errorf("treasury burn redirect cannot be negative, got %s", p.TreasuryBurnRedirect.String())
	}
	if p.TreasuryBurnRedirect.GT(maxTreasuryRedirect) {
		return fmt.Errorf("treasury burn redirect cannot exceed 20%%, got %s", p.TreasuryBurnRedirect.String())
	}

	// ========================================
	// FEE-BURN: Validate fee burn ratios
	// ========================================

	if p.FeeBurnEnabled {
		// Validate burn ratio is between 0 and 1
		if p.FeeBurnRatio.IsNegative() || p.FeeBurnRatio.GT(math.LegacyOneDec()) {
			return fmt.Errorf("fee burn ratio must be between 0 and 1, got %s", p.FeeBurnRatio.String())
		}

		// Validate treasury fee ratio is between 0 and 1
		if p.TreasuryFeeRatio.IsNegative() || p.TreasuryFeeRatio.GT(math.LegacyOneDec()) {
			return fmt.Errorf("treasury fee ratio must be between 0 and 1, got %s", p.TreasuryFeeRatio.String())
		}

		// Validate that ratios sum to 1.0 (100% of fees must be accounted for)
		sum := p.FeeBurnRatio.Add(p.TreasuryFeeRatio)
		if !sum.Equal(math.LegacyOneDec()) {
			return fmt.Errorf("fee burn ratio + treasury fee ratio must equal 1.0, got %s", sum.String())
		}
	}

	// ========================================
	// P0-FEE-001: Validate min gas price
	// ========================================

	if !p.MinGasPrice.IsNil() && p.MinGasPrice.IsNegative() {
		return fmt.Errorf("min gas price cannot be negative, got %s", p.MinGasPrice.String())
	}

	// ========================================
	// Validate gas conversion ratios (0-1.0)
	// ========================================

	if p.GasConversionRatioContinuity.IsNegative() || p.GasConversionRatioContinuity.GT(math.LegacyOneDec()) {
		return fmt.Errorf("gas conversion ratio continuity must be between 0 and 1, got %s", p.GasConversionRatioContinuity.String())
	}

	if p.GasConversionRatioSequencer.IsNegative() || p.GasConversionRatioSequencer.GT(math.LegacyOneDec()) {
		return fmt.Errorf("gas conversion ratio sequencer must be between 0 and 1, got %s", p.GasConversionRatioSequencer.String())
	}

	// ========================================
	// P0-POC-001: Validate PoC alpha (0-1.0)
	// ========================================

	if !p.PocAlpha.IsNil() {
		if p.PocAlpha.IsNegative() {
			return fmt.Errorf("poc alpha cannot be negative, got %s", p.PocAlpha.String())
		}

		maxAlpha := math.LegacyOneDec() // 1.0 maximum
		if p.PocAlpha.GT(maxAlpha) {
			return fmt.Errorf("poc alpha cannot exceed 1.0, got %s", p.PocAlpha.String())
		}
	}

	// ========================================
	// Validate IBC parameters
	// ========================================

	if p.RewardStreamInterval == 0 {
		return fmt.Errorf("reward stream interval must be positive, got %d", p.RewardStreamInterval)
	}

	if p.RewardStreamInterval > 10000 {
		return fmt.Errorf("reward stream interval too large (max 10000 blocks), got %d", p.RewardStreamInterval)
	}

	// IBC channels should be non-empty (but can be updated later via governance)
	// Not enforcing strict format here as channels can be established after genesis

	// ========================================
	// P0-GOV-001: Validate governance parameters
	// ========================================

	if !p.MinProposalDeposit.IsPositive() {
		return fmt.Errorf("min proposal deposit must be positive, got %s", p.MinProposalDeposit.String())
	}

	if p.QuorumPercentage.LTE(math.LegacyZeroDec()) || p.QuorumPercentage.GTE(math.LegacyOneDec()) {
		return fmt.Errorf("quorum percentage must be between 0 and 1, got %s", p.QuorumPercentage.String())
	}

	if p.PassPercentage.LTE(math.LegacyZeroDec()) || p.PassPercentage.GTE(math.LegacyOneDec()) {
		return fmt.Errorf("pass percentage must be between 0 and 1, got %s", p.PassPercentage.String())
	}

	if p.VotingPeriod == 0 {
		return fmt.Errorf("voting period must be positive, got %d", p.VotingPeriod)
	}

	if p.VotingPeriod > 2592000 { // 30 days in seconds
		return fmt.Errorf("voting period too long (max 30 days), got %d seconds", p.VotingPeriod)
	}

	if p.ParamChangeDelay == 0 {
		return fmt.Errorf("param change delay must be positive, got %d", p.ParamChangeDelay)
	}

	if p.ParamChangeDelay > 604800 { // 7 days in seconds
		return fmt.Errorf("param change delay too long (max 7 days), got %d seconds", p.ParamChangeDelay)
	}

	// ========================================
	// ADAPTIVE BURN: Validate dynamic burn parameters
	// ========================================

	if p.AdaptiveBurnEnabled {
		// Validate min_burn_ratio (0.70 - 1.00)
		minBurnProtocolFloor := math.LegacyNewDecWithPrec(70, 2) // 0.70 = 70%
		if p.MinBurnRatio.LT(minBurnProtocolFloor) || p.MinBurnRatio.GT(math.LegacyOneDec()) {
			return fmt.Errorf("min burn ratio must be between 0.70 and 1.00, got %s", p.MinBurnRatio.String())
		}

		// Validate max_burn_ratio (0.70 - 0.95)
		maxBurnProtocolCap := math.LegacyNewDecWithPrec(95, 2) // 0.95 = 95%
		if p.MaxBurnRatio.LT(minBurnProtocolFloor) || p.MaxBurnRatio.GT(maxBurnProtocolCap) {
			return fmt.Errorf("max burn ratio must be between 0.70 and 0.95, got %s", p.MaxBurnRatio.String())
		}

		// Validate min <= default <= max
		if p.MinBurnRatio.GT(p.DefaultBurnRatio) {
			return fmt.Errorf("min burn ratio (%s) cannot exceed default (%s)", p.MinBurnRatio.String(), p.DefaultBurnRatio.String())
		}
		if p.DefaultBurnRatio.GT(p.MaxBurnRatio) {
			return fmt.Errorf("default burn ratio (%s) cannot exceed max (%s)", p.DefaultBurnRatio.String(), p.MaxBurnRatio.String())
		}

		// Validate block_congestion_threshold (0.50 - 1.00)
		if p.BlockCongestionThreshold.LT(math.LegacyNewDecWithPrec(50, 2)) || p.BlockCongestionThreshold.GT(math.LegacyOneDec()) {
			return fmt.Errorf("block congestion threshold must be between 0.50 and 1.00, got %s", p.BlockCongestionThreshold.String())
		}

		// Validate tx_per_day_target (positive)
		if p.TxPerDayTarget == 0 {
			return fmt.Errorf("tx per day target must be positive, got %d", p.TxPerDayTarget)
		}

		// Validate treasury_floor_pct (0 - 0.20)
		maxTreasuryFloor := math.LegacyNewDecWithPrec(20, 2) // 0.20 = 20%
		if p.TreasuryFloorPct.IsNegative() || p.TreasuryFloorPct.GT(maxTreasuryFloor) {
			return fmt.Errorf("treasury floor pct must be between 0 and 0.20, got %s", p.TreasuryFloorPct.String())
		}

		// Validate burn_adjustment_smoothing (10 - 1000 blocks)
		if p.BurnAdjustmentSmoothing < 10 || p.BurnAdjustmentSmoothing > 1000 {
			return fmt.Errorf("burn adjustment smoothing must be between 10 and 1000 blocks, got %d", p.BurnAdjustmentSmoothing)
		}

		// Validate last_applied_burn_ratio is within bounds
		if !p.LastAppliedBurnRatio.IsZero() {
			if p.LastAppliedBurnRatio.LT(p.MinBurnRatio) || p.LastAppliedBurnRatio.GT(p.MaxBurnRatio) {
				return fmt.Errorf("last applied burn ratio (%s) must be between min (%s) and max (%s)",
					p.LastAppliedBurnRatio.String(), p.MinBurnRatio.String(), p.MaxBurnRatio.String())
			}
		}
	}

	return nil
}

// FormatString returns a human-readable string representation of the params
func (p TokenomicsParams) FormatString() string {
	return fmt.Sprintf(`TokenomicsParams:
  Supply:
    Total Cap:        %s OMNI
    Current Supply:   %s OMNI
    Total Minted:     %s OMNI
    Total Burned:     %s OMNI
  Inflation:
    Rate:             %s%% (min: %s%%, max: %s%%)
  Emissions:
    Staking:          %s%%
    PoC:              %s%%
    Sequencer:        %s%%
    Treasury:         %s%%
  Burn Rates:
    PoS Gas:          %s%%
    PoC Anchoring:    %s%%
    Sequencer Gas:    %s%%
    Smart Contracts:  %s%%
    AI Queries:       %s%%
    Messaging:        %s%%
  Treasury:
    Burn Redirect:    %s%%
  Gas Conversion:
    Continuity:       %sx
    Sequencer:        %sx
  IBC:
    Reward Interval:  %d blocks
    Continuity Channel: %s
    Sequencer Channel:  %s
  Governance:
    Min Deposit:      %s OMNI
    Quorum:           %s%%
    Pass Threshold:   %s%%
    Voting Period:    %d seconds
    Param Delay:      %d seconds`,
		formatOMNI(p.TotalSupplyCap),
		formatOMNI(p.CurrentTotalSupply),
		formatOMNI(p.TotalMinted),
		formatOMNI(p.TotalBurned),
		formatPercent(p.InflationRate),
		formatPercent(p.InflationMin),
		formatPercent(p.InflationMax),
		formatPercent(p.EmissionSplitStaking),
		formatPercent(p.EmissionSplitPoc),
		formatPercent(p.EmissionSplitSequencer),
		formatPercent(p.EmissionSplitTreasury),
		formatPercent(p.BurnRatePosGas),
		formatPercent(p.BurnRatePocAnchoring),
		formatPercent(p.BurnRateSequencerGas),
		formatPercent(p.BurnRateSmartContracts),
		formatPercent(p.BurnRateAiQueries),
		formatPercent(p.BurnRateMessaging),
		formatPercent(p.TreasuryBurnRedirect),
		p.GasConversionRatioContinuity.String(),
		p.GasConversionRatioSequencer.String(),
		p.RewardStreamInterval,
		p.ContinuityIbcChannel,
		p.SequencerIbcChannel,
		formatOMNI(p.MinProposalDeposit),
		formatPercent(p.QuorumPercentage),
		formatPercent(p.PassPercentage),
		p.VotingPeriod,
		p.ParamChangeDelay,
	)
}

// formatOMNI converts micro-OMNI to OMNI with decimals
func formatOMNI(amount math.Int) string {
	// Divide by 1,000,000 to get OMNI from omniphi (6 decimals)
	divisor := math.NewInt(1_000_000)
	omni := amount.Quo(divisor)
	remainder := amount.Mod(divisor)

	if remainder.IsZero() {
		return omni.String()
	}

	// Format with decimals
	return fmt.Sprintf("%s.%06d", omni.String(), remainder.Uint64())
}

// formatPercent converts decimal to percentage string
func formatPercent(dec math.LegacyDec) string {
	pct := dec.MulInt64(100)
	return pct.String()
}
