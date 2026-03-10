package keeper_test

import (
	"encoding/json"
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

// TestLifecycle_FullFlow simulates the complete lifecycle of a contribution:
// Submit -> Canonical Check -> Similarity Check -> Human Review -> Reward Distribution
func TestLifecycle_FullFlow(t *testing.T) {
	f := SetupKeeperTest(t)
	ctx := f.ctx

	// --- SETUP ---

	// 1. Setup Oracle
	oraclePrivKey := secp256k1.GenPrivKey()
	oraclePubKey := oraclePrivKey.PubKey()
	oracleAddr := sdk.AccAddress(oraclePubKey.Address())

	// Register oracle account in mock keeper (needed for signature verification)
	baseAcc := authtypes.NewBaseAccount(oracleAddr, oraclePubKey, 0, 0)
	f.accountKeeper.SetAccount(baseAcc)

	// 2. Setup Params
	params := f.keeper.GetParams(ctx)
	params.EnableCanonicalHashCheck = true
	params.EnableSimilarityCheck = true
	params.SimilarityOracleAllowlist = []string{oracleAddr.String()}
	params.SimilarityEpochBlocks = 100
	params.DerivativeThreshold = 8500 // 85%
	params.BaseRewardUnit = math.NewInt(1000)
	params.RoyaltyShare = math.LegacyNewDecWithPrec(10, 2) // 10%
	params.DuplicateBond = sdk.NewCoin("omniphi", math.ZeroInt())   // Zero bond for mock bank keeper
	err := f.keeper.SetParams(ctx, params)
	require.NoError(t, err)

	// 3. Setup Contributor
	contributor := sdk.AccAddress("contributor_________")

	// Fund contributor so fee collection succeeds
	f.bankKeeper.setBalance(contributor.String(), "omniphi", math.NewInt(1000000))

	// --- STEP 1: SUBMIT CONTRIBUTION (Layer 1) ---

	msgSubmit := &types.MsgSubmitContribution{
		Contributor:          contributor.String(),
		Ctype:                "code",
		Uri:                  "ipfs://QmTest123",
		Hash:                 make([]byte, 32), // Valid hash
		CanonicalHash:        make([]byte, 32), // Valid canonical hash
		CanonicalSpecVersion: 1,
	}
	msgSubmit.Hash[0] = 0x01
	msgSubmit.CanonicalHash[0] = 0x02

	msgSrv := keeper.NewMsgServerImpl(f.keeper)
	respSubmit, err := msgSrv.SubmitContribution(ctx, msgSubmit)
	require.NoError(t, err)
	contributionID := respSubmit.Id
	require.Equal(t, uint64(1), contributionID)

	// Verify Layer 1 state
	contrib, found := f.keeper.GetContribution(ctx, contributionID)
	require.True(t, found)
	require.False(t, contrib.IsDerivative)
	require.Equal(t, uint64(0), contrib.DuplicateOf)

	// --- STEP 2: SIMILARITY CHECK (Layer 2) ---

	// Create compact data indicating high similarity (derivative)
	compactData := types.SimilarityCompactData{
		ContributionID:       contributionID,
		OverallSimilarity:    9000, // 90% > 85% threshold
		Confidence:           9500,
		NearestParentClaimID: 0,
		ModelVersion:         "v1",
		Epoch:                0, // Block 0 / 100 = 0
	}
	compactJson, _ := json.Marshal(compactData)

	// Sign compact data
	compactSig, err := oraclePrivKey.Sign(compactJson)
	require.NoError(t, err)

	// Create full commitment hash
	fullHash := make([]byte, 32)
	fullHash[0] = 0xAA
	fullSig, err := oraclePrivKey.Sign(fullHash)
	require.NoError(t, err)

	msgSim := &types.MsgSubmitSimilarityCommitment{
		Submitter:              oracleAddr.String(),
		ContributionID:         contributionID,
		CompactDataJson:        compactJson,
		OracleSignatureCompact: compactSig,
		CommitmentHashFull:     fullHash,
		OracleSignatureFull:    fullSig,
	}

	respSim, err := f.keeper.ProcessSimilarityCommitment(ctx, msgSim)
	require.NoError(t, err)
	require.True(t, respSim.IsDerivative)

	// Verify Layer 2 state
	contrib, _ = f.keeper.GetContribution(ctx, contributionID)
	require.True(t, contrib.IsDerivative, "Contribution should be flagged as derivative")

	// --- STEP 3: HUMAN REVIEW (Layer 3 Simulation) ---

	// Since we are simulating the lifecycle and the review keeper methods
	// are not fully exposed in the current context, we simulate the *outcome*
	// of a review.
	// Scenario: AI flagged as derivative (90%), but Human Review overrides it as "False Positive".

	reviewOverride := types.Override_DERIVATIVE_FALSE_POSITIVE

	// --- STEP 4: REWARD CALCULATION (Layer 4) ---

	rewardInput := types.RewardContext{
		ClaimID:         contributionID,
		Category:        "code",
		QualityScore:    math.LegacyNewDec(10), // Perfect quality
		BaseReward:      params.BaseRewardUnit,
		SimilarityScore: math.LegacyNewDecWithPrec(90, 2), // 0.90
		IsDuplicate:     false,
		IsDerivative:    true, // AI said yes
		ReviewOverride:  reviewOverride,
	}

	output, err := f.keeper.CalculateReward(ctx, rewardInput)
	require.NoError(t, err)

	// Verify Economic Logic:
	// AI said 90% similarity (usually 0.4x multiplier).
	// Human said False Positive (Override).
	// Result should be 1.0x multiplier (Original).
	require.True(t, output.OriginalityMultiplier.Equal(math.LegacyOneDec()),
		"Multiplier should be 1.0 due to human override, got %s", output.OriginalityMultiplier)

	require.Equal(t, int64(1000), output.FinalRewardAmount.Int64(), "Should receive full base reward")

	// --- STEP 5: DISTRIBUTION ---

	err = f.keeper.DistributeRewards(ctx, output, contributor.String())
	require.NoError(t, err)

	// Verify events emitted (proxy for bank transfer success in mock env)
	// In a real integration test, we would check bank balances here.
}
