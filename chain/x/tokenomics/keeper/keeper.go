package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// Keeper maintains the link to storage and exposes getter/setter methods for the various parts of the state machine
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger

	// Expected keepers
	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper
	govKeeper     types.GovKeeper
	ibcKeeper     types.IBCKeeper

	// Module authority (x/gov module account)
	authority string
}

// NewKeeper creates a new tokenomics Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
	govKeeper types.GovKeeper,
	ibcKeeper types.IBCKeeper,
	authority string,
) Keeper {
	// Ensure required keepers are set
	if accountKeeper == nil {
		panic("accountKeeper cannot be nil")
	}
	if bankKeeper == nil {
		panic("bankKeeper cannot be nil")
	}
	if stakingKeeper == nil {
		panic("stakingKeeper cannot be nil")
	}

	// Ensure the module account is set
	if addr := accountKeeper.GetModuleAddress(types.ModuleName); addr == nil {
		panic(fmt.Sprintf("module account %s has not been set", types.ModuleName))
	}

	// GovKeeper and IBCKeeper are optional for testnet
	// They will be wired up when IBC and advanced governance features are enabled
	if govKeeper == nil {
		logger.Warn("GovKeeper is nil - governance integration will be limited")
	}
	if ibcKeeper == nil {
		logger.Warn("IBCKeeper is nil - cross-chain features will be disabled")
	}

	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		stakingKeeper: stakingKeeper,
		govKeeper:     govKeeper,
		ibcKeeper:     ibcKeeper,
		authority:     authority,
	}
}

// GetAuthority returns the module's authority
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.logger.With("module", "x/"+types.ModuleName, "height", sdkCtx.BlockHeight())
}

// GetParams retrieves the module parameters
func (k Keeper) GetParams(ctx context.Context) types.TokenomicsParams {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil || bz == nil {
		// Return default params if not set
		return types.DefaultParams()
	}

	var params types.TokenomicsParams
	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// SetParams sets the module parameters
func (k Keeper) SetParams(ctx context.Context, params types.TokenomicsParams) error {
	// P0-CAP-005: Enforce supply cap immutability
	// The total supply cap cannot be changed after initialization
	existingParams := k.GetParams(ctx)
	if !existingParams.TotalSupplyCap.IsZero() && !params.TotalSupplyCap.Equal(existingParams.TotalSupplyCap) {
		return fmt.Errorf("total supply cap is immutable and cannot be changed from %s to %s",
			existingParams.TotalSupplyCap.String(), params.TotalSupplyCap.String())
	}

	// Validate params before setting
	if err := params.Validate(); err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&params)
	return store.Set(types.ParamsKey, bz)
}

// GetCurrentSupply returns the current circulating supply
func (k Keeper) GetCurrentSupply(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyCurrentSupply)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var supply math.Int
	if err := supply.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return supply
}

// SetCurrentSupply updates the current supply
func (k Keeper) SetCurrentSupply(ctx context.Context, supply math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := supply.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.KeyCurrentSupply, bz)
}

// GetTotalMinted returns cumulative minted tokens
func (k Keeper) GetTotalMinted(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyTotalMinted)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var minted math.Int
	if err := minted.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return minted
}

// SetTotalMinted updates the total minted counter
func (k Keeper) SetTotalMinted(ctx context.Context, minted math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := minted.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.KeyTotalMinted, bz)
}

// GetTotalBurned returns cumulative burned tokens
func (k Keeper) GetTotalBurned(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyTotalBurned)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var burned math.Int
	if err := burned.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return burned
}

// SetTotalBurned updates the total burned counter
func (k Keeper) SetTotalBurned(ctx context.Context, burned math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := burned.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.KeyTotalBurned, bz)
}

// ValidateSupplyCap checks if minting would exceed the hard cap
func (k Keeper) ValidateSupplyCap(ctx context.Context, mintAmount math.Int) error {
	params := k.GetParams(ctx)
	currentSupply := k.GetCurrentSupply(ctx)

	newSupply := currentSupply.Add(mintAmount)
	if newSupply.GT(params.TotalSupplyCap) {
		return types.ErrSupplyCapExceeded
	}

	return nil
}

// GetNextBurnID returns the next burn record ID
func (k Keeper) GetNextBurnID(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyNextBurnID)
	if err != nil || bz == nil {
		return 1
	}

	// Decode uint64 from bytes (big-endian)
	id := uint64(0)
	for i := 0; i < 8 && i < len(bz); i++ {
		id = (id << 8) | uint64(bz[i])
	}
	return id
}

// IncrementBurnID increments and returns the next burn ID
func (k Keeper) IncrementBurnID(ctx context.Context) uint64 {
	id := k.GetNextBurnID(ctx)
	nextID := id + 1

	// Encode uint64 to bytes (big-endian)
	bz := make([]byte, 8)
	bz[0] = byte(nextID >> 56)
	bz[1] = byte(nextID >> 48)
	bz[2] = byte(nextID >> 40)
	bz[3] = byte(nextID >> 32)
	bz[4] = byte(nextID >> 24)
	bz[5] = byte(nextID >> 16)
	bz[6] = byte(nextID >> 8)
	bz[7] = byte(nextID)

	store := k.storeService.OpenKVStore(ctx)
	_ = store.Set(types.KeyNextBurnID, bz)

	return id
}

// GetTreasuryAddress returns the treasury account address
func (k Keeper) GetTreasuryAddress(ctx context.Context) sdk.AccAddress {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyTreasuryAddress)
	if err != nil || bz == nil {
		// Return module account as fallback
		return k.accountKeeper.GetModuleAddress(types.ModuleName)
	}

	return sdk.AccAddress(bz)
}

// SetTreasuryAddress sets the treasury account address
func (k Keeper) SetTreasuryAddress(ctx context.Context, addr sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.KeyTreasuryAddress, addr.Bytes())
}

// ── Distribution tracking methods ───────────────────────────────────────────

// RecordDistribution increments the cumulative distribution for a category
// and updates the last distribution height.
func (k Keeper) RecordDistribution(ctx context.Context, category string, amount math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Increment cumulative distributed
	key := types.GetDistributedKey(category)
	current := k.getIntFromStore(ctx, key)
	if err := store.Set(key, []byte(current.Add(amount).String())); err != nil {
		return err
	}

	// Update last distribution height
	height := sdkCtx.BlockHeight()
	heightBytes := sdk.Uint64ToBigEndian(uint64(height))
	return store.Set(types.KeyLastDistributionHeight, heightBytes)
}

// GetCumulativeDistributed returns the total distributed amount for a category.
func (k Keeper) GetCumulativeDistributed(ctx context.Context, category string) math.Int {
	return k.getIntFromStore(ctx, types.GetDistributedKey(category))
}

// GetLastDistributionHeight returns the block height of the last distribution.
func (k Keeper) GetLastDistributionHeight(ctx context.Context) int64 {
	return k.getHeightFromStore(ctx, types.KeyLastDistributionHeight)
}

// IncrementBurnCount increments the total burn count.
func (k Keeper) IncrementBurnCount(ctx context.Context) error {
	store := k.storeService.OpenKVStore(ctx)
	current := k.getUint64FromStore(ctx, types.KeyBurnCount)
	return store.Set(types.KeyBurnCount, sdk.Uint64ToBigEndian(current+1))
}

// GetBurnCount returns the total number of burns.
func (k Keeper) GetBurnCount(ctx context.Context) uint64 {
	return k.getUint64FromStore(ctx, types.KeyBurnCount)
}

// RecordIBCReward tracks IBC rewards received.
func (k Keeper) RecordIBCReward(ctx context.Context, amount math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	current := k.getIntFromStore(ctx, types.KeyIBCRewardsReceived)
	if err := store.Set(types.KeyIBCRewardsReceived, []byte(current.Add(amount).String())); err != nil {
		return err
	}

	height := sdkCtx.BlockHeight()
	return store.Set(types.KeyLastRewardHeight, sdk.Uint64ToBigEndian(uint64(height)))
}

// GetIBCRewardsReceived returns total IBC rewards received.
func (k Keeper) GetIBCRewardsReceived(ctx context.Context) math.Int {
	return k.getIntFromStore(ctx, types.KeyIBCRewardsReceived)
}

// GetLastRewardHeight returns the last IBC reward block height.
func (k Keeper) GetLastRewardHeight(ctx context.Context) int64 {
	return k.getHeightFromStore(ctx, types.KeyLastRewardHeight)
}

// SetLastBurnReportHeight records the last burn report height.
func (k Keeper) SetLastBurnReportHeight(ctx context.Context) error {
	store := k.storeService.OpenKVStore(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := sdkCtx.BlockHeight()
	return store.Set(types.KeyLastBurnReportHeight, sdk.Uint64ToBigEndian(uint64(height)))
}

// GetLastBurnReportHeight returns the last burn report block height.
func (k Keeper) GetLastBurnReportHeight(ctx context.Context) int64 {
	return k.getHeightFromStore(ctx, types.KeyLastBurnReportHeight)
}

// RecordTreasuryInflation tracks treasury inflows from inflation.
func (k Keeper) RecordTreasuryInflation(ctx context.Context, amount math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	current := k.getIntFromStore(ctx, types.KeyTreasuryFromInflation)
	return store.Set(types.KeyTreasuryFromInflation, []byte(current.Add(amount).String()))
}

// GetTreasuryFromInflation returns total treasury inflows from inflation.
func (k Keeper) GetTreasuryFromInflation(ctx context.Context) math.Int {
	return k.getIntFromStore(ctx, types.KeyTreasuryFromInflation)
}

// ── Store helpers ────────────────────────────────────────────────────────────

func (k Keeper) getIntFromStore(ctx context.Context, key []byte) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}
	val, ok := math.NewIntFromString(string(bz))
	if !ok {
		return math.ZeroInt()
	}
	return val
}

func (k Keeper) getUint64FromStore(ctx context.Context, key []byte) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(key)
	if err != nil || bz == nil || len(bz) < 8 {
		return 0
	}
	return sdk.BigEndianToUint64(bz)
}

func (k Keeper) getHeightFromStore(ctx context.Context, key []byte) int64 {
	return int64(k.getUint64FromStore(ctx, key))
}
