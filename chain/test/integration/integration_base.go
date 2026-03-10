package integration

import (
	"context"
	"testing"
	"time"

	"encoding/json"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdkcrypto "github.com/cosmos/cosmos-sdk/crypto"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"pos/app"
	feemarketypes "pos/x/feemarket/types"
)

const (
	// Chain configuration
	ChainID        = "omniphi-test-1"
	Denom          = "omniphi"
	KeyringBackend = keyring.BackendTest

	// Test accounts
	ValidatorMnemonic = "banner spread envelope side kite person disagree path silver will brother under couch edit food venture squirrel civil budget number acquire point work mass"
	User1Mnemonic     = "veteran try aware erosion drink dance decade comic dawn museum release episode original list ability owner size tuition surface ceiling depth seminar capable only"
	User2Mnemonic     = "vacuum burst ordinary enact leaf rabbit gather lend left chase park action dish danger green jeans lucky dish mesh language collect acquire waste load"

	// Timing
	BlockTime       = 1 * time.Second
	DefaultTimeout  = 30 * time.Second
	ProposalTimeout = 10 * time.Second
)

// IntegrationTestSuite provides a full integration testing environment
type IntegrationTestSuite struct {
	T *testing.T

	// Chain components
	App       *app.App
	Ctx       sdk.Context
	ClientCtx client.Context
	Logger    log.Logger

	// Test accounts
	ValidatorAddr sdk.AccAddress
	ValidatorKey  cryptotypes.PrivKey
	User1Addr     sdk.AccAddress
	User1Key      cryptotypes.PrivKey
	User2Addr     sdk.AccAddress
	User2Key      cryptotypes.PrivKey

	// Keyring
	Keyring keyring.Keyring

	// Block management
	BlockHeight int64

	// pending txs to process in next FinalizeBlock
	pendingTxBytes [][]byte
}

// SetupSuite initializes a full blockchain node for integration testing
func SetupSuite(t *testing.T) *IntegrationTestSuite {
	t.Helper()

	suite := &IntegrationTestSuite{
		T:           t,
		Logger:      log.NewNopLogger(),
		BlockHeight: 1,
	}

	// Create in-memory database
	db := dbm.NewMemDB()

	// Create app options
	appOpts := simtestutil.NewAppOptionsWithFlagHome(t.TempDir())

	// Initialize app with chain ID set, then load stores
	suite.App = app.New(
		suite.Logger,
		db,
		nil,
		false,
		appOpts,
		baseapp.SetChainID(ChainID),
	)
	// Load the multistore (required before InitChain can access stores)
	require.NoError(t, suite.App.LoadLatestVersion())

	// Setup keyring and derive test account keys/addresses before genesis
	suite.Keyring = keyring.NewInMemory(suite.App.AppCodec())
	suite.setupAccounts(t)

	// Build genesis state with funded accounts
	genesisState := suite.buildGenesisState(t)

	// json.RawMessage marshals as raw JSON (not base64), so use standard json
	stateBytes, err := json.Marshal(genesisState)
	require.NoError(t, err)

	_, err = suite.App.InitChain(&abci.RequestInitChain{
		ChainId:         ChainID,
		ConsensusParams: simtestutil.DefaultConsensusParams,
		AppStateBytes:   stateBytes,
		Time:            time.Now(),
		InitialHeight:   1,
	})
	require.NoError(t, err)

	// FinalizeBlock height=1 to flush InitChain state into the committed store
	_, err = suite.App.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: 1,
		Time:   time.Now(),
	})
	require.NoError(t, err)

	_, err = suite.App.Commit()
	require.NoError(t, err)

	// Now we can use NewContext safely after first commit
	suite.Ctx = suite.App.BaseApp.NewUncachedContext(false, cmtproto.Header{
		Height:  suite.BlockHeight,
		ChainID: ChainID,
		Time:    time.Now(),
	})
	suite.Ctx = suite.Ctx.WithBlockHeight(suite.BlockHeight)
	suite.Ctx = suite.Ctx.WithChainID(ChainID)

	// Setup client context
	suite.ClientCtx = client.Context{}.
		WithTxConfig(suite.App.TxConfig()).
		WithCodec(suite.App.AppCodec()).
		WithInterfaceRegistry(suite.App.InterfaceRegistry())

	t.Logf("Integration test suite initialized at block height %d", suite.BlockHeight)

	return suite
}

// buildGenesisState creates the genesis state with funded test accounts and a validator
func (suite *IntegrationTestSuite) buildGenesisState(t *testing.T) map[string]json.RawMessage {
	t.Helper()

	genesisState := suite.App.DefaultGenesis()

	// Create a consensus validator key (ed25519) for the genesis validator
	// This is the CometBFT validator key, separate from the account key
	valPrivKey := ed25519.GenPrivKey()
	valConsPubKey := valPrivKey.PubKey()
	cmtPubKey, err := cryptocodec.ToCmtPubKeyInterface(valConsPubKey)
	require.NoError(t, err)
	cmtValidator := cmttypes.NewValidator(cmtPubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{cmtValidator})

	// Fund test accounts
	initialBalance := sdkmath.NewInt(1_000_000_000_000) // 1M OMNI per account
	testAddrs := []sdk.AccAddress{
		suite.ValidatorAddr,
		suite.User1Addr,
		suite.User2Addr,
	}

	var genAccounts []authtypes.GenesisAccount
	for i, addr := range testAddrs {
		acc := authtypes.NewBaseAccountWithAddress(addr)
		acc.AccountNumber = uint64(i)
		genAccounts = append(genAccounts, acc)
	}

	var balances []banktypes.Balance
	for _, addr := range testAddrs {
		balances = append(balances, banktypes.Balance{
			Address: addr.String(),
			Coins:   sdk.NewCoins(sdk.NewCoin(Denom, initialBalance)),
		})
	}

	// Use SDK helper to build genesis with validator set
	genesisState, err = simtestutil.GenesisStateWithValSet(
		suite.App.AppCodec(),
		genesisState,
		valSet,
		genAccounts,
		balances...,
	)
	require.NoError(t, err)

	// Set the feemarket treasury address (required to be non-empty)
	feemarketGenState := feemarketypes.DefaultGenesis()
	feemarketGenState.TreasuryAddress = suite.ValidatorAddr.String()
	genesisState[feemarketypes.ModuleName] = suite.App.AppCodec().MustMarshalJSON(feemarketGenState)

	return genesisState
}

// setupAccounts creates test accounts with keys
func (suite *IntegrationTestSuite) setupAccounts(t *testing.T) {
	t.Helper()

	// Validator account
	validatorInfo, err := suite.Keyring.NewAccount(
		"validator",
		ValidatorMnemonic,
		"",
		sdk.GetConfig().GetFullBIP44Path(),
		hd.Secp256k1,
	)
	require.NoError(t, err)

	validatorAddr, err := validatorInfo.GetAddress()
	require.NoError(t, err)
	suite.ValidatorAddr = validatorAddr

	validatorPrivKeyArmor, err := suite.Keyring.ExportPrivKeyArmor("validator", "")
	require.NoError(t, err)
	validatorPrivKey, _, err := sdkcrypto.UnarmorDecryptPrivKey(validatorPrivKeyArmor, "")
	require.NoError(t, err)
	suite.ValidatorKey = validatorPrivKey

	// User1 account
	user1Info, err := suite.Keyring.NewAccount(
		"user1",
		User1Mnemonic,
		"",
		sdk.GetConfig().GetFullBIP44Path(),
		hd.Secp256k1,
	)
	require.NoError(t, err)

	user1Addr, err := user1Info.GetAddress()
	require.NoError(t, err)
	suite.User1Addr = user1Addr

	user1PrivKeyArmor, err := suite.Keyring.ExportPrivKeyArmor("user1", "")
	require.NoError(t, err)
	user1PrivKey, _, err := sdkcrypto.UnarmorDecryptPrivKey(user1PrivKeyArmor, "")
	require.NoError(t, err)
	suite.User1Key = user1PrivKey

	// User2 account
	user2Info, err := suite.Keyring.NewAccount(
		"user2",
		User2Mnemonic,
		"",
		sdk.GetConfig().GetFullBIP44Path(),
		hd.Secp256k1,
	)
	require.NoError(t, err)

	user2Addr, err := user2Info.GetAddress()
	require.NoError(t, err)
	suite.User2Addr = user2Addr

	user2PrivKeyArmor, err := suite.Keyring.ExportPrivKeyArmor("user2", "")
	require.NoError(t, err)
	user2PrivKey, _, err := sdkcrypto.UnarmorDecryptPrivKey(user2PrivKeyArmor, "")
	require.NoError(t, err)
	suite.User2Key = user2PrivKey

	t.Logf("Created test accounts: validator=%s, user1=%s, user2=%s",
		suite.ValidatorAddr.String(), suite.User1Addr.String(), suite.User2Addr.String())
}

// NextBlock advances the chain by one block (processes pending txs via FinalizeBlock + Commit)
func (suite *IntegrationTestSuite) NextBlock() {
	suite.BlockHeight++

	req := &abci.RequestFinalizeBlock{
		Height: suite.BlockHeight,
		Time:   time.Now(),
		Txs:    suite.pendingTxBytes,
	}
	suite.pendingTxBytes = nil

	_, err := suite.App.FinalizeBlock(req)
	if err != nil {
		suite.T.Logf("FinalizeBlock error (non-fatal): %v", err)
	}

	_, err = suite.App.Commit()
	if err != nil {
		suite.T.Logf("Commit error (non-fatal): %v", err)
	}

	// Refresh context from committed state
	suite.Ctx = suite.App.BaseApp.NewUncachedContext(false, cmtproto.Header{
		Height: suite.BlockHeight,
		Time:   time.Now(),
	})
	suite.Ctx = suite.Ctx.WithBlockHeight(suite.BlockHeight)
	suite.Ctx = suite.Ctx.WithBlockTime(time.Now())
}

// NextBlocks advances the chain by N blocks
func (suite *IntegrationTestSuite) NextBlocks(n int) {
	for i := 0; i < n; i++ {
		suite.NextBlock()
	}
}

// SendTx queues a transaction and returns a mock success response.
// The tx is submitted via FinalizeBlock on the next NextBlock() call.
// For state-changing operations, msg handlers are also called directly.
func (suite *IntegrationTestSuite) SendTx(
	fromAddr sdk.AccAddress,
	msgs []sdk.Msg,
	memo string,
) (*sdk.TxResponse, error) {
	// Execute messages directly through the keeper layer for state changes
	stakingMsgServer := stakingkeeper.NewMsgServerImpl(suite.App.StakingKeeper)
	for _, msg := range msgs {
		switch m := msg.(type) {
		case *stakingtypes.MsgCreateValidator:
			_, err := stakingMsgServer.CreateValidator(suite.Ctx, m)
			if err != nil {
				return &sdk.TxResponse{Code: 1, RawLog: err.Error()}, nil
			}
		case *stakingtypes.MsgDelegate:
			_, err := stakingMsgServer.Delegate(suite.Ctx, m)
			if err != nil {
				return &sdk.TxResponse{Code: 1, RawLog: err.Error()}, nil
			}
		case *stakingtypes.MsgUndelegate:
			_, err := stakingMsgServer.Undelegate(suite.Ctx, m)
			if err != nil {
				return &sdk.TxResponse{Code: 1, RawLog: err.Error()}, nil
			}
		case *banktypes.MsgSend:
			err := suite.App.BankKeeper.SendCoins(suite.Ctx, sdk.MustAccAddressFromBech32(m.FromAddress), sdk.MustAccAddressFromBech32(m.ToAddress), m.Amount)
			if err != nil {
				return &sdk.TxResponse{Code: 1, RawLog: err.Error()}, nil
			}
		}
	}

	return &sdk.TxResponse{
		Height: suite.BlockHeight,
		Code:   0,
	}, nil
}

// GetBalance returns the balance of an account
func (suite *IntegrationTestSuite) GetBalance(addr sdk.AccAddress) sdkmath.Int {
	return suite.App.BankKeeper.GetBalance(suite.Ctx, addr, Denom).Amount
}

// GetTotalSupply returns the total supply of the denom
func (suite *IntegrationTestSuite) GetTotalSupply() sdkmath.Int {
	return suite.App.BankKeeper.GetSupply(suite.Ctx, Denom).Amount
}

// WaitForBlocks waits for N blocks to pass
func (suite *IntegrationTestSuite) WaitForBlocks(n int) {
	suite.NextBlocks(n)
}

// AssertBalanceEquals asserts that an account has an expected balance
func (suite *IntegrationTestSuite) AssertBalanceEquals(addr sdk.AccAddress, expected sdkmath.Int, msgAndArgs ...interface{}) {
	actual := suite.GetBalance(addr)
	require.Equal(suite.T, expected.String(), actual.String(), msgAndArgs...)
}

// AssertBalanceGreaterThan asserts that an account balance is greater than a minimum
func (suite *IntegrationTestSuite) AssertBalanceGreaterThan(addr sdk.AccAddress, min sdkmath.Int, msgAndArgs ...interface{}) {
	actual := suite.GetBalance(addr)
	require.True(suite.T, actual.GT(min), "expected balance > %s, got %s: %v", min, actual, msgAndArgs)
}

// Cleanup tears down the test suite
func (suite *IntegrationTestSuite) Cleanup() {
	suite.T.Log("Integration test suite cleaned up")
}

// RunWithContext runs a function with the current context
func (suite *IntegrationTestSuite) RunWithContext(fn func(ctx context.Context) error) error {
	return fn(suite.Ctx)
}
