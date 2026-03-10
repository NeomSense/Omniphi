package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterLegacyAminoCodec registers the module's types on the legacy Amino codec
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/repgov/UpdateParams")
	legacy.RegisterAminoMsg(cdc, &MsgDelegateReputation{}, "pos/repgov/DelegateReputation")
	legacy.RegisterAminoMsg(cdc, &MsgUndelegateReputation{}, "pos/repgov/UndelegateReputation")
}

// RegisterInterfaces registers the module's interface types with the InterfaceRegistry.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	// Manual types — registration handled by module's RegisterServices method
}
