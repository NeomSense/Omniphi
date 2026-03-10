package types

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingKeeper defines the expected staking keeper methods
type StakingKeeper interface {
	GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error)
	GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error)
	TotalBondedTokens(ctx context.Context) (math.Int, error)
	PowerReduction(ctx context.Context) math.Int
}

// SlashingKeeper defines the expected slashing keeper methods for signing info
type SlashingKeeper interface {
	GetValidatorSigningInfo(ctx context.Context, consAddr sdk.ConsAddress) (slashingtypes.ValidatorSigningInfo, error)
}

// DistrKeeper defines the expected distribution keeper methods (for reward hook)
// This is intentionally minimal — we only need to read/adjust validator rewards.
type DistrKeeper interface {
	GetValidatorOutstandingRewardsCoins(ctx context.Context, val sdk.ValAddress) (sdk.DecCoins, error)
}

// PocKeeper defines the optional PoC keeper for endorsement participation data
type PocKeeper interface {
	// GetEndorsementParticipationRate returns [0,1] indicating how actively this validator
	// participates in PoV endorsements relative to opportunities. Returns 0 if unknown.
	GetEndorsementParticipationRate(ctx context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error)

	// GetValidatorOriginalityMetrics returns the average originality multiplier and average
	// quality score for contributions endorsed by this validator in the current epoch.
	// Returns (1.0, 0.5, nil) defaults if no data is available.
	GetValidatorOriginalityMetrics(ctx context.Context, valAddr sdk.ValAddress) (avgOriginality, avgQuality math.LegacyDec, err error)
}

// PorKeeper defines the optional PoR keeper for fraud/uptime data (stub)
type PorKeeper interface {
	// HasFraudulentAttestation returns true if this validator endorsed an invalid PoR batch
	// within the given lookback window. Returns false if PoR is not yet live.
	HasFraudulentAttestation(ctx context.Context, valAddr sdk.ValAddress, lookbackEpochs int64) (bool, error)
}
