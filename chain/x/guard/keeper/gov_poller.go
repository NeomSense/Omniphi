package keeper

import (
	"context"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// PollGovernanceProposals checks for newly passed proposals and queues them.
// This is called from EndBlocker to detect proposals that transitioned to PASSED status.
// The per-block limit is governance-configurable via MaxProposalsPerBlock.
func (k Keeper) PollGovernanceProposals(ctx context.Context) error {
	params := k.GetParams(ctx)

	// When timelock integration is active, proposals arrive via direct keeper
	// call (OnTimelockQueued) instead of polling. Skip the expensive iteration.
	if params.TimelockIntegrationEnabled {
		return nil
	}

	lastProcessed := k.GetLastProcessedProposalID(ctx)
	maxToProcess := params.MaxProposalsPerBlock

	processed := uint64(0)
	highestSeen := lastProcessed

	// Iterate proposals starting from last processed
	err := k.govKeeper.IterateProposals(ctx, func(proposal govtypes.Proposal) (stop bool) {
		// Skip if already processed
		if proposal.Id <= lastProcessed {
			return false
		}

		// Track highest ID seen
		if proposal.Id > highestSeen {
			highestSeen = proposal.Id
		}

		// Only process PASSED proposals
		if proposal.Status != govtypes.StatusPassed {
			return false
		}

		// Check if already queued (idempotency)
		if _, exists := k.GetQueuedExecution(ctx, proposal.Id); exists {
			return false
		}

		// Queue the proposal for guarded execution
		if err := k.OnProposalPassed(ctx, proposal.Id); err != nil {
			k.logger.Error("failed to queue passed proposal",
				"proposal_id", proposal.Id,
				"error", err,
			)
			// Continue processing other proposals
		}

		processed++
		if processed >= maxToProcess {
			return true // stop iteration
		}

		return false
	})

	if err != nil {
		return err
	}

	// Update last processed ID
	if highestSeen > lastProcessed {
		if err := k.SetLastProcessedProposalID(ctx, highestSeen); err != nil {
			return err
		}
	}

	if processed > 0 {
		k.logger.Info("processed governance proposals",
			"count", processed,
			"highest_id", highestSeen,
		)
	}

	return nil
}
