package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"pos/x/rewardmult/types"
)

type queryServer struct {
	k Keeper
}

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns a QueryServer implementation
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k: k}
}

// Params returns the module parameters
func (qs queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	return &types.QueryParamsResponse{Params: qs.k.GetParams(ctx)}, nil
}

// ValidatorMultiplierQuery returns the multiplier for a specific validator
func (qs queryServer) ValidatorMultiplierQuery(ctx context.Context, req *types.QueryValidatorMultiplierRequest) (*types.QueryValidatorMultiplierResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.ValidatorAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "validator_address is required")
	}

	vm, found := qs.k.GetValidatorMultiplier(ctx, req.ValidatorAddress)
	if !found {
		return nil, status.Errorf(codes.NotFound, "multiplier not found for validator %s", req.ValidatorAddress)
	}

	return &types.QueryValidatorMultiplierResponse{Multiplier: vm}, nil
}

// AllMultipliers returns all validator multipliers
func (qs queryServer) AllMultipliers(ctx context.Context, req *types.QueryAllMultipliersRequest) (*types.QueryAllMultipliersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	multipliers := qs.k.GetAllValidatorMultipliers(ctx)
	if multipliers == nil {
		multipliers = []types.ValidatorMultiplier{}
	}

	return &types.QueryAllMultipliersResponse{Multipliers: multipliers}, nil
}

// ClampPressure returns clamp pressure telemetry for the latest epoch.
// This is observability-only — it does not affect any logic.
func (qs queryServer) ClampPressure(ctx context.Context, req *types.QueryClampPressureRequest) (*types.QueryClampPressureResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	resp := qs.k.GetClampPressure(ctx)
	return &resp, nil
}

// StakeSnapshot returns the epoch stake snapshots for a given epoch.
func (qs queryServer) StakeSnapshot(ctx context.Context, req *types.QueryStakeSnapshotRequest) (*types.QueryStakeSnapshotResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.Epoch < 0 {
		return nil, status.Error(codes.InvalidArgument, "epoch must be non-negative")
	}

	snapshots := qs.k.GetEpochStakeSnapshots(ctx, req.Epoch)
	if snapshots == nil {
		snapshots = []types.EpochStakeSnapshot{}
	}

	return &types.QueryStakeSnapshotResponse{Snapshots: snapshots}, nil
}
