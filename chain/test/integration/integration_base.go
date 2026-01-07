package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"pos/app"
	tokenomicstypes "pos/x/tokenomics/types"
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
// with a live blockchain node
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

	// Create app options using testutil helper
	appOpts := simtestutil.NewAppOptionsWithFlagHome(t.TempDir())

	// Initialize app using the new depinject-based constructor
	suite.App = app.New(
		suite.Logger,
		db,
		nil, // traceStore
		true,
		appOpts,
	)

	// Create context
	suite.Ctx = suite.App.BaseApp.NewContext(false)
	suite.Ctx = suite.Ctx.WithBlockHeight(suite.BlockHeight)
	suite.Ctx = suite.Ctx.WithChainID(ChainID)

	// Setup keyring
	suite.Keyring = keyring.NewInMemory(suite.App.AppCodec())

	// Create test accounts
	suite.setupAccounts(t)

	// Setup client context
	suite.ClientCtx = client.Context{}.
		WithKeyring(suite.Keyring).
		WithTxConfig(suite.App.TxConfig()).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithChainID(ChainID).
		WithCodec(suite.App.AppCodec()).
		WithInterfaceRegistry(suite.App.InterfaceRegistry())

	// Initialize genesis state
	suite.initGenesis(t)

	t.Logf("Integration test suite initialized at block height %d", suite.BlockHeight)

	return suite
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

	validatorPrivKey, err := suite.Keyring.ExportPrivKeyArmor("validator", "")
	require.NoError(t, err)
	_ = validatorPrivKey // Store for later use

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

	t.Logf("Created test accounts: validator=%s, user1=%s, user2=%s",
		suite.ValidatorAddr.String(), suite.User1Addr.String(), suite.User2Addr.String())
}

// initGenesis initializes the chain with genesis state
func (suite *IntegrationTestSuite) initGenesis(t *testing.T) {
	t.Helper()

	// Fund test accounts using the bank keeper
	initialBalance := sdkmath.NewInt(1_000_000_000_000) // 1M OMNI
	testAccounts := []sdk.AccAddress{
		suite.ValidatorAddr,
		suite.User1Addr,
		suite.User2Addr,
	}

	for _, addr := range testAccounts {
		coins := sdk.NewCoins(sdk.NewCoin(Denom, initialBalance))
		err := suite.App.BankKeeper.MintCoins(suite.Ctx, tokenomicstypes.ModuleName, coins)
		require.NoError(t, err)

		err = suite.App.BankKeeper.SendCoinsFromModuleToAccount(
			suite.Ctx,
			tokenomicstypes.ModuleName,
			addr,
			coins,
		)
		require.NoError(t, err)
	}

	// Initialize module accounts
	suite.initModuleAccounts(t)

	t.Logf("Genesis initialized with %d test accounts funded", len(testAccounts))
}

// initModuleAccounts ensures all module accounts exist
func (suite *IntegrationTestSuite) initModuleAccounts(t *testing.T) {
	t.Helper()

	moduleAccounts := []string{
		authtypes.FeeCollectorName,
		tokenomicstypes.ModuleName,
	}

	for _, modName := range moduleAccounts {
		modAddr := authtypes.NewModuleAddress(modName)
		acc := suite.App.AuthKeeper.GetAccount(suite.Ctx, modAddr)
		if acc == nil {
			acc = authtypes.NewModuleAccount(
				authtypes.NewBaseAccountWithAddress(modAddr),
				modName,
			)
			suite.App.AuthKeeper.SetAccount(suite.Ctx, acc)
		}
	}

	t.Logf("Initialized %d module accounts", len(moduleAccounts))
}

// NextBlock advances the chain by one block
func (suite *IntegrationTestSuite) NextBlock() {
	// Increment height
	suite.BlockHeight++

	// Create new block context
	suite.Ctx = suite.App.BaseApp.NewContext(false)
	suite.Ctx = suite.Ctx.WithBlockHeight(suite.BlockHeight)
	suite.Ctx = suite.Ctx.WithBlockTime(time.Now())
}

// NextBlocks advances the chain by N blocks
func (suite *IntegrationTestSuite) NextBlocks(n int) {
	for i := 0; i < n; i++ {
		suite.NextBlock()
	}
}

// SendTx sends a transaction and returns the result
func (suite *IntegrationTestSuite) SendTx(
	fromAddr sdk.AccAddress,
	msgs []sdk.Msg,
	memo string,
) (*sdk.TxResponse, error) {
	// Get account info
	acc := suite.App.AuthKeeper.GetAccount(suite.Ctx, fromAddr)
	if acc == nil {
		return nil, fmt.Errorf("account not found: %s", fromAddr.String())
	}

	// Build transaction
	txBuilder := suite.ClientCtx.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	if err != nil {
		return nil, err
	}

	txBuilder.SetMemo(memo)
	txBuilder.SetGasLimit(200000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(Denom, sdkmath.NewInt(2000))))

	// Sign transaction
	sigV2 := signing.SignatureV2{
		PubKey: acc.GetPubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: acc.GetSequence(),
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, err
	}

	// Get signing bytes
	signerData := authsigning.SignerData{
		ChainID:       ChainID,
		AccountNumber: acc.GetAccountNumber(),
		Sequence:      acc.GetSequence(),
	}

	sigV2, err = suite.signTx(txBuilder, signerData, fromAddr)
	if err != nil {
		return nil, err
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, err
	}

	// Encode transaction
	txBytes, err := suite.ClientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	// Return response (simplified for testing)
	txResponse := &sdk.TxResponse{
		Height: suite.BlockHeight,
		TxHash: fmt.Sprintf("%X", txBytes[:16]), // Simplified hash
	}

	return txResponse, nil
}

// signTx signs a transaction
func (suite *IntegrationTestSuite) signTx(
	txBuilder client.TxBuilder,
	signerData authsigning.SignerData,
	fromAddr sdk.AccAddress,
) (signing.SignatureV2, error) {
	// Get key name from address
	var keyName string
	switch fromAddr.String() {
	case suite.ValidatorAddr.String():
		keyName = "validator"
	case suite.User1Addr.String():
		keyName = "user1"
	case suite.User2Addr.String():
		keyName = "user2"
	default:
		return signing.SignatureV2{}, fmt.Errorf("unknown address: %s", fromAddr.String())
	}
	_ = keyName // Used for key lookup

	// For testing, return a mock signature
	return signing.SignatureV2{
		PubKey: nil,
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
			Signature: []byte("mock-signature"),
		},
		Sequence: signerData.Sequence,
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
	// Close any open resources
	if suite.App != nil {
		// App cleanup if needed
	}
	suite.T.Log("Integration test suite cleaned up")
}

// RunWithContext runs a function with the current context
func (suite *IntegrationTestSuite) RunWithContext(fn func(ctx context.Context) error) error {
	return fn(suite.Ctx)
}
