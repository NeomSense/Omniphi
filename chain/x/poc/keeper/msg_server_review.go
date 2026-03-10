package keeper

import (
	"context"

	"pos/x/poc/types"
)

// StartReview handles initiating a human review session for a contribution.
func (ms msgServer) StartReview(goCtx context.Context, msg *types.MsgStartReview) (*types.MsgStartReviewResponse, error) {
	return ms.ProcessStartReview(goCtx, msg)
}

// CastReviewVote handles an assigned reviewer casting their vote.
func (ms msgServer) CastReviewVote(goCtx context.Context, msg *types.MsgCastReviewVote) (*types.MsgCastReviewVoteResponse, error) {
	return ms.ProcessCastReviewVote(goCtx, msg)
}

// FinalizeReview handles finalizing a review session after voting period.
func (ms msgServer) FinalizeReview(goCtx context.Context, msg *types.MsgFinalizeReview) (*types.MsgFinalizeReviewResponse, error) {
	return ms.ProcessFinalizeReview(goCtx, msg)
}

// AppealReview handles filing an appeal against a review decision.
func (ms msgServer) AppealReview(goCtx context.Context, msg *types.MsgAppealReview) (*types.MsgAppealReviewResponse, error) {
	return ms.ProcessAppealReview(goCtx, msg)
}

// ResolveAppeal handles governance resolving an appeal.
func (ms msgServer) ResolveAppeal(goCtx context.Context, msg *types.MsgResolveAppeal) (*types.MsgResolveAppealResponse, error) {
	return ms.ProcessResolveAppeal(goCtx, msg)
}
