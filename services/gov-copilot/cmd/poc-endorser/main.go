// PoC Endorser — Omniphi Layer 3 Automated Proof-of-Value Endorsement Service.
//
// Watches for poc_similarity_commitment events via CometBFT WebSocket,
// fetches contribution content from IPFS, evaluates quality using AI (OpenAI),
// and automatically broadcasts MsgEndorse transactions to approve or reject.
//
// Pipeline: Event → Fetch → Evaluate → Endorse
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gov-copilot/internal/endorser"
	"gov-copilot/internal/evaluator"
	"gov-copilot/internal/fetcher"
	"gov-copilot/internal/pocconfig"
	"gov-copilot/internal/watcher"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "poc-endorser: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := pocconfig.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("poc-endorser starting")
	log.Printf("  chain-id:        %s", cfg.ChainID)
	log.Printf("  node-ws:         %s", cfg.NodeWSURL)
	log.Printf("  node-rpc:        %s", cfg.NodeRPCURL)
	log.Printf("  key:             %s", cfg.KeyName)
	log.Printf("  ai-model:        %s (%s)", cfg.OpenAIModel, cfg.OpenAIBaseURL)
	log.Printf("  ipfs-gateway:    %s", cfg.IPFSGatewayURL)
	log.Printf("  approve >= %.1f", cfg.ApproveThreshold)
	log.Printf("  reject  <= %.1f", cfg.RejectThreshold)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize components
	bc := endorser.NewBroadcaster(cfg)

	// Pre-flight: verify validator key exists
	if err := bc.CheckValidatorKey(ctx); err != nil {
		return fmt.Errorf("validator key check failed: %w", err)
	}

	w := watcher.New(cfg.NodeWSURL)
	f := fetcher.New(cfg)
	e := evaluator.NewOpenAIEvaluator(evaluator.EvalConfig{
		OpenAIBaseURL:    cfg.OpenAIBaseURL,
		OpenAIModel:      cfg.OpenAIModel,
		OpenAIAPIKey:     cfg.OpenAIAPIKey,
		AITimeout:        cfg.AITimeout,
		AIMaxRetries:     cfg.AIMaxRetries,
		ApproveThreshold: cfg.ApproveThreshold,
		RejectThreshold:  cfg.RejectThreshold,
	})

	// Optional: check IPFS gateway connectivity
	f.WaitForIPFS(ctx)

	// Create and run the orchestrator
	orch := endorser.NewOrchestrator(cfg, w, f, e, bc)

	log.Printf("poc-endorser ready — listening for similarity events")

	if err := orch.Run(ctx); err != nil {
		return fmt.Errorf("orchestrator: %w", err)
	}

	log.Printf("poc-endorser stopped")
	return nil
}
