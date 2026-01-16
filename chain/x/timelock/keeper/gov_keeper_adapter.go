package keeper

import (
	"context"

	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// GovKeeperAdapter adapts the standard gov keeper to implement GovKeeperI interface
// This allows timelock to access proposals through a stable interface
type GovKeeperAdapter struct {
	keeper *govkeeper.Keeper
}

// NewGovKeeperAdapter creates a new adapter for the gov keeper
func NewGovKeeperAdapter(keeper *govkeeper.Keeper) *GovKeeperAdapter {
	return &GovKeeperAdapter{keeper: keeper}
}

// GetProposal retrieves a proposal by ID from the gov keeper's Proposals collection
func (a *GovKeeperAdapter) GetProposal(ctx context.Context, proposalID uint64) (govv1.Proposal, error) {
	return a.keeper.Proposals.Get(ctx, proposalID)
}

// SetProposal stores a proposal in the gov keeper's Proposals collection
func (a *GovKeeperAdapter) SetProposal(ctx context.Context, proposal govv1.Proposal) error {
	return a.keeper.Proposals.Set(ctx, proposal.Id, proposal)
}

// DeleteProposal removes a proposal from the gov keeper's Proposals collection
func (a *GovKeeperAdapter) DeleteProposal(ctx context.Context, proposalID uint64) error {
	return a.keeper.Proposals.Remove(ctx, proposalID)
}
