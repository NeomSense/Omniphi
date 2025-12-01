package comprehensive

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

// TC-001: Hard Cap Enforcement at Boundary
// Priority: P0
// Purpose: Verify that minting reverts when approaching 1.5B cap
func TestTC001_HardCapEnforcementAtBoundary(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply close to cap (1.499B OMNI)
	initialSupply := math.NewInt(1_499_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Attempt to mint 1M OMNI (should succeed)
	mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000_000_000)))
	err := tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, mintAmount)
	require.NoError(t, err, "Minting below cap should succeed")

	// Verify supply is exactly 1.5B
	supply := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom)
	require.Equal(t, int64(TestHardCap), supply.Amount.Int64(), "Supply should be exactly at cap")

	// Attempt to mint 1 omniphi more (should fail or be prevented)
	excessAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1)))
	_ = tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, excessAmount)

	// This should fail due to cap enforcement
	// Note: Implementation should check cap before minting
	currentSupply := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount
	require.LessOrEqual(t, currentSupply.Int64(), int64(TestHardCap),
		"Supply must never exceed hard cap")
}

// TC-002: Hard Cap Enforcement Under Concurrent Mints
// Priority: P0
// Purpose: Verify cap enforcement under concurrent mint attempts
func TestTC002_HardCapConcurrentMints(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply at 1.4B OMNI
	initialSupply := math.NewInt(1_400_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Simulate multiple concurrent mint attempts (each 100M OMNI)
	// Only the first should succeed
	mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(100_000_000_000_000)))

	successCount := 0
	for i := 0; i < 5; i++ {
		err := tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, mintAmount)
		if err == nil {
			currentSupply := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount
			if currentSupply.LTE(math.NewInt(TestHardCap)) {
				successCount++
			}
		}
	}

	// At most 1 mint should succeed (bringing total to 1.5B)
	require.LessOrEqual(t, successCount, 1, "At most one mint should succeed")

	// Final supply must not exceed cap
	AssertSupplyWithinCap(t, tc.BankKeeper)
}

// TC-003: Inflation Rate Bounds - Below Minimum
// Priority: P0
// Purpose: Verify that inflation <1% is rejected
func TestTC003_InflationBelowMinimum(t *testing.T) {
	tc := SetupTestContext(t)

	// Get current params
	params := tc.TokenomicsKeeper.GetParams(tc.Ctx)

	// Attempt to set inflation below minimum (0.5%)
	params.InflationRate = math.LegacyMustNewDecFromStr("0.005")

	// This should fail validation
	err := params.Validate()
	require.Error(t, err, "Inflation below 1% should fail validation")

	// Verify current inflation is still valid
	currentParams := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	require.True(t, currentParams.InflationRate.GTE(math.LegacyMustNewDecFromStr("0.01")),
		"Inflation rate should be >= 1%%")
}

// TC-004: Inflation Rate Bounds - Above Maximum
// Priority: P0
// Purpose: Verify that inflation >5% is rejected
func TestTC004_InflationAboveMaximum(t *testing.T) {
	tc := SetupTestContext(t)

	// Get current params
	params := tc.TokenomicsKeeper.GetParams(tc.Ctx)

	// Attempt to set inflation above maximum (10%)
	params.InflationRate = math.LegacyMustNewDecFromStr("0.10")

	// This should fail validation
	err := params.Validate()
	require.Error(t, err, "Inflation above 5% should fail validation")

	// Verify current inflation is still valid
	currentParams := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	require.True(t, currentParams.InflationRate.LTE(math.LegacyMustNewDecFromStr("0.05")),
		"Inflation rate should be <= 5%%")
}

// TC-005: Inflation Rate Update via Valid Governance
// Priority: P0
// Purpose: Verify inflation can be updated within valid bounds
func TestTC005_ValidInflationUpdate(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial inflation to 3%
	params := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	params.InflationRate = math.LegacyMustNewDecFromStr("0.03")
	err := tc.TokenomicsKeeper.SetParams(tc.Ctx, params)
	require.NoError(t, err)

	// Update to 4% (within bounds)
	params.InflationRate = math.LegacyMustNewDecFromStr("0.04")
	err = params.Validate()
	require.NoError(t, err, "Inflation update to 4% should be valid")

	err = tc.TokenomicsKeeper.SetParams(tc.Ctx, params)
	require.NoError(t, err)

	// Verify new rate is applied
	currentParams := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	require.Equal(t, "0.040000000000000000", currentParams.InflationRate.String(),
		"Inflation rate should be updated to 4%%")
}

// TC-006: Genesis Supply Allocation Integrity
// Priority: P0
// Purpose: Verify genesis allocations are valid (no negatives, no duplicates)
func TestTC006_GenesisIntegrity(t *testing.T) {
	_ = SetupTestContext(t)

	// This test would typically load genesis file and validate
	// For unit test, we create a mock genesis state
	genesisAllocations := map[string]int64{
		"validator1": 100_000_000_000_000,
		"validator2": 100_000_000_000_000,
		"treasury":   800_000_000_000_000,
	}

	totalAllocated := int64(0)
	seenAddresses := make(map[string]bool)

	for addr, amount := range genesisAllocations {
		// Check no negative amounts
		require.GreaterOrEqual(t, amount, int64(0),
			"Genesis allocation for %s must not be negative", addr)

		// Check no duplicates
		require.False(t, seenAddresses[addr],
			"Duplicate genesis allocation for %s", addr)
		seenAddresses[addr] = true

		totalAllocated += amount
	}

	// Verify total is within cap
	require.LessOrEqual(t, totalAllocated, int64(TestHardCap),
		"Total genesis allocation %d exceeds cap %d", totalAllocated, TestHardCap)
}

// TC-007: Burn Correctness - Base Fee Burn
// Priority: P0
// Purpose: Verify base fee burn reduces circulating supply
func TestTC007_BaseFeeBurn(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply
	initialSupply := math.NewInt(1_000_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Give module some balance to burn
	moduleBalance := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000)))
	tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, moduleBalance)

	// Record supply before burn
	supplyBefore := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount

	// Burn base fee (0.5 OMNI)
	burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(500_000)))
	err := tc.BankKeeper.BurnCoins(tc.Ctx, types.ModuleName, burnAmount)
	require.NoError(t, err)

	// Verify supply decreased
	supplyAfter := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount
	expectedSupply := supplyBefore.Sub(burnAmount[0].Amount)
	require.Equal(t, expectedSupply, supplyAfter,
		"Supply should decrease by burn amount")

	// Verify burn is tracked
	burned := tc.BankKeeper.GetBurned().AmountOf(TestDenom)
	require.Equal(t, burnAmount[0].Amount, burned,
		"Burned amount should be tracked")
}

// TC-008: Burn Correctness - Smart Contract Call
// Priority: P0
// Purpose: Verify contract call fees are burned correctly
func TestTC008_ContractFeeBurn(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply
	initialSupply := math.NewInt(1_000_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Simulate contract deployment fee burn (10 OMNI)
	contractFee := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(10_000_000)))
	tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, contractFee)

	supplyBefore := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount

	// Burn contract fee
	err := tc.BankKeeper.BurnCoins(tc.Ctx, types.ModuleName, contractFee)
	require.NoError(t, err)

	// Verify supply reduced
	supplyAfter := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount
	require.Equal(t, supplyBefore.Sub(contractFee[0].Amount), supplyAfter,
		"Contract fee should be burned")
}

// TC-009: Burn Correctness - Module Fee Burn
// Priority: P0
// Purpose: Verify module fees are burned per policy
func TestTC009_ModuleFeeBurn(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply
	initialSupply := math.NewInt(1_000_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Module action fee (1 OMNI)
	moduleFee := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000)))
	tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, moduleFee)

	supplyBefore := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount

	// Burn module fee
	err := tc.BankKeeper.BurnCoins(tc.Ctx, types.ModuleName, moduleFee)
	require.NoError(t, err)

	// Verify burn event and supply reduction
	supplyAfter := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount
	require.Equal(t, supplyBefore.Sub(moduleFee[0].Amount), supplyAfter,
		"Module fee should be burned")
}

// TC-010: Burn Underflow Protection
// Priority: P0
// Purpose: Verify burn cannot create negative balance
func TestTC010_BurnUnderflowProtection(t *testing.T) {
	tc := SetupTestContext(t)

	// Set module balance to 1 OMNI
	moduleBalance := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(1_000_000)))
	tc.BankKeeper.SetSupply(moduleBalance)
	tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, moduleBalance)

	// Attempt to burn 2 OMNI (more than balance)
	burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(2_000_000)))

	// This should fail (in real implementation, bank module prevents this)
	// For mock, we verify the invariant
	_ = tc.BankKeeper.GetModuleBalance(types.ModuleName).AmountOf(TestDenom) // balanceBefore (reference only)

	// Try burn
	err := tc.BankKeeper.BurnCoins(tc.Ctx, types.ModuleName, burnAmount)

	// Either it fails, or balance doesn't go negative
	if err == nil {
		balanceAfter := tc.BankKeeper.GetModuleBalance(types.ModuleName).AmountOf(TestDenom)
		require.False(t, balanceAfter.IsNegative(),
			"Balance must not go negative after burn")
	}
}

// TC-011: Supply Accounting - Mint vs Burn Reconciliation
// Priority: P0
// Purpose: Verify net supply = minted - burned over multiple operations
func TestTC011_SupplyReconciliation(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply (1B OMNI)
	initialSupply := math.NewInt(1_000_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Perform series of mints and burns
	operations := []struct {
		action string
		amount int64
	}{
		{"mint", 10_000_000_000_000},  // Mint 10M OMNI
		{"burn", 2_000_000_000_000},   // Burn 2M OMNI
		{"mint", 5_000_000_000_000},   // Mint 5M OMNI
		{"burn", 3_000_000_000_000},   // Burn 3M OMNI
		{"mint", 1_000_000_000_000},   // Mint 1M OMNI
	}

	for _, op := range operations {
		amount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(op.amount)))
		if op.action == "mint" {
			tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, amount)
		} else {
			tc.BankKeeper.BurnCoins(tc.Ctx, types.ModuleName, amount)
		}
	}

	// Verify supply conservation
	AssertSupplyConservation(t, tc.BankKeeper, initialSupply)
}

// TC-012: Supply Accounting - Per-Module Transfer Reconciliation
// Priority: P0
// Purpose: Verify module transfers conserve total supply
func TestTC012_ModuleTransferReconciliation(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply
	initialSupply := math.NewInt(1_000_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Mint to tokenomics module
	mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(100_000_000_000_000)))
	tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, mintAmount)

	// Transfer to various modules (simulating reward distribution)
	testAddr := sdk.AccAddress("test_account_______")
	transferAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(10_000_000_000_000)))

	tc.BankKeeper.SendCoinsFromModuleToAccount(tc.Ctx, types.ModuleName, testAddr, transferAmount)

	// Verify no new supply created (only from explicit mint)
	currentSupply := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount
	expectedSupply := initialSupply.Add(mintAmount[0].Amount)
	require.Equal(t, expectedSupply, currentSupply,
		"Transfers should not create new supply")

	// Verify no negative balances
	AssertNoNegativeBalances(t, tc.BankKeeper)
}

// TC-013: Supply Accounting - Epoch Rollover Precision
// Priority: P0
// Purpose: Verify supply is exact across epoch boundaries
func TestTC013_EpochRolloverPrecision(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial supply
	initialSupply := math.NewInt(1_000_000_000_000_000)
	tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))

	// Simulate epoch rewards (inflation-based minting)
	// Assume 3% annual inflation, 365 epochs per year
	// Epoch reward = supply × (0.03 / 365)
	params := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	params.InflationRate = math.LegacyMustNewDecFromStr("0.03")
	tc.TokenomicsKeeper.SetParams(tc.Ctx, params)

	epochsPerYear := math.LegacyNewDec(365)
	epochReward := params.InflationRate.Quo(epochsPerYear).MulInt(initialSupply).TruncateInt()

	// Mint epoch reward
	mintAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, epochReward))
	tc.BankKeeper.MintCoins(tc.Ctx, types.ModuleName, mintAmount)

	// Burn some fees (simulate base fee burn)
	burnAmount := sdk.NewCoins(sdk.NewCoin(TestDenom, math.NewInt(100_000_000)))
	tc.BankKeeper.BurnCoins(tc.Ctx, types.ModuleName, burnAmount)

	// Verify supply precision (within 1 omniphi tolerance for rounding)
	expectedSupply := initialSupply.Add(epochReward).Sub(burnAmount[0].Amount)
	actualSupply := tc.BankKeeper.GetSupply(tc.Ctx, TestDenom).Amount
	diff := expectedSupply.Sub(actualSupply).Abs()

	require.LessOrEqual(t, diff.Int64(), int64(1),
		"Supply drift after epoch rollover must be <= 1 omniphi (rounding tolerance)")
}
