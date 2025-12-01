package performance

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

// TestTPS_MintOperations measures transactions per second for mint operations
//
// Priority: P0
// Target: >= 100 TPS for basic mint operations
// Duration: ~10 seconds
func TestTPS_MintOperations(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== TPS TEST: Mint Operations ===")

	// Target metrics
	targetTPS := 100.0
	iterations := 1000

	// Execute mint operations
	for i := 0; i < iterations; i++ {
		err := ptc.MeasureOperation("mint", func() error {
			mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000))) // 1 OMNI
			return ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, mintAmount)
		})
		require.NoError(t, err)
		ptc.TxCount++
	}

	// Generate report
	report := ptc.GeneratePerformanceReport()
	t.Log(report)

	// Assert performance
	ptc.AssertTPS(targetTPS, "Mint operations should achieve >= 100 TPS")
	ptc.AssertOperationPerformance("mint", 1*time.Millisecond, 100.0)

	elapsed := time.Since(ptc.StartTime)
	actualTPS := float64(ptc.TxCount) / elapsed.Seconds()

	t.Logf("✓ Mint TPS: %.2f (target: >= %.2f)", actualTPS, targetTPS)
	t.Logf("✓ Total operations: %d in %s", iterations, elapsed)
}

// TestTPS_BurnOperations measures transactions per second for burn operations
//
// Priority: P0
// Target: >= 100 TPS for burn operations
// Duration: ~10 seconds
func TestTPS_BurnOperations(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== TPS TEST: Burn Operations ===")

	// Setup: Mint initial supply
	initialMint := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000_000_000))) // 1M OMNI
	err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, initialMint)
	require.NoError(t, err)

	// Target metrics
	targetTPS := 100.0
	iterations := 1000

	// Execute burn operations
	for i := 0; i < iterations; i++ {
		err := ptc.MeasureOperation("burn", func() error {
			burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000))) // 1 OMNI
			return ptc.BankKeeper.BurnCoins(ptc.Ctx, types.ModuleName, burnAmount)
		})
		require.NoError(t, err)
		ptc.TxCount++
	}

	// Generate report
	report := ptc.GeneratePerformanceReport()
	t.Log(report)

	// Assert performance
	ptc.AssertTPS(targetTPS, "Burn operations should achieve >= 100 TPS")
	ptc.AssertOperationPerformance("burn", 1*time.Millisecond, 100.0)

	elapsed := time.Since(ptc.StartTime)
	actualTPS := float64(ptc.TxCount) / elapsed.Seconds()

	t.Logf("✓ Burn TPS: %.2f (target: >= %.2f)", actualTPS, targetTPS)
	t.Logf("✓ Total operations: %d in %s", iterations, elapsed)
}

// TestTPS_TransferOperations measures transactions per second for transfers
//
// Priority: P0
// Target: >= 100 TPS for transfer operations
// Duration: ~10 seconds
func TestTPS_TransferOperations(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== TPS TEST: Transfer Operations ===")

	// Setup: Create test accounts and fund them
	sender := sdk.AccAddress("sender______________")
	receiver := sdk.AccAddress("receiver____________")

	initialBalance := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000_000_000))) // 1M OMNI
	err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, initialBalance)
	require.NoError(t, err)

	err = ptc.BankKeeper.SendCoinsFromModuleToAccount(ptc.Ctx, types.ModuleName, sender, initialBalance)
	require.NoError(t, err)

	// Target metrics
	targetTPS := 100.0
	iterations := 1000

	// Execute transfer operations
	for i := 0; i < iterations; i++ {
		err := ptc.MeasureOperation("transfer", func() error {
			transferAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000))) // 1 OMNI
			// Transfer from sender to module, then to receiver
			err := ptc.BankKeeper.SendCoinsFromAccountToModule(ptc.Ctx, sender, types.ModuleName, transferAmount)
			if err != nil {
				return err
			}
			return ptc.BankKeeper.SendCoinsFromModuleToAccount(ptc.Ctx, types.ModuleName, receiver, transferAmount)
		})
		require.NoError(t, err)
		ptc.TxCount++
	}

	// Generate report
	report := ptc.GeneratePerformanceReport()
	t.Log(report)

	// Assert performance
	ptc.AssertTPS(targetTPS, "Transfer operations should achieve >= 100 TPS")
	ptc.AssertOperationPerformance("transfer", 1*time.Millisecond, 100.0)

	elapsed := time.Since(ptc.StartTime)
	actualTPS := float64(ptc.TxCount) / elapsed.Seconds()

	t.Logf("✓ Transfer TPS: %.2f (target: >= %.2f)", actualTPS, targetTPS)
	t.Logf("✓ Total operations: %d in %s", iterations, elapsed)
}

// TestTPS_MixedOperations measures TPS for a mix of operations (realistic workload)
//
// Priority: P1
// Target: >= 80 TPS for mixed operations
// Duration: ~15 seconds
func TestTPS_MixedOperations(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== TPS TEST: Mixed Operations (Realistic Workload) ===")

	// Setup: Create test accounts
	accounts := make([]sdk.AccAddress, 10)
	for i := 0; i < 10; i++ {
		accounts[i] = sdk.AccAddress([]byte("account" + string(rune('0'+i)) + "_______"))
	}

	// Fund accounts
	initialBalance := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(100_000_000_000))) // 100k OMNI
	for _, acc := range accounts {
		err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, initialBalance)
		require.NoError(t, err)
		err = ptc.BankKeeper.SendCoinsFromModuleToAccount(ptc.Ctx, types.ModuleName, acc, initialBalance)
		require.NoError(t, err)
	}

	// Target metrics
	targetTPS := 80.0
	iterations := 1200

	// Execute mixed operations (40% mint, 30% burn, 30% transfer)
	for i := 0; i < iterations; i++ {
		operation := i % 10

		var err error
		switch {
		case operation < 4: // 40% mints
			err = ptc.MeasureOperation("mint", func() error {
				mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(500_000)))
				return ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, mintAmount)
			})

		case operation < 7: // 30% burns
			err = ptc.MeasureOperation("burn", func() error {
				burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(500_000)))
				return ptc.BankKeeper.BurnCoins(ptc.Ctx, types.ModuleName, burnAmount)
			})

		default: // 30% transfers
			sender := accounts[i%10]
			err = ptc.MeasureOperation("transfer", func() error {
				transferAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(100_000)))
				return ptc.BankKeeper.SendCoinsFromAccountToModule(ptc.Ctx, sender, types.ModuleName, transferAmount)
			})
		}

		require.NoError(t, err)
		ptc.TxCount++
	}

	// Generate report
	report := ptc.GeneratePerformanceReport()
	t.Log(report)

	// Assert performance
	ptc.AssertTPS(targetTPS, "Mixed operations should achieve >= 80 TPS")

	elapsed := time.Since(ptc.StartTime)
	actualTPS := float64(ptc.TxCount) / elapsed.Seconds()

	t.Logf("✓ Mixed workload TPS: %.2f (target: >= %.2f)", actualTPS, targetTPS)
	t.Logf("✓ Operation breakdown:")
	t.Logf("    Mint:     %.1f%% success, avg %s", ptc.GetSuccessRate("mint"), ptc.GetAverageDuration("mint"))
	t.Logf("    Burn:     %.1f%% success, avg %s", ptc.GetSuccessRate("burn"), ptc.GetAverageDuration("burn"))
	t.Logf("    Transfer: %.1f%% success, avg %s", ptc.GetSuccessRate("transfer"), ptc.GetAverageDuration("transfer"))
}

// BenchmarkMintOperation benchmarks single mint operation
func BenchmarkMintOperation(b *testing.B) {
	ptc := SetupPerformanceTest(&testing.T{})

	mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, mintAmount)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBurnOperation benchmarks single burn operation
func BenchmarkBurnOperation(b *testing.B) {
	ptc := SetupPerformanceTest(&testing.T{})

	// Setup: Mint initial supply
	initialMint := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(int64(b.N)*1_000_000)))
	err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, initialMint)
	if err != nil {
		b.Fatal(err)
	}

	burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ptc.BankKeeper.BurnCoins(ptc.Ctx, types.ModuleName, burnAmount)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParamUpdate benchmarks parameter update operations
func BenchmarkParamUpdate(b *testing.B) {
	ptc := SetupPerformanceTest(&testing.T{})

	params := types.DefaultParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate inflation rate to trigger updates
		if i%2 == 0 {
			params.InflationRate = math.LegacyNewDecWithPrec(3, 2) // 3%
		} else {
			params.InflationRate = math.LegacyNewDecWithPrec(4, 2) // 4%
		}

		err := ptc.TokenomicsKeeper.SetParams(ptc.Ctx, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}
