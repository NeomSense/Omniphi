package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank keeper methods
type BankKeeper interface {
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// AccountKeeper defines the expected account keeper methods
type AccountKeeper interface {
	GetModuleAddress(moduleName string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
}

// PocKeeper defines the expected PoC keeper for contribution data.
// This uses an adapter pattern — the real PoC keeper doesn't directly satisfy this interface.
// Instead, an adapter is wired in app.go that wraps the PoC keeper.
type PocKeeper interface {
	// GetCreditAmount returns the accumulated credit amount for a contributor address
	GetCreditAmount(ctx context.Context, addr string) math.Int

	// IsContributionRewarded checks if a contribution has been rewarded
	IsContributionRewarded(ctx context.Context, id uint64) bool

	// GetPendingRewardsAmount returns the pending rewards amount for a contributor
	GetPendingRewardsAmount(ctx context.Context, addr string) math.Int
}
