package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/contracts/types"
)

// GetTxCmd returns the tx commands for the contracts module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contracts",
		Short: "Intent Contract transaction commands",
	}

	cmd.AddCommand(
		CmdDeployContract(),
		CmdInstantiateContract(),
	)

	return cmd
}

// CmdDeployContract deploys a new contract schema with Wasm bytecode.
func CmdDeployContract() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [name] [wasm-file] [schema-file]",
		Short: "Deploy a new Intent Contract schema",
		Long: `Deploy a new Intent Contract schema with its Wasm constraint validator.

The schema file is a JSON file describing the contract's intent methods:
{
  "description": "Escrow contract",
  "domain_tag": "contract.escrow",
  "intent_schemas": [
    {"method": "fund", "params": [{"name": "amount", "type_hint": "u128"}], "capabilities": ["ContractCall"]},
    {"method": "release", "params": [], "capabilities": ["ContractCall"]}
  ],
  "max_gas_per_call": 1000000,
  "max_state_bytes": 65536
}`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			name := args[0]
			wasmFile := args[1]
			schemaFile := args[2]

			// Read Wasm bytecode
			wasmBytes, err := os.ReadFile(wasmFile)
			if err != nil {
				return fmt.Errorf("failed to read wasm file: %w", err)
			}

			// Read schema definition
			schemaBytes, err := os.ReadFile(schemaFile)
			if err != nil {
				return fmt.Errorf("failed to read schema file: %w", err)
			}

			var schemaDef struct {
				Description   string               `json:"description"`
				DomainTag     string               `json:"domain_tag"`
				IntentSchemas []types.IntentSchema  `json:"intent_schemas"`
				MaxGasPerCall uint64                `json:"max_gas_per_call"`
				MaxStateBytes uint64                `json:"max_state_bytes"`
			}
			if err := json.Unmarshal(schemaBytes, &schemaDef); err != nil {
				return fmt.Errorf("failed to parse schema file: %w", err)
			}

			deployer := clientCtx.GetFromAddress().String()

			msg := &types.MsgDeployContract{
				Deployer:      deployer,
				Name:          name,
				Description:   schemaDef.Description,
				DomainTag:     schemaDef.DomainTag,
				IntentSchemas: schemaDef.IntentSchemas,
				MaxGasPerCall: schemaDef.MaxGasPerCall,
				MaxStateBytes: schemaDef.MaxStateBytes,
				WasmBytecode:  wasmBytes,
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			// Wrap as sdk.Msg for signing
			sdkMsg := &contractsTxMsg{inner: msg}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), sdkMsg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdInstantiateContract creates an instance of a deployed contract schema.
func CmdInstantiateContract() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instantiate [schema-id] [label]",
		Short: "Create an instance of a deployed Intent Contract",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			creator := clientCtx.GetFromAddress().String()
			admin, _ := cmd.Flags().GetString("admin")
			if admin == "" {
				admin = creator
			}

			msg := &types.MsgInstantiateContract{
				Creator:  creator,
				SchemaID: args[0],
				Label:    args[1],
				Admin:    admin,
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			sdkMsg := &contractsTxMsg{inner: msg}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), sdkMsg)
		},
	}

	cmd.Flags().String("admin", "", "Admin address for contract migration (defaults to creator)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// contractsTxMsg wraps deploy/instantiate messages to satisfy sdk.Msg interface.
// This is a temporary adapter until proto types are fully wired.
type contractsTxMsg struct {
	inner interface{ ValidateBasic() error }
}

func (m *contractsTxMsg) ProtoMessage()             {}
func (m *contractsTxMsg) Reset()                    {}
func (m *contractsTxMsg) String() string            { return fmt.Sprintf("%+v", m.inner) }
func (m *contractsTxMsg) ValidateBasic() error      { return m.inner.ValidateBasic() }
func (m *contractsTxMsg) GetSigners() []sdk.AccAddress { return nil }
