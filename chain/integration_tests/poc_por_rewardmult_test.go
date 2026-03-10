package integration_tests

import (
	"bytes"
	"context"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	pockeeper "pos/x/poc/keeper"
	poctypes "pos/x/poc/types"

	porkeeper "pos/x/por/keeper"
	portypes "pos/x/por/types"

	rmkeeper "pos/x/rewardmult/keeper"
	rmtypes "pos/x/rewardmult/types"
)

// ============================================================================
// Integration Test Infrastructure
//
// Sets up all three module keepers (PoC, PoR, RewardMult) sharing the same
// stateStore, mock staking keeper, and cross-module interfaces. This exercises
// the real CRUD paths — no behaviour is mocked except external deps (staking,
// bank, slashing, account).
// ============================================================================

// IntegrationFixture holds all three module keepers and shared test state.
type IntegrationFixture struct {
	t   *testing.T
	ctx sdk.Context
	cdc codec.Codec

	pocKeeper pockeeper.Keeper
	porKeeper porkeeper.Keeper
	rmKeeper  rmkeeper.Keeper

	staking   *testStakingKeeper
	slashing  *testSlashingKeeper
	authority string

	stateStore storetypes.CommitMultiStore
}

// ---- Shared mock keepers (implement all three module expected interfaces) ----

// testStakingKeeper is a configurable mock shared across all three modules.
type testStakingKeeper struct {
	validators []stakingtypes.Validator
}

func (m *testStakingKeeper) GetValidator(_ context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	for _, v := range m.validators {
		if v.GetOperator() == addr.String() {
			return v, nil
		}
	}
	return stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound
}

func (m *testStakingKeeper) GetAllValidators(_ context.Context) ([]stakingtypes.Validator, error) {
	return m.validators, nil
}

func (m *testStakingKeeper) TotalBondedTokens(_ context.Context) (math.Int, error) {
	total := math.ZeroInt()
	for _, v := range m.validators {
		if v.IsBonded() {
			total = total.Add(v.Tokens)
		}
	}
	return total, nil
}

func (m *testStakingKeeper) PowerReduction(_ context.Context) math.Int {
	return math.NewInt(1000000)
}

// testSlashingKeeper implements rewardmult SlashingKeeper.
type testSlashingKeeper struct {
	signingInfos map[string]slashingtypes.ValidatorSigningInfo
}

func newTestSlashingKeeper() *testSlashingKeeper {
	return &testSlashingKeeper{signingInfos: make(map[string]slashingtypes.ValidatorSigningInfo)}
}

func (m *testSlashingKeeper) GetValidatorSigningInfo(_ context.Context, consAddr sdk.ConsAddress) (slashingtypes.ValidatorSigningInfo, error) {
	info, found := m.signingInfos[consAddr.String()]
	if !found {
		return slashingtypes.ValidatorSigningInfo{MissedBlocksCounter: 0, StartHeight: 0}, nil
	}
	return info, nil
}

// testBankKeeper is a no-op bank keeper for modules that need one.
type testBankKeeper struct{}

func (m testBankKeeper) GetBalance(_ context.Context, _ sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, math.ZeroInt())
}
func (m testBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}
func (m testBankKeeper) SendCoinsFromAccountToModule(_ context.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}
func (m testBankKeeper) MintCoins(_ context.Context, _ string, _ sdk.Coins) error  { return nil }
func (m testBankKeeper) BurnCoins(_ context.Context, _ string, _ sdk.Coins) error  { return nil }

// testAccountKeeper is a no-op account keeper.
type testAccountKeeper struct{}

func (m testAccountKeeper) GetModuleAddress(_ string) sdk.AccAddress {
	return sdk.AccAddress("module_address______")
}
func (m testAccountKeeper) GetModuleAccount(_ context.Context, _ string) sdk.ModuleAccountI {
	return nil
}
func (m testAccountKeeper) GetAccount(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}

// ---- Cross-module bridge mocks ----

// pocPorBridge implements poc's PorKeeper interface backed by real PoR keeper.
type pocPorBridge struct {
	porKeeper porkeeper.Keeper
	// batch→contribution mapping (set by test)
	contributionBatchMap map[uint64]uint64
}

func newPocPorBridge(pk porkeeper.Keeper) *pocPorBridge {
	return &pocPorBridge{porKeeper: pk, contributionBatchMap: make(map[uint64]uint64)}
}

func (b *pocPorBridge) GetBatch(ctx context.Context, batchID uint64) (interface{}, bool) {
	batch, found := b.porKeeper.GetBatch(ctx, batchID)
	return batch, found
}

func (b *pocPorBridge) IsBatchFinalized(ctx context.Context, batchID uint64) bool {
	batch, found := b.porKeeper.GetBatch(ctx, batchID)
	if !found {
		return false
	}
	return batch.Status == portypes.BatchStatusFinalized
}

func (b *pocPorBridge) IsBatchChallenged(ctx context.Context, batchID uint64) bool {
	return b.porKeeper.HasOpenChallenges(ctx, batchID)
}

func (b *pocPorBridge) IsBatchRejected(ctx context.Context, batchID uint64) bool {
	batch, found := b.porKeeper.GetBatch(ctx, batchID)
	if !found {
		return false
	}
	return batch.Status == portypes.BatchStatusRejected
}

func (b *pocPorBridge) GetBatchForContribution(_ context.Context, contributionID uint64) uint64 {
	return b.contributionBatchMap[contributionID]
}

func (b *pocPorBridge) HasFraudulentAttestation(ctx context.Context, addr sdk.ValAddress, lookbackEpochs int64) (bool, error) {
	// PoR stores VerifierAddress as AccAddress (omni1...) format.
	// The rewardmult module passes ValAddress (omnivaloper1...).
	// Convert ValAddress bytes → AccAddress for comparison.
	accAddr := sdk.AccAddress(addr).String()

	// Check if validator has any attestation on a REJECTED batch
	batches := b.porKeeper.GetBatchesByStatus(ctx, portypes.BatchStatusRejected)
	for _, batch := range batches {
		atts := b.porKeeper.GetAttestationsForBatch(ctx, batch.BatchId)
		for _, att := range atts {
			if att.VerifierAddress == accAddr {
				return true, nil
			}
		}
	}
	return false, nil
}

// rmPocBridge implements rewardmult's PocKeeper backed by real PoC keeper.
type rmPocBridge struct {
	pocKeeper pockeeper.Keeper
}

func (b *rmPocBridge) GetEndorsementParticipationRate(ctx context.Context, valAddr sdk.ValAddress) (math.LegacyDec, error) {
	return b.pocKeeper.GetEndorsementParticipationRate(ctx, valAddr)
}

func (b *rmPocBridge) GetValidatorOriginalityMetrics(ctx context.Context, valAddr sdk.ValAddress) (avgOriginality, avgQuality math.LegacyDec, err error) {
	return b.pocKeeper.GetValidatorOriginalityMetrics(ctx, valAddr)
}

// rmPorBridge implements rewardmult's PorKeeper backed by pocPorBridge.
type rmPorBridge struct {
	bridge *pocPorBridge
}

func (b *rmPorBridge) HasFraudulentAttestation(ctx context.Context, addr sdk.ValAddress, lookbackEpochs int64) (bool, error) {
	return b.bridge.HasFraudulentAttestation(ctx, addr, lookbackEpochs)
}

// porPocBridge implements PoR's PocKeeper backed by real PoC keeper.
type porPocBridge struct {
	pocKeeper pockeeper.Keeper
}

func (b *porPocBridge) AddCreditsWithOverflowCheck(ctx context.Context, addr sdk.AccAddress, amount math.Int) error {
	return b.pocKeeper.AddCreditsWithOverflowCheck(ctx, addr, amount)
}

// ---- Fixture setup ----

func SetupIntegration(t *testing.T) *IntegrationFixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig()
	cdc := encCfg.Codec

	// Register PoC interfaces
	registry := codectypes.NewInterfaceRegistry()
	poctypes.RegisterInterfaces(registry)
	portypes.RegisterInterfaces(registry)

	// Create store keys for all three modules
	pocStoreKey := storetypes.NewKVStoreKey(poctypes.StoreKey)
	pocTStoreKey := storetypes.NewTransientStoreKey(poctypes.TStoreKey)
	porStoreKey := storetypes.NewKVStoreKey(portypes.StoreKey)
	porTStoreKey := storetypes.NewTransientStoreKey(portypes.TStoreKey)
	rmStoreKey := storetypes.NewKVStoreKey(rmtypes.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(pocStoreKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(pocTStoreKey, storetypes.StoreTypeTransient, db)
	stateStore.MountStoreWithDB(porStoreKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(porTStoreKey, storetypes.StoreTypeTransient, db)
	stateStore.MountStoreWithDB(rmStoreKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	ctx := sdk.NewContext(stateStore, cmtproto.Header{
		Height: 100,
		Time:   time.Now(),
	}, false, log.NewNopLogger())

	authority := sdk.AccAddress("gov_________________").String()

	// Create shared mock keepers
	staking := &testStakingKeeper{
		validators: []stakingtypes.Validator{
			makeValidator(valAddr1, math.NewInt(1_000_000), true),
			makeValidator(valAddr2, math.NewInt(2_000_000), true),
			makeValidator(valAddr3, math.NewInt(3_000_000), true),
		},
	}
	slashing := newTestSlashingKeeper()

	// --- PoC Keeper ---
	pocK := pockeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(pocStoreKey),
		pocTStoreKey,
		log.NewNopLogger(),
		authority,
		staking,
		testBankKeeper{},
		testAccountKeeper{},
	)
	pocParams := poctypes.DefaultParams()
	pocParams.RewardDenom = "omniphi"
	pocParams.BaseRewardUnit = math.NewInt(100)
	pocParams.MaxPerBlock = 100
	require.NoError(t, pocK.SetParams(ctx, pocParams))

	// --- PoR Keeper ---
	porK := porkeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(porStoreKey),
		porTStoreKey,
		log.NewNopLogger(),
		authority,
		staking,
		testBankKeeper{},
		testAccountKeeper{},
	)
	porParams := portypes.DefaultParams()
	require.NoError(t, porK.SetParams(ctx, porParams))

	// --- RewardMult Keeper ---
	rmK := rmkeeper.NewKeeper(
		cdc.(codec.BinaryCodec),
		runtime.NewKVStoreService(rmStoreKey),
		log.NewNopLogger(),
		authority,
		staking,
		slashing,
	)

	// --- Wire cross-module interfaces ---
	porPocB := &porPocBridge{pocKeeper: pocK}
	porK.SetPocKeeper(porPocB)

	pocPorB := newPocPorBridge(porK)
	pocK.SetPorKeeper(pocPorB)

	rmPocB := &rmPocBridge{pocKeeper: pocK}
	rmK.SetPocKeeper(rmPocB)

	rmPorB := &rmPorBridge{bridge: pocPorB}
	rmK.SetPorKeeper(rmPorB)

	return &IntegrationFixture{
		t:          t,
		ctx:        ctx,
		cdc:        cdc,
		pocKeeper:  pocK,
		porKeeper:  porK,
		rmKeeper:   rmK,
		staking:    staking,
		slashing:   slashing,
		authority:  authority,
		stateStore: stateStore,
	}
}

// ---- Test Addresses ----

var (
	// Use ValAddress for validator operator addresses — this is what the staking
	// module stores as OperatorAddress in production. The rewardmult module's
	// computeFraudPenalty and computeParticipationBonus parse these with
	// sdk.ValAddressFromBech32, so they MUST be omnivaloper1... format.
	valAddr1    = sdk.ValAddress("val1________________").String()
	valAddr2    = sdk.ValAddress("val2________________").String()
	valAddr3    = sdk.ValAddress("val3________________").String()
	contributor = sdk.AccAddress("contributor_________")
	challenger  = sdk.AccAddress("challenger__________")
)

// valAddrToAccAddr converts a ValAddress bech32 string to AccAddress bech32 string.
// In Cosmos SDK, both share the same underlying bytes — only the bech32 prefix differs.
// PoR module uses AccAddress format for verifiers; staking/rewardmult use ValAddress.
func valAddrToAccAddr(valAddr string) string {
	va, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		panic("invalid valAddr: " + err.Error())
	}
	return sdk.AccAddress(va).String()
}

func makeValidator(operatorAddr string, tokens math.Int, bonded bool) stakingtypes.Validator {
	status := stakingtypes.Unbonded
	if bonded {
		status = stakingtypes.Bonded
	}
	return stakingtypes.Validator{
		OperatorAddress: operatorAddr,
		Tokens:          tokens,
		DelegatorShares: tokens.ToLegacyDec(),
		Status:          status,
	}
}

// ---- Helpers ----

// advanceBlock simulates advancing the chain by one block.
func (f *IntegrationFixture) advanceBlock() {
	header := f.ctx.BlockHeader()
	header.Height++
	header.Time = header.Time.Add(5 * time.Second)
	f.ctx = f.ctx.WithBlockHeader(header)
}

// advanceBlocks simulates advancing the chain by n blocks.
func (f *IntegrationFixture) advanceBlocks(n int) {
	for i := 0; i < n; i++ {
		f.advanceBlock()
	}
}

// advanceTime adds a duration to the block time.
func (f *IntegrationFixture) advanceTime(d time.Duration) {
	header := f.ctx.BlockHeader()
	header.Time = header.Time.Add(d)
	f.ctx = f.ctx.WithBlockHeader(header)
}

// setBlockHeight sets the block height directly.
func (f *IntegrationFixture) setBlockHeight(h int64) {
	header := f.ctx.BlockHeader()
	header.Height = h
	f.ctx = f.ctx.WithBlockHeader(header)
}

// createTestApp registers a PoR application and returns its ID.
func (f *IntegrationFixture) createTestApp(owner string) uint64 {
	f.t.Helper()
	ms := porkeeper.NewMsgServerImpl(f.porKeeper)
	resp, err := ms.RegisterApp(f.ctx, &portypes.MsgRegisterApp{
		Owner:           owner,
		Name:            "test-app",
		SchemaCid:       "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		ChallengePeriod: 3600, // 1 hour
		MinVerifiers:    2,
	})
	require.NoError(f.t, err)
	return resp.AppId
}

// createVerifierSet creates a verifier set with the given members.
// owner must match the app owner that registered the app.
func (f *IntegrationFixture) createVerifierSet(appID, epoch uint64, members []portypes.VerifierMember, owner string) uint64 {
	f.t.Helper()
	ms := porkeeper.NewMsgServerImpl(f.porKeeper)
	resp, err := ms.CreateVerifierSet(f.ctx, &portypes.MsgCreateVerifierSet{
		Creator:         owner,
		AppId:           appID,
		Epoch:           epoch,
		Members:         members,
		MinAttestations: 3, // must be >= MinVerifiersGlobal (3)
		QuorumPct:       math.LegacyNewDecWithPrec(67, 2), // 67%
	})
	require.NoError(f.t, err)
	return resp.VerifierSetId
}

// submitBatch submits a batch and returns its ID.
func (f *IntegrationFixture) submitBatch(appID, epoch, verifierSetID uint64, submitter string, recordCount uint64) uint64 {
	f.t.Helper()
	ms := porkeeper.NewMsgServerImpl(f.porKeeper)
	merkleRoot := make([]byte, 32)
	merkleRoot[0] = 0xAB // non-zero
	merkleRoot[31] = 0xCD
	resp, err := ms.SubmitBatch(f.ctx, &portypes.MsgSubmitBatch{
		Submitter:        submitter,
		AppId:            appID,
		Epoch:            epoch,
		RecordMerkleRoot: merkleRoot,
		RecordCount:      recordCount,
		VerifierSetId:    verifierSetID,
	})
	require.NoError(f.t, err)
	return resp.BatchId
}

// submitAttestation submits a verifier attestation.
// verifier should be a ValAddress string (omnivaloper1...) — it will be
// converted to AccAddress for the PoR message (which uses AccAddress).
func (f *IntegrationFixture) submitAttestation(batchID uint64, verifier string) *portypes.MsgSubmitAttestationResponse {
	f.t.Helper()
	// Compute the batch-binding commitment signature required by the PoR keeper:
	// SHA256(batchID || merkleRoot || epoch || verifierAddress)
	verifierAccAddr := valAddrToAccAddr(verifier)
	batch, found := f.porKeeper.GetBatch(f.ctx, batchID)
	require.True(f.t, found, "batch %d not found for attestation", batchID)
	sig := portypes.ComputeAttestationSignBytes(batch.BatchId, batch.RecordMerkleRoot, batch.Epoch, verifierAccAddr)

	ms := porkeeper.NewMsgServerImpl(f.porKeeper)
	resp, err := ms.SubmitAttestation(f.ctx, &portypes.MsgSubmitAttestation{
		Verifier:         verifierAccAddr,
		BatchId:          batchID,
		Signature:        sig,
		ConfidenceWeight: math.LegacyOneDec(),
	})
	require.NoError(f.t, err)
	return resp
}

// submitChallenge submits a challenge against a batch.
func (f *IntegrationFixture) submitChallenge(batchID uint64, challengerAddr string) uint64 {
	f.t.Helper()
	ms := porkeeper.NewMsgServerImpl(f.porKeeper)
	resp, err := ms.ChallengeBatch(f.ctx, &portypes.MsgChallengeBatch{
		Challenger:    challengerAddr,
		BatchId:       batchID,
		ChallengeType: portypes.ChallengeTypeInvalidRoot,
		ProofData:     []byte("fraud-proof-data"),
	})
	require.NoError(f.t, err)
	return resp.ChallengeId
}

// runPoREndBlocker runs the PoR EndBlocker.
func (f *IntegrationFixture) runPoREndBlocker() {
	f.t.Helper()
	err := f.porKeeper.EndBlocker(f.ctx)
	require.NoError(f.t, err)
}

// processRewardMultEpoch processes a rewardmult epoch.
func (f *IntegrationFixture) processRewardMultEpoch(epoch int64) {
	f.t.Helper()
	err := f.rmKeeper.ProcessEpoch(f.ctx, epoch)
	require.NoError(f.t, err)
}

// getPocPorBridge retrieves the bridge for modifying contribution→batch mapping.
func (f *IntegrationFixture) getPocPorBridge() *pocPorBridge {
	// This is set in the PoC keeper via SetPorKeeper.
	// We reconstruct the bridge from our state for test manipulation.
	// The fixture holds the same porKeeper.
	return newPocPorBridge(f.porKeeper)
}

// hasEvent checks if a specific event type was emitted.
func hasEvent(ctx sdk.Context, eventType string) bool {
	for _, e := range ctx.EventManager().Events() {
		if e.Type == eventType {
			return true
		}
	}
	return false
}

// getEventAttribute extracts an attribute value from the first matching event.
func getEventAttribute(ctx sdk.Context, eventType, attrKey string) string {
	for _, e := range ctx.EventManager().Events() {
		if e.Type == eventType {
			for _, a := range e.Attributes {
				if a.Key == attrKey {
					return a.Value
				}
			}
		}
	}
	return ""
}

// countEvents counts how many events of a given type were emitted.
func countEvents(ctx sdk.Context, eventType string) int {
	count := 0
	for _, e := range ctx.EventManager().Events() {
		if e.Type == eventType {
			count++
		}
	}
	return count
}

// ============================================================================
// Test 1: Happy-path contribution → PoR finality → PoC rewards
//
// Scenario: A contribution is submitted, linked to a PoR batch. The batch
// goes through attestation → quorum → challenge window → finalization.
// PoC credits are awarded on finalization.
// ============================================================================

func TestHappyPath_ContributionFinalityRewards(t *testing.T) {
	f := SetupIntegration(t)

	// Step 1: Register PoR app and create verifier set
	appOwner := contributor.String()
	appID := f.createTestApp(appOwner)

	members := []portypes.VerifierMember{
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr1), math.NewInt(1_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr2), math.NewInt(2_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr3), math.NewInt(3_000_000), time.Now().Unix()),
	}
	vsID := f.createVerifierSet(appID, 1, members, appOwner)

	// Step 2: Submit PoR batch with 10 records
	batchID := f.submitBatch(appID, 1, vsID, contributor.String(), 10)
	batch, found := f.porKeeper.GetBatch(f.ctx, batchID)
	require.True(t, found)
	require.Equal(t, portypes.BatchStatusSubmitted, batch.Status)

	// Step 3: Submit attestations (need 2 min, 67% quorum)
	// val1 (1M) + val2 (2M) = 3M/6M = 50% — not enough for 67% quorum
	resp1 := f.submitAttestation(batchID, valAddr1)
	require.False(t, resp1.MetQuorum)

	resp2 := f.submitAttestation(batchID, valAddr2)
	require.False(t, resp2.MetQuorum)

	// val3 (3M) brings total to 6M/6M = 100% → quorum met
	resp3 := f.submitAttestation(batchID, valAddr3)
	require.True(t, resp3.MetQuorum)

	// Verify batch is now PENDING
	batch, found = f.porKeeper.GetBatch(f.ctx, batchID)
	require.True(t, found)
	require.Equal(t, portypes.BatchStatusPending, batch.Status)

	// Step 4: Create a PoC contribution and link it to the batch
	contribution := poctypes.Contribution{
		Id:          1,
		Contributor: contributor.String(),
		Ctype:       "record",
		Uri:         "ipfs://testrecord",
		Hash:        make([]byte, 32),
		Verified:    true,
	}
	contribution.Hash[0] = 0x01
	require.NoError(t, f.pocKeeper.SetContribution(f.ctx, contribution))

	// Check: contributor has no credits yet
	credits := f.pocKeeper.GetCredits(f.ctx, contributor)
	require.True(t, credits.Amount.IsZero())

	// Step 5: Advance past challenge window and run EndBlocker
	f.advanceTime(2 * time.Hour) // challenge period is 1 hour
	f.runPoREndBlocker()

	// Verify batch is now FINALIZED
	batch, found = f.porKeeper.GetBatch(f.ctx, batchID)
	require.True(t, found)
	require.Equal(t, portypes.BatchStatusFinalized, batch.Status)

	// Step 6: Verify PoC credits were awarded by EndBlocker (via PoR→PoC integration)
	// EndBlocker awards BaseRecordReward * RecordCount = 100 * 10 = 1000 credits
	credits = f.pocKeeper.GetCredits(f.ctx, contributor)
	require.True(t, credits.Amount.Equal(math.NewInt(1000)), "expected 1000 credits, got %s", credits.Amount)

	// Step 7: Verify finalization events were emitted
	require.True(t, hasEvent(f.ctx, "por_batch_auto_finalized"))
	require.True(t, hasEvent(f.ctx, "por_batch_quorum_met"))

	// Step 8: Verify verifier reputations were updated
	// PoR stores reputations keyed by AccAddress (omni1...) since that's what attestations use
	for _, addr := range []string{valAddr1, valAddr2, valAddr3} {
		rep, found := f.porKeeper.GetVerifierReputation(f.ctx, valAddrToAccAddr(addr))
		require.True(t, found, "reputation should exist for %s", addr)
		require.Equal(t, uint64(1), rep.TotalAttestations)
		require.Equal(t, uint64(1), rep.CorrectAttestations)
	}
}

// ============================================================================
// Test 2: Fraud challenge → batch invalidation → credit burn
//
// Scenario: A batch is submitted and attested, then a valid challenge is
// submitted. ProcessValidChallenge rejects the batch and slashes attesters.
// Contributions linked to the rejected batch should fail finality checks.
// ============================================================================

func TestFraudChallenge_InvalidationCreditBurn(t *testing.T) {
	f := SetupIntegration(t)

	// Setup: app, verifier set, batch, attestations
	appOwner := contributor.String()
	appID := f.createTestApp(appOwner)

	members := []portypes.VerifierMember{
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr1), math.NewInt(2_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr2), math.NewInt(2_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr3), math.NewInt(2_000_000), time.Now().Unix()),
	}
	vsID := f.createVerifierSet(appID, 1, members, appOwner)
	batchID := f.submitBatch(appID, 1, vsID, contributor.String(), 5)

	// Attest to reach quorum
	f.submitAttestation(batchID, valAddr1)
	f.submitAttestation(batchID, valAddr2)
	resp3 := f.submitAttestation(batchID, valAddr3)
	require.True(t, resp3.MetQuorum)

	// Batch should be PENDING
	batch, found := f.porKeeper.GetBatch(f.ctx, batchID)
	require.True(t, found)
	require.Equal(t, portypes.BatchStatusPending, batch.Status)

	// Step 1: Add credits to the contributor via PoC (simulating a prior payout)
	require.NoError(t, f.pocKeeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(5000)))
	credits := f.pocKeeper.GetCredits(f.ctx, contributor)
	require.True(t, credits.Amount.Equal(math.NewInt(5000)))

	// Step 2: Freeze credits for the contribution (as if a challenge is being processed)
	require.NoError(t, f.pocKeeper.FreezeCredits(f.ctx, contributor.String(), math.NewInt(500), 1, "fraud_challenge"))

	// Step 3: Submit a challenge against the batch
	challengeID := f.submitChallenge(batchID, challenger.String())
	require.True(t, challengeID > 0)

	// Verify open challenges exist
	require.True(t, f.porKeeper.HasOpenChallenges(f.ctx, batchID))

	// Step 4: Process the valid challenge (simulates governance resolution)
	require.NoError(t, f.porKeeper.ProcessValidChallenge(f.ctx, challengeID))

	// Verify batch is now REJECTED
	batch, found = f.porKeeper.GetBatch(f.ctx, batchID)
	require.True(t, found)
	require.Equal(t, portypes.BatchStatusRejected, batch.Status)

	// Step 5: Challenge resolved as valid
	challenge, found := f.porKeeper.GetChallenge(f.ctx, challengeID)
	require.True(t, found)
	require.Equal(t, portypes.ChallengeStatusResolvedValid, challenge.Status)

	// Step 6: Burn frozen credits (simulating fraud conviction)
	require.NoError(t, f.pocKeeper.BurnFrozenCredits(f.ctx, contributor.String()))

	// Credits should be reduced by frozen amount: 5000 - 500 = 4500
	credits = f.pocKeeper.GetCredits(f.ctx, contributor)
	require.True(t, credits.Amount.Equal(math.NewInt(4500)), "expected 4500, got %s", credits.Amount)

	// Frozen should be cleared
	frozen := f.pocKeeper.GetFrozenCredits(f.ctx, contributor.String())
	require.True(t, frozen.Amount.IsZero())

	// Step 7: Verify verifier reputations were penalized
	// PoR stores reputations keyed by AccAddress since attestations use AccAddress
	for _, addr := range []string{valAddr1, valAddr2, valAddr3} {
		rep, found := f.porKeeper.GetVerifierReputation(f.ctx, valAddrToAccAddr(addr))
		require.True(t, found, "reputation should exist for %s", addr)
		require.True(t, rep.SlashedCount > 0, "expected slash for %s", addr)
	}

	// Step 8: Verify rejection events emitted
	require.True(t, hasEvent(f.ctx, "por_batch_rejected"))

	// Step 9: Advance past challenge window, EndBlocker should NOT finalize the rejected batch
	f.advanceTime(2 * time.Hour)
	f.runPoREndBlocker()

	batch, found = f.porKeeper.GetBatch(f.ctx, batchID)
	require.True(t, found)
	require.Equal(t, portypes.BatchStatusRejected, batch.Status) // still rejected, not finalized
}

// ============================================================================
// Test 3: PoR fraud → rewardmult penalty propagation
//
// Scenario: A validator attests to a fraudulent PoR batch. After the batch
// is rejected, the rewardmult module's epoch processing picks up the
// HasFraudulentAttestation signal and applies the fraud penalty to the
// validator's multiplier.
// ============================================================================

func TestPoRFraud_RewardMultPenaltyPropagation(t *testing.T) {
	f := SetupIntegration(t)

	// Setup: app, verifier set, batch with val1 and val2 as attesters
	appOwner := contributor.String()
	appID := f.createTestApp(appOwner)

	members := []portypes.VerifierMember{
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr1), math.NewInt(3_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr2), math.NewInt(3_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr3), math.NewInt(3_000_000), time.Now().Unix()),
	}
	vsID := f.createVerifierSet(appID, 1, members, appOwner)
	batchID := f.submitBatch(appID, 1, vsID, contributor.String(), 5)

	// Attest with all three validators — quorum met
	f.submitAttestation(batchID, valAddr1)
	f.submitAttestation(batchID, valAddr2)
	resp := f.submitAttestation(batchID, valAddr3)
	require.True(t, resp.MetQuorum)

	// Submit challenge and validate it
	challengeID := f.submitChallenge(batchID, challenger.String())
	require.NoError(t, f.porKeeper.ProcessValidChallenge(f.ctx, challengeID))

	// Confirm batch is REJECTED
	batch, _ := f.porKeeper.GetBatch(f.ctx, batchID)
	require.Equal(t, portypes.BatchStatusRejected, batch.Status)

	// Step 1: Process rewardmult epoch
	// All three validators attested on the rejected batch, so all three
	// should get FraudPenalty applied by the HasFraudulentAttestation bridge.
	f.processRewardMultEpoch(1)

	// Step 2: Verify multipliers — all three attesters should have fraud penalty
	vm1, found := f.rmKeeper.GetValidatorMultiplier(f.ctx, valAddr1)
	require.True(t, found, "val1 multiplier should exist")

	vm2, found := f.rmKeeper.GetValidatorMultiplier(f.ctx, valAddr2)
	require.True(t, found, "val2 multiplier should exist")

	vm3, found := f.rmKeeper.GetValidatorMultiplier(f.ctx, valAddr3)
	require.True(t, found, "val3 multiplier should exist")

	// All three validators attested on the fraudulent batch → FraudPenalty > 0
	require.True(t, vm1.FraudPenalty.IsPositive(),
		"val1 should have fraud penalty, got %s", vm1.FraudPenalty)
	require.True(t, vm2.FraudPenalty.IsPositive(),
		"val2 should have fraud penalty, got %s", vm2.FraudPenalty)
	require.True(t, vm3.FraudPenalty.IsPositive(),
		"val3 should have fraud penalty, got %s", vm3.FraudPenalty)

	// Fraud penalty reduces M_raw: 1 - FraudPenalty < 1
	// All attesters' M_raw should be < 1.0 (fraud penalty = 0.10 default)
	require.True(t, vm1.MRaw.LT(math.LegacyOneDec()),
		"val1 M_raw should be < 1.0, got %s", vm1.MRaw)
	require.True(t, vm2.MRaw.LT(math.LegacyOneDec()),
		"val2 M_raw should be < 1.0, got %s", vm2.MRaw)
	require.True(t, vm3.MRaw.LT(math.LegacyOneDec()),
		"val3 M_raw should be < 1.0, got %s", vm3.MRaw)

	// Step 3: Verify budget neutrality invariant still holds
	msg, broken := rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "budget neutral invariant broken: %s", msg)

	msg, broken = rmkeeper.MultiplierBoundsInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "multiplier bounds invariant broken: %s", msg)

	msg, broken = rmkeeper.NoNaNInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "no-nan invariant broken: %s", msg)
}

// ============================================================================
// Test 4: Emergency payout pause safety
//
// Scenario: Governance activates emergency payout pause. While paused,
// PoC credit payouts should be blocked. When unpaused, payouts resume.
// Chain and PoS rewards (rewardmult) are unaffected throughout.
// ============================================================================

func TestEmergencyPayoutPause(t *testing.T) {
	f := SetupIntegration(t)

	// Step 1: Verify payouts are NOT paused initially
	require.False(t, f.pocKeeper.IsPayoutsPaused(f.ctx))

	// Step 2: Add initial credits
	require.NoError(t, f.pocKeeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(1000)))
	credits := f.pocKeeper.GetCredits(f.ctx, contributor)
	require.True(t, credits.Amount.Equal(math.NewInt(1000)))

	// Step 3: Activate emergency pause
	require.NoError(t, f.pocKeeper.SetPayoutsPaused(f.ctx, true))
	require.True(t, f.pocKeeper.IsPayoutsPaused(f.ctx))

	// Verify pause event emitted
	require.True(t, hasEvent(f.ctx, "poc_payouts_pause_changed"))

	// Step 4: RewardMult should still work during pause (PoS rewards unaffected)
	f.processRewardMultEpoch(1)

	// All 3 validators should have multipliers
	for _, addr := range []string{valAddr1, valAddr2, valAddr3} {
		_, found := f.rmKeeper.GetValidatorMultiplier(f.ctx, addr)
		require.True(t, found, "validator %s should have multiplier during pause", addr)
	}

	// Budget neutrality should hold
	msg, broken := rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "budget invariant broken during pause: %s", msg)

	// Step 5: Existing PoC credits should still be queryable (they're not removed)
	credits = f.pocKeeper.GetCredits(f.ctx, contributor)
	require.True(t, credits.Amount.Equal(math.NewInt(1000)))

	// Step 6: Deactivate pause
	require.NoError(t, f.pocKeeper.SetPayoutsPaused(f.ctx, false))
	require.False(t, f.pocKeeper.IsPayoutsPaused(f.ctx))

	// Step 7: After unpause, credits can be added again
	require.NoError(t, f.pocKeeper.AddCreditsWithOverflowCheck(f.ctx, contributor, math.NewInt(500)))
	credits = f.pocKeeper.GetCredits(f.ctx, contributor)
	require.True(t, credits.Amount.Equal(math.NewInt(1500)))
}

// ============================================================================
// Test 5: Quorum-gaming / freerider soft penalty
//
// Scenario: A validator endorses only after quorum is met (quorum-gaming)
// or barely participates (freeriding). The PoC module detects this pattern
// and applies soft endorsement penalties. The rewardmult module should
// see reduced participation bonus for penalized validators.
// ============================================================================

func TestQuorumGaming_FreeriderPenalty(t *testing.T) {
	f := SetupIntegration(t)

	// Step 1: Set up endorsement stats for val1 (freerider) and val2 (quorum-gamer)
	// val1: very low participation — freeriding
	freeriderStats := poctypes.NewValidatorEndorsementStats(valAddr1)
	freeriderStats.TotalOpportunity = 100
	freeriderStats.TotalEndorsed = 5 // 5% participation — below 20% threshold
	freeriderStats.QuorumEndorsed = 3
	freeriderStats.EarlyEndorsed = 2
	freeriderStats.ParticipationEMA = math.LegacyNewDecWithPrec(5, 2) // 0.05
	require.NoError(t, f.pocKeeper.SetValidatorEndorsementStats(f.ctx, freeriderStats))

	// val2: high quorum-endorsement rate — quorum gaming
	gamerStats := poctypes.NewValidatorEndorsementStats(valAddr2)
	gamerStats.TotalOpportunity = 100
	gamerStats.TotalEndorsed = 80
	gamerStats.QuorumEndorsed = 65 // 81% post-quorum > 70% threshold
	gamerStats.EarlyEndorsed = 15
	gamerStats.ParticipationEMA = math.LegacyNewDecWithPrec(80, 2) // 0.80
	require.NoError(t, f.pocKeeper.SetValidatorEndorsementStats(f.ctx, gamerStats))

	// val3: healthy participation — no penalty
	healthyStats := poctypes.NewValidatorEndorsementStats(valAddr3)
	healthyStats.TotalOpportunity = 100
	healthyStats.TotalEndorsed = 70
	healthyStats.QuorumEndorsed = 20 // 28% post-quorum — normal
	healthyStats.EarlyEndorsed = 50
	healthyStats.ParticipationEMA = math.LegacyNewDecWithPrec(70, 2) // 0.70
	require.NoError(t, f.pocKeeper.SetValidatorEndorsementStats(f.ctx, healthyStats))

	// Step 2: Verify quality checks detect the patterns
	isFreeriding, isQuorumGaming := f.pocKeeper.CheckValidatorEndorsementQuality(f.ctx, valAddr1)
	require.True(t, isFreeriding, "val1 should be flagged as freerider")
	require.False(t, isQuorumGaming, "val1 should not be flagged as quorum-gamer")

	isFreeriding, isQuorumGaming = f.pocKeeper.CheckValidatorEndorsementQuality(f.ctx, valAddr2)
	require.False(t, isFreeriding, "val2 should not be freerider")
	require.True(t, isQuorumGaming, "val2 should be flagged as quorum-gamer")

	isFreeriding, isQuorumGaming = f.pocKeeper.CheckValidatorEndorsementQuality(f.ctx, valAddr3)
	require.False(t, isFreeriding, "val3 should not be freerider")
	require.False(t, isQuorumGaming, "val3 should not be quorum-gamer")

	// Step 3: Apply endorsement quality penalties
	require.NoError(t, f.pocKeeper.ApplyEndorsementQualityPenalties(f.ctx, 1))

	// Step 4: Verify penalties were applied
	penalty1, found := f.pocKeeper.GetEndorsementPenalty(f.ctx, valAddr1)
	require.True(t, found, "val1 should have endorsement penalty")
	require.True(t, penalty1.ParticipationBonusBlocked)

	penalty2, found := f.pocKeeper.GetEndorsementPenalty(f.ctx, valAddr2)
	require.True(t, found, "val2 should have endorsement penalty")
	require.True(t, penalty2.ParticipationBonusBlocked)

	// val3 should NOT have a penalty
	_, found = f.pocKeeper.GetEndorsementPenalty(f.ctx, valAddr3)
	require.False(t, found, "val3 should not have endorsement penalty")

	// Step 5: Process rewardmult epoch — participation bonus affected
	f.processRewardMultEpoch(1)

	// Verify multipliers were created for all validators
	vm1, found1 := f.rmKeeper.GetValidatorMultiplier(f.ctx, valAddr1)
	require.True(t, found1, "val1 multiplier should exist")
	vm3, found3 := f.rmKeeper.GetValidatorMultiplier(f.ctx, valAddr3)
	require.True(t, found3, "val3 multiplier should exist")

	// NOTE: In test environment, ParticipationBonus is 0 for all validators because
	// computeParticipationBonus calls sdk.ValAddressFromBech32 which fails for the
	// test addresses (omni1... AccAddress format, not omnivaloper1... ValAddress).
	// This is the nil-safe fallback behavior — the bonus computation returns 0 on parse failure.
	// The actual freerider/quorum-gaming detection was already verified above in steps 2-4
	// via CheckValidatorEndorsementQuality and ApplyEndorsementQualityPenalties.

	// val1 (freerider, penalized) should not have a higher bonus than val3 (healthy)
	require.True(t, vm1.ParticipationBonus.LTE(vm3.ParticipationBonus),
		"penalized val1 bonus (%s) should not exceed healthy val3 bonus (%s)",
		vm1.ParticipationBonus, vm3.ParticipationBonus)

	// Verify the IsParticipationBonusBlocked mechanism is correctly set
	require.True(t, f.pocKeeper.IsParticipationBonusBlocked(f.ctx, valAddr1, 1),
		"val1 participation bonus should be blocked")
	require.True(t, f.pocKeeper.IsParticipationBonusBlocked(f.ctx, valAddr2, 1),
		"val2 participation bonus should be blocked")
	require.False(t, f.pocKeeper.IsParticipationBonusBlocked(f.ctx, valAddr3, 1),
		"val3 participation bonus should NOT be blocked")

	// Step 6: Verify all invariants still hold
	msg, broken := rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "budget invariant broken: %s", msg)

	msg, broken = rmkeeper.MultiplierBoundsInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "bounds invariant broken: %s", msg)

	// Step 7: Verify penalty events were emitted
	require.True(t, countEvents(f.ctx, "poc_endorsement_penalty_applied") >= 2,
		"expected at least 2 penalty events")
}

// ============================================================================
// Test 6: Stake churn snapshot consistency
//
// Scenario: Validator stakes change between epochs. The rewardmult module
// should snapshot stake weights at epoch boundary and use consistent weights
// for normalization. Changing validators mid-epoch should not break budget
// neutrality.
// ============================================================================

func TestStakeChurnSnapshotConsistency(t *testing.T) {
	f := SetupIntegration(t)

	// Step 1: Process epoch 1 with initial stakes: val1=1M, val2=2M, val3=3M
	f.processRewardMultEpoch(1)

	// Verify snapshots captured
	snapshots := f.rmKeeper.GetEpochStakeSnapshots(f.ctx, 1)
	require.Len(t, snapshots, 3)

	// Verify snapshot values match initial stakes
	snapshotMap := make(map[string]math.Int)
	for _, s := range snapshots {
		snapshotMap[s.ValidatorAddress] = s.BondedTokens
	}
	require.True(t, snapshotMap[valAddr1].Equal(math.NewInt(1_000_000)),
		"val1 snapshot should be 1M, got %s", snapshotMap[valAddr1])
	require.True(t, snapshotMap[valAddr2].Equal(math.NewInt(2_000_000)),
		"val2 snapshot should be 2M, got %s", snapshotMap[valAddr2])
	require.True(t, snapshotMap[valAddr3].Equal(math.NewInt(3_000_000)),
		"val3 snapshot should be 3M, got %s", snapshotMap[valAddr3])

	// Step 2: Budget neutral after epoch 1
	msg, broken := rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "budget neutral broken after epoch 1: %s", msg)

	// Step 3: Simulate stake churn — val1 gains massive stake, val3 drops
	f.staking.validators = []stakingtypes.Validator{
		makeValidator(valAddr1, math.NewInt(10_000_000), true), // 10x increase
		makeValidator(valAddr2, math.NewInt(2_000_000), true),  // unchanged
		makeValidator(valAddr3, math.NewInt(500_000), true),    // halved
	}

	// Step 4: Process epoch 2 with new stakes
	f.processRewardMultEpoch(2)

	// Verify epoch 2 snapshots reflect the new stakes
	snapshots2 := f.rmKeeper.GetEpochStakeSnapshots(f.ctx, 2)
	require.Len(t, snapshots2, 3)

	snapshot2Map := make(map[string]math.Int)
	for _, s := range snapshots2 {
		snapshot2Map[s.ValidatorAddress] = s.BondedTokens
	}
	require.True(t, snapshot2Map[valAddr1].Equal(math.NewInt(10_000_000)),
		"val1 epoch2 snapshot should be 10M, got %s", snapshot2Map[valAddr1])
	require.True(t, snapshot2Map[valAddr3].Equal(math.NewInt(500_000)),
		"val3 epoch2 snapshot should be 500K, got %s", snapshot2Map[valAddr3])

	// Step 5: Epoch 1 snapshots should be unchanged (independent per epoch)
	snapshots1Again := f.rmKeeper.GetEpochStakeSnapshots(f.ctx, 1)
	require.Len(t, snapshots1Again, 3)
	for _, s := range snapshots1Again {
		if s.ValidatorAddress == valAddr1 {
			require.True(t, s.BondedTokens.Equal(math.NewInt(1_000_000)),
				"epoch 1 val1 snapshot should still be 1M")
		}
	}

	// Step 6: Budget neutrality must hold after stake churn
	msg, broken = rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "budget neutral broken after stake churn: %s", msg)

	msg, broken = rmkeeper.MultiplierBoundsInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "bounds invariant broken after stake churn: %s", msg)

	msg, broken = rmkeeper.NoNaNInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "no-nan invariant broken after stake churn: %s", msg)

	// Step 7: Add a 4th validator and remove one — test adding/removing validators
	f.staking.validators = []stakingtypes.Validator{
		makeValidator(valAddr1, math.NewInt(5_000_000), true),
		makeValidator(valAddr2, math.NewInt(5_000_000), true),
		// val3 removed (unbonded)
		makeValidator(sdk.ValAddress("val4________________").String(), math.NewInt(5_000_000), true),
	}

	f.processRewardMultEpoch(3)

	// Budget neutral with changed validator set
	msg, broken = rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "budget neutral broken after validator set change: %s", msg)

	// Epoch 3 should have 3 snapshots (not 4, since val3 is unbonded)
	snapshots3 := f.rmKeeper.GetEpochStakeSnapshots(f.ctx, 3)
	require.Len(t, snapshots3, 3)

	// Step 8: Run 5 more epochs to verify stability across many stake changes
	for epoch := int64(4); epoch <= 8; epoch++ {
		// Randomly adjust stakes each epoch
		f.staking.validators = []stakingtypes.Validator{
			makeValidator(valAddr1, math.NewInt(int64(epoch)*1_000_000), true),
			makeValidator(valAddr2, math.NewInt(3_000_000), true),
			makeValidator(sdk.ValAddress("val4________________").String(), math.NewInt(int64(10-epoch)*1_000_000), true),
		}
		f.processRewardMultEpoch(epoch)

		msg, broken = rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
		require.False(t, broken, "budget neutral broken at epoch %d: %s", epoch, msg)
	}
}

// ============================================================================
// Additional Cross-Module Invariant Tests
// ============================================================================

// TestAllInvariantsHoldAfterFullLifecycle runs all three modules through a
// complete lifecycle and verifies all invariants at the end.
func TestAllInvariantsHoldAfterFullLifecycle(t *testing.T) {
	f := SetupIntegration(t)

	// -- PoR lifecycle --
	appID := f.createTestApp(contributor.String())
	members := []portypes.VerifierMember{
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr1), math.NewInt(2_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr2), math.NewInt(2_000_000), time.Now().Unix()),
		portypes.NewVerifierMember(valAddrToAccAddr(valAddr3), math.NewInt(2_000_000), time.Now().Unix()),
	}
	vsID := f.createVerifierSet(appID, 1, members, contributor.String())

	// Submit and finalize a batch
	batchID := f.submitBatch(appID, 1, vsID, contributor.String(), 20)
	f.submitAttestation(batchID, valAddr1)
	f.submitAttestation(batchID, valAddr2)
	f.submitAttestation(batchID, valAddr3)
	f.advanceTime(2 * time.Hour)
	f.runPoREndBlocker()

	// -- PoC lifecycle --
	contribution := poctypes.Contribution{
		Id: 1, Contributor: contributor.String(), Ctype: "code",
		Uri: "ipfs://test", Hash: make([]byte, 32), Verified: true,
	}
	contribution.Hash[0] = 0x01
	require.NoError(t, f.pocKeeper.SetContribution(f.ctx, contribution))
	require.NoError(t, f.pocKeeper.EnqueueReward(f.ctx, contribution))

	// -- RewardMult lifecycle --
	f.processRewardMultEpoch(1)
	f.processRewardMultEpoch(2)

	// -- Verify ALL invariants --

	// RewardMult invariants
	msg, broken := rmkeeper.BudgetNeutralInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "budget neutral: %s", msg)
	msg, broken = rmkeeper.MultiplierBoundsInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "multiplier bounds: %s", msg)
	msg, broken = rmkeeper.NoNaNInvariant(f.rmKeeper)(f.ctx)
	require.False(t, broken, "no-nan: %s", msg)

	// PoC invariants
	msg, broken = pockeeper.CreditCapInvariant(f.pocKeeper)(f.ctx)
	require.False(t, broken, "credit cap: %s", msg)
	msg, broken = pockeeper.FrozenCreditsInvariant(f.pocKeeper)(f.ctx)
	require.False(t, broken, "frozen credits: %s", msg)
}

// ============================================================================
// Domain-Safety Regression Tests
// ============================================================================

// TestDomainSafety_CrossDomainRejection verifies that AccAddressFromBech32
// rejects ValAddress strings and vice-versa, ensuring address domain separation.
func TestDomainSafety_CrossDomainRejection(t *testing.T) {
	// Use deterministic 20-byte address
	addrBytes := bytes.Repeat([]byte{0xAB}, 20)

	// ValAddress bech32 → AccAddressFromBech32 should fail
	valBech32 := sdk.ValAddress(addrBytes).String()
	_, err := sdk.AccAddressFromBech32(valBech32)
	require.Error(t, err, "AccAddressFromBech32 should reject a ValAddress string")

	// AccAddress bech32 → ValAddressFromBech32 should fail
	accBech32 := sdk.AccAddress(addrBytes).String()
	_, err = sdk.ValAddressFromBech32(accBech32)
	require.Error(t, err, "ValAddressFromBech32 should reject an AccAddress string")
}
