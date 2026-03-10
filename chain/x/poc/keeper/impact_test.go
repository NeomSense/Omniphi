package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// TestImpactParams_DefaultsAndPersistence verifies that ImpactParams round-trip
// through the store and that defaults are returned when no params are set.
func TestImpactParams_DefaultsAndPersistence(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Default params should be returned before any Set.
	got := fixture.keeper.GetImpactParams(ctx)
	def := types.DefaultImpactParams()
	require.Equal(t, def.MultiplierMinBps, got.MultiplierMinBps)
	require.Equal(t, def.MultiplierMaxBps, got.MultiplierMaxBps)
	require.False(t, got.EnableImpactScoring, "impact scoring should be disabled by default")

	// Mutate and persist.
	def.EnableImpactScoring = true
	def.MultiplierMaxBps = 20000
	require.NoError(t, fixture.keeper.SetImpactParams(ctx, def))

	// Round-trip.
	got2 := fixture.keeper.GetImpactParams(ctx)
	require.True(t, got2.EnableImpactScoring)
	require.Equal(t, uint32(20000), got2.MultiplierMaxBps)
}

// TestImpactRecord_CRUD verifies create/read for ContributionImpactRecord.
func TestImpactRecord_CRUD(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	_, found := fixture.keeper.GetImpactRecord(ctx, 99)
	require.False(t, found, "should not exist yet")

	rec := types.ContributionImpactRecord{
		ClaimID:             99,
		UtilityScore:        42,
		ImpactScore:         55,
		ImpactMultiplierBps: 11000,
		ReuseCount:          3,
		DependencyCount:     2,
		InvocationCount:     10,
		LastUpdatedEpoch:    5,
	}
	require.NoError(t, fixture.keeper.SetImpactRecord(ctx, rec))

	got, found := fixture.keeper.GetImpactRecord(ctx, 99)
	require.True(t, found)
	require.Equal(t, rec.UtilityScore, got.UtilityScore)
	require.Equal(t, rec.ImpactMultiplierBps, got.ImpactMultiplierBps)
}

// TestImpactProfile_CRUD verifies create/read for ContributorImpactProfile.
func TestImpactProfile_CRUD(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	addr := "cosmos1qyqszqgpqyqszqgpqyqszqgpqyqszqgpjnst99"

	// Unset profile returns neutral trust.
	profile := fixture.keeper.GetImpactProfile(ctx, addr)
	require.Equal(t, uint32(10000), profile.TrustAdjustmentFactor)

	profile.AggregateImpactScore = 200
	profile.HighImpactCount = 3
	profile.TrustAdjustmentFactor = 12000
	require.NoError(t, fixture.keeper.SetImpactProfile(ctx, profile))

	got := fixture.keeper.GetImpactProfile(ctx, addr)
	require.Equal(t, uint32(200), got.AggregateImpactScore)
	require.Equal(t, uint32(12000), got.TrustAdjustmentFactor)
}

// TestUsageEdge_SelfReferenceFilter verifies that self-references are dropped.
func TestUsageEdge_SelfReferenceFilter(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Enable impact scoring.
	ip := types.DefaultImpactParams()
	ip.EnableImpactScoring = true
	ip.SelfReferenceFilterEnabled = true
	require.NoError(t, fixture.keeper.SetImpactParams(ctx, ip))

	// Create a contribution for the parent.
	parentContrib := types.Contribution{
		Id:          1,
		Contributor: "alice",
		Ctype:       "code",
	}
	require.NoError(t, fixture.keeper.SetContribution(ctx, parentContrib))

	// Self-reference: child contributor == parent contributor.
	selfEdge := types.ContributionUsageEdge{
		ParentClaimID:    1,
		ChildClaimID:     2,
		ReferenceType:    "provenance",
		ChildContributor: "alice",
		Timestamp:        1000,
		Epoch:            1,
	}
	require.NoError(t, fixture.keeper.RecordUsageEdge(ctx, selfEdge))

	// The edge should NOT be stored (self-reference dropped).
	edges := fixture.keeper.GetUsageEdgesByParent(ctx, 1)
	require.Empty(t, edges, "self-reference should be filtered out")
}

// TestUsageEdge_CrossContributorRecorded verifies that cross-contributor edges ARE stored.
func TestUsageEdge_CrossContributorRecorded(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Enable impact scoring.
	ip := types.DefaultImpactParams()
	ip.EnableImpactScoring = true
	ip.SelfReferenceFilterEnabled = true
	require.NoError(t, fixture.keeper.SetImpactParams(ctx, ip))

	// Parent contribution belongs to alice.
	parentContrib := types.Contribution{
		Id:          10,
		Contributor: "alice",
		Ctype:       "code",
	}
	require.NoError(t, fixture.keeper.SetContribution(ctx, parentContrib))

	// Bob references alice's contribution.
	edge := types.ContributionUsageEdge{
		ParentClaimID:    10,
		ChildClaimID:     20,
		ReferenceType:    "provenance",
		ChildContributor: "bob",
		Timestamp:        1001,
		Epoch:            1,
	}
	require.NoError(t, fixture.keeper.RecordUsageEdge(ctx, edge))

	edges := fixture.keeper.GetUsageEdgesByParent(ctx, 10)
	require.Len(t, edges, 1)
	require.Equal(t, "bob", edges[0].ChildContributor)
}

// TestCalculateImpactRecord_NoEdges verifies zero scores with no usage edges.
func TestCalculateImpactRecord_NoEdges(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	ip := types.DefaultImpactParams()
	ip.EnableImpactScoring = true
	require.NoError(t, fixture.keeper.SetImpactParams(ctx, ip))

	record := fixture.keeper.CalculateImpactRecord(ctx, 999)
	require.Equal(t, uint32(0), record.UtilityScore)
	require.Equal(t, uint32(0), record.ImpactScore)
	// Multiplier must be at lower bound.
	require.Equal(t, ip.MultiplierMinBps, record.ImpactMultiplierBps)
}

// TestCalculateImpactRecord_WithEdges verifies scoring increases with edges.
func TestCalculateImpactRecord_WithEdges(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	ip := types.DefaultImpactParams()
	ip.EnableImpactScoring = true
	require.NoError(t, fixture.keeper.SetImpactParams(ctx, ip))

	// Seed a parent contribution.
	parentContrib := types.Contribution{
		Id:          5,
		Contributor: "alice",
		Ctype:       "code",
	}
	require.NoError(t, fixture.keeper.SetContribution(ctx, parentContrib))

	// Record 5 provenance edges from distinct contributors.
	for i := uint64(0); i < 5; i++ {
		edge := types.ContributionUsageEdge{
			ParentClaimID:    5,
			ChildClaimID:     100 + i,
			ReferenceType:    "provenance",
			ChildContributor: "contributor-" + string(rune('a'+i)),
			Timestamp:        1000,
			Epoch:            1,
		}
		_ = fixture.keeper.RecordUsageEdge(ctx, edge)
	}

	record := fixture.keeper.CalculateImpactRecord(ctx, 5)
	require.Equal(t, uint64(5), record.ClaimID)
	require.Greater(t, record.UtilityScore, uint32(0), "utility score should be positive with 5 edges")
	require.Greater(t, record.ImpactScore, uint32(0), "impact score should be positive")
	require.GreaterOrEqual(t, record.ImpactMultiplierBps, ip.MultiplierMinBps)
	require.LessOrEqual(t, record.ImpactMultiplierBps, ip.MultiplierMaxBps)
}

// TestGetEffectiveImpactMultiplier_Disabled verifies 1.0x when disabled.
func TestGetEffectiveImpactMultiplier_Disabled(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	// Default params: EnableImpactScoring = false.
	mult := fixture.keeper.GetEffectiveImpactMultiplier(ctx, "alice")
	require.True(t, mult.Equal(math.LegacyOneDec()), "should return 1.0 when disabled")
}

// TestGetEffectiveImpactMultiplier_Enabled verifies non-neutral multiplier when on.
func TestGetEffectiveImpactMultiplier_Enabled(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	ip := types.DefaultImpactParams()
	ip.EnableImpactScoring = true
	require.NoError(t, fixture.keeper.SetImpactParams(ctx, ip))

	// Create a contribution for alice.
	contrib := types.Contribution{
		Id:          7,
		Contributor: "alice",
		Ctype:       "code",
	}
	require.NoError(t, fixture.keeper.SetContribution(ctx, contrib))

	// Store an impact record with a high multiplier.
	rec := types.ContributionImpactRecord{
		ClaimID:             7,
		ImpactScore:         80,
		UtilityScore:        70,
		ImpactMultiplierBps: 13000, // 1.3x
		LastUpdatedEpoch:    1,
	}
	require.NoError(t, fixture.keeper.SetImpactRecord(ctx, rec))

	// Store a profile so TotalTrackedClaims > 0.
	require.NoError(t, fixture.keeper.SetImpactProfile(ctx, types.ContributorImpactProfile{
		Address:               "alice",
		TrustAdjustmentFactor: 10000, // neutral
		TotalTrackedClaims:    1,
	}))

	mult := fixture.keeper.GetEffectiveImpactMultiplier(ctx, "alice")
	// 13000 * 10000 / 10000 = 13000 bps = 1.3x — clamped to [0.8, 1.5]
	expected := types.ImpactMultiplierDec(13000)
	require.True(t, mult.Equal(expected), "expected 1.3x, got %s", mult.String())
}

// TestProcessImpactUpdates_NoError verifies EndBlocker doesn't error with empty queue.
func TestProcessImpactUpdates_NoError(t *testing.T) {
	fixture := SetupKeeperTest(t)
	ctx := fixture.ctx

	ip := types.DefaultImpactParams()
	ip.EnableImpactScoring = true
	ip.EpochBatchSize = 3
	require.NoError(t, fixture.keeper.SetImpactParams(ctx, ip))

	// Empty queue — should not panic or error.
	err := fixture.keeper.ProcessImpactUpdates(ctx)
	require.NoError(t, err)
}

// TestImpactParams_Clamp verifies ClampImpactMultiplier helper.
func TestImpactParams_Clamp(t *testing.T) {
	require.Equal(t, uint32(8000), types.ClampImpactMultiplier(5000, 8000, 15000))
	require.Equal(t, uint32(15000), types.ClampImpactMultiplier(20000, 8000, 15000))
	require.Equal(t, uint32(11000), types.ClampImpactMultiplier(11000, 8000, 15000))
}

// TestImpactMultiplierDec verifies basis-point to decimal conversion.
func TestImpactMultiplierDec(t *testing.T) {
	require.True(t, types.ImpactMultiplierDec(10000).Equal(math.LegacyOneDec()))
	require.True(t, types.ImpactMultiplierDec(8000).Equal(math.LegacyNewDecWithPrec(8, 1)))
	require.True(t, types.ImpactMultiplierDec(15000).Equal(math.LegacyNewDecWithPrec(15, 1)))
}
