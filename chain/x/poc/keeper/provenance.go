package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ============================================================================
// Provenance Entry CRUD
// ============================================================================

// SetProvenanceEntry stores a provenance entry and writes all indexes.
func (k Keeper) SetProvenanceEntry(ctx context.Context, entry types.ProvenanceEntry) error {
	store := k.storeService.OpenKVStore(ctx)

	bz, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal provenance entry: %w", err)
	}

	// Primary record
	if err := store.Set(types.GetProvenanceEntryKey(entry.ClaimID), bz); err != nil {
		return err
	}

	// Sentinel value for all indexes (existence-only)
	sentinel := []byte{}

	// Child index: parent → child
	if entry.ParentClaimID > 0 {
		key := types.GetProvenanceChildIndexKey(entry.ParentClaimID, entry.ClaimID)
		if err := store.Set(key, sentinel); err != nil {
			return err
		}
	}

	// Hash index: canonical_hash → claim
	// Enforce fixed 32-byte hash to prevent ambiguous index keys
	if len(entry.CanonicalHash) > 0 {
		if len(entry.CanonicalHash) != types.CanonicalHashSize {
			return fmt.Errorf("canonical hash must be %d bytes, got %d", types.CanonicalHashSize, len(entry.CanonicalHash))
		}
		key := types.GetProvenanceHashIndexKey(entry.CanonicalHash, entry.ClaimID)
		if err := store.Set(key, sentinel); err != nil {
			return err
		}
	}

	// Submitter index: submitter → claim
	if entry.Submitter != "" {
		key := types.GetProvenanceSubmitterIndexKey(entry.Submitter, entry.ClaimID)
		if err := store.Set(key, sentinel); err != nil {
			return err
		}
	}

	// Category index: category → claim
	if entry.Category != "" {
		key := types.GetProvenanceCategoryIndexKey(entry.Category, entry.ClaimID)
		if err := store.Set(key, sentinel); err != nil {
			return err
		}
	}

	// Epoch index: epoch → claim
	key := types.GetProvenanceEpochIndexKey(entry.Epoch, entry.ClaimID)
	if err := store.Set(key, sentinel); err != nil {
		return err
	}

	// Update stats
	return k.updateProvenanceStats(ctx, entry)
}

// GetProvenanceEntry retrieves a provenance entry by claim ID.
func (k Keeper) GetProvenanceEntry(ctx context.Context, claimID uint64) (types.ProvenanceEntry, bool) {
	store := k.storeService.OpenKVStore(ctx)

	bz, err := store.Get(types.GetProvenanceEntryKey(claimID))
	if err != nil || bz == nil {
		return types.ProvenanceEntry{}, false
	}

	var entry types.ProvenanceEntry
	if err := json.Unmarshal(bz, &entry); err != nil {
		return types.ProvenanceEntry{}, false
	}
	return entry, true
}

// HasProvenanceEntry checks if a provenance entry exists for a claim ID.
func (k Keeper) HasProvenanceEntry(ctx context.Context, claimID uint64) bool {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.GetProvenanceEntryKey(claimID))
	return err == nil && bz != nil
}

// ============================================================================
// DAG Validation
// ============================================================================

// validateAndComputeLineage walks ancestors to compute depth and detect cycles.
// Returns the depth for the new entry (parent.Depth + 1) or 0 for root claims.
func (k Keeper) validateAndComputeLineage(ctx context.Context, parentClaimID uint64, maxDepth uint32) (uint32, error) {
	if parentClaimID == 0 {
		return 0, nil // root claim
	}

	// Look up immediate parent to get our depth
	parent, found := k.GetProvenanceEntry(ctx, parentClaimID)
	if !found {
		return 0, types.ErrProvenanceParentNotFound
	}

	depth := parent.Depth + 1
	if depth > maxDepth {
		return 0, types.ErrProvenanceMaxDepthExceeded
	}

	// Walk ancestors to verify no cycles (O(depth) check)
	visited := make(map[uint64]bool)
	visited[parentClaimID] = true
	currentID := parent.ParentClaimID

	for currentID > 0 {
		if visited[currentID] {
			return 0, types.ErrProvenanceCycleDetected
		}
		visited[currentID] = true

		ancestor, found := k.GetProvenanceEntry(ctx, currentID)
		if !found {
			return 0, types.ErrProvenanceParentNotFound
		}

		currentID = ancestor.ParentClaimID
	}

	return depth, nil
}

// resolveDerivationReason determines the derivation reason from contribution and review state.
func resolveDerivationReason(contribution types.Contribution, session *types.ReviewSession) types.DerivationReason {
	if !contribution.IsDerivative && contribution.ParentClaimId == 0 {
		return types.DerivationNone
	}

	// Check if human review overrode the AI decision
	if session != nil {
		switch session.OverrideApplied {
		case types.OverrideDerivativeTruePositive:
			return types.DerivationHuman
		case types.OverrideNotDerivativeFalseNegative:
			return types.DerivationHuman
		}
	}

	// If the contribution has a parent, check if similarity engine flagged it
	if contribution.IsDerivative {
		return types.DerivationAI
	}

	// Has a parent but not flagged as derivative — submitter self-declared
	if contribution.ParentClaimId > 0 {
		return types.DerivationExplicit
	}

	return types.DerivationNone
}

// ============================================================================
// Registration
// ============================================================================

// RegisterProvenance registers an accepted contribution in the provenance registry.
// This is called from finalizeReviewSession when a contribution is accepted.
// Returns nil if the registry is disabled (no-op).
func (k Keeper) RegisterProvenance(ctx context.Context, contribution types.Contribution, session *types.ReviewSession) error {
	params := k.GetParams(ctx)

	// No-op if registry is disabled
	if !params.EnableProvenanceRegistry {
		return nil
	}

	// Check not already registered
	if k.HasProvenanceEntry(ctx, contribution.Id) {
		return types.ErrProvenanceAlreadyRegistered
	}

	// Validate lineage and compute depth
	maxDepth := params.MaxProvenanceDepth
	if maxDepth == 0 {
		maxDepth = types.DefaultMaxProvenanceDepth
	}
	depth, err := k.validateAndComputeLineage(ctx, contribution.ParentClaimId, maxDepth)
	if err != nil {
		return fmt.Errorf("lineage validation failed: %w", err)
	}

	// Resolve derivation reason
	derivReason := resolveDerivationReason(contribution, session)

	// Get similarity score and compute originality multiplier
	simScore := math.LegacyZeroDec()
	if simRecord, found := k.GetSimilarityCommitment(ctx, contribution.Id); found {
		simScore = math.LegacyNewDecWithPrec(int64(simRecord.CompactData.OverallSimilarity), 4) // e.g., 8500 → 0.8500
	}
	origMult := k.resolveOriginalityMultiplier(params, simScore)

	// Get quality score from review session
	var qualityScore uint32
	if session != nil {
		qualityScore = session.FinalQuality
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	epoch := k.GetCurrentEpoch(ctx)
	schemaVersion := params.ProvenanceSchemaVersion
	if schemaVersion == 0 {
		schemaVersion = types.DefaultProvenanceSchemaVersion
	}

	entry := types.ProvenanceEntry{
		ClaimID:               contribution.Id,
		CanonicalHash:         contribution.CanonicalHash,
		Category:              contribution.Ctype,
		Submitter:             contribution.Contributor,
		ParentClaimID:         contribution.ParentClaimId,
		IsDerivative:          contribution.IsDerivative,
		DerivationReason:      derivReason,
		OriginalityMultiplier: origMult,
		QualityScore:          qualityScore,
		Depth:                 depth,
		AcceptedAtHeight:      sdkCtx.BlockHeight(),
		AcceptedAtTime:        sdkCtx.BlockTime().Unix(),
		Epoch:                 epoch,
		SchemaVersion:         schemaVersion,
	}

	if err := k.SetProvenanceEntry(ctx, entry); err != nil {
		return fmt.Errorf("failed to store provenance entry: %w", err)
	}

	// Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_provenance_registered",
		sdk.NewAttribute("claim_id", strconv.FormatUint(entry.ClaimID, 10)),
		sdk.NewAttribute("submitter", entry.Submitter),
		sdk.NewAttribute("category", entry.Category),
		sdk.NewAttribute("parent_claim_id", strconv.FormatUint(entry.ParentClaimID, 10)),
		sdk.NewAttribute("depth", strconv.FormatUint(uint64(entry.Depth), 10)),
		sdk.NewAttribute("is_derivative", strconv.FormatBool(entry.IsDerivative)),
		sdk.NewAttribute("derivation_reason", strconv.FormatUint(uint64(entry.DerivationReason), 10)),
		sdk.NewAttribute("originality_multiplier", entry.OriginalityMultiplier.String()),
		sdk.NewAttribute("quality_score", strconv.FormatUint(uint64(entry.QualityScore), 10)),
	))

	return nil
}

// ============================================================================
// Index Iterators
// ============================================================================

// GetProvenanceChildren returns all child claim IDs for a given parent.
func (k Keeper) GetProvenanceChildren(ctx context.Context, parentClaimID uint64) []uint64 {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.GetProvenanceChildIndexPrefix(parentClaimID)

	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var children []uint64
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) >= prefixLen+8 {
			childID := sdk.BigEndianToUint64(key[prefixLen:])
			children = append(children, childID)
		}
	}
	return children
}

// GetProvenanceByHash returns all claim IDs that share a given canonical hash.
func (k Keeper) GetProvenanceByHash(ctx context.Context, canonicalHash []byte) []uint64 {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.GetProvenanceHashIndexPrefix(canonicalHash)

	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var claimIDs []uint64
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) >= prefixLen+8 {
			claimID := sdk.BigEndianToUint64(key[prefixLen:])
			claimIDs = append(claimIDs, claimID)
		}
	}
	return claimIDs
}

// GetProvenanceBySubmitter returns all claim IDs for a given submitter address.
func (k Keeper) GetProvenanceBySubmitter(ctx context.Context, addr string) []uint64 {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.GetProvenanceSubmitterIndexPrefix(addr)

	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var claimIDs []uint64
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) >= prefixLen+8 {
			claimID := sdk.BigEndianToUint64(key[prefixLen:])
			claimIDs = append(claimIDs, claimID)
		}
	}
	return claimIDs
}

// GetProvenanceByCategory returns all claim IDs for a given category.
func (k Keeper) GetProvenanceByCategory(ctx context.Context, category string) []uint64 {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.GetProvenanceCategoryIndexPrefix(category)

	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var claimIDs []uint64
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) >= prefixLen+8 {
			claimID := sdk.BigEndianToUint64(key[prefixLen:])
			claimIDs = append(claimIDs, claimID)
		}
	}
	return claimIDs
}

// GetProvenanceByEpochRange returns all claim IDs in the given epoch range [startEpoch, endEpoch].
func (k Keeper) GetProvenanceByEpochRange(ctx context.Context, startEpoch, endEpoch uint64) []uint64 {
	store := k.storeService.OpenKVStore(ctx)
	startKey := types.GetProvenanceEpochIndexPrefix(startEpoch)
	endKey := types.GetProvenanceEpochIndexPrefix(endEpoch + 1)

	iter, err := store.Iterator(startKey, endKey)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var claimIDs []uint64
	// Each key is: prefix(1) + epoch(8) + claimID(8) = 17 bytes
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) >= 17 {
			claimID := sdk.BigEndianToUint64(key[9:]) // skip prefix(1) + epoch(8)
			claimIDs = append(claimIDs, claimID)
		}
	}
	return claimIDs
}

// ============================================================================
// Lineage Path
// ============================================================================

// ComputeLineagePath walks from a claim to its root ancestor, returning the path ordered root→leaf.
func (k Keeper) ComputeLineagePath(ctx context.Context, claimID uint64) ([]types.ProvenanceEntry, error) {
	params := k.GetParams(ctx)
	maxDepth := params.MaxProvenanceDepth
	if maxDepth == 0 {
		maxDepth = types.DefaultMaxProvenanceDepth
	}

	var path []types.ProvenanceEntry
	currentID := claimID
	visited := make(map[uint64]bool)

	for currentID > 0 && uint32(len(path)) <= maxDepth {
		if visited[currentID] {
			return nil, types.ErrProvenanceCycleDetected
		}
		visited[currentID] = true

		entry, found := k.GetProvenanceEntry(ctx, currentID)
		if !found {
			return nil, types.ErrProvenanceNotFound
		}

		path = append(path, entry)
		currentID = entry.ParentClaimID
	}

	// Reverse to root→leaf order
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path, nil
}

// ============================================================================
// Stats
// ============================================================================

// GetProvenanceStats retrieves aggregate provenance statistics.
func (k Keeper) GetProvenanceStats(ctx context.Context) types.ProvenanceStats {
	store := k.storeService.OpenKVStore(ctx)

	bz, err := store.Get(types.KeyPrefixProvenanceStats)
	if err != nil || bz == nil {
		return types.NewProvenanceStats()
	}

	var stats types.ProvenanceStats
	if err := json.Unmarshal(bz, &stats); err != nil {
		return types.NewProvenanceStats()
	}

	// Ensure map is initialized
	if stats.CategoryCounts == nil {
		stats.CategoryCounts = make(map[string]uint64)
	}

	return stats
}

// updateProvenanceStats increments aggregate statistics for a newly registered entry.
func (k Keeper) updateProvenanceStats(ctx context.Context, entry types.ProvenanceEntry) error {
	stats := k.GetProvenanceStats(ctx)

	stats.TotalEntries++
	if entry.ParentClaimID == 0 {
		stats.RootEntries++
	}
	if entry.IsDerivative {
		stats.DerivativeCount++
	}
	if entry.Depth > stats.MaxDepthSeen {
		stats.MaxDepthSeen = entry.Depth
	}
	stats.CategoryCounts[entry.Category]++

	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("failed to marshal provenance stats: %w", err)
	}
	return store.Set(types.KeyPrefixProvenanceStats, bz)
}
