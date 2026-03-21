package types

import "encoding/json"

// Params configures the x/poseq module.
type Params struct {
	// AuthorizedSubmitter is the bech32 address allowed to submit ExportBatch
	// records. Typically the PoSeq bridge relayer account. Empty = any address.
	AuthorizedSubmitter string `json:"authorized_submitter"`

	// AutoApplySuspensions controls whether committee suspension recommendations
	// are automatically applied (true) or queued for governance review (false).
	AutoApplySuspensions bool `json:"auto_apply_suspensions"`

	// MaxEvidencePerEpoch caps how many evidence packets can be submitted per
	// epoch to prevent state bloat from malicious/faulty relayers.
	MaxEvidencePerEpoch uint32 `json:"max_evidence_per_epoch"`

	// MaxEscalationsPerEpoch caps governance escalation records per epoch.
	MaxEscalationsPerEpoch uint32 `json:"max_escalations_per_epoch"`

	// InactivitySuspendEpochs is the number of consecutive missed epochs before
	// a node is automatically suspended. Triggers when missed > threshold.
	// 0 = disabled. Default: 4.
	InactivitySuspendEpochs uint32 `json:"inactivity_suspend_epochs"`

	// FaultJailThreshold is the number of fault events in an epoch that trigger
	// automatic jailing. 0 = disabled. Default: 5.
	FaultJailThreshold uint32 `json:"fault_jail_threshold"`

	// MinBondAmount is the minimum declared bond for committee eligibility.
	// 0 = bonding not required. Default: 0 (not required in Phase 5).
	MinBondAmount uint64 `json:"min_bond_amount"`

	// MaxSlashQueueDepth is the maximum number of pending slash entries before
	// oldest are evicted. Default: 1000.
	MaxSlashQueueDepth uint32 `json:"max_slash_queue_depth"`

	// RewardBaseMultiplierBps is the base reward multiplier for bonded nodes
	// in basis points (10000 = 1.0x). Default: 10000.
	RewardBaseMultiplierBps uint32 `json:"reward_base_multiplier_bps"`

	// ── Phase 6: Committee quality and slashing enforcement ──────────────────

	// MinBondForCommittee is the minimum AvailableBond required to be included
	// in a committee snapshot. 0 = disabled. Default: 0.
	MinBondForCommittee uint64 `json:"min_bond_for_committee"`

	// MinParticipationBps is the minimum participation rate (in basis points)
	// over the trailing window required for committee inclusion. 0 = disabled.
	// Default: 0.
	MinParticipationBps uint32 `json:"min_participation_bps"`

	// MaxFaultHistoryEpochs is the number of trailing epochs used to evaluate
	// fault history for committee admission. Default: 5.
	MaxFaultHistoryEpochs uint32 `json:"max_fault_history_epochs"`

	// SlashExecutionEnabled controls whether ExecuteSlash actually reduces the
	// AvailableBond. When false, slash records are stored but bond is untouched.
	// Default: false (Phase 5 behavior). Set true to enable Phase 6.
	SlashExecutionEnabled bool `json:"slash_execution_enabled"`

	// MaxEvidenceAgEpochs is the maximum age (in epochs) of evidence that can
	// be used in a slash. Evidence older than this is rejected as stale.
	// 0 = no limit. Default: 10.
	MaxEvidenceAgeEpochs uint32 `json:"max_evidence_age_epochs"`

	// RequireQCSignatures controls whether CommitExecution requires non-empty
	// QCSignatures. When false (default), empty QC is accepted with a warning
	// log (suitable for devnet/bootstrap). When true, empty QC is rejected
	// — this is the mainnet enforcement flag.
	RequireQCSignatures bool `json:"require_qc_signatures"`
}

func DefaultParams() Params {
	return Params{
		AuthorizedSubmitter:     "",
		AutoApplySuspensions:    true,
		MaxEvidencePerEpoch:     256,
		MaxEscalationsPerEpoch:  32,
		InactivitySuspendEpochs: 4,
		FaultJailThreshold:      5,
		MinBondAmount:           0,
		MaxSlashQueueDepth:      1000,
		RewardBaseMultiplierBps: 10000,
		MinBondForCommittee:     0,
		MinParticipationBps:     0,
		MaxFaultHistoryEpochs:   5,
		SlashExecutionEnabled:   true,
		MaxEvidenceAgeEpochs:    10,
		RequireQCSignatures:     false,
	}
}

func (p Params) Validate() error {
	if p.MaxEvidencePerEpoch == 0 {
		return ErrInvalidExportBatch.Wrap("max_evidence_per_epoch must be > 0")
	}
	if p.MaxEscalationsPerEpoch == 0 {
		return ErrInvalidExportBatch.Wrap("max_escalations_per_epoch must be > 0")
	}
	return nil
}

func (p Params) Marshal() ([]byte, error)   { return json.Marshal(p) }
func (p *Params) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }
