package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"pos/x/poc/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	storetypes "cosmossdk.io/store/types"
)

// CreateVestingSchedule stores a new vesting schedule and updates the aggregate balance.
func (k Keeper) CreateVestingSchedule(ctx context.Context, schedule types.VestingSchedule) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.storeService.OpenKVStore(ctx)

	key := types.GetVestingScheduleKey(schedule.Contributor, schedule.ClaimID)
	bz, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	if err := store.Set(key, bz); err != nil {
		return err
	}

	// Update aggregate vesting balance
	balance := k.GetVestingBalance(ctx, schedule.Contributor)
	balance = balance.Add(schedule.TotalAmount)
	balKey := types.GetVestingBalanceKey(schedule.Contributor)
	balBz, _ := json.Marshal(balance.String())
	if err := store.Set(balKey, balBz); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"poc_vesting_created",
		sdk.NewAttribute("contributor", schedule.Contributor),
		sdk.NewAttribute("claim_id", fmt.Sprintf("%d", schedule.ClaimID)),
		sdk.NewAttribute("amount", schedule.TotalAmount.String()),
		sdk.NewAttribute("vesting_epochs", fmt.Sprintf("%d", schedule.VestingEpochs)),
	))

	return nil
}

// GetVestingSchedule retrieves a vesting schedule for a contributor and claim.
func (k Keeper) GetVestingSchedule(ctx context.Context, contributor string, claimID uint64) (types.VestingSchedule, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetVestingScheduleKey(contributor, claimID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.VestingSchedule{}, false
	}

	var schedule types.VestingSchedule
	if err := json.Unmarshal(bz, &schedule); err != nil {
		return types.VestingSchedule{}, false
	}
	return schedule, true
}

// SetVestingSchedule updates an existing vesting schedule.
func (k Keeper) SetVestingSchedule(ctx context.Context, schedule types.VestingSchedule) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetVestingScheduleKey(schedule.Contributor, schedule.ClaimID)

	bz, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	return store.Set(key, bz)
}

// DeleteVestingSchedule removes a terminal vesting schedule from the store.
// Call this when a schedule reaches Completed or ClawedBack status to prevent
// unbounded iterator growth in ProcessVestingReleases.
func (k Keeper) DeleteVestingSchedule(ctx context.Context, contributor string, claimID uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetVestingScheduleKey(contributor, claimID)
	return store.Delete(key)
}

// GetVestingBalance returns the aggregate unvested balance for a contributor.
func (k Keeper) GetVestingBalance(ctx context.Context, contributor string) math.Int {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetVestingBalanceKey(contributor)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return math.ZeroInt()
	}

	var balStr string
	if err := json.Unmarshal(bz, &balStr); err != nil {
		return math.ZeroInt()
	}
	bal, ok := math.NewIntFromString(balStr)
	if !ok {
		return math.ZeroInt()
	}
	return bal
}

// ProcessVestingReleases iterates all active vesting schedules and releases matured tranches.
// Called from EndBlocker at each epoch boundary. Never panics.
// Capped at GetMaxVestingReleasesPerEpoch() schedules per call to prevent burst stalls.
func (k Keeper) ProcessVestingReleases(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentEpoch := k.GetCurrentEpoch(ctx)
	params := k.GetParams(ctx)
	store := k.storeService.OpenKVStore(ctx)
	maxReleases := int(k.GetMaxVestingReleasesPerEpoch(ctx))
	processed := 0

	// Iterate all vesting schedules (prefix scan on 0x23)
	iterator, err := store.Iterator(
		types.KeyPrefixVestingSchedule,
		storetypes.PrefixEndBytes(types.KeyPrefixVestingSchedule),
	)
	if err != nil {
		return nil // don't panic
	}
	defer iterator.Close()

	for ; iterator.Valid() && processed < maxReleases; iterator.Next() {
		var schedule types.VestingSchedule
		if err := json.Unmarshal(iterator.Value(), &schedule); err != nil {
			continue
		}

		if schedule.Status != types.VestingStatusActive {
			continue // skip Completed, ClawedBack, and Paused schedules
		}

		// Calculate releasable amount
		elapsed := int64(currentEpoch) - int64(schedule.StartEpoch)
		if elapsed <= 0 {
			continue
		}

		if schedule.VestingEpochs <= 0 {
			schedule.VestingEpochs = 1
		}

		var totalReleasable math.Int
		if elapsed >= schedule.VestingEpochs {
			// Fully vested
			totalReleasable = schedule.TotalAmount
		} else {
			// Linear vesting: (elapsed / total) * amount
			totalReleasable = schedule.TotalAmount.Mul(math.NewInt(elapsed)).Quo(math.NewInt(schedule.VestingEpochs))
		}

		newRelease := totalReleasable.Sub(schedule.ReleasedAmount)
		if newRelease.IsZero() || newRelease.IsNegative() {
			continue
		}

		// Release funds
		recipientAddr, err := sdk.AccAddressFromBech32(schedule.Contributor)
		if err != nil {
			continue
		}

		releaseCoin := sdk.NewCoin(params.RewardDenom, newRelease)
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, sdk.NewCoins(releaseCoin)); err != nil {
			// Log and continue — don't panic in EndBlocker
			continue
		}

		schedule.ReleasedAmount = schedule.ReleasedAmount.Add(newRelease)

		// Check if fully vested
		if schedule.ReleasedAmount.GTE(schedule.TotalAmount) {
			// Terminal state: delete the record to keep the iterator bounded.
			// The completed event is emitted before deletion for audit purposes.
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"poc_vesting_completed",
				sdk.NewAttribute("contributor", schedule.Contributor),
				sdk.NewAttribute("claim_id", fmt.Sprintf("%d", schedule.ClaimID)),
				sdk.NewAttribute("total_vested", schedule.TotalAmount.String()),
			))
			_ = k.DeleteVestingSchedule(ctx, schedule.Contributor, schedule.ClaimID)
		} else {
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"poc_vesting_released",
				sdk.NewAttribute("contributor", schedule.Contributor),
				sdk.NewAttribute("claim_id", fmt.Sprintf("%d", schedule.ClaimID)),
				sdk.NewAttribute("amount", newRelease.String()),
				sdk.NewAttribute("remaining", schedule.TotalAmount.Sub(schedule.ReleasedAmount).String()),
			))
			// Still active — update in place
			if err := k.SetVestingSchedule(ctx, schedule); err != nil {
				continue
			}
		}

		// Update aggregate balance
		balance := k.GetVestingBalance(ctx, schedule.Contributor)
		balance = balance.Sub(newRelease)
		if balance.IsNegative() {
			balance = math.ZeroInt()
		}
		balKey := types.GetVestingBalanceKey(schedule.Contributor)
		balBz, _ := json.Marshal(balance.String())
		_ = store.Set(balKey, balBz)
		processed++
	}

	return nil
}

// PauseVesting freezes an active vesting schedule during an appeal/dispute.
// No-op if the schedule doesn't exist or isn't active.
func (k Keeper) PauseVesting(ctx context.Context, contributor string, claimID uint64) error {
	schedule, found := k.GetVestingSchedule(ctx, contributor, claimID)
	if !found || schedule.Status != types.VestingStatusActive {
		return nil
	}
	schedule.Status = types.VestingStatusPaused
	return k.SetVestingSchedule(ctx, schedule)
}

// ResumeVesting unfreezes a paused vesting schedule after an appeal is resolved.
// No-op if the schedule doesn't exist or isn't paused.
func (k Keeper) ResumeVesting(ctx context.Context, contributor string, claimID uint64) error {
	schedule, found := k.GetVestingSchedule(ctx, contributor, claimID)
	if !found || schedule.Status != types.VestingStatusPaused {
		return nil
	}
	schedule.Status = types.VestingStatusActive
	return k.SetVestingSchedule(ctx, schedule)
}

// ClawbackVesting marks a vesting schedule as clawed back. Unvested funds stay in module.
func (k Keeper) ClawbackVesting(ctx context.Context, contributor string, claimID uint64) (math.Int, error) {
	schedule, found := k.GetVestingSchedule(ctx, contributor, claimID)
	if !found {
		return math.ZeroInt(), types.ErrVestingNotFound
	}

	if schedule.Status != types.VestingStatusActive && schedule.Status != types.VestingStatusPaused {
		return math.ZeroInt(), nil // already completed or clawed back
	}

	// Calculate unvested amount
	unvested := schedule.TotalAmount.Sub(schedule.ReleasedAmount)
	if unvested.IsNegative() {
		unvested = math.ZeroInt()
	}

	// Terminal state: delete the record to keep ProcessVestingReleases iterator bounded.
	// Delete instead of setting ClawedBack to avoid accumulating terminal records.
	if err := k.DeleteVestingSchedule(ctx, contributor, claimID); err != nil {
		return math.ZeroInt(), err
	}

	// Update aggregate balance
	store := k.storeService.OpenKVStore(ctx)
	balance := k.GetVestingBalance(ctx, contributor)
	balance = balance.Sub(unvested)
	if balance.IsNegative() {
		balance = math.ZeroInt()
	}
	balKey := types.GetVestingBalanceKey(contributor)
	balBz, _ := json.Marshal(balance.String())
	_ = store.Set(balKey, balBz)

	return unvested, nil
}
