package types_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"pos/x/tokenomics/types"
)

func TestTeamVestingConfig(t *testing.T) {
	totalAmount := math.NewInt(200_000_000_000_000) // 200M OMNI
	config := types.TeamVestingConfig(totalAmount)

	require.Equal(t, "Team", config.Name)
	require.Equal(t, totalAmount, config.TotalAmount)
	require.Equal(t, int64(12), config.CliffMonths, "Team vesting should have 1-year (12-month) cliff")
	require.Equal(t, int64(48), config.VestMonths, "Team vesting should have 4-year (48-month) linear unlock")

	err := types.ValidateVestingConfig(config)
	require.NoError(t, err, "Team vesting config should be valid")
}

func TestAdvisorVestingConfig(t *testing.T) {
	totalAmount := math.NewInt(30_000_000_000_000) // 30M OMNI
	config := types.AdvisorVestingConfig(totalAmount)

	require.Equal(t, "Advisor", config.Name)
	require.Equal(t, totalAmount, config.TotalAmount)
	require.Equal(t, int64(6), config.CliffMonths, "Advisor vesting should have 6-month cliff")
	require.Equal(t, int64(24), config.VestMonths, "Advisor vesting should have 2-year (24-month) linear unlock")

	err := types.ValidateVestingConfig(config)
	require.NoError(t, err, "Advisor vesting config should be valid")
}

func TestCalculateVestingPeriod(t *testing.T) {
	genesisTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	// Test team vesting: 1-year cliff + 4-year vest
	startTime, endTime := types.CalculateVestingPeriod(genesisTime, 12, 48)

	expectedStart := genesisTime.AddDate(0, 12, 0) // June 1, 2026
	expectedEnd := expectedStart.AddDate(0, 48, 0) // June 1, 2030

	require.Equal(t, expectedStart.Unix(), startTime)
	require.Equal(t, expectedEnd.Unix(), endTime)
}

func TestGetVestedAmount(t *testing.T) {
	originalVesting := math.NewInt(200_000_000_000_000) // 200M OMNI

	genesisTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	startTime := genesisTime.AddDate(0, 12, 0).Unix() // After 1-year cliff
	endTime := genesisTime.AddDate(0, 60, 0).Unix()   // After full 5 years

	tests := []struct {
		name           string
		currentTime    time.Time
		expectedVested math.Int
	}{
		{
			name:           "Before cliff (genesis)",
			currentTime:    genesisTime,
			expectedVested: math.ZeroInt(),
		},
		{
			name:           "Before cliff (6 months)",
			currentTime:    genesisTime.AddDate(0, 6, 0),
			expectedVested: math.ZeroInt(),
		},
		{
			name:           "At cliff start (12 months)",
			currentTime:    genesisTime.AddDate(0, 12, 0),
			expectedVested: math.ZeroInt(), // Linear vesting starts here
		},
		{
			name:           "25% through vesting (24 months total = 12 cliff + 12 vest)",
			currentTime:    genesisTime.AddDate(0, 24, 0),
			expectedVested: originalVesting.QuoRaw(4), // 25% vested
		},
		{
			name:           "50% through vesting (36 months total = 12 cliff + 24 vest)",
			currentTime:    genesisTime.AddDate(0, 36, 0),
			expectedVested: originalVesting.QuoRaw(2), // 50% vested
		},
		{
			name:           "75% through vesting (48 months total = 12 cliff + 36 vest)",
			currentTime:    genesisTime.AddDate(0, 48, 0),
			expectedVested: originalVesting.MulRaw(3).QuoRaw(4), // 75% vested
		},
		{
			name:           "100% vested (60 months = 12 cliff + 48 vest)",
			currentTime:    genesisTime.AddDate(0, 60, 0),
			expectedVested: originalVesting,
		},
		{
			name:           "After vesting complete",
			currentTime:    genesisTime.AddDate(0, 72, 0),
			expectedVested: originalVesting,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vested := types.GetVestedAmount(
				originalVesting,
				startTime,
				endTime,
				tc.currentTime.Unix(),
			)

			// Use approximate comparison due to month day variations
			// Allow 0.1% tolerance
			tolerance := tc.expectedVested.QuoRaw(1000) // 0.1%
			diff := vested.Sub(tc.expectedVested).Abs()

			require.True(t, diff.LTE(tolerance),
				"Vested amount %s differs from expected %s by more than tolerance %s at %s",
				vested, tc.expectedVested, tolerance, tc.currentTime)
		})
	}
}

func TestGetVestingAmount(t *testing.T) {
	originalVesting := math.NewInt(200_000_000_000_000) // 200M OMNI

	genesisTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	startTime := genesisTime.AddDate(0, 12, 0).Unix()
	endTime := genesisTime.AddDate(0, 60, 0).Unix()

	// At 50% vesting progress
	currentTime := genesisTime.AddDate(0, 36, 0).Unix() // 12 cliff + 24 vest (50% of 48)

	vested := types.GetVestedAmount(originalVesting, startTime, endTime, currentTime)
	vesting := types.GetVestingAmount(originalVesting, startTime, endTime, currentTime)

	// Vested + Vesting should equal Original
	require.Equal(t, originalVesting, vested.Add(vesting), "Vested + Vesting must equal Original")

	// At ~50%, both should be approximately equal
	// Allow 0.1% tolerance due to month day variations
	tolerance := originalVesting.QuoRaw(1000) // 0.1%
	diff := vested.Sub(vesting).Abs()
	require.True(t, diff.LTE(tolerance),
		"At ~50% progress, vested (%s) and vesting (%s) amounts should be approximately equal", vested, vesting)
}

func TestValidateVestingConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    types.VestingConfig
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid config",
			config: types.VestingConfig{
				Name:        "Test",
				TotalAmount: math.NewInt(1000000),
				CliffMonths: 12,
				VestMonths:  48,
			},
			expectErr: false,
		},
		{
			name: "Zero amount",
			config: types.VestingConfig{
				Name:        "Test",
				TotalAmount: math.ZeroInt(),
				CliffMonths: 12,
				VestMonths:  48,
			},
			expectErr: true,
			errMsg:    "total amount must be positive",
		},
		{
			name: "Negative cliff",
			config: types.VestingConfig{
				Name:        "Test",
				TotalAmount: math.NewInt(1000000),
				CliffMonths: -1,
				VestMonths:  48,
			},
			expectErr: true,
			errMsg:    "cliff months cannot be negative",
		},
		{
			name: "Zero vest period",
			config: types.VestingConfig{
				Name:        "Test",
				TotalAmount: math.NewInt(1000000),
				CliffMonths: 12,
				VestMonths:  0,
			},
			expectErr: true,
			errMsg:    "vest months must be positive",
		},
		{
			name: "No cliff is valid",
			config: types.VestingConfig{
				Name:        "Test",
				TotalAmount: math.NewInt(1000000),
				CliffMonths: 0,
				VestMonths:  24,
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateVestingConfig(tc.config)
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateContinuousVestingAccount(t *testing.T) {
	addr := sdk.AccAddress([]byte("test_address"))
	baseAccount := &authtypes.BaseAccount{
		Address:       addr.String(),
		AccountNumber: 1,
		Sequence:      0,
	}

	originalVesting := sdk.NewCoins(sdk.NewCoin("uomni", math.NewInt(200_000_000_000_000)))
	startTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Unix()
	endTime := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC).Unix()

	vestingAccount := types.CreateContinuousVestingAccount(
		baseAccount,
		originalVesting,
		startTime,
		endTime,
	)

	require.NotNil(t, vestingAccount)
	require.Equal(t, addr.String(), vestingAccount.GetAddress().String())
	require.Equal(t, originalVesting, vestingAccount.GetOriginalVesting())
	require.Equal(t, startTime, vestingAccount.StartTime)
	require.Equal(t, endTime, vestingAccount.EndTime)
	require.True(t, vestingAccount.GetDelegatedFree().IsZero())
	require.True(t, vestingAccount.GetDelegatedVesting().IsZero())
}
