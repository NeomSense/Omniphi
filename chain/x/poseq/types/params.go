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
}

func DefaultParams() Params {
	return Params{
		AuthorizedSubmitter:    "",
		AutoApplySuspensions:   false,
		MaxEvidencePerEpoch:    256,
		MaxEscalationsPerEpoch: 32,
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

func (p Params) Marshal() ([]byte, error)       { return json.Marshal(p) }
func (p *Params) Unmarshal(bz []byte) error     { return json.Unmarshal(bz, p) }
