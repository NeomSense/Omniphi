package types

// DefaultParams returns default module parameters
func DefaultParams() Params {
	// Approximately 3 days at 5s blocks
	blocksPerDay := uint64(17280)

	return Params{
		// Base delays per tier
		DelayLowBlocks:      blocksPerDay,        // 1 day
		DelayMedBlocks:      blocksPerDay * 2,    // 2 days
		DelayHighBlocks:     blocksPerDay * 7,    // 7 days
		DelayCriticalBlocks: blocksPerDay * 14,   // 14 days

		// Gate windows
		VisibilityWindowBlocks:     blocksPerDay,     // 1 day visibility
		ShockAbsorberWindowBlocks:  blocksPerDay * 2, // 2 days shock absorber

		// Thresholds in basis points
		ThresholdDefaultBps:  5000, // 50%
		ThresholdHighBps:     6667, // 66.67%
		ThresholdCriticalBps: 7500, // 75%

		// Treasury throttle
		TreasuryThrottleEnabled:       true,
		TreasuryMaxOutflowBpsPerDay:   1000, // 10% per day max

		// Stability checks
		EnableStabilityChecks:  true,
		MaxValidatorChurnBps:   2000, // 20% max validator power change

		// AI flags (disabled for now)
		AdvisoryAiEnabled: false,
		BindingAiEnabled:  false,
		AiShadowMode:      false,

		// CRITICAL confirmation
		CriticalRequiresSecondConfirm:     true,
		CriticalSecondConfirmWindowBlocks: blocksPerDay * 3, // 3 days to confirm

		// Extensions when conditions fail
		ExtensionHighBlocks:     blocksPerDay,     // 1 day extension
		ExtensionCriticalBlocks: blocksPerDay * 3, // 3 days extension

		// Anti-DOS: bounded per-block processing
		MaxProposalsPerBlock: 10,  // max proposals polled from gov per block
		MaxQueueScanDepth:    100, // max queue entries scanned per block

		// Timelock integration: receive proposals via direct keeper call from x/timelock
		TimelockIntegrationEnabled: true,
	}
}

// Validate performs basic validation of module parameters
func (p Params) Validate() error {
	if p.DelayLowBlocks == 0 || p.DelayMedBlocks == 0 || p.DelayHighBlocks == 0 || p.DelayCriticalBlocks == 0 {
		return ErrInvalidParams.Wrap("delay blocks must be greater than 0")
	}

	if p.DelayLowBlocks > p.DelayMedBlocks || p.DelayMedBlocks > p.DelayHighBlocks || p.DelayHighBlocks > p.DelayCriticalBlocks {
		return ErrInvalidParams.Wrap("delays must be ordered: LOW <= MED <= HIGH <= CRITICAL")
	}

	if p.VisibilityWindowBlocks == 0 || p.ShockAbsorberWindowBlocks == 0 {
		return ErrInvalidParams.Wrap("gate window blocks must be greater than 0")
	}

	if p.ThresholdDefaultBps > 10000 || p.ThresholdHighBps > 10000 || p.ThresholdCriticalBps > 10000 {
		return ErrInvalidParams.Wrap("thresholds cannot exceed 10000 basis points (100%)")
	}

	if p.ThresholdDefaultBps > p.ThresholdHighBps || p.ThresholdHighBps > p.ThresholdCriticalBps {
		return ErrInvalidParams.Wrap("thresholds must be ordered: DEFAULT <= HIGH <= CRITICAL")
	}

	if p.TreasuryMaxOutflowBpsPerDay > 10000 {
		return ErrInvalidParams.Wrap("treasury max outflow cannot exceed 10000 basis points (100%)")
	}

	if p.MaxValidatorChurnBps > 10000 {
		return ErrInvalidParams.Wrap("max validator churn cannot exceed 10000 basis points (100%)")
	}

	if p.ExtensionHighBlocks == 0 || p.ExtensionCriticalBlocks == 0 {
		return ErrInvalidParams.Wrap("extension blocks must be greater than 0")
	}

	if p.MaxProposalsPerBlock == 0 {
		return ErrInvalidParams.Wrap("max_proposals_per_block must be greater than 0")
	}
	if p.MaxQueueScanDepth == 0 {
		return ErrInvalidParams.Wrap("max_queue_scan_depth must be greater than 0")
	}
	if p.MaxProposalsPerBlock > 1000 {
		return ErrInvalidParams.Wrap("max_proposals_per_block cannot exceed 1000")
	}
	if p.MaxQueueScanDepth > 10000 {
		return ErrInvalidParams.Wrap("max_queue_scan_depth cannot exceed 10000")
	}

	return nil
}
