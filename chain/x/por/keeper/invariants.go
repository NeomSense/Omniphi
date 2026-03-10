package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// RegisterInvariants registers all module invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "batch-integrity", BatchIntegrityInvariant(k))
	ir.RegisterRoute(types.ModuleName, "attestation-consistency", AttestationConsistencyInvariant(k))
	ir.RegisterRoute(types.ModuleName, "reputation-non-negative", ReputationNonNegativeInvariant(k))
}

// AllInvariants runs all invariants of the module
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		msg, broken := BatchIntegrityInvariant(k)(ctx)
		if broken {
			return msg, broken
		}
		msg, broken = AttestationConsistencyInvariant(k)(ctx)
		if broken {
			return msg, broken
		}
		return ReputationNonNegativeInvariant(k)(ctx)
	}
}

// BatchIntegrityInvariant checks that all batches have valid status and reference valid apps
func BatchIntegrityInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		var broken bool

		batches := k.GetAllBatches(ctx)
		for _, batch := range batches {
			if !batch.Status.IsValid() {
				msg += fmt.Sprintf("batch %d has invalid status: %d\n", batch.BatchId, batch.Status)
				broken = true
			}

			if len(batch.RecordMerkleRoot) != types.MerkleRootLength {
				msg += fmt.Sprintf("batch %d has invalid merkle root length: %d\n", batch.BatchId, len(batch.RecordMerkleRoot))
				broken = true
			}

			if batch.RecordCount == 0 {
				msg += fmt.Sprintf("batch %d has zero record count\n", batch.BatchId)
				broken = true
			}

			// Finalized batches must have a finalization timestamp
			if batch.Status == types.BatchStatusFinalized && batch.FinalizedAt == 0 {
				msg += fmt.Sprintf("batch %d is finalized but has no finalization timestamp\n", batch.BatchId)
				broken = true
			}
		}

		if broken {
			return sdk.FormatInvariant(types.ModuleName, "batch-integrity", msg), true
		}
		return sdk.FormatInvariant(types.ModuleName, "batch-integrity", "all batches have valid state"), false
	}
}

// AttestationConsistencyInvariant checks that all attestations reference valid batches
func AttestationConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		var broken bool

		attestations := k.GetAllAttestations(ctx)
		for _, att := range attestations {
			_, found := k.GetBatch(ctx, att.BatchId)
			if !found {
				msg += fmt.Sprintf("attestation references non-existent batch %d (verifier: %s)\n", att.BatchId, att.VerifierAddress)
				broken = true
			}
		}

		if broken {
			return sdk.FormatInvariant(types.ModuleName, "attestation-consistency", msg), true
		}
		return sdk.FormatInvariant(types.ModuleName, "attestation-consistency", "all attestations reference valid batches"), false
	}
}

// ReputationNonNegativeInvariant checks that all reputation scores are non-negative
func ReputationNonNegativeInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		var broken bool

		reps := k.GetAllReputations(ctx)
		for _, rep := range reps {
			if rep.ReputationScore.IsNegative() {
				msg += fmt.Sprintf("verifier %s has negative reputation score: %s\n", rep.Address, rep.ReputationScore)
				broken = true
			}
			if rep.CorrectAttestations > rep.TotalAttestations {
				msg += fmt.Sprintf("verifier %s has more correct attestations (%d) than total (%d)\n",
					rep.Address, rep.CorrectAttestations, rep.TotalAttestations)
				broken = true
			}
		}

		if broken {
			return sdk.FormatInvariant(types.ModuleName, "reputation-non-negative", msg), true
		}
		return sdk.FormatInvariant(types.ModuleName, "reputation-non-negative", "all reputations are non-negative"), false
	}
}
