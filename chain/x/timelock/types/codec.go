package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers the necessary x/timelock interfaces and concrete types
// on the provided LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgExecuteOperation{}, "pos/x/timelock/MsgExecuteOperation")
	legacy.RegisterAminoMsg(cdc, &MsgCancelOperation{}, "pos/x/timelock/MsgCancelOperation")
	legacy.RegisterAminoMsg(cdc, &MsgEmergencyExecute{}, "pos/x/timelock/MsgEmergencyExecute")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/x/timelock/MsgUpdateParams")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateGuardian{}, "pos/x/timelock/MsgUpdateGuardian")
}

// RegisterInterfaces registers the x/timelock interfaces types with the interface registry
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgExecuteOperation{},
		&MsgCancelOperation{},
		&MsgEmergencyExecute{},
		&MsgUpdateParams{},
		&MsgUpdateGuardian{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
