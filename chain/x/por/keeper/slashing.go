package keeper

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// SlashRecord tracks a slashing event for audit purposes
type SlashRecord struct {
	BatchId      uint64   `json:"batch_id"`
	ChallengeId  uint64   `json:"challenge_id"`
	Verifier     string   `json:"verifier"`
	SlashAmount  math.Int `json:"slash_amount"`
	Reason       string   `json:"reason"`
	Timestamp    int64    `json:"timestamp"`
}

// ProcessValidChallenge handles a challenge that has been validated as correct.
// It rejects the batch, slashes dishonest verifiers, and rewards the challenger.
// SAFETY: All errors are logged, never panicked.
func (k Keeper) ProcessValidChallenge(ctx sdk.Context, challengeID uint64) error {
	challenge, found := k.GetChallenge(ctx, challengeID)
	if !found {
		return types.ErrChallengeNotFound.Wrapf("challenge_id: %d", challengeID)
	}

	if challenge.Status != types.ChallengeStatusOpen {
		return fmt.Errorf("challenge %d is not in OPEN status", challengeID)
	}

	batch, found := k.GetBatch(ctx, challenge.BatchId)
	if !found {
		return types.ErrBatchNotFound.Wrapf("batch_id: %d", challenge.BatchId)
	}

	params := k.GetParams(ctx)
	now := ctx.BlockTime().Unix()

	// 1. Reject the batch
	if err := k.UpdateBatchStatus(ctx, &batch, types.BatchStatusRejected); err != nil {
		return fmt.Errorf("failed to reject batch: %w", err)
	}

	// 2. Resolve the challenge as valid
	challenge.Status = types.ChallengeStatusResolvedValid
	challenge.ResolvedAt = now
	challenge.ResolvedBy = "fraud_proof"
	if err := k.SetChallenge(ctx, challenge); err != nil {
		return fmt.Errorf("failed to update challenge: %w", err)
	}

	// 3. Slash dishonest verifiers who attested to the fraudulent batch
	attestations := k.GetAttestationsForBatch(ctx, challenge.BatchId)
	totalSlashed := math.ZeroInt()

	for _, att := range attestations {
		slashAmount, err := k.slashVerifier(ctx, att.VerifierAddress, params.SlashFractionDishonest, challenge.BatchId, challengeID)
		if err != nil {
			k.Logger().Error("failed to slash verifier",
				"verifier", att.VerifierAddress,
				"batch_id", challenge.BatchId,
				"error", err,
			)
			continue
		}
		totalSlashed = totalSlashed.Add(slashAmount)

		// Update reputation
		rep := k.GetOrCreateVerifierReputation(ctx, att.VerifierAddress)
		rep.SlashedCount++
		rep.ReputationScore = rep.ReputationScore.Sub(math.NewInt(10)) // heavy penalty
		if rep.ReputationScore.IsNegative() {
			rep.ReputationScore = math.ZeroInt()
		}
		if err := k.SetVerifierReputation(ctx, rep); err != nil {
			k.Logger().Error("failed to update slashed verifier reputation",
				"verifier", att.VerifierAddress, "error", err,
			)
		}
	}

	// 4. SECURITY (F4): Refund challenge bond — valid challenge proven
	if challenge.BondAmount.IsPositive() {
		challengerAddr, err := sdk.AccAddressFromBech32(challenge.Challenger)
		if err == nil {
			refundCoins := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, challenge.BondAmount))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, refundCoins); err != nil {
				k.Logger().Error("failed to refund challenge bond",
					"challenger", challenge.Challenger,
					"bond", challenge.BondAmount,
					"error", err,
				)
			} else {
				ctx.EventManager().EmitEvent(sdk.NewEvent(
					"por_challenge_bond_refunded",
					sdk.NewAttribute("challenger", challenge.Challenger),
					sdk.NewAttribute("bond_amount", challenge.BondAmount.String()),
					sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challengeID)),
				))
			}
		}
	}

	// 5. Reward the challenger (fraction of total slashed amount)
	if totalSlashed.IsPositive() && !params.ChallengerRewardRatio.IsZero() {
		rewardDec := params.ChallengerRewardRatio.MulInt(totalSlashed)
		rewardAmount := rewardDec.TruncateInt()

		if rewardAmount.IsPositive() {
			challengerAddr, err := sdk.AccAddressFromBech32(challenge.Challenger)
			if err == nil {
				reward := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, rewardAmount))
				if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, reward); err != nil {
					k.Logger().Error("failed to reward challenger",
						"challenger", challenge.Challenger,
						"reward", rewardAmount,
						"error", err,
					)
				} else {
					ctx.EventManager().EmitEvent(sdk.NewEvent(
						"por_challenger_rewarded",
						sdk.NewAttribute("challenger", challenge.Challenger),
						sdk.NewAttribute("reward", rewardAmount.String()),
						sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challengeID)),
					))
				}
			}
		}
	}

	k.Logger().Info("valid challenge processed",
		"challenge_id", challengeID,
		"batch_id", challenge.BatchId,
		"verifiers_slashed", len(attestations),
		"total_slashed", totalSlashed,
	)

	// Emit events
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"por_batch_rejected",
		sdk.NewAttribute("batch_id", fmt.Sprintf("%d", challenge.BatchId)),
		sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challengeID)),
		sdk.NewAttribute("challenger", challenge.Challenger),
		sdk.NewAttribute("verifiers_slashed", fmt.Sprintf("%d", len(attestations))),
		sdk.NewAttribute("total_slashed", totalSlashed.String()),
	))

	return nil
}

// slashVerifier slashes a verifier's stake and records the slashing event.
// Returns the slashed amount. If slashing keeper is not available, returns zero.
func (k Keeper) slashVerifier(ctx sdk.Context, verifierAddr string, fraction math.LegacyDec, batchID, challengeID uint64) (math.Int, error) {
	// Record slash for audit trail regardless of whether slashing keeper is available
	record := SlashRecord{
		BatchId:     batchID,
		ChallengeId: challengeID,
		Verifier:    verifierAddr,
		SlashAmount: math.ZeroInt(),
		Reason:      "dishonest_attestation",
		Timestamp:   ctx.BlockTime().Unix(),
	}

	// If slashing keeper is available, perform the actual slash
	if k.slashingKeeper != nil {
		// SECURITY (re-audit): Verifier addresses are stored as AccAddress (omni1...).
		// Convert AccAddress bytes to ValAddress for staking/slashing lookup.
		// ValAddressFromBech32 expects omnivaloper1... prefix and will always fail here.
		accAddr, err := sdk.AccAddressFromBech32(verifierAddr)
		if err != nil {
			k.Logger().Debug("invalid verifier address, skipping on-chain slash",
				"verifier", verifierAddr,
			)
			k.saveSlashRecord(ctx, record)
			return math.ZeroInt(), nil
		}
		valAddr := sdk.ValAddress(accAddr)
		consAddr := sdk.ConsAddress(valAddr)
		slashed, err := k.slashingKeeper.SlashWithInfractionReason(
			ctx,
			consAddr,
			fraction,
			0, // power
			ctx.BlockHeight(),
			"por_dishonest_attestation",
		)
		if err != nil {
			k.saveSlashRecord(ctx, record)
			return math.ZeroInt(), fmt.Errorf("slashing failed for %s: %w", verifierAddr, err)
		}

		// Jail the validator to prevent immediate resumption
		if err := k.slashingKeeper.Jail(ctx, consAddr); err != nil {
			k.Logger().Error("failed to jail fraudulent verifier",
				"verifier", verifierAddr,
				"error", err,
			)
		}

		record.SlashAmount = slashed
		k.saveSlashRecord(ctx, record)
		return slashed, nil
	}

	// No slashing keeper available - just record the event
	k.saveSlashRecord(ctx, record)
	return math.ZeroInt(), nil
}

// saveSlashRecord stores a slash record for audit purposes
func (k Keeper) saveSlashRecord(ctx sdk.Context, record SlashRecord) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		k.Logger().Error("failed to marshal slash record", "error", err)
		return
	}
	key := types.GetSlashRecordKey(record.BatchId, record.Verifier)
	if err := kvStore.Set(key, bz); err != nil {
		k.Logger().Error("failed to store slash record", "error", err)
	}
}

// ProcessInvalidChallenge handles a challenge that was proven to be invalid (frivolous).
func (k Keeper) ProcessInvalidChallenge(ctx sdk.Context, challengeID uint64) error {
	challenge, found := k.GetChallenge(ctx, challengeID)
	if !found {
		return types.ErrChallengeNotFound.Wrapf("challenge_id: %d", challengeID)
	}

	if challenge.Status != types.ChallengeStatusOpen {
		return fmt.Errorf("challenge %d is not in OPEN status", challengeID)
	}

	now := ctx.BlockTime().Unix()

	// Mark challenge as resolved invalid
	challenge.Status = types.ChallengeStatusResolvedInvalid
	challenge.ResolvedAt = now
	challenge.ResolvedBy = "challenge_rejected"
	if err := k.SetChallenge(ctx, challenge); err != nil {
		return fmt.Errorf("failed to update challenge: %w", err)
	}

	// SECURITY (F4): Burn the challenge bond — frivolous challenge penalized
	if challenge.BondAmount.IsPositive() {
		burnCoins := sdk.NewCoins(sdk.NewCoin(k.GetParams(ctx).RewardDenom, challenge.BondAmount))
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
			k.Logger().Error("failed to burn challenge bond",
				"challenger", challenge.Challenger,
				"bond", challenge.BondAmount,
				"error", err,
			)
		} else {
			ctx.EventManager().EmitEvent(sdk.NewEvent(
				"por_challenge_bond_burned",
				sdk.NewAttribute("challenger", challenge.Challenger),
				sdk.NewAttribute("bond_amount", challenge.BondAmount.String()),
				sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challengeID)),
			))
		}
	}

	k.Logger().Info("invalid challenge processed",
		"challenge_id", challengeID,
		"batch_id", challenge.BatchId,
		"challenger", challenge.Challenger,
	)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"por_challenge_rejected",
		sdk.NewAttribute("challenge_id", fmt.Sprintf("%d", challengeID)),
		sdk.NewAttribute("batch_id", fmt.Sprintf("%d", challenge.BatchId)),
		sdk.NewAttribute("challenger", challenge.Challenger),
	))

	return nil
}
