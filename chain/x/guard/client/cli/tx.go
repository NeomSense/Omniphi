package cli

import (
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"pos/x/guard/types"
)

// GetTxCmd returns the transaction commands for the guard module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdSubmitAdvisoryLink(),
		CmdConfirmExecution(),
	)

	return cmd
}

// CmdSubmitAdvisoryLink creates a command to submit an advisory link for a proposal
func CmdSubmitAdvisoryLink() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-advisory-link [proposal-id] [report-hash] [report-url]",
		Short: "Submit an advisory report link for a governance proposal",
		Long: `Submit an advisory report link for a governance proposal.
The report-hash must be a 64-character hex SHA256 hash of the report content.

Example:
  posd tx guard submit-advisory-link 3 abcd1234...5678 ipfs://QmXyz... --from reporter`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid proposal ID: %w", err)
			}

			msg := &types.MsgSubmitAdvisoryLink{
				Reporter:   clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
				ReportHash: args[1],
				Uri:        args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdConfirmExecution creates a command to confirm execution of a CRITICAL proposal
func CmdConfirmExecution() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confirm-execution [proposal-id]",
		Short: "Confirm execution for a CRITICAL proposal requiring second confirmation",
		Long: `Confirm execution for a CRITICAL-tier proposal that requires a second confirmation.
Only the governance authority can perform this operation.

Example:
  posd tx guard confirm-execution 5 --from governance`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid proposal ID: %w", err)
			}

			msg := &types.MsgConfirmExecution{
				Authority:  clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
