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

// GetPreviousBlockGasUsed returns the gas used in the previous block
func (k Keeper) GetPreviousBlockGasUsed(ctx context.Context) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.PreviousBlockGasUsedKey)
	if err != nil || len(bz) == 0 {
		return 0
	}
	if len(bz) != 8 {
		return 0
	}
	return int64(bz[0]) | int64(bz[1])<<8 | int64(bz[2])<<16 | int64(bz[3])<<24 |
		int64(bz[4])<<32 | int64(bz[5])<<40 | int64(bz[6])<<48 | int64(bz[7])<<56
}

// SetPreviousBlockGasUsed sets the gas used in the previous block
func (k Keeper) SetPreviousBlockGasUsed(ctx context.Context, gasUsed int64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	bz[0] = byte(gasUsed)
	bz[1] = byte(gasUsed >> 8)
	bz[2] = byte(gasUsed >> 16)
	bz[3] = byte(gasUsed >> 24)
	bz[4] = byte(gasUsed >> 32)
	bz[5] = byte(gasUsed >> 40)
	bz[6] = byte(gasUsed >> 48)
	bz[7] = byte(gasUsed >> 56)
	return store.Set(types.PreviousBlockGasUsedKey, bz)
}

// GetMaxBlockGas returns the max block gas limit
func (k Keeper) GetMaxBlockGas(ctx context.Context) int64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.MaxBlockGasKey)
	if err != nil || len(bz) == 0 {
		return 0
	}
	if len(bz) != 8 {
		return 0
	}
	return int64(bz[0]) | int64(bz[1])<<8 | int64(bz[2])<<16 | int64(bz[3])<<24 |
		int64(bz[4])<<32 | int64(bz[5])<<40 | int64(bz[6])<<48 | int64(bz[7])<<56
}

// SetMaxBlockGas sets the max block gas limit
func (k Keeper) SetMaxBlockGas(ctx context.Context, maxGas int64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	bz[0] = byte(maxGas)
	bz[1] = byte(maxGas >> 8)
	bz[2] = byte(maxGas >> 16)
	bz[3] = byte(maxGas >> 24)
	bz[4] = byte(maxGas >> 32)
	bz[5] = byte(maxGas >> 40)
	bz[6] = byte(maxGas >> 48)
	bz[7] = byte(maxGas >> 56)
	return store.Set(types.MaxBlockGasKey, bz)
}
