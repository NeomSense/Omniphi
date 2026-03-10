package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterLegacyAminoCodec registers the module's types on the legacy Amino codec
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgRegisterApp{}, "pos/por/RegisterApp")
	legacy.RegisterAminoMsg(cdc, &MsgCreateVerifierSet{}, "pos/por/CreateVerifierSet")
	legacy.RegisterAminoMsg(cdc, &MsgSubmitBatch{}, "pos/por/SubmitBatch")
	legacy.RegisterAminoMsg(cdc, &MsgSubmitAttestation{}, "pos/por/SubmitAttestation")
	legacy.RegisterAminoMsg(cdc, &MsgChallengeBatch{}, "pos/por/ChallengeBatch")
	legacy.RegisterAminoMsg(cdc, &MsgFinalizeBatch{}, "pos/por/FinalizeBatch")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/por/UpdateParams")
}

// RegisterInterfaces registers the module's interface types with the InterfaceRegistry.
// For manual (non-protoc-generated) types, this is a no-op since the message types
// don't have unique proto type URLs yet. When proto generation is set up, this will
// register implementations and the MsgServiceDesc.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	// Manual types cannot be registered with RegisterImplementations because they lack
	// unique proto type URLs (all resolve to "/" which causes conflicts).
	// This will be replaced with proper registration once proto types are generated:
	//
	// registry.RegisterImplementations((*sdk.Msg)(nil),
	//     &MsgRegisterApp{},
	//     &MsgCreateVerifierSet{},
	//     &MsgSubmitBatch{},
	//     &MsgSubmitAttestation{},
	//     &MsgChallengeBatch{},
	//     &MsgFinalizeBatch{},
	//     &MsgUpdateParams{},
	// )
	// msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
