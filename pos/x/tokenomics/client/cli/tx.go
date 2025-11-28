package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"pos/x/tokenomics/types"
)

// GetTxCmd returns the transaction commands for the tokenomics module
func GetTxCmd() *cobra.Command {
	tokenomicsTxCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Tokenomics transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	tokenomicsTxCmd.AddCommand(
		GetCmdBurn(),
		GetCmdReportBurn(),
	)

	return tokenomicsTxCmd
}

// GetCmdBurn implements the burn command
func GetCmdBurn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn [amount] [source]",
		Short: "Burn tokens (amount in uomni, source: pos_gas, poc_anchoring, etc.)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Parse amount
			amount, ok := math.NewIntFromString(args[0])
			if !ok {
				return fmt.Errorf("invalid amount: %s", args[0])
			}

			// Map string to BurnSource enum
			sourceMap := map[string]types.BurnSource{
				"pos_gas":          types.BurnSource_BURN_SOURCE_POS_GAS,
				"poc_anchoring":    types.BurnSource_BURN_SOURCE_POC_ANCHORING,
				"sequencer_gas":    types.BurnSource_BURN_SOURCE_SEQUENCER_GAS,
				"smart_contracts":  types.BurnSource_BURN_SOURCE_SMART_CONTRACTS,
				"ai_queries":       types.BurnSource_BURN_SOURCE_AI_QUERIES,
				"messaging":        types.BurnSource_BURN_SOURCE_MESSAGING,
			}

			source, ok := sourceMap[args[1]]
			if !ok {
				return fmt.Errorf("invalid burn source: %s", args[1])
			}

			msg := &types.MsgBurnTokens{
				Burner:  clientCtx.GetFromAddress().String(),
				Amount:  amount,
				Source:  source,
				ChainId: "", // Will be filled by handler
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// GetCmdReportBurn implements the report-burn command (for cross-chain burns)
func GetCmdReportBurn() *cobra.Command{
	cmd := &cobra.Command{
		Use:   "report-burn [amount] [source] [chain-id] [block-height] [tx-hash]",
		Short: "Report a burn from another chain (IBC)",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Parse amount
			amount, ok := math.NewIntFromString(args[0])
			if !ok {
				return fmt.Errorf("invalid amount: %s", args[0])
			}

			// Parse source
			sourceMap := map[string]types.BurnSource{
				"pos_gas":          types.BurnSource_BURN_SOURCE_POS_GAS,
				"poc_anchoring":    types.BurnSource_BURN_SOURCE_POC_ANCHORING,
				"sequencer_gas":    types.BurnSource_BURN_SOURCE_SEQUENCER_GAS,
				"smart_contracts":  types.BurnSource_BURN_SOURCE_SMART_CONTRACTS,
				"ai_queries":       types.BurnSource_BURN_SOURCE_AI_QUERIES,
				"messaging":        types.BurnSource_BURN_SOURCE_MESSAGING,
			}

			source, ok := sourceMap[args[1]]
			if !ok {
				return fmt.Errorf("invalid burn source: %s", args[1])
			}

			chainID := args[2]

			// Parse block height
			blockHeight, err := strconv.ParseInt(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid block height: %s", args[3])
			}

			txHash := args[4]

			msg := &types.MsgReportBurn{
				Reporter:    clientCtx.GetFromAddress().String(),
				Amount:      amount,
				Source:      source,
				ChainId:     chainID,
				BlockHeight: blockHeight,
				TxHash:      txHash,
				Proof:       []byte{}, // TODO: Add Merkle proof support
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
