package ante

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"pos/x/feemarket/types"
)

// ===============================================================================
// ANCHOR LANE MAX TX GAS ENFORCEMENT
// ===============================================================================
// This decorator ensures NO transaction can exceed the MaxTxGas limit.
// It is the first line of defense against oversized transactions.
//
// This is NOT a smart contract execution environment.
// The anchor lane exists solely for: staking, governance, PoC, and protocol state.
// High-throughput smart contracts run on PoSeq, which commits to this anchor.
// ===============================================================================

// MaxTxGasDecorator enforces the anchor lane MaxTxGas limit.
// This is a STRICT limit that cannot be bypassed.
//
// ===============================================================================
// RATIONALE (WHY 2M MAX TX GAS):
// ===============================================================================
// 1. 2M gas = 3.3% of a 60M block
// 2. Requires 30+ max-size txs to fill a block (no single tx dominance)
// 3. Forces heavy computation off the anchor lane (to PoSeq)
// 4. Prevents governance griefing attacks
// 5. Ensures PoC submissions cannot monopolize blocks
// 6. No smart contracts exist on anchor lane - 2M is sufficient
// 7. Protects lower-end validators from resource exhaustion
// ===============================================================================
type MaxTxGasDecorator struct {
	feemarketKeeper FeeMarketKeeper
}

// FeeMarketKeeper defines the interface for the feemarket keeper
type FeeMarketKeeper interface {
	GetParams(ctx context.Context) types.FeeMarketParams
}

// NewMaxTxGasDecorator creates a new MaxTxGasDecorator
func NewMaxTxGasDecorator(fmk FeeMarketKeeper) MaxTxGasDecorator {
	return MaxTxGasDecorator{
		feemarketKeeper: fmk,
	}
}

// AnteHandle enforces MaxTxGas limit
func (mtg MaxTxGasDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "tx must be a FeeTx")
	}

	// Get the gas limit requested by the transaction
	gasLimit := feeTx.GetGas()

	// Get MaxTxGas from feemarket params
	params := mtg.feemarketKeeper.GetParams(ctx)
	maxTxGas := uint64(params.MaxTxGas)

	// Enforce protocol hard cap as additional safety
	protocolHardCap := uint64(types.ProtocolMaxTxGasHardCap)
	if maxTxGas > protocolHardCap {
		maxTxGas = protocolHardCap
	}

	// STRICT ENFORCEMENT: Reject transactions exceeding MaxTxGas
	if gasLimit > maxTxGas {
		return ctx, errorsmod.Wrapf(
			types.ErrInvalidMaxTxGas,
			"transaction gas limit (%d) exceeds anchor lane MaxTxGas (%d). "+
				"The anchor lane is for staking, governance, and PoC only. "+
				"Heavy computation must use PoSeq. "+
				"Reduce gas limit or split into smaller transactions.",
			gasLimit, maxTxGas,
		)
	}

	return next(ctx, tx, simulate)
}

// ValidateMaxTxGas is a helper function for direct validation
// Can be used in other parts of the codebase
func ValidateMaxTxGas(gasLimit uint64, maxTxGas int64) error {
	max := uint64(maxTxGas)

	// Always enforce protocol hard cap
	protocolCap := uint64(types.ProtocolMaxTxGasHardCap)
	if max > protocolCap {
		max = protocolCap
	}

	if gasLimit > max {
		return fmt.Errorf(
			"transaction gas (%d) exceeds MaxTxGas (%d) - "+
				"anchor lane does not support heavy computation",
			gasLimit, max,
		)
	}
	return nil
}

// GetAnchorLaneMaxTxGas returns the current MaxTxGas for the anchor lane
// This is useful for clients to know the limit before submitting
func GetAnchorLaneMaxTxGas() int64 {
	return types.AnchorLaneMaxTxGas
}

// GetProtocolMaxTxGasHardCap returns the protocol hard cap
// Governance cannot exceed this value
func GetProtocolMaxTxGasHardCap() int64 {
	return types.ProtocolMaxTxGasHardCap
}
