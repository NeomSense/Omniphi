package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"pos/x/timelock/types"
)

// GetTxCmd returns the transaction commands for the timelock module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdExecuteOperation(),
		CmdCancelOperation(),
		CmdEmergencyExecute(),
	)

	return cmd
}

// CmdExecuteOperation creates a command to execute a pending timelock operation
func CmdExecuteOperation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute [operation-id]",
		Short: "Execute a pending timelock operation after the delay has passed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			operationID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid operation ID: %w", err)
			}

			msg := &types.MsgExecuteOperation{
				Executor:    clientCtx.GetFromAddress().String(),
				OperationId: operationID,
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdCancelOperation creates a command to cancel a pending timelock operation
func CmdCancelOperation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel [operation-id] [reason]",
		Short: "Cancel a pending timelock operation (guardian or governance only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			operationID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid operation ID: %w", err)
			}

			msg := &types.MsgCancelOperation{
				Authority:   clientCtx.GetFromAddress().String(),
				OperationId: operationID,
				Reason:      args[1],
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdEmergencyExecute creates a command for guardian to execute an operation immediately
func CmdEmergencyExecute() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emergency-execute [operation-id] [justification]",
		Short: "Emergency execute an operation (guardian only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			operationID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid operation ID: %w", err)
			}

			msg := &types.MsgEmergencyExecute{
				Authority:     clientCtx.GetFromAddress().String(),
				OperationId:   operationID,
				Justification: args[1],
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
