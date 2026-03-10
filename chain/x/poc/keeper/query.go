package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/types/query"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"pos/x/poc/types"
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

// Contribution returns a single contribution by ID
func (qs queryServer) Contribution(goCtx context.Context, req *types.QueryContributionRequest) (*types.QueryContributionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	contribution, found := qs.GetContribution(goCtx, req.Id)
	if !found {
		return nil, status.Error(codes.NotFound, "contribution not found")
	}

	return &types.QueryContributionResponse{Contribution: contribution}, nil
}

// Contributions returns all contributions with optional filtering
// PERFORMANCE OPTIMIZATION: Added pagination support to prevent loading entire state
func (qs queryServer) Contributions(goCtx context.Context, req *types.QueryContributionsRequest) (*types.QueryContributionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	var contributions []types.Contribution
	var pageRes *query.PageResponse
	var err error

	// Use pagination if provided
	if req.Pagination != nil {
		contributions, pageRes, err = qs.GetContributionsPaginated(goCtx, req)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		// Fallback to non-paginated (legacy support, but limit to 1000)
		count := 0
		const maxNonPaginated = 1000

		err = qs.IterateContributions(goCtx, func(contribution types.Contribution) bool {
			// Apply filters
			if req.Contributor != "" && contribution.Contributor != req.Contributor {
				return false
			}

			if req.Ctype != "" && contribution.Ctype != req.Ctype {
				return false
			}

			if req.Verified >= 0 {
				wantVerified := req.Verified == 1
				if contribution.Verified != wantVerified {
					return false
				}
			}

			contributions = append(contributions, contribution)
			count++

			// Stop if we hit the limit
			return count >= maxNonPaginated
		})

		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &types.QueryContributionsResponse{
		Contributions: contributions,
		Pagination:    pageRes,
	}, nil
}

// Credits returns the credit balance and tier for an address
func (qs queryServer) Credits(goCtx context.Context, req *types.QueryCreditsRequest) (*types.QueryCreditsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

	credits := qs.GetCredits(goCtx, addr)
	tier := qs.GetTier(goCtx, credits.Amount)

	return &types.QueryCreditsResponse{
		Credits: credits,
		Tier:    tier,
	}, nil
}

// FeeMetrics queries the cumulative fee burn statistics
func (qs queryServer) FeeMetrics(goCtx context.Context, req *types.QueryFeeMetricsRequest) (*types.QueryFeeMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	metrics := qs.GetFeeMetrics(goCtx)

	return &types.QueryFeeMetricsResponse{
		Metrics: metrics,
	}, nil
}

// ContributorFeeStats queries fee statistics for a specific contributor
func (qs queryServer) ContributorFeeStats(goCtx context.Context, req *types.QueryContributorFeeStatsRequest) (*types.QueryContributorFeeStatsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

	stats := qs.GetContributorFeeStats(goCtx, addr)

	return &types.QueryContributorFeeStatsResponse{
		Stats: stats,
	}, nil
}

// ============================================================================
// Provenance Registry Queries (Layer 5)
// ============================================================================

// ProvenanceEntry returns a single provenance entry by claim ID
func (qs queryServer) ProvenanceEntry(goCtx context.Context, req *types.QueryProvenanceEntryRequest) (*types.QueryProvenanceEntryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	entry, found := qs.GetProvenanceEntry(goCtx, req.ClaimId)
	if !found {
		return nil, status.Error(codes.NotFound, "provenance entry not found")
	}

	return &types.QueryProvenanceEntryResponse{Entry: entry}, nil
}

// ProvenanceChildren returns all child claims of a parent
func (qs queryServer) ProvenanceChildren(goCtx context.Context, req *types.QueryProvenanceChildrenRequest) (*types.QueryProvenanceChildrenResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	childIDs := qs.GetProvenanceChildren(goCtx, req.ParentClaimId)

	var entries []types.ProvenanceEntry
	for _, id := range childIDs {
		if entry, found := qs.GetProvenanceEntry(goCtx, id); found {
			entries = append(entries, entry)
		}
	}

	return &types.QueryProvenanceChildrenResponse{
		ChildClaimIds: childIDs,
		Entries:       entries,
	}, nil
}

// ProvenanceLineage computes the full lineage path from root to a claim
func (qs queryServer) ProvenanceLineage(goCtx context.Context, req *types.QueryProvenanceLineageRequest) (*types.QueryProvenanceLineageResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	path, err := qs.ComputeLineagePath(goCtx, req.ClaimId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &types.QueryProvenanceLineageResponse{Path: path}, nil
}

// ProvenanceByHash returns all provenance entries matching a canonical hash
func (qs queryServer) ProvenanceByHash(goCtx context.Context, req *types.QueryProvenanceByHashRequest) (*types.QueryProvenanceByHashResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if len(req.CanonicalHash) == 0 {
		return nil, status.Error(codes.InvalidArgument, "canonical_hash is required")
	}

	claimIDs := qs.GetProvenanceByHash(goCtx, req.CanonicalHash)

	var entries []types.ProvenanceEntry
	for _, id := range claimIDs {
		if entry, found := qs.GetProvenanceEntry(goCtx, id); found {
			entries = append(entries, entry)
		}
	}

	return &types.QueryProvenanceByHashResponse{Entries: entries}, nil
}

// ProvenanceBySubmitter returns all provenance entries for a submitter address
func (qs queryServer) ProvenanceBySubmitter(goCtx context.Context, req *types.QueryProvenanceBySubmitterRequest) (*types.QueryProvenanceBySubmitterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if req.Submitter == "" {
		return nil, status.Error(codes.InvalidArgument, "submitter is required")
	}

	claimIDs := qs.GetProvenanceBySubmitter(goCtx, req.Submitter)

	var entries []types.ProvenanceEntry
	for _, id := range claimIDs {
		if entry, found := qs.GetProvenanceEntry(goCtx, id); found {
			entries = append(entries, entry)
		}
	}

	return &types.QueryProvenanceBySubmitterResponse{Entries: entries}, nil
}

// ProvenanceStats returns aggregate provenance registry statistics
func (qs queryServer) ProvenanceStats(goCtx context.Context, req *types.QueryProvenanceStatsRequest) (*types.QueryProvenanceStatsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	stats := qs.Keeper.GetProvenanceStats(goCtx)

	return &types.QueryProvenanceStatsResponse{Stats: stats}, nil
}
