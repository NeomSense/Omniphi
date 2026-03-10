package types

// hardening_types.go — Dynamic Deterministic Guard v2 types
//
// These types extend the guard module with continuous risk reevaluation,
// tier escalation, threshold escalation, cross-proposal risk coupling,
// and global emergency hardening mode.
//
// Design constraints:
//   - No floats — all arithmetic uses uint64 with basis-point precision
//   - Monotonic invariant: tier and threshold may only increase, never decrease
//   - AI can never shorten delay or lower threshold (MergeRulesAndAI still uses max())
//   - Deterministic: identical inputs produce identical outputs across all validators

// ============================================================================
// Reevaluation Record
// ============================================================================

// ReevaluationRecord stores the result of the latest risk reevaluation for a
// queued proposal. Each reevaluation is appended as an immutable audit entry.
type ReevaluationRecord struct {
	ProposalID         uint64   `json:"proposal_id"`
	ReevaluatedAtHeight int64  `json:"reevaluated_at_height"`
	PreviousTier       RiskTier `json:"previous_tier"`
	NewTier            RiskTier `json:"new_tier"`
	PreviousThreshold  uint64   `json:"previous_threshold_bps"`
	NewThreshold       uint64   `json:"new_threshold_bps"`
	PreviousDelay      uint64   `json:"previous_delay_blocks"`
	NewDelay           uint64   `json:"new_delay_blocks"`
	EscalationReasons  []string `json:"escalation_reasons"`
}

// ============================================================================
// Aggregate Risk Window
// ============================================================================

// AggregateRiskSnapshot captures the rolling-window state of all active
// (non-terminal) proposals in the guard queue.
type AggregateRiskSnapshot struct {
	// WindowBlocks is the lookback window size in blocks.
	WindowBlocks uint64 `json:"window_blocks"`

	// ActiveProposalCount is the number of non-terminal proposals in the window.
	ActiveProposalCount uint64 `json:"active_proposal_count"`

	// TreasurySpendsActive counts proposals classified as TREASURY_SPEND.
	TreasurySpendsActive uint64 `json:"treasury_spends_active"`

	// ParamChangesActive counts proposals classified as PARAM_CHANGE or CONSENSUS_CRITICAL.
	ParamChangesActive uint64 `json:"param_changes_active"`

	// UpgradesActive counts proposals classified as SOFTWARE_UPGRADE.
	UpgradesActive uint64 `json:"upgrades_active"`

	// CumulativeTreasuryBps is the sum of treasury spend BPS across active proposals.
	CumulativeTreasuryBps uint64 `json:"cumulative_treasury_bps"`

	// HighestTier is the max tier among all active proposals.
	HighestTier RiskTier `json:"highest_tier"`
}

// ============================================================================
// Threshold Escalation Record
// ============================================================================

// ThresholdEscalationRecord tracks dynamic threshold adjustments for a proposal.
type ThresholdEscalationRecord struct {
	ProposalID        uint64 `json:"proposal_id"`
	OriginalThreshold uint64 `json:"original_threshold_bps"`
	CurrentThreshold  uint64 `json:"current_threshold_bps"`
	EscalationCount   uint64 `json:"escalation_count"`
	LastEscalatedAt   int64  `json:"last_escalated_at_height"`
}

// ============================================================================
// DDG v2 Constants
// ============================================================================

const (
	// ── Escalation thresholds ──

	// ValidatorChurnSoftThresholdBps is the churn level (bps) above which
	// tier escalation kicks in. Below MaxValidatorChurnBps (hard).
	ValidatorChurnSoftThresholdBps uint64 = 1000 // 10%

	// ValidatorChurnHardThresholdBps is the churn level that triggers
	// threshold escalation (supermajority requirement). Equal to params.MaxValidatorChurnBps.
	// This is read from params at runtime; this constant is a fallback.
	ValidatorChurnHardThresholdBps uint64 = 2000 // 20%

	// TreasuryCumulativeSoftThresholdBps is the cumulative treasury outflow (bps)
	// across active proposals that triggers tier escalation.
	TreasuryCumulativeSoftThresholdBps uint64 = 1500 // 15%

	// MultiParamChangeThreshold is the number of concurrent PARAM_CHANGE/CONSENSUS
	// proposals that triggers escalation.
	MultiParamChangeThreshold uint64 = 3

	// ── Instability detection ──

	// InstabilityWindowBlocks is how many blocks of continuous instability
	// (churn > soft threshold) before threshold escalation activates.
	InstabilityWindowBlocks uint64 = 8640 // ~12 hours at 5s blocks

	// ThresholdEscalationStepBps is how much the threshold increases per escalation.
	ThresholdEscalationStepBps uint64 = 500 // +5% per escalation

	// SupermajorityThresholdBps is the maximum threshold achievable via escalation.
	SupermajorityThresholdBps uint64 = 9000 // 90%

	// ── Aggregate risk window ──

	// AggregateRiskWindowBlocks is the lookback window for cross-proposal coupling.
	AggregateRiskWindowBlocks uint64 = 17280 // ~1 day

	// TreasuryStackingThresholdBps is the cumulative treasury BPS that triggers
	// new proposals to be escalated.
	TreasuryStackingThresholdBps uint64 = 2000 // 20%

	// ParamMutationBurstThreshold is the number of active param-change proposals
	// that qualifies as a "burst" for aggregate risk.
	ParamMutationBurstThreshold uint64 = 3

	// UpgradeClusteringThreshold is the number of active upgrade proposals
	// that qualifies as "clustering" for aggregate risk.
	UpgradeClusteringThreshold uint64 = 2

	// ── Emergency hardening mode ──

	// HardeningDelayMultiplierNum / Den = 1.5x (3/2)
	HardeningDelayMultiplierNum uint64 = 3
	HardeningDelayMultiplierDen uint64 = 2
)
