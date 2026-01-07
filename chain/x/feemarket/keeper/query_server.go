package keeper

import (
	"context"

	"pos/x/feemarket/types"
)

var _ types.QueryServer = queryServer{}

// queryServer implements the Query gRPC service
type queryServer struct {
	types.UnimplementedQueryServer
	k Keeper
}

// NewQueryServer creates a new gRPC query server
func NewQueryServer(keeper Keeper) types.QueryServer {
	return queryServer{k: keeper}
}

// Params returns the current module parameters
func (qs queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, types.ErrInvalidParams.Wrap("empty request")
	}

	params := qs.k.GetParams(ctx)

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// BaseFee returns the current base fee and effective gas price
func (qs queryServer) BaseFee(ctx context.Context, req *types.QueryBaseFeeRequest) (*types.QueryBaseFeeResponse, error) {
	if req == nil {
		return nil, types.ErrInvalidBaseFee.Wrap("empty request")
	}

	baseFee := qs.k.GetCurrentBaseFee(ctx)
	effectiveGasPrice := qs.k.GetEffectiveGasPrice(ctx)
	params := qs.k.GetParams(ctx)

	return &types.QueryBaseFeeResponse{
		BaseFee:           baseFee,
		MinGasPrice:       params.MinGasPrice,
		EffectiveGasPrice: effectiveGasPrice,
	}, nil
}

// BlockUtilization returns current and previous block utilization metrics
func (qs queryServer) BlockUtilization(ctx context.Context, req *types.QueryBlockUtilizationRequest) (*types.QueryBlockUtilizationResponse, error) {
	if req == nil {
		return nil, types.ErrInvalidUtilization.Wrap("empty request")
	}

	currentUtilization := qs.k.GetBlockUtilization(ctx)
	params := qs.k.GetParams(ctx)

	// Get stored block gas values from previous block
	blockGasUsed := qs.k.GetPreviousBlockGasUsed(ctx)
	maxBlockGas := qs.k.GetMaxBlockGas(ctx)

	return &types.QueryBlockUtilizationResponse{
		Utilization:       currentUtilization,
		BlockGasUsed:      blockGasUsed,
		MaxBlockGas:       maxBlockGas,
		TargetUtilization: params.TargetBlockUtilization,
	}, nil
}

// BurnTier returns the current burn tier and percentage
func (qs queryServer) BurnTier(ctx context.Context, req *types.QueryBurnTierRequest) (*types.QueryBurnTierResponse, error) {
	if req == nil {
		return nil, types.ErrInvalidBurnRatio.Wrap("empty request")
	}

	burnRate, tierName := qs.k.SelectBurnTier(ctx)
	utilization := qs.k.GetBlockUtilization(ctx)

	return &types.QueryBurnTierResponse{
		Tier:           tierName,
		BurnPercentage: burnRate,
		Utilization:    utilization,
	}, nil
}

// FeeStats returns cumulative fee statistics
func (qs queryServer) FeeStats(ctx context.Context, req *types.QueryFeeStatsRequest) (*types.QueryFeeStatsResponse, error) {
	if req == nil {
		return nil, types.ErrInvalidParams.Wrap("empty request")
	}

	totalBurned := qs.k.GetCumulativeBurned(ctx)
	totalToTreasury := qs.k.GetCumulativeToTreasury(ctx)
	totalToValidators := qs.k.GetCumulativeToValidators(ctx)

	// Calculate total fees processed (all three components)
	totalFeesProcessed := totalBurned.
		Add(totalToTreasury).
		Add(totalToValidators)

	// Get treasury address from keeper
	treasuryAddr := ""
	params := qs.k.GetParams(ctx)
	if params.TreasuryFeeRatio.IsPositive() {
		// Treasury address would be stored in genesis/params
		treasuryAddr = "" // TODO: Get from genesis or params
	}

	return &types.QueryFeeStatsResponse{
		TotalBurned:        totalBurned,
		TotalToTreasury:    totalToTreasury,
		TotalToValidators:  totalToValidators,
		TotalFeesProcessed: totalFeesProcessed,
		TreasuryAddress:    treasuryAddr,
	}, nil
}
