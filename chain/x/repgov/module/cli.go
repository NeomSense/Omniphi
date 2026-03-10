package module

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"pos/x/repgov/keeper"
	"pos/x/repgov/types"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "RepGov module transactions",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	return cmd
}

// GetQueryCmd returns the query commands for this module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Query commands for the repgov module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryVoterWeight(),
		CmdQueryAllWeights(),
	)

	return cmd
}

// CmdQueryParams returns the params query command
func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the repgov module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			_ = clientCtx
			fmt.Println("Use: posd query repgov params (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryVoterWeight queries a specific voter's governance weight
func CmdQueryVoterWeight() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "weight [address]",
		Short: "Query a voter's governance weight",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Querying governance weight for: %s\n", args[0])
			fmt.Println("Use: posd query repgov weight <address> (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryAllWeights queries all voter weights
func CmdQueryAllWeights() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "weights",
		Short: "Query all voter governance weights",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Use: posd query repgov weights (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// DirectQueryVoterWeight is a helper for direct keeper querying
func DirectQueryVoterWeight(k keeper.Keeper, ctx context.Context, addr string) (string, error) {
	vw, found := k.GetVoterWeight(ctx, addr)
	if !found {
		return "", fmt.Errorf("voter weight not found for %s", addr)
	}
	bz, err := json.MarshalIndent(vw, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bz), nil
}
