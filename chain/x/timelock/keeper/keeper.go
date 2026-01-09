package keeper

import (
	"context"
	"encoding/hex"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/timelock/types"
)

// Keeper manages the timelock module state
type Keeper struct {
	cdc       codec.Codec
	storeKey  store.KVStoreService
	logger    log.Logger
	authority string // governance module address

	// Message router for executing operations
	msgRouter baseapp.MessageRouter

	// Collections for type-safe state management
	Schema            collections.Schema
	Params            collections.Item[types.Params]
	Operations        collections.Map[uint64, types.QueuedOperation]
	OperationsByHash  collections.Map[string, uint64]
	NextOperationID   collections.Sequence
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

// ----------------------------------------------------------------------------
// Parameter Management
// ----------------------------------------------------------------------------

// GetParams returns the module parameters
func (k Keeper) GetParams(ctx context.Context) (types.Params, error) {
	return k.Params.Get(ctx)
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
		if collections.ErrNotFound.Is(err) {
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
		if collections.ErrNotFound.Is(err) {
			return nil, types.ErrOperationNotFound
		}
		return nil, err
	}
	return k.GetOperation(ctx, opID)
}

// SetOperation stores an operation
func (k Keeper) SetOperation(ctx context.Context, op *types.QueuedOperation) error {
	// Store the operation
	if err := k.Operations.Set(ctx, op.ID, *op); err != nil {
		return err
	}

	// Store hash index
	hashStr := hex.EncodeToString(op.OperationHash)
	if err := k.OperationsByHash.Set(ctx, hashStr, op.ID); err != nil {
		return err
	}

	return nil
}

// QueueOperation creates and stores a new queued operation
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

	// Get next operation ID
	opID, err := k.GetNextOperationID(ctx)
	if err != nil {
		return nil, err
	}

	// Create the operation
	op, err := types.NewQueuedOperation(
		opID,
		proposalID,
		messages,
		executor,
		sdkCtx.BlockTime(),
		params.MinDelay,
		params.GracePeriod,
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

	k.logger.Info("operation queued",
		"operation_id", op.ID,
		"proposal_id", proposalID,
		"executable_at", op.ExecutableAt,
		"expires_at", op.ExpiresAt,
		"hash", hashStr,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"operation_queued",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.ID)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
			sdk.NewAttribute("executable_at", op.ExecutableAt.String()),
			sdk.NewAttribute("expires_at", op.ExpiresAt.String()),
			sdk.NewAttribute("operation_hash", hashStr),
		),
	)

	return op, nil
}

// ExecuteOperation executes a queued operation
func (k Keeper) ExecuteOperation(ctx context.Context, operationID uint64, executor string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()

	// Get the operation
	op, err := k.GetOperation(ctx, operationID)
	if err != nil {
		return err
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
			types.ErrOperationNotExecutable, op.ExecutableAt, now)
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
				"operation_id", op.ID, "error", setErr)
		}
		return fmt.Errorf("%w: %v", types.ErrMessageExecutionFailed, err)
	}

	// Mark as executed
	op.MarkExecuted(now)
	if err := k.SetOperation(ctx, op); err != nil {
		return err
	}

	k.logger.Info("operation executed",
		"operation_id", op.ID,
		"proposal_id", op.ProposalID,
		"executor", executor,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"operation_executed",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.ID)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalID)),
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

	// Mark as cancelled
	op.MarkCancelled(sdkCtx.BlockTime(), reason)
	if err := k.SetOperation(ctx, op); err != nil {
		return err
	}

	k.logger.Info("operation cancelled",
		"operation_id", op.ID,
		"proposal_id", op.ProposalID,
		"canceller", canceller,
		"reason", reason,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"operation_cancelled",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.ID)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalID)),
			sdk.NewAttribute("canceller", canceller),
			sdk.NewAttribute("reason", reason),
		),
	)

	return nil
}

// EmergencyExecute executes an operation with reduced delay (guardian only)
func (k Keeper) EmergencyExecute(ctx context.Context, operationID uint64, guardian string, justification string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime()

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

	// Check status
	if !op.IsQueued() {
		return types.ErrOperationNotQueued
	}

	// Check if can emergency execute (emergency delay has passed)
	if !op.CanEmergencyExecute(now, params.EmergencyDelay) {
		emergencyTime := op.QueuedAt.Add(params.EmergencyDelay)
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
				"operation_id", op.ID, "error", setErr)
		}
		return fmt.Errorf("%w: %v", types.ErrMessageExecutionFailed, err)
	}

	// Mark as executed
	op.MarkExecuted(now)
	if err := k.SetOperation(ctx, op); err != nil {
		return err
	}

	k.logger.Warn("emergency operation executed",
		"operation_id", op.ID,
		"proposal_id", op.ProposalID,
		"guardian", guardian,
		"justification", justification,
	)

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"emergency_execution",
			sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.ID)),
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalID)),
			sdk.NewAttribute("guardian", guardian),
			sdk.NewAttribute("justification", justification),
		),
	)

	return nil
}

// executeMessages executes all messages in an operation
func (k Keeper) executeMessages(ctx context.Context, op *types.QueuedOperation) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get messages from operation
	msgs, err := op.GetMessages(k.cdc)
	if err != nil {
		return fmt.Errorf("failed to unpack messages: %w", err)
	}

	// Execute each message
	for i, msg := range msgs {
		handler := k.msgRouter.Handler(msg)
		if handler == nil {
			return fmt.Errorf("no handler for message %d (%s)", i, sdk.MsgTypeURL(msg))
		}

		_, err := handler(sdkCtx, msg)
		if err != nil {
			return fmt.Errorf("message %d (%s) execution failed: %w", i, sdk.MsgTypeURL(msg), err)
		}

		k.logger.Debug("message executed",
			"operation_id", op.ID,
			"message_index", i,
			"message_type", sdk.MsgTypeURL(msg),
		)
	}

	return nil
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
		if op.ProposalID == proposalID {
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
				"operation_id", op.ID,
				"proposal_id", op.ProposalID,
			)

			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent(
					"operation_expired",
					sdk.NewAttribute("operation_id", fmt.Sprintf("%d", op.ID)),
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", op.ProposalID)),
				),
			)
		}
		return false, nil
	})
}
