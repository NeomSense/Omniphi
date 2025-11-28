package module

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"pos/x/poc/types"
)

// decodeHashString decodes a hex string (with or without 0x prefix) to bytes
func decodeHashString(hashStr string) ([]byte, error) {
	// Remove 0x or 0X prefix if present
	hashStr = strings.TrimPrefix(hashStr, "0x")
	hashStr = strings.TrimPrefix(hashStr, "0X")

	// Decode hex string to bytes
	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}

	return hashBytes, nil
}

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetCmdSubmitContribution(),
		GetCmdEndorse(),
		GetCmdWithdrawPOCRewards(),
	)

	return cmd
}

// GetCmdSubmitContribution implements the submit-contribution command
func GetCmdSubmitContribution() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-contribution [ctype] [uri] [hash]",
		Short: "Submit a proof-of-contribution",
		Long: `Submit a proof-of-contribution with a cryptographic hash.

The hash must be a valid SHA256 (64 hex characters) or SHA512 (128 hex characters) hash.
The hash can be provided with or without the '0x' prefix.

Examples:
  posd tx poc submit-contribution code ipfs://QmHash... 0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 --from bob
  posd tx poc submit-contribution code ipfs://QmHash... e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 --from bob`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Decode the hex hash string
			hashBytes, err := decodeHashString(args[2])
			if err != nil {
				return fmt.Errorf("failed to decode hash: %w", err)
			}

			msg := &types.MsgSubmitContribution{
				Contributor: clientCtx.GetFromAddress().String(),
				Ctype:       args[0],
				Uri:         args[1],
				Hash:        hashBytes,
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

// GetCmdEndorse implements the endorse command
func GetCmdEndorse() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endorse [contribution-id] [decision]",
		Short: "Endorse or reject a contribution (validators only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			contributionID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid contribution ID: %w", err)
			}

			decision, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid decision (must be true or false): %w", err)
			}

			msg := &types.MsgEndorse{
				Validator:      clientCtx.GetFromAddress().String(),
				ContributionId: contributionID,
				Decision:       decision,
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

// GetCmdWithdrawPOCRewards implements the withdraw-poc-rewards command
func GetCmdWithdrawPOCRewards() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw-poc-rewards",
		Short: "Withdraw accumulated PoC credits as coins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgWithdrawPOCRewards{
				Address: clientCtx.GetFromAddress().String(),
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

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetCmdQueryParams(),
		GetCmdQueryContribution(),
		GetCmdQueryContributions(),
		GetCmdQueryCredits(),
	)

	return cmd
}

// GetCmdQueryParams implements the query params command
func GetCmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the current module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			req := &types.QueryParamsRequest{}

			res, err := queryClient.Params(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryContribution implements the query contribution command
func GetCmdQueryContribution() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contribution [id]",
		Short: "Query a contribution by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid contribution ID: %w", err)
			}

			queryClient := types.NewQueryClient(clientCtx)
			req := &types.QueryContributionRequest{Id: id}

			res, err := queryClient.Contribution(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryContributions implements the query contributions command
func GetCmdQueryContributions() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contributions",
		Short: "Query all contributions with optional filtering",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			contributor, _ := cmd.Flags().GetString("contributor")
			ctype, _ := cmd.Flags().GetString("ctype")
			verified, _ := cmd.Flags().GetBool("verified")

			var verifiedFilter int32
			if verified {
				verifiedFilter = 1
			}

			queryClient := types.NewQueryClient(clientCtx)
			req := &types.QueryContributionsRequest{
				Contributor: contributor,
				Ctype:       ctype,
				Verified:    verifiedFilter,
			}

			res, err := queryClient.Contributions(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	cmd.Flags().String("contributor", "", "Filter by contributor address")
	cmd.Flags().String("ctype", "", "Filter by contribution type")
	cmd.Flags().Bool("verified", false, "Filter by verified status")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryCredits implements the query credits command
func GetCmdQueryCredits() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credits [address]",
		Short: "Query credit balance and tier for an address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			req := &types.QueryCreditsRequest{Address: args[0]}

			res, err := queryClient.Credits(cmd.Context(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
