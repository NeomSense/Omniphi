package performance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

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

const (
	TestDenom = "omniphi"
)

// PerformanceTestContext provides utilities for performance testing
type PerformanceTestContext struct {
	T                *testing.T
	Ctx              sdk.Context
	TokenomicsKeeper keeper.Keeper
	BankKeeper       *MockBankKeeper
	StakingKeeper    *MockStakingKeeper
	Cdc              moduletestutil.TestEncodingConfig

	// Performance metrics
	StartTime     time.Time
	BlockCount    int64
	TxCount       int64
	OperationLog  []OperationMetric
}

// OperationMetric tracks performance of individual operations
type OperationMetric struct {
	Name      string
	StartTime time.Time
	Duration  time.Duration
	Success   bool
	Error     error
}

// SetupPerformanceTest creates a test context optimized for performance testing
func SetupPerformanceTest(t *testing.T) *PerformanceTestContext {
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
	bankKeeper := NewMockBankKeeper()
	stakingKeeper := NewMockStakingKeeper()

	// Create tokenomics keeper
	tokenomicsKeeper := keeper.NewKeeper(
		cdc.Codec,
		runtime.NewKVStoreService(storeKey),
		log.NewNopLogger(),
		NewMockAccountKeeper(),
		bankKeeper,
		stakingKeeper,
		nil, // govKeeper (optional)
		nil, // ibcKeeper (optional)
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	// Set default params
	params := types.DefaultParams()
	require.NoError(t, tokenomicsKeeper.SetParams(ctx, params))

	return &PerformanceTestContext{
		T:                t,
		Ctx:              ctx,
		TokenomicsKeeper: tokenomicsKeeper,
		BankKeeper:       bankKeeper,
		StakingKeeper:    stakingKeeper,
		Cdc:              cdc,
		StartTime:        time.Now(),
		BlockCount:       0,
		TxCount:          0,
		OperationLog:     make([]OperationMetric, 0),
	}
}

// SimulateBlock advances the context to the next block
func (ptc *PerformanceTestContext) SimulateBlock() {
	ptc.BlockCount++
	ptc.Ctx = ptc.Ctx.WithBlockHeight(ptc.BlockCount)
	ptc.Ctx = ptc.Ctx.WithBlockTime(time.Now())
}

// SimulateBlocks advances multiple blocks efficiently
func (ptc *PerformanceTestContext) SimulateBlocks(n int64) {
	for i := int64(0); i < n; i++ {
		ptc.SimulateBlock()
	}
}

// RecordOperation logs a performance metric for an operation
func (ptc *PerformanceTestContext) RecordOperation(name string, duration time.Duration, success bool, err error) {
	metric := OperationMetric{
		Name:      name,
		StartTime: time.Now(),
		Duration:  duration,
		Success:   success,
		Error:     err,
	}
	ptc.OperationLog = append(ptc.OperationLog, metric)
}

// MeasureOperation executes a function and records its performance
func (ptc *PerformanceTestContext) MeasureOperation(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	ptc.RecordOperation(name, duration, err == nil, err)
	return err
}

// GetAverageDuration returns the average duration for operations with a given name
func (ptc *PerformanceTestContext) GetAverageDuration(name string) time.Duration {
	var total time.Duration
	count := 0

	for _, metric := range ptc.OperationLog {
		if metric.Name == name && metric.Success {
			total += metric.Duration
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / time.Duration(count)
}

// GetSuccessRate returns the success rate for operations with a given name
func (ptc *PerformanceTestContext) GetSuccessRate(name string) float64 {
	total := 0
	success := 0

	for _, metric := range ptc.OperationLog {
		if metric.Name == name {
			total++
			if metric.Success {
				success++
			}
		}
	}

	if total == 0 {
		return 0
	}

	return float64(success) / float64(total) * 100.0
}

// GeneratePerformanceReport creates a summary of performance metrics
func (ptc *PerformanceTestContext) GeneratePerformanceReport() string {
	elapsed := time.Since(ptc.StartTime)

	report := fmt.Sprintf(`
=== PERFORMANCE TEST REPORT ===

Duration: %s
Blocks Simulated: %d
Transactions: %d
Block Rate: %.2f blocks/sec
Transaction Rate: %.2f tx/sec

Operation Metrics:
`,
		elapsed,
		ptc.BlockCount,
		ptc.TxCount,
		float64(ptc.BlockCount)/elapsed.Seconds(),
		float64(ptc.TxCount)/elapsed.Seconds(),
	)

	// Group operations by name
	operationStats := make(map[string]struct {
		count    int
		success  int
		totalDur time.Duration
	})

	for _, metric := range ptc.OperationLog {
		stats := operationStats[metric.Name]
		stats.count++
		if metric.Success {
			stats.success++
			stats.totalDur += metric.Duration
		}
		operationStats[metric.Name] = stats
	}

	for name, stats := range operationStats {
		avgDur := time.Duration(0)
		if stats.success > 0 {
			avgDur = stats.totalDur / time.Duration(stats.success)
		}
		successRate := float64(stats.success) / float64(stats.count) * 100.0

		report += fmt.Sprintf("  %s: %d ops, %.1f%% success, avg %s\n",
			name, stats.count, successRate, avgDur)
	}

	return report
}

// AssertTPS verifies transactions per second meets minimum threshold
func (ptc *PerformanceTestContext) AssertTPS(minTPS float64, msgAndArgs ...interface{}) {
	elapsed := time.Since(ptc.StartTime)
	actualTPS := float64(ptc.TxCount) / elapsed.Seconds()

	require.GreaterOrEqual(ptc.T, actualTPS, minTPS,
		"TPS too low: expected >= %.2f, got %.2f: %v", minTPS, actualTPS, msgAndArgs)
}

// AssertBlockRate verifies blocks per second meets minimum threshold
func (ptc *PerformanceTestContext) AssertBlockRate(minBlocksPerSec float64, msgAndArgs ...interface{}) {
	elapsed := time.Since(ptc.StartTime)
	actualRate := float64(ptc.BlockCount) / elapsed.Seconds()

	require.GreaterOrEqual(ptc.T, actualRate, minBlocksPerSec,
		"Block rate too low: expected >= %.2f, got %.2f: %v", minBlocksPerSec, actualRate, msgAndArgs)
}

// AssertOperationPerformance verifies operation meets performance criteria
func (ptc *PerformanceTestContext) AssertOperationPerformance(
	name string,
	maxAvgDuration time.Duration,
	minSuccessRate float64,
	msgAndArgs ...interface{},
) {
	avgDur := ptc.GetAverageDuration(name)
	successRate := ptc.GetSuccessRate(name)

	require.LessOrEqual(ptc.T, avgDur, maxAvgDuration,
		"Operation '%s' too slow: expected <= %s, got %s: %v",
		name, maxAvgDuration, avgDur, msgAndArgs)

	require.GreaterOrEqual(ptc.T, successRate, minSuccessRate,
		"Operation '%s' success rate too low: expected >= %.1f%%, got %.1f%%: %v",
		name, minSuccessRate, successRate, msgAndArgs)
}

// ================================================================
// Mock Keepers (simplified versions for performance testing)
// ================================================================

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
	if acc, ok := m.accounts[addr.String()]; ok {
		return acc
	}
	return nil
}

func (m *MockAccountKeeper) GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI {
	// Return nil for performance tests - not needed
	return nil
}

func (m *MockAccountKeeper) SetAccount(ctx context.Context, acc sdk.AccountI) {
	// Store account for performance tests
	m.accounts[acc.GetAddress().String()] = acc
}

type MockBankKeeper struct {
	mu             sync.RWMutex
	balances       map[string]sdk.Coins
	supply         sdk.Coins
	burned         sdk.Coins
	minted         sdk.Coins
	moduleBalances map[string]sdk.Coins
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

func (m *MockBankKeeper) GetSupply(ctx context.Context, denom string) sdk.Coin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return sdk.NewCoin(denom, m.supply.AmountOf(denom))
}

func (m *MockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.supply = m.supply.Add(amt...)
	m.minted = m.minted.Add(amt...)

	modBalance := m.moduleBalances[moduleName]
	m.moduleBalances[moduleName] = modBalance.Add(amt...)

	return nil
}

func (m *MockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.supply = m.supply.Sub(amt...)
	m.burned = m.burned.Add(amt...)

	modBalance := m.moduleBalances[moduleName]
	m.moduleBalances[moduleName] = modBalance.Sub(amt...)

	return nil
}

func (m *MockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deduct from module
	modBalance := m.moduleBalances[senderModule]
	m.moduleBalances[senderModule] = modBalance.Sub(amt...)

	// Add to account
	accBalance := m.balances[recipientAddr.String()]
	m.balances[recipientAddr.String()] = accBalance.Add(amt...)

	return nil
}

func (m *MockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deduct from account
	accBalance := m.balances[senderAddr.String()]
	m.balances[senderAddr.String()] = accBalance.Sub(amt...)

	// Add to module
	modBalance := m.moduleBalances[recipientModule]
	m.moduleBalances[recipientModule] = modBalance.Add(amt...)

	return nil
}

func (m *MockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if coins, ok := m.balances[addr.String()]; ok {
		return sdk.NewCoin(denom, coins.AmountOf(denom))
	}
	return sdk.NewCoin(denom, math.ZeroInt())
}

func (m *MockBankKeeper) GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if coins, ok := m.balances[addr.String()]; ok {
		return coins
	}
	return sdk.NewCoins()
}

func (m *MockBankKeeper) SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deduct from sender module
	senderBalance := m.moduleBalances[senderModule]
	m.moduleBalances[senderModule] = senderBalance.Sub(amt...)

	// Add to recipient module
	recipientBalance := m.moduleBalances[recipientModule]
	m.moduleBalances[recipientModule] = recipientBalance.Add(amt...)

	return nil
}

func (m *MockBankKeeper) SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	// For performance tests, all coins are spendable
	return m.GetAllBalances(ctx, addr)
}

func (m *MockBankKeeper) SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deduct from sender
	senderBalance := m.balances[fromAddr.String()]
	m.balances[fromAddr.String()] = senderBalance.Sub(amt...)

	// Add to recipient
	recipientBalance := m.balances[toAddr.String()]
	m.balances[toAddr.String()] = recipientBalance.Add(amt...)

	return nil
}

func (m *MockBankKeeper) GetModuleBalance(moduleName string) sdk.Coins {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if coins, ok := m.moduleBalances[moduleName]; ok {
		return coins
	}
	return sdk.NewCoins()
}

type MockStakingKeeper struct {
	bondedRatio math.LegacyDec
}

func NewMockStakingKeeper() *MockStakingKeeper {
	return &MockStakingKeeper{
		bondedRatio: math.LegacyNewDecWithPrec(67, 2), // 67% bonded
	}
}

func (m *MockStakingKeeper) BondedRatio(ctx context.Context) (math.LegacyDec, error) {
	return m.bondedRatio, nil
}

func (m *MockStakingKeeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	// For testing: assume 250M bonded out of 375M total
	return math.NewInt(250_000_000_000_000), nil
}

func (m *MockStakingKeeper) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	// Return empty validator list for performance tests
	return []stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error) {
	// Return empty validator list for performance tests
	return []stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	// Return empty validator for performance tests
	return stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	// Standard power reduction factor (1 token = 1 power)
	return math.NewInt(1_000_000)
}
