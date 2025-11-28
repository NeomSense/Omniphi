package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers the necessary x/feemarket interfaces and concrete types
// on the provided LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	legacy.RegisterAminoMsg(cdc, &MsgUpdateParams{}, "pos/x/feemarket/MsgUpdateParams")
}

// RegisterInterfaces registers the x/feemarket interfaces types with the interface registry
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgUpdateParams{},
	)

	// NOTE: msgservice.RegisterMsgServiceDesc is intentionally not called here
	// because there's an issue with the generated proto file descriptor that causes
	// "error unzipping file description" panic. The module works fine without it
	// since we only use UpdateParams via governance and don't need gRPC reflection.
	// This can be re-enabled once proto generation is fixed.
	_ = msgservice.RegisterMsgServiceDesc
}
