package keeper

import (
	"context"
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// KeyExtendedGenesis is the store key for the JSON-encoded ExtendedGenesisState sidecar.
// Using a dedicated key avoids proto field descriptor regeneration requirements.
var KeyExtendedGenesis = []byte{0x50}

// ExtendedGenesisState captures all PoC module state that cannot fit in the proto-generated
// GenesisState (which only holds Params, Contributions, Credits, NextContributionId, FeeMetrics,
// ContributorFeeStats). On export this struct is JSON-marshalled and stored at KeyExtendedGenesis
// so that InitGenesis can restore full module state across network upgrades.
type ExtendedGenesisState struct {
	VestingSchedules      []types.VestingSchedule              `json:"vesting_schedules"`
	ARVSSchedules         []types.ARVSVestingSchedule          `json:"arvs_schedules"`
	ReviewSessions        []types.ReviewSession                `json:"review_sessions"`
	ReviewerProfiles      []types.ReviewerProfile              `json:"reviewer_profiles"`
	ProvenanceEntries     []types.ProvenanceEntry              `json:"provenance_entries"`
	ContributorStats      []types.ContributorStats             `json:"contributor_stats"`
	CtypeWeights          map[string]uint32                    `json:"ctype_weights"`
	MinQualityForEmission uint32                               `json:"min_quality_for_emission"`
	// Layer 5: Utility & Impact Scoring state
	ImpactRecords  []types.ContributionImpactRecord  `json:"impact_records,omitempty"`
	ImpactProfiles []types.ContributorImpactProfile  `json:"impact_profiles,omitempty"`
	UsageEdges     []types.ContributionUsageEdge     `json:"usage_edges,omitempty"`
	ImpactParams   *types.ImpactParams               `json:"impact_params,omitempty"`
}

// InitGenesis initializes the module's state from a provided genesis state
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	// Set params
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}

	// Set next contribution ID
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(types.KeyNextContributionID, sdk.Uint64ToBigEndian(gs.NextContributionId)); err != nil {
		return err
	}

	// Import contributions
	for _, contribution := range gs.Contributions {
		if err := k.SetContribution(ctx, contribution); err != nil {
			return err
		}
	}

	// Import credits
	for _, credits := range gs.Credits {
		if err := k.SetCredits(ctx, credits); err != nil {
			return err
		}
	}

	// Import fee metrics
	if err := k.SetFeeMetrics(ctx, gs.FeeMetrics); err != nil {
		return err
	}

	// Import contributor fee stats
	for _, stats := range gs.ContributorFeeStats {
		if err := k.SetContributorFeeStats(ctx, stats); err != nil {
			return err
		}
	}

	// Import extended genesis sidecar (vesting, ARVS, reviews, provenance, stats, sidecars)
	extBz, extErr := store.Get(KeyExtendedGenesis)
	if extErr == nil && len(extBz) > 0 {
		var ext ExtendedGenesisState
		if jsonErr := json.Unmarshal(extBz, &ext); jsonErr == nil {
			for _, vs := range ext.VestingSchedules {
				_ = k.SetVestingSchedule(ctx, vs)
			}
			for _, as := range ext.ARVSSchedules {
				_ = k.SetARVSVestingSchedule(ctx, as)
			}
			for _, rs := range ext.ReviewSessions {
				_ = k.SetReviewSession(ctx, rs)
			}
			for _, rp := range ext.ReviewerProfiles {
				_ = k.SetReviewerProfile(ctx, rp)
			}
			for _, pe := range ext.ProvenanceEntries {
				_ = k.SetProvenanceEntry(ctx, pe)
			}
			for _, cs := range ext.ContributorStats {
				_ = k.SetContributorStats(ctx, cs)
			}
			if len(ext.CtypeWeights) > 0 {
				_ = k.SetCtypeWeights(ctx, ext.CtypeWeights)
			}
			if ext.MinQualityForEmission > 0 {
				_ = k.SetMinQualityForEmission(ctx, ext.MinQualityForEmission)
			}
			// Layer 5: restore impact scoring state
			for _, ir := range ext.ImpactRecords {
				_ = k.SetImpactRecord(ctx, ir)
			}
			for _, ip := range ext.ImpactProfiles {
				_ = k.SetImpactProfile(ctx, ip)
			}
			for _, ue := range ext.UsageEdges {
				// Restore edges directly without re-triggering anti-gaming checks or queue.
				store2 := k.storeService.OpenKVStore(ctx)
				if bz2, e2 := json.Marshal(ue); e2 == nil {
					_ = store2.Set(types.GetUsageEdgeKey(ue.ParentClaimID, ue.ChildClaimID), bz2)
					_ = store2.Set(types.GetUsageEdgeByParentKey(ue.ParentClaimID, ue.ChildClaimID), []byte{0x01})
				}
			}
			if ext.ImpactParams != nil {
				_ = k.SetImpactParams(ctx, *ext.ImpactParams)
			}
		}
	}

	return nil
}

// ExportGenesis returns the module's exported genesis
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	contributions := k.GetAllContributions(ctx)
	credits := k.GetAllCredits(ctx)

	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyNextContributionID)

	var nextID uint64
	if err != nil || bz == nil {
		// If there's an error or no value, start from 1
		// Log warning but don't fail genesis export
		k.logger.Warn("failed to get next contribution ID during genesis export, defaulting to 1", "error", err)
		nextID = 1
	} else {
		nextID = sdk.BigEndianToUint64(bz)
	}

	// Export fee metrics
	feeMetrics := k.GetFeeMetrics(ctx)

	// Export contributor fee stats
	contributorFeeStats, err := k.GetAllContributorFeeStats(ctx)
	if err != nil {
		// Log error but don't fail genesis export - use empty slice
		k.logger.Error("failed to export contributor fee stats", "error", err)
		contributorFeeStats = []types.ContributorFeeStats{}
	}

	// Build and persist extended genesis sidecar (state not representable in proto GenesisState)
	impactParams := k.GetImpactParams(ctx)
	ext := ExtendedGenesisState{
		VestingSchedules:      k.GetAllVestingSchedules(ctx),
		ARVSSchedules:         k.GetAllARVSVestingSchedules(ctx),
		ReviewSessions:        k.GetAllReviewSessions(ctx),
		ReviewerProfiles:      k.GetAllReviewerProfiles(ctx),
		ProvenanceEntries:     k.GetAllProvenanceEntries(ctx),
		ContributorStats:      k.GetAllContributorStats(ctx),
		CtypeWeights:          k.GetCtypeWeights(ctx),
		MinQualityForEmission: k.GetMinQualityForEmission(ctx),
		// Layer 5
		ImpactRecords:  k.GetAllImpactRecords(ctx),
		ImpactProfiles: k.GetAllImpactProfiles(ctx),
		UsageEdges:     k.GetAllUsageEdges(ctx),
		ImpactParams:   &impactParams,
	}
	if extBz, extErr := json.Marshal(ext); extErr == nil {
		extStore := k.storeService.OpenKVStore(ctx)
		if setErr := extStore.Set(KeyExtendedGenesis, extBz); setErr != nil {
			k.logger.Error("failed to persist extended genesis sidecar", "error", setErr)
		}
	}

	return &types.GenesisState{
		Params:              params,
		Contributions:       contributions,
		Credits:             credits,
		NextContributionId:  nextID,
		FeeMetrics:          feeMetrics,
		ContributorFeeStats: contributorFeeStats,
	}
}
