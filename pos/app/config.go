package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func init() {
	// Set bond denom to match our tokenomics module
	sdk.DefaultBondDenom = "omniphi"

	// Setup Ethereum-style addresses BEFORE sealing config
	SetupEthereumAddressFormat()
}
