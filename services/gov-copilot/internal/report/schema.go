// Package report defines the stable advisory report schema and template generator.
package report

import (
	"fmt"
	"strings"
	"time"

	"gov-copilot/internal/chain"
)

// Report is the structured advisory output. All fields are stable and must
// not be renamed — the JSON is hashed on-chain.
type Report struct {
	ProposalID  uint64 `json:"proposal_id"`
	ChainID     string `json:"chain_id"`
	CreatedAt   string `json:"created_at"` // RFC3339
	Reporter    string `json:"reporter"`
	AIProvider  string `json:"ai_provider"` // "deepseek" or "template"

	Risk     RiskSection     `json:"risk"`
	Timeline TimelineSection `json:"timeline"`

	Summary                  string   `json:"summary"`
	KeyChanges               []string `json:"key_changes"`
	WhatCouldGoWrong         []string `json:"what_could_go_wrong"`
	RecommendedSafetyActions []string `json:"recommended_safety_actions"`
}

// RiskSection contains guard module risk evaluation data.
type RiskSection struct {
	TierRules   string `json:"tier_rules"`
	TierAI      string `json:"tier_ai"`
	TierFinal   string `json:"tier_final"`
	TreasuryBps uint64 `json:"treasury_bps"`
	ChurnBps    uint64 `json:"churn_bps"`
}

// TimelineSection contains execution gate status.
type TimelineSection struct {
	CurrentGate        string `json:"current_gate"`
	EarliestExecHeight uint64 `json:"earliest_exec_height"`
	Notes              string `json:"notes"`
}

// NewReport creates a Report with metadata fields populated.
func NewReport(proposalID uint64, chainID, reporter, aiProvider string) *Report {
	return &Report{
		ProposalID: proposalID,
		ChainID:    chainID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Reporter:   reporter,
		AIProvider: aiProvider,
	}
}

// ---------- Template fallback generator ----------

// GenerateTemplate produces a deterministic report from guard data alone,
// without calling any AI provider.
func GenerateTemplate(
	proposal *chain.Proposal,
	riskReport *chain.RiskReport,
	queuedExec *chain.QueuedExecution,
	chainID, reporter string,
) *Report {
	r := NewReport(proposal.ID, chainID, reporter, "template")

	// Populate risk section
	if riskReport != nil {
		r.Risk = RiskSection{
			TierRules: riskReport.Tier,
			TierAI:    riskReport.AITier,
			TierFinal: riskReport.Tier, // rules tier is authoritative
		}
	}

	// Populate timeline section
	if queuedExec != nil {
		r.Timeline = TimelineSection{
			CurrentGate:        queuedExec.GateState,
			EarliestExecHeight: queuedExec.EarliestExecHeight,
		}
	}

	// Build summary
	tier := "UNKNOWN"
	if riskReport != nil {
		tier = riskReport.Tier
	}
	r.Summary = fmt.Sprintf(
		"Proposal %d (%s): Guard tier_final=%s. Template-generated advisory.",
		proposal.ID, proposal.Title, tier,
	)

	// Key changes from message types
	r.KeyChanges = describeMessages(proposal.MessageTypes)

	// What could go wrong (tier-based)
	r.WhatCouldGoWrong = risksByTier(tier, proposal.MessageTypes)

	// Recommended safety actions
	r.RecommendedSafetyActions = safetyActions(tier, proposal.MessageTypes, queuedExec)

	return r
}

func describeMessages(msgTypes []string) []string {
	if len(msgTypes) == 0 {
		return []string{"Text-only proposal (no executable messages)"}
	}
	out := make([]string, 0, len(msgTypes))
	for _, mt := range msgTypes {
		out = append(out, fmt.Sprintf("Message: %s", mt))
	}
	return out
}

func risksByTier(tier string, msgTypes []string) []string {
	risks := []string{}

	switch strings.ToUpper(tier) {
	case "RISK_TIER_CRITICAL", "CRITICAL":
		for _, mt := range msgTypes {
			lower := strings.ToLower(mt)
			if strings.Contains(lower, "upgrade") {
				risks = append(risks, "Software upgrade: validators must coordinate binary switch; failed upgrade halts chain")
			}
			if strings.Contains(lower, "consensus") {
				risks = append(risks, "Consensus-critical parameter change: incorrect values can fork or halt the network")
			}
		}
		risks = append(risks, "CRITICAL tier: maximum delay and threshold enforced by guard module")

	case "RISK_TIER_HIGH", "HIGH":
		for _, mt := range msgTypes {
			lower := strings.ToLower(mt)
			if strings.Contains(lower, "communitypoolspend") || strings.Contains(lower, "treasury") {
				risks = append(risks, "Treasury spend: verify recipient address and amount are correct")
			}
			if strings.Contains(lower, "slashing") {
				risks = append(risks, "Slashing parameter change: reducing penalties weakens validator accountability")
			}
		}
		risks = append(risks, "HIGH tier: extended delay period required before execution")

	case "RISK_TIER_MED", "MED":
		risks = append(risks, "Parameter change: verify new values are within safe operational bounds")
		risks = append(risks, "MED tier: standard delay period applies")

	default:
		risks = append(risks, "LOW tier: minimal risk identified by guard module")
	}

	return risks
}

func safetyActions(tier string, msgTypes []string, qe *chain.QueuedExecution) []string {
	actions := []string{}

	switch strings.ToUpper(tier) {
	case "RISK_TIER_CRITICAL", "CRITICAL":
		actions = append(actions, "Deploy to testnet and validate before mainnet execution")
		for _, mt := range msgTypes {
			if strings.Contains(strings.ToLower(mt), "upgrade") {
				actions = append(actions, "Verify upgrade binary hash matches the proposal")
				actions = append(actions, "Ensure all validators have the new binary ready before execution height")
			}
		}
		actions = append(actions, "Require second confirmation via MsgConfirmExecution")

	case "RISK_TIER_HIGH", "HIGH":
		for _, mt := range msgTypes {
			lower := strings.ToLower(mt)
			if strings.Contains(lower, "communitypoolspend") || strings.Contains(lower, "treasury") {
				actions = append(actions, "Validate spend recipient address and amount")
				actions = append(actions, "Check treasury balance can sustain the spend")
			}
			if strings.Contains(lower, "slashing") {
				actions = append(actions, "Model impact of reduced slashing on validator behavior")
			}
		}
		actions = append(actions, "Wait for full delay period before voting")

	case "RISK_TIER_MED", "MED":
		actions = append(actions, "Review parameter changes against current mainnet values")
		actions = append(actions, "Check for unintended side effects on dependent modules")

	default:
		actions = append(actions, "Standard review process sufficient")
	}

	if qe != nil && qe.EarliestExecHeight > 0 {
		actions = append(actions, fmt.Sprintf("Respect guard delay: earliest execution at height %d", qe.EarliestExecHeight))
	}

	return actions
}
