package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingKeeper defines the expected staking keeper methods
type StakingKeeper interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
	GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error)
	TotalBondedTokens(ctx context.Context) (math.Int, error)
	PowerReduction(ctx context.Context) math.Int
}

// BankKeeper defines the expected bank keeper methods
type BankKeeper interface {
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// AccountKeeper defines the expected account keeper methods
type AccountKeeper interface {
	GetModuleAddress(moduleName string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
}

// IdentityKeeper defines the expected identity keeper methods (optional - used for PoA layer)
// This interface will be implemented by x/identity module when available
// If not available, identity checks will fail-safe and reject submissions requiring identity
type IdentityKeeper interface {
	// IsVerified checks if an address has completed identity verification (KYC/DID)
	IsVerified(ctx context.Context, addr sdk.AccAddress) bool

	// GetIdentityLevel returns the verification level of an address (0 = none, 1 = basic, 2 = enhanced, etc.)
	// This can be used for tiered identity requirements in the future
	GetIdentityLevel(ctx context.Context, addr sdk.AccAddress) uint32
}
