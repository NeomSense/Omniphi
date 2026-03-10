// Gov Copilot — Omniphi Layer 3 Advisory Intelligence Service.
//
// Polls x/gov for new proposals, fetches x/guard risk data, generates
// structured JSON advisory reports using DeepSeek (or a template fallback),
// stores them locally, and posts MsgSubmitAdvisoryLink on-chain.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gov-copilot/internal/ai/deepseek"
	"gov-copilot/internal/chain"
	"gov-copilot/internal/config"
	"gov-copilot/internal/report"
	"gov-copilot/internal/state"
	"gov-copilot/internal/store/local"
	"gov-copilot/internal/uri"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gov-copilot: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("gov-copilot starting")
	log.Printf("  node:      %s", cfg.PosdNode)
	log.Printf("  chain-id:  %s", cfg.ChainID)
	log.Printf("  key:       %s", cfg.KeyName)
	log.Printf("  ai-mode:   %s", cfg.AIMode)
	log.Printf("  poll:      %ds", cfg.PollIntervalSeconds)
	log.Printf("  reports:   %s", cfg.ReportDir)
	if cfg.ReportPublicEnabled {
		log.Printf("  public-url: %s", cfg.ReportPublicBaseURL)
	} else {
		log.Printf("  public-url: disabled (using file:// URIs)")
	}

	chainClient := chain.NewClient(cfg)

	var dsClient *deepseek.Client
	if cfg.AIMode == "deepseek" {
		dsClient = deepseek.NewClient(cfg)
		log.Printf("  deepseek:  %s (%s)", cfg.DeepSeekBaseURL, cfg.DeepSeekModel)
	}

	st, err := state.Load(cfg.StateFile)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	log.Printf("  state:     %s (last_seen=%d)", cfg.StateFile, st.LastSeenProposalID)

	store, err := local.NewStore(cfg.ReportDir)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}

	// Start optional built-in HTTP file server for dev/testnet
	if cfg.ReportHTTPServeEnabled {
		go func() {
			mux := http.NewServeMux()
			fs := http.FileServer(http.Dir(cfg.ReportDir))
			mux.Handle("/reports/", http.StripPrefix("/reports/", fs))
			log.Printf("  http-server: serving %s at http://%s/reports/", cfg.ReportDir, cfg.ReportHTTPBindAddr)
			if err := http.ListenAndServe(cfg.ReportHTTPBindAddr, mux); err != nil {
				log.Printf("http server error: %v", err)
			}
		}()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initial poll immediately, then on interval
	if err := pollOnce(ctx, cfg, chainClient, dsClient, st, store); err != nil {
		log.Printf("initial poll error: %v", err)
	}

	ticker := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := pollOnce(ctx, cfg, chainClient, dsClient, st, store); err != nil {
				log.Printf("poll error: %v", err)
			}
		case <-ctx.Done():
			log.Printf("shutting down")
			return nil
		}
	}
}

func pollOnce(
	ctx context.Context,
	cfg *config.Config,
	cc *chain.Client,
	ds *deepseek.Client,
	st *state.State,
	store *local.Store,
) error {
	// Fetch proposals — try all statuses, we let the guard module handle filtering
	proposals, err := cc.ListProposals(ctx, "")
	if err != nil {
		return fmt.Errorf("list proposals: %w", err)
	}

	for _, p := range proposals {
		if p.ID == 0 {
			continue
		}

		// Skip already processed
		if st.IsProcessed(p.ID) {
			continue
		}

		// Skip if advisory link already exists on-chain
		if cc.HasAdvisoryLink(ctx, p.ID) {
			log.Printf("proposal %d: advisory link already exists, skipping", p.ID)
			if err := st.MarkProcessed(p.ID, "exists-on-chain"); err != nil {
				log.Printf("state save error: %v", err)
			}
			continue
		}

		log.Printf("proposal %d: processing (%s)", p.ID, p.Title)

		if err := processProposal(ctx, cfg, cc, ds, st, store, &p); err != nil {
			log.Printf("proposal %d: error: %v", p.ID, err)
			continue
		}
	}

	return nil
}

func processProposal(
	ctx context.Context,
	cfg *config.Config,
	cc *chain.Client,
	ds *deepseek.Client,
	st *state.State,
	store *local.Store,
	proposal *chain.Proposal,
) error {
	// Fetch guard data (best-effort)
	riskReport, err := cc.GetRiskReport(ctx, proposal.ID)
	if err != nil {
		log.Printf("proposal %d: risk report unavailable: %v", proposal.ID, err)
	}

	queuedExec, err := cc.GetQueuedExecution(ctx, proposal.ID)
	if err != nil {
		log.Printf("proposal %d: queued execution unavailable: %v", proposal.ID, err)
	}

	// Generate report
	var r *report.Report

	if ds != nil {
		aiReport, err := ds.GenerateReport(ctx, proposal, riskReport, queuedExec)
		if err != nil {
			log.Printf("proposal %d: DeepSeek failed (%v), falling back to template", proposal.ID, err)
		} else {
			r = aiReport
			// Populate metadata that the AI doesn't fill
			r.ProposalID = proposal.ID
			r.ChainID = cfg.ChainID
			r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
			r.Reporter = cfg.ReporterID
			r.AIProvider = "deepseek"

			// Populate risk/timeline from guard data
			if riskReport != nil {
				r.Risk = report.RiskSection{
					TierRules:   riskReport.Tier,
					TierAI:      riskReport.AITier,
					TierFinal:   riskReport.Tier,
					TreasuryBps: 0, // filled from risk report if available
					ChurnBps:    0,
				}
			}
			if queuedExec != nil {
				r.Timeline = report.TimelineSection{
					CurrentGate:        queuedExec.GateState,
					EarliestExecHeight: queuedExec.EarliestExecHeight,
				}
			}
		}
	}

	// Fallback to template if AI didn't produce a report
	if r == nil {
		r = report.GenerateTemplate(proposal, riskReport, queuedExec, cfg.ChainID, cfg.ReporterID)
	}

	// Store report locally
	filePath, reportHash, err := store.SaveReport(r)
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	log.Printf("proposal %d: report saved to %s (hash=%s)", proposal.ID, filePath, reportHash[:16]+"...")

	// Generate URI: public HTTPS when enabled, file:// fallback
	var reportURI string
	if cfg.ReportPublicEnabled {
		publicURI, err := uri.MakePublicURI(cfg.ReportPublicBaseURL, proposal.ID)
		if err != nil {
			return fmt.Errorf("make public URI: %w", err)
		}
		reportURI = publicURI
	} else {
		log.Printf("proposal %d: REPORT_PUBLIC_ENABLED=false, using file:// URI (not browser-fetchable)", proposal.ID)
		reportURI = "file://" + filePath
	}

	// Submit advisory link on-chain
	if err := cc.SubmitAdvisoryLink(ctx, proposal.ID, reportURI, reportHash); err != nil {
		log.Printf("proposal %d: submit advisory link failed: %v", proposal.ID, err)
		// Still mark as processed — report is stored locally
	}

	// Update state
	if err := st.MarkProcessed(proposal.ID, reportHash); err != nil {
		log.Printf("proposal %d: state update failed: %v", proposal.ID, err)
	}

	log.Printf("proposal %d: done", proposal.ID)
	return nil
}
