package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterLegacyAminoCodec registers the module's types on the legacy Amino codec
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/rewardmult/UpdateParams")
}

// RegisterInterfaces registers the module's interface types with the InterfaceRegistry.
// Manual types without proto generation - no-op until proto types are generated.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	// Manual types cannot be registered with RegisterImplementations because they lack
	// unique proto type URLs. This will be replaced with proper registration once
	// proto types are generated.
}
