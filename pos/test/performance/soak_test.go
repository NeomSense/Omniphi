package performance

import (
	"fmt"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

// TestSoak_10kBlocks_SupplyConservation validates supply conservation over 10,000 blocks
//
// Priority: P0
// Purpose: Verify no drift/accumulation errors over extended runtime
// Duration: ~30-60 seconds
// Validates: Total Supply = Minted - Burned (conservation law)
func TestSoak_10kBlocks_SupplyConservation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	ptc := SetupPerformanceTest(t)

	t.Log("=== SOAK TEST: 10k Blocks - Supply Conservation ===")

	// Initial state
	initialSupply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount
	totalMinted := math.ZeroInt()
	totalBurned := math.ZeroInt()

	// Run for 10,000 blocks
	blocks := int64(10000)
	t.Logf("Simulating %d blocks...", blocks)

	for block := int64(1); block <= blocks; block++ {
		ptc.SimulateBlock()

		// Every 10 blocks: mint some tokens
		if block%10 == 0 {
			mintAmount := math.NewInt(1_000_000) // 1 OMNI
			err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, mintAmount)))
			require.NoError(t, err)
			totalMinted = totalMinted.Add(mintAmount)
		}

		// Every 15 blocks: burn some tokens
		if block%15 == 0 {
			burnAmount := math.NewInt(500_000) // 0.5 OMNI
			err := ptc.BankKeeper.BurnCoins(ptc.Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, burnAmount)))
			require.NoError(t, err)
			totalBurned = totalBurned.Add(burnAmount)
		}

		// Every 1000 blocks: verify conservation law
		if block%1000 == 0 {
			currentSupply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount
			expectedSupply := initialSupply.Add(totalMinted).Sub(totalBurned)

			require.Equal(t, expectedSupply.String(), currentSupply.String(),
				"Supply conservation violated at block %d", block)

			t.Logf("  Block %d: Supply=%s, Minted=%s, Burned=%s ✓",
				block,
				formatAmount(currentSupply),
				formatAmount(totalMinted),
				formatAmount(totalBurned))
		}
	}

	// Final verification
	finalSupply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount
	expectedFinalSupply := initialSupply.Add(totalMinted).Sub(totalBurned)

	require.Equal(t, expectedFinalSupply.String(), finalSupply.String(),
		"Final supply conservation check failed")

	t.Logf("✓ Supply conservation maintained over %d blocks", blocks)
	t.Logf("✓ Final supply: %s OMNI", formatAmount(finalSupply))
	t.Logf("✓ Total minted: %s OMNI", formatAmount(totalMinted))
	t.Logf("✓ Total burned: %s OMNI", formatAmount(totalBurned))
	t.Logf("✓ Net change: %s OMNI", formatAmount(totalMinted.Sub(totalBurned)))
}

// TestSoak_10kBlocks_InflationAccumulation validates inflation calculations over time
//
// Priority: P0
// Purpose: Verify inflation accumulates correctly without rounding errors
// Duration: ~30-60 seconds
func TestSoak_10kBlocks_InflationAccumulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	ptc := SetupPerformanceTest(t)

	t.Log("=== SOAK TEST: 10k Blocks - Inflation Accumulation ===")

	// Set inflation parameters
	params := types.DefaultParams()
	params.InflationRate = math.LegacyNewDecWithPrec(3, 2) // 3% annual
	params.CurrentTotalSupply = math.NewInt(375_000_000_000_000) // 375M OMNI
	err := ptc.TokenomicsKeeper.SetParams(ptc.Ctx, params)
	require.NoError(t, err)

	initialSupply := params.CurrentTotalSupply
	blocksPerYear := int64(365 * 24 * 3600 / 7) // ~7 second blocks

	// Run for 10,000 blocks
	blocks := int64(10000)
	t.Logf("Simulating %d blocks (%.2f%% of a year)...", blocks, float64(blocks)/float64(blocksPerYear)*100)

	accumulatedInflation := math.ZeroInt()

	for block := int64(1); block <= blocks; block++ {
		ptc.SimulateBlock()

		// Calculate block inflation
		// blockInflation = totalSupply * inflationRate / blocksPerYear
		currentSupply := ptc.TokenomicsKeeper.GetParams(ptc.Ctx).CurrentTotalSupply
		blockInflationDec := params.InflationRate.MulInt(currentSupply).QuoInt64(blocksPerYear)
		blockInflation := blockInflationDec.TruncateInt()

		if blockInflation.GT(math.ZeroInt()) {
			// Mint inflation
			err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName,
				sdk.NewCoins(sdk.NewCoin(TestDenom, blockInflation)))
			require.NoError(t, err)

			accumulatedInflation = accumulatedInflation.Add(blockInflation)

			// Update params
			params.CurrentTotalSupply = currentSupply.Add(blockInflation)
			params.TotalMinted = params.TotalMinted.Add(blockInflation)
			err = ptc.TokenomicsKeeper.SetParams(ptc.Ctx, params)
			require.NoError(t, err)
		}

		// Every 1000 blocks: report progress
		if block%1000 == 0 {
			currentSupply = ptc.TokenomicsKeeper.GetParams(ptc.Ctx).CurrentTotalSupply
			actualIncrease := currentSupply.Sub(initialSupply)

			t.Logf("  Block %d: Supply=%s, Inflated=%s ✓",
				block,
				formatAmount(currentSupply),
				formatAmount(actualIncrease))
		}
	}

	// Final verification
	finalParams := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)
	finalSupply := finalParams.CurrentTotalSupply
	actualIncrease := finalSupply.Sub(initialSupply)

	// Expected inflation for this period
	// expectedInflation = initialSupply * inflationRate * (blocks / blocksPerYear)
	expectedInflationDec := params.InflationRate.
		MulInt(initialSupply).
		MulInt64(blocks).
		QuoInt64(blocksPerYear)
	expectedInflation := expectedInflationDec.TruncateInt()

	// Allow small rounding difference (< 0.1%)
	diff := actualIncrease.Sub(expectedInflation).Abs()
	maxDiff := expectedInflation.QuoRaw(1000) // 0.1%

	require.True(t, diff.LTE(maxDiff),
		"Inflation drift too large: expected %s, got %s, diff %s (max %s)",
		formatAmount(expectedInflation),
		formatAmount(actualIncrease),
		formatAmount(diff),
		formatAmount(maxDiff))

	t.Logf("✓ Inflation accumulated correctly over %d blocks", blocks)
	t.Logf("✓ Initial supply: %s OMNI", formatAmount(initialSupply))
	t.Logf("✓ Final supply: %s OMNI", formatAmount(finalSupply))
	t.Logf("✓ Expected inflation: %s OMNI", formatAmount(expectedInflation))
	t.Logf("✓ Actual inflation: %s OMNI", formatAmount(actualIncrease))
	t.Logf("✓ Drift: %s OMNI (%.6f%%)", formatAmount(diff), float64(diff.Int64())/float64(expectedInflation.Int64())*100)
}

// TestSoak_25kBlocks_ParamUpdates validates parameter updates work correctly over time
//
// Priority: P1
// Purpose: Verify parameter updates don't cause drift or corruption
// Duration: ~60-90 seconds
func TestSoak_25kBlocks_ParamUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	ptc := SetupPerformanceTest(t)

	t.Log("=== SOAK TEST: 25k Blocks - Parameter Updates ===")

	blocks := int64(25000)
	t.Logf("Simulating %d blocks with frequent param updates...", blocks)

	paramUpdateCount := 0

	for block := int64(1); block <= blocks; block++ {
		ptc.SimulateBlock()

		// Every 100 blocks: update a parameter
		if block%100 == 0 {
			params := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)

			// Cycle through different inflation rates
			switch (block / 100) % 5 {
			case 0:
				params.InflationRate = math.LegacyNewDecWithPrec(1, 2) // 1%
			case 1:
				params.InflationRate = math.LegacyNewDecWithPrec(2, 2) // 2%
			case 2:
				params.InflationRate = math.LegacyNewDecWithPrec(3, 2) // 3%
			case 3:
				params.InflationRate = math.LegacyNewDecWithPrec(4, 2) // 4%
			case 4:
				params.InflationRate = math.LegacyNewDecWithPrec(5, 2) // 5%
			}

			err := ptc.TokenomicsKeeper.SetParams(ptc.Ctx, params)
			require.NoError(t, err)
			paramUpdateCount++
		}

		// Every 1000 blocks: verify params are still valid
		if block%1000 == 0 {
			params := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)
			err := params.Validate()
			require.NoError(t, err, "Params invalid at block %d", block)

			t.Logf("  Block %d: Inflation=%.2f%%, Updates=%d ✓",
				block,
				params.InflationRate.MustFloat64()*100,
				paramUpdateCount)
		}
	}

	// Final validation
	finalParams := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)
	err := finalParams.Validate()
	require.NoError(t, err, "Final params invalid")

	t.Logf("✓ Completed %d blocks with %d parameter updates", blocks, paramUpdateCount)
	t.Logf("✓ Final params valid")
	t.Logf("✓ No param corruption detected")
}

// TestSoak_50kBlocks_ExtendedStability validates extreme long-term stability
//
// Priority: P2
// Purpose: Stress test for very long-running chains
// Duration: ~2-3 minutes
func TestSoak_50kBlocks_ExtendedStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping extended soak test in short mode")
	}

	ptc := SetupPerformanceTest(t)

	t.Log("=== SOAK TEST: 50k Blocks - Extended Stability ===")

	blocks := int64(50000)
	t.Logf("Simulating %d blocks (this may take 2-3 minutes)...", blocks)

	// Setup initial supply for burns
	initialSupply := math.NewInt(1_000_000_000_000) // 1M OMNI
	err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))
	require.NoError(t, err)

	// Track various metrics
	mintCount := 0
	burnCount := 0
	paramUpdateCount := 0

	for block := int64(1); block <= blocks; block++ {
		ptc.SimulateBlock()

		// Varied operations
		switch block % 7 {
		case 0: // Mint
			err := ptc.BankKeeper.MintCoins(ptc.Ctx, types.ModuleName,
				sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(100_000))))
			require.NoError(t, err)
			mintCount++

		case 1: // Burn
			err := ptc.BankKeeper.BurnCoins(ptc.Ctx, types.ModuleName,
				sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(50_000))))
			require.NoError(t, err)
			burnCount++

		case 2: // Param update
			params := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)
			params.InflationRate = math.LegacyNewDecWithPrec(int64(2+(block%3)), 2)
			err := ptc.TokenomicsKeeper.SetParams(ptc.Ctx, params)
			require.NoError(t, err)
			paramUpdateCount++
		}

		// Progress report every 5000 blocks
		if block%5000 == 0 {
			supply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount
			params := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)

			t.Logf("  Block %d/%d: Supply=%s, Mints=%d, Burns=%d, ParamUpdates=%d",
				block, blocks,
				formatAmount(supply),
				mintCount, burnCount, paramUpdateCount)

			// Validate params
			err := params.Validate()
			require.NoError(t, err, "Params invalid at block %d", block)
		}
	}

	// Final validation
	finalSupply := ptc.BankKeeper.GetSupply(ptc.Ctx, TestDenom).Amount
	finalParams := ptc.TokenomicsKeeper.GetParams(ptc.Ctx)
	errValidate := finalParams.Validate()
	require.NoError(t, errValidate)

	t.Logf("✓ Completed %d blocks successfully", blocks)
	t.Logf("✓ Final supply: %s OMNI", formatAmount(finalSupply))
	t.Logf("✓ Total mints: %d", mintCount)
	t.Logf("✓ Total burns: %d", burnCount)
	t.Logf("✓ Total param updates: %d", paramUpdateCount)
	t.Logf("✓ No stability issues detected")
}

// formatAmount formats a math.Int as OMNI with appropriate scaling
func formatAmount(amount math.Int) string {
	if amount.IsZero() {
		return "0"
	}

	// Convert to OMNI (6 decimals)
	divisor := math.NewInt(1_000_000)
	omni := amount.Quo(divisor)

	// For large numbers, use scientific notation
	if omni.GT(math.NewInt(1_000_000)) {
		millions := float64(omni.Int64()) / 1_000_000.0
		return fmt.Sprintf("%.2fM", millions)
	}

	return omni.String()
}
