package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// msgServer is a wrapper around the Keeper that implements the MsgServer interface
type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// UpdateParams updates the tokenomics module parameters
// P0-GOV-001 to P0-GOV-008: Governance controls
// P0-PERM-002: Only governance can update parameters
func (ms msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// P0-PERM-002: Validate authority
	if msg.Authority != ms.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	// P0-INF-003 to P0-INF-005: Validate parameters (including inflation bounds)
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Set the new parameters
	if err := ms.SetParams(ctx, msg.Params); err != nil {
		return nil, fmt.Errorf("failed to set parameters: %w", err)
	}

	// OBS-001: Emit parameter update event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"update_params",
			sdk.NewAttribute("authority", msg.Authority),
			sdk.NewAttribute("inflation_rate", msg.Params.InflationRate.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)

	ms.Logger(ctx).Info("parameters updated via governance",
		"authority", msg.Authority,
		"inflation_rate", msg.Params.InflationRate.String(),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// MintTokens mints new OMNI tokens
// P0-CAP-001 to P0-CAP-006: Hard cap enforcement
// P0-PERM-001: Only inflation module (via governance) can mint
func (ms msgServer) MintTokens(goCtx context.Context, msg *types.MsgMintTokens) (*types.MsgMintTokensResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// P0-PERM-001: Validate authority (only inflation/governance module can mint)
	if msg.Authority != ms.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	// Parse recipient address
	recipient, err := sdk.AccAddressFromBech32(msg.Recipient)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient address: %w", err)
	}

	// Execute mint with cap enforcement
	if err := ms.Keeper.MintTokens(ctx, msg.Amount, recipient, msg.Reason); err != nil {
		return nil, err
	}

	// Calculate new totals for response
	newTotalSupply := ms.GetCurrentSupply(ctx)
	newTotalMinted := ms.GetTotalMinted(ctx)
	params := ms.GetParams(ctx)
	remainingMintable := params.TotalSupplyCap.Sub(newTotalSupply)

	return &types.MsgMintTokensResponse{
		NewTotalSupply:     newTotalSupply,
		NewTotalMinted:     newTotalMinted,
		RemainingMintable:  remainingMintable,
	}, nil
}

// BurnTokens burns OMNI tokens with treasury redirect
// P0-BURN-001 to P0-BURN-008: Burn correctness
func (ms msgServer) BurnTokens(goCtx context.Context, msg *types.MsgBurnTokens) (*types.MsgBurnTokensResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Parse burner address
	burner, err := sdk.AccAddressFromBech32(msg.Burner)
	if err != nil {
		return nil, fmt.Errorf("invalid burner address: %w", err)
	}

	// Execute burn with treasury redirect
	amountBurned, amountToTreasury, err := ms.Keeper.BurnTokens(ctx, burner, msg.Amount, msg.Source, msg.ChainId)
	if err != nil {
		return nil, err
	}

	// Calculate new totals for response
	newTotalSupply := ms.GetCurrentSupply(ctx)
	newTotalBurned := ms.GetTotalBurned(ctx)

	return &types.MsgBurnTokensResponse{
		NewTotalSupply:   newTotalSupply,
		NewTotalBurned:   newTotalBurned,
		AmountBurned:     amountBurned,
		AmountToTreasury: amountToTreasury,
	}, nil
}

// DistributeRewards distributes inflationary rewards across staking, PoC, sequencer, and treasury
// P0-DIST-001 to P0-DIST-005: Distribution invariants
// P0-IBC-001 to P0-IBC-006: IBC reward streaming
func (ms msgServer) DistributeRewards(goCtx context.Context, msg *types.MsgDistributeRewards) (*types.MsgDistributeRewardsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// P0-PERM-001: Validate authority (only inflation module can distribute)
	if msg.Authority != ms.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	// P0-DIST-001: Validate total rewards don't exceed budget
	var totalDistributed math.Int
	for _, recipient := range msg.Recipients {
		totalDistributed = totalDistributed.Add(recipient.Amount)
	}

	if totalDistributed.GT(msg.TotalRewards) {
		return nil, types.ErrEmissionBudgetExceeded
	}

	// Distribute rewards locally and via IBC
	localDist := math.ZeroInt()
	ibcDist := math.ZeroInt()
	var ibcPacketsSent uint32

	for _, recipient := range msg.Recipients {
		recipientAddr, err := sdk.AccAddressFromBech32(recipient.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid recipient address %s: %w", recipient.Address, err)
		}

		if recipient.DestinationChain == "" {
			// Local distribution
			coins := sdk.NewCoins(sdk.NewCoin(types.BondDenom, recipient.Amount))
			if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, coins); err != nil {
				return nil, fmt.Errorf("failed to distribute rewards to %s: %w", recipient.Address, err)
			}
			localDist = localDist.Add(recipient.Amount)
		} else {
			// IBC distribution (P0-IBC-001 to P0-IBC-006)
			// Note: Full IBC implementation in ibc.go
			// For now, queue for IBC transmission
			ibcDist = ibcDist.Add(recipient.Amount)
			ibcPacketsSent++

			// Emit event for IBC reward (actual packet send in EndBlock)
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"ibc_reward_queued",
					sdk.NewAttribute("destination_chain", recipient.DestinationChain),
					sdk.NewAttribute("channel", recipient.IbcChannel),
					sdk.NewAttribute("amount", recipient.Amount.String()),
					sdk.NewAttribute("recipient", recipient.Address),
				),
			)
		}
	}

	// OBS-001: Emit distribution event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"distribute_rewards",
			sdk.NewAttribute("total_rewards", msg.TotalRewards.String()),
			sdk.NewAttribute("local_distributed", localDist.String()),
			sdk.NewAttribute("ibc_distributed", ibcDist.String()),
			sdk.NewAttribute("ibc_packets_sent", fmt.Sprintf("%d", ibcPacketsSent)),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", msg.BlockHeight)),
		),
	)

	ms.Logger(ctx).Info("rewards distributed",
		"total", msg.TotalRewards.String(),
		"local", localDist.String(),
		"ibc", ibcDist.String(),
		"packets", ibcPacketsSent,
	)

	return &types.MsgDistributeRewardsResponse{
		TotalDistributed: totalDistributed,
		LocalDistributed: localDist,
		IbcDistributed:   ibcDist,
		IbcPacketsSent:   ibcPacketsSent,
	}, nil
}

// ReportBurn reports a burn event from another chain (via IBC)
// P0-BURN-IBC-001 to P0-BURN-IBC-006: Cross-chain burn reporting
func (ms msgServer) ReportBurn(goCtx context.Context, msg *types.MsgReportBurn) (*types.MsgReportBurnResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// P0-BURN-IBC-002: Validate IBC proof (simplified for now)
	// In full implementation, this would verify merkle proof from source chain
	if len(msg.Proof) == 0 {
		return nil, types.ErrInvalidProof
	}

	// P0-BURN-IBC-003: Check for duplicate report (idempotency)
	// Use tx_hash as unique identifier
	// In production, would store processed tx hashes to prevent duplicates

	// Update global burn counters
	totalBurned := ms.GetTotalBurned(ctx)
	newBurned := totalBurned.Add(msg.Amount)

	if err := ms.SetTotalBurned(ctx, newBurned); err != nil {
		return nil, fmt.Errorf("failed to update total burned: %w", err)
	}

	// Update current supply (burns on other chains reduce global supply)
	currentSupply := ms.GetCurrentSupply(ctx)
	newSupply := currentSupply.Sub(msg.Amount)

	if err := ms.SetCurrentSupply(ctx, newSupply); err != nil {
		return nil, fmt.Errorf("failed to update current supply: %w", err)
	}

	// Update per-chain burn tracking
	ms.IncrementBurnsByChain(ctx, msg.ChainId, msg.Amount)

	// OBS-001: Emit burn report event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"report_burn",
			sdk.NewAttribute("chain_id", msg.ChainId),
			sdk.NewAttribute("amount", msg.Amount.String()),
			sdk.NewAttribute("source", msg.Source.String()),
			sdk.NewAttribute("source_block_height", fmt.Sprintf("%d", msg.BlockHeight)),
			sdk.NewAttribute("source_tx_hash", msg.TxHash),
			sdk.NewAttribute("new_total_burned", newBurned.String()),
		),
	)

	ms.Logger(ctx).Info("burn reported from remote chain",
		"chain_id", msg.ChainId,
		"amount", msg.Amount.String(),
		"source", msg.Source.String(),
		"new_total_burned", newBurned.String(),
	)

	return &types.MsgReportBurnResponse{
		Recorded:        true,
		NewTotalBurned:  newBurned,
	}, nil
}
