package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/suite"

	"pos/x/tokenomics/keeper"
	"pos/x/tokenomics/types"
)

// MockAccountKeeper is a mock implementation of AccountKeeper
type MockAccountKeeper struct {
	accounts map[string]sdk.AccountI
}

func NewMockAccountKeeper() *MockAccountKeeper {
	return &MockAccountKeeper{
		accounts: make(map[string]sdk.AccountI),
	}
}

func (m *MockAccountKeeper) GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	return m.accounts[addr.String()]
}

func (m *MockAccountKeeper) SetAccount(ctx context.Context, acc sdk.AccountI) {
	m.accounts[acc.GetAddress().String()] = acc
}

func (m *MockAccountKeeper) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}

func (m *MockAccountKeeper) GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI {
	addr := authtypes.NewModuleAddress(name)
	if acc := m.accounts[addr.String()]; acc != nil {
		if modAcc, ok := acc.(sdk.ModuleAccountI); ok {
			return modAcc
		}
	}
	return nil
}

// MockBankKeeper is a mock implementation of BankKeeper
type MockBankKeeper struct {
	balances map[string]sdk.Coins
	supply   sdk.Coins
}

func NewMockBankKeeper() *MockBankKeeper {
	return &MockBankKeeper{
		balances: make(map[string]sdk.Coins),
		supply:   sdk.NewCoins(),
	}
}

func (m *MockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	moduleAddr := authtypes.NewModuleAddress(senderModule).String()
	recipientStr := recipientAddr.String()

	// Deduct from module
	if bal, ok := m.balances[moduleAddr]; ok {
		newBal, hasNeg := bal.SafeSub(amt...)
		if hasNeg {
			return types.ErrInsufficientFunds
		}
		m.balances[moduleAddr] = newBal
	}

	// Add to recipient
	if bal, ok := m.balances[recipientStr]; ok {
		m.balances[recipientStr] = bal.Add(amt...)
	} else {
		m.balances[recipientStr] = amt
	}

	return nil
}

func (m *MockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	senderStr := senderAddr.String()
	moduleAddr := authtypes.NewModuleAddress(recipientModule).String()

	// Deduct from sender
	if bal, ok := m.balances[senderStr]; ok {
		newBal, hasNeg := bal.SafeSub(amt...)
		if hasNeg {
			return types.ErrInsufficientFunds
		}
		m.balances[senderStr] = newBal
	}

	// Add to module
	if bal, ok := m.balances[moduleAddr]; ok {
		m.balances[moduleAddr] = bal.Add(amt...)
	} else {
		m.balances[moduleAddr] = amt
	}

	return nil
}

func (m *MockBankKeeper) SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	fromStr := fromAddr.String()
	toStr := toAddr.String()

	// Deduct from sender
	if bal, ok := m.balances[fromStr]; ok {
		newBal, hasNeg := bal.SafeSub(amt...)
		if hasNeg {
			return types.ErrInsufficientFunds
		}
		m.balances[fromStr] = newBal
	} else {
		return types.ErrInsufficientFunds
	}

	// Add to recipient
	if bal, ok := m.balances[toStr]; ok {
		m.balances[toStr] = bal.Add(amt...)
	} else {
		m.balances[toStr] = amt
	}

	return nil
}

func (m *MockBankKeeper) SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	senderAddr := authtypes.NewModuleAddress(senderModule).String()
	recipientAddr := authtypes.NewModuleAddress(recipientModule).String()

	// Deduct from sender module
	if bal, ok := m.balances[senderAddr]; ok {
		newBal, hasNeg := bal.SafeSub(amt...)
		if hasNeg {
			return types.ErrInsufficientFunds
		}
		m.balances[senderAddr] = newBal
	}

	// Add to recipient module
	if bal, ok := m.balances[recipientAddr]; ok {
		m.balances[recipientAddr] = bal.Add(amt...)
	} else {
		m.balances[recipientAddr] = amt
	}

	return nil
}

func (m *MockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	moduleAddr := authtypes.NewModuleAddress(moduleName).String()

	// Add to module balance
	if bal, ok := m.balances[moduleAddr]; ok {
		m.balances[moduleAddr] = bal.Add(amt...)
	} else {
		m.balances[moduleAddr] = amt
	}

	// Add to total supply
	m.supply = m.supply.Add(amt...)

	return nil
}

func (m *MockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	moduleAddr := authtypes.NewModuleAddress(moduleName).String()

	// Deduct from module balance
	if bal, ok := m.balances[moduleAddr]; ok {
		newBal, hasNeg := bal.SafeSub(amt...)
		if hasNeg {
			return types.ErrInsufficientFunds
		}
		m.balances[moduleAddr] = newBal
	} else {
		return types.ErrInsufficientFunds
	}

	// Deduct from total supply
	newSupply, hasNeg := m.supply.SafeSub(amt...)
	if hasNeg {
		return types.ErrInsufficientFunds
	}
	m.supply = newSupply

	return nil
}

func (m *MockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if bal, ok := m.balances[addr.String()]; ok {
		return sdk.NewCoin(denom, bal.AmountOf(denom))
	}
	return sdk.NewCoin(denom, math.ZeroInt())
}

func (m *MockBankKeeper) GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	if bal, ok := m.balances[addr.String()]; ok {
		return bal
	}
	return sdk.NewCoins()
}

func (m *MockBankKeeper) SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	return m.GetAllBalances(ctx, addr)
}

// MockStakingKeeper is a mock implementation of StakingKeeper
type MockStakingKeeper struct {
	bondedTokens math.Int
}

func NewMockStakingKeeper() *MockStakingKeeper {
	return &MockStakingKeeper{
		bondedTokens: math.ZeroInt(),
	}
}

func (m *MockStakingKeeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	return []stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error) {
	return []stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	return m.bondedTokens, nil
}

func (m *MockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	return sdk.DefaultPowerReduction
}

// KeeperTestSuite is the test suite for the tokenomics keeper
type KeeperTestSuite struct {
	suite.Suite

	ctx           sdk.Context
	keeper        keeper.Keeper
	accountKeeper *MockAccountKeeper
	bankKeeper    *MockBankKeeper
	stakingKeeper *MockStakingKeeper
	encCfg        moduletestutil.TestEncodingConfig
}

// SetupTest sets up the test suite
func (suite *KeeperTestSuite) SetupTest() {
	key := storetypes.NewKVStoreKey(types.ModuleName)
	testCtx := testutil.DefaultContextWithDB(suite.T(), key, storetypes.NewTransientStoreKey("transient_test"))
	suite.ctx = testCtx.Ctx

	// Create encoding config
	suite.encCfg = moduletestutil.MakeTestEncodingConfig()
	types.RegisterInterfaces(suite.encCfg.InterfaceRegistry)

	// Create mock keepers
	suite.accountKeeper = NewMockAccountKeeper()
	suite.bankKeeper = NewMockBankKeeper()
	suite.stakingKeeper = NewMockStakingKeeper()

	// Create tokenomics keeper
	suite.keeper = keeper.NewKeeper(
		suite.encCfg.Codec,
		runtime.NewKVStoreService(key),
		log.NewNopLogger(),
		suite.accountKeeper,
		suite.bankKeeper,
		suite.stakingKeeper,
		nil, // GovKeeper
		nil, // IBCKeeper
		authtypes.NewModuleAddress("gov").String(),
	)

	// Initialize default params
	params := types.DefaultParams()
	err := suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)
}

// TestKeeperTestSuite runs the test suite
func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

// TestSuiteWrapper wraps KeeperTestSuite for direct function tests
type TestSuiteWrapper struct {
	Ctx           sdk.Context
	Keeper        keeper.Keeper
	AccountKeeper *MockAccountKeeper
	BankKeeper    *MockBankKeeper
	StakingKeeper *MockStakingKeeper
}

// SetupTestSuite creates a test suite for non-suite-style tests
func SetupTestSuite(t *testing.T) *TestSuiteWrapper {
	key := storetypes.NewKVStoreKey(types.ModuleName)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	// Create encoding config
	encCfg := moduletestutil.MakeTestEncodingConfig()
	types.RegisterInterfaces(encCfg.InterfaceRegistry)

	// Create mock keepers
	accountKeeper := NewMockAccountKeeper()
	bankKeeper := NewMockBankKeeper()
	stakingKeeper := NewMockStakingKeeper()

	// Create tokenomics keeper
	k := keeper.NewKeeper(
		encCfg.Codec,
		runtime.NewKVStoreService(key),
		log.NewNopLogger(),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		nil, // GovKeeper
		nil, // IBCKeeper
		authtypes.NewModuleAddress("gov").String(),
	)

	// Initialize default params
	params := types.DefaultParams()
	err := k.SetParams(ctx, params)
	if err != nil {
		t.Fatalf("failed to set params: %v", err)
	}

	return &TestSuiteWrapper{
		Ctx:           ctx,
		Keeper:        k,
		AccountKeeper: accountKeeper,
		BankKeeper:    bankKeeper,
		StakingKeeper: stakingKeeper,
	}
}

// ==================== P0-CAP Tests: Supply Cap Enforcement ====================

// TestSupplyCapEnforcement_P0_CAP_001 tests that minting is rejected when it would exceed the cap
func (suite *KeeperTestSuite) TestSupplyCapEnforcement_P0_CAP_001() {
	params := suite.keeper.GetParams(suite.ctx)

	// Set current supply to just below cap
	almostCap := params.TotalSupplyCap.Sub(math.NewInt(1000))
	err := suite.keeper.SetCurrentSupply(suite.ctx, almostCap)
	suite.Require().NoError(err)

	// Attempt to mint more than remaining
	recipient := sdk.AccAddress([]byte("recipient"))
	tooMuch := math.NewInt(2000) // Would exceed cap by 1000

	err = suite.keeper.MintTokens(suite.ctx, tooMuch, recipient, "test mint")
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, types.ErrSupplyCapExceeded)
}

// TestSupplyCapEnforcement_P0_CAP_002 tests minting exactly to the cap succeeds
func (suite *KeeperTestSuite) TestSupplyCapEnforcement_P0_CAP_002() {
	params := suite.keeper.GetParams(suite.ctx)

	// Set current supply to zero
	err := suite.keeper.SetCurrentSupply(suite.ctx, math.ZeroInt())
	suite.Require().NoError(err)

	// Mint exactly to cap
	recipient := sdk.AccAddress([]byte("recipient"))
	err = suite.keeper.MintTokens(suite.ctx, params.TotalSupplyCap, recipient, "mint to cap")
	suite.Require().NoError(err)

	// Verify supply equals cap
	currentSupply := suite.keeper.GetCurrentSupply(suite.ctx)
	suite.Require().True(currentSupply.Equal(params.TotalSupplyCap))
}

// TestSupplyCapEnforcement_P0_CAP_003 tests that cap is validated BEFORE minting
func (suite *KeeperTestSuite) TestSupplyCapEnforcement_P0_CAP_003() {
	params := suite.keeper.GetParams(suite.ctx)

	// Set supply just below cap
	almostCap := params.TotalSupplyCap.Sub(math.NewInt(100))
	err := suite.keeper.SetCurrentSupply(suite.ctx, almostCap)
	suite.Require().NoError(err)

	// Record initial state
	initialSupply := suite.keeper.GetCurrentSupply(suite.ctx)
	initialMinted := suite.keeper.GetTotalMinted(suite.ctx)

	// Attempt to exceed cap
	recipient := sdk.AccAddress([]byte("recipient"))
	err = suite.keeper.MintTokens(suite.ctx, math.NewInt(200), recipient, "exceed cap")
	suite.Require().Error(err)

	// Verify NO state change occurred (atomic failure)
	finalSupply := suite.keeper.GetCurrentSupply(suite.ctx)
	finalMinted := suite.keeper.GetTotalMinted(suite.ctx)
	suite.Require().True(initialSupply.Equal(finalSupply), "supply should not change on failed mint")
	suite.Require().True(initialMinted.Equal(finalMinted), "total minted should not change on failed mint")
}

// TestSupplyCapWarnings_P0_CAP_004 tests warning events at thresholds
func (suite *KeeperTestSuite) TestSupplyCapWarnings_P0_CAP_004() {
	params := suite.keeper.GetParams(suite.ctx)

	// Test 80% threshold
	threshold80 := params.TotalSupplyCap.MulRaw(80).QuoRaw(100)
	err := suite.keeper.SetCurrentSupply(suite.ctx, threshold80)
	suite.Require().NoError(err)

	// Mint a small amount to trigger warning check
	recipient := sdk.AccAddress([]byte("recipient"))
	err = suite.keeper.MintTokens(suite.ctx, math.NewInt(1000), recipient, "trigger 80% warning")
	suite.Require().NoError(err)

	// Event emission is checked via the context's event manager
	events := suite.ctx.EventManager().Events()
	hasWarning := false
	for _, event := range events {
		if event.Type == "supply_cap_warning" {
			hasWarning = true
			break
		}
	}
	suite.Require().True(hasWarning, "should emit supply cap warning event")
}

// TestSupplyCapImmutable_P0_CAP_005 tests that supply cap cannot be changed via params
func (suite *KeeperTestSuite) TestSupplyCapImmutable_P0_CAP_005() {
	params := suite.keeper.GetParams(suite.ctx)
	originalCap := params.TotalSupplyCap

	// Attempt to change cap
	params.TotalSupplyCap = originalCap.MulRaw(2) // Try to double it
	err := suite.keeper.SetParams(suite.ctx, params)

	// Should fail - supply cap is immutable
	suite.Require().Error(err, "changing supply cap should be rejected")
	suite.Require().Contains(err.Error(), "immutable", "error should mention cap is immutable")

	// Verify cap is preserved
	newParams := suite.keeper.GetParams(suite.ctx)
	suite.Require().True(newParams.TotalSupplyCap.Equal(originalCap),
		"supply cap should remain unchanged after failed update attempt")
}

// TestSupplyCapOverflow_P0_CAP_006 tests protection against integer overflow
func (suite *KeeperTestSuite) TestSupplyCapOverflow_P0_CAP_006() {
	// Set supply to max safe value
	veryLarge := math.NewInt(1<<62 - 1) // Just below overflow threshold
	err := suite.keeper.SetCurrentSupply(suite.ctx, veryLarge)
	suite.Require().NoError(err)

	// Attempt to mint more (would overflow if not checked)
	recipient := sdk.AccAddress([]byte("recipient"))
	err = suite.keeper.MintTokens(suite.ctx, math.NewInt(1<<62), recipient, "overflow attempt")

	// Should be rejected (either by cap check or overflow protection)
	suite.Require().Error(err)
}

// ==================== P0-INF Tests: Inflation Bounds ====================

// TestInflationBounds_P0_INF_001 tests inflation rate is within configured range
func (suite *KeeperTestSuite) TestInflationBounds_P0_INF_001() {
	params := suite.keeper.GetParams(suite.ctx)

	// Verify default is within bounds
	suite.Require().True(params.InflationRate.GTE(params.InflationMin), "inflation rate below minimum")
	suite.Require().True(params.InflationRate.LTE(params.InflationMax), "inflation rate above maximum")

	// Verify bounds are valid (min < max)
	suite.Require().True(params.InflationMin.LT(params.InflationMax), "inflation min should be less than max")
}

// TestInflationBounds_P0_INF_002 tests setting inflation below minimum is rejected
func (suite *KeeperTestSuite) TestInflationBounds_P0_INF_002() {
	params := suite.keeper.GetParams(suite.ctx)

	// Attempt to set inflation below minimum (use a small positive value below min)
	// min is 0.5% (0.005), so 0.3% (0.003) is below min but still positive
	params.InflationRate = math.LegacyNewDecWithPrec(3, 3) // 0.3%
	err := suite.keeper.SetParams(suite.ctx, params)

	suite.Require().Error(err)
	suite.Require().ErrorIs(err, types.ErrInflationBelowMin)
}

// TestInflationBounds_P0_INF_003 tests setting inflation above maximum is rejected
func (suite *KeeperTestSuite) TestInflationBounds_P0_INF_003() {
	params := suite.keeper.GetParams(suite.ctx)

	// Attempt to set inflation above maximum
	params.InflationRate = params.InflationMax.Add(math.LegacyNewDecWithPrec(1, 2)) // max + 1%
	err := suite.keeper.SetParams(suite.ctx, params)

	suite.Require().Error(err)
	suite.Require().ErrorIs(err, types.ErrInflationAboveMax)
}

// TestInflationProtocolCap_P0_INF_004 tests 3% protocol cap cannot be exceeded
func (suite *KeeperTestSuite) TestInflationProtocolCap_P0_INF_004() {
	params := suite.keeper.GetParams(suite.ctx)

	// Attempt to set inflation_max above 3% (protocol hard cap)
	params.InflationMax = math.LegacyNewDecWithPrec(4, 2) // 4%
	err := suite.keeper.SetParams(suite.ctx, params)

	suite.Require().Error(err)
	suite.Require().ErrorIs(err, types.ErrProtocolCapViolation)
}

// TestInflationProtocolCap_P0_INF_005 tests setting inflation_max to exactly 3% succeeds
func (suite *KeeperTestSuite) TestInflationProtocolCap_P0_INF_005() {
	params := suite.keeper.GetParams(suite.ctx)

	// Set inflation_max to exactly 3% (protocol hard cap)
	params.InflationMax = math.LegacyNewDecWithPrec(3, 2)
	params.InflationRate = math.LegacyNewDecWithPrec(3, 2) // Also set rate to 3%
	err := suite.keeper.SetParams(suite.ctx, params)

	suite.Require().NoError(err)

	// Verify it was set
	newParams := suite.keeper.GetParams(suite.ctx)
	suite.Require().True(newParams.InflationMax.Equal(math.LegacyNewDecWithPrec(3, 2)))
}

// TestBlockProvisions_P0_INF_006 tests block provisions calculation accuracy
func (suite *KeeperTestSuite) TestBlockProvisions_P0_INF_006() {
	// Set known supply and inflation
	supply := math.NewInt(1_000_000_000_000) // 1M OMNI
	err := suite.keeper.SetCurrentSupply(suite.ctx, supply)
	suite.Require().NoError(err)

	params := suite.keeper.GetParams(suite.ctx)
	params.InflationRate = math.LegacyNewDecWithPrec(3, 2) // 3%
	err = suite.keeper.SetParams(suite.ctx, params)
	suite.Require().NoError(err)

	// Calculate block provisions
	blockProvisions := suite.keeper.CalculateBlockProvisions(suite.ctx)

	// Expected: (1M OMNI * 0.03) / blocks_per_year
	// blocks_per_year = 4,500,857 (from code)
	annualProvisions := params.InflationRate.MulInt(supply) // 30,000 OMNI
	expected := annualProvisions.QuoInt64(4500857)

	suite.Require().True(blockProvisions.Equal(expected),
		"block provisions should match: annual_provisions / blocks_per_year")
}
