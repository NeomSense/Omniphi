package keeper

import (
	"context"

	"pos/x/poc/types"
)

// SubmitSimilarityCommitment handles oracle-signed similarity commitment submissions.
// This delegates to ProcessSimilarityCommitment which implements the full pipeline.
func (ms msgServer) SubmitSimilarityCommitment(goCtx context.Context, msg *types.MsgSubmitSimilarityCommitment) (*types.MsgSubmitSimilarityCommitmentResponse, error) {
	return ms.ProcessSimilarityCommitment(goCtx, msg)
}
