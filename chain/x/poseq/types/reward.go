package types

// EpochRewardScore is the per-operator/node reward attribution for one epoch.
// Used as input to the reward distribution module (Phase 6).
//
// All scores are integer basis points (0–10000). No floating-point.
type EpochRewardScore struct {
	// NodeID is the 32-byte hex node identity.
	NodeID string `json:"node_id"`

	// OperatorAddress is the associated Cosmos operator address (if bonded).
	OperatorAddress string `json:"operator_address,omitempty"`

	// Epoch this score covers.
	Epoch uint64 `json:"epoch"`

	// BaseScoreBps: base participation score (proposals + attestations vs eligible).
	// 0–10000. Computed from performance records.
	BaseScoreBps uint32 `json:"base_score_bps"`

	// UptimeScoreBps: liveness score — epochs active / total epochs observed.
	// 0–10000.
	UptimeScoreBps uint32 `json:"uptime_score_bps"`

	// PoCMultiplierBps: Proof-of-Contribution multiplier from the slow lane.
	// 5000 = 0.5x, 10000 = 1.0x, 15000 = 1.5x.
	// Default 10000 (neutral) when no PoC data is available.
	PoCMultiplierBps uint32 `json:"poc_multiplier_bps"`

	// FaultPenaltyBps: penalty deducted for fault events. Subtracted from base.
	FaultPenaltyBps uint32 `json:"fault_penalty_bps"`

	// FinalScoreBps: final reward multiplier after all adjustments.
	// = clamp((BaseScoreBps + UptimeScoreBps) / 2 * PoCMultiplierBps / 10000 - FaultPenaltyBps, 0, 20000)
	FinalScoreBps uint32 `json:"final_score_bps"`

	// IsBonded indicates whether the operator had an active bond this epoch.
	IsBonded bool `json:"is_bonded"`
}

// ComputeRewardScore computes FinalScoreBps from the component fields.
// All arithmetic is integer-only, no floating-point.
//
// Formula:
//
//	combined = (BaseScoreBps + UptimeScoreBps) / 2
//	scaled   = combined * PoCMultiplierBps / 10000
//	final    = clamp(scaled - FaultPenaltyBps, 0, 20000)
func ComputeRewardScore(base, uptime, pocMult, faultPenalty uint32) uint32 {
	combined := (uint64(base) + uint64(uptime)) / 2
	scaled := combined * uint64(pocMult) / 10000
	if faultPenalty >= uint32(scaled) {
		return 0
	}
	final := uint32(scaled) - faultPenalty
	if final > 20000 {
		return 20000
	}
	return final
}

// PoCMultiplierRecord stores the PoC-derived multiplier for an operator at an epoch.
// Populated by governance or a future PoC integration hook.
type PoCMultiplierRecord struct {
	OperatorAddress string `json:"operator_address"`
	Epoch           uint64 `json:"epoch"`
	// MultiplierBps: 5000–15000 (0.5x–1.5x). Default 10000 (1.0x).
	MultiplierBps uint32 `json:"multiplier_bps"`
	// Source: "governance", "poc_hook", or "default"
	Source string `json:"source"`
}
