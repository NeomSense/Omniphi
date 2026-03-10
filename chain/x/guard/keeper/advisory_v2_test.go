package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"pos/x/guard/keeper"
	"pos/x/guard/types"
)

// ============================================================================
// Layer 3 v2 Advisory Test Suite
// ============================================================================

type AdvisoryV2TestSuite struct {
	suite.Suite

	ctx    sdk.Context
	keeper keeper.Keeper
	cdc    codec.Codec

	govKeeper     *MockGovKeeper
	stakingKeeper *MockStakingKeeper
	bankKeeper    *MockBankKeeper
}

func TestAdvisoryV2TestSuite(t *testing.T) {
	suite.Run(t, new(AdvisoryV2TestSuite))
}

func (s *AdvisoryV2TestSuite) SetupTest() {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(s.T(), stateStore.LoadLatestVersion())

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	s.cdc = codec.NewProtoCodec(interfaceRegistry)

	s.ctx = sdk.NewContext(stateStore, tmproto.Header{Height: 100, Time: time.Now()}, false, log.NewNopLogger())

	s.govKeeper = NewMockGovKeeper()
	s.stakingKeeper = NewMockStakingKeeper()
	s.bankKeeper = NewMockBankKeeper()

	authority := sdk.AccAddress("authority_________").String()
	storeService := runtime.NewKVStoreService(storeKey)

	s.keeper = keeper.NewKeeper(
		s.cdc,
		storeService,
		authority,
		s.govKeeper,
		s.stakingKeeper,
		s.bankKeeper,
		log.NewNopLogger(),
	)

	err := s.keeper.SetParams(s.ctx, types.DefaultParams())
	require.NoError(s.T(), err)
}

// helper: queue a proposal
func (s *AdvisoryV2TestSuite) queueProposal(id uint64, msgTypeURL string) {
	var msgs []*codectypes.Any
	if msgTypeURL != "" {
		msgs = []*codectypes.Any{{TypeUrl: msgTypeURL}}
	}
	proposal := govtypes.Proposal{
		Id:       id,
		Messages: msgs,
		Status:   govtypes.StatusPassed,
	}
	s.govKeeper.SetProposal(proposal)
	err := s.keeper.OnProposalPassed(s.ctx, id)
	require.NoError(s.T(), err)
}

// ============================================================================
// Tests: Advisory Types Constants
// ============================================================================

func (s *AdvisoryV2TestSuite) TestAdvisoryTypes_AllValid() {
	allTypes := types.AllAdvisoryTypes()
	s.GreaterOrEqual(len(allTypes), 5)

	for _, t := range allTypes {
		s.True(types.IsValidAdvisoryType(t), "type %q should be valid", t)
	}
}

func (s *AdvisoryV2TestSuite) TestAdvisoryTypes_InvalidRejected() {
	s.False(types.IsValidAdvisoryType("INVALID"))
	s.False(types.IsValidAdvisoryType(""))
	s.False(types.IsValidAdvisoryType("risk_analysis")) // case-sensitive
}

func (s *AdvisoryV2TestSuite) TestAdvisorySchemaVersion() {
	s.Equal("v2", types.AdvisorySchemaVersion)
}

// ============================================================================
// Tests: Versioned Advisory Entry CRUD
// ============================================================================

func (s *AdvisoryV2TestSuite) TestSubmitAdvisoryEntryV2_Basic() {
	s.queueProposal(1, "/cosmos.mint.v1beta1.MsgUpdateParams")

	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "reporter1",
		URI:          "ipfs://Qm123",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeRiskAnalysis,
		TrackName:    "TRACK_PARAM_CHANGE",
	}

	advisoryID, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)
	s.Equal(uint64(1), advisoryID)

	// Retrieve
	retrieved, found := s.keeper.GetAdvisoryEntryV2(s.ctx, 1, 1)
	s.True(found)
	s.Equal("reporter1", retrieved.Reporter)
	s.Equal(types.AdvisorySchemaVersion, retrieved.SchemaVersion)
	s.Equal(types.AdvisoryTypeRiskAnalysis, retrieved.AdvisoryType)
	s.Equal(int64(100), retrieved.SubmittedAt) // from ctx block height
}

func (s *AdvisoryV2TestSuite) TestSubmitAdvisoryEntryV2_AutoIncrementID() {
	s.queueProposal(1, "")

	for i := 0; i < 3; i++ {
		entry := types.AdvisoryEntryV2{
			ProposalID:   1,
			Reporter:     "reporter",
			URI:          "ipfs://test",
			ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			AdvisoryType: types.AdvisoryTypeCommunityNote,
		}
		id, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
		require.NoError(s.T(), err)
		s.Equal(uint64(i+1), id)
	}

	// Verify all 3 are retrievable
	entries := s.keeper.GetAdvisoriesForProposal(s.ctx, 1)
	s.Equal(3, len(entries))
}

func (s *AdvisoryV2TestSuite) TestSubmitAdvisoryEntryV2_SnapshotsRiskTier() {
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "reporter",
		URI:          "ipfs://test",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeThreatReport,
	}

	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)

	retrieved, found := s.keeper.GetAdvisoryEntryV2(s.ctx, 1, 1)
	s.True(found)
	s.Equal(types.RISK_TIER_CRITICAL, retrieved.RiskTierAtSubmission)
}

func (s *AdvisoryV2TestSuite) TestGetAdvisoryEntryV2_NotFound() {
	_, found := s.keeper.GetAdvisoryEntryV2(s.ctx, 999, 1)
	s.False(found)
}

func (s *AdvisoryV2TestSuite) TestGetAdvisoriesForProposal_Empty() {
	entries := s.keeper.GetAdvisoriesForProposal(s.ctx, 999)
	s.Equal(0, len(entries))
}

// ============================================================================
// Tests: Advisory → Risk Correlation
// ============================================================================

func (s *AdvisoryV2TestSuite) TestAdvisoryCorrelation_RecordOnTerminal() {
	s.queueProposal(1, "/cosmos.mint.v1beta1.MsgUpdateParams")

	// Submit an advisory
	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "reporter",
		URI:          "ipfs://test",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeAuditReport,
	}
	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)

	// Record correlation on terminal
	err = s.keeper.RecordAdvisoryCorrelation(s.ctx, 1, "EXECUTED")
	require.NoError(s.T(), err)

	// Retrieve
	corr, found := s.keeper.GetAdvisoryCorrelation(s.ctx, 1)
	s.True(found)
	s.Equal(uint64(1), corr.ProposalID)
	s.Equal(uint64(1), corr.AdvisoryCount)
	s.Equal("EXECUTED", corr.ExecutionOutcome)
	s.Equal(int64(100), corr.FinalHeight)
}

func (s *AdvisoryV2TestSuite) TestAdvisoryCorrelation_NoAdvisories_NoRecord() {
	s.queueProposal(1, "")

	// No advisories submitted — should not create a correlation
	err := s.keeper.RecordAdvisoryCorrelation(s.ctx, 1, "EXECUTED")
	require.NoError(s.T(), err)

	_, found := s.keeper.GetAdvisoryCorrelation(s.ctx, 1)
	s.False(found)
}

func (s *AdvisoryV2TestSuite) TestAdvisoryCorrelation_NotFound() {
	_, found := s.keeper.GetAdvisoryCorrelation(s.ctx, 999)
	s.False(found)
}

// ============================================================================
// Tests: Attack Memory Dataset
// ============================================================================

func (s *AdvisoryV2TestSuite) TestAttackMemory_RecordOnAbort() {
	s.queueProposal(1, "/cosmos.mint.v1beta1.MsgUpdateParams")

	err := s.keeper.RecordAttackMemory(s.ctx, 1, types.MemoryTriggerAborted)
	require.NoError(s.T(), err)

	// Retrieve by feature hash
	report, _ := s.keeper.GetRiskReport(s.ctx, 1)
	entry, found := s.keeper.GetAttackMemoryEntry(s.ctx, report.FeaturesHash)
	s.True(found)
	s.Equal(uint64(1), entry.ProposalID)
	s.Equal(types.MemoryTriggerAborted, entry.TriggerReason)
	s.Equal("PARAM_CHANGE", entry.ProposalType)
}

func (s *AdvisoryV2TestSuite) TestAttackMemory_RecordByProposalIndex() {
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	err := s.keeper.RecordAttackMemory(s.ctx, 1, types.MemoryTriggerCriticalEscalation)
	require.NoError(s.T(), err)

	// Retrieve by proposal ID reverse index
	featureHash, found := s.keeper.GetAttackMemoryByProposal(s.ctx, 1)
	s.True(found)
	s.NotEmpty(featureHash)

	// Retrieve the actual entry
	entry, found := s.keeper.GetAttackMemoryEntry(s.ctx, featureHash)
	s.True(found)
	s.Equal("SOFTWARE_UPGRADE", entry.ProposalType)
}

func (s *AdvisoryV2TestSuite) TestAttackMemory_GetAll() {
	s.queueProposal(1, "/cosmos.mint.v1beta1.MsgUpdateParams")
	s.queueProposal(2, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	_ = s.keeper.RecordAttackMemory(s.ctx, 1, types.MemoryTriggerAborted)
	_ = s.keeper.RecordAttackMemory(s.ctx, 2, types.MemoryTriggerCriticalEscalation)

	entries := s.keeper.GetAllAttackMemoryEntries(s.ctx)
	s.GreaterOrEqual(len(entries), 2)
}

func (s *AdvisoryV2TestSuite) TestAttackMemory_NotFound() {
	_, found := s.keeper.GetAttackMemoryEntry(s.ctx, "nonexistent_hash")
	s.False(found)
}

func (s *AdvisoryV2TestSuite) TestAttackMemory_NoReportSkips() {
	// No proposal queued = no risk report = should silently skip
	err := s.keeper.RecordAttackMemory(s.ctx, 999, types.MemoryTriggerAborted)
	require.NoError(s.T(), err)
}

// ============================================================================
// Tests: Advisory Indexing
// ============================================================================

func (s *AdvisoryV2TestSuite) TestQueryAdvisoriesByTier() {
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade") // CRITICAL
	s.queueProposal(2, "")                                                // LOW (text)

	// Submit advisories
	entry1 := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "r1",
		URI:          "ipfs://1",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeThreatReport,
	}
	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry1)
	require.NoError(s.T(), err)

	entry2 := types.AdvisoryEntryV2{
		ProposalID:   2,
		Reporter:     "r2",
		URI:          "ipfs://2",
		ReportHash:   "1234560123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeCommunityNote,
	}
	_, err = s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry2)
	require.NoError(s.T(), err)

	// Query by CRITICAL tier
	criticalEntries := s.keeper.QueryAdvisoriesByTier(s.ctx, types.RISK_TIER_CRITICAL)
	s.Equal(1, len(criticalEntries))
	s.Equal(uint64(1), criticalEntries[0].ProposalID)

	// Query by LOW tier
	lowEntries := s.keeper.QueryAdvisoriesByTier(s.ctx, types.RISK_TIER_LOW)
	s.Equal(1, len(lowEntries))
	s.Equal(uint64(2), lowEntries[0].ProposalID)
}

func (s *AdvisoryV2TestSuite) TestQueryAdvisoriesByTrack() {
	s.queueProposal(1, "")

	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "r1",
		URI:          "ipfs://1",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeRiskAnalysis,
		TrackName:    "TRACK_UPGRADE",
	}
	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)

	entries := s.keeper.QueryAdvisoriesByTrack(s.ctx, "TRACK_UPGRADE")
	s.Equal(1, len(entries))

	// Query non-matching track
	entries = s.keeper.QueryAdvisoriesByTrack(s.ctx, "TRACK_OTHER")
	s.Equal(0, len(entries))
}

func (s *AdvisoryV2TestSuite) TestQueryAdvisoriesByOutcome() {
	s.queueProposal(1, "")

	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "r1",
		URI:          "ipfs://1",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeAuditReport,
	}
	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)

	// Record correlation (which writes outcome index)
	err = s.keeper.RecordAdvisoryCorrelation(s.ctx, 1, "EXECUTED")
	require.NoError(s.T(), err)

	entries := s.keeper.QueryAdvisoriesByOutcome(s.ctx, "EXECUTED")
	s.Equal(1, len(entries))
	s.Equal("EXECUTED", entries[0].Outcome)

	// Query non-matching outcome
	entries = s.keeper.QueryAdvisoriesByOutcome(s.ctx, "ABORTED")
	s.Equal(0, len(entries))
}

func (s *AdvisoryV2TestSuite) TestQueryAdvisoriesByTier_Empty() {
	entries := s.keeper.QueryAdvisoriesByTier(s.ctx, types.RISK_TIER_HIGH)
	s.Equal(0, len(entries))
}

// ============================================================================
// Tests: OnProposalTerminal Integration
// ============================================================================

func (s *AdvisoryV2TestSuite) TestOnProposalTerminal_Executed() {
	s.queueProposal(1, "")

	// Submit advisory
	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "r1",
		URI:          "ipfs://1",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeRiskAnalysis,
	}
	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)

	// Trigger terminal hook
	s.keeper.OnProposalTerminal(s.ctx, 1, "EXECUTED")

	// Correlation should exist
	corr, found := s.keeper.GetAdvisoryCorrelation(s.ctx, 1)
	s.True(found)
	s.Equal("EXECUTED", corr.ExecutionOutcome)
}

func (s *AdvisoryV2TestSuite) TestOnProposalTerminal_Aborted_RecordsAttackMemory() {
	s.queueProposal(1, "/cosmos.mint.v1beta1.MsgUpdateParams")

	// Submit advisory
	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "r1",
		URI:          "ipfs://1",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeThreatReport,
	}
	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)

	// Trigger terminal hook with ABORTED
	s.keeper.OnProposalTerminal(s.ctx, 1, "ABORTED")

	// Correlation should exist
	corr, found := s.keeper.GetAdvisoryCorrelation(s.ctx, 1)
	s.True(found)
	s.Equal("ABORTED", corr.ExecutionOutcome)

	// Attack memory should exist
	report, _ := s.keeper.GetRiskReport(s.ctx, 1)
	memEntry, found := s.keeper.GetAttackMemoryEntry(s.ctx, report.FeaturesHash)
	s.True(found)
	s.Equal(types.MemoryTriggerAborted, memEntry.TriggerReason)
}

func (s *AdvisoryV2TestSuite) TestOnProposalTerminal_NoAdvisories_NoCorrelation() {
	s.queueProposal(1, "")

	// Trigger terminal without any advisories
	s.keeper.OnProposalTerminal(s.ctx, 1, "EXECUTED")

	// No correlation should be created
	_, found := s.keeper.GetAdvisoryCorrelation(s.ctx, 1)
	s.False(found)
}

// ============================================================================
// Tests: Non-Binding Invariant
// ============================================================================

func (s *AdvisoryV2TestSuite) TestNonBinding_AdvisoryDoesNotAffectTier() {
	s.queueProposal(1, "")

	reportBefore, _ := s.keeper.GetRiskReport(s.ctx, 1)
	execBefore, _ := s.keeper.GetQueuedExecution(s.ctx, 1)

	// Submit an advisory claiming HIGH risk
	entry := types.AdvisoryEntryV2{
		ProposalID:     1,
		Reporter:       "hostile_reporter",
		URI:            "ipfs://evil",
		ReportHash:     "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType:   types.AdvisoryTypeModelPrediction,
		RiskPrediction: "CRITICAL",
	}
	_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	require.NoError(s.T(), err)

	// Risk report and execution should be UNCHANGED
	reportAfter, _ := s.keeper.GetRiskReport(s.ctx, 1)
	execAfter, _ := s.keeper.GetQueuedExecution(s.ctx, 1)

	s.Equal(reportBefore.Tier, reportAfter.Tier, "advisory must not change tier")
	s.Equal(reportBefore.ComputedDelayBlocks, reportAfter.ComputedDelayBlocks, "advisory must not change delay")
	s.Equal(reportBefore.ComputedThresholdBps, reportAfter.ComputedThresholdBps, "advisory must not change threshold")
	s.Equal(execBefore.GateState, execAfter.GateState, "advisory must not change gate state")
}

func (s *AdvisoryV2TestSuite) TestNonBinding_CorrelationDoesNotAffectGate() {
	s.queueProposal(1, "")
	execBefore, _ := s.keeper.GetQueuedExecution(s.ctx, 1)

	// Submit advisory and trigger correlation
	entry := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "r1",
		URI:          "ipfs://1",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeAuditReport,
	}
	_, _ = s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
	_ = s.keeper.RecordAdvisoryCorrelation(s.ctx, 1, "EXECUTED")

	// Gate state must be unchanged
	execAfter, _ := s.keeper.GetQueuedExecution(s.ctx, 1)
	s.Equal(execBefore.GateState, execAfter.GateState)
}

func (s *AdvisoryV2TestSuite) TestNonBinding_AttackMemoryDoesNotAffectDelay() {
	s.queueProposal(1, "/cosmos.mint.v1beta1.MsgUpdateParams")
	reportBefore, _ := s.keeper.GetRiskReport(s.ctx, 1)

	// Record attack memory
	_ = s.keeper.RecordAttackMemory(s.ctx, 1, types.MemoryTriggerAborted)

	// Report delay must be unchanged
	reportAfter, _ := s.keeper.GetRiskReport(s.ctx, 1)
	s.Equal(reportBefore.ComputedDelayBlocks, reportAfter.ComputedDelayBlocks)
	s.Equal(reportBefore.Tier, reportAfter.Tier)
}

// ============================================================================
// Tests: Memory Trigger Constants
// ============================================================================

func (s *AdvisoryV2TestSuite) TestMemoryTriggerConstants() {
	s.NotEmpty(types.MemoryTriggerAborted)
	s.NotEmpty(types.MemoryTriggerMultipleEscalation)
	s.NotEmpty(types.MemoryTriggerMultipleExtension)
	s.NotEmpty(types.MemoryTriggerCriticalEscalation)
	s.Greater(types.EscalationCountMemoryThreshold, uint64(0))
	s.Greater(types.ExtensionCountMemoryThreshold, uint64(0))
}

// ============================================================================
// Tests: Multiple advisories per proposal
// ============================================================================

func (s *AdvisoryV2TestSuite) TestMultipleAdvisories_PerProposal() {
	s.queueProposal(1, "")

	advisoryTypes := []string{
		types.AdvisoryTypeRiskAnalysis,
		types.AdvisoryTypeThreatReport,
		types.AdvisoryTypeCommunityNote,
	}

	for _, at := range advisoryTypes {
		entry := types.AdvisoryEntryV2{
			ProposalID:   1,
			Reporter:     "reporter",
			URI:          "ipfs://test",
			ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			AdvisoryType: at,
		}
		_, err := s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry)
		require.NoError(s.T(), err)
	}

	entries := s.keeper.GetAdvisoriesForProposal(s.ctx, 1)
	s.Equal(3, len(entries))

	// Each should have unique advisory ID
	ids := make(map[uint64]bool)
	for _, e := range entries {
		s.False(ids[e.AdvisoryID], "advisory IDs should be unique")
		ids[e.AdvisoryID] = true
	}
}

// ============================================================================
// Tests: Cross-proposal advisory isolation
// ============================================================================

func (s *AdvisoryV2TestSuite) TestAdvisories_IsolatedByProposal() {
	s.queueProposal(1, "")
	s.queueProposal(2, "")

	entry1 := types.AdvisoryEntryV2{
		ProposalID:   1,
		Reporter:     "r1",
		URI:          "ipfs://1",
		ReportHash:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeRiskAnalysis,
	}
	_, _ = s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry1)

	entry2 := types.AdvisoryEntryV2{
		ProposalID:   2,
		Reporter:     "r2",
		URI:          "ipfs://2",
		ReportHash:   "1234560123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AdvisoryType: types.AdvisoryTypeThreatReport,
	}
	_, _ = s.keeper.SubmitAdvisoryEntryV2(s.ctx, entry2)

	entries1 := s.keeper.GetAdvisoriesForProposal(s.ctx, 1)
	entries2 := s.keeper.GetAdvisoriesForProposal(s.ctx, 2)

	s.Equal(1, len(entries1))
	s.Equal(1, len(entries2))
	s.Equal(uint64(1), entries1[0].ProposalID)
	s.Equal(uint64(2), entries2[0].ProposalID)
}
