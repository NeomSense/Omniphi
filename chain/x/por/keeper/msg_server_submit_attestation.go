package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// SubmitAttestation handles MsgSubmitAttestation - a verifier attests to a batch's validity
func (ms msgServer) SubmitAttestation(goCtx context.Context, msg *types.MsgSubmitAttestation) (*types.MsgSubmitAttestationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate the message
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	// Get the batch
	batch, found := ms.GetBatch(goCtx, msg.BatchId)
	if !found {
		return nil, types.ErrBatchNotFound.Wrapf("batch_id: %d", msg.BatchId)
	}

	// Batch must be in SUBMITTED status to accept attestations
	if batch.Status != types.BatchStatusSubmitted {
		return nil, types.ErrBatchNotSubmitted.Wrapf(
			"batch %d is in status %s, expected SUBMITTED", msg.BatchId, batch.Status,
		)
	}

	// Get the verifier set for this batch
	vs, found := ms.GetVerifierSet(goCtx, batch.VerifierSetId)
	if !found {
		return nil, types.ErrVerifierSetNotFound.Wrapf("verifier_set_id: %d", batch.VerifierSetId)
	}

	// SECURITY: Use canonical address comparison to prevent double-attestation
	// Convert both addresses to canonical form before comparing
	verifierAddr, err := sdk.AccAddressFromBech32(msg.Verifier)
	if err != nil {
		return nil, fmt.Errorf("invalid verifier address: %w", err)
	}
	canonicalVerifier := verifierAddr.String()

	// Verify the signer is a member of the verifier set (using canonical address)
	isMember := false
	for _, m := range vs.Members {
		memberAddr, mErr := sdk.AccAddressFromBech32(m.Address)
		if mErr != nil {
			continue
		}
		if memberAddr.String() == canonicalVerifier {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, types.ErrNotVerifier.Wrapf("address: %s, verifier_set_id: %d", canonicalVerifier, batch.VerifierSetId)
	}

	// SECURITY: Check reputation gate — reject verifiers below minimum reputation
	params := ms.GetParams(goCtx)
	if params.MinReputationForAttestation.IsPositive() {
		rep := ms.GetOrCreateVerifierReputation(goCtx, canonicalVerifier)
		if rep.ReputationScore.LT(params.MinReputationForAttestation) {
			return nil, types.ErrInsufficientReputation.Wrapf(
				"verifier %s has reputation %s, minimum required: %s",
				canonicalVerifier, rep.ReputationScore, params.MinReputationForAttestation,
			)
		}
	}

	// SECURITY: Check for duplicate attestation using canonical address
	_, alreadyAttested := ms.GetAttestation(goCtx, msg.BatchId, canonicalVerifier)
	if alreadyAttested {
		return nil, types.ErrAlreadyAttested.Wrapf("verifier: %s, batch_id: %d", canonicalVerifier, msg.BatchId)
	}

	// SECURITY (F5): Verify attestation signature is a batch-binding commitment.
	// The signature must be SHA256(batchID || merkleRoot || epoch || verifierAddress) —
	// this proves the verifier intentionally attested to THIS specific batch and prevents
	// third-party forgery where an observer computes the hash and submits on behalf of others.
	expectedSig := types.ComputeAttestationSignBytes(batch.BatchId, batch.RecordMerkleRoot, batch.Epoch, canonicalVerifier)
	if !bytesEqual(msg.Signature, expectedSig) {
		return nil, types.ErrInvalidSignature.Wrapf(
			"attestation signature does not match expected batch-binding commitment for batch %d",
			msg.BatchId,
		)
	}

	// Create and store the attestation
	now := ctx.BlockTime().Unix()
	attestation := types.NewAttestation(
		msg.BatchId,
		canonicalVerifier,
		msg.Signature,
		msg.ConfidenceWeight,
		now,
	)

	if err := ms.SetAttestation(goCtx, attestation); err != nil {
		return nil, fmt.Errorf("failed to store attestation: %w", err)
	}

	// Get all attestations for this batch to check quorum
	attestations := ms.GetAttestationsForBatch(goCtx, msg.BatchId)
	attestationCount := uint32(len(attestations))

	// Check if quorum is met:
	// 1. Minimum attestation count met
	// 2. Weighted quorum percentage met
	metQuorum := false
	if attestationCount >= vs.MinAttestations {
		// Calculate weighted attestation sum
		totalWeight := vs.GetTotalWeight()
		if totalWeight.IsPositive() {
			weightedSum := math.LegacyZeroDec()
			for _, att := range attestations {
				// Get the member weight for this attester (canonical comparison)
				attAddr, aErr := sdk.AccAddressFromBech32(att.VerifierAddress)
				if aErr != nil {
					continue
				}
				for _, m := range vs.Members {
					mAddr, mErr := sdk.AccAddressFromBech32(m.Address)
					if mErr != nil {
						continue
					}
					if mAddr.String() == attAddr.String() {
						// confidence_weight * member.Weight / total_weight
						memberWeightDec := math.LegacyNewDecFromInt(m.Weight)
						weightedSum = weightedSum.Add(att.ConfidenceWeight.Mul(memberWeightDec))
						break
					}
				}
			}

			totalWeightDec := math.LegacyNewDecFromInt(totalWeight)
			quorumRatio := weightedSum.Quo(totalWeightDec)

			if quorumRatio.GTE(vs.QuorumPct) {
				metQuorum = true
			}
		}
	}

	// If quorum is met, transition batch to PENDING
	if metQuorum {
		if err := ms.UpdateBatchStatus(goCtx, &batch, types.BatchStatusPending); err != nil {
			return nil, fmt.Errorf("failed to update batch status to PENDING: %w", err)
		}

		ms.Logger().Info("batch quorum met, transitioning to PENDING",
			"batch_id", msg.BatchId,
			"attestation_count", attestationCount,
			"min_attestations", vs.MinAttestations,
		)

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"por_batch_quorum_met",
			sdk.NewAttribute("batch_id", fmt.Sprintf("%d", msg.BatchId)),
			sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", attestationCount)),
			sdk.NewAttribute("status", types.BatchStatusPending.String()),
		))
	}

	// Update verifier reputation (increment total attestations)
	rep := ms.GetOrCreateVerifierReputation(goCtx, canonicalVerifier)
	rep.TotalAttestations++
	if err := ms.SetVerifierReputation(goCtx, rep); err != nil {
		// Non-critical: log but don't fail the attestation
		ms.Logger().Error("failed to update verifier reputation", "verifier", canonicalVerifier, "error", err)
	}

	ms.Logger().Info("attestation submitted",
		"batch_id", msg.BatchId,
		"verifier", canonicalVerifier,
		"attestation_count", attestationCount,
		"met_quorum", metQuorum,
	)

	// Emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"por_submit_attestation",
			sdk.NewAttribute("batch_id", fmt.Sprintf("%d", msg.BatchId)),
			sdk.NewAttribute("verifier", canonicalVerifier),
			sdk.NewAttribute("confidence_weight", msg.ConfidenceWeight.String()),
			sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", attestationCount)),
			sdk.NewAttribute("met_quorum", fmt.Sprintf("%t", metQuorum)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, canonicalVerifier),
		),
	})

	return &types.MsgSubmitAttestationResponse{
		AttestationCount: attestationCount,
		MetQuorum:        metQuorum,
	}, nil
}
