package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

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

// ProcessPendingRewards processes all verified contributions and distributes rewards
// Called during EndBlocker
func (k Keeper) ProcessPendingRewards(ctx context.Context) error {
	params := k.GetParams(ctx)

	// Get module balance available for distribution
	moduleAddr := k.accountKeeper.GetModuleAddress(types.ModuleName)
	availableBalance := k.bankKeeper.GetBalance(ctx, moduleAddr, params.RewardDenom)

	if availableBalance.Amount.IsZero() {
		// No funds to distribute
		return nil
	}

	// Get all verified contributions that haven't been rewarded
	contributions := k.GetAllContributions(ctx)
	var pendingContributions []types.Contribution

	for _, c := range contributions {
		if c.Verified && !c.Rewarded {
			pendingContributions = append(pendingContributions, c)
		}
	}

	if len(pendingContributions) == 0 {
		// No pending contributions to reward
		return nil
	}

	// Calculate total credits for pending contributions
	totalCredits := math.ZeroInt()
	for _, c := range pendingContributions {
		weight := k.weightFor(ctx, c)
		credits := params.BaseRewardUnit.Mul(weight)
		totalCredits = totalCredits.Add(credits)
	}

	if totalCredits.IsZero() {
		return nil
	}

	// Distribute rewards proportionally
	totalDistributed := math.ZeroInt()
	for i, c := range pendingContributions {
		weight := k.weightFor(ctx, c)
		credits := params.BaseRewardUnit.Mul(weight)

		// Calculate share of available balance
		// share = (credits / totalCredits) * availableBalance
		share := credits.Mul(availableBalance.Amount).Quo(totalCredits)

		// For last contribution, distribute remaining balance to avoid dust
		if i == len(pendingContributions)-1 {
			share = availableBalance.Amount.Sub(totalDistributed)
		}

		if share.IsPositive() {
			contributor, err := sdk.AccAddressFromBech32(c.Contributor)
			if err != nil {
				k.logger.Error("invalid contributor address",
					"contribution_id", c.Id,
					"address", c.Contributor,
					"error", err)
				continue
			}

			// Send reward directly to contributor
			coins := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, share))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, contributor, coins); err != nil {
				k.logger.Error("failed to send reward",
					"contribution_id", c.Id,
					"contributor", c.Contributor,
					"amount", share.String(),
					"error", err)
				continue
			}

			// Mark contribution as rewarded
			c.Rewarded = true
			if err := k.SetContribution(ctx, c); err != nil {
				k.logger.Error("failed to update contribution reward status",
					"contribution_id", c.Id,
					"error", err)
				return fmt.Errorf("failed to update contribution %d reward status: %w", c.Id, err)
			}

			// Emit reward event
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent(
					"poc_reward_distributed",
					sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", c.Id)),
					sdk.NewAttribute("contributor", c.Contributor),
					sdk.NewAttribute("amount", share.String()),
					sdk.NewAttribute("credits", credits.String()),
				),
			)

			totalDistributed = totalDistributed.Add(share)
		}
	}

	k.logger.Info("PoC rewards processed",
		"total_distributed", totalDistributed.String(),
		"contributions_rewarded", len(pendingContributions))

	return nil
}

// GetPendingRewardsAmount calculates total pending rewards for an address
func (k Keeper) GetPendingRewardsAmount(ctx context.Context, addr sdk.AccAddress) math.Int {
	params := k.GetParams(ctx)
	contributions := k.GetAllContributions(ctx)

	totalCredits := math.ZeroInt()
	userCredits := math.ZeroInt()

	for _, c := range contributions {
		if c.Verified && !c.Rewarded {
			weight := k.weightFor(ctx, c)
			credits := params.BaseRewardUnit.Mul(weight)
			totalCredits = totalCredits.Add(credits)

			if c.Contributor == addr.String() {
				userCredits = userCredits.Add(credits)
			}
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
