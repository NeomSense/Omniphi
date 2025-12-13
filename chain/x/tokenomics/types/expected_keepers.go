package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// AccountKeeper defines the expected account keeper
type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx context.Context, acc sdk.AccountI)
	GetModuleAddress(name string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
}

// BankKeeper defines the expected bank keeper
type BankKeeper interface {
	SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
}

// StakingKeeper defines the expected staking keeper
type StakingKeeper interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
	GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error)
	GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
	TotalBondedTokens(ctx context.Context) (math.Int, error)
	PowerReduction(ctx context.Context) math.Int
}

// GovKeeper defines the expected governance keeper
type GovKeeper interface {
	// Methods for proposal validation and execution
}

// IBCKeeper defines the expected IBC keeper
type IBCKeeper interface {
	// SendPacket sends an IBC packet
	// SendPacket(ctx context.Context, channelID string, packet interface{}) error
}

// DistributionKeeper defines the expected distribution keeper
type DistributionKeeper interface {
	// FundCommunityPool transfers coins to the community pool
	// FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}
