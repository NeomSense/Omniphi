package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                   DefaultParams(),
		CurrentBaseFee:           math.LegacyMustNewDecFromStr("0.05"), // Match BaseFeeInitial
		PreviousBlockUtilization: math.LegacyZeroDec(),
		TreasuryAddress:          "",  // Must be set before genesis
		CumulativeBurned:         math.ZeroInt(),
		CumulativeToTreasury:     math.ZeroInt(),
		CumulativeToValidators:   math.ZeroInt(),
	}
}

// Validate performs basic genesis state validation
func (gs GenesisState) Validate() error {
	// If params are empty/nil, skip validation (will use defaults in InitGenesis)
	// This allows gentx to work with empty genesis
	if gs.Params.MinGasPrice.IsNil() || gs.Params.MinGasPrice.IsZero() {
		// Empty genesis is valid - will be populated with defaults
		return nil
	}

	// Validate params
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Validate current base fee
	if gs.CurrentBaseFee.IsNegative() {
		return fmt.Errorf("current base fee cannot be negative: %s", gs.CurrentBaseFee)
	}

	// Validate previous block utilization (0.0 - 1.0)
	if gs.PreviousBlockUtilization.IsNegative() || gs.PreviousBlockUtilization.GT(math.LegacyOneDec()) {
		return fmt.Errorf("previous block utilization must be between 0 and 1, got: %s", gs.PreviousBlockUtilization)
	}

	// Validate treasury address if set
	if gs.TreasuryAddress != "" {
		if _, err := sdk.AccAddressFromBech32(gs.TreasuryAddress); err != nil {
			return fmt.Errorf("invalid treasury address: %w", err)
		}
	}

	// Validate cumulative amounts are non-negative
	if gs.CumulativeBurned.IsNegative() {
		return fmt.Errorf("cumulative burned cannot be negative: %s", gs.CumulativeBurned)
	}
	if gs.CumulativeToTreasury.IsNegative() {
		return fmt.Errorf("cumulative to treasury cannot be negative: %s", gs.CumulativeToTreasury)
	}
	if gs.CumulativeToValidators.IsNegative() {
		return fmt.Errorf("cumulative to validators cannot be negative: %s", gs.CumulativeToValidators)
	}

	return nil
}

// DefaultGenesisState is an alias for DefaultGenesis for compatibility
func DefaultGenesisState() *GenesisState {
	return DefaultGenesis()
}
