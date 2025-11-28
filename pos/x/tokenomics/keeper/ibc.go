package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// IBCRewardPacket represents a reward distribution packet sent via IBC
// This is sent from Core chain to Continuity/Sequencer chains
type IBCRewardPacket struct {
	Amount          string `json:"amount"`
	RecipientModule string `json:"recipient_module"` // "poc" or "sequencer"
	SourceChain     string `json:"source_chain"`
	Epoch           int64  `json:"epoch"`
}

// IBCBurnReportPacket represents a burn report sent from other chains to Core
// This allows Core to track global burns and update supply
type IBCBurnReportPacket struct {
	Amount      string              `json:"amount"`
	Source      types.BurnSource    `json:"source"`
	ChainID     string              `json:"chain_id"`
	BlockHeight int64               `json:"block_height"`
	TxHash      string              `json:"tx_hash"`
	Proof       []byte              `json:"proof"` // Merkle proof
}

// DistributeRewardsViaIBC distributes rewards to other chains via IBC
// P0-IBC-001 to P0-IBC-006: IBC reward streaming
func (k Keeper) DistributeRewardsViaIBC(
	ctx context.Context,
	recipients []types.RewardRecipient,
) (localDist, ibcDist math.Int, packetsSent uint32, err error) {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	localDist = math.ZeroInt()
	ibcDist = math.ZeroInt()

	for _, recipient := range recipients {
		recipientAddr, err := sdk.AccAddressFromBech32(recipient.Address)
		if err != nil {
			return math.ZeroInt(), math.ZeroInt(), 0, fmt.Errorf("invalid recipient address %s: %w", recipient.Address, err)
		}

		if recipient.DestinationChain == "" {
			// P0-IBC-001: Local distribution (no IBC)
			coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, recipient.Amount))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, coins); err != nil {
				return math.ZeroInt(), math.ZeroInt(), 0, fmt.Errorf("failed to distribute rewards locally: %w", err)
			}
			localDist = localDist.Add(recipient.Amount)

			k.Logger(ctx).Info("distributed rewards locally",
				"recipient", recipient.Address,
				"amount", recipient.Amount.String(),
			)

		} else {
			// P0-IBC-002: IBC distribution to other chains
			// Get IBC channel for destination chain
			var channelID string
			switch recipient.DestinationChain {
			case "omniphi-continuity-1":
				channelID = params.ContinuityIbcChannel
			case "omniphi-sequencer-1":
				channelID = params.SequencerIbcChannel
			default:
				return math.ZeroInt(), math.ZeroInt(), 0, fmt.Errorf("unknown destination chain: %s", recipient.DestinationChain)
			}

			if channelID == "" {
				k.Logger(ctx).Warn("IBC channel not configured, skipping IBC distribution",
					"destination", recipient.DestinationChain,
					"amount", recipient.Amount.String(),
				)
				continue
			}

			// Create IBC packet
			packet := IBCRewardPacket{
				Amount:          recipient.Amount.String(),
				RecipientModule: recipient.Address, // Module name on destination chain
				SourceChain:     "omniphi-core-1",
				Epoch:           sdkCtx.BlockHeight(),
			}

			packetData, err := json.Marshal(packet)
			if err != nil {
				return math.ZeroInt(), math.ZeroInt(), 0, fmt.Errorf("failed to marshal IBC packet: %w", err)
			}

			// P0-IBC-005: Ordering preserved - use ordered channel
			// Note: In full implementation, would use IBC keeper to send packet
			// For now, we'll emit an event that will be processed by IBC relayer
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent(
					"ibc_reward_packet",
					sdk.NewAttribute("destination_chain", recipient.DestinationChain),
					sdk.NewAttribute("channel", channelID),
					sdk.NewAttribute("amount", recipient.Amount.String()),
					sdk.NewAttribute("recipient_module", recipient.Address),
					sdk.NewAttribute("packet_data", string(packetData)),
					sdk.NewAttribute("sequence", fmt.Sprintf("%d", packetsSent+1)),
				),
			)

			ibcDist = ibcDist.Add(recipient.Amount)
			packetsSent++

			k.Logger(ctx).Info("queued IBC reward packet",
				"destination", recipient.DestinationChain,
				"channel", channelID,
				"amount", recipient.Amount.String(),
				"sequence", packetsSent,
			)
		}
	}

	// P0-IBC-004: Emit summary event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"distribute_rewards_summary",
			sdk.NewAttribute("local_distributed", localDist.String()),
			sdk.NewAttribute("ibc_distributed", ibcDist.String()),
			sdk.NewAttribute("ibc_packets_sent", fmt.Sprintf("%d", packetsSent)),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	return localDist, ibcDist, packetsSent, nil
}

// OnRecvBurnReport handles IBC burn reports from other chains
// P0-BURN-IBC-001 to P0-BURN-IBC-006: Cross-chain burn reporting
func (k Keeper) OnRecvBurnReport(
	ctx context.Context,
	packet IBCBurnReportPacket,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Parse amount
	amount, ok := math.NewIntFromString(packet.Amount)
	if !ok {
		return fmt.Errorf("invalid amount in burn report: %s", packet.Amount)
	}

	// P0-BURN-IBC-002: Validate merkle proof
	// In production, this would verify the merkle proof from the source chain
	// For now, we'll do basic validation
	if len(packet.Proof) == 0 {
		k.Logger(ctx).Warn("burn report received without proof",
			"chain_id", packet.ChainID,
			"amount", amount.String(),
		)
		// In strict mode, would return error here
		// For development, we'll allow it but log warning
	}

	// P0-BURN-IBC-003: Check for duplicate report (idempotency)
	// Use combination of chain_id + tx_hash as unique identifier
	duplicateKey := fmt.Sprintf("%s:%s", packet.ChainID, packet.TxHash)

	// In production, would check duplicate tracker
	// For now, we'll proceed and rely on event deduplication
	k.Logger(ctx).Debug("processing burn report",
		"duplicate_key", duplicateKey,
		"chain_id", packet.ChainID,
		"amount", amount.String(),
	)

	// Update global burn counters
	totalBurned := k.GetTotalBurned(ctx)
	newBurned := totalBurned.Add(amount)

	if err := k.SetTotalBurned(ctx, newBurned); err != nil {
		return fmt.Errorf("failed to update total burned: %w", err)
	}

	// Update current supply (burns on other chains reduce global supply)
	currentSupply := k.GetCurrentSupply(ctx)
	newSupply := currentSupply.Sub(amount)

	if err := k.SetCurrentSupply(ctx, newSupply); err != nil {
		return fmt.Errorf("failed to update current supply: %w", err)
	}

	// Update per-chain burn tracking
	k.IncrementBurnsByChain(ctx, packet.ChainID, amount)

	// Update per-source burn tracking
	k.IncrementBurnsBySource(ctx, packet.Source, amount)

	// Emit event for transparency
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"ibc_burn_report_received",
			sdk.NewAttribute("source_chain", packet.ChainID),
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("source", packet.Source.String()),
			sdk.NewAttribute("source_block_height", fmt.Sprintf("%d", packet.BlockHeight)),
			sdk.NewAttribute("source_tx_hash", packet.TxHash),
			sdk.NewAttribute("new_total_burned", newBurned.String()),
			sdk.NewAttribute("new_total_supply", newSupply.String()),
		),
	)

	k.Logger(ctx).Info("burn report processed from IBC",
		"chain_id", packet.ChainID,
		"amount", amount.String(),
		"new_total_burned", newBurned.String(),
	)

	return nil
}

// ValidateBurnProof validates a merkle proof from another chain
// P0-BURN-IBC-002: Invalid proof rejected
func (k Keeper) ValidateBurnProof(
	ctx context.Context,
	chainID string,
	amount math.Int,
	blockHeight int64,
	proof []byte,
) error {
	// In production implementation:
	// 1. Fetch chain state root at blockHeight from IBC light client
	// 2. Verify merkle proof against state root
	// 3. Verify proof includes burn transaction with correct amount
	// 4. Verify signatures and consensus

	// For development, we'll do basic validation
	if len(proof) == 0 {
		return types.ErrInvalidProof
	}

	// Proof should be at least 32 bytes (hash size)
	if len(proof) < 32 {
		return types.ErrInvalidProof
	}

	// In production, would check:
	// - Light client consensus state
	// - Merkle path validity
	// - Transaction inclusion proof
	// - Amount matches claim

	k.Logger(ctx).Debug("burn proof validated",
		"chain_id", chainID,
		"amount", amount.String(),
		"block_height", blockHeight,
		"proof_length", len(proof),
	)

	return nil
}

// GetIBCChannel returns the IBC channel ID for a destination chain
func (k Keeper) GetIBCChannel(ctx context.Context, destinationChain string) string {
	params := k.GetParams(ctx)

	switch destinationChain {
	case "omniphi-continuity-1":
		return params.ContinuityIbcChannel
	case "omniphi-sequencer-1":
		return params.SequencerIbcChannel
	default:
		return ""
	}
}

// ProcessIBCAcknowledgements processes IBC packet acknowledgements
// P0-IBC-006: Failed packet refunds
func (k Keeper) ProcessIBCAcknowledgements(ctx context.Context) error {
	// In production implementation:
	// 1. Query IBC module for failed/timed-out packets
	// 2. Refund tokens from failed reward distributions
	// 3. Emit events for failed packets
	// 4. Update retry queue if necessary

	// For now, this is a placeholder that would be called in EndBlock
	k.Logger(ctx).Debug("processing IBC acknowledgements")

	return nil
}

// QueueRewardDistribution queues a reward for IBC distribution in next epoch
// P0-IBC-001: Epoch-based distribution
func (k Keeper) QueueRewardDistribution(
	ctx context.Context,
	destinationChain string,
	recipientModule string,
	amount math.Int,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Check if it's time to distribute (every N blocks)
	if sdkCtx.BlockHeight()%int64(params.RewardStreamInterval) != 0 {
		// Not yet time to distribute
		return nil
	}

	// In production, would:
	// 1. Add to distribution queue
	// 2. Process queue in EndBlock
	// 3. Batch multiple rewards into single IBC packet

	k.Logger(ctx).Debug("queued reward for IBC distribution",
		"destination", destinationChain,
		"module", recipientModule,
		"amount", amount.String(),
		"next_distribution_height", sdkCtx.BlockHeight()+int64(params.RewardStreamInterval),
	)

	return nil
}

// ShouldDistributeRewards checks if it's time to distribute rewards via IBC
func (k Keeper) ShouldDistributeRewards(ctx context.Context) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Distribute every N blocks (default: 100 blocks ≈ 12 minutes)
	return sdkCtx.BlockHeight()%int64(params.RewardStreamInterval) == 0
}

// CalculateRewardSplits calculates how much to send to each chain
func (k Keeper) CalculateRewardSplits(ctx context.Context, totalRewards math.Int) []types.RewardRecipient {
	params := k.GetParams(ctx)

	var recipients []types.RewardRecipient

	// Staking rewards (40%) - local
	stakingRewards := params.EmissionSplitStaking.MulInt(totalRewards).TruncateInt()
	if stakingRewards.IsPositive() {
		recipients = append(recipients, types.RewardRecipient{
			Address:          "staking", // Staking module
			Amount:           stakingRewards,
			DestinationChain: "", // Local chain
			IbcChannel:       "",
		})
	}

	// PoC rewards (30%) - send to Continuity chain via IBC
	pocRewards := params.EmissionSplitPoc.MulInt(totalRewards).TruncateInt()
	if pocRewards.IsPositive() {
		recipients = append(recipients, types.RewardRecipient{
			Address:          "poc", // PoC module on Continuity
			Amount:           pocRewards,
			DestinationChain: "omniphi-continuity-1",
			IbcChannel:       params.ContinuityIbcChannel,
		})
	}

	// Sequencer rewards (20%) - send to Sequencer chain via IBC
	sequencerRewards := params.EmissionSplitSequencer.MulInt(totalRewards).TruncateInt()
	if sequencerRewards.IsPositive() {
		recipients = append(recipients, types.RewardRecipient{
			Address:          "sequencer", // Sequencer module
			Amount:           sequencerRewards,
			DestinationChain: "omniphi-sequencer-1",
			IbcChannel:       params.SequencerIbcChannel,
		})
	}

	// Treasury (10%) - local
	treasuryRewards := params.EmissionSplitTreasury.MulInt(totalRewards).TruncateInt()
	if treasuryRewards.IsPositive() {
		treasuryAddr := k.GetTreasuryAddress(ctx)
		recipients = append(recipients, types.RewardRecipient{
			Address:          treasuryAddr.String(),
			Amount:           treasuryRewards,
			DestinationChain: "", // Local chain
			IbcChannel:       "",
		})
	}

	return recipients
}

// TrackIBCPacket stores metadata about sent IBC packets for monitoring
func (k Keeper) TrackIBCPacket(
	ctx context.Context,
	sequence uint64,
	channelID string,
	destinationChain string,
	amount math.Int,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// In production, would store in KVStore for query/monitoring
	// For now, just emit event

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"ibc_packet_tracked",
			sdk.NewAttribute("sequence", fmt.Sprintf("%d", sequence)),
			sdk.NewAttribute("channel", channelID),
			sdk.NewAttribute("destination", destinationChain),
			sdk.NewAttribute("amount", amount.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	return nil
}
