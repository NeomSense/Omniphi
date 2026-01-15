package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// AccountKeeper defines the expected account keeper interface
type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}

// BankKeeper defines the expected bank keeper interface
type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// GovKeeper defines the expected governance keeper interface
type GovKeeper interface {
	// SetProposal stores a proposal
	SetProposal(ctx context.Context, proposal govv1.Proposal) error

	// DeleteProposal removes a proposal from state
	DeleteProposal(ctx context.Context, proposalID uint64) error
}
