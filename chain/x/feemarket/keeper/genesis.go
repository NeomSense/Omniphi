package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/feemarket/types"
)

// InitGenesis initializes the module's state from a provided genesis state
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	k.Logger(ctx).Info("initializing feemarket module from genesis")

	// If genesis is empty (zero values), use defaults
	// This handles the case where `posd init` creates empty genesis
	params := genState.Params
	if params.MinGasPrice.IsNil() || params.MinGasPrice.IsZero() {
		k.Logger(ctx).Warn("genesis params are empty or invalid, using defaults")
		defaults := types.DefaultGenesis()
		params = defaults.Params
		genState.CurrentBaseFee = defaults.CurrentBaseFee
		genState.PreviousBlockUtilization = defaults.PreviousBlockUtilization
		genState.CumulativeBurned = defaults.CumulativeBurned
		genState.CumulativeToTreasury = defaults.CumulativeToTreasury
		genState.CumulativeToValidators = defaults.CumulativeToValidators
	}

	// Set parameters
	if err := k.SetParams(ctx, params); err != nil {
		return err
	}

	// Set current base fee
	if err := k.SetCurrentBaseFee(ctx, genState.CurrentBaseFee); err != nil {
		return err
	}

	// Set previous block utilization
	if err := k.SetPreviousBlockUtilization(ctx, genState.PreviousBlockUtilization); err != nil {
		return err
	}

	// Set treasury address if provided
	if genState.TreasuryAddress != "" {
		treasuryAddr, err := sdk.AccAddressFromBech32(genState.TreasuryAddress)
		if err != nil {
			return types.ErrTreasuryAddressNotSet.Wrapf("invalid treasury address: %s", genState.TreasuryAddress)
		}
		if err := k.SetTreasuryAddress(ctx, treasuryAddr); err != nil {
			return err
		}
	}

	// Set cumulative statistics
	if err := k.SetCumulativeBurned(ctx, genState.CumulativeBurned); err != nil {
		return err
	}

	if err := k.SetCumulativeToTreasury(ctx, genState.CumulativeToTreasury); err != nil {
		return err
	}

	if err := k.SetCumulativeToValidators(ctx, genState.CumulativeToValidators); err != nil {
		return err
	}

	k.Logger(ctx).Info("feemarket module initialized",
		"base_fee", genState.CurrentBaseFee.String(),
		"treasury_address", genState.TreasuryAddress,
	)

	return nil
}

// ExportGenesis exports the module's state to a genesis state
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	k.Logger(ctx).Info("exporting feemarket module genesis state")

	treasuryAddr := k.GetTreasuryAddress(ctx)
	treasuryAddrStr := ""
	if len(treasuryAddr) > 0 {
		treasuryAddrStr = treasuryAddr.String()
	}

	return &types.GenesisState{
		Params:                   k.GetParams(ctx),
		CurrentBaseFee:           k.GetCurrentBaseFee(ctx),
		PreviousBlockUtilization: k.GetPreviousBlockUtilization(ctx),
		TreasuryAddress:          treasuryAddrStr,
		CumulativeBurned:         k.GetCumulativeBurned(ctx),
		CumulativeToTreasury:     k.GetCumulativeToTreasury(ctx),
		CumulativeToValidators:   k.GetCumulativeToValidators(ctx),
	}
}
