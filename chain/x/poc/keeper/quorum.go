package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// HasQuorum checks if a contribution has reached the required quorum
func (k Keeper) HasQuorum(ctx context.Context, c types.Contribution) (bool, error) {
	// Get total bonded tokens
	total, err := k.stakingKeeper.TotalBondedTokens(ctx)
	if err != nil {
		return false, err
	}

	if total.IsZero() {
		return false, nil
	}

	// Calculate approval power
	approvalPower := c.GetApprovalPower()

	// Calculate required threshold
	params := k.GetParams(ctx)
	requiredPower := math.LegacyNewDecFromInt(total).Mul(params.QuorumPct).TruncateInt()

	// Check if approval power meets or exceeds threshold
	return approvalPower.GTE(requiredPower), nil
}

// AddEndorsement adds an endorsement to a contribution and checks for quorum
// SECURITY FIX: CVE-2025-POC-002 - Prevents endorsement double-counting via canonical address comparison
func (k Keeper) AddEndorsement(ctx context.Context, contributionID uint64, endorsement types.Endorsement) (verified bool, err error) {
	contribution, found := k.GetContribution(ctx, contributionID)
	if !found {
		return false, types.ErrContributionNotFound
	}

	// SECURITY FIX: Convert endorsement address to canonical validator address
	valAddr, err := sdk.ValAddressFromBech32(endorsement.ValAddr)
	if err != nil {
		// Try as account address, convert to validator address
		accAddr, err2 := sdk.AccAddressFromBech32(endorsement.ValAddr)
		if err2 != nil {
			return false, fmt.Errorf("invalid validator address format: %w", err)
		}
		valAddr = sdk.ValAddress(accAddr)
	}

	// SECURITY FIX: Check against ALL existing endorsements using canonical comparison
	for _, existingEndorsement := range contribution.Endorsements {
		existingValAddr, err := sdk.ValAddressFromBech32(existingEndorsement.ValAddr)
		if err != nil {
			// Try converting from account address
			existingAccAddr, _ := sdk.AccAddressFromBech32(existingEndorsement.ValAddr)
			existingValAddr = sdk.ValAddress(existingAccAddr)
		}

		// Compare canonical validator addresses
		if valAddr.Equals(existingValAddr) {
			return false, types.ErrAlreadyEndorsed
		}
	}

	// CRITICAL FIX: Get validator and use bonded tokens (not consensus power) for quorum
	validator, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return false, types.ErrNotValidator
	}

	tokens := validator.GetTokens()
	if tokens.IsZero() {
		return false, types.ErrZeroPower
	}

	// Create endorsement with canonical validator address and bonded tokens
	canonicalEndorsement := types.NewEndorsement(
		valAddr.String(), // Use canonical validator address
		endorsement.Decision,
		tokens,
		sdk.UnwrapSDKContext(ctx).BlockTime().Unix(),
	)

	// Add endorsement
	contribution.AddEndorsement(canonicalEndorsement)

	// Check if quorum is reached (only for approvals)
	if canonicalEndorsement.Decision && !contribution.Verified {
		hasQuorum, err := k.HasQuorum(ctx, contribution)
		if err != nil {
			return false, err
		}

		if hasQuorum {
			contribution.Verified = true
			// Enqueue reward for the contributor
			if err := k.EnqueueReward(ctx, contribution); err != nil {
				return false, err
			}
			verified = true
		}
	}

	// Save updated contribution
	if err := k.SetContribution(ctx, contribution); err != nil {
		return false, err
	}

	return verified, nil
}
