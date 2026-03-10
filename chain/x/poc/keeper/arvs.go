package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ============================================================================
// Adaptive Reward Vesting System (ARVS) Keeper Methods
//
// The ARVS replaces the fixed ImmediateRewardRatio + linear VestingEpochs split
// with a fully dynamic, multi-stage vesting schedule driven by a risk score
// computed from five signals:
//
//   1. CategoryRisk        — how risky is this contribution type?
//   2. TrustScore          — contributor reputation (inverse: low trust = high risk)
//   3. SimilarityScore     — originality (high sim = high risk)
//   4. VerifierConfidence  — reviewer certainty (low confidence = high risk)
//   5. DisputeRate         — historical dispute rate for this contributor
//
// The risk score maps to one of five vesting profiles, each defining tranched
// unlock stages (immediate + delayed). Derivatives and repeat offenders are
// assigned special profiles directly.
//
// On fraud confirmation the unvested balance is distributed via BountyDistribution
// (challenger / burn / treasury / reviewer penalty pool).
// ============================================================================

// ============================================================================
// 1. Risk Score Calculation
// ============================================================================

// ComputeRiskScore calculates a risk score in [0, 10000] bps from the five signals.
//
// Formula (all terms in basis points):
//
//	risk = (categoryNorm * categoryWeight)
//	      + ((1 - trust) * reputationWeight)
//	      + (similarity * originalityWeight)
//	      + ((1 - confidence) * confidenceWeight)
//	      + (disputeRate * disputeWeight)
//
// Each factor is normalised to [0, 1] before weighting. The weights themselves
// are in bps and sum to 10000, so the output is also in bps [0, 10000].
func ComputeRiskScore(input types.RiskScoreInput, weights types.ARVSWeights) uint32 {
	// Normalise CategoryRisk: Low=1 → 0.33, Med=2 → 0.67, High=3 → 1.0
	categoryNorm := math.LegacyNewDec(int64(input.CategoryRisk)).Quo(math.LegacyNewDec(3))

	// Trust inverse: (1 - trustScore). trustScore ∈ [0.1, 1.0]
	trustInverse := math.LegacyOneDec().Sub(input.TrustScore)
	if trustInverse.IsNegative() {
		trustInverse = math.LegacyZeroDec()
	}

	// Similarity ∈ [0, 1]
	similarity := input.SimilarityScore
	if similarity.IsNegative() {
		similarity = math.LegacyZeroDec()
	} else if similarity.GT(math.LegacyOneDec()) {
		similarity = math.LegacyOneDec()
	}

	// Confidence inverse: (1 - confidence). confidence ∈ [0, 1]
	confidenceInverse := math.LegacyOneDec().Sub(input.VerifierConfidence)
	if confidenceInverse.IsNegative() {
		confidenceInverse = math.LegacyZeroDec()
	}

	// Dispute rate ∈ [0, 1]
	disputeRate := input.DisputeRate
	if disputeRate.IsNegative() {
		disputeRate = math.LegacyZeroDec()
	} else if disputeRate.GT(math.LegacyOneDec()) {
		disputeRate = math.LegacyOneDec()
	}

	// Weighted sum (each weight is in bps, so divide by 10000 to normalise)
	w := math.LegacyNewDec(10000)
	score := categoryNorm.Mul(math.LegacyNewDec(int64(weights.CategoryWeight))).Quo(w).
		Add(trustInverse.Mul(math.LegacyNewDec(int64(weights.ReputationWeight))).Quo(w)).
		Add(similarity.Mul(math.LegacyNewDec(int64(weights.OriginalityWeight))).Quo(w)).
		Add(confidenceInverse.Mul(math.LegacyNewDec(int64(weights.ConfidenceWeight))).Quo(w)).
		Add(disputeRate.Mul(math.LegacyNewDec(int64(weights.DisputeWeight))).Quo(w))

	// Re-scale to [0, 10000]
	scoreBps := score.Mul(w).TruncateInt64()
	if scoreBps < 0 {
		return 0
	}
	if scoreBps > 10000 {
		return 10000
	}
	return uint32(scoreBps)
}

// SelectVestingProfile maps a RiskScoreInput to the appropriate VestingProfile.
// Override logic (checked before risk score):
//  1. IsRepeatOffender → RepeatOffender profile
//  2. IsDerivative     → Derivative profile
//  3. risk < lowThreshold  → Low Risk profile
//  4. risk > highThreshold → High Risk profile
//  5. else             → Medium Risk profile
func SelectVestingProfile(
	input types.RiskScoreInput,
	weights types.ARVSWeights,
	profiles []types.VestingProfile,
	lowThreshold, highThreshold uint32,
) (types.VestingProfile, uint32) {
	// Find a profile by ID in the list
	findProfile := func(id types.VestingProfileID) (types.VestingProfile, bool) {
		for _, p := range profiles {
			if p.ProfileID == id {
				return p, true
			}
		}
		return types.VestingProfile{}, false
	}

	// Override 1: repeat offender
	if input.IsRepeatOffender {
		if p, ok := findProfile(types.VestingProfileRepeatOffender); ok {
			return p, 10000 // max risk score
		}
	}

	// Override 2: derivative
	if input.IsDerivative {
		if p, ok := findProfile(types.VestingProfileDerivative); ok {
			scoreBps := ComputeRiskScore(input, weights)
			return p, scoreBps
		}
	}

	// Compute risk score
	scoreBps := ComputeRiskScore(input, weights)

	var targetID types.VestingProfileID
	switch {
	case scoreBps <= lowThreshold:
		targetID = types.VestingProfileLowRisk
	case scoreBps >= highThreshold:
		targetID = types.VestingProfileHighRisk
	default:
		targetID = types.VestingProfileMediumRisk
	}

	if p, ok := findProfile(targetID); ok {
		return p, scoreBps
	}

	// Fallback: use first available profile (should never happen with valid params)
	if len(profiles) > 0 {
		return profiles[0], scoreBps
	}
	return types.VestingProfile{}, scoreBps
}

// ============================================================================
// 2. ARVS Vesting Schedule CRUD
// ============================================================================

// GetARVSVestingSchedule retrieves an ARVS vesting schedule.
func (k Keeper) GetARVSVestingSchedule(ctx context.Context, contributor string, claimID uint64) (types.ARVSVestingSchedule, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetARVSVestingKey(contributor, claimID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ARVSVestingSchedule{}, false
	}

	var schedule types.ARVSVestingSchedule
	if err := json.Unmarshal(bz, &schedule); err != nil {
		return types.ARVSVestingSchedule{}, false
	}
	return schedule, true
}

// SetARVSVestingSchedule stores an ARVS vesting schedule.
func (k Keeper) SetARVSVestingSchedule(ctx context.Context, schedule types.ARVSVestingSchedule) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetARVSVestingKey(schedule.Contributor, schedule.ClaimID)

	bz, err := json.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("failed to marshal ARVS vesting schedule: %w", err)
	}
	return store.Set(key, bz)
}

// ============================================================================
// 3. ARVS Reward Distribution — replaces simple ImmediateRatio + linear vesting
// ============================================================================

// BuildRiskScoreInput constructs an RiskScoreInput from all available signals.
func (k Keeper) BuildRiskScoreInput(
	ctx context.Context,
	contributor string,
	category string,
	similarityScore math.LegacyDec,
	verifierConfidence math.LegacyDec,
	isDerivative bool,
	params types.Params,
) types.RiskScoreInput {
	// Trust score from ContributorStats
	stats := k.GetContributorStats(ctx, contributor)
	trustScore := stats.ReputationScore
	if trustScore.IsNil() {
		trustScore = math.LegacyOneDec()
	}

	// Dispute rate: overturnedReviews / totalSubmissions
	var disputeRate math.LegacyDec
	if stats.TotalSubmissions > 0 {
		disputeRate = math.LegacyNewDec(int64(stats.OverturnedReviews)).
			Quo(math.LegacyNewDec(int64(stats.TotalSubmissions)))
	} else {
		disputeRate = math.LegacyZeroDec()
	}

	// Category risk
	riskMap := types.DefaultCategoryRiskMap()
	catRisk := types.CategoryRiskLevel(category, riskMap)

	// Repeat offender check
	isRepeatOffender := false
	if params.RepeatOffenderThreshold > 0 {
		offenseCount := stats.DuplicateCount + stats.FraudCount
		isRepeatOffender = offenseCount >= params.RepeatOffenderThreshold
	}

	return types.RiskScoreInput{
		CategoryRisk:       catRisk,
		TrustScore:         trustScore,
		SimilarityScore:    similarityScore,
		VerifierConfidence: verifierConfidence,
		DisputeRate:        disputeRate,
		IsDerivative:       isDerivative,
		IsRepeatOffender:   isRepeatOffender,
	}
}

// CreateARVSVestingSchedule constructs and stores an ARVSVestingSchedule with
// multi-stage tranches derived from the selected VestingProfile.
// Returns the immediate payout amount (stage 0 with DelayEpochs == 0).
func (k Keeper) CreateARVSVestingSchedule(
	ctx context.Context,
	contributor string,
	claimID uint64,
	totalAmount math.Int,
	profile types.VestingProfile,
	riskScoreBps uint32,
) (immediateAmount math.Int, err error) {
	currentEpoch := k.GetCurrentEpoch(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	immediateAmount = math.ZeroInt()
	var stages []types.ARVSStageEntry
	remaining := totalAmount

	for i, stage := range profile.Stages {
		// Calculate tranche amount from basis points
		stageAmt := math.NewInt(int64(stage.UnlockBps)).Mul(totalAmount).Quo(math.NewInt(10000))

		// Last stage gets the remainder to avoid rounding loss
		if i == len(profile.Stages)-1 {
			stageAmt = remaining
		}
		if stageAmt.IsNegative() {
			stageAmt = math.ZeroInt()
		}
		remaining = remaining.Sub(stageAmt)

		unlockAtEpoch := currentEpoch + uint64(stage.DelayEpochs)

		entry := types.ARVSStageEntry{
			StageIndex:    uint32(i),
			UnlockAtEpoch: unlockAtEpoch,
			Amount:        stageAmt,
			Released:      false,
		}
		stages = append(stages, entry)

		if stage.DelayEpochs == 0 {
			immediateAmount = immediateAmount.Add(stageAmt)
			entry.Released = true // mark as immediately released
			stages[i] = entry
		}
	}

	schedule := types.ARVSVestingSchedule{
		Contributor:    contributor,
		ClaimID:        claimID,
		ProfileID:      profile.ProfileID,
		RiskScoreBps:   riskScoreBps,
		TotalAmount:    totalAmount,
		ReleasedAmount: immediateAmount,
		Stages:         stages,
		Status:         types.VestingStatusActive,
		StartEpoch:     currentEpoch,
	}

	if err := k.SetARVSVestingSchedule(ctx, schedule); err != nil {
		return math.ZeroInt(), err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_arvs_schedule_created",
		sdk.NewAttribute("contributor", contributor),
		sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
		sdk.NewAttribute("profile", profile.Name),
		sdk.NewAttribute("risk_score_bps", fmt.Sprintf("%d", riskScoreBps)),
		sdk.NewAttribute("total_amount", totalAmount.String()),
		sdk.NewAttribute("immediate_amount", immediateAmount.String()),
		sdk.NewAttribute("stages", fmt.Sprintf("%d", len(stages))),
	))

	return immediateAmount, nil
}

// DistributeRewardsARVS is the ARVS-aware replacement for DistributeRewards.
// It selects a risk-adjusted vesting profile and creates a multi-stage schedule.
// Returns an error only if a critical step (immediate token transfer) fails.
func (k Keeper) DistributeRewardsARVS(
	ctx context.Context,
	output types.RewardOutput,
	recipient string,
	similarityScore math.LegacyDec,
	verifierConfidence math.LegacyDec,
	category string,
	isDerivative bool,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	recipientAddr, err := sdk.AccAddressFromBech32(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	// 1. Pay multi-level royalties (same as existing DistributeRewards)
	for _, route := range output.RoyaltyRoutes {
		if route.Amount.IsPositive() {
			royaltyCoin := sdk.NewCoin(params.RewardDenom, route.Amount)
			royaltyAddr, err := sdk.AccAddressFromBech32(route.Recipient)
			if err != nil {
				continue
			}
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, royaltyAddr, sdk.NewCoins(royaltyCoin)); err != nil {
				continue
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

	// 2. Build risk score input and select vesting profile
	riskInput := k.BuildRiskScoreInput(ctx, recipient, category, similarityScore, verifierConfidence, isDerivative, params)
	profile, riskScoreBps := SelectVestingProfile(
		riskInput,
		params.ARVSWeights,
		params.ARVSVestingProfiles,
		params.ARVSRiskScoreLowThreshold,
		params.ARVSRiskScoreHighThreshold,
	)

	// 3. Determine total distributable reward
	totalReward := output.FinalRewardAmount
	if totalReward.IsZero() {
		return nil
	}

	// 4. Build ARVS schedule and get immediate amount
	immediateAmount, err := k.CreateARVSVestingSchedule(
		ctx, recipient, output.ClaimID, totalReward, profile, riskScoreBps,
	)
	if err != nil {
		return fmt.Errorf("failed to create ARVS vesting schedule: %w", err)
	}

	// 5. Pay immediate tranche now
	if immediateAmount.IsPositive() {
		immediateCoin := sdk.NewCoin(params.RewardDenom, immediateAmount)
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, sdk.NewCoins(immediateCoin)); err != nil {
			return err
		}
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"poc_arvs_reward_immediate",
			sdk.NewAttribute("amount", immediateCoin.String()),
			sdk.NewAttribute("recipient", recipient),
			sdk.NewAttribute("claim_id", fmt.Sprintf("%d", output.ClaimID)),
			sdk.NewAttribute("profile", profile.Name),
		))
	}

	return nil
}

// ============================================================================
// 4. ARVS EndBlocker — Process Stage Releases
// ============================================================================

// ProcessARVSVestingReleases iterates all active ARVS schedules and releases
// any tranches whose unlock epoch has arrived. Called from EndBlocker.
// Never panics.
func (k Keeper) ProcessARVSVestingReleases(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentEpoch := k.GetCurrentEpoch(ctx)
	params := k.GetParams(ctx)

	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(
		types.KeyPrefixARVSVesting,
		storetypes.PrefixEndBytes(types.KeyPrefixARVSVesting),
	)
	if err != nil {
		return nil // don't panic in EndBlocker
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var schedule types.ARVSVestingSchedule
		if err := json.Unmarshal(iterator.Value(), &schedule); err != nil {
			continue
		}

		if schedule.Status != types.VestingStatusActive {
			continue // skip Paused, Completed, ClawedBack
		}

		recipientAddr, err := sdk.AccAddressFromBech32(schedule.Contributor)
		if err != nil {
			continue
		}

		released := false
		for i, stage := range schedule.Stages {
			if stage.Released {
				continue
			}
			if currentEpoch < stage.UnlockAtEpoch {
				continue // not yet due
			}
			if stage.Amount.IsZero() {
				schedule.Stages[i].Released = true
				continue
			}

			// Release this tranche
			coin := sdk.NewCoin(params.RewardDenom, stage.Amount)
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, sdk.NewCoins(coin)); err != nil {
				k.Logger().Error("ARVS: failed to release vesting tranche",
					"contributor", schedule.Contributor,
					"claim_id", schedule.ClaimID,
					"stage", i,
					"error", err.Error())
				continue
			}

			schedule.Stages[i].Released = true
			schedule.ReleasedAmount = schedule.ReleasedAmount.Add(stage.Amount)
			released = true

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"poc_arvs_stage_released",
				sdk.NewAttribute("contributor", schedule.Contributor),
				sdk.NewAttribute("claim_id", fmt.Sprintf("%d", schedule.ClaimID)),
				sdk.NewAttribute("stage_index", fmt.Sprintf("%d", i)),
				sdk.NewAttribute("amount", stage.Amount.String()),
				sdk.NewAttribute("epoch", fmt.Sprintf("%d", currentEpoch)),
			))
		}

		// Check if fully completed
		if released || schedule.ReleasedAmount.GTE(schedule.TotalAmount) {
			allReleased := true
			for _, s := range schedule.Stages {
				if !s.Released {
					allReleased = false
					break
				}
			}
			if allReleased {
				schedule.Status = types.VestingStatusCompleted
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"poc_arvs_vesting_completed",
					sdk.NewAttribute("contributor", schedule.Contributor),
					sdk.NewAttribute("claim_id", fmt.Sprintf("%d", schedule.ClaimID)),
					sdk.NewAttribute("total_vested", schedule.TotalAmount.String()),
				))
			}

			if err := k.SetARVSVestingSchedule(ctx, schedule); err != nil {
				k.Logger().Error("ARVS: failed to update vesting schedule", "error", err.Error())
			}
		}
	}

	return nil
}

// ============================================================================
// 5. ARVS Pause / Resume / Clawback
// ============================================================================

// PauseARVSVesting freezes an ARVS schedule during appeal. No-op if not active.
func (k Keeper) PauseARVSVesting(ctx context.Context, contributor string, claimID uint64) error {
	schedule, found := k.GetARVSVestingSchedule(ctx, contributor, claimID)
	if !found || schedule.Status != types.VestingStatusActive {
		return nil
	}
	schedule.Status = types.VestingStatusPaused
	return k.SetARVSVestingSchedule(ctx, schedule)
}

// ResumeARVSVesting unfreezes a paused ARVS schedule. No-op if not paused.
func (k Keeper) ResumeARVSVesting(ctx context.Context, contributor string, claimID uint64) error {
	schedule, found := k.GetARVSVestingSchedule(ctx, contributor, claimID)
	if !found || schedule.Status != types.VestingStatusPaused {
		return nil
	}
	schedule.Status = types.VestingStatusActive
	return k.SetARVSVestingSchedule(ctx, schedule)
}

// ClawbackARVSVesting marks an ARVS schedule as clawed back and returns the
// unvested amount (sum of unreleased stages). Funds remain in the module account.
func (k Keeper) ClawbackARVSVesting(ctx context.Context, contributor string, claimID uint64) (math.Int, error) {
	schedule, found := k.GetARVSVestingSchedule(ctx, contributor, claimID)
	if !found {
		return math.ZeroInt(), nil // no-op
	}
	if schedule.Status != types.VestingStatusActive && schedule.Status != types.VestingStatusPaused {
		return math.ZeroInt(), nil // already completed or clawed back
	}

	// Sum unreleased stages
	unvested := math.ZeroInt()
	for _, stage := range schedule.Stages {
		if !stage.Released {
			unvested = unvested.Add(stage.Amount)
		}
	}

	schedule.Status = types.VestingStatusClawedBack
	if err := k.SetARVSVestingSchedule(ctx, schedule); err != nil {
		return math.ZeroInt(), err
	}

	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		"poc_arvs_vesting_clawedback",
		sdk.NewAttribute("contributor", contributor),
		sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
		sdk.NewAttribute("unvested_amount", unvested.String()),
	))

	return unvested, nil
}

// ============================================================================
// 6. Bounty Distribution
// ============================================================================

// DistributeBounty distributes slashed unvested rewards to challenger, burn,
// treasury, and reviewer penalty pool per the BountyDistribution params.
// Called after ExecuteClawback when ARVSEnableBounty is true.
func (k Keeper) DistributeBounty(
	ctx context.Context,
	unvestedAmount math.Int,
	challenger string,
	claimID uint64,
) error {
	if unvestedAmount.IsZero() {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)
	dist := params.ARVSBountyDistribution

	// Challenger share
	challengerAmt := math.NewInt(int64(dist.ChallengerBps)).Mul(unvestedAmount).Quo(math.NewInt(10000))
	// Burn share
	burnAmt := math.NewInt(int64(dist.BurnBps)).Mul(unvestedAmount).Quo(math.NewInt(10000))
	// Treasury share
	treasuryAmt := math.NewInt(int64(dist.TreasuryBps)).Mul(unvestedAmount).Quo(math.NewInt(10000))
	// Reviewer penalty pool = remainder (avoids rounding dust)
	reviewerPoolAmt := unvestedAmount.Sub(challengerAmt).Sub(burnAmt).Sub(treasuryAmt)
	if reviewerPoolAmt.IsNegative() {
		reviewerPoolAmt = math.ZeroInt()
	}

	denom := params.RewardDenom

	// Pay challenger
	if challengerAmt.IsPositive() && challenger != "" {
		challAddr, err := sdk.AccAddressFromBech32(challenger)
		if err == nil {
			coin := sdk.NewCoin(denom, challengerAmt)
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challAddr, sdk.NewCoins(coin)); err != nil {
				k.Logger().Error("bounty: failed to pay challenger", "error", err.Error())
			} else {
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"poc_bounty_challenger_paid",
					sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
					sdk.NewAttribute("challenger", challenger),
					sdk.NewAttribute("amount", coin.String()),
				))
			}
		}
	}

	// Burn share
	if burnAmt.IsPositive() {
		burnCoin := sdk.NewCoin(denom, burnAmt)
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(burnCoin)); err != nil {
			k.Logger().Error("bounty: failed to burn", "error", err.Error())
		} else {
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"poc_bounty_burned",
				sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
				sdk.NewAttribute("amount", burnCoin.String()),
			))
		}
	}

	// Treasury share
	if treasuryAmt.IsPositive() && params.ARVSTreasuryAddress != "" {
		treasuryAddr, err := sdk.AccAddressFromBech32(params.ARVSTreasuryAddress)
		if err == nil {
			coin := sdk.NewCoin(denom, treasuryAmt)
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, treasuryAddr, sdk.NewCoins(coin)); err != nil {
				k.Logger().Error("bounty: failed to pay treasury", "error", err.Error())
			} else {
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"poc_bounty_treasury_paid",
					sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
					sdk.NewAttribute("amount", coin.String()),
				))
			}
		}
	}

	// Reviewer penalty pool — accumulate in module for future distribution
	// (stays in module account; governance decides when/how to distribute)
	if reviewerPoolAmt.IsPositive() {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"poc_bounty_reviewer_pool",
			sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
			sdk.NewAttribute("amount", sdk.NewCoin(denom, reviewerPoolAmt).String()),
		))
	}

	return nil
}
