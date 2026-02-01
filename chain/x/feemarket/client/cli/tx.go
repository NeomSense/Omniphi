package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"pos/x/feemarket/types"
)

// GetTxCmd returns the transaction commands for the feemarket module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Transaction commands for the feemarket module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdUpdateParams(),
	)

	return cmd
}

// CmdUpdateParams creates a governance proposal to update feemarket parameters
func CmdUpdateParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-params",
		Short: "Submit a governance proposal to update feemarket parameters",
		Long: `Submit a governance proposal to update feemarket module parameters.

This command helps you create a parameter change proposal that must be voted on by governance.

Parameters that can be updated:
- min_gas_price: Minimum gas price floor (e.g., "0.05")
- base_fee_enabled: Enable/disable EIP-1559 base fee (true/false)
- base_fee_initial: Initial base fee value (e.g., "0.05")
- elasticity_multiplier: Base fee adjustment factor (e.g., "1.125")
- max_tip_ratio: Maximum tip percentage (e.g., "0.20")
- target_block_utilization: Target block usage (e.g., "0.33")
- max_tx_gas: Maximum gas per transaction (e.g., "10000000")
- free_tx_quota: Free transactions per block (e.g., "100")
- burn_cool: Burn rate for low utilization (e.g., "0.10")
- burn_normal: Burn rate for normal utilization (e.g., "0.20")
- burn_hot: Burn rate for high utilization (e.g., "0.40")
- util_cool_threshold: Threshold for cool tier (e.g., "0.16")
- util_hot_threshold: Threshold for hot tier (e.g., "0.33")
- validator_fee_ratio: Validator share post-burn (e.g., "0.70")
- treasury_fee_ratio: Treasury share post-burn (e.g., "0.30")
- max_burn_ratio: Maximum burn cap (e.g., "0.50")
- min_gas_price_floor: Hard minimum floor (e.g., "0.025")

Example - Create proposal JSON file (proposal.json):
{
  "title": "Update Fee Market Parameters",
  "summary": "Increase burn rate for hot tier to 50%",
  "messages": [
    {
      "@type": "/pos.feemarket.v1.MsgUpdateParams",
      "authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700j6z2zm3",
      "params": {
        "min_gas_price": "0.050000000000000000",
        "base_fee_enabled": true,
        "base_fee_initial": "0.050000000000000000",
        "elasticity_multiplier": "1.125000000000000000",
        "max_tip_ratio": "0.200000000000000000",
        "target_block_utilization": "0.330000000000000000",
        "max_tx_gas": "10000000",
        "free_tx_quota": "100",
        "burn_cool": "0.100000000000000000",
        "burn_normal": "0.200000000000000000",
        "burn_hot": "0.500000000000000000",
        "util_cool_threshold": "0.160000000000000000",
        "util_hot_threshold": "0.330000000000000000",
        "validator_fee_ratio": "0.700000000000000000",
        "treasury_fee_ratio": "0.300000000000000000",
        "max_burn_ratio": "0.500000000000000000",
        "min_gas_price_floor": "0.025000000000000000"
      }
    }
  ],
  "deposit": "10000000omniphi",
  "metadata": "ipfs://CID"
}

Then submit:
$ posd tx gov submit-proposal proposal.json --from validator

Note: This requires governance module integration. Parameters can only be changed
through governance proposals that must be voted on by token holders.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is informational - actual proposal submission goes through gov module
			fmt.Println("To update feemarket parameters:")
			fmt.Println("1. Create a governance proposal JSON file (see example above)")
			fmt.Println("2. Submit using: posd tx gov submit-proposal proposal.json --from <key>")
			fmt.Println("3. Vote on the proposal: posd tx gov vote <proposal-id> yes --from <key>")
			fmt.Println("4. Wait for voting period to end")
			fmt.Println("5. If passed, parameters will be updated automatically")

			return nil
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// Helper function to create a parameter update message (for programmatic use)
func NewMsgUpdateParams(authority string, params types.FeeMarketParams) *types.MsgUpdateParams {
	return &types.MsgUpdateParams{
		Authority: authority,
		Params:    params,
	}
}

// ValidateParams validates parameter values before submission
func ValidateParams(params types.FeeMarketParams) error {
	return params.Validate()
}
