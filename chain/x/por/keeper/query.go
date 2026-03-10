package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"pos/x/por/types"
)

var _ types.QueryServer = queryServer{}

type queryServer struct {
	Keeper
}

// NewQueryServerImpl returns an implementation of the QueryServer interface
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

// Params returns the module parameters
func (qs queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	params := qs.GetParams(goCtx)
	return &types.QueryParamsResponse{Params: params}, nil
}

// App returns a single registered app by ID
func (qs queryServer) App(goCtx context.Context, req *types.QueryAppRequest) (*types.QueryAppResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	app, found := qs.GetApp(goCtx, req.AppId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "app %d not found", req.AppId)
	}
	return &types.QueryAppResponse{App: app}, nil
}

// Apps returns all registered apps
func (qs queryServer) Apps(goCtx context.Context, req *types.QueryAppsRequest) (*types.QueryAppsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	apps := qs.GetAllApps(goCtx)
	return &types.QueryAppsResponse{Apps: apps}, nil
}

// VerifierSet returns a verifier set by ID
func (qs queryServer) VerifierSet(goCtx context.Context, req *types.QueryVerifierSetRequest) (*types.QueryVerifierSetResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	vs, found := qs.GetVerifierSet(goCtx, req.VerifierSetId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "verifier set %d not found", req.VerifierSetId)
	}
	return &types.QueryVerifierSetResponse{VerifierSet: vs}, nil
}

// Batch returns a single batch commitment by ID
func (qs queryServer) Batch(goCtx context.Context, req *types.QueryBatchRequest) (*types.QueryBatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	batch, found := qs.GetBatch(goCtx, req.BatchId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "batch %d not found", req.BatchId)
	}
	return &types.QueryBatchResponse{Batch: batch}, nil
}

// Batches returns batches with optional status filter
func (qs queryServer) Batches(goCtx context.Context, req *types.QueryBatchesRequest) (*types.QueryBatchesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	var batches []types.BatchCommitment
	if req.Status != nil {
		batches = qs.GetBatchesByStatus(goCtx, *req.Status)
	} else {
		batches = qs.GetAllBatches(goCtx)
	}

	return &types.QueryBatchesResponse{Batches: batches}, nil
}

// BatchesByEpoch returns all batches in a given epoch
func (qs queryServer) BatchesByEpoch(goCtx context.Context, req *types.QueryBatchesByEpochRequest) (*types.QueryBatchesByEpochResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	batches := qs.GetBatchesByEpoch(goCtx, req.Epoch)
	return &types.QueryBatchesByEpochResponse{Batches: batches}, nil
}

// Attestations returns all attestations for a given batch
func (qs queryServer) Attestations(goCtx context.Context, req *types.QueryAttestationsRequest) (*types.QueryAttestationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	attestations := qs.GetAttestationsForBatch(goCtx, req.BatchId)
	return &types.QueryAttestationsResponse{Attestations: attestations}, nil
}

// Challenges returns all challenges for a given batch
func (qs queryServer) Challenges(goCtx context.Context, req *types.QueryChallengesRequest) (*types.QueryChallengesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	challenges := qs.GetChallengesForBatch(goCtx, req.BatchId)
	return &types.QueryChallengesResponse{Challenges: challenges}, nil
}

// VerifierReputation returns the reputation record for a verifier address
func (qs queryServer) VerifierReputation(goCtx context.Context, req *types.QueryVerifierReputationRequest) (*types.QueryVerifierReputationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	rep, found := qs.GetVerifierReputation(goCtx, req.Address)
	if !found {
		return nil, status.Errorf(codes.NotFound, "reputation not found for %s", req.Address)
	}
	return &types.QueryVerifierReputationResponse{Reputation: rep}, nil
}
