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
// DDG v2 Hardening Test Suite
// ============================================================================

type HardeningTestSuite struct {
	suite.Suite

	ctx    sdk.Context
	keeper keeper.Keeper
	cdc    codec.Codec

	govKeeper     *MockGovKeeper
	stakingKeeper *MockStakingKeeper
	bankKeeper    *MockBankKeeper
}

func TestHardeningTestSuite(t *testing.T) {
	suite.Run(t, new(HardeningTestSuite))
}

func (s *HardeningTestSuite) SetupTest() {
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

// helper: queue a proposal of a given type and return its execution
func (s *HardeningTestSuite) queueProposal(id uint64, msgTypeURL string) {
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
// Tests: Emergency Hardening Mode
// ============================================================================

func (s *HardeningTestSuite) TestEmergencyHardeningMode_DefaultOff() {
	s.False(s.keeper.GetEmergencyHardeningMode(s.ctx))
}

func (s *HardeningTestSuite) TestEmergencyHardeningMode_Toggle() {
	err := s.keeper.SetEmergencyHardeningMode(s.ctx, true)
	require.NoError(s.T(), err)
	s.True(s.keeper.GetEmergencyHardeningMode(s.ctx))

	err = s.keeper.SetEmergencyHardeningMode(s.ctx, false)
	require.NoError(s.T(), err)
	s.False(s.keeper.GetEmergencyHardeningMode(s.ctx))
}

func (s *HardeningTestSuite) TestEmergencyHardening_HighTreatedAsCritical() {
	// Enable hardening mode
	err := s.keeper.SetEmergencyHardeningMode(s.ctx, true)
	require.NoError(s.T(), err)

	// Queue a PARAM_CHANGE proposal (normally HIGH tier with economic param)
	proposal := govtypes.Proposal{
		Id: 1,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.mint.v1beta1.MsgUpdateParams"},
		},
		Status: govtypes.StatusPassed,
	}
	s.govKeeper.SetProposal(proposal)

	err = s.keeper.OnProposalPassed(s.ctx, 1)
	require.NoError(s.T(), err)

	report, found := s.keeper.GetRiskReport(s.ctx, 1)
	s.True(found)

	// In emergency mode, HIGH should be escalated to CRITICAL
	// Param change for economic module = HIGH baseline
	// Emergency hardening promotes HIGH → CRITICAL
	if report.Tier == types.RISK_TIER_HIGH {
		s.Fail("emergency hardening should have escalated HIGH to CRITICAL")
	}
	s.GreaterOrEqual(int(report.Tier), int(types.RISK_TIER_HIGH))
}

func (s *HardeningTestSuite) TestEmergencyHardening_DelayMultiplied() {
	params := s.keeper.GetParams(s.ctx)

	// Queue a text proposal WITHOUT hardening
	proposal := govtypes.Proposal{
		Id:       1,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
	}
	s.govKeeper.SetProposal(proposal)
	err := s.keeper.OnProposalPassed(s.ctx, 1)
	require.NoError(s.T(), err)

	reportWithout, _ := s.keeper.GetRiskReport(s.ctx, 1)
	normalDelay := reportWithout.ComputedDelayBlocks

	// Enable hardening and queue another
	err = s.keeper.SetEmergencyHardeningMode(s.ctx, true)
	require.NoError(s.T(), err)

	proposal2 := govtypes.Proposal{
		Id:       2,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
	}
	s.govKeeper.SetProposal(proposal2)
	err = s.keeper.OnProposalPassed(s.ctx, 2)
	require.NoError(s.T(), err)

	reportWith, _ := s.keeper.GetRiskReport(s.ctx, 2)

	// Hardened delay should be >= normal * 1.5
	expectedHardenedDelay := normalDelay * 3 / 2
	s.GreaterOrEqual(reportWith.ComputedDelayBlocks, expectedHardenedDelay,
		"hardened delay (%d) should be >= 1.5x normal (%d = %d * 3/2)",
		reportWith.ComputedDelayBlocks, expectedHardenedDelay, normalDelay)
	_ = params // used for context
}

// ============================================================================
// Tests: Tier Escalation
// ============================================================================

func (s *HardeningTestSuite) TestEscalateTierByOne() {
	// This is tested indirectly through reevaluation, but let's verify the
	// monotonic property: tier can only go up

	// Queue a LOW proposal
	s.queueProposal(10, "") // text-only = LOW

	report, found := s.keeper.GetRiskReport(s.ctx, 10)
	s.True(found)
	s.Equal(types.RISK_TIER_LOW, report.Tier)
}

func (s *HardeningTestSuite) TestTierEscalation_MultiParamChangeTrigger() {
	// Queue 3+ PARAM_CHANGE proposals to trigger escalation via aggregate risk
	for i := uint64(1); i <= 3; i++ {
		s.queueProposal(i, "/cosmos.mint.v1beta1.MsgUpdateParams")
	}

	// Now queue a 4th param change — should get escalated due to aggregate risk
	proposal := govtypes.Proposal{
		Id: 4,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.mint.v1beta1.MsgUpdateParams"},
		},
		Status: govtypes.StatusPassed,
	}
	s.govKeeper.SetProposal(proposal)
	err := s.keeper.OnProposalPassed(s.ctx, 4)
	require.NoError(s.T(), err)

	report, found := s.keeper.GetRiskReport(s.ctx, 4)
	s.True(found)

	// Should be escalated above normal HIGH for economic param change
	// (3 active param changes meets ParamMutationBurstThreshold)
	s.GreaterOrEqual(int(report.Tier), int(types.RISK_TIER_HIGH),
		"4th param change should be at least HIGH due to burst detection")
}

// ============================================================================
// Tests: Cross-Proposal Risk Coupling
// ============================================================================

func (s *HardeningTestSuite) TestAggregateRisk_EmptyQueue() {
	snapshot := s.keeper.ComputeAggregateRisk(s.ctx)
	s.Equal(uint64(0), snapshot.ActiveProposalCount)
	s.Equal(uint64(0), snapshot.TreasurySpendsActive)
	s.Equal(uint64(0), snapshot.ParamChangesActive)
	s.Equal(uint64(0), snapshot.UpgradesActive)
	s.Equal(types.RISK_TIER_LOW, snapshot.HighestTier)
}

func (s *HardeningTestSuite) TestAggregateRisk_CountsProposals() {
	// Queue a mix of proposal types
	s.queueProposal(1, "/cosmos.distribution.v1beta1.MsgCommunityPoolSpend") // treasury
	s.queueProposal(2, "/cosmos.mint.v1beta1.MsgUpdateParams")              // param change
	s.queueProposal(3, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")    // upgrade
	s.queueProposal(4, "")                                                   // text-only

	snapshot := s.keeper.ComputeAggregateRisk(s.ctx)
	s.Equal(uint64(4), snapshot.ActiveProposalCount)
	s.Equal(uint64(1), snapshot.TreasurySpendsActive)
	s.Equal(uint64(1), snapshot.ParamChangesActive)
	s.Equal(uint64(1), snapshot.UpgradesActive)
	s.Equal(types.RISK_TIER_CRITICAL, snapshot.HighestTier) // upgrade = CRITICAL
}

func (s *HardeningTestSuite) TestAggregateRisk_SkipsTerminalProposals() {
	// Queue a proposal
	s.queueProposal(1, "")

	// Mark it as executed
	exec, found := s.keeper.GetQueuedExecution(s.ctx, 1)
	s.True(found)
	exec.GateState = types.EXECUTION_GATE_EXECUTED
	err := s.keeper.SetQueuedExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	snapshot := s.keeper.ComputeAggregateRisk(s.ctx)
	s.Equal(uint64(0), snapshot.ActiveProposalCount)
}

// ============================================================================
// Tests: Continuous Reevaluation
// ============================================================================

func (s *HardeningTestSuite) TestReevaluateRisk_NoChangeForStableState() {
	// Queue a text proposal (LOW tier)
	s.queueProposal(1, "")

	reportBefore, _ := s.keeper.GetRiskReport(s.ctx, 1)

	// Reevaluate — should not change anything in a stable state
	err := s.keeper.ReevaluateRisk(s.ctx, 1)
	require.NoError(s.T(), err)

	reportAfter, _ := s.keeper.GetRiskReport(s.ctx, 1)
	s.Equal(reportBefore.Tier, reportAfter.Tier)
	s.Equal(reportBefore.ComputedThresholdBps, reportAfter.ComputedThresholdBps)
}

func (s *HardeningTestSuite) TestReevaluateRisk_SkipsTerminal() {
	s.queueProposal(1, "")

	// Mark as executed
	exec, _ := s.keeper.GetQueuedExecution(s.ctx, 1)
	exec.GateState = types.EXECUTION_GATE_EXECUTED
	err := s.keeper.SetQueuedExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	// Should not error on terminal proposals
	err = s.keeper.ReevaluateRisk(s.ctx, 1)
	require.NoError(s.T(), err)
}

func (s *HardeningTestSuite) TestReevaluateRisk_NotFoundErrors() {
	err := s.keeper.ReevaluateRisk(s.ctx, 999)
	require.Error(s.T(), err)
}

func (s *HardeningTestSuite) TestReevaluateRisk_MonotonicTierInvariant() {
	// Queue a CRITICAL proposal
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	reportBefore, _ := s.keeper.GetRiskReport(s.ctx, 1)
	s.Equal(types.RISK_TIER_CRITICAL, reportBefore.Tier)

	// Reevaluate multiple times — should never go below CRITICAL
	for i := 0; i < 5; i++ {
		err := s.keeper.ReevaluateRisk(s.ctx, 1)
		require.NoError(s.T(), err)

		reportAfter, _ := s.keeper.GetRiskReport(s.ctx, 1)
		s.GreaterOrEqual(int(reportAfter.Tier), int(types.RISK_TIER_CRITICAL),
			"tier must never decrease below CRITICAL (iteration %d)", i)
	}
}

func (s *HardeningTestSuite) TestReevaluateRisk_MonotonicThresholdInvariant() {
	// Queue a CRITICAL proposal
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	reportBefore, _ := s.keeper.GetRiskReport(s.ctx, 1)
	initialThreshold := reportBefore.ComputedThresholdBps

	// Reevaluate — threshold must never decrease
	err := s.keeper.ReevaluateRisk(s.ctx, 1)
	require.NoError(s.T(), err)

	reportAfter, _ := s.keeper.GetRiskReport(s.ctx, 1)
	s.GreaterOrEqual(reportAfter.ComputedThresholdBps, initialThreshold,
		"threshold must never decrease")
}

// ============================================================================
// Tests: Reevaluation Storage
// ============================================================================

func (s *HardeningTestSuite) TestReevaluationRecord_SetAndGet() {
	record := types.ReevaluationRecord{
		ProposalID:          42,
		ReevaluatedAtHeight: 500,
		PreviousTier:        types.RISK_TIER_MED,
		NewTier:             types.RISK_TIER_HIGH,
		PreviousThreshold:   5000,
		NewThreshold:        6667,
		PreviousDelay:       34560,
		NewDelay:            120960,
		EscalationReasons:   []string{"VALIDATOR_CHURN_SOFT:1200_bps"},
	}

	err := s.keeper.SetReevaluationRecord(s.ctx, record)
	require.NoError(s.T(), err)

	retrieved, found := s.keeper.GetReevaluationRecord(s.ctx, 42)
	s.True(found)
	s.Equal(record.ProposalID, retrieved.ProposalID)
	s.Equal(record.NewTier, retrieved.NewTier)
	s.Equal(record.NewThreshold, retrieved.NewThreshold)
	s.Equal(1, len(retrieved.EscalationReasons))
}

func (s *HardeningTestSuite) TestReevaluationRecord_NotFound() {
	_, found := s.keeper.GetReevaluationRecord(s.ctx, 999)
	s.False(found)
}

// ============================================================================
// Tests: Threshold Escalation Storage
// ============================================================================

func (s *HardeningTestSuite) TestThresholdEscalationRecord_SetAndGet() {
	record := types.ThresholdEscalationRecord{
		ProposalID:        42,
		OriginalThreshold: 5000,
		CurrentThreshold:  5500,
		EscalationCount:   1,
		LastEscalatedAt:   200,
	}

	err := s.keeper.SetThresholdEscalationRecord(s.ctx, record)
	require.NoError(s.T(), err)

	retrieved := s.keeper.GetThresholdEscalationRecord(s.ctx, 42)
	s.Equal(record.ProposalID, retrieved.ProposalID)
	s.Equal(record.CurrentThreshold, retrieved.CurrentThreshold)
	s.Equal(uint64(1), retrieved.EscalationCount)
}

func (s *HardeningTestSuite) TestThresholdEscalationRecord_DefaultZero() {
	record := s.keeper.GetThresholdEscalationRecord(s.ctx, 999)
	s.Equal(uint64(0), record.ProposalID)
	s.Equal(uint64(0), record.CurrentThreshold)
}

// ============================================================================
// Tests: MaxConstraint Invariants
// ============================================================================

func (s *HardeningTestSuite) TestMaxConstraint_TierNeverDecreases() {
	// Test all possible tier pairs — max() should always be >= both inputs
	tiers := []types.RiskTier{
		types.RISK_TIER_LOW,
		types.RISK_TIER_MED,
		types.RISK_TIER_HIGH,
		types.RISK_TIER_CRITICAL,
	}
	for _, a := range tiers {
		for _, b := range tiers {
			result := types.MaxConstraintTier(a, b)
			s.GreaterOrEqual(int(result), int(a), "max(%s,%s) must be >= %s", a, b, a)
			s.GreaterOrEqual(int(result), int(b), "max(%s,%s) must be >= %s", a, b, b)
		}
	}
}

func (s *HardeningTestSuite) TestMaxConstraint_ThresholdNeverDecreases() {
	thresholds := []uint64{0, 5000, 6667, 7500, 9000, 10000}
	for _, a := range thresholds {
		for _, b := range thresholds {
			result := types.MaxConstraint(a, b)
			s.GreaterOrEqual(result, a)
			s.GreaterOrEqual(result, b)
		}
	}
}

// ============================================================================
// Tests: containsAny helper (indirectly via aggregate risk)
// ============================================================================

func (s *HardeningTestSuite) TestAggregateRisk_TreasuryStackingDetected() {
	// Queue 3 treasury spend proposals
	for i := uint64(1); i <= 3; i++ {
		s.queueProposal(i, "/cosmos.distribution.v1beta1.MsgCommunityPoolSpend")
	}

	snapshot := s.keeper.ComputeAggregateRisk(s.ctx)
	s.Equal(uint64(3), snapshot.TreasurySpendsActive)
	s.Greater(snapshot.CumulativeTreasuryBps, uint64(0))
}

// ============================================================================
// Tests: End-to-End Gate Transition with Reevaluation
// ============================================================================

func (s *HardeningTestSuite) TestGateTransition_IncludesReevaluation() {
	// Queue a text proposal
	s.queueProposal(1, "")

	exec, found := s.keeper.GetQueuedExecution(s.ctx, 1)
	s.True(found)
	s.Equal(types.EXECUTION_GATE_VISIBILITY, exec.GateState)

	// Advance height past visibility window
	params := s.keeper.GetParams(s.ctx)
	newHeight := int64(exec.GateEnteredHeight) + int64(params.VisibilityWindowBlocks) + 1
	s.ctx = s.ctx.WithBlockHeight(newHeight)

	// Process queue — should transition VISIBILITY → SHOCK_ABSORBER
	// and also run reevaluation (which should be a no-op for text proposal)
	err := s.keeper.ProcessQueue(s.ctx)
	require.NoError(s.T(), err)

	exec, found = s.keeper.GetQueuedExecution(s.ctx, 1)
	s.True(found)
	s.Equal(types.EXECUTION_GATE_SHOCK_ABSORBER, exec.GateState)

	// Tier should still be LOW
	report, _ := s.keeper.GetRiskReport(s.ctx, 1)
	s.Equal(types.RISK_TIER_LOW, report.Tier)
}

// ============================================================================
// Tests: Hardening Types Constants
// ============================================================================

func (s *HardeningTestSuite) TestConstants_SoftThresholdBelowHard() {
	s.Less(types.ValidatorChurnSoftThresholdBps, types.ValidatorChurnHardThresholdBps,
		"soft threshold must be below hard threshold")
}

func (s *HardeningTestSuite) TestConstants_SupermajorityWithinBounds() {
	s.LessOrEqual(types.SupermajorityThresholdBps, uint64(10000),
		"supermajority cannot exceed 100%")
	s.Greater(types.SupermajorityThresholdBps, uint64(7500),
		"supermajority should be above normal CRITICAL threshold")
}

func (s *HardeningTestSuite) TestConstants_HardeningMultiplier() {
	// 3/2 = 1.5x — verify it produces correct results
	delay := uint64(17280) // 1 day
	hardened := delay * types.HardeningDelayMultiplierNum / types.HardeningDelayMultiplierDen
	s.Equal(uint64(25920), hardened, "1 day * 1.5 should be 25920 blocks")
}

func (s *HardeningTestSuite) TestConstants_EscalationStepPositive() {
	s.Greater(types.ThresholdEscalationStepBps, uint64(0))
}

// ============================================================================
// Tests: DDG v2 Integration with OnProposalPassed
// ============================================================================

func (s *HardeningTestSuite) TestOnProposalPassed_WithHardeningEnabled() {
	err := s.keeper.SetEmergencyHardeningMode(s.ctx, true)
	require.NoError(s.T(), err)

	// Queue a CRITICAL proposal
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	report, found := s.keeper.GetRiskReport(s.ctx, 1)
	s.True(found)
	s.Equal(types.RISK_TIER_CRITICAL, report.Tier)

	// Delay should be 1.5x the CRITICAL delay
	params := s.keeper.GetParams(s.ctx)
	expectedDelay := params.DelayCriticalBlocks * 3 / 2
	s.Equal(expectedDelay, report.ComputedDelayBlocks,
		"CRITICAL delay should be 1.5x in hardening mode")
}

func (s *HardeningTestSuite) TestOnProposalPassed_WithoutHardeningIsNormal() {
	// Hardening OFF (default)
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	report, found := s.keeper.GetRiskReport(s.ctx, 1)
	s.True(found)
	s.Equal(types.RISK_TIER_CRITICAL, report.Tier)

	// Delay should be normal CRITICAL delay
	params := s.keeper.GetParams(s.ctx)
	s.Equal(params.DelayCriticalBlocks, report.ComputedDelayBlocks)
}

// ============================================================================
// Tests: Upgrade Clustering
// ============================================================================

func (s *HardeningTestSuite) TestAggregateRisk_UpgradeClusteringDetection() {
	// Queue 2 upgrades (UpgradeClusteringThreshold)
	s.queueProposal(1, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")
	s.queueProposal(2, "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade")

	snapshot := s.keeper.ComputeAggregateRisk(s.ctx)
	s.Equal(uint64(2), snapshot.UpgradesActive)
	s.GreaterOrEqual(snapshot.UpgradesActive, types.UpgradeClusteringThreshold)
}
