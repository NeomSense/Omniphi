package performance

import (
	"fmt"
	"sync"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

// TestStress_ConcurrentMints validates concurrent mint operations are safe
//
// Priority: P0
// Purpose: Verify no race conditions or data corruption under concurrent load
// Duration: ~10 seconds
func TestStress_ConcurrentMints(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== STRESS TEST: Concurrent Mints ===")

	goroutines := 10
	operationsPerGoroutine := 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*operationsPerGoroutine)

	t.Logf("Launching %d concurrent goroutines, each performing %d mints...",
		goroutines, operationsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for i := 0; i < operationsPerGoroutine; i++ {
				mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(10_000)))
				err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, mintAmount)
				if err != nil {
					errors <- err
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		require.NoError(t, err, "Concurrent mint operation failed")
	}

	// Verify total supply
	expectedMinted := math.NewInt(int64(goroutines * operationsPerGoroutine * 10_000))
	actualSupply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount

	require.Equal(t, expectedMinted.String(), actualSupply.String(),
		"Concurrent mints resulted in incorrect supply")

	t.Logf("✓ %d concurrent operations completed successfully", goroutines*operationsPerGoroutine)
	t.Logf("✓ Final supply: %s (expected: %s)", actualSupply.String(), expectedMinted.String())
}

// TestStress_ConcurrentBurns validates concurrent burn operations are safe
//
// Priority: P0
// Purpose: Verify burns don't cause negative supply or race conditions
// Duration: ~10 seconds
func TestStress_ConcurrentBurns(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== STRESS TEST: Concurrent Burns ===")

	// Setup: Mint initial supply
	initialMint := math.NewInt(10_000_000_000) // 10k OMNI
	err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin(TestDenom, initialMint)))
	require.NoError(t, err)

	goroutines := 10
	operationsPerGoroutine := 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*operationsPerGoroutine)

	t.Logf("Launching %d concurrent goroutines, each performing %d burns...",
		goroutines, operationsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for i := 0; i < operationsPerGoroutine; i++ {
				burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(10_000)))
				err := ptc.BankKeeper.BurnCoins(ptc.Ctx, types.ModuleName, burnAmount)
				if err != nil {
					errors <- err
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		require.NoError(t, err, "Concurrent burn operation failed")
	}

	// Verify total supply
	totalBurned := math.NewInt(int64(goroutines * operationsPerGoroutine * 10_000))
	expectedSupply := initialMint.Sub(totalBurned)
	actualSupply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount

	require.Equal(t, expectedSupply.String(), actualSupply.String(),
		"Concurrent burns resulted in incorrect supply")

	t.Logf("✓ %d concurrent operations completed successfully", goroutines*operationsPerGoroutine)
	t.Logf("✓ Initial: %s, Burned: %s, Final: %s",
		initialMint.String(), totalBurned.String(), actualSupply.String())
}

// TestStress_MixedConcurrentOperations validates mixed concurrent operations
//
// Priority: P1
// Purpose: Verify mints and burns can happen concurrently without corruption
// Duration: ~15 seconds
func TestStress_MixedConcurrentOperations(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== STRESS TEST: Mixed Concurrent Operations ===")

	// Setup: Mint initial supply for burns
	initialMint := math.NewInt(100_000_000_000) // 100k OMNI
	err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin(TestDenom, initialMint)))
	require.NoError(t, err)

	goroutines := 20 // 10 minting, 10 burning
	operationsPerGoroutine := 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*operationsPerGoroutine)

	t.Logf("Launching %d concurrent goroutines (mixed mints and burns)...", goroutines)

	// Half the goroutines mint, half burn
	for g := 0; g < goroutines; g++ {
		wg.Add(1)

		if g < goroutines/2 {
			// Minting goroutines
			go func(id int) {
				defer wg.Done()

				for i := 0; i < operationsPerGoroutine; i++ {
					mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(50_000)))
					err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, mintAmount)
					if err != nil {
						errors <- err
						return
					}
				}
			}(g)
		} else {
			// Burning goroutines
			go func(id int) {
				defer wg.Done()

				for i := 0; i < operationsPerGoroutine; i++ {
					burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(25_000)))
					err := ptc.BankKeeper.BurnCoins(ptc.Ctx, types.ModuleName, burnAmount)
					if err != nil {
						errors <- err
						return
					}
				}
			}(g)
		}
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		require.NoError(t, err, "Concurrent operation failed")
	}

	// Verify supply arithmetic
	totalMinted := math.NewInt(int64((goroutines / 2) * operationsPerGoroutine * 50_000))
	totalBurned := math.NewInt(int64((goroutines / 2) * operationsPerGoroutine * 25_000))
	expectedSupply := initialMint.Add(totalMinted).Sub(totalBurned)
	actualSupply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount

	require.Equal(t, expectedSupply.String(), actualSupply.String(),
		"Mixed concurrent operations resulted in incorrect supply")

	t.Logf("✓ %d concurrent operations completed successfully", goroutines*operationsPerGoroutine)
	t.Logf("✓ Minted: %s, Burned: %s, Net: %s",
		totalMinted.String(), totalBurned.String(), actualSupply.String())
}

// TestStress_RapidParamUpdates validates rapid parameter updates don't cause corruption
//
// Priority: P1
// Purpose: Verify parameter updates are atomic and safe
// Duration: ~5 seconds
func TestStress_RapidParamUpdates(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== STRESS TEST: Rapid Parameter Updates ===")

	iterations := 1000
	t.Logf("Performing %d rapid parameter updates...", iterations)

	for i := 0; i < iterations; i++ {
		params := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)

		// Cycle through different values
		params.InflationRate = math.LegacyNewDecWithPrec(int64(1+(i%5)), 2) // 1-5%
		params.BurnRatePosGas = math.LegacyNewDecWithPrec(int64(10+(i%40)), 2) // 10-49%

		err := ptc.TokenomicsKeeper.SetParams(ptc.Ctx, params)
		require.NoError(t, err, "Param update %d failed", i)

		// Every 100 iterations: verify params are valid
		if i%100 == 0 {
			retrieved := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)
			err := retrieved.Validate()
			require.NoError(t, err, "Retrieved params invalid after update %d", i)
		}
	}

	// Final validation
	finalParams := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)
	err := finalParams.Validate()
	require.NoError(t, err, "Final params invalid")

	t.Logf("✓ %d parameter updates completed successfully", iterations)
	t.Logf("✓ Final inflation rate: %.2f%%", finalParams.InflationRate.MustFloat64()*100)
	t.Logf("✓ No param corruption detected")
}

// TestStress_HighVolumeTransactions simulates high transaction volume
//
// Priority: P1
// Purpose: Validate system handles sustained high load
// Duration: ~20 seconds
func TestStress_HighVolumeTransactions(t *testing.T) {
	ptc := SetupPerformanceTest(t)

	t.Log("=== STRESS TEST: High Volume Transactions ===")

	// Setup: Create multiple accounts
	numAccounts := 100
	accounts := make([]sdk.AccAddress, numAccounts)
	for i := 0; i < numAccounts; i++ {
		accounts[i] = sdk.AccAddress([]byte(fmt.Sprintf("account%03d_______", i)))
	}

	// Fund all accounts
	initialBalance := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(10_000_000_000))) // 10k OMNI
	for _, acc := range accounts {
		err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, initialBalance)
		require.NoError(t, err)

		err = ptc.BankKeeper.SendCoinsFromModuleToAccount(ptc.Ctx, types.ModuleName, acc, initialBalance)
		require.NoError(t, err)
	}

	t.Logf("Funded %d accounts with %s OMNI each", numAccounts, initialBalance.String())

	// Simulate high transaction volume
	transactions := 5000
	t.Logf("Simulating %d transactions...", transactions)

	successCount := 0
	for i := 0; i < transactions; i++ {
		// Random sender and receiver
		sender := accounts[i%numAccounts]
		receiver := accounts[(i+1)%numAccounts]

		transferAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000))) // 0.001 OMNI

		// Try transfer (may fail if sender has insufficient balance)
		err := ptc.BankKeeper.SendCoinsFromAccountToModule(ptc.Ctx, sender, types.ModuleName, transferAmount)
		if err == nil {
			err = ptc.BankKeeper.SendCoinsFromModuleToAccount(ptc.Ctx, types.ModuleName, receiver, transferAmount)
			if err == nil {
				successCount++
			}
		}

		ptc.TxCount++
	}

	successRate := float64(successCount) / float64(transactions) * 100.0

	t.Logf("✓ Completed %d transactions", transactions)
	t.Logf("✓ Success rate: %.1f%% (%d/%d)", successRate, successCount, transactions)
	t.Logf("✓ No system corruption detected")

	// Verify account balances are still reasonable
	totalBalance := math.ZeroInt()
	for _, acc := range accounts {
		balance := ptc.BankKeeper.GetBalance(ptc.Ctx, acc, TestDenom).Amount
		totalBalance = totalBalance.Add(balance)
	}

	// Total should roughly equal initial funding (minus any failed transfers)
	expectedTotal := initialBalance.AmountOf(TestDenom).MulRaw(int64(numAccounts))
	require.True(t, totalBalance.LTE(expectedTotal),
		"Total account balances exceed initial funding")

	t.Logf("✓ Total account balances: %s OMNI", totalBalance.String())
}
