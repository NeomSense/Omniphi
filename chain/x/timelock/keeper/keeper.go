package keeper

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"pos/x/timelock/types"
)

// GovKeeperI defines the interface we need from the gov keeper
type GovKeeperI interface {
	GetProposal(ctx context.Context, proposalID uint64) (govv1.Proposal, error)
	SetProposal(ctx context.Context, proposal govv1.Proposal) error
	DeleteProposal(ctx context.Context, proposalID uint64) error
}

// Keeper manages the timelock module state
type Keeper struct {
	cdc       codec.Codec
	storeKey  store.KVStoreService
	logger    log.Logger
	authority string // governance module address

	// Message router for executing operations
	msgRouter baseapp.MessageRouter

	// Gov keeper reference for accessing proposals (set after initialization)
	govKeeper GovKeeperI

	// Guard keeper reference for notifying guard module of queued proposals
	guardKeeper types.GuardKeeperI

	// Collections for type-safe state management
	Schema           collections.Schema
	Params           collections.Item[types.Params]
	Operations       collections.Map[uint64, types.QueuedOperation]
	OperationsByHash collections.Map[string, uint64]
	NextOperationID  collections.Sequence
	PendingProposals collections.Map[uint64, bool] // Proposals pending timelock processing
}

// NewKeeper creates a new timelock keeper
func NewKeeper(
	cdc codec.Codec,
	storeKey store.KVStoreService,
	logger log.Logger,
	authority string,
	msgRouter baseapp.MessageRouter,
) *Keeper {
	sb := collections.NewSchemaBuilder(storeKey)

	k := &Keeper{
		cdc:       cdc,
		storeKey:  storeKey,
		logger:    logger.With("module", types.ModuleName),
		authority: authority,
		msgRouter: msgRouter,

		Params: collections.NewItem(
			sb,
			collections.NewPrefix(types.ParamsKey),
			"params",
			codec.CollValue[types.Params](cdc),
		),
		Operations: collections.NewMap(
			sb,
			collections.NewPrefix(types.OperationKeyPrefix),
			"operations",
			collections.Uint64Key,
			codec.CollValue[types.QueuedOperation](cdc),
		),
		OperationsByHash: collections.NewMap(
			sb,
			collections.NewPrefix(types.OperationByHashKeyPrefix),
			"operations_by_hash",
			collections.StringKey,
			collections.Uint64Value,
		),
		NextOperationID: collections.NewSequence(
			sb,
			collections.NewPrefix(types.NextOperationIDKey),
			"next_operation_id",
		),
		PendingProposals: collections.NewMap(
			sb,
			collections.NewPrefix([]byte("pending_proposals")),
			"pending_proposals",
			collections.Uint64Key,
			collections.BoolValue,
		),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build schema: %v", err))
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the governance module address
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns the module logger
func (k Keeper) Logger() log.Logger {
	return k.logger
}

// SetGovKeeper sets the gov keeper reference
// This must be called after keeper initialization in app.go
func (k *Keeper) SetGovKeeper(govKeeper GovKeeperI) {
	k.govKeeper = govKeeper
}

// SetGuardKeeper sets the guard keeper reference for proposal notifications.
// This must be called after keeper initialization in app.go.
func (k *Keeper) SetGuardKeeper(gk types.GuardKeeperI) {
	k.guardKeeper = gk
}

// ----------------------------------------------------------------------------
// Parameter Management
// ----------------------------------------------------------------------------

// GetParams returns the module parameters
func (k Keeper) GetParams(ctx context.Context) (types.Params, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		// If params don't exist, return default params
		// This can happen before genesis initialization
		if errors.Is(err, collections.ErrNotFound) {
			return types.DefaultParams(), nil
		}
		return types.Params{}, err
	}
	return params, nil
}

// SetParams sets the module parameters
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	return k.Params.Set(ctx, params)
}

// GetGuardian returns the guardian address
func (k Keeper) GetGuardian(ctx context.Context) (string, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return "", err
	}
	return params.Guardian, nil
}

// IsGuardian checks if the given address is the guardian
func (k Keeper) IsGuardian(ctx context.Context, addr string) (bool, error) {
	guardian, err := k.GetGuardian(ctx)
	if err != nil {
		return false, err
	}
	return guardian == addr, nil
}

// ----------------------------------------------------------------------------
// Operation Management
// ----------------------------------------------------------------------------

// GetNextOperationID returns and increments the next operation ID
func (k Keeper) GetNextOperationID(ctx context.Context) (uint64, error) {
	return k.NextOperationID.Next(ctx)
}

// GetOperation returns an operation by ID
func (k Keeper) GetOperation(ctx context.Context, operationID uint64) (*types.QueuedOperation, error) {
	op, err := k.Operations.Get(ctx, operationID)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return nil, types.ErrOperationNotFound
		}
		return nil, err
	}
	return &op, nil
}

// GetOperationByHash returns an operation by its hash
func (k Keeper) GetOperationByHash(ctx context.Context, hash []byte) (*types.QueuedOperation, error) {
	hashStr := hex.EncodeToString(hash)
	opID, err := k.OperationsByHash.Get(ctx, hashStr)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return nil, types.ErrOperationNotFound
		}
		return nil, err
	}
	return k.GetOperation(ctx, opID)
}

// SetOperation stores an operation
func (k Keeper) SetOperation(ctx context.Context, op *types.QueuedOperation) error {
	// Store the operation
	if err := k.Operations.Set(ctx, op.Id, *op); err != nil {
		return err
	}

	// Store hash index
	hashStr := hex.EncodeToString(op.OperationHash)
	if err := k.OperationsByHash.Set(ctx, hashStr, op.Id); err != nil {
		return err
	}

	return nil
}

// QueueOperation creates and stores a new queued operation.
//
// AST v2 extension: before queuing, this method:
//  1. Resolves the execution track for the proposal's message types.
//  2. Checks whether the track is paused (blocks new queuing).
//  3. Computes the adaptive delay via the multi-factor formula.
//  4. Records treasury outflow (for proposals on TRACK_TREASURY) to the
//     rolling 24-hour window and applies escalation if threshold exceeded.
//  5. Stores an immutable OperationTrackRecord alongside the operation so
//     that track classification cannot be re-evaluated or modified later.
//
// All existing behaviour (hash computation, duplicate detection, event emission)
// is preserved. The function signature is unchanged to maintain call-site compatibility.
func (k Keeper) QueueOperation(
	ctx context.Context,
	proposalID uint64,
	messages []sdk.Msg,
	executor string,
) (*types.QueuedOperation, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// Validate messages
	if len(messages) == 0 {
		return nil, types.ErrNoMessages
	}

	// --- AST v2: Track resolution and paused-gate check ---

	// Extract type URLs for classification (does not require proto decode)
	msgTypeURLs := make([]string, len(messages))
	for i, msg := range messages {
		msgTypeURLs[i] = sdk.MsgTypeURL(msg)
	}

	track, err := k.TrackForProposal(ctx, msgTypeURLs)
	if err != nil {
		// Non-fatal: fall back to TRACK_OTHER and log
		k.logger.Warn("track resolution failed, using TRACK_OTHER",
			"proposal_id", proposalID, "error", err)
		track = types.Track{
			Name:       string(types.TrackOther),
			Multiplier: types.DelayPrecision,
		}
	}

	// Gate: paused track blocks new queuing
	if track.Paused {
		return nil, fmt.Errorf("%w: track %s is paused", types.ErrTrackPaused, track.Name)
	}

	// --- AST v2: Cumulative treasury outflow detection ---
	cumulativeEscalate := false
	if track.Name == string(types.TrackTreasury) {
		// We don't have the exact spend bps here (that's in the guard risk report),
		// so we record a sentinel value and rely on the guard's risk tier for the
		// economic multiplier. Escalation tracking is still valuable at this layer
		// because timelock sees every treasury proposal regardless of guard status.
		//
		// We use 500 bps (5%) as a conservative floor when the exact amount is
		// unknown; the guard's EconomicImpactMultiplier handles the precise figure.
		_, cumulativeEscalate, err = k.RecordTreasuryOutflow(ctx, 500)
		if err != nil {
			k.logger.Warn("failed to record treasury outflow (non-fatal)",
				"proposal_id", proposalID, "error", err)
			cumulativeEscalate = false
		}

		if cumulativeEscalate {
			k.logger.Warn("cumulative 24h treasury outflow threshold exceeded — escalating delay",
				"proposal_id", proposalID)
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent(
					"cumulative_risk_escalation",
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
					sdk.NewAttribute("track", track.Name),
					sdk.NewAttribute("reason", "cumulative_24h_treasury_threshold_exceeded"),
				),
			)
		}
	}

	// --- AST v2: Param mutation frequency for TRACK_PARAM_CHANGE ---
	mutationFreqExceeded := false
	if track.Name == string(types.TrackParamChange) || track.Name == string(types.TrackConsensus) {
		mutationFreqExceeded, err = k.IncrementParamMutationCount(ctx)
		if err != nil {
			k.logger.Warn("failed to update param mutation counter (non-fatal)",
				"proposal_id", proposalID, "error", err)
			mutationFreqExceeded = false
		}
		if mutationFreqExceeded {
			k.logger.Warn("param mutation frequency threshold exceeded — applying extra delay",
				"proposal_id", proposalID)
		}
	}

	// --- AST v2: Adaptive delay computation ---
	// riskTierStr is empty here because the guard hasn't scored this proposal yet
	// (guard is notified after queuing).  We pass "" which maps to MED (1.5×) as
	// a safe conservative default.  The guard's own delay (block-based) is additive
	// on top of this time-based delay.
	adaptiveDelay := k.ComputeAdaptiveDelay(
		params.MinDelaySeconds,
		"", // risk tier not yet available; guard will add block-based delay
		0,  // economic bps not yet known; escalation handled separately above
		track,
		cumulativeEscalate,
		mutationFreqExceeded,
	)

	k.logger.Info("adaptive delay computed for proposal",
		"proposal_id", proposalID,
		"track", track.Name,
		"base_delay_seconds", params.MinDelaySeconds,
		"adaptive_delay_seconds", adaptiveDelay,
		"cumulative_escalate", cumulativeEscalate,
		"mutation_freq_exceeded", mutationFreqExceeded,
	)

	// Get next operation ID
	opID, err := k.GetNextOperationID(ctx)
	if err != nil {
		return nil, err
	}

	// Create the operation using the adaptive delay
	op, err := types.NewQueuedOperation(
		opID,
		proposalID,
		messages,
		executor,
		sdkCtx.BlockTime(),
		adaptiveDelay,
		params.GracePeriodSeconds,
		k.cdc,
	)
	if err != nil {
		return nil, err
	}

	// Check for duplicate hash
	hashStr := hex.EncodeToString(op.OperationHash)
	exists, err := k.OperationsByHash.Has(ctx, hashStr)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, types.ErrOperationAlreadyExists
	}

	// Store the operation
	if err := k.SetOperation(ctx, op); err != nil {
		return nil, err
	}

	// --- AST v2: Persist immutable track record ---
	trackRecord := types.OperationTrackRecord{
		OperationID:          opID,
		TrackName:            track.Name,
		ComputedDelaySeconds: adaptiveDelay,
	}
	if err := k.SetOperationTrackRecord(ctx, trackRecord); err != nil {
		// Non-fatal: operation is already stored. Log and continue.
		k.logger.Error("failed to store operation track record (non-fatal)",
			"operation_id", opID, "error", err)
	}

	k.logger.Info("operation queued",
		"operation_id", op.Id,
		"proposal_id", proposalID,
		"track", track.Name,
		"track_multiplier", track.Multiplier,
		"adaptive_delay_seconds", adaptiveDelay,
		"executable_at", op.ExecutableTime(),
		"expires_at", op.ExpiresTime(),
		"hash", hashStr,
	)

	// Emit enriched event (backward-compatible: all original attributes preserved)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"operation_queued",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.Id)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
			sdk.NewAttribute("executable_at", op.ExecutableTime().String()),
			sdk.NewAttribute("expires_at", op.ExpiresTime().String()),
			sdk.NewAttribute("operation_hash", hashStr),
			// AST v2 additions
			sdk.NewAttribute("track", track.Name),
			sdk.NewAttribute("track_multiplier", fmt.Sprintf("%d", track.Multiplier)),
			sdk.NewAttribute("adaptive_delay_seconds", fmt.Sprintf("%d", adaptiveDelay)),
		),
	)

	return op, nil
}

// ExecuteOperation executes a queued operation
func (k Keeper) ExecuteOperation(ctx context.Context, operationID uint64, executor string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()

	// If guard integration is enabled, guard is the sole executor.
	if k.guardKeeper != nil && k.guardKeeper.IsTimelockIntegrationEnabled(ctx) {
		return types.ErrExecutionDisabled
	}

	// Get the operation
	op, err := k.GetOperation(ctx, operationID)
	if err != nil {
		return err
	}

	// Prevent double execution if guard already executed this proposal.
	if k.guardKeeper != nil && k.guardKeeper.HasExecutionMarker(ctx, op.ProposalId) {
		return types.ErrOperationAlreadyExecuted
	}

	// Validate executor
	if op.Executor != executor {
		return types.ErrExecutorMismatch
	}

	// Check status
	if !op.IsQueued() {
		switch op.Status {
		case types.OperationStatusExecuted:
			return types.ErrOperationAlreadyExecuted
		case types.OperationStatusCancelled:
			return types.ErrOperationCancelled
		case types.OperationStatusExpired:
			return types.ErrOperationExpired
		default:
			return types.ErrOperationNotQueued
		}
	}

	// Check if expired
	if op.IsExpired(now) {
		op.MarkExpired()
		if err := k.SetOperation(ctx, op); err != nil {
			return err
		}
		return types.ErrOperationExpired
	}

	// Check if executable
	if !op.IsExecutable(now) {
		return fmt.Errorf("%w: executable at %v, current time %v",
			types.ErrOperationNotExecutable, op.ExecutableTime(), now)
	}

	// Verify hash integrity
	if !op.VerifyHash() {
		return types.ErrOperationHashMismatch
	}

	// Execute the messages
	if err := k.executeMessages(ctx, op); err != nil {
		op.MarkFailed(now, err)
		if setErr := k.SetOperation(ctx, op); setErr != nil {
			k.logger.Error("failed to update operation after execution failure",
				"operation_id", op.Id, "error", setErr)
		}
		return fmt.Errorf("%w: %v", types.ErrMessageExecutionFailed, err)
	}

	// Mark as executed
	op.MarkExecuted(now)
	if err := k.SetOperation(ctx, op); err != nil {
		return err
	}

	k.logger.Info("operation executed",
		"operation_id", op.Id,
		"proposal_id", op.ProposalId,
		"executor", executor,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"operation_executed",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.Id)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalId)),
			sdk.NewAttribute("executor", executor),
		),
	)

	return nil
}

// CancelOperation cancels a queued operation
func (k Keeper) CancelOperation(ctx context.Context, operationID uint64, canceller string, reason string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate cancellation reason
	if err := types.ValidateCancelReason(reason); err != nil {
		return err
	}

	// Verify canceller is guardian or governance
	isGuardian, err := k.IsGuardian(ctx, canceller)
	if err != nil {
		return err
	}
	if !isGuardian && canceller != k.authority {
		return types.ErrNotGuardian
	}

	// Get the operation
	op, err := k.GetOperation(ctx, operationID)
	if err != nil {
		return err
	}

	// Check status
	if !op.IsQueued() {
		return types.ErrOperationNotQueued
	}

	// SECURITY: Prevent guardian from canceling operations that modify guardian role or timelock params.
	// This prevents the guardian from making themselves irremovable by canceling governance proposals
	// that would replace or remove them.
	if isGuardian && canceller != k.authority {
		for _, anyMsg := range op.Messages {
			if anyMsg.TypeUrl == "/pos.timelock.v1.MsgUpdateGuardian" ||
				anyMsg.TypeUrl == "/pos.timelock.v1.MsgUpdateParams" {
				k.logger.Warn("GUARDIAN CANCEL BLOCKED: attempted to cancel protected operation",
					"operation_id", op.Id,
					"guardian", canceller,
					"blocked_msg_type", anyMsg.TypeUrl,
				)
				return types.ErrGuardianCannotCancelProtected
			}
		}
	}

	// Mark as cancelled
	op.MarkCancelled(sdkCtx.BlockTime(), reason)
	if err := k.SetOperation(ctx, op); err != nil {
		return err
	}

	k.logger.Info("operation cancelled",
		"operation_id", op.Id,
		"proposal_id", op.ProposalId,
		"canceller", canceller,
		"reason", reason,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"operation_cancelled",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.Id)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalId)),
			sdk.NewAttribute("canceller", canceller),
			sdk.NewAttribute("reason", reason),
		),
	)

	// SECURITY: Track guardian cancellation frequency and auto-revoke if excessive.
	// This prevents a guardian from DoS-ing governance by spamming cancels on
	// non-protected operations.
	if isGuardian && canceller != k.authority {
		k.trackGuardianCancel(ctx, canceller)
	}

	return nil
}

// EmergencyExecute executes an operation with reduced delay (guardian only)
func (k Keeper) EmergencyExecute(ctx context.Context, operationID uint64, guardian string, justification string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()

	// If guard integration is enabled, guard is the sole executor.
	if k.guardKeeper != nil && k.guardKeeper.IsTimelockIntegrationEnabled(ctx) {
		return types.ErrExecutionDisabled
	}

	// Validate justification
	if err := types.ValidateJustification(justification); err != nil {
		return err
	}

	// Verify guardian
	isGuardian, err := k.IsGuardian(ctx, guardian)
	if err != nil {
		return err
	}
	if !isGuardian {
		return types.ErrNotGuardian
	}

	// Get params for emergency delay
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	// Get the operation
	op, err := k.GetOperation(ctx, operationID)
	if err != nil {
		return err
	}

	// Prevent double execution if guard already executed this proposal.
	if k.guardKeeper != nil && k.guardKeeper.HasExecutionMarker(ctx, op.ProposalId) {
		return types.ErrOperationAlreadyExecuted
	}

	// Check status
	if !op.IsQueued() {
		return types.ErrOperationNotQueued
	}

	// SECURITY: Prevent emergency execution of operations that modify guardian role or timelock params.
	// The guardian must not be able to fast-track changes to their own role or the timelock
	// configuration, bypassing the full governance delay. These must go through normal execution.
	for _, anyMsg := range op.Messages {
		if anyMsg.TypeUrl == "/pos.timelock.v1.MsgUpdateGuardian" ||
			anyMsg.TypeUrl == "/pos.timelock.v1.MsgUpdateParams" {
			k.logger.Warn("EMERGENCY EXECUTE BLOCKED: protected operation",
				"operation_id", op.Id,
				"guardian", guardian,
				"blocked_msg_type", anyMsg.TypeUrl,
			)
			return types.ErrProtectedOperationEmergency
		}
	}

	// Check if can emergency execute (emergency delay has passed)
	if !op.CanEmergencyExecute(now, params.EmergencyDelaySeconds) {
		emergencyTime := time.Unix(op.QueuedAtUnix+int64(params.EmergencyDelaySeconds), 0)
		return fmt.Errorf("%w: emergency executable at %v, current time %v",
			types.ErrEmergencyNotEligible, emergencyTime, now)
	}

	// Verify hash integrity
	if !op.VerifyHash() {
		return types.ErrOperationHashMismatch
	}

	// Execute the messages
	if err := k.executeMessages(ctx, op); err != nil {
		op.MarkFailed(now, err)
		if setErr := k.SetOperation(ctx, op); setErr != nil {
			k.logger.Error("failed to update operation after emergency execution failure",
				"operation_id", op.Id, "error", setErr)
		}
		return fmt.Errorf("%w: %v", types.ErrMessageExecutionFailed, err)
	}

	// Mark as executed
	op.MarkExecuted(now)
	if err := k.SetOperation(ctx, op); err != nil {
		return err
	}

	k.logger.Warn("emergency operation executed",
		"operation_id", op.Id,
		"proposal_id", op.ProposalId,
		"guardian", guardian,
		"justification", justification,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"emergency_execution",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.Id)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalId)),
			sdk.NewAttribute("guardian", guardian),
			sdk.NewAttribute("justification", justification),
		),
	)

	return nil
}

// executeMessages executes all messages in an operation
// MaxAutoExecutionGas is the maximum gas allowed for timelock auto-execution
// per operation during EndBlock. This prevents governance proposals with
// expensive operations from consuming excessive block gas.
// 2M gas is sufficient for parameter changes, token transfers, and validator
// operations while preventing abuse.
const MaxAutoExecutionGas uint64 = 2_000_000

func (k Keeper) executeMessages(ctx context.Context, op *types.QueuedOperation) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// SECURITY: Create a gas-limited context for auto-execution.
	// This prevents governance operations from consuming unlimited gas
	// during EndBlock, which could slow block production or be used
	// as a resource exhaustion vector.
	gasLimitedCtx := sdkCtx.WithGasMeter(storetypes.NewGasMeter(MaxAutoExecutionGas))

	// Get messages from operation
	msgs, err := op.GetSDKMessages(k.cdc)
	if err != nil {
		return fmt.Errorf("failed to unpack messages: %w", err)
	}

	// SECURITY: Limit number of messages per operation to prevent
	// batched operations from bypassing per-message gas limits
	const maxMessagesPerOperation = 10
	if len(msgs) > maxMessagesPerOperation {
		return fmt.Errorf("operation contains %d messages, exceeding limit of %d",
			len(msgs), maxMessagesPerOperation)
	}

	// Execute each message with gas metering and atomicity
	cacheCtx, writeCache := gasLimitedCtx.CacheContext()
	var events sdk.Events

	for i, msg := range msgs {
		handler := k.msgRouter.Handler(msg)
		if handler == nil {
			return fmt.Errorf("no handler for message %d (%s)", i, sdk.MsgTypeURL(msg))
		}

		res, err := safeExecuteHandler(cacheCtx, msg, handler)
		if err != nil {
			return fmt.Errorf("message %d (%s) execution failed: %w", i, sdk.MsgTypeURL(msg), err)
		}

		events = append(events, res.GetEvents()...)

		k.logger.Debug("message executed",
			"operation_id", op.Id,
			"message_index", i,
			"message_type", sdk.MsgTypeURL(msg),
			"gas_used", cacheCtx.GasMeter().GasConsumed(),
		)
	}

	// All messages succeeded — commit state changes
	writeCache()
	sdkCtx.EventManager().EmitEvents(events)

	k.logger.Info("operation messages executed",
		"operation_id", op.Id,
		"total_messages", len(msgs),
		"total_gas_used", gasLimitedCtx.GasMeter().GasConsumed(),
		"gas_limit", MaxAutoExecutionGas,
	)

	return nil
}

// safeExecuteHandler executes handler(msg) and recovers from panics.
func safeExecuteHandler(ctx sdk.Context, msg sdk.Msg, handler func(sdk.Context, sdk.Msg) (*sdk.Result, error)) (res *sdk.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	return handler(ctx, msg)
}

// ----------------------------------------------------------------------------
// Query Helpers
// ----------------------------------------------------------------------------

// GetQueuedOperations returns all operations in QUEUED status
func (k Keeper) GetQueuedOperations(ctx context.Context) ([]*types.QueuedOperation, error) {
	var ops []*types.QueuedOperation

	err := k.Operations.Walk(ctx, nil, func(id uint64, op types.QueuedOperation) (stop bool, err error) {
		if op.Status == types.OperationStatusQueued {
			opCopy := op
			ops = append(ops, &opCopy)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return ops, nil
}

// GetExecutableOperations returns all operations ready for execution
func (k Keeper) GetExecutableOperations(ctx context.Context) ([]*types.QueuedOperation, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()
	var ops []*types.QueuedOperation

	err := k.Operations.Walk(ctx, nil, func(id uint64, op types.QueuedOperation) (stop bool, err error) {
		if op.IsExecutable(now) {
			opCopy := op
			ops = append(ops, &opCopy)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return ops, nil
}

// GetOperationsByProposal returns all operations for a proposal
func (k Keeper) GetOperationsByProposal(ctx context.Context, proposalID uint64) ([]*types.QueuedOperation, error) {
	var ops []*types.QueuedOperation

	err := k.Operations.Walk(ctx, nil, func(id uint64, op types.QueuedOperation) (stop bool, err error) {
		if op.ProposalId == proposalID {
			opCopy := op
			ops = append(ops, &opCopy)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return ops, nil
}

// MarkExpiredOperations marks all expired operations
func (k Keeper) MarkExpiredOperations(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()

	return k.Operations.Walk(ctx, nil, func(id uint64, op types.QueuedOperation) (stop bool, err error) {
		if op.Status == types.OperationStatusQueued && op.IsExpired(now) {
			op.MarkExpired()
			if err := k.SetOperation(ctx, &op); err != nil {
				return false, err
			}

			k.logger.Info("operation expired",
				"operation_id", op.Id,
				"proposal_id", op.ProposalId,
			)

			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent(
					"operation_expired",
					sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.Id)),
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalId)),
				),
			)
		}
		return false, nil
	})
}

// MarkProposalForTimelock marks a proposal ID for timelock processing
// This is called from the gov hooks when a proposal's voting period ends
func (k Keeper) MarkProposalForTimelock(ctx context.Context, proposalID uint64) error {
	return k.PendingProposals.Set(ctx, proposalID, true)
}

// GetPendingProposals retrieves all proposals pending timelock processing
func (k Keeper) GetPendingProposals(ctx context.Context) ([]uint64, error) {
	var proposalIDs []uint64
	err := k.PendingProposals.Walk(ctx, nil, func(proposalID uint64, _ bool) (stop bool, err error) {
		proposalIDs = append(proposalIDs, proposalID)
		return false, nil
	})
	return proposalIDs, err
}

// ClearPendingProposal removes a proposal from the pending list
func (k Keeper) ClearPendingProposal(ctx context.Context, proposalID uint64) error {
	return k.PendingProposals.Remove(ctx, proposalID)
}

// ProcessPendingProposals processes all proposals marked for timelock
// This runs in EndBlocker BEFORE the gov module's EndBlocker
//
// SECURITY: This function MUST successfully process or reject all pending proposals.
// If a proposal cannot be queued in timelock, it must be marked as FAILED in the gov module
// to prevent the gov module from executing it immediately, bypassing the timelock.
func (k Keeper) ProcessPendingProposals(ctx context.Context) error {
	if k.govKeeper == nil {
		// Gov keeper not set yet, skip processing
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get all pending proposals
	proposalIDs, err := k.GetPendingProposals(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending proposals: %w", err)
	}

	var criticalErrors []error

	for _, proposalID := range proposalIDs {
		// Process each proposal that passed governance
		if err := k.processProposal(ctx, sdkCtx, proposalID); err != nil {
			k.logger.Error("failed to process proposal for timelock",
				"proposal_id", proposalID,
				"error", err,
			)

			// CRITICAL: Even if queueing fails, we MUST prevent gov module from
			// executing this proposal immediately. Try to mark it as failed.
			if markErr := k.markProposalFailed(ctx, proposalID); markErr != nil {
				// This is a critical error - proposal might bypass timelock
				k.logger.Error("CRITICAL: failed to mark proposal as failed after queue error",
					"proposal_id", proposalID,
					"queue_error", err,
					"mark_error", markErr,
				)
				criticalErrors = append(criticalErrors, fmt.Errorf(
					"proposal %d: queue failed (%v), mark failed failed (%v)",
					proposalID, err, markErr,
				))
				continue
			}

			// Clear from pending - we've handled it (by marking failed)
			_ = k.ClearPendingProposal(ctx, proposalID)
			continue
		}

		// Clear from pending list after successful processing
		if err := k.ClearPendingProposal(ctx, proposalID); err != nil {
			k.logger.Error("failed to clear pending proposal",
				"proposal_id", proposalID,
				"error", err,
			)
		}
	}

	// If any critical errors occurred where we couldn't prevent gov module execution,
	// we must halt the chain to prevent timelock bypass
	if len(criticalErrors) > 0 {
		return fmt.Errorf("critical timelock errors (potential bypass): %v", criticalErrors)
	}

	return nil
}

// markProposalFailed marks a proposal as failed to prevent gov module execution
func (k Keeper) markProposalFailed(ctx context.Context, proposalID uint64) error {
	proposal, err := k.govKeeper.GetProposal(ctx, proposalID)
	if err != nil {
		return fmt.Errorf("failed to get proposal: %w", err)
	}

	// Only modify if still in a state where gov module might execute it
	if proposal.Status == govv1.StatusPassed {
		proposal.Status = govv1.StatusFailed
		if err := k.govKeeper.SetProposal(ctx, proposal); err != nil {
			return fmt.Errorf("failed to set proposal status: %w", err)
		}
		k.logger.Info("marked proposal as failed to prevent gov execution bypass",
			"proposal_id", proposalID,
		)
	}

	return nil
}

// processProposal handles the timelock queueing for a single proposal
func (k Keeper) processProposal(ctx context.Context, sdkCtx sdk.Context, proposalID uint64) error {
	// Retrieve the proposal from the gov keeper
	proposal, err := k.govKeeper.GetProposal(ctx, proposalID)
	if err != nil {
		return fmt.Errorf("failed to retrieve proposal %d: %w", proposalID, err)
	}

	// Only process proposals that have PASSED status
	if proposal.Status != govv1.StatusPassed {
		k.logger.Info("skipping proposal - not in PASSED status",
			"proposal_id", proposalID,
			"status", proposal.Status.String(),
		)
		return nil
	}

	// Text-only proposals (zero executable messages) don't need timelock queueing.
	// Leave the proposal as StatusPassed so it displays correctly in governance UIs.
	if len(proposal.Messages) == 0 {
		k.logger.Info("text-only proposal, skipping timelock",
			"proposal_id", proposalID,
		)
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				"timelock_text_proposal_skipped",
				sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
				sdk.NewAttribute("reason", "EMPTY_PROPOSAL_MSGS"),
			),
		)
		return nil
	}

	k.logger.Info("queueing passed governance proposal in timelock",
		"proposal_id", proposalID,
		"num_messages", len(proposal.Messages),
		"height", sdkCtx.BlockHeight(),
	)

	// Convert proto Any messages to sdk.Msg
	messages := make([]sdk.Msg, len(proposal.Messages))
	for i, anyMsg := range proposal.Messages {
		var msg sdk.Msg
		if err := k.cdc.UnpackAny(anyMsg, &msg); err != nil {
			return fmt.Errorf("failed to unpack message %d from proposal %d: %w", i, proposalID, err)
		}
		messages[i] = msg
	}

	// Queue the operation in timelock with the governance module as executor
	// The governance module authority will be the one executing after the delay
	operation, err := k.QueueOperation(ctx, proposalID, messages, k.authority)
	if err != nil {
		return fmt.Errorf("failed to queue operation for proposal %d: %w", proposalID, err)
	}

	k.logger.Info("proposal successfully queued in timelock",
		"proposal_id", proposalID,
		"operation_id", operation.Id,
		"executable_time", operation.ExecutableTime(),
		"queued_at", time.Unix(operation.QueuedAtUnix, 0),
	)

	// Notify guard module so it can perform risk evaluation and queue for guarded execution.
	// Non-fatal: timelock proceeds regardless of guard evaluation outcome.
	if k.guardKeeper != nil {
		if err := k.guardKeeper.OnTimelockQueued(ctx, proposalID); err != nil {
			k.logger.Error("guard notification failed (non-fatal)",
				"proposal_id", proposalID,
				"error", err,
			)
		}
	}

	// CRITICAL: Prevent the gov module from executing this proposal
	// We mark it as FAILED so the gov EndBlocker skips it
	// The actual execution will happen via timelock after the delay period
	proposal.Status = govv1.StatusFailed
	if err := k.govKeeper.SetProposal(ctx, proposal); err != nil {
		// This is critical - if we can't update the proposal status,
		// the gov module might execute it immediately, bypassing timelock
		k.logger.Error("CRITICAL: failed to update proposal status to prevent execution",
			"proposal_id", proposalID,
			"error", err,
		)
		return fmt.Errorf("failed to update proposal status for proposal %d: %w", proposalID, err)
	}

	k.logger.Info("proposal status updated to prevent immediate execution",
		"proposal_id", proposalID,
		"status", "FAILED",
		"note", "actual execution will occur via timelock after delay",
	)

	return nil
}

// MaxOperationsPerBlock limits the number of timelock operations that can be
// auto-executed in a single EndBlock. This prevents a burst of queued governance
// proposals from consuming excessive block time. Remaining operations will be
// executed in subsequent blocks.
const MaxOperationsPerBlock = 5

// AutoExecuteReadyOperations executes all operations that have passed their timelock delay.
// This runs in EndBlocker and solves the execution deadlock where module accounts cannot sign.
// Operations are executed automatically by the keeper itself, not requiring a signed message.
//
// SECURITY: Limited to MaxOperationsPerBlock per block to prevent governance-driven
// resource exhaustion. Each operation is individually gas-capped by executeMessages.
func (k Keeper) AutoExecuteReadyOperations(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()

	// If guard integration is enabled, guard is the sole executor.
	if k.guardKeeper != nil && k.guardKeeper.IsTimelockIntegrationEnabled(ctx) {
		k.logger.Info("auto-execution disabled: guard is authoritative executor")
		return nil
	}

	var executedCount, failedCount, skippedCount int

	err := k.Operations.Walk(ctx, nil, func(id uint64, op types.QueuedOperation) (stop bool, err error) {
		// Only process queued operations that are ready for execution
		if op.Status != types.OperationStatusQueued {
			return false, nil
		}

		// Check if expired first
		if op.IsExpired(now) {
			op.MarkExpired()
			if err := k.SetOperation(ctx, &op); err != nil {
				k.logger.Error("failed to mark operation as expired",
					"operation_id", op.Id,
					"error", err,
				)
			}
			return false, nil
		}

		// Check if ready for execution (passed timelock delay)
		if !op.IsExecutable(now) {
			return false, nil
		}

		// SECURITY: Enforce per-block execution cap to prevent governance-driven
		// resource exhaustion from many queued operations executing in one block.
		// Remaining operations will execute in subsequent blocks.
		if executedCount+failedCount >= MaxOperationsPerBlock {
			skippedCount++
			return false, nil
		}

		// Verify hash integrity before execution
		if !op.VerifyHash() {
			k.logger.Error("operation hash verification failed - skipping",
				"operation_id", op.Id,
				"proposal_id", op.ProposalId,
			)
			op.MarkFailed(now, types.ErrOperationHashMismatch)
			if err := k.SetOperation(ctx, &op); err != nil {
				k.logger.Error("failed to update operation after hash failure",
					"operation_id", op.Id, "error", err)
			}
			return false, nil
		}

		// Prevent double execution if guard already executed this proposal.
		if k.guardKeeper != nil && k.guardKeeper.HasExecutionMarker(ctx, op.ProposalId) {
			k.logger.Warn("auto-execution skipped: guard already executed proposal",
				"operation_id", op.Id,
				"proposal_id", op.ProposalId,
			)
			op.MarkFailed(now, types.ErrOperationAlreadyExecuted)
			if err := k.SetOperation(ctx, &op); err != nil {
				k.logger.Error("failed to update operation after guard execution check",
					"operation_id", op.Id, "error", err)
			}
			failedCount++
			return false, nil
		}

		// --- AST v2: Track freeze check ---
		// Execution is deferred (not failed) when a track is frozen.
		// The operation remains QUEUED and will be retried each block until
		// the freeze height passes.  This does NOT count against the per-block cap.
		if trackFrozen, trackName := k.isOperationTrackFrozen(ctx, op.Id, sdkCtx.BlockHeight()); trackFrozen {
			k.logger.Info("auto-execution deferred: track frozen",
				"operation_id", op.Id,
				"proposal_id", op.ProposalId,
				"track", trackName,
			)
			skippedCount++
			return false, nil
		}

		k.logger.Info("auto-executing timelock operation",
			"operation_id", op.Id,
			"proposal_id", op.ProposalId,
			"queued_at", time.Unix(op.QueuedAtUnix, 0),
			"executable_at", op.ExecutableTime(),
		)

		// Execute the messages
		if err := k.executeMessages(ctx, &op); err != nil {
			k.logger.Error("auto-execution failed",
				"operation_id", op.Id,
				"proposal_id", op.ProposalId,
				"error", err,
			)
			op.MarkFailed(now, err)
			if setErr := k.SetOperation(ctx, &op); setErr != nil {
				k.logger.Error("failed to update operation after execution failure",
					"operation_id", op.Id, "error", setErr)
			}
			failedCount++

			// Emit failure event
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent(
					"operation_auto_execute_failed",
					sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.Id)),
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalId)),
					sdk.NewAttribute("error", err.Error()),
				),
			)
			return false, nil
		}

		// Mark as executed
		op.MarkExecuted(now)
		if err := k.SetOperation(ctx, &op); err != nil {
			k.logger.Error("failed to update operation after execution",
				"operation_id", op.Id, "error", err)
			return false, err
		}

		executedCount++

		k.logger.Info("operation auto-executed successfully",
			"operation_id", op.Id,
			"proposal_id", op.ProposalId,
		)

		// Emit success event
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				"operation_auto_executed",
				sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.Id)),
				sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalId)),
				sdk.NewAttribute("executed_at", now.String()),
			),
		)

		return false, nil
	})

	if executedCount > 0 || failedCount > 0 || skippedCount > 0 {
		k.logger.Info("auto-execution complete",
			"executed", executedCount,
			"failed", failedCount,
			"deferred_to_next_block", skippedCount,
			"per_block_limit", MaxOperationsPerBlock,
		)
	}

	return err
}

// IsTrackFrozen checks if the track for a given operation is frozen.
// Exposed for use by x/guard to enforce freeze during guarded execution.
func (k Keeper) IsTrackFrozen(ctx context.Context, operationID uint64) (bool, string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.isOperationTrackFrozen(ctx, operationID, sdkCtx.BlockHeight())
}

// ----------------------------------------------------------------------------
// Guardian Cancel Tracking & Auto-Revoke
// ----------------------------------------------------------------------------

// trackGuardianCancel tracks guardian cancellation frequency and auto-revokes
// the guardian if they exceed MaxGuardianCancelsPerWindow cancels within a
// rolling window of GuardianCancelWindowBlocks blocks.
func (k Keeper) trackGuardianCancel(ctx context.Context, guardian string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	windowStart := k.getGuardianCancelWindowStart(ctx, guardian)

	// If window has expired, reset it
	if currentHeight-windowStart > types.GuardianCancelWindowBlocks {
		k.setGuardianCancelWindowStart(ctx, guardian, currentHeight)
		k.setGuardianCancelCount(ctx, guardian, 1)
		return
	}

	// Increment count within window
	count := k.getGuardianCancelCount(ctx, guardian) + 1
	k.setGuardianCancelCount(ctx, guardian, count)

	if count >= types.MaxGuardianCancelsPerWindow {
		// Auto-revoke: clear guardian address
		params, err := k.GetParams(ctx)
		if err != nil {
			k.logger.Error("failed to get params for guardian auto-revoke", "error", err)
			return
		}

		oldGuardian := params.Guardian
		params.Guardian = ""

		// Use Params.Set directly to bypass validation (empty guardian is valid)
		if err := k.Params.Set(ctx, params); err != nil {
			k.logger.Error("failed to auto-revoke guardian", "error", err)
			return
		}

		k.logger.Warn("GUARDIAN AUTO-REVOKED due to excessive cancellations",
			"revoked_guardian", oldGuardian,
			"cancel_count", count,
			"window_blocks", types.GuardianCancelWindowBlocks,
			"block_height", currentHeight,
		)

		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				"guardian_auto_revoked",
				sdk.NewAttribute("guardian", oldGuardian),
				sdk.NewAttribute("cancel_count", fmt.Sprintf("%d", count)),
				sdk.NewAttribute("max_allowed", fmt.Sprintf("%d", types.MaxGuardianCancelsPerWindow)),
				sdk.NewAttribute("block_height", fmt.Sprintf("%d", currentHeight)),
			),
		)

		// Reset tracking state
		k.setGuardianCancelCount(ctx, guardian, 0)
		k.setGuardianCancelWindowStart(ctx, guardian, 0)
	}
}

// getGuardianCancelCount returns the number of cancellations by the guardian in the current window
func (k Keeper) getGuardianCancelCount(ctx context.Context, guardian string) uint64 {
	store := k.storeKey.OpenKVStore(ctx)
	key := append(types.KeyGuardianCancelCount, []byte(guardian)...)
	bz, err := store.Get(key)
	if err != nil || bz == nil || len(bz) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

// setGuardianCancelCount stores the cancellation count for the guardian
func (k Keeper) setGuardianCancelCount(ctx context.Context, guardian string, count uint64) {
	store := k.storeKey.OpenKVStore(ctx)
	key := append(types.KeyGuardianCancelCount, []byte(guardian)...)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, count)
	_ = store.Set(key, bz)
}

// getGuardianCancelWindowStart returns the block height when the cancel window started
func (k Keeper) getGuardianCancelWindowStart(ctx context.Context, guardian string) int64 {
	store := k.storeKey.OpenKVStore(ctx)
	key := append(types.KeyGuardianCancelWindowStart, []byte(guardian)...)
	bz, err := store.Get(key)
	if err != nil || bz == nil || len(bz) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bz))
}

// setGuardianCancelWindowStart stores the block height when the cancel window started
func (k Keeper) setGuardianCancelWindowStart(ctx context.Context, guardian string, height int64) {
	store := k.storeKey.OpenKVStore(ctx)
	key := append(types.KeyGuardianCancelWindowStart, []byte(guardian)...)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))
	_ = store.Set(key, bz)
}
