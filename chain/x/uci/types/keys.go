package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "uci"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// KVStore key prefixes
var (
	// KeyParams stores the module parameters
	KeyParams = []byte{0x01}

	// KeyPrefixAdapter stores registered DePIN adapters
	// adapter_id -> Adapter (JSON)
	KeyPrefixAdapter = []byte{0x02}

	// KeyNextAdapterID is the auto-incrementing adapter ID counter
	KeyNextAdapterID = []byte{0x03}

	// KeyPrefixContributionMapping maps external DePIN contribution to PoC contribution
	// adapter_id | external_id -> ContributionMapping (JSON)
	KeyPrefixContributionMapping = []byte{0x04}

	// KeyPrefixAdapterStats stores statistics per adapter
	// adapter_id -> AdapterStats (JSON)
	KeyPrefixAdapterStats = []byte{0x05}

	// KeyPrefixOracleAttestation stores oracle-signed attestations for DePIN data
	// adapter_id | batch_id -> OracleAttestation (JSON)
	KeyPrefixOracleAttestation = []byte{0x06}

	// KeyPrefixAdapterByOwner maps owner address to adapter IDs for fast lookups
	// owner_address | adapter_id -> (empty, existence marker)
	KeyPrefixAdapterByOwner = []byte{0x07}
)

// GetAdapterKey returns the store key for an adapter by ID
func GetAdapterKey(adapterID uint64) []byte {
	return append(KeyPrefixAdapter, sdk.Uint64ToBigEndian(adapterID)...)
}

// GetContributionMappingKey returns the store key for a contribution mapping
func GetContributionMappingKey(adapterID uint64, externalID string) []byte {
	key := append(KeyPrefixContributionMapping, sdk.Uint64ToBigEndian(adapterID)...)
	key = append(key, byte('/'))
	return append(key, []byte(externalID)...)
}

// GetContributionMappingPrefixKey returns the prefix key for all contribution mappings of an adapter
func GetContributionMappingPrefixKey(adapterID uint64) []byte {
	key := append(KeyPrefixContributionMapping, sdk.Uint64ToBigEndian(adapterID)...)
	return append(key, byte('/'))
}

// GetAdapterStatsKey returns the store key for an adapter's statistics
func GetAdapterStatsKey(adapterID uint64) []byte {
	return append(KeyPrefixAdapterStats, sdk.Uint64ToBigEndian(adapterID)...)
}

// GetOracleAttestationKey returns the store key for an oracle attestation
func GetOracleAttestationKey(adapterID uint64, batchID string) []byte {
	key := append(KeyPrefixOracleAttestation, sdk.Uint64ToBigEndian(adapterID)...)
	key = append(key, byte('/'))
	return append(key, []byte(batchID)...)
}

// GetAdapterByOwnerKey returns the store key for the owner -> adapter index
func GetAdapterByOwnerKey(owner string, adapterID uint64) []byte {
	key := append(KeyPrefixAdapterByOwner, []byte(owner)...)
	key = append(key, byte('/'))
	return append(key, sdk.Uint64ToBigEndian(adapterID)...)
}

// GetAdapterByOwnerPrefixKey returns the prefix key for all adapters owned by an address
func GetAdapterByOwnerPrefixKey(owner string) []byte {
	key := append(KeyPrefixAdapterByOwner, []byte(owner)...)
	return append(key, byte('/'))
}
