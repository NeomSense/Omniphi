package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterLegacyAminoCodec registers the module's types on the legacy Amino codec
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/uci/UpdateParams")
	legacy.RegisterAminoMsg(cdc, &MsgRegisterAdapter{}, "pos/uci/RegisterAdapter")
	legacy.RegisterAminoMsg(cdc, &MsgSuspendAdapter{}, "pos/uci/SuspendAdapter")
	legacy.RegisterAminoMsg(cdc, &MsgSubmitDePINContribution{}, "pos/uci/SubmitDePINContribution")
	legacy.RegisterAminoMsg(cdc, &MsgSubmitOracleAttestation{}, "pos/uci/SubmitOracleAttestation")
	legacy.RegisterAminoMsg(cdc, &MsgUpdateAdapterConfig{}, "pos/uci/UpdateAdapterConfig")
}

// RegisterInterfaces registers the module's interface types with the InterfaceRegistry.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	// Manual types — registration handled by module's RegisterServices method
}
