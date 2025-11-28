# PoC Submission Fee Burn Implementation Guide

**Status**: DESIGN COMPLETE - Ready for Implementation
**Author**: Senior Blockchain Developer
**Date**: 2025-10-27
**Module**: x/poc (Proof of Contribution)

---

## 🎯 Executive Summary

This guide provides a complete implementation of the PoC Submission Fee Burn mechanism - a sophisticated tokenomics enhancement that:

- **Collects fees** on every contribution submission (default: 1000uomni = 0.001 OMNI)
- **Burns 75%** of fees (deflationary mechanism)
- **Redirects 25%** to PoC reward pool (incentive mechanism)
- **DAO-governed** with min/max bounds (50%-90% burn ratio)
- **Full transparency** with cumulative metrics and per-contributor analytics

---

## 📐 Architecture Overview

### **Fee Flow Diagram**

```
Contributor submits contribution
         ↓
  [Collect 1000uomni fee]
         ↓
     [Split Fee]
    /           \
   ↓             ↓
[Burn 750]   [Reward 250]
   ↓             ↓
[Supply↓]   [PoC Rewards Pool]
```

### **State Management**

```
Global Metrics:
- total_fees_collected: Cumulative fees
- total_burned: Cumulative burned
- total_reward_redirect: Cumulative to rewards
- last_updated_height: Audit trail

Per-Contributor Stats:
- total_fees_paid
- total_burned
- submission_count
- first/last_submission_height
```

---

## 🔧 Implementation Checklist

### Phase 1: Proto Definitions ✅

**Files Modified:**
- ✅ `proto/pos/poc/v1/params.proto` - Added 6 new fee parameters
- ✅ `proto/pos/poc/v1/fee_metrics.proto` - New file for metrics tracking
- ✅ `proto/pos/poc/v1/query.proto` - Added fee metrics queries
- ✅ `proto/pos/poc/v1/genesis.proto` - Added fee metrics to genesis

**New Parameters:**
```protobuf
message Params {
  // Existing params...

  // NEW: Fee burn parameters
  string submission_fee = 8;            // Default: 1000uomni
  string submission_burn_ratio = 9;     // Default: 0.75 (75%)
  string min_submission_fee = 10;       // Bound: 100uomni
  string max_submission_fee = 11;       // Bound: 100000uomni
  string min_burn_ratio = 12;           // Bound: 0.50 (50%)
  string max_burn_ratio = 13;           // Bound: 0.90 (90%)
}
```

### Phase 2: Generate Protos ⚠️

**Status**: Proto generation has configuration issues with buf
**Workaround**: Manual proto code generation completed for fee_metrics.pb.go

**Action Required:**
```bash
# Fix buf configuration first, then:
cd /path/to/pos
buf generate

# Verify generated files:
ls x/poc/types/*fee*.pb.go
# Expected: fee_metrics.pb.go
```

**Files that need regeneration:**
- `x/poc/types/params.pb.go` - Contains new fee params
- `x/poc/types/genesis.pb.go` - Contains fee metrics
- `x/poc/types/query.pb.go` - Contains fee query RPCs
- `x/poc/types/fee_metrics.pb.go` - New (manually created stub exists)

### Phase 3: Update types/params.go ⏳

**File**: `x/poc/types/params.go`

**Required Changes:**

1. **Add DefaultParams for fee parameters:**

```go
func DefaultParams() Params {
	return Params{
		// Existing defaults...
		QuorumPct:         math.LegacyNewDecWithPrec(67, 2), // 67%
		BaseRewardUnit:    math.NewInt(1000),
		InflationShare:    math.LegacyZeroDec(),
		MaxPerBlock:       10,
		Tiers:             DefaultTiers(),
		RewardDenom:       "omniphi",
		MaxContributionsToKeep: 100000,

		// NEW: Fee burn defaults
		SubmissionFee:        types.NewCoin("uomni", math.NewInt(1000)),  // 0.001 OMNI
		SubmissionBurnRatio:  math.LegacyNewDecWithPrec(75, 2),           // 75%
		MinSubmissionFee:     types.NewCoin("uomni", math.NewInt(100)),   // 0.0001 OMNI
		MaxSubmissionFee:     types.NewCoin("uomni", math.NewInt(100000)), // 0.1 OMNI
		MinBurnRatio:         math.LegacyNewDecWithPrec(50, 2),           // 50%
		MaxBurnRatio:         math.LegacyNewDecWithPrec(90, 2),           // 90%
	}
}
```

2. **Enhance Validate() method:**

```go
func (p Params) Validate() error {
	// Existing validations...

	// NEW: Validate fee parameters
	if err := validateSubmissionFee(p.SubmissionFee); err != nil {
		return err
	}
	if err := validateBurnRatio(p.SubmissionBurnRatio); err != nil {
		return err
	}
	if err := validateFeeWithinBounds(p.SubmissionFee, p.MinSubmissionFee, p.MaxSubmissionFee); err != nil {
		return err
	}
	if err := validateRatioWithinBounds(p.SubmissionBurnRatio, p.MinBurnRatio, p.MaxBurnRatio); err != nil {
		return err
	}

	return nil
}

func validateSubmissionFee(fee types.Coin) error {
	if !fee.IsValid() {
		return fmt.Errorf("invalid submission fee: %s", fee)
	}
	if fee.IsNegative() {
		return fmt.Errorf("submission fee cannot be negative: %s", fee)
	}
	if fee.Denom == "" {
		return fmt.Errorf("submission fee denom cannot be empty")
	}
	return nil
}

func validateBurnRatio(ratio math.LegacyDec) error {
	if ratio.IsNegative() {
		return fmt.Errorf("burn ratio cannot be negative: %s", ratio)
	}
	if ratio.GT(math.LegacyOneDec()) {
		return fmt.Errorf("burn ratio cannot exceed 1.0: %s", ratio)
	}
	return nil
}

func validateFeeWithinBounds(fee, min, max types.Coin) error {
	if fee.Denom != min.Denom || fee.Denom != max.Denom {
		return fmt.Errorf("fee denom mismatch: fee=%s, min=%s, max=%s", fee.Denom, min.Denom, max.Denom)
	}
	if fee.Amount.LT(min.Amount) {
		return fmt.Errorf("submission fee %s below minimum %s", fee, min)
	}
	if fee.Amount.GT(max.Amount) {
		return fmt.Errorf("submission fee %s above maximum %s", fee, max)
	}
	return nil
}

func validateRatioWithinBounds(ratio, min, max math.LegacyDec) error {
	if ratio.LT(min) {
		return fmt.Errorf("burn ratio %s below minimum %s", ratio, min)
	}
	if ratio.GT(max) {
		return fmt.Errorf("burn ratio %s above maximum %s", ratio, max)
	}
	return nil
}
```

### Phase 4: Implement Keeper Fee Logic ⏳

**File**: `x/poc/keeper/fee_burn.go` (NEW FILE)

```go
package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"pos/x/poc/types"
)

// Storage keys for fee metrics
var (
	FeeMetricsKey           = collections.NewPrefix([]byte("fee_metrics"))
	ContributorFeeStatsKey  = collections.NewPrefix([]byte("contributor_fee_stats"))
)

// CollectAndBurnSubmissionFee handles the submission fee collection and burn/reward split
// This is the core function called during contribution submission
//
// SECURITY: This function is atomic - if any step fails, entire operation reverts
// AUDIT TRAIL: Emits detailed events for transparency
func (k Keeper) CollectAndBurnSubmissionFee(
	ctx context.Context,
	contributor sdk.AccAddress,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Step 1: Collect fee from contributor
	feeCoins := sdk.NewCoins(params.SubmissionFee)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		contributor,
		types.ModuleName,
		feeCoins,
	); err != nil {
		return fmt.Errorf("failed to collect submission fee from %s: %w", contributor, err)
	}

	// Step 2: Calculate burn and reward amounts
	feeAmountDec := math.LegacyNewDecFromInt(params.SubmissionFee.Amount)
	burnAmountDec := feeAmountDec.Mul(params.SubmissionBurnRatio)
	burnAmount := burnAmountDec.TruncateInt()
	rewardAmount := params.SubmissionFee.Amount.Sub(burnAmount)

	burnCoins := sdk.NewCoins(sdk.NewCoin(params.SubmissionFee.Denom, burnAmount))
	rewardCoins := sdk.NewCoins(sdk.NewCoin(params.SubmissionFee.Denom, rewardAmount))

	// Step 3: Burn coins (reduces total supply)
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnCoins); err != nil {
		return fmt.Errorf("failed to burn submission fee: %w", err)
	}

	// Step 4: Transfer reward portion to PoC reward pool
	// NOTE: This keeps coins in the module account for later distribution
	// The module account acts as the reward pool
	// Alternatively, could transfer to a separate "poc_rewards" module account

	// Step 5: Update global fee metrics (atomic)
	if err := k.UpdateFeeMetrics(ctx, feeCoins, burnCoins, rewardCoins); err != nil {
		return fmt.Errorf("failed to update fee metrics: %w", err)
	}

	// Step 6: Update contributor-specific stats (atomic)
	if err := k.UpdateContributorFeeStats(ctx, contributor, feeCoins, burnCoins); err != nil {
		return fmt.Errorf("failed to update contributor fee stats: %w", err)
	}

	// Step 7: Emit detailed events for transparency
	sdkCtx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_submission_fee_collected",
			sdk.NewAttribute("contributor", contributor.String()),
			sdk.NewAttribute("fee_paid", feeCoins.String()),
			sdk.NewAttribute("burned", burnCoins.String()),
			sdk.NewAttribute("to_rewards", rewardCoins.String()),
			sdk.NewAttribute("burn_ratio", params.SubmissionBurnRatio.String()),
		),
	})

	return nil
}

// UpdateFeeMetrics updates the global cumulative fee metrics
func (k Keeper) UpdateFeeMetrics(
	ctx context.Context,
	feesCollected, burned, rewardRedirect sdk.Coins,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	metrics := k.GetFeeMetrics(ctx)

	// Add to cumulative totals
	metrics.TotalFeesCollected = metrics.TotalFeesCollected.Add(feesCollected...)
	metrics.TotalBurned = metrics.TotalBurned.Add(burned...)
	metrics.TotalRewardRedirect = metrics.TotalRewardRedirect.Add(rewardRedirect...)
	metrics.LastUpdatedHeight = sdkCtx.BlockHeight()

	k.SetFeeMetrics(ctx, metrics)
	return nil
}

// UpdateContributorFeeStats updates per-contributor fee statistics
func (k Keeper) UpdateContributorFeeStats(
	ctx context.Context,
	contributor sdk.AccAddress,
	feesPaid, burned sdk.Coins,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	stats := k.GetContributorFeeStats(ctx, contributor)

	// Initialize if first submission
	if stats.SubmissionCount == 0 {
		stats.Address = contributor.String()
		stats.FirstSubmissionHeight = sdkCtx.BlockHeight()
	}

	// Update cumulative stats
	stats.TotalFeesPaid = stats.TotalFeesPaid.Add(feesPaid...)
	stats.TotalBurned = stats.TotalBurned.Add(burned...)
	stats.SubmissionCount++
	stats.LastSubmissionHeight = sdkCtx.BlockHeight()

	k.SetContributorFeeStats(ctx, stats)
	return nil
}

// GetFeeMetrics retrieves global fee metrics
func (k Keeper) GetFeeMetrics(ctx context.Context) types.FeeMetrics {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(FeeMetricsKey)
	if err != nil || bz == nil {
		// Return empty metrics if not found
		return types.FeeMetrics{
			TotalFeesCollected:  sdk.NewCoins(),
			TotalBurned:         sdk.NewCoins(),
			TotalRewardRedirect: sdk.NewCoins(),
			LastUpdatedHeight:   0,
		}
	}

	var metrics types.FeeMetrics
	k.cdc.MustUnmarshal(bz, &metrics)
	return metrics
}

// SetFeeMetrics stores global fee metrics
func (k Keeper) SetFeeMetrics(ctx context.Context, metrics types.FeeMetrics) {
	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&metrics)
	if err := store.Set(FeeMetricsKey, bz); err != nil {
		panic(err)
	}
}

// GetContributorFeeStats retrieves contributor-specific fee stats
func (k Keeper) GetContributorFeeStats(ctx context.Context, addr sdk.AccAddress) types.ContributorFeeStats {
	store := k.storeService.OpenKVStore(ctx)
	key := append(ContributorFeeStatsKey, addr.Bytes()...)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		// Return empty stats if not found
		return types.ContributorFeeStats{
			Address:               addr.String(),
			TotalFeesPaid:         sdk.NewCoins(),
			TotalBurned:           sdk.NewCoins(),
			SubmissionCount:       0,
			FirstSubmissionHeight: 0,
			LastSubmissionHeight:  0,
		}
	}

	var stats types.ContributorFeeStats
	k.cdc.MustUnmarshal(bz, &stats)
	return stats
}

// SetContributorFeeStats stores contributor-specific fee stats
func (k Keeper) SetContributorFeeStats(ctx context.Context, stats types.ContributorFeeStats) {
	addr, err := sdk.AccAddressFromBech32(stats.Address)
	if err != nil {
		panic(err)
	}

	store := k.storeService.OpenKVStore(ctx)
	key := append(ContributorFeeStatsKey, addr.Bytes()...)
	bz := k.cdc.MustMarshal(&stats)
	if err := store.Set(key, bz); err != nil {
		panic(err)
	}
}

// GetAllContributorFeeStats retrieves all contributor fee stats (for genesis export)
func (k Keeper) GetAllContributorFeeStats(ctx context.Context) []types.ContributorFeeStats {
	var allStats []types.ContributorFeeStats
	store := k.storeService.OpenKVStore(ctx)

	iterator, err := store.Iterator(ContributorFeeStatsKey, nil)
	if err != nil {
		panic(err)
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var stats types.ContributorFeeStats
		k.cdc.MustUnmarshal(iterator.Value(), &stats)
		allStats = append(allStats, stats)
	}

	return allStats
}
```

### Phase 5: Update msg_server_submit_contribution.go ⏳

**File**: `x/poc/keeper/msg_server_submit_contribution.go`

**Required Changes:**

Insert fee collection BEFORE creating the contribution:

```go
func (ms msgServer) SubmitContribution(
	goCtx context.Context,
	msg *types.MsgSubmitContribution,
) (*types.MsgSubmitContributionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Existing rate limit check...
	if err := ms.CheckRateLimit(goCtx); err != nil {
		return nil, err
	}

	// NEW: Collect and burn submission fee (BEFORE creating contribution)
	contributor, err := sdk.AccAddressFromBech32(msg.Contributor)
	if err != nil {
		return nil, fmt.Errorf("invalid contributor address: %w", err)
	}

	if err := ms.CollectAndBurnSubmissionFee(goCtx, contributor); err != nil {
		return nil, fmt.Errorf("submission fee collection failed: %w", err)
	}

	// Existing contribution creation logic...
	contributionID := ms.GetNextContributionID(goCtx)

	contribution := types.Contribution{
		Id:          contributionID,
		Contributor: msg.Contributor,
		Ctype:       msg.Ctype,
		Uri:         msg.Uri,
		Hash:        msg.Hash,
		Endorsements: []types.Endorsement{},
		Verified:    false,
		BlockHeight: ctx.BlockHeight(),
		BlockTime:   ctx.BlockTime().Unix(),
		Rewarded:    false,
	}

	ms.SetContribution(goCtx, contribution)

	// Existing event emission...
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"poc_contribution_submitted",
			sdk.NewAttribute("id", fmt.Sprintf("%d", contribution.Id)),
			sdk.NewAttribute("contributor", contribution.Contributor),
			sdk.NewAttribute("type", contribution.Ctype),
		),
	})

	return &types.MsgSubmitContributionResponse{
		ContributionId: contributionID,
	}, nil
}
```

### Phase 6: Implement Query Handlers ⏳

**File**: `x/poc/keeper/query.go`

**Add new query handlers:**

```go
// FeeMetrics queries the cumulative fee burn statistics
func (k Keeper) FeeMetrics(
	goCtx context.Context,
	req *types.QueryFeeMetricsRequest,
) (*types.QueryFeeMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	metrics := k.GetFeeMetrics(goCtx)

	return &types.QueryFeeMetricsResponse{
		Metrics: metrics,
	}, nil
}

// ContributorFeeStats queries fee statistics for a specific contributor
func (k Keeper) ContributorFeeStats(
	goCtx context.Context,
	req *types.QueryContributorFeeStatsRequest,
) (*types.QueryContributorFeeStatsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid address")
	}

	stats := k.GetContributorFeeStats(goCtx, addr)

	return &types.QueryContributorFeeStatsResponse{
		Stats: stats,
	}, nil
}
```

### Phase 7: Update Genesis State ⏳

**File**: `x/poc/keeper/genesis.go`

**Update InitGenesis:**

```go
func (k Keeper) InitGenesis(ctx context.Context, data types.GenesisState) error {
	// Existing initialization...
	if err := k.SetParams(ctx, data.Params); err != nil {
		return err
	}

	for _, contribution := range data.Contributions {
		k.SetContribution(ctx, contribution)
	}

	for _, credits := range data.Credits {
		k.SetCredits(ctx, credits)
	}

	k.SetNextContributionID(ctx, data.NextContributionId)

	// NEW: Initialize fee metrics
	k.SetFeeMetrics(ctx, data.FeeMetrics)

	// NEW: Initialize contributor fee stats
	for _, stats := range data.ContributorFeeStats {
		k.SetContributorFeeStats(ctx, stats)
	}

	return nil
}
```

**Update ExportGenesis:**

```go
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:            k.GetParams(ctx),
		Contributions:     k.GetAllContributions(ctx),
		Credits:           k.GetAllCredits(ctx),
		NextContributionId: k.GetNextContributionID(ctx),
		FeeMetrics:        k.GetFeeMetrics(ctx),                  // NEW
		ContributorFeeStats: k.GetAllContributorFeeStats(ctx),   // NEW
	}
}
```

### Phase 8: CLI Commands ⏳

**File**: `x/poc/module/cli.go` or new `x/poc/client/cli/query_fees.go`

**Add CLI query commands:**

```go
// GetQueryCmd returns the cli query commands for the poc module
func GetQueryCmd() *cobra.Command {
	pocQueryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the poc module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	pocQueryCmd.AddCommand(
		// Existing commands...
		CmdQueryParams(),
		CmdQueryContribution(),
		CmdQueryContributions(),
		CmdQueryCredits(),
		// NEW: Fee query commands
		CmdQueryFeeMetrics(),
		CmdQueryContributorFeeStats(),
	)

	return pocQueryCmd
}

// CmdQueryFeeMetrics queries the cumulative fee burn statistics
func CmdQueryFeeMetrics() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fee-metrics",
		Short: "Query cumulative fee burn statistics",
		Long: `Query the cumulative PoC submission fee statistics including:
- Total fees collected
- Total burned
- Total redirected to reward pool
- Last updated block height

Example:
$ posd query poc fee-metrics`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FeeMetrics(
				cmd.Context(),
				&types.QueryFeeMetricsRequest{},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryContributorFeeStats queries fee stats for a specific contributor
func CmdQueryContributorFeeStats() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contributor-fee-stats [address]",
		Short: "Query fee statistics for a specific contributor",
		Long: `Query submission fee statistics for a contributor including:
- Total fees paid
- Total burned from their fees
- Number of submissions
- First and last submission heights

Example:
$ posd query poc contributor-fee-stats omni1abc...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			address := args[0]

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ContributorFeeStats(
				cmd.Context(),
				&types.QueryContributorFeeStatsRequest{
					Address: address,
				},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

### Phase 9: Unit Tests ⏳

**File**: `x/poc/keeper/fee_burn_test.go` (NEW FILE)

**Required Test Cases:**

```go
package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"pos/x/poc/types"
)

// TestCollectAndBurnSubmissionFee_Success tests successful fee collection and burn
func TestCollectAndBurnSubmissionFee_Success(t *testing.T) {
	// Setup test environment
	// Create contributor account with sufficient balance
	// Call CollectAndBurnSubmissionFee
	// Verify:
	// - Fee deducted from contributor
	// - Correct amount burned
	// - Correct amount to rewards
	// - Metrics updated
	// - Events emitted
}

// TestCollectAndBurnSubmissionFee_InsufficientBalance tests failure when contributor has insufficient balance
func TestCollectAndBurnSubmissionFee_InsufficientBalance(t *testing.T) {
	// Setup contributor with balance < submission_fee
	// Call CollectAndBurnSubmissionFee
	// Verify: Returns error
	// Verify: No state changes (atomicity)
}

// TestCollectAndBurnSubmissionFee_BurnRatioEdgeCases tests edge cases for burn ratio
func TestCollectAndBurnSubmissionFee_BurnRatioEdgeCases(t *testing.T) {
	testCases := []struct {
		name       string
		burnRatio  math.LegacyDec
		feeAmount  math.Int
		expectBurn math.Int
		expectReward math.Int
	}{
		{
			name:      "50% burn ratio",
			burnRatio: math.LegacyNewDecWithPrec(50, 2),
			feeAmount: math.NewInt(1000),
			expectBurn: math.NewInt(500),
			expectReward: math.NewInt(500),
		},
		{
			name:      "90% burn ratio",
			burnRatio: math.LegacyNewDecWithPrec(90, 2),
			feeAmount: math.NewInt(1000),
			expectBurn: math.NewInt(900),
			expectReward: math.NewInt(100),
		},
		{
			name:      "75% with rounding",
			burnRatio: math.LegacyNewDecWithPrec(75, 2),
			feeAmount: math.NewInt(1001),
			expectBurn: math.NewInt(750),  // Truncated
			expectReward: math.NewInt(251),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test each case
		})
	}
}

// TestFeeMetrics_Accumulation tests cumulative metrics across multiple submissions
func TestFeeMetrics_Accumulation(t *testing.T) {
	// Submit 3 contributions from different contributors
	// Verify cumulative metrics are correct
	// Verify last_updated_height is latest
}

// TestContributorFeeStats_MultipleSubmissions tests per-contributor stats
func TestContributorFeeStats_MultipleSubmissions(t *testing.T) {
	// Submit 5 contributions from same contributor
	// Verify:
	// - Total fees paid = 5 * submission_fee
	// - Submission count = 5
	// - First/last submission heights correct
}

// TestParamValidation_FeeBounds tests parameter validation
func TestParamValidation_FeeBounds(t *testing.T) {
	testCases := []struct {
		name        string
		params      types.Params
		expectError bool
	}{
		{
			name: "fee within bounds - valid",
			params: types.Params{
				SubmissionFee:    sdk.NewCoin("uomni", math.NewInt(5000)),
				MinSubmissionFee: sdk.NewCoin("uomni", math.NewInt(100)),
				MaxSubmissionFee: sdk.NewCoin("uomni", math.NewInt(100000)),
			},
			expectError: false,
		},
		{
			name: "fee below minimum - invalid",
			params: types.Params{
				SubmissionFee:    sdk.NewCoin("uomni", math.NewInt(50)),
				MinSubmissionFee: sdk.NewCoin("uomni", math.NewInt(100)),
			},
			expectError: true,
		},
		{
			name: "fee above maximum - invalid",
			params: types.Params{
				SubmissionFee:    sdk.NewCoin("uomni", math.NewInt(200000)),
				MaxSubmissionFee: sdk.NewCoin("uomni", math.NewInt(100000)),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestParamValidation_BurnRatioBounds tests burn ratio validation
func TestParamValidation_BurnRatioBounds(t *testing.T) {
	// Test min/max burn ratio enforcement
}

// TestGenesisImportExport tests fee metrics in genesis
func TestGenesisImportExport(t *testing.T) {
	// Export genesis with fee metrics
	// Import into new chain
	// Verify all metrics preserved
}

// TestSubmissionWithFeeBurn_Integration tests full flow
func TestSubmissionWithFeeBurn_Integration(t *testing.T) {
	// Setup chain with default params
	// Contributor submits contribution
	// Verify:
	// 1. Fee collected
	// 2. Correct amount burned
	// 3. Correct amount to rewards
	// 4. Contribution created
	// 5. Events emitted
	// 6. Metrics updated
	// 7. Contributor stats updated
}
```

---

## 📊 Expected Behavior

### **Submission Flow:**

```bash
$ posd tx poc submit --type data --uri ipfs://Q... --from contributor1

💰 Submission fee paid: 1,000 uomni (0.001 OMNI)
🔥 Burned: 750 uomni
🎁 Added to PoC rewards: 250 uomni
✅ Contribution submitted with ID: 42
```

### **Query Examples:**

```bash
# Query global fee metrics
$ posd query poc fee-metrics
{
  "metrics": {
    "total_fees_collected": [{"denom": "uomni", "amount": "125000"}],
    "total_burned": [{"denom": "uomni", "amount": "93750"}],
    "total_reward_redirect": [{"denom": "uomni", "amount": "31250"}],
    "last_updated_height": "12345"
  }
}

# Query contributor stats
$ posd query poc contributor-fee-stats omni1abc...
{
  "stats": {
    "address": "omni1abc...",
    "total_fees_paid": [{"denom": "uomni", "amount": "5000"}],
    "total_burned": [{"denom": "uomni", "amount": "3750"}],
    "submission_count": "5",
    "first_submission_height": "1000",
    "last_submission_height": "12345"
  }
}
```

### **Governance Proposal:**

```bash
# Update fee parameters via governance
$ posd tx gov submit-proposal param-change proposal.json

# proposal.json example:
{
  "title": "Increase PoC Submission Fee",
  "description": "Increase submission fee to 5000uomni due to spam",
  "changes": [
    {
      "subspace": "poc",
      "key": "SubmissionFee",
      "value": "\"5000uomni\""
    }
  ]
}
```

---

## 🔐 Security Considerations

### **Implemented Protections:**

1. **Atomicity**: Fee collection, burn, and reward split are atomic - if any step fails, entire transaction reverts
2. **Validation**: Strict parameter validation with min/max bounds
3. **Overflow Protection**: Uses math.Int and math.LegacyDec (safe arithmetic)
4. **Re-entrancy Prevention**: Follows existing PoC module patterns (seen in WithdrawCredits)
5. **Event Emission**: Complete audit trail for all fee operations
6. **Balance Checks**: BankKeeper enforces sufficient balance before transfer

### **Attack Vectors Mitigated:**

- **Fee Bypass**: Cannot submit without paying fee (checked in msg_server)
- **Parameter Manipulation**: DAO-governed with bounds (50%-90% burn ratio)
- **State Corruption**: Atomic operations prevent partial state updates
- **Spam**: Fee creates economic cost for submissions
- **Governance Attacks**: Min/max bounds prevent extreme parameter values

---

## 🚀 Deployment Checklist

### **Pre-Deployment:**

- [ ] Complete proto generation (fix buf configuration)
- [ ] Implement all keeper methods (fee_burn.go)
- [ ] Update msg_server_submit_contribution.go
- [ ] Implement query handlers
- [ ] Update genesis init/export
- [ ] Add CLI commands
- [ ] Write all unit tests (target: 90%+ coverage)
- [ ] Run integration tests
- [ ] Audit by security team

### **Deployment:**

- [ ] Create genesis with default fee parameters
- [ ] Deploy to testnet
- [ ] Test submission flow end-to-end
- [ ] Test governance parameter updates
- [ ] Monitor fee metrics
- [ ] Verify supply reduction from burns
- [ ] Deploy to mainnet

### **Post-Deployment:**

- [ ] Monitor daily burn rate
- [ ] Track contributor adoption
- [ ] Analyze fee rebate opportunities for high-quality contributors
- [ ] Create governance proposal templates
- [ ] Update user documentation

---

## 📈 Success Metrics

### **KPIs to Track:**

- **Total fees collected** (weekly/monthly)
- **Total burned** (deflationary impact)
- **Total to rewards pool** (incentive pool growth)
- **Submissions per day** (activity level)
- **Unique contributors** (network participation)
- **Average fee per contributor** (engagement level)

### **Expected Impact:**

- **Deflationary Pressure**: 75% of fees permanently removed from circulation
- **Spam Reduction**: Economic cost discourages low-quality submissions
- **Reward Pool Growth**: 25% of fees fund contributor rewards
- **DAO Control**: Community can adjust fees based on network needs

---

## 🔗 Integration Points

### **Tokenomics Module:**

- PoC fees are **independent** of inflation distribution
- Fees provide **additional deflationary mechanism** beyond base burn
- Reward pool can be used in conjunction with inflation-based rewards

### **Governance Module:**

- All fee parameters are **DAO-governed**
- Proposals can adjust fee amounts and burn ratios
- Min/max bounds prevent extreme changes

### **Bank Module:**

- Uses `SendCoinsFromAccountToModule` for fee collection
- Uses `BurnCoins` for deflationary mechanism
- No new module accounts required (uses existing poc module account)

---

## 📝 Next Steps

1. **Fix Proto Generation** (BLOCKED - see Phase 2)
   - Debug buf configuration
   - Regenerate all .pb.go files
   - Verify compilation

2. **Implement Phase 3-5** (Core Logic)
   - params.go validation
   - fee_burn.go keeper methods
   - msg_server update

3. **Implement Phase 6-7** (Queries & Genesis)
   - Query handlers
   - Genesis init/export

4. **Implement Phase 8-9** (CLI & Tests)
   - CLI commands
   - Comprehensive test suite

5. **Integration Testing**
   - End-to-end submission flow
   - Governance param updates
   - Genesis import/export

6. **Documentation**
   - User guide
   - API documentation
   - Integration examples

---

## 🎓 Educational Notes

### **Why 75% Burn / 25% Rewards?**

- **Deflationary First**: Primary goal is to reduce supply (tokenomics health)
- **Incentive Alignment**: 25% ensures quality contributions are rewarded
- **DAO Flexibility**: Governance can adjust ratio between 50%-90%
- **Market Dynamics**: Higher burn = deflationary pressure, lower burn = more rewards

### **Why Min/Max Bounds?**

- **Prevent Extremes**: Protects against governance attacks
- **Predictability**: Contributors know fee won't spike overnight
- **Stability**: Smooth parameter transitions via governance

### **Why Per-Contributor Stats?**

- **Future Features**: Enables fee rebates for high-quality contributors
- **Analytics**: Understand contributor behavior and engagement
- **Transparency**: Contributors can see their fee history

---

**STATUS**: Design and proto definitions complete. Ready for implementation of keeper logic, msg server updates, and tests.

**ESTIMATED EFFORT**:
- Core Implementation: 8-12 hours
- Testing: 4-6 hours
- Integration: 2-4 hours
- **Total**: ~20 hours for senior developer

**BLOCKERS**:
- Proto generation buf configuration (can be worked around with manual generation)

---

Generated by Claude Code (Senior Blockchain Developer Mode)
Document Version: 1.0
Last Updated: 2025-10-27
