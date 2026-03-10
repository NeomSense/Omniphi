package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// maxRewardsPerBlock caps how many pending rewards are processed per EndBlocker call.
// Prevents a burst of pending contributions from stalling the chain. Remaining
// entries are processed in subsequent blocks (FIFO by contribution ID).
const maxRewardsPerBlock = 50

// DistributeEmissions distributes PoC inflation emissions to the module for rewards
// Called by tokenomics module during EndBlocker
// This receives 30% of total inflation (emission_split_poc = 0.30)
func (k Keeper) DistributeEmissions(ctx context.Context, amount sdk.Coins) error {
	if amount.IsZero() {
		return nil
	}

	params := k.GetParams(ctx)

	// Verify denomination matches reward denom
	if len(amount) != 1 || amount[0].Denom != params.RewardDenom {
		return fmt.Errorf("invalid emission denomination: expected %s, got %v", params.RewardDenom, amount)
	}

	// Mint tokens to PoC module
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, amount); err != nil {
		return fmt.Errorf("failed to mint PoC emissions: %w", err)
	}

	// Emit event for monitoring
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"poc_emissions_received",
			sdk.NewAttribute("module", types.ModuleName),
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	k.logger.Info("PoC emissions distributed",
		"amount", amount.String(),
		"height", sdkCtx.BlockHeight())

	return nil
}

// ProcessPendingRewards distributes token rewards to verified contributions
// that haven't been rewarded yet. Uses the pending-reward index (O(pending))
// instead of scanning all contributions (O(all)), and caps processing at
// maxRewardsPerBlock per EndBlocker call to prevent burst stalls.
// Called during EndBlocker.
func (k Keeper) ProcessPendingRewards(ctx context.Context) error {
	params := k.GetParams(ctx)

	// Get module balance available for distribution
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	availableBalance := k.bankKeeper.GetBalance(ctx, moduleAddr, params.RewardDenom)

	if availableBalance.Amount.IsZero() {
		return nil
	}

	minQuality := k.GetMinQualityForEmission(ctx)

	// Collect up to maxRewardsPerBlock pending contributions via the index.
	// The index contains only verified-not-yet-rewarded IDs (written by EnqueueReward,
	// deleted here on success), so this is O(pending) not O(all).
	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(
		types.KeyPrefixPendingRewardIndex,
		storetypes.PrefixEndBytes(types.KeyPrefixPendingRewardIndex),
	)
	if err != nil {
		return nil // don't halt chain
	}
	defer iterator.Close()

	var pendingContributions []types.Contribution
	for ; iterator.Valid() && len(pendingContributions) < maxRewardsPerBlock; iterator.Next() {
		// Decode contribution ID from the key suffix (8 bytes big-endian uint64)
		keyBytes := iterator.Key()
		if len(keyBytes) < len(types.KeyPrefixPendingRewardIndex)+8 {
			continue
		}
		idBytes := keyBytes[len(types.KeyPrefixPendingRewardIndex):]
		cID := binary.BigEndian.Uint64(idBytes)

		c, found := k.GetContribution(ctx, cID)
		if !found || !c.Verified || c.Rewarded {
			// Stale index entry — clean it up and move on
			_ = k.removePendingRewardIndex(ctx, cID)
			continue
		}

		// Quality floor: skip low-quality submissions (leave in index for potential governance change)
		if minQuality > 0 {
			session, found := k.GetReviewSession(ctx, c.Id)
			if found && session.FinalQuality < minQuality {
				k.logger.Info("contribution below quality floor, skipping emission",
					"contribution_id", c.Id,
					"final_quality", session.FinalQuality,
					"min_quality", minQuality,
				)
				continue
			}
		}
		pendingContributions = append(pendingContributions, c)
	}

	if len(pendingContributions) == 0 {
		return nil
	}

	// Calculate total credits for this batch, applying RewardMult multiplier
	totalCredits := math.ZeroInt()
	for _, c := range pendingContributions {
		weight := k.weightFor(ctx, c)
		credits := params.BaseRewardUnit.Mul(weight)
		rmMult := k.getContributionRewardMultiplier(ctx, c)
		credits = rmMult.MulInt(credits).TruncateInt()
		totalCredits = totalCredits.Add(credits)
	}

	if totalCredits.IsZero() {
		return nil
	}

	// Distribute rewards proportionally across this batch
	totalDistributed := math.ZeroInt()
	for i, c := range pendingContributions {
		weight := k.weightFor(ctx, c)
		credits := params.BaseRewardUnit.Mul(weight)
		rmMult := k.getContributionRewardMultiplier(ctx, c)
		credits = rmMult.MulInt(credits).TruncateInt()

		share := credits.Mul(availableBalance.Amount).Quo(totalCredits)

		// Last entry in batch gets remainder to avoid dust accumulation
		if i == len(pendingContributions)-1 {
			remainder := availableBalance.Amount.Sub(totalDistributed)
			if remainder.IsPositive() {
				share = remainder
			}
		}

		if !share.IsPositive() {
			continue
		}

		contributor, err := sdk.AccAddressFromBech32(c.Contributor)
		if err != nil {
			k.logger.Error("invalid contributor address",
				"contribution_id", c.Id,
				"address", c.Contributor,
				"error", err)
			_ = k.removePendingRewardIndex(ctx, c.Id)
			continue
		}

		coins := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, share))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, contributor, coins); err != nil {
			k.logger.Error("failed to send reward",
				"contribution_id", c.Id,
				"contributor", c.Contributor,
				"amount", share.String(),
				"error", err)
			continue // leave in index — will retry next block
		}

		// Mark rewarded and remove from pending index atomically
		c.Rewarded = true
		if err := k.SetContribution(ctx, c); err != nil {
			k.logger.Error("failed to mark contribution rewarded",
				"contribution_id", c.Id,
				"error", err)
			return fmt.Errorf("failed to update contribution %d reward status: %w", c.Id, err)
		}
		_ = k.removePendingRewardIndex(ctx, c.Id)

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				"poc_reward_distributed",
				sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", c.Id)),
				sdk.NewAttribute("contributor", c.Contributor),
				sdk.NewAttribute("amount", share.String()),
				sdk.NewAttribute("credits", credits.String()),
				sdk.NewAttribute("rewardmult_applied", rmMult.String()),
			),
		)

		totalDistributed = totalDistributed.Add(share)
	}

	k.logger.Info("PoC rewards processed",
		"total_distributed", totalDistributed.String(),
		"contributions_rewarded", len(pendingContributions))

	return nil
}

// GetPendingRewardsAmount calculates total pending rewards for an address.
// Uses the pending-reward index for O(pending) iteration.
func (k Keeper) GetPendingRewardsAmount(ctx context.Context, addr sdk.AccAddress) math.Int {
	params := k.GetParams(ctx)

	totalCredits := math.ZeroInt()
	userCredits := math.ZeroInt()

	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(
		types.KeyPrefixPendingRewardIndex,
		storetypes.PrefixEndBytes(types.KeyPrefixPendingRewardIndex),
	)
	if err != nil {
		return math.ZeroInt()
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		keyBytes := iterator.Key()
		if len(keyBytes) < len(types.KeyPrefixPendingRewardIndex)+8 {
			continue
		}
		idBytes := keyBytes[len(types.KeyPrefixPendingRewardIndex):]
		cID := binary.BigEndian.Uint64(idBytes)

		c, found := k.GetContribution(ctx, cID)
		if !found || !c.Verified || c.Rewarded {
			continue
		}

		weight := k.weightFor(ctx, c)
		credits := params.BaseRewardUnit.Mul(weight)
		totalCredits = totalCredits.Add(credits)

		if c.Contributor == addr.String() {
			userCredits = userCredits.Add(credits)
		}
	}

	if totalCredits.IsZero() {
		return math.ZeroInt()
	}

	// Calculate user's share of module balance
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	availableBalance := k.bankKeeper.GetBalance(ctx, moduleAddr, params.RewardDenom)

	if availableBalance.Amount.IsZero() {
		return math.ZeroInt()
	}

	// pending = (userCredits / totalCredits) * availableBalance
	pending := userCredits.Mul(availableBalance.Amount).Quo(totalCredits)
	return pending
}

// getContributionRewardMultiplier computes the power-weighted average RewardMult
// multiplier for a contribution's endorsing validators. Returns 1.0 (neutral) if
// the rewardmult keeper is not available or no endorsements exist.
func (k Keeper) getContributionRewardMultiplier(ctx context.Context, c types.Contribution) math.LegacyDec {
	if k.rewardmultKeeper == nil || len(c.Endorsements) == 0 {
		return math.LegacyOneDec()
	}

	totalPower := math.LegacyZeroDec()
	weightedMult := math.LegacyZeroDec()

	for _, e := range c.Endorsements {
		if !e.Decision {
			continue // only count approving endorsements
		}
		power := e.Power.ToLegacyDec()
		if power.IsZero() {
			continue
		}

		mult := k.rewardmultKeeper.GetEffectiveMultiplier(ctx, e.ValAddr)
		weightedMult = weightedMult.Add(power.Mul(mult))
		totalPower = totalPower.Add(power)
	}

	if totalPower.IsZero() {
		return math.LegacyOneDec()
	}

	return weightedMult.Quo(totalPower)
}
