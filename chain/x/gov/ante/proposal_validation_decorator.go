// Package ante provides custom ante handler decorators for governance
package ante

import (
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	govkeeper "pos/x/gov/keeper"
)

// ProposalValidationDecorator validates governance proposals before they enter the mempool
// This prevents invalid proposals from wasting deposits and network resources
type ProposalValidationDecorator struct {
	validator *govkeeper.ProposalValidator
	logger    log.Logger
}

// NewProposalValidationDecorator creates a new ProposalValidationDecorator
func NewProposalValidationDecorator(
	cdc codec.Codec,
	msgRouter baseapp.MessageRouter,
	logger log.Logger,
) ProposalValidationDecorator {
	config := govkeeper.DefaultProposalValidatorConfig()
	validator := govkeeper.NewProposalValidator(cdc, msgRouter, logger, nil, config)

	return ProposalValidationDecorator{
		validator: validator,
		logger:    logger.With("decorator", "proposal-validation"),
	}
}

// AnteHandle implements the AnteDecorator interface
// It validates governance proposal messages before the transaction is accepted
func (pvd ProposalValidationDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	// Check all messages in the transaction
	for _, msg := range tx.GetMsgs() {
		// Check if this is a governance submit proposal message
		submitProposal, ok := msg.(*govv1.MsgSubmitProposal)
		if !ok {
			continue
		}

		pvd.logger.Debug("validating governance proposal",
			"proposer", submitProposal.Proposer,
			"title", submitProposal.Title,
		)

		// Validate the proposal
		if err := pvd.validator.ValidateProposal(ctx, submitProposal); err != nil {
			pvd.logger.Error("proposal validation failed",
				"proposer", submitProposal.Proposer,
				"title", submitProposal.Title,
				"error", err,
			)
			return ctx, err
		}

		pvd.logger.Info("proposal validation passed",
			"proposer", submitProposal.Proposer,
			"title", submitProposal.Title,
		)
	}

	return next(ctx, tx, simulate)
}

// ProposalValidationDecoratorOptions holds options for the decorator
type ProposalValidationDecoratorOptions struct {
	// EnableSimulation enables message simulation during validation
	EnableSimulation bool

	// MaxSimulationGas is the maximum gas for message simulation
	MaxSimulationGas uint64

	// SkipInSimulation skips validation during tx simulation (CheckTx simulate mode)
	SkipInSimulation bool
}

// DefaultProposalValidationDecoratorOptions returns default options
func DefaultProposalValidationDecoratorOptions() ProposalValidationDecoratorOptions {
	return ProposalValidationDecoratorOptions{
		EnableSimulation: true,
		MaxSimulationGas: 10_000_000,
		SkipInSimulation: false,
	}
}

// NewProposalValidationDecoratorWithOptions creates a decorator with custom options
func NewProposalValidationDecoratorWithOptions(
	cdc codec.Codec,
	msgRouter baseapp.MessageRouter,
	logger log.Logger,
	opts ProposalValidationDecoratorOptions,
) ProposalValidationDecorator {
	config := govkeeper.ProposalValidatorConfig{
		EnableSimulation: opts.EnableSimulation,
		MaxGasLimit:      opts.MaxSimulationGas,
	}
	validator := govkeeper.NewProposalValidator(cdc, msgRouter, logger, nil, config)

	return ProposalValidationDecorator{
		validator: validator,
		logger:    logger.With("decorator", "proposal-validation"),
	}
}
