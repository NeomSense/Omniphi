package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ============================================================================
// PoC Hardening Upgrade - Keeper Methods (v2)
// ============================================================================

// ============================================================================
// Finality Enforcement (Requirement 1)
// ============================================================================

// GetContributionFinality retrieves the finality status for a contribution
func (k Keeper) GetContributionFinality(ctx context.Context, contributionID uint64) types.ContributionFinality {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetContributionFinalityKey(contributionID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		// Default: pending finality (for backwards compatibility with existing contributions)
		return types.ContributionFinality{
			ContributionID: contributionID,
			Status:         types.FinalityStatusPending,
		}
	}

	var finality types.ContributionFinality
	if err := finality.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal contribution finality", "id", contributionID, "error", err)
		return types.ContributionFinality{ContributionID: contributionID, Status: types.FinalityStatusPending}
	}
	return finality
}

// SetContributionFinality stores the finality status for a contribution
func (k Keeper) SetContributionFinality(ctx context.Context, finality types.ContributionFinality) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetContributionFinalityKey(finality.ContributionID)

	bz, err := finality.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal contribution finality: %w", err)
	}

	return store.Set(key, bz)
}

// IsContributionFinal checks if a contribution has reached finality
// This is the main gate for reward payments
func (k Keeper) IsContributionFinal(ctx context.Context, contributionID uint64) bool {
	// First check if PoR module is available
	if k.porKeeper != nil {
		batchID := k.porKeeper.GetBatchForContribution(ctx, contributionID)
		if batchID > 0 {
			// Contribution is linked to PoR batch - use PoR finality
			return k.porKeeper.IsBatchFinalized(ctx, batchID)
		}
	}

	// No PoR or not linked to batch - use direct PoV finality
	// For direct PoV: verified = final (immediate finality)
	contribution, found := k.GetContribution(ctx, contributionID)
	if !found {
		return false
	}

	// If verified via PoV quorum, consider it final
	// This maintains backwards compatibility with existing PoV flow
	return contribution.Verified
}

// IsContributionChallenged checks if a contribution is under active challenge
func (k Keeper) IsContributionChallenged(ctx context.Context, contributionID uint64) bool {
	if k.porKeeper == nil {
		return false // No PoR = no challenges possible
	}

	batchID := k.porKeeper.GetBatchForContribution(ctx, contributionID)
	if batchID == 0 {
		return false // Not linked to PoR batch
	}

	return k.porKeeper.IsBatchChallenged(ctx, batchID)
}

// IsContributionInvalidated checks if a contribution was invalidated by fraud proof
func (k Keeper) IsContributionInvalidated(ctx context.Context, contributionID uint64) bool {
	if k.porKeeper == nil {
		return false
	}

	batchID := k.porKeeper.GetBatchForContribution(ctx, contributionID)
	if batchID == 0 {
		return false
	}

	return k.porKeeper.IsBatchRejected(ctx, batchID)
}

// FinalizeContribution marks a contribution as finalized (called after PoV quorum or PoR finality)
func (k Keeper) FinalizeContribution(ctx context.Context, contributionID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	finality := types.ContributionFinality{
		ContributionID: contributionID,
		Status:         types.FinalityStatusFinal,
		FinalizedAt:    sdkCtx.BlockHeight(),
	}

	if err := k.SetContributionFinality(ctx, finality); err != nil {
		return err
	}

	// Emit finalization event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"poc_contribution_finalized",
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	return nil
}

// ============================================================================
// ReputationScore System (Requirement 2)
// ============================================================================

// GetReputationScore retrieves the reputation score for an address
func (k Keeper) GetReputationScore(ctx context.Context, addr string) types.ReputationScore {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReputationScoreKey(addr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.NewReputationScore(addr)
	}

	var rs types.ReputationScore
	if err := rs.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal reputation score", "address", addr, "error", err)
		return types.NewReputationScore(addr)
	}
	return rs
}

// SetReputationScore stores the reputation score for an address
func (k Keeper) SetReputationScore(ctx context.Context, rs types.ReputationScore) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetReputationScoreKey(rs.Address)

	bz, err := rs.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal reputation score: %w", err)
	}

	return store.Set(key, bz)
}

// UpdateReputationScore updates the reputation score when new credits are earned
func (k Keeper) UpdateReputationScore(ctx context.Context, addr string, newCredits math.Int, currentEpoch int64) error {
	rs := k.GetReputationScore(ctx, addr)

	// Use slow-moving EMA (alpha = 0.1) for reputation
	alpha := math.LegacyNewDecWithPrec(10, 2) // 0.10
	rs.UpdateWithEMA(newCredits, alpha, currentEpoch)

	return k.SetReputationScore(ctx, rs)
}

// GetGovBoostFromReputation calculates governance boost from reputation score
// This replaces the raw credit-based GovWeightBoost for governance
func (k Keeper) GetGovBoostFromReputation(ctx context.Context, addr sdk.AccAddress) math.LegacyDec {
	rs := k.GetReputationScore(ctx, addr.String())

	if rs.Score.IsZero() || rs.Score.IsNegative() {
		return math.LegacyZeroDec()
	}

	// Boost = min(10%, score / 1,000,000)
	// Requires 1,000,000 reputation score for max 10% boost
	boost := rs.Score.Quo(math.LegacyNewDec(1000000))
	maxBoost := math.LegacyNewDecWithPrec(10, 2) // 0.10 (10%)

	if boost.GT(maxBoost) {
		return maxBoost
	}

	return boost
}

// ============================================================================
// Credit Hardening (Requirement 3)
// ============================================================================

// GetEpochCredits retrieves credits earned in a specific epoch
func (k Keeper) GetEpochCredits(ctx context.Context, addr string, epoch uint64) types.EpochCredits {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEpochCreditsKey(addr, epoch)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.NewEpochCredits(addr, epoch)
	}

	var ec types.EpochCredits
	if err := ec.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal epoch credits", "address", addr, "epoch", epoch, "error", err)
		return types.NewEpochCredits(addr, epoch)
	}
	return ec
}

// SetEpochCredits stores epoch credits
func (k Keeper) SetEpochCredits(ctx context.Context, ec types.EpochCredits) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEpochCreditsKey(ec.Address, ec.Epoch)

	bz, err := ec.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal epoch credits: %w", err)
	}

	return store.Set(key, bz)
}

// GetTypeCredits retrieves credits earned for a specific contribution type
func (k Keeper) GetTypeCredits(ctx context.Context, addr string, ctype string) types.TypeCredits {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetTypeCreditsKey(addr, ctype)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.NewTypeCredits(addr, ctype)
	}

	var tc types.TypeCredits
	if err := tc.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal type credits", "address", addr, "ctype", ctype, "error", err)
		return types.NewTypeCredits(addr, ctype)
	}
	return tc
}

// SetTypeCredits stores type credits
func (k Keeper) SetTypeCredits(ctx context.Context, tc types.TypeCredits) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetTypeCreditsKey(tc.Address, tc.Ctype)

	bz, err := tc.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal type credits: %w", err)
	}

	return store.Set(key, bz)
}

// AddCreditsWithCaps adds credits with epoch and type cap enforcement
// This is the hardened version that should be used instead of AddCreditsWithOverflowCheck
func (k Keeper) AddCreditsWithCaps(ctx context.Context, addr sdk.AccAddress, amount math.Int, ctype string, epoch uint64) error {
	// Check total credit cap (100,000)
	existingCredits := k.GetCredits(ctx, addr)
	totalCap := math.NewInt(types.DefaultCreditCap)

	if existingCredits.Amount.Add(amount).GT(totalCap) {
		// Apply diminishing returns instead of hard reject
		effectiveAmount := types.DiminishingReturnsCurve(amount, totalCap.Sub(existingCredits.Amount))
		if effectiveAmount.IsZero() {
			return types.ErrCreditCapExceeded
		}
		amount = effectiveAmount
	}

	// Check epoch cap (10,000 per epoch)
	epochCredits := k.GetEpochCredits(ctx, addr.String(), epoch)
	epochCap := math.NewInt(types.DefaultEpochCreditCap)

	if epochCredits.Credits.Add(amount).GT(epochCap) {
		remaining := epochCap.Sub(epochCredits.Credits)
		if remaining.IsZero() || remaining.IsNegative() {
			return types.ErrEpochCreditCapExceeded
		}
		// Apply diminishing returns for epoch cap too
		amount = types.DiminishingReturnsCurve(amount, remaining)
		if amount.IsZero() {
			return types.ErrEpochCreditCapExceeded
		}
	}

	// Check type cap (50,000 per type)
	typeCredits := k.GetTypeCredits(ctx, addr.String(), ctype)
	typeCap := math.NewInt(types.DefaultTypeCreditCap)

	if typeCredits.Credits.Add(amount).GT(typeCap) {
		remaining := typeCap.Sub(typeCredits.Credits)
		if remaining.IsZero() || remaining.IsNegative() {
			return types.ErrTypeCreditCapExceeded
		}
		amount = types.DiminishingReturnsCurve(amount, remaining)
		if amount.IsZero() {
			return types.ErrTypeCreditCapExceeded
		}
	}

	// All caps passed - add the credits
	if err := k.AddCreditsWithOverflowCheck(ctx, addr, amount); err != nil {
		return err
	}

	// Update epoch tracking
	epochCredits.Credits = epochCredits.Credits.Add(amount)
	if err := k.SetEpochCredits(ctx, epochCredits); err != nil {
		return err
	}

	// Update type tracking
	typeCredits.Credits = typeCredits.Credits.Add(amount)
	typeCredits.Count++
	if err := k.SetTypeCredits(ctx, typeCredits); err != nil {
		return err
	}

	return nil
}

// ============================================================================
// Endorsement Quality Tracking (Requirement 4)
// ============================================================================

// GetValidatorEndorsementStats retrieves endorsement stats for a validator
func (k Keeper) GetValidatorEndorsementStats(ctx context.Context, valAddr string) types.ValidatorEndorsementStats {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetValidatorEndorsementStatsKey(valAddr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.NewValidatorEndorsementStats(valAddr)
	}

	var ves types.ValidatorEndorsementStats
	if err := ves.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal validator endorsement stats", "validator", valAddr, "error", err)
		return types.NewValidatorEndorsementStats(valAddr)
	}
	return ves
}

// SetValidatorEndorsementStats stores validator endorsement stats
func (k Keeper) SetValidatorEndorsementStats(ctx context.Context, ves types.ValidatorEndorsementStats) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetValidatorEndorsementStatsKey(ves.ValidatorAddress)

	bz, err := ves.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal validator endorsement stats: %w", err)
	}

	return store.Set(key, bz)
}

// RecordEndorsement records an endorsement for validator quality tracking
func (k Keeper) RecordEndorsement(ctx context.Context, valAddr string, contribution types.Contribution, isEarly bool) error {
	stats := k.GetValidatorEndorsementStats(ctx, valAddr)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	stats.TotalEndorsed++
	stats.LastUpdated = sdkCtx.BlockHeight()

	if isEarly {
		stats.EarlyEndorsed++
	}

	// Check if this is a post-quorum endorsement (contribution already verified)
	if contribution.Verified {
		stats.QuorumEndorsed++
	}

	// Update participation EMA (alpha = 0.2 for faster response)
	alpha := math.LegacyNewDecWithPrec(20, 2) // 0.20
	stats.UpdateParticipationEMA(alpha)

	return k.SetValidatorEndorsementStats(ctx, stats)
}

// RecordEndorsementOpportunity records that a validator had an opportunity to endorse
func (k Keeper) RecordEndorsementOpportunity(ctx context.Context, valAddr string) error {
	stats := k.GetValidatorEndorsementStats(ctx, valAddr)
	stats.TotalOpportunity++
	return k.SetValidatorEndorsementStats(ctx, stats)
}

// GetEndorsementParticipationRate returns the endorsement participation rate for a validator
// This is exposed for x/rewardmult to use in PoS multiplier calculations
func (k Keeper) GetEndorsementParticipationRate(ctx context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error) {
	stats := k.GetValidatorEndorsementStats(ctx, valAddr.String())
	return stats.ParticipationEMA, nil
}

// GetValidatorOriginalityMetrics returns the average originality multiplier and quality score
// for contributions endorsed by this validator in the current epoch.
// This is exposed for x/rewardmult to use in quality-adjusted multiplier bonuses.
func (k Keeper) GetValidatorOriginalityMetrics(ctx context.Context, valAddr sdk.ValAddress) (avgOriginality, avgQuality math.LegacyDec, err error) {
	// Defaults: neutral originality (1.0), mid quality (0.5)
	defaultOriginality := math.LegacyOneDec()
	defaultQuality := math.LegacyNewDecWithPrec(5, 1) // 0.5

	params := k.GetParams(ctx)
	epoch := k.GetCurrentEpoch(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Approximate epoch block range (100 blocks per epoch default)
	epochDuration := int64(100)
	if k.epochsKeeper != nil {
		epochDuration = k.epochsKeeper.GetEpochDuration(ctx)
	}
	epochStart := int64(epoch) * epochDuration
	epochEnd := epochStart + epochDuration

	valAddrStr := valAddr.String()
	totalOriginality := math.LegacyZeroDec()
	totalQuality := math.LegacyZeroDec()
	count := int64(0)

	_ = k.IterateContributions(ctx, func(c types.Contribution) bool {
		// Only count verified contributions within this epoch
		if !c.Verified || c.BlockHeight < epochStart || c.BlockHeight >= epochEnd {
			return false
		}

		// Check if this validator endorsed this contribution
		endorsed := false
		for _, e := range c.Endorsements {
			if e.ValAddr == valAddrStr {
				endorsed = true
				break
			}
		}
		if !endorsed {
			return false
		}

		// Compute originality multiplier for this contribution
		sim := math.LegacyZeroDec() // default 0 similarity = fully original
		// Check similarity commitment if available
		if sc, found := k.GetSimilarityCommitment(ctx, c.Id); found {
			// OverallSimilarity is scaled by 10000 (e.g., 8500 = 85.00%)
			sim = math.LegacyNewDec(int64(sc.CompactData.OverallSimilarity)).Quo(math.LegacyNewDec(10000))
		}
		origMult := k.resolveOriginalityMultiplier(params, sim)

		// Quality score: use review quality if available, else default 0.5
		quality := defaultQuality
		if session, found := k.GetReviewSession(ctx, c.Id); found && session.FinalQuality > 0 {
			quality = math.LegacyNewDec(int64(session.FinalQuality)).Quo(math.LegacyNewDec(100))
		}

		totalOriginality = totalOriginality.Add(origMult)
		totalQuality = totalQuality.Add(quality)
		count++
		return false
	})

	if count == 0 {
		return defaultOriginality, defaultQuality, nil
	}

	countDec := math.LegacyNewDec(count)
	avgOriginality = totalOriginality.Quo(countDec)
	avgQuality = totalQuality.Quo(countDec)

	_ = sdkCtx // used for potential logging
	return avgOriginality, avgQuality, nil
}

// CheckValidatorEndorsementQuality checks if a validator is meeting quality thresholds
func (k Keeper) CheckValidatorEndorsementQuality(ctx context.Context, valAddr string) (isFreeriding bool, isQuorumGaming bool) {
	stats := k.GetValidatorEndorsementStats(ctx, valAddr)

	minRate := math.LegacyNewDecWithPrec(types.DefaultMinValidatorParticipationBps, 4) // 20%
	maxQuorumRate := math.LegacyNewDecWithPrec(types.DefaultMaxQuorumEndorsementRateBps, 4) // 70%

	return stats.IsFreeriding(minRate), stats.IsQuorumGaming(maxQuorumRate)
}

// ============================================================================
// Fraud & Rollback Safety (Requirement 6)
// ============================================================================

// GetFrozenCredits retrieves frozen credits for an address
func (k Keeper) GetFrozenCredits(ctx context.Context, addr string) types.FrozenCredits {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetFrozenCreditsKey(addr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.FrozenCredits{Address: addr, Amount: math.ZeroInt()}
	}

	var fc types.FrozenCredits
	if err := fc.Unmarshal(bz); err != nil {
		k.logger.Error("failed to unmarshal frozen credits", "address", addr, "error", err)
		return types.FrozenCredits{Address: addr, Amount: math.ZeroInt()}
	}
	return fc
}

// SetFrozenCredits stores frozen credits
func (k Keeper) SetFrozenCredits(ctx context.Context, fc types.FrozenCredits) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetFrozenCreditsKey(fc.Address)

	bz, err := fc.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal frozen credits: %w", err)
	}

	return store.Set(key, bz)
}

// FreezeCredits freezes credits for an address due to a challenge
func (k Keeper) FreezeCredits(ctx context.Context, addr string, amount math.Int, contributionID uint64, reason string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	fc := types.NewFrozenCredits(addr, amount, contributionID, sdkCtx.BlockHeight(), reason)
	if err := k.SetFrozenCredits(ctx, fc); err != nil {
		return err
	}

	// Emit freeze event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"poc_credits_frozen",
			sdk.NewAttribute("address", addr),
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", contributionID)),
			sdk.NewAttribute("reason", reason),
		),
	)

	return nil
}

// UnfreezeCredits unfreezes credits after challenge resolution
func (k Keeper) UnfreezeCredits(ctx context.Context, addr string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	fc := k.GetFrozenCredits(ctx, addr)
	if fc.Amount.IsZero() {
		return nil // Nothing to unfreeze
	}

	// Clear frozen credits
	fc.Amount = math.ZeroInt()
	if err := k.SetFrozenCredits(ctx, fc); err != nil {
		return err
	}

	// Emit unfreeze event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"poc_credits_unfrozen",
			sdk.NewAttribute("address", addr),
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", fc.ContributionID)),
		),
	)

	return nil
}

// BurnFrozenCredits burns frozen credits when fraud is proven
func (k Keeper) BurnFrozenCredits(ctx context.Context, addr string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	fc := k.GetFrozenCredits(ctx, addr)
	if fc.Amount.IsZero() {
		return nil
	}

	// Get current credits and subtract frozen amount
	accAddr, err := sdk.AccAddressFromBech32(addr)
	if err != nil {
		return err
	}

	credits := k.GetCredits(ctx, accAddr)
	if credits.Amount.LT(fc.Amount) {
		// Can't burn more than available
		credits.Amount = math.ZeroInt()
	} else {
		credits.Amount = credits.Amount.Sub(fc.Amount)
	}

	if err := k.SetCredits(ctx, credits); err != nil {
		return err
	}

	// Clear frozen credits
	burnedAmount := fc.Amount
	fc.Amount = math.ZeroInt()
	if err := k.SetFrozenCredits(ctx, fc); err != nil {
		return err
	}

	// Emit burn event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"poc_credits_burned_fraud",
			sdk.NewAttribute("address", addr),
			sdk.NewAttribute("amount_burned", burnedAmount.String()),
			sdk.NewAttribute("contribution_id", fmt.Sprintf("%d", fc.ContributionID)),
			sdk.NewAttribute("reason", fc.Reason),
		),
	)

	return nil
}

// GetAvailableCredits returns credits minus frozen amount
func (k Keeper) GetAvailableCredits(ctx context.Context, addr sdk.AccAddress) math.Int {
	credits := k.GetCredits(ctx, addr)
	frozen := k.GetFrozenCredits(ctx, addr.String())

	available := credits.Amount.Sub(frozen.Amount)
	if available.IsNegative() {
		return math.ZeroInt()
	}
	return available
}

// GetClaimNonce retrieves the current claim nonce for an address
func (k Keeper) GetClaimNonce(ctx context.Context, addr string) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetClaimNonceKey(addr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return 0
	}

	return sdk.BigEndianToUint64(bz)
}

// IncrementClaimNonce increments the claim nonce for an address
func (k Keeper) IncrementClaimNonce(ctx context.Context, addr string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetClaimNonceKey(addr)

	nonce := k.GetClaimNonce(ctx, addr)
	return store.Set(key, sdk.Uint64ToBigEndian(nonce+1))
}

// ============================================================================
// Decay Mechanism (Credit Decay)
// ============================================================================

// GetLastDecayEpoch retrieves the last epoch when decay was applied
func (k Keeper) GetLastDecayEpoch(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyLastDecayEpoch)
	if err != nil || bz == nil {
		return 0
	}
	return sdk.BigEndianToUint64(bz)
}

// SetLastDecayEpoch stores the last decay epoch
func (k Keeper) SetLastDecayEpoch(ctx context.Context, epoch uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyLastDecayEpoch, sdk.Uint64ToBigEndian(epoch))
}

// ApplyCreditDecay applies decay to all credit balances
// Called at epoch boundaries
func (k Keeper) ApplyCreditDecay(ctx context.Context, currentEpoch uint64) error {
	lastDecay := k.GetLastDecayEpoch(ctx)
	if currentEpoch <= lastDecay {
		return nil // Already applied for this epoch
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Decay rate: 0.5% per epoch = 0.005
	decayRate := math.LegacyNewDecWithPrec(types.DefaultCreditDecayRateBps, 4) // 0.0050

	var totalDecayed math.Int = math.ZeroInt()
	var accountsDecayed uint64 = 0

	// Iterate all credits and apply decay
	err := k.IterateCredits(ctx, func(credits types.Credits) bool {
		if credits.Amount.IsZero() {
			return false
		}

		// Calculate decay amount
		decayAmount := math.LegacyNewDecFromInt(credits.Amount).Mul(decayRate).TruncateInt()
		if decayAmount.IsZero() {
			return false
		}

		// Apply decay
		credits.Amount = credits.Amount.Sub(decayAmount)
		if credits.Amount.IsNegative() {
			credits.Amount = math.ZeroInt()
		}

		// Save updated credits
		if err := k.SetCredits(ctx, credits); err != nil {
			k.logger.Error("failed to save decayed credits", "address", credits.Address, "error", err)
			return false
		}

		// Also decay reputation score
		rs := k.GetReputationScore(ctx, credits.Address)
		rs.ApplyDecay(decayRate)
		if err := k.SetReputationScore(ctx, rs); err != nil {
			k.logger.Error("failed to save decayed reputation", "address", credits.Address, "error", err)
		}

		totalDecayed = totalDecayed.Add(decayAmount)
		accountsDecayed++

		return false
	})

	if err != nil {
		return err
	}

	// Record decay epoch
	if err := k.SetLastDecayEpoch(ctx, currentEpoch); err != nil {
		return err
	}

	// Emit decay event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"poc_credit_decay_applied",
			sdk.NewAttribute("epoch", fmt.Sprintf("%d", currentEpoch)),
			sdk.NewAttribute("total_decayed", totalDecayed.String()),
			sdk.NewAttribute("accounts_affected", fmt.Sprintf("%d", accountsDecayed)),
			sdk.NewAttribute("decay_rate_bps", fmt.Sprintf("%d", types.DefaultCreditDecayRateBps)),
		),
	)

	k.logger.Info("credit decay applied",
		"epoch", currentEpoch,
		"total_decayed", totalDecayed.String(),
		"accounts", accountsDecayed,
	)

	return nil
}
