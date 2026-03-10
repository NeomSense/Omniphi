package keeper

import (
	"context"

	"pos/x/repgov/types"
)

type queryServer struct {
	k Keeper
}

// NewQueryServerImpl returns an implementation of the QueryServer interface
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{k: k}
}

func (qs *queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return &types.QueryParamsResponse{Params: qs.k.GetParams(ctx)}, nil
}

func (qs *queryServer) VoterWeight(ctx context.Context, req *types.QueryVoterWeightRequest) (*types.QueryVoterWeightResponse, error) {
	vw, found := qs.k.GetVoterWeight(ctx, req.Address)
	if !found {
		return nil, types.ErrVoterWeightNotFound
	}
	return &types.QueryVoterWeightResponse{Weight: vw}, nil
}

func (qs *queryServer) AllVoterWeights(ctx context.Context, req *types.QueryAllVoterWeightsRequest) (*types.QueryAllVoterWeightsResponse, error) {
	return &types.QueryAllVoterWeightsResponse{Weights: qs.k.GetAllVoterWeights(ctx)}, nil
}

func (qs *queryServer) Delegations(ctx context.Context, req *types.QueryDelegationsRequest) (*types.QueryDelegationsResponse, error) {
	return &types.QueryDelegationsResponse{Delegations: qs.k.GetDelegationsFrom(ctx, req.Address)}, nil
}

func (qs *queryServer) TallyOverride(ctx context.Context, req *types.QueryTallyOverrideRequest) (*types.QueryTallyOverrideResponse, error) {
	to, found := qs.k.GetTallyOverride(ctx, req.ProposalID)
	if !found {
		return nil, types.ErrInvalidProposalID
	}
	return &types.QueryTallyOverrideResponse{Override: to}, nil
}
