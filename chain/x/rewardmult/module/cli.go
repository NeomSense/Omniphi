package module

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"pos/x/rewardmult/keeper"
	"pos/x/rewardmult/types"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "RewardMult module transactions",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	// Governance-only param updates are submitted via gov proposals, not direct TX commands.
	return cmd
}

// GetQueryCmd returns the query commands for this module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Query commands for the rewardmult module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryMultiplier(),
		CmdQueryAllMultipliers(),
	)

	return cmd
}

// CmdQueryParams returns the params query command
func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the rewardmult module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			_ = clientCtx

			// For manual types, we use a direct keeper query pattern
			// In production with gRPC, this would use the query client
			fmt.Println("Use: posd query rewardmult params (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryMultiplier returns a specific validator's multiplier
func CmdQueryMultiplier() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multiplier [validator-address]",
		Short: "Query a validator's reward multiplier",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			_ = clientCtx
			fmt.Printf("Querying multiplier for validator: %s\n", args[0])
			fmt.Println("Use: posd query rewardmult multiplier <valoper> (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryAllMultipliers returns all validator multipliers
func CmdQueryAllMultipliers() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multipliers",
		Short: "Query all validator reward multipliers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			_ = clientCtx
			fmt.Println("Use: posd query rewardmult multipliers (requires running node with gRPC)")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// DirectQueryMultiplier is a helper for direct keeper querying (for tests and scripts)
func DirectQueryMultiplier(k keeper.Keeper, ctx context.Context, valAddr string) (string, error) {
	vm, found := k.GetValidatorMultiplier(ctx, valAddr)
	if !found {
		return "", fmt.Errorf("multiplier not found for %s", valAddr)
	}
	bz, err := json.MarshalIndent(vm, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bz), nil
}
