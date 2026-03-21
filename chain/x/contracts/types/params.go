package types

import (
	"encoding/json"
	"fmt"
)

// Params configures the x/contracts module.
type Params struct {
	// MaxWasmSize is the maximum allowed Wasm bytecode size in bytes.
	MaxWasmSize uint64 `json:"max_wasm_size"`

	// MinDeploymentBond is the minimum bond (in base denom) required to deploy.
	MinDeploymentBond uint64 `json:"min_deployment_bond"`

	// MaxContractsPerDeployer limits schemas per address to prevent spam.
	MaxContractsPerDeployer uint32 `json:"max_contracts_per_deployer"`

	// ConstraintValidatorGasLimit is the max wazero fuel for a single validation.
	ConstraintValidatorGasLimit uint64 `json:"constraint_validator_gas_limit"`

	// MaxInstancesPerSchema limits how many instances can be created per schema.
	MaxInstancesPerSchema uint32 `json:"max_instances_per_schema"`

	// MaxStateBytesHardCap is the absolute maximum any contract can declare.
	MaxStateBytesHardCap uint64 `json:"max_state_bytes_hard_cap"`
}

func DefaultParams() Params {
	return Params{
		MaxWasmSize:                 1_048_576, // 1 MB
		MinDeploymentBond:           100_000,   // 0.1 OMNI
		MaxContractsPerDeployer:     50,
		ConstraintValidatorGasLimit: 10_000_000,
		MaxInstancesPerSchema:       1000,
		MaxStateBytesHardCap:        1_048_576, // 1 MB
	}
}

func (p Params) Validate() error {
	if p.MaxWasmSize == 0 {
		return fmt.Errorf("max_wasm_size must be > 0")
	}
	if p.ConstraintValidatorGasLimit == 0 {
		return fmt.Errorf("constraint_validator_gas_limit must be > 0")
	}
	if p.MaxStateBytesHardCap == 0 {
		return fmt.Errorf("max_state_bytes_hard_cap must be > 0")
	}
	return nil
}

func (p Params) Marshal() ([]byte, error)   { return json.Marshal(p) }
func (p *Params) Unmarshal(bz []byte) error  { return json.Unmarshal(bz, p) }
