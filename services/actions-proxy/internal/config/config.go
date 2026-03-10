// Package config loads actions-proxy configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all service configuration.
type Config struct {
	// Server
	BindAddr string

	// Auth
	APIKey string

	// Chain / CLI
	PosdBin          string
	PosdNode         string
	PosdChainID      string
	PosdHome         string
	KeyName          string
	KeyringBackend   string
	TxGas            string
	TxGasAdjustment  string
	TxFees           string
	TxGasPrices      string
	TxBroadcastMode  string
	TxTimeoutSeconds int

	// Rate limit
	RateLimitRPS   float64
	RateLimitBurst int

	// CORS
	CORSAllowOrigins []string
}

// Load reads configuration from environment variables with defaults.
func Load() (*Config, error) {
	c := &Config{
		BindAddr:         envOr("BIND_ADDR", "127.0.0.1:8090"),
		APIKey:           os.Getenv("API_KEY"),
		PosdBin:          envOr("POSD_BIN", "posd"),
		PosdNode:         envOr("POSD_NODE", "http://localhost:26657"),
		PosdChainID:      os.Getenv("POSD_CHAIN_ID"),
		PosdHome:         os.Getenv("POSD_HOME"),
		KeyName:          os.Getenv("KEY_NAME"),
		KeyringBackend:   envOr("KEYRING_BACKEND", "test"),
		TxGas:            envOr("TX_GAS", "auto"),
		TxGasAdjustment:  envOr("TX_GAS_ADJUSTMENT", "1.3"),
		TxFees:           os.Getenv("TX_FEES"),
		TxGasPrices:      os.Getenv("TX_GAS_PRICES"),
		TxBroadcastMode:  envOr("TX_BROADCAST_MODE", "sync"),
		TxTimeoutSeconds: envIntOr("TX_TIMEOUT_SECONDS", 30),
		RateLimitRPS:     envFloatOr("RATE_LIMIT_RPS", 0.2),
		RateLimitBurst:   envIntOr("RATE_LIMIT_BURST", 2),
		CORSAllowOrigins: parseCORSOrigins(envOr("CORS_ALLOW_ORIGINS", "*")),
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("API_KEY is required")
	}
	if c.PosdChainID == "" {
		return fmt.Errorf("POSD_CHAIN_ID is required")
	}
	if c.KeyName == "" {
		return fmt.Errorf("KEY_NAME is required")
	}
	if c.TxFees == "" && c.TxGasPrices == "" {
		return fmt.Errorf("TX_FEES or TX_GAS_PRICES is required")
	}
	return nil
}

// TxFeeFlags returns CLI flags for transaction fees.
func (c *Config) TxFeeFlags() []string {
	if c.TxFees != "" {
		return []string{"--fees", c.TxFees}
	}
	flags := []string{"--gas-prices", c.TxGasPrices}
	return flags
}

func parseCORSOrigins(s string) []string {
	parts := strings.Split(s, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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

func envFloatOr(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
