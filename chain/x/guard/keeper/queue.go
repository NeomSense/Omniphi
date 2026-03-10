package keeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"pos/x/guard/types"
)

// OnTimelockQueued is called by the timelock module when a passed proposal
// is queued for delayed execution. This replaces the polling mechanism
// when timelock integration is enabled.
func (k Keeper) OnTimelockQueued(ctx context.Context, proposalID uint64) error {
	params := k.GetParams(ctx)
	if !params.TimelockIntegrationEnabled {
		return nil
	}
	if err := k.OnProposalPassed(ctx, proposalID); err != nil {
		return err
	}
	// Issue F: Set handover flag to authorize StatusFailed execution
	return k.SetTimelockHandover(ctx, proposalID)
}

// OnProposalPassed is called when a governance proposal passes
// Creates a risk report and queues the proposal for guarded execution
func (k Keeper) OnProposalPassed(ctx context.Context, proposalID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	// Get the proposal from gov
	proposal, err := k.govKeeper.GetProposal(ctx, proposalID)
	if err != nil {
		return fmt.Errorf("failed to get proposal %d: %w", proposalID, err)
	}

	// Check if already queued (idempotency)
	if _, exists := k.GetQueuedExecution(ctx, proposalID); exists {
		k.logger.Debug("proposal already queued", "proposal_id", proposalID)
		return nil
	}

	// Step 1: Generate risk report
	report, err := k.EvaluateProposal(ctx, proposal)
	if err != nil {
		return fmt.Errorf("failed to evaluate proposal %d: %w", proposalID, err)
	}

	// Step 1b: DDG v2 — Cross-proposal risk coupling
	aggregate := k.ComputeAggregateRisk(ctx)
	aggregateReasons := k.ApplyAggregateRiskEscalation(ctx, &report, aggregate)
	if len(aggregateReasons) > 0 {
		k.logger.Info("aggregate risk escalation applied",
			"proposal_id", proposalID,
			"reasons", aggregateReasons,
		)
	}

	// Step 1c: DDG v2 — Emergency hardening mode
	if k.GetEmergencyHardeningMode(ctx) {
		hardenReasons := []string{}
		report.Tier, report.ComputedThresholdBps, report.ComputedDelayBlocks, hardenReasons =
			k.applyEmergencyHardening(report.Tier, report.ComputedThresholdBps, report.ComputedDelayBlocks, hardenReasons)
		if len(hardenReasons) > 0 {
			k.logger.Info("emergency hardening applied",
				"proposal_id", proposalID,
				"reasons", hardenReasons,
			)
		}
	}

	if err := k.SetRiskReport(ctx, report); err != nil {
		return fmt.Errorf("failed to store risk report: %w", err)
	}

	// Step 2: Create queued execution
	currentHeight := uint64(sdkCtx.BlockHeight())
	earliestExecHeight := currentHeight + report.ComputedDelayBlocks

	// Determine if second confirmation required
	requiresConfirm := params.CriticalRequiresSecondConfirm && report.Tier == types.RISK_TIER_CRITICAL

	queuedExec := types.QueuedExecution{
		ProposalId:            proposalID,
		QueuedHeight:          currentHeight,
		EarliestExecHeight:    earliestExecHeight,
		GateState:             types.EXECUTION_GATE_VISIBILITY,
		GateEnteredHeight:     currentHeight,
		Tier:                  report.Tier,
		RequiredThresholdBps:  report.ComputedThresholdBps,
		RequiresSecondConfirm: requiresConfirm,
		SecondConfirmReceived: false,
		StatusNote:            "Queued for guarded execution",
	}

	if err := k.SetQueuedExecution(ctx, queuedExec); err != nil {
		return fmt.Errorf("failed to queue execution: %w", err)
	}

	// Emit event with full observability attributes
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_proposal_queued",
		sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", proposalID)),
		sdk.NewAttribute("tier", report.Tier.GetTierName()),
		sdk.NewAttribute("score", fmt.Sprintf("%d", report.Score)),
		sdk.NewAttribute("delay_blocks", fmt.Sprintf("%d", report.ComputedDelayBlocks)),
		sdk.NewAttribute("threshold_bps", fmt.Sprintf("%d", report.ComputedThresholdBps)),
		sdk.NewAttribute("queued_height", fmt.Sprintf("%d", currentHeight)),
		sdk.NewAttribute("earliest_exec_height", fmt.Sprintf("%d", earliestExecHeight)),
		sdk.NewAttribute("requires_confirmation", fmt.Sprintf("%t", requiresConfirm)),
		sdk.NewAttribute("reason_codes", report.ReasonCodes),
	))

	k.logger.Info("proposal queued for guarded execution",
		"proposal_id", proposalID,
		"tier", report.Tier.GetTierName(),
		"delay_blocks", report.ComputedDelayBlocks,
	)

	return nil
}

// ProcessQueue processes queued proposals that are eligible for advancement.
// Called from EndBlocker. Bounded by MaxQueueScanDepth to prevent DOS.
func (k Keeper) ProcessQueue(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)
	currentHeight := uint64(sdkCtx.BlockHeight())

	scanned := uint64(0)
	maxScan := params.MaxQueueScanDepth

	var proposalsToMove []uint64

	// Iterate proposals ready for processing
	k.IterateQueueByHeight(ctx, currentHeight, func(proposalID uint64) bool {
		scanned++
		if scanned > maxScan {
			k.logger.Warn("queue scan depth limit reached",
				"max_scan_depth", maxScan,
				"height", currentHeight,
			)
			return true // stop iteration
		}

		exec, found := k.GetQueuedExecution(ctx, proposalID)
		if !found {
			// Orphaned index entry, mark for cleanup
			proposalsToMove = append(proposalsToMove, proposalID)
			return false // continue
		}

		// Issue D: Cleanup terminal entries
		if exec.IsTerminal() {
			// Will be removed from current height index by move logic
			proposalsToMove = append(proposalsToMove, proposalID)
			return false // continue
		}

		// Process gate transitions
		if err := k.ProcessGateTransition(ctx, &exec); err != nil {
			k.logger.Error("failed to process gate transition",
				"proposal_id", proposalID,
				"error", err,
			)
		}

		// Issue C: Ensure index is updated if execution is deferred
		proposalsToMove = append(proposalsToMove, proposalID)

		return false // continue to next
	})

	// Process index updates
	for _, pid := range proposalsToMove {
		// Remove from current height
		if err := k.DeleteQueueIndexEntry(ctx, currentHeight, pid); err != nil {
			k.logger.Error("failed to delete queue index", "proposal_id", pid, "error", err)
		}

		// Re-index if not terminal
		exec, found := k.GetQueuedExecution(ctx, pid)
		if found && !exec.IsTerminal() {
			nextHeight := exec.EarliestExecHeight
			if nextHeight <= currentHeight {
				nextHeight = currentHeight + 1
			}
			if err := k.SetQueueIndexEntry(ctx, nextHeight, pid); err != nil {
				k.logger.Error("failed to update queue index", "proposal_id", pid, "error", err)
			}
		}
	}

	return nil
}

// ProcessGateTransition advances a queued execution through gate states
func (k Keeper) ProcessGateTransition(ctx context.Context, exec *types.QueuedExecution) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)
	currentHeight := uint64(sdkCtx.BlockHeight())

	// Not ready yet
	if currentHeight < exec.EarliestExecHeight {
		return nil
	}

	blocksInGate := currentHeight - exec.GateEnteredHeight

	// DDG v2: Continuous risk reevaluation at each gate check
	if err := k.ReevaluateRisk(ctx, exec.ProposalId); err != nil {
		k.logger.Error("risk reevaluation failed (non-fatal)",
			"proposal_id", exec.ProposalId,
			"error", err,
		)
	}
	// Re-read execution in case reevaluation changed tier/threshold
	if updatedExec, found := k.GetQueuedExecution(ctx, exec.ProposalId); found {
		*exec = updatedExec
	}

	switch exec.GateState {
	case types.EXECUTION_GATE_VISIBILITY:
		// VISIBILITY -> SHOCK_ABSORBER
		if blocksInGate >= params.VisibilityWindowBlocks {
			exec.GateState = types.EXECUTION_GATE_SHOCK_ABSORBER
			exec.GateEnteredHeight = currentHeight
			exec.StatusNote = "Entered shock absorber phase"

			if err := k.SetQueuedExecution(ctx, *exec); err != nil {
				return err
			}

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"guard_gate_transition",
				sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
				sdk.NewAttribute("from", "VISIBILITY"),
				sdk.NewAttribute("to", "SHOCK_ABSORBER"),
				sdk.NewAttribute("tier", exec.Tier.GetTierName()),
				sdk.NewAttribute("current_height", fmt.Sprintf("%d", currentHeight)),
				sdk.NewAttribute("earliest_exec_height", fmt.Sprintf("%d", exec.EarliestExecHeight)),
			))
		}

	case types.EXECUTION_GATE_SHOCK_ABSORBER:
		// SHOCK_ABSORBER -> CONDITIONAL_EXECUTION
		if blocksInGate >= params.ShockAbsorberWindowBlocks {
			// Enforce treasury throttling during this window
			if params.TreasuryThrottleEnabled {
				k.logger.Debug("treasury throttle check passed", "proposal_id", exec.ProposalId)
			}

			exec.GateState = types.EXECUTION_GATE_CONDITIONAL_EXECUTION
			exec.GateEnteredHeight = currentHeight
			exec.StatusNote = "Entered conditional execution phase"

			// Snapshot validator power when entering conditional execution
			if params.EnableStabilityChecks {
				if err := k.SnapshotValidatorPower(ctx, exec.ProposalId); err != nil {
					k.logger.Error("failed to snapshot validator power",
						"proposal_id", exec.ProposalId,
						"error", err,
					)
					// Continue — stability checks will be skipped if no snapshot
				}
			}

			if err := k.SetQueuedExecution(ctx, *exec); err != nil {
				return err
			}

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"guard_gate_transition",
				sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
				sdk.NewAttribute("from", "SHOCK_ABSORBER"),
				sdk.NewAttribute("to", "CONDITIONAL_EXECUTION"),
				sdk.NewAttribute("tier", exec.Tier.GetTierName()),
				sdk.NewAttribute("current_height", fmt.Sprintf("%d", currentHeight)),
				sdk.NewAttribute("earliest_exec_height", fmt.Sprintf("%d", exec.EarliestExecHeight)),
			))
		}

	case types.EXECUTION_GATE_CONDITIONAL_EXECUTION:
		// CONDITIONAL_EXECUTION -> READY (or extend if checks fail)
		stabilityPassed := true

		if params.EnableStabilityChecks {
			// Perform stability checks with real validator power churn detection
			passed, churnBps := k.CheckStabilityConditions(ctx, exec.ProposalId, params)
			stabilityPassed = passed
			if !stabilityPassed {
				// Extend delay
				var extension uint64
				if exec.Tier == types.RISK_TIER_CRITICAL {
					extension = params.ExtensionCriticalBlocks
				} else {
					extension = params.ExtensionHighBlocks
				}

				// Update earliest exec height and reset gate
				oldHeight := exec.EarliestExecHeight
				exec.EarliestExecHeight = currentHeight + extension
				exec.GateEnteredHeight = currentHeight
				exec.StatusNote = fmt.Sprintf("Stability checks failed (churn=%d bps, max=%d bps), extended by %d blocks",
					churnBps, params.MaxValidatorChurnBps, extension)

				// Remove stale height-index entry before SetQueuedExecution writes the new one
				_ = k.DeleteQueueIndexEntry(ctx, oldHeight, exec.ProposalId)

				// Re-snapshot for next check
				if err := k.SnapshotValidatorPower(ctx, exec.ProposalId); err != nil {
					k.logger.Error("failed to re-snapshot validator power", "error", err)
				}

				if err := k.SetQueuedExecution(ctx, *exec); err != nil {
					return err
				}

				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"guard_execution_extended",
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
					sdk.NewAttribute("reason", "stability_checks_failed"),
					sdk.NewAttribute("churn_bps", fmt.Sprintf("%d", churnBps)),
					sdk.NewAttribute("extension_blocks", fmt.Sprintf("%d", extension)),
				))

				k.logger.Warn("proposal execution extended due to stability checks",
					"proposal_id", exec.ProposalId,
					"churn_bps", churnBps,
					"extension_blocks", extension,
				)
			}
		}

		if stabilityPassed {
			exec.GateState = types.EXECUTION_GATE_READY
			exec.GateEnteredHeight = currentHeight
			exec.StatusNote = "Ready for execution"

			if err := k.SetQueuedExecution(ctx, *exec); err != nil {
				return err
			}

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"guard_gate_transition",
				sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
				sdk.NewAttribute("from", "CONDITIONAL_EXECUTION"),
				sdk.NewAttribute("to", "READY"),
				sdk.NewAttribute("tier", exec.Tier.GetTierName()),
				sdk.NewAttribute("current_height", fmt.Sprintf("%d", currentHeight)),
				sdk.NewAttribute("earliest_exec_height", fmt.Sprintf("%d", exec.EarliestExecHeight)),
			))
		}

	case types.EXECUTION_GATE_READY:
		// READY -> EXECUTED (or wait for confirmation)
		if exec.NeedsConfirmation() {
			// Check if confirmation window expired
			if blocksInGate > params.CriticalSecondConfirmWindowBlocks {
				// Extend or abort
				exec.StatusNote = "Confirmation window expired, extending"
				exec.EarliestExecHeight = currentHeight + params.ExtensionCriticalBlocks
				exec.GateState = types.EXECUTION_GATE_VISIBILITY // restart gates
				exec.GateEnteredHeight = currentHeight

				if err := k.SetQueuedExecution(ctx, *exec); err != nil {
					return err
				}

				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"guard_execution_extended",
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
					sdk.NewAttribute("reason", "confirmation_expired"),
				))
			} else {
				// Emit event reminding that confirmation is needed
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"guard_execution_confirm_required",
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
					sdk.NewAttribute("blocks_remaining", fmt.Sprintf("%d", params.CriticalSecondConfirmWindowBlocks-blocksInGate)),
				))
			}
			return nil
		}

		// Re-check threshold
		if err := k.VerifyVotingThreshold(ctx, exec); err != nil {
			exec.GateState = types.EXECUTION_GATE_ABORTED
			exec.StatusNote = fmt.Sprintf("Aborted: %v", err)

			if err := k.SetQueuedExecution(ctx, *exec); err != nil {
				return err
			}

			// Layer 3 v2: Record terminal state (non-binding)
			k.OnProposalTerminal(ctx, exec.ProposalId, "ABORTED")

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"guard_proposal_aborted",
				sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
				sdk.NewAttribute("reason", "threshold_not_met"),
			))

			return nil
		}

		// Execute proposal
		if err := k.ExecuteProposal(ctx, exec); err != nil {
			// Fix P3: Handle deferred execution (e.g. frozen track)
			if errors.Is(err, types.ErrExecutionDeferred) {
				// Backoff to prevent busy loop, but do NOT abort
				backoff := uint64(10) // Retry after 10 blocks
				exec.EarliestExecHeight = currentHeight + backoff
				exec.StatusNote = fmt.Sprintf("Execution deferred: %v", err)

				// Update state (stays in READY, but EarliestExecHeight moves forward)
				if err := k.SetQueuedExecution(ctx, *exec); err != nil {
					return err
				}
				// Return nil to indicate "processed" (deferred is a valid outcome)
				return nil
			}

			exec.GateState = types.EXECUTION_GATE_ABORTED
			exec.StatusNote = fmt.Sprintf("Execution failed: %v", err)

			if err := k.SetQueuedExecution(ctx, *exec); err != nil {
				return err
			}

			// Layer 3 v2: Record terminal state (non-binding)
			k.OnProposalTerminal(ctx, exec.ProposalId, "ABORTED")

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"guard_proposal_aborted",
				sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
				sdk.NewAttribute("reason", "execution_failed"),
				sdk.NewAttribute("error", err.Error()),
			))

			return nil
		}

		// Mark as executed and record execution marker (bypass detection)
		exec.GateState = types.EXECUTION_GATE_EXECUTED
		exec.StatusNote = "Successfully executed"

		if err := k.SetExecutionMarker(ctx, exec.ProposalId); err != nil {
			k.logger.Error("failed to set execution marker", "proposal_id", exec.ProposalId, "error", err)
		}

		if err := k.SetQueuedExecution(ctx, *exec); err != nil {
			return err
		}

		// Layer 3 v2: Record terminal state (non-binding)
		k.OnProposalTerminal(ctx, exec.ProposalId, "EXECUTED")

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"guard_proposal_executed",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", exec.ProposalId)),
			sdk.NewAttribute("tier", exec.Tier.GetTierName()),
			sdk.NewAttribute("current_height", fmt.Sprintf("%d", currentHeight)),
			sdk.NewAttribute("queued_height", fmt.Sprintf("%d", exec.QueuedHeight)),
		))

		k.logger.Info("proposal executed successfully",
			"proposal_id", exec.ProposalId,
		)
	}

	return nil
}

// ============================================================================
// ExecuteProposal — Real message dispatch through SDK router
// ============================================================================

// ExecuteProposal executes a proposal's messages through the SDK message router.
// Uses CacheContext for transactional semantics: if any message fails, all state
// changes are rolled back and the proposal is aborted.
//
// Security invariants enforced:
//   - Gate state MUST be READY (prevents bypass from other states)
//   - Proposal MUST have governance status PASSED
//   - Execution marker MUST NOT already exist (prevents double execution)
func (k Keeper) ExecuteProposal(ctx context.Context, exec *types.QueuedExecution) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// ---- Pre-dispatch assertions ----

	// 1. Gate state must be READY
	if exec.GateState != types.EXECUTION_GATE_READY {
		return types.ErrNotReady.Wrapf(
			"gate_state=%s, expected READY", exec.GateState.GetGateStateName())
	}

	// 2. Double-execution guard: check execution marker
	if k.HasExecutionMarker(ctx, exec.ProposalId) {
		return types.ErrDoubleExecution.Wrapf("proposal_id=%d", exec.ProposalId)
	}

	// 2b. Issue A: Check Timelock Track Freeze
	if k.timelockKeeper != nil {
		if frozen, _ := k.timelockKeeper.IsTrackFrozen(ctx, exec.ProposalId); frozen {
			// Fix P3: Return deferred error instead of generic failure
			return types.ErrExecutionDeferred.Wrapf("track frozen for proposal %d", exec.ProposalId)
		}
	}

	// 3. Router must be set
	if k.router == nil {
		return types.ErrExecutionFailed.Wrap("message router not configured")
	}

	// 4. Governance proposal must be in an executable status.
	// StatusPassed = normal gov flow. StatusFailed = timelock-intercepted (timelock
	// deliberately sets FAILED to prevent gov from executing immediately).
	proposal, err := k.govKeeper.GetProposal(ctx, exec.ProposalId)
	if err != nil {
		return types.ErrExecutionFailed.Wrapf("failed to get proposal %d: %v", exec.ProposalId, err)
	}

	// Issue F: Verify Handover if StatusFailed
	if proposal.Status == govtypes.StatusFailed && !k.HasTimelockHandover(ctx, exec.ProposalId) {
		return types.ErrExecutionFailed.Wrap("proposal is FAILED but no timelock handover marker found")
	}

	if !isProposalExecutable(proposal.Status) {
		return types.ErrProposalNotPassed.Wrapf(
			"proposal %d status=%s, expected PASSED or FAILED (timelock)", exec.ProposalId, proposal.Status.String())
	}

	// ---- Dispatch ----

	// No messages = text-only proposal, nothing to execute
	if len(proposal.Messages) == 0 {
		k.logger.Info("text-only proposal, nothing to execute", "proposal_id", exec.ProposalId)
		return nil
	}

	// Unpack Any messages into sdk.Msg using interface registry
	msgs, err := k.unpackProposalMessages(proposal)
	if err != nil {
		return types.ErrExecutionFailed.Wrapf("failed to unpack messages: %v", err)
	}

	// Use CacheContext so all changes are atomic:
	// if any message fails, none of the state changes persist
	cacheCtx, writeCache := sdkCtx.CacheContext()
	var events sdk.Events

	for i, msg := range msgs {
		handler := k.router.Handler(msg)
		if handler == nil {
			return types.ErrExecutionFailed.Wrapf("no handler for message %d (type: %T)", i, msg)
		}

		// safeExecuteHandler recovers from panics
		res, err := safeExecuteHandler(cacheCtx, msg, handler)
		if err != nil {
			return types.ErrExecutionFailed.Wrapf("message %d failed: %v", i, err)
		}

		events = append(events, res.GetEvents()...)
	}

	// All messages succeeded — commit state changes
	writeCache()

	// Emit collected events on the original context
	sdkCtx.EventManager().EmitEvents(events)

	k.logger.Info("executed proposal messages",
		"proposal_id", exec.ProposalId,
		"num_messages", len(msgs),
	)

	return nil
}

// unpackProposalMessages unpacks []*codectypes.Any from a proposal into []sdk.Msg.
// If interfaceRegistry is set, uses UnpackAny for proper deserialization.
// Falls back to reading cached values from the Any if already unpacked.
func (k Keeper) unpackProposalMessages(proposal govtypes.Proposal) ([]sdk.Msg, error) {
	msgs := make([]sdk.Msg, 0, len(proposal.Messages))

	for i, anyMsg := range proposal.Messages {
		// Try cached value first (set when gov module loaded the proposal)
		if cached := anyMsg.GetCachedValue(); cached != nil {
			msg, ok := cached.(sdk.Msg)
			if ok {
				msgs = append(msgs, msg)
				continue
			}
		}

		// Fall back to unpacking via interface registry
		if k.interfaceRegistry != nil {
			var msg sdk.Msg
			if err := k.interfaceRegistry.UnpackAny(anyMsg, &msg); err != nil {
				return nil, fmt.Errorf("failed to unpack message %d (type_url=%s): %w", i, anyMsg.TypeUrl, err)
			}
			msgs = append(msgs, msg)
			continue
		}

		return nil, fmt.Errorf("cannot unpack message %d: no cached value and no interface registry", i)
	}

	return msgs, nil
}

// safeExecuteHandler executes handler(msg) and recovers from panics,
// matching the same pattern used by x/gov in the Cosmos SDK.
func safeExecuteHandler(ctx sdk.Context, msg sdk.Msg, handler MsgServiceHandler) (res *sdk.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	res, err = handler(ctx, msg)
	return
}

// ============================================================================
// CalculateTreasurySpendPercentage — Real community pool query
// ============================================================================

// CalculateTreasurySpendPercentage calculates what basis points of the community pool
// a proposal intends to spend. Parses MsgCommunityPoolSpend messages and queries
// the distribution module's FeePool for the current community pool balance.
// Only considers the chain's bond denom; unknown denom spends are treated conservatively.
// Returns bps (0-10000).
func (k Keeper) CalculateTreasurySpendPercentage(ctx context.Context, proposal govtypes.Proposal) uint64 {
	// If no distr keeper, fall back to conservative estimate
	if k.distrKeeper == nil {
		k.logger.Debug("distr keeper not set, using conservative treasury estimate")
		return 500 // 5% default
	}

	// Parse spend amount from proposal messages
	spendAmount := k.parseTreasurySpendAmount(ctx, proposal)
	if spendAmount.IsZero() {
		return 0
	}

	// Query community pool balance
	feePool, err := k.distrKeeper.GetFeePool(ctx)
	if err != nil {
		k.logger.Error("failed to get fee pool", "error", err)
		return 500 // conservative fallback
	}

	// Get bond denom from staking params
	bondDenom := k.getBondDenom(ctx)

	// Find pool balance for bond denom (truncate DecCoins -> Int)
	poolBalance := math.ZeroInt()
	for _, coin := range feePool.CommunityPool {
		if coin.Denom == bondDenom {
			poolBalance = coin.Amount.TruncateInt()
			break
		}
	}

	// Handle pool=0: if spending > 0 from empty pool, return max risk
	if poolBalance.IsZero() || poolBalance.IsNegative() {
		if spendAmount.IsPositive() {
			return 10000 // 100% — spending from empty pool
		}
		return 0
	}

	// bps = (spend * 10000) / pool — integer math
	bps := spendAmount.Mul(math.NewInt(10000)).Quo(poolBalance)

	// Clamp to 10000
	if bps.GT(math.NewInt(10000)) {
		return 10000
	}

	return bps.Uint64()
}

// parseTreasurySpendAmount extracts the total spend amount (in bond denom) from
// proposal messages. Supports MsgCommunityPoolSpend and MsgSend from gov account.
func (k Keeper) parseTreasurySpendAmount(ctx context.Context, proposal govtypes.Proposal) math.Int {
	total := math.ZeroInt()
	bondDenom := k.getBondDenom(ctx)

	for _, anyMsg := range proposal.Messages {
		// Try to get cached value
		cached := anyMsg.GetCachedValue()
		if cached == nil && k.interfaceRegistry != nil {
			var msg sdk.Msg
			if err := k.interfaceRegistry.UnpackAny(anyMsg, &msg); err == nil {
				cached = msg
			}
		}
		if cached == nil {
			continue
		}

		// Check for MsgCommunityPoolSpend
		if spend, ok := cached.(*distrtypes.MsgCommunityPoolSpend); ok {
			for _, coin := range spend.Amount {
				if coin.Denom == bondDenom {
					total = total.Add(coin.Amount)
				}
			}
			continue
		}

		// Check for MsgSend (gov account sending from treasury)
		if send, ok := cached.(*banktypes.MsgSend); ok {
			// Only count if sender is the gov module account
			if send.FromAddress == k.authority {
				for _, coin := range send.Amount {
					if coin.Denom == bondDenom {
						total = total.Add(coin.Amount)
					}
				}
			}
		}
	}

	return total
}

// getBondDenom returns the chain's bond denom.
// This matches the value set in app/config.go (sdk.DefaultBondDenom = "omniphi")
// and tokenomics/types/keys.go (BondDenom = "omniphi").
// Defined here to avoid a cross-module import dependency.
const chainBondDenom = "omniphi"

func (k Keeper) getBondDenom(ctx context.Context) string {
	return chainBondDenom
}

// ============================================================================
// CheckStabilityConditions — Real validator power churn detection
// ============================================================================

// ValidatorPowerSnapshot stores a point-in-time snapshot of validator voting power
// Issue B: Use deterministic structure (slice) instead of map
type ValidatorPowerSnapshot struct {
	Height     int64            `json:"height"`
	TotalPower int64            `json:"total_power"`
	Validators []ValidatorPower `json:"validators"` // Sorted by address
}

type ValidatorPower struct {
	Address string `json:"address"`
	Power   int64  `json:"power"`
}

// SnapshotValidatorPower captures the current bonded validator power and stores it
// keyed by proposal ID. Called when entering CONDITIONAL_EXECUTION gate.
func (k Keeper) SnapshotValidatorPower(ctx context.Context, proposalID uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	var validators []ValidatorPower
	var totalPower int64

	powerReduction := k.stakingKeeper.PowerReduction(ctx)

	err := k.stakingKeeper.IterateBondedValidatorsByPower(ctx, func(index int64, val stakingtypes.ValidatorI) bool {
		// Use consensus power (tokens / power_reduction)
		cp := val.GetConsensusPower(powerReduction)
		addr := val.GetOperator()
		validators = append(validators, ValidatorPower{Address: addr, Power: cp})
		totalPower += cp
		return false // continue
	})
	if err != nil {
		return fmt.Errorf("failed to iterate validators: %w", err)
	}

	// Sort for determinism
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Address < validators[j].Address
	})

	snapshot := ValidatorPowerSnapshot{
		Height:     sdkCtx.BlockHeight(),
		TotalPower: totalPower,
		Validators: validators,
	}

	// Store as JSON in KV store
	bz, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	store := k.storeService.OpenKVStore(ctx)
	key := types.GetValidatorPowerSnapshotKey(proposalID)
	return store.Set(key, bz)
}

// GetValidatorPowerSnapshot retrieves a stored validator power snapshot
func (k Keeper) GetValidatorPowerSnapshot(ctx context.Context, proposalID uint64) (ValidatorPowerSnapshot, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetValidatorPowerSnapshotKey(proposalID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return ValidatorPowerSnapshot{}, false
	}

	var snapshot ValidatorPowerSnapshot
	if err := json.Unmarshal(bz, &snapshot); err != nil {
		k.logger.Error("failed to unmarshal power snapshot", "error", err)
		return ValidatorPowerSnapshot{}, false
	}

	return snapshot, true
}

// CheckStabilityConditions compares the current validator power distribution against
// the snapshot taken at gate entry. Returns (passed, churnBps).
// churnBps = sum(|P_now - P_snap|) * 10000 / max(totalPowerSnap, 1)
func (k Keeper) CheckStabilityConditions(ctx context.Context, proposalID uint64, params types.Params) (bool, uint64) {
	snapshot, found := k.GetValidatorPowerSnapshot(ctx, proposalID)
	if !found {
		// No snapshot available — pass by default (snapshot may have failed)
		k.logger.Warn("no validator power snapshot found, skipping stability check",
			"proposal_id", proposalID)
		return true, 0
	}

	// Get current validator power
	currentPowers := make(map[string]int64)
	var currentTotal int64

	powerReduction := k.stakingKeeper.PowerReduction(ctx)

	err := k.stakingKeeper.IterateBondedValidatorsByPower(ctx, func(index int64, val stakingtypes.ValidatorI) bool {
		cp := val.GetConsensusPower(powerReduction)
		addr := val.GetOperator()
		currentPowers[addr] = cp
		currentTotal += cp
		return false
	})
	if err != nil {
		k.logger.Error("failed to iterate validators for stability check", "error", err)
		return true, 0 // pass on error (don't block execution due to iteration failure)
	}

	// Compute churn: sum of absolute differences across union of validators
	// Convert snapshot slice to map for comparison
	snapMap := make(map[string]int64)
	for _, v := range snapshot.Validators {
		snapMap[v.Address] = v.Power
	}
	churnDelta := computeChurnDelta(snapMap, currentPowers)

	// churn_bps = (churnDelta * 10000) / max(totalPowerSnap, 1)
	denominator := snapshot.TotalPower
	if denominator <= 0 {
		denominator = 1
	}

	churnBps := uint64(churnDelta * 10000 / denominator)

	passed := churnBps <= params.MaxValidatorChurnBps

	if !passed {
		k.logger.Warn("stability check failed",
			"proposal_id", proposalID,
			"churn_bps", churnBps,
			"max_churn_bps", params.MaxValidatorChurnBps,
			"snapshot_total", snapshot.TotalPower,
			"current_total", currentTotal,
		)
	}

	return passed, churnBps
}

// computeChurnDelta computes sum(|P_now[v] - P_snap[v]|) across the union of validators.
// Deterministic: iterates keys in sorted order.
func computeChurnDelta(snapshot, current map[string]int64) int64 {
	// Build union of all validator addresses
	allAddrs := make(map[string]struct{})
	for addr := range snapshot {
		allAddrs[addr] = struct{}{}
	}
	for addr := range current {
		allAddrs[addr] = struct{}{}
	}

	// Sort for determinism
	sorted := make([]string, 0, len(allAddrs))
	for addr := range allAddrs {
		sorted = append(sorted, addr)
	}
	sort.Strings(sorted)

	var delta int64
	for _, addr := range sorted {
		snapPower := snapshot[addr] // 0 if not in snapshot (new validator)
		curPower := current[addr]   // 0 if not in current (removed validator)
		diff := snapPower - curPower
		if diff < 0 {
			diff = -diff
		}
		delta += diff
	}

	return delta
}

// ============================================================================
// VerifyVotingThreshold
// ============================================================================

// VerifyVotingThreshold checks if proposal met required voting threshold
func (k Keeper) VerifyVotingThreshold(ctx context.Context, exec *types.QueuedExecution) error {
	proposal, err := k.govKeeper.GetProposal(ctx, exec.ProposalId)
	if err != nil {
		return err
	}

	// Check proposal status (accept both PASSED and FAILED for timelock-intercepted)
	if !isProposalExecutable(proposal.Status) {
		return types.ErrProposalNotPassed.Wrapf("proposal status: %s", proposal.Status.String())
	}

	// Calculate actual yes vote percentage
	// In SDK v0.53, vote counts are strings that need to be parsed
	yesCount, err := ParseVoteCount(proposal.FinalTallyResult.YesCount)
	if err != nil {
		return types.ErrThresholdNotMet.Wrapf("invalid yes count: %v", err)
	}

	noCount, _ := ParseVoteCount(proposal.FinalTallyResult.NoCount)
	abstainCount, _ := ParseVoteCount(proposal.FinalTallyResult.AbstainCount)
	vetoCount, _ := ParseVoteCount(proposal.FinalTallyResult.NoWithVetoCount)

	totalVotes := yesCount + noCount + abstainCount + vetoCount

	if totalVotes == 0 {
		return types.ErrThresholdNotMet.Wrap("no votes cast")
	}

	// Calculate yes percentage in basis points
	yesVoteBps := yesCount * 10000 / totalVotes

	if yesVoteBps < exec.RequiredThresholdBps {
		return types.ErrThresholdNotMet.Wrapf(
			"yes votes: %d bps, required: %d bps",
			yesVoteBps,
			exec.RequiredThresholdBps,
		)
	}

	return nil
}

// ParseVoteCount parses a vote count string to uint64
// In SDK v0.53, vote counts are stored as strings
func ParseVoteCount(count string) (uint64, error) {
	if count == "" || count == "0" {
		return 0, nil
	}

	// Try to parse as math.Int first (may have large numbers)
	intVal, ok := math.NewIntFromString(count)
	if !ok {
		// Fallback to strconv
		val, err := strconv.ParseUint(count, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid vote count: %s", count)
		}
		return val, nil
	}

	// Convert to uint64 (may truncate for very large values, but that's OK for percentages)
	return intVal.Uint64(), nil
}

// isProposalExecutable returns true if the proposal status allows guard execution.
// StatusPassed is the normal governance outcome. StatusFailed is also accepted
// because the timelock module deliberately sets FAILED to prevent the gov module
// from executing proposals immediately — the actual execution happens via timelock.
func isProposalExecutable(status govtypes.ProposalStatus) bool {
	return status == govtypes.StatusPassed || status == govtypes.StatusFailed
}

// SetTimelockHandover sets a flag indicating timelock has authorized this proposal
func (k Keeper) SetTimelockHandover(ctx context.Context, proposalID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetTimelockHandoverKey(proposalID)
	return store.Set(key, []byte{1})
}

// HasTimelockHandover checks if timelock has authorized this proposal
func (k Keeper) HasTimelockHandover(ctx context.Context, proposalID uint64) bool {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetTimelockHandoverKey(proposalID)
	has, _ := store.Has(key)
	return has
}
