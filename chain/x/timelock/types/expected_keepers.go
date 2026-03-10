package types

import "context"

// GuardKeeperI is the interface the timelock module uses to notify the guard
// module when a proposal is queued for delayed execution. Defined in
// timelock/types (not guard/keeper) to avoid import cycles.
type GuardKeeperI interface {
	// OnTimelockQueued is called after a passed proposal is queued in timelock.
	// The guard module uses this to perform risk evaluation and queue for
	// guarded execution. Errors are non-fatal to the timelock pipeline.
	OnTimelockQueued(ctx context.Context, proposalID uint64) error

	// IsTimelockIntegrationEnabled returns true when guard is authoritative executor.
	IsTimelockIntegrationEnabled(ctx context.Context) bool

	// HasExecutionMarker returns true if guard already executed the proposal.
	HasExecutionMarker(ctx context.Context, proposalID uint64) bool
}
