package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers the necessary x/guard interfaces and concrete types
// on the provided LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/x/guard/MsgUpdateParams")
	legacy.RegisterAminoMsg(cdc, &MsgConfirmExecution{}, "pos/x/guard/MsgConfirmExecution")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateAIModel{}, "pos/x/guard/MsgUpdateAIModel")
	legacy.RegisterAminoMsg(cdc, &MsgSubmitAdvisoryLink{}, "pos/x/guard/MsgSubmitAdvisoryLink")
}

// RegisterInterfaces registers the x/guard interfaces types with the interface registry
func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
		&MsgConfirmExecution{},
		&MsgUpdateAIModel{},
		&MsgSubmitAdvisoryLink{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &Msg_ServiceDesc)
}
