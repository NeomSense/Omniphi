package types

import "fmt"

// GenesisState defines the timelock module's genesis state
type GenesisState struct {
	Params          Params             `json:"params"`
	Operations      []QueuedOperation  `json:"operations"`
	NextOperationId uint64             `json:"next_operation_id"`
}

// DefaultGenesisState returns the default genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:          DefaultParams(),
		Operations:      []QueuedOperation{},
		NextOperationId: 1,
	}
}

// Validate validates the genesis state
func (gs *GenesisState) Validate() error {
	// Validate params
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Validate operation IDs are unique and sequential
	seenIDs := make(map[uint64]bool)
	maxID := uint64(0)

	for i, op := range gs.Operations {
		// Check for duplicate IDs
		if seenIDs[op.ID] {
			return fmt.Errorf("duplicate operation ID %d at index %d", op.ID, i)
		}
		seenIDs[op.ID] = true

		// Track max ID
		if op.ID > maxID {
			maxID = op.ID
		}

		// Validate operation
		if err := op.Validate(); err != nil {
			return fmt.Errorf("invalid operation %d: %w", op.ID, err)
		}
	}

	// Ensure next operation ID is valid
	if len(gs.Operations) > 0 && gs.NextOperationId <= maxID {
		return fmt.Errorf("next_operation_id (%d) must be greater than max existing ID (%d)",
			gs.NextOperationId, maxID)
	}

	return nil
}
