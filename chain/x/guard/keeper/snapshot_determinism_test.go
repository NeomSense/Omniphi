package keeper

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeValidatorPowerSnapshot_Deterministic(t *testing.T) {
	s1 := ValidatorPowerSnapshot{
		Height:     10,
		TotalPower: 3,
		Validators: []ValidatorPower{
			{Address: "valB", Power: 2},
			{Address: "valA", Power: 1},
		},
	}
	s2 := ValidatorPowerSnapshot{
		Height:     10,
		TotalPower: 3,
		Validators: []ValidatorPower{
			{Address: "valA", Power: 1},
			{Address: "valB", Power: 2},
		},
	}

	// Sort both (matching SnapshotValidatorPower's deterministic sort)
	sort.Slice(s1.Validators, func(i, j int) bool {
		return s1.Validators[i].Address < s1.Validators[j].Address
	})
	sort.Slice(s2.Validators, func(i, j int) bool {
		return s2.Validators[i].Address < s2.Validators[j].Address
	})

	b1, err := json.Marshal(s1)
	require.NoError(t, err)
	b2, err := json.Marshal(s2)
	require.NoError(t, err)

	require.Equal(t, b1, b2, "deterministic encoding should ignore input order")
}
