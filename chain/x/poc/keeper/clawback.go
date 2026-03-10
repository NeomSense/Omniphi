package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"pos/x/poc/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExecuteClawback claws back rewards from a fraudulent contributor.
func (k Keeper) ExecuteClawback(ctx context.Context, claimID uint64, reason string, authority string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	if reason == "" {
		return types.ErrInvalidClawbackReason
	}

	// Check for existing clawback
	if _, found := k.GetClawbackRecord(ctx, claimID); found {
		return types.ErrClawbackAlreadyApplied
	}

	// Get contribution
	contrib, found := k.GetContribution(ctx, claimID)
	if !found {
		return types.ErrContributionNotFound
	}

	contributor := contrib.Contributor
	record := types.ClawbackRecord{
		Contributor:       contributor,
		ClaimID:           claimID,
		Reason:            reason,
		AmountClawedBack:  math.ZeroInt(),
		VestingClawedBack: math.ZeroInt(),
		BondSlashed:       math.ZeroInt(),
		ExecutedAtBlock:   sdkCtx.BlockHeight(),
		Authority:         authority,
	}

	// 1. Clawback active vesting
	unvested, err := k.ClawbackVesting(ctx, contributor, claimID)
	if err == nil {
		record.VestingClawedBack = unvested
	}

	// 2. Best-effort immediate balance clawback
	// Try to recoup some of the immediate reward from the contributor's balance
	contributorAddr, _ := sdk.AccAddressFromBech32(contributor)
	balance := k.bankKeeper.GetBalance(ctx, contributorAddr, params.RewardDenom)
	// Clawback up to the immediate portion: we estimate as ImmediateRewardRatio * BaseRewardUnit
	// In practice, we try to clawback the minimum of their balance and the estimated amount
	estimatedImmediate := params.ImmediateRewardRatio.MulInt(params.BaseRewardUnit).TruncateInt()
	clawbackAmt := math.MinInt(balance.Amount, estimatedImmediate)
	if clawbackAmt.IsPositive() {
		clawbackCoin := sdk.NewCoin(params.RewardDenom, clawbackAmt)
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, contributorAddr, types.ModuleName, sdk.NewCoins(clawbackCoin)); err == nil {
			record.AmountClawedBack = clawbackAmt
		}
	}

	// 3. Update ContributorStats
	stats := k.GetContributorStats(ctx, contributor)
	stats.FraudCount++
	stats.TotalClawedBack = stats.TotalClawedBack.Add(record.AmountClawedBack).Add(record.VestingClawedBack)
	// Decay reputation by 25% per fraud
	stats.ReputationScore = stats.ReputationScore.Mul(math.LegacyNewDecWithPrec(75, 2))
	if stats.ReputationScore.LT(math.LegacyNewDecWithPrec(1, 1)) {
		stats.ReputationScore = math.LegacyNewDecWithPrec(1, 1)
	}
	// Increase bond multiplier
	stats.CurrentBondMultiplier = stats.CurrentBondMultiplier.Add(math.LegacyNewDecWithPrec(50, 2)) // +0.50x per fraud
	stats.LastUpdatedBlock = sdkCtx.BlockHeight()
	_ = k.SetContributorStats(ctx, stats)

	// 4. Store clawback record
	if err := k.SetClawbackRecord(ctx, record); err != nil {
		return types.ErrClawbackFailed
	}

	// 5. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_clawback_executed",
		sdk.NewAttribute("contributor", contributor),
		sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
		sdk.NewAttribute("reason", reason),
		sdk.NewAttribute("amount_clawed_back", record.AmountClawedBack.String()),
		sdk.NewAttribute("vesting_clawed_back", record.VestingClawedBack.String()),
		sdk.NewAttribute("authority", authority),
	))

	return nil
}

// GetClawbackRecord retrieves a clawback record by claim ID.
func (k Keeper) GetClawbackRecord(ctx context.Context, claimID uint64) (types.ClawbackRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetClawbackRecordKey(claimID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ClawbackRecord{}, false
	}

	var record types.ClawbackRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.ClawbackRecord{}, false
	}
	return record, true
}

// SetClawbackRecord stores a clawback record.
func (k Keeper) SetClawbackRecord(ctx context.Context, record types.ClawbackRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetClawbackRecordKey(record.ClaimID)

	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return store.Set(key, bz)
}
