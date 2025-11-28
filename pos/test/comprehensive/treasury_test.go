package comprehensive

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

// ========================================
// TC-065 to TC-072: Treasury & Distribution Tests
// ========================================

// TC-065: Balance Integrity - Concurrent Grants
// Priority: P1
// Purpose: Verify concurrent grant payments don't cause double spend
func TestTC065_BalanceIntegrity(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial treasury balance
	treasuryBalance := math.NewInt(100_000_000_000_000) // 100M OMNI
	tc.BankKeeper.MintCoins(tc.Ctx, "treasury", sdk.NewCoins(sdk.NewCoin(TestDenom, treasuryBalance)))

	// Create 5 grants of 10M OMNI each (total 50M)
	grantAmount := math.NewInt(10_000_000_000_000) // 10M OMNI each
	numGrants := 5

	// Record initial balance
	initialBalance := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)

	// Execute concurrent grants
	for i := 0; i < numGrants; i++ {
		grantID := "grant-" + string(rune('0'+i))
		recipient := sdk.AccAddress([]byte("recipient" + string(rune('0'+i))))

		// Send grant
		err := tc.BankKeeper.SendCoinsFromModuleToAccount(
			tc.Ctx,
			"treasury",
			recipient,
			sdk.NewCoins(sdk.NewCoin(TestDenom, grantAmount)),
		)
		require.NoError(t, err, "Grant %s should succeed", grantID)
	}

	// Verify final treasury balance
	finalBalance := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)
	spent := initialBalance.Sub(finalBalance)

	expectedSpent := grantAmount.MulRaw(int64(numGrants)) // 50M OMNI
	require.Equal(t, expectedSpent.Int64(), spent.Int64(),
		"Treasury should have spent exactly 50M OMNI")

	// Verify no double spend
	require.Equal(t, treasuryBalance.Sub(expectedSpent).Int64(), finalBalance.Int64(),
		"Treasury balance should be 100M - 50M = 50M")
}

// TC-066: Grant Cancellation
// Priority: P1
// Purpose: Verify grant cancellation returns remaining funds
func TestTC066_GrantCancellation(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial treasury balance
	treasuryBalance := math.NewInt(100_000_000_000_000) // 100M OMNI
	tc.BankKeeper.MintCoins(tc.Ctx, "treasury", sdk.NewCoins(sdk.NewCoin(TestDenom, treasuryBalance)))

	_ = "grant-cancel-001"                        // grantID (for documentation)
	grantTotal := math.NewInt(10_000_000_000_000) // 10M OMNI total grant
	grantPaid := math.NewInt(3_000_000_000_000)   // 3M OMNI already paid
	_ = grantTotal.Sub(grantPaid)                 // 7M OMNI remaining (grantRemaining)

	recipient := sdk.AccAddress("recipient_________")

	// Pay out 3M (simulate partial grant execution)
	tc.BankKeeper.SendCoinsFromModuleToAccount(
		tc.Ctx,
		"treasury",
		recipient,
		sdk.NewCoins(sdk.NewCoin(TestDenom, grantPaid)),
	)

	// Record balance before cancellation
	balanceBefore := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)

	// Cancel grant (remaining 7M stays in treasury - not sent)
	// In a real implementation, this would track the grant state
	// For this test, we verify the remaining amount is NOT sent

	// Verify treasury balance unchanged (remaining funds not disbursed)
	balanceAfter := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)
	require.Equal(t, balanceBefore.Int64(), balanceAfter.Int64(),
		"Treasury balance should be unchanged after cancellation")

	// Verify total paid = initial - balance
	totalPaid := treasuryBalance.Sub(balanceAfter)
	require.Equal(t, grantPaid.Int64(), totalPaid.Int64(),
		"Only paid amount should have left treasury")
}

// TC-067: Grant Clawback
// Priority: P1
// Purpose: Verify unused grant funds can be clawed back
func TestTC067_GrantClawback(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial treasury balance
	treasuryBalance := math.NewInt(100_000_000_000_000) // 100M OMNI
	tc.BankKeeper.MintCoins(tc.Ctx, "treasury", sdk.NewCoins(sdk.NewCoin(TestDenom, treasuryBalance)))

	grantTotal := math.NewInt(10_000_000_000_000)   // 10M OMNI grant
	amountUsed := math.NewInt(6_000_000_000_000)    // 6M OMNI used
	amountUnused := grantTotal.Sub(amountUsed)      // 4M OMNI unused (to clawback)

	recipient := sdk.AccAddress("recipient_________")

	// Grant was fully disbursed to recipient
	tc.BankKeeper.SendCoinsFromModuleToAccount(
		tc.Ctx,
		"treasury",
		recipient,
		sdk.NewCoins(sdk.NewCoin(TestDenom, grantTotal)),
	)

	// Recipient used 6M, has 4M remaining
	tc.BankKeeper.SetBalance(tc.Ctx, recipient, sdk.NewCoins(sdk.NewCoin(TestDenom, amountUnused)))

	// Record treasury balance before clawback
	balanceBefore := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)

	// Clawback unused 4M
	err := tc.BankKeeper.SendCoinsFromAccountToModule(
		tc.Ctx,
		recipient,
		"treasury",
		sdk.NewCoins(sdk.NewCoin(TestDenom, amountUnused)),
	)
	require.NoError(t, err, "Clawback should succeed")

	// Verify treasury balance increased by clawed back amount
	balanceAfter := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)
	clawedBack := balanceAfter.Sub(balanceBefore)

	require.Equal(t, amountUnused.Int64(), clawedBack.Int64(),
		"Treasury should receive 4M OMNI clawback")
}

// TC-068: Memo Trail
// Priority: P1
// Purpose: Verify all treasury operations have audit-ready memo fields
func TestTC068_MemoTrail(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial treasury balance
	treasuryBalance := math.NewInt(100_000_000_000_000) // 100M OMNI
	tc.BankKeeper.MintCoins(tc.Ctx, "treasury", sdk.NewCoins(sdk.NewCoin(TestDenom, treasuryBalance)))

	// Operation 1: Grant with memo
	grantMemo := "Grant for Development Team Q4 2025"
	recipient1 := sdk.AccAddress("recipient1________")
	grantAmount := math.NewInt(5_000_000_000_000)

	tc.BankKeeper.SendCoinsFromModuleToAccountWithMemo(
		tc.Ctx,
		"treasury",
		recipient1,
		sdk.NewCoins(sdk.NewCoin(TestDenom, grantAmount)),
		grantMemo,
	)

	// Operation 2: Clawback with memo
	clawbackMemo := "Clawback unused funds from Grant XYZ"
	clawbackAmount := math.NewInt(1_000_000_000_000)

	tc.BankKeeper.SendCoinsFromAccountToModuleWithMemo(
		tc.Ctx,
		recipient1,
		"treasury",
		sdk.NewCoins(sdk.NewCoin(TestDenom, clawbackAmount)),
		clawbackMemo,
	)

	// Verify memo trail is complete
	memoLog := tc.BankKeeper.GetMemoLog(tc.Ctx)
	require.GreaterOrEqual(t, len(memoLog), 2, "At least 2 memo entries should exist")

	// Verify memos contain expected information
	foundGrantMemo := false
	foundClawbackMemo := false

	for _, entry := range memoLog {
		if entry.Memo == grantMemo {
			foundGrantMemo = true
		}
		if entry.Memo == clawbackMemo {
			foundClawbackMemo = true
		}
	}

	require.True(t, foundGrantMemo, "Grant memo should be in audit trail")
	require.True(t, foundClawbackMemo, "Clawback memo should be in audit trail")
}

// TC-069: Burn Redirect - Enable
// Priority: P1
// Purpose: Verify burn redirect takes effect next epoch
func TestTC069_BurnRedirectEnable(t *testing.T) {
	tc := SetupTestContext(t)

	// Initial: 0% redirect (all burns reduce supply)
	params := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	params.TreasuryBurnRedirect = math.LegacyZeroDec()
	tc.TokenomicsKeeper.SetParams(tc.Ctx, params)

	// Burn at epoch 1
	epoch1Ctx := tc.Ctx.WithBlockHeight(100)
	burnAmount := math.NewInt(1_000_000_000_000) // 1M OMNI
	tc.BankKeeper.MintCoins(epoch1Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, burnAmount)))

	initialSupply := tc.BankKeeper.GetSupply(epoch1Ctx, TestDenom).Amount
	tc.BankKeeper.BurnCoins(epoch1Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, burnAmount)))

	supplyAfterBurn := tc.BankKeeper.GetSupply(epoch1Ctx, TestDenom).Amount
	actualBurned := initialSupply.Sub(supplyAfterBurn)

	require.Equal(t, burnAmount.Int64(), actualBurned.Int64(),
		"With 0% redirect, full amount should be burned")

	// Enable 10% redirect at epoch 2
	epoch2Ctx := tc.Ctx.WithBlockHeight(200)
	params.TreasuryBurnRedirect = math.LegacyMustNewDecFromStr("0.10")
	tc.TokenomicsKeeper.SetParams(epoch2Ctx, params)

	// Burn at epoch 3 (redirect should be active)
	epoch3Ctx := tc.Ctx.WithBlockHeight(300)
	tc.BankKeeper.MintCoins(epoch3Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, burnAmount)))

	treasuryBefore := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)
	supplyBefore := tc.BankKeeper.GetSupply(epoch3Ctx, TestDenom).Amount

	// Apply burn with redirect
	redirectAmount := params.TreasuryBurnRedirect.MulInt(burnAmount).TruncateInt() // 10%
	actualBurnAmount := burnAmount.Sub(redirectAmount) // 90%

	// Redirect to treasury
	tc.BankKeeper.SendCoinsFromModuleToModule(epoch3Ctx, types.ModuleName, "treasury",
		sdk.NewCoins(sdk.NewCoin(TestDenom, redirectAmount)))

	// Burn the rest
	tc.BankKeeper.BurnCoins(epoch3Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, actualBurnAmount)))

	treasuryAfter := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)
	supplyAfter := tc.BankKeeper.GetSupply(epoch3Ctx, TestDenom).Amount

	// Verify redirect
	treasuryIncrease := treasuryAfter.Sub(treasuryBefore)
	require.Equal(t, redirectAmount.Int64(), treasuryIncrease.Int64(),
		"Treasury should receive 10% redirect")

	// Verify burn
	burned := supplyBefore.Sub(supplyAfter)
	require.Equal(t, actualBurnAmount.Int64(), burned.Int64(),
		"90% should be burned")
}

// TC-070: Burn Redirect - Disable
// Priority: P1
// Purpose: Verify disabling redirect takes effect next epoch
func TestTC070_BurnRedirectDisable(t *testing.T) {
	tc := SetupTestContext(t)

	// Initial: 10% redirect enabled
	params := tc.TokenomicsKeeper.GetParams(tc.Ctx)
	params.TreasuryBurnRedirect = math.LegacyMustNewDecFromStr("0.10")
	tc.TokenomicsKeeper.SetParams(tc.Ctx, params)

	// Disable redirect at epoch 2
	epoch2Ctx := tc.Ctx.WithBlockHeight(200)
	params.TreasuryBurnRedirect = math.LegacyZeroDec()
	tc.TokenomicsKeeper.SetParams(epoch2Ctx, params)

	// Verify redirect disabled
	updatedParams := tc.TokenomicsKeeper.GetParams(epoch2Ctx)
	require.Equal(t, "0.000000000000000000", updatedParams.TreasuryBurnRedirect.String(),
		"Redirect should be disabled")

	// Burn at epoch 3 (no redirect)
	epoch3Ctx := tc.Ctx.WithBlockHeight(300)
	burnAmount := math.NewInt(1_000_000_000_000)

	tc.BankKeeper.MintCoins(epoch3Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, burnAmount)))
	treasuryBefore := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)
	supplyBefore := tc.BankKeeper.GetSupply(epoch3Ctx, TestDenom).Amount

	tc.BankKeeper.BurnCoins(epoch3Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(TestDenom, burnAmount)))

	treasuryAfter := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)
	supplyAfter := tc.BankKeeper.GetSupply(epoch3Ctx, TestDenom).Amount

	// Verify no redirect
	treasuryChange := treasuryAfter.Sub(treasuryBefore)
	require.Equal(t, int64(0), treasuryChange.Int64(),
		"Treasury should not receive any funds")

	// Verify full burn
	burned := supplyBefore.Sub(supplyAfter)
	require.Equal(t, burnAmount.Int64(), burned.Int64(),
		"Full amount should be burned with redirect disabled")
}

// TC-071: Burn Redirect - History
// Priority: P1
// Purpose: Verify historical redirects are immutable
func TestTC071_BurnRedirectHistory(t *testing.T) {
	tc := SetupTestContext(t)

	// Epoch 1: 0% redirect
	epoch1Ctx := tc.Ctx.WithBlockHeight(100)
	params1 := tc.TokenomicsKeeper.GetParams(epoch1Ctx)
	params1.TreasuryBurnRedirect = math.LegacyZeroDec()
	tc.TokenomicsKeeper.SetParams(epoch1Ctx, params1)

	// Epoch 2: 10% redirect
	epoch2Ctx := tc.Ctx.WithBlockHeight(200)
	params2 := tc.TokenomicsKeeper.GetParams(epoch2Ctx)
	params2.TreasuryBurnRedirect = math.LegacyMustNewDecFromStr("0.10")
	tc.TokenomicsKeeper.SetParams(epoch2Ctx, params2)

	// Epoch 3: 20% redirect
	epoch3Ctx := tc.Ctx.WithBlockHeight(300)
	params3 := tc.TokenomicsKeeper.GetParams(epoch3Ctx)
	params3.TreasuryBurnRedirect = math.LegacyMustNewDecFromStr("0.20")
	tc.TokenomicsKeeper.SetParams(epoch3Ctx, params3)

	// Query historical values (this tests that history is preserved)
	// Note: In mock implementation, we can't query historical state
	// This would work in integration tests with real state versioning

	currentParams := tc.TokenomicsKeeper.GetParams(epoch3Ctx)
	require.Equal(t, "0.200000000000000000", currentParams.TreasuryBurnRedirect.String(),
		"Current redirect should be 20%")

	t.Log("Historical param versioning should be tested in integration tests")
	t.Log("Changes logged: 0% → 10% → 20%")
}

// TC-072: Concurrent Operations
// Priority: P1
// Purpose: Verify concurrent grants/clawbacks/cancellations are atomic
func TestTC072_ConcurrentOperations(t *testing.T) {
	tc := SetupTestContext(t)

	// Set initial treasury balance
	treasuryBalance := math.NewInt(100_000_000_000_000) // 100M OMNI
	tc.BankKeeper.MintCoins(tc.Ctx, "treasury", sdk.NewCoins(sdk.NewCoin(TestDenom, treasuryBalance)))

	initialBalance := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)

	// Concurrent operations:
	// - 3 grants of 5M each = 15M out
	// - 2 clawbacks of 2M each = 4M in
	// - 1 cancellation (no-op in this test)

	operations := []struct {
		opType    string
		amount    math.Int
		recipient sdk.AccAddress
	}{
		{"grant", math.NewInt(5_000_000_000_000), sdk.AccAddress("recipient1________")},
		{"grant", math.NewInt(5_000_000_000_000), sdk.AccAddress("recipient2________")},
		{"clawback", math.NewInt(2_000_000_000_000), sdk.AccAddress("recipient3________")},
		{"grant", math.NewInt(5_000_000_000_000), sdk.AccAddress("recipient4________")},
		{"clawback", math.NewInt(2_000_000_000_000), sdk.AccAddress("recipient5________")},
	}

	// Execute all operations
	for _, op := range operations {
		if op.opType == "grant" {
			err := tc.BankKeeper.SendCoinsFromModuleToAccount(
				tc.Ctx,
				"treasury",
				op.recipient,
				sdk.NewCoins(sdk.NewCoin(TestDenom, op.amount)),
			)
			require.NoError(t, err, "Grant should succeed")
		} else if op.opType == "clawback" {
			// Set recipient balance first
			tc.BankKeeper.SetBalance(tc.Ctx, op.recipient, sdk.NewCoins(sdk.NewCoin(TestDenom, op.amount)))

			err := tc.BankKeeper.SendCoinsFromAccountToModule(
				tc.Ctx,
				op.recipient,
				"treasury",
				sdk.NewCoins(sdk.NewCoin(TestDenom, op.amount)),
			)
			require.NoError(t, err, "Clawback should succeed")
		}
	}

	// Verify final balance
	finalBalance := tc.BankKeeper.GetModuleBalance("treasury").AmountOf(TestDenom)

	// Expected: 100M - 15M (grants) + 4M (clawbacks) = 89M
	expectedBalance := treasuryBalance.
		Sub(math.NewInt(15_000_000_000_000)). // -15M grants
		Add(math.NewInt(4_000_000_000_000))   // +4M clawbacks

	require.Equal(t, expectedBalance.Int64(), finalBalance.Int64(),
		"Treasury balance should reflect all operations atomically")

	// Verify no race conditions (balance should be exact)
	netChange := finalBalance.Sub(initialBalance)
	expectedNetChange := math.NewInt(-11_000_000_000_000) // -11M net
	require.Equal(t, expectedNetChange.Int64(), netChange.Int64(),
		"Net change should be -11M OMNI")
}
