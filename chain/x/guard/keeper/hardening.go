package keeper

// hardening.go — Dynamic Deterministic Guard v2
//
// Extends x/guard with:
//   1. Continuous risk reevaluation (ReevaluateRisk)
//   2. Tier escalation rules
//   3. Threshold escalation during instability
//   4. Cross-proposal risk coupling (ComputeAggregateRisk)
//   5. Global emergency hardening mode
//
// Invariants preserved:
//   - AI never shortens delay
//   - AI never lowers threshold
//   - MergeRulesAndAI still uses max()
//   - Tier may only escalate, never downgrade (monotonic)
//   - Threshold may only increase, never decrease (monotonic)
//   - Deterministic integer-only arithmetic, no floats

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"pos/x/guard/types"
)

// ============================================================================
// 1. Continuous Risk Reevaluation
// ============================================================================

// ReevaluateRisk re-evaluates the risk of a queued proposal.
//
// Called at:
//   - Each gate transition (VISIBILITY→SHOCK_ABSORBER, etc.)
//   - When validator churn exceeds the soft threshold
//   - When cumulative treasury impact changes
//
// Rules:
//   - Recomputes features from the current proposal state
//   - Merges with existing report using max()
//   - NEVER downgrades tier (monotonic constraint)
//   - NEVER lowers threshold
//   - Records reevaluation for observability
func (k Keeper) ReevaluateRisk(ctx context.Context, proposalID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get existing execution and report
	exec, found := k.GetQueuedExecution(ctx, proposalID)
	if !found {
		return types.ErrQueuedExecutionNotFound.Wrapf("proposal_id=%d", proposalID)
	}

	// Don't reevaluate terminal proposals
	if exec.IsTerminal() {
		return nil
	}

	existingReport, found := k.GetRiskReport(ctx, proposalID)
	if !found {
		return types.ErrRiskReportNotFound.Wrapf("proposal_id=%d", proposalID)
	}

	// Get the proposal from gov to recompute features
	proposal, err := k.govKeeper.GetProposal(ctx, proposalID)
	if err != nil {
		return fmt.Errorf("failed to get proposal %d: %w", proposalID, err)
	}

	// Recompute risk from current state
	freshReport, err := k.EvaluateProposal(ctx, proposal)
	if err != nil {
		return fmt.Errorf("failed to reevaluate proposal %d: %w", proposalID, err)
	}

	// Apply escalation from aggregate risk
	aggregateSnapshot := k.ComputeAggregateRisk(ctx)
	escalationReasons := []string{}

	// Check tier escalation conditions
	newTier := types.MaxConstraintTier(existingReport.Tier, freshReport.Tier)
	newThreshold := types.MaxConstraint(existingReport.ComputedThresholdBps, freshReport.ComputedThresholdBps)
	newDelay := types.MaxConstraint(existingReport.ComputedDelayBlocks, freshReport.ComputedDelayBlocks)

	// Tier escalation from environment conditions
	escalatedTier, tierReasons := k.computeTierEscalation(ctx, newTier, aggregateSnapshot)
	if escalatedTier > newTier {
		newTier = escalatedTier
		escalationReasons = append(escalationReasons, tierReasons...)
	}

	// Threshold escalation from instability
	escalatedThreshold, thresholdReasons := k.computeThresholdEscalation(ctx, proposalID, newThreshold)
	if escalatedThreshold > newThreshold {
		newThreshold = escalatedThreshold
		escalationReasons = append(escalationReasons, thresholdReasons...)
	}

	// Apply emergency hardening mode
	if k.GetEmergencyHardeningMode(ctx) {
		newTier, newThreshold, newDelay, escalationReasons = k.applyEmergencyHardening(
			newTier, newThreshold, newDelay, escalationReasons)
	}

	// Recompute delay from the (possibly escalated) tier
	params := k.GetParams(ctx)
	tierDelay := types.TierToDelayBlocks(newTier, params)
	newDelay = types.MaxConstraint(newDelay, tierDelay)

	// Recompute threshold from the (possibly escalated) tier
	tierThreshold := types.TierToThresholdBps(newTier, params)
	newThreshold = types.MaxConstraint(newThreshold, tierThreshold)

	// ── Monotonic invariant enforcement ──
	// Final values must never be less than existing values.
	newTier = types.MaxConstraintTier(newTier, existingReport.Tier)
	newThreshold = types.MaxConstraint(newThreshold, existingReport.ComputedThresholdBps)
	newDelay = types.MaxConstraint(newDelay, existingReport.ComputedDelayBlocks)

	// Check if anything changed
	changed := newTier != existingReport.Tier ||
		newThreshold != existingReport.ComputedThresholdBps ||
		newDelay != existingReport.ComputedDelayBlocks

	if !changed {
		return nil // no escalation needed
	}

	// Update EarliestExecHeight if delay changed
	if newDelay > existingReport.ComputedDelayBlocks {
		oldHeight := exec.EarliestExecHeight
		exec.EarliestExecHeight = exec.QueuedHeight + newDelay

		// Remove old index entry so it doesn't get processed at the wrong height
		_ = k.DeleteQueueIndexEntry(ctx, oldHeight, proposalID)
	}

	// Record reevaluation
	record := types.ReevaluationRecord{
		ProposalID:          proposalID,
		ReevaluatedAtHeight: sdkCtx.BlockHeight(),
		PreviousTier:        existingReport.Tier,
		NewTier:             newTier,
		PreviousThreshold:   existingReport.ComputedThresholdBps,
		NewThreshold:        newThreshold,
		PreviousDelay:       existingReport.ComputedDelayBlocks,
		NewDelay:            newDelay,
		EscalationReasons:   escalationReasons,
	}

	if err := k.SetReevaluationRecord(ctx, record); err != nil {
		k.logger.Error("failed to store reevaluation record", "error", err)
	}

	// Update the risk report
	existingReport.Tier = newTier
	existingReport.ComputedThresholdBps = newThreshold
	existingReport.ComputedDelayBlocks = newDelay
	if err := k.SetRiskReport(ctx, existingReport); err != nil {
		return fmt.Errorf("failed to update risk report: %w", err)
	}

	// Update the queued execution
	exec.Tier = newTier
	exec.RequiredThresholdBps = newThreshold

	// If tier escalated to CRITICAL, require second confirmation
	if newTier == types.RISK_TIER_CRITICAL && !exec.RequiresSecondConfirm {
		if params.CriticalRequiresSecondConfirm {
			exec.RequiresSecondConfirm = true
			exec.SecondConfirmReceived = false
			escalationReasons = append(escalationReasons, "CRITICAL_CONFIRM_NOW_REQUIRED")
		}
	}

	if err := k.SetQueuedExecution(ctx, exec); err != nil {
		return fmt.Errorf("failed to update queued execution: %w", err)
	}

	// Emit event
	reasonsJSON, _ := json.Marshal(escalationReasons)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_risk_reevaluated",
		sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
		sdk.NewAttribute("previous_tier", record.PreviousTier.GetTierName()),
		sdk.NewAttribute("new_tier", record.NewTier.GetTierName()),
		sdk.NewAttribute("previous_threshold_bps", fmt.Sprintf("%d", record.PreviousThreshold)),
		sdk.NewAttribute("new_threshold_bps", fmt.Sprintf("%d", record.NewThreshold)),
		sdk.NewAttribute("previous_delay_blocks", fmt.Sprintf("%d", record.PreviousDelay)),
		sdk.NewAttribute("new_delay_blocks", fmt.Sprintf("%d", record.NewDelay)),
		sdk.NewAttribute("escalation_reasons", string(reasonsJSON)),
	))

	k.logger.Info("risk reevaluated",
		"proposal_id", proposalID,
		"prev_tier", record.PreviousTier.GetTierName(),
		"new_tier", record.NewTier.GetTierName(),
		"reasons", escalationReasons,
	)

	return nil
}

// ============================================================================
// 2. Tier Escalation Rules
// ============================================================================

// computeTierEscalation checks environmental conditions and returns the
// escalated tier (always >= input tier) plus reasons.
//
// Escalation conditions:
//   - Validator churn > soft threshold
//   - Treasury cumulative outflow > soft threshold
//   - Multiple PARAM_CHANGE proposals active
func (k Keeper) computeTierEscalation(
	ctx context.Context,
	currentTier types.RiskTier,
	aggregate types.AggregateRiskSnapshot,
) (types.RiskTier, []string) {
	escalatedTier := currentTier
	reasons := []string{}

	// Condition 1: Validator churn above soft threshold
	churnBps := k.getCurrentValidatorChurnBps(ctx)
	if churnBps > types.ValidatorChurnSoftThresholdBps {
		escalatedTier = escalateTierByOne(escalatedTier)
		reasons = append(reasons, fmt.Sprintf("VALIDATOR_CHURN_SOFT:%d_bps", churnBps))
	}

	// Condition 2: Cumulative treasury outflow above soft threshold
	if aggregate.CumulativeTreasuryBps > types.TreasuryCumulativeSoftThresholdBps {
		escalatedTier = escalateTierByOne(escalatedTier)
		reasons = append(reasons, fmt.Sprintf("CUMULATIVE_TREASURY:%d_bps", aggregate.CumulativeTreasuryBps))
	}

	// Condition 3: Multiple param-change proposals active concurrently
	if aggregate.ParamChangesActive >= types.MultiParamChangeThreshold {
		escalatedTier = escalateTierByOne(escalatedTier)
		reasons = append(reasons, fmt.Sprintf("MULTI_PARAM_CHANGE:%d_active", aggregate.ParamChangesActive))
	}

	return escalatedTier, reasons
}

// escalateTierByOne raises a tier by one level, capped at CRITICAL.
func escalateTierByOne(tier types.RiskTier) types.RiskTier {
	switch tier {
	case types.RISK_TIER_LOW:
		return types.RISK_TIER_MED
	case types.RISK_TIER_MED:
		return types.RISK_TIER_HIGH
	case types.RISK_TIER_HIGH:
		return types.RISK_TIER_CRITICAL
	default:
		return types.RISK_TIER_CRITICAL
	}
}

// getCurrentValidatorChurnBps computes the current validator power churn
// by comparing against the most recent power snapshot. Returns 0 if no
// baseline is available.
func (k Keeper) getCurrentValidatorChurnBps(ctx context.Context) uint64 {
	// Iterate all queued executions to find one with a snapshot
	// Use the most recent snapshot as baseline
	var bestSnapshot ValidatorPowerSnapshot
	bestFound := false

	store := k.storeService.OpenKVStore(ctx)
	startKey := types.ValidatorPowerSnapshotPrefix
	endKey := types.PrefixEnd(types.ValidatorPowerSnapshotPrefix)

	iterator, err := store.Iterator(startKey, endKey)
	if err != nil {
		return 0
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var snap ValidatorPowerSnapshot
		if err := json.Unmarshal(iterator.Value(), &snap); err != nil {
			continue
		}
		if !bestFound || snap.Height > bestSnapshot.Height {
			bestSnapshot = snap
			bestFound = true
		}
	}

	if !bestFound {
		return 0
	}

	// Get current power
	currentPowers := make(map[string]int64)
	powerReduction := k.stakingKeeper.PowerReduction(ctx)
	err = k.stakingKeeper.IterateBondedValidatorsByPower(ctx, func(index int64, val stakingtypes.ValidatorI) bool {
		cp := val.GetConsensusPower(powerReduction)
		addr := val.GetOperator()
		currentPowers[addr] = cp
		return false
	})
	if err != nil {
		return 0
	}

	// Convert snapshot slice to map for computeChurnDelta
	snapPowers := make(map[string]int64)
	for _, v := range bestSnapshot.Validators {
		snapPowers[v.Address] = v.Power
	}
	churnDelta := computeChurnDelta(snapPowers, currentPowers)
	denom := bestSnapshot.TotalPower
	if denom <= 0 {
		denom = 1
	}

	return uint64(churnDelta * 10000 / denom)
}

// ============================================================================
// 3. Threshold Escalation During Instability
// ============================================================================

// computeThresholdEscalation checks instability conditions and returns
// the escalated threshold (always >= input threshold) plus reasons.
//
// Conditions:
//   - Validator churn > hard threshold
//   - Network unstable for > InstabilityWindowBlocks
//
// Threshold may increase but NEVER decrease.
func (k Keeper) computeThresholdEscalation(
	ctx context.Context,
	proposalID uint64,
	currentThreshold uint64,
) (uint64, []string) {
	reasons := []string{}
	newThreshold := currentThreshold

	churnBps := k.getCurrentValidatorChurnBps(ctx)
	params := k.GetParams(ctx)

	// Hard threshold: churn exceeds max allowed
	if churnBps > params.MaxValidatorChurnBps {
		step := types.ThresholdEscalationStepBps
		escalated := currentThreshold + step
		if escalated > types.SupermajorityThresholdBps {
			escalated = types.SupermajorityThresholdBps
		}
		if escalated > newThreshold {
			newThreshold = escalated
			reasons = append(reasons, fmt.Sprintf("CHURN_HARD_EXCEEDED:%d_bps>%d_bps", churnBps, params.MaxValidatorChurnBps))
		}
	}

	// Soft threshold with sustained instability
	if churnBps > types.ValidatorChurnSoftThresholdBps {
		// Check how long instability has been sustained
		escRecord := k.GetThresholdEscalationRecord(ctx, proposalID)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		currentHeight := sdkCtx.BlockHeight()

		if escRecord.LastEscalatedAt > 0 {
			blocksSinceEscalation := uint64(currentHeight - escRecord.LastEscalatedAt)
			if blocksSinceEscalation >= types.InstabilityWindowBlocks {
				// Sustained instability: escalate threshold again
				step := types.ThresholdEscalationStepBps
				escalated := currentThreshold + step
				if escalated > types.SupermajorityThresholdBps {
					escalated = types.SupermajorityThresholdBps
				}
				if escalated > newThreshold {
					newThreshold = escalated
					reasons = append(reasons, fmt.Sprintf("SUSTAINED_INSTABILITY:%d_blocks", blocksSinceEscalation))
				}
			}
		}
	}

	// Persist escalation record if threshold changed
	if newThreshold > currentThreshold {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		existing := k.GetThresholdEscalationRecord(ctx, proposalID)
		record := types.ThresholdEscalationRecord{
			ProposalID:        proposalID,
			OriginalThreshold: existing.OriginalThreshold,
			CurrentThreshold:  newThreshold,
			EscalationCount:   existing.EscalationCount + 1,
			LastEscalatedAt:   sdkCtx.BlockHeight(),
		}
		if existing.OriginalThreshold == 0 {
			record.OriginalThreshold = currentThreshold
		}
		if err := k.SetThresholdEscalationRecord(ctx, record); err != nil {
			k.logger.Error("failed to store threshold escalation record", "error", err)
		}
	}

	return newThreshold, reasons
}

// ============================================================================
// 4. Cross-Proposal Risk Coupling
// ============================================================================

// ComputeAggregateRisk scans all active (non-terminal) queued proposals
// and computes an aggregate risk snapshot.
//
// Detects:
//   - Treasury stacking (multiple treasury spends active)
//   - Parameter mutation bursts (multiple param changes active)
//   - Upgrade clustering (multiple upgrades active)
//
// Returns a snapshot used to escalate new and existing proposals.
func (k Keeper) ComputeAggregateRisk(ctx context.Context) types.AggregateRiskSnapshot {
	snapshot := types.AggregateRiskSnapshot{
		WindowBlocks: types.AggregateRiskWindowBlocks,
		HighestTier:  types.RISK_TIER_LOW,
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := uint64(sdkCtx.BlockHeight())

	// Fix P2: Iterate only active executions using the secondary index
	// This prevents O(N) scan of all historical proposals
	k.IterateActiveExecutions(ctx, func(exec types.QueuedExecution) bool {
		// Double-check terminal status (should be handled by index, but safe to check)

		// Skip terminal
		if exec.IsTerminal() {
			return false
		}

		// Skip if outside window
		if currentHeight > exec.QueuedHeight && (currentHeight-exec.QueuedHeight) > types.AggregateRiskWindowBlocks {
			return false
		}

		snapshot.ActiveProposalCount++
		snapshot.HighestTier = types.MaxConstraintTier(snapshot.HighestTier, exec.Tier)

		// Classify proposal to count by category
		report, found := k.GetRiskReport(ctx, exec.ProposalId)
		if !found {
			return false
		}

		// Detect proposal type from reason codes
		reasonCodes := report.ReasonCodes
		if containsAny(reasonCodes, "TREASURY_SPEND") {
			snapshot.TreasurySpendsActive++
			// Add estimated treasury BPS from the report's score as a proxy
			// (actual BPS would require re-parsing, but the score correlates)
			if report.Score >= 85 {
				snapshot.CumulativeTreasuryBps += 2500
			} else if report.Score >= 70 {
				snapshot.CumulativeTreasuryBps += 1000
			} else if report.Score >= 50 {
				snapshot.CumulativeTreasuryBps += 500
			} else {
				snapshot.CumulativeTreasuryBps += 200
			}
		}
		if containsAny(reasonCodes, "PARAM_CHANGE", "CONSENSUS_CRITICAL") {
			snapshot.ParamChangesActive++
		}
		if containsAny(reasonCodes, "SOFTWARE_UPGRADE") {
			snapshot.UpgradesActive++
		}
		return false
	})

	return snapshot
}

// ApplyAggregateRiskEscalation adjusts a proposal's tier based on aggregate risk.
// Called during OnProposalPassed to account for existing queue pressure.
func (k Keeper) ApplyAggregateRiskEscalation(
	ctx context.Context,
	report *types.RiskReport,
	aggregate types.AggregateRiskSnapshot,
) []string {
	reasons := []string{}

	// Treasury stacking: if cumulative treasury BPS is high, escalate new treasury proposals
	if containsAny(report.ReasonCodes, "TREASURY_SPEND") &&
		aggregate.CumulativeTreasuryBps >= types.TreasuryStackingThresholdBps {
		newTier := escalateTierByOne(report.Tier)
		if newTier > report.Tier {
			report.Tier = newTier
			reasons = append(reasons, fmt.Sprintf("TREASURY_STACKING:%d_cumulative_bps", aggregate.CumulativeTreasuryBps))
		}
	}

	// Parameter mutation burst
	if containsAny(report.ReasonCodes, "PARAM_CHANGE", "CONSENSUS_CRITICAL") &&
		aggregate.ParamChangesActive >= types.ParamMutationBurstThreshold {
		newTier := escalateTierByOne(report.Tier)
		if newTier > report.Tier {
			report.Tier = newTier
			reasons = append(reasons, fmt.Sprintf("PARAM_MUTATION_BURST:%d_active", aggregate.ParamChangesActive))
		}
	}

	// Upgrade clustering
	if containsAny(report.ReasonCodes, "SOFTWARE_UPGRADE") &&
		aggregate.UpgradesActive >= types.UpgradeClusteringThreshold {
		newTier := escalateTierByOne(report.Tier)
		if newTier > report.Tier {
			report.Tier = newTier
			reasons = append(reasons, fmt.Sprintf("UPGRADE_CLUSTERING:%d_active", aggregate.UpgradesActive))
		}
	}

	// If tier changed, recompute delay and threshold
	if len(reasons) > 0 {
		params := k.GetParams(ctx)
		report.ComputedDelayBlocks = types.MaxConstraint(
			report.ComputedDelayBlocks,
			types.TierToDelayBlocks(report.Tier, params),
		)
		report.ComputedThresholdBps = types.MaxConstraint(
			report.ComputedThresholdBps,
			types.TierToThresholdBps(report.Tier, params),
		)
	}

	return reasons
}

// ============================================================================
// 5. Global Emergency Hardening Mode
// ============================================================================

// GetEmergencyHardeningMode reads the emergency hardening flag from the store.
func (k Keeper) GetEmergencyHardeningMode(ctx context.Context) bool {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.EmergencyHardeningKey)
	if err != nil || bz == nil || len(bz) == 0 {
		return false
	}
	return bz[0] == 0x01
}

// SetEmergencyHardeningMode sets the emergency hardening flag.
// When enabling, records the activation block height for auto-expiry.
// Governance-only — enforced at the msg_server level.
func (k Keeper) SetEmergencyHardeningMode(ctx context.Context, enabled bool) error {
	store := k.storeService.OpenKVStore(ctx)
	val := byte(0x00)
	if enabled {
		val = 0x01
		// Record activation height for auto-expiry
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		heightBz := sdk.Uint64ToBigEndian(uint64(sdkCtx.BlockHeight()))
		if err := store.Set(types.EmergencyHardeningActivatedAtKey, heightBz); err != nil {
			return fmt.Errorf("failed to record emergency hardening activation height: %w", err)
		}
	} else {
		// Clear activation height on disable
		_ = store.Delete(types.EmergencyHardeningActivatedAtKey)
	}
	return store.Set(types.EmergencyHardeningKey, []byte{val})
}

// emergencyHardeningMaxBlocksDefault is the default auto-expiry duration.
// ~72h at 6s/block. Governance can set a custom value via MsgSetEmergencyHardeningMaxBlocks
// (stored at key 0x1C in the KV store).
const emergencyHardeningMaxBlocksDefault int64 = 50400

// getEmergencyHardeningMaxBlocks returns the configured max duration in blocks,
// falling back to the default if not set.
func (k Keeper) getEmergencyHardeningMaxBlocks(ctx context.Context) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get([]byte{0x1C})
	if err != nil || len(bz) < 8 {
		return emergencyHardeningMaxBlocksDefault
	}
	v := int64(sdk.BigEndianToUint64(bz))
	if v < 100 {
		return emergencyHardeningMaxBlocksDefault
	}
	return v
}

// CheckAndExpireEmergencyHardening auto-expires emergency hardening mode if the
// maximum duration has elapsed since activation. Called from EndBlocker. Never panics.
func (k Keeper) CheckAndExpireEmergencyHardening(ctx context.Context) {
	if !k.GetEmergencyHardeningMode(ctx) {
		return
	}

	maxBlocks := k.getEmergencyHardeningMaxBlocks(ctx)

	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.EmergencyHardeningActivatedAtKey)
	if err != nil || len(bz) < 8 {
		return // no activation record; leave flag as-is
	}

	activatedAt := int64(sdk.BigEndianToUint64(bz))
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if sdkCtx.BlockHeight()-activatedAt >= maxBlocks {
		if err := k.SetEmergencyHardeningMode(ctx, false); err != nil {
			k.Logger().Error("failed to auto-expire emergency hardening", "error", err)
			return
		}
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"guard_emergency_hardening_expired",
			sdk.NewAttribute("activated_at", fmt.Sprintf("%d", activatedAt)),
			sdk.NewAttribute("expired_at", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
			sdk.NewAttribute("duration_blocks", fmt.Sprintf("%d", sdkCtx.BlockHeight()-activatedAt)),
		))
		k.Logger().Info("emergency hardening auto-expired",
			"activated_at", activatedAt,
			"expired_at", sdkCtx.BlockHeight(),
		)
	}
}

// applyEmergencyHardening transforms tier/threshold/delay when hardening is active.
//
// Rules:
//   - All HIGH proposals treated as CRITICAL
//   - All CRITICAL require second confirmation (handled in caller)
//   - All delays multiplied by 1.5 (3/2 integer arithmetic)
func (k Keeper) applyEmergencyHardening(
	tier types.RiskTier,
	threshold uint64,
	delay uint64,
	reasons []string,
) (types.RiskTier, uint64, uint64, []string) {
	newTier := tier
	newThreshold := threshold
	newDelay := delay

	// HIGH → CRITICAL
	if tier == types.RISK_TIER_HIGH {
		newTier = types.RISK_TIER_CRITICAL
		reasons = append(reasons, "EMERGENCY_HARDENING:HIGH_TO_CRITICAL")
	}

	// All delays * 1.5
	newDelay = delay * types.HardeningDelayMultiplierNum / types.HardeningDelayMultiplierDen
	if newDelay < delay {
		newDelay = delay // overflow protection
	}
	if newDelay > delay {
		reasons = append(reasons, "EMERGENCY_HARDENING:DELAY_1.5X")
	}

	// Ensure threshold matches the (possibly escalated) tier
	if newTier == types.RISK_TIER_CRITICAL && newThreshold < 7500 {
		newThreshold = 7500 // CRITICAL minimum
		reasons = append(reasons, "EMERGENCY_HARDENING:CRITICAL_THRESHOLD")
	}

	return newTier, newThreshold, newDelay, reasons
}

// ============================================================================
// Storage helpers
// ============================================================================

// SetReevaluationRecord stores a reevaluation record.
func (k Keeper) SetReevaluationRecord(ctx context.Context, record types.ReevaluationRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal reevaluation record: %w", err)
	}
	key := types.GetReevaluationRecordKey(record.ProposalID)
	return store.Set(key, bz)
}

// GetReevaluationRecord retrieves the latest reevaluation record for a proposal.
func (k Keeper) GetReevaluationRecord(ctx context.Context, proposalID uint64) (types.ReevaluationRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReevaluationRecordKey(proposalID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ReevaluationRecord{}, false
	}
	var record types.ReevaluationRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.ReevaluationRecord{}, false
	}
	return record, true
}

// SetThresholdEscalationRecord stores a threshold escalation record.
func (k Keeper) SetThresholdEscalationRecord(ctx context.Context, record types.ThresholdEscalationRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal threshold escalation record: %w", err)
	}
	key := types.GetThresholdEscalationKey(record.ProposalID)
	return store.Set(key, bz)
}

// GetThresholdEscalationRecord retrieves the threshold escalation record for a proposal.
func (k Keeper) GetThresholdEscalationRecord(ctx context.Context, proposalID uint64) types.ThresholdEscalationRecord {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetThresholdEscalationKey(proposalID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ThresholdEscalationRecord{}
	}
	var record types.ThresholdEscalationRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.ThresholdEscalationRecord{}
	}
	return record
}

// ============================================================================
// String helpers (deterministic, no external deps)
// ============================================================================

// containsAny checks if the JSON-encoded string s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
