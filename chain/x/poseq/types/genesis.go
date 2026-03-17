package types

// GenesisState defines the x/poseq module genesis state.
type GenesisState struct {
	Params Params `json:"params"`
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}
