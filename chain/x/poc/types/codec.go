package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers the necessary x/poc interfaces and concrete types
// on the provided LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgSubmitContribution{}, "pos/poc/SubmitContribution")
	legacy.RegisterAminoMsg(cdc, &MsgEndorse{}, "pos/poc/Endorse")
	legacy.RegisterAminoMsg(cdc, &MsgWithdrawPOCRewards{}, "pos/poc/WithdrawPOCRewards")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/poc/UpdateParams")
}

// RegisterInterfaces registers the x/poc interfaces types with the interface registry
func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitContribution{},
		&MsgEndorse{},
		&MsgWithdrawPOCRewards{},
		&MsgUpdateParams{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
