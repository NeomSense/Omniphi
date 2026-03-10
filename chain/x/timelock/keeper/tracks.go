package keeper

// tracks.go — AST v2 track system
//
// All track state is stored directly in the raw KV store obtained from
// k.storeKey.OpenKVStore(ctx), using the key prefixes defined in types/keys.go.
// We deliberately do NOT use cosmos collections here because the track set is
// small and fixed (5 names), and using raw bytes gives us clear prefix isolation
// without touching the existing collections.Schema.
//
// Security invariants maintained:
//  1. Track multipliers are always in [1000, 5000] (enforced in Track.Validate).
//  2. Adaptive delay is always clamped to [AbsoluteMinDelaySeconds, AbsoluteMaxDelaySeconds].
//  3. Paused tracks block queuing, not just execution.
//  4. Frozen tracks block execution only — queuing is allowed (transparency).
//  5. Guardian CANNOT freeze tracks; only governance authority can.
//  6. Track data is written at QueueTime and never mutated afterward for an
//     existing operation (determinism: operation tracks are immutable once set).

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/timelock/types"
)

// -------------------------------------------------------------------------
// Track CRUD
// -------------------------------------------------------------------------

// GetTrack returns the Track configuration for the named track.
// Returns ErrTrackNotFound if the track has not been initialised.
func (k Keeper) GetTrack(ctx context.Context, name string) (types.Track, error) {
	store := k.storeKey.OpenKVStore(ctx)
	bz, err := store.Get(types.GetTrackKey(name))
	if err != nil {
		return types.Track{}, err
	}
	if bz == nil {
		return types.Track{}, fmt.Errorf("%w: %s", types.ErrTrackNotFound, name)
	}
	var t types.Track
	if err := json.Unmarshal(bz, &t); err != nil {
		return types.Track{}, fmt.Errorf("failed to unmarshal track %s: %w", name, err)
	}
	return t, nil
}

// SetTrack persists a Track configuration.
// Validates the track before writing.
func (k Keeper) SetTrack(ctx context.Context, t types.Track) error {
	if err := t.Validate(); err != nil {
		return err
	}
	store := k.storeKey.OpenKVStore(ctx)
	bz, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal track %s: %w", t.Name, err)
	}
	return store.Set(types.GetTrackKey(t.Name), bz)
}

// GetAllTracks returns all five canonical tracks. If a track is missing from
// the store (e.g. fresh chain before InitGenesis) the default for that track
// is returned transparently.
func (k Keeper) GetAllTracks(ctx context.Context) []types.Track {
	defaults := types.DefaultTracks()
	result := make([]types.Track, 0, len(defaults))
	for _, def := range defaults {
		t, err := k.GetTrack(ctx, def.Name)
		if err != nil {
			// Fallback to default (should only happen if genesis didn't run yet)
			result = append(result, def)
		} else {
			result = append(result, t)
		}
	}
	return result
}

// InitDefaultTracks writes all default tracks to the store. Called from
// InitGenesis. Idempotent: safe to call multiple times (overwrites).
func (k Keeper) InitDefaultTracks(ctx context.Context) error {
	for _, t := range types.DefaultTracks() {
		if err := k.SetTrack(ctx, t); err != nil {
			return fmt.Errorf("failed to init track %s: %w", t.Name, err)
		}
	}
	return nil
}

// -------------------------------------------------------------------------
// Track classification
// -------------------------------------------------------------------------

// TrackForProposal resolves the execution track for a governance proposal.
// Uses ClassifyTrackByMessageTypes on the type URLs from proposal.Messages.
// This is called at QueueTime and the result is stored permanently
// (OperationTrackRecord) so re-classification never occurs.
func (k Keeper) TrackForProposal(ctx context.Context, messageTypeURLs []string) (types.Track, error) {
	trackName := types.ClassifyTrackByMessageTypes(messageTypeURLs)
	t, err := k.GetTrack(ctx, string(trackName))
	if err != nil {
		// Fallback: if the track somehow doesn't exist, use TRACK_OTHER default.
		k.logger.Warn("track not found in store, using TRACK_OTHER fallback",
			"requested", string(trackName))
		return types.Track{
			Name:       string(types.TrackOther),
			Multiplier: 1000,
		}, nil
	}
	return t, nil
}

// -------------------------------------------------------------------------
// Operation ↔ Track record
// -------------------------------------------------------------------------

// SetOperationTrackRecord persists the track assignment for an operation.
// Must be called exactly once, at QueueTime. Never overwritten.
func (k Keeper) SetOperationTrackRecord(ctx context.Context, rec types.OperationTrackRecord) error {
	store := k.storeKey.OpenKVStore(ctx)
	bz, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal operation track record: %w", err)
	}
	return store.Set(types.GetOperationTrackKey(rec.OperationID), bz)
}

// GetOperationTrackRecord returns the track record for an operation.
func (k Keeper) GetOperationTrackRecord(ctx context.Context, operationID uint64) (types.OperationTrackRecord, error) {
	store := k.storeKey.OpenKVStore(ctx)
	bz, err := store.Get(types.GetOperationTrackKey(operationID))
	if err != nil {
		return types.OperationTrackRecord{}, err
	}
	if bz == nil {
		return types.OperationTrackRecord{}, fmt.Errorf("operation track record not found for operation %d", operationID)
	}
	var rec types.OperationTrackRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return types.OperationTrackRecord{}, err
	}
	return rec, nil
}

// -------------------------------------------------------------------------
// Adaptive delay computation
// -------------------------------------------------------------------------

// ComputeAdaptiveDelay implements the multi-factor delay formula:
//
//	delay = MinDelaySeconds × RiskMultiplier × EconomicMultiplier × TrackMultiplier
//	      (all multipliers are fixed-point with DelayPrecision = 1000)
//
// The result is then clamped to [AbsoluteMinDelaySeconds, AbsoluteMaxDelaySeconds].
//
// Parameters:
//   - minDelay: params.MinDelaySeconds (base)
//   - riskTierStr: string name of the guard risk tier ("LOW","MED","HIGH","CRITICAL")
//     or "" to default to 1.0×
//   - economicImpactBps: treasury spend as basis points of community pool (0 = none)
//   - track: the resolved Track for this proposal
//   - cumulativeEscalate: true when 24h cumulative treasury outflow exceeds threshold
//   - mutationFreqExceeded: true when param-change mutation frequency is too high
//
// Return value is always a safe, non-overflowing uint64.
func (k Keeper) ComputeAdaptiveDelay(
	minDelay uint64,
	riskTierStr string,
	economicImpactBps uint64,
	track types.Track,
	cumulativeEscalate bool,
	mutationFreqExceeded bool,
) uint64 {
	// --- Factor 1: Risk tier multiplier ---
	riskMult := riskMultiplierForTier(riskTierStr)

	// --- Factor 2: Economic impact multiplier ---
	econMult := types.DefaultEconomicImpactMultiplierBase // 1.0×
	if economicImpactBps >= 2500 {                        // ≥ 25%
		econMult = types.DefaultEconomicImpactMultiplierHigh // 2.0×
	} else if economicImpactBps >= 500 { // ≥ 5%
		econMult = types.DefaultEconomicImpactMultiplierMed // 1.4×
	}

	// --- Factor 3: Track multiplier ---
	trackMult := track.Multiplier
	if trackMult < types.DelayPrecision {
		trackMult = types.DelayPrecision // floor: never below 1×
	}

	// --- Factor 4: Cumulative treasury escalation ---
	escalateMult := types.DelayPrecision // 1.0×
	if cumulativeEscalate {
		escalateMult = 1500 // 1.5×
	}

	// --- Factor 5: Mutation frequency ---
	mutMult := types.DelayPrecision // 1.0×
	if mutationFreqExceeded {
		mutMult = types.MutationFreqMultiplier // 1.5×
	}

	// --- Combine all factors ---
	// Apply multipliers one at a time, dividing at each step by DelayPrecision
	// to keep the value in range and prevent intermediate overflow.
	p := types.DelayPrecision
	delay := mulDiv(minDelay, riskMult, p)
	delay = mulDiv(delay, econMult, p)
	delay = mulDiv(delay, trackMult, p)
	delay = mulDiv(delay, escalateMult, p)
	delay = mulDiv(delay, mutMult, p)

	// --- Clamp ---
	if delay < types.AbsoluteMinDelaySeconds {
		delay = types.AbsoluteMinDelaySeconds
	}
	if delay > types.AbsoluteMaxDelaySeconds {
		delay = types.AbsoluteMaxDelaySeconds
	}

	return delay
}

// mulDiv computes (a * b) / c without intermediate overflow for typical governance
// delay values (all fit comfortably in uint64 when values are ≤ 30 days × 5000).
func mulDiv(a, b, c uint64) uint64 {
	if c == 0 {
		return a
	}
	return a * b / c
}

// riskMultiplierForTier converts a guard risk tier string to a fixed-point multiplier.
// String constants avoid import cycles (guard imports timelock via expected_keepers).
func riskMultiplierForTier(tier string) uint64 {
	switch tier {
	case "RISK_TIER_LOW", "LOW":
		return types.DefaultRiskMultiplierLow
	case "RISK_TIER_MED", "MED":
		return types.DefaultRiskMultiplierMed
	case "RISK_TIER_HIGH", "HIGH":
		return types.DefaultRiskMultiplierHigh
	case "RISK_TIER_CRITICAL", "CRITICAL":
		return types.DefaultRiskMultiplierCritical
	default:
		// Unknown tier: treat as MED (conservative default)
		return types.DefaultRiskMultiplierMed
	}
}

// -------------------------------------------------------------------------
// Cumulative treasury outflow tracking
// -------------------------------------------------------------------------

// GetTreasuryOutflowWindow returns the current rolling 24-hour treasury window.
// Returns a zero-value window (WindowStartUnix=0) if no window exists yet.
func (k Keeper) GetTreasuryOutflowWindow(ctx context.Context) (types.TreasuryOutflowWindow, error) {
	store := k.storeKey.OpenKVStore(ctx)
	bz, err := store.Get(types.TreasuryWindowKey)
	if err != nil {
		return types.TreasuryOutflowWindow{}, err
	}
	if bz == nil {
		return types.TreasuryOutflowWindow{}, nil
	}
	var w types.TreasuryOutflowWindow
	if err := json.Unmarshal(bz, &w); err != nil {
		return types.TreasuryOutflowWindow{}, err
	}
	return w, nil
}

// setTreasuryOutflowWindow persists the rolling outflow window.
func (k Keeper) setTreasuryOutflowWindow(ctx context.Context, w types.TreasuryOutflowWindow) error {
	store := k.storeKey.OpenKVStore(ctx)
	bz, err := json.Marshal(w)
	if err != nil {
		return err
	}
	return store.Set(types.TreasuryWindowKey, bz)
}

// RecordTreasuryOutflow adds the given outflow (in basis points of community pool
// at the time of queuing) to the rolling 24-hour window.  Returns the updated
// cumulative total and whether the escalation threshold is exceeded.
func (k Keeper) RecordTreasuryOutflow(
	ctx context.Context,
	outflowBps uint64,
) (cumulativeBps uint64, escalate bool, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()
	height := sdkCtx.BlockHeight()

	w, err := k.GetTreasuryOutflowWindow(ctx)
	if err != nil {
		return 0, false, err
	}

	// Reset if window is stale
	if w.WindowStartUnix == 0 || now-w.WindowStartUnix > types.CumulativeTreasuryWindow {
		w = types.TreasuryOutflowWindow{
			WindowStartUnix:  now,
			TotalOutflowBps:  0,
			LastUpdateHeight: height,
		}
	}

	// Add new outflow — cap at 10000 bps to prevent phantom overflow
	w.TotalOutflowBps += outflowBps
	if w.TotalOutflowBps > 10000 {
		w.TotalOutflowBps = 10000
	}
	w.LastUpdateHeight = height

	if err := k.setTreasuryOutflowWindow(ctx, w); err != nil {
		return 0, false, err
	}

	escalate = w.TotalOutflowBps >= types.DefaultCumulativeTreasuryEscalateBps
	return w.TotalOutflowBps, escalate, nil
}

// -------------------------------------------------------------------------
// Param mutation frequency tracking
// -------------------------------------------------------------------------

// IncrementParamMutationCount increments the rolling param-change mutation
// counter. Returns true if the threshold is exceeded (triggering extra delay).
func (k Keeper) IncrementParamMutationCount(ctx context.Context) (exceeded bool, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlock := sdkCtx.BlockHeight()

	store := k.storeKey.OpenKVStore(ctx)

	// Read current window start block and count (packed as 16 bytes)
	bz, err := store.Get(types.ParamChangeFreqKeyPrefix)
	if err != nil {
		return false, err
	}

	var windowStart int64
	var count uint64
	if bz != nil && len(bz) >= 16 {
		windowStart = int64(binary.BigEndian.Uint64(bz[:8]))
		count = binary.BigEndian.Uint64(bz[8:16])
	}

	// Reset if window expired
	if currentBlock-windowStart > types.ParamChangeMutationWindowBlocks {
		windowStart = currentBlock
		count = 0
	}

	count++

	// Persist: pack [8-byte window start][8-byte count]
	packed := make([]byte, 16)
	binary.BigEndian.PutUint64(packed[:8], uint64(windowStart))
	binary.BigEndian.PutUint64(packed[8:], count)
	if err := store.Set(types.ParamChangeFreqKeyPrefix, packed); err != nil {
		return false, err
	}

	return count >= types.ParamChangeMutationThreshold, nil
}

// GetParamMutationCount returns the current param mutation count in the active window.
func (k Keeper) GetParamMutationCount(ctx context.Context) (count uint64, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.storeKey.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamChangeFreqKeyPrefix)
	if err != nil {
		return 0, err
	}
	if bz == nil || len(bz) < 16 {
		return 0, nil
	}
	windowStart := int64(binary.BigEndian.Uint64(bz[:8]))
	if sdkCtx.BlockHeight()-windowStart > types.ParamChangeMutationWindowBlocks {
		return 0, nil // window expired
	}
	return binary.BigEndian.Uint64(bz[8:16]), nil
}

// -------------------------------------------------------------------------
// Track freeze management
// -------------------------------------------------------------------------

// FreezeTrack sets FreezeUntilHeight on the named track.
// Governance-only: callers must verify authority before calling.
func (k Keeper) FreezeTrack(
	ctx context.Context,
	trackName string,
	freezeUntilHeight int64,
	reason string,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	if len(reason) < 10 {
		return fmt.Errorf("freeze reason must be at least 10 characters")
	}

	if freezeUntilHeight <= currentHeight {
		return fmt.Errorf("freeze_until_height %d must be greater than current height %d",
			freezeUntilHeight, currentHeight)
	}

	duration := freezeUntilHeight - currentHeight
	if duration > types.MaxFreezeDurationBlocks {
		return fmt.Errorf("%w: requested %d blocks, max %d",
			types.ErrFreezeTooLong, duration, types.MaxFreezeDurationBlocks)
	}

	t, err := k.GetTrack(ctx, trackName)
	if err != nil {
		return err
	}

	t.FreezeUntilHeight = freezeUntilHeight

	if err := k.SetTrack(ctx, t); err != nil {
		return err
	}

	k.logger.Warn("track frozen by governance",
		"track", trackName,
		"freeze_until_height", freezeUntilHeight,
		"current_height", currentHeight,
		"reason", reason,
	)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"timelock_track_frozen",
			sdk.NewAttribute("track", trackName),
			sdk.NewAttribute("freeze_until_height", fmt.Sprintf("%d", freezeUntilHeight)),
			sdk.NewAttribute("current_height", fmt.Sprintf("%d", currentHeight)),
			sdk.NewAttribute("reason", reason),
		),
	)

	return nil
}

// -------------------------------------------------------------------------
// Frozen-check helper for AutoExecuteReadyOperations
// -------------------------------------------------------------------------

// isOperationTrackFrozen returns (true, trackName) if the operation's resolved
// track is currently frozen at currentHeight.  Returns (false, "") if not frozen,
// if no track record exists (e.g. pre-v2 operations), or on any store error.
// This is intentionally non-fatal: if the record is missing we allow execution
// to proceed (safe default — the track freeze is a new feature).
func (k Keeper) isOperationTrackFrozen(ctx context.Context, operationID uint64, currentHeight int64) (bool, string) {
	rec, err := k.GetOperationTrackRecord(ctx, operationID)
	if err != nil {
		// Pre-v2 operation or missing record: not frozen
		return false, ""
	}
	t, err := k.GetTrack(ctx, rec.TrackName)
	if err != nil {
		// Track not found: not frozen
		return false, ""
	}
	return t.IsFrozen(currentHeight), rec.TrackName
}

// -------------------------------------------------------------------------
// Composite stability predicate
// -------------------------------------------------------------------------

// CompositeStabilityPredicateResult holds the output of the stability predicate.
type CompositeStabilityPredicateResult struct {
	// Passed is true when all stability conditions are met.
	Passed bool
	// Reasons lists which predicates failed (empty if Passed=true).
	Reasons []string
	// SuggestedExtensionSeconds is a non-zero hint for how long to delay
	// if conditions are not met.
	SuggestedExtensionSeconds uint64
}

// CheckCompositeStabilityPredicates evaluates all stability sub-predicates for
// a proposed operation and returns an aggregate result.
func (k Keeper) CheckCompositeStabilityPredicates(
	ctx context.Context,
	trackName string,
) CompositeStabilityPredicateResult {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()
	var reasons []string

	// 1. Track freeze (execution gate)
	t, err := k.GetTrack(ctx, trackName)
	if err == nil && t.IsFrozen(currentHeight) {
		reasons = append(reasons, fmt.Sprintf("track %s frozen until height %d", trackName, t.FreezeUntilHeight))
	}

	// 2. Track paused (defence-in-depth)
	if err == nil && t.Paused {
		reasons = append(reasons, fmt.Sprintf("track %s is paused", trackName))
	}

	// 3. Cumulative treasury
	w, wErr := k.GetTreasuryOutflowWindow(ctx)
	if wErr == nil {
		now := sdkCtx.BlockTime().Unix()
		if w.WindowStartUnix != 0 && now-w.WindowStartUnix <= types.CumulativeTreasuryWindow {
			if w.TotalOutflowBps >= types.DefaultCumulativeTreasuryEscalateBps {
				reasons = append(reasons,
					fmt.Sprintf("cumulative 24h treasury outflow %d bps >= threshold %d bps",
						w.TotalOutflowBps, types.DefaultCumulativeTreasuryEscalateBps))
			}
		}
	}

	// 4. Param mutation frequency
	mutCount, mErr := k.GetParamMutationCount(ctx)
	if mErr == nil && mutCount >= types.ParamChangeMutationThreshold {
		reasons = append(reasons,
			fmt.Sprintf("param mutation frequency %d >= threshold %d in current window",
				mutCount, types.ParamChangeMutationThreshold))
	}

	passed := len(reasons) == 0

	var suggestedExt uint64
	if !passed {
		suggestedExt = 86400 // 24h suggested extension
	}

	return CompositeStabilityPredicateResult{
		Passed:                    passed,
		Reasons:                   reasons,
		SuggestedExtensionSeconds: suggestedExt,
	}
}
