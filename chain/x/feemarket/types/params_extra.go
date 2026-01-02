package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// ===============================================================================
// ANCHOR LANE GAS CONSTANTS
// ===============================================================================
// These values are calculated for ~100 TPS with 4-second blocks.
// Formula: target_TPS × block_time × avg_tx_gas = 100 × 4 × 150,000 = 60,000,000
//
// CRITICAL: These define protocol-level hard limits. Changes require careful
// analysis of validator hardware capabilities and network decentralization.
//
// IMPORTANT: This is NOT a smart contract execution environment.
// The anchor lane exists solely for: staking, governance, PoC, and protocol state.
// High-throughput smart contracts run on PoSeq, which commits to this anchor.
// ===============================================================================

const (
	// =========================================================================
	// PROTOCOL HARD CAPS (IMMUTABLE - CANNOT BE EXCEEDED VIA GOVERNANCE)
	// =========================================================================

	// ProtocolMaxBlockGasHardCap is the absolute maximum block gas (120M)
	// This is an immutable protocol limit - governance cannot exceed this
	ProtocolMaxBlockGasHardCap int64 = 120_000_000

	// ProtocolMaxTxGasHardCap is the absolute maximum tx gas (10M)
	// Allows existing chains with higher limits to continue operating
	// New deployments should use AnchorLaneMaxTxGas (2M) for decentralization
	// TODO: Lower to 5M via coordinated upgrade once all chains migrate
	ProtocolMaxTxGasHardCap int64 = 10_000_000

	// ProtocolMinBlockGas is the minimum block gas (10M)
	// Prevents governance from making the chain unusable
	ProtocolMinBlockGas int64 = 10_000_000

	// =========================================================================
	// ANCHOR LANE DEFAULTS (Governance can adjust within hard cap bounds)
	// =========================================================================

	// AnchorLaneMaxBlockGas is the default maximum gas per block (60M)
	// Calculated: 100 TPS × 4s blocks × 150k avg gas = 60,000,000
	// Safe operational range: 45M - 90M
	AnchorLaneMaxBlockGas int64 = 60_000_000

	// AnchorLaneMaxTxGas is the maximum gas per transaction (2M)
	// =========================================================================
	// WHY 2M IS THE CORRECT VALUE:
	// =========================================================================
	// 1. 2M gas = 3.3% of a 60M block
	// 2. Requires 30+ max-size txs to fill a block (no single tx dominance)
	// 3. Forces heavy computation off the anchor lane (to PoSeq)
	// 4. Prevents governance griefing attacks
	// 5. Ensures PoC submissions cannot monopolize blocks
	// 6. No smart contracts exist on anchor lane - 2M is sufficient
	// 7. Protects lower-end validators from resource exhaustion
	// =========================================================================
	AnchorLaneMaxTxGas int64 = 2_000_000

	// AnchorLaneTargetBlockGas is the target gas utilization (20M = 33% of 60M)
	// EIP-1559 base fee targets this utilization level
	// 33% target leaves 67% headroom for burst absorption
	AnchorLaneTargetBlockGas int64 = 20_000_000

	// =========================================================================
	// BLOCK TIME CONSTANTS
	// =========================================================================

	// AnchorLaneTargetBlockTimeMs is the target block time (4000ms = 4s)
	AnchorLaneTargetBlockTimeMs int64 = 4000

	// AnchorLaneMaxBlockExecutionMs is the max execution time before warning (2500ms)
	// Block execution should complete well within the block time
	AnchorLaneMaxBlockExecutionMs int64 = 2500

	// =========================================================================
	// TPS CALCULATION REFERENCE
	// =========================================================================
	// Target TPS: 100
	// Block time: 4s
	// Avg tx gas: 150,000
	// Formula: TPS × block_time × avg_gas = block_gas
	// Result: 100 × 4 × 150,000 = 60,000,000
	// =========================================================================
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

// DefaultParams returns the default feemarket parameters for the anchor lane.
// Tuned for ~100 TPS sustained throughput with 4-second block times.
func DefaultParams() FeeMarketParams {
	return FeeMarketParams{
		// =========================================================================
		// GAS PRICING (EIP-1559 Style)
		// =========================================================================
		// Conservative pricing for anchor lane (staking, governance, PoC)
		// Not tuned for high-frequency trading or smart contract spam

		// MinGasPrice: Protocol floor for gas pricing (0.025 omniphi/gas)
		// For a 150k gas tx: 0.025 × 150,000 = 3,750 omniphi minimum fee
		MinGasPrice: math.LegacyMustNewDecFromStr("0.025"),

		// BaseFeeEnabled: Use EIP-1559 dynamic base fee
		BaseFeeEnabled: true,

		// BaseFeeInitial: Starting base fee (0.025 omniphi/gas)
		// Same as min to start at floor, will adjust based on utilization
		BaseFeeInitial: math.LegacyMustNewDecFromStr("0.025"),

		// ElasticityMultiplier: How fast base fee adjusts (12.5% per block)
		// Conservative for anchor lane - allows gradual price discovery
		ElasticityMultiplier: math.LegacyMustNewDecFromStr("1.125"),

		// MaxTipRatio: Maximum priority fee as ratio of base fee (20%)
		// Prevents excessive tip wars on anchor transactions
		MaxTipRatio: math.LegacyMustNewDecFromStr("0.20"),

		// =========================================================================
		// BLOCK UTILIZATION (Anchor Lane Targets)
		// =========================================================================
		// 33% target leaves 67% headroom for burst absorption
		// Critical for PoC submissions and governance vote spikes

		// TargetBlockUtilization: EIP-1559 target (33%)
		// Below target = base fee decreases, above = increases
		TargetBlockUtilization: math.LegacyMustNewDecFromStr("0.33"),

		// MaxTxGas: Per-transaction gas limit (2M)
		// Prevents any single tx from using >3.3% of block capacity
		// Requires 30+ txs to fill a block - no single tx dominance
		// Forces heavy computation to PoSeq (off anchor lane)
		MaxTxGas: AnchorLaneMaxTxGas,

		// FreeTxQuota: Reserved capacity for system transactions (100 tx/block)
		// Ensures critical transactions can always be included
		FreeTxQuota: 100,

		// =========================================================================
		// ADAPTIVE BURN TIERS
		// =========================================================================
		// Burn rate increases with utilization to deter spam during congestion
		// Tiers: Cool (low load) → Normal → Hot (high load)

		// BurnCool: Burn rate when utilization < 16% (10% burn)
		// Low burn during quiet periods
		BurnCool: math.LegacyMustNewDecFromStr("0.10"),

		// BurnNormal: Burn rate when 16% ≤ utilization < 33% (20% burn)
		// Normal operating range burn
		BurnNormal: math.LegacyMustNewDecFromStr("0.20"),

		// BurnHot: Burn rate when utilization ≥ 33% (40% burn)
		// Aggressive burn during congestion
		BurnHot: math.LegacyMustNewDecFromStr("0.40"),

		// UtilCoolThreshold: Boundary for cool tier (16%)
		// Below this = low activity, minimal burn
		UtilCoolThreshold: math.LegacyMustNewDecFromStr("0.16"),

		// UtilHotThreshold: Boundary for hot tier (33%)
		// At/above target = high demand, max burn
		UtilHotThreshold: math.LegacyMustNewDecFromStr("0.33"),

		// =========================================================================
		// FEE DISTRIBUTION (Post-Burn)
		// =========================================================================
		// After burning, remaining fees split between validators and treasury

		// ValidatorFeeRatio: Validators receive 70% of post-burn fees
		ValidatorFeeRatio: math.LegacyMustNewDecFromStr("0.70"),

		// TreasuryFeeRatio: Treasury receives 30% of post-burn fees
		TreasuryFeeRatio: math.LegacyMustNewDecFromStr("0.30"),

		// =========================================================================
		// SAFETY LIMITS (Protocol Enforced)
		// =========================================================================
		// Hard caps to prevent runaway burns or economic attacks

		// MaxBurnRatio: Hard cap on burn percentage (50%)
		// IMMUTABLE SAFETY: Cannot burn more than half of fees
		MaxBurnRatio: math.LegacyMustNewDecFromStr("0.50"),

		// MinGasPriceFloor: Absolute minimum gas price (0.01 omniphi/gas)
		// Governance cannot set min_gas_price below this
		MinGasPriceFloor: math.LegacyMustNewDecFromStr("0.01"),

		// =========================================================================
		// ACTIVITY MULTIPLIERS (Single-Pass Burn Model)
		// =========================================================================
		// Different activity types have different burn rates
		// Multiplier × base burn rate = effective burn for activity type

		// MultiplierMessaging: IBC/messaging (50% of base burn)
		// Low burn to encourage cross-chain communication
		MultiplierMessaging: math.LegacyMustNewDecFromStr("0.50"),

		// MultiplierPosGas: Standard PoS operations (100% = baseline)
		// Staking, delegation, undelegation at baseline burn
		MultiplierPosGas: math.LegacyMustNewDecFromStr("1.00"),

		// MultiplierPocAnchoring: PoC contributions (75% of base)
		// Slightly discounted to encourage contributions
		MultiplierPocAnchoring: math.LegacyMustNewDecFromStr("0.75"),

		// MultiplierSmartContracts: Contract execution (150% of base)
		// Higher burn for compute-intensive operations
		// Note: Smart contracts primarily run on PoSeq, not anchor lane
		MultiplierSmartContracts: math.LegacyMustNewDecFromStr("1.50"),

		// MultiplierAiQueries: AI/oracle queries (125% of base)
		MultiplierAiQueries: math.LegacyMustNewDecFromStr("1.25"),

		// MultiplierSequencer: Sequencer commits (125% of base)
		// PoSeq commits to anchor lane at elevated rate
		MultiplierSequencer: math.LegacyMustNewDecFromStr("1.25"),

		// MinMultiplier: Floor for any multiplier (25%)
		MinMultiplier: math.LegacyMustNewDecFromStr("0.25"),

		// MaxMultiplier: Ceiling for any multiplier (200%)
		MaxMultiplier: math.LegacyMustNewDecFromStr("2.00"),
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

	// =========================================================================
	// PROTOCOL HARD CAP ENFORCEMENT (IMMUTABLE)
	// =========================================================================
	// MaxTxGas is strictly limited to protect anchor lane decentralization.
	// This is NOT a smart contract execution environment.
	// Heavy computation belongs on PoSeq, not the anchor lane.
	// =========================================================================
	if p.MaxTxGas < 100_000 {
		return fmt.Errorf("max tx gas must be at least 100k, got: %d", p.MaxTxGas)
	}
	if p.MaxTxGas > ProtocolMaxTxGasHardCap {
		return fmt.Errorf("max tx gas (%d) exceeds protocol hard cap (%d) - this limit protects anchor lane decentralization",
			p.MaxTxGas, ProtocolMaxTxGasHardCap)
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
