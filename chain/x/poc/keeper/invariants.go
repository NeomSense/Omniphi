package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// RegisterInvariants registers all poc invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "credits-non-negative", CreditsNonNegativeInvariant(k))
	ir.RegisterRoute(types.ModuleName, "contribution-integrity", ContributionIntegrityInvariant(k))
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
		return msg, broken
	}
}
