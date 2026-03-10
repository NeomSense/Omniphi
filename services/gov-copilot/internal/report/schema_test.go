package report_test

import (
	"encoding/json"
	"testing"

	"gov-copilot/internal/chain"
	"gov-copilot/internal/report"
)

func TestReportJSONRoundtrip(t *testing.T) {
	r := &report.Report{
		ProposalID: 42,
		ChainID:    "omniphi-1",
		CreatedAt:  "2026-02-17T00:00:00Z",
		Reporter:   "gov-copilot-v1",
		AIProvider: "deepseek",
		Risk: report.RiskSection{
			TierRules:   "RISK_TIER_HIGH",
			TierAI:      "RISK_TIER_MED",
			TierFinal:   "RISK_TIER_HIGH",
			TreasuryBps: 500,
			ChurnBps:    100,
		},
		Timeline: report.TimelineSection{
			CurrentGate:        "EXECUTION_GATE_CONDITIONAL_EXECUTION",
			EarliestExecHeight: 120960,
			Notes:              "stability check passed",
		},
		Summary:                  "Proposal 42: parameter change with HIGH risk",
		KeyChanges:               []string{"Reduce slash fraction from 5% to 1%"},
		WhatCouldGoWrong:         []string{"Validators may misbehave with lower penalties"},
		RecommendedSafetyActions: []string{"Model impact on validator behavior"},
	}

	// Marshal
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Unmarshal back
	var decoded report.Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify fields
	if decoded.ProposalID != 42 {
		t.Errorf("proposal_id: got %d, want 42", decoded.ProposalID)
	}
	if decoded.ChainID != "omniphi-1" {
		t.Errorf("chain_id: got %q, want %q", decoded.ChainID, "omniphi-1")
	}
	if decoded.Risk.TierFinal != "RISK_TIER_HIGH" {
		t.Errorf("tier_final: got %q, want %q", decoded.Risk.TierFinal, "RISK_TIER_HIGH")
	}
	if decoded.Timeline.EarliestExecHeight != 120960 {
		t.Errorf("earliest_exec_height: got %d, want 120960", decoded.Timeline.EarliestExecHeight)
	}
	if len(decoded.KeyChanges) != 1 {
		t.Errorf("key_changes length: got %d, want 1", len(decoded.KeyChanges))
	}
	if len(decoded.WhatCouldGoWrong) != 1 {
		t.Errorf("what_could_go_wrong length: got %d, want 1", len(decoded.WhatCouldGoWrong))
	}
	if len(decoded.RecommendedSafetyActions) != 1 {
		t.Errorf("recommended_safety_actions length: got %d, want 1", len(decoded.RecommendedSafetyActions))
	}
}

func TestReportJSONStableKeys(t *testing.T) {
	r := &report.Report{
		ProposalID: 1,
		ChainID:    "test-1",
		CreatedAt:  "2026-01-01T00:00:00Z",
		Reporter:   "test",
		AIProvider: "template",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Ensure expected keys are present
	s := string(data)
	requiredKeys := []string{
		`"proposal_id"`, `"chain_id"`, `"created_at"`, `"reporter"`,
		`"ai_provider"`, `"risk"`, `"timeline"`, `"summary"`,
		`"key_changes"`, `"what_could_go_wrong"`, `"recommended_safety_actions"`,
	}
	for _, key := range requiredKeys {
		if !contains(s, key) {
			t.Errorf("missing key %s in JSON output", key)
		}
	}
}

func TestGenerateTemplate_Critical(t *testing.T) {
	proposal := &chain.Proposal{
		ID:           10,
		Title:        "Software Upgrade v2",
		MessageTypes: []string{"/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade"},
	}
	rr := &chain.RiskReport{
		Tier:  "RISK_TIER_CRITICAL",
		Score: 85,
	}
	qe := &chain.QueuedExecution{
		GateState:          "EXECUTION_GATE_VISIBILITY",
		EarliestExecHeight: 200000,
	}

	r := report.GenerateTemplate(proposal, rr, qe, "omniphi-1", "test-reporter")

	if r.ProposalID != 10 {
		t.Errorf("proposal_id: got %d, want 10", r.ProposalID)
	}
	if r.AIProvider != "template" {
		t.Errorf("ai_provider: got %q, want %q", r.AIProvider, "template")
	}
	if len(r.WhatCouldGoWrong) == 0 {
		t.Error("expected non-empty what_could_go_wrong for CRITICAL tier")
	}
	if len(r.RecommendedSafetyActions) == 0 {
		t.Error("expected non-empty recommended_safety_actions for CRITICAL tier")
	}

	// Should mention upgrade-related actions
	found := false
	for _, a := range r.RecommendedSafetyActions {
		if contains(a, "binary") || contains(a, "testnet") || contains(a, "upgrade") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected upgrade-related safety action for CRITICAL software upgrade")
	}
}

func TestGenerateTemplate_Low(t *testing.T) {
	proposal := &chain.Proposal{
		ID:    1,
		Title: "Simple text proposal",
	}

	r := report.GenerateTemplate(proposal, nil, nil, "test-1", "test")

	if r.Risk.TierFinal != "" {
		// No risk report means tier is UNKNOWN
	}
	if len(r.WhatCouldGoWrong) == 0 {
		t.Error("expected non-empty what_could_go_wrong even for LOW tier")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
