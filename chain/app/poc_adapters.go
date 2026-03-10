package app

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	pockeeper "pos/x/poc/keeper"
	poctypes "pos/x/poc/types"
)

// ========================================================================
// PoC Keeper Adapters
//
// These adapters wrap the real PoC keeper to satisfy the PocKeeper interfaces
// defined by x/repgov, x/royalty, and x/uci. The adapters translate between
// the PoC keeper's concrete types and the simplified interface types expected
// by downstream modules.
// ========================================================================

// RepgovPocKeeperAdapter wraps the PoC keeper to satisfy repgov's PocKeeper interface.
type RepgovPocKeeperAdapter struct {
	keeper *pockeeper.Keeper
}

func NewRepgovPocKeeperAdapter(k *pockeeper.Keeper) *RepgovPocKeeperAdapter {
	return &RepgovPocKeeperAdapter{keeper: k}
}

// GetCreditAmount returns the accumulated credit amount for a contributor.
func (a *RepgovPocKeeperAdapter) GetCreditAmount(ctx context.Context, addr string) math.Int {
	accAddr, err := sdk.AccAddressFromBech32(addr)
	if err != nil {
		return math.ZeroInt()
	}
	credits := a.keeper.GetCredits(ctx, accAddr)
	return credits.Amount
}

// GetReputationScoreValue returns the reputation score as a decimal [0, 1].
func (a *RepgovPocKeeperAdapter) GetReputationScoreValue(ctx context.Context, addr string) math.LegacyDec {
	score := a.keeper.GetReputationScore(ctx, addr)
	return score.Score
}

// GetEndorsementParticipationRate delegates directly to the PoC keeper.
func (a *RepgovPocKeeperAdapter) GetEndorsementParticipationRate(ctx context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error) {
	return a.keeper.GetEndorsementParticipationRate(ctx, valAddr)
}

// GetValidatorOriginalityMetrics delegates directly to the PoC keeper.
func (a *RepgovPocKeeperAdapter) GetValidatorOriginalityMetrics(ctx context.Context, valAddr sdk.ValAddress) (avgOriginality, avgQuality math.LegacyDec, err error) {
	return a.keeper.GetValidatorOriginalityMetrics(ctx, valAddr)
}

// ========================================================================

// RoyaltyPocKeeperAdapter wraps the PoC keeper to satisfy royalty's PocKeeper interface.
type RoyaltyPocKeeperAdapter struct {
	keeper *pockeeper.Keeper
}

func NewRoyaltyPocKeeperAdapter(k *pockeeper.Keeper) *RoyaltyPocKeeperAdapter {
	return &RoyaltyPocKeeperAdapter{keeper: k}
}

// GetCreditAmount returns the accumulated credit amount for a contributor.
func (a *RoyaltyPocKeeperAdapter) GetCreditAmount(ctx context.Context, addr string) math.Int {
	accAddr, err := sdk.AccAddressFromBech32(addr)
	if err != nil {
		return math.ZeroInt()
	}
	credits := a.keeper.GetCredits(ctx, accAddr)
	return credits.Amount
}

// IsContributionRewarded checks if a contribution exists and has been rewarded.
func (a *RoyaltyPocKeeperAdapter) IsContributionRewarded(ctx context.Context, id uint64) bool {
	contribution, found := a.keeper.GetContribution(ctx, id)
	if !found {
		return false
	}
	return contribution.Rewarded
}

// GetPendingRewardsAmount returns the pending rewards amount for a contributor.
func (a *RoyaltyPocKeeperAdapter) GetPendingRewardsAmount(ctx context.Context, addr string) math.Int {
	accAddr, err := sdk.AccAddressFromBech32(addr)
	if err != nil {
		return math.ZeroInt()
	}
	return a.keeper.GetPendingRewardsAmount(ctx, accAddr)
}

// ========================================================================

// UCIPocKeeperAdapter wraps the PoC keeper to satisfy uci's PocKeeper interface.
// It creates contributions directly via the keeper's SetContribution method.
type UCIPocKeeperAdapter struct {
	keeper *pockeeper.Keeper
}

func NewUCIPocKeeperAdapter(k *pockeeper.Keeper) *UCIPocKeeperAdapter {
	return &UCIPocKeeperAdapter{keeper: k}
}

// SubmitContribution creates a new contribution in the PoC module.
// This bypasses the MsgServer and creates the contribution directly via the keeper.
func (a *UCIPocKeeperAdapter) SubmitContribution(ctx context.Context, contributor string, hash string, uri string, ctype string) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	id, err := a.keeper.GetNextContributionID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get next contribution ID: %w", err)
	}

	contribution := poctypes.Contribution{
		Id:          id,
		Contributor: contributor,
		Ctype:       ctype,
		Uri:         uri,
		Hash:        []byte(hash),
		BlockHeight: sdkCtx.BlockHeight(),
		BlockTime:   sdkCtx.BlockTime().Unix(),
	}

	if err := a.keeper.SetContribution(ctx, contribution); err != nil {
		return 0, fmt.Errorf("failed to set contribution: %w", err)
	}

	return id, nil
}
