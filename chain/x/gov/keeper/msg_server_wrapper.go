// Package keeper provides governance message server wrapper with proposal validation
package keeper

import (
	"context"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"pos/x/gov/types"
)

// MsgServerWrapper wraps the standard gov MsgServer with proposal validation
type MsgServerWrapper struct {
	govv1.MsgServer
	validator *ProposalValidator
	logger    log.Logger
}

// NewMsgServerWrapper creates a new MsgServerWrapper that validates proposals before submission
func NewMsgServerWrapper(
	govMsgServer govv1.MsgServer,
	cdc codec.Codec,
	msgRouter baseapp.MessageRouter,
	logger log.Logger,
) govv1.MsgServer {
	config := DefaultProposalValidatorConfig()
	validator := NewProposalValidator(cdc, msgRouter, logger, nil, config)

	return &MsgServerWrapper{
		MsgServer: govMsgServer,
		validator: validator,
		logger:    logger.With("module", "gov-msg-server-wrapper"),
	}
}

// SubmitProposal validates the proposal messages before submitting to the underlying gov module
func (w *MsgServerWrapper) SubmitProposal(ctx context.Context, msg *govv1.MsgSubmitProposal) (*govv1.MsgSubmitProposalResponse, error) {
	w.logger.Info("validating proposal before submission",
		"proposer", msg.Proposer,
		"title", msg.Title,
	)

	// Validate the proposal
	if err := w.validator.ValidateProposal(ctx, msg); err != nil {
		w.logger.Error("proposal validation failed",
			"proposer", msg.Proposer,
			"title", msg.Title,
			"error", err,
		)
		return nil, types.ErrProposalValidationFailed.Wrapf("proposal validation failed: %v", err)
	}

	w.logger.Info("proposal validation passed, submitting to gov module",
		"proposer", msg.Proposer,
		"title", msg.Title,
	)

	// Call the underlying gov module's SubmitProposal
	return w.MsgServer.SubmitProposal(ctx, msg)
}

// GovMsgServerWrapperFactory creates wrapped gov message servers
type GovMsgServerWrapperFactory struct {
	cdc       codec.Codec
	msgRouter baseapp.MessageRouter
	logger    log.Logger
}

// NewGovMsgServerWrapperFactory creates a new factory
func NewGovMsgServerWrapperFactory(
	cdc codec.Codec,
	msgRouter baseapp.MessageRouter,
	logger log.Logger,
) *GovMsgServerWrapperFactory {
	return &GovMsgServerWrapperFactory{
		cdc:       cdc,
		msgRouter: msgRouter,
		logger:    logger,
	}
}

// WrapGovMsgServer wraps a gov keeper's message server with validation
func (f *GovMsgServerWrapperFactory) WrapGovMsgServer(govKeeper *govkeeper.Keeper) govv1.MsgServer {
	originalMsgServer := govkeeper.NewMsgServerImpl(govKeeper)
	return NewMsgServerWrapper(originalMsgServer, f.cdc, f.msgRouter, f.logger)
}
