package keeper_test

import (
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"

	"pos/x/guard/types"
)

func (suite *KeeperTestSuite) TestReevaluateRisk_UpdatesEarliestExecHeight() {
	// Create proposal with slashing update -> HIGH tier
	proposal := govtypes.Proposal{
		Id:     1,
		Status: govtypes.StatusPassed,
		Messages: []*codectypes.Any{
			{TypeUrl: "/cosmos.slashing.v1.MsgUpdateParams"},
		},
	}
	suite.govKeeper.SetProposal(proposal)

	// Queue proposal
	err := suite.keeper.OnProposalPassed(suite.ctx, proposal.Id)
	require.NoError(suite.T(), err)

	execBefore, found := suite.keeper.GetQueuedExecution(suite.ctx, proposal.Id)
	require.True(suite.T(), found)
	oldEarliest := execBefore.EarliestExecHeight

	// Enable emergency hardening to force delay escalation
	err = suite.keeper.SetEmergencyHardeningMode(suite.ctx, true)
	require.NoError(suite.T(), err)

	// Advance block time to ensure reevaluation sees a new height
	suite.ctx = suite.ctx.WithBlockHeight(suite.ctx.BlockHeight() + 1)
	suite.ctx = suite.ctx.WithBlockTime(suite.ctx.BlockTime().Add(1 * time.Second))

	err = suite.keeper.ReevaluateRisk(suite.ctx, proposal.Id)
	require.NoError(suite.T(), err)

	execAfter, found := suite.keeper.GetQueuedExecution(suite.ctx, proposal.Id)
	require.True(suite.T(), found)

	require.GreaterOrEqual(suite.T(), execAfter.EarliestExecHeight, oldEarliest,
		"earliest exec height must not decrease")
	require.Greater(suite.T(), execAfter.EarliestExecHeight, oldEarliest,
		"earliest exec height should increase after delay escalation")

	// Sanity: exec remains non-terminal
	require.False(suite.T(), execAfter.IsTerminal())
}

func (suite *KeeperTestSuite) TestQueuePruning_RemovesTerminalIndex() {
	// Create terminal queued execution and index
	exec := types.QueuedExecution{
		ProposalId:         42,
		QueuedHeight:       uint64(suite.ctx.BlockHeight()),
		EarliestExecHeight: uint64(suite.ctx.BlockHeight()),
		GateState:          types.EXECUTION_GATE_EXECUTED,
		GateEnteredHeight:  uint64(suite.ctx.BlockHeight()),
	}
	err := suite.keeper.SetQueuedExecution(suite.ctx, exec)
	require.NoError(suite.T(), err)

	store := suite.ctx.KVStore(suite.storeKey)
	indexKey := types.GetQueueIndexKey(exec.EarliestExecHeight, exec.ProposalId)
	require.NotNil(suite.T(), store.Get(indexKey), "queue index must exist before pruning")

	// ProcessQueue should prune terminal entries
	err = suite.keeper.ProcessQueue(suite.ctx)
	require.NoError(suite.T(), err)

	require.Nil(suite.T(), store.Get(indexKey), "queue index should be pruned for terminal entry")
}
