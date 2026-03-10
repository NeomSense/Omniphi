package keeper

import (
	"context"

	"pos/x/uci/types"
)

type queryServer struct {
	k Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{k: k}
}

func (qs *queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return &types.QueryParamsResponse{Params: qs.k.GetParams(ctx)}, nil
}

func (qs *queryServer) Adapter(ctx context.Context, req *types.QueryAdapterRequest) (*types.QueryAdapterResponse, error) {
	adapter, found := qs.k.GetAdapter(ctx, req.AdapterID)
	if !found {
		return nil, types.ErrAdapterNotFound
	}
	return &types.QueryAdapterResponse{Adapter: adapter}, nil
}

func (qs *queryServer) AdaptersByOwner(ctx context.Context, req *types.QueryAdaptersByOwnerRequest) (*types.QueryAdaptersByOwnerResponse, error) {
	return &types.QueryAdaptersByOwnerResponse{Adapters: qs.k.GetAdaptersByOwner(ctx, req.Owner)}, nil
}

func (qs *queryServer) AllAdapters(ctx context.Context, req *types.QueryAllAdaptersRequest) (*types.QueryAllAdaptersResponse, error) {
	return &types.QueryAllAdaptersResponse{Adapters: qs.k.GetAllAdapters(ctx)}, nil
}

func (qs *queryServer) ContributionMapping(ctx context.Context, req *types.QueryContributionMappingRequest) (*types.QueryContributionMappingResponse, error) {
	cm, found := qs.k.GetContributionMapping(ctx, req.AdapterID, req.ExternalID)
	if !found {
		return nil, types.ErrMappingNotFound
	}
	return &types.QueryContributionMappingResponse{Mapping: cm}, nil
}

func (qs *queryServer) AdapterStats(ctx context.Context, req *types.QueryAdapterStatsRequest) (*types.QueryAdapterStatsResponse, error) {
	stats := qs.k.GetAdapterStats(ctx, req.AdapterID)
	return &types.QueryAdapterStatsResponse{Stats: stats}, nil
}
