package comprehensive

import (
	"context"
	"fmt"
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
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/keeper"
	"pos/x/tokenomics/types"
)

// TestContext provides common test setup for comprehensive tests
type TestContext struct {
	Ctx              sdk.Context
	TokenomicsKeeper keeper.Keeper
	AccountKeeper    *MockAccountKeeper
	BankKeeper       *MockBankKeeper
	StakingKeeper    *MockStakingKeeper
	PoCKeeper        *MockPoCKeeper
	Cdc              moduletestutil.TestEncodingConfig
}

// SetupTestContext creates a new test context with all necessary keepers
func SetupTestContext(t *testing.T) *TestContext {
	t.Helper()

	// Create encoding config
	cdc := moduletestutil.MakeTestEncodingConfig()
	types.RegisterInterfaces(cdc.InterfaceRegistry)

	// Create store key
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	// Create context
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	// Create mock keepers
	accountKeeper := NewMockAccountKeeper()
	bankKeeper := NewMockBankKeeper()
	stakingKeeper := NewMockStakingKeeper()
	pocKeeper := NewMockPoCKeeper(bankKeeper)

	// Create tokenomics keeper
	tokenomicsKeeper := keeper.NewKeeper(
		cdc.Codec,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		nil, // govKeeper (optional)
		nil, // ibcKeeper (optional)
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	// Set default params
	params := types.DefaultParams()
	require.NoError(t, tokenomicsKeeper.SetParams(ctx, params))

	return &TestContext{
		Ctx:              ctx,
		TokenomicsKeeper: tokenomicsKeeper,
		AccountKeeper:    accountKeeper,
		BankKeeper:       bankKeeper,
		StakingKeeper:    stakingKeeper,
		PoCKeeper:        pocKeeper,
		Cdc:              cdc,
	}
}

// MemoEntry tracks transactions with memo fields for audit
type MemoEntry struct {
	From   string
	To     string
	Amount sdk.Coins
	Memo   string
}

// MockAccountKeeper implements a mock account keeper for testing
type MockAccountKeeper struct {
	accounts map[string]sdk.AccountI
}

func NewMockAccountKeeper() *MockAccountKeeper {
	return &MockAccountKeeper{
		accounts: make(map[string]sdk.AccountI),
	}
}

func (m *MockAccountKeeper) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}

func (m *MockAccountKeeper) GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	if acc := m.accounts[addr.String()]; acc != nil {
		return acc
	}
	return nil
}

func (m *MockAccountKeeper) SetAccount(ctx context.Context, acc sdk.AccountI) {
	m.accounts[acc.GetAddress().String()] = acc
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

// MockBankKeeper implements a mock bank keeper for testing
type MockBankKeeper struct {
	balances       map[string]sdk.Coins
	supply         sdk.Coins
	burned         sdk.Coins
	minted         sdk.Coins
	moduleBalances map[string]sdk.Coins
	memoLog        []MemoEntry
}

func NewMockBankKeeper() *MockBankKeeper {
	return &MockBankKeeper{
		balances:       make(map[string]sdk.Coins),
		supply:         sdk.NewCoins(),
		burned:         sdk.NewCoins(),
		minted:         sdk.NewCoins(),
		moduleBalances: make(map[string]sdk.Coins),
	}
}

func (m *MockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if coins, ok := m.balances[addr.String()]; ok {
		return sdk.NewCoin(denom, coins.AmountOf(denom))
	}
	return sdk.NewCoin(denom, math.ZeroInt())
}

func (m *MockBankKeeper) GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	if coins, ok := m.balances[addr.String()]; ok {
		return coins
	}
	return sdk.NewCoins()
}

func (m *MockBankKeeper) SetBalance(ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins) {
	m.balances[addr.String()] = coins
}

func (m *MockBankKeeper) SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	// For testing purposes, assume all balances are spendable
	return m.GetAllBalances(ctx, addr)
}

func (m *MockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	// Deduct from module
	moduleAddr := authtypes.NewModuleAddress(senderModule)
	if balance, ok := m.moduleBalances[moduleAddr.String()]; ok {
		newBalance := balance.Sub(amt...)
		m.moduleBalances[moduleAddr.String()] = newBalance
	}

	// Add to account
	if balance, ok := m.balances[recipientAddr.String()]; ok {
		m.balances[recipientAddr.String()] = balance.Add(amt...)
	} else {
		m.balances[recipientAddr.String()] = amt
	}
	return nil
}

func (m *MockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	// Deduct from account
	if balance, ok := m.balances[senderAddr.String()]; ok {
		newBalance := balance.Sub(amt...)
		m.balances[senderAddr.String()] = newBalance
	}

	// Add to module
	moduleAddr := authtypes.NewModuleAddress(recipientModule)
	if balance, ok := m.moduleBalances[moduleAddr.String()]; ok {
		m.moduleBalances[moduleAddr.String()] = balance.Add(amt...)
	} else {
		m.moduleBalances[moduleAddr.String()] = amt
	}
	return nil
}

func (m *MockBankKeeper) SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	// Deduct from sender module
	senderAddr := authtypes.NewModuleAddress(senderModule)
	if balance, ok := m.moduleBalances[senderAddr.String()]; ok {
		newBalance := balance.Sub(amt...)
		m.moduleBalances[senderAddr.String()] = newBalance
	}

	// Add to recipient module
	recipientAddr := authtypes.NewModuleAddress(recipientModule)
	if balance, ok := m.moduleBalances[recipientAddr.String()]; ok {
		m.moduleBalances[recipientAddr.String()] = balance.Add(amt...)
	} else {
		m.moduleBalances[recipientAddr.String()] = amt
	}
	return nil
}

func (m *MockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	// Check hard cap before minting
	newSupply := m.supply.Add(amt...)
	if newSupply.AmountOf(TestDenom).GT(math.NewInt(TestHardCap)) {
		return fmt.Errorf("minting would exceed hard cap: current=%s, amount=%s, cap=%d",
			m.supply.AmountOf(TestDenom), amt.AmountOf(TestDenom), TestHardCap)
	}

	// Track minted coins
	m.minted = m.minted.Add(amt...)
	m.supply = m.supply.Add(amt...)

	// Add to module balance
	moduleAddr := authtypes.NewModuleAddress(moduleName)
	if balance, ok := m.moduleBalances[moduleAddr.String()]; ok {
		m.moduleBalances[moduleAddr.String()] = balance.Add(amt...)
	} else {
		m.moduleBalances[moduleAddr.String()] = amt
	}
	return nil
}

func (m *MockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	// Check module has sufficient balance before burning
	moduleAddr := authtypes.NewModuleAddress(moduleName)
	if balance, ok := m.moduleBalances[moduleAddr.String()]; ok {
		if !balance.IsAllGTE(amt) {
			return fmt.Errorf("insufficient module balance to burn: have=%s, want=%s",
				balance.AmountOf(TestDenom), amt.AmountOf(TestDenom))
		}
		m.moduleBalances[moduleAddr.String()] = balance.Sub(amt...)
	} else {
		return fmt.Errorf("module %s has no balance", moduleName)
	}

	// Track burned coins
	m.burned = m.burned.Add(amt...)
	m.supply = m.supply.Sub(amt...)

	return nil
}

func (m *MockBankKeeper) GetSupply(ctx context.Context, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.supply.AmountOf(denom))
}

func (m *MockBankKeeper) SetSupply(supply sdk.Coins) {
	m.supply = supply
}

func (m *MockBankKeeper) GetMinted() sdk.Coins {
	return m.minted
}

func (m *MockBankKeeper) GetBurned() sdk.Coins {
	return m.burned
}

func (m *MockBankKeeper) GetModuleBalance(moduleName string) sdk.Coins {
	moduleAddr := authtypes.NewModuleAddress(moduleName)
	if balance, ok := m.moduleBalances[moduleAddr.String()]; ok {
		return balance
	}
	return sdk.NewCoins()
}

// MockStakingKeeper implements a mock staking keeper for testing
type MockStakingKeeper struct {
	totalBonded math.Int
}

func NewMockStakingKeeper() *MockStakingKeeper {
	return &MockStakingKeeper{
		totalBonded: math.NewInt(1000000000), // 1B default bonded
	}
}

func (m *MockStakingKeeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	return m.totalBonded, nil
}

func (m *MockStakingKeeper) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	// Return empty validator list for testing
	return []stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error) {
	// Return empty validator list for testing
	return []stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	// Return empty validator for testing
	return stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	// Standard power reduction for testing (1,000,000 omniphi = 1 voting power)
	return math.NewInt(1000000)
}

func (m *MockStakingKeeper) SetTotalBonded(amount math.Int) {
	m.totalBonded = amount
}

// Constants for testing
const (
	TestDenom = "omniphi"
	TestHardCap = 1_500_000_000_000_000 // 1.5B OMNI with 6 decimals (in omniphi base units)
	TestMinInflation = 0.01             // 1%
	TestMaxInflation = 0.05             // 5%
)

// Helper functions for common test operations

// AssertSupplyWithinCap verifies that total supply never exceeds hard cap
func AssertSupplyWithinCap(t *testing.T, bankKeeper *MockBankKeeper) {
	t.Helper()
	supply := bankKeeper.GetSupply(sdk.Context{}, TestDenom)
	require.LessOrEqual(t, supply.Amount.Int64(), int64(TestHardCap),
		"Supply %d exceeds hard cap %d", supply.Amount.Int64(), TestHardCap)
}

// AssertSupplyConservation verifies that supply = minted - burned
func AssertSupplyConservation(t *testing.T, bankKeeper *MockBankKeeper, initialSupply math.Int) {
	t.Helper()
	currentSupply := bankKeeper.GetSupply(sdk.Context{}, TestDenom).Amount
	minted := bankKeeper.GetMinted().AmountOf(TestDenom)
	burned := bankKeeper.GetBurned().AmountOf(TestDenom)

	expected := initialSupply.Add(minted).Sub(burned)
	require.Equal(t, expected, currentSupply,
		"Supply conservation failed: initial=%s + minted=%s - burned=%s != current=%s",
		initialSupply, minted, burned, currentSupply)
}

// AssertNoNegativeBalances verifies that no account or module has negative balance
func AssertNoNegativeBalances(t *testing.T, bankKeeper *MockBankKeeper) {
	t.Helper()

	// Check all account balances
	for addr, balance := range bankKeeper.balances {
		for _, coin := range balance {
			require.False(t, coin.Amount.IsNegative(),
				"Account %s has negative balance: %s", addr, coin)
		}
	}

	// Check all module balances
	for module, balance := range bankKeeper.moduleBalances {
		for _, coin := range balance {
			require.False(t, coin.Amount.IsNegative(),
				"Module %s has negative balance: %s", module, coin)
		}
	}
}

// ========================================
// MockPoCKeeper - PoC Merit Engine Mock
// ========================================

type Contribution struct {
	ID          string
	Contributor string
	Data        string
	Status      string // "pending", "verified", "rejected"
	Endorsements map[string]bool // validator -> vote (true=yes, false=no)
}

type SlashEvent struct {
	Validator sdk.ValAddress
	Reason    string
	Amount    math.Int
}

type MockPoCKeeper struct {
	contributions map[string]*Contribution
	credits       map[string]int64 // address -> credits
	slashEvents   map[string]*SlashEvent
	bankKeeper    *MockBankKeeper
}

func NewMockPoCKeeper(bankKeeper *MockBankKeeper) *MockPoCKeeper {
	return &MockPoCKeeper{
		contributions: make(map[string]*Contribution),
		credits:       make(map[string]int64),
		slashEvents:   make(map[string]*SlashEvent),
		bankKeeper:    bankKeeper,
	}
}

func (m *MockPoCKeeper) SubmitContribution(ctx sdk.Context, id string, contributor sdk.AccAddress, data string) {
	m.contributions[id] = &Contribution{
		ID:          id,
		Contributor: contributor.String(),
		Data:        data,
		Status:      "pending",
		Endorsements: make(map[string]bool),
	}
}

func (m *MockPoCKeeper) Endorse(ctx sdk.Context, contributionID string, validator sdk.ValAddress, vote bool) {
	if contrib, ok := m.contributions[contributionID]; ok {
		contrib.Endorsements[validator.String()] = vote
	}
}

func (m *MockPoCKeeper) EndorseWithError(ctx sdk.Context, contributionID string, validator sdk.ValAddress, vote bool) error {
	if contrib, ok := m.contributions[contributionID]; ok {
		if _, already := contrib.Endorsements[validator.String()]; already {
			return fmt.Errorf("validator already endorsed this contribution")
		}
		contrib.Endorsements[validator.String()] = vote
	}
	return nil
}

func (m *MockPoCKeeper) ProcessEndorsements(ctx sdk.Context, contributionID string) {
	contrib, ok := m.contributions[contributionID]
	if !ok {
		return
	}

	// Count yes votes
	yesVotes := 0
	totalVotes := len(contrib.Endorsements)

	for _, vote := range contrib.Endorsements {
		if vote {
			yesVotes++
		}
	}

	// Require ≥66.7% (2/3) yes votes
	if totalVotes == 0 {
		contrib.Status = "pending"
		return
	}

	yesPercent := float64(yesVotes) / float64(totalVotes)
	if yesPercent >= 0.667 {
		contrib.Status = "verified"
	} else {
		contrib.Status = "rejected"
	}
}

func (m *MockPoCKeeper) GetContribution(ctx sdk.Context, id string) *Contribution {
	return m.contributions[id]
}

func (m *MockPoCKeeper) MintCredits(ctx sdk.Context, contributionID string, contributor sdk.AccAddress, amount int64) {
	contrib := m.contributions[contributionID]
	if contrib != nil && contrib.Status == "verified" {
		m.credits[contributor.String()] += amount
	}
}

func (m *MockPoCKeeper) GetCredits(ctx sdk.Context, address sdk.AccAddress) int64 {
	return m.credits[address.String()]
}

func (m *MockPoCKeeper) SetCredits(ctx sdk.Context, address sdk.AccAddress, credits int64) {
	m.credits[address.String()] = credits
}

func (m *MockPoCKeeper) CalculateEffectivePower(stake math.Int, credits int64, alpha math.LegacyDec) math.Int {
	// Power = Stake × (1 + α × Credits)
	creditBonus := alpha.MulInt64(credits)                  // α × Credits
	multiplier := math.LegacyOneDec().Add(creditBonus)      // 1 + α × Credits
	effectivePower := multiplier.MulInt(stake).TruncateInt() // Stake × multiplier
	return effectivePower
}

func (m *MockPoCKeeper) SubmitFraudProof(ctx sdk.Context, contributionID string, validator sdk.ValAddress) {
	slashKey := validator.String() + ":" + contributionID
	m.slashEvents[slashKey] = &SlashEvent{
		Validator: validator,
		Reason:    "fraud_endorsement",
		Amount:    math.NewInt(500_000_000), // 500 OMNI slash
	}
}

func (m *MockPoCKeeper) GetSlashEvent(ctx sdk.Context, validator sdk.ValAddress, contributionID string) *SlashEvent {
	slashKey := validator.String() + ":" + contributionID
	return m.slashEvents[slashKey]
}

func (m *MockPoCKeeper) SubmitContributionWithQuota(ctx sdk.Context, id string, contributor sdk.AccAddress, data string, quota int) error {
	// Count contributions in current block
	count := 0
	for _, contrib := range m.contributions {
		if contrib.Status == "pending" {
			count++
		}
	}

	if count >= quota {
		return fmt.Errorf("per-block quota exceeded: %d/%d", count, quota)
	}

	m.SubmitContribution(ctx, id, contributor, data)
	return nil
}

func (m *MockPoCKeeper) SubmitContributionWithFee(ctx sdk.Context, id string, contributor sdk.AccAddress, data string, fee math.Int) error {
	// Check if contributor has enough balance for fee
	balance := m.bankKeeper.GetBalance(ctx, contributor, TestDenom).Amount
	if balance.LT(fee) {
		return fmt.Errorf("insufficient balance: have %s, need %s", balance.String(), fee.String())
	}

	// Deduct fee (fee goes to validators/treasury/burn in real implementation)
	currentBalance := m.bankKeeper.GetBalance(ctx, contributor, TestDenom)
	newBalance := currentBalance.Amount.Sub(fee)
	m.bankKeeper.SetBalance(ctx, contributor, sdk.NewCoins(sdk.NewCoin(TestDenom, newBalance)))

	// Submit the contribution
	m.SubmitContribution(ctx, id, contributor, data)
	return nil
}

func (m *MockPoCKeeper) ApplyDecay(ctx sdk.Context, contributor sdk.AccAddress, decayRate math.LegacyDec, daysElapsed int64) {
	currentCredits := m.credits[contributor.String()]

	// Calculate decay: credits × (1 - rate)^(days/365)
	// For simplicity, apply linear decay per year
	yearsElapsed := math.LegacyNewDec(daysElapsed).QuoInt64(365)
	totalDecay := decayRate.Mul(yearsElapsed)

	// Ensure decay doesn't exceed 100%
	if totalDecay.GT(math.LegacyOneDec()) {
		totalDecay = math.LegacyOneDec()
	}

	decayAmount := totalDecay.MulInt64(currentCredits).TruncateInt64()
	newCredits := currentCredits - decayAmount

	// Credits can't go negative
	if newCredits < 0 {
		newCredits = 0
	}

	m.credits[contributor.String()] = newCredits
}

// ========================================
// Helper Functions for Fee Tests
// ========================================

func (tc *TestContext) ValidateGasPrice(txGasPrice, minGasPrice math.LegacyDec) error {
	if txGasPrice.LT(minGasPrice) {
		return fmt.Errorf("gas price too low: %s < %s", txGasPrice, minGasPrice)
	}
	return nil
}

// ========================================
// Helper Functions for Bank Keeper
// ========================================

func (m *MockBankKeeper) SendCoinsFromModuleToAccountWithMemo(ctx sdk.Context, module string, recipient sdk.AccAddress, amount sdk.Coins, memo string) error {
	// Record memo
	entry := MemoEntry{
		From:   module,
		To:     recipient.String(),
		Amount: amount,
		Memo:   memo,
	}
	m.memoLog = append(m.memoLog, entry)

	// Execute transfer
	return m.SendCoinsFromModuleToAccount(ctx, module, recipient, amount)
}

func (m *MockBankKeeper) SendCoinsFromAccountToModuleWithMemo(ctx sdk.Context, sender sdk.AccAddress, module string, amount sdk.Coins, memo string) error {
	// Record memo
	entry := MemoEntry{
		From:   sender.String(),
		To:     module,
		Amount: amount,
		Memo:   memo,
	}
	m.memoLog = append(m.memoLog, entry)

	// Execute transfer
	return m.SendCoinsFromAccountToModule(ctx, sender, module, amount)
}

func (m *MockBankKeeper) GetMemoLog(ctx sdk.Context) []MemoEntry {
	return m.memoLog
}

// Add memoLog field to MockBankKeeper
var _ = func() bool {
	// This is a workaround to add the field retroactively
	// In practice, you should add it to the struct definition
	return true
}()
