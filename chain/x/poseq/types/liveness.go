package types

// ─── LivenessEvent ────────────────────────────────────────────────────────────

// LivenessEvent records that a PoSeq node was observed active in an epoch.
// Submitted by the authorized relayer as part of ExportBatch.LivenessEvents.
type LivenessEvent struct {
	// NodeID is the 32-byte node identity.
	NodeID []byte `json:"node_id"`
	// Epoch is the PoSeq epoch in which the node was observed.
	Epoch uint64 `json:"epoch"`
	// LastSeenSlot is the most recent HotStuff slot in which the node participated.
	LastSeenSlot uint64 `json:"last_seen_slot"`
	// WasProposer is true if the node proposed at least one batch this epoch.
	WasProposer bool `json:"was_proposer"`
	// WasAttestor is true if the node cast at least one attestation this epoch.
	WasAttestor bool `json:"was_attestor"`
}

// ─── NodePerformanceRecord ────────────────────────────────────────────────────

// NodePerformanceRecord stores per-node per-epoch attribution data.
// Submitted once per epoch per node by the authorized relayer.
// All rates are expressed as basis points (0–10000) — no floating-point.
type NodePerformanceRecord struct {
	// NodeID is the 32-byte node identity.
	NodeID []byte `json:"node_id"`
	// Epoch is the PoSeq epoch this record covers.
	Epoch uint64 `json:"epoch"`
	// ProposalsCount is the number of batch proposals made by this node.
	ProposalsCount uint64 `json:"proposals_count"`
	// AttestationsCount is the number of attestations cast by this node.
	AttestationsCount uint64 `json:"attestations_count"`
	// MissedAttestations is the number of attestation opportunities missed.
	MissedAttestations uint64 `json:"missed_attestations"`
	// FaultEvents is the number of misbehavior events attributed to this node.
	FaultEvents uint64 `json:"fault_events"`
	// ParticipationRateBps is the participation rate in basis points (0–10000).
	// Computed as: (attestations_count * 10000) / (attestations_count + missed_attestations).
	// Integer arithmetic only — never floating-point.
	ParticipationRateBps uint32 `json:"participation_rate_bps"`
}

// ─── InactivityEvent ──────────────────────────────────────────────────────────

// InactivityEvent records that a PoSeq node missed one or more consecutive epochs.
// Submitted by the authorized relayer as part of ExportBatch.InactivityEvents.
// Used by the auto-enforcement step to trigger suspension.
type InactivityEvent struct {
	// NodeID is the 32-byte node identity.
	NodeID []byte `json:"node_id"`
	// Epoch is the current PoSeq epoch.
	Epoch uint64 `json:"epoch"`
	// MissedEpochs is the number of consecutive epochs this node has been absent.
	MissedEpochs uint64 `json:"missed_epochs"`
}

// ─── StatusRecommendation ─────────────────────────────────────────────────────

// StatusRecommendation is sent by PoSeq to recommend a lifecycle status change
// for a sequencer (e.g. inactivity → Suspended).
// Applied immediately if Params.AutoApplySuspensions is true;
// otherwise stored for governance review via the ExportBatch audit trail.
type StatusRecommendation struct {
	// NodeID is the 32-byte node identity.
	NodeID []byte `json:"node_id"`
	// RecommendedStatus is the target status: "Suspended" or "Jailed".
	// Only degrading statuses are recommended by PoSeq.
	RecommendedStatus string `json:"recommended_status"`
	// Reason is a human-readable explanation for audit logging.
	Reason string `json:"reason"`
	// Epoch is the PoSeq epoch at which the recommendation was generated.
	Epoch uint64 `json:"epoch"`
}
