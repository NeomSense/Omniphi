package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"

	"pos/x/poseq/types"
)

// GetTxCmd returns the CLI transaction root command for x/poseq.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "poseq",
		Short:                      "Transaction commands for the poseq module",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdRegisterSequencer(),
		CmdDeactivateSequencer(),
		CmdAnchorSettlement(),
		CmdSubmitCommitteeSnapshot(),
		CmdTransitionSequencer(),
		// Phase 5: Economic Enforcement Layer
		CmdDeclareOperatorBond(),
		CmdWithdrawOperatorBond(),
	)

	return cmd
}

// CmdRegisterSequencer registers a new sequencer on-chain.
func CmdRegisterSequencer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-sequencer [node-id-hex] [pubkey-hex] [moniker] [epoch]",
		Short: "Register a new PoSeq sequencer",
		Long:  "Register a new sequencer with the given node ID, Ed25519 public key, moniker, and starting epoch.",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			nodeID := args[0]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d", len(nodeID))
			}
			if _, err := hex.DecodeString(nodeID); err != nil {
				return fmt.Errorf("invalid node_id hex: %w", err)
			}

			pubKey := args[1]
			if len(pubKey) != 64 {
				return fmt.Errorf("pubkey must be 64-char hex, got %d", len(pubKey))
			}
			if _, err := hex.DecodeString(pubKey); err != nil {
				return fmt.Errorf("invalid pubkey hex: %w", err)
			}

			moniker := args[2]
			if len(moniker) > 64 {
				return fmt.Errorf("moniker must be <= 64 chars, got %d", len(moniker))
			}

			epoch, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}

			msg := &types.MsgRegisterSequencer{
				Sender:    clientCtx.GetFromAddress().String(),
				NodeID:    nodeID,
				PublicKey: pubKey,
				Moniker:   moniker,
				Epoch:     epoch,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdDeactivateSequencer deactivates a sequencer the sender operates.
func CmdDeactivateSequencer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deactivate-sequencer [node-id-hex] [reason]",
		Short: "Deactivate a sequencer you operate",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			nodeID := args[0]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d", len(nodeID))
			}

			msg := &types.MsgDeactivateSequencer{
				Sender: clientCtx.GetFromAddress().String(),
				NodeID: nodeID,
				Reason: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdSubmitCommitteeSnapshot submits a pre-built committee snapshot on-chain.
// The snapshot JSON file must contain a CommitteeSnapshot with a valid hash.
// Governance authority only.
func CmdSubmitCommitteeSnapshot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-committee-snapshot [epoch] [snapshot-json-file]",
		Short: "Submit a committee snapshot for an epoch (governance only)",
		Long:  "Submit a committee snapshot JSON file for the given epoch. The file must contain a valid CommitteeSnapshot with a pre-computed snapshot_hash.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			epoch, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}

			import_bz, err := os.ReadFile(args[1])
			if err != nil {
				return fmt.Errorf("reading snapshot file: %w", err)
			}

			var snap types.CommitteeSnapshot
			if err := json.Unmarshal(import_bz, &snap); err != nil {
				return fmt.Errorf("parsing snapshot JSON: %w", err)
			}
			if snap.Epoch != epoch {
				return fmt.Errorf("snapshot epoch %d does not match argument epoch %d", snap.Epoch, epoch)
			}

			msg := &types.MsgSubmitCommitteeSnapshot{
				Authority: clientCtx.GetFromAddress().String(),
				Snapshot:  snap,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdTransitionSequencer performs an explicit FSM lifecycle transition.
// Governance authority only.
func CmdTransitionSequencer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transition-sequencer [node-id-hex] [to-status] [reason] [epoch]",
		Short: "Transition a sequencer to a new lifecycle status (governance only)",
		Long:  "Allowed statuses: Active, Suspended, Jailed, Retired. FSM rules are enforced on-chain.",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			nodeID := args[0]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d", len(nodeID))
			}
			if _, err := hex.DecodeString(nodeID); err != nil {
				return fmt.Errorf("invalid node_id hex: %w", err)
			}

			toStatus := types.SequencerStatus(args[1])
			switch toStatus {
			case types.SequencerStatusActive,
				types.SequencerStatusSuspended,
				types.SequencerStatusJailed,
				types.SequencerStatusRetired:
				// valid
			default:
				return fmt.Errorf("invalid to-status %q: must be Active, Suspended, Jailed, or Retired", args[1])
			}

			epoch, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}

			msg := &types.MsgTransitionSequencer{
				Authority: clientCtx.GetFromAddress().String(),
				NodeID:    nodeID,
				ToStatus:  toStatus,
				Reason:    args[2],
				Epoch:     epoch,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdDeclareOperatorBond declares an operator bond for a node (Phase 5 — no token movement).
func CmdDeclareOperatorBond() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "declare-operator-bond [node-id-hex] [amount] [denom] [epoch]",
		Short: "Declare an operator bond for a PoSeq node (declaration only — no tokens moved in Phase 5)",
		Long:  "Associates the sender's operator address with a bond declaration for the given node. No tokens are transferred in Phase 5.",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			nodeID := args[0]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d", len(nodeID))
			}
			if _, err := hex.DecodeString(nodeID); err != nil {
				return fmt.Errorf("invalid node_id hex: %w", err)
			}

			amount, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}
			if amount == 0 {
				return fmt.Errorf("amount must be > 0")
			}

			denom := args[2]
			if denom == "" {
				return fmt.Errorf("denom must not be empty")
			}

			epoch, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}

			msg := &types.MsgDeclareOperatorBond{
				OperatorAddress: clientCtx.GetFromAddress().String(),
				NodeID:          nodeID,
				BondAmount:      amount,
				BondDenom:       denom,
				Epoch:           epoch,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdWithdrawOperatorBond withdraws the operator's bond declaration for a node.
func CmdWithdrawOperatorBond() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw-operator-bond [node-id-hex] [epoch]",
		Short: "Withdraw the operator bond declaration for a PoSeq node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			nodeID := args[0]
			if len(nodeID) != 64 {
				return fmt.Errorf("node_id must be 64-char hex, got %d", len(nodeID))
			}
			if _, err := hex.DecodeString(nodeID); err != nil {
				return fmt.Errorf("invalid node_id hex: %w", err)
			}

			epoch, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}

			msg := &types.MsgWithdrawOperatorBond{
				OperatorAddress: clientCtx.GetFromAddress().String(),
				NodeID:          nodeID,
				Epoch:           epoch,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdAnchorSettlement anchors a settlement record on-chain.
func CmdAnchorSettlement() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "anchor-settlement [settlement-hash] [batch-hash] [post-state-root] [receipt-hash] [epoch] [seq-num] [settled-count] [failed-count]",
		Short: "Anchor a settlement result on-chain",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Validate all hex fields are 64-char
			hexFields := []struct {
				name string
				val  string
			}{
				{"settlement_hash", args[0]},
				{"batch_hash", args[1]},
				{"post_state_root", args[2]},
				{"receipt_hash", args[3]},
			}
			for _, f := range hexFields {
				if len(f.val) != 64 {
					return fmt.Errorf("%s must be 64-char hex, got %d", f.name, len(f.val))
				}
				if _, err := hex.DecodeString(f.val); err != nil {
					return fmt.Errorf("invalid %s hex: %w", f.name, err)
				}
			}

			epoch, err := strconv.ParseUint(args[4], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch: %w", err)
			}
			seqNum, err := strconv.ParseUint(args[5], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid sequence_number: %w", err)
			}
			settled64, err := strconv.ParseUint(args[6], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid settled_count: %w", err)
			}
			failed64, err := strconv.ParseUint(args[7], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid failed_count: %w", err)
			}

			msg := &types.MsgAnchorSettlement{
				Sender: clientCtx.GetFromAddress().String(),
				Anchor: types.SettlementAnchorRecord{
					SettlementHash:       args[0],
					BatchHash:            args[1],
					PostStateRoot:        args[2],
					ExecutionReceiptHash: args[3],
					Epoch:                epoch,
					SequenceNumber:       seqNum,
					SettledCount:         uint32(settled64),
					FailedCount:          uint32(failed64),
					SubmitterAddress:     clientCtx.GetFromAddress().String(),
				},
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
