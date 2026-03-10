// Package chain wraps posd CLI calls for querying and broadcasting transactions.
package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"gov-copilot/internal/config"
)

// Client executes posd CLI commands against a running node.
type Client struct {
	cfg *config.Config
}

// NewClient creates a chain client backed by the posd binary.
func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

// runQuery executes a posd query command and returns the raw JSON output.
func (c *Client) runQuery(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{
		"--node", c.cfg.PosdNode,
		"--chain-id", c.cfg.ChainID,
		"-o", "json",
	}
	full := append(args, base...)
	cmd := exec.CommandContext(ctx, "posd", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("posd %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

// runTx executes a posd tx command and returns the raw JSON response.
func (c *Client) runTx(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{
		"--node", c.cfg.PosdNode,
		"--chain-id", c.cfg.ChainID,
		"--from", c.cfg.KeyName,
		"--keyring-backend", c.cfg.KeyringBackend,
		"-y",
		"-o", "json",
	}
	base = append(base, c.cfg.TxFeeFlags()...)
	full := append(args, base...)
	cmd := exec.CommandContext(ctx, "posd", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("posd %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

// ---------- Governance Queries ----------

// Proposal represents a governance proposal from the chain.
type Proposal struct {
	ID           uint64   `json:"id"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Status       string   `json:"status"`
	MessageTypes []string `json:"message_types"`
}

// ListProposals returns proposals with the given status (e.g. "2" for VOTING_PERIOD).
// If status is empty, returns all proposals.
func (c *Client) ListProposals(ctx context.Context, status string) ([]Proposal, error) {
	args := []string{"query", "gov", "proposals"}
	if status != "" {
		args = append(args, "--status", status)
	}
	out, err := c.runQuery(ctx, args...)
	if err != nil {
		return nil, err
	}

	var result struct {
		Proposals []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Summary  string `json:"summary"`
			Status   string `json:"status"`
			Messages []struct {
				TypeURL string `json:"@type"`
			} `json:"messages"`
		} `json:"proposals"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse proposals: %w", err)
	}

	proposals := make([]Proposal, 0, len(result.Proposals))
	for _, p := range result.Proposals {
		var id uint64
		fmt.Sscanf(p.ID, "%d", &id)
		msgTypes := make([]string, len(p.Messages))
		for i, m := range p.Messages {
			msgTypes[i] = m.TypeURL
		}
		proposals = append(proposals, Proposal{
			ID:           id,
			Title:        p.Title,
			Summary:      p.Summary,
			Status:       p.Status,
			MessageTypes: msgTypes,
		})
	}
	return proposals, nil
}

// GetProposal fetches a single proposal by ID.
func (c *Client) GetProposal(ctx context.Context, id uint64) (*Proposal, error) {
	args := []string{"query", "gov", "proposal", fmt.Sprintf("%d", id)}
	out, err := c.runQuery(ctx, args...)
	if err != nil {
		return nil, err
	}

	var result struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Summary  string `json:"summary"`
		Status   string `json:"status"`
		Messages []struct {
			TypeURL string `json:"@type"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse proposal: %w", err)
	}

	var pid uint64
	fmt.Sscanf(result.ID, "%d", &pid)
	msgTypes := make([]string, len(result.Messages))
	for i, m := range result.Messages {
		msgTypes[i] = m.TypeURL
	}
	return &Proposal{
		ID:           pid,
		Title:        result.Title,
		Summary:      result.Summary,
		Status:       result.Status,
		MessageTypes: msgTypes,
	}, nil
}

// ---------- Guard Queries ----------

// RiskReport mirrors the on-chain RiskReport for a proposal.
type RiskReport struct {
	ProposalID           uint64 `json:"proposal_id"`
	Tier                 string `json:"tier"`
	Score                uint32 `json:"score"`
	ComputedDelayBlocks  uint64 `json:"computed_delay_blocks"`
	ComputedThresholdBps uint64 `json:"computed_threshold_bps"`
	ReasonCodes          string `json:"reason_codes"`
	ModelVersion         string `json:"model_version"`
	AIScore              uint32 `json:"ai_score"`
	AITier               string `json:"ai_tier"`
	AIModelVersion       string `json:"ai_model_version"`
	FeatureSchemaHash    string `json:"feature_schema_hash"`
}

// QueuedExecution mirrors the on-chain QueuedExecution state.
type QueuedExecution struct {
	ProposalID            uint64 `json:"proposal_id"`
	QueuedHeight          uint64 `json:"queued_height"`
	EarliestExecHeight    uint64 `json:"earliest_exec_height"`
	GateState             string `json:"gate_state"`
	Tier                  string `json:"tier"`
	RequiredThresholdBps  uint64 `json:"required_threshold_bps"`
	RequiresSecondConfirm bool   `json:"requires_second_confirm"`
	StatusNote            string `json:"status_note"`
}

// AdvisoryLink mirrors the on-chain AdvisoryLink.
type AdvisoryLink struct {
	ProposalID uint64 `json:"proposal_id"`
	URI        string `json:"uri"`
	ReportHash string `json:"report_hash"`
	Reporter   string `json:"reporter"`
	CreatedAt  int64  `json:"created_at"`
}

// GetRiskReport fetches the guard risk report for a proposal.
func (c *Client) GetRiskReport(ctx context.Context, proposalID uint64) (*RiskReport, error) {
	args := []string{"query", "guard", "risk-report", fmt.Sprintf("%d", proposalID)}
	out, err := c.runQuery(ctx, args...)
	if err != nil {
		return nil, err
	}

	var result struct {
		Report json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		// Try direct parse
		var rr RiskReport
		if err2 := json.Unmarshal(out, &rr); err2 != nil {
			return nil, fmt.Errorf("parse risk report: %w", err)
		}
		return &rr, nil
	}

	var rr RiskReport
	if err := json.Unmarshal(result.Report, &rr); err != nil {
		return nil, fmt.Errorf("parse risk report inner: %w", err)
	}
	return &rr, nil
}

// GetQueuedExecution fetches the queued execution state for a proposal.
func (c *Client) GetQueuedExecution(ctx context.Context, proposalID uint64) (*QueuedExecution, error) {
	args := []string{"query", "guard", "queued-execution", fmt.Sprintf("%d", proposalID)}
	out, err := c.runQuery(ctx, args...)
	if err != nil {
		return nil, err
	}

	var result struct {
		Execution json.RawMessage `json:"execution"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		var qe QueuedExecution
		if err2 := json.Unmarshal(out, &qe); err2 != nil {
			return nil, fmt.Errorf("parse queued execution: %w", err)
		}
		return &qe, nil
	}

	var qe QueuedExecution
	if err := json.Unmarshal(result.Execution, &qe); err != nil {
		return nil, fmt.Errorf("parse queued execution inner: %w", err)
	}
	return &qe, nil
}

// GetAdvisoryLink fetches the advisory link for a proposal (if it exists).
func (c *Client) GetAdvisoryLink(ctx context.Context, proposalID uint64) (*AdvisoryLink, error) {
	args := []string{"query", "guard", "advisory-link", fmt.Sprintf("%d", proposalID)}
	out, err := c.runQuery(ctx, args...)
	if err != nil {
		return nil, err
	}

	var result struct {
		Link json.RawMessage `json:"link"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		var al AdvisoryLink
		if err2 := json.Unmarshal(out, &al); err2 != nil {
			return nil, fmt.Errorf("parse advisory link: %w", err)
		}
		return &al, nil
	}

	var al AdvisoryLink
	if err := json.Unmarshal(result.Link, &al); err != nil {
		return nil, fmt.Errorf("parse advisory link inner: %w", err)
	}
	return &al, nil
}

// HasAdvisoryLink returns true if an advisory link already exists for this proposal.
func (c *Client) HasAdvisoryLink(ctx context.Context, proposalID uint64) bool {
	link, err := c.GetAdvisoryLink(ctx, proposalID)
	if err != nil {
		return false
	}
	return link != nil && link.URI != ""
}

// SubmitAdvisoryLink broadcasts MsgSubmitAdvisoryLink via the posd CLI.
func (c *Client) SubmitAdvisoryLink(ctx context.Context, proposalID uint64, uri, reportHash string) error {
	// Validate URI scheme before passing to CLI to prevent injection
	if !strings.HasPrefix(uri, "file://") && !strings.HasPrefix(uri, "http://") &&
		!strings.HasPrefix(uri, "https://") && !strings.HasPrefix(uri, "ipfs://") {
		return fmt.Errorf("unsupported URI scheme: %s (must be file://, http://, https://, or ipfs://)", uri)
	}

	args := []string{
		"tx", "guard", "submit-advisory-link",
		fmt.Sprintf("%d", proposalID),
		uri,
		reportHash,
	}
	out, err := c.runTx(ctx, args...)
	if err != nil {
		return err
	}

	// Check tx response for errors
	var txResp struct {
		Code   int    `json:"code"`
		RawLog string `json:"raw_log"`
		TxHash string `json:"txhash"`
	}
	if err := json.Unmarshal(out, &txResp); err != nil {
		log.Printf("tx response (unparseable): %s", string(out))
		return nil // tx may have succeeded even if we can't parse
	}
	if txResp.Code != 0 {
		return fmt.Errorf("tx failed (code %d): %s", txResp.Code, txResp.RawLog)
	}

	log.Printf("Advisory link submitted: tx=%s", txResp.TxHash)
	return nil
}
