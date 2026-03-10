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

// PocKeeper defines the expected PoC keeper for reputation data.
// This uses an adapter pattern — the real PoC keeper doesn't directly satisfy this interface.
// Instead, an adapter is wired in app.go that wraps the PoC keeper.
type PocKeeper interface {
	// GetCreditAmount returns the accumulated credit amount for a contributor address.
	GetCreditAmount(ctx context.Context, addr string) math.Int

	// GetReputationScoreValue returns the reputation score [0, 1] for a contributor.
	GetReputationScoreValue(ctx context.Context, addr string) math.LegacyDec

	// GetEndorsementParticipationRate returns validator endorsement participation [0, 1]
	GetEndorsementParticipationRate(ctx context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error)

	// GetValidatorOriginalityMetrics returns avg originality and quality metrics
	GetValidatorOriginalityMetrics(ctx context.Context, valAddr sdk.ValAddress) (avgOriginality, avgQuality math.LegacyDec, err error)
}

// SlashingKeeper defines the expected slashing keeper for uptime data
type SlashingKeeper interface {
	GetValidatorSigningInfo(ctx context.Context, consAddr sdk.ConsAddress) (info interface{}, err error)
}

// GovKeeper defines the expected gov keeper interface for tally integration
type GovKeeper interface {
	// GetProposal returns a proposal by ID
	GetProposal(ctx context.Context, proposalID uint64) (interface{}, error)
}
