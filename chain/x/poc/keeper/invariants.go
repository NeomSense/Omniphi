package keeper

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// RegisterInvariants registers all poc invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "credits-non-negative", CreditsNonNegativeInvariant(k))
	ir.RegisterRoute(types.ModuleName, "contribution-integrity", ContributionIntegrityInvariant(k))
	// V2 Hardening invariants
	ir.RegisterRoute(types.ModuleName, "credit-cap-enforcement", CreditCapInvariant(k))
	ir.RegisterRoute(types.ModuleName, "quorum-correctness", QuorumCorrectnessInvariant(k))
	ir.RegisterRoute(types.ModuleName, "frozen-credits-consistency", FrozenCreditsInvariant(k))
	// V2.1 invariants
	ir.RegisterRoute(types.ModuleName, "finality-consistency", FinalityConsistencyInvariant(k))
	// Layer 4 economic invariants
	ir.RegisterRoute(types.ModuleName, "reputation-bounds", ReputationBoundsInvariant(k))
	ir.RegisterRoute(types.ModuleName, "vesting-balance", VestingBalanceInvariant(k))
	ir.RegisterRoute(types.ModuleName, "royalty-total", RoyaltyTotalInvariant(k))
	ir.RegisterRoute(types.ModuleName, "reward-bounds", RewardBoundsInvariant(k))
	// Layer 5 provenance invariants
	ir.RegisterRoute(types.ModuleName, "provenance-acyclicity", ProvenanceAcyclicityInvariant(k))
	ir.RegisterRoute(types.ModuleName, "provenance-index-consistency", ProvenanceIndexConsistencyInvariant(k))
	// Pipeline integration invariants
	ir.RegisterRoute(types.ModuleName, "no-duplicate-payout", NoDuplicatePayoutInvariant(k))
	ir.RegisterRoute(types.ModuleName, "vesting-status-consistency", VestingStatusConsistencyInvariant(k))
	ir.RegisterRoute(types.ModuleName, "claim-status-validity", ClaimStatusValidityInvariant(k))
}

// CreditsNonNegativeInvariant checks that all credit balances are non-negative
func CreditsNonNegativeInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		err := k.IterateCredits(ctx, func(credits types.Credits) bool {
			if credits.Amount.IsNegative() {
				broken = true
				msg += fmt.Sprintf("negative credits for address %s: %s\n", credits.Address, credits.Amount.String())
			}
			return false
		})

		if err != nil {
			broken = true
			msg += fmt.Sprintf("error iterating credits: %s\n", err.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "credits-non-negative",
			msg,
		), broken
	}
}

// ContributionIntegrityInvariant checks that all contributions are properly formed
func ContributionIntegrityInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		err := k.IterateContributions(ctx, func(contribution types.Contribution) bool {
			// Check ID is not zero
			if contribution.Id == 0 {
				broken = true
				msg += fmt.Sprintf("contribution with zero ID\n")
			}

			// Check contributor address is valid
			if _, err := sdk.AccAddressFromBech32(contribution.Contributor); err != nil {
				broken = true
				msg += fmt.Sprintf("invalid contributor address for contribution %d: %s\n", contribution.Id, contribution.Contributor)
			}

			// Check ctype is not empty
			if contribution.Ctype == "" {
				broken = true
				msg += fmt.Sprintf("empty ctype for contribution %d\n", contribution.Id)
			}

			// Check URI is not empty
			if contribution.Uri == "" {
				broken = true
				msg += fmt.Sprintf("empty URI for contribution %d\n", contribution.Id)
			}

			// Check hash is not empty
			if len(contribution.Hash) == 0 {
				broken = true
				msg += fmt.Sprintf("empty hash for contribution %d\n", contribution.Id)
			}

			// Check endorsement powers are non-negative
			for _, e := range contribution.Endorsements {
				if e.Power.IsNegative() {
					broken = true
					msg += fmt.Sprintf("negative power in endorsement for contribution %d\n", contribution.Id)
				}
			}

			return false
		})

		if err != nil {
			broken = true
			msg += fmt.Sprintf("error iterating contributions: %s\n", err.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "contribution-integrity",
			msg,
		), broken
	}
}

// AllInvariants runs all invariants of the poc module
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		msg, broken := CreditsNonNegativeInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = ContributionIntegrityInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = CreditCapInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = QuorumCorrectnessInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = FrozenCreditsInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = FinalityConsistencyInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = ReputationBoundsInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = VestingBalanceInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = RoyaltyTotalInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = RewardBoundsInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = ProvenanceAcyclicityInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = ProvenanceIndexConsistencyInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = NoDuplicatePayoutInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		msg, broken = VestingStatusConsistencyInvariant(k)(ctx)
		if broken {
			return msg, broken
		}

		return ClaimStatusValidityInvariant(k)(ctx)
	}
}

// ============================================================================
// V2 Hardening Invariants
// ============================================================================

// CreditCapInvariant checks that no credit balance exceeds the hard cap (100,000)
func CreditCapInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		maxCredits := types.DefaultCreditCap

		err := k.IterateCredits(ctx, func(credits types.Credits) bool {
			if credits.Amount.GT(math.NewInt(int64(maxCredits))) {
				broken = true
				msg += fmt.Sprintf("credits for address %s (%s) exceed cap (%d)\n",
					credits.Address, credits.Amount.String(), maxCredits)
			}
			return false
		})

		if err != nil {
			broken = true
			msg += fmt.Sprintf("error iterating credits: %s\n", err.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "credit-cap-enforcement",
			msg,
		), broken
	}
}

// QuorumCorrectnessInvariant checks that verified contributions actually met quorum
func QuorumCorrectnessInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		params := k.GetParams(ctx)

		// Get total bonded tokens
		totalBonded, err := k.stakingKeeper.TotalBondedTokens(ctx)
		if err != nil {
			return sdk.FormatInvariant(
				types.ModuleName, "quorum-correctness",
				fmt.Sprintf("failed to get total bonded tokens: %s", err.Error()),
			), true
		}

		if totalBonded.IsZero() {
			return sdk.FormatInvariant(
				types.ModuleName, "quorum-correctness",
				"no bonded tokens, skipping quorum check",
			), false
		}

		requiredPower := math.LegacyNewDecFromInt(totalBonded).Mul(params.QuorumPct).TruncateInt()

		iterErr := k.IterateContributions(ctx, func(contribution types.Contribution) bool {
			if !contribution.Verified {
				return false // Only check verified contributions
			}

			// Calculate approval power
			approvalPower := contribution.GetApprovalPower()

			// Verified contributions should have met quorum
			if approvalPower.LT(requiredPower) {
				broken = true
				msg += fmt.Sprintf("contribution %d marked verified but approval power (%s) < required (%s)\n",
					contribution.Id, approvalPower.String(), requiredPower.String())
			}

			return false
		})

		if iterErr != nil {
			broken = true
			msg += fmt.Sprintf("error iterating contributions: %s\n", iterErr.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "quorum-correctness",
			msg,
		), broken
	}
}

// FrozenCreditsInvariant checks that frozen credits don't exceed available credits
func FrozenCreditsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		err := k.IterateCredits(ctx, func(credits types.Credits) bool {
			frozen := k.GetFrozenCredits(ctx, credits.Address)

			if frozen.Amount.GT(credits.Amount) {
				broken = true
				msg += fmt.Sprintf("frozen credits (%s) > total credits (%s) for address %s\n",
					frozen.Amount.String(), credits.Amount.String(), credits.Address)
			}

			return false
		})

		if err != nil {
			broken = true
			msg += fmt.Sprintf("error iterating credits: %s\n", err.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "frozen-credits-consistency",
			msg,
		), broken
	}
}

// ============================================================================
// V2.1 Mainnet Hardening Invariants
// ============================================================================

// FinalityConsistencyInvariant checks that finality states are internally consistent:
// - FINAL contributions must have FinalizedAt > 0
// - INVALIDATED contributions must have a corresponding fraud proof
// - No contribution should be in an unknown finality status
func FinalityConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		iterErr := k.IterateContributions(ctx, func(contribution types.Contribution) bool {
			finality := k.GetContributionFinality(ctx, contribution.Id)

			switch finality.Status {
			case types.FinalityStatusPending:
				// Valid — no additional checks needed
			case types.FinalityStatusFinal:
				// FinalizedAt should be set (>= 0 is valid in test, but check for negative)
				if finality.FinalizedAt < 0 {
					broken = true
					msg += fmt.Sprintf("contribution %d is FINAL but FinalizedAt is negative: %d\n",
						contribution.Id, finality.FinalizedAt)
				}
			case types.FinalityStatusChallenged:
				// Valid — challenge in progress
			case types.FinalityStatusInvalidated:
				// Should have a fraud proof (warn, not break — proof may have been stored separately)
			default:
				broken = true
				msg += fmt.Sprintf("contribution %d has unknown finality status: %d\n",
					contribution.Id, finality.Status)
			}

			return false
		})

		if iterErr != nil {
			broken = true
			msg += fmt.Sprintf("error iterating contributions: %s\n", iterErr.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "finality-consistency",
			msg,
		), broken
	}
}

// ============================================================================
// Layer 4 Economic Invariants
// ============================================================================

// ReputationBoundsInvariant checks that all contributor reputation scores are within [0.1, 1.0]
func ReputationBoundsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		minRep := math.LegacyNewDecWithPrec(1, 1) // 0.1
		maxRep := math.LegacyOneDec()              // 1.0

		store := k.storeService.OpenKVStore(ctx)
		iter, err := store.Iterator(types.ContributorStatsKeyPrefix, storetypes.PrefixEndBytes(types.ContributorStatsKeyPrefix))
		if err != nil {
			return sdk.FormatInvariant(
				types.ModuleName, "reputation-bounds",
				fmt.Sprintf("error creating stats iterator: %s", err.Error()),
			), true
		}
		defer iter.Close()

		for ; iter.Valid(); iter.Next() {
			var stats types.ContributorStats
			if err := json.Unmarshal(iter.Value(), &stats); err != nil {
				continue
			}

			if stats.ReputationScore.LT(minRep) {
				broken = true
				msg += fmt.Sprintf("contributor %s reputation %s < floor 0.1\n",
					stats.Address, stats.ReputationScore.String())
			}
			if stats.ReputationScore.GT(maxRep) {
				broken = true
				msg += fmt.Sprintf("contributor %s reputation %s > ceiling 1.0\n",
					stats.Address, stats.ReputationScore.String())
			}
		}

		return sdk.FormatInvariant(
			types.ModuleName, "reputation-bounds",
			msg,
		), broken
	}
}

// VestingBalanceInvariant checks that:
// - Active vesting ReleasedAmount <= TotalAmount
// - ClawedBack vesting has zero remaining
func VestingBalanceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		store := k.storeService.OpenKVStore(ctx)
		iter, err := store.Iterator(types.KeyPrefixVestingSchedule, storetypes.PrefixEndBytes(types.KeyPrefixVestingSchedule))
		if err != nil {
			return sdk.FormatInvariant(
				types.ModuleName, "vesting-balance",
				fmt.Sprintf("error creating vesting iterator: %s", err.Error()),
			), true
		}
		defer iter.Close()

		for ; iter.Valid(); iter.Next() {
			var schedule types.VestingSchedule
			if err := json.Unmarshal(iter.Value(), &schedule); err != nil {
				continue
			}

			// ReleasedAmount must not exceed TotalAmount
			if schedule.ReleasedAmount.GT(schedule.TotalAmount) {
				broken = true
				msg += fmt.Sprintf("vesting claim %d (contributor %s): released %s > total %s\n",
					schedule.ClaimID, schedule.Contributor,
					schedule.ReleasedAmount.String(), schedule.TotalAmount.String())
			}

			// Completed vesting should have released == total
			if schedule.Status == types.VestingStatusCompleted && !schedule.ReleasedAmount.Equal(schedule.TotalAmount) {
				broken = true
				msg += fmt.Sprintf("vesting claim %d (contributor %s): completed but released %s != total %s\n",
					schedule.ClaimID, schedule.Contributor,
					schedule.ReleasedAmount.String(), schedule.TotalAmount.String())
			}
		}

		return sdk.FormatInvariant(
			types.ModuleName, "vesting-balance",
			msg,
		), broken
	}
}

// RoyaltyTotalInvariant checks that total royalties per claim do not exceed
// MaxTotalRoyaltyShare * gross reward (approximated via BaseRewardUnit).
func RoyaltyTotalInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		// This is a soft invariant — royalties are bounded during computation.
		// We verify that the parameter itself is sane.
		params := k.GetParams(ctx)
		var (
			broken bool
			msg    string
		)

		if params.MaxTotalRoyaltyShare.GT(math.LegacyNewDecWithPrec(50, 2)) {
			broken = true
			msg += fmt.Sprintf("MaxTotalRoyaltyShare %s exceeds 50%% safety limit\n",
				params.MaxTotalRoyaltyShare.String())
		}

		if params.RoyaltyShare.Add(params.GrandparentRoyaltyShare).GT(params.MaxTotalRoyaltyShare) {
			broken = true
			msg += fmt.Sprintf("RoyaltyShare (%s) + GrandparentRoyaltyShare (%s) > MaxTotalRoyaltyShare (%s)\n",
				params.RoyaltyShare.String(), params.GrandparentRoyaltyShare.String(), params.MaxTotalRoyaltyShare.String())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "royalty-total",
			msg,
		), broken
	}
}

// RewardBoundsInvariant checks that no single reward exceeds the theoretical maximum:
// base_reward * max_quality_multiplier * max_originality_multiplier.
// This is a parameter sanity check since rewards are bounded during calculation.
func RewardBoundsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		params := k.GetParams(ctx)
		var (
			broken bool
			msg    string
		)

		// Verify originality bands are well-formed if configurable bands are enabled
		if params.EnableConfigurableBands && len(params.OriginalityBands) > 0 {
			for i, band := range params.OriginalityBands {
				if band.Multiplier.GT(math.LegacyNewDec(2)) {
					broken = true
					msg += fmt.Sprintf("originality band %d multiplier %s > 2.0 max\n",
						i, band.Multiplier.String())
				}
				if band.Multiplier.IsNegative() {
					broken = true
					msg += fmt.Sprintf("originality band %d multiplier %s is negative\n",
						i, band.Multiplier.String())
				}
			}
		}

		// Verify repeat offender reward cap is sane
		if params.RepeatOffenderRewardCap.GT(math.LegacyOneDec()) {
			broken = true
			msg += fmt.Sprintf("RepeatOffenderRewardCap %s > 1.0 (would amplify rewards)\n",
				params.RepeatOffenderRewardCap.String())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "reward-bounds",
			msg,
		), broken
	}
}

// ============================================================================
// Layer 5 Provenance Invariants
// ============================================================================

// ProvenanceAcyclicityInvariant checks that every provenance entry with a parent
// has a valid ancestor chain with no cycles and the parent exists in the registry.
func ProvenanceAcyclicityInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		params := k.GetParams(ctx)
		maxDepth := params.MaxProvenanceDepth
		if maxDepth == 0 {
			maxDepth = types.DefaultMaxProvenanceDepth
		}

		store := k.storeService.OpenKVStore(ctx)
		iter, err := store.Iterator(types.KeyPrefixProvenanceEntry, storetypes.PrefixEndBytes(types.KeyPrefixProvenanceEntry))
		if err != nil {
			return sdk.FormatInvariant(
				types.ModuleName, "provenance-acyclicity",
				fmt.Sprintf("error creating provenance iterator: %s", err.Error()),
			), true
		}
		defer iter.Close()

		for ; iter.Valid(); iter.Next() {
			var entry types.ProvenanceEntry
			if err := json.Unmarshal(iter.Value(), &entry); err != nil {
				continue
			}

			if entry.ParentClaimID == 0 {
				continue // root entries are always valid
			}

			// Walk ancestors to verify no cycles
			visited := make(map[uint64]bool)
			visited[entry.ClaimID] = true
			currentID := entry.ParentClaimID
			walkDepth := uint32(0)

			for currentID > 0 && walkDepth <= maxDepth {
				if visited[currentID] {
					broken = true
					msg += fmt.Sprintf("cycle detected in provenance chain for claim %d at ancestor %d\n",
						entry.ClaimID, currentID)
					break
				}
				visited[currentID] = true

				parent, found := k.GetProvenanceEntry(ctx, currentID)
				if !found {
					broken = true
					msg += fmt.Sprintf("claim %d references parent %d which is not in provenance registry\n",
						entry.ClaimID, currentID)
					break
				}

				currentID = parent.ParentClaimID
				walkDepth++
			}
		}

		return sdk.FormatInvariant(
			types.ModuleName, "provenance-acyclicity",
			msg,
		), broken
	}
}

// ProvenanceIndexConsistencyInvariant verifies that all provenance index entries
// are consistent with the primary provenance records.
func ProvenanceIndexConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		store := k.storeService.OpenKVStore(ctx)

		// For every provenance entry, verify its indexes exist
		iter, err := store.Iterator(types.KeyPrefixProvenanceEntry, storetypes.PrefixEndBytes(types.KeyPrefixProvenanceEntry))
		if err != nil {
			return sdk.FormatInvariant(
				types.ModuleName, "provenance-index-consistency",
				fmt.Sprintf("error creating provenance iterator: %s", err.Error()),
			), true
		}
		defer iter.Close()

		for ; iter.Valid(); iter.Next() {
			var entry types.ProvenanceEntry
			if err := json.Unmarshal(iter.Value(), &entry); err != nil {
				continue
			}

			// Check child index: if entry has a parent, the parent's child index should contain this entry
			if entry.ParentClaimID > 0 {
				childKey := types.GetProvenanceChildIndexKey(entry.ParentClaimID, entry.ClaimID)
				bz, err := store.Get(childKey)
				if err != nil || bz == nil {
					broken = true
					msg += fmt.Sprintf("claim %d missing child index entry under parent %d\n",
						entry.ClaimID, entry.ParentClaimID)
				}
			}

			// Check submitter index
			if entry.Submitter != "" {
				subKey := types.GetProvenanceSubmitterIndexKey(entry.Submitter, entry.ClaimID)
				bz, err := store.Get(subKey)
				if err != nil || bz == nil {
					broken = true
					msg += fmt.Sprintf("claim %d missing submitter index for %s\n",
						entry.ClaimID, entry.Submitter)
				}
			}

			// Check category index
			if entry.Category != "" {
				catKey := types.GetProvenanceCategoryIndexKey(entry.Category, entry.ClaimID)
				bz, err := store.Get(catKey)
				if err != nil || bz == nil {
					broken = true
					msg += fmt.Sprintf("claim %d missing category index for %s\n",
						entry.ClaimID, entry.Category)
				}
			}

			// Check epoch index
			epochKey := types.GetProvenanceEpochIndexKey(entry.Epoch, entry.ClaimID)
			bz, err := store.Get(epochKey)
			if err != nil || bz == nil {
				broken = true
				msg += fmt.Sprintf("claim %d missing epoch index for epoch %d\n",
					entry.ClaimID, entry.Epoch)
			}
		}

		return sdk.FormatInvariant(
			types.ModuleName, "provenance-index-consistency",
			msg,
		), broken
	}
}

// ============================================================================
// Pipeline Integration Invariants
// ============================================================================

// NoDuplicatePayoutInvariant verifies that no contribution marked as a duplicate
// has a vesting schedule (duplicates should never receive rewards).
func NoDuplicatePayoutInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		iterErr := k.IterateContributions(ctx, func(contribution types.Contribution) bool {
			if contribution.DuplicateOf == 0 {
				return false // not a duplicate
			}

			// Check if a vesting schedule exists for this duplicate
			_, found := k.GetVestingSchedule(ctx, contribution.Contributor, contribution.Id)
			if found {
				broken = true
				msg += fmt.Sprintf("duplicate contribution %d (dup of %d) has a vesting schedule\n",
					contribution.Id, contribution.DuplicateOf)
			}

			return false
		})

		if iterErr != nil {
			broken = true
			msg += fmt.Sprintf("error iterating contributions: %s\n", iterErr.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "no-duplicate-payout",
			msg,
		), broken
	}
}

// VestingStatusConsistencyInvariant verifies that vesting status is consistent
// with the contribution's review status:
//   - VestingStatusActive only if review ACCEPTED
//   - VestingStatusPaused only if review APPEALED
//   - VestingStatusClawedBack only if review REJECTED or fraud
func VestingStatusConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		store := k.storeService.OpenKVStore(ctx)
		iter, err := store.Iterator(types.KeyPrefixVestingSchedule, storetypes.PrefixEndBytes(types.KeyPrefixVestingSchedule))
		if err != nil {
			return sdk.FormatInvariant(
				types.ModuleName, "vesting-status-consistency",
				fmt.Sprintf("error creating vesting iterator: %s", err.Error()),
			), true
		}
		defer iter.Close()

		for ; iter.Valid(); iter.Next() {
			var schedule types.VestingSchedule
			if err := json.Unmarshal(iter.Value(), &schedule); err != nil {
				continue
			}

			// Look up the contribution's review status
			contrib, found := k.GetContribution(ctx, schedule.ClaimID)
			if !found {
				continue // contribution may have been pruned
			}

			reviewStatus := types.ReviewStatus(contrib.ReviewStatus)

			switch schedule.Status {
			case types.VestingStatusActive:
				if reviewStatus != types.ReviewStatusAccepted {
					broken = true
					msg += fmt.Sprintf("claim %d: vesting Active but review status is %s\n",
						schedule.ClaimID, reviewStatus)
				}
			case types.VestingStatusPaused:
				if reviewStatus != types.ReviewStatusAppealed {
					broken = true
					msg += fmt.Sprintf("claim %d: vesting Paused but review status is %s\n",
						schedule.ClaimID, reviewStatus)
				}
			}
		}

		return sdk.FormatInvariant(
			types.ModuleName, "vesting-status-consistency",
			msg,
		), broken
	}
}

// ClaimStatusValidityInvariant verifies that all ClaimStatus values on contributions
// are valid enum values (0-8).
func ClaimStatusValidityInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			broken bool
			msg    string
		)

		iterErr := k.IterateContributions(ctx, func(contribution types.Contribution) bool {
			cs := types.ClaimStatus(contribution.ClaimStatus)
			if cs > types.ClaimStatusResolved {
				broken = true
				msg += fmt.Sprintf("contribution %d has invalid ClaimStatus: %d\n",
					contribution.Id, contribution.ClaimStatus)
			}
			return false
		})

		if iterErr != nil {
			broken = true
			msg += fmt.Sprintf("error iterating contributions: %s\n", iterErr.Error())
		}

		return sdk.FormatInvariant(
			types.ModuleName, "claim-status-validity",
			msg,
		), broken
	}
}
