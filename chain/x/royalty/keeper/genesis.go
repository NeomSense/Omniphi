package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/royalty/types"
)

func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) error {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}

	for _, token := range gs.Tokens {
		if err := k.SetRoyaltyToken(ctx, token); err != nil {
			return err
		}
	}

	if gs.NextTokenID > 0 {
		if err := k.SetNextTokenID(ctx, gs.NextTokenID); err != nil {
			return err
		}
	}

	return nil
}

func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	// Export all tokens
	var tokens []types.RoyaltyToken
	// Iterate all tokens by iterating the token prefix
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.KeyPrefixRoyaltyToken, append(types.KeyPrefixRoyaltyToken, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF))
	if err == nil {
		defer iter.Close()
		for ; iter.Valid(); iter.Next() {
			var token types.RoyaltyToken
			if err := token.Unmarshal(iter.Value()); err == nil {
				tokens = append(tokens, token)
			}
		}
	}

	return &types.GenesisState{
		Params:      k.GetParams(ctx),
		Tokens:      tokens,
		NextTokenID: k.GetNextTokenID(ctx),
	}
}
