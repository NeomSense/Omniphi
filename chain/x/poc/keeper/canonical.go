package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"

	"pos/x/poc/types"
)

// ========== Canonical Hash Registry ==========

// GetCanonicalRegistry retrieves all claims for a canonical hash.
// Returns the registry and true if found, or an empty registry and false if not.
func (k Keeper) GetCanonicalRegistry(ctx context.Context, canonicalHash []byte) (types.CanonicalRegistry, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetCanonicalRegistryKey(canonicalHash)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.CanonicalRegistry{}, false
	}

	var registry types.CanonicalRegistry
	if err := json.Unmarshal(bz, &registry); err != nil {
		return types.CanonicalRegistry{}, false
	}
	return registry, true
}

// SetCanonicalRegistry stores the registry entry for a canonical hash.
func (k Keeper) SetCanonicalRegistry(ctx context.Context, registry types.CanonicalRegistry) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetCanonicalRegistryKey(registry.CanonicalHash)

	bz, err := json.Marshal(registry)
	if err != nil {
		return fmt.Errorf("failed to marshal canonical registry: %w", err)
	}

	return store.Set(key, bz)
}

// CheckCanonicalHash checks if a canonical hash already exists in the registry.
// Returns (exists, firstClaim, error). If exists is true, firstClaim points to the
// earliest registered claim for this hash.
func (k Keeper) CheckCanonicalHash(ctx context.Context, canonicalHash []byte) (bool, *types.ClaimRecord, error) {
	registry, found := k.GetCanonicalRegistry(ctx, canonicalHash)
	if !found || len(registry.Claims) == 0 {
		return false, nil, nil
	}

	return true, &registry.Claims[0], nil
}

// RegisterCanonicalClaim adds a new claim to the canonical registry for the given hash.
// Returns (isDuplicate, existingClaims, error). If the hash already has claims,
// isDuplicate is true and existingClaims contains all prior claims.
func (k Keeper) RegisterCanonicalClaim(ctx context.Context, canonicalHash []byte, claim types.ClaimRecord) (bool, []types.ClaimRecord, error) {
	registry, found := k.GetCanonicalRegistry(ctx, canonicalHash)

	isDuplicate := false
	var existingClaims []types.ClaimRecord

	if found && len(registry.Claims) > 0 {
		isDuplicate = true
		existingClaims = make([]types.ClaimRecord, len(registry.Claims))
		copy(existingClaims, registry.Claims)
	} else {
		registry = types.CanonicalRegistry{
			CanonicalHash: canonicalHash,
			Claims:        []types.ClaimRecord{},
		}
	}

	registry.Claims = append(registry.Claims, claim)

	if err := k.SetCanonicalRegistry(ctx, registry); err != nil {
		return false, nil, fmt.Errorf("failed to register canonical claim: %w", err)
	}

	return isDuplicate, existingClaims, nil
}

// ========== Duplicate Submission Records ==========

// SetDuplicateRecord stores a record marking a contribution as a duplicate.
func (k Keeper) SetDuplicateRecord(ctx context.Context, record types.DuplicateRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetDuplicateSubmissionKey(record.ContributionID)

	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal duplicate record: %w", err)
	}

	return store.Set(key, bz)
}

// GetDuplicateRecord retrieves a duplicate record for a contribution ID.
func (k Keeper) GetDuplicateRecord(ctx context.Context, contributionID uint64) (types.DuplicateRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetDuplicateSubmissionKey(contributionID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.DuplicateRecord{}, false
	}

	var record types.DuplicateRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.DuplicateRecord{}, false
	}
	return record, true
}

// ========== Epoch Submission Rate Limiting ==========

// GetDuplicateCount returns the number of duplicate submissions by an address in a given epoch.
func (k Keeper) GetDuplicateCount(ctx context.Context, addr string, epoch uint64) (uint32, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEpochSubmissionRateKey(addr, epoch)

	bz, err := store.Get(key)
	if err != nil {
		return 0, err
	}
	if bz == nil {
		return 0, nil
	}

	var count uint32
	if err := json.Unmarshal(bz, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// IncrementDuplicateCount increments the duplicate counter for an address in the current epoch.
func (k Keeper) IncrementDuplicateCount(ctx context.Context, addr string, epoch uint64) error {
	count, err := k.GetDuplicateCount(ctx, addr, epoch)
	if err != nil {
		return err
	}

	count++

	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEpochSubmissionRateKey(addr, epoch)

	bz, err := json.Marshal(count)
	if err != nil {
		return err
	}

	return store.Set(key, bz)
}

// ========== Canonical Registry Iteration ==========

// IterateCanonicalRegistries iterates over all canonical registry entries.
func (k Keeper) IterateCanonicalRegistries(ctx context.Context, cb func(registry types.CanonicalRegistry) (stop bool)) error {
	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(types.KeyPrefixCanonicalRegistry, storetypes.PrefixEndBytes(types.KeyPrefixCanonicalRegistry))
	if err != nil {
		return err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var registry types.CanonicalRegistry
		if err := json.Unmarshal(iterator.Value(), &registry); err != nil {
			continue
		}
		if cb(registry) {
			break
		}
	}

	return nil
}
