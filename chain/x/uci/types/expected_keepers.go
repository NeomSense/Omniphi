package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank keeper methods
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// AccountKeeper defines the expected account keeper methods
type AccountKeeper interface {
	GetModuleAddress(moduleName string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
}

// PocKeeper defines the expected PoC keeper for submitting contributions on behalf of DePIN contributors
type PocKeeper interface {
	// SubmitContribution submits a contribution to the PoC module on behalf of a DePIN contributor.
	// Returns the PoC contribution ID and any error.
	SubmitContribution(ctx context.Context, contributor string, hash string, uri string, ctype string) (uint64, error)
}

// RewardMultKeeper defines the optional reward multiplier keeper (stub for future integration)
type RewardMultKeeper interface {
	// GetMultiplier returns the effective reward multiplier for a validator
	GetMultiplier(ctx context.Context, valAddr string) (math.LegacyDec, error)
}
