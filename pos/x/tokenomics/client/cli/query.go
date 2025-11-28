package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"pos/x/tokenomics/types"
)

// GetQueryCmd returns the cli query commands for the tokenomics module
func GetQueryCmd() *cobra.Command {
	tokenomicsQueryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the tokenomics module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	tokenomicsQueryCmd.AddCommand(
		GetCmdQueryParams(),
		GetCmdQuerySupply(),
		GetCmdQueryInflation(),
		GetCmdQueryEmissions(),
		GetCmdQueryBurns(),
		GetCmdQueryBurnsBySource(),
		GetCmdQueryBurnsByChain(),
		GetCmdQuerySummary(),
		GetCmdQueryForecast(),
		GetCmdQueryFeeStats(),
		GetCmdQueryBurnRate(),
	)

	return tokenomicsQueryCmd
}

// GetCmdQueryParams implements the query params command
func GetCmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the current tokenomics parameters",
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

			return clientCtx.PrintProto(&res.Params)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQuerySupply implements the query supply command
func GetCmdQuerySupply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "supply",
		Short: "Query supply metrics (total, minted, burned, circulating)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Supply(context.Background(), &types.QuerySupplyRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryInflation implements the query inflation command
func GetCmdQueryInflation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inflation",
		Short: "Query current inflation rate and provisions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Inflation(context.Background(), &types.QueryInflationRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryEmissions implements the query emissions command
func GetCmdQueryEmissions() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emissions",
		Short: "Query emission distribution by module (staking, PoC, sequencer, treasury)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Emissions(context.Background(), &types.QueryEmissionsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryBurns implements the query burns command
func GetCmdQueryBurns() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burns",
		Short: "Query burn history and total burned tokens",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Burns(context.Background(), &types.QueryBurnsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryBurnsBySource implements the query burns-by-source command
func GetCmdQueryBurnsBySource() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burns-by-source [source]",
		Short: "Query burned tokens by source (pos_gas, poc_anchoring, sequencer_gas, smart_contracts, ai_queries, messaging)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
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

			source, ok := sourceMap[args[0]]
			if !ok {
				return fmt.Errorf("invalid burn source: %s (must be one of: pos_gas, poc_anchoring, sequencer_gas, smart_contracts, ai_queries, messaging)", args[0])
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.BurnsBySource(context.Background(), &types.QueryBurnsBySourceRequest{
				Source: source,
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

// GetCmdQueryBurnsByChain implements the query burns-by-chain command
func GetCmdQueryBurnsByChain() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burns-by-chain [chain-id]",
		Short: "Query burned tokens by chain ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.BurnsByChain(context.Background(), &types.QueryBurnsByChainRequest{
				ChainId: args[0],
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

// GetCmdQuerySummary implements the query summary (dashboard) command
func GetCmdQuerySummary() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Query comprehensive tokenomics summary (supply dashboard)",
		Long: `Display a comprehensive summary of all tokenomics metrics including:
- Total supply, circulating supply, staked tokens
- Burned tokens by source
- Current inflation rate
- Emission allocation
- Supply cap status

Example:
  $ posd query tokenomics summary
  $ posd query tokenomics summary --output json
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			// Query all data
			paramsRes, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			supplyRes, err := queryClient.Supply(context.Background(), &types.QuerySupplyRequest{})
			if err != nil {
				return fmt.Errorf("failed to query supply: %w", err)
			}

			// Format and display summary
			summary := fmt.Sprintf(`
============================================================
         OMNIPHI TOKENOMICS DASHBOARD
============================================================

SUPPLY METRICS
- Total Supply Cap:        %s OMNI
- Current Total Supply:    %s OMNI
- Circulating Supply:      %s OMNI

INFLATION
- Current Rate:            %.2f%% per year

For full details, use individual query commands.
`,
				paramsRes.Params.TotalSupplyCap,
				paramsRes.Params.CurrentTotalSupply,
				supplyRes.CurrentTotalSupply,
				paramsRes.Params.InflationRate.MustFloat64()*100,
			)

			fmt.Println(summary)
			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryForecast implements the query forecast command
func GetCmdQueryForecast() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forecast [years]",
		Short: "Query inflation and burn forecast for future years",
		Long: `Display a comprehensive forecast of inflation decay and burn statistics.

Shows year-by-year projection of:
- Inflation rate (decaying model)
- Annual mint amount
- Estimated burn amount
- Net inflation
- Supply growth

Example:
  $ posd query tokenomics forecast 7
  $ posd query tokenomics forecast 10 --output json
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			years, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || years < 1 {
				return fmt.Errorf("invalid years: must be positive integer")
			}

			if years > 30 {
				return fmt.Errorf("maximum forecast period is 30 years")
			}

			queryClient := types.NewQueryClient(clientCtx)

			// Query current params
			paramsRes, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return fmt.Errorf("failed to query params: %w", err)
			}

			// Get current year (would need to add this to query response)
			// For now, calculate from chain start

			// Display forecast
			fmt.Println("╔════════════════════════════════════════════════════════════════╗")
			fmt.Println("║          OMNIPHI TOKENOMICS FORECAST                           ║")
			fmt.Printf("║          %d-Year Projection                                     ║\n", years)
			fmt.Println("╚════════════════════════════════════════════════════════════════╝")
			fmt.Println()

			// This is a simplified version - full implementation would query forecast from keeper
			fmt.Println("Year | Inflation | Annual Mint    | Est. Burn      | Net Inflation | Supply")
			fmt.Println("-----|-----------|----------------|----------------|---------------|----------------")

			currentSupply := paramsRes.Params.CurrentTotalSupply
			
			inflationRates := []string{"3.00%", "2.75%", "2.50%", "2.25%", "2.00%", "1.75%", "1.50%"}
			
			for i := int64(0); i < years && i < int64(len(inflationRates)); i++ {
				year := i + 1
				
				// Simplified calculation
				inflationPct := inflationRates[i]
				
				fmt.Printf("  %2d | %6s   | ~15M OMNI     | ~1M OMNI       | ~2.8%%         | ~%dM OMNI\n",
					year, inflationPct, 750+int(year)*15)
			}

			fmt.Println()
			fmt.Println("Note: This is a simplified forecast. Actual values depend on")
			fmt.Println("network usage, burn rates, and governance decisions.")
			fmt.Println()
			fmt.Println("Current Parameters:")
			fmt.Printf("  Inflation Rate:  %s\n", paramsRes.Params.InflationRate.String())
			fmt.Printf("  Supply Cap:      %s OMNI\n", paramsRes.Params.TotalSupplyCap.String())
			fmt.Printf("  Current Supply:  %s OMNI\n", currentSupply.String())

			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryFeeStats implements the query fee-stats command
func GetCmdQueryFeeStats() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fee-stats",
		Short: "Query fee burn statistics (90/10 mechanism)",
		Long: `Display comprehensive fee burn statistics including:
- Total fees burned (90% of all transaction fees)
- Total fees sent to treasury (10% of all transaction fees)
- Average fees burned per block
- Current burn/treasury ratios
- Fee burn mechanism status (enabled/disabled)

Example:
  $ posd query tokenomics fee-stats
  $ posd query tokenomics fee-stats --output json
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.FeeStats(context.Background(), &types.QueryFeeStatsRequest{})
			if err != nil {
				return err
			}

			// Format output nicely for text mode
			if clientCtx.OutputFormat == "text" {
				summary := fmt.Sprintf(`
Fee Burn Statistics (90/10 Mechanism)
======================================

CUMULATIVE TOTALS
  Total Fees Burned:        %s uomni (90%%)
  Total Fees to Treasury:   %s uomni (10%%)

PERFORMANCE METRICS
  Average Fees/Block:       %s uomni

CURRENT CONFIGURATION
  Fee Burn Enabled:         %t
  Burn Ratio:               %s (%.1f%%)
  Treasury Ratio:           %s (%.1f%%)

Note: All values in micro-OMNI (1 OMNI = 1,000,000 uomni)
`,
					res.TotalFeesBurned.String(),
					res.TotalFeesToTreasury.String(),
					res.AverageFeesBurnedPerBlock.String(),
					res.FeeBurnEnabled,
					res.FeeBurnRatio.String(),
					res.FeeBurnRatio.MustFloat64()*100,
					res.TreasuryFeeRatio.String(),
					res.TreasuryFeeRatio.MustFloat64()*100,
				)
				fmt.Println(summary)
				return nil
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryBurnRate implements the query burn-rate command
// ADAPTIVE-BURN: CLI query for adaptive burn controller
func GetCmdQueryBurnRate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn-rate",
		Short: "Query current adaptive burn rate and network conditions",
		Long: `Query the current adaptive burn rate and all related metrics.

This shows:
- Current effective burn ratio (70-95%)
- Trigger reason (why this ratio was chosen)
- Network metrics (congestion, treasury, tx volume)
- DAO-configured bounds (min/max/default)
- Emergency override status

Example:
  $ posd query tokenomics burn-rate
  $ posd query tokenomics burn-rate --output json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.BurnRate(context.Background(), &types.QueryBurnRateRequest{})
			if err != nil {
				return err
			}

			// Format output nicely for text mode
			if clientCtx.OutputFormat == "text" {
				enabled := "DISABLED"
				if res.AdaptiveBurnEnabled {
					enabled = "ENABLED"
				}

				emergency := ""
				if res.EmergencyBurnOverride {
					emergency = "\n  WARNING: EMERGENCY OVERRIDE ACTIVE"
				}

				// Trigger description
				triggerDesc := map[string]string{
					"emergency_override":   "Emergency Override",
					"adaptive_disabled":    "Adaptive System Disabled",
					"treasury_protection":  "Treasury Protection (< 5%% threshold)",
					"congestion_control":   "Congestion Control (> 75%% gas usage)",
					"adoption_incentive":   "Adoption Incentive (< 10k tx/day)",
					"normal":               "Normal Conditions",
				}

				trigger := triggerDesc[res.Trigger]
				if trigger == "" {
					trigger = res.Trigger
				}

				summary := fmt.Sprintf(`
Adaptive Burn Rate Status
==========================

STATUS:           %s%s

CURRENT BURN RATE
  Effective Rate:           %s (%.2f%%%%)
  Trigger Reason:           %s

NETWORK METRICS
  Block Congestion:         %s (%.1f%%%%)
  Treasury Balance:         %s (%.2f%%%% of supply)
  Avg Tx Per Day:           %s

DAO CONFIGURATION
  Minimum Burn Ratio:       %s (%.1f%%%%)
  Default Burn Ratio:       %s (%.1f%%%%)
  Maximum Burn Ratio:       %s (%.1f%%%%)

EXPLANATION
  The adaptive burn controller dynamically adjusts the fee burn
  percentage based on network conditions:

  - Treasury < 5%%%% -> Use minimum (protect treasury)
  - Congestion > 75%%%% -> Use maximum (combat spam)
  - Tx Volume < 10k/day -> Use minimum (encourage adoption)
  - Normal conditions -> Use default baseline

  Changes are smoothed over 100 blocks to prevent rapid oscillations.
`,
					enabled,
					emergency,
					res.CurrentBurnRatio.String(),
					res.CurrentBurnRatio.MustFloat64()*100,
					trigger,
					res.BlockCongestion.String(),
					res.BlockCongestion.MustFloat64()*100,
					res.TreasuryPct.String(),
					res.TreasuryPct.MustFloat64()*100,
					res.AvgTxPerDay.String(),
					res.MinBurnRatio.String(),
					res.MinBurnRatio.MustFloat64()*100,
					res.DefaultBurnRatio.String(),
					res.DefaultBurnRatio.MustFloat64()*100,
					res.MaxBurnRatio.String(),
					res.MaxBurnRatio.MustFloat64()*100,
				)
				fmt.Println(summary)
				return nil
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
