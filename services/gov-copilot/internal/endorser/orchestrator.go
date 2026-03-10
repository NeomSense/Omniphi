package endorser

import (
	"context"
	"fmt"
	"log"
	"sync"

	"gov-copilot/internal/evaluator"
	"gov-copilot/internal/fetcher"
	"gov-copilot/internal/pocconfig"
	"gov-copilot/internal/watcher"
)

// Orchestrator coordinates the full endorsement pipeline:
//
//	Event → Fetch → Evaluate → Endorse
type Orchestrator struct {
	cfg         *pocconfig.Config
	watcher     *watcher.Watcher
	fetcher     *fetcher.Fetcher
	evaluator   evaluator.Evaluator
	broadcaster *Broadcaster

	// Track processed contributions to avoid duplicate endorsements
	processed sync.Map
}

// NewOrchestrator creates a new pipeline orchestrator.
func NewOrchestrator(
	cfg *pocconfig.Config,
	w *watcher.Watcher,
	f *fetcher.Fetcher,
	e evaluator.Evaluator,
	b *Broadcaster,
) *Orchestrator {
	return &Orchestrator{
		cfg:         cfg,
		watcher:     w,
		fetcher:     f,
		evaluator:   e,
		broadcaster: b,
	}
}

// Run starts the orchestrator. It blocks until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) error {
	events, err := o.watcher.Watch(ctx)
	if err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	log.Printf("[orchestrator] pipeline started, listening for similarity events...")

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				log.Printf("[orchestrator] event channel closed, shutting down")
				return nil
			}
			o.handleEvent(ctx, evt)

		case <-ctx.Done():
			log.Printf("[orchestrator] context cancelled, shutting down")
			return nil
		}
	}
}

// handleEvent processes a single similarity event through the full pipeline.
func (o *Orchestrator) handleEvent(ctx context.Context, evt watcher.SimilarityEvent) {
	// Dedup: skip if already processed
	if _, loaded := o.processed.LoadOrStore(evt.ContributionID, true); loaded {
		log.Printf("[orchestrator] contribution %d already processed, skipping", evt.ContributionID)
		return
	}

	log.Printf("[orchestrator] processing contribution %d (derivative=%v similarity=%d confidence=%d)",
		evt.ContributionID, evt.IsDerivative, evt.OverallSimilarity, evt.Confidence)

	// Step 1: Fetch contribution metadata from chain
	contrib, err := o.fetcher.FetchContribution(ctx, evt.ContributionID)
	if err != nil {
		log.Printf("[orchestrator] ERROR: fetch contribution %d: %v", evt.ContributionID, err)
		o.processed.Delete(evt.ContributionID) // allow retry
		return
	}

	// Step 2: If already verified, skip
	if contrib.Verified {
		log.Printf("[orchestrator] contribution %d already verified, skipping", evt.ContributionID)
		return
	}

	// Step 3: Fetch content (skip for derivatives — they'll be auto-rejected)
	var content *fetcher.ContentResult
	if !evt.IsDerivative {
		content, err = o.fetcher.FetchContent(ctx, contrib)
		if err != nil {
			log.Printf("[orchestrator] ERROR: fetch content %d: %v", evt.ContributionID, err)
			o.processed.Delete(evt.ContributionID) // allow retry
			return
		}
	} else {
		// Create a minimal content result for derivative auto-reject
		content = &fetcher.ContentResult{
			Contribution: *contrib,
			ContentType:  "derivative",
		}
	}

	// Step 4: Evaluate with AI
	evalResult, err := o.evaluator.Evaluate(ctx, content, evt.IsDerivative)
	if err != nil {
		log.Printf("[orchestrator] ERROR: evaluate contribution %d: %v", evt.ContributionID, err)
		o.processed.Delete(evt.ContributionID) // allow retry
		return
	}

	log.Printf("[orchestrator] contribution %d scored: quality=%.1f originality=%.1f correctness=%.1f weighted=%.2f → %s",
		evt.ContributionID,
		evalResult.Score.Quality,
		evalResult.Score.Originality,
		evalResult.Score.Correctness,
		evalResult.Score.Weighted,
		evalResult.Decision)

	if evalResult.Score.Reasoning != "" {
		log.Printf("[orchestrator] contribution %d reasoning: %s", evt.ContributionID, evalResult.Score.Reasoning)
	}

	// Step 5: Act on decision
	switch evalResult.Decision {
	case evaluator.DecisionApprove:
		txResult, err := o.broadcaster.Endorse(ctx, evt.ContributionID, true)
		if err != nil {
			log.Printf("[orchestrator] ERROR: endorse (approve) contribution %d: %v", evt.ContributionID, err)
			return
		}
		log.Printf("[orchestrator] APPROVED contribution %d (tx=%s score=%.2f)",
			evt.ContributionID, txResult.TxHash, evalResult.Score.Weighted)

	case evaluator.DecisionReject, evaluator.DecisionAutoReject:
		txResult, err := o.broadcaster.Endorse(ctx, evt.ContributionID, false)
		if err != nil {
			log.Printf("[orchestrator] ERROR: endorse (reject) contribution %d: %v", evt.ContributionID, err)
			return
		}
		action := "REJECTED"
		if evalResult.Decision == evaluator.DecisionAutoReject {
			action = "AUTO-REJECTED (derivative)"
		}
		log.Printf("[orchestrator] %s contribution %d (tx=%s score=%.2f)",
			action, evt.ContributionID, txResult.TxHash, evalResult.Score.Weighted)

	case evaluator.DecisionManualReview:
		log.Printf("[orchestrator] MANUAL REVIEW NEEDED: contribution %d (score=%.2f, thresholds: reject<=%.1f, approve>=%.1f)",
			evt.ContributionID, evalResult.Score.Weighted, o.cfg.RejectThreshold, o.cfg.ApproveThreshold)
	}
}
