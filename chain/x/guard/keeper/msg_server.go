package keeper

import (
	"context"
	"encoding/hex"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/guard/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// UpdateParams updates the module parameters
func (ms msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Verify authority
	if ms.GetAuthority() != msg.Authority {
		return nil, types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			ms.GetAuthority(),
			msg.Authority,
		)
	}

	// Validate params
	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}

	// Set params
	if err := ms.SetParams(goCtx, msg.Params); err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_params_updated",
		sdk.NewAttribute("authority", msg.Authority),
	))

	return &types.MsgUpdateParamsResponse{}, nil
}

// ConfirmExecution confirms execution of a CRITICAL proposal
func (ms msgServer) ConfirmExecution(goCtx context.Context, msg *types.MsgConfirmExecution) (*types.MsgConfirmExecutionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Verify authority (must be gov module authority)
	if ms.GetAuthority() != msg.Authority {
		return nil, types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			ms.GetAuthority(),
			msg.Authority,
		)
	}

	// Get queued execution
	exec, found := ms.GetQueuedExecution(goCtx, msg.ProposalId)
	if !found {
		return nil, types.ErrQueuedExecutionNotFound.Wrapf("proposal_id: %d", msg.ProposalId)
	}

	// Verify it's in READY state
	if exec.GateState != types.EXECUTION_GATE_READY {
		return nil, types.ErrNotReady.Wrapf(
			"proposal %d is in state %s, expected READY",
			msg.ProposalId,
			exec.GateState.GetGateStateName(),
		)
	}

	// Verify it requires confirmation
	if !exec.RequiresSecondConfirm {
		return nil, types.ErrInvalidGateState.Wrap("proposal does not require second confirmation")
	}

	// Verify not already confirmed
	if exec.SecondConfirmReceived {
		return nil, types.ErrInvalidGateState.Wrap("proposal already confirmed")
	}

	// Set confirmation
	exec.SecondConfirmReceived = true
	exec.StatusNote = fmt.Sprintf("Second confirmation received: %s", msg.Justification)

	if err := ms.SetQueuedExecution(goCtx, exec); err != nil {
		return nil, err
	}

	// Emit event
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_execution_confirmed",
		sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", msg.ProposalId)),
		sdk.NewAttribute("authority", msg.Authority),
		sdk.NewAttribute("justification", msg.Justification),
	))

	ms.Logger().Info("CRITICAL proposal execution confirmed",
		"proposal_id", msg.ProposalId,
		"authority", msg.Authority,
	)

	return &types.MsgConfirmExecutionResponse{}, nil
}

// UpdateAIModel updates the AI model configuration (Layer 2)
func (ms msgServer) UpdateAIModel(goCtx context.Context, msg *types.MsgUpdateAIModel) (*types.MsgUpdateAIModelResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Verify authority
	if ms.GetAuthority() != msg.Authority {
		return nil, types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			ms.GetAuthority(),
			msg.Authority,
		)
	}

	// Verify feature schema hash matches current schema
	currentSchemaHash := types.ComputeFeatureSchemaHash()
	if msg.FeatureSchemaHash != currentSchemaHash {
		return nil, fmt.Errorf("feature_schema_hash mismatch: expected %s, got %s",
			currentSchemaHash, msg.FeatureSchemaHash)
	}

	// Create linear scoring model
	model := types.LinearScoringModel{
		Weights:           msg.Weights,
		Bias:              msg.Bias,
		Scale:             msg.Scale,
		ModelVersion:      msg.ModelVersion,
		FeatureSchemaHash: msg.FeatureSchemaHash,
	}

	// Validate model (checks weights count, scale > 0, schema hash)
	if err := model.Validate(); err != nil {
		return nil, fmt.Errorf("invalid model: %w", err)
	}

	// Compute weights hash
	weightsHash := model.ComputeWeightsHash()

	// Create metadata
	metadata := types.AIModelMetadata{
		ModelVersion:      msg.ModelVersion,
		WeightsHash:       weightsHash,
		FeatureSchemaHash: currentSchemaHash,
		ActivatedHeight:   msg.ActivatedHeight,
	}

	// Validate metadata
	if err := metadata.Validate(); err != nil {
		return nil, fmt.Errorf("invalid metadata: %w", err)
	}

	// Store model and metadata
	if err := ms.SetLinearModel(goCtx, model); err != nil {
		return nil, fmt.Errorf("failed to store model: %w", err)
	}

	if err := ms.SetAIModelMetadata(goCtx, metadata); err != nil {
		return nil, fmt.Errorf("failed to store metadata: %w", err)
	}

	// Emit event
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_ai_model_updated",
		sdk.NewAttribute("model_version", msg.ModelVersion),
		sdk.NewAttribute("weights_hash", weightsHash),
		sdk.NewAttribute("activated_height", fmt.Sprintf("%d", msg.ActivatedHeight)),
	))

	ms.Logger().Info("AI model updated",
		"model_version", msg.ModelVersion,
		"weights_count", len(msg.Weights),
		"weights_hash", weightsHash,
	)

	return &types.MsgUpdateAIModelResponse{}, nil
}

// SubmitAdvisoryLink submits an advisory report link (Layer 3)
func (ms msgServer) SubmitAdvisoryLink(goCtx context.Context, msg *types.MsgSubmitAdvisoryLink) (*types.MsgSubmitAdvisoryLinkResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate proposal ID
	if msg.ProposalId == 0 {
		return nil, types.ErrInvalidProposalID
	}

	// Validate URI
	if msg.Uri == "" {
		return nil, fmt.Errorf("uri cannot be empty")
	}

	// Validate report hash (must be non-empty SHA256 hex, exactly 64 lowercase hex chars)
	if msg.ReportHash == "" {
		return nil, fmt.Errorf("report_hash cannot be empty")
	}
	if len(msg.ReportHash) != 64 {
		return nil, fmt.Errorf("report_hash must be 64-char hex SHA256, got %d chars", len(msg.ReportHash))
	}
	if _, err := hex.DecodeString(msg.ReportHash); err != nil {
		return nil, fmt.Errorf("report_hash must be valid hex: %w", err)
	}

	// Reject overwrite of existing advisory link
	existingLink, found := ms.GetAdvisoryLink(goCtx, msg.ProposalId)
	if found && existingLink.Uri != "" {
		return nil, fmt.Errorf("advisory link already exists for proposal %d (reporter: %s)", msg.ProposalId, existingLink.Reporter)
	}

	// Create advisory link (proto-generated type)
	link := types.AdvisoryLink{
		ProposalId: msg.ProposalId,
		Uri:        msg.Uri,
		ReportHash: msg.ReportHash,
		Reporter:   msg.Reporter,
		CreatedAt:  ctx.BlockHeight(),
	}

	// Store link
	if err := ms.SetAdvisoryLink(goCtx, link); err != nil {
		return nil, fmt.Errorf("failed to store advisory link: %w", err)
	}

	// Emit event
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_advisory_link_added",
		sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", msg.ProposalId)),
		sdk.NewAttribute("reporter", msg.Reporter),
		sdk.NewAttribute("uri", msg.Uri),
		sdk.NewAttribute("report_hash", msg.ReportHash),
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", ctx.BlockHeight())),
	))

	ms.Logger().Info("Advisory link submitted",
		"proposal_id", msg.ProposalId,
		"reporter", msg.Reporter,
		"uri", msg.Uri,
	)

	return &types.MsgSubmitAdvisoryLinkResponse{}, nil
}
