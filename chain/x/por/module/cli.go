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

	"pos/x/por/types"
)

// decodeHexString decodes a hex string (with or without 0x prefix) to bytes
func decodeHexString(hexStr string) ([]byte, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	hexStr = strings.TrimPrefix(hexStr, "0X")
	return hex.DecodeString(hexStr)
}

// GetTxCmd returns the transaction commands for the por module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetCmdRegisterApp(),
		GetCmdSubmitBatch(),
		GetCmdSubmitAttestation(),
		GetCmdChallengeBatch(),
	)

	return cmd
}

// GetCmdRegisterApp implements the register-app command
func GetCmdRegisterApp() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-app [name] [schema-cid] [challenge-period] [min-verifiers]",
		Short: "Register a new application in the PoR module",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			challengePeriod, err := strconv.ParseInt(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid challenge period: %w", err)
			}

			minVerifiers, err := strconv.ParseUint(args[3], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid min verifiers: %w", err)
			}

			msg := &types.MsgRegisterApp{
				Owner:           clientCtx.GetFromAddress().String(),
				Name:            args[0],
				SchemaCid:       args[1],
				ChallengePeriod: challengePeriod,
				MinVerifiers:    uint32(minVerifiers),
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

// GetCmdSubmitBatch implements the submit-batch command
func GetCmdSubmitBatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-batch [app-id] [epoch] [merkle-root-hex] [record-count] [verifier-set-id]",
		Short: "Submit a batch commitment (merkle root) for off-chain records",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			appID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid app ID: %w", err)
			}

			epoch, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}

			merkleRoot, err := decodeHexString(args[2])
			if err != nil {
				return fmt.Errorf("invalid merkle root hex: %w", err)
			}

			recordCount, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid record count: %w", err)
			}

			verifierSetID, err := strconv.ParseUint(args[4], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid verifier set ID: %w", err)
			}

			msg := &types.MsgSubmitBatch{
				Submitter:        clientCtx.GetFromAddress().String(),
				AppId:            appID,
				Epoch:            epoch,
				RecordMerkleRoot: merkleRoot,
				RecordCount:      recordCount,
				VerifierSetId:    verifierSetID,
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

// GetCmdSubmitAttestation implements the submit-attestation command
func GetCmdSubmitAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-attestation [batch-id] [signature-hex] [confidence-weight]",
		Short: "Submit a verifier attestation for a batch",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			batchID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid batch ID: %w", err)
			}

			signature, err := decodeHexString(args[1])
			if err != nil {
				return fmt.Errorf("invalid signature hex: %w", err)
			}

			// Parse confidence weight as a decimal string (e.g., "0.95")
			confWeight, err := parseDecimal(args[2])
			if err != nil {
				return fmt.Errorf("invalid confidence weight: %w", err)
			}

			msg := &types.MsgSubmitAttestation{
				Verifier:         clientCtx.GetFromAddress().String(),
				BatchId:          batchID,
				Signature:        signature,
				ConfidenceWeight: confWeight,
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

// GetCmdChallengeBatch implements the challenge-batch command
func GetCmdChallengeBatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge-batch [batch-id] [challenge-type] [proof-data-hex]",
		Short: "Submit a fraud proof challenge against a batch",
		Long: `Submit a fraud proof challenge against a pending batch.

Challenge types:
  0 = INVALID_ROOT (merkle root does not match records)
  1 = DOUBLE_INCLUSION (same record in multiple batches)
  2 = MISSING_RECORD (claimed records missing from tree)
  3 = INVALID_SCHEMA (records don't conform to schema)`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			batchID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid batch ID: %w", err)
			}

			challengeTypeUint, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid challenge type: %w", err)
			}

			proofData, err := decodeHexString(args[2])
			if err != nil {
				return fmt.Errorf("invalid proof data hex: %w", err)
			}

			msg := &types.MsgChallengeBatch{
				Challenger:    clientCtx.GetFromAddress().String(),
				BatchId:       batchID,
				ChallengeType: types.ChallengeType(challengeTypeUint),
				ProofData:     proofData,
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

// parseDecimal parses a string decimal like "0.95" into math.LegacyDec
func parseDecimal(s string) (types.Dec, error) {
	return types.ParseDec(s)
}

// GetQueryCmd returns the query commands for the por module
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
		GetCmdQueryApp(),
		GetCmdQueryApps(),
		GetCmdQueryBatch(),
		GetCmdQueryBatchesByEpoch(),
	)

	return cmd
}

// GetCmdQueryParams implements the query params command
func GetCmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the current PoR module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			_ = clientCtx
			fmt.Println("Query params - requires proto-generated query client")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryApp implements the query app command
func GetCmdQueryApp() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app [app-id]",
		Short: "Query a registered app by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			_ = clientCtx
			fmt.Printf("Query app %s - requires proto-generated query client\n", args[0])
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryApps implements the query apps command
func GetCmdQueryApps() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Query all registered apps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			_ = clientCtx
			fmt.Println("Query apps - requires proto-generated query client")
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryBatch implements the query batch command
func GetCmdQueryBatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch [batch-id]",
		Short: "Query a batch commitment by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			_ = clientCtx
			fmt.Printf("Query batch %s - requires proto-generated query client\n", args[0])
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCmdQueryBatchesByEpoch implements the query batches-by-epoch command
func GetCmdQueryBatchesByEpoch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batches-by-epoch [epoch]",
		Short: "Query all batches in a given epoch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			_ = clientCtx
			fmt.Printf("Query batches for epoch %s - requires proto-generated query client\n", args[0])
			return nil
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
