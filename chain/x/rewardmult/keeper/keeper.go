package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/rewardmult/types"
)

// Keeper manages the state and business logic for x/rewardmult
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger
	authority    string // governance module account

	// Required keepers
	stakingKeeper  types.StakingKeeper
	slashingKeeper types.SlashingKeeper

	// Optional keepers (nil-safe)
	pocKeeper types.PocKeeper
	porKeeper types.PorKeeper
}

// NewKeeper creates a new Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
	stakingKeeper types.StakingKeeper,
	slashingKeeper types.SlashingKeeper,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	return Keeper{
		cdc:            cdc,
		storeService:   storeService,
		logger:         logger,
		authority:      authority,
		stakingKeeper:  stakingKeeper,
		slashingKeeper: slashingKeeper,
		pocKeeper:      nil,
		porKeeper:      nil,
	}
}

// SetPocKeeper sets the optional PoC keeper (called post-init)
func (k *Keeper) SetPocKeeper(pocKeeper types.PocKeeper) {
	k.pocKeeper = pocKeeper
}

// SetPorKeeper sets the optional PoR keeper (called post-init)
func (k *Keeper) SetPorKeeper(porKeeper types.PorKeeper) {
	k.porKeeper = porKeeper
}

// RecordEndorsementParticipation records that a validator participated (or not)
// in a PoV endorsement opportunity. This data feeds into the participation bonus
// computed during epoch processing. Currently a no-op stub — participation rate
// is read directly from the PoC keeper via GetEndorsementParticipationRate.
func (k Keeper) RecordEndorsementParticipation(ctx context.Context, valAddr sdk.ValAddress, participated bool) error {
	// Participation tracking is handled by x/poc's ValidatorEndorsementStats.
	// This method satisfies the RewardmultKeeper interface so PoC can call it
	// if needed for future direct-push metrics.
	return nil
}

// GetAuthority returns the module's authority address
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger
func (k Keeper) Logger() log.Logger {
	return k.logger
}

// ========== ValidatorMultiplier CRUD ==========

// SetValidatorMultiplier stores a validator's multiplier state
func (k Keeper) SetValidatorMultiplier(ctx context.Context, vm types.ValidatorMultiplier) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(vm)
	if err != nil {
		return fmt.Errorf("failed to marshal validator multiplier: %w", err)
	}
	key := types.GetValidatorMultiplierKey(vm.ValidatorAddress)
	return kvStore.Set(key, bz)
}

// GetValidatorMultiplier retrieves a validator's multiplier state
func (k Keeper) GetValidatorMultiplier(ctx context.Context, valAddr string) (types.ValidatorMultiplier, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetValidatorMultiplierKey(valAddr)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.ValidatorMultiplier{}, false
	}
	var vm types.ValidatorMultiplier
	if err := json.Unmarshal(bz, &vm); err != nil {
		k.logger.Error("failed to unmarshal validator multiplier", "error", err)
		return types.ValidatorMultiplier{}, false
	}
	return vm, true
}

// GetAllValidatorMultipliers returns all stored multipliers
func (k Keeper) GetAllValidatorMultipliers(ctx context.Context) []types.ValidatorMultiplier {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(
		types.KeyPrefixValidatorMultiplier,
		storetypes.PrefixEndBytes(types.KeyPrefixValidatorMultiplier),
	)
	if err != nil {
		k.logger.Error("failed to create multiplier iterator", "error", err)
		return nil
	}
	defer iter.Close()

	var multipliers []types.ValidatorMultiplier
	for ; iter.Valid(); iter.Next() {
		var vm types.ValidatorMultiplier
		if err := json.Unmarshal(iter.Value(), &vm); err != nil {
			k.logger.Error("failed to unmarshal multiplier during iteration", "error", err)
			continue
		}
		multipliers = append(multipliers, vm)
	}
	return multipliers
}

// GetEffectiveMultiplier returns the effective multiplier for a validator.
// Returns 1.0 (neutral) if no multiplier is stored.
func (k Keeper) GetEffectiveMultiplier(ctx context.Context, valAddr string) math.LegacyDec {
	vm, found := k.GetValidatorMultiplier(ctx, valAddr)
	if !found {
		return math.LegacyOneDec()
	}
	return vm.MEffective
}

// ========== EMA History CRUD ==========

// SetEMAHistory stores a validator's EMA history
func (k Keeper) SetEMAHistory(ctx context.Context, history types.EMAHistory) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal EMA history: %w", err)
	}
	key := types.GetEMAHistoryKey(history.ValidatorAddress)
	return kvStore.Set(key, bz)
}

// GetEMAHistory retrieves a validator's EMA history
func (k Keeper) GetEMAHistory(ctx context.Context, valAddr string) types.EMAHistory {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetEMAHistoryKey(valAddr)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.NewEMAHistory(valAddr)
	}
	var history types.EMAHistory
	if err := json.Unmarshal(bz, &history); err != nil {
		k.logger.Error("failed to unmarshal EMA history", "error", err)
		return types.NewEMAHistory(valAddr)
	}
	return history
}

// GetAllEMAHistories returns all stored EMA histories
func (k Keeper) GetAllEMAHistories(ctx context.Context) []types.EMAHistory {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(
		types.KeyPrefixEMAHistory,
		storetypes.PrefixEndBytes(types.KeyPrefixEMAHistory),
	)
	if err != nil {
		k.logger.Error("failed to create EMA history iterator", "error", err)
		return nil
	}
	defer iter.Close()

	var histories []types.EMAHistory
	for ; iter.Valid(); iter.Next() {
		var h types.EMAHistory
		if err := json.Unmarshal(iter.Value(), &h); err != nil {
			k.logger.Error("failed to unmarshal EMA history during iteration", "error", err)
			continue
		}
		histories = append(histories, h)
	}
	return histories
}

// ========== Slash Event Tracking ==========

// RecordSlashEvent records that a validator was slashed at the given epoch.
// Backward-compatible wrapper that records with InfractionUnknown type.
func (k Keeper) RecordSlashEvent(ctx context.Context, valAddr string, epoch int64) error {
	return k.RecordSlashEventWithType(ctx, valAddr, epoch, types.InfractionUnknown, math.LegacyZeroDec())
}

// RecordSlashEventWithType records a slash event with infraction classification.
func (k Keeper) RecordSlashEventWithType(ctx context.Context, valAddr string, epoch int64, infractionType string, slashFraction math.LegacyDec) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	event := types.SlashEvent{
		ValidatorAddress: valAddr,
		Epoch:            epoch,
		InfractionType:   infractionType,
		SlashFraction:    slashFraction,
	}
	bz, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal slash event: %w", err)
	}
	key := types.GetSlashEventKey(valAddr, epoch)
	return kvStore.Set(key, bz)
}

// HasSlashInLookback checks if a validator was slashed within the lookback window
func (k Keeper) HasSlashInLookback(ctx context.Context, valAddr string, currentEpoch int64, lookbackEpochs int64) bool {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetSlashEventPrefixKey(valAddr)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return false
	}
	defer iter.Close()

	earliestEpoch := currentEpoch - lookbackEpochs
	for ; iter.Valid(); iter.Next() {
		var event types.SlashEvent
		if err := json.Unmarshal(iter.Value(), &event); err != nil {
			continue
		}
		if event.Epoch >= earliestEpoch {
			return true
		}
	}
	return false
}

// SlashDecayFraction returns a [0,1] fraction indicating how much of the slash penalty
// should be applied, based on linear decay. 1.0 = just slashed, 0.0 = fully decayed.
func (k Keeper) SlashDecayFraction(ctx context.Context, valAddr string, currentEpoch int64, lookbackEpochs int64) math.LegacyDec {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetSlashEventPrefixKey(valAddr)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return math.LegacyZeroDec()
	}
	defer iter.Close()

	maxFraction := math.LegacyZeroDec()
	earliestEpoch := currentEpoch - lookbackEpochs

	for ; iter.Valid(); iter.Next() {
		var event types.SlashEvent
		if err := json.Unmarshal(iter.Value(), &event); err != nil {
			continue
		}
		if event.Epoch < earliestEpoch {
			continue
		}
		// Linear decay: (lookback - (current - slash_epoch)) / lookback
		epochsSinceSlash := currentEpoch - event.Epoch
		remaining := lookbackEpochs - epochsSinceSlash
		if remaining <= 0 {
			continue
		}
		fraction := math.LegacyNewDec(remaining).Quo(math.LegacyNewDec(lookbackEpochs))
		if fraction.GT(maxFraction) {
			maxFraction = fraction
		}
	}
	return maxFraction
}

// SlashDecayFractionByType returns a [0,1] fraction for slash events of a specific infraction type.
// Same linear decay logic as SlashDecayFraction, but filters by InfractionType.
// Events with InfractionUnknown (legacy) are included in all type queries for backward compatibility.
func (k Keeper) SlashDecayFractionByType(ctx context.Context, valAddr string, currentEpoch int64, lookbackEpochs int64, infractionType string) math.LegacyDec {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetSlashEventPrefixKey(valAddr)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return math.LegacyZeroDec()
	}
	defer iter.Close()

	maxFraction := math.LegacyZeroDec()
	earliestEpoch := currentEpoch - lookbackEpochs

	for ; iter.Valid(); iter.Next() {
		var event types.SlashEvent
		if err := json.Unmarshal(iter.Value(), &event); err != nil {
			continue
		}
		if event.Epoch < earliestEpoch {
			continue
		}
		// Filter by infraction type; include unknown (legacy) events in all queries
		if event.InfractionType != infractionType && event.InfractionType != types.InfractionUnknown && event.InfractionType != "" {
			continue
		}
		// Linear decay: (lookback - (current - slash_epoch)) / lookback
		epochsSinceSlash := currentEpoch - event.Epoch
		remaining := lookbackEpochs - epochsSinceSlash
		if remaining <= 0 {
			continue
		}
		fraction := math.LegacyNewDec(remaining).Quo(math.LegacyNewDec(lookbackEpochs))
		if fraction.GT(maxFraction) {
			maxFraction = fraction
		}
	}
	return maxFraction
}

// ========== Last Processed Epoch ==========

// SetLastProcessedEpoch stores the last processed epoch number
func (k Keeper) SetLastProcessedEpoch(ctx context.Context, epoch int64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.KeyLastProcessedEpoch, sdk.Uint64ToBigEndian(uint64(epoch)))
}

// GetLastProcessedEpoch retrieves the last processed epoch number
func (k Keeper) GetLastProcessedEpoch(ctx context.Context) int64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyLastProcessedEpoch)
	if err != nil || bz == nil {
		return 0
	}
	return int64(sdk.BigEndianToUint64(bz))
}
