package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/uci/types"
)

type msgServer struct {
	k Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{k: k}
}

func (ms *msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != ms.k.authority {
		return nil, types.ErrInvalidAuthority
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}
	if err := ms.k.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}

func (ms *msgServer) RegisterAdapter(ctx context.Context, msg *types.MsgRegisterAdapter) (*types.MsgRegisterAdapterResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.Enabled {
		return nil, types.ErrModuleDisabled
	}

	// Check max adapters
	allAdapters := ms.k.GetAllAdapters(ctx)
	if int64(len(allAdapters)) >= params.MaxAdapters {
		return nil, types.ErrMaxAdaptersExceeded
	}

	// Charge registration fee
	if params.AdapterRegistrationFee.IsPositive() {
		ownerAddr, err := sdk.AccAddressFromBech32(msg.Owner)
		if err != nil {
			return nil, types.ErrInvalidAdapterConfig
		}
		fee := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, params.AdapterRegistrationFee))
		if err := ms.k.bankKeeper.SendCoinsFromAccountToModule(ctx, ownerAddr, types.ModuleName, fee); err != nil {
			return nil, fmt.Errorf("insufficient funds for adapter registration fee: %w", err)
		}
		// Burn registration fee
		if err := ms.k.bankKeeper.BurnCoins(ctx, types.ModuleName, fee); err != nil {
			ms.k.logger.Error("failed to burn registration fee", "error", err)
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	adapterID := ms.k.GetNextAdapterID(ctx)

	rewardShare := params.DefaultRewardShare
	if !msg.RewardShare.IsNil() && msg.RewardShare.IsPositive() {
		rewardShare = msg.RewardShare
	}

	adapter := types.NewAdapter(
		adapterID,
		msg.Name,
		msg.Owner,
		msg.SchemaCID,
		msg.OracleAllowlist,
		msg.NetworkType,
		sdkCtx.BlockHeight(),
		rewardShare,
		msg.Description,
	)

	if err := ms.k.SetAdapter(ctx, adapter); err != nil {
		return nil, err
	}
	if err := ms.k.SetNextAdapterID(ctx, adapterID+1); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"uci_adapter_registered",
			sdk.NewAttribute("adapter_id", fmt.Sprintf("%d", adapterID)),
			sdk.NewAttribute("name", msg.Name),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("network_type", msg.NetworkType),
		),
	)

	return &types.MsgRegisterAdapterResponse{AdapterID: adapterID}, nil
}

func (ms *msgServer) SuspendAdapter(ctx context.Context, msg *types.MsgSuspendAdapter) (*types.MsgSuspendAdapterResponse, error) {
	adapter, found := ms.k.GetAdapter(ctx, msg.AdapterID)
	if !found {
		return nil, types.ErrAdapterNotFound
	}

	// Only owner or governance authority can suspend
	if msg.Authority != adapter.Owner && msg.Authority != ms.k.authority {
		return nil, types.ErrAdapterOwnerMismatch
	}

	adapter.Status = types.AdapterStatusSuspended
	if err := ms.k.SetAdapter(ctx, adapter); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"uci_adapter_suspended",
			sdk.NewAttribute("adapter_id", fmt.Sprintf("%d", msg.AdapterID)),
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgSuspendAdapterResponse{}, nil
}

func (ms *msgServer) SubmitDePINContribution(ctx context.Context, msg *types.MsgSubmitDePINContribution) (*types.MsgSubmitDePINContributionResponse, error) {
	pocID, err := ms.k.ProcessDePINContribution(
		ctx,
		msg.AdapterID,
		msg.ExternalID,
		msg.Contributor,
		msg.DataHash,
		msg.DataURI,
	)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"uci_contribution_submitted",
			sdk.NewAttribute("adapter_id", fmt.Sprintf("%d", msg.AdapterID)),
			sdk.NewAttribute("external_id", msg.ExternalID),
			sdk.NewAttribute("poc_contribution_id", fmt.Sprintf("%d", pocID)),
			sdk.NewAttribute("contributor", msg.Contributor),
		),
	)

	return &types.MsgSubmitDePINContributionResponse{
		PocContributionID: pocID,
	}, nil
}

func (ms *msgServer) SubmitOracleAttestation(ctx context.Context, msg *types.MsgSubmitOracleAttestation) (*types.MsgSubmitOracleAttestationResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.Enabled {
		return nil, types.ErrModuleDisabled
	}

	adapter, found := ms.k.GetAdapter(ctx, msg.AdapterID)
	if !found {
		return nil, types.ErrAdapterNotFound
	}

	if !ms.k.IsOracleAuthorized(adapter, msg.OracleAddress) {
		return nil, types.ErrOracleNotAuthorized
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	attestation := types.OracleAttestation{
		AdapterID:         msg.AdapterID,
		BatchID:           msg.BatchID,
		OracleAddress:     msg.OracleAddress,
		AttestationHash:   msg.AttestationHash,
		ContributionCount: msg.ContributionCount,
		AttestedAtHeight:  sdkCtx.BlockHeight(),
		Signature:         msg.Signature,
	}

	if err := ms.k.SetOracleAttestation(ctx, attestation); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"uci_oracle_attestation",
			sdk.NewAttribute("adapter_id", fmt.Sprintf("%d", msg.AdapterID)),
			sdk.NewAttribute("batch_id", msg.BatchID),
			sdk.NewAttribute("oracle", msg.OracleAddress),
			sdk.NewAttribute("contributions", fmt.Sprintf("%d", msg.ContributionCount)),
		),
	)

	return &types.MsgSubmitOracleAttestationResponse{}, nil
}

func (ms *msgServer) UpdateAdapterConfig(ctx context.Context, msg *types.MsgUpdateAdapterConfig) (*types.MsgUpdateAdapterConfigResponse, error) {
	adapter, found := ms.k.GetAdapter(ctx, msg.AdapterID)
	if !found {
		return nil, types.ErrAdapterNotFound
	}

	if msg.Owner != adapter.Owner {
		return nil, types.ErrAdapterOwnerMismatch
	}

	if len(msg.OracleAllowlist) > 0 {
		adapter.OracleAllowlist = msg.OracleAllowlist
	}
	if msg.SchemaCID != "" {
		adapter.SchemaCID = msg.SchemaCID
	}
	if !msg.RewardShare.IsNil() && msg.RewardShare.IsPositive() {
		adapter.RewardShare = msg.RewardShare
	}

	if err := ms.k.SetAdapter(ctx, adapter); err != nil {
		return nil, err
	}

	return &types.MsgUpdateAdapterConfigResponse{}, nil
}

// ensure math is used
var _ = math.ZeroInt
