package endorser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gov-copilot/internal/pocconfig"
)

// Broadcaster signs and broadcasts MsgEndorse transactions via posd CLI.
type Broadcaster struct {
	cfg *pocconfig.Config

	// Sequence mutex prevents tx sequence collisions when broadcasting concurrently
	mu sync.Mutex
}

// NewBroadcaster creates a new transaction broadcaster.
func NewBroadcaster(cfg *pocconfig.Config) *Broadcaster {
	return &Broadcaster{cfg: cfg}
}

// TxResult holds the result of a broadcast transaction.
type TxResult struct {
	TxHash string
	Code   int
	RawLog string
}

// Endorse broadcasts a MsgEndorse transaction for the given contribution.
// decision=true means approve, decision=false means reject.
func (b *Broadcaster) Endorse(ctx context.Context, contributionID uint64, decision bool) (*TxResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	decisionStr := "false"
	if decision {
		decisionStr = "true"
	}

	args := []string{
		"tx", "poc", "endorse",
		fmt.Sprintf("%d", contributionID),
		decisionStr,
		"--node", b.cfg.NodeRPCURL,
		"--chain-id", b.cfg.ChainID,
		"--from", b.cfg.KeyName,
		"--keyring-backend", b.cfg.KeyringBackend,
		"-y",
		"-o", "json",
	}
	args = append(args, b.cfg.TxFeeFlags()...)

	log.Printf("[broadcaster] broadcasting MsgEndorse: contribution=%d decision=%v", contributionID, decision)

	cmd := exec.CommandContext(ctx, "posd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("posd tx poc endorse: %w\n%s", err, string(out))
	}

	var txResp struct {
		Code   int    `json:"code"`
		RawLog string `json:"raw_log"`
		TxHash string `json:"txhash"`
	}
	if err := json.Unmarshal(out, &txResp); err != nil {
		// Transaction may have succeeded even if response is unparseable
		log.Printf("[broadcaster] tx response (unparseable): %s", truncate(string(out), 200))
		return &TxResult{RawLog: string(out)}, nil
	}

	result := &TxResult{
		TxHash: txResp.TxHash,
		Code:   txResp.Code,
		RawLog: txResp.RawLog,
	}

	if txResp.Code != 0 {
		return result, fmt.Errorf("tx failed (code %d): %s", txResp.Code, txResp.RawLog)
	}

	log.Printf("[broadcaster] MsgEndorse submitted: tx=%s contribution=%d decision=%v",
		txResp.TxHash, contributionID, decision)

	// Brief pause to let the node's sequence counter update
	time.Sleep(500 * time.Millisecond)

	return result, nil
}

// CheckValidatorKey verifies that the configured key exists in the keyring.
func (b *Broadcaster) CheckValidatorKey(ctx context.Context) error {
	args := []string{
		"keys", "show", b.cfg.KeyName,
		"--keyring-backend", b.cfg.KeyringBackend,
		"-o", "json",
	}

	cmd := exec.CommandContext(ctx, "posd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("key %q not found in keyring: %w\n%s", b.cfg.KeyName, err, string(out))
	}

	var keyInfo struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(out, &keyInfo); err != nil {
		log.Printf("[broadcaster] key info (unparseable): %s", string(out))
		return nil // key exists but output format may differ
	}

	log.Printf("[broadcaster] validator key: name=%s address=%s", keyInfo.Name, keyInfo.Address)
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GetValidatorAddress returns the bech32 address of the configured key.
func (b *Broadcaster) GetValidatorAddress(ctx context.Context) (string, error) {
	args := []string{
		"keys", "show", b.cfg.KeyName,
		"--keyring-backend", b.cfg.KeyringBackend,
		"--bech", "val",
		"-o", "json",
	}

	cmd := exec.CommandContext(ctx, "posd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get validator address: %w\n%s", err, string(out))
	}

	var keyInfo struct {
		Address string `json:"address"`
	}
	// Try parsing as JSON, fall back to acc address if val address fails
	if err := json.Unmarshal(out, &keyInfo); err != nil {
		// Retry without --bech val (get acc address)
		args2 := []string{
			"keys", "show", b.cfg.KeyName,
			"--keyring-backend", b.cfg.KeyringBackend,
			"-o", "json",
		}
		cmd2 := exec.CommandContext(ctx, "posd", args2...)
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return "", fmt.Errorf("get validator address (retry): %w", err2)
		}
		if err := json.Unmarshal(out2, &keyInfo); err != nil {
			return "", fmt.Errorf("parse key info: %w", err)
		}
	}

	return strings.TrimSpace(keyInfo.Address), nil
}
