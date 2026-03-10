package keeper

import (
	"context"
	"fmt"
	"strings"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"pos/x/guard/types"
)

// ============================================================================
// Layer 2: Deterministic AI Evaluation
// ============================================================================

// AIEvalResult holds the result of AI inference for a proposal.
// This is a keeper-internal type; the proto RiskReport carries the on-chain fields.
type AIEvalResult struct {
	Score         uint32
	Tier          types.RiskTier
	DelayBlocks   uint64
	ThresholdBps  uint64
	ModelVersion  string
	SchemaHash    string
	FeaturesHash  string
}

// EvaluateProposalAI performs deterministic AI-based risk evaluation.
// Returns AIEvalResult or error if AI model is not configured.
func (k Keeper) EvaluateProposalAI(ctx context.Context, proposal govtypes.Proposal) (AIEvalResult, error) {
	// Get linear scoring model
	model, found := k.GetLinearModel(ctx)
	if !found {
		return AIEvalResult{}, fmt.Errorf("linear scoring model not found")
	}

	// Validate model (checks weights count, scale, schema hash)
	if err := model.Validate(); err != nil {
		return AIEvalResult{}, fmt.Errorf("model validation failed: %w", err)
	}

	// Extract features from proposal
	fv := k.ExtractFeatureVector(ctx, proposal)

	// Validate features
	if err := fv.Validate(); err != nil {
		return AIEvalResult{}, fmt.Errorf("invalid features: %w", err)
	}

	// Run deterministic integer-only inference
	features := fv.GetFeatureVector()
	score, err := model.InferRiskScore(features)
	if err != nil {
		return AIEvalResult{}, fmt.Errorf("inference failed: %w", err)
	}

	// Convert score to tier
	tier := types.ScoreToTier(score)

	// Get params for delay/threshold computation
	params := k.GetParams(ctx)
	delayBlocks := types.TierToDelayBlocks(tier, params)
	thresholdBps := types.TierToThresholdBps(tier, params)

	return AIEvalResult{
		Score:        score,
		Tier:         tier,
		DelayBlocks:  delayBlocks,
		ThresholdBps: thresholdBps,
		ModelVersion: model.ModelVersion,
		SchemaHash:   model.FeatureSchemaHash,
		FeaturesHash: fv.ComputeFeaturesHash(),
	}, nil
}

// ExtractFeatureVector extracts canonical FeatureVectorV1 from a proposal.
func (k Keeper) ExtractFeatureVector(ctx context.Context, proposal govtypes.Proposal) types.FeatureVectorV1 {
	fv := types.FeatureVectorV1{}

	// Classify proposal type using Layer 1 classification
	proposalType := k.ClassifyProposalType(ctx, proposal)

	// Set one-hot encoding for proposal type
	switch proposalType {
	case ProposalTypeSoftwareUpgrade:
		fv.IsUpgrade = 1
	case ProposalTypeParamChange:
		fv.IsParamChange = 1
	case ProposalTypeTreasurySpend:
		fv.IsTreasurySpend = 1
	case ProposalTypeSlashingReduction:
		fv.IsSlashingChange = 1
	}
	// IsPocRuleChange and IsPoseqRuleChange require specific message type detection

	// Extract treasury spend BPS
	if proposalType == ProposalTypeTreasurySpend {
		spendPct := k.CalculateTreasurySpendPercentage(ctx, proposal)
		bps := int32(spendPct * 100) // % to BPS
		if bps > 10000 {
			bps = 10000
		}
		fv.TreasurySpendBps = bps
	}

	// Analyze messages for risk indicators
	modulesSet := make(map[string]bool)
	for _, msg := range proposal.Messages {
		msgType := msg.TypeUrl
		msgTypeLower := strings.ToLower(msgType)

		// Consensus-critical detection
		if isConsensusCriticalMsg(msgTypeLower) {
			fv.TouchesConsensusCritical = 1
		}

		// Slashing reduction detection
		if strings.Contains(msgTypeLower, "slashing") {
			fv.ReducesSlashing = 1
			fv.IsSlashingChange = 1
		}

		// Validator rule changes
		if strings.Contains(msgTypeLower, "staking") || strings.Contains(msgTypeLower, "validator") {
			fv.ChangesValidatorRules = 1
		}

		// PoC rule change detection
		if strings.Contains(msgTypeLower, "poc") {
			fv.IsPocRuleChange = 1
		}

		// PoSEQ rule change detection
		if strings.Contains(msgTypeLower, "poseq") || strings.Contains(msgTypeLower, "rewardmult") {
			fv.IsPoseqRuleChange = 1
		}

		// Extract module name for count
		parts := strings.Split(msgType, ".")
		if len(parts) >= 2 {
			modulesSet[parts[1]] = true
		}
	}

	// Count modules touched (clamped to 50)
	modulesCount := int32(len(modulesSet))
	if modulesCount > 50 {
		modulesCount = 50
	}
	fv.ModulesTouchedCount = modulesCount

	return fv
}

// MergeRulesAndAI merges Layer 1 (rules) and Layer 2 (AI) evaluations
// using the constitutional invariant: AI can only constrain, never relax.
//
// final_tier   = max(rules_tier, ai_tier)
// final_delay  = max(rules_delay, ai_delay)
// final_thresh = max(rules_thresh, ai_thresh)
// final_score  = max(rules_score, ai_score)
//
// Hard invariant: if final < rules for ANY parameter, log error and abort merge.
func (k Keeper) MergeRulesAndAI(
	rulesReport types.RiskReport,
	ai AIEvalResult,
	shadowMode bool,
) types.RiskReport {
	// Always populate AI fields on the report for observability
	merged := rulesReport
	merged.AiScore = ai.Score
	merged.AiTier = ai.Tier
	merged.AiDelayBlocks = ai.DelayBlocks
	merged.AiThresholdBps = ai.ThresholdBps
	merged.AiModelVersion = ai.ModelVersion
	merged.FeatureSchemaHash = ai.SchemaHash
	merged.AiFeaturesHash = ai.FeaturesHash

	if shadowMode {
		// Shadow mode: record AI results but do NOT alter rules-based enforcement
		k.logger.Info("AI evaluation (shadow mode)",
			"proposal_id", rulesReport.ProposalId,
			"rules_tier", rulesReport.Tier,
			"ai_tier", ai.Tier,
			"rules_score", rulesReport.Score,
			"ai_score", ai.Score)
		return merged
	}

	// Binding mode: merge using MaxConstraint
	finalTier := types.MaxConstraintTier(rulesReport.Tier, ai.Tier)
	finalDelay := types.MaxConstraint(rulesReport.ComputedDelayBlocks, ai.DelayBlocks)
	finalThreshold := types.MaxConstraint(rulesReport.ComputedThresholdBps, ai.ThresholdBps)
	finalScore := rulesReport.Score
	if ai.Score > rulesReport.Score {
		finalScore = ai.Score
	}

	// Hard invariant check: final must never be less than rules
	if finalTier < rulesReport.Tier || finalDelay < rulesReport.ComputedDelayBlocks ||
		finalThreshold < rulesReport.ComputedThresholdBps || finalScore < rulesReport.Score {
		// This should be mathematically impossible given max() operations above,
		// but we check explicitly as a safety net. Abort merge if violated.
		k.logger.Error("INVARIANT VIOLATION: AI merge would relax rules — aborting merge",
			"proposal_id", rulesReport.ProposalId,
			"rules_tier", rulesReport.Tier, "final_tier", finalTier,
			"rules_delay", rulesReport.ComputedDelayBlocks, "final_delay", finalDelay,
			"rules_threshold", rulesReport.ComputedThresholdBps, "final_threshold", finalThreshold)
		return merged // return with AI fields populated but rules enforcement unchanged
	}

	// Apply merged values
	merged.Tier = finalTier
	merged.Score = finalScore
	merged.ComputedDelayBlocks = finalDelay
	merged.ComputedThresholdBps = finalThreshold
	merged.ModelVersion = fmt.Sprintf("%s+%s", rulesReport.ModelVersion, ai.ModelVersion)

	k.logger.Info("Merged rules and AI evaluation",
		"proposal_id", rulesReport.ProposalId,
		"rules_tier", rulesReport.Tier, "ai_tier", ai.Tier, "final_tier", finalTier,
		"rules_delay", rulesReport.ComputedDelayBlocks, "ai_delay", ai.DelayBlocks, "final_delay", finalDelay,
		"rules_threshold", rulesReport.ComputedThresholdBps, "ai_threshold", ai.ThresholdBps, "final_threshold", finalThreshold)

	return merged
}

// isConsensusCriticalMsg checks if a message type affects consensus-critical parameters
func isConsensusCriticalMsg(msgTypeLower string) bool {
	consensusCritical := []string{
		"staking", "consensus", "baseapp", "validator", "evidence", "slashing",
	}
	for _, critical := range consensusCritical {
		if strings.Contains(msgTypeLower, critical) {
			return true
		}
	}
	return false
}
