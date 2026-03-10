package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"pos/x/guard/types"
)

// ProposalType represents categorized proposal types
type ProposalType string

const (
	ProposalTypeSoftwareUpgrade    ProposalType = "SOFTWARE_UPGRADE"
	ProposalTypeConsensusCritical  ProposalType = "CONSENSUS_CRITICAL"
	ProposalTypeTreasurySpend      ProposalType = "TREASURY_SPEND"
	ProposalTypeSlashingReduction  ProposalType = "SLASHING_REDUCTION"
	ProposalTypeParamChange        ProposalType = "PARAM_CHANGE"
	ProposalTypeTextOnly           ProposalType = "TEXT_ONLY"
	ProposalTypeOther              ProposalType = "OTHER"
)

// EvaluateProposal performs deterministic risk evaluation on a governance proposal
// Returns a RiskReport with tier, score, computed delay, and required threshold
func (k Keeper) EvaluateProposal(ctx context.Context, proposal govtypes.Proposal) (types.RiskReport, error) {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Step 1: Classify proposal type and extract features
	proposalType, features := k.ClassifyProposal(ctx, proposal)
	reasonCodes := []string{}

	// Step 2: Determine base risk tier
	tier := types.RISK_TIER_LOW
	score := uint32(10) // base score

	switch proposalType {
	case ProposalTypeSoftwareUpgrade:
		tier = types.RISK_TIER_CRITICAL
		score = 95
		reasonCodes = append(reasonCodes, "SOFTWARE_UPGRADE")

	case ProposalTypeConsensusCritical:
		tier = types.RISK_TIER_CRITICAL
		score = 90
		reasonCodes = append(reasonCodes, "CONSENSUS_CRITICAL")

	case ProposalTypeSlashingReduction:
		tier = types.RISK_TIER_HIGH
		score = 80
		reasonCodes = append(reasonCodes, "SLASHING_REDUCTION")

	case ProposalTypeTreasurySpend:
		// Assess spend amount as bps of treasury (100 bps = 1%)
		spendBps := k.CalculateTreasurySpendPercentage(ctx, proposal)
		if spendBps >= 2500 { // 25%
			tier = types.RISK_TIER_CRITICAL
			score = 85
			reasonCodes = append(reasonCodes, "TREASURY_SPEND_CRITICAL")
		} else if spendBps >= 1000 { // 10%
			tier = types.RISK_TIER_HIGH
			score = 70
			reasonCodes = append(reasonCodes, "TREASURY_SPEND_HIGH")
		} else if spendBps >= 500 { // 5%
			tier = types.RISK_TIER_MED
			score = 50
			reasonCodes = append(reasonCodes, "TREASURY_SPEND_MED")
		} else {
			tier = types.RISK_TIER_LOW
			score = 30
			reasonCodes = append(reasonCodes, "TREASURY_SPEND_LOW")
		}

	case ProposalTypeParamChange:
		// Check if param changes touch consensus-critical modules
		if features["touches_consensus"] == "true" {
			tier = types.RISK_TIER_CRITICAL
			score = 85
			reasonCodes = append(reasonCodes, "PARAM_CHANGE_CONSENSUS")
		} else if features["touches_economic"] == "true" {
			tier = types.RISK_TIER_HIGH
			score = 65
			reasonCodes = append(reasonCodes, "PARAM_CHANGE_ECONOMIC")
		} else {
			tier = types.RISK_TIER_MED
			score = 40
			reasonCodes = append(reasonCodes, "PARAM_CHANGE")
		}

	case ProposalTypeTextOnly:
		tier = types.RISK_TIER_LOW
		score = 5
		reasonCodes = append(reasonCodes, "TEXT_ONLY")

	default:
		tier = types.RISK_TIER_MED
		score = 30
		reasonCodes = append(reasonCodes, "OTHER")
	}

	// Step 3: Compute delay based on tier
	var computedDelay uint64
	switch tier {
	case types.RISK_TIER_LOW:
		computedDelay = params.DelayLowBlocks
	case types.RISK_TIER_MED:
		computedDelay = params.DelayMedBlocks
	case types.RISK_TIER_HIGH:
		computedDelay = params.DelayHighBlocks
	case types.RISK_TIER_CRITICAL:
		computedDelay = params.DelayCriticalBlocks
	}

	// Apply multipliers based on features
	if features["high_economic_impact"] == "true" {
		computedDelay = computedDelay * 12 / 10 // 1.2x multiplier
		score = min(score+10, 100)
		reasonCodes = append(reasonCodes, "HIGH_ECONOMIC_IMPACT")
	}

	// Step 4: Determine required threshold
	var requiredThreshold uint64
	switch tier {
	case types.RISK_TIER_LOW, types.RISK_TIER_MED:
		requiredThreshold = params.ThresholdDefaultBps
	case types.RISK_TIER_HIGH:
		requiredThreshold = params.ThresholdHighBps
	case types.RISK_TIER_CRITICAL:
		requiredThreshold = params.ThresholdCriticalBps
	}

	// Step 5: Compute features hash for determinism verification
	featuresHash := k.ComputeFeaturesHash(proposal.Id, proposalType, features)

	// Step 6: Serialize reason codes
	reasonCodesJSON, _ := json.Marshal(reasonCodes)

	// Step 7: Create risk report from Layer 1 (rules)
	report := types.RiskReport{
		ProposalId:           proposal.Id,
		Tier:                 tier,
		Score:                score,
		ComputedDelayBlocks:  computedDelay,
		ComputedThresholdBps: requiredThreshold,
		ReasonCodes:          string(reasonCodesJSON),
		FeaturesHash:         featuresHash,
		ModelVersion:         "rules-v1",
		CreatedAt:            sdkCtx.BlockTime(),
	}

	// Step 8: Layer 2 AI evaluation (if enabled)
	aiParams := k.GetParams(ctx)
	if aiParams.BindingAiEnabled || aiParams.AiShadowMode {
		aiResult, err := k.EvaluateProposalAI(ctx, proposal)
		if err != nil {
			// Log error but don't fail - continue with rules-only evaluation
			k.logger.Error("AI evaluation failed, using rules-only",
				"proposal_id", proposal.Id,
				"error", err)
		} else {
			// Determine mode: shadow if AiShadowMode is true OR BindingAiEnabled is false
			shadowMode := aiParams.AiShadowMode || !aiParams.BindingAiEnabled

			// Merge AI with rules using MaxConstraint principle
			report = k.MergeRulesAndAI(report, aiResult, shadowMode)

			k.logger.Info("AI evaluation completed",
				"proposal_id", proposal.Id,
				"ai_tier", aiResult.Tier,
				"ai_score", aiResult.Score,
				"shadow_mode", shadowMode)
		}
	}

	return report, nil
}

// ClassifyProposal categorizes a proposal and extracts features
func (k Keeper) ClassifyProposal(ctx context.Context, proposal govtypes.Proposal) (ProposalType, map[string]string) {
	features := make(map[string]string)

	// Check message types
	for _, msg := range proposal.Messages {
		msgType := msg.TypeUrl

		// Software upgrade
		if strings.Contains(msgType, "MsgSoftwareUpgrade") || strings.Contains(msgType, "/upgrade.") {
			return ProposalTypeSoftwareUpgrade, features
		}

		// Consensus-critical params
		if strings.Contains(msgType, "consensus") || strings.Contains(msgType, "staking") {
			features["touches_consensus"] = "true"
			return ProposalTypeConsensusCritical, features
		}

		// Slashing param changes (reduction in slash fractions is critical)
		if strings.Contains(msgType, "slashing") && strings.Contains(msgType, "MsgUpdateParams") {
			return ProposalTypeSlashingReduction, features
		}

		// Treasury spend (community pool spend, bank sends from gov module)
		if strings.Contains(msgType, "MsgCommunityPoolSpend") || strings.Contains(msgType, "distribution") {
			features["treasury_spend"] = "true"
			return ProposalTypeTreasurySpend, features
		}

		if strings.Contains(msgType, "bank") && strings.Contains(msgType, "MsgSend") {
			features["treasury_spend"] = "true"
			return ProposalTypeTreasurySpend, features
		}

		// Param changes
		if strings.Contains(msgType, "MsgUpdateParams") || strings.Contains(msgType, "ParameterChangeProposal") {
			// Check which module's params are being changed
			if strings.Contains(msgType, "staking") || strings.Contains(msgType, "slashing") || strings.Contains(msgType, "consensus") {
				features["touches_consensus"] = "true"
			}
			if strings.Contains(msgType, "mint") || strings.Contains(msgType, "distribution") || strings.Contains(msgType, "bank") {
				features["touches_economic"] = "true"
			}
			return ProposalTypeParamChange, features
		}
	}

	// No messages = text-only proposal
	if len(proposal.Messages) == 0 {
		return ProposalTypeTextOnly, features
	}

	return ProposalTypeOther, features
}

// NOTE: CalculateTreasurySpendPercentage is implemented in queue.go with real
// community pool balance queries via DistrKeeper.

// ComputeFeaturesHash creates a deterministic hash of proposal features.
// Keys are sorted lexicographically to ensure identical output across all validators.
func (k Keeper) ComputeFeaturesHash(proposalID uint64, proposalType ProposalType, features map[string]string) string {
	// Create canonical feature string
	canonicalFeatures := fmt.Sprintf("proposal_id=%d|type=%s", proposalID, proposalType)

	// Sort keys for deterministic iteration
	keys := make([]string, 0, len(features))
	for key := range features {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		canonicalFeatures += fmt.Sprintf("|%s=%s", key, features[key])
	}

	// Compute SHA256
	hash := sha256.Sum256([]byte(canonicalFeatures))
	return fmt.Sprintf("%x", hash)
}

// min returns the minimum of two uint32 values
func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// ClassifyProposalType returns just the proposal type (used by AI evaluation)
func (k Keeper) ClassifyProposalType(ctx context.Context, proposal govtypes.Proposal) ProposalType {
	proposalType, _ := k.ClassifyProposal(ctx, proposal)
	return proposalType
}
