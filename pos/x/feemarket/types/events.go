package types

// Event types for the feemarket module
const (
	EventTypeBaseFeeUpdate    = "base_fee_update"
	EventTypeFeesProcessed    = "fees_processed"
	EventTypeBurnTierChange   = "burn_tier_change"
	EventTypeTreasuryTransfer = "treasury_transfer"

	AttributeKeyOldBaseFee       = "old_base_fee"
	AttributeKeyNewBaseFee       = "new_base_fee"
	AttributeKeyUtilization      = "utilization"
	AttributeKeyTotalFees        = "total_fees"
	AttributeKeyBurnAmount       = "burn_amount"
	AttributeKeyTreasuryAmount   = "treasury_amount"
	AttributeKeyValidatorAmount  = "validator_amount"
	AttributeKeyBurnTier         = "burn_tier"
	AttributeKeyBurnPercentage   = "burn_percentage"
)
