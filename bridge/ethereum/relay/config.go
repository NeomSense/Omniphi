// Package relay implements the Omniphi <-> Ethereum bridge relay service.
//
// The relay watches for deposit events on Ethereum and burn events on Omniphi,
// co-signs attestations with other relayers, and submits the corresponding
// mint / withdrawal transactions to the destination chain.
package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all relay-service configuration.
type Config struct {
	// ── Ethereum settings ───────────────────────────────────────────────
	EthereumRPC            string `json:"ethereum_rpc"`
	BridgeContractAddress  string `json:"bridge_contract_address"`
	EthPrivateKey          string `json:"private_key"`
	EthChainID             int64  `json:"eth_chain_id"`
	EthConfirmations       uint64 `json:"eth_confirmations"`       // blocks to wait before acting on events
	EthPollInterval        Duration `json:"eth_poll_interval"`      // how often to poll for new logs
	EthStartBlock          uint64 `json:"eth_start_block"`          // block to begin scanning from (0 = latest)

	// ── Omniphi settings ────────────────────────────────────────────────
	OmniphiRPC             string `json:"omniphi_rpc"`
	OmniphiChainID         string `json:"omniphi_chain_id"`
	OmniphiKeyName         string `json:"omniphi_key_name"`        // keyring key name for signing Cosmos txs
	OmniphiKeyringBackend  string `json:"omniphi_keyring_backend"` // "test", "file", "os"
	OmniphiKeyringDir      string `json:"omniphi_keyring_dir"`
	OmniphiGasPrice        string `json:"omniphi_gas_price"`       // e.g. "0.025uomni"
	OmniphiGasAdjustment   float64 `json:"omniphi_gas_adjustment"`

	// ── Relay behaviour ─────────────────────────────────────────────────
	RetryMaxAttempts       int      `json:"retry_max_attempts"`
	RetryBaseDelay         Duration `json:"retry_base_delay"`
	RetryMaxDelay          Duration `json:"retry_max_delay"`
	BatchSize              int      `json:"batch_size"`            // max events per poll cycle

	// ── Health-check server ─────────────────────────────────────────────
	HealthAddr             string `json:"health_addr"` // e.g. ":8080"

	// ── Logging ─────────────────────────────────────────────────────────
	LogLevel               string `json:"log_level"` // "debug", "info", "warn", "error"
}

// Duration wraps time.Duration for JSON marshalling (accepts Go duration
// strings like "5s", "2m30s").
type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

// DefaultConfig returns a Config pre-filled with sane defaults.
func DefaultConfig() Config {
	return Config{
		EthereumRPC:           "http://127.0.0.1:8545",
		EthChainID:            1,
		EthConfirmations:      12,
		EthPollInterval:       Duration{15 * time.Second},
		OmniphiRPC:            "http://127.0.0.1:26657",
		OmniphiChainID:        "omniphi-testnet-2",
		OmniphiKeyringBackend: "test",
		OmniphiGasPrice:       "0.025uomni",
		OmniphiGasAdjustment:  1.5,
		RetryMaxAttempts:      10,
		RetryBaseDelay:        Duration{2 * time.Second},
		RetryMaxDelay:         Duration{5 * time.Minute},
		BatchSize:             50,
		HealthAddr:            ":8081",
		LogLevel:              "info",
	}
}

// LoadConfig reads a JSON config file and returns the parsed Config.
// Missing fields are filled from DefaultConfig().
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate checks that all required fields are set and internally consistent.
func (c *Config) Validate() error {
	var errs []string

	if c.EthereumRPC == "" {
		errs = append(errs, "ethereum_rpc is required")
	}
	if c.BridgeContractAddress == "" {
		errs = append(errs, "bridge_contract_address is required")
	}
	if c.EthPrivateKey == "" {
		errs = append(errs, "private_key is required")
	}
	if c.OmniphiRPC == "" {
		errs = append(errs, "omniphi_rpc is required")
	}
	if c.OmniphiChainID == "" {
		errs = append(errs, "omniphi_chain_id is required")
	}
	if c.EthChainID <= 0 {
		errs = append(errs, "eth_chain_id must be positive")
	}
	if c.EthConfirmations == 0 {
		errs = append(errs, "eth_confirmations must be > 0")
	}
	if c.EthPollInterval.Duration < time.Second {
		errs = append(errs, "eth_poll_interval must be >= 1s")
	}
	if c.RetryMaxAttempts <= 0 {
		errs = append(errs, "retry_max_attempts must be positive")
	}
	if c.RetryBaseDelay.Duration <= 0 {
		errs = append(errs, "retry_base_delay must be positive")
	}
	if c.BatchSize <= 0 {
		errs = append(errs, "batch_size must be positive")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
