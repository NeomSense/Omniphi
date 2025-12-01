package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"

	"pos/x/tokenomics/types"
)

// InitGenesis initializes the tokenomics module state from genesis
// P0-GEN-001 to P0-GEN-006: Genesis integrity
func (k Keeper) InitGenesis(ctx context.Context, data types.GenesisState) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// P0-GEN-004: Validate genesis state
	if err := data.Validate(); err != nil {
		return fmt.Errorf("invalid genesis state: %w", err)
	}

	// P0-GEN-001: Validate allocations sum to genesis supply
	totalAllocated := math.ZeroInt()
	for _, allocation := range data.Allocations {
		totalAllocated = totalAllocated.Add(allocation.Amount)
	}

	if !totalAllocated.Equal(data.SupplyState.CurrentTotalSupply) {
		return fmt.Errorf(
			"genesis allocations (%s) do not equal genesis supply (%s)",
			totalAllocated.String(),
			data.SupplyState.CurrentTotalSupply.String(),
		)
	}

	// Set parameters
	if err := k.SetParams(ctx, data.Params); err != nil {
		return fmt.Errorf("failed to set parameters: %w", err)
	}

	// Initialize supply counters
	if err := k.SetCurrentSupply(ctx, data.SupplyState.CurrentTotalSupply); err != nil {
		return fmt.Errorf("failed to set current supply: %w", err)
	}

	if err := k.SetTotalMinted(ctx, data.SupplyState.TotalMinted); err != nil {
		return fmt.Errorf("failed to set total minted: %w", err)
	}

	if err := k.SetTotalBurned(ctx, data.SupplyState.TotalBurned); err != nil {
		return fmt.Errorf("failed to set total burned: %w", err)
	}

	// P0-GEN-005: Process genesis allocations (including vesting)
	for _, allocation := range data.Allocations {
		if err := k.ProcessGenesisAllocation(ctx, allocation); err != nil {
			return fmt.Errorf("failed to process allocation for %s: %w", allocation.Address, err)
		}
	}

	// Initialize treasury state
	if data.TreasuryState.TreasuryAddress != "" {
		treasuryAddr, err := sdk.AccAddressFromBech32(data.TreasuryState.TreasuryAddress)
		if err != nil {
			return fmt.Errorf("invalid treasury address: %w", err)
		}

		if err := k.SetTreasuryAddress(ctx, treasuryAddr); err != nil {
			return fmt.Errorf("failed to set treasury address: %w", err)
		}

		// Set treasury inflows
		store := k.storeService.OpenKVStore(ctx)
		bz, err := data.TreasuryState.TotalInflows.Marshal()
		if err != nil {
			return fmt.Errorf("failed to marshal treasury inflows: %w", err)
		}
		if err := store.Set(types.KeyTreasuryInflows, bz); err != nil {
			return fmt.Errorf("failed to set treasury inflows: %w", err)
		}
	}

	// Initialize burn records (for chain upgrades)
	for _, record := range data.BurnRecords {
		store := k.storeService.OpenKVStore(ctx)
		bz := k.cdc.MustMarshal(&record)
		if err := store.Set(types.GetBurnRecordKey(record.BurnId), bz); err != nil {
			return fmt.Errorf("failed to set burn record %d: %w", record.BurnId, err)
		}

		// Update per-source and per-chain counters
		k.IncrementBurnsBySource(ctx, record.Source, record.Amount)
		k.IncrementBurnsByChain(ctx, record.ChainId, record.Amount)
	}

	// Initialize emission records (for chain upgrades)
	for _, record := range data.EmissionRecords {
		store := k.storeService.OpenKVStore(ctx)
		bz := k.cdc.MustMarshal(&record)
		if err := store.Set(types.GetEmissionRecordKey(record.EmissionId), bz); err != nil {
			return fmt.Errorf("failed to set emission record %d: %w", record.EmissionId, err)
		}
	}

	// Initialize chain states
	for _, chainState := range data.ChainStates {
		if err := k.SetChainState(ctx, chainState); err != nil {
			return fmt.Errorf("failed to set chain state for %s: %w", chainState.ChainId, err)
		}
	}

	// Emit genesis initialization event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"tokenomics_genesis_initialized",
			sdk.NewAttribute("genesis_supply", data.SupplyState.CurrentTotalSupply.String()),
			sdk.NewAttribute("allocations_count", fmt.Sprintf("%d", len(data.Allocations))),
			sdk.NewAttribute("treasury_address", data.TreasuryState.TreasuryAddress),
		),
	)

	k.Logger(ctx).Info("tokenomics genesis initialized",
		"genesis_supply", data.SupplyState.CurrentTotalSupply.String(),
		"allocations", len(data.Allocations),
		"treasury", data.TreasuryState.TreasuryAddress,
	)

	return nil
}

// ProcessGenesisAllocation processes a single genesis allocation
// P0-GEN-005: Vesting accounts created correctly
func (k Keeper) ProcessGenesisAllocation(ctx context.Context, allocation types.GenesisAllocation) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	recipient, err := sdk.AccAddressFromBech32(allocation.Address)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, allocation.Amount))

	if allocation.IsVested && allocation.VestingSchedule != nil {
		// P0-GEN-005: Create vesting account
		return k.CreateVestingAccount(ctx, recipient, coins, *allocation.VestingSchedule)
	}

	// Direct allocation (no vesting)
	// Mint to module account first
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return fmt.Errorf("failed to mint coins for allocation: %w", err)
	}

	// Transfer to recipient
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, coins); err != nil {
		return fmt.Errorf("failed to send coins to recipient: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"genesis_allocation",
			sdk.NewAttribute("recipient", allocation.Address),
			sdk.NewAttribute("amount", allocation.Amount.String()),
			sdk.NewAttribute("category", allocation.Category.String()),
			sdk.NewAttribute("is_vested", fmt.Sprintf("%t", allocation.IsVested)),
		),
	)

	return nil
}

// CreateVestingAccount creates a vesting account for a genesis allocation
// P0-GEN-005: Vesting accounts created correctly
func (k Keeper) CreateVestingAccount(
	ctx context.Context,
	address sdk.AccAddress,
	coins sdk.Coins,
	schedule types.VestingSchedule,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get the account (or create base account if doesn't exist)
	acc := k.accountKeeper.GetAccount(ctx, address)
	if acc == nil {
		acc = authtypes.NewBaseAccountWithAddress(address)
		k.accountKeeper.SetAccount(ctx, acc)
	}

	// Calculate start and end times
	startTime := schedule.StartTime
	if startTime == 0 {
		startTime = sdkCtx.BlockTime().Unix()
	}

	endTime := startTime + int64(schedule.CliffDuration) + int64(schedule.VestingDuration)

	// Mint coins to module account
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return fmt.Errorf("failed to mint coins for vesting: %w", err)
	}

	// Create continuous vesting account or periodic vesting account
	var vestingAcc authtypes.AccountI

	var err error
	if schedule.IsContinuous {
		// Continuous (linear) vesting
		vestingAcc, err = vestingtypes.NewContinuousVestingAccount(
			acc.(*authtypes.BaseAccount),
			coins,
			startTime,
			endTime,
		)
		if err != nil {
			return fmt.Errorf("failed to create continuous vesting account: %w", err)
		}
	} else {
		// Periodic (milestone-based) vesting
		// For now, use continuous vesting
		// In production, would implement custom periodic schedule
		vestingAcc, err = vestingtypes.NewContinuousVestingAccount(
			acc.(*authtypes.BaseAccount),
			coins,
			startTime,
			endTime,
		)
		if err != nil {
			return fmt.Errorf("failed to create periodic vesting account: %w", err)
		}
	}

	// Set the vesting account
	k.accountKeeper.SetAccount(ctx, vestingAcc)

	// Transfer coins from module to vesting account
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, address, coins); err != nil {
		return fmt.Errorf("failed to send coins to vesting account: %w", err)
	}

	k.Logger(ctx).Info("created vesting account",
		"address", address.String(),
		"amount", coins.String(),
		"start_time", startTime,
		"end_time", endTime,
		"cliff_duration", schedule.CliffDuration,
		"vesting_duration", schedule.VestingDuration,
	)

	return nil
}

// ExportGenesis exports the tokenomics module state to genesis
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)

	supplyState := types.SupplyState{
		CurrentTotalSupply: k.GetCurrentSupply(ctx),
		TotalMinted:        k.GetTotalMinted(ctx),
		TotalBurned:        k.GetTotalBurned(ctx),
	}

	// Export treasury state
	treasuryAddr := k.GetTreasuryAddress(ctx)
	store := k.storeService.OpenKVStore(ctx)

	var totalInflows math.Int
	bz, err := store.Get(types.KeyTreasuryInflows)
	if err == nil && bz != nil {
		_ = totalInflows.Unmarshal(bz)
	} else {
		totalInflows = math.ZeroInt()
	}

	treasuryState := types.TreasuryState{
		TreasuryAddress: treasuryAddr.String(),
		InitialBalance:  k.bankKeeper.GetBalance(ctx, treasuryAddr, types.BondDenom).Amount,
		TotalInflows:    totalInflows,
		FromInflation:   math.ZeroInt(), // TODO: Track separately
		FromBurnRedirect: totalInflows,
	}

	// Export chain states
	chainStates := k.GetAllChainStates(ctx)

	// Note: For normal genesis export, we don't export allocations
	// Allocations are only in initial genesis
	// Burn records and emission records can be exported for chain upgrades

	return &types.GenesisState{
		Params:          params,
		SupplyState:     supplyState,
		Allocations:     []types.GenesisAllocation{}, // Empty for export
		BurnRecords:     []types.BurnRecord{},        // Could export if needed
		EmissionRecords: []types.EmissionRecord{},    // Could export if needed
		TreasuryState:   treasuryState,
		ChainStates:     chainStates,
	}
}

// SetChainState sets the state for a specific chain
func (k Keeper) SetChainState(ctx context.Context, state types.ChainState) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetChainStateKey(state.ChainId)
	bz := k.cdc.MustMarshal(&state)
	return store.Set(key, bz)
}

// GetChainState gets the state for a specific chain
func (k Keeper) GetChainState(ctx context.Context, chainID string) (types.ChainState, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetChainStateKey(chainID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.ChainState{}, false
	}

	var state types.ChainState
	k.cdc.MustUnmarshal(bz, &state)
	return state, true
}

// GetAllChainStates gets all chain states
func (k Keeper) GetAllChainStates(ctx context.Context) []types.ChainState {
	var states []types.ChainState

	// Predefined chains (in production, would iterate store)
	chainIDs := []string{
		"omniphi-core-1",
		"omniphi-continuity-1",
		"omniphi-sequencer-1",
	}

	for _, chainID := range chainIDs {
		if state, found := k.GetChainState(ctx, chainID); found {
			states = append(states, state)
		}
	}

	return states
}

// DefaultGenesisState returns the default genesis state
func DefaultGenesisState() *types.GenesisState {
	params := types.DefaultParams()

	// Default genesis: Empty supply (will be set by bank module genesis accounts)
	// This allows flexible genesis account configuration without hardcoded allocations
	genesisSupply := math.ZeroInt()

	supplyState := types.SupplyState{
		CurrentTotalSupply: genesisSupply,
		TotalMinted:        genesisSupply,
		TotalBurned:        math.ZeroInt(),
	}

	// Empty allocations - actual genesis accounts are managed by bank module
	// This prevents conflicts with genesis add-genesis-account commands
	allocations := []types.GenesisAllocation{}

	// Use valid treasury address with correct checksum
	// Generated address for treasury operations
	treasuryState := types.TreasuryState{
		TreasuryAddress:  "omni1fxrswwy7a63xvrcsj6pcwvna923gluh7z8nvtl",
		InitialBalance:   math.ZeroInt(),
		TotalInflows:     math.ZeroInt(),
		FromInflation:    math.ZeroInt(),
		FromBurnRedirect: math.ZeroInt(),
	}

	// Initialize chain states
	chainStates := []types.ChainState{
		{
			ChainId:              "omniphi-core-1",
			TotalBurned:          math.ZeroInt(),
			TotalRewardsSent:     math.ZeroInt(),
			IbcChannel:           "",
			IsActive:             true,
			LastSyncHeight:       0,
		},
		{
			ChainId:              "omniphi-continuity-1",
			TotalBurned:          math.ZeroInt(),
			TotalRewardsSent:     math.ZeroInt(),
			IbcChannel:           "channel-0",
			IsActive:             false, // Not active at genesis
			LastSyncHeight:       0,
		},
		{
			ChainId:              "omniphi-sequencer-1",
			TotalBurned:          math.ZeroInt(),
			TotalRewardsSent:     math.ZeroInt(),
			IbcChannel:           "channel-1",
			IsActive:             false, // Not active at genesis
			LastSyncHeight:       0,
		},
	}

	return &types.GenesisState{
		Params:          params,
		SupplyState:     supplyState,
		Allocations:     allocations,
		BurnRecords:     []types.BurnRecord{},
		EmissionRecords: []types.EmissionRecord{},
		TreasuryState:   treasuryState,
		ChainStates:     chainStates,
	}
}

// NOTE: Validation functions moved to types/genesis_validation.go
