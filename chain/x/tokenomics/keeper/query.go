package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	"pos/x/tokenomics/types"
)

// queryServer is a wrapper around the Keeper that implements the QueryServer interface
type queryServer struct {
	types.UnimplementedQueryServer
	Keeper
}

// NewQueryServerImpl returns an implementation of the QueryServer interface
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = queryServer{}

// Params returns the tokenomics module parameters
// DASH-001: Dashboard support
func (qs queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// Supply returns comprehensive supply metrics
// DASH-001: Global supply chart
// P0-ACCT-001: Supply accounting verification
func (qs queryServer) Supply(goCtx context.Context, req *types.QuerySupplyRequest) (*types.QuerySupplyResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	metrics := qs.GetSupplyMetrics(ctx)
	return &metrics, nil
}

// Inflation returns current inflation metrics
// DASH-002: Inflation tracking
func (qs queryServer) Inflation(goCtx context.Context, req *types.QueryInflationRequest) (*types.QueryInflationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	// Calculate annual provisions
	annualProvisions := qs.CalculateAnnualProvisions(ctx)

	// Calculate block provisions
	blockProvisions := qs.CalculateBlockProvisions(ctx)

	// Estimate blocks per year (7 second blocks)
	const blocksPerYear uint64 = 4500857 // 365.25 * 24 * 3600 / 7

	return &types.QueryInflationResponse{
		CurrentInflationRate: params.InflationRate,
		InflationMin:         params.InflationMin,
		InflationMax:         params.InflationMax,
		AnnualProvisions:     annualProvisions,
		BlockProvisions:      blockProvisions,
		BlocksPerYear:        blocksPerYear,
	}, nil
}

// Emissions returns emission distribution metrics
// DASH-003: Epoch reward flows
func (qs queryServer) Emissions(goCtx context.Context, req *types.QueryEmissionsRequest) (*types.QueryEmissionsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	// Calculate total annual emissions
	totalAnnualEmissions := qs.CalculateAnnualProvisions(ctx)

	// Calculate per-category allocations
	allocations := []types.EmissionAllocation{
		{
			Category:   "staking",
			Percentage: params.EmissionSplitStaking,
			AnnualAmount: params.EmissionSplitStaking.MulInt(totalAnnualEmissions).TruncateInt(),
			TotalDistributed: math.ZeroInt(), // TODO: Track cumulative distributions
		},
		{
			Category:   "poc",
			Percentage: params.EmissionSplitPoc,
			AnnualAmount: params.EmissionSplitPoc.MulInt(totalAnnualEmissions).TruncateInt(),
			TotalDistributed: math.ZeroInt(),
		},
		{
			Category:   "sequencer",
			Percentage: params.EmissionSplitSequencer,
			AnnualAmount: params.EmissionSplitSequencer.MulInt(totalAnnualEmissions).TruncateInt(),
			TotalDistributed: math.ZeroInt(),
		},
		{
			Category:   "treasury",
			Percentage: params.EmissionSplitTreasury,
			AnnualAmount: params.EmissionSplitTreasury.MulInt(totalAnnualEmissions).TruncateInt(),
			TotalDistributed: math.ZeroInt(),
		},
	}

	return &types.QueryEmissionsResponse{
		Allocations:          allocations,
		TotalAnnualEmissions: totalAnnualEmissions,
		LastDistributionHeight: 0, // TODO: Track last distribution
	}, nil
}

// Burns returns paginated burn history
// DASH-001: Per-chain burn breakdown
// P0-BURN-005: Burn records queryable
func (qs queryServer) Burns(goCtx context.Context, req *types.QueryBurnsRequest) (*types.QueryBurnsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	_ = qs.storeService.OpenKVStore(ctx) // Store for future pagination implementation

	// Default pagination if not provided
	pageReq := req.Pagination
	if pageReq == nil {
		pageReq = &query.PageRequest{
			Limit: 100,
		}
	}

	var burns []types.BurnRecord
	var pageRes *query.PageResponse

	// Iterate through burn records with pagination
	// Note: This is a simplified implementation
	// In production, would use proper iterator with prefix scanning
	burnID := uint64(1)
	maxID := qs.GetNextBurnID(ctx)
	var collected uint64

	for burnID < maxID && collected < pageReq.Limit {
		if record, found := qs.GetBurnRecord(ctx, burnID); found {
			burns = append(burns, record)
			collected++
		}
		burnID++
	}

	pageRes = &query.PageResponse{
		Total: maxID - 1, // Total burn records
	}

	totalBurned := qs.GetTotalBurned(ctx)

	return &types.QueryBurnsResponse{
		Burns:       burns,
		Pagination:  pageRes,
		TotalBurned: totalBurned,
	}, nil
}

// BurnsBySource returns burns filtered by source
// DASH-002: Per-source burn statistics
// P0-BURN-006: Per-source burn rates respected
func (qs queryServer) BurnsBySource(goCtx context.Context, req *types.QueryBurnsBySourceRequest) (*types.QueryBurnsBySourceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	// Get total burns for this source
	totalAmount := qs.GetBurnsBySource(ctx, req.Source)

	// Get burn rate for this source
	var currentBurnRate math.LegacyDec
	switch req.Source {
	case types.BurnSource_BURN_SOURCE_POS_GAS:
		currentBurnRate = params.BurnRatePosGas
	case types.BurnSource_BURN_SOURCE_POC_ANCHORING:
		currentBurnRate = params.BurnRatePocAnchoring
	case types.BurnSource_BURN_SOURCE_SEQUENCER_GAS:
		currentBurnRate = params.BurnRateSequencerGas
	case types.BurnSource_BURN_SOURCE_SMART_CONTRACTS:
		currentBurnRate = params.BurnRateSmartContracts
	case types.BurnSource_BURN_SOURCE_AI_QUERIES:
		currentBurnRate = params.BurnRateAiQueries
	case types.BurnSource_BURN_SOURCE_MESSAGING:
		currentBurnRate = params.BurnRateMessaging
	default:
		currentBurnRate = math.LegacyZeroDec()
	}

	// Create statistics
	stats := types.BurnsBySourceStats{
		Source:           req.Source,
		TotalAmount:      totalAmount,
		BurnCount:        0, // TODO: Track count separately
		CurrentBurnRate:  currentBurnRate,
		AverageBurnAmount: math.LegacyZeroDec(), // TODO: Calculate from count
	}

	// Note: In full implementation, would iterate burn records to collect matching burns
	// For now, returning empty list with aggregated stats
	var burns []types.BurnRecord

	return &types.QueryBurnsBySourceResponse{
		Burns:      burns,
		Pagination: nil,
		Stats:      stats,
	}, nil
}

// BurnsByChain returns burns filtered by chain
// DASH-002: Per-chain burn breakdown
// P0-BURN-008: Cross-chain burn tracking
func (qs queryServer) BurnsByChain(goCtx context.Context, req *types.QueryBurnsByChainRequest) (*types.QueryBurnsByChainResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Get total burns for this chain
	totalBurnedOnChain := qs.GetBurnsByChain(ctx, req.ChainId)

	// Note: In full implementation, would iterate burn records to collect matching burns
	// For now, returning empty list with aggregated total
	var burns []types.BurnRecord

	return &types.QueryBurnsByChainResponse{
		Burns:              burns,
		Pagination:         nil,
		TotalBurnedOnChain: totalBurnedOnChain,
	}, nil
}

// Treasury returns DAO treasury status
// DASH-001: Treasury tracking
// P1-TREAS-001: Treasury balance integrity
func (qs queryServer) Treasury(goCtx context.Context, req *types.QueryTreasuryRequest) (*types.QueryTreasuryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	// Get treasury address
	treasuryAddr := qs.GetTreasuryAddress(ctx)

	// Get current balance
	balance := qs.bankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom)

	// Get total inflows
	store := qs.storeService.OpenKVStore(ctx)
	var totalInflows math.Int
	bz, err := store.Get(types.KeyTreasuryInflows)
	if err == nil && bz != nil {
		_ = totalInflows.Unmarshal(bz)
	} else {
		totalInflows = math.ZeroInt()
	}

	return &types.QueryTreasuryResponse{
		TreasuryBalance:         balance.Amount,
		TotalTreasuryInflows:    totalInflows,
		FromInflation:           math.ZeroInt(), // TODO: Track separately
		FromBurnRedirect:        totalInflows,   // Simplified: assume all from redirects
		TreasuryBurnRedirectPct: params.TreasuryBurnRedirect,
		TreasuryAddress:         treasuryAddr.String(),
	}, nil
}

// Projections returns multi-year supply forecasts
// DASH-005: Projections dashboard
func (qs queryServer) Projections(goCtx context.Context, req *types.QueryProjectionsRequest) (*types.QueryProjectionsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)
	currentSupply := qs.GetCurrentSupply(ctx)
	currentMinted := qs.GetTotalMinted(ctx)
	currentBurned := qs.GetTotalBurned(ctx)

	// Validate years_ahead
	yearsAhead := req.YearsAhead
	if yearsAhead == 0 {
		yearsAhead = 5 // Default to 5 years
	}
	if yearsAhead > 10 {
		yearsAhead = 10 // Cap at 10 years
	}

	// Calculate net inflation rate (minting - burning)
	netInflationRate := qs.CalculateNetInflationRate(ctx)

	var projections []types.SupplyProjection

	// Project supply for each year
	projectedSupply := currentSupply
	projectedMinted := currentMinted
	projectedBurned := currentBurned

	for year := uint32(1); year <= yearsAhead; year++ {
		// Calculate annual minting
		annualMinting := params.InflationRate.MulInt(projectedSupply).TruncateInt()
		projectedMinted = projectedMinted.Add(annualMinting)

		// Estimate annual burning (assuming constant burn rate)
		avgBurnRate := params.BurnRatePosGas.
			Add(params.BurnRatePocAnchoring).
			Add(params.BurnRateSequencerGas).
			QuoInt64(3)
		annualBurning := avgBurnRate.MulInt(annualMinting).TruncateInt()
		projectedBurned = projectedBurned.Add(annualBurning)

		// Net supply change
		netChange := annualMinting.Sub(annualBurning)
		projectedSupply = projectedSupply.Add(netChange)

		// Check if approaching cap
		var yearsUntilCap uint32
		if projectedSupply.LT(params.TotalSupplyCap) {
			remaining := params.TotalSupplyCap.Sub(projectedSupply)
			if netChange.IsPositive() {
				yearsUntilCap = uint32(math.LegacyNewDecFromInt(remaining).Quo(math.LegacyNewDecFromInt(netChange)).TruncateInt64())
			}
		}

		projections = append(projections, types.SupplyProjection{
			Year:             year,
			ProjectedSupply:  projectedSupply,
			ProjectedMinted:  projectedMinted,
			ProjectedBurned:  projectedBurned,
			NetGrowthRate:    netInflationRate,
			YearsUntilCap:    yearsUntilCap,
		})
	}

	assumptions := fmt.Sprintf(
		"Assumptions: Inflation=%s%%, AvgBurnRate=%s%%, NetGrowth=%s%%",
		params.InflationRate.MulInt64(100).String(),
		(params.BurnRatePosGas.Add(params.BurnRatePocAnchoring).Add(params.BurnRateSequencerGas).QuoInt64(3)).MulInt64(100).String(),
		netInflationRate.MulInt64(100).String(),
	)

	return &types.QueryProjectionsResponse{
		Projections: projections,
		Assumptions: assumptions,
	}, nil
}

// ChainMetrics returns per-chain tokenomics metrics
// DASH-002: Per-chain statistics
func (qs queryServer) ChainMetrics(goCtx context.Context, req *types.QueryChainMetricsRequest) (*types.QueryChainMetricsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	// Get chain-specific metrics
	totalBurned := qs.GetBurnsByChain(ctx, req.ChainId)

	// Determine IBC channel and gas conversion ratio
	var ibcChannel string
	var gasConversionRatio math.LegacyDec

	switch req.ChainId {
	case "omniphi-continuity-1":
		ibcChannel = params.ContinuityIbcChannel
		gasConversionRatio = params.GasConversionRatioContinuity
	case "omniphi-sequencer-1":
		ibcChannel = params.SequencerIbcChannel
		gasConversionRatio = params.GasConversionRatioSequencer
	default:
		ibcChannel = ""
		gasConversionRatio = math.LegacyOneDec() // Core chain = 1.0x
	}

	return &types.QueryChainMetricsResponse{
		ChainId:                req.ChainId,
		TotalBurned:            totalBurned,
		TotalRewardsReceived:   math.ZeroInt(), // TODO: Track IBC rewards received
		NetFlow:                math.ZeroInt(), // TODO: Calculate net flow
		IbcChannel:             ibcChannel,
		GasConversionRatio:     gasConversionRatio,
		LastRewardHeight:       0, // TODO: Track last reward
		LastBurnReportHeight:   0, // TODO: Track last burn report
	}, nil
}

// FeeStats returns fee burn statistics (90/10 mechanism)
func (qs queryServer) FeeStats(goCtx context.Context, req *types.QueryFeeStatsRequest) (*types.QueryFeeStatsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	return &types.QueryFeeStatsResponse{
		TotalFeesBurned:           qs.GetTotalFeesBurned(ctx),
		TotalFeesToTreasury:       qs.GetTotalFeesToTreasury(ctx),
		AverageFeesBurnedPerBlock: qs.GetAverageFeesBurnedPerBlock(ctx),
		FeeBurnEnabled:            params.FeeBurnEnabled,
		FeeBurnRatio:              params.FeeBurnRatio,
		TreasuryFeeRatio:          params.TreasuryFeeRatio,
	}, nil
}

// BurnRate returns the current adaptive burn rate and all related metrics
// ADAPTIVE-BURN: Dashboard support for adaptive burn controller
func (qs queryServer) BurnRate(goCtx context.Context, req *types.QueryBurnRateRequest) (*types.QueryBurnRateResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)

	// Get current burn ratio and trigger
	currentRatio, trigger := qs.GetAdaptiveBurnRatio(ctx)

	// Get network metrics
	congestion := qs.GetBlockCongestion(ctx)
	treasuryPct := qs.GetTreasuryPct(ctx)
	avgTxPerDay := qs.GetAvgTxPerDay(ctx)

	return &types.QueryBurnRateResponse{
		AdaptiveBurnEnabled:     params.AdaptiveBurnEnabled,
		CurrentBurnRatio:        currentRatio,
		Trigger:                 trigger,
		MinBurnRatio:            params.MinBurnRatio,
		MaxBurnRatio:            params.MaxBurnRatio,
		DefaultBurnRatio:        params.DefaultBurnRatio,
		BlockCongestion:         congestion,
		TreasuryPct:             treasuryPct,
		AvgTxPerDay:             avgTxPerDay,
		EmergencyBurnOverride:   params.EmergencyBurnOverride,
	}, nil
}
