package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ContractStatus tracks the lifecycle of a deployed contract schema.
type ContractStatus string

const (
	ContractStatusActive     ContractStatus = "ACTIVE"
	ContractStatusSuspended  ContractStatus = "SUSPENDED"
	ContractStatusDeprecated ContractStatus = "DEPRECATED"
)

// IntentSchemaField describes a parameter in a contract intent method.
type IntentSchemaField struct {
	Name     string `json:"name"`
	TypeHint string `json:"type_hint"` // e.g. "u128", "address", "bytes"
}

// IntentSchema describes a single intent method the contract supports.
type IntentSchema struct {
	Method       string              `json:"method"`
	Params       []IntentSchemaField `json:"params"`
	Capabilities []string            `json:"capabilities"` // required capability names
}

// ContractSchema is the on-chain registration of a deployed contract.
type ContractSchema struct {
	SchemaID      string         `json:"schema_id"`       // hex-encoded 32-byte ID
	Deployer      string         `json:"deployer"`        // bech32 address
	Version       uint64         `json:"version"`
	Name          string         `json:"name"`            // human-readable contract name
	Description   string         `json:"description"`
	DomainTag     string         `json:"domain_tag"`      // safety kernel domain
	IntentSchemas []IntentSchema `json:"intent_schemas"`
	MaxGasPerCall uint64         `json:"max_gas_per_call"`
	MaxStateBytes uint64         `json:"max_state_bytes"`
	ValidatorHash string         `json:"validator_hash"`  // SHA256 of Wasm bytecode (hex)
	WasmSize      uint64         `json:"wasm_size"`       // bytes
	Status        ContractStatus `json:"status"`
	DeployedAt    int64          `json:"deployed_at"`     // block height
}

// ComputeSchemaID deterministically derives a schema ID from deployer + name + version.
func ComputeSchemaID(deployer, name string, version uint64) [32]byte {
	h := sha256.New()
	h.Write([]byte("OMNIPHI_CONTRACT_SCHEMA_V1"))
	h.Write([]byte(deployer))
	h.Write([]byte(name))
	vBytes := make([]byte, 8)
	vBytes[0] = byte(version >> 56)
	vBytes[1] = byte(version >> 48)
	vBytes[2] = byte(version >> 40)
	vBytes[3] = byte(version >> 32)
	vBytes[4] = byte(version >> 24)
	vBytes[5] = byte(version >> 16)
	vBytes[6] = byte(version >> 8)
	vBytes[7] = byte(version)
	h.Write(vBytes)
	var id [32]byte
	copy(id[:], h.Sum(nil))
	return id
}

func (s ContractSchema) Marshal() ([]byte, error)   { return json.Marshal(s) }
func (s *ContractSchema) Unmarshal(bz []byte) error  { return json.Unmarshal(bz, s) }

// ContractInstance represents a deployed instance of a contract schema.
type ContractInstance struct {
	InstanceID  uint64 `json:"instance_id"`
	SchemaID    string `json:"schema_id"`    // hex
	Creator     string `json:"creator"`      // bech32
	Admin       string `json:"admin"`        // bech32 (can migrate)
	Label       string `json:"label"`        // human-readable label
	CreatedAt   int64  `json:"created_at"`   // block height
}

func (i ContractInstance) Marshal() ([]byte, error)  { return json.Marshal(i) }
func (i *ContractInstance) Unmarshal(bz []byte) error { return json.Unmarshal(bz, i) }

// MsgDeployContract deploys a new contract schema with Wasm bytecode.
type MsgDeployContract struct {
	Deployer      string         `json:"deployer"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	DomainTag     string         `json:"domain_tag"`
	IntentSchemas []IntentSchema `json:"intent_schemas"`
	MaxGasPerCall uint64         `json:"max_gas_per_call"`
	MaxStateBytes uint64         `json:"max_state_bytes"`
	WasmBytecode  []byte         `json:"wasm_bytecode"`
}

func (m MsgDeployContract) ValidateBasic() error {
	if m.Deployer == "" {
		return fmt.Errorf("deployer cannot be empty")
	}
	if m.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(m.WasmBytecode) == 0 {
		return fmt.Errorf("wasm_bytecode cannot be empty")
	}
	if m.MaxGasPerCall == 0 {
		return fmt.Errorf("max_gas_per_call must be > 0")
	}
	if m.MaxStateBytes == 0 {
		return fmt.Errorf("max_state_bytes must be > 0")
	}
	if len(m.IntentSchemas) == 0 {
		return fmt.Errorf("at least one intent_schema is required")
	}
	return nil
}

// MsgInstantiateContract creates an instance of a deployed schema.
type MsgInstantiateContract struct {
	Creator  string `json:"creator"`
	SchemaID string `json:"schema_id"` // hex
	Label    string `json:"label"`
	Admin    string `json:"admin"`     // optional admin for migration
}

func (m MsgInstantiateContract) ValidateBasic() error {
	if m.Creator == "" {
		return fmt.Errorf("creator cannot be empty")
	}
	if m.SchemaID == "" {
		return fmt.Errorf("schema_id cannot be empty")
	}
	if len(m.SchemaID) != 64 {
		return fmt.Errorf("schema_id must be 64 hex chars (32 bytes)")
	}
	if _, err := hex.DecodeString(m.SchemaID); err != nil {
		return fmt.Errorf("schema_id must be valid hex: %w", err)
	}
	return nil
}

// GenesisState defines the contracts module genesis.
type GenesisState struct {
	Params    Params           `json:"params"`
	Schemas   []ContractSchema `json:"schemas"`
	Instances []ContractInstance `json:"instances"`
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:    DefaultParams(),
		Schemas:   []ContractSchema{},
		Instances: []ContractInstance{},
	}
}

func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}
