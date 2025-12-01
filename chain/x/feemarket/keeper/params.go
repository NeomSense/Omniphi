package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/feemarket/types"
)

// GetParams returns the current feemarket parameters
func (k Keeper) GetParams(ctx context.Context) types.FeeMarketParams {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil || len(bz) == 0 {
		return types.DefaultParams()
	}

	var params types.FeeMarketParams
	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// SetParams sets the feemarket parameters
func (k Keeper) SetParams(ctx context.Context, params types.FeeMarketParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&params)
	return store.Set(types.ParamsKey, bz)
}

// GetCurrentBaseFee returns the current EIP-1559 base fee
func (k Keeper) GetCurrentBaseFee(ctx context.Context) math.LegacyDec {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.CurrentBaseFeeKey)
	if err != nil || len(bz) == 0 {
		params := k.GetParams(ctx)
		return params.BaseFeeInitial
	}

	var baseFee math.LegacyDec
	if err := baseFee.Unmarshal(bz); err != nil {
		params := k.GetParams(ctx)
		return params.BaseFeeInitial
	}
	return baseFee
}

// SetCurrentBaseFee sets the current EIP-1559 base fee
func (k Keeper) SetCurrentBaseFee(ctx context.Context, baseFee math.LegacyDec) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := baseFee.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.CurrentBaseFeeKey, bz)
}

// GetTreasuryAddress returns the configured treasury address
func (k Keeper) GetTreasuryAddress(ctx context.Context) sdk.AccAddress {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TreasuryAddressKey)
	if err != nil || len(bz) == 0 {
		return nil
	}
	return sdk.AccAddress(bz)
}

// SetTreasuryAddress sets the treasury address
func (k Keeper) SetTreasuryAddress(ctx context.Context, addr sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.TreasuryAddressKey, addr.Bytes())
}

// GetCumulativeBurned returns the total amount burned since genesis
func (k Keeper) GetCumulativeBurned(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.CumulativeBurnedKey)
	if err != nil || len(bz) == 0 {
		return math.ZeroInt()
	}

	var amount math.Int
	if err := amount.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return amount
}

// IncrementCumulativeBurned adds to the cumulative burned amount
func (k Keeper) IncrementCumulativeBurned(ctx context.Context, amount math.Int) error {
	current := k.GetCumulativeBurned(ctx)
	newTotal := current.Add(amount)

	store := k.storeService.OpenKVStore(ctx)
	bz, err := newTotal.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.CumulativeBurnedKey, bz)
}

// GetCumulativeToTreasury returns the total amount sent to treasury
func (k Keeper) GetCumulativeToTreasury(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.CumulativeToTreasuryKey)
	if err != nil || len(bz) == 0 {
		return math.ZeroInt()
	}

	var amount math.Int
	if err := amount.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return amount
}

// IncrementCumulativeToTreasury adds to the cumulative treasury amount
func (k Keeper) IncrementCumulativeToTreasury(ctx context.Context, amount math.Int) error {
	current := k.GetCumulativeToTreasury(ctx)
	newTotal := current.Add(amount)

	store := k.storeService.OpenKVStore(ctx)
	bz, err := newTotal.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.CumulativeToTreasuryKey, bz)
}

// GetCumulativeToValidators returns the total amount sent to validators
func (k Keeper) GetCumulativeToValidators(ctx context.Context) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.CumulativeToValidatorsKey)
	if err != nil || len(bz) == 0 {
		return math.ZeroInt()
	}

	var amount math.Int
	if err := amount.Unmarshal(bz); err != nil {
		return math.ZeroInt()
	}
	return amount
}

// IncrementCumulativeToValidators adds to the cumulative validator amount
func (k Keeper) IncrementCumulativeToValidators(ctx context.Context, amount math.Int) error {
	current := k.GetCumulativeToValidators(ctx)
	newTotal := current.Add(amount)

	store := k.storeService.OpenKVStore(ctx)
	bz, err := newTotal.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.CumulativeToValidatorsKey, bz)
}

// SetCumulativeBurned sets the cumulative burned amount (for genesis)
func (k Keeper) SetCumulativeBurned(ctx context.Context, amount math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := amount.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.CumulativeBurnedKey, bz)
}

// SetCumulativeToTreasury sets the cumulative treasury amount (for genesis)
func (k Keeper) SetCumulativeToTreasury(ctx context.Context, amount math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := amount.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.CumulativeToTreasuryKey, bz)
}

// SetCumulativeToValidators sets the cumulative validator amount (for genesis)
func (k Keeper) SetCumulativeToValidators(ctx context.Context, amount math.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := amount.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.CumulativeToValidatorsKey, bz)
}

// GetPreviousBlockUtilization returns the utilization from the previous block
func (k Keeper) GetPreviousBlockUtilization(ctx context.Context) math.LegacyDec {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.PreviousBlockUtilizationKey)
	if err != nil || len(bz) == 0 {
		return math.LegacyZeroDec()
	}

	var util math.LegacyDec
	if err := util.Unmarshal(bz); err != nil {
		return math.LegacyZeroDec()
	}
	return util
}

// SetPreviousBlockUtilization sets the utilization from the previous block
func (k Keeper) SetPreviousBlockUtilization(ctx context.Context, util math.LegacyDec) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := util.Marshal()
	if err != nil {
		return err
	}
	return store.Set(types.PreviousBlockUtilizationKey, bz)
}
