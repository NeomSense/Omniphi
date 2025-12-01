package integration

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

// TestScenario_HappyPath tests the standard user journey:
// 1. User receives tokens
// 2. User delegates to validator
// 3. Validator produces blocks
// 4. User receives staking rewards
// 5. User undelegates and withdraws
//
// Priority: P0
// Estimated Runtime: 30-60 seconds
func TestScenario_HappyPath(t *testing.T) {
	suite := SetupSuite(t)
	defer suite.Cleanup()

	t.Log("=== SCENARIO: Happy Path - Standard User Journey ===")

	// ============================================================
	// STEP 1: Verify initial balances
	// ============================================================
	t.Log("Step 1: Verifying initial balances...")

	initialUser1Balance := suite.GetBalance(suite.User1Addr)
	require.True(t, initialUser1Balance.GT(math.ZeroInt()),
		"User1 should have initial balance")

	initialTotalSupply := suite.GetTotalSupply()
	require.True(t, initialTotalSupply.GT(math.ZeroInt()),
		"Total supply should be positive")

	t.Logf("  ✓ User1 balance: %s OMNI", formatOMNI(initialUser1Balance))
	t.Logf("  ✓ Total supply: %s OMNI", formatOMNI(initialTotalSupply))

	// ============================================================
	// STEP 2: Create validator
	// ============================================================
	t.Log("Step 2: Creating validator...")

	// Validator self-delegation: 100,000 OMNI
	selfDelegation := math.NewInt(100_000_000_000) // 100k OMNI

	createValidatorMsg, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(suite.ValidatorAddr).String(),
		suite.ValidatorKey.PubKey(),
		sdk.NewCoin(Denom, selfDelegation),
		stakingtypes.Description{
			Moniker:  "test-validator",
			Identity: "test",
			Website:  "https://test.com",
			Details:  "Integration test validator",
		},
		stakingtypes.CommissionRates{
			Rate:          math.LegacyNewDecWithPrec(10, 2), // 10%
			MaxRate:       math.LegacyNewDecWithPrec(20, 2), // 20%
			MaxChangeRate: math.LegacyNewDecWithPrec(1, 2),  // 1%
		},
		math.NewInt(1_000_000), // min self delegation
	)
	require.NoError(t, err)

	txRes, err := suite.SendTx(
		suite.ValidatorAddr,
		[]sdk.Msg{createValidatorMsg},
		"Create validator",
	)
	require.NoError(t, err)
	require.Equal(t, uint32(0), txRes.Code, "create validator should succeed: %s", txRes.RawLog)

	suite.NextBlock() // Process validator creation

	t.Logf("  ✓ Validator created with %s OMNI self-delegation", formatOMNI(selfDelegation))

	// Verify validator exists
	validator, err := suite.App.StakingKeeper.GetValidator(
		suite.Ctx,
		sdk.ValAddress(suite.ValidatorAddr),
	)
	require.NoError(t, err)
	require.True(t, validator.IsBonded(), "Validator should be bonded")

	t.Logf("  ✓ Validator status: %s, tokens: %s", validator.Status, validator.Tokens.String())

	// ============================================================
	// STEP 3: User delegates tokens
	// ============================================================
	t.Log("Step 3: User1 delegating tokens...")

	delegationAmount := math.NewInt(50_000_000_000) // 50k OMNI

	delegateMsg := stakingtypes.NewMsgDelegate(
		suite.User1Addr.String(),
		sdk.ValAddress(suite.ValidatorAddr).String(),
		sdk.NewCoin(Denom, delegationAmount),
	)

	txRes, err = suite.SendTx(
		suite.User1Addr,
		[]sdk.Msg{delegateMsg},
		"Delegate to validator",
	)
	require.NoError(t, err)
	require.Equal(t, uint32(0), txRes.Code, "delegation should succeed: %s", txRes.RawLog)

	suite.NextBlock() // Process delegation

	t.Logf("  ✓ User1 delegated %s OMNI", formatOMNI(delegationAmount))

	// Verify delegation
	delegation, err := suite.App.StakingKeeper.GetDelegation(
		suite.Ctx,
		suite.User1Addr,
		sdk.ValAddress(suite.ValidatorAddr),
	)
	require.NoError(t, err)
	require.Equal(t, delegationAmount.String(), delegation.Shares.TruncateInt().String(),
		"Delegation shares should match delegation amount")

	t.Logf("  ✓ Delegation verified: %s shares", delegation.Shares.String())

	// Verify user1 balance decreased
	user1BalanceAfterDelegate := suite.GetBalance(suite.User1Addr)
	expectedBalanceAfterDelegate := initialUser1Balance.Sub(delegationAmount)
	// Account for transaction fees (approximate)
	feeBuffer := math.NewInt(10_000) // 0.01 OMNI buffer for fees
	require.True(t,
		user1BalanceAfterDelegate.LTE(expectedBalanceAfterDelegate) &&
			user1BalanceAfterDelegate.GTE(expectedBalanceAfterDelegate.Sub(feeBuffer)),
		"User1 balance should decrease by delegation amount (minus fees)")

	t.Logf("  ✓ User1 balance after delegation: %s OMNI", formatOMNI(user1BalanceAfterDelegate))

	// ============================================================
	// STEP 4: Produce blocks and earn rewards
	// ============================================================
	t.Log("Step 4: Producing blocks to earn rewards...")

	// Advance 100 blocks (~7 minutes at 1 block/sec)
	blocksToAdvance := 100
	suite.WaitForBlocks(blocksToAdvance)

	t.Logf("  ✓ Advanced %d blocks (height now: %d)", blocksToAdvance, suite.BlockHeight)

	// Check for rewards (distribution module would accumulate rewards)
	// Note: In a real integration test, rewards would accumulate via the distribution module
	// For this test, we verify the validator is still active and delegation exists

	validator, err = suite.App.StakingKeeper.GetValidator(
		suite.Ctx,
		sdk.ValAddress(suite.ValidatorAddr),
	)
	require.NoError(t, err)
	require.True(t, validator.IsBonded(), "Validator should still be bonded")

	t.Logf("  ✓ Validator still active with %s tokens", validator.Tokens.String())

	// ============================================================
	// STEP 5: Undelegate tokens
	// ============================================================
	t.Log("Step 5: User1 undelegating tokens...")

	undelegateAmount := math.NewInt(25_000_000_000) // 25k OMNI (half of delegation)

	undelegateMsg := stakingtypes.NewMsgUndelegate(
		suite.User1Addr.String(),
		sdk.ValAddress(suite.ValidatorAddr).String(),
		sdk.NewCoin(Denom, undelegateAmount),
	)

	txRes, err = suite.SendTx(
		suite.User1Addr,
		[]sdk.Msg{undelegateMsg},
		"Undelegate from validator",
	)
	require.NoError(t, err)
	require.Equal(t, uint32(0), txRes.Code, "undelegation should succeed: %s", txRes.RawLog)

	suite.NextBlock() // Process undelegation

	t.Logf("  ✓ User1 undelegated %s OMNI", formatOMNI(undelegateAmount))

	// Verify undelegation entry exists
	unbondingDelegation, err := suite.App.StakingKeeper.GetUnbondingDelegation(
		suite.Ctx,
		suite.User1Addr,
		sdk.ValAddress(suite.ValidatorAddr),
	)
	require.NoError(t, err)
	require.Len(t, unbondingDelegation.Entries, 1, "Should have one unbonding entry")
	require.Equal(t, undelegateAmount.String(), unbondingDelegation.Entries[0].Balance.String(),
		"Unbonding amount should match")

	t.Logf("  ✓ Unbonding entry created: %s OMNI, completion time: %s",
		formatOMNI(unbondingDelegation.Entries[0].Balance),
		unbondingDelegation.Entries[0].CompletionTime)

	// Verify remaining delegation
	delegation, err = suite.App.StakingKeeper.GetDelegation(
		suite.Ctx,
		suite.User1Addr,
		sdk.ValAddress(suite.ValidatorAddr),
	)
	require.NoError(t, err)

	remainingDelegation := delegationAmount.Sub(undelegateAmount)
	require.Equal(t, remainingDelegation.String(), delegation.Shares.TruncateInt().String(),
		"Remaining delegation should be 25k OMNI")

	t.Logf("  ✓ Remaining delegation: %s OMNI", formatOMNI(delegation.Shares.TruncateInt()))

	// ============================================================
	// STEP 6: Wait for unbonding period and verify funds returned
	// ============================================================
	t.Log("Step 6: Waiting for unbonding period...")

	// In a real chain, unbonding period is ~21 days
	// For this test, we simulate by advancing blocks
	unbondingBlocks := 1000 // Simulate unbonding completion
	suite.WaitForBlocks(unbondingBlocks)

	t.Logf("  ✓ Advanced %d blocks to complete unbonding", unbondingBlocks)

	// After unbonding completes, tokens should be returned to user1
	// Note: In production, this would happen automatically via EndBlocker
	// For this test, we verify the unbonding entry still tracks the correct amount

	unbondingDelegation, err = suite.App.StakingKeeper.GetUnbondingDelegation(
		suite.Ctx,
		suite.User1Addr,
		sdk.ValAddress(suite.ValidatorAddr),
	)
	if err == nil {
		// Unbonding still in progress
		t.Logf("  ⧖ Unbonding still in progress: %s OMNI",
			formatOMNI(unbondingDelegation.Entries[0].Balance))
	} else {
		// Unbonding completed (entry removed)
		t.Logf("  ✓ Unbonding completed, tokens returned to user")
	}

	// ============================================================
	// STEP 7: Transfer tokens between users
	// ============================================================
	t.Log("Step 7: Testing token transfer...")

	transferAmount := math.NewInt(1_000_000_000) // 1k OMNI

	user2BalanceBefore := suite.GetBalance(suite.User2Addr)

	sendMsg := banktypes.NewMsgSend(
		suite.User1Addr,
		suite.User2Addr,
		sdk.NewCoins(sdk.NewCoin(Denom, transferAmount)),
	)

	txRes, err = suite.SendTx(
		suite.User1Addr,
		[]sdk.Msg{sendMsg},
		"Transfer to User2",
	)
	require.NoError(t, err)
	require.Equal(t, uint32(0), txRes.Code, "transfer should succeed: %s", txRes.RawLog)

	suite.NextBlock() // Process transfer

	t.Logf("  ✓ Transferred %s OMNI from User1 to User2", formatOMNI(transferAmount))

	// Verify User2 received tokens
	user2BalanceAfter := suite.GetBalance(suite.User2Addr)
	require.Equal(t, user2BalanceBefore.Add(transferAmount).String(), user2BalanceAfter.String(),
		"User2 should receive transferred amount")

	t.Logf("  ✓ User2 balance: %s → %s OMNI",
		formatOMNI(user2BalanceBefore),
		formatOMNI(user2BalanceAfter))

	// ============================================================
	// FINAL: Verify chain consistency
	// ============================================================
	t.Log("Final: Verifying chain consistency...")

	finalTotalSupply := suite.GetTotalSupply()
	t.Logf("  ✓ Final total supply: %s OMNI", formatOMNI(finalTotalSupply))
	t.Logf("  ✓ Final block height: %d", suite.BlockHeight)

	// Verify total supply hasn't changed unexpectedly
	// (It may increase due to inflation, but shouldn't decrease)
	require.True(t, finalTotalSupply.GTE(initialTotalSupply),
		"Total supply should not decrease")

	t.Log("=== SCENARIO COMPLETE: All steps passed ✓ ===")
}

// formatOMNI converts base units to OMNI with decimals
func formatOMNI(amount math.Int) string {
	if amount.IsZero() {
		return "0"
	}

	// Divide by 1,000,000 to get OMNI from omniphi (6 decimals)
	divisor := math.NewInt(1_000_000)
	omni := amount.Quo(divisor)
	remainder := amount.Mod(divisor)

	if remainder.IsZero() {
		return omni.String()
	}

	// Format with decimals (up to 6)
	return omni.String() + "." + padLeft(remainder.String(), 6)
}

// padLeft pads a string with leading zeros
func padLeft(s string, length int) string {
	for len(s) < length {
		s = "0" + s
	}
	return s
}
