package address

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestFormatOmniAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "0x to 1x conversion",
			input:    "0x1234567890abcdef1234567890abcdef12345678",
			expected: "1x1234567890abcdef1234567890abcdef12345678",
		},
		{
			name:     "Already 1x format",
			input:    "1x1234567890abcdef1234567890abcdef12345678",
			expected: "1x1234567890abcdef1234567890abcdef12345678",
		},
		{
			name:     "No prefix - add 1x",
			input:    "1234567890abcdef1234567890abcdef12345678",
			expected: "1x1234567890abcdef1234567890abcdef12345678",
		},
		{
			name:     "Mixed case",
			input:    "0x1234567890AbCdEf1234567890AbCdEf12345678",
			expected: "1x1234567890AbCdEf1234567890AbCdEf12345678",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatOmniAddress(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeToEthAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "1x to 0x conversion",
			input:    "1x1234567890abcdef1234567890abcdef12345678",
			expected: "0x1234567890abcdef1234567890abcdef12345678",
		},
		{
			name:     "Already 0x format",
			input:    "0x1234567890abcdef1234567890abcdef12345678",
			expected: "0x1234567890abcdef1234567890abcdef12345678",
		},
		{
			name:     "No prefix - add 0x",
			input:    "1234567890abcdef1234567890abcdef12345678",
			expected: "0x1234567890abcdef1234567890abcdef12345678",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeToEthAddress(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestParseOmniAddress(t *testing.T) {
	validEthAddr := "0x1234567890abcdef1234567890abcdef12345678"
	expectedBytes := common.HexToAddress(validEthAddr).Bytes()

	tests := []struct {
		name        string
		input       string
		expectError bool
		checkBytes  bool
	}{
		{
			name:        "Valid 0x address",
			input:       validEthAddr,
			expectError: false,
			checkBytes:  true,
		},
		{
			name:        "Valid 1x address",
			input:       "1x1234567890abcdef1234567890abcdef12345678",
			expectError: false,
			checkBytes:  true,
		},
		{
			name:        "Invalid hex",
			input:       "1xGGGG567890abcdef1234567890abcdef12345678",
			expectError: true,
		},
		{
			name:        "Too short",
			input:       "1x1234",
			expectError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := ParseOmniAddress(tc.input)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.checkBytes {
					require.Equal(t, expectedBytes, addr.Bytes())
				}
			}
		})
	}
}

func TestAccAddressToOmni(t *testing.T) {
	ethAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	sdkAddr := sdk.AccAddress(ethAddr.Bytes())

	result := AccAddressToOmni(sdkAddr)
	expected := "1x1234567890abcdef1234567890abcdef12345678"

	require.Equal(t, expected, result)
}

func TestAccAddressToEth(t *testing.T) {
	ethAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	sdkAddr := sdk.AccAddress(ethAddr.Bytes())

	result := AccAddressToEth(sdkAddr)
	expected := "0x1234567890abcdef1234567890abcdef12345678"

	require.Equal(t, expected, result)
}

func TestIsValidOmniAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid 0x address",
			input:    "0x1234567890abcdef1234567890abcdef12345678",
			expected: true,
		},
		{
			name:     "Valid 1x address",
			input:    "1x1234567890abcdef1234567890abcdef12345678",
			expected: true,
		},
		{
			name:     "Invalid hex",
			input:    "1xGGGG567890abcdef1234567890abcdef12345678",
			expected: false,
		},
		{
			name:     "Too short",
			input:    "1x1234",
			expected: false,
		},
		{
			name:     "Empty",
			input:    "",
			expected: false,
		},
		{
			name:     "Random string",
			input:    "hello world",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidOmniAddress(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	// Start with 0x address
	original := "0x1234567890abcdef1234567890abcdef12345678"

	// Convert to 1x
	omni := FormatOmniAddress(original)
	require.Equal(t, "1x1234567890abcdef1234567890abcdef12345678", omni)

	// Convert back to 0x
	eth := NormalizeToEthAddress(omni)
	require.Equal(t, original, eth)

	// Parse and convert to sdk.AccAddress
	addr, err := ParseOmniAddress(omni)
	require.NoError(t, err)

	// Convert sdk.AccAddress back to 1x
	result := AccAddressToOmni(addr)
	require.Equal(t, omni, result)
}

func TestValAddressToOmni(t *testing.T) {
	// Create a validator address (20 bytes)
	valBytes, _ := hex.DecodeString("1234567890abcdef1234567890abcdef12345678")
	valAddr := sdk.ValAddress(valBytes)

	result := ValAddressToOmni(valAddr)
	expected := "1x1234567890abcdef1234567890abcdef12345678"

	require.Equal(t, expected, result)
}

func TestParseValAddress(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Valid 0x validator address",
			input:       "0x1234567890abcdef1234567890abcdef12345678",
			expectError: false,
		},
		{
			name:        "Valid 1x validator address",
			input:       "1x1234567890abcdef1234567890abcdef12345678",
			expectError: false,
		},
		{
			name:        "Invalid hex",
			input:       "1xGGGG",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := ParseValAddress(tc.input)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, addr)
			}
		})
	}
}

func TestGetAddressInfo(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedValid bool
		expectedType  string
	}{
		{
			name:          "Valid 0x address",
			input:         "0x1234567890abcdef1234567890abcdef12345678",
			expectedValid: true,
			expectedType:  "ethereum",
		},
		{
			name:          "Valid 1x address",
			input:         "1x1234567890abcdef1234567890abcdef12345678",
			expectedValid: true,
			expectedType:  "ethereum",
		},
		{
			name:          "Invalid address",
			input:         "invalid",
			expectedValid: false,
			expectedType:  "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := GetAddressInfo(tc.input)

			require.Equal(t, tc.expectedValid, info.Valid)
			require.Equal(t, tc.expectedType, info.Type)

			if tc.expectedValid && tc.expectedType == "ethereum" {
				require.NotEmpty(t, info.Display)
				require.NotEmpty(t, info.Ethereum)
				require.True(t, len(info.Display) > 0)
				require.True(t, len(info.Ethereum) > 0)
			}
		})
	}
}

func TestMustParseOmniAddress(t *testing.T) {
	// Valid address should not panic
	validAddr := "1x1234567890abcdef1234567890abcdef12345678"
	require.NotPanics(t, func() {
		addr := MustParseOmniAddress(validAddr)
		require.NotNil(t, addr)
	})

	// Invalid address should panic
	invalidAddr := "invalid"
	require.Panics(t, func() {
		MustParseOmniAddress(invalidAddr)
	})
}

func TestEmptyAddressHandling(t *testing.T) {
	// Test empty sdk.AccAddress
	var emptyAddr sdk.AccAddress

	omni := AccAddressToOmni(emptyAddr)
	require.Equal(t, "", omni)

	eth := AccAddressToEth(emptyAddr)
	require.Equal(t, "", eth)

	// Test empty ValAddress
	var emptyVal sdk.ValAddress
	valOmni := ValAddressToOmni(emptyVal)
	require.Equal(t, "", valOmni)
}

func TestAddressLength(t *testing.T) {
	// Ethereum addresses should be exactly 20 bytes
	addr := "0x1234567890abcdef1234567890abcdef12345678"
	parsed, err := ParseOmniAddress(addr)
	require.NoError(t, err)
	require.Equal(t, EthAddressLength, len(parsed.Bytes()))
}

func TestCaseInsensitivity(t *testing.T) {
	// Ethereum addresses are case-insensitive
	lower := "0x1234567890abcdef1234567890abcdef12345678"
	upper := "0x1234567890ABCDEF1234567890ABCDEF12345678"
	mixed := "0x1234567890AbCdEf1234567890aBcDeF12345678"

	addrLower, err1 := ParseOmniAddress(lower)
	addrUpper, err2 := ParseOmniAddress(upper)
	addrMixed, err3 := ParseOmniAddress(mixed)

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	// All should produce the same bytes
	require.Equal(t, addrLower.Bytes(), addrUpper.Bytes())
	require.Equal(t, addrLower.Bytes(), addrMixed.Bytes())
}
