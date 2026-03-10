package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all service configuration, loaded from environment variables.
type Config struct {
	// Chain
	PosdNode       string
	ChainID        string
	KeyName        string
	KeyringBackend string
	TxFees         string
	TxGasPrices    string
	TxGas          string

	// Polling
	PollIntervalSeconds int
	ReportDir           string
	ReporterID          string

	// DeepSeek
	DeepSeekAPIKey  string
	DeepSeekBaseURL string
	DeepSeekModel   string
	AIMode          string // "deepseek" or "template"
	AITimeoutSecs   int
	AIMaxRetries    int

	// Public Report Hosting
	ReportPublicEnabled bool   // generate public URIs (default true)
	ReportPublicBaseURL string // e.g. "https://copilot.omniphi.org/reports"

	// Built-in HTTP file server (dev/testnet only)
	ReportHTTPServeEnabled bool   // serve REPORT_DIR over HTTP (default false)
	ReportHTTPBindAddr     string // listen address for the file server

	// State
	StateFile string
}

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
	c := &Config{
		PosdNode:            envOr("POSD_NODE", "http://localhost:26657"),
		ChainID:             os.Getenv("POSD_CHAIN_ID"),
		KeyName:             os.Getenv("KEY_NAME"),
		KeyringBackend:      envOr("KEYRING_BACKEND", "test"),
		TxFees:              os.Getenv("TX_FEES"),
		TxGasPrices:         os.Getenv("TX_GAS_PRICES"),
		TxGas:               os.Getenv("TX_GAS"),
		PollIntervalSeconds: envIntOr("POLL_INTERVAL_SECONDS", 10),
		ReportDir:           envOr("REPORT_DIR", "./reports"),
		ReporterID:          envOr("REPORTER_ID", "gov-copilot-v1"),
		DeepSeekAPIKey:      os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekBaseURL:     envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekModel:       envOr("DEEPSEEK_MODEL", "deepseek-chat"),
		AIMode:              envOr("AI_MODE", "deepseek"),
		AITimeoutSecs:       envIntOr("AI_TIMEOUT_SECONDS", 20),
		AIMaxRetries:        envIntOr("AI_MAX_RETRIES", 2),

		ReportPublicEnabled:    envBoolOr("REPORT_PUBLIC_ENABLED", true),
		ReportPublicBaseURL:    strings.TrimRight(os.Getenv("REPORT_PUBLIC_BASE_URL"), "/"),
		ReportHTTPServeEnabled: envBoolOr("REPORT_HTTP_SERVE_ENABLED", false),
		ReportHTTPBindAddr:     envOr("REPORT_HTTP_BIND_ADDR", "127.0.0.1:8088"),

		StateFile: envOr("STATE_FILE", "./state.json"),
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.ChainID == "" {
		return fmt.Errorf("POSD_CHAIN_ID is required")
	}
	if c.KeyName == "" {
		return fmt.Errorf("KEY_NAME is required")
	}
	if c.AIMode != "deepseek" && c.AIMode != "template" {
		return fmt.Errorf("AI_MODE must be 'deepseek' or 'template', got %q", c.AIMode)
	}
	if c.AIMode == "deepseek" && c.DeepSeekAPIKey == "" {
		return fmt.Errorf("DEEPSEEK_API_KEY is required when AI_MODE=deepseek")
	}
	if c.TxFees == "" && c.TxGasPrices == "" {
		return fmt.Errorf("TX_FEES or TX_GAS_PRICES is required")
	}
	if c.ReportPublicEnabled && c.ReportPublicBaseURL == "" {
		return fmt.Errorf("REPORT_PUBLIC_BASE_URL is required when REPORT_PUBLIC_ENABLED=true")
	}
	return nil
}

// TxFeeFlags returns CLI flags for transaction fees.
func (c *Config) TxFeeFlags() []string {
	if c.TxFees != "" {
		return []string{"--fees", c.TxFees}
	}
	flags := []string{"--gas-prices", c.TxGasPrices}
	if c.TxGas != "" {
		flags = append(flags, "--gas", c.TxGas)
	} else {
		flags = append(flags, "--gas", "auto")
	}
	return flags
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBoolOr(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
