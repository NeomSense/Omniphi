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

type KeeperTestSuite struct {
	suite.Suite

	ctx      sdk.Context
	keeper   keeper.Keeper
	cdc      codec.Codec
	storeKey *storetypes.KVStoreKey

	govKeeper     *MockGovKeeper
	stakingKeeper *MockStakingKeeper
	bankKeeper    *MockBankKeeper
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

func (suite *KeeperTestSuite) SetupTest() {
	// Setup store
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(suite.T(), stateStore.LoadLatestVersion())

	// Setup codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	suite.cdc = codec.NewProtoCodec(interfaceRegistry)

	// Setup context
	suite.ctx = sdk.NewContext(stateStore, tmproto.Header{Height: 1, Time: time.Now()}, false, log.NewNopLogger())

	// Setup mock keepers
	suite.govKeeper = NewMockGovKeeper()
	suite.stakingKeeper = NewMockStakingKeeper()
	suite.bankKeeper = NewMockBankKeeper()

	// Create keeper
	authority := sdk.AccAddress("authority_________").String()
	storeService := runtime.NewKVStoreService(storeKey)
	suite.storeKey = storeKey

	suite.keeper = keeper.NewKeeper(
		suite.cdc,
		storeService,
		authority,
		suite.govKeeper,
		suite.stakingKeeper,
		suite.bankKeeper,
		log.NewNopLogger(),
	)

	// Set default params
	err := suite.keeper.SetParams(suite.ctx, types.DefaultParams())
	require.NoError(suite.T(), err)
}

// ============================================================================
// Tests: Parameters
// ============================================================================

func (suite *KeeperTestSuite) TestParams_DefaultValues() {
	params := suite.keeper.GetParams(suite.ctx)

	blocksPerDay := uint64(17280)
	suite.Equal(blocksPerDay, params.DelayLowBlocks, "LOW delay should be 1 day")
	suite.Equal(blocksPerDay*2, params.DelayMedBlocks, "MED delay should be 2 days")
	suite.Equal(blocksPerDay*7, params.DelayHighBlocks, "HIGH delay should be 7 days")
	suite.Equal(blocksPerDay*14, params.DelayCriticalBlocks, "CRITICAL delay should be 14 days")

	suite.Equal(uint64(5000), params.ThresholdDefaultBps, "Default threshold should be 50%")
	suite.Equal(uint64(6667), params.ThresholdHighBps, "HIGH threshold should be 66.67%")
	suite.Equal(uint64(7500), params.ThresholdCriticalBps, "CRITICAL threshold should be 75%")

	suite.True(params.TreasuryThrottleEnabled, "Treasury throttle should be enabled")
	suite.True(params.EnableStabilityChecks, "Stability checks should be enabled")
	suite.True(params.CriticalRequiresSecondConfirm, "CRITICAL confirmation should be required")
}

func (suite *KeeperTestSuite) TestParams_UpdateAndRetrieve() {
	newParams := types.DefaultParams()
	newParams.DelayLowBlocks = 1000
	newParams.ThresholdDefaultBps = 6000

	err := suite.keeper.SetParams(suite.ctx, newParams)
	require.NoError(suite.T(), err)

	retrieved := suite.keeper.GetParams(suite.ctx)
	suite.Equal(uint64(1000), retrieved.DelayLowBlocks)
	suite.Equal(uint64(6000), retrieved.ThresholdDefaultBps)
}

func (suite *KeeperTestSuite) TestParams_Validation() {
	invalidParams := types.DefaultParams()
	invalidParams.DelayLowBlocks = 0 // Invalid: must be > 0

	err := suite.keeper.SetParams(suite.ctx, invalidParams)
	require.Error(suite.T(), err)
}

// ============================================================================
// Tests: Risk Evaluation
// ============================================================================

func (suite *KeeperTestSuite) TestRiskEvaluation_SoftwareUpgrade() {
	proposal := govtypes.Proposal{
		Id: 1,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade"},
		},
		Status: govtypes.StatusPassed,
	}

	report, err := suite.keeper.EvaluateProposal(suite.ctx, proposal)
	require.NoError(suite.T(), err)

	suite.Equal(types.RISK_TIER_CRITICAL, report.Tier, "Software upgrade should be CRITICAL")
	suite.Equal(uint32(95), report.Score, "Score should be 95")
	suite.Equal(suite.keeper.GetParams(suite.ctx).DelayCriticalBlocks, report.ComputedDelayBlocks)
	suite.Equal(suite.keeper.GetParams(suite.ctx).ThresholdCriticalBps, report.ComputedThresholdBps)
	suite.Contains(report.ReasonCodes, "SOFTWARE_UPGRADE")
}

func (suite *KeeperTestSuite) TestRiskEvaluation_ConsensusCritical() {
	proposal := govtypes.Proposal{
		Id: 2,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.staking.v1beta1.MsgUpdateParams"},
		},
		Status: govtypes.StatusPassed,
	}

	report, err := suite.keeper.EvaluateProposal(suite.ctx, proposal)
	require.NoError(suite.T(), err)

	suite.Equal(types.RISK_TIER_CRITICAL, report.Tier, "Consensus param change should be CRITICAL")
	suite.Equal("rules-v1", report.ModelVersion)
}

func (suite *KeeperTestSuite) TestRiskEvaluation_TreasurySpend() {
	proposal := govtypes.Proposal{
		Id: 3,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.distribution.v1beta1.MsgCommunityPoolSpend"},
		},
		Status: govtypes.StatusPassed,
	}

	report, err := suite.keeper.EvaluateProposal(suite.ctx, proposal)
	require.NoError(suite.T(), err)

	suite.Equal(types.RISK_TIER_MED, report.Tier, "Treasury spend should be at least MED")
	suite.Contains(report.ReasonCodes, "TREASURY_SPEND")
}

func (suite *KeeperTestSuite) TestRiskEvaluation_TextProposal() {
	proposal := govtypes.Proposal{
		Id:       4,
		Messages: []*codectypes.Any{}, // No messages = text-only
		Status:   govtypes.StatusPassed,
	}

	report, err := suite.keeper.EvaluateProposal(suite.ctx, proposal)
	require.NoError(suite.T(), err)

	suite.Equal(types.RISK_TIER_LOW, report.Tier, "Text proposal should be LOW")
	suite.Equal(uint32(5), report.Score)
	suite.Contains(report.ReasonCodes, "TEXT_ONLY")
}

func (suite *KeeperTestSuite) TestRiskEvaluation_FeaturesHash() {
	proposal := govtypes.Proposal{
		Id: 5,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.bank.v1beta1.MsgSend"},
		},
		Status: govtypes.StatusPassed,
	}

	report1, err := suite.keeper.EvaluateProposal(suite.ctx, proposal)
	require.NoError(suite.T(), err)

	report2, err := suite.keeper.EvaluateProposal(suite.ctx, proposal)
	require.NoError(suite.T(), err)

	suite.Equal(report1.FeaturesHash, report2.FeaturesHash, "Hash should be deterministic")
	suite.NotEmpty(report1.FeaturesHash, "Hash should not be empty")
}

// ============================================================================
// Tests: Storage
// ============================================================================

func (suite *KeeperTestSuite) TestRiskReport_SetAndGet() {
	report := types.RiskReport{
		ProposalId:           42,
		Tier:                 types.RISK_TIER_HIGH,
		Score:                70,
		ComputedDelayBlocks:  100000,
		ComputedThresholdBps: 6667,
		ReasonCodes:          `["TEST"]`,
		FeaturesHash:         "abc123",
		ModelVersion:         "rules-v1",
		CreatedAt:            suite.ctx.BlockTime(),
	}

	err := suite.keeper.SetRiskReport(suite.ctx, report)
	require.NoError(suite.T(), err)

	retrieved, found := suite.keeper.GetRiskReport(suite.ctx, 42)
	suite.True(found)
	suite.Equal(report.ProposalId, retrieved.ProposalId)
	suite.Equal(report.Tier, retrieved.Tier)
	suite.Equal(report.Score, retrieved.Score)
}

func (suite *KeeperTestSuite) TestRiskReport_NotFound() {
	_, found := suite.keeper.GetRiskReport(suite.ctx, 999)
	suite.False(found)
}

func (suite *KeeperTestSuite) TestQueuedExecution_SetAndGet() {
	exec := types.QueuedExecution{
		ProposalId:            100,
		QueuedHeight:          1000,
		EarliestExecHeight:    2000,
		GateState:             types.EXECUTION_GATE_VISIBILITY,
		GateEnteredHeight:     1000,
		Tier:                  types.RISK_TIER_MED,
		RequiredThresholdBps:  5000,
		RequiresSecondConfirm: false,
		SecondConfirmReceived: false,
		StatusNote:            "Test",
	}

	err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	retrieved, found := suite.keeper.GetQueuedExecution(suite.ctx, 100)
	suite.True(found)
	suite.Equal(exec.ProposalId, retrieved.ProposalId)
	suite.Equal(exec.GateState, retrieved.GateState)
}

func (suite *KeeperTestSuite) TestLastProcessedProposalID() {
	// Initial value should be 0
	lastID := suite.keeper.GetLastProcessedProposalID(suite.ctx)
	suite.Equal(uint64(0), lastID)

	// Set and retrieve
	err := suite.keeper.SetLastProcessedProposalID(suite.ctx, 42)
	require.NoError(suite.T(), err)

	retrieved := suite.keeper.GetLastProcessedProposalID(suite.ctx)
	suite.Equal(uint64(42), retrieved)
}

// ============================================================================
// Tests: Queue Processing
// ============================================================================

func (suite *KeeperTestSuite) TestOnProposalPassed_CreatesQueueEntry() {
	proposalID := uint64(1)

	// Setup mock proposal
	proposal := govtypes.Proposal{
		Id:       proposalID,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	err := suite.keeper.OnProposalPassed(suite.ctx, proposalID)
	require.NoError(suite.T(), err)

	// Verify risk report created
	report, found := suite.keeper.GetRiskReport(suite.ctx, proposalID)
	suite.True(found, "Risk report should be created")
	suite.Equal(proposalID, report.ProposalId)

	// Verify queued execution created
	exec, found := suite.keeper.GetQueuedExecution(suite.ctx, proposalID)
	suite.True(found, "Queued execution should be created")
	suite.Equal(proposalID, exec.ProposalId)
	suite.Equal(types.EXECUTION_GATE_VISIBILITY, exec.GateState, "Should start in VISIBILITY")
}

func (suite *KeeperTestSuite) TestOnProposalPassed_Idempotent() {
	proposalID := uint64(2)

	proposal := govtypes.Proposal{
		Id:       proposalID,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	// First call
	err := suite.keeper.OnProposalPassed(suite.ctx, proposalID)
	require.NoError(suite.T(), err)

	// Second call should not error (idempotent)
	err = suite.keeper.OnProposalPassed(suite.ctx, proposalID)
	require.NoError(suite.T(), err)
}

func (suite *KeeperTestSuite) TestOnProposalPassed_CriticalRequiresConfirm() {
	proposalID := uint64(3)

	// Software upgrade = CRITICAL
	proposal := govtypes.Proposal{
		Id: proposalID,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmossdk.io/x/upgrade/v1.MsgSoftwareUpgrade"},
		},
		Status: govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	err := suite.keeper.OnProposalPassed(suite.ctx, proposalID)
	require.NoError(suite.T(), err)

	exec, found := suite.keeper.GetQueuedExecution(suite.ctx, proposalID)
	suite.True(found)
	suite.True(exec.RequiresSecondConfirm, "CRITICAL proposal should require confirmation")
	suite.False(exec.SecondConfirmReceived, "Confirmation should not be received yet")
}

// ============================================================================
// Tests: Helper Methods
// ============================================================================

func (suite *KeeperTestSuite) TestExecutionGateState_IsTerminal() {
	suite.True(types.EXECUTION_GATE_EXECUTED.IsTerminal())
	suite.True(types.EXECUTION_GATE_ABORTED.IsTerminal())
	suite.False(types.EXECUTION_GATE_READY.IsTerminal())
	suite.False(types.EXECUTION_GATE_VISIBILITY.IsTerminal())
}

func (suite *KeeperTestSuite) TestQueuedExecution_IsReady() {
	exec := types.QueuedExecution{
		GateState: types.EXECUTION_GATE_READY,
	}
	suite.True(exec.IsReady())

	exec.GateState = types.EXECUTION_GATE_VISIBILITY
	suite.False(exec.IsReady())
}

func (suite *KeeperTestSuite) TestQueuedExecution_NeedsConfirmation() {
	exec := types.QueuedExecution{
		RequiresSecondConfirm: true,
		SecondConfirmReceived: false,
	}
	suite.True(exec.NeedsConfirmation())

	exec.SecondConfirmReceived = true
	suite.False(exec.NeedsConfirmation())

	exec.RequiresSecondConfirm = false
	exec.SecondConfirmReceived = false
	suite.False(exec.NeedsConfirmation())
}

func (suite *KeeperTestSuite) TestMaxConstraint() {
	suite.Equal(uint64(100), types.MaxConstraint(100, 50))
	suite.Equal(uint64(100), types.MaxConstraint(50, 100))
	suite.Equal(uint64(100), types.MaxConstraint(100, 100))
}

func (suite *KeeperTestSuite) TestMaxConstraintTier() {
	suite.Equal(types.RISK_TIER_CRITICAL, types.MaxConstraintTier(types.RISK_TIER_CRITICAL, types.RISK_TIER_LOW))
	suite.Equal(types.RISK_TIER_HIGH, types.MaxConstraintTier(types.RISK_TIER_MED, types.RISK_TIER_HIGH))
	suite.Equal(types.RISK_TIER_MED, types.MaxConstraintTier(types.RISK_TIER_MED, types.RISK_TIER_MED))
}

func (suite *KeeperTestSuite) TestParseVoteCount() {
	// Test normal numbers
	count, err := keeper.ParseVoteCount("12345")
	require.NoError(suite.T(), err)
	suite.Equal(uint64(12345), count)

	// Test zero
	count, err = keeper.ParseVoteCount("0")
	require.NoError(suite.T(), err)
	suite.Equal(uint64(0), count)

	// Test empty string
	count, err = keeper.ParseVoteCount("")
	require.NoError(suite.T(), err)
	suite.Equal(uint64(0), count)

	// Test large number (math.Int format)
	count, err = keeper.ParseVoteCount("999999999999999")
	require.NoError(suite.T(), err)
	suite.Greater(count, uint64(0))
}
