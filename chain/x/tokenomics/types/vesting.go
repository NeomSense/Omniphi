package types

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

// VestingConfig defines a vesting schedule configuration for helper functions
type VestingConfig struct {
	Name        string
	TotalAmount math.Int
	CliffMonths int64
	VestMonths  int64
}

// CreateContinuousVestingAccount creates a continuous vesting account
func CreateContinuousVestingAccount(
	baseAccount *authtypes.BaseAccount,
	originalVesting sdk.Coins,
	startTime, endTime int64,
) *vestingtypes.ContinuousVestingAccount {
	baseVestingAccount := &vestingtypes.BaseVestingAccount{
		BaseAccount:      baseAccount,
		OriginalVesting:  originalVesting,
		DelegatedFree:    sdk.NewCoins(),
		DelegatedVesting: sdk.NewCoins(),
		EndTime:          endTime,
	}

	return &vestingtypes.ContinuousVestingAccount{
		BaseVestingAccount: baseVestingAccount,
		StartTime:          startTime,
	}
}

// CalculateVestingPeriod calculates vesting start/end times
func CalculateVestingPeriod(genesisTime time.Time, cliffMonths, vestMonths int64) (int64, int64) {
	cliffEnd := genesisTime.AddDate(0, int(cliffMonths), 0)
	vestEnd := cliffEnd.AddDate(0, int(vestMonths), 0)
	return cliffEnd.Unix(), vestEnd.Unix()
}

// TeamVestingConfig returns the team vesting configuration
// 1-year cliff, 4-year linear unlock
func TeamVestingConfig(totalAmount math.Int) VestingConfig {
	return VestingConfig{
		Name:        "Team",
		TotalAmount: totalAmount,
		CliffMonths: 12, // 1 year
		VestMonths:  48, // 4 years
	}
}

// AdvisorVestingConfig returns the advisor vesting configuration
// 6-month cliff, 2-year linear unlock
func AdvisorVestingConfig(totalAmount math.Int) VestingConfig {
	return VestingConfig{
		Name:        "Advisor",
		TotalAmount: totalAmount,
		CliffMonths: 6,  // 6 months
		VestMonths:  24, // 2 years
	}
}

// ValidateVestingConfig validates a vesting configuration
func ValidateVestingConfig(config VestingConfig) error {
	if config.TotalAmount.IsNil() || config.TotalAmount.LTE(math.ZeroInt()) {
		return fmt.Errorf("total amount must be positive")
	}
	if config.CliffMonths < 0 {
		return fmt.Errorf("cliff months cannot be negative")
	}
	if config.VestMonths <= 0 {
		return fmt.Errorf("vest months must be positive")
	}
	return nil
}

// GetVestedAmount calculates the vested amount at a given time
func GetVestedAmount(
	originalVesting math.Int,
	startTime, endTime, currentTime int64,
) math.Int {
	// Before cliff/start - nothing vested
	if currentTime < startTime {
		return math.ZeroInt()
	}

	// After vesting period - fully vested
	if currentTime >= endTime {
		return originalVesting
	}

	// During vesting - linear proportion
	vestingPeriod := endTime - startTime
	elapsedTime := currentTime - startTime

	// vested = original * (elapsed / total)
	vested := originalVesting.Mul(math.NewInt(elapsedTime)).Quo(math.NewInt(vestingPeriod))
	return vested
}

// GetVestingAmount calculates the remaining vesting amount at a given time
func GetVestingAmount(
	originalVesting math.Int,
	startTime, endTime, currentTime int64,
) math.Int {
	vested := GetVestedAmount(originalVesting, startTime, endTime, currentTime)
	return originalVesting.Sub(vested)
}
