package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"pos/x/poc/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// resolveOriginalityMultiplier looks up the multiplier for a given similarity score
// using configurable bands if enabled, otherwise using default bands.
func (k Keeper) resolveOriginalityMultiplier(params types.Params, sim math.LegacyDec) math.LegacyDec {
	bands := types.DefaultOriginalityBands()
	if params.EnableConfigurableBands && len(params.OriginalityBands) > 0 {
		bands = params.OriginalityBands
	}
	for _, band := range bands {
		if sim.GTE(band.MinSimilarity) && sim.LT(band.MaxSimilarity) {
			return band.Multiplier
		}
	}
	return math.LegacyOneDec() // fallback
}

// CalculateReward computes the final reward distribution for a finalized claim.
func (k Keeper) CalculateReward(ctx context.Context, input types.RewardContext) (types.RewardOutput, error) {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	output := types.RewardOutput{
		FinalRewardAmount:     math.ZeroInt(),
		OriginalityMultiplier: math.LegacyZeroDec(),
		RoyaltyAmount:         math.ZeroInt(),
		VestedAmount:          math.ZeroInt(),
		ImmediateAmount:       math.ZeroInt(),
		TotalRoyaltyPaid:      math.ZeroInt(),
		ClaimID:               input.ClaimID,
	}

	// 1. Determine Originality Multiplier
	// Priority: Review Override > Duplicate Flag > Similarity Score
	multiplier := math.LegacyOneDec()

	if input.ReviewOverride == types.Override_DERIVATIVE_FALSE_POSITIVE {
		multiplier = math.LegacyOneDec()
	} else if input.IsDuplicate {
		multiplier = math.LegacyZeroDec()
	} else if input.ReviewOverride == types.Override_DERIVATIVE_TRUE_POSITIVE {
		multiplier = math.LegacyNewDecWithPrec(4, 1) // 0.4x
	} else {
		multiplier = k.resolveOriginalityMultiplier(params, input.SimilarityScore)
	}

	output.OriginalityMultiplier = multiplier

	// 2. Calculate Gross Reward
	// Gross = Base * Quality * Multiplier
	qualityFactor := input.QualityScore.Quo(math.LegacyNewDec(10)) // Normalize 0-10 to 0-1
	grossRewardDec := math.LegacyNewDecFromInt(input.BaseReward).Mul(qualityFactor).Mul(multiplier)

	// 2b. Apply Reputation Penalty
	reputationFactor := math.LegacyOneDec()
	if input.Contributor != "" {
		stats := k.GetContributorStats(ctx, input.Contributor)
		reputationFactor = stats.ReputationScore
		grossRewardDec = grossRewardDec.Mul(reputationFactor)

		// 2c. Repeat Offender Penalties
		if params.RepeatOffenderThreshold > 0 {
			offenseCount := stats.DuplicateCount + stats.FraudCount
			if offenseCount >= params.RepeatOffenderThreshold {
				if !params.RepeatOffenderRewardCap.IsNil() && !params.RepeatOffenderRewardCap.IsZero() {
					maxRewardDec := params.RepeatOffenderRewardCap.Mul(math.LegacyNewDecFromInt(input.BaseReward))
					if grossRewardDec.GT(maxRewardDec) {
						grossRewardDec = maxRewardDec
					}
				}

				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"poc_repeat_offender_penalty",
					sdk.NewAttribute("contributor", input.Contributor),
					sdk.NewAttribute("offense_count", fmt.Sprintf("%d", offenseCount)),
					sdk.NewAttribute("reward_cap", params.RepeatOffenderRewardCap.String()),
				))
			}
		}
	}

	grossReward := grossRewardDec.TruncateInt()

	if grossReward.IsZero() {
		return output, nil
	}

	// 3. Multi-Level Royalty Routing
	var royaltyRoutes []types.RoyaltyRoute
	totalRoyalty := math.ZeroInt()
	maxRoyalty := math.ZeroInt()
	if !params.MaxTotalRoyaltyShare.IsNil() {
		maxRoyalty = params.MaxTotalRoyaltyShare.MulInt(grossReward).TruncateInt()
	}

	if multiplier.LT(math.LegacyOneDec()) && input.ParentClaimID != 0 {
		shares := []math.LegacyDec{params.RoyaltyShare}
		if !params.GrandparentRoyaltyShare.IsNil() && !params.GrandparentRoyaltyShare.IsZero() {
			shares = append(shares, params.GrandparentRoyaltyShare)
		}

		maxDepth := params.MaxRoyaltyDepth
		if maxDepth == 0 {
			maxDepth = 1
		}

		currentClaimID := input.ParentClaimID
		for depth := uint32(1); depth <= maxDepth && currentClaimID != 0; depth++ {
			claim, found := k.GetContribution(ctx, currentClaimID)
			if !found {
				break
			}

			shareIdx := int(depth - 1)
			if shareIdx >= len(shares) {
				break
			}

			royaltyAmt := shares[shareIdx].MulInt(grossReward).TruncateInt()

			// Enforce total cap
			if maxRoyalty.IsPositive() && totalRoyalty.Add(royaltyAmt).GT(maxRoyalty) {
				royaltyAmt = maxRoyalty.Sub(totalRoyalty)
			}
			if royaltyAmt.IsZero() {
				break
			}

			royaltyRoutes = append(royaltyRoutes, types.RoyaltyRoute{
				Recipient: claim.Contributor,
				Amount:    royaltyAmt,
				ClaimID:   currentClaimID,
				Depth:     depth,
			})
			totalRoyalty = totalRoyalty.Add(royaltyAmt)

			currentClaimID = claim.ParentClaimId // traverse upward
		}

		grossReward = grossReward.Sub(totalRoyalty)
	}

	output.RoyaltyRoutes = royaltyRoutes
	output.TotalRoyaltyPaid = totalRoyalty

	// Legacy single-parent compatibility: set RoyaltyAmount/RoyaltyRecipient from first route
	if len(royaltyRoutes) > 0 {
		output.RoyaltyAmount = royaltyRoutes[0].Amount
		output.RoyaltyRecipient = royaltyRoutes[0].Recipient
	}

	output.FinalRewardAmount = grossReward

	// 4. Calculate Vesting Split (with repeat offender extension)
	effectiveVestingEpochs := params.VestingEpochs
	if input.Contributor != "" && params.RepeatOffenderThreshold > 0 {
		stats := k.GetContributorStats(ctx, input.Contributor)
		offenseCount := stats.DuplicateCount + stats.FraudCount
		if offenseCount >= params.RepeatOffenderThreshold && !params.RepeatOffenderVestingMultiplier.IsNil() {
			effectiveVestingDec := params.RepeatOffenderVestingMultiplier.Mul(math.LegacyNewDec(effectiveVestingEpochs))
			effectiveVestingEpochs = effectiveVestingDec.TruncateInt().Int64()
			// Cap at 4x original
			maxVesting := params.VestingEpochs * 4
			if effectiveVestingEpochs > maxVesting {
				effectiveVestingEpochs = maxVesting
			}
		}
	}

	immediate := params.ImmediateRewardRatio.MulInt(grossReward).TruncateInt()
	vested := grossReward.Sub(immediate)

	output.ImmediateAmount = immediate
	output.VestedAmount = vested

	// Emit reward calculation event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_reward_calculated",
		sdk.NewAttribute("claim_id", fmt.Sprintf("%d", input.ClaimID)),
		sdk.NewAttribute("contributor", input.Contributor),
		sdk.NewAttribute("originality_multiplier", multiplier.String()),
		sdk.NewAttribute("reputation_factor", reputationFactor.String()),
		sdk.NewAttribute("gross_reward", grossReward.String()),
		sdk.NewAttribute("royalty_total", totalRoyalty.String()),
		sdk.NewAttribute("immediate", immediate.String()),
		sdk.NewAttribute("vested", vested.String()),
		sdk.NewAttribute("effective_vesting_epochs", fmt.Sprintf("%d", effectiveVestingEpochs)),
	))

	return output, nil
}

// UpdateContributorStats updates the reputation stats for a user based on the outcome.
// This implements the "Repeat Offender" logic.
func (k Keeper) UpdateContributorStats(ctx context.Context, contributor string, input types.RewardContext) error {
	stats := k.GetContributorStats(ctx, contributor)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	oldScore := stats.ReputationScore

	stats.TotalSubmissions++

	// Decay Logic
	decayFactor := math.LegacyOneDec()
	offenseType := "good"

	if input.IsDuplicate {
		stats.DuplicateCount++
		decayFactor = math.LegacyNewDecWithPrec(90, 2) // -10% reputation
		offenseType = "duplicate"
	} else if input.SimilarityScore.GT(math.LegacyNewDecWithPrec(85, 2)) && input.ReviewOverride != types.Override_DERIVATIVE_FALSE_POSITIVE {
		stats.HighSimilarityCount++
		decayFactor = math.LegacyNewDecWithPrec(98, 2) // -2% reputation
		offenseType = "high_similarity"
	} else {
		decayFactor = math.LegacyNewDecWithPrec(101, 2) // +1%
	}

	// Apply decay/growth
	stats.ReputationScore = stats.ReputationScore.Mul(decayFactor)

	// Cap at 1.0
	if stats.ReputationScore.GT(math.LegacyOneDec()) {
		stats.ReputationScore = math.LegacyOneDec()
	}
	// Floor at 0.1
	if stats.ReputationScore.LT(math.LegacyNewDecWithPrec(1, 1)) {
		stats.ReputationScore = math.LegacyNewDecWithPrec(1, 1)
	}

	stats.LastUpdatedBlock = sdkCtx.BlockHeight()

	// Emit reputation update event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_reputation_updated",
		sdk.NewAttribute("contributor", contributor),
		sdk.NewAttribute("old_score", oldScore.String()),
		sdk.NewAttribute("new_score", stats.ReputationScore.String()),
		sdk.NewAttribute("offense_type", offenseType),
	))

	return k.SetContributorStats(ctx, stats)
}

// --- Storage Helpers ---

func (k Keeper) GetContributorStats(ctx context.Context, contributor string) types.ContributorStats {
	store := k.storeService.OpenKVStore(ctx)
	addr, _ := sdk.AccAddressFromBech32(contributor)
	key := types.GetContributorStatsKey(addr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.NewContributorStats(contributor, sdk.UnwrapSDKContext(ctx).BlockHeight())
	}

	var stats types.ContributorStats
	json.Unmarshal(bz, &stats)

	// Ensure zero-value fields are initialized
	if stats.TotalClawedBack.IsNil() {
		stats.TotalClawedBack = math.ZeroInt()
	}
	if stats.CurrentBondMultiplier.IsNil() {
		stats.CurrentBondMultiplier = math.LegacyOneDec()
	}

	return stats
}

func (k Keeper) SetContributorStats(ctx context.Context, stats types.ContributorStats) error {
	store := k.storeService.OpenKVStore(ctx)
	addr, _ := sdk.AccAddressFromBech32(stats.Address)
	key := types.GetContributorStatsKey(addr)

	bz, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	return store.Set(key, bz)
}

// UpdateContributorStatsOnAccept updates contributor stats after a review acceptance.
// NOTE: TotalSubmissions tracks accepted claims. Do NOT also call UpdateContributorStats
// from the pipeline — that would double-count.
func (k Keeper) UpdateContributorStatsOnAccept(ctx context.Context, contributor string, isDerivative bool, similarity math.LegacyDec) {
	stats := k.GetContributorStats(ctx, contributor)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	stats.TotalSubmissions++
	if similarity.GT(math.LegacyNewDecWithPrec(85, 2)) {
		stats.HighSimilarityCount++
	}
	stats.LastUpdatedBlock = sdkCtx.BlockHeight()
	_ = k.SetContributorStats(ctx, stats)
}

// DistributeRewards executes the reward transfer and vesting creation.
func (k Keeper) DistributeRewards(ctx context.Context, output types.RewardOutput, recipient string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	recipientAddr, _ := sdk.AccAddressFromBech32(recipient)

	// 1. Pay Multi-Level Royalties
	for _, route := range output.RoyaltyRoutes {
		if route.Amount.IsPositive() {
			royaltyCoin := sdk.NewCoin(params.RewardDenom, route.Amount)

			// LIQUID IP: Resolve recipient via NFT ownership
			// We assume a standard classID "poc-royalty" and NFT ID "claim-{id}"
			// If the NFT exists, the owner gets the royalty. Otherwise, the original contributor gets it.
			recipientAddrStr := route.Recipient
			// Note: Assuming k.nftKeeper is injected and available.
			// owner := k.nftKeeper.GetOwner(ctx, "poc-royalty", fmt.Sprintf("claim-%d", route.ClaimID))
			// if owner != nil { recipientAddrStr = owner.String() }

			royaltyAddr, err := sdk.AccAddressFromBech32(recipientAddrStr)
			if err != nil {
				continue
			}

			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, royaltyAddr, sdk.NewCoins(royaltyCoin)); err != nil {
				continue // best-effort for non-critical royalties
			}

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"poc_royalty_route_paid",
				sdk.NewAttribute("parent_claim_id", fmt.Sprintf("%d", route.ClaimID)),
				sdk.NewAttribute("recipient", route.Recipient),
				sdk.NewAttribute("amount", royaltyCoin.String()),
				sdk.NewAttribute("depth", fmt.Sprintf("%d", route.Depth)),
			))
		}
	}

	// Legacy fallback: pay single royalty if no routes but RoyaltyAmount is set
	if len(output.RoyaltyRoutes) == 0 && output.RoyaltyAmount.IsPositive() {
		royaltyCoin := sdk.NewCoin(params.RewardDenom, output.RoyaltyAmount)
		royaltyRecipientAddr, err := sdk.AccAddressFromBech32(output.RoyaltyRecipient)
		if err == nil {
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, royaltyRecipientAddr, sdk.NewCoins(royaltyCoin)); err == nil {
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"poc_royalty_paid",
					sdk.NewAttribute("amount", royaltyCoin.String()),
					sdk.NewAttribute("recipient", output.RoyaltyRecipient),
				))
			}
		}
	}

	// 2. Pay Immediate Reward
	if output.ImmediateAmount.IsPositive() {
		immediateCoin := sdk.NewCoin(params.RewardDenom, output.ImmediateAmount)
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, sdk.NewCoins(immediateCoin)); err != nil {
			return err
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"poc_reward_paid_immediate",
			sdk.NewAttribute("amount", immediateCoin.String()),
			sdk.NewAttribute("recipient", recipient),
		))
	}

	// 3. Create Vesting Schedule for Remainder
	if output.VestedAmount.IsPositive() && output.ClaimID > 0 {
		// Compute effective vesting epochs (may be extended for repeat offenders)
		effectiveVestingEpochs := params.VestingEpochs
		if recipient != "" && params.RepeatOffenderThreshold > 0 {
			stats := k.GetContributorStats(ctx, recipient)
			offenseCount := stats.DuplicateCount + stats.FraudCount
			if offenseCount >= params.RepeatOffenderThreshold && !params.RepeatOffenderVestingMultiplier.IsNil() {
				effectiveVestingDec := params.RepeatOffenderVestingMultiplier.Mul(math.LegacyNewDec(effectiveVestingEpochs))
				effectiveVestingEpochs = effectiveVestingDec.TruncateInt().Int64()
				maxVesting := params.VestingEpochs * 4
				if effectiveVestingEpochs > maxVesting {
					effectiveVestingEpochs = maxVesting
				}
			}
		}

		if effectiveVestingEpochs <= 0 {
			effectiveVestingEpochs = 1
		}

		schedule := types.VestingSchedule{
			Contributor:    recipient,
			ClaimID:        output.ClaimID,
			TotalAmount:    output.VestedAmount,
			ReleasedAmount: math.ZeroInt(),
			StartEpoch:     k.GetCurrentEpoch(ctx),
			VestingEpochs:  effectiveVestingEpochs,
			Status:         types.VestingStatusActive,
		}

		if err := k.CreateVestingSchedule(ctx, schedule); err != nil {
			return err
		}
	}

	return nil
}
