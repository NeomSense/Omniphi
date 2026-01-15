package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"pos/x/timelock/types"
)

// GetQueryCmd returns the cli query commands for the timelock module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryOperation(),
		CmdQueryQueuedOperations(),
		CmdQueryExecutableOperations(),
	)

	return cmd
}

// CmdQueryParams queries the timelock parameters
func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the timelock module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryOperation queries a specific timelock operation
func CmdQueryOperation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operation [operation-id]",
		Short: "Query a timelock operation by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			operationID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid operation ID: %w", err)
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Operation(context.Background(), &types.QueryOperationRequest{
				OperationId: operationID,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryQueuedOperations queries all queued timelock operations
func CmdQueryQueuedOperations() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queued",
		Short: "Query all queued timelock operations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.QueuedOperations(context.Background(), &types.QueryQueuedOperationsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryExecutableOperations queries all executable timelock operations
func CmdQueryExecutableOperations() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "executable",
		Short: "Query all operations ready for execution (delay period has passed)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ExecutableOperations(context.Background(), &types.QueryExecutableOperationsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
