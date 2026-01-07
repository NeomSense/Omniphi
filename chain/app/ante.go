package app

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/log"
	circuitante "cosmossdk.io/x/circuit/ante"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"

	feemarketante "pos/x/feemarket/ante"
	feemarketkeeper "pos/x/feemarket/keeper"
	govante "pos/x/gov/ante"
)

// HandlerOptions extends the SDK's AnteHandler options with custom module keepers
type HandlerOptions struct {
	ante.HandlerOptions

	CircuitKeeper   *circuitkeeper.Keeper
	FeemarketKeeper *feemarketkeeper.Keeper

	// ProposalValidation options for governance proposal validation
	Codec     codec.Codec
	MsgRouter baseapp.MessageRouter
	Logger    log.Logger
}

// NewAnteHandler returns an AnteHandler that checks and increments sequence
// numbers, checks signatures & account numbers, and deducts fees from the first
// signer. This also includes the MaxTxGasDecorator from the feemarket module
// to enforce the anchor lane max_tx_gas limit.
func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	if options.AccountKeeper == nil {
		return nil, errors.Wrap(sdkerrors.ErrLogic, "account keeper is required for ante builder")
	}

	if options.BankKeeper == nil {
		return nil, errors.Wrap(sdkerrors.ErrLogic, "bank keeper is required for ante builder")
	}

	if options.SignModeHandler == nil {
		return nil, errors.Wrap(sdkerrors.ErrLogic, "sign mode handler is required for ante builder")
	}

	if options.FeemarketKeeper == nil {
		return nil, errors.Wrap(sdkerrors.ErrLogic, "feemarket keeper is required for ante builder")
	}

	anteDecorators := []sdk.AnteDecorator{
		ante.NewSetUpContextDecorator(), // outermost AnteDecorator. SetUpContext must be called first
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		// MaxTxGasDecorator MUST be early to reject oversized transactions before further processing
		// This enforces the anchor lane max_tx_gas limit (default 2M gas)
		feemarketante.NewMaxTxGasDecorator(options.FeemarketKeeper),
		ante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, options.TxFeeChecker),
		ante.NewSetPubKeyDecorator(options.AccountKeeper), // SetPubKeyDecorator must be called before all signature verification decorators
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
	}

	// Add proposal validation decorator if codec and logger are provided
	// This validates governance proposals before they are accepted into the mempool
	if options.Codec != nil && options.Logger != nil {
		// Insert ProposalValidationDecorator after ValidateBasicDecorator
		// This ensures proposals are validated early but after basic tx validation
		proposalValidator := govante.NewProposalValidationDecorator(
			options.Codec,
			options.MsgRouter,
			options.Logger,
		)
		// Insert at position 4 (after ValidateBasicDecorator)
		anteDecorators = append(anteDecorators[:4], append([]sdk.AnteDecorator{proposalValidator}, anteDecorators[4:]...)...)
	}

	return sdk.ChainAnteDecorators(anteDecorators...), nil
}

// Ensure the feemarket keeper implements the interface needed by the decorator
var _ feemarketante.FeeMarketKeeper = (*feemarketkeeper.Keeper)(nil)
