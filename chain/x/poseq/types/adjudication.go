package types

// AdjudicationPath determines how an evidence event is processed.
//
//   - Automatic: deterministic penalty applied immediately on ingest (or at epoch end).
//   - GovernanceReview: escalated for manual governance vote before any penalty.
type AdjudicationPath string

const (
	AdjudicationPathAutomatic        AdjudicationPath = "Automatic"
	AdjudicationPathGovernanceReview AdjudicationPath = "GovernanceReview"
)

// AdjudicationDecision is the outcome of adjudication.
//
//   - Pending: not yet decided.
//   - Penalized: automatic penalty applied (slash_bps deducted from bond). Only for Moderate.
//   - Escalated: sent to governance for manual review. For Severe and Critical.
//   - Dismissed: evidence informational or insufficient; no action. Minor severity routes here.
type AdjudicationDecision string

const (
	AdjudicationDecisionPending    AdjudicationDecision = "Pending"
	AdjudicationDecisionPenalized  AdjudicationDecision = "Penalized"
	AdjudicationDecisionEscalated  AdjudicationDecision = "Escalated"
	AdjudicationDecisionDismissed  AdjudicationDecision = "Dismissed"
)

// AdjudicationRecord tracks the adjudication outcome for one evidence packet.
//
// Keyed by evidence packet hash (0x13 prefix).
// One record per packet hash — write-once for Penalized/Dismissed; mutable for Pending→final.
type AdjudicationRecord struct {
	// PacketHash is the 32-byte evidence packet hash, hex-encoded.
	PacketHash string `json:"packet_hash"`

	// NodeID is the 32-byte hex node identity of the accused sequencer.
	NodeID string `json:"node_id"`

	// MisbehaviorType is the string type from the evidence packet (e.g. "DoubleProposal").
	MisbehaviorType string `json:"misbehavior_type"`

	// Epoch is the epoch the misbehavior occurred in.
	Epoch uint64 `json:"epoch"`

	// Path is Automatic or GovernanceReview.
	Path AdjudicationPath `json:"path"`

	// Decision is the current state. Starts at Pending.
	Decision AdjudicationDecision `json:"decision"`

	// SlashBps is the slash applied (0 if Dismissed or not yet decided).
	SlashBps uint32 `json:"slash_bps"`

	// DecidedAtEpoch is the epoch when a final decision was recorded. 0 = pending.
	DecidedAtEpoch uint64 `json:"decided_at_epoch,omitempty"`

	// Reason is a human-readable explanation of the decision.
	Reason string `json:"reason,omitempty"`

	// AutoApplied is true if the penalty was applied without governance review.
	AutoApplied bool `json:"auto_applied,omitempty"`
}

// SlashBpsForSeverity returns the default slash_bps for a given severity string.
// These are the deterministic defaults used in automatic adjudication.
//
//	Minor    → 0 bps (Informational — no slash)
//	Moderate → 300 bps (3%)
//	Severe   → 1000 bps (10%)
//	Critical → 2000 bps (20%)
func SlashBpsForSeverity(severity string) uint32 {
	switch severity {
	case "Minor":
		return 0
	case "Moderate":
		return 300
	case "Severe":
		return 1000
	case "Critical":
		return 2000
	default:
		return 0
	}
}

// IsAutoAdjudicable returns true for severities that are auto-penalized
// without governance review. Minor is NOT in this set — it is Informational
// (routed Automatic → Dismissed, but not penalized).
//
//   - Moderate → automatic (Penalized)
//   - Minor → Informational (Automatic → Dismissed, no slash, NOT auto-adjudicable)
//   - Severe and Critical → governance review required
//
// Mirrors Rust adjudication/engine.rs: is_auto_adjudicable returns true only for "Moderate".
func IsAutoAdjudicable(severity string) bool {
	switch severity {
	case "Moderate":
		return true
	default:
		return false
	}
}

// SequencerRankingProfile is the computed ranking for a sequencer at a given epoch.
//
// Rank score = AvailableBond (normalized) × PoC multiplier × performance score.
// Used to determine committee ordering and tier classification.
type SequencerRankingProfile struct {
	// NodeID is the 32-byte hex node identity.
	NodeID string `json:"node_id"`

	// OperatorAddress is the bech32 operator address.
	OperatorAddress string `json:"operator_address"`

	// Epoch this profile was computed for.
	Epoch uint64 `json:"epoch"`

	// AvailableBond is the operator's current available bond.
	AvailableBond uint64 `json:"available_bond"`

	// PoCMultiplierBps is the PoC score multiplier (5000–15000).
	PoCMultiplierBps uint32 `json:"poc_multiplier_bps"`

	// ParticipationRateBps from the last performance record (0–10000).
	ParticipationRateBps uint32 `json:"participation_rate_bps"`

	// FaultEventsRecent is the total fault events in the trailing MaxFaultHistoryEpochs.
	FaultEventsRecent uint64 `json:"fault_events_recent"`

	// RankScore is the composite score for ordering (higher = better).
	// Computed as: participation_rate_bps * poc_multiplier_bps / 10000
	// (bond is used as a gate, not a continuous rank factor)
	RankScore uint32 `json:"rank_score"`

	// Tier classifies the sequencer into a market tier.
	Tier SequencerTier `json:"tier"`
}

// SequencerTier classifies sequencer quality.
//
//   - Elite: bonded, poc_mult > 11000, participation > 9000, faults == 0
//   - Established: bonded, poc_mult >= 9000, participation >= 7000, faults <= 2
//   - Standard: bonded, meets min thresholds, no critical faults
//   - Probationary: recently activated or recovering; limited history
//   - Underperforming: participation < min_participation_bps or recent faults
type SequencerTier string

const (
	SequencerTierElite          SequencerTier = "Elite"
	SequencerTierEstablished    SequencerTier = "Established"
	SequencerTierStandard       SequencerTier = "Standard"
	SequencerTierProbationary   SequencerTier = "Probationary"
	SequencerTierUnderperforming SequencerTier = "Underperforming"
)

// ClassifyTier returns the SequencerTier for the given ranking parameters.
// All thresholds are integers — no floating point.
//
// Parameters:
//   - isBonded: whether operator has an active bond
//   - pocMultBps: PoC multiplier in basis points
//   - participationBps: participation rate in basis points
//   - faultEventsRecent: fault events in trailing MaxFaultHistoryEpochs
//   - epochsSinceActivation: epochs since the node became Active (0 = just activated)
func ClassifyTier(isBonded bool, pocMultBps, participationBps uint32, faultEventsRecent uint64, epochsSinceActivation uint64) SequencerTier {
	if !isBonded {
		return SequencerTierUnderperforming
	}
	if epochsSinceActivation < 3 {
		return SequencerTierProbationary
	}
	if participationBps < 5000 || faultEventsRecent > 5 {
		return SequencerTierUnderperforming
	}
	if pocMultBps > 11000 && participationBps > 9000 && faultEventsRecent == 0 {
		return SequencerTierElite
	}
	if pocMultBps >= 9000 && participationBps >= 7000 && faultEventsRecent <= 2 {
		return SequencerTierEstablished
	}
	return SequencerTierStandard
}

// EpochSettlementRecord captures the net reward computation for one node in one epoch.
//
// All fields are integer basis points or absolute amounts.
// No floating-point arithmetic.
type EpochSettlementRecord struct {
	// NodeID is the 32-byte hex node identity.
	NodeID string `json:"node_id"`

	// OperatorAddress is the bech32 operator address. Empty if unbonded.
	OperatorAddress string `json:"operator_address,omitempty"`

	// Epoch this settlement covers.
	Epoch uint64 `json:"epoch"`

	// GrossRewardScore is the raw final_score_bps before deductions.
	GrossRewardScore uint32 `json:"gross_reward_score_bps"`

	// PoCMultiplierBps applied in this epoch.
	PoCMultiplierBps uint32 `json:"poc_multiplier_bps"`

	// FaultPenaltyBps applied from fault events.
	FaultPenaltyBps uint32 `json:"fault_penalty_bps"`

	// SlashPenaltyBps from any slash executed this epoch.
	// Expressed as bps of the GrossRewardScore.
	SlashPenaltyBps uint32 `json:"slash_penalty_bps"`

	// NetRewardScore after all deductions. Clamped to [0, 20000].
	NetRewardScore uint32 `json:"net_reward_score_bps"`

	// IsBonded indicates whether the operator had an active bond this epoch.
	IsBonded bool `json:"is_bonded"`

	// BondMultiplierApplied is true if a bond-based multiplier was used.
	BondMultiplierApplied bool `json:"bond_multiplier_applied"`

	// SlashExecutedThisEpoch is the number of slashes executed this epoch.
	SlashExecutedThisEpoch uint32 `json:"slash_executed_this_epoch,omitempty"`
}
