package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// Validate validates the genesis state
func (gs GenesisState) Validate() error {
	// Validate params
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Validate supply state
	if gs.SupplyState.CurrentTotalSupply.IsNegative() {
		return fmt.Errorf("current supply cannot be negative")
	}

	if gs.SupplyState.TotalMinted.IsNegative() {
		return fmt.Errorf("total minted cannot be negative")
	}

	if gs.SupplyState.TotalBurned.IsNegative() {
		return fmt.Errorf("total burned cannot be negative")
	}

	// Verify conservation law
	expected := gs.SupplyState.TotalMinted.Sub(gs.SupplyState.TotalBurned)
	if !gs.SupplyState.CurrentTotalSupply.Equal(expected) {
		return fmt.Errorf(
			"supply accounting error: current (%s) != minted (%s) - burned (%s)",
			gs.SupplyState.CurrentTotalSupply.String(),
			gs.SupplyState.TotalMinted.String(),
			gs.SupplyState.TotalBurned.String(),
		)
	}

	// Validate allocations
	seenAddresses := make(map[string]bool)
	totalAllocated := math.ZeroInt()

	for i, alloc := range gs.Allocations {
		if alloc.Address == "" {
			return fmt.Errorf("allocation %d has empty address", i)
		}

		if seenAddresses[alloc.Address] {
			return fmt.Errorf("duplicate allocation address: %s", alloc.Address)
		}
		seenAddresses[alloc.Address] = true

		if alloc.Amount.IsNegative() || alloc.Amount.IsZero() {
			return fmt.Errorf("allocation %d (%s) has invalid amount: %s", i, alloc.Address, alloc.Amount.String())
		}

		totalAllocated = totalAllocated.Add(alloc.Amount)

		// Validate vesting schedule if present
		if alloc.IsVested && alloc.VestingSchedule != nil {
			if alloc.VestingSchedule.VestingDuration == 0 {
				return fmt.Errorf("allocation %d has zero vesting duration", i)
			}
			if alloc.VestingSchedule.CliffDuration > alloc.VestingSchedule.VestingDuration {
				return fmt.Errorf("allocation %d has invalid vesting schedule: cliff > vesting duration", i)
			}
		}
	}

	// Verify allocations sum to genesis supply
	if !totalAllocated.Equal(gs.SupplyState.CurrentTotalSupply) {
		return fmt.Errorf(
			"allocations (%s) do not equal genesis supply (%s)",
			totalAllocated.String(),
			gs.SupplyState.CurrentTotalSupply.String(),
		)
	}

	// Validate treasury state
	if gs.TreasuryState.TreasuryAddress == "" {
		return fmt.Errorf("treasury address cannot be empty")
	}

	if gs.TreasuryState.InitialBalance.IsNegative() {
		return fmt.Errorf("treasury initial balance cannot be negative")
	}

	return nil
}

// ValidateGenesisState is an alias for backwards compatibility
// The actual Validate method is on the GenesisState type above
