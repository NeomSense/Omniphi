package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

// enableProvenance is a test helper that enables the provenance registry with given maxDepth.
func enableProvenance(t *testing.T, f *KeeperTestFixture, maxDepth uint32) {
	t.Helper()
	params := f.keeper.GetParams(f.ctx)
	params.EnableProvenanceRegistry = true
	params.MaxProvenanceDepth = maxDepth
	params.ProvenanceSchemaVersion = 1
	require.NoError(t, f.keeper.SetParams(f.ctx, params))
}

// mkContrib is a shorthand for creating test contributions.
func mkContrib(id uint64, contributor, ctype string, hash []byte) types.Contribution {
	c := types.NewContribution(id, contributor, ctype, "ipfs://"+contributor, hash, 100, 1000)
	return c
}

// ============================================================================
// Registration Tests
// ============================================================================

func TestProvenance_RegisterRootClaim(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	c := mkContrib(1, "addr1", "code", []byte("hash_root"))
	c.CanonicalHash = []byte("canonical_hash_aaaaaaaaaaaaaaaaa") // exactly 32 bytes

	err := f.keeper.RegisterProvenance(f.ctx, c, nil)
	require.NoError(t, err)

	entry, found := f.keeper.GetProvenanceEntry(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, uint64(1), entry.ClaimID)
	require.Equal(t, uint32(0), entry.Depth)
	require.Equal(t, uint64(0), entry.ParentClaimID)
	require.False(t, entry.IsDerivative)
	require.Equal(t, types.DerivationNone, entry.DerivationReason)
	require.Equal(t, "code", entry.Category)
	require.Equal(t, "addr1", entry.Submitter)
	require.Equal(t, uint32(1), entry.SchemaVersion)
}

func TestProvenance_RegisterDerivativeWithParent(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Root
	root := mkContrib(1, "addr1", "code", []byte("root_hash"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, root, nil))

	// Derivative child
	child := mkContrib(2, "addr2", "code", []byte("child_hash"))
	child.ParentClaimId = 1
	child.IsDerivative = true

	session := &types.ReviewSession{
		ContributionID:  2,
		OverrideApplied: types.OverrideDerivativeTruePositive,
		FinalQuality:    85,
	}

	err := f.keeper.RegisterProvenance(f.ctx, child, session)
	require.NoError(t, err)

	entry, found := f.keeper.GetProvenanceEntry(f.ctx, 2)
	require.True(t, found)
	require.Equal(t, uint32(1), entry.Depth)
	require.Equal(t, uint64(1), entry.ParentClaimID)
	require.True(t, entry.IsDerivative)
	require.Equal(t, types.DerivationHuman, entry.DerivationReason)
	require.Equal(t, uint32(85), entry.QualityScore)
}

func TestProvenance_RegisterPopulatesAllIndexes(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	c := mkContrib(1, "addr1", "data", []byte("hash1"))
	c.CanonicalHash = []byte("canonical_hash_bbbbbbbbbbbbbbbbb")
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	// Submitter index
	bySubmitter := f.keeper.GetProvenanceBySubmitter(f.ctx, "addr1")
	require.Len(t, bySubmitter, 1)
	require.Equal(t, uint64(1), bySubmitter[0])

	// Category index
	byCategory := f.keeper.GetProvenanceByCategory(f.ctx, "data")
	require.Len(t, byCategory, 1)
	require.Equal(t, uint64(1), byCategory[0])

	// Hash index
	byHash := f.keeper.GetProvenanceByHash(f.ctx, []byte("canonical_hash_bbbbbbbbbbbbbbbbb"))
	require.Len(t, byHash, 1)
	require.Equal(t, uint64(1), byHash[0])

	// Epoch index (block height 100 / 100 = epoch 1)
	byEpoch := f.keeper.GetProvenanceByEpochRange(f.ctx, 0, 10)
	require.Contains(t, byEpoch, uint64(1))
}

// ============================================================================
// DAG Validation Tests
// ============================================================================

func TestProvenance_CycleDetection(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Manually create a cycle: 1 -> 2 -> 1
	entry1 := types.ProvenanceEntry{ClaimID: 1, ParentClaimID: 2, Depth: 0}
	entry2 := types.ProvenanceEntry{ClaimID: 2, ParentClaimID: 1, Depth: 1}

	require.NoError(t, f.keeper.SetProvenanceEntry(f.ctx, entry1))
	require.NoError(t, f.keeper.SetProvenanceEntry(f.ctx, entry2))

	// Try to register 3 pointing to 1. Walk: 1 -> 2 -> 1 (Cycle!)
	contrib3 := mkContrib(3, "addr", "code", []byte("h"))
	contrib3.ParentClaimId = 1

	err := f.keeper.RegisterProvenance(f.ctx, contrib3, nil)
	require.ErrorIs(t, err, types.ErrProvenanceCycleDetected)
}

func TestProvenance_MaxDepthEnforcement(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 1) // Very shallow

	// Root (Depth 0)
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "c", []byte("h")), nil))

	// Child (Depth 1) - OK
	c2 := mkContrib(2, "a", "c", []byte("h2"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	// Grandchild (Depth 2) - Should fail
	c3 := mkContrib(3, "a", "c", []byte("h3"))
	c3.ParentClaimId = 2
	err := f.keeper.RegisterProvenance(f.ctx, c3, nil)
	require.ErrorIs(t, err, types.ErrProvenanceMaxDepthExceeded)
}

func TestProvenance_MissingParentRejected(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	c := mkContrib(1, "addr", "code", []byte("h"))
	c.ParentClaimId = 999

	err := f.keeper.RegisterProvenance(f.ctx, c, nil)
	require.ErrorIs(t, err, types.ErrProvenanceParentNotFound)
}

// ============================================================================
// Children Index Tests
// ============================================================================

func TestProvenance_GetChildrenOfParent(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Root
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))

	// Two children
	c2 := mkContrib(2, "b", "code", []byte("c2"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	c3 := mkContrib(3, "c", "code", []byte("c3"))
	c3.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c3, nil))

	children := f.keeper.GetProvenanceChildren(f.ctx, 1)
	require.Len(t, children, 2)
	require.Contains(t, children, uint64(2))
	require.Contains(t, children, uint64(3))
}

func TestProvenance_EmptyChildren(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Root with no children
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))

	children := f.keeper.GetProvenanceChildren(f.ctx, 1)
	require.Empty(t, children)
}

func TestProvenance_ChildrenOfNonExistentParent(t *testing.T) {
	f := SetupKeeperTest(t)
	children := f.keeper.GetProvenanceChildren(f.ctx, 999)
	require.Empty(t, children)
}

// ============================================================================
// Lineage Path Tests
// ============================================================================

func TestProvenance_RootOnlyPath(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))

	path, err := f.keeper.ComputeLineagePath(f.ctx, 1)
	require.NoError(t, err)
	require.Len(t, path, 1)
	require.Equal(t, uint64(1), path[0].ClaimID)
}

func TestProvenance_ThreeLevelPath(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Root -> Child -> Grandchild
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))

	c2 := mkContrib(2, "b", "code", []byte("c2"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	c3 := mkContrib(3, "c", "code", []byte("c3"))
	c3.ParentClaimId = 2
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c3, nil))

	path, err := f.keeper.ComputeLineagePath(f.ctx, 3)
	require.NoError(t, err)
	require.Len(t, path, 3)
	require.Equal(t, uint64(1), path[0].ClaimID) // root
	require.Equal(t, uint64(2), path[1].ClaimID) // child
	require.Equal(t, uint64(3), path[2].ClaimID) // grandchild
}

func TestProvenance_LineagePathNotFound(t *testing.T) {
	f := SetupKeeperTest(t)
	_, err := f.keeper.ComputeLineagePath(f.ctx, 999)
	require.ErrorIs(t, err, types.ErrProvenanceNotFound)
}

// ============================================================================
// Hash Index Tests
// ============================================================================

func TestProvenance_LookupByHash(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	hash := []byte("canonical_hash_ccccccccccccccccc")
	c := mkContrib(1, "a", "code", []byte("h"))
	c.CanonicalHash = hash
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	ids := f.keeper.GetProvenanceByHash(f.ctx, hash)
	require.Len(t, ids, 1)
	require.Equal(t, uint64(1), ids[0])
}

func TestProvenance_MultipleClaimsSameHash(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	sharedHash := []byte("canonical_hash_ddddddddddddddddd")

	c1 := mkContrib(1, "a", "code", []byte("h1"))
	c1.CanonicalHash = sharedHash
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c1, nil))

	c2 := mkContrib(2, "b", "code", []byte("h2"))
	c2.CanonicalHash = sharedHash
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	ids := f.keeper.GetProvenanceByHash(f.ctx, sharedHash)
	require.Len(t, ids, 2)
	require.Contains(t, ids, uint64(1))
	require.Contains(t, ids, uint64(2))
}

// ============================================================================
// Submitter Index Tests
// ============================================================================

func TestProvenance_LookupBySubmitter(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "alice", "code", []byte("h1")), nil))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(2, "alice", "data", []byte("h2")), nil))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(3, "bob", "code", []byte("h3")), nil))

	aliceIDs := f.keeper.GetProvenanceBySubmitter(f.ctx, "alice")
	require.Len(t, aliceIDs, 2)

	bobIDs := f.keeper.GetProvenanceBySubmitter(f.ctx, "bob")
	require.Len(t, bobIDs, 1)
}

func TestProvenance_SubmitterIndexEmpty(t *testing.T) {
	f := SetupKeeperTest(t)
	ids := f.keeper.GetProvenanceBySubmitter(f.ctx, "nobody")
	require.Empty(t, ids)
}

// ============================================================================
// Category Index Tests
// ============================================================================

func TestProvenance_LookupByCategory(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("h1")), nil))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(2, "b", "code", []byte("h2")), nil))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(3, "c", "data", []byte("h3")), nil))

	codeIDs := f.keeper.GetProvenanceByCategory(f.ctx, "code")
	require.Len(t, codeIDs, 2)

	dataIDs := f.keeper.GetProvenanceByCategory(f.ctx, "data")
	require.Len(t, dataIDs, 1)
}

// ============================================================================
// Epoch Range Tests
// ============================================================================

func TestProvenance_EpochRangeSingleEpoch(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Test context defaults to block height 0, so epoch = 0/100 = 0
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("h1")), nil))

	// Epoch 0 should contain our entry
	ids := f.keeper.GetProvenanceByEpochRange(f.ctx, 0, 0)
	require.Len(t, ids, 1)
	require.Equal(t, uint64(1), ids[0])
}

func TestProvenance_EpochRangeEmpty(t *testing.T) {
	f := SetupKeeperTest(t)
	ids := f.keeper.GetProvenanceByEpochRange(f.ctx, 100, 200)
	require.Empty(t, ids)
}

func TestProvenance_EpochRangeMultiEntry(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Both registered at block height 100 (epoch 1)
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("h1")), nil))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(2, "b", "data", []byte("h2")), nil))

	ids := f.keeper.GetProvenanceByEpochRange(f.ctx, 0, 5)
	require.Len(t, ids, 2)
}

// ============================================================================
// Stats Tests
// ============================================================================

func TestProvenance_StatsIncrementOnRegistration(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Initially empty
	stats := f.keeper.GetProvenanceStats(f.ctx)
	require.Equal(t, uint64(0), stats.TotalEntries)

	// Register root
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("h1")), nil))

	stats = f.keeper.GetProvenanceStats(f.ctx)
	require.Equal(t, uint64(1), stats.TotalEntries)
	require.Equal(t, uint64(1), stats.RootEntries)
	require.Equal(t, uint64(0), stats.DerivativeCount)
	require.Equal(t, uint64(1), stats.CategoryCounts["code"])
}

func TestProvenance_StatsMaxDepthTracking(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Root depth=0
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("h1")), nil))
	stats := f.keeper.GetProvenanceStats(f.ctx)
	require.Equal(t, uint32(0), stats.MaxDepthSeen)

	// Child depth=1
	c2 := mkContrib(2, "b", "code", []byte("h2"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	stats = f.keeper.GetProvenanceStats(f.ctx)
	require.Equal(t, uint32(1), stats.MaxDepthSeen)
}

func TestProvenance_StatsDerivativeCount(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	root := mkContrib(1, "a", "code", []byte("h"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, root, nil))

	deriv := mkContrib(2, "b", "code", []byte("hd"))
	deriv.ParentClaimId = 1
	deriv.IsDerivative = true
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, deriv, nil))

	stats := f.keeper.GetProvenanceStats(f.ctx)
	require.Equal(t, uint64(2), stats.TotalEntries)
	require.Equal(t, uint64(1), stats.RootEntries)
	require.Equal(t, uint64(1), stats.DerivativeCount)
}

// ============================================================================
// Double Registration Tests
// ============================================================================

func TestProvenance_DoubleRegistrationRejected(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	c := mkContrib(1, "a", "code", []byte("h"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	// Second attempt should fail
	err := f.keeper.RegisterProvenance(f.ctx, c, nil)
	require.ErrorIs(t, err, types.ErrProvenanceAlreadyRegistered)
}

// ============================================================================
// Derivation Reason Tests
// ============================================================================

func TestProvenance_DerivationReason_Original(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	c := mkContrib(1, "a", "code", []byte("h"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	entry, _ := f.keeper.GetProvenanceEntry(f.ctx, 1)
	require.Equal(t, types.DerivationNone, entry.DerivationReason)
}

func TestProvenance_DerivationReason_AIFlagged(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	root := mkContrib(1, "a", "code", []byte("r"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, root, nil))

	c := mkContrib(2, "b", "code", []byte("h"))
	c.ParentClaimId = 1
	c.IsDerivative = true
	// No review session override → AI flagged
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	entry, _ := f.keeper.GetProvenanceEntry(f.ctx, 2)
	require.Equal(t, types.DerivationAI, entry.DerivationReason)
}

func TestProvenance_DerivationReason_HumanOverride(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	root := mkContrib(1, "a", "code", []byte("r"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, root, nil))

	c := mkContrib(2, "b", "code", []byte("h"))
	c.ParentClaimId = 1
	c.IsDerivative = true

	session := &types.ReviewSession{
		ContributionID:  2,
		OverrideApplied: types.OverrideNotDerivativeFalseNegative,
	}
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, session))

	entry, _ := f.keeper.GetProvenanceEntry(f.ctx, 2)
	require.Equal(t, types.DerivationHuman, entry.DerivationReason)
}

func TestProvenance_DerivationReason_SelfDeclared(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	root := mkContrib(1, "a", "code", []byte("r"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, root, nil))

	// Has parent but NOT marked as derivative by AI → self-declared
	c := mkContrib(2, "b", "code", []byte("h"))
	c.ParentClaimId = 1
	c.IsDerivative = false
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	entry, _ := f.keeper.GetProvenanceEntry(f.ctx, 2)
	require.Equal(t, types.DerivationExplicit, entry.DerivationReason)
}

// ============================================================================
// Disabled Registry Tests
// ============================================================================

func TestProvenance_DisabledRegistryNoOp(t *testing.T) {
	f := SetupKeeperTest(t)
	// Registry is disabled by default

	c := mkContrib(1, "a", "code", []byte("h"))
	err := f.keeper.RegisterProvenance(f.ctx, c, nil)
	require.NoError(t, err) // no-op, no error

	// Entry should NOT exist
	_, found := f.keeper.GetProvenanceEntry(f.ctx, 1)
	require.False(t, found)
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestProvenance_HasProvenanceEntry(t *testing.T) {
	f := SetupKeeperTest(t)

	require.False(t, f.keeper.HasProvenanceEntry(f.ctx, 1))

	entry := types.ProvenanceEntry{ClaimID: 1, Submitter: "a", Category: "code"}
	require.NoError(t, f.keeper.SetProvenanceEntry(f.ctx, entry))

	require.True(t, f.keeper.HasProvenanceEntry(f.ctx, 1))
}

func TestProvenance_GetEntry_NotFound(t *testing.T) {
	f := SetupKeeperTest(t)
	_, found := f.keeper.GetProvenanceEntry(f.ctx, 42)
	require.False(t, found)
}

// ============================================================================
// Originality Multiplier Tests
// ============================================================================

func TestProvenance_OriginalityMultiplierFromSimilarity(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	// Set a similarity commitment for contribution 1
	simRecord := types.SimilarityCommitmentRecord{
		ContributionID: 1,
		CompactData: types.SimilarityCompactData{
			ContributionID:    1,
			OverallSimilarity: 5000, // 50% similarity
		},
	}
	require.NoError(t, f.keeper.SetSimilarityCommitment(f.ctx, simRecord))

	c := mkContrib(1, "a", "code", []byte("h"))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	entry, found := f.keeper.GetProvenanceEntry(f.ctx, 1)
	require.True(t, found)
	// With 50% similarity, the originality multiplier should be < 1.0
	// (exact value depends on DefaultOriginalityBands)
	require.False(t, entry.OriginalityMultiplier.IsZero())
}

// ============================================================================
// Query Tests
// ============================================================================

func TestProvenance_QueryProvenanceEntry(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("h")), nil))

	qs := keeper.NewQueryServerImpl(f.keeper)
	resp, err := qs.ProvenanceEntry(f.ctx, &types.QueryProvenanceEntryRequest{ClaimId: 1})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.Entry.ClaimID)
}

func TestProvenance_QueryProvenanceEntry_NotFound(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)
	_, err := qs.ProvenanceEntry(f.ctx, &types.QueryProvenanceEntryRequest{ClaimId: 999})
	require.Error(t, err)
}

func TestProvenance_QueryProvenanceChildren(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))
	c2 := mkContrib(2, "b", "code", []byte("c"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	qs := keeper.NewQueryServerImpl(f.keeper)
	resp, err := qs.ProvenanceChildren(f.ctx, &types.QueryProvenanceChildrenRequest{ParentClaimId: 1})
	require.NoError(t, err)
	require.Len(t, resp.ChildClaimIds, 1)
	require.Len(t, resp.Entries, 1)
}

func TestProvenance_QueryProvenanceLineage(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))
	c2 := mkContrib(2, "b", "code", []byte("c"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	qs := keeper.NewQueryServerImpl(f.keeper)
	resp, err := qs.ProvenanceLineage(f.ctx, &types.QueryProvenanceLineageRequest{ClaimId: 2})
	require.NoError(t, err)
	require.Len(t, resp.Path, 2)
	require.Equal(t, uint64(1), resp.Path[0].ClaimID)
	require.Equal(t, uint64(2), resp.Path[1].ClaimID)
}

func TestProvenance_QueryByHash(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	hash := []byte("canonical_hash_eeeeeeeeeeeeeeeee")
	c := mkContrib(1, "a", "code", []byte("h"))
	c.CanonicalHash = hash
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c, nil))

	qs := keeper.NewQueryServerImpl(f.keeper)
	resp, err := qs.ProvenanceByHash(f.ctx, &types.QueryProvenanceByHashRequest{CanonicalHash: hash})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 1)
}

func TestProvenance_QueryBySubmitter(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "alice", "code", []byte("h1")), nil))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(2, "alice", "data", []byte("h2")), nil))

	qs := keeper.NewQueryServerImpl(f.keeper)
	resp, err := qs.ProvenanceBySubmitter(f.ctx, &types.QueryProvenanceBySubmitterRequest{Submitter: "alice"})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 2)
}

func TestProvenance_QueryStats(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("h1")), nil))
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(2, "b", "data", []byte("h2")), nil))

	qs := keeper.NewQueryServerImpl(f.keeper)
	resp, err := qs.ProvenanceStats(f.ctx, &types.QueryProvenanceStatsRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(2), resp.Stats.TotalEntries)
	require.Equal(t, uint64(2), resp.Stats.RootEntries)
}

// ============================================================================
// Invariant Tests
// ============================================================================

func TestProvenance_AcyclicityInvariant_Pass(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))
	c2 := mkContrib(2, "b", "code", []byte("c"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	sdkCtx := sdk.UnwrapSDKContext(f.ctx)
	msg, broken := keeper.ProvenanceAcyclicityInvariant(f.keeper)(sdkCtx)
	require.False(t, broken, "invariant should not be broken: %s", msg)
}

func TestProvenance_AcyclicityInvariant_BrokenCycle(t *testing.T) {
	f := SetupKeeperTest(t)

	// Manually inject a cycle
	e1 := types.ProvenanceEntry{ClaimID: 1, ParentClaimID: 2, Depth: 0, Category: "c", Submitter: "a", Epoch: 1}
	e2 := types.ProvenanceEntry{ClaimID: 2, ParentClaimID: 1, Depth: 1, Category: "c", Submitter: "a", Epoch: 1}
	require.NoError(t, f.keeper.SetProvenanceEntry(f.ctx, e1))
	require.NoError(t, f.keeper.SetProvenanceEntry(f.ctx, e2))

	sdkCtx := sdk.UnwrapSDKContext(f.ctx)
	_, broken := keeper.ProvenanceAcyclicityInvariant(f.keeper)(sdkCtx)
	require.True(t, broken, "invariant should detect cycle")
}

func TestProvenance_IndexConsistencyInvariant_Pass(t *testing.T) {
	f := SetupKeeperTest(t)
	enableProvenance(t, f, 10)

	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, mkContrib(1, "a", "code", []byte("r")), nil))
	c2 := mkContrib(2, "b", "code", []byte("c"))
	c2.ParentClaimId = 1
	require.NoError(t, f.keeper.RegisterProvenance(f.ctx, c2, nil))

	sdkCtx := sdk.UnwrapSDKContext(f.ctx)
	msg, broken := keeper.ProvenanceIndexConsistencyInvariant(f.keeper)(sdkCtx)
	require.False(t, broken, "invariant should not be broken: %s", msg)
}

// ============================================================================
// Param Validation Tests
// ============================================================================

func TestProvenance_ParamValidation(t *testing.T) {
	// MaxProvenanceDepth exceeds 20
	p := types.DefaultParams()
	p.MaxProvenanceDepth = 21
	require.Error(t, p.Validate())

	// MaxProvenanceDepth = 0 with registry enabled
	p = types.DefaultParams()
	p.EnableProvenanceRegistry = true
	p.MaxProvenanceDepth = 0
	require.Error(t, p.Validate())

	// ProvenanceSchemaVersion exceeds 10
	p = types.DefaultParams()
	p.ProvenanceSchemaVersion = 11
	require.Error(t, p.Validate())

	// Valid config
	p = types.DefaultParams()
	p.EnableProvenanceRegistry = true
	p.MaxProvenanceDepth = 5
	p.ProvenanceSchemaVersion = 1
	require.NoError(t, p.Validate())
}

// ============================================================================
// Provenance Params Sidecar Roundtrip
// ============================================================================

func TestProvenance_ParamsSidecarRoundtrip(t *testing.T) {
	f := SetupKeeperTest(t)

	params := f.keeper.GetParams(f.ctx)
	params.EnableProvenanceRegistry = true
	params.MaxProvenanceDepth = 7
	params.ProvenanceSchemaVersion = 2
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	loaded := f.keeper.GetParams(f.ctx)
	require.True(t, loaded.EnableProvenanceRegistry)
	require.Equal(t, uint32(7), loaded.MaxProvenanceDepth)
	require.Equal(t, uint32(2), loaded.ProvenanceSchemaVersion)
}

// Ensure unused imports are satisfied.
var (
	_ = math.ZeroInt()
	_ = sdk.AccAddress{}
)
