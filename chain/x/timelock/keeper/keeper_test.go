package keeper_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/timelock/types"
)

// TimelockTestSuite tests the timelock keeper functionality
type TimelockTestSuite struct {
	suite.Suite
}

func TestTimelockTestSuite(t *testing.T) {
	suite.Run(t, new(TimelockTestSuite))
}

// TestParams tests parameter validation
func (s *TimelockTestSuite) TestParams() {
	testCases := []struct {
		name        string
		params      types.Params
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default params",
			params:      types.DefaultParams(),
			expectError: false,
		},
		{
			name: "min_delay below absolute minimum",
			params: types.Params{
				MinDelaySeconds:       1800, // 30 minutes - below 3600 (1 hour)
				MaxDelaySeconds:       14 * 24 * 3600,
				GracePeriodSeconds:    7 * 24 * 3600,
				EmergencyDelaySeconds: 3600,
				Guardian:              "",
			},
			expectError: true,
			errorMsg:    "minimum delay is below absolute minimum",
		},
		{
			name: "max_delay above absolute maximum",
			params: types.Params{
				MinDelaySeconds:       24 * 3600,
				MaxDelaySeconds:       60 * 24 * 3600, // 60 days, above 30 day limit
				GracePeriodSeconds:    7 * 24 * 3600,
				EmergencyDelaySeconds: 3600,
				Guardian:              "",
			},
			expectError: true,
			errorMsg:    "maximum delay exceeds limit",
		},
		{
			name: "min_delay greater than max_delay",
			params: types.Params{
				MinDelaySeconds:       48 * 3600,
				MaxDelaySeconds:       24 * 3600, // Less than min_delay
				GracePeriodSeconds:    7 * 24 * 3600,
				EmergencyDelaySeconds: 3600,
				Guardian:              "",
			},
			expectError: true,
			errorMsg:    "min_delay",
		},
		{
			name: "grace_period below minimum",
			params: types.Params{
				MinDelaySeconds:       24 * 3600,
				MaxDelaySeconds:       14 * 24 * 3600,
				GracePeriodSeconds:    1800, // 30 minutes - below 3600 (1 hour)
				EmergencyDelaySeconds: 3600,
				Guardian:              "",
			},
			expectError: true,
			errorMsg:    "grace period",
		},
		{
			name: "emergency_delay below absolute minimum",
			params: types.Params{
				MinDelaySeconds:       24 * 3600,
				MaxDelaySeconds:       14 * 24 * 3600,
				GracePeriodSeconds:    7 * 24 * 3600,
				EmergencyDelaySeconds: 1800, // 30 minutes - below 3600 (1 hour)
				Guardian:              "",
			},
			expectError: true,
			errorMsg:    "emergency delay must be at least",
		},
		{
			name: "emergency_delay equal to min_delay",
			params: types.Params{
				MinDelaySeconds:       24 * 3600,
				MaxDelaySeconds:       14 * 24 * 3600,
				GracePeriodSeconds:    7 * 24 * 3600,
				EmergencyDelaySeconds: 24 * 3600, // Same as min_delay
				Guardian:              "",
			},
			expectError: true,
			errorMsg:    "emergency_delay",
		},
		{
			name: "valid custom params",
			params: types.Params{
				MinDelaySeconds:       48 * 3600,
				MaxDelaySeconds:       7 * 24 * 3600,
				GracePeriodSeconds:    3 * 24 * 3600,
				EmergencyDelaySeconds: 2 * 3600,
				Guardian:              "omni1...",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			err := tc.params.Validate()
			if tc.expectError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errorMsg)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

// TestCancelReasonValidation tests cancel reason validation
func (s *TimelockTestSuite) TestCancelReasonValidation() {
	testCases := []struct {
		name        string
		reason      string
		expectError bool
	}{
		{
			name:        "too short",
			reason:      "short",
			expectError: true,
		},
		{
			name:        "minimum valid length",
			reason:      "Valid reas", // 10 characters
			expectError: false,
		},
		{
			name:        "too long",
			reason:      string(make([]byte, 501)),
			expectError: true,
		},
		{
			name:        "valid reason",
			reason:      "Security vulnerability discovered in the proposed parameter change",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			err := types.ValidateCancelReason(tc.reason)
			if tc.expectError {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

// TestJustificationValidation tests emergency justification validation
func (s *TimelockTestSuite) TestJustificationValidation() {
	testCases := []struct {
		name          string
		justification string
		expectError   bool
	}{
		{
			name:          "too short",
			justification: "Too short reason",
			expectError:   true,
		},
		{
			name:          "minimum valid length",
			justification: "This is a valid just", // 20 characters
			expectError:   false,
		},
		{
			name:          "too long",
			justification: string(make([]byte, 1001)),
			expectError:   true,
		},
		{
			name:          "valid justification",
			justification: "Critical security patch required immediately to prevent potential exploit of vulnerability CVE-2026-0001",
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			err := types.ValidateJustification(tc.justification)
			if tc.expectError {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

// TestOperationStatus tests operation status helpers
func (s *TimelockTestSuite) TestOperationStatus() {
	s.Run("terminal status check", func() {
		s.False(types.OperationStatusQueued.IsTerminal())
		s.True(types.OperationStatusExecuted.IsTerminal())
		s.True(types.OperationStatusCancelled.IsTerminal())
		s.True(types.OperationStatusExpired.IsTerminal())
		s.True(types.OperationStatusFailed.IsTerminal())
	})
}

// TestSecurityConstants tests that security constants are set correctly
func TestSecurityConstants(t *testing.T) {
	require.Equal(t, uint64(3600), types.AbsoluteMinDelaySeconds,
		"Absolute minimum delay should be 3600 seconds (1 hour)")

	require.Equal(t, uint64(30*24*3600), types.AbsoluteMaxDelaySeconds,
		"Absolute maximum delay should be 2592000 seconds (30 days)")

	require.Equal(t, uint64(3600), types.AbsoluteMinGracePeriodSeconds,
		"Absolute minimum grace period should be 3600 seconds (1 hour)")

	require.Equal(t, uint64(24*3600), types.DefaultMinDelaySeconds,
		"Default minimum delay should be 86400 seconds (24 hours)")

	require.Equal(t, uint64(14*24*3600), types.DefaultMaxDelaySeconds,
		"Default maximum delay should be 1209600 seconds (14 days)")

	require.Equal(t, uint64(7*24*3600), types.DefaultGracePeriodSeconds,
		"Default grace period should be 604800 seconds (7 days)")

	require.Equal(t, uint64(3600), types.DefaultEmergencyDelaySeconds,
		"Default emergency delay should be 3600 seconds (1 hour)")

	// Also test the time.Duration constants for backward compatibility
	require.Equal(t, 1*time.Hour, types.AbsoluteMinDelay,
		"Absolute minimum delay should be 1 hour")

	require.Equal(t, 30*24*time.Hour, types.AbsoluteMaxDelay,
		"Absolute maximum delay should be 30 days")

	require.Equal(t, 24*time.Hour, types.DefaultMinDelay,
		"Default minimum delay should be 24 hours")
}

// TestGenesisValidation tests genesis state validation
func TestGenesisValidation(t *testing.T) {
	testCases := []struct {
		name        string
		genesis     *types.GenesisState
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default genesis",
			genesis:     types.DefaultGenesisState(),
			expectError: false,
		},
		{
			name: "invalid params",
			genesis: &types.GenesisState{
				Params: types.Params{
					MinDelaySeconds:       1800, // Invalid - below absolute minimum
					MaxDelaySeconds:       14 * 24 * 3600,
					GracePeriodSeconds:    7 * 24 * 3600,
					EmergencyDelaySeconds: 3600,
				},
				Operations:      []types.QueuedOperation{},
				NextOperationId: 1,
			},
			expectError: true,
			errorMsg:    "invalid params",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.genesis.Validate()
			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestMsgValidateBasic tests message validation
func TestMsgValidateBasic(t *testing.T) {
	// Create a valid test address using SDK's test utilities
	// Use cosmos prefix since that's the default in test context
	validAddr := sdk.AccAddress(make([]byte, 20)).String()

	t.Run("MsgExecuteOperation", func(t *testing.T) {
		// Valid message
		msg := types.MsgExecuteOperation{
			Executor:    validAddr,
			OperationId: 1,
		}
		require.NoError(t, msg.ValidateBasic())

		// Invalid executor
		msg.Executor = "invalid"
		require.Error(t, msg.ValidateBasic())

		// Zero operation ID
		msg.Executor = validAddr
		msg.OperationId = 0
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("MsgCancelOperation", func(t *testing.T) {
		// Valid message
		msg := types.MsgCancelOperation{
			Authority:   validAddr,
			OperationId: 1,
			Reason:      "Security concern discovered during review",
		}
		require.NoError(t, msg.ValidateBasic())

		// Invalid authority
		msg.Authority = "invalid"
		require.Error(t, msg.ValidateBasic())

		// Reason too short
		msg.Authority = validAddr
		msg.Reason = "short"
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("MsgEmergencyExecute", func(t *testing.T) {
		// Valid message
		msg := types.MsgEmergencyExecute{
			Authority:     validAddr,
			OperationId:   1,
			Justification: "Critical security vulnerability requires immediate patching",
		}
		require.NoError(t, msg.ValidateBasic())

		// Justification too short
		msg.Justification = "Too short"
		require.Error(t, msg.ValidateBasic())
	})

	t.Run("MsgUpdateGuardian", func(t *testing.T) {
		// Valid message
		msg := types.MsgUpdateGuardian{
			Authority:   validAddr,
			NewGuardian: validAddr,
		}
		require.NoError(t, msg.ValidateBasic())

		// Invalid new guardian
		msg.NewGuardian = "invalid"
		require.Error(t, msg.ValidateBasic())
	})
}
