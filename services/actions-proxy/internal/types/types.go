// Package types defines shared request/response types for the actions proxy.
package types

// ConfirmRequest is the JSON body for POST /confirm-execution.
type ConfirmRequest struct {
	ProposalID string `json:"proposal_id"`
}

// ConfirmResponse is the JSON response for POST /confirm-execution.
type ConfirmResponse struct {
	ProposalID uint64      `json:"proposal_id"`
	Eligible   bool        `json:"eligible"`
	Action     string      `json:"action"`
	Result     string      `json:"result"` // "submitted", "already_confirmed", "rejected"
	Tx         interface{} `json:"tx,omitempty"`
	Message    string      `json:"message"`
}

// HealthResponse is the JSON response for GET /health.
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service"`
	Version string `json:"version"`
}

// GuardStatus holds the parsed guard query result for a proposal.
type GuardStatus struct {
	ProposalID            uint64 `json:"proposal_id"`
	GateState             string `json:"gate_state"`
	Tier                  string `json:"tier"`
	RequiresSecondConfirm bool   `json:"requires_second_confirm"`
	SecondConfirmReceived bool   `json:"second_confirm_received"`
	StatusNote            string `json:"status_note"`
}
