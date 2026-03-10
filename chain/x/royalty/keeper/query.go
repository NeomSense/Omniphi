package keeper

import (
	"context"

	"pos/x/royalty/types"
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

func (qs *queryServer) RoyaltyToken(ctx context.Context, req *types.QueryRoyaltyTokenRequest) (*types.QueryRoyaltyTokenResponse, error) {
	token, found := qs.k.GetRoyaltyToken(ctx, req.TokenID)
	if !found {
		return nil, types.ErrTokenNotFound
	}
	return &types.QueryRoyaltyTokenResponse{Token: token}, nil
}

func (qs *queryServer) TokensByOwner(ctx context.Context, req *types.QueryTokensByOwnerRequest) (*types.QueryTokensByOwnerResponse, error) {
	return &types.QueryTokensByOwnerResponse{Tokens: qs.k.GetTokensByOwner(ctx, req.Owner)}, nil
}

func (qs *queryServer) TokensByClaim(ctx context.Context, req *types.QueryTokensByClaimRequest) (*types.QueryTokensByClaimResponse, error) {
	return &types.QueryTokensByClaimResponse{Tokens: qs.k.GetTokensByClaim(ctx, req.ClaimID)}, nil
}

func (qs *queryServer) AccumulatedRoyalties(ctx context.Context, req *types.QueryAccumulatedRoyaltiesRequest) (*types.QueryAccumulatedRoyaltiesResponse, error) {
	return &types.QueryAccumulatedRoyaltiesResponse{Amount: qs.k.GetAccumulatedRoyalty(ctx, req.TokenID)}, nil
}
