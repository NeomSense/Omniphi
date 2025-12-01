package cli

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	"pos/x/poc/types"
)

// CmdQueryFeeMetrics queries the cumulative fee burn statistics
func CmdQueryFeeMetrics() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fee-metrics",
		Short: "Query cumulative PoC submission fee burn statistics",
		Long: `Query the cumulative PoC submission fee statistics including:
- Total fees collected from all submissions
- Total amount burned (deflationary impact)
- Total amount redirected to reward pool
- Last updated block height

This provides transparency into the fee burn mechanism and its impact on tokenomics.

Example:
$ posd query poc fee-metrics`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.FeeMetrics(
				context.Background(),
				&types.QueryFeeMetricsRequest{},
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

// CmdQueryContributorFeeStats queries fee statistics for a specific contributor
func CmdQueryContributorFeeStats() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contributor-fee-stats [address]",
		Short: "Query submission fee statistics for a specific contributor",
		Long: `Query detailed fee statistics for a contributor address including:
- Total fees paid across all submissions
- Total amount burned from their submissions
- Number of submissions made
- First and last submission block heights

This enables analytics on contributor behavior and engagement.

Example:
$ posd query poc contributor-fee-stats omni1abc...xyz`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			address := args[0]

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.ContributorFeeStats(
				context.Background(),
				&types.QueryContributorFeeStatsRequest{
					Address: address,
				},
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
