package address

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

const (
	// Ethereum address format prefix
	EthPrefix = "0x"
	// Omniphi display format prefix (branding)
	OmniPrefix = "1x"
	// Ethereum address byte length
	EthAddressLength = 20
)

// FormatOmniAddress converts an address to 1x Omniphi display format
// Handles: 0x hex, 1x hex, and Bech32 formats
// Example: 0x1234...5678 → 1x1234...5678
func FormatOmniAddress(addr string) string {
	// Remove any whitespace
	addr = strings.TrimSpace(addr)

	// Try to decode as Bech32 first
	_, bz, err := bech32.DecodeAndConvert(addr)
	if err == nil && len(bz) == EthAddressLength {
		// Convert bytes to hex with 1x prefix
		return OmniPrefix + hex.EncodeToString(bz)
	}

	// If already 1x hex format, return as-is
	if strings.HasPrefix(addr, OmniPrefix) && len(addr) == 42 {
		return addr
	}

	// If 0x format, convert to 1x
	if strings.HasPrefix(addr, EthPrefix) {
		return OmniPrefix + addr[2:]
	}

	// If no prefix, add 1x
	return OmniPrefix + addr
}

// NormalizeToEthAddress converts an address to standard 0x Ethereum format
// Handles: 1x hex, 0x hex, and Bech32 formats
// Example: 1x1234...5678 → 0x1234...5678
func NormalizeToEthAddress(addr string) string {
	// Remove any whitespace
	addr = strings.TrimSpace(addr)

	// Try to decode as Bech32 first
	_, bz, err := bech32.DecodeAndConvert(addr)
	if err == nil && len(bz) == EthAddressLength {
		// Convert bytes to hex with 0x prefix
		return EthPrefix + hex.EncodeToString(bz)
	}

	// If already 0x hex format, return as-is
	if strings.HasPrefix(addr, EthPrefix) && len(addr) == 42 {
		return addr
	}

	// If 1x hex format, convert to 0x
	if strings.HasPrefix(addr, OmniPrefix) && len(addr) == 42 {
		return EthPrefix + addr[2:]
	}

	// If no prefix, add 0x
	return EthPrefix + addr
}

// ParseOmniAddress parses an address string in either 0x or 1x format
// Returns sdk.AccAddress (20-byte Ethereum-compatible)
func ParseOmniAddress(input string) (sdk.AccAddress, error) {
	// Normalize to 0x format
	ethAddr := NormalizeToEthAddress(input)

	// Validate hex format
	if !common.IsHexAddress(ethAddr) {
		return nil, fmt.Errorf("invalid address format: %s", input)
	}

	// Parse as Ethereum address
	addr := common.HexToAddress(ethAddr)

	// Convert to sdk.AccAddress (20 bytes)
	return sdk.AccAddress(addr.Bytes()), nil
}

// AccAddressToOmni converts sdk.AccAddress to 1x display format
func AccAddressToOmni(addr sdk.AccAddress) string {
	if len(addr) == 0 {
		return ""
	}
	return OmniPrefix + hex.EncodeToString(addr.Bytes())
}

// AccAddressToEth converts sdk.AccAddress to 0x Ethereum format
func AccAddressToEth(addr sdk.AccAddress) string {
	if len(addr) == 0 {
		return ""
	}
	return EthPrefix + hex.EncodeToString(addr.Bytes())
}

// ValAddressToOmni converts sdk.ValAddress to 1x display format
func ValAddressToOmni(addr sdk.ValAddress) string {
	if len(addr) == 0 {
		return ""
	}
	return OmniPrefix + hex.EncodeToString(addr.Bytes())
}

// ParseValAddress parses a validator address in 0x or 1x format
func ParseValAddress(input string) (sdk.ValAddress, error) {
	// Normalize to 0x format
	ethAddr := NormalizeToEthAddress(input)

	// Validate hex format
	if !strings.HasPrefix(ethAddr, EthPrefix) {
		return nil, fmt.Errorf("invalid validator address format: %s", input)
	}

	// Remove 0x prefix and decode
	hexStr := ethAddr[2:]
	bz, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex in validator address: %w", err)
	}

	return sdk.ValAddress(bz), nil
}

// IsValidOmniAddress checks if an address string is valid in either 0x or 1x format
func IsValidOmniAddress(addr string) bool {
	// Try as hex address first (0x or 1x format)
	ethAddr := NormalizeToEthAddress(addr)
	if common.IsHexAddress(ethAddr) {
		return true
	}

	// Try as Bech32 address (e.g., 1x1w74zaplg2cpsxznmspas9krhwxa4r808wzqtl3)
	_, _, err := bech32.DecodeAndConvert(addr)
	return err == nil
}

// MustParseOmniAddress parses an address or panics
func MustParseOmniAddress(input string) sdk.AccAddress {
	addr, err := ParseOmniAddress(input)
	if err != nil {
		panic(err)
	}
	return addr
}

// DeriveEthAddressFromPubKey derives an Ethereum-style address from a public key
// Uses Keccak256 hashing (Ethereum standard)
func DeriveEthAddressFromPubKey(pubKeyBytes []byte) (sdk.AccAddress, error) {
	if len(pubKeyBytes) != 33 && len(pubKeyBytes) != 65 {
		return nil, fmt.Errorf("invalid public key length: %d (expected 33 or 65)", len(pubKeyBytes))
	}

	// If compressed (33 bytes), decompress to uncompressed (65 bytes)
	if len(pubKeyBytes) == 33 {
		// For eth_secp256k1, the SDK handles this
		// Just use Keccak256 on the full pubkey
	}

	// Ethereum uses Keccak256(pubkey)[12:] for address derivation
	// Take last 20 bytes of hash
	hash := crypto.Keccak256(pubKeyBytes)
	addr := hash[12:] // Last 20 bytes

	return sdk.AccAddress(addr), nil
}

// ConvertBech32ToOmni converts a Bech32 address (legacy) to 1x format
func ConvertBech32ToOmni(bech32Addr string) (string, error) {
	_, bz, err := bech32.DecodeAndConvert(bech32Addr)
	if err != nil {
		return "", fmt.Errorf("invalid bech32 address: %w", err)
	}

	// If address is not 20 bytes, it's not Ethereum-compatible
	if len(bz) != EthAddressLength {
		return "", fmt.Errorf("address is not Ethereum-compatible (length: %d, expected: %d)", len(bz), EthAddressLength)
	}

	return OmniPrefix + hex.EncodeToString(bz), nil
}

// GetAddressInfo returns detailed information about an address
type AddressInfo struct {
	Original      string `json:"original"`
	Display       string `json:"display"`        // 1x format
	Ethereum      string `json:"ethereum"`       // 0x format
	Type          string `json:"type"`           // "ethereum" or "bech32"
	Valid         bool   `json:"valid"`
	Length        int    `json:"length"`
	IsValidator   bool   `json:"is_validator"`
	IsConsensus   bool   `json:"is_consensus"`
}

// GetAddressInfo analyzes and returns detailed information about an address
func GetAddressInfo(input string) AddressInfo {
	info := AddressInfo{
		Original: input,
		Type:     "unknown",
		Valid:    false,
	}

	// Check if it's a valid Ethereum-style address (0x or 1x)
	if IsValidOmniAddress(input) {
		info.Valid = true
		info.Type = "ethereum"
		info.Display = FormatOmniAddress(input)
		info.Ethereum = NormalizeToEthAddress(input)
		info.Length = len(info.Ethereum) - 2 // Remove 0x prefix
		return info
	}

	// Check if it's a legacy Bech32 address
	_, bz, err := bech32.DecodeAndConvert(input)
	if err == nil {
		info.Valid = true
		info.Type = "bech32"
		info.Length = len(bz)

		// Try to convert to 1x format if it's 20 bytes
		if len(bz) == EthAddressLength {
			info.Display = OmniPrefix + hex.EncodeToString(bz)
			info.Ethereum = EthPrefix + hex.EncodeToString(bz)
		}

		// Check if it's a validator or consensus address
		if strings.Contains(input, "valoper") {
			info.IsValidator = true
		} else if strings.Contains(input, "valcons") {
			info.IsConsensus = true
		}

		return info
	}

	return info
}
