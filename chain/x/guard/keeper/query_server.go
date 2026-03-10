package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/guard/types"
)

// GuardStatus is a consolidated view of the guard module's current state.
// This is returned by GetGuardStatus() and is useful for dashboards and monitoring.
type GuardStatus struct {
	CurrentHeight        int64  `json:"current_height"`
	LastProcessedID      uint64 `json:"last_processed_proposal_id"`
	QueuedCount          uint64 `json:"queued_count"`
	PendingCount         uint64 `json:"pending_count"`   // non-terminal, non-executed
	ExecutedCount        uint64 `json:"executed_count"`
	AbortedCount         uint64 `json:"aborted_count"`
	MaxProposalsPerBlock uint64 `json:"max_proposals_per_block"`
	MaxQueueScanDepth    uint64 `json:"max_queue_scan_depth"`
}

// GetGuardStatus returns a consolidated status snapshot of the guard module.
func (k Keeper) GetGuardStatus(ctx context.Context) GuardStatus {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	status := GuardStatus{
		CurrentHeight:        sdkCtx.BlockHeight(),
		LastProcessedID:      k.GetLastProcessedProposalID(ctx),
		MaxProposalsPerBlock: params.MaxProposalsPerBlock,
		MaxQueueScanDepth:    params.MaxQueueScanDepth,
	}

	// Count queued executions by state
	store := k.storeService.OpenKVStore(ctx)
	startKey := types.QueuedExecutionPrefix
	endKey := types.PrefixEnd(types.QueuedExecutionPrefix)

	iterator, err := store.Iterator(startKey, endKey)
	if err != nil {
		return status
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		bz := iterator.Value()
		var exec types.QueuedExecution
		k.cdc.MustUnmarshal(bz, &exec)

		status.QueuedCount++
		switch {
		case exec.GateState == types.EXECUTION_GATE_EXECUTED:
			status.ExecutedCount++
		case exec.GateState == types.EXECUTION_GATE_ABORTED:
			status.AbortedCount++
		default:
			status.PendingCount++
		}
	}

	return status
}

type queryServer struct {
	types.UnimplementedQueryServer
	Keeper
}

// NewQueryServerImpl returns an implementation of the QueryServer interface
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = queryServer{}

// Params returns the module parameters
func (qs queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params := qs.GetParams(goCtx)
	return &types.QueryParamsResponse{Params: &params}, nil
}

// RiskReport returns the risk report for a proposal
func (qs queryServer) RiskReport(goCtx context.Context, req *types.QueryRiskReportRequest) (*types.QueryRiskReportResponse, error) {
	if req.ProposalId == 0 {
		return nil, types.ErrInvalidProposalID
	}

	report, found := qs.GetRiskReport(goCtx, req.ProposalId)
	if !found {
		return nil, types.ErrRiskReportNotFound.Wrapf("proposal_id: %d", req.ProposalId)
	}

	return &types.QueryRiskReportResponse{Report: &report}, nil
}

// QueuedExecution returns the queued execution state for a proposal
func (qs queryServer) QueuedExecution(goCtx context.Context, req *types.QueryQueuedExecutionRequest) (*types.QueryQueuedExecutionResponse, error) {
	if req.ProposalId == 0 {
		return nil, types.ErrInvalidProposalID
	}

	exec, found := qs.GetQueuedExecution(goCtx, req.ProposalId)
	if !found {
		return nil, types.ErrQueuedExecutionNotFound.Wrapf("proposal_id: %d", req.ProposalId)
	}

	return &types.QueryQueuedExecutionResponse{Execution: &exec}, nil
}

// AdvisoryLink returns the advisory link for a proposal
func (qs queryServer) AdvisoryLink(goCtx context.Context, req *types.QueryAdvisoryLinkRequest) (*types.QueryAdvisoryLinkResponse, error) {
	if req.ProposalId == 0 {
		return nil, types.ErrInvalidProposalID
	}

	link, found := qs.GetAdvisoryLink(goCtx, req.ProposalId)
	if !found {
		return nil, fmt.Errorf("advisory link not found for proposal %d", req.ProposalId)
	}

	return &types.QueryAdvisoryLinkResponse{Link: &link}, nil
}
