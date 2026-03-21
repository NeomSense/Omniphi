package types

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	ModuleName = "contracts"
	StoreKey   = ModuleName
	RouterKey  = ModuleName
)

// KVStore key prefixes.
var (
	KeyParams                  = []byte{0x01}
	KeyPrefixSchema            = []byte{0x02} // schema_id (32 bytes) → ContractSchema JSON
	KeyPrefixWasmBytecode      = []byte{0x03} // schema_id (32 bytes) → raw Wasm bytes
	KeyPrefixInstance          = []byte{0x04} // instance_id (uint64) → ContractInstance JSON
	KeyNextInstanceID          = []byte{0x05} // → uint64 auto-increment
	KeyPrefixSchemaByDeployer  = []byte{0x06} // deployer_addr / schema_id → existence marker
	KeyPrefixInstanceBySchema  = []byte{0x07} // schema_id / instance_id → existence marker
)

func GetSchemaKey(schemaID [32]byte) []byte {
	return append(KeyPrefixSchema, schemaID[:]...)
}

func GetWasmKey(schemaID [32]byte) []byte {
	return append(KeyPrefixWasmBytecode, schemaID[:]...)
}

func GetInstanceKey(instanceID uint64) []byte {
	return append(KeyPrefixInstance, sdk.Uint64ToBigEndian(instanceID)...)
}

func GetNextInstanceIDKey() []byte {
	return KeyNextInstanceID
}

func GetSchemaByDeployerKey(deployer string, schemaID [32]byte) []byte {
	key := append(KeyPrefixSchemaByDeployer, []byte(deployer)...)
	key = append(key, byte('/'))
	return append(key, schemaID[:]...)
}

func GetSchemaByDeployerPrefixKey(deployer string) []byte {
	key := append(KeyPrefixSchemaByDeployer, []byte(deployer)...)
	return append(key, byte('/'))
}

func GetInstanceBySchemaKey(schemaID [32]byte, instanceID uint64) []byte {
	key := append(KeyPrefixInstanceBySchema, schemaID[:]...)
	key = append(key, byte('/'))
	return append(key, sdk.Uint64ToBigEndian(instanceID)...)
}

func GetInstanceBySchemaPrefixKey(schemaID [32]byte) []byte {
	key := append(KeyPrefixInstanceBySchema, schemaID[:]...)
	return append(key, byte('/'))
}
