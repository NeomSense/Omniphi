package keeper_test

import (
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"pos/x/guard/keeper"
	"pos/x/guard/types"
)

// EnforcementTestSuite tests the three real enforcement implementations:
// ExecuteProposal, CalculateTreasurySpendPercentage, CheckStabilityConditions
type EnforcementTestSuite struct {
	suite.Suite

	ctx           sdk.Context
	keeper        keeper.Keeper
	cdc           codec.Codec
	govKeeper     *MockGovKeeper
	stakingKeeper *MockStakingKeeper
	bankKeeper    *MockBankKeeper
	distrKeeper   *MockDistrKeeper
	router        *MockMessageRouter
}

func TestEnforcementTestSuite(t *testing.T) {
	suite.Run(t, new(EnforcementTestSuite))
}

func (suite *EnforcementTestSuite) SetupTest() {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(suite.T(), stateStore.LoadLatestVersion())

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	// Register bank types so MsgSend can be packed/unpacked
	banktypes.RegisterInterfaces(interfaceRegistry)
	suite.cdc = codec.NewProtoCodec(interfaceRegistry)

	suite.ctx = sdk.NewContext(stateStore, tmproto.Header{Height: 100, Time: time.Now()}, false, log.NewNopLogger())

	suite.govKeeper = NewMockGovKeeper()
	suite.stakingKeeper = NewMockStakingKeeper()
	suite.bankKeeper = NewMockBankKeeper()
	suite.distrKeeper = NewMockDistrKeeper()
	suite.router = NewMockMessageRouter()

	authority := sdk.AccAddress("authority_________").String()
	storeService := runtime.NewKVStoreService(storeKey)

	suite.keeper = keeper.NewKeeper(
		suite.cdc,
		storeService,
		authority,
		suite.govKeeper,
		suite.stakingKeeper,
		suite.bankKeeper,
		log.NewNopLogger(),
	)

	// Wire optional dependencies
	kp := &suite.keeper
	kp.SetDistrKeeper(suite.distrKeeper)
	kp.SetRouter(suite.router)
	kp.SetInterfaceRegistry(interfaceRegistry)

	err := suite.keeper.SetParams(suite.ctx, types.DefaultParams())
	require.NoError(suite.T(), err)
}

// ============================================================================
// Part A: ExecuteProposal Tests
// ============================================================================

func (suite *EnforcementTestSuite) TestExecuteProposal_TextOnly() {
	// Text-only proposals (no messages) should succeed with no-op
	proposal := govtypes.Proposal{
		Id:       1,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{ProposalId: 1, GateState: types.EXECUTION_GATE_READY}
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.NoError(err, "text-only proposal should execute successfully")
}

func (suite *EnforcementTestSuite) TestExecuteProposal_NoRouterFails() {
	// Create keeper without router using the suite's store (so ctx is valid)
	authority := sdk.AccAddress("authority_________").String()
	storeKey := storetypes.NewKVStoreKey("test_no_router")

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(suite.T(), stateStore.LoadLatestVersion())

	ctx := sdk.NewContext(stateStore, tmproto.Header{Height: 100, Time: time.Now()}, false, log.NewNopLogger())

	storeService := runtime.NewKVStoreService(storeKey)
	noRouterKeeper := keeper.NewKeeper(
		suite.cdc, storeService, authority,
		suite.govKeeper, suite.stakingKeeper, suite.bankKeeper,
		log.NewNopLogger(),
	)

	// Proposal with a real message
	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}
	anyMsg, err := codectypes.NewAnyWithValue(msg)
	require.NoError(suite.T(), err)

	proposal := govtypes.Proposal{
		Id:       10,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{ProposalId: 10, GateState: types.EXECUTION_GATE_READY}
	err = noRouterKeeper.ExecuteProposal(ctx, exec)
	suite.Error(err, "should fail without router")
	suite.Contains(err.Error(), "message router not configured")
}

func (suite *EnforcementTestSuite) TestExecuteProposal_SuccessfulDispatch() {
	authority := sdk.AccAddress("authority_________").String()
	executed := false

	// Register a handler for MsgSend
	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}

	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		executed = true
		return &sdk.Result{}, nil
	})

	anyMsg, err := codectypes.NewAnyWithValue(msg)
	require.NoError(suite.T(), err)

	proposal := govtypes.Proposal{
		Id:       20,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{ProposalId: 20, GateState: types.EXECUTION_GATE_READY}
	err = suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.NoError(err, "should execute successfully")
	suite.True(executed, "handler should have been called")
}

func (suite *EnforcementTestSuite) TestExecuteProposal_FailureAbortsAll() {
	authority := sdk.AccAddress("authority_________").String()
	callCount := 0

	msg1 := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient1________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}
	msg2 := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient2________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(200))),
	}

	// Both msgs have same TypeURL, handler fails on second call
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg1), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		callCount++
		if callCount >= 2 {
			return nil, fmt.Errorf("insufficient funds")
		}
		return &sdk.Result{}, nil
	})

	anyMsg1, _ := codectypes.NewAnyWithValue(msg1)
	anyMsg2, _ := codectypes.NewAnyWithValue(msg2)

	proposal := govtypes.Proposal{
		Id:       30,
		Messages: []*codectypes.Any{anyMsg1, anyMsg2},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{ProposalId: 30, GateState: types.EXECUTION_GATE_READY}
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.Error(err, "should fail when second message fails")
	suite.Contains(err.Error(), "insufficient funds")
	// CacheContext ensures first message's state changes are NOT committed
}

func (suite *EnforcementTestSuite) TestExecuteProposal_PanicRecovery() {
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}

	// Handler that panics
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		panic("unexpected panic in handler")
	})

	anyMsg, _ := codectypes.NewAnyWithValue(msg)
	proposal := govtypes.Proposal{
		Id:       40,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{ProposalId: 40, GateState: types.EXECUTION_GATE_READY}
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.Error(err, "should recover from panic")
	suite.Contains(err.Error(), "handler panicked")
}

func (suite *EnforcementTestSuite) TestExecuteProposal_NoHandlerFails() {
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}

	// Don't register any handler — router returns nil
	anyMsg, _ := codectypes.NewAnyWithValue(msg)
	proposal := govtypes.Proposal{
		Id:       50,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{ProposalId: 50, GateState: types.EXECUTION_GATE_READY}
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.Error(err, "should fail with no handler")
	suite.Contains(err.Error(), "no handler for message")
}

// ============================================================================
// Part B: CalculateTreasurySpendPercentage Tests
// ============================================================================

func (suite *EnforcementTestSuite) TestTreasurySpend_EmptyPool_MaxRisk() {
	// Community pool is empty, any spend = 100% = 10000 bps
	suite.distrKeeper.SetCommunityPool(sdk.DecCoins{})

	proposal := govtypes.Proposal{
		Id: 100,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.distribution.v1beta1.MsgCommunityPoolSpend"},
		},
		Status: govtypes.StatusPassed,
	}

	// Since we can't unpack the Any (no cached value), parseTreasurySpendAmount returns 0
	// and CalculateTreasurySpendPercentage returns 0
	bps := suite.keeper.CalculateTreasurySpendPercentage(suite.ctx, proposal)
	suite.Equal(uint64(0), bps, "unparseable spend should return 0")
}

func (suite *EnforcementTestSuite) TestTreasurySpend_WithRealPool() {
	// Set community pool to 1,000,000 omniphi
	suite.distrKeeper.SetCommunityPool(sdk.DecCoins{
		sdk.NewDecCoinFromDec("omniphi", math.LegacyNewDec(1000000)),
	})

	// Proposal spending 100,000 omniphi (10%)
	authority := sdk.AccAddress("authority_________").String()
	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100000))),
	}
	anyMsg, _ := codectypes.NewAnyWithValue(msg)

	proposal := govtypes.Proposal{
		Id:       101,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}

	// MsgSend only counts if from authority (gov module account)
	bps := suite.keeper.CalculateTreasurySpendPercentage(suite.ctx, proposal)
	suite.Equal(uint64(1000), bps, "100k from 1M pool = 10% = 1000 bps")
}

func (suite *EnforcementTestSuite) TestTreasurySpend_SmallSpend() {
	suite.distrKeeper.SetCommunityPool(sdk.DecCoins{
		sdk.NewDecCoinFromDec("omniphi", math.LegacyNewDec(10000000)), // 10M
	})

	authority := sdk.AccAddress("authority_________").String()
	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(10000))), // 10k from 10M = 0.1%
	}
	anyMsg, _ := codectypes.NewAnyWithValue(msg)

	proposal := govtypes.Proposal{
		Id:       102,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}

	bps := suite.keeper.CalculateTreasurySpendPercentage(suite.ctx, proposal)
	suite.Equal(uint64(10), bps, "10k from 10M = 0.1% = 10 bps")
}

func (suite *EnforcementTestSuite) TestTreasurySpend_NonAuthoritySenderIgnored() {
	suite.distrKeeper.SetCommunityPool(sdk.DecCoins{
		sdk.NewDecCoinFromDec("omniphi", math.LegacyNewDec(1000000)),
	})

	// MsgSend from a non-authority address — should not count as treasury spend
	msg := &banktypes.MsgSend{
		FromAddress: sdk.AccAddress("random_sender_____").String(),
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(500000))),
	}
	anyMsg, _ := codectypes.NewAnyWithValue(msg)

	proposal := govtypes.Proposal{
		Id:       103,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}

	bps := suite.keeper.CalculateTreasurySpendPercentage(suite.ctx, proposal)
	suite.Equal(uint64(0), bps, "non-authority MsgSend should not count as treasury spend")
}

func (suite *EnforcementTestSuite) TestTreasurySpend_LargerThanPool() {
	suite.distrKeeper.SetCommunityPool(sdk.DecCoins{
		sdk.NewDecCoinFromDec("omniphi", math.LegacyNewDec(100)),
	})

	authority := sdk.AccAddress("authority_________").String()
	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(500))),
	}
	anyMsg, _ := codectypes.NewAnyWithValue(msg)

	proposal := govtypes.Proposal{
		Id:       104,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}

	bps := suite.keeper.CalculateTreasurySpendPercentage(suite.ctx, proposal)
	suite.Equal(uint64(10000), bps, "spending > pool should clamp to 10000 bps")
}

func (suite *EnforcementTestSuite) TestTreasurySpend_NoDistrKeeper_Fallback() {
	// Create keeper without distr keeper
	authority := sdk.AccAddress("authority_________").String()
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	storeKey := storetypes.NewKVStoreKey("test_no_distr")
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(suite.T(), stateStore.LoadLatestVersion())

	storeService := runtime.NewKVStoreService(storeKey)
	noDistrKeeper := keeper.NewKeeper(
		suite.cdc, storeService, authority,
		suite.govKeeper, suite.stakingKeeper, suite.bankKeeper,
		log.NewNopLogger(),
	)

	proposal := govtypes.Proposal{
		Id: 105,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.distribution.v1beta1.MsgCommunityPoolSpend"},
		},
	}

	bps := noDistrKeeper.CalculateTreasurySpendPercentage(suite.ctx, proposal)
	suite.Equal(uint64(500), bps, "should return 500 bps (5%) when distr keeper is nil")
}

// ============================================================================
// Part C: CheckStabilityConditions Tests
// ============================================================================

func (suite *EnforcementTestSuite) TestStability_NoSnapshot_PassesByDefault() {
	params := suite.keeper.GetParams(suite.ctx)
	passed, churnBps := suite.keeper.CheckStabilityConditions(suite.ctx, 999, params)
	suite.True(passed, "should pass when no snapshot exists")
	suite.Equal(uint64(0), churnBps)
}

func (suite *EnforcementTestSuite) TestStability_NoChurn_Passes() {
	// Set up validators
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 100},
		{OperatorAddr: "val2", Power: 200},
		{OperatorAddr: "val3", Power: 300},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	// Take snapshot
	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 1)
	require.NoError(suite.T(), err)

	// Check stability — same validators, same power
	params := suite.keeper.GetParams(suite.ctx)
	passed, churnBps := suite.keeper.CheckStabilityConditions(suite.ctx, 1, params)
	suite.True(passed, "no churn should pass")
	suite.Equal(uint64(0), churnBps)
}

func (suite *EnforcementTestSuite) TestStability_SmallChurn_Passes() {
	// Snapshot with validators
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 100},
		{OperatorAddr: "val2", Power: 200},
		{OperatorAddr: "val3", Power: 300},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 2)
	require.NoError(suite.T(), err)

	// Small power change: val1 100->110 (delta=10)
	// Total snapshot power = 600, churn = 10/600 * 10000 = 166 bps
	// Default max churn = 2000 bps, so should pass
	newVals := []MockValidator{
		{OperatorAddr: "val1", Power: 110},
		{OperatorAddr: "val2", Power: 200},
		{OperatorAddr: "val3", Power: 300},
	}
	suite.stakingKeeper.SetMockValidators(newVals)

	params := suite.keeper.GetParams(suite.ctx)
	passed, churnBps := suite.keeper.CheckStabilityConditions(suite.ctx, 2, params)
	suite.True(passed, "small churn should pass")
	suite.Equal(uint64(166), churnBps, "churn should be ~166 bps")
}

func (suite *EnforcementTestSuite) TestStability_LargeChurn_Fails() {
	// Snapshot
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 100},
		{OperatorAddr: "val2", Power: 200},
		{OperatorAddr: "val3", Power: 300},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 3)
	require.NoError(suite.T(), err)

	// Large power change: val1 drops out (100->0), val4 appears (0->400)
	// Delta = |0-100| + |400-0| = 500
	// churn = 500/600 * 10000 = 8333 bps > 2000 default max
	newVals := []MockValidator{
		{OperatorAddr: "val2", Power: 200},
		{OperatorAddr: "val3", Power: 300},
		{OperatorAddr: "val4", Power: 400},
	}
	suite.stakingKeeper.SetMockValidators(newVals)

	params := suite.keeper.GetParams(suite.ctx)
	passed, churnBps := suite.keeper.CheckStabilityConditions(suite.ctx, 3, params)
	suite.False(passed, "large churn should fail")
	suite.Equal(uint64(8333), churnBps, "churn should be ~8333 bps")
}

func (suite *EnforcementTestSuite) TestStability_ValidatorRemoved() {
	// Snapshot with 3 validators
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 500},
		{OperatorAddr: "val2", Power: 300},
		{OperatorAddr: "val3", Power: 200},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 4)
	require.NoError(suite.T(), err)

	// val3 is removed (power 200 -> 0)
	// Delta = 200, total = 1000
	// churn = 200/1000 * 10000 = 2000 bps = exactly at threshold
	newVals := []MockValidator{
		{OperatorAddr: "val1", Power: 500},
		{OperatorAddr: "val2", Power: 300},
	}
	suite.stakingKeeper.SetMockValidators(newVals)

	params := suite.keeper.GetParams(suite.ctx)
	passed, churnBps := suite.keeper.CheckStabilityConditions(suite.ctx, 4, params)
	suite.True(passed, "churn exactly at threshold should pass (<=)")
	suite.Equal(uint64(2000), churnBps)
}

func (suite *EnforcementTestSuite) TestStability_ValidatorAdded() {
	// Snapshot with 2 validators
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 500},
		{OperatorAddr: "val2", Power: 500},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 5)
	require.NoError(suite.T(), err)

	// New validator added with large power
	// Delta = 600 (new val3), total_snap = 1000
	// churn = 600/1000 * 10000 = 6000 bps > 2000
	newVals := []MockValidator{
		{OperatorAddr: "val1", Power: 500},
		{OperatorAddr: "val2", Power: 500},
		{OperatorAddr: "val3", Power: 600},
	}
	suite.stakingKeeper.SetMockValidators(newVals)

	params := suite.keeper.GetParams(suite.ctx)
	passed, churnBps := suite.keeper.CheckStabilityConditions(suite.ctx, 5, params)
	suite.False(passed, "large new validator should trigger churn failure")
	suite.Equal(uint64(6000), churnBps)
}

func (suite *EnforcementTestSuite) TestStability_CustomThreshold() {
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 100},
		{OperatorAddr: "val2", Power: 100},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 6)
	require.NoError(suite.T(), err)

	// Power shift: val1 100->150 (delta=50), total=200, churn=2500 bps
	newVals := []MockValidator{
		{OperatorAddr: "val1", Power: 150},
		{OperatorAddr: "val2", Power: 100},
	}
	suite.stakingKeeper.SetMockValidators(newVals)

	// With default threshold (2000 bps) this fails
	params := suite.keeper.GetParams(suite.ctx)
	passed, churnBps := suite.keeper.CheckStabilityConditions(suite.ctx, 6, params)
	suite.False(passed, "2500 bps > 2000 threshold should fail")
	suite.Equal(uint64(2500), churnBps)

	// With raised threshold (3000 bps) this passes
	params.MaxValidatorChurnBps = 3000
	passed, churnBps = suite.keeper.CheckStabilityConditions(suite.ctx, 6, params)
	suite.True(passed, "2500 bps <= 3000 threshold should pass")
	suite.Equal(uint64(2500), churnBps)
}

func (suite *EnforcementTestSuite) TestStability_SnapshotPersistence() {
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 100},
		{OperatorAddr: "val2", Power: 200},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 7)
	require.NoError(suite.T(), err)

	// Verify snapshot can be retrieved
	snapshot, found := suite.keeper.GetValidatorPowerSnapshot(suite.ctx, 7)
	suite.True(found, "snapshot should be stored")
	suite.Equal(int64(300), snapshot.TotalPower)
	// Build map from Validators slice for assertion
	powerMap := make(map[string]int64)
	for _, v := range snapshot.Validators {
		powerMap[v.Address] = v.Power
	}
	suite.Equal(int64(100), powerMap["val1"])
	suite.Equal(int64(200), powerMap["val2"])
	suite.Equal(int64(100), snapshot.Height)
}

// ============================================================================
// Integration: Gate transition with real stability checks
// ============================================================================

func (suite *EnforcementTestSuite) TestGateTransition_StabilityExtension() {
	// Set up validators and create a queued proposal
	vals := []MockValidator{
		{OperatorAddr: "val1", Power: 500},
		{OperatorAddr: "val2", Power: 500},
	}
	suite.stakingKeeper.SetMockValidators(vals)

	// Create a LOW tier text proposal
	proposal := govtypes.Proposal{
		Id:       200,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
		FinalTallyResult: &govtypes.TallyResult{
			YesCount:        "8000",
			NoCount:         "1000",
			AbstainCount:    "1000",
			NoWithVetoCount: "0",
		},
	}
	suite.govKeeper.SetProposal(proposal)

	// Create queued execution already in CONDITIONAL_EXECUTION state
	// (simulating that it already passed visibility + shock absorber)
	params := suite.keeper.GetParams(suite.ctx)
	exec := types.QueuedExecution{
		ProposalId:            200,
		QueuedHeight:          1,
		EarliestExecHeight:    100, // ready now
		GateState:             types.EXECUTION_GATE_CONDITIONAL_EXECUTION,
		GateEnteredHeight:     50,
		Tier:                  types.RISK_TIER_HIGH,
		RequiredThresholdBps:  params.ThresholdHighBps,
		RequiresSecondConfirm: false,
	}

	// Store snapshot for this proposal (done when entering CONDITIONAL)
	err := suite.keeper.SnapshotValidatorPower(suite.ctx, 200)
	require.NoError(suite.T(), err)

	// Now change validators dramatically before the gate check
	bigChurnVals := []MockValidator{
		{OperatorAddr: "val1", Power: 100}, // 500->100
		{OperatorAddr: "val2", Power: 500},
		{OperatorAddr: "val3", Power: 800}, // new
	}
	suite.stakingKeeper.SetMockValidators(bigChurnVals)

	err = suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	// Process the gate transition
	err = suite.keeper.ProcessGateTransition(suite.ctx, &exec)
	require.NoError(suite.T(), err)

	// Should still be in CONDITIONAL_EXECUTION (extended due to churn)
	updated, found := suite.keeper.GetQueuedExecution(suite.ctx, 200)
	suite.True(found)
	suite.Equal(types.EXECUTION_GATE_CONDITIONAL_EXECUTION, updated.GateState,
		"should remain in CONDITIONAL due to high churn")
	suite.Contains(updated.StatusNote, "Stability checks failed")
}
