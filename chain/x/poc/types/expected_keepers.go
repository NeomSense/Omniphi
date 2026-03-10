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
	// GetAccount retrieves an account by address (used by similarity oracle signature verification)
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
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

// ============================================================================
// PoC Hardening Upgrade Keeper Interfaces (v2)
// ============================================================================

// PorKeeper defines the expected PoR keeper interface for finality integration
// This interface is used when PoR module is live to provide record finality
type PorKeeper interface {
	// GetBatch returns a batch by ID
	GetBatch(ctx context.Context, batchID uint64) (batch interface{}, found bool)

	// IsBatchFinalized returns true if the batch has passed challenge window
	IsBatchFinalized(ctx context.Context, batchID uint64) bool

	// IsBatchChallenged returns true if the batch has an open challenge
	IsBatchChallenged(ctx context.Context, batchID uint64) bool

	// IsBatchRejected returns true if the batch was invalidated by fraud proof
	IsBatchRejected(ctx context.Context, batchID uint64) bool

	// GetBatchForContribution returns the batch ID that contains a contribution
	// Returns 0 if the contribution is not linked to any batch (direct PoV mode)
	GetBatchForContribution(ctx context.Context, contributionID uint64) uint64

	// HasFraudulentAttestation returns true if an address has fraudulent attestations
	// in the lookback window (used by x/rewardmult for fraud penalties)
	HasFraudulentAttestation(ctx context.Context, addr sdk.ValAddress, lookbackEpochs int64) (bool, error)
}

// EpochsKeeper defines the expected epochs keeper interface for epoch tracking
type EpochsKeeper interface {
	// GetCurrentEpoch returns the current epoch number
	GetCurrentEpoch(ctx context.Context) uint64

	// GetEpochDuration returns the duration of an epoch in blocks
	GetEpochDuration(ctx context.Context) int64
}

// RewardmultKeeper defines the expected rewardmult keeper interface for metrics export
type RewardmultKeeper interface {
	// RecordEndorsementParticipation records validator endorsement participation
	// Called when an endorsement is submitted to track participation metrics
	RecordEndorsementParticipation(ctx context.Context, valAddr sdk.ValAddress, participated bool) error

	// GetEffectiveMultiplier returns the effective reward multiplier for a validator.
	// Returns 1.0 (neutral) if no multiplier data is available.
	GetEffectiveMultiplier(ctx context.Context, valAddr string) math.LegacyDec
}

// SlashingKeeper defines the expected slashing keeper interface for fraud endorsement penalties.
// OPTIONAL: If not set, fraud endorsement slashing is skipped (soft penalties still apply).
type SlashingKeeper interface {
	// Slash slashes a validator's bonded tokens by slashFactor
	Slash(ctx context.Context, consAddr sdk.ConsAddress, infractionHeight int64, power int64, slashFactor math.LegacyDec) (math.Int, error)

	// Jail jails a validator
	Jail(ctx context.Context, consAddr sdk.ConsAddress) error
}
