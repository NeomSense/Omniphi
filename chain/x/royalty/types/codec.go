package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/royalty/UpdateParams")
	legacy.RegisterAminoMsg(cdc, &MsgTokenizeRoyalty{}, "pos/royalty/TokenizeRoyalty")
	legacy.RegisterAminoMsg(cdc, &MsgTransferToken{}, "pos/royalty/TransferToken")
	legacy.RegisterAminoMsg(cdc, &MsgClaimRoyalties{}, "pos/royalty/ClaimRoyalties")
	legacy.RegisterAminoMsg(cdc, &MsgFractionalizeToken{}, "pos/royalty/FractionalizeToken")
	legacy.RegisterAminoMsg(cdc, &MsgListToken{}, "pos/royalty/ListToken")
	legacy.RegisterAminoMsg(cdc, &MsgBuyToken{}, "pos/royalty/BuyToken")
}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {}
