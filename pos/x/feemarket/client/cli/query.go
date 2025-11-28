package cli

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"pos/x/feemarket/types"
)

// GetQueryCmd returns the CLI query commands for the feemarket module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the feemarket module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQueryBaseFee(),
		CmdQueryBlockUtilization(),
		CmdQueryBurnTier(),
		CmdQueryFeeStats(),
	)

	return cmd
}

// CmdQueryParams queries the feemarket module parameters
func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query feemarket module parameters",
		Long: `Query the current feemarket module parameters including:
- Minimum gas price and base fee settings
- Elasticity multiplier for EIP-1559
- Adaptive burn tier percentages (cool/normal/hot)
- Utilization thresholds for tier selection
- Fee distribution ratios (validators/treasury)
- Safety limits (max burn ratio, min gas price floor)

Example:
$ posd query feemarket params`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(
				context.Background(),
				&types.QueryParamsRequest{},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryBaseFee queries the current EIP-1559 base fee
func CmdQueryBaseFee() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "base-fee",
		Short: "Query current EIP-1559 base fee",
		Long: `Query the current dynamic base fee and effective gas price.

The base fee adjusts based on block utilization:
- If utilization > target: fee increases (congested)
- If utilization < target: fee decreases (underutilized)
- If utilization = target: fee stays same (balanced)

Example:
$ posd query feemarket base-fee`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.BaseFee(
				context.Background(),
				&types.QueryBaseFeeRequest{},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryBlockUtilization queries current block utilization metrics
func CmdQueryBlockUtilization() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block-utilization",
		Short: "Query current block utilization metrics",
		Long: `Query block gas utilization metrics including:
- Current block utilization (gas_used / max_block_gas)
- Previous block utilization (used for base fee calculation)
- Target utilization (optimal network load)

Utilization ranges:
- 0.0 - 0.16: Cool (low congestion)
- 0.16 - 0.33: Normal (moderate congestion)
- 0.33+: Hot (high congestion)

Example:
$ posd query feemarket block-utilization`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.BlockUtilization(
				context.Background(),
				&types.QueryBlockUtilizationRequest{},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryBurnTier queries the current adaptive burn tier
func CmdQueryBurnTier() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn-tier",
		Short: "Query current adaptive burn tier",
		Long: `Query the current burn tier based on block utilization.

Burn tiers:
- Cool (<16% utilization): 10% burn
- Normal (16-33% utilization): 20% burn
- Hot (>33% utilization): 40% burn

Higher congestion = higher burn rate (deflationary pressure)
Lower congestion = lower burn rate (supply stability)

Example:
$ posd query feemarket burn-tier`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.BurnTier(
				context.Background(),
				&types.QueryBurnTierRequest{},
			)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryFeeStats queries cumulative fee statistics
func CmdQueryFeeStats() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fee-stats",
		Short: "Query cumulative fee statistics",
		Long: `Query cumulative fee statistics since genesis including:
- Total fees collected
- Total amount burned (deflationary impact)
- Total sent to treasury
- Total sent to validators
- Percentage breakdowns

This provides transparency into the fee mechanism and tokenomics.

Example:
$ posd query feemarket fee-stats`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.FeeStats(
				context.Background(),
				&types.QueryFeeStatsRequest{},
			)
			if err != nil {
				return err
			}

			// Pretty print the statistics
			fmt.Printf("Fee Statistics:\n")
			fmt.Printf("  Total Fees Processed: %s\n", res.TotalFeesProcessed.String())
			fmt.Printf("  Total Burned: %s\n", res.TotalBurned.String())
			fmt.Printf("  Total to Treasury: %s\n", res.TotalToTreasury.String())
			fmt.Printf("  Total to Validators: %s\n", res.TotalToValidators.String())
			if res.TreasuryAddress != "" {
				fmt.Printf("  Treasury Address: %s\n", res.TreasuryAddress)
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
