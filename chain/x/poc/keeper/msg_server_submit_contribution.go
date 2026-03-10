package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// SubmitContribution handles the submission of a new contribution
func (ms msgServer) SubmitContribution(goCtx context.Context, msg *types.MsgSubmitContribution) (*types.MsgSubmitContributionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Convert contributor address
	contributor, err := sdk.AccAddressFromBech32(msg.Contributor)
	if err != nil {
		return nil, fmt.Errorf("invalid contributor address: %w", err)
	}

	// THREE-LAYER VERIFICATION PIPELINE
	// Layer 1: PoE (Proof of Existence) - Already validated in msg.ValidateBasic()
	// Layer 2: PoA (Proof of Authority) - Check C-Score and identity requirements
	// Layer 3: PoV (Proof of Value) - Validator endorsements (happens later)

	// LAYER 2: PoA - Check access control requirements
	if err := ms.CheckProofOfAuthority(goCtx, contributor, msg.Ctype); err != nil {
		return nil, err
	}

	// UCI: Universal Contribution Interface Validation
	// Assuming msg.ProofType is an enum: 0=Static, 1=ZK_Snark, 2=Storage_Audit
	// Assuming msg.ProofData is []byte
	/*
		switch msg.ProofType {
		case types.ProofType_ZK_SNARK:
			if len(msg.ProofData) == 0 {
				return nil, types.ErrInvalidProof.Wrap("zk-snark proof data missing")
			}
			// Future: Verify ZK proof against verifier contract/module
		case types.ProofType_STORAGE_AUDIT:
			if len(msg.ProofData) == 0 {
				return nil, types.ErrInvalidProof.Wrap("storage audit data missing")
			}
		case types.ProofType_STATIC_CONTENT:
			if msg.Uri == "" {
				return nil, types.ErrInvalidURI.Wrap("static content requires URI")
			}
		}
	*/

	// Check rate limit
	if err := ms.CheckRateLimit(goCtx); err != nil {
		return nil, err
	}

	// LAYER 1.5: Canonical Hash Deduplication
	params := ms.GetParams(goCtx)
	isDuplicate := false
	var duplicateOf uint64
	var duplicateOriginalSubmitter string

	if params.EnableCanonicalHashCheck && len(msg.CanonicalHash) > 0 {
		// 1. Calculate and collect escalated bond
		bond, bondErr := ms.CalculateEscalatedBond(goCtx, contributor)
		if bondErr != nil {
			return nil, fmt.Errorf("bond calculation failed: %w", bondErr)
		}
		if err := ms.CollectDuplicateBond(goCtx, contributor, bond); err != nil {
			return nil, err
		}

		// 2. Check canonical hash registry
		exists, existingClaim, checkErr := ms.CheckCanonicalHash(goCtx, msg.CanonicalHash)
		if checkErr != nil {
			// Refund bond on internal error
			if bond.IsPositive() {
				if refundErr := ms.RefundDuplicateBondDirect(goCtx, contributor, bond); refundErr != nil {
					return nil, fmt.Errorf("canonical hash check failed (%v) and bond refund also failed: %w", checkErr, refundErr)
				}
			}
			return nil, fmt.Errorf("canonical hash check failed: %w", checkErr)
		}

		if exists {
			isDuplicate = true
			duplicateOf = existingClaim.ClaimID
			duplicateOriginalSubmitter = existingClaim.Submitter

			// Rate limit check for duplicates
			epoch := ms.GetCurrentEpoch(goCtx)
			dupCount, err := ms.GetDuplicateCount(goCtx, msg.Contributor, epoch)
			if err != nil {
				return nil, fmt.Errorf("failed to get duplicate count: %w", err)
			}
			if dupCount >= params.MaxDuplicatesPerEpoch {
				// Refund bond since we're rejecting outright
				if bond.IsPositive() {
					if err := ms.RefundDuplicateBondDirect(goCtx, contributor, bond); err != nil {
						return nil, fmt.Errorf("rate limit exceeded and bond refund failed: %w", err)
					}
				}
				return nil, types.ErrDuplicateRateLimitExceeded.Wrapf(
					"address %s has %d duplicates this epoch (max %d)",
					msg.Contributor, dupCount, params.MaxDuplicatesPerEpoch)
			}
			if err := ms.IncrementDuplicateCount(goCtx, msg.Contributor, epoch); err != nil {
				return nil, fmt.Errorf("failed to increment duplicate count: %w", err)
			}

			// Slash bond for confirmed duplicate
			if bond.IsPositive() {
				if err := ms.SlashDuplicateBondDirect(goCtx, contributor, bond); err != nil {
					return nil, fmt.Errorf("failed to slash duplicate bond: %w", err)
				}
			}
		} else if bond.IsPositive() {
			// H-POC-01 FIX: Not a duplicate. The bond's purpose is fulfilled. Refund it.
			if err := ms.RefundDuplicateBondDirect(goCtx, contributor, bond); err != nil {
				return nil, fmt.Errorf("failed to refund bond for original contribution: %w", err)
			}
		}
	}

	// CALCULATE 3-LAYER FEE
	// This combines:
	// 1. Base Fee Model (static base)
	// 2. Epoch-Adaptive Fee (dynamic congestion multiplier)
	// 3. C-Score Weighted Discount (reputation-based reduction)
	finalFee, epochMultiplier, cscoreDiscount, err := ms.Calculate3LayerFee(goCtx, contributor)
	if err != nil {
		return nil, fmt.Errorf("fee calculation failed: %w", err)
	}

	// INCREMENT BLOCK SUBMISSION COUNTER (for next submission's epoch multiplier)
	ms.IncrementBlockSubmissions(goCtx)

	// COLLECT AND SPLIT FEE BEFORE creating contribution
	// This ensures atomicity - if fee payment fails, contribution is not created
	// Split: 50% burned, 50% to reward pool
	if err := ms.CollectAndSplit3LayerFee(goCtx, contributor, finalFee, epochMultiplier, cscoreDiscount); err != nil {
		return nil, fmt.Errorf("fee collection failed: %w", err)
	}

	// Get next ID
	id, err := ms.GetNextContributionID(goCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next contribution ID: %w", err)
	}

	// Create contribution
	contribution := types.NewContribution(
		id,
		msg.Contributor,
		msg.Ctype,
		msg.Uri,
		msg.Hash,
		ctx.BlockHeight(),
		ctx.BlockTime().Unix(),
		// msg.ProofType,
		// msg.ProofData,
	)

	// Set canonical hash fields on contribution
	if len(msg.CanonicalHash) > 0 {
		contribution.CanonicalHash = msg.CanonicalHash
		contribution.CanonicalSpecVersion = msg.CanonicalSpecVersion
	}
	if isDuplicate {
		contribution.DuplicateOf = duplicateOf
	}

	// Store contribution
	if err := ms.SetContribution(goCtx, contribution); err != nil {
		return nil, err
	}

	// Post-creation: register canonical claim or store duplicate record
	if params.EnableCanonicalHashCheck && len(msg.CanonicalHash) > 0 {
		if isDuplicate {
			// Store duplicate record with the assigned contribution ID
			dupRecord := types.DuplicateRecord{
				ContributionID:    id,
				CanonicalHash:     msg.CanonicalHash,
				OriginalClaimID:   duplicateOf,
				OriginalSubmitter: duplicateOriginalSubmitter,
			}
			if err := ms.SetDuplicateRecord(goCtx, dupRecord); err != nil {
				return nil, fmt.Errorf("failed to set duplicate record: %w", err)
			}
		} else {
			// Register the new canonical claim
			claim := types.ClaimRecord{
				ClaimID:        id,
				Submitter:      msg.Contributor,
				Category:       msg.Ctype,
				StoragePointer: msg.Uri,
				BlockHeight:    ctx.BlockHeight(),
				SpecVersion:    msg.CanonicalSpecVersion,
			}
			if _, _, err := ms.RegisterCanonicalClaim(goCtx, msg.CanonicalHash, claim); err != nil {
				return nil, fmt.Errorf("failed to register canonical claim: %w", err)
			}
		}
	}

	// Emit event
	submitEvent := sdk.NewEvent(
		"poc_submit",
		sdk.NewAttribute("id", fmt.Sprintf("%d", id)),
		sdk.NewAttribute("contributor", msg.Contributor),
		sdk.NewAttribute("ctype", msg.Ctype),
		sdk.NewAttribute("uri", msg.Uri),
	)
	if isDuplicate {
		submitEvent = submitEvent.AppendAttributes(
			sdk.NewAttribute("duplicate_of", fmt.Sprintf("%d", duplicateOf)),
			sdk.NewAttribute("canonical_hash", types.CanonicalHashHex(msg.CanonicalHash)),
		)
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		submitEvent,
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Contributor),
		),
	})

	// Set unified claim status
	if isDuplicate {
		ms.TransitionClaimStatus(goCtx, id, types.ClaimStatusDuplicate)
	} else {
		ms.TransitionClaimStatus(goCtx, id, types.ClaimStatusAwaitingSimilarity)
	}

	return &types.MsgSubmitContributionResponse{
		Id: id,
	}, nil
}
