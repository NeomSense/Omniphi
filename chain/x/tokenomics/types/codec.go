package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers the necessary types for the tokenomics module
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgMintTokens{}, "pos/tokenomics/MsgMintTokens", nil)
	cdc.RegisterConcrete(&MsgBurnTokens{}, "pos/tokenomics/MsgBurnTokens", nil)
	cdc.RegisterConcrete(&MsgDistributeRewards{}, "pos/tokenomics/MsgDistributeRewards", nil)
	cdc.RegisterConcrete(&MsgReportBurn{}, "pos/tokenomics/MsgReportBurn", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "pos/tokenomics/MsgUpdateParams", nil)
}

// RegisterInterfaces registers the module's interface types
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgMintTokens{},
		&MsgBurnTokens{},
		&MsgDistributeRewards{},
		&MsgReportBurn{},
		&MsgUpdateParams{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
