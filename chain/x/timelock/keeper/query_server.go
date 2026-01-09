package keeper

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/types/query"

	"pos/x/timelock/types"
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
func (qs queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	params, err := qs.Keeper.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// Operation returns a specific operation by ID
func (qs queryServer) Operation(ctx context.Context, req *types.QueryOperationRequest) (*types.QueryOperationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	op, err := qs.Keeper.GetOperation(ctx, req.OperationId)
	if err != nil {
		return nil, err
	}

	return &types.QueryOperationResponse{
		Operation: op,
	}, nil
}

// Operations returns all operations with optional status filter
func (qs queryServer) Operations(ctx context.Context, req *types.QueryOperationsRequest) (*types.QueryOperationsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	var ops []types.QueuedOperation

	err := qs.Keeper.Operations.Walk(ctx, nil, func(id uint64, op types.QueuedOperation) (stop bool, err error) {
		// Apply status filter if specified
		if req.Status != types.OperationStatusUnspecified && op.Status != req.Status {
			return false, nil
		}
		ops = append(ops, op)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	// Apply pagination (simplified - in production use collections pagination)
	start, end := 0, len(ops)
	if req.Pagination != nil {
		if req.Pagination.Offset > 0 {
			start = int(req.Pagination.Offset)
		}
		if req.Pagination.Limit > 0 && start+int(req.Pagination.Limit) < end {
			end = start + int(req.Pagination.Limit)
		}
	}
	if start > len(ops) {
		start = len(ops)
	}
	if end > len(ops) {
		end = len(ops)
	}

	return &types.QueryOperationsResponse{
		Operations: ops[start:end],
		Pagination: &query.PageResponse{
			Total: uint64(len(ops)),
		},
	}, nil
}

// QueuedOperations returns all operations in QUEUED status
func (qs queryServer) QueuedOperations(ctx context.Context, req *types.QueryQueuedOperationsRequest) (*types.QueryQueuedOperationsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	ops, err := qs.Keeper.GetQueuedOperations(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to non-pointer slice
	result := make([]types.QueuedOperation, len(ops))
	for i, op := range ops {
		result[i] = *op
	}

	return &types.QueryQueuedOperationsResponse{
		Operations: result,
		Pagination: &query.PageResponse{
			Total: uint64(len(ops)),
		},
	}, nil
}

// ExecutableOperations returns operations ready for execution
func (qs queryServer) ExecutableOperations(ctx context.Context, req *types.QueryExecutableOperationsRequest) (*types.QueryExecutableOperationsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	ops, err := qs.Keeper.GetExecutableOperations(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to non-pointer slice
	result := make([]types.QueuedOperation, len(ops))
	for i, op := range ops {
		result[i] = *op
	}

	return &types.QueryExecutableOperationsResponse{
		Operations: result,
		Pagination: &query.PageResponse{
			Total: uint64(len(ops)),
		},
	}, nil
}

// OperationByHash returns an operation by its hash
func (qs queryServer) OperationByHash(ctx context.Context, req *types.QueryOperationByHashRequest) (*types.QueryOperationByHashResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Decode hex hash
	hash, err := hex.DecodeString(req.Hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash format: %w", err)
	}

	op, err := qs.Keeper.GetOperationByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	return &types.QueryOperationByHashResponse{
		Operation: op,
	}, nil
}

// OperationsByProposal returns all operations for a governance proposal
func (qs queryServer) OperationsByProposal(ctx context.Context, req *types.QueryOperationsByProposalRequest) (*types.QueryOperationsByProposalResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	ops, err := qs.Keeper.GetOperationsByProposal(ctx, req.ProposalId)
	if err != nil {
		return nil, err
	}

	// Convert to non-pointer slice
	result := make([]types.QueuedOperation, len(ops))
	for i, op := range ops {
		result[i] = *op
	}

	return &types.QueryOperationsByProposalResponse{
		Operations: result,
	}, nil
}
