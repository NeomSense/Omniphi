package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GovKeeper defines the expected governance keeper interface
type GovKeeper interface {
	// GetProposal returns a proposal by ID
	GetProposal(ctx context.Context, proposalID uint64) (govtypes.Proposal, error)

	// IterateProposals iterates over all proposals
	IterateProposals(ctx context.Context, cb func(proposal govtypes.Proposal) (stop bool)) error

	// GetParams returns gov module params
	GetParams(ctx context.Context) (govtypes.Params, error)
}

// StakingKeeper defines the expected staking keeper interface
type StakingKeeper interface {
	// TotalBondedTokens returns the total bonded tokens
	TotalBondedTokens(ctx context.Context) (math.Int, error)

	// GetAllValidators returns all validators
	GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error)

	// Validator returns a validator by operator address
	Validator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.ValidatorI, error)

	// PowerReduction returns the power reduction factor for consensus power
	PowerReduction(ctx context.Context) math.Int

	// IterateBondedValidatorsByPower iterates over bonded validators by power
	IterateBondedValidatorsByPower(ctx context.Context, fn func(index int64, validator stakingtypes.ValidatorI) (stop bool)) error
}

// BankKeeper defines the expected bank keeper interface
type BankKeeper interface {
	// GetBalance returns the balance of a specific denom for an account
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin

	// GetAllBalances returns all balances for an account
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins

	// SpendableCoins returns the spendable balance for an account
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
}

// DistrKeeper defines the expected distribution keeper interface
type DistrKeeper interface {
	// GetFeePool returns the community pool fee pool
	GetFeePool(ctx context.Context) (distrtypes.FeePool, error)
}

// MessageRouter defines the interface for routing sdk.Msg to handlers
// This mirrors baseapp.MessageRouter from the Cosmos SDK
type MessageRouter interface {
	Handler(msg sdk.Msg) MsgServiceHandler
}

// MsgServiceHandler is the handler function signature for sdk messages
type MsgServiceHandler = func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error)

// TimelockKeeperI defines the interface needed from timelock
type TimelockKeeperI interface {
	IsTrackFrozen(ctx context.Context, operationID uint64) (bool, string)
}
