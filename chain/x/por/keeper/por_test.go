package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"pos/x/por/keeper"
	"pos/x/por/types"
)

// Test addresses - proper bech32 generated from fixed byte arrays
var (
	testOwner      = sdk.AccAddress("owner_____________").String()
	testSubmitter  = sdk.AccAddress("submitter_________").String()
	testChallenger = sdk.AccAddress("challenger________").String()
	testVerifier1  = sdk.AccAddress("verifier1_________").String()
	testVerifier2  = sdk.AccAddress("verifier2_________").String()
	testVerifier3  = sdk.AccAddress("verifier3_________").String()
)

// ============================================================================
// Params Tests
// ============================================================================

func TestParams_SetAndGet(t *testing.T) {
	f := SetupKeeperTest(t)

	params := f.keeper.GetParams(f.ctx)
	require.Equal(t, types.DefaultMaxBatchesPerBlock, params.MaxBatchesPerBlock)
	require.Equal(t, types.DefaultMinVerifiersGlobal, params.MinVerifiersGlobal)
	require.Equal(t, types.DefaultMaxFinalizationsPerBlock, params.MaxFinalizationsPerBlock)
}

func TestParams_Validate(t *testing.T) {
	// Valid params
	params := types.DefaultParams()
	require.NoError(t, params.Validate())

	// Invalid: zero max batches
	invalid := types.DefaultParams()
	invalid.MaxBatchesPerBlock = 0
	require.Error(t, invalid.Validate())

	// Invalid: min > max challenge period
	invalid2 := types.DefaultParams()
	invalid2.MinChallengePeriod = 1000
	invalid2.MaxChallengePeriod = 500
	require.Error(t, invalid2.Validate())

	// Invalid: negative slash fraction
	invalid3 := types.DefaultParams()
	invalid3.SlashFractionDishonest = math.LegacyNewDec(-1)
	require.Error(t, invalid3.Validate())
}

// ============================================================================
// App CRUD Tests
// ============================================================================

func TestApp_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	// Get next app ID
	appID, err := f.keeper.GetNextAppID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), appID)

	// Create and store app
	app := types.NewApp(appID, "TestApp", testOwner, "QmTest123", 86400, 3, time.Now().Unix())
	err = f.keeper.SetApp(f.ctx, app)
	require.NoError(t, err)

	// Retrieve app
	got, found := f.keeper.GetApp(f.ctx, appID)
	require.True(t, found)
	require.Equal(t, "TestApp", got.Name)
	require.Equal(t, testOwner, got.Owner)
	require.Equal(t, types.AppStatusActive, got.Status)

	// Not found
	_, found = f.keeper.GetApp(f.ctx, 999)
	require.False(t, found)

	// GetAllApps
	apps := f.keeper.GetAllApps(f.ctx)
	require.Len(t, apps, 1)
}

func TestApp_AutoIncrementID(t *testing.T) {
	f := SetupKeeperTest(t)

	id1, err := f.keeper.GetNextAppID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), id1)

	id2, err := f.keeper.GetNextAppID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), id2)

	id3, err := f.keeper.GetNextAppID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(3), id3)
}

// ============================================================================
// VerifierSet CRUD Tests
// ============================================================================

func TestVerifierSet_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	vsID, err := f.keeper.GetNextVerifierSetID(f.ctx)
	require.NoError(t, err)

	members := []types.VerifierMember{
		types.NewVerifierMember(testVerifier1, math.NewInt(1000000), time.Now().Unix()),
		types.NewVerifierMember(testVerifier2, math.NewInt(2000000), time.Now().Unix()),
		types.NewVerifierMember(testVerifier3, math.NewInt(3000000), time.Now().Unix()),
	}

	vs := types.NewVerifierSet(vsID, 1, members, 2, math.LegacyNewDecWithPrec(67, 2), 1)
	err = f.keeper.SetVerifierSet(f.ctx, vs)
	require.NoError(t, err)

	got, found := f.keeper.GetVerifierSet(f.ctx, vsID)
	require.True(t, found)
	require.Len(t, got.Members, 3)
	require.Equal(t, uint32(2), got.MinAttestations)

	// Total weight
	totalWeight := got.GetTotalWeight()
	require.Equal(t, math.NewInt(6000000), totalWeight)

	// IsMember
	require.True(t, got.IsMember(testVerifier1))
	require.False(t, got.IsMember(sdk.AccAddress("unknown___________").String()))
}

// ============================================================================
// BatchCommitment CRUD Tests
// ============================================================================

func TestBatch_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	batchID, err := f.keeper.GetNextBatchID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), batchID)

	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	now := time.Now().Unix()
	batch := types.NewBatchCommitment(batchID, 1, merkleRoot, 1000, 1, 1, testSubmitter, now+86400, now)
	err = f.keeper.SetBatch(f.ctx, batch)
	require.NoError(t, err)

	// Retrieve by ID
	got, found := f.keeper.GetBatch(f.ctx, batchID)
	require.True(t, found)
	require.Equal(t, uint64(1000), got.RecordCount)
	require.Equal(t, types.BatchStatusSubmitted, got.Status)

	// Retrieve by status
	submitted := f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusSubmitted)
	require.Len(t, submitted, 1)

	pending := f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusPending)
	require.Len(t, pending, 0)

	// Retrieve by epoch
	epochBatches := f.keeper.GetBatchesByEpoch(f.ctx, 1)
	require.Len(t, epochBatches, 1)
}

func TestBatch_StatusTransition(t *testing.T) {
	f := SetupKeeperTest(t)

	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	now := time.Now().Unix()
	batch := types.NewBatchCommitment(1, 1, merkleRoot, 100, 1, 1, testSubmitter, now+86400, now)
	err := f.keeper.SetBatch(f.ctx, batch)
	require.NoError(t, err)

	// Transition to PENDING
	err = f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusPending)
	require.NoError(t, err)

	// Verify old status index removed, new one added
	submitted := f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusSubmitted)
	require.Len(t, submitted, 0)

	pending := f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusPending)
	require.Len(t, pending, 1)

	// Transition to FINALIZED
	err = f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusFinalized)
	require.NoError(t, err)

	pending = f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusPending)
	require.Len(t, pending, 0)

	finalized := f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusFinalized)
	require.Len(t, finalized, 1)
}

// ============================================================================
// Attestation Tests
// ============================================================================

func TestAttestation_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	att := types.NewAttestation(1, testVerifier1, []byte("sig1"), math.LegacyNewDecWithPrec(95, 2), time.Now().Unix())
	err := f.keeper.SetAttestation(f.ctx, att)
	require.NoError(t, err)

	got, found := f.keeper.GetAttestation(f.ctx, 1, testVerifier1)
	require.True(t, found)
	require.Equal(t, testVerifier1, got.VerifierAddress)

	// Not found for different verifier
	_, found = f.keeper.GetAttestation(f.ctx, 1, sdk.AccAddress("unknown___________").String())
	require.False(t, found)

	// Get all for batch
	att2 := types.NewAttestation(1, testVerifier2, []byte("sig2"), math.LegacyNewDecWithPrec(90, 2), time.Now().Unix())
	err = f.keeper.SetAttestation(f.ctx, att2)
	require.NoError(t, err)

	batchAtts := f.keeper.GetAttestationsForBatch(f.ctx, 1)
	require.Len(t, batchAtts, 2)
}

// ============================================================================
// Challenge Tests
// ============================================================================

func TestChallenge_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	chalID, err := f.keeper.GetNextChallengeID(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), chalID)

	challenge := types.NewChallenge(chalID, 1, testChallenger, types.ChallengeTypeInvalidRoot, []byte("proof"), time.Now().Unix())
	err = f.keeper.SetChallenge(f.ctx, challenge)
	require.NoError(t, err)

	got, found := f.keeper.GetChallenge(f.ctx, chalID)
	require.True(t, found)
	require.Equal(t, types.ChallengeStatusOpen, got.Status)
	require.Equal(t, types.ChallengeTypeInvalidRoot, got.ChallengeType)

	// Get for batch
	batchChallenges := f.keeper.GetChallengesForBatch(f.ctx, 1)
	require.Len(t, batchChallenges, 1)

	// Has open challenges
	require.True(t, f.keeper.HasOpenChallenges(f.ctx, 1))
	require.False(t, f.keeper.HasOpenChallenges(f.ctx, 999))
}

// ============================================================================
// VerifierReputation Tests
// ============================================================================

func TestVerifierReputation_CRUD(t *testing.T) {
	f := SetupKeeperTest(t)

	// GetOrCreate returns default
	rep := f.keeper.GetOrCreateVerifierReputation(f.ctx, testVerifier1)
	require.Equal(t, testVerifier1, rep.Address)
	require.Equal(t, uint64(0), rep.TotalAttestations)
	require.Equal(t, math.ZeroInt(), rep.ReputationScore)

	// Update and save
	rep.TotalAttestations = 10
	rep.CorrectAttestations = 8
	rep.ReputationScore = math.NewInt(8)
	err := f.keeper.SetVerifierReputation(f.ctx, rep)
	require.NoError(t, err)

	// Retrieve
	got, found := f.keeper.GetVerifierReputation(f.ctx, testVerifier1)
	require.True(t, found)
	require.Equal(t, uint64(10), got.TotalAttestations)
	require.Equal(t, uint64(8), got.CorrectAttestations)
}

// ============================================================================
// Rate Limiting Tests
// ============================================================================

func TestRateLimit(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set max batches per block to 3
	params := f.keeper.GetParams(f.ctx)
	params.MaxBatchesPerBlock = 3
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// First 3 should pass
	require.NoError(t, f.keeper.CheckRateLimit(f.ctx))
	require.NoError(t, f.keeper.CheckRateLimit(f.ctx))
	require.NoError(t, f.keeper.CheckRateLimit(f.ctx))

	// 4th should fail
	err = f.keeper.CheckRateLimit(f.ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrRateLimitExceeded)
}

// ============================================================================
// Genesis Tests
// ============================================================================

func TestGenesis_InitAndExport(t *testing.T) {
	f := SetupKeeperTest(t)

	// Init with default genesis
	gs := types.DefaultGenesis()
	err := f.keeper.InitGenesis(f.ctx, *gs)
	require.NoError(t, err)

	// Export genesis
	exported := f.keeper.ExportGenesis(f.ctx)
	require.NotNil(t, exported)
	require.Equal(t, gs.Params.MaxBatchesPerBlock, exported.Params.MaxBatchesPerBlock)
}

func TestGenesis_ValidateDefault(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NoError(t, gs.Validate())
}

// ============================================================================
// Invariant Tests
// ============================================================================

func TestInvariants_Clean(t *testing.T) {
	f := SetupKeeperTest(t)

	// With empty state, all invariants should pass
	msg, broken := keeper.AllInvariants(f.keeper)(f.ctx)
	require.False(t, broken, "invariants broken on clean state: %s", msg)
}

func TestInvariants_WithValidData(t *testing.T) {
	f := SetupKeeperTest(t)

	// Add valid batch
	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}
	batch := types.NewBatchCommitment(1, 1, merkleRoot, 100, 1, 1, testSubmitter, time.Now().Unix()+86400, time.Now().Unix())
	require.NoError(t, f.keeper.SetBatch(f.ctx, batch))

	// Add valid attestation
	att := types.NewAttestation(1, testVerifier1, []byte("sig"), math.LegacyOneDec(), time.Now().Unix())
	require.NoError(t, f.keeper.SetAttestation(f.ctx, att))

	// Add valid reputation
	rep := types.NewVerifierReputation(testVerifier1)
	rep.TotalAttestations = 5
	rep.CorrectAttestations = 4
	rep.ReputationScore = math.NewInt(4)
	require.NoError(t, f.keeper.SetVerifierReputation(f.ctx, rep))

	msg, broken := keeper.AllInvariants(f.keeper)(f.ctx)
	require.False(t, broken, "invariants broken with valid data: %s", msg)
}

// ============================================================================
// Message Validation Tests
// ============================================================================

func TestMsgRegisterApp_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgRegisterApp
		wantErr bool
	}{
		{
			name: "valid",
			msg: types.MsgRegisterApp{
				Owner:           testOwner,
				Name:            "TestApp",
				SchemaCid:       "QmTest123",
				ChallengePeriod: 86400,
				MinVerifiers:    3,
			},
			wantErr: false,
		},
		{
			name: "empty owner",
			msg: types.MsgRegisterApp{
				Owner:           "",
				Name:            "TestApp",
				SchemaCid:       "QmTest123",
				ChallengePeriod: 86400,
				MinVerifiers:    3,
			},
			wantErr: true,
		},
		{
			name: "empty name",
			msg: types.MsgRegisterApp{
				Owner:           testOwner,
				Name:            "",
				SchemaCid:       "QmTest123",
				ChallengePeriod: 86400,
				MinVerifiers:    3,
			},
			wantErr: true,
		},
		{
			name: "zero challenge period",
			msg: types.MsgRegisterApp{
				Owner:           testOwner,
				Name:            "TestApp",
				SchemaCid:       "QmTest123",
				ChallengePeriod: 0,
				MinVerifiers:    3,
			},
			wantErr: true,
		},
		{
			name: "zero min verifiers",
			msg: types.MsgRegisterApp{
				Owner:           testOwner,
				Name:            "TestApp",
				SchemaCid:       "QmTest123",
				ChallengePeriod: 86400,
				MinVerifiers:    0,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgSubmitBatch_ValidateBasic(t *testing.T) {
	validRoot := make([]byte, 32)
	for i := range validRoot {
		validRoot[i] = byte(i + 1)
	}

	allZeros := make([]byte, 32)
	allOnes := make([]byte, 32)
	for i := range allOnes {
		allOnes[i] = 0xFF
	}

	tests := []struct {
		name    string
		msg     types.MsgSubmitBatch
		wantErr bool
	}{
		{
			name: "valid",
			msg: types.MsgSubmitBatch{
				Submitter:        testSubmitter,
				AppId:            1,
				Epoch:            1,
				RecordMerkleRoot: validRoot,
				RecordCount:      100,
				VerifierSetId:    1,
			},
			wantErr: false,
		},
		{
			name: "all-zeros merkle root rejected",
			msg: types.MsgSubmitBatch{
				Submitter:        testSubmitter,
				AppId:            1,
				Epoch:            1,
				RecordMerkleRoot: allZeros,
				RecordCount:      100,
				VerifierSetId:    1,
			},
			wantErr: true,
		},
		{
			name: "all-ones merkle root rejected",
			msg: types.MsgSubmitBatch{
				Submitter:        testSubmitter,
				AppId:            1,
				Epoch:            1,
				RecordMerkleRoot: allOnes,
				RecordCount:      100,
				VerifierSetId:    1,
			},
			wantErr: true,
		},
		{
			name: "zero record count",
			msg: types.MsgSubmitBatch{
				Submitter:        testSubmitter,
				AppId:            1,
				Epoch:            1,
				RecordMerkleRoot: validRoot,
				RecordCount:      0,
				VerifierSetId:    1,
			},
			wantErr: true,
		},
		{
			name: "wrong length merkle root",
			msg: types.MsgSubmitBatch{
				Submitter:        testSubmitter,
				AppId:            1,
				Epoch:            1,
				RecordMerkleRoot: []byte("short"),
				RecordCount:      100,
				VerifierSetId:    1,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// MsgServer Integration Tests
// ============================================================================

func TestMsgServer_RegisterApp(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	// Register an app
	resp, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner:           testOwner,
		Name:            "MyApp",
		SchemaCid:       "QmTestSchema",
		ChallengePeriod: 86400,
		MinVerifiers:    3,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.AppId)

	// Verify app stored
	app, found := f.keeper.GetApp(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, "MyApp", app.Name)
}

func TestMsgServer_RegisterApp_DuplicateName(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	msg := &types.MsgRegisterApp{
		Owner:           testOwner,
		Name:            "MyApp",
		SchemaCid:       "QmTestSchema",
		ChallengePeriod: 86400,
		MinVerifiers:    3,
	}

	_, err := ms.RegisterApp(f.ctx, msg)
	require.NoError(t, err)

	// Duplicate name should fail
	_, err = ms.RegisterApp(f.ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrDuplicateAppName)
}

func TestMsgServer_SubmitBatch(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	// Register test validators with sufficient bonded tokens
	v1Addr, _ := sdk.AccAddressFromBech32(testVerifier1)
	v2Addr, _ := sdk.AccAddressFromBech32(testVerifier2)
	v3Addr, _ := sdk.AccAddressFromBech32(testVerifier3)
	f.stakingKeeper.RegisterValidator(v1Addr, math.NewInt(100000000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v2Addr, math.NewInt(100000000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v3Addr, math.NewInt(100000000), stakingtypes.Bonded)

	// Register app first
	_, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner:           testOwner,
		Name:            "MyApp",
		SchemaCid:       "QmTestSchema",
		ChallengePeriod: 86400,
		MinVerifiers:    3,
	})
	require.NoError(t, err)

	// Create verifier set
	_, err = ms.CreateVerifierSet(f.ctx, &types.MsgCreateVerifierSet{
		Creator:         testOwner,
		AppId:           1,
		Epoch:           1,
		MinAttestations: 3,
		QuorumPct:       math.LegacyNewDecWithPrec(67, 2),
		Members: []types.VerifierMember{
			{Address: testVerifier1, Weight: math.NewInt(1000000)},
			{Address: testVerifier2, Weight: math.NewInt(1000000)},
			{Address: testVerifier3, Weight: math.NewInt(1000000)},
		},
	})
	require.NoError(t, err)

	// Submit batch
	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	resp, err := ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter:        testSubmitter,
		AppId:            1,
		Epoch:            1,
		RecordMerkleRoot: merkleRoot,
		RecordCount:      500,
		VerifierSetId:    1,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.BatchId)

	// Verify batch stored
	batch, found := f.keeper.GetBatch(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, types.BatchStatusSubmitted, batch.Status)
	require.Equal(t, uint64(500), batch.RecordCount)
}

func TestMsgServer_UpdateParams_AuthorityOnly(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	authority := f.keeper.GetAuthority()

	// Non-authority should fail
	_, err := ms.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: sdk.AccAddress("random____________").String(),
		Params:    types.DefaultParams(),
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidAuthority)

	// Authority should succeed
	newParams := types.DefaultParams()
	newParams.MaxBatchesPerBlock = 50
	_, err = ms.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	require.NoError(t, err)

	params := f.keeper.GetParams(f.ctx)
	require.Equal(t, uint32(50), params.MaxBatchesPerBlock)
}

// ============================================================================
// Query Server Tests
// ============================================================================

func TestQueryServer_Params(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	resp, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, types.DefaultMaxBatchesPerBlock, resp.Params.MaxBatchesPerBlock)
}

func TestQueryServer_App(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Not found
	_, err := qs.App(f.ctx, &types.QueryAppRequest{AppId: 1})
	require.Error(t, err)

	// Create app
	app := types.NewApp(1, "TestApp", testOwner, "QmTest", 86400, 3, time.Now().Unix())
	require.NoError(t, f.keeper.SetApp(f.ctx, app))

	// Found
	resp, err := qs.App(f.ctx, &types.QueryAppRequest{AppId: 1})
	require.NoError(t, err)
	require.Equal(t, "TestApp", resp.App.Name)
}

func TestQueryServer_Batches_StatusFilter(t *testing.T) {
	f := SetupKeeperTest(t)
	qs := keeper.NewQueryServerImpl(f.keeper)

	// Create batches with different statuses
	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	now := time.Now().Unix()
	batch1 := types.NewBatchCommitment(1, 1, merkleRoot, 100, 1, 1, testSubmitter, now+86400, now)
	require.NoError(t, f.keeper.SetBatch(f.ctx, batch1))

	batch2 := types.NewBatchCommitment(2, 1, merkleRoot, 200, 1, 1, testSubmitter, now+86400, now)
	require.NoError(t, f.keeper.SetBatch(f.ctx, batch2))
	require.NoError(t, f.keeper.UpdateBatchStatus(f.ctx, &batch2, types.BatchStatusPending))

	// All batches
	resp, err := qs.Batches(f.ctx, &types.QueryBatchesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Batches, 2)

	// Only submitted
	status := types.BatchStatusSubmitted
	resp, err = qs.Batches(f.ctx, &types.QueryBatchesRequest{Status: &status})
	require.NoError(t, err)
	require.Len(t, resp.Batches, 1)

	// Only pending
	status = types.BatchStatusPending
	resp, err = qs.Batches(f.ctx, &types.QueryBatchesRequest{Status: &status})
	require.NoError(t, err)
	require.Len(t, resp.Batches, 1)
}

// ============================================================================
// EndBlocker Tests
// ============================================================================

func TestEndBlocker_FinalizesExpiredBatches(t *testing.T) {
	f := SetupKeeperTest(t)

	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	// Create a PENDING batch with expired challenge window
	now := f.ctx.BlockTime().Unix()
	batch := types.NewBatchCommitment(1, 1, merkleRoot, 100, 1, 1, testSubmitter, now-100, now-200)
	require.NoError(t, f.keeper.SetBatch(f.ctx, batch))
	require.NoError(t, f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusPending))

	// Run EndBlocker
	sdkCtx := f.ctx
	err := f.keeper.EndBlocker(sdkCtx)
	require.NoError(t, err)

	// Verify batch is now FINALIZED
	got, found := f.keeper.GetBatch(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, types.BatchStatusFinalized, got.Status)
	require.Greater(t, got.FinalizedAt, int64(0))
}

func TestEndBlocker_SkipsBatchesWithOpenChallenges(t *testing.T) {
	f := SetupKeeperTest(t)

	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	// Create PENDING batch with expired challenge window
	now := f.ctx.BlockTime().Unix()
	batch := types.NewBatchCommitment(1, 1, merkleRoot, 100, 1, 1, testSubmitter, now-100, now-200)
	require.NoError(t, f.keeper.SetBatch(f.ctx, batch))
	require.NoError(t, f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusPending))

	// Add an open challenge
	challenge := types.NewChallenge(1, 1, testChallenger, types.ChallengeTypeInvalidRoot, []byte("proof"), now)
	require.NoError(t, f.keeper.SetChallenge(f.ctx, challenge))

	// Run EndBlocker
	err := f.keeper.EndBlocker(f.ctx)
	require.NoError(t, err)

	// Batch should still be PENDING (not finalized)
	got, found := f.keeper.GetBatch(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, types.BatchStatusPending, got.Status)
}

func TestEndBlocker_RespectsFinalizationCap(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set cap to 2
	params := f.keeper.GetParams(f.ctx)
	params.MaxFinalizationsPerBlock = 2
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	merkleRoot := make([]byte, 32)
	for i := range merkleRoot {
		merkleRoot[i] = byte(i + 1)
	}

	now := f.ctx.BlockTime().Unix()

	// Create 5 PENDING batches with expired windows
	for i := uint64(1); i <= 5; i++ {
		batch := types.NewBatchCommitment(i, 1, merkleRoot, 100, 1, 1, testSubmitter, now-100, now-200)
		require.NoError(t, f.keeper.SetBatch(f.ctx, batch))
		require.NoError(t, f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusPending))
	}

	// Run EndBlocker
	err := f.keeper.EndBlocker(f.ctx)
	require.NoError(t, err)

	// Only 2 should be finalized
	finalized := f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusFinalized)
	require.Equal(t, 2, len(finalized))

	// 3 should still be pending
	pending := f.keeper.GetBatchesByStatus(f.ctx, types.BatchStatusPending)
	require.Equal(t, 3, len(pending))
}

// ============================================================================
// Security Tests
// ============================================================================

func TestSecurity_MerkleRootValidation(t *testing.T) {
	// All zeros rejected
	allZeros := make([]byte, 32)
	msg := types.MsgSubmitBatch{
		Submitter:        testSubmitter,
		AppId:            1,
		Epoch:            1,
		RecordMerkleRoot: allZeros,
		RecordCount:      100,
		VerifierSetId:    1,
	}
	require.Error(t, msg.ValidateBasic())

	// All ones rejected
	allOnes := make([]byte, 32)
	for i := range allOnes {
		allOnes[i] = 0xFF
	}
	msg.RecordMerkleRoot = allOnes
	require.Error(t, msg.ValidateBasic())

	// Wrong length rejected
	msg.RecordMerkleRoot = []byte{1, 2, 3}
	require.Error(t, msg.ValidateBasic())

	// Valid root accepted
	validRoot := make([]byte, 32)
	for i := range validRoot {
		validRoot[i] = byte(i + 1)
	}
	msg.RecordMerkleRoot = validRoot
	require.NoError(t, msg.ValidateBasic())
}

func TestSecurity_DuplicateVerifierInSet(t *testing.T) {
	msg := types.MsgCreateVerifierSet{
		Creator:         testOwner,
		AppId:           1,
		Epoch:           1,
		MinAttestations: 3,
		QuorumPct:       math.LegacyNewDecWithPrec(67, 2),
		Members: []types.VerifierMember{
			{Address: testVerifier1, Weight: math.NewInt(1000000)},
			{Address: testVerifier1, Weight: math.NewInt(1000000)}, // duplicate!
			{Address: testVerifier2, Weight: math.NewInt(1000000)},
		},
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrDuplicateVerifier)
}

func TestSecurity_BatchIsTerminal(t *testing.T) {
	batch := types.BatchCommitment{Status: types.BatchStatusSubmitted}
	require.False(t, batch.IsTerminal())

	batch.Status = types.BatchStatusPending
	require.False(t, batch.IsTerminal())

	batch.Status = types.BatchStatusFinalized
	require.True(t, batch.IsTerminal())

	batch.Status = types.BatchStatusRejected
	require.True(t, batch.IsTerminal())
}

// ============================================================================
// Enum String Tests
// ============================================================================

func TestBatchStatus_String(t *testing.T) {
	require.Equal(t, "SUBMITTED", types.BatchStatusSubmitted.String())
	require.Equal(t, "PENDING", types.BatchStatusPending.String())
	require.Equal(t, "FINALIZED", types.BatchStatusFinalized.String())
	require.Equal(t, "REJECTED", types.BatchStatusRejected.String())
	require.Contains(t, types.BatchStatus(99).String(), "UNKNOWN")
}

func TestChallengeType_String(t *testing.T) {
	require.Equal(t, "INVALID_ROOT", types.ChallengeTypeInvalidRoot.String())
	require.Equal(t, "DOUBLE_INCLUSION", types.ChallengeTypeDoubleInclusion.String())
	require.Equal(t, "MISSING_RECORD", types.ChallengeTypeMissingRecord.String())
	require.Equal(t, "INVALID_SCHEMA", types.ChallengeTypeInvalidSchema.String())
}

func TestChallengeStatus_String(t *testing.T) {
	require.Equal(t, "OPEN", types.ChallengeStatusOpen.String())
	require.Equal(t, "RESOLVED_VALID", types.ChallengeStatusResolvedValid.String())
	require.Equal(t, "RESOLVED_INVALID", types.ChallengeStatusResolvedInvalid.String())
}

// ============================================================================
// Keeper Authority Tests
// ============================================================================

func TestKeeper_GetAuthority(t *testing.T) {
	f := SetupKeeperTest(t)
	authority := f.keeper.GetAuthority()
	require.NotEmpty(t, authority)

	// Verify it's a valid bech32 address
	_, err := sdk.AccAddressFromBech32(authority)
	require.NoError(t, err)
}
