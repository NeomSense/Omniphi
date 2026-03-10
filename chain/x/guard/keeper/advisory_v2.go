package keeper

// advisory_v2.go — Layer 3 v2: Advisory Intelligence + Attack Memory
//
// NON-BINDING INVARIANT:
//   This file MUST NOT be imported by, or called from, any code path that
//   determines tier, delay, threshold, or gate progression. The functions
//   here are for observability, historical correlation, and future AI
//   dataset anchoring ONLY.
//
//   Specifically, none of these functions are called from:
//     - EvaluateProposal
//     - ProcessGateTransition
//     - ReevaluateRisk
//     - MergeRulesAndAI
//     - computeTierEscalation
//     - computeThresholdEscalation
//
//   They ARE called from:
//     - SubmitAdvisoryLink (msg_server.go) — user-initiated observability
//     - OnProposalTerminal — called when a proposal reaches EXECUTED/ABORTED
//     - Query handlers — read-only

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/guard/types"
)

// ============================================================================
// 1. Versioned Advisory Entry CRUD
// ============================================================================

// SubmitAdvisoryEntryV2 stores a versioned advisory entry with auto-incremented ID
// and maintains secondary indices for lightweight querying.
//
// NON-BINDING: This function does not affect tier, delay, threshold, or gate state.
func (k Keeper) SubmitAdvisoryEntryV2(ctx context.Context, entry types.AdvisoryEntryV2) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Auto-increment advisory ID for this proposal
	advisoryID := k.nextAdvisoryID(ctx, entry.ProposalID)
	entry.AdvisoryID = advisoryID
	entry.SubmittedAt = sdkCtx.BlockHeight()
	entry.SchemaVersion = types.AdvisorySchemaVersion

	// Snapshot the current risk tier at submission time
	report, found := k.GetRiskReport(ctx, entry.ProposalID)
	if found {
		entry.RiskTierAtSubmission = report.Tier
	}

	// Store the advisory entry
	if err := k.setAdvisoryEntryV2(ctx, entry); err != nil {
		return 0, fmt.Errorf("failed to store advisory entry: %w", err)
	}

	// Write secondary indices
	k.writeAdvisoryIndices(ctx, entry)

	// Emit event (non-binding, observability only)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_advisory_submitted_v2",
		sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", entry.ProposalID)),
		sdk.NewAttribute("advisory_id", fmt.Sprintf("%d", advisoryID)),
		sdk.NewAttribute("advisory_type", entry.AdvisoryType),
		sdk.NewAttribute("reporter", entry.Reporter),
		sdk.NewAttribute("risk_tier_at_submission", entry.RiskTierAtSubmission.GetTierName()),
		sdk.NewAttribute("schema_version", entry.SchemaVersion),
	))

	k.logger.Info("advisory entry submitted (non-binding)",
		"proposal_id", entry.ProposalID,
		"advisory_id", advisoryID,
		"type", entry.AdvisoryType,
	)

	return advisoryID, nil
}

// GetAdvisoryEntryV2 retrieves a specific advisory entry.
func (k Keeper) GetAdvisoryEntryV2(ctx context.Context, proposalID, advisoryID uint64) (types.AdvisoryEntryV2, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAdvisoryEntryV2Key(proposalID, advisoryID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.AdvisoryEntryV2{}, false
	}
	var entry types.AdvisoryEntryV2
	if err := json.Unmarshal(bz, &entry); err != nil {
		return types.AdvisoryEntryV2{}, false
	}
	return entry, true
}

// GetAdvisoriesForProposal returns all advisory entries for a proposal.
func (k Keeper) GetAdvisoriesForProposal(ctx context.Context, proposalID uint64) []types.AdvisoryEntryV2 {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.GetAdvisoryEntryV2PrefixByProposal(proposalID)
	endKey := types.PrefixEnd(prefix)

	iterator, err := store.Iterator(prefix, endKey)
	if err != nil {
		return nil
	}
	defer iterator.Close()

	var entries []types.AdvisoryEntryV2
	for ; iterator.Valid(); iterator.Next() {
		var entry types.AdvisoryEntryV2
		if err := json.Unmarshal(iterator.Value(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// ============================================================================
// 2. Advisory → Risk Correlation
// ============================================================================

// RecordAdvisoryCorrelation creates or updates the correlation record for a
// proposal when it reaches a terminal state. Captures the relationship between
// advisory submissions and the actual execution outcome.
//
// NON-BINDING: Pure observability — does not affect any gate/tier/delay logic.
func (k Keeper) RecordAdvisoryCorrelation(ctx context.Context, proposalID uint64, outcome string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	advisories := k.GetAdvisoriesForProposal(ctx, proposalID)
	if len(advisories) == 0 {
		return nil // no advisories to correlate
	}

	// Get the risk report for final tier
	report, _ := k.GetRiskReport(ctx, proposalID)

	// Count escalations from reevaluation record
	var escalationCount uint64
	reeval, found := k.GetReevaluationRecord(ctx, proposalID)
	if found && reeval.NewTier > reeval.PreviousTier {
		escalationCount = 1 // simplified; full count would require audit trail
	}

	correlation := types.AdvisoryCorrelation{
		ProposalID:       proposalID,
		AdvisoryCount:    uint64(len(advisories)),
		RiskTierAtFirst:  advisories[0].RiskTierAtSubmission,
		FinalRiskTier:    report.Tier,
		ExecutionOutcome: outcome,
		FinalHeight:      sdkCtx.BlockHeight(),
		EscalationCount:  escalationCount,
	}

	if err := k.setAdvisoryCorrelation(ctx, correlation); err != nil {
		return fmt.Errorf("failed to store advisory correlation: %w", err)
	}

	// Update outcome in secondary indices for all advisories
	for _, entry := range advisories {
		k.updateAdvisoryOutcomeIndex(ctx, entry, outcome)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_advisory_correlated",
		sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
		sdk.NewAttribute("advisory_count", fmt.Sprintf("%d", len(advisories))),
		sdk.NewAttribute("outcome", outcome),
		sdk.NewAttribute("final_tier", report.Tier.GetTierName()),
	))

	return nil
}

// GetAdvisoryCorrelation retrieves the correlation record for a proposal.
func (k Keeper) GetAdvisoryCorrelation(ctx context.Context, proposalID uint64) (types.AdvisoryCorrelation, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAdvisoryCorrelationKey(proposalID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.AdvisoryCorrelation{}, false
	}
	var corr types.AdvisoryCorrelation
	if err := json.Unmarshal(bz, &corr); err != nil {
		return types.AdvisoryCorrelation{}, false
	}
	return corr, true
}

// ============================================================================
// 3. Attack Memory Dataset
// ============================================================================

// RecordAttackMemory stores an attack memory entry when behavioral signals
// indicate an anomalous proposal lifecycle. Called when:
//   - A proposal is aborted
//   - A proposal is escalated multiple times
//   - A proposal's delay is extended multiple times
//   - A proposal is escalated to CRITICAL tier
//
// NON-BINDING: Pure dataset accumulation — never read by gate/tier/delay logic.
func (k Keeper) RecordAttackMemory(ctx context.Context, proposalID uint64, trigger string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get the risk report for feature hash and classification
	report, found := k.GetRiskReport(ctx, proposalID)
	if !found {
		return nil // no report = nothing to memorize
	}

	// Get the queued execution for outcome
	exec, found := k.GetQueuedExecution(ctx, proposalID)
	if !found {
		return nil
	}

	// Determine outcome
	outcome := "PENDING"
	if exec.GateState == types.EXECUTION_GATE_EXECUTED {
		outcome = "EXECUTED"
	} else if exec.GateState == types.EXECUTION_GATE_ABORTED {
		outcome = "ABORTED"
	}

	// Get escalation count from reevaluation
	var escalationCount uint64
	reeval, revalFound := k.GetReevaluationRecord(ctx, proposalID)
	if revalFound && reeval.NewTier > reeval.PreviousTier {
		escalationCount++
	}

	// Extract proposal type from reason codes
	proposalType := "UNKNOWN"
	if containsAny(report.ReasonCodes, "SOFTWARE_UPGRADE") {
		proposalType = "SOFTWARE_UPGRADE"
	} else if containsAny(report.ReasonCodes, "CONSENSUS_CRITICAL") {
		proposalType = "CONSENSUS_CRITICAL"
	} else if containsAny(report.ReasonCodes, "TREASURY_SPEND") {
		proposalType = "TREASURY_SPEND"
	} else if containsAny(report.ReasonCodes, "PARAM_CHANGE") {
		proposalType = "PARAM_CHANGE"
	} else if containsAny(report.ReasonCodes, "TEXT_ONLY") {
		proposalType = "TEXT_ONLY"
	}

	featureHash := report.FeaturesHash
	if featureHash == "" {
		featureHash = fmt.Sprintf("proposal_%d", proposalID)
	}

	entry := types.AttackMemoryEntry{
		FeatureHash:      featureHash,
		ProposalID:       proposalID,
		RecordedAt:       sdkCtx.BlockHeight(),
		ProposalType:     proposalType,
		InitialTier:      report.Tier, // initial tier from report
		FinalTier:        exec.Tier,   // final tier from execution
		EscalationCount:  escalationCount,
		WasAborted:       exec.GateState == types.EXECUTION_GATE_ABORTED,
		WasExecuted:      exec.GateState == types.EXECUTION_GATE_EXECUTED,
		ExecutionOutcome: outcome,
		TriggerReason:    trigger,
	}

	if err := k.setAttackMemoryEntry(ctx, entry); err != nil {
		return fmt.Errorf("failed to store attack memory entry: %w", err)
	}

	// Also store by proposal ID for reverse lookup
	if err := k.setAttackMemoryByProposal(ctx, proposalID, featureHash); err != nil {
		k.logger.Error("failed to index attack memory by proposal", "error", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_attack_memory_recorded",
		sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
		sdk.NewAttribute("feature_hash", featureHash),
		sdk.NewAttribute("trigger", trigger),
		sdk.NewAttribute("initial_tier", entry.InitialTier.GetTierName()),
		sdk.NewAttribute("final_tier", entry.FinalTier.GetTierName()),
		sdk.NewAttribute("outcome", outcome),
	))

	k.logger.Info("attack memory recorded",
		"proposal_id", proposalID,
		"trigger", trigger,
		"feature_hash", featureHash[:min32(16, len(featureHash))],
	)

	return nil
}

// GetAttackMemoryEntry retrieves an attack memory entry by feature hash.
func (k Keeper) GetAttackMemoryEntry(ctx context.Context, featureHash string) (types.AttackMemoryEntry, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAttackMemoryKey(featureHash)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.AttackMemoryEntry{}, false
	}
	var entry types.AttackMemoryEntry
	if err := json.Unmarshal(bz, &entry); err != nil {
		return types.AttackMemoryEntry{}, false
	}
	return entry, true
}

// GetAttackMemoryByProposal retrieves the feature hash for a proposal's attack memory.
func (k Keeper) GetAttackMemoryByProposal(ctx context.Context, proposalID uint64) (string, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAttackMemoryByProposalKey(proposalID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

// GetAllAttackMemoryEntries retrieves all attack memory entries.
func (k Keeper) GetAllAttackMemoryEntries(ctx context.Context) []types.AttackMemoryEntry {
	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(types.AttackMemoryPrefix, types.PrefixEnd(types.AttackMemoryPrefix))
	if err != nil {
		return nil
	}
	defer iterator.Close()

	var entries []types.AttackMemoryEntry
	for ; iterator.Valid(); iterator.Next() {
		var entry types.AttackMemoryEntry
		if err := json.Unmarshal(iterator.Value(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// ============================================================================
// 4. Advisory Indexing Queries
// ============================================================================

// QueryAdvisoriesByTier returns all advisory index entries for a given risk tier.
func (k Keeper) QueryAdvisoriesByTier(ctx context.Context, tier types.RiskTier) []types.AdvisoryIndexEntry {
	prefix := types.GetAdvisoryIndexByTierPrefixForTier(tier)
	endKey := types.PrefixEnd(prefix)
	return k.queryAdvisoryIndexByPrefix(ctx, prefix, endKey)
}

// QueryAdvisoriesByTrack returns all advisory index entries for a given track name.
func (k Keeper) QueryAdvisoriesByTrack(ctx context.Context, trackName string) []types.AdvisoryIndexEntry {
	prefix := types.GetAdvisoryIndexByTrackPrefixForTrack(trackName)
	endKey := types.PrefixEnd(prefix)
	return k.queryAdvisoryIndexByPrefix(ctx, prefix, endKey)
}

// QueryAdvisoriesByOutcome returns all advisory index entries for a given outcome.
func (k Keeper) QueryAdvisoriesByOutcome(ctx context.Context, outcome string) []types.AdvisoryIndexEntry {
	prefix := types.GetAdvisoryIndexByOutcomePrefixForOutcome(outcome)
	endKey := types.PrefixEnd(prefix)
	return k.queryAdvisoryIndexByPrefix(ctx, prefix, endKey)
}

// ============================================================================
// 5. Terminal Lifecycle Hook
// ============================================================================

// OnProposalTerminal is called when a proposal reaches a terminal state
// (EXECUTED or ABORTED). Records advisory correlations and attack memory.
//
// NON-BINDING: This is a pure side-effect for observability. It does not
// affect the outcome of the terminal transition.
func (k Keeper) OnProposalTerminal(ctx context.Context, proposalID uint64, outcome string) {
	// Record advisory correlation
	if err := k.RecordAdvisoryCorrelation(ctx, proposalID, outcome); err != nil {
		k.logger.Error("failed to record advisory correlation (non-fatal)",
			"proposal_id", proposalID, "error", err)
	}

	// Record attack memory for aborted proposals
	if outcome == "ABORTED" {
		if err := k.RecordAttackMemory(ctx, proposalID, types.MemoryTriggerAborted); err != nil {
			k.logger.Error("failed to record attack memory for abort (non-fatal)",
				"proposal_id", proposalID, "error", err)
		}
	}

	// Record attack memory if proposal was escalated multiple times
	reeval, found := k.GetReevaluationRecord(ctx, proposalID)
	if found && reeval.NewTier > reeval.PreviousTier {
		if reeval.NewTier == types.RISK_TIER_CRITICAL && reeval.PreviousTier < types.RISK_TIER_CRITICAL {
			if err := k.RecordAttackMemory(ctx, proposalID, types.MemoryTriggerCriticalEscalation); err != nil {
				k.logger.Error("failed to record escalation memory (non-fatal)",
					"proposal_id", proposalID, "error", err)
			}
		}
	}
}

// ============================================================================
// Internal storage helpers
// ============================================================================

func (k Keeper) setAdvisoryEntryV2(ctx context.Context, entry types.AdvisoryEntryV2) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	key := types.GetAdvisoryEntryV2Key(entry.ProposalID, entry.AdvisoryID)
	return store.Set(key, bz)
}

func (k Keeper) setAdvisoryCorrelation(ctx context.Context, corr types.AdvisoryCorrelation) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(corr)
	if err != nil {
		return err
	}
	key := types.GetAdvisoryCorrelationKey(corr.ProposalID)
	return store.Set(key, bz)
}

func (k Keeper) setAttackMemoryEntry(ctx context.Context, entry types.AttackMemoryEntry) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	key := types.GetAttackMemoryKey(entry.FeatureHash)
	return store.Set(key, bz)
}

func (k Keeper) setAttackMemoryByProposal(ctx context.Context, proposalID uint64, featureHash string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAttackMemoryByProposalKey(proposalID)
	return store.Set(key, []byte(featureHash))
}

// nextAdvisoryID returns the next advisory ID for a proposal and increments the counter.
func (k Keeper) nextAdvisoryID(ctx context.Context, proposalID uint64) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAdvisoryCounterKey(proposalID)

	bz, err := store.Get(key)
	var current uint64
	if err == nil && bz != nil && len(bz) == 8 {
		current = types.BigEndianToUint64(bz)
	}

	next := current + 1
	_ = store.Set(key, types.SdkUint64ToBigEndian(next))
	return next
}

// writeAdvisoryIndices writes secondary index entries for an advisory.
func (k Keeper) writeAdvisoryIndices(ctx context.Context, entry types.AdvisoryEntryV2) {
	store := k.storeService.OpenKVStore(ctx)

	indexEntry := types.AdvisoryIndexEntry{
		ProposalID:   entry.ProposalID,
		AdvisoryID:   entry.AdvisoryID,
		Tier:         entry.RiskTierAtSubmission,
		TrackName:    entry.TrackName,
		AdvisoryType: entry.AdvisoryType,
	}

	bz, err := json.Marshal(indexEntry)
	if err != nil {
		return
	}

	// Index by tier
	tierKey := types.GetAdvisoryIndexByTierKey(entry.RiskTierAtSubmission, entry.ProposalID, entry.AdvisoryID)
	_ = store.Set(tierKey, bz)

	// Index by track (if available)
	if entry.TrackName != "" {
		trackKey := types.GetAdvisoryIndexByTrackKey(entry.TrackName, entry.ProposalID, entry.AdvisoryID)
		_ = store.Set(trackKey, bz)
	}
}

// updateAdvisoryOutcomeIndex writes the outcome index entry after terminal state.
func (k Keeper) updateAdvisoryOutcomeIndex(ctx context.Context, entry types.AdvisoryEntryV2, outcome string) {
	store := k.storeService.OpenKVStore(ctx)

	indexEntry := types.AdvisoryIndexEntry{
		ProposalID:   entry.ProposalID,
		AdvisoryID:   entry.AdvisoryID,
		Tier:         entry.RiskTierAtSubmission,
		TrackName:    entry.TrackName,
		Outcome:      outcome,
		AdvisoryType: entry.AdvisoryType,
	}

	bz, err := json.Marshal(indexEntry)
	if err != nil {
		return
	}

	outcomeKey := types.GetAdvisoryIndexByOutcomeKey(outcome, entry.ProposalID, entry.AdvisoryID)
	_ = store.Set(outcomeKey, bz)
}

// queryAdvisoryIndexByPrefix iterates an advisory index prefix and returns entries.
func (k Keeper) queryAdvisoryIndexByPrefix(ctx context.Context, prefix, endKey []byte) []types.AdvisoryIndexEntry {
	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(prefix, endKey)
	if err != nil {
		return nil
	}
	defer iterator.Close()

	var entries []types.AdvisoryIndexEntry
	for ; iterator.Valid(); iterator.Next() {
		var entry types.AdvisoryIndexEntry
		if err := json.Unmarshal(iterator.Value(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// min32 returns the minimum of two ints (for string truncation).
func min32(a, b int) int {
	if a < b {
		return a
	}
	return b
}
