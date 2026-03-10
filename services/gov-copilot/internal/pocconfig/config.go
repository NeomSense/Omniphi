package pocconfig

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config defines the configuration for the PoC Endorser service.
// It is extracted to a leaf package to avoid circular imports between
// endorser, fetcher, and evaluator.
type Config struct {
	ChainID          string        `yaml:"chain_id"`
	NodeWSURL        string        `yaml:"node_ws_url"`
	NodeRPCURL       string        `yaml:"node_rpc_url"`
	KeyName          string        `yaml:"key_name"`
	OpenAIModel      string        `yaml:"openai_model"`
	OpenAIBaseURL    string        `yaml:"openai_base_url"`
	OpenAIAPIKey     string        `yaml:"openai_api_key"`
	IPFSGatewayURL   string        `yaml:"ipfs_gateway_url"`
	ApproveThreshold float64       `yaml:"approve_threshold"`
	RejectThreshold  float64       `yaml:"reject_threshold"`
	AITimeout        time.Duration `yaml:"ai_timeout"`
	AIMaxRetries     int           `yaml:"ai_max_retries"`
}

// Load reads the configuration from config.yaml or environment variables.
func Load() (*Config, error) {
	// Default configuration
	cfg := &Config{
		ChainID:          "omniphi-1",
		NodeWSURL:        "ws://localhost:26657/websocket",
		NodeRPCURL:       "http://localhost:26657",
		KeyName:          "validator",
		OpenAIModel:      "gpt-4",
		OpenAIBaseURL:    "https://api.openai.com/v1",
		IPFSGatewayURL:   "https://ipfs.io/ipfs/",
		ApproveThreshold: 7.0,
		RejectThreshold:  3.0,
		AITimeout:        30 * time.Second,
		AIMaxRetries:     3,
	}

	// Attempt to load from config.yaml
	f, err := os.Open("config.yaml")
	if err == nil {
		defer f.Close()
		if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config.yaml: %w", err)
		}
	}

	// Environment variable overrides (critical for secrets)
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("VALIDATOR_KEY_NAME"); v != "" {
		cfg.KeyName = v
	}

	return cfg, nil
}
