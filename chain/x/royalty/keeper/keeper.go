package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/royalty/types"
)

// Keeper manages the state and business logic for x/royalty
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger
	authority    string

	bankKeeper    types.BankKeeper
	accountKeeper types.AccountKeeper

	// Optional keepers (nil-safe, set post-init)
	pocKeeper types.PocKeeper
}

// NewKeeper creates a new Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		authority:     authority,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
		pocKeeper:     nil,
	}
}

// SetPocKeeper sets the optional PoC keeper (called post-init)
func (k *Keeper) SetPocKeeper(pocKeeper types.PocKeeper) {
	k.pocKeeper = pocKeeper
}

func (k Keeper) GetAuthority() string { return k.authority }
func (k Keeper) Logger() log.Logger   { return k.logger }

// ========== RoyaltyToken CRUD ==========

func (k Keeper) GetNextTokenID(ctx context.Context) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyNextTokenID)
	if err != nil || bz == nil {
		return 1
	}
	return sdk.BigEndianToUint64(bz)
}

func (k Keeper) SetNextTokenID(ctx context.Context, id uint64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.KeyNextTokenID, sdk.Uint64ToBigEndian(id))
}

func (k Keeper) SetRoyaltyToken(ctx context.Context, token types.RoyaltyToken) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal royalty token: %w", err)
	}
	key := types.GetRoyaltyTokenKey(token.TokenID)
	if err := kvStore.Set(key, bz); err != nil {
		return err
	}

	// Update owner index
	ownerKey := types.GetOwnerIndexKey(token.Owner, token.TokenID)
	if err := kvStore.Set(ownerKey, []byte{}); err != nil {
		return err
	}

	// Update claim index
	claimKey := types.GetClaimIndexKey(token.ClaimID, token.TokenID)
	return kvStore.Set(claimKey, []byte{})
}

func (k Keeper) GetRoyaltyToken(ctx context.Context, tokenID uint64) (types.RoyaltyToken, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetRoyaltyTokenKey(tokenID))
	if err != nil || bz == nil {
		return types.RoyaltyToken{}, false
	}
	var token types.RoyaltyToken
	if err := json.Unmarshal(bz, &token); err != nil {
		k.logger.Error("failed to unmarshal royalty token", "error", err)
		return types.RoyaltyToken{}, false
	}
	return token, true
}

func (k Keeper) GetTokensByOwner(ctx context.Context, owner string) []types.RoyaltyToken {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetOwnerIndexPrefixKey(owner)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var tokens []types.RoyaltyToken
	for ; iter.Valid(); iter.Next() {
		// Extract token ID from key suffix
		key := iter.Key()
		if len(key) < 8 {
			continue
		}
		tokenID := sdk.BigEndianToUint64(key[len(key)-8:])
		token, found := k.GetRoyaltyToken(ctx, tokenID)
		if found {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func (k Keeper) GetTokensByClaim(ctx context.Context, claimID uint64) []types.RoyaltyToken {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetClaimIndexPrefixKey(claimID)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var tokens []types.RoyaltyToken
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) < 8 {
			continue
		}
		tokenID := sdk.BigEndianToUint64(key[len(key)-8:])
		token, found := k.GetRoyaltyToken(ctx, tokenID)
		if found {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// ========== Accumulated Royalties CRUD ==========

func (k Keeper) GetAccumulatedRoyalty(ctx context.Context, tokenID uint64) math.Int {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetAccumulatedRoyaltyKey(tokenID))
	if err != nil || bz == nil {
		return math.ZeroInt()
	}
	var ar types.AccumulatedRoyalty
	if err := json.Unmarshal(bz, &ar); err != nil {
		return math.ZeroInt()
	}
	return ar.Amount
}

func (k Keeper) SetAccumulatedRoyalty(ctx context.Context, tokenID uint64, amount math.Int) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	ar := types.AccumulatedRoyalty{TokenID: tokenID, Amount: amount}
	bz, err := json.Marshal(ar)
	if err != nil {
		return err
	}
	return kvStore.Set(types.GetAccumulatedRoyaltyKey(tokenID), bz)
}

// AddAccumulatedRoyalty adds to the accumulated royalties for a token
func (k Keeper) AddAccumulatedRoyalty(ctx context.Context, tokenID uint64, amount math.Int) error {
	current := k.GetAccumulatedRoyalty(ctx, tokenID)
	return k.SetAccumulatedRoyalty(ctx, tokenID, current.Add(amount))
}

// ========== Listing CRUD ==========

func (k Keeper) SetListing(ctx context.Context, listing types.Listing) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(listing)
	if err != nil {
		return err
	}
	return kvStore.Set(types.GetListingKey(listing.TokenID), bz)
}

func (k Keeper) GetListing(ctx context.Context, tokenID uint64) (types.Listing, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetListingKey(tokenID))
	if err != nil || bz == nil {
		return types.Listing{}, false
	}
	var listing types.Listing
	if err := json.Unmarshal(bz, &listing); err != nil {
		return types.Listing{}, false
	}
	return listing, true
}

func (k Keeper) DeleteListing(ctx context.Context, tokenID uint64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Delete(types.GetListingKey(tokenID))
}

// ========== Fractionalization ==========

func (k Keeper) SetFractionalToken(ctx context.Context, ft types.FractionalToken) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(ft)
	if err != nil {
		return err
	}
	key := types.GetFractionalizationKey(ft.ParentTokenID, ft.FractionIndex)
	return kvStore.Set(key, bz)
}

func (k Keeper) GetFractionalTokens(ctx context.Context, parentTokenID uint64) []types.FractionalToken {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetFractionalizationPrefixKey(parentTokenID)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var fractions []types.FractionalToken
	for ; iter.Valid(); iter.Next() {
		var ft types.FractionalToken
		if err := json.Unmarshal(iter.Value(), &ft); err != nil {
			continue
		}
		fractions = append(fractions, ft)
	}
	return fractions
}

// ========== Royalty Distribution Hook ==========

// OnContributionRewarded is called by PoC when a contribution receives a reward.
// It distributes the royalty portion to all token holders for that claim.
func (k Keeper) OnContributionRewarded(ctx context.Context, claimID uint64, rewardAmount math.Int) error {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return nil
	}

	tokens := k.GetTokensByClaim(ctx, claimID)
	if len(tokens) == 0 {
		return nil
	}

	for _, token := range tokens {
		if token.Status != types.TokenStatusActive {
			continue
		}

		// Calculate royalty for this token
		royaltyAmount := token.RoyaltyShare.MulInt(rewardAmount).TruncateInt()
		if royaltyAmount.IsZero() {
			continue
		}

		if err := k.AddAccumulatedRoyalty(ctx, token.TokenID, royaltyAmount); err != nil {
			k.logger.Error("failed to accumulate royalty",
				"token_id", token.TokenID,
				"amount", royaltyAmount,
				"error", err,
			)
			continue
		}
	}

	return nil
}

// OnContributionAccepted is called by the PoC module when a contribution is accepted by review.
// It mints a new RoyaltyToken for the contributor, assigning the default royalty share from params,
// and records the initial reward amount via OnContributionRewarded for proportional accumulation.
// This satisfies the types.RoyaltyKeeper interface expected by x/poc.
func (k Keeper) OnContributionAccepted(ctx context.Context, claimID uint64, owner string, rewardAmount math.Int) error {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return nil
	}

	// Skip if a token already exists for this claim (idempotent)
	if existing := k.GetTokensByClaim(ctx, claimID); len(existing) > 0 {
		// Token already minted — just accumulate the reward
		return k.OnContributionRewarded(ctx, claimID, rewardAmount)
	}

	// Mint a new royalty token with the minimum share (governance can transfer/fractionalize later)
	tokenID := k.GetNextTokenID(ctx)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	token := types.NewRoyaltyToken(tokenID, claimID, owner, params.MinRoyaltyShare, sdkCtx.BlockHeight())

	if err := k.SetRoyaltyToken(ctx, token); err != nil {
		return fmt.Errorf("failed to set royalty token: %w", err)
	}

	if err := k.SetNextTokenID(ctx, tokenID+1); err != nil {
		return fmt.Errorf("failed to increment next token ID: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"royalty_token_minted",
		sdk.NewAttribute("token_id", fmt.Sprintf("%d", tokenID)),
		sdk.NewAttribute("claim_id", fmt.Sprintf("%d", claimID)),
		sdk.NewAttribute("owner", owner),
		sdk.NewAttribute("royalty_share", params.MinRoyaltyShare.String()),
	))

	// Record initial reward in accumulated royalties
	if rewardAmount.IsPositive() {
		return k.OnContributionRewarded(ctx, claimID, rewardAmount)
	}
	return nil
}

// FreezeTokensForClaim freezes all tokens backed by a claim (called on fraud detection)
func (k Keeper) FreezeTokensForClaim(ctx context.Context, claimID uint64) error {
	tokens := k.GetTokensByClaim(ctx, claimID)
	for _, token := range tokens {
		token.Status = types.TokenStatusFrozen
		if err := k.SetRoyaltyToken(ctx, token); err != nil {
			k.logger.Error("failed to freeze token", "token_id", token.TokenID, "error", err)
		}
	}
	return nil
}
