// Actions Proxy — Omniphi backend service for governance tx actions.
//
// Provides a secure HTTP API for the React governance dashboard to trigger
// confirm-execution for CRITICAL proposals without browser-side signing.
package main

import (
	"fmt"
	"log"
	"os"

	"actions-proxy/internal/chain"
	"actions-proxy/internal/config"
	"actions-proxy/internal/ratelimit"
	"actions-proxy/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "actions-proxy: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("actions-proxy starting")
	log.Printf("  bind:       %s", cfg.BindAddr)
	log.Printf("  chain-id:   %s", cfg.PosdChainID)
	log.Printf("  node:       %s", cfg.PosdNode)
	log.Printf("  key:        %s", cfg.KeyName)
	log.Printf("  rate-limit: %.2f rps, burst %d", cfg.RateLimitRPS, cfg.RateLimitBurst)
	log.Printf("  cors:       %v", cfg.CORSAllowOrigins)

	chainClient := chain.NewClient(cfg)
	limiter := ratelimit.New(cfg.RateLimitRPS, cfg.RateLimitBurst)
	srv := server.New(cfg, chainClient, limiter)

	log.Printf("listening on %s", cfg.BindAddr)
	return srv.ListenAndServe()
}
