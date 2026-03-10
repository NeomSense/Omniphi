package module

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"pos/x/uci/types"
)

func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "UCI module transactions",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	return cmd
}

func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Query commands for the UCI module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryAdapter(),
		CmdQueryAllAdapters(),
	)

	return cmd
}

func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the UCI module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Use: posd query uci params (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryAdapter() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapter [adapter-id]",
		Short: "Query a DePIN adapter by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Querying DePIN adapter: %s\n", args[0])
			fmt.Println("Use: posd query uci adapter <id> (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryAllAdapters() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapters",
		Short: "Query all registered DePIN adapters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Use: posd query uci adapters (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
