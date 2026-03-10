package types

// advisory_types.go — Layer 3 v2: Advisory Intelligence + Attack Memory
//
// NON-BINDING INVARIANT: Nothing in this file or the advisory keeper may
// affect tier, delay, threshold, or gate progression. These types exist
// solely for observability, historical correlation, and future AI dataset
// anchoring.
//
// Design constraints:
//   - All types are JSON-serialized (no proto regen required)
//   - Indexed for lightweight queries by tier, track, outcome
//   - Immutable once written (append-only audit trail)

// ============================================================================
// Advisory Schema Constants
// ============================================================================

const (
	// AdvisorySchemaVersion is the current advisory schema version.
	AdvisorySchemaVersion = "v2"

	// Advisory types
	AdvisoryTypeRiskAnalysis   = "RISK_ANALYSIS"
	AdvisoryTypeThreatReport   = "THREAT_REPORT"
	AdvisoryTypeAuditReport    = "AUDIT_REPORT"
	AdvisoryTypeCommunityNote  = "COMMUNITY_NOTE"
	AdvisoryTypeModelPrediction = "MODEL_PREDICTION"
)

// AllAdvisoryTypes returns all valid advisory type strings.
func AllAdvisoryTypes() []string {
	return []string{
		AdvisoryTypeRiskAnalysis,
		AdvisoryTypeThreatReport,
		AdvisoryTypeAuditReport,
		AdvisoryTypeCommunityNote,
		AdvisoryTypeModelPrediction,
	}
}

// IsValidAdvisoryType checks if a string is a known advisory type.
func IsValidAdvisoryType(t string) bool {
	for _, at := range AllAdvisoryTypes() {
		if t == at {
			return true
		}
	}
	return false
}

// ============================================================================
// Versioned Advisory Entry (extends the existing proto AdvisoryLink)
// ============================================================================

// AdvisoryEntryV2 is the versioned advisory schema. It extends the original
// AdvisoryLink with structured metadata for correlation and indexing.
//
// NON-BINDING: This type is NEVER read by EvaluateProposal, ProcessGateTransition,
// or any code path that affects tier/delay/threshold/gate.
type AdvisoryEntryV2 struct {
	// ── Identity ──
	ProposalID  uint64 `json:"proposal_id"`
	AdvisoryID  uint64 `json:"advisory_id"`  // auto-incremented per proposal
	Reporter    string `json:"reporter"`
	SubmittedAt int64  `json:"submitted_at"` // block height

	// ── Content reference ──
	URI        string `json:"uri"`         // IPFS CID or HTTP URI
	ReportHash string `json:"report_hash"` // SHA256 hex of report bytes

	// ── v2 structured fields ──
	AdvisoryType   string `json:"advisory_type"`   // one of AdvisoryType* constants
	ModelVersion   string `json:"model_version"`   // model that produced prediction (if any)
	RiskPrediction string `json:"risk_prediction"` // free-text tier prediction: "LOW"/"MED"/"HIGH"/"CRITICAL"
	SchemaVersion  string `json:"schema_version"`  // always AdvisorySchemaVersion

	// ── Snapshot context (frozen at submission time) ──
	RiskTierAtSubmission RiskTier `json:"risk_tier_at_submission"`
	TrackName            string   `json:"track_name"` // from x/timelock track classification
}

// ============================================================================
// Advisory → Risk Correlation
// ============================================================================

// AdvisoryCorrelation links an advisory to the final execution outcome.
// Written when a proposal reaches a terminal state (EXECUTED or ABORTED).
// Enables historical accuracy analysis of advisory predictions.
//
// NON-BINDING: This is a pure observability record.
type AdvisoryCorrelation struct {
	ProposalID       uint64   `json:"proposal_id"`
	AdvisoryCount    uint64   `json:"advisory_count"`    // how many advisories were submitted
	RiskTierAtFirst  RiskTier `json:"risk_tier_at_first"` // tier when first advisory was submitted
	FinalRiskTier    RiskTier `json:"final_risk_tier"`    // tier at execution/abort
	ExecutionOutcome string   `json:"execution_outcome"`  // "EXECUTED", "ABORTED", "EXPIRED"
	FinalHeight      int64    `json:"final_height"`       // block height of terminal transition
	EscalationCount  uint64   `json:"escalation_count"`   // number of tier escalations
	ExtensionCount   uint64   `json:"extension_count"`    // number of delay extensions
}

// ============================================================================
// Attack Memory Dataset
// ============================================================================

// AttackMemoryEntry is an immutable record stored when a proposal is cancelled,
// escalated multiple times, or extended multiple times. Forms the training
// dataset for future AI model improvements.
//
// Indexed by FeatureHash for deduplication and pattern matching.
//
// NON-BINDING: This type is NEVER read by any gate/tier/delay logic.
type AttackMemoryEntry struct {
	// ── Identity ──
	FeatureHash string `json:"feature_hash"` // SHA256 of proposal features (from RiskReport)
	ProposalID  uint64 `json:"proposal_id"`  // first proposal that triggered this entry
	RecordedAt  int64  `json:"recorded_at"`  // block height

	// ── Classification at time of incident ──
	ProposalType string   `json:"proposal_type"` // from ClassifyProposal
	InitialTier  RiskTier `json:"initial_tier"`
	FinalTier    RiskTier `json:"final_tier"`
	TrackName    string   `json:"track_name"`

	// ── Behavioral signals ──
	EscalationCount uint64 `json:"escalation_count"` // number of tier escalations
	ExtensionCount  uint64 `json:"extension_count"`  // number of delay extensions
	WasAborted      bool   `json:"was_aborted"`
	WasExecuted     bool   `json:"was_executed"`

	// ── Outcome label (for ML training) ──
	ExecutionOutcome string `json:"execution_outcome"` // "EXECUTED", "ABORTED", "EXPIRED"

	// ── Trigger reason ──
	TriggerReason string `json:"trigger_reason"` // why memory was recorded
}

// Attack memory trigger reasons
const (
	MemoryTriggerAborted           = "PROPOSAL_ABORTED"
	MemoryTriggerMultipleEscalation = "MULTIPLE_ESCALATION"
	MemoryTriggerMultipleExtension  = "MULTIPLE_EXTENSION"
	MemoryTriggerCriticalEscalation = "ESCALATED_TO_CRITICAL"

	// Thresholds for triggering memory recording
	EscalationCountMemoryThreshold = uint64(2) // 2+ escalations = memory entry
	ExtensionCountMemoryThreshold  = uint64(2) // 2+ extensions = memory entry
)

// ============================================================================
// Advisory Index Entry (lightweight secondary index)
// ============================================================================

// AdvisoryIndexEntry is a compact secondary index record for querying
// advisories by tier, track, or outcome.
type AdvisoryIndexEntry struct {
	ProposalID  uint64 `json:"proposal_id"`
	AdvisoryID  uint64 `json:"advisory_id"`
	Tier        RiskTier `json:"tier"`
	TrackName   string   `json:"track_name"`
	Outcome     string   `json:"outcome"` // populated on terminal state
	AdvisoryType string  `json:"advisory_type"`
}
