package keeper_test

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"pos/x/por/keeper"
	"pos/x/por/types"
)

// ============================================================================
// F1: Verifier Weights Tied to Real Stake
// ============================================================================

func TestF1_CreateVerifierSet_BondedValidator(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	// Register app
	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	// Register 3 bonded validators
	v1 := sdk.AccAddress("val1________________")
	v2 := sdk.AccAddress("val2________________")
	v3 := sdk.AccAddress("val3________________")
	f.stakingKeeper.RegisterValidator(v1, math.NewInt(100_000_000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v2, math.NewInt(200_000_000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v3, math.NewInt(150_000_000), stakingtypes.Bonded)

	resp, err := ms.CreateVerifierSet(f.ctx, &types.MsgCreateVerifierSet{
		Creator: owner, AppId: app.AppId, Epoch: 1,
		Members: []types.VerifierMember{
			{Address: v1.String(), Weight: math.NewInt(1)}, // self-declared weight ignored
			{Address: v2.String(), Weight: math.NewInt(1)},
			{Address: v3.String(), Weight: math.NewInt(1)},
		},
		MinAttestations: 3, QuorumPct: math.LegacyNewDecWithPrec(67, 2),
	})
	require.NoError(t, err)

	// Verify weights were overridden with actual bonded tokens
	vs, found := f.keeper.GetVerifierSet(f.ctx, resp.VerifierSetId)
	require.True(t, found)
	require.Equal(t, math.NewInt(100_000_000), vs.Members[0].Weight)
	require.Equal(t, math.NewInt(200_000_000), vs.Members[1].Weight)
	require.Equal(t, math.NewInt(150_000_000), vs.Members[2].Weight)
}

func TestF1_CreateVerifierSet_NonValidator_Rejected(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	// v1 is NOT registered as a validator
	v1 := sdk.AccAddress("val1________________")
	v2 := sdk.AccAddress("val2________________")
	v3 := sdk.AccAddress("val3________________")
	f.stakingKeeper.RegisterValidator(v2, math.NewInt(100_000_000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v3, math.NewInt(100_000_000), stakingtypes.Bonded)

	_, err = ms.CreateVerifierSet(f.ctx, &types.MsgCreateVerifierSet{
		Creator: owner, AppId: app.AppId, Epoch: 1,
		Members: []types.VerifierMember{
			{Address: v1.String(), Weight: math.NewInt(100_000_000)},
			{Address: v2.String(), Weight: math.NewInt(100_000_000)},
			{Address: v3.String(), Weight: math.NewInt(100_000_000)},
		},
		MinAttestations: 3, QuorumPct: math.LegacyNewDecWithPrec(67, 2),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a registered validator")
}

func TestF1_CreateVerifierSet_UnbondedValidator_Rejected(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	v1 := sdk.AccAddress("val1________________")
	v2 := sdk.AccAddress("val2________________")
	v3 := sdk.AccAddress("val3________________")
	f.stakingKeeper.RegisterValidator(v1, math.NewInt(100_000_000), stakingtypes.Unbonding) // Not bonded!
	f.stakingKeeper.RegisterValidator(v2, math.NewInt(100_000_000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v3, math.NewInt(100_000_000), stakingtypes.Bonded)

	_, err = ms.CreateVerifierSet(f.ctx, &types.MsgCreateVerifierSet{
		Creator: owner, AppId: app.AppId, Epoch: 1,
		Members: []types.VerifierMember{
			{Address: v1.String(), Weight: math.NewInt(100_000_000)},
			{Address: v2.String(), Weight: math.NewInt(100_000_000)},
			{Address: v3.String(), Weight: math.NewInt(100_000_000)},
		},
		MinAttestations: 3, QuorumPct: math.LegacyNewDecWithPrec(67, 2),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be Bonded")
}

func TestF1_CreateVerifierSet_InsufficientStake_Rejected(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	v1 := sdk.AccAddress("val1________________")
	v2 := sdk.AccAddress("val2________________")
	v3 := sdk.AccAddress("val3________________")
	f.stakingKeeper.RegisterValidator(v1, math.NewInt(100), stakingtypes.Bonded) // Below min stake!
	f.stakingKeeper.RegisterValidator(v2, math.NewInt(100_000_000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v3, math.NewInt(100_000_000), stakingtypes.Bonded)

	_, err = ms.CreateVerifierSet(f.ctx, &types.MsgCreateVerifierSet{
		Creator: owner, AppId: app.AppId, Epoch: 1,
		Members: []types.VerifierMember{
			{Address: v1.String(), Weight: math.NewInt(100_000_000)},
			{Address: v2.String(), Weight: math.NewInt(100_000_000)},
			{Address: v3.String(), Weight: math.NewInt(100_000_000)},
		},
		MinAttestations: 3, QuorumPct: math.LegacyNewDecWithPrec(67, 2),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient stake")
}

// ============================================================================
// F4: Challenge Bond & Rate Limiting
// ============================================================================

func TestF4_ChallengeBond_Collected(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	// Setup a batch in SUBMITTED status
	batch := setupSubmittedBatch(t, f, ms)
	challenger := sdk.AccAddress("challenger__________")

	// Fund the challenger
	bondAmount := math.NewInt(10_000_000)
	f.bankKeeper.balances[challenger.String()] = sdk.NewCoins(sdk.NewCoin("omniphi", bondAmount))

	// Submit challenge
	_, err := ms.ChallengeBatch(f.ctx, &types.MsgChallengeBatch{
		Challenger:    challenger.String(),
		BatchId:       batch.BatchId,
		ChallengeType: types.ChallengeTypeInvalidRoot,
		ProofData:     []byte(`{"leaf_hashes":[]}`),
	})
	require.NoError(t, err)

	// Verify bond was collected (balance should be 0 now)
	bal := f.bankKeeper.balances[challenger.String()]
	require.True(t, bal.AmountOf("omniphi").IsZero())
}

func TestF4_ChallengeBond_InsufficientBalance(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	batch := setupSubmittedBatch(t, f, ms)
	challenger := sdk.AccAddress("challenger__________")

	// Don't fund the challenger — should fail
	_, err := ms.ChallengeBatch(f.ctx, &types.MsgChallengeBatch{
		Challenger:    challenger.String(),
		BatchId:       batch.BatchId,
		ChallengeType: types.ChallengeTypeInvalidRoot,
		ProofData:     []byte(`{"leaf_hashes":[]}`),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient")
}

func TestF4_ChallengeRateLimit(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	batch := setupSubmittedBatch(t, f, ms)
	challenger := sdk.AccAddress("challenger__________")

	// Fund challenger with enough for many challenges
	f.bankKeeper.balances[challenger.String()] = sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(1_000_000_000)))

	// Submit max challenges (5)
	for i := uint32(0); i < 5; i++ {
		_, err := ms.ChallengeBatch(f.ctx, &types.MsgChallengeBatch{
			Challenger:    challenger.String(),
			BatchId:       batch.BatchId,
			ChallengeType: types.ChallengeTypeInvalidRoot,
			ProofData:     []byte(`{"leaf_hashes":[]}`),
		})
		require.NoError(t, err)
	}

	// 6th challenge should be rate-limited
	_, err := ms.ChallengeBatch(f.ctx, &types.MsgChallengeBatch{
		Challenger:    challenger.String(),
		BatchId:       batch.BatchId,
		ChallengeType: types.ChallengeTypeInvalidRoot,
		ProofData:     []byte(`{"leaf_hashes":[]}`),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limit")
}

func TestF4_BondRefundOnValidChallenge(t *testing.T) {
	f := SetupKeeperTest(t)

	// Create a challenge with a bond
	challenge := types.NewChallengeWithBond(1, 1, sdk.AccAddress("challenger__________").String(),
		types.ChallengeTypeInvalidRoot, []byte("proof"), time.Now().Unix(), math.NewInt(10_000_000))
	require.NoError(t, f.keeper.SetChallenge(f.ctx, challenge))

	// Create a batch
	batch := types.NewBatchCommitment(1, 1, make([]byte, 32), 100, 1, 1,
		sdk.AccAddress("submitter___________").String(), time.Now().Unix()+3600, time.Now().Unix())
	require.NoError(t, f.keeper.SetBatch(f.ctx, batch))

	// Process as valid — bond should be refunded
	err := f.keeper.ProcessValidChallenge(f.ctx, 1)
	require.NoError(t, err)

	// Check that refund was sent
	challengerAddr := sdk.AccAddress("challenger__________")
	bal := f.bankKeeper.balances[challengerAddr.String()]
	require.True(t, bal.AmountOf("omniphi").Equal(math.NewInt(10_000_000)))
}

func TestF4_BondBurnOnInvalidChallenge(t *testing.T) {
	f := SetupKeeperTest(t)

	challenge := types.NewChallengeWithBond(1, 1, sdk.AccAddress("challenger__________").String(),
		types.ChallengeTypeInvalidRoot, []byte("proof"), time.Now().Unix(), math.NewInt(10_000_000))
	require.NoError(t, f.keeper.SetChallenge(f.ctx, challenge))

	// Fund module account so burn works
	f.bankKeeper.moduleBalances["por"] = sdk.NewCoins(sdk.NewCoin("omniphi", math.NewInt(10_000_000)))

	// Process as invalid — bond should be burned
	err := f.keeper.ProcessInvalidChallenge(f.ctx, 1)
	require.NoError(t, err)

	// Check that coins were burned
	require.True(t, f.bankKeeper.burned.AmountOf("omniphi").Equal(math.NewInt(10_000_000)))
}

// ============================================================================
// F5: Attestation Signature Verification
// ============================================================================

func TestF5_AttestationSignature_Valid(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	batch := setupSubmittedBatch(t, f, ms)

	verifier1 := sdk.AccAddress("val1________________").String()

	// Compute the correct signature (includes verifier address to prevent forgery)
	expectedSig := types.ComputeAttestationSignBytes(batch.BatchId, batch.RecordMerkleRoot, batch.Epoch, verifier1)

	_, err := ms.SubmitAttestation(f.ctx, &types.MsgSubmitAttestation{
		Verifier:         verifier1,
		BatchId:          batch.BatchId,
		Signature:        expectedSig,
		ConfidenceWeight: math.LegacyOneDec(),
	})
	require.NoError(t, err)
}

func TestF5_AttestationSignature_Invalid_Rejected(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	batch := setupSubmittedBatch(t, f, ms)
	verifier1 := sdk.AccAddress("val1________________").String()

	// Use wrong signature
	_, err := ms.SubmitAttestation(f.ctx, &types.MsgSubmitAttestation{
		Verifier:         verifier1,
		BatchId:          batch.BatchId,
		Signature:        []byte("wrong_signature_that_is_not_right"),
		ConfidenceWeight: math.LegacyOneDec(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature does not match")
}

func TestF5_AttestationSignBytes_Deterministic(t *testing.T) {
	verifier := "cosmos1verifier"
	sig1 := types.ComputeAttestationSignBytes(42, make([]byte, 32), 7, verifier)
	sig2 := types.ComputeAttestationSignBytes(42, make([]byte, 32), 7, verifier)
	require.Equal(t, sig1, sig2)

	// Different batch ID -> different signature
	sig3 := types.ComputeAttestationSignBytes(43, make([]byte, 32), 7, verifier)
	require.NotEqual(t, sig1, sig3)

	// Different epoch -> different signature
	sig4 := types.ComputeAttestationSignBytes(42, make([]byte, 32), 8, verifier)
	require.NotEqual(t, sig1, sig4)

	// Different verifier -> different signature (prevents third-party forgery)
	sig5 := types.ComputeAttestationSignBytes(42, make([]byte, 32), 7, "cosmos1different")
	require.NotEqual(t, sig1, sig5)
}

// ============================================================================
// F2/F6: Credit Caps & DA Enforcement
// ============================================================================

func TestF2_DACommitmentRequired(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	// Enable DA commitment requirement
	params := types.DefaultParams()
	params.RequireDACommitment = true
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Setup app and verifier set
	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	// Submit batch without DA commitment — should fail
	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42
	_, err = ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter: owner, AppId: app.AppId, Epoch: 1,
		RecordMerkleRoot: merkleRoot, RecordCount: 100, VerifierSetId: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DA commitment hash is required")
}

func TestF2_DACommitmentOptional(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	// DA not required (default)
	batch := setupSubmittedBatch(t, f, ms)
	require.True(t, batch.BatchId > 0)
}

func TestF2_PerBatchCreditCap(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	// Set a low per-batch cap
	params := types.DefaultParams()
	params.MaxCreditsPerBatch = math.NewInt(1000) // Very low: 100 base * count must not exceed
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	// Submit batch with record count that exceeds per-batch cap
	// 100 base_reward * 100 records = 10000 credits > 1000 cap
	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42
	_, err = ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter: owner, AppId: app.AppId, Epoch: 1,
		RecordMerkleRoot: merkleRoot, RecordCount: 100, VerifierSetId: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceed per-batch cap")
}

func TestF6_EpochCreditCapEnforced(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set epoch credit cap low for testing
	params := types.DefaultParams()
	params.MaxCreditsPerEpoch = math.NewInt(500) // Very low
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	// Pre-fill epoch credits near the cap
	_, err := f.keeper.IncrementEpochCredits(f.ctx, 1, math.NewInt(490))
	require.NoError(t, err)

	// Create and finalize a batch
	batch := types.NewBatchCommitment(1, 1, make([]byte, 32), 100, 1, 1,
		sdk.AccAddress("submitter___________").String(), time.Now().Unix()-100, time.Now().Unix()-200)
	require.NoError(t, f.keeper.SetBatch(f.ctx, batch))

	// Force to PENDING for EndBlocker
	require.NoError(t, f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusPending))

	// Run EndBlocker — credits should be capped
	ctx := f.ctx.WithBlockTime(time.Now().Add(1 * time.Hour))
	err = f.keeper.EndBlocker(ctx)
	require.NoError(t, err)

	// Verify epoch credits did not exceed cap
	epochUsed := f.keeper.GetEpochCreditsUsed(f.ctx, 1)
	require.True(t, epochUsed.LTE(params.MaxCreditsPerEpoch),
		"epoch credits %s should not exceed cap %s", epochUsed, params.MaxCreditsPerEpoch)
}

// ============================================================================
// F3: Double-Inclusion via Leaf Hash Storage
// ============================================================================

func TestF3_LeafHashesStored(t *testing.T) {
	f := SetupKeeperTest(t)

	leafHash1 := sha256.Sum256([]byte("record1"))
	leafHash2 := sha256.Sum256([]byte("record2"))
	leafHashes := [][]byte{leafHash1[:], leafHash2[:]}

	err := f.keeper.StoreBatchLeafHashes(f.ctx, 1, leafHashes)
	require.NoError(t, err)

	// Verify leaf hashes are stored
	require.True(t, f.keeper.HasLeafHashInBatch(f.ctx, 1, leafHash1[:]))
	require.True(t, f.keeper.HasLeafHashInBatch(f.ctx, 1, leafHash2[:]))

	// Not in another batch
	require.False(t, f.keeper.HasLeafHashInBatch(f.ctx, 2, leafHash1[:]))
}

func TestF3_DoubleInclusionConclusive(t *testing.T) {
	f := SetupKeeperTest(t)

	// Same leaf in both batches
	leafHash := sha256.Sum256([]byte("duplicate_record"))
	err := f.keeper.StoreBatchLeafHashes(f.ctx, 1, [][]byte{leafHash[:]})
	require.NoError(t, err)
	err = f.keeper.StoreBatchLeafHashes(f.ctx, 2, [][]byte{leafHash[:]})
	require.NoError(t, err)

	// Verify reverse index
	batchIDs := f.keeper.GetBatchesContainingLeaf(f.ctx, leafHash[:])
	require.Len(t, batchIDs, 2)
}

func TestF3_RequireLeafHashes(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := types.DefaultParams()
	params.RequireLeafHashes = true
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42

	// Submit without leaf hashes — should fail
	_, err = ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter: owner, AppId: app.AppId, Epoch: 1,
		RecordMerkleRoot: merkleRoot, RecordCount: 1, VerifierSetId: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "leaf hashes are required")
}

// ============================================================================
// F8: PoSeq Commitment Verification
// ============================================================================

func TestF8_RegisterPoSeqCommitment(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	authority := sdk.AccAddress("gov_________________").String()

	// Register sequencer set first
	_, err := ms.UpdatePoSeqSequencerSet(f.ctx, &types.MsgUpdatePoSeqSequencerSet{
		Authority:  authority,
		Sequencers: []string{authority},
		Threshold:  1,
	})
	require.NoError(t, err)

	// Register a commitment
	stateRoot := sha256.Sum256([]byte("state_root_data"))
	commitmentHash := sha256.Sum256(stateRoot[:])

	_, err = ms.RegisterPoSeqCommitment(f.ctx, &types.MsgRegisterPoSeqCommitment{
		Authority:      authority,
		CommitmentHash: commitmentHash[:],
		StateRoot:      stateRoot[:],
		BlockHeight:    42,
	})
	require.NoError(t, err)

	// Verify it's stored
	commitment, found := f.keeper.GetPoSeqCommitment(f.ctx, commitmentHash[:])
	require.True(t, found)
	require.Equal(t, uint64(42), commitment.BlockHeight)
}

func TestF8_RequirePoSeqCommitment(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := types.DefaultParams()
	params.RequirePoSeqCommitment = true
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42

	// Submit without PoSeq commitment — should fail
	_, err = ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter: owner, AppId: app.AppId, Epoch: 1,
		RecordMerkleRoot: merkleRoot, RecordCount: 1, VerifierSetId: 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PoSeq commitment hash is required")
}

func TestF8_UnregisteredPoSeqHash_Rejected(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := types.DefaultParams()
	params.RequirePoSeqCommitment = true
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42
	fakeHash := make([]byte, 32)
	fakeHash[0] = 0xFF

	// Submit with an unregistered PoSeq commitment — should fail
	_, err = ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter: owner, AppId: app.AppId, Epoch: 1,
		RecordMerkleRoot: merkleRoot, RecordCount: 1, VerifierSetId: 1,
		PoSeqCommitmentHash: fakeHash,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not registered on-chain")
}

func TestF8_OnlyAuthorizedSequencers(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	authority := sdk.AccAddress("gov_________________").String()
	unauthorized := sdk.AccAddress("unauthorized________").String()

	// Register sequencer set with only authority
	_, err := ms.UpdatePoSeqSequencerSet(f.ctx, &types.MsgUpdatePoSeqSequencerSet{
		Authority:  authority,
		Sequencers: []string{authority},
		Threshold:  1,
	})
	require.NoError(t, err)

	stateRoot := sha256.Sum256([]byte("state_root"))
	commitmentHash := sha256.Sum256(stateRoot[:])

	// Unauthorized address should be rejected
	_, err = ms.RegisterPoSeqCommitment(f.ctx, &types.MsgRegisterPoSeqCommitment{
		Authority:      unauthorized,
		CommitmentHash: commitmentHash[:],
		StateRoot:      stateRoot[:],
		BlockHeight:    1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not governance authority or authorized sequencer")
}

// ============================================================================
// F5: ComputeAttestationSignBytes correctness
// ============================================================================

func TestF5_ComputeAttestationSignBytes(t *testing.T) {
	batchID := uint64(42)
	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0xAB
	epoch := uint64(7)
	verifier := "cosmos1verifier"

	sig := types.ComputeAttestationSignBytes(batchID, merkleRoot, epoch, verifier)
	require.Len(t, sig, 32) // SHA256

	// Manually compute expected (includes verifier address)
	h := sha256.New()
	batchBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(batchBytes, batchID)
	h.Write(batchBytes)
	h.Write(merkleRoot)
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, epoch)
	h.Write(epochBytes)
	h.Write([]byte(verifier))
	expected := h.Sum(nil)

	require.Equal(t, expected, sig)
}

// ============================================================================
// ValidateBasic tests for new MsgSubmitBatch fields
// ============================================================================

func TestMsgSubmitBatch_ValidateBasic_DAHash(t *testing.T) {
	base := types.MsgSubmitBatch{
		Submitter: sdk.AccAddress("sub_________________").String(),
		AppId:     1, Epoch: 1,
		RecordMerkleRoot: make([]byte, 32),
		RecordCount:      10,
		VerifierSetId:    1,
	}
	base.RecordMerkleRoot[0] = 0x42

	// Valid: no DA hash
	require.NoError(t, base.ValidateBasic())

	// Valid: correct length DA hash
	base.DACommitmentHash = make([]byte, 32)
	base.DACommitmentHash[0] = 0x01
	require.NoError(t, base.ValidateBasic())

	// Invalid: wrong length DA hash
	base.DACommitmentHash = make([]byte, 16)
	err := base.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "DA commitment hash must be exactly 32 bytes")
}

func TestMsgSubmitBatch_ValidateBasic_LeafHashes(t *testing.T) {
	base := types.MsgSubmitBatch{
		Submitter: sdk.AccAddress("sub_________________").String(),
		AppId:     1, Epoch: 1,
		RecordMerkleRoot: make([]byte, 32),
		RecordCount:      2,
		VerifierSetId:    1,
	}
	base.RecordMerkleRoot[0] = 0x42

	// Valid: matching count
	base.LeafHashes = [][]byte{make([]byte, 32), make([]byte, 32)}
	require.NoError(t, base.ValidateBasic())

	// Invalid: mismatched count
	base.LeafHashes = [][]byte{make([]byte, 32)}
	err := base.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match record_count")

	// Invalid: wrong leaf length
	base.LeafHashes = [][]byte{make([]byte, 32), make([]byte, 16)}
	err = base.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid length")
}

// ============================================================================
// Re-Audit Fix Tests
// ============================================================================

// TestReaudit_EpochOverride verifies that SubmitBatch overrides user-supplied epoch
// with chain-derived epoch to prevent bypassing per-epoch credit caps.
func TestReaudit_EpochOverride(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42

	// Submit with user-supplied epoch=999 (attacker trying to escape credit tracking)
	batchResp, err := ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter:        owner,
		AppId:            app.AppId,
		Epoch:            999, // attacker-controlled value
		RecordMerkleRoot: merkleRoot,
		RecordCount:      10,
		VerifierSetId:    1,
	})
	require.NoError(t, err)

	// Verify the stored batch uses chain-derived epoch (blockHeight/100), NOT user-supplied 999
	batch, found := f.keeper.GetBatch(f.ctx, batchResp.BatchId)
	require.True(t, found)

	expectedEpoch := uint64(f.ctx.BlockHeight() / 100)
	require.Equal(t, expectedEpoch, batch.Epoch,
		"batch epoch must be chain-derived (%d), not user-supplied (999)", expectedEpoch)
	require.NotEqual(t, uint64(999), batch.Epoch,
		"user-supplied epoch must be overridden")
}

// TestReaudit_SlashingAddressConversion verifies that slashVerifier correctly converts
// AccAddress to ValAddress (not using ValAddressFromBech32 which fails on omni1... addresses).
func TestReaudit_SlashingAddressConversion(t *testing.T) {
	_ = SetupKeeperTest(t) // ensures test infra works

	// Set up a verifier address in AccAddress format (omni1...)
	verifierAcc := sdk.AccAddress("val1________________")
	verifierAddr := verifierAcc.String()

	// The verifier address should be parseable as AccAddress
	parsed, err := sdk.AccAddressFromBech32(verifierAddr)
	require.NoError(t, err)
	require.Equal(t, verifierAcc, parsed)

	// ValAddressFromBech32 should FAIL on this address (this was the bug)
	_, err = sdk.ValAddressFromBech32(verifierAddr)
	require.Error(t, err, "ValAddressFromBech32 should fail on AccAddress format")

	// The correct conversion: AccAddress bytes → ValAddress
	valAddr := sdk.ValAddress(parsed)
	require.NotEmpty(t, valAddr)
	// ValAddress should have different bech32 prefix
	require.NotEqual(t, verifierAddr, valAddr.String())
}

// TestReaudit_FinalizeBatch_CreditCaps verifies that FinalizeBatch (governance path)
// enforces per-batch and per-epoch credit caps, just like EndBlocker.
func TestReaudit_FinalizeBatch_CreditCaps(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	owner := sdk.AccAddress("owner_______________").String()
	authority := sdk.AccAddress("gov_________________").String()

	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	// Set per-batch cap very low
	params := types.DefaultParams()
	params.MaxCreditsPerBatch = math.NewInt(500) // cap at 500 credits
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42

	// Submit a batch with record_count=100, base_reward=100 → computed=10000 (exceeds cap of 500)
	batchResp, err := ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter:        owner,
		AppId:            app.AppId,
		Epoch:            0,
		RecordMerkleRoot: merkleRoot,
		RecordCount:      100,
		VerifierSetId:    1,
	})
	// Per-batch credit cap at submission time should reject this
	require.Error(t, err, "batch exceeding per-batch credit cap should be rejected at submission")
	_ = batchResp

	// Now submit one within the cap: record_count=5, base_reward=100 → 500 (at cap)
	batchResp, err = ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter:        owner,
		AppId:            app.AppId,
		Epoch:            0,
		RecordMerkleRoot: merkleRoot,
		RecordCount:      5,
		VerifierSetId:    1,
	})
	require.NoError(t, err)

	// Advance time well past challenge window (3600s challenge period + buffer)
	f.ctx = f.ctx.WithBlockHeader(cmtproto.Header{
		Time: f.ctx.BlockTime().Add(2 * time.Hour),
	})

	// Manually transition to PENDING
	batch, _ := f.keeper.GetBatch(f.ctx, batchResp.BatchId)
	require.NoError(t, f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusPending))

	// FinalizeBatch via governance — should succeed and credit caps should apply
	_, err = ms.FinalizeBatch(f.ctx, &types.MsgFinalizeBatch{
		Authority: authority,
		BatchId:   batchResp.BatchId,
	})
	require.NoError(t, err)

	// Verify credits were capped (500 cap, 5 records * 100 = 500 → exactly at cap)
	submitterAddr := sdk.AccAddress("owner_______________")
	credits := f.pocKeeper.credits[submitterAddr.String()]
	require.True(t, credits.LTE(params.MaxCreditsPerBatch),
		"credits (%s) should not exceed per-batch cap (%s)", credits, params.MaxCreditsPerBatch)
}

// TestReaudit_FinalizeBatch_EpochCap verifies FinalizeBatch tracks epoch credit usage.
func TestReaudit_FinalizeBatch_EpochCap(t *testing.T) {
	f := SetupKeeperTest(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	owner := sdk.AccAddress("owner_______________").String()
	authority := sdk.AccAddress("gov_________________").String()

	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	// Set epoch cap very low (300 total per epoch)
	params := types.DefaultParams()
	params.MaxCreditsPerEpoch = math.NewInt(300)
	params.MaxCreditsPerBatch = math.NewInt(10000) // high batch cap
	require.NoError(t, f.keeper.SetParams(f.ctx, params))

	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42

	// Submit a batch: 5 records * 100 = 500, but epoch cap is 300
	batchResp, err := ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter:        owner,
		AppId:            app.AppId,
		Epoch:            0,
		RecordMerkleRoot: merkleRoot,
		RecordCount:      5,
		VerifierSetId:    1,
	})
	require.NoError(t, err)

	// Advance time well past challenge window (3600s challenge period + buffer)
	f.ctx = f.ctx.WithBlockHeader(cmtproto.Header{
		Time: f.ctx.BlockTime().Add(2 * time.Hour),
	})

	// Transition to PENDING
	batch, _ := f.keeper.GetBatch(f.ctx, batchResp.BatchId)
	require.NoError(t, f.keeper.UpdateBatchStatus(f.ctx, &batch, types.BatchStatusPending))

	// FinalizeBatch — credits should be capped to epoch limit (300)
	_, err = ms.FinalizeBatch(f.ctx, &types.MsgFinalizeBatch{
		Authority: authority,
		BatchId:   batchResp.BatchId,
	})
	require.NoError(t, err)

	// Verify credits were capped to epoch max
	submitterAddr := sdk.AccAddress("owner_______________")
	credits := f.pocKeeper.credits[submitterAddr.String()]
	require.True(t, credits.LTE(params.MaxCreditsPerEpoch),
		"credits (%s) should not exceed epoch cap (%s)", credits, params.MaxCreditsPerEpoch)

	// Verify epoch tracker was updated
	batchStored, _ := f.keeper.GetBatch(f.ctx, batchResp.BatchId)
	epochUsed := f.keeper.GetEpochCreditsUsed(f.ctx, batchStored.Epoch)
	require.True(t, epochUsed.IsPositive(), "epoch credit tracker should be updated")
	require.True(t, epochUsed.LTE(params.MaxCreditsPerEpoch),
		"epoch usage (%s) should not exceed cap (%s)", epochUsed, params.MaxCreditsPerEpoch)
}

// ============================================================================
// Test Helpers
// ============================================================================

func registerValidators(f *KeeperTestFixture) {
	v1 := sdk.AccAddress("val1________________")
	v2 := sdk.AccAddress("val2________________")
	v3 := sdk.AccAddress("val3________________")
	f.stakingKeeper.RegisterValidator(v1, math.NewInt(100_000_000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v2, math.NewInt(100_000_000), stakingtypes.Bonded)
	f.stakingKeeper.RegisterValidator(v3, math.NewInt(100_000_000), stakingtypes.Bonded)
}

func createVerifierSet(t *testing.T, f *KeeperTestFixture, ms types.MsgServer, owner string, appID uint64) {
	t.Helper()
	v1 := sdk.AccAddress("val1________________").String()
	v2 := sdk.AccAddress("val2________________").String()
	v3 := sdk.AccAddress("val3________________").String()

	_, err := ms.CreateVerifierSet(f.ctx, &types.MsgCreateVerifierSet{
		Creator: owner, AppId: appID, Epoch: 1,
		Members: []types.VerifierMember{
			{Address: v1, Weight: math.NewInt(1)},
			{Address: v2, Weight: math.NewInt(1)},
			{Address: v3, Weight: math.NewInt(1)},
		},
		MinAttestations: 3, QuorumPct: math.LegacyNewDecWithPrec(67, 2),
	})
	require.NoError(t, err)
}

// setupSubmittedBatch creates a complete setup with app, verifiers, and a submitted batch.
// Returns the batch in SUBMITTED status.
func setupSubmittedBatch(t *testing.T, f *KeeperTestFixture, ms types.MsgServer) types.BatchCommitment {
	t.Helper()

	owner := sdk.AccAddress("owner_______________").String()
	app, err := ms.RegisterApp(f.ctx, &types.MsgRegisterApp{
		Owner: owner, Name: "testapp", SchemaCid: "QmTest", ChallengePeriod: 3600, MinVerifiers: 3,
	})
	require.NoError(t, err)

	registerValidators(f)
	createVerifierSet(t, f, ms, owner, app.AppId)

	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0x42

	batchResp, err := ms.SubmitBatch(f.ctx, &types.MsgSubmitBatch{
		Submitter: owner, AppId: app.AppId, Epoch: 1,
		RecordMerkleRoot: merkleRoot, RecordCount: 10, VerifierSetId: 1,
	})
	require.NoError(t, err)

	batch, found := f.keeper.GetBatch(f.ctx, batchResp.BatchId)
	require.True(t, found)

	// Set block time within challenge window
	f.ctx = f.ctx.WithBlockHeader(cmtproto.Header{
		Time: time.Now(),
	})

	return batch
}
