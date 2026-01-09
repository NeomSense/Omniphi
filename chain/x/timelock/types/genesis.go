package types

import "fmt"

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
		if seenIDs[op.Id] {
			return fmt.Errorf("duplicate operation ID %d at index %d", op.Id, i)
		}
		seenIDs[op.Id] = true

		// Track max ID
		if op.Id > maxID {
			maxID = op.Id
		}

		// Validate operation
		if err := op.Validate(); err != nil {
			return fmt.Errorf("invalid operation %d: %w", op.Id, err)
		}
	}

	// Ensure next operation ID is valid
	if len(gs.Operations) > 0 && gs.NextOperationId <= maxID {
		return fmt.Errorf("next_operation_id (%d) must be greater than max existing ID (%d)",
			gs.NextOperationId, maxID)
	}

	return nil
}
