package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// InitGenesis initializes the module state from genesis data
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	// Set params first
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return fmt.Errorf("failed to set params: %w", err)
	}

	kvStore := k.storeService.OpenKVStore(ctx)

	// Set ID counters
	if err := kvStore.Set(types.KeyNextAppID, sdk.Uint64ToBigEndian(gs.NextAppId)); err != nil {
		return fmt.Errorf("failed to set next app ID: %w", err)
	}
	if err := kvStore.Set(types.KeyNextBatchID, sdk.Uint64ToBigEndian(gs.NextBatchId)); err != nil {
		return fmt.Errorf("failed to set next batch ID: %w", err)
	}
	if err := kvStore.Set(types.KeyNextChallengeID, sdk.Uint64ToBigEndian(gs.NextChallengeId)); err != nil {
		return fmt.Errorf("failed to set next challenge ID: %w", err)
	}
	if err := kvStore.Set(types.KeyNextVerifierSetID, sdk.Uint64ToBigEndian(gs.NextVerifierSetId)); err != nil {
		return fmt.Errorf("failed to set next verifier set ID: %w", err)
	}

	// Import all apps
	for _, app := range gs.Apps {
		if err := k.SetApp(ctx, app); err != nil {
			return fmt.Errorf("failed to set app %d: %w", app.AppId, err)
		}
	}

	// Import all verifier sets
	for _, vs := range gs.VerifierSets {
		if err := k.SetVerifierSet(ctx, vs); err != nil {
			return fmt.Errorf("failed to set verifier set %d: %w", vs.VerifierSetId, err)
		}
	}

	// Import all batches
	for _, batch := range gs.Batches {
		if err := k.SetBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to set batch %d: %w", batch.BatchId, err)
		}
	}

	// Import all attestations
	for _, att := range gs.Attestations {
		if err := k.SetAttestation(ctx, att); err != nil {
			return fmt.Errorf("failed to set attestation for batch %d: %w", att.BatchId, err)
		}
	}

	// Import all challenges
	for _, challenge := range gs.Challenges {
		if err := k.SetChallenge(ctx, challenge); err != nil {
			return fmt.Errorf("failed to set challenge %d: %w", challenge.ChallengeId, err)
		}
	}

	// Import all reputations
	for _, rep := range gs.Reputations {
		if err := k.SetVerifierReputation(ctx, rep); err != nil {
			return fmt.Errorf("failed to set reputation for %s: %w", rep.Address, err)
		}
	}

	return nil
}

// ExportGenesis exports the module state to genesis data
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Read ID counters
	nextAppID := readUint64(kvStore, types.KeyNextAppID)
	nextBatchID := readUint64(kvStore, types.KeyNextBatchID)
	nextChallengeID := readUint64(kvStore, types.KeyNextChallengeID)
	nextVerifierSetID := readUint64(kvStore, types.KeyNextVerifierSetID)

	return &types.GenesisState{
		Params:            params,
		Apps:              k.GetAllApps(ctx),
		VerifierSets:      k.GetAllVerifierSets(ctx),
		Batches:           k.GetAllBatches(ctx),
		Attestations:      k.GetAllAttestations(ctx),
		Challenges:        k.GetAllChallenges(ctx),
		Reputations:       k.GetAllReputations(ctx),
		NextAppId:         nextAppID,
		NextBatchId:       nextBatchID,
		NextChallengeId:   nextChallengeID,
		NextVerifierSetId: nextVerifierSetID,
	}
}

// readUint64 is a helper to safely read a uint64 from store
func readUint64(kvStore interface{ Get([]byte) ([]byte, error) }, key []byte) uint64 {
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return 1
	}
	return sdk.BigEndianToUint64(bz)
}
