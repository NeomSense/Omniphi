package keeper_test

import (
	"context"
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

// SecurityTestSuite tests the production hardening features:
// - Bypass prevention (pre-dispatch assertions)
// - Double execution prevention
// - Anti-DOS bounded processing
// - Invariants
// - Event correctness
type SecurityTestSuite struct {
	suite.Suite

	ctx            sdk.Context
	keeper         keeper.Keeper
	cdc            codec.Codec
	govKeeper      *MockGovKeeper
	stakingKeeper  *MockStakingKeeper
	bankKeeper     *MockBankKeeper
	distrKeeper    *MockDistrKeeper
	router         *MockMessageRouter
	timelockKeeper *MockTimelockKeeper
}

func TestSecurityTestSuite(t *testing.T) {
	suite.Run(t, new(SecurityTestSuite))
}

func (suite *SecurityTestSuite) SetupTest() {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(suite.T(), stateStore.LoadLatestVersion())

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	suite.cdc = codec.NewProtoCodec(interfaceRegistry)

	suite.ctx = sdk.NewContext(stateStore, tmproto.Header{Height: 100, Time: time.Now()}, false, log.NewNopLogger())

	suite.govKeeper = NewMockGovKeeper()
	suite.stakingKeeper = NewMockStakingKeeper()
	suite.bankKeeper = NewMockBankKeeper()
	suite.distrKeeper = NewMockDistrKeeper()
	suite.router = NewMockMessageRouter()
	suite.timelockKeeper = &MockTimelockKeeper{}

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

	kp := &suite.keeper
	kp.SetDistrKeeper(suite.distrKeeper)
	kp.SetRouter(suite.router)
	kp.SetInterfaceRegistry(interfaceRegistry)
	kp.SetTimelockKeeper(suite.timelockKeeper)

	err := suite.keeper.SetParams(suite.ctx, types.DefaultParams())
	require.NoError(suite.T(), err)
}

// MockTimelockKeeper for testing freeze check
type MockTimelockKeeper struct {
	Frozen bool
}

func (m *MockTimelockKeeper) IsTrackFrozen(ctx context.Context, opID uint64) (bool, string) {
	if m.Frozen {
		return true, "TRACK_FROZEN"
	}
	return false, ""
}

func (m *MockTimelockKeeper) SetFrozen(frozen bool) {
	m.Frozen = frozen
}

// ============================================================================
// Part 1: Bypass Prevention Tests
// ============================================================================

func (suite *SecurityTestSuite) TestBypass_ExecuteRejectsNonReadyState() {
	// ExecuteProposal must reject exec with gate_state != READY
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		return &sdk.Result{}, nil
	})

	anyMsg, _ := codectypes.NewAnyWithValue(msg)
	proposal := govtypes.Proposal{
		Id:       1,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	// Try every non-READY state
	nonReadyStates := []types.ExecutionGateState{
		types.EXECUTION_GATE_VISIBILITY,
		types.EXECUTION_GATE_SHOCK_ABSORBER,
		types.EXECUTION_GATE_CONDITIONAL_EXECUTION,
		types.EXECUTION_GATE_EXECUTED,
		types.EXECUTION_GATE_ABORTED,
	}

	for _, state := range nonReadyStates {
		exec := &types.QueuedExecution{
			ProposalId: 1,
			GateState:  state,
		}
		err := suite.keeper.ExecuteProposal(suite.ctx, exec)
		suite.Error(err, "should reject execution from state %s", state.GetGateStateName())
		suite.Contains(err.Error(), "not ready for execution",
			"error for state %s should mention not ready", state.GetGateStateName())
	}
}

func (suite *SecurityTestSuite) TestBypass_ExecuteAcceptsReadyState() {
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		return &sdk.Result{}, nil
	})

	anyMsg, _ := codectypes.NewAnyWithValue(msg)
	proposal := govtypes.Proposal{
		Id:       2,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{
		ProposalId: 2,
		GateState:  types.EXECUTION_GATE_READY,
	}
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.NoError(err, "READY state should be accepted")
}

func (suite *SecurityTestSuite) TestBypass_ExecuteRejectsNonPassedProposal() {
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		return &sdk.Result{}, nil
	})

	anyMsg, _ := codectypes.NewAnyWithValue(msg)

	// Proposal with REJECTED status
	proposal := govtypes.Proposal{
		Id:       3,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusRejected,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{
		ProposalId: 3,
		GateState:  types.EXECUTION_GATE_READY,
	}
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.Error(err, "should reject non-PASSED proposal")
	suite.Contains(err.Error(), "did not pass")
}

func (suite *SecurityTestSuite) TestBypass_ExecuteRejectsFrozenTrack() {
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		return &sdk.Result{}, nil
	})

	anyMsg, _ := codectypes.NewAnyWithValue(msg)
	proposal := govtypes.Proposal{
		Id:       4,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := &types.QueuedExecution{
		ProposalId: 4,
		GateState:  types.EXECUTION_GATE_READY,
	}

	suite.timelockKeeper.SetFrozen(true)
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.Error(err, "should reject execution when track is frozen")
	suite.Contains(err.Error(), "track frozen")
}

// ============================================================================
// Part 2: Double Execution Prevention
// ============================================================================

func (suite *SecurityTestSuite) TestDoubleExec_MarkerPreventsReExecution() {
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}

	callCount := 0
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		callCount++
		return &sdk.Result{}, nil
	})

	anyMsg, _ := codectypes.NewAnyWithValue(msg)
	proposal := govtypes.Proposal{
		Id:       10,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	// First execution should succeed
	exec := &types.QueuedExecution{
		ProposalId: 10,
		GateState:  types.EXECUTION_GATE_READY,
	}
	err := suite.keeper.ExecuteProposal(suite.ctx, exec)
	suite.NoError(err, "first execution should succeed")
	suite.Equal(1, callCount, "handler called once")

	// Manually set execution marker (normally done by ProcessGateTransition)
	err = suite.keeper.SetExecutionMarker(suite.ctx, 10)
	suite.NoError(err)

	// Second execution should be rejected
	exec2 := &types.QueuedExecution{
		ProposalId: 10,
		GateState:  types.EXECUTION_GATE_READY,
	}
	err = suite.keeper.ExecuteProposal(suite.ctx, exec2)
	suite.Error(err, "second execution should be rejected")
	suite.Contains(err.Error(), "already executed")
	suite.Equal(1, callCount, "handler should NOT be called again")
}

func (suite *SecurityTestSuite) TestDoubleExec_MarkerSetByGateTransition() {
	// Verify that ProcessGateTransition sets the execution marker
	authority := sdk.AccAddress("authority_________").String()

	msg := &banktypes.MsgSend{
		FromAddress: authority,
		ToAddress:   sdk.AccAddress("recipient_________").String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(100))),
	}
	suite.router.RegisterHandler(sdk.MsgTypeURL(msg), func(ctx sdk.Context, req sdk.Msg) (*sdk.Result, error) {
		return &sdk.Result{}, nil
	})

	anyMsg, _ := codectypes.NewAnyWithValue(msg)
	proposal := govtypes.Proposal{
		Id:       11,
		Messages: []*codectypes.Any{anyMsg},
		Status:   govtypes.StatusPassed,
		FinalTallyResult: &govtypes.TallyResult{
			YesCount:        "8000",
			NoCount:         "1000",
			AbstainCount:    "1000",
			NoWithVetoCount: "0",
		},
	}
	suite.govKeeper.SetProposal(proposal)

	// No marker before execution
	suite.False(suite.keeper.HasExecutionMarker(suite.ctx, 11))

	// Create exec in READY state
	exec := types.QueuedExecution{
		ProposalId:           11,
		QueuedHeight:         1,
		EarliestExecHeight:   50,
		GateState:            types.EXECUTION_GATE_READY,
		GateEnteredHeight:    50,
		Tier:                 types.RISK_TIER_LOW,
		RequiredThresholdBps: 5000,
	}
	err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	// Process gate transition (should execute and set marker)
	err = suite.keeper.ProcessGateTransition(suite.ctx, &exec)
	require.NoError(suite.T(), err)

	// Marker should now exist
	suite.True(suite.keeper.HasExecutionMarker(suite.ctx, 11),
		"execution marker should be set after successful execution")

	// Verify state is EXECUTED
	stored, found := suite.keeper.GetQueuedExecution(suite.ctx, 11)
	suite.True(found)
	suite.Equal(types.EXECUTION_GATE_EXECUTED, stored.GateState)
}

// ============================================================================
// Part 3: Anti-DOS Bounded Processing
// ============================================================================

func (suite *SecurityTestSuite) TestAntiDOS_QueueScanDepthBounded() {
	// Queue many proposals and verify only MaxQueueScanDepth are processed
	params := types.DefaultParams()
	params.MaxQueueScanDepth = 5 // Set very low for testing
	params.MaxProposalsPerBlock = 100
	// Set short gate windows so processing can advance
	params.VisibilityWindowBlocks = 1
	params.ShockAbsorberWindowBlocks = 1
	err := suite.keeper.SetParams(suite.ctx, params)
	require.NoError(suite.T(), err)

	// Queue 20 proposals at height 50 (before current height of 100)
	for i := uint64(1); i <= 20; i++ {
		proposal := govtypes.Proposal{
			Id:       i,
			Messages: []*codectypes.Any{},
			Status:   govtypes.StatusPassed,
		}
		suite.govKeeper.SetProposal(proposal)

		exec := types.QueuedExecution{
			ProposalId:         i,
			QueuedHeight:       10,
			EarliestExecHeight: 50,
			GateState:          types.EXECUTION_GATE_VISIBILITY,
			GateEnteredHeight:  10,
			Tier:               types.RISK_TIER_LOW,
		}
		err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
		require.NoError(suite.T(), err)
	}

	// Process queue — should be bounded by MaxQueueScanDepth=5
	err = suite.keeper.ProcessQueue(suite.ctx)
	require.NoError(suite.T(), err)

	// Count how many advanced past VISIBILITY
	advancedCount := 0
	for i := uint64(1); i <= 20; i++ {
		exec, found := suite.keeper.GetQueuedExecution(suite.ctx, i)
		if found && exec.GateState != types.EXECUTION_GATE_VISIBILITY {
			advancedCount++
		}
	}

	// At most 5 should have been processed (scan depth limit)
	suite.LessOrEqual(advancedCount, 5,
		"at most MaxQueueScanDepth proposals should be processed")
}

func (suite *SecurityTestSuite) TestAntiDOS_PollerBounded() {
	// Create many passed proposals and verify polling is bounded
	params := types.DefaultParams()
	params.MaxProposalsPerBlock = 3
	err := suite.keeper.SetParams(suite.ctx, params)
	require.NoError(suite.T(), err)

	for i := uint64(1); i <= 10; i++ {
		proposal := govtypes.Proposal{
			Id:       i,
			Messages: []*codectypes.Any{},
			Status:   govtypes.StatusPassed,
		}
		suite.govKeeper.SetProposal(proposal)
	}

	// Poll — should process at most 3
	err = suite.keeper.PollGovernanceProposals(suite.ctx)
	require.NoError(suite.T(), err)

	// Count how many were queued
	queuedCount := 0
	for i := uint64(1); i <= 10; i++ {
		if _, found := suite.keeper.GetQueuedExecution(suite.ctx, i); found {
			queuedCount++
		}
	}

	suite.LessOrEqual(queuedCount, 3,
		"at most MaxProposalsPerBlock proposals should be polled")
}

func (suite *SecurityTestSuite) TestAntiDOS_DefaultParamsValid() {
	params := types.DefaultParams()
	suite.Equal(uint64(10), params.MaxProposalsPerBlock)
	suite.Equal(uint64(100), params.MaxQueueScanDepth)

	err := params.Validate()
	suite.NoError(err, "default params should be valid")
}

func (suite *SecurityTestSuite) TestAntiDOS_ParamsValidation() {
	// Zero values should fail
	params := types.DefaultParams()
	params.MaxProposalsPerBlock = 0
	err := params.Validate()
	suite.Error(err, "max_proposals_per_block=0 should fail validation")

	params = types.DefaultParams()
	params.MaxQueueScanDepth = 0
	err = params.Validate()
	suite.Error(err, "max_queue_scan_depth=0 should fail validation")

	// Excessive values should fail
	params = types.DefaultParams()
	params.MaxProposalsPerBlock = 1001
	err = params.Validate()
	suite.Error(err, "max_proposals_per_block>1000 should fail validation")

	params = types.DefaultParams()
	params.MaxQueueScanDepth = 10001
	err = params.Validate()
	suite.Error(err, "max_queue_scan_depth>10000 should fail validation")
}

// ============================================================================
// Part 4: Invariants
// ============================================================================

func (suite *SecurityTestSuite) TestInvariant_BypassDetection_Clean() {
	// When all EXECUTED proposals have markers, invariant should pass
	exec := types.QueuedExecution{
		ProposalId:         100,
		QueuedHeight:       1,
		EarliestExecHeight: 50,
		GateState:          types.EXECUTION_GATE_EXECUTED,
	}
	err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	// Set matching marker
	err = suite.keeper.SetExecutionMarker(suite.ctx, 100)
	require.NoError(suite.T(), err)

	invariant := keeper.ExecutionBypassInvariant(suite.keeper)
	msg, broken := invariant(suite.ctx)
	suite.False(broken, "invariant should not be broken: %s", msg)
}

func (suite *SecurityTestSuite) TestInvariant_BypassDetection_Broken() {
	// EXECUTED without marker = bypass detected
	exec := types.QueuedExecution{
		ProposalId:         200,
		QueuedHeight:       1,
		EarliestExecHeight: 50,
		GateState:          types.EXECUTION_GATE_EXECUTED,
	}
	err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	// Deliberately do NOT set execution marker

	invariant := keeper.ExecutionBypassInvariant(suite.keeper)
	msg, broken := invariant(suite.ctx)
	suite.True(broken, "invariant should be broken when marker is missing")
	suite.Contains(msg, "200", "should report the bypassed proposal ID")
}

func (suite *SecurityTestSuite) TestInvariant_QueueConsistency_Clean() {
	proposal := govtypes.Proposal{
		Id:       300,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	exec := types.QueuedExecution{
		ProposalId:         300,
		QueuedHeight:       1,
		EarliestExecHeight: 50,
		GateState:          types.EXECUTION_GATE_VISIBILITY,
	}
	err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	invariant := keeper.QueueConsistencyInvariant(suite.keeper)
	msg, broken := invariant(suite.ctx)
	suite.False(broken, "invariant should not be broken: %s", msg)
}

func (suite *SecurityTestSuite) TestInvariant_QueueConsistency_ExecutedWithoutConfirm() {
	// EXECUTED + RequiresSecondConfirm=true + SecondConfirmReceived=false
	exec := types.QueuedExecution{
		ProposalId:            400,
		QueuedHeight:          1,
		EarliestExecHeight:    50,
		GateState:             types.EXECUTION_GATE_EXECUTED,
		RequiresSecondConfirm: true,
		SecondConfirmReceived: false,
	}
	err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	// Also need a matching marker (so bypass invariant doesn't flag it)
	err = suite.keeper.SetExecutionMarker(suite.ctx, 400)
	require.NoError(suite.T(), err)

	invariant := keeper.QueueConsistencyInvariant(suite.keeper)
	msg, broken := invariant(suite.ctx)
	suite.True(broken, "invariant should be broken: EXECUTED without 2nd confirm")
	suite.Contains(msg, "400")
	suite.Contains(msg, "missing required 2nd confirmation")
}

// ============================================================================
// Part 5: Execution Marker Store Operations
// ============================================================================

func (suite *SecurityTestSuite) TestExecutionMarker_SetAndCheck() {
	suite.False(suite.keeper.HasExecutionMarker(suite.ctx, 1))

	err := suite.keeper.SetExecutionMarker(suite.ctx, 1)
	suite.NoError(err)

	suite.True(suite.keeper.HasExecutionMarker(suite.ctx, 1))
	suite.False(suite.keeper.HasExecutionMarker(suite.ctx, 2))
}

func (suite *SecurityTestSuite) TestExecutionMarker_Iterate() {
	// Set markers for proposals 5, 10, 15
	for _, id := range []uint64{5, 10, 15} {
		err := suite.keeper.SetExecutionMarker(suite.ctx, id)
		suite.NoError(err)
	}

	var found []uint64
	suite.keeper.IterateExecutionMarkers(suite.ctx, func(proposalID uint64) bool {
		found = append(found, proposalID)
		return false
	})

	suite.Len(found, 3)
	suite.Contains(found, uint64(5))
	suite.Contains(found, uint64(10))
	suite.Contains(found, uint64(15))
}

// ============================================================================
// Part 6: GuardStatus Consolidated Query
// ============================================================================

func (suite *SecurityTestSuite) TestGuardStatus_Counts() {
	// Set up various proposals in different states
	states := map[uint64]types.ExecutionGateState{
		1: types.EXECUTION_GATE_VISIBILITY,
		2: types.EXECUTION_GATE_SHOCK_ABSORBER,
		3: types.EXECUTION_GATE_CONDITIONAL_EXECUTION,
		4: types.EXECUTION_GATE_READY,
		5: types.EXECUTION_GATE_EXECUTED,
		6: types.EXECUTION_GATE_ABORTED,
		7: types.EXECUTION_GATE_EXECUTED,
	}

	for id, state := range states {
		exec := types.QueuedExecution{
			ProposalId:         id,
			QueuedHeight:       1,
			EarliestExecHeight: 50,
			GateState:          state,
		}
		err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
		require.NoError(suite.T(), err)
	}

	status := suite.keeper.GetGuardStatus(suite.ctx)
	suite.Equal(uint64(7), status.QueuedCount, "total queued")
	suite.Equal(uint64(2), status.ExecutedCount, "executed count")
	suite.Equal(uint64(1), status.AbortedCount, "aborted count")
	suite.Equal(uint64(4), status.PendingCount, "pending count")
	suite.Equal(int64(100), status.CurrentHeight)
}

// ============================================================================
// Part 7: Event Enrichment Verification
// ============================================================================

func (suite *SecurityTestSuite) TestEvents_ProposalQueuedHasFullAttributes() {
	proposal := govtypes.Proposal{
		Id:       500,
		Messages: []*codectypes.Any{},
		Status:   govtypes.StatusPassed,
	}
	suite.govKeeper.SetProposal(proposal)

	// Clear any existing events
	suite.ctx = suite.ctx.WithEventManager(sdk.NewEventManager())

	err := suite.keeper.OnProposalPassed(suite.ctx, 500)
	require.NoError(suite.T(), err)

	events := suite.ctx.EventManager().Events()

	// Find the guard_proposal_queued event
	var queuedEvent *sdk.Event
	for i := range events {
		if events[i].Type == "guard_proposal_queued" {
			queuedEvent = &events[i]
			break
		}
	}

	suite.NotNil(queuedEvent, "guard_proposal_queued event should be emitted")

	// Verify all required attributes exist
	attrMap := make(map[string]string)
	for _, attr := range queuedEvent.Attributes {
		attrMap[attr.Key] = attr.Value
	}

	requiredAttrs := []string{
		"proposal_id", "tier", "score", "delay_blocks",
		"threshold_bps", "queued_height", "earliest_exec_height",
		"requires_confirmation", "reason_codes",
	}

	for _, attr := range requiredAttrs {
		suite.Contains(attrMap, attr,
			fmt.Sprintf("guard_proposal_queued should have attribute %q", attr))
	}

	suite.Equal("500", attrMap["proposal_id"])
	suite.Equal("LOW", attrMap["tier"])
	suite.Equal("100", attrMap["queued_height"])
}

// ============================================================================
// Part 8: Post-Audit Fix Verification (P2 & P3)
// ============================================================================

func (suite *SecurityTestSuite) TestFix_ComputeAggregateRisk_UsesActiveIndex() {
	// Create 100 terminal proposals (should NOT be scanned)
	for i := uint64(1); i <= 100; i++ {
		exec := types.QueuedExecution{
			ProposalId: i,
			GateState:  types.EXECUTION_GATE_EXECUTED, // Terminal
		}
		// SetQueuedExecution handles index maintenance
		err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
		require.NoError(suite.T(), err)
	}

	// Create 1 active proposal
	activeExec := types.QueuedExecution{
		ProposalId:   101,
		GateState:    types.EXECUTION_GATE_READY,
		QueuedHeight: uint64(suite.ctx.BlockHeight()),
		Tier:         types.RISK_TIER_CRITICAL,
	}
	err := suite.keeper.SetQueuedExecution(suite.ctx, activeExec)
	require.NoError(suite.T(), err)

	// Compute aggregate risk
	// If it scans all 101, it might be slow, but we check correctness here.
	// The real proof is in the code using IterateActiveExecutions.
	snapshot := suite.keeper.ComputeAggregateRisk(suite.ctx)

	// Should only count the 1 active proposal
	suite.Equal(uint64(1), snapshot.ActiveProposalCount)
	suite.Equal(types.RISK_TIER_CRITICAL, snapshot.HighestTier)
}

func (suite *SecurityTestSuite) TestFix_TrackFreeze_DefersNotAborts() {
	// Setup proposal in READY state
	exec := types.QueuedExecution{
		ProposalId:         200,
		GateState:          types.EXECUTION_GATE_READY,
		EarliestExecHeight: 100,
	}
	// Mock gov proposal with sufficient votes to pass threshold check
	proposal := govtypes.Proposal{
		Id:     200,
		Status: govtypes.StatusPassed,
		FinalTallyResult: &govtypes.TallyResult{
			YesCount:        "8000",
			NoCount:         "1000",
			AbstainCount:    "500",
			NoWithVetoCount: "500",
		},
	}
	suite.govKeeper.SetProposal(proposal)

	// Freeze track
	suite.timelockKeeper.SetFrozen(true)

	// Process transition
	err := suite.keeper.ProcessGateTransition(suite.ctx, &exec)
	require.NoError(suite.T(), err)

	// Verify state
	updated, found := suite.keeper.GetQueuedExecution(suite.ctx, 200)
	suite.True(found)

	// Should NOT be ABORTED
	suite.NotEqual(types.EXECUTION_GATE_ABORTED, updated.GateState)
	suite.Equal(types.EXECUTION_GATE_READY, updated.GateState)

	// Should be deferred (EarliestExecHeight increased)
	suite.Greater(updated.EarliestExecHeight, uint64(100))
	suite.Contains(updated.StatusNote, "deferred")
}
