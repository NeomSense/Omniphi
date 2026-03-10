package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	ModuleName = "royalty"
	StoreKey   = ModuleName
	RouterKey  = ModuleName
)

// KVStore key prefixes
var (
	// KeyParams stores the module parameters
	KeyParams = []byte{0x01}

	// KeyPrefixRoyaltyToken stores tokenized royalty stream records
	// token_id -> RoyaltyToken (JSON)
	KeyPrefixRoyaltyToken = []byte{0x02}

	// KeyNextTokenID is the auto-incrementing token ID counter
	KeyNextTokenID = []byte{0x03}

	// KeyPrefixOwnerIndex maps owner → token_id for fast lookups
	// owner_address | token_id -> (empty, existence marker)
	KeyPrefixOwnerIndex = []byte{0x04}

	// KeyPrefixClaimIndex maps claim_id → token_id for royalty distribution
	// claim_id | token_id -> (empty, existence marker)
	KeyPrefixClaimIndex = []byte{0x05}

	// KeyPrefixRoyaltyPayment stores historical payment records
	// token_id | epoch -> RoyaltyPayment (JSON)
	KeyPrefixRoyaltyPayment = []byte{0x06}

	// KeyPrefixFractionalization stores fractionalized token records
	// parent_token_id | fraction_index -> FractionalToken (JSON)
	KeyPrefixFractionalization = []byte{0x07}

	// KeyPrefixListingIndex stores active marketplace listings
	// token_id -> Listing (JSON)
	KeyPrefixListingIndex = []byte{0x08}

	// KeyPrefixAccumulatedRoyalties tracks pending royalties per token
	// token_id -> AccumulatedRoyalty (JSON)
	KeyPrefixAccumulatedRoyalties = []byte{0x09}

	// KeyTotalRoyaltiesDistributed tracks total royalties distributed globally
	KeyTotalRoyaltiesDistributed = []byte{0x0A}
)

func GetRoyaltyTokenKey(tokenID uint64) []byte {
	return append(KeyPrefixRoyaltyToken, sdk.Uint64ToBigEndian(tokenID)...)
}

func GetOwnerIndexKey(owner string, tokenID uint64) []byte {
	key := append(KeyPrefixOwnerIndex, []byte(owner)...)
	key = append(key, byte('/'))
	return append(key, sdk.Uint64ToBigEndian(tokenID)...)
}

func GetOwnerIndexPrefixKey(owner string) []byte {
	key := append(KeyPrefixOwnerIndex, []byte(owner)...)
	return append(key, byte('/'))
}

func GetClaimIndexKey(claimID, tokenID uint64) []byte {
	key := append(KeyPrefixClaimIndex, sdk.Uint64ToBigEndian(claimID)...)
	return append(key, sdk.Uint64ToBigEndian(tokenID)...)
}

func GetClaimIndexPrefixKey(claimID uint64) []byte {
	return append(KeyPrefixClaimIndex, sdk.Uint64ToBigEndian(claimID)...)
}

func GetRoyaltyPaymentKey(tokenID uint64, epoch int64) []byte {
	key := append(KeyPrefixRoyaltyPayment, sdk.Uint64ToBigEndian(tokenID)...)
	return append(key, sdk.Uint64ToBigEndian(uint64(epoch))...)
}

func GetFractionalizationKey(parentTokenID uint64, fractionIndex uint32) []byte {
	key := append(KeyPrefixFractionalization, sdk.Uint64ToBigEndian(parentTokenID)...)
	return append(key, sdk.Uint64ToBigEndian(uint64(fractionIndex))...)
}

func GetFractionalizationPrefixKey(parentTokenID uint64) []byte {
	return append(KeyPrefixFractionalization, sdk.Uint64ToBigEndian(parentTokenID)...)
}

func GetListingKey(tokenID uint64) []byte {
	return append(KeyPrefixListingIndex, sdk.Uint64ToBigEndian(tokenID)...)
}

func GetAccumulatedRoyaltyKey(tokenID uint64) []byte {
	return append(KeyPrefixAccumulatedRoyalties, sdk.Uint64ToBigEndian(tokenID)...)
}
