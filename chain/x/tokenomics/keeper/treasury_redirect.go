package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// ============================================================================
// TREASURY REDIRECT MECHANISM
// ============================================================================
// Treasury Redirect is a POST-COLLECTION allocation rule, NOT a fee.
// It operates ONLY on funds already allocated to treasury inflows.
//
// Flow: User → Fees → Burn → Validator/Treasury Split → Treasury Redirect
//
// Security Requirements:
// - REDIRECT-001: Max redirect ratio capped at 10% (protocol enforced)
// - REDIRECT-002: Only operates on NEW inflows, not total treasury balance
// - REDIRECT-003: Redirect targets must be whitelisted addresses
// - REDIRECT-004: Execution is atomic (all or nothing)
// - REDIRECT-005: No impact on validator revenue
// - REDIRECT-006: No double taxation (operates post-collection only)

const (
	// MaxRedirectRatio is the protocol-enforced maximum redirect ratio (10%)
	// Governance CANNOT exceed this value
	MaxRedirectRatio = "0.10"

	// DefaultRedirectInterval is the default execution interval (100 blocks)
	DefaultRedirectInterval = uint64(100)
)

// RedirectTarget represents a destination for redirected funds
type RedirectTarget struct {
	Name    string
	Address sdk.AccAddress
	Ratio   math.LegacyDec
}

// RedirectResult contains the result of a redirect execution
type RedirectResult struct {
	TotalInflows    math.Int
	RedirectAmount  math.Int
	RetainedAmount  math.Int
	Allocations     []RedirectAllocation
	ExecutedAtBlock int64
}

// RedirectAllocation represents a single allocation to a target
type RedirectAllocation struct {
	Target  string
	Address sdk.AccAddress
	Amount  math.Int
	Ratio   math.LegacyDec
}

// ProcessTreasuryRedirect executes the treasury redirect mechanism
// This should be called during EndBlock at the configured interval
func (k Keeper) ProcessTreasuryRedirect(ctx context.Context) (*RedirectResult, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Check if redirect is enabled
	if !params.TreasuryRedirectEnabled {
		k.Logger(ctx).Debug("treasury redirect disabled, skipping")
		return nil, nil
	}

	// Check if it's time to execute (based on interval)
	currentHeight := sdkCtx.BlockHeight()
	lastRedirectHeight := k.GetLastRedirectHeight(ctx)
	interval := params.RedirectExecutionInterval
	if interval == 0 {
		interval = DefaultRedirectInterval
	}

	if currentHeight-lastRedirectHeight < int64(interval) {
		// Not yet time to execute
		return nil, nil
	}

	// Get accumulated inflows since last redirect
	accumulatedInflows := k.GetAccumulatedRedirectInflows(ctx)
	if accumulatedInflows.IsZero() {
		// No inflows to redirect
		k.SetLastRedirectHeight(ctx, currentHeight)
		return nil, nil
	}

	// REDIRECT-001: Enforce protocol cap on redirect ratio
	redirectRatio := params.TreasuryRedirectRatio
	maxRatio := math.LegacyMustNewDecFromStr(MaxRedirectRatio)
	if redirectRatio.GT(maxRatio) {
		k.Logger(ctx).Warn("redirect ratio exceeds protocol cap, clamping to max",
			"requested", redirectRatio.String(),
			"max", maxRatio.String())
		redirectRatio = maxRatio
	}

	// Calculate redirect amount (max 10% of inflows)
	redirectAmount := redirectRatio.MulInt(accumulatedInflows).TruncateInt()
	retainedAmount := accumulatedInflows.Sub(redirectAmount)

	if redirectAmount.IsZero() {
		// Nothing to redirect
		k.SetLastRedirectHeight(ctx, currentHeight)
		k.ResetAccumulatedRedirectInflows(ctx)
		return nil, nil
	}

	// Validate target ratios sum to 100%
	targetRatios := []math.LegacyDec{
		params.RedirectToEcosystemGrants,
		params.RedirectToBuyAndBurn,
		params.RedirectToInsuranceFund,
		params.RedirectToResearchFund,
	}

	sum := math.LegacyZeroDec()
	for _, ratio := range targetRatios {
		sum = sum.Add(ratio)
	}

	if !sum.Equal(math.LegacyOneDec()) {
		return nil, fmt.Errorf("redirect target ratios must sum to 1.0, got %s", sum.String())
	}

	// Get redirect target addresses
	targets, err := k.GetRedirectTargets(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get redirect targets: %w", err)
	}

	// Get treasury address (source of funds)
	treasuryAddr := k.GetTreasuryAddress(ctx)

	// REDIRECT-004: Execute allocations atomically
	allocations := make([]RedirectAllocation, 0, len(targets))
	totalAllocated := math.ZeroInt()

	for i, target := range targets {
		// Calculate allocation for this target
		var allocationAmount math.Int
		if i == len(targets)-1 {
			// Last target gets remainder to avoid dust
			allocationAmount = redirectAmount.Sub(totalAllocated)
		} else {
			allocationAmount = target.Ratio.MulInt(redirectAmount).TruncateInt()
		}

		if allocationAmount.IsZero() {
			continue
		}

		// Transfer from treasury to target
		coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, allocationAmount))
		if err := k.bankKeeper.SendCoins(ctx, treasuryAddr, target.Address, coins); err != nil {
			return nil, fmt.Errorf("failed to transfer to %s: %w", target.Name, err)
		}

		allocations = append(allocations, RedirectAllocation{
			Target:  target.Name,
			Address: target.Address,
			Amount:  allocationAmount,
			Ratio:   target.Ratio,
		})

		totalAllocated = totalAllocated.Add(allocationAmount)

		// Emit allocation event
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeTreasuryAllocation,
				sdk.NewAttribute(types.AttributeKeyAllocationTarget, target.Name),
				sdk.NewAttribute(types.AttributeKeyAllocationAmount, allocationAmount.String()),
				sdk.NewAttribute(types.AttributeKeyAllocationRatio, target.Ratio.String()),
			),
		)

		k.Logger(ctx).Info("treasury redirect allocation",
			"target", target.Name,
			"amount", allocationAmount.String(),
			"ratio", target.Ratio.String())
	}

	// Update state
	k.SetLastRedirectHeight(ctx, currentHeight)
	k.ResetAccumulatedRedirectInflows(ctx)
	k.IncrementTotalRedirected(ctx, totalAllocated)

	// Emit main redirect event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeTreasuryRedirect,
			sdk.NewAttribute(types.AttributeKeyTotalInflows, accumulatedInflows.String()),
			sdk.NewAttribute(types.AttributeKeyRedirectAmount, totalAllocated.String()),
			sdk.NewAttribute(types.AttributeKeyRetainedAmount, retainedAmount.String()),
			sdk.NewAttribute(types.AttributeKeyRedirectRatio, redirectRatio.String()),
			sdk.NewAttribute(types.AttributeKeyRedirectBlockHeight, fmt.Sprintf("%d", currentHeight)),
		),
	)

	k.Logger(ctx).Info("treasury redirect executed",
		"total_inflows", accumulatedInflows.String(),
		"redirected", totalAllocated.String(),
		"retained", retainedAmount.String(),
		"redirect_ratio", redirectRatio.String(),
		"allocations_count", len(allocations),
		"block_height", currentHeight)

	return &RedirectResult{
		TotalInflows:    accumulatedInflows,
		RedirectAmount:  totalAllocated,
		RetainedAmount:  retainedAmount,
		Allocations:     allocations,
		ExecutedAtBlock: currentHeight,
	}, nil
}

// GetRedirectTargets returns the configured redirect target addresses and ratios
func (k Keeper) GetRedirectTargets(ctx context.Context, params types.TokenomicsParams) ([]RedirectTarget, error) {
	targets := make([]RedirectTarget, 0, 4)

	// Ecosystem Grants
	if !params.RedirectToEcosystemGrants.IsZero() {
		addr := k.GetEcosystemGrantsAddress(ctx)
		if addr == nil {
			return nil, fmt.Errorf("ecosystem grants address not configured")
		}
		targets = append(targets, RedirectTarget{
			Name:    "ecosystem_grants",
			Address: addr,
			Ratio:   params.RedirectToEcosystemGrants,
		})
	}

	// Buy and Burn
	if !params.RedirectToBuyAndBurn.IsZero() {
		addr := k.GetBuyAndBurnAddress(ctx)
		if addr == nil {
			return nil, fmt.Errorf("buy and burn address not configured")
		}
		targets = append(targets, RedirectTarget{
			Name:    "buy_and_burn",
			Address: addr,
			Ratio:   params.RedirectToBuyAndBurn,
		})
	}

	// Insurance Fund
	if !params.RedirectToInsuranceFund.IsZero() {
		addr := k.GetInsuranceFundAddress(ctx)
		if addr == nil {
			return nil, fmt.Errorf("insurance fund address not configured")
		}
		targets = append(targets, RedirectTarget{
			Name:    "insurance_fund",
			Address: addr,
			Ratio:   params.RedirectToInsuranceFund,
		})
	}

	// Research Fund
	if !params.RedirectToResearchFund.IsZero() {
		addr := k.GetResearchFundAddress(ctx)
		if addr == nil {
			return nil, fmt.Errorf("research fund address not configured")
		}
		targets = append(targets, RedirectTarget{
			Name:    "research_fund",
			Address: addr,
			Ratio:   params.RedirectToResearchFund,
		})
	}

	return targets, nil
}

// ============================================================================
// STATE ACCESSORS
// ============================================================================

// GetAccumulatedRedirectInflows returns the accumulated inflows since last redirect
func (k Keeper) GetAccumulatedRedirectInflows(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyAccumulatedRedirectInflows)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var amount math.Int
	if err := amount.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return amount
}

// SetAccumulatedRedirectInflows sets the accumulated inflows
func (k Keeper) SetAccumulatedRedirectInflows(ctx context.Context, amount math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := amount.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.KeyAccumulatedRedirectInflows, bz)
}

// IncrementAccumulatedRedirectInflows adds to accumulated inflows
func (k Keeper) IncrementAccumulatedRedirectInflows(ctx context.Context, amount math.Int) {
	current := k.GetAccumulatedRedirectInflows(ctx)
	newAmount := current.Add(amount)
	_ = k.SetAccumulatedRedirectInflows(ctx, newAmount)
}

// ResetAccumulatedRedirectInflows resets accumulated inflows to zero
func (k Keeper) ResetAccumulatedRedirectInflows(ctx context.Context) {
	_ = k.SetAccumulatedRedirectInflows(ctx, math.ZeroInt())
}

// GetLastRedirectHeight returns the block height of last redirect execution
func (k Keeper) GetLastRedirectHeight(ctx context.Context) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyLastRedirectHeight)
	if err != nil || bz == nil {
		return 0
	}

	if len(bz) != 8 {
		return 0
	}

	height := int64(bz[0])<<56 | int64(bz[1])<<48 | int64(bz[2])<<40 | int64(bz[3])<<32 |
		int64(bz[4])<<24 | int64(bz[5])<<16 | int64(bz[6])<<8 | int64(bz[7])
	return height
}

// SetLastRedirectHeight sets the block height of last redirect execution
func (k Keeper) SetLastRedirectHeight(ctx context.Context, height int64) {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	bz[0] = byte(height >> 56)
	bz[1] = byte(height >> 48)
	bz[2] = byte(height >> 40)
	bz[3] = byte(height >> 32)
	bz[4] = byte(height >> 24)
	bz[5] = byte(height >> 16)
	bz[6] = byte(height >> 8)
	bz[7] = byte(height)
	_ = store.Set(types.KeyLastRedirectHeight, bz)
}

// GetTotalRedirected returns the cumulative amount redirected
func (k Keeper) GetTotalRedirected(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyTotalRedirected)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var amount math.Int
	if err := amount.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return amount
}

// IncrementTotalRedirected adds to cumulative redirected amount
func (k Keeper) IncrementTotalRedirected(ctx context.Context, amount math.Int) {
	store := k.storeService.OpenKVStore(ctx)
	current := k.GetTotalRedirected(ctx)
	newTotal := current.Add(amount)
	if bz, err := newTotal.Marshal(); err == nil {
		_ = store.Set(types.KeyTotalRedirected, bz)
	}
}

// ============================================================================
// TARGET ADDRESS ACCESSORS
// ============================================================================

// GetEcosystemGrantsAddress returns the ecosystem grants target address
func (k Keeper) GetEcosystemGrantsAddress(ctx context.Context) sdk.AccAddress {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyEcosystemGrantsAddress)
	if err != nil || bz == nil {
		return nil
	}
	return sdk.AccAddress(bz)
}

// SetEcosystemGrantsAddress sets the ecosystem grants target address
func (k Keeper) SetEcosystemGrantsAddress(ctx context.Context, addr sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyEcosystemGrantsAddress, addr.Bytes())
}

// GetBuyAndBurnAddress returns the buy-and-burn target address
func (k Keeper) GetBuyAndBurnAddress(ctx context.Context) sdk.AccAddress {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyBuyAndBurnAddress)
	if err != nil || bz == nil {
		return nil
	}
	return sdk.AccAddress(bz)
}

// SetBuyAndBurnAddress sets the buy-and-burn target address
func (k Keeper) SetBuyAndBurnAddress(ctx context.Context, addr sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyBuyAndBurnAddress, addr.Bytes())
}

// GetInsuranceFundAddress returns the insurance fund target address
func (k Keeper) GetInsuranceFundAddress(ctx context.Context) sdk.AccAddress {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyInsuranceFundAddress)
	if err != nil || bz == nil {
		return nil
	}
	return sdk.AccAddress(bz)
}

// SetInsuranceFundAddress sets the insurance fund target address
func (k Keeper) SetInsuranceFundAddress(ctx context.Context, addr sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyInsuranceFundAddress, addr.Bytes())
}

// GetResearchFundAddress returns the research fund target address
func (k Keeper) GetResearchFundAddress(ctx context.Context) sdk.AccAddress {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyResearchFundAddress)
	if err != nil || bz == nil {
		return nil
	}
	return sdk.AccAddress(bz)
}

// SetResearchFundAddress sets the research fund target address
func (k Keeper) SetResearchFundAddress(ctx context.Context, addr sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyResearchFundAddress, addr.Bytes())
}

// ============================================================================
// VALIDATION HELPERS
// ============================================================================

// ValidateTreasuryRedirectParams validates treasury redirect parameters
func ValidateTreasuryRedirectParams(params types.TokenomicsParams) error {
	// Validate redirect ratio is within protocol cap
	maxRatio := math.LegacyMustNewDecFromStr(MaxRedirectRatio)
	if params.TreasuryRedirectRatio.GT(maxRatio) {
		return fmt.Errorf("treasury redirect ratio %s exceeds protocol cap %s",
			params.TreasuryRedirectRatio.String(), maxRatio.String())
	}

	if params.TreasuryRedirectRatio.IsNegative() {
		return fmt.Errorf("treasury redirect ratio cannot be negative")
	}

	// Validate target ratios
	if params.TreasuryRedirectEnabled {
		targetSum := params.RedirectToEcosystemGrants.
			Add(params.RedirectToBuyAndBurn).
			Add(params.RedirectToInsuranceFund).
			Add(params.RedirectToResearchFund)

		if !targetSum.Equal(math.LegacyOneDec()) {
			return fmt.Errorf("redirect target ratios must sum to 1.0, got %s", targetSum.String())
		}

		// Validate individual ratios are non-negative
		if params.RedirectToEcosystemGrants.IsNegative() {
			return fmt.Errorf("ecosystem grants ratio cannot be negative")
		}
		if params.RedirectToBuyAndBurn.IsNegative() {
			return fmt.Errorf("buy and burn ratio cannot be negative")
		}
		if params.RedirectToInsuranceFund.IsNegative() {
			return fmt.Errorf("insurance fund ratio cannot be negative")
		}
		if params.RedirectToResearchFund.IsNegative() {
			return fmt.Errorf("research fund ratio cannot be negative")
		}
	}

	// Validate execution interval
	if params.RedirectExecutionInterval > 10000 {
		return fmt.Errorf("redirect execution interval cannot exceed 10000 blocks")
	}

	return nil
}
