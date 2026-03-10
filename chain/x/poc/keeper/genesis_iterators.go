package keeper

import (
	"context"
	"encoding/json"

	storetypes "cosmossdk.io/store/types"

	"pos/x/poc/types"
)

// ============================================================================
// GetAll* iterators for genesis export.
// These scan each prefix and return every record of that type.
// All errors are silently skipped (best-effort; genesis export must not panic).
// ============================================================================

// GetAllVestingSchedules returns every legacy VestingSchedule in the store.
func (k Keeper) GetAllVestingSchedules(ctx context.Context) []types.VestingSchedule {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixVestingSchedule,
		storetypes.PrefixEndBytes(types.KeyPrefixVestingSchedule),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.VestingSchedule
	for ; iter.Valid(); iter.Next() {
		var s types.VestingSchedule
		if err := json.Unmarshal(iter.Value(), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

// GetAllARVSVestingSchedules returns every ARVSVestingSchedule in the store.
func (k Keeper) GetAllARVSVestingSchedules(ctx context.Context) []types.ARVSVestingSchedule {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixARVSVesting,
		storetypes.PrefixEndBytes(types.KeyPrefixARVSVesting),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ARVSVestingSchedule
	for ; iter.Valid(); iter.Next() {
		var s types.ARVSVestingSchedule
		if err := json.Unmarshal(iter.Value(), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

// GetAllReviewSessions returns every ReviewSession in the store.
func (k Keeper) GetAllReviewSessions(ctx context.Context) []types.ReviewSession {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixReviewSession,
		storetypes.PrefixEndBytes(types.KeyPrefixReviewSession),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ReviewSession
	for ; iter.Valid(); iter.Next() {
		var s types.ReviewSession
		if err := json.Unmarshal(iter.Value(), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out
}

// GetAllReviewerProfiles returns every ReviewerProfile in the store.
func (k Keeper) GetAllReviewerProfiles(ctx context.Context) []types.ReviewerProfile {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixReviewerProfile,
		storetypes.PrefixEndBytes(types.KeyPrefixReviewerProfile),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ReviewerProfile
	for ; iter.Valid(); iter.Next() {
		var p types.ReviewerProfile
		if err := json.Unmarshal(iter.Value(), &p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

// GetAllProvenanceEntries returns every ProvenanceEntry in the store.
func (k Keeper) GetAllProvenanceEntries(ctx context.Context) []types.ProvenanceEntry {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixProvenanceEntry,
		storetypes.PrefixEndBytes(types.KeyPrefixProvenanceEntry),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ProvenanceEntry
	for ; iter.Valid(); iter.Next() {
		var e types.ProvenanceEntry
		if err := json.Unmarshal(iter.Value(), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// GetAllContributorStats returns every ContributorStats in the store.
func (k Keeper) GetAllContributorStats(ctx context.Context) []types.ContributorStats {
	store := k.storeService.OpenKVStore(ctx)
	// ContributorStats is stored at ContributorStatsKeyPrefix (0x40) — see types/reward.go
	iter, err := store.Iterator(
		types.ContributorStatsKeyPrefix,
		storetypes.PrefixEndBytes(types.ContributorStatsKeyPrefix),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ContributorStats
	for ; iter.Valid(); iter.Next() {
		var s types.ContributorStats
		if err := json.Unmarshal(iter.Value(), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out
}
