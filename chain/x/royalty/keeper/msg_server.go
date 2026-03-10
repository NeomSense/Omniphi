package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/royalty/types"
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

func (ms *msgServer) TokenizeRoyalty(ctx context.Context, msg *types.MsgTokenizeRoyalty) (*types.MsgTokenizeRoyaltyResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.Enabled {
		return nil, types.ErrModuleDisabled
	}

	// Validate royalty share bounds
	if msg.RoyaltyShare.LT(params.MinRoyaltyShare) || msg.RoyaltyShare.GT(params.MaxRoyaltyShare) {
		return nil, types.ErrInvalidRoyaltyShare
	}

	// Verify the contribution has been rewarded (if PoC keeper available)
	// Only rewarded contributions can be tokenized for royalty streams
	if ms.k.pocKeeper != nil {
		if !ms.k.pocKeeper.IsContributionRewarded(ctx, msg.ClaimID) {
			return nil, types.ErrClaimNotFound
		}
	}

	// Check no existing token for this claim with same owner
	existingTokens := ms.k.GetTokensByClaim(ctx, msg.ClaimID)
	totalShare := msg.RoyaltyShare
	for _, t := range existingTokens {
		totalShare = totalShare.Add(t.RoyaltyShare)
	}
	if totalShare.GT(params.MaxRoyaltyShare) {
		return nil, types.ErrInvalidRoyaltyShare
	}

	// Mint the token
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	tokenID := ms.k.GetNextTokenID(ctx)
	token := types.NewRoyaltyToken(tokenID, msg.ClaimID, msg.Creator, msg.RoyaltyShare, sdkCtx.BlockHeight())
	token.Metadata = msg.Metadata

	if err := ms.k.SetRoyaltyToken(ctx, token); err != nil {
		return nil, err
	}
	if err := ms.k.SetNextTokenID(ctx, tokenID+1); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"royalty_token_created",
			sdk.NewAttribute("token_id", fmt.Sprintf("%d", tokenID)),
			sdk.NewAttribute("claim_id", fmt.Sprintf("%d", msg.ClaimID)),
			sdk.NewAttribute("owner", msg.Creator),
			sdk.NewAttribute("royalty_share", msg.RoyaltyShare.String()),
		),
	)

	return &types.MsgTokenizeRoyaltyResponse{TokenID: tokenID}, nil
}

func (ms *msgServer) TransferToken(ctx context.Context, msg *types.MsgTransferToken) (*types.MsgTransferTokenResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.TransferEnabled {
		return nil, types.ErrTokenNotTransferable
	}

	token, found := ms.k.GetRoyaltyToken(ctx, msg.TokenID)
	if !found {
		return nil, types.ErrTokenNotFound
	}

	if token.Owner != msg.Sender {
		return nil, types.ErrNotTokenOwner
	}

	if token.Status == types.TokenStatusFrozen {
		return nil, types.ErrTokenFrozen
	}

	// Remove old owner index
	kvStore := ms.k.storeService.OpenKVStore(ctx)
	oldOwnerKey := types.GetOwnerIndexKey(token.Owner, token.TokenID)
	_ = kvStore.Delete(oldOwnerKey)

	// Update owner
	token.Owner = msg.Recipient
	if err := ms.k.SetRoyaltyToken(ctx, token); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"royalty_token_transferred",
			sdk.NewAttribute("token_id", fmt.Sprintf("%d", msg.TokenID)),
			sdk.NewAttribute("from", msg.Sender),
			sdk.NewAttribute("to", msg.Recipient),
		),
	)

	return &types.MsgTransferTokenResponse{}, nil
}

func (ms *msgServer) ClaimRoyalties(ctx context.Context, msg *types.MsgClaimRoyalties) (*types.MsgClaimRoyaltiesResponse, error) {
	token, found := ms.k.GetRoyaltyToken(ctx, msg.TokenID)
	if !found {
		return nil, types.ErrTokenNotFound
	}

	if token.Owner != msg.Owner {
		return nil, types.ErrNotTokenOwner
	}

	if token.Status == types.TokenStatusFrozen {
		return nil, types.ErrTokenFrozen
	}

	accumulated := ms.k.GetAccumulatedRoyalty(ctx, msg.TokenID)
	if accumulated.IsZero() {
		return nil, types.ErrNoAccumulatedRoyalties
	}

	// Send royalties from module to owner
	params := ms.k.GetParams(ctx)
	ownerAddr, err := sdk.AccAddressFromBech32(token.Owner)
	if err != nil {
		return nil, types.ErrInvalidAddress
	}

	coins := sdk.NewCoins(sdk.NewCoin(params.RewardDenom, accumulated))
	if err := ms.k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, ownerAddr, coins); err != nil {
		return nil, err
	}

	// Update token payouts
	token.TotalPayouts = token.TotalPayouts.Add(accumulated)
	if err := ms.k.SetRoyaltyToken(ctx, token); err != nil {
		return nil, err
	}

	// Reset accumulated
	if err := ms.k.SetAccumulatedRoyalty(ctx, msg.TokenID, math.ZeroInt()); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"royalty_claimed",
			sdk.NewAttribute("token_id", fmt.Sprintf("%d", msg.TokenID)),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("amount", accumulated.String()),
		),
	)

	return &types.MsgClaimRoyaltiesResponse{Amount: accumulated}, nil
}

func (ms *msgServer) FractionalizeToken(ctx context.Context, msg *types.MsgFractionalizeToken) (*types.MsgFractionalizeTokenResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.FractionalizationEnabled {
		return nil, types.ErrModuleDisabled
	}

	token, found := ms.k.GetRoyaltyToken(ctx, msg.TokenID)
	if !found {
		return nil, types.ErrTokenNotFound
	}
	if token.Owner != msg.Owner {
		return nil, types.ErrNotTokenOwner
	}
	if token.IsFractionalized {
		return nil, types.ErrTokenAlreadyFractionalized
	}
	if int64(msg.Fractions) > params.MaxFractions {
		return nil, types.ErrMaxFractionsExceeded
	}

	// Create fractions — equal share each
	sharePerFraction := token.RoyaltyShare.Quo(math.LegacyNewDec(int64(msg.Fractions)))

	for i := uint32(0); i < msg.Fractions; i++ {
		ft := types.FractionalToken{
			ParentTokenID: token.TokenID,
			FractionIndex: i,
			Owner:         token.Owner,
			Share:         sharePerFraction,
			TotalPayouts:  math.ZeroInt(),
		}
		if err := ms.k.SetFractionalToken(ctx, ft); err != nil {
			return nil, err
		}
	}

	// Mark parent as fractionalized
	token.IsFractionalized = true
	token.FractionCount = msg.Fractions
	if err := ms.k.SetRoyaltyToken(ctx, token); err != nil {
		return nil, err
	}

	return &types.MsgFractionalizeTokenResponse{}, nil
}

func (ms *msgServer) ListToken(ctx context.Context, msg *types.MsgListToken) (*types.MsgListTokenResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.TransferEnabled {
		return nil, types.ErrTokenNotTransferable
	}

	token, found := ms.k.GetRoyaltyToken(ctx, msg.TokenID)
	if !found {
		return nil, types.ErrTokenNotFound
	}
	if token.Owner != msg.Seller {
		return nil, types.ErrNotTokenOwner
	}
	if token.Status != types.TokenStatusActive {
		return nil, types.ErrTokenFrozen
	}

	_, exists := ms.k.GetListing(ctx, msg.TokenID)
	if exists {
		return nil, types.ErrListingAlreadyExists
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	listing := types.Listing{
		TokenID:  msg.TokenID,
		Seller:   msg.Seller,
		AskPrice: msg.AskPrice,
		Denom:    msg.Denom,
		ListedAt: sdkCtx.BlockHeight(),
	}

	if err := ms.k.SetListing(ctx, listing); err != nil {
		return nil, err
	}

	return &types.MsgListTokenResponse{}, nil
}

func (ms *msgServer) BuyToken(ctx context.Context, msg *types.MsgBuyToken) (*types.MsgBuyTokenResponse, error) {
	params := ms.k.GetParams(ctx)
	if !params.TransferEnabled {
		return nil, types.ErrTokenNotTransferable
	}

	listing, found := ms.k.GetListing(ctx, msg.TokenID)
	if !found {
		return nil, types.ErrListingNotFound
	}

	token, found := ms.k.GetRoyaltyToken(ctx, msg.TokenID)
	if !found {
		return nil, types.ErrTokenNotFound
	}

	buyerAddr, err := sdk.AccAddressFromBech32(msg.Buyer)
	if err != nil {
		return nil, types.ErrInvalidAddress
	}
	sellerAddr, err := sdk.AccAddressFromBech32(listing.Seller)
	if err != nil {
		return nil, types.ErrInvalidAddress
	}

	// Calculate marketplace fee
	feeAmount := params.MarketplaceFee.MulInt(listing.AskPrice).TruncateInt()
	sellerAmount := listing.AskPrice.Sub(feeAmount)

	// Transfer payment: buyer -> seller (minus fee)
	paymentCoins := sdk.NewCoins(sdk.NewCoin(listing.Denom, sellerAmount))
	if err := ms.k.bankKeeper.SendCoins(ctx, buyerAddr, sellerAddr, paymentCoins); err != nil {
		return nil, types.ErrInsufficientFunds
	}

	// Send fee to module (burn or treasury)
	if feeAmount.IsPositive() {
		feeCoins := sdk.NewCoins(sdk.NewCoin(listing.Denom, feeAmount))
		if err := ms.k.bankKeeper.SendCoinsFromAccountToModule(ctx, buyerAddr, types.ModuleName, feeCoins); err != nil {
			return nil, err
		}
		// Burn the fee
		if err := ms.k.bankKeeper.BurnCoins(ctx, types.ModuleName, feeCoins); err != nil {
			ms.k.logger.Error("failed to burn marketplace fee", "error", err)
		}
	}

	// Transfer token ownership
	kvStore := ms.k.storeService.OpenKVStore(ctx)
	oldOwnerKey := types.GetOwnerIndexKey(token.Owner, token.TokenID)
	_ = kvStore.Delete(oldOwnerKey)

	token.Owner = msg.Buyer
	if err := ms.k.SetRoyaltyToken(ctx, token); err != nil {
		return nil, err
	}

	// Remove listing
	if err := ms.k.DeleteListing(ctx, msg.TokenID); err != nil {
		return nil, err
	}

	return &types.MsgBuyTokenResponse{}, nil
}

func (ms *msgServer) DelistToken(ctx context.Context, msg *types.MsgDelistToken) (*types.MsgDelistTokenResponse, error) {
	listing, found := ms.k.GetListing(ctx, msg.TokenID)
	if !found {
		return nil, types.ErrListingNotFound
	}
	if listing.Seller != msg.Seller {
		return nil, types.ErrNotTokenOwner
	}

	if err := ms.k.DeleteListing(ctx, msg.TokenID); err != nil {
		return nil, err
	}

	return &types.MsgDelistTokenResponse{}, nil
}

// ensure fmt is used
var _ = fmt.Sprintf
