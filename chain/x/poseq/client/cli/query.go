package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

// GetQueryCmd returns the CLI query root command for x/poseq.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "poseq",
		Short:                      "Query commands for the poseq module",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdQueryParams(),
		CmdQuerySequencer(),
		CmdQueryAllSequencers(),
		CmdQueryCommittedBatch(),
		CmdQueryCheckpointAnchor(),
		CmdQuerySettlementAnchor(),
		CmdQueryEpochState(),
		CmdQueryCommitteeSnapshot(),
		CmdQueryLivenessEvent(),
		CmdQueryPerformanceRecord(),
		CmdQueryEpochPerformance(),
		// Phase 5: Economic Enforcement Layer
		CmdQueryOperatorBond(),
		CmdQueryRewardScore(),
		CmdQueryEpochRewardScores(),
		CmdQuerySlashQueue(),
		CmdQueryOperatorProfile(),
	)

	return cmd
}

// CmdQueryParams returns the params query command.
func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query poseq module parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// For manual-types modules without protobuf gRPC, query via REST or direct store.
			return printJSON(clientCtx, map[string]string{"note": "use gRPC endpoint /omniphi.poseq.v1.Query/Params"})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQuerySequencer queries a single sequencer by node_id hex.
func CmdQuerySequencer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sequencer [node-id-hex]",
		Short: "Query a sequencer by node ID (64-char hex)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			nodeID := args[0]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d chars", len(nodeID))
			}
			return printJSON(clientCtx, map[string]string{
				"query":   "sequencer",
				"node_id": nodeID,
				"note":    "use gRPC endpoint /omniphi.poseq.v1.Query/Sequencer",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryAllSequencers lists all registered sequencers.
func CmdQueryAllSequencers() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sequencers",
		Short: "List all registered sequencers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			return printJSON(clientCtx, map[string]string{
				"query": "all-sequencers",
				"note":  "use gRPC endpoint /omniphi.poseq.v1.Query/AllSequencers",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryCommittedBatch queries a committed batch by batch_id hex.
func CmdQueryCommittedBatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "committed-batch [batch-id-hex]",
		Short: "Query a committed batch by batch ID (64-char hex)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			batchID := args[0]
			if len(batchID) != 64 {
				return fmt.Errorf("batch_id must be 64-char hex, got %d chars", len(batchID))
			}
			return printJSON(clientCtx, map[string]string{
				"query":    "committed-batch",
				"batch_id": batchID,
				"note":     "use gRPC endpoint /omniphi.poseq.v1.Query/CommittedBatch",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryCheckpointAnchor queries a checkpoint anchor by epoch and slot.
func CmdQueryCheckpointAnchor() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkpoint-anchor [epoch] [slot]",
		Short: "Query a checkpoint anchor by epoch and slot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			slot, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid slot: %w", err)
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query": "checkpoint-anchor",
				"epoch": epoch,
				"slot":  slot,
				"note":  "use gRPC endpoint /omniphi.poseq.v1.Query/CheckpointAnchor",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQuerySettlementAnchor queries a settlement anchor by batch_hash hex.
func CmdQuerySettlementAnchor() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settlement-anchor [batch-hash-hex]",
		Short: "Query a settlement anchor by batch hash (64-char hex)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			batchHash := args[0]
			if len(batchHash) != 64 {
				return fmt.Errorf("batch_hash must be 64-char hex, got %d chars", len(batchHash))
			}
			return printJSON(clientCtx, map[string]string{
				"query":      "settlement-anchor",
				"batch_hash": batchHash,
				"note":       "use gRPC endpoint /omniphi.poseq.v1.Query/SettlementAnchor",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryEpochState queries epoch state by epoch number.
func CmdQueryEpochState() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch-state [epoch]",
		Short: "Query epoch state by epoch number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query": "epoch-state",
				"epoch": epoch,
				"note":  "use gRPC endpoint /omniphi.poseq.v1.Query/EpochState",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryCommitteeSnapshot queries the committee snapshot for an epoch.
func CmdQueryCommitteeSnapshot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "committee-snapshot [epoch]",
		Short: "Query the committee snapshot for an epoch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query": "committee-snapshot",
				"epoch": epoch,
				"note":  "use gRPC endpoint /omniphi.poseq.v1.Query/CommitteeSnapshot",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryLivenessEvent queries the liveness event for a (epoch, node_id) pair.
func CmdQueryLivenessEvent() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "liveness-event [epoch] [node-id-hex]",
		Short: "Query the liveness event for a node in an epoch",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			nodeID := args[1]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d chars", len(nodeID))
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query":   "liveness-event",
				"epoch":   epoch,
				"node_id": nodeID,
				"note":    "use gRPC endpoint /omniphi.poseq.v1.Query/LivenessEvent",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryPerformanceRecord queries the performance record for a (epoch, node_id) pair.
func CmdQueryPerformanceRecord() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "performance-record [epoch] [node-id-hex]",
		Short: "Query the performance record for a node in an epoch",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			nodeID := args[1]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d chars", len(nodeID))
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query":   "performance-record",
				"epoch":   epoch,
				"node_id": nodeID,
				"note":    "use gRPC endpoint /omniphi.poseq.v1.Query/PerformanceRecord",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryEpochPerformance queries all performance records for an epoch.
func CmdQueryEpochPerformance() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch-performance [epoch]",
		Short: "Query all performance records for an epoch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query": "epoch-performance",
				"epoch": epoch,
				"note":  "use gRPC endpoint /omniphi.poseq.v1.Query/EpochPerformance",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// ─── Phase 5: Economic Enforcement Layer query commands ───────────────────────

// CmdQueryOperatorBond queries the operator bond for an (operator-address, node-id) pair.
func CmdQueryOperatorBond() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator-bond [operator-address] [node-id-hex]",
		Short: "Query the operator bond for a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			operatorAddr := args[0]
			nodeID := args[1]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d chars", len(nodeID))
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query":            "operator-bond",
				"operator_address": operatorAddr,
				"node_id":          nodeID,
				"note":             "use gRPC endpoint /omniphi.poseq.v1.Query/OperatorBond",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryRewardScore queries the reward score for a (epoch, node-id) pair.
func CmdQueryRewardScore() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reward-score [epoch] [node-id-hex]",
		Short: "Query the reward score for a node in an epoch",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			nodeID := args[1]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d chars", len(nodeID))
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query":   "reward-score",
				"epoch":   epoch,
				"node_id": nodeID,
				"note":    "use gRPC endpoint /omniphi.poseq.v1.Query/RewardScore",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryEpochRewardScores queries all reward scores for a given epoch.
func CmdQueryEpochRewardScores() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epoch-reward-scores [epoch]",
		Short: "Query all reward scores for an epoch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			return printJSON(clientCtx, map[string]interface{}{
				"query": "epoch-reward-scores",
				"epoch": epoch,
				"note":  "use gRPC endpoint /omniphi.poseq.v1.Query/EpochRewardScores",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQuerySlashQueue queries all pending slash queue entries.
func CmdQuerySlashQueue() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slash-queue",
		Short: "Query all pending slash queue entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			return printJSON(clientCtx, map[string]string{
				"query": "slash-queue",
				"note":  "use gRPC endpoint /omniphi.poseq.v1.Query/SlashQueue",
			})
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// CmdQueryOperatorProfile queries the full economic profile for a node.
func CmdQueryOperatorProfile() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator-profile [node-id-hex]",
		Short: "Query the full economic profile for a node (sequencer + bond + reward score + slash entries)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			nodeID := args[0]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d chars", len(nodeID))
			}
			epochStr, _ := cmd.Flags().GetString("epoch")
			payload := map[string]interface{}{
				"query":   "operator-profile",
				"node_id": nodeID,
				"note":    "use gRPC endpoint /omniphi.poseq.v1.Query/OperatorProfile",
			}
			if epochStr != "" {
				epoch, err := strconv.ParseUint(epochStr, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid --epoch: %w", err)
				}
				payload["epoch"] = epoch
			}
			return printJSON(clientCtx, payload)
		},
	}
	cmd.Flags().String("epoch", "", "Epoch for which to fetch reward score (optional)")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func printJSON(clientCtx client.Context, v interface{}) error {
	bz, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return clientCtx.PrintString(string(bz) + "\n")
}
