package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/guard/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger

	// authority is the address capable of executing governance operations (usually gov module)
	authority string

	// govKeeper is used to query and execute proposals
	govKeeper GovKeeper

	// stakingKeeper is used for validator queries
	stakingKeeper StakingKeeper

	// bankKeeper is used for treasury balance queries
	bankKeeper BankKeeper

	// distrKeeper is used for community pool balance queries
	distrKeeper DistrKeeper

	// router is the message service router for executing proposal messages
	router MessageRouter

	// timelockKeeper is used to check for track freezes
	timelockKeeper TimelockKeeperI

	// interfaceRegistry unpacks Any-encoded messages from proposals
	interfaceRegistry codectypes.InterfaceRegistry
}

// NewKeeper creates a new guard Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	authority string,
	govKeeper GovKeeper,
	stakingKeeper StakingKeeper,
	bankKeeper BankKeeper,
	logger log.Logger,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Errorf("invalid authority address: %w", err))
	}

	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		authority:     authority,
		govKeeper:     govKeeper,
		stakingKeeper: stakingKeeper,
		bankKeeper:    bankKeeper,
		logger:        logger,
	}
}

// SetDistrKeeper sets the distribution keeper (post-construction dependency injection)
func (k *Keeper) SetDistrKeeper(dk DistrKeeper) {
	k.distrKeeper = dk
}

// SetRouter sets the message service router (post-construction dependency injection)
func (k *Keeper) SetRouter(router MessageRouter) {
	k.router = router
}

// SetTimelockKeeper sets the timelock keeper reference (post-construction dependency injection)
func (k *Keeper) SetTimelockKeeper(tk TimelockKeeperI) {
	k.timelockKeeper = tk
}

// SetInterfaceRegistry sets the interface registry for unpacking Any messages
func (k *Keeper) SetInterfaceRegistry(ir codectypes.InterfaceRegistry) {
	k.interfaceRegistry = ir
}

// GetAuthority returns the module's authority address
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger
func (k Keeper) Logger() log.Logger {
	return k.logger
}

// IsTimelockIntegrationEnabled returns true when the guard module is configured
// as the authoritative executor for governance proposals (via timelock handoff).
// Required by timelock/types.GuardKeeperI interface.
func (k Keeper) IsTimelockIntegrationEnabled(ctx context.Context) bool {
	return k.GetParams(ctx).TimelockIntegrationEnabled
}

// ============================================================================
// Params
// ============================================================================

// SetParams sets the module parameters
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&params)
	return store.Set(types.ParamsKey, bz)
}

// GetParams returns the current module parameters
func (k Keeper) GetParams(ctx context.Context) types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}

	var params types.Params
	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// ============================================================================
// RiskReport Storage
// ============================================================================

// SetRiskReport stores a risk report for a proposal
func (k Keeper) SetRiskReport(ctx context.Context, report types.RiskReport) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&report)
	key := types.GetRiskReportKey(report.ProposalId)
	return store.Set(key, bz)
}

// GetRiskReport retrieves a risk report for a proposal
func (k Keeper) GetRiskReport(ctx context.Context, proposalID uint64) (types.RiskReport, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetRiskReportKey(proposalID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.RiskReport{}, false
	}

	var report types.RiskReport
	k.cdc.MustUnmarshal(bz, &report)
	return report, true
}

// ============================================================================
// QueuedExecution Storage
// ============================================================================

// SetQueuedExecution stores a queued execution
func (k Keeper) SetQueuedExecution(ctx context.Context, exec types.QueuedExecution) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&exec)
	key := types.GetQueuedExecutionKey(exec.ProposalId)
	if err := store.Set(key, bz); err != nil {
		return err
	}

	// Also update the height index for efficient queue processing
	indexKey := types.GetQueueIndexKey(exec.EarliestExecHeight, exec.ProposalId)
	if err := store.Set(indexKey, []byte{1}); err != nil {
		return err
	}

	// Maintain Active Execution Index for bounded aggregate-risk scans.
	activeKey := types.GetActiveExecutionIndexKey(exec.ProposalId)
	if exec.IsTerminal() {
		return store.Delete(activeKey)
	}
	return store.Set(activeKey, []byte{1})
}

// GetQueuedExecution retrieves a queued execution
func (k Keeper) GetQueuedExecution(ctx context.Context, proposalID uint64) (types.QueuedExecution, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetQueuedExecutionKey(proposalID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.QueuedExecution{}, false
	}

	var exec types.QueuedExecution
	k.cdc.MustUnmarshal(bz, &exec)
	return exec, true
}

// SetQueueIndexEntry writes a height-based queue index entry for a proposal.
// Used by ProcessQueue to re-index proposals at their updated earliest execution height.
func (k Keeper) SetQueueIndexEntry(ctx context.Context, height uint64, proposalID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	indexKey := types.GetQueueIndexKey(height, proposalID)
	return store.Set(indexKey, []byte{1})
}

// DeleteQueueIndexEntry removes a queue index entry (used when updating earliest_exec_height)
func (k Keeper) DeleteQueueIndexEntry(ctx context.Context, height uint64, proposalID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	indexKey := types.GetQueueIndexKey(height, proposalID)
	return store.Delete(indexKey)
}

// IterateQueueByHeight iterates over all queued executions at or before a given height
func (k Keeper) IterateQueueByHeight(ctx context.Context, maxHeight uint64, cb func(proposalID uint64) (stop bool)) {
	store := k.storeService.OpenKVStore(ctx)

	// Iterate from the queue index prefix
	startKey := types.QueueIndexByHeightPrefix
	endKey := types.GetQueueIndexPrefixByHeight(maxHeight + 1)

	iterator, err := store.Iterator(startKey, endKey)
	if err != nil {
		k.logger.Error("failed to create iterator", "error", err)
		return
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		// Key format: prefix (1 byte) | height (8 bytes) | proposalID (8 bytes)
		if len(key) != 17 {
			continue
		}
		proposalID := types.BigEndianToUint64(key[9:])
		if cb(proposalID) {
			break
		}
	}
}

// IterateActiveExecutions iterates over all non-terminal queued executions.
// Used by ComputeAggregateRisk to avoid scanning history.
func (k Keeper) IterateActiveExecutions(ctx context.Context, cb func(exec types.QueuedExecution) (stop bool)) {
	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(types.ActiveExecutionIndexPrefix, types.PrefixEnd(types.ActiveExecutionIndexPrefix))
	if err != nil {
		k.logger.Error("failed to create active execution iterator", "error", err)
		return
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		// Key is Prefix (1) + ProposalID (8)
		key := iterator.Key()
		if len(key) < 9 {
			continue
		}
		proposalID := types.BigEndianToUint64(key[1:])

		exec, found := k.GetQueuedExecution(ctx, proposalID)
		if found {
			if cb(exec) {
				break
			}
		}
	}
}

// ============================================================================
// Execution Markers (bypass-detection support)
// ============================================================================

// SetExecutionMarker records that a proposal was executed through x/guard.
// Called after successful execution in ProcessGateTransition.
func (k Keeper) SetExecutionMarker(ctx context.Context, proposalID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetExecutionMarkerKey(proposalID)
	return store.Set(key, []byte{1})
}

// HasExecutionMarker checks whether a proposal has been executed through x/guard.
func (k Keeper) HasExecutionMarker(ctx context.Context, proposalID uint64) bool {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetExecutionMarkerKey(proposalID)
	bz, err := store.Get(key)
	return err == nil && bz != nil
}

// IterateExecutionMarkers iterates over all execution markers.
func (k Keeper) IterateExecutionMarkers(ctx context.Context, cb func(proposalID uint64) (stop bool)) {
	store := k.storeService.OpenKVStore(ctx)

	startKey := types.ExecutionMarkerPrefix
	endKey := types.PrefixEnd(types.ExecutionMarkerPrefix)

	iterator, err := store.Iterator(startKey, endKey)
	if err != nil {
		k.logger.Error("failed to create execution marker iterator", "error", err)
		return
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		if len(key) != 9 { // 1 prefix + 8 proposalID
			continue
		}
		proposalID := types.BigEndianToUint64(key[1:])
		if cb(proposalID) {
			break
		}
	}
}

// ============================================================================
// LastProcessedProposalID
// ============================================================================

// GetLastProcessedProposalID returns the last processed proposal ID from gov
func (k Keeper) GetLastProcessedProposalID(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.LastProcessedProposalIDKey)
	if err != nil || bz == nil {
		return 0
	}
	return types.BigEndianToUint64(bz)
}

// SetLastProcessedProposalID sets the last processed proposal ID
func (k Keeper) SetLastProcessedProposalID(ctx context.Context, proposalID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.LastProcessedProposalIDKey, types.SdkUint64ToBigEndian(proposalID))
}
