package app

import (
	"encoding/hex"
	"fmt"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
)

var setupOnce sync.Once

// SetupEthereumAddressFormat configures the chain to use Ethereum-style addresses (0x...)
// Internal: Uses "omni" Bech32 prefix for SDK compatibility
// Display: Converts to 1x hex format in CLI/UI layer
// Storage: 20-byte Ethereum addresses
func SetupEthereumAddressFormat() {
	setupOnce.Do(func() {
		config := sdk.GetConfig()

		// Set coin type to Ethereum (60) for HD wallet compatibility
		config.SetCoinType(ChainCoinType)

		// Set address verifier to accept ONLY 20-byte Ethereum addresses
		config.SetAddressVerifier(func(bytes []byte) error {
			if len(bytes) != 20 {
				return fmt.Errorf("address must be exactly 20 bytes (Ethereum-compatible), got %d", len(bytes))
			}
			return nil
		})

		// Use "omni" prefix for Bech32 encoding (SDK requirement)
		// CLI/UI will convert these to 1x format for display
		config.SetBech32PrefixForAccount("omni", "omnipub")
		config.SetBech32PrefixForValidator("omnivaloper", "omnivaloperpub")
		config.SetBech32PrefixForConsensusNode("omnivalcons", "omnivalconspub")

		// Seal the config to prevent further changes
		config.Seal()
	})
}

// EthereumAddressFromString parses an address string in 0x or 1x format
// Accepts both 0x (Ethereum standard) and 1x (Omniphi display) formats
// Returns sdk.AccAddress (20-byte Ethereum-compatible)
func EthereumAddressFromString(addressStr string) (sdk.AccAddress, error) {
	// Support both 0x and 1x prefixes
	if len(addressStr) >= 42 { // 0x + 40 hex chars = 42
		prefix := addressStr[:2]
		if prefix == "0x" || prefix == "1x" {
			// Remove prefix and decode hex
			hexStr := addressStr[2:]
			bz, err := hex.DecodeString(hexStr)
			if err != nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid hex string: %v", err)
			}

			// Validate length
			if len(bz) != 20 {
				return nil, sdkerrors.ErrInvalidAddress.Wrapf("address must be exactly 20 bytes, got %d", len(bz))
			}

			return sdk.AccAddress(bz), nil
		}
	}

	// Try parsing as raw Ethereum address using go-ethereum
	if common.IsHexAddress(addressStr) {
		addr := common.HexToAddress(addressStr)
		return sdk.AccAddress(addr.Bytes()), nil
	}

	// Try parsing as legacy Bech32 for backwards compatibility
	addr, err := sdk.AccAddressFromBech32(addressStr)
	if err == nil && len(addr) == 20 {
		return addr, nil
	}

	return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid address format: expected 0x... or 1x... format, got %s", addressStr)
}

// FormatEthereumAddress formats an sdk.AccAddress as a 0x Ethereum address string
// This is the INTERNAL representation used by the chain
func FormatEthereumAddress(addr sdk.AccAddress) string {
	if len(addr) == 0 {
		return ""
	}
	return "0x" + hex.EncodeToString(addr.Bytes())
}

// DisplayOmniAddress formats an sdk.AccAddress as a 1x Omniphi branded address string
// This is the DISPLAY representation shown to users in CLI/UI
func DisplayOmniAddress(addr sdk.AccAddress) string {
	if len(addr) == 0 {
		return ""
	}
	return "1x" + hex.EncodeToString(addr.Bytes())
}

// FormatOmniphiAddress is an alias for DisplayOmniAddress for backwards compatibility
func FormatOmniphiAddress(addr sdk.AccAddress) string {
	return DisplayOmniAddress(addr)
}

// ParseOmniAddress parses a 1x address and returns the underlying sdk.AccAddress
// Accepts: 1x..., 0x..., or legacy Bech32
func ParseOmniAddress(addressStr string) (sdk.AccAddress, error) {
	return EthereumAddressFromString(addressStr)
}

// MustEthereumAddressFromString parses an address or panics
func MustEthereumAddressFromString(addressStr string) sdk.AccAddress {
	addr, err := EthereumAddressFromString(addressStr)
	if err != nil {
		panic(err)
	}
	return addr
}
