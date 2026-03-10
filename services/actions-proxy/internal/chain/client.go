// Package chain wraps posd CLI calls for querying guard status and broadcasting txs.
package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"actions-proxy/internal/config"
	"actions-proxy/internal/types"
)

// Client executes posd CLI commands.
type Client struct {
	cfg *config.Config
}

// NewClient creates a chain client.
func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

// runQuery executes a posd query command and returns raw JSON output.
func (c *Client) runQuery(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{
		"--node", c.cfg.PosdNode,
		"--chain-id", c.cfg.PosdChainID,
		"-o", "json",
	}
	if c.cfg.PosdHome != "" {
		base = append(base, "--home", c.cfg.PosdHome)
	}
	full := append(args, base...)

	timeout := time.Duration(c.cfg.TxTimeoutSeconds) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.cfg.PosdBin, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("posd %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

// runTx executes a posd tx command and returns raw JSON output.
func (c *Client) runTx(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{
		"--node", c.cfg.PosdNode,
		"--chain-id", c.cfg.PosdChainID,
		"--from", c.cfg.KeyName,
		"--keyring-backend", c.cfg.KeyringBackend,
		"--gas", c.cfg.TxGas,
		"--gas-adjustment", c.cfg.TxGasAdjustment,
		"--broadcast-mode", c.cfg.TxBroadcastMode,
		"-y",
		"-o", "json",
	}
	base = append(base, c.cfg.TxFeeFlags()...)
	if c.cfg.PosdHome != "" {
		base = append(base, "--home", c.cfg.PosdHome)
	}
	full := append(args, base...)

	timeout := time.Duration(c.cfg.TxTimeoutSeconds) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.cfg.PosdBin, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("posd %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

// GetGuardStatus queries the guard module for a proposal's execution status.
// Tries queued-execution query to get gate state and confirmation status.
func (c *Client) GetGuardStatus(ctx context.Context, proposalID uint64) (*types.GuardStatus, error) {
	args := []string{"query", "guard", "queued-execution", fmt.Sprintf("%d", proposalID)}
	out, err := c.runQuery(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("query guard status: %w", err)
	}

	// Try to parse with wrapper first, then direct
	var wrapper struct {
		Execution json.RawMessage `json:"execution"`
	}
	if err := json.Unmarshal(out, &wrapper); err == nil && len(wrapper.Execution) > 0 {
		out = wrapper.Execution
	}

	var status types.GuardStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parse guard status: %w", err)
	}

	return &status, nil
}

// GetRiskTier queries the guard risk report for a proposal to get the final tier.
func (c *Client) GetRiskTier(ctx context.Context, proposalID uint64) (string, error) {
	args := []string{"query", "guard", "risk-report", fmt.Sprintf("%d", proposalID)}
	out, err := c.runQuery(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("query risk report: %w", err)
	}

	// Try wrapper, then direct
	var wrapper struct {
		Report json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(out, &wrapper); err == nil && len(wrapper.Report) > 0 {
		out = wrapper.Report
	}

	var report struct {
		Tier string `json:"tier"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		return "", fmt.Errorf("parse risk report: %w", err)
	}

	return report.Tier, nil
}

// TxResult holds the parsed tx broadcast response.
type TxResult struct {
	Code   int    `json:"code"`
	TxHash string `json:"txhash"`
	RawLog string `json:"raw_log"`
}

// ConfirmExecution broadcasts MsgConfirmExecution for a CRITICAL proposal.
func (c *Client) ConfirmExecution(ctx context.Context, proposalID uint64, justification string) (*TxResult, error) {
	args := []string{
		"tx", "guard", "confirm-execution",
		fmt.Sprintf("%d", proposalID),
		"--justification", justification,
	}
	out, err := c.runTx(ctx, args...)
	if err != nil {
		return nil, err
	}

	var result TxResult
	if err := json.Unmarshal(out, &result); err != nil {
		// Return raw output as message if we can't parse
		return &TxResult{
			Code:   -1,
			RawLog: string(out),
		}, nil
	}

	return &result, nil
}
