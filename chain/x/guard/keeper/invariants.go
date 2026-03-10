package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"pos/x/guard/types"
)

// RegisterInvariants registers all guard module invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "execution-bypass", ExecutionBypassInvariant(k))
	ir.RegisterRoute(types.ModuleName, "queue-consistency", QueueConsistencyInvariant(k))
}

// ExecutionBypassInvariant checks that no governance proposal was executed
// outside the x/guard pipeline. For every PASSED proposal that has been
// "executed" by governance, there must be a corresponding execution marker
// in x/guard's store.
//
// This detects the critical attack vector where an attacker somehow routes
// proposal execution around x/guard's gate state machine.
func ExecutionBypassInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var bypassed []uint64

		// Collect all proposals that guard has markers for
		guardExecuted := make(map[uint64]bool)
		k.IterateExecutionMarkers(ctx, func(proposalID uint64) bool {
			guardExecuted[proposalID] = true
			return false
		})

		// Iterate all queued executions that show EXECUTED state
		// Every EXECUTED queued execution should have a matching marker
		store := k.storeService.OpenKVStore(ctx)
		startKey := types.QueuedExecutionPrefix
		endKey := types.PrefixEnd(types.QueuedExecutionPrefix)

		iterator, err := store.Iterator(startKey, endKey)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "execution-bypass",
				fmt.Sprintf("failed to iterate queued executions: %v", err)), true
		}
		defer iterator.Close()

		for ; iterator.Valid(); iterator.Next() {
			bz := iterator.Value()
			var exec types.QueuedExecution
			k.cdc.MustUnmarshal(bz, &exec)

			if exec.GateState == types.EXECUTION_GATE_EXECUTED {
				if !guardExecuted[exec.ProposalId] {
					bypassed = append(bypassed, exec.ProposalId)
				}
			}
		}

		if len(bypassed) > 0 {
			return sdk.FormatInvariant(types.ModuleName, "execution-bypass",
				fmt.Sprintf("proposals executed without guard marker: %v", bypassed)), true
		}

		return sdk.FormatInvariant(types.ModuleName, "execution-bypass",
			"all executed proposals have guard markers"), false
	}
}

// QueueConsistencyInvariant verifies that queued executions are in valid states:
// - No execution can be in EXECUTED state without a corresponding execution marker
// - No execution can have GateState == UNSPECIFIED (0)
// - If RequiresSecondConfirm is true and GateState is EXECUTED,
//   then SecondConfirmReceived must be true
func QueueConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var violations []string

		store := k.storeService.OpenKVStore(ctx)
		startKey := types.QueuedExecutionPrefix
		endKey := types.PrefixEnd(types.QueuedExecutionPrefix)

		iterator, err := store.Iterator(startKey, endKey)
		if err != nil {
			return sdk.FormatInvariant(types.ModuleName, "queue-consistency",
				fmt.Sprintf("failed to iterate: %v", err)), true
		}
		defer iterator.Close()

		for ; iterator.Valid(); iterator.Next() {
			bz := iterator.Value()
			var exec types.QueuedExecution
			k.cdc.MustUnmarshal(bz, &exec)

			// Check for unspecified gate state
			if exec.GateState == types.EXECUTION_GATE_UNSPECIFIED {
				violations = append(violations,
					fmt.Sprintf("proposal %d has UNSPECIFIED gate state", exec.ProposalId))
			}

			// If EXECUTED + requires 2nd confirm, confirm must have been received
			if exec.GateState == types.EXECUTION_GATE_EXECUTED &&
				exec.RequiresSecondConfirm && !exec.SecondConfirmReceived {
				violations = append(violations,
					fmt.Sprintf("proposal %d is EXECUTED but missing required 2nd confirmation", exec.ProposalId))
			}

			// If gov proposal is not in an executable status but gate is non-terminal,
			// that's suspicious — unless timelock deliberately set StatusFailed.
			proposal, err := k.govKeeper.GetProposal(ctx, exec.ProposalId)
			if err == nil && !exec.IsTerminal() {
				if proposal.Status == govtypes.StatusRejected {
					// Rejected is always a violation — proposals should not be queued after rejection.
					violations = append(violations,
						fmt.Sprintf("proposal %d is REJECTED in gov but still active in guard queue (state=%s)",
							exec.ProposalId, exec.GateState.GetGateStateName()))
				} else if proposal.Status == govtypes.StatusFailed {
					// StatusFailed is valid ONLY if timelock set it (handover marker exists).
					// Without a handover marker, this indicates a genuine failure.
					if !k.HasTimelockHandover(ctx, exec.ProposalId) {
						violations = append(violations,
							fmt.Sprintf("proposal %d is FAILED without timelock handover but still active in guard queue (state=%s)",
								exec.ProposalId, exec.GateState.GetGateStateName()))
					}
				}
			}
		}

		if len(violations) > 0 {
			return sdk.FormatInvariant(types.ModuleName, "queue-consistency",
				fmt.Sprintf("%d violations: %v", len(violations), violations)), true
		}

		return sdk.FormatInvariant(types.ModuleName, "queue-consistency",
			"all queued executions are consistent"), false
	}
}
