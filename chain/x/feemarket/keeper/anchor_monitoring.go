package keeper

import (
	"context"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/feemarket/types"
)

// ===============================================================================
// ANCHOR LANE MONITORING
// ===============================================================================
// These functions emit events and log warnings for anchor lane health monitoring.
// Operators should watch for:
// - EventTypeHighUtilization: Sustained >70% utilization (may indicate issues)
// - EventTypeLongBlockExecution: Block execution >2.5s (validator performance)
// - EventTypeTxGasViolation: Transaction exceeds MaxTxGas (should never happen with ante)
//
// Target metrics for healthy anchor lane:
// - Block time: ~4 seconds
// - Utilization: 20-50% typical, <70% sustained
// - Block execution: <2.5 seconds typical
// - MaxTxGas: 2,000,000 (3.3% of block - prevents single tx dominance)
//
// Gas sizing rationale (IMMUTABLE DESIGN CONSTRAINTS):
// - MaxBlockGas: 60M (target_TPS × block_time × avg_tx_gas = 100 × 4 × 150k)
// - MaxTxGas: 2M (3.3% of block, requires 30+ txs to fill)
// - Protocol hard caps cannot be exceeded via governance
// ===============================================================================

const (
	// HighUtilizationThreshold triggers a warning when utilization exceeds this
	// 70% is concerning; 90%+ is critical
	HighUtilizationThreshold = "0.70"

	// LongExecutionThresholdMs triggers a warning when block execution exceeds this
	// 2500ms leaves ~1.5s headroom for consensus in a 4s block
	LongExecutionThresholdMs = 2500

	// CriticalUtilizationThreshold logs at error level
	CriticalUtilizationThreshold = "0.90"

	// CriticalExecutionThresholdMs logs at error level
	CriticalExecutionThresholdMs = 3500
)

// EmitBlockMetrics emits per-block performance metrics for monitoring
// This should be called at EndBlock to track anchor lane health
func (k Keeper) EmitBlockMetrics(ctx context.Context, gasUsed, gasLimit int64, txCount int, executionTimeMs int64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Calculate utilization
	var utilization math.LegacyDec
	if gasLimit > 0 {
		utilization = math.LegacyNewDec(gasUsed).QuoInt64(gasLimit)
	} else {
		utilization = math.LegacyZeroDec()
	}

	// Emit block metrics event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBlockMetrics,
			sdk.NewAttribute(types.AttributeKeyBlockHeight, math.NewInt(sdkCtx.BlockHeight()).String()),
			sdk.NewAttribute(types.AttributeKeyGasUsed, math.NewInt(gasUsed).String()),
			sdk.NewAttribute(types.AttributeKeyGasLimit, math.NewInt(gasLimit).String()),
			sdk.NewAttribute(types.AttributeKeyUtilization, utilization.String()),
			sdk.NewAttribute(types.AttributeKeyTxCount, math.NewInt(int64(txCount)).String()),
			sdk.NewAttribute(types.AttributeKeyExecutionTime, math.NewInt(executionTimeMs).String()),
		),
	)

	// Check for high utilization warning
	k.checkUtilizationThresholds(ctx, utilization)

	// Check for long execution warning
	k.checkExecutionThresholds(ctx, executionTimeMs)
}

// checkUtilizationThresholds emits warnings for high gas utilization
func (k Keeper) checkUtilizationThresholds(ctx context.Context, utilization math.LegacyDec) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	highThreshold := math.LegacyMustNewDecFromStr(HighUtilizationThreshold)
	criticalThreshold := math.LegacyMustNewDecFromStr(CriticalUtilizationThreshold)

	if utilization.GTE(criticalThreshold) {
		// Critical: >90% utilization
		k.Logger(ctx).Error("CRITICAL: Anchor lane utilization at critical level",
			"utilization", utilization.String(),
			"threshold", CriticalUtilizationThreshold,
			"block_height", sdkCtx.BlockHeight(),
		)

		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeHighUtilization,
				sdk.NewAttribute(types.AttributeKeyWarningType, "critical"),
				sdk.NewAttribute(types.AttributeKeyWarningThreshold, CriticalUtilizationThreshold),
				sdk.NewAttribute(types.AttributeKeyWarningActual, utilization.String()),
				sdk.NewAttribute(types.AttributeKeyBlockHeight, math.NewInt(sdkCtx.BlockHeight()).String()),
			),
		)
	} else if utilization.GTE(highThreshold) {
		// Warning: >70% utilization
		k.Logger(ctx).Warn("WARNING: Anchor lane utilization elevated",
			"utilization", utilization.String(),
			"threshold", HighUtilizationThreshold,
			"block_height", sdkCtx.BlockHeight(),
		)

		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeHighUtilization,
				sdk.NewAttribute(types.AttributeKeyWarningType, "warning"),
				sdk.NewAttribute(types.AttributeKeyWarningThreshold, HighUtilizationThreshold),
				sdk.NewAttribute(types.AttributeKeyWarningActual, utilization.String()),
				sdk.NewAttribute(types.AttributeKeyBlockHeight, math.NewInt(sdkCtx.BlockHeight()).String()),
			),
		)
	}
}

// checkExecutionThresholds emits warnings for slow block execution
func (k Keeper) checkExecutionThresholds(ctx context.Context, executionTimeMs int64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if executionTimeMs >= CriticalExecutionThresholdMs {
		// Critical: >3.5s execution
		k.Logger(ctx).Error("CRITICAL: Block execution time exceeds safe limit",
			"execution_time_ms", executionTimeMs,
			"threshold_ms", CriticalExecutionThresholdMs,
			"block_height", sdkCtx.BlockHeight(),
		)

		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeLongBlockExecution,
				sdk.NewAttribute(types.AttributeKeyWarningType, "critical"),
				sdk.NewAttribute(types.AttributeKeyWarningThreshold, math.NewInt(CriticalExecutionThresholdMs).String()),
				sdk.NewAttribute(types.AttributeKeyWarningActual, math.NewInt(executionTimeMs).String()),
				sdk.NewAttribute(types.AttributeKeyBlockHeight, math.NewInt(sdkCtx.BlockHeight()).String()),
			),
		)
	} else if executionTimeMs >= LongExecutionThresholdMs {
		// Warning: >2.5s execution
		k.Logger(ctx).Warn("WARNING: Block execution time elevated",
			"execution_time_ms", executionTimeMs,
			"threshold_ms", LongExecutionThresholdMs,
			"block_height", sdkCtx.BlockHeight(),
		)

		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeLongBlockExecution,
				sdk.NewAttribute(types.AttributeKeyWarningType, "warning"),
				sdk.NewAttribute(types.AttributeKeyWarningThreshold, math.NewInt(LongExecutionThresholdMs).String()),
				sdk.NewAttribute(types.AttributeKeyWarningActual, math.NewInt(executionTimeMs).String()),
				sdk.NewAttribute(types.AttributeKeyBlockHeight, math.NewInt(sdkCtx.BlockHeight()).String()),
			),
		)
	}
}

// ValidateAnchorLaneConfig validates that anchor lane parameters are within safe bounds
// This should be called at genesis and when parameters are updated
//
// IMPORTANT: This function enforces protocol hard caps that CANNOT be exceeded via governance.
// The hard caps are defined in types.ProtocolMax*HardCap constants.
func ValidateAnchorLaneConfig(maxBlockGas, maxTxGas int64, targetUtil math.LegacyDec) error {
	// Validate max block gas against protocol hard caps
	if maxBlockGas < types.ProtocolMinBlockGas {
		return types.ErrInvalidMaxBlockGas.Wrapf("too low: %d < %d minimum",
			maxBlockGas, types.ProtocolMinBlockGas)
	}
	if maxBlockGas > types.ProtocolMaxBlockGasHardCap {
		return types.ErrInvalidMaxBlockGas.Wrapf("exceeds protocol hard cap: %d > %d",
			maxBlockGas, types.ProtocolMaxBlockGasHardCap)
	}

	// Validate max tx gas against protocol hard cap
	if maxTxGas > types.ProtocolMaxTxGasHardCap {
		return types.ErrInvalidMaxTxGas.Wrapf("exceeds protocol hard cap: %d > %d",
			maxTxGas, types.ProtocolMaxTxGasHardCap)
	}

	// Validate max tx gas is reasonable relative to block gas
	// Max tx should be at most 5% of block gas to prevent single-tx dominance
	// (2M / 60M = 3.3%, which is safe)
	maxReasonableTxGas := maxBlockGas / 20 // 5% of block
	if maxTxGas > maxReasonableTxGas {
		return types.ErrInvalidMaxTxGas.Wrapf(
			"max_tx_gas (%d) > 5%% of max_block_gas (%d). "+
				"Anchor lane requires no single transaction to dominate a block. "+
				"Recommended: max_tx_gas = %d (current anchor lane default)",
			maxTxGas, maxReasonableTxGas, types.AnchorLaneMaxTxGas)
	}

	// Validate target utilization (20% - 50% for anchor lane)
	minTarget := math.LegacyMustNewDecFromStr("0.20")
	maxTarget := math.LegacyMustNewDecFromStr("0.50")
	if targetUtil.LT(minTarget) || targetUtil.GT(maxTarget) {
		return types.ErrInvalidMaxBlockGas.Wrapf("target_utilization (%s) outside anchor lane range [0.20, 0.50]",
			targetUtil.String())
	}

	return nil
}

// ValidateMaxTxGasForAnchorLane validates a single transaction's gas limit
// This is a convenience function for ante handlers and tx validation
func ValidateMaxTxGasForAnchorLane(txGasLimit uint64, maxTxGas int64) error {
	if int64(txGasLimit) > maxTxGas {
		return types.ErrInvalidMaxTxGas.Wrapf(
			"transaction gas limit (%d) exceeds MaxTxGas (%d). "+
				"The anchor lane is for staking, governance, and PoC only. "+
				"Heavy computation must use PoSeq.",
			txGasLimit, maxTxGas)
	}
	return nil
}

// CalculateEffectiveTPS calculates the effective TPS based on current block metrics
// blockTimeSeconds should be the actual time since last block
func CalculateEffectiveTPS(txCount int, blockTimeSeconds float64) float64 {
	if blockTimeSeconds <= 0 {
		return 0
	}
	return float64(txCount) / blockTimeSeconds
}

// IsWithinAnchorLaneTargets checks if current metrics are within healthy ranges
func IsWithinAnchorLaneTargets(utilization math.LegacyDec, executionTimeMs int64, tps float64) (healthy bool, warnings []string) {
	warnings = make([]string, 0)
	healthy = true

	// Check utilization (target: 20-50%, warning: >70%)
	highUtil := math.LegacyMustNewDecFromStr(HighUtilizationThreshold)
	if utilization.GT(highUtil) {
		warnings = append(warnings, "utilization above 70%")
		healthy = false
	}

	// Check execution time (warning: >2500ms)
	if executionTimeMs > LongExecutionThresholdMs {
		warnings = append(warnings, "block execution exceeds 2.5s")
		healthy = false
	}

	// Check TPS (target: 50-150, warning: <25 or >200)
	if tps < 25 {
		warnings = append(warnings, "TPS below minimum target")
	}
	if tps > 200 {
		warnings = append(warnings, "TPS above safe maximum")
		healthy = false
	}

	return healthy, warnings
}

// BlockExecutionTimer is a helper for timing block execution
type BlockExecutionTimer struct {
	startTime time.Time
}

// NewBlockExecutionTimer creates a new timer starting now
func NewBlockExecutionTimer() *BlockExecutionTimer {
	return &BlockExecutionTimer{
		startTime: time.Now(),
	}
}

// ElapsedMs returns the elapsed time in milliseconds
func (t *BlockExecutionTimer) ElapsedMs() int64 {
	return time.Since(t.startTime).Milliseconds()
}
