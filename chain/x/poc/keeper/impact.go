package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ============================================================================
// Layer 5: Utility & Impact Scoring
// ============================================================================

// GetImpactParams returns the Layer 5 governance parameters from the JSON sidecar.
func (k Keeper) GetImpactParams(ctx context.Context) types.ImpactParams {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyImpactParams)
	if err != nil || bz == nil {
		return types.DefaultImpactParams()
	}
	var p types.ImpactParams
	if err := json.Unmarshal(bz, &p); err != nil {
		return types.DefaultImpactParams()
	}
	return p
}

// SetImpactParams persists the Layer 5 governance parameters.
func (k Keeper) SetImpactParams(ctx context.Context, p types.ImpactParams) error {
	bz, err := json.Marshal(p)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyImpactParams, bz)
}

// ============================================================================
// ContributionImpactRecord CRUD
// ============================================================================

// GetImpactRecord returns the ContributionImpactRecord for a claim, or (zero, false).
func (k Keeper) GetImpactRecord(ctx context.Context, claimID uint64) (types.ContributionImpactRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.GetImpactRecordKey(claimID))
	if err != nil || bz == nil {
		return types.ContributionImpactRecord{}, false
	}
	var r types.ContributionImpactRecord
	if err := json.Unmarshal(bz, &r); err != nil {
		return types.ContributionImpactRecord{}, false
	}
	return r, true
}

// SetImpactRecord writes a ContributionImpactRecord for a claim.
func (k Keeper) SetImpactRecord(ctx context.Context, r types.ContributionImpactRecord) error {
	bz, err := json.Marshal(r)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.GetImpactRecordKey(r.ClaimID), bz)
}

// GetAllImpactRecords returns all ContributionImpactRecord entries (for genesis export).
func (k Keeper) GetAllImpactRecords(ctx context.Context) []types.ContributionImpactRecord {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixImpactRecord,
		storetypes.PrefixEndBytes(types.KeyPrefixImpactRecord),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ContributionImpactRecord
	for ; iter.Valid(); iter.Next() {
		var r types.ContributionImpactRecord
		if err := json.Unmarshal(iter.Value(), &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// ============================================================================
// ContributorImpactProfile CRUD
// ============================================================================

// GetImpactProfile returns the ContributorImpactProfile for an address, or a zero-value profile.
func (k Keeper) GetImpactProfile(ctx context.Context, addr string) types.ContributorImpactProfile {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.GetImpactProfileKey(addr))
	if err != nil || bz == nil {
		return types.ContributorImpactProfile{
			Address:               addr,
			TrustAdjustmentFactor: 10000, // neutral: 1.0x
		}
	}
	var p types.ContributorImpactProfile
	if err := json.Unmarshal(bz, &p); err != nil {
		return types.ContributorImpactProfile{
			Address:               addr,
			TrustAdjustmentFactor: 10000,
		}
	}
	return p
}

// SetImpactProfile writes a ContributorImpactProfile.
func (k Keeper) SetImpactProfile(ctx context.Context, p types.ContributorImpactProfile) error {
	bz, err := json.Marshal(p)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.GetImpactProfileKey(p.Address), bz)
}

// GetAllImpactProfiles returns all ContributorImpactProfile entries (for genesis export).
func (k Keeper) GetAllImpactProfiles(ctx context.Context) []types.ContributorImpactProfile {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixImpactProfile,
		storetypes.PrefixEndBytes(types.KeyPrefixImpactProfile),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ContributorImpactProfile
	for ; iter.Valid(); iter.Next() {
		var p types.ContributorImpactProfile
		if err := json.Unmarshal(iter.Value(), &p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ============================================================================
// Usage Graph Edges
// ============================================================================

// RecordUsageEdge adds a ContributionUsageEdge to the graph and enqueues the parent
// for impact recalculation. Self-references are filtered when the governance param
// SelfReferenceFilterEnabled is true.
func (k Keeper) RecordUsageEdge(ctx context.Context, edge types.ContributionUsageEdge) error {
	ip := k.GetImpactParams(ctx)
	if !ip.EnableImpactScoring {
		return nil
	}

	// Self-reference filter: parent and child must have different contributors.
	if ip.SelfReferenceFilterEnabled {
		parentContrib, found := k.GetContribution(ctx, edge.ParentClaimID)
		if found && parentContrib.Contributor == edge.ChildContributor {
			return nil // silently drop self-reference
		}
	}

	// Deduplicate: only one edge per (parent, child, referenceType).
	store := k.storeService.OpenKVStore(ctx)
	edgeKey := types.GetUsageEdgeKey(edge.ParentClaimID, edge.ChildClaimID)
	existing, err := store.Get(edgeKey)
	if err == nil && len(existing) > 0 {
		// Edge already exists — only update if reference type changes to invocation
		// (invocations have richer signal than provenance-only links).
		if edge.ReferenceType != "invocation" {
			return nil
		}
	}

	// Enforce MaxEdgesPerClaim by checking the count via index prefix iteration.
	if ip.MaxEdgesPerClaim > 0 {
		count := k.countUsageEdgesForParent(ctx, edge.ParentClaimID)
		if count >= int(ip.MaxEdgesPerClaim) {
			return nil // silently drop overflow edges
		}
	}

	// Write edge.
	bz, err := json.Marshal(edge)
	if err != nil {
		return err
	}
	if err := store.Set(edgeKey, bz); err != nil {
		return err
	}

	// Write parent index.
	indexKey := types.GetUsageEdgeByParentKey(edge.ParentClaimID, edge.ChildClaimID)
	if err := store.Set(indexKey, []byte{0x01}); err != nil {
		return err
	}

	// Enqueue parent claim for impact update.
	return k.enqueueImpactUpdate(ctx, edge.ParentClaimID)
}

// countUsageEdgesForParent counts the number of outgoing edges for a parent claim.
func (k Keeper) countUsageEdgesForParent(ctx context.Context, parentClaimID uint64) int {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.GetUsageEdgeByParentPrefix(parentClaimID)
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return 0
	}
	defer iter.Close()
	count := 0
	for ; iter.Valid(); iter.Next() {
		count++
	}
	return count
}

// GetUsageEdgesByParent returns all usage edges where the given claimID is the parent.
func (k Keeper) GetUsageEdgesByParent(ctx context.Context, parentClaimID uint64) []types.ContributionUsageEdge {
	store := k.storeService.OpenKVStore(ctx)
	// Iterate the by-parent index to get child IDs, then fetch the actual edges.
	prefix := types.GetUsageEdgeByParentPrefix(parentClaimID)
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ContributionUsageEdge
	for ; iter.Valid(); iter.Next() {
		// Key layout: prefix | parent_id(8) | child_id(8) — we need the child from the key.
		keyBytes := iter.Key()
		if len(keyBytes) < 8 {
			continue
		}
		childID := sdk.BigEndianToUint64(keyBytes[len(keyBytes)-8:])
		edgeKey := types.GetUsageEdgeKey(parentClaimID, childID)
		bz, err := store.Get(edgeKey)
		if err != nil || bz == nil {
			continue
		}
		var e types.ContributionUsageEdge
		if err := json.Unmarshal(bz, &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// GetAllUsageEdges returns all usage edges (for genesis export).
func (k Keeper) GetAllUsageEdges(ctx context.Context) []types.ContributionUsageEdge {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixUsageEdge,
		storetypes.PrefixEndBytes(types.KeyPrefixUsageEdge),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.ContributionUsageEdge
	for ; iter.Valid(); iter.Next() {
		var e types.ContributionUsageEdge
		if err := json.Unmarshal(iter.Value(), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ============================================================================
// Impact Update Queue
// ============================================================================

// enqueueImpactUpdate adds a claim ID to the impact update queue.
// Idempotent: writing the same ID twice is safe (presence-only index).
func (k Keeper) enqueueImpactUpdate(ctx context.Context, claimID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.GetImpactUpdateQueueKey(claimID), []byte{0x01})
}

// dequeueImpactUpdate removes a claim ID from the update queue.
func (k Keeper) dequeueImpactUpdate(ctx context.Context, claimID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.GetImpactUpdateQueueKey(claimID))
}

// ============================================================================
// Circular Loop Detection
// ============================================================================

// hasCircularDependency returns true if claimID is reachable from itself via the
// provenance/usage graph using iterative DFS. Guards against Layer 5 gaming via
// artificial circular dependency inflation.
func (k Keeper) hasCircularDependency(ctx context.Context, startClaimID uint64, maxDepth uint32) bool {
	if maxDepth == 0 {
		return false
	}
	visited := map[uint64]bool{startClaimID: true}
	queue := []uint64{startClaimID}
	depth := 0

	for len(queue) > 0 && depth < int(maxDepth) {
		nextQueue := []uint64{}
		for _, claimID := range queue {
			edges := k.GetUsageEdgesByParent(ctx, claimID)
			for _, e := range edges {
				if e.ChildClaimID == startClaimID {
					return true // cycle detected
				}
				if !visited[e.ChildClaimID] {
					visited[e.ChildClaimID] = true
					nextQueue = append(nextQueue, e.ChildClaimID)
				}
			}
		}
		queue = nextQueue
		depth++
	}
	return false
}

// ============================================================================
// Impact Score Calculation
// ============================================================================

// CalculateImpactRecord computes a fresh ContributionImpactRecord for a claim
// using all currently recorded usage edges and governance params.
func (k Keeper) CalculateImpactRecord(ctx context.Context, claimID uint64) types.ContributionImpactRecord {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ip := k.GetImpactParams(ctx)

	edges := k.GetUsageEdgesByParent(ctx, claimID)

	// Count distinct referrers and validators.
	distinctContributors := map[string]bool{}
	distinctValidators := map[string]bool{}
	var reuseCount, invocationCount uint64
	for _, e := range edges {
		switch e.ReferenceType {
		case "provenance":
			reuseCount++
			distinctContributors[e.ChildContributor] = true
		case "invocation":
			invocationCount++
		case "endorsement":
			distinctValidators[e.ChildContributor] = true
		}
	}

	// Clamp counts for scoring (avoid score inflation).
	reuseCapped := min64(reuseCount, 100)
	invocationCapped := min64(invocationCount, 500)
	validatorCapped := min64(uint64(len(distinctValidators)), 50)

	// Utility score [0, 100] = weighted sum of signals, normalized.
	reuseSignal := float32(reuseCapped) / 100.0
	invocationSignal := float32(invocationCapped) / 500.0
	validatorSignal := float32(validatorCapped) / 50.0

	utilityRaw := reuseSignal*float32(ip.UtilityReuseWeight)/10000.0 +
		invocationSignal*float32(ip.UtilityInvocationWeight)/10000.0 +
		validatorSignal*float32(ip.UtilityValidatorWeight)/10000.0
	utilityScore := clampUint32(uint32(utilityRaw*100), 0, 100)

	// Dependency count = distinct contributors who reference this claim.
	dependencyCount := uint32(len(distinctContributors))

	// Impact score [0, 100] = utility + dependency depth + trajectory (simplified as EMA).
	depSignal := float32(min64(uint64(dependencyCount), 50)) / 50.0
	depScore := depSignal * float32(ip.ImpactDependencyWeight) / 10000.0 * 100.0

	utilityContrib := float32(utilityScore) * float32(ip.ImpactUtilityWeight) / 10000.0

	// Trajectory: compare to existing impact score to compute improvement trend.
	var trajectoryScore float32
	if existing, found := k.GetImpactRecord(ctx, claimID); found {
		delta := float32(utilityScore) - float32(existing.UtilityScore)
		if delta > 0 {
			trajectoryScore = delta / 100.0 * float32(ip.ImpactTrajectoryWeight) / 10000.0 * 100.0
		}
	}

	impactRaw := utilityContrib + depScore + trajectoryScore
	impactScore := clampUint32(uint32(impactRaw), 0, 100)

	// Anomaly detection: set flag if circular dependency found.
	anomalyFlag := k.hasCircularDependency(ctx, claimID, ip.DiminishingReturnDepth)

	// ImpactMultiplier calculation (stored in basis points):
	//   multiplier_bps = 10000 + (impactScore - 50) * 100
	//   bounds: [MultiplierMinBps, MultiplierMaxBps]
	rawMultBps := int32(10000) + (int32(impactScore)-50)*100
	if rawMultBps < 0 {
		rawMultBps = 0
	}
	multBps := types.ClampImpactMultiplier(uint32(rawMultBps), ip.MultiplierMinBps, ip.MultiplierMaxBps)

	// Retrieve epoch from ctx.
	currentEpoch := uint64(sdkCtx.BlockHeight() / 100) // approximate; replace with EpochsKeeper if available

	return types.ContributionImpactRecord{
		ClaimID:                claimID,
		UtilityScore:           utilityScore,
		ImpactScore:            impactScore,
		ImpactMultiplierBps:    multBps,
		ReuseCount:             uint32(reuseCapped),
		DependencyCount:        dependencyCount,
		InvocationCount:        invocationCount,
		ValidatorAdoptionCount: uint32(validatorCapped),
		LastUpdatedEpoch:       currentEpoch,
		AnomalyFlag:            anomalyFlag,
	}
}

// ============================================================================
// Contributor Profile Update
// ============================================================================

// UpdateImpactProfile recalculates and writes the ContributorImpactProfile for
// a contributor address based on all their current ContributionImpactRecords.
func (k Keeper) UpdateImpactProfile(ctx context.Context, contributor string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ip := k.GetImpactParams(ctx)
	profile := k.GetImpactProfile(ctx, contributor)

	// Retrieve all impact records for this contributor's claims.
	contributions := k.GetContributionsByContributor(ctx, contributor)

	var totalImpact, totalUtility uint64
	var highImpactCount, lowImpactPattern, trackedCount uint32

	for _, contrib := range contributions {
		record, found := k.GetImpactRecord(ctx, contrib.Id)
		if !found {
			continue
		}
		trackedCount++
		totalImpact += uint64(record.ImpactScore)
		totalUtility += uint64(record.UtilityScore)

		if record.ImpactScore >= ip.HighImpactThreshold {
			highImpactCount++
		}
		if record.ImpactScore < ip.LowImpactThreshold {
			lowImpactPattern++
		}
	}

	// Aggregate impact capped at 10000.
	aggregateImpact := clampUint32(uint32(totalImpact), 0, 10000)

	avgUtility := uint32(0)
	if trackedCount > 0 {
		avgUtility = uint32(totalUtility / uint64(trackedCount))
	}

	// Trust adjustment: apply penalty for low-impact pattern exceeding limit.
	trust := profile.TrustAdjustmentFactor
	if trust == 0 {
		trust = 10000 // initialize to neutral
	}
	if lowImpactPattern > ip.LowImpactPatternLimit {
		excess := lowImpactPattern - ip.LowImpactPatternLimit
		penalty := uint32(excess) * ip.TrustPenaltyBps
		if penalty > trust-ip.TrustMinBps {
			trust = ip.TrustMinBps
		} else {
			trust -= penalty
		}
	}
	// Recovery for high-impact contributions.
	recovery := highImpactCount * ip.TrustRecoveryBps
	trust += recovery
	if trust > ip.TrustMaxBps {
		trust = ip.TrustMaxBps
	}

	currentEpoch := uint64(sdkCtx.BlockHeight() / 100)

	_ = k.SetImpactProfile(ctx, types.ContributorImpactProfile{
		Address:               contributor,
		AggregateImpactScore:  aggregateImpact,
		AverageUtilityScore:   avgUtility,
		HighImpactCount:       highImpactCount,
		LowImpactPatternCount: lowImpactPattern,
		TrustAdjustmentFactor: trust,
		LastUpdatedEpoch:      currentEpoch,
		TotalTrackedClaims:    trackedCount,
	})
}

// ============================================================================
// GetEffectiveImpactMultiplier
// ============================================================================

// GetEffectiveImpactMultiplier returns the combined impact multiplier for a contributor
// as a LegacyDec, incorporating both the best claim's ImpactMultiplier and the
// contributor's TrustAdjustmentFactor.
//
// Returns 1.0 (neutral) when Layer 5 is disabled or no records exist.
func (k Keeper) GetEffectiveImpactMultiplier(ctx context.Context, contributor string) math.LegacyDec {
	ip := k.GetImpactParams(ctx)
	if !ip.EnableImpactScoring {
		return math.LegacyOneDec()
	}

	profile := k.GetImpactProfile(ctx, contributor)
	if profile.TotalTrackedClaims == 0 {
		return math.LegacyOneDec()
	}

	// Find best ImpactMultiplier across contributor's claims.
	contributions := k.GetContributionsByContributor(ctx, contributor)
	bestMultBps := uint32(ip.MultiplierMinBps)
	for _, contrib := range contributions {
		record, found := k.GetImpactRecord(ctx, contrib.Id)
		if !found {
			continue
		}
		if record.AnomalyFlag {
			continue // ignore anomalous contributions
		}
		if record.ImpactMultiplierBps > bestMultBps {
			bestMultBps = record.ImpactMultiplierBps
		}
	}

	// Combine claim multiplier with trust adjustment.
	// effectiveBps = bestMultBps * trustBps / 10000
	trustBps := profile.TrustAdjustmentFactor
	if trustBps == 0 {
		trustBps = 10000
	}
	effectiveBps := uint64(bestMultBps) * uint64(trustBps) / 10000
	clamped := types.ClampImpactMultiplier(uint32(effectiveBps), ip.MultiplierMinBps, ip.MultiplierMaxBps)

	return types.ImpactMultiplierDec(clamped)
}

// ============================================================================
// EndBlocker: Incremental Impact Update Pass
// ============================================================================

// ProcessImpactUpdates drains up to EpochBatchSize entries from the impact update
// queue, recalculates each claim's ImpactRecord, and updates the contributor's profile.
// Called from EndBlock after review finalization.
func (k Keeper) ProcessImpactUpdates(ctx context.Context) error {
	ip := k.GetImpactParams(ctx)
	if !ip.EnableImpactScoring {
		return nil
	}

	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(
		types.KeyPrefixImpactUpdateQueue,
		storetypes.PrefixEndBytes(types.KeyPrefixImpactUpdateQueue),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	batchSize := int(ip.EpochBatchSize)
	if batchSize <= 0 {
		batchSize = 50
	}

	updatedContributors := map[string]bool{}

	processed := 0
	for ; iter.Valid() && processed < batchSize; iter.Next() {
		// Extract claimID from queue key.
		keyBytes := iter.Key()
		if len(keyBytes) < 8 {
			continue
		}
		claimID := sdk.BigEndianToUint64(keyBytes[len(keyBytes)-8:])

		// Recalculate impact record.
		record := k.CalculateImpactRecord(ctx, claimID)
		if err := k.SetImpactRecord(ctx, record); err != nil {
			k.logger.Error("failed to set impact record", "claim_id", claimID, "error", err)
			continue
		}

		// Emit event.
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				"poc_impact_score_updated",
				sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
				sdk.NewAttribute("impact_score", itoa(record.ImpactScore)),
				sdk.NewAttribute("utility_score", itoa(record.UtilityScore)),
				sdk.NewAttribute("multiplier_bps", itoa(record.ImpactMultiplierBps)),
			),
		)

		// Mark contributor for profile update.
		if contrib, found := k.GetContribution(ctx, claimID); found {
			updatedContributors[contrib.Contributor] = true
		}

		// Dequeue.
		_ = k.dequeueImpactUpdate(ctx, claimID)
		processed++
	}

	// Update contributor profiles for all affected contributors.
	for contributor := range updatedContributors {
		k.UpdateImpactProfile(ctx, contributor)
	}

	return nil
}

// ============================================================================
// Helpers
// ============================================================================

// min64 returns the smaller of two uint64 values.
func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// clampUint32 clamps a uint32 within [min, max].
func clampUint32(v, minV, maxV uint32) uint32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

// itoa converts a uint32 to a decimal string attribute value.
func itoa(v uint32) string {
	return fmt.Sprintf("%d", v)
}

// GetContributionsByContributor returns all contributions by a given contributor address.
// Uses the contributor index at KeyPrefixContributorIndex.
func (k Keeper) GetContributionsByContributor(ctx context.Context, contributor string) []types.Contribution {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(types.KeyPrefixContributorIndex, []byte(contributor)...)
	iter, err := store.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var out []types.Contribution
	for ; iter.Valid(); iter.Next() {
		// The key suffix is the contribution ID (8 bytes big-endian).
		keyBytes := iter.Key()
		if len(keyBytes) < 8 {
			continue
		}
		id := sdk.BigEndianToUint64(keyBytes[len(keyBytes)-8:])
		contrib, found := k.GetContribution(ctx, id)
		if !found {
			continue
		}
		out = append(out, contrib)
	}
	return out
}
