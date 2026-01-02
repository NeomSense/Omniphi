package types

// Event types for the feemarket module
const (
	EventTypeBaseFeeUpdate    = "base_fee_update"
	EventTypeFeesProcessed    = "fees_processed"
	EventTypeBurnTierChange   = "burn_tier_change"
	EventTypeTreasuryTransfer = "treasury_transfer"
	EventTypeUnifiedBurn      = "unified_burn" // Single-pass burn model event

	// Anchor Lane Monitoring Events
	EventTypeBlockMetrics       = "anchor_block_metrics"        // Per-block performance metrics
	EventTypeHighUtilization    = "anchor_high_utilization"     // Warning: utilization > 70%
	EventTypeLongBlockExecution = "anchor_long_block_execution" // Warning: execution > 2.5s

	AttributeKeyOldBaseFee         = "old_base_fee"
	AttributeKeyNewBaseFee         = "new_base_fee"
	AttributeKeyUtilization        = "utilization"
	AttributeKeyTotalFees          = "total_fees"
	AttributeKeyBurnAmount         = "burn_amount"
	AttributeKeyTreasuryAmount     = "treasury_amount"
	AttributeKeyValidatorAmount    = "validator_amount"
	AttributeKeyBurnTier           = "burn_tier"
	AttributeKeyBurnPercentage     = "burn_percentage"
	AttributeKeyActivityType       = "activity_type"
	AttributeKeyBaseBurnRate       = "base_burn_rate"
	AttributeKeyActivityMultiplier = "activity_multiplier"
	AttributeKeyEffectiveBurnRate  = "effective_burn_rate"
	AttributeKeyWasCapped          = "was_capped"

	// Anchor Lane Monitoring Attributes
	AttributeKeyBlockHeight        = "block_height"
	AttributeKeyGasUsed            = "gas_used"
	AttributeKeyGasLimit           = "gas_limit"
	AttributeKeyTxCount            = "tx_count"
	AttributeKeyExecutionTime      = "execution_time_ms"
	AttributeKeyWarningType        = "warning_type"
	AttributeKeyWarningThreshold   = "threshold"
	AttributeKeyWarningActual      = "actual_value"
)
