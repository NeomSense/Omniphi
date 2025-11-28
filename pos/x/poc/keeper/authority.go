package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// CheckProofOfAuthority performs comprehensive PoA verification for a contribution submission
// This is the second layer of the three-layer verification system (PoE → PoA → PoV)
//
// PoA Layer checks:
// 1. C-Score requirements (reputation gating)
// 2. Identity verification requirements (KYC/DID gating)
// 3. Exemption checks (governance/emergency bypass)
//
// Security: All checks are deterministic and gas-metered
// Performance: O(1) lookups, minimal state reads
func (k Keeper) CheckProofOfAuthority(ctx context.Context, contributor sdk.AccAddress, ctype string) error {
	params := k.GetParams(ctx)

	// 1. Check if contributor is exempt from all PoA checks
	if k.IsExemptAddress(ctx, contributor) {
		k.Logger().Debug("contributor exempt from PoA checks",
			"contributor", contributor.String(),
			"ctype", ctype,
		)
		return nil
	}

	// 2. Check minimum C-Score requirement (if enabled)
	if params.EnableCscoreGating {
		if err := k.CheckMinimumCScore(ctx, contributor, ctype); err != nil {
			return err
		}
	}

	// 3. Check identity verification requirement (if enabled)
	if params.EnableIdentityGating {
		if err := k.CheckIdentityRequirement(ctx, contributor, ctype); err != nil {
			return err
		}
	}

	return nil
}

// CheckMinimumCScore verifies contributor meets minimum C-Score for the contribution type
//
// Algorithm:
// 1. Lookup required C-Score for ctype from params
// 2. If no requirement exists, allow (permissionless)
// 3. Get contributor's current C-Score from state
// 4. Compare and enforce minimum threshold
//
// Gas cost: ~5,000 gas (2 state reads + math operations)
func (k Keeper) CheckMinimumCScore(ctx context.Context, contributor sdk.AccAddress, ctype string) error {
	// Get required C-Score for this contribution type
	requiredScore, hasRequirement := k.GetRequiredCScore(ctx, ctype)
	if !hasRequirement {
		// No requirement for this type - allow submission
		return nil
	}

	// Get contributor's current C-Score
	credits := k.GetCredits(ctx, contributor)
	currentScore := credits.Amount

	// Enforce minimum threshold
	if currentScore.LT(requiredScore) {
		k.Logger().Info("insufficient C-Score for contribution type",
			"contributor", contributor.String(),
			"ctype", ctype,
			"required", requiredScore.String(),
			"current", currentScore.String(),
		)

		return types.ErrInsufficientCScore.Wrapf(
			"contribution type '%s' requires %s C-Score, you have %s (need %s more)",
			ctype,
			requiredScore.String(),
			currentScore.String(),
			requiredScore.Sub(currentScore).String(),
		)
	}

	k.Logger().Debug("C-Score requirement met",
		"contributor", contributor.String(),
		"ctype", ctype,
		"required", requiredScore.String(),
		"current", currentScore.String(),
	)

	return nil
}

// GetRequiredCScore retrieves the minimum C-Score requirement for a contribution type
//
// Returns:
// - requiredScore: The minimum C-Score needed
// - hasRequirement: true if a requirement exists, false if type is unrestricted
//
// Gas cost: ~2,000 gas (1 param read + map lookup)
func (k Keeper) GetRequiredCScore(ctx context.Context, ctype string) (math.Int, bool) {
	params := k.GetParams(ctx)

	// Check if gating is enabled
	if !params.EnableCscoreGating {
		return math.ZeroInt(), false
	}

	// Lookup requirement in map
	requiredScore, exists := params.MinCscoreForCtype[ctype]
	if !exists {
		// No requirement for this type
		return math.ZeroInt(), false
	}

	return requiredScore, true
}

// CheckIdentityRequirement verifies identity verification for contribution types that require it
//
// Algorithm:
// 1. Check if identity verification is required for this ctype
// 2. If required, verify with identity keeper (if available)
// 3. Graceful fallback if x/identity module not loaded
//
// Security:
// - Fail-safe: If identity module unavailable, reject submissions requiring identity
// - Deterministic: No external calls, all checks on-chain
// - Gas metered: Early returns minimize cost
//
// Gas cost: ~3,000 gas (param read + identity check)
func (k Keeper) CheckIdentityRequirement(ctx context.Context, contributor sdk.AccAddress, ctype string) error {
	params := k.GetParams(ctx)

	// Check if this contribution type requires identity verification
	requiresIdentity, exists := params.RequireIdentityForCtype[ctype]
	if !exists || !requiresIdentity {
		// No identity requirement for this type
		return nil
	}

	// Identity verification required - check if module is available
	if k.identityKeeper != nil {
		// x/identity module is available - perform verification
		if !k.identityKeeper.IsVerified(ctx, contributor) {
			k.Logger().Info("identity verification failed for contributor",
				"contributor", contributor.String(),
				"ctype", ctype,
			)

			return types.ErrIdentityNotVerified.Wrapf(
				"contribution type '%s' requires verified identity (KYC/DID)",
				ctype,
			)
		}

		// Identity verified successfully
		k.Logger().Debug("identity verification passed",
			"contributor", contributor.String(),
			"ctype", ctype,
		)
		return nil
	}

	// x/identity module not available - fail-safe rejection
	k.Logger().Warn("identity verification required but x/identity module not available",
		"contributor", contributor.String(),
		"ctype", ctype,
	)

	return types.ErrIdentityCheckFailed.Wrapf(
		"contribution type '%s' requires verified identity, but identity module is not available",
		ctype,
	)
}

// IsExemptAddress checks if an address is exempt from PoA verification
//
// Exempt addresses bypass:
// - C-Score requirements
// - Identity verification
//
// Use cases:
// - Governance multisig addresses
// - Emergency response accounts
// - Grandfathered historical contributors
//
// Security: Only governance can add/remove exempt addresses via param updates
//
// Gas cost: ~1,500 gas (param read + linear search)
// Note: Exempt list expected to be very small (<10 addresses)
func (k Keeper) IsExemptAddress(ctx context.Context, addr sdk.AccAddress) bool {
	params := k.GetParams(ctx)

	addrStr := addr.String()
	for _, exemptAddr := range params.ExemptAddresses {
		if exemptAddr == addrStr {
			return true
		}
	}

	return false
}

// GetCScoreRequirements returns all C-Score requirements as a map for query endpoints
//
// Returns: map[ctype]required_score
// Gas cost: ~2,000 gas
func (k Keeper) GetCScoreRequirements(ctx context.Context) map[string]math.Int {
	params := k.GetParams(ctx)

	if !params.EnableCscoreGating {
		return make(map[string]math.Int)
	}

	// Return a copy to prevent external modification
	requirements := make(map[string]math.Int, len(params.MinCscoreForCtype))
	for ctype, score := range params.MinCscoreForCtype {
		requirements[ctype] = score
	}

	return requirements
}

// GetIdentityRequirements returns all identity requirements as a map for query endpoints
//
// Returns: map[ctype]requires_identity
// Gas cost: ~2,000 gas
func (k Keeper) GetIdentityRequirements(ctx context.Context) map[string]bool {
	params := k.GetParams(ctx)

	if !params.EnableIdentityGating {
		return make(map[string]bool)
	}

	// Return a copy to prevent external modification
	requirements := make(map[string]bool, len(params.RequireIdentityForCtype))
	for ctype, required := range params.RequireIdentityForCtype {
		requirements[ctype] = required
	}

	return requirements
}

// CanSubmitContribution is a read-only check to determine if a contributor can submit a given ctype
//
// This is useful for:
// - CLI/UI validation before submitting transaction
// - Query endpoints for contributor dashboards
// - Frontend gating logic
//
// Returns:
// - canSubmit: true if all PoA checks would pass
// - reason: human-readable explanation if canSubmit is false
//
// Gas cost: ~8,000 gas (comprehensive check simulation)
func (k Keeper) CanSubmitContribution(ctx context.Context, contributor sdk.AccAddress, ctype string) (canSubmit bool, reason string) {
	// Check if exempt (bypass all checks)
	if k.IsExemptAddress(ctx, contributor) {
		return true, "contributor is exempt from access control"
	}

	params := k.GetParams(ctx)

	// Check C-Score requirement
	if params.EnableCscoreGating {
		requiredScore, hasRequirement := k.GetRequiredCScore(ctx, ctype)
		if hasRequirement {
			credits := k.GetCredits(ctx, contributor)
			if credits.Amount.LT(requiredScore) {
				return false, fmt.Sprintf(
					"insufficient C-Score: need %s, have %s (need %s more)",
					requiredScore.String(),
					credits.Amount.String(),
					requiredScore.Sub(credits.Amount).String(),
				)
			}
		}
	}

	// Check identity requirement
	if params.EnableIdentityGating {
		requiresIdentity, exists := params.RequireIdentityForCtype[ctype]
		if exists && requiresIdentity {
			// For now, always fail if identity required (module not implemented)
			return false, fmt.Sprintf(
				"contribution type '%s' requires verified identity (not yet available)",
				ctype,
			)
		}
	}

	return true, "all requirements met"
}
