package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// GovHooks implements the gov module's GovHooks interface
// This is the key integration point - when a proposal passes,
// instead of executing immediately, we queue it for timelock execution
type GovHooks struct {
	keeper *Keeper
}

var _ govtypes.GovHooks = GovHooks{}

// NewGovHooks creates a new GovHooks instance
func NewGovHooks(keeper *Keeper) GovHooks {
	return GovHooks{keeper: keeper}
}

// AfterProposalSubmission is called after a proposal is submitted
func (h GovHooks) AfterProposalSubmission(ctx context.Context, proposalID uint64) error {
	// No action needed at submission time
	return nil
}

// AfterProposalDeposit is called after a deposit is made on a proposal
func (h GovHooks) AfterProposalDeposit(ctx context.Context, proposalID uint64, depositorAddr sdk.AccAddress) error {
	// No action needed at deposit time
	return nil
}

// AfterProposalVote is called after a vote is cast on a proposal
func (h GovHooks) AfterProposalVote(ctx context.Context, proposalID uint64, voterAddr sdk.AccAddress) error {
	// No action needed at vote time
	return nil
}

// AfterProposalFailedMinDeposit is called when a proposal fails to meet minimum deposit
func (h GovHooks) AfterProposalFailedMinDeposit(ctx context.Context, proposalID uint64) error {
	// No action needed - proposal never entered voting
	return nil
}

// AfterProposalVotingPeriodEnded is called when a proposal's voting period ends
// This is where we intercept passed proposals and queue them for timelock execution
func (h GovHooks) AfterProposalVotingPeriodEnded(ctx context.Context, proposalID uint64) error {
	// NOTE: This hook is called AFTER the voting period ends but BEFORE
	// the standard gov module executes the messages. To properly implement
	// timelock, we need to modify the gov module's EndBlocker or use
	// a wrapper approach. For now, this hook just logs the event.

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	h.keeper.Logger().Info("proposal voting period ended",
		"proposal_id", proposalID,
		"height", sdkCtx.BlockHeight(),
	)

	return nil
}

// TimelockProposalHandler is a custom proposal handler that queues messages
// instead of executing them immediately. This is used with a wrapped gov keeper.
type TimelockProposalHandler struct {
	timelockKeeper *Keeper
}

// NewTimelockProposalHandler creates a new timelock proposal handler
func NewTimelockProposalHandler(keeper *Keeper) *TimelockProposalHandler {
	return &TimelockProposalHandler{
		timelockKeeper: keeper,
	}
}

// HandleProposal queues a passed proposal for timelock execution
// This should be called by a modified gov EndBlocker when a proposal passes
func (h *TimelockProposalHandler) HandleProposal(ctx context.Context, proposal govv1.Proposal) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Only handle passed proposals
	if proposal.Status != govv1.StatusPassed {
		return nil
	}

	// Get messages from proposal
	msgs, err := proposal.GetMsgs()
	if err != nil {
		return fmt.Errorf("failed to get proposal messages: %w", err)
	}

	if len(msgs) == 0 {
		// Text proposal - no messages to execute
		h.timelockKeeper.Logger().Info("text proposal passed, no messages to queue",
			"proposal_id", proposal.Id,
		)
		return nil
	}

	// Queue the operation with governance as executor
	op, err := h.timelockKeeper.QueueOperation(
		ctx,
		proposal.Id,
		msgs,
		h.timelockKeeper.GetAuthority(), // Gov module is the executor
	)
	if err != nil {
		return fmt.Errorf("failed to queue proposal %d: %w", proposal.Id, err)
	}

	h.timelockKeeper.Logger().Info("proposal queued for timelock execution",
		"proposal_id", proposal.Id,
		"operation_id", op.ID,
		"executable_at", op.ExecutableAt,
		"expires_at", op.ExpiresAt,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"proposal_timelocked",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposal.Id)),
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.ID)),
			sdk.NewAttribute("executable_at", op.ExecutableAt.String()),
		),
	)

	return nil
}

// GovKeeperWrapper wraps the standard gov keeper to intercept proposal execution
// This is an alternative approach to hooks for chains that need tighter integration
type GovKeeperWrapper struct {
	govKeeper      interface{} // Standard gov keeper
	timelockKeeper *Keeper
}

// NewGovKeeperWrapper creates a wrapper around the gov keeper
func NewGovKeeperWrapper(govKeeper interface{}, timelockKeeper *Keeper) *GovKeeperWrapper {
	return &GovKeeperWrapper{
		govKeeper:      govKeeper,
		timelockKeeper: timelockKeeper,
	}
}

// ExecuteProposal intercepts proposal execution and queues for timelock
// NOTE: This requires the gov keeper to expose ExecuteProposal or similar method
// The exact implementation depends on your Cosmos SDK version
func (w *GovKeeperWrapper) ExecuteProposal(ctx context.Context, proposal govv1.Proposal) error {
	handler := NewTimelockProposalHandler(w.timelockKeeper)
	return handler.HandleProposal(ctx, proposal)
}
