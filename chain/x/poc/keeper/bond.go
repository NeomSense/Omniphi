package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/types"
)

// ========== Bond Escrow ==========

// CollectDuplicateBond escrows the bond from the submitter to the PoC module account.
// The bond is held until the contribution is verified (refund) or confirmed duplicate (slash).
func (k Keeper) CollectDuplicateBond(ctx context.Context, contributor sdk.AccAddress, bond sdk.Coin) error {
	if bond.IsZero() {
		return nil
	}

	// Check balance first for a better error message
	balance := k.bankKeeper.GetBalance(ctx, contributor, bond.Denom)
	if balance.Amount.LT(bond.Amount) {
		return types.ErrInsufficientBond.Wrapf(
			"need %s but only have %s", bond, balance)
	}

	// Transfer bond from contributor to module
	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, contributor, types.ModuleName, sdk.NewCoins(bond))
	if err != nil {
		return types.ErrBondEscrowFailed.Wrapf("failed to escrow bond: %s", err)
	}

	// Record escrow
	record := types.BondEscrowRecord{
		ContributionID: 0, // Will be set by caller after contribution ID is assigned
		Contributor:    contributor.String(),
		Amount:         bond.String(),
	}

	return k.setBondEscrow(ctx, contributor.String(), record)
}

// RefundDuplicateBond returns the escrowed bond to the contributor.
// Called when a contribution is verified as original.
func (k Keeper) RefundDuplicateBond(ctx context.Context, contributor sdk.AccAddress, contributionID uint64) error {
	escrow, found := k.getBondEscrow(ctx, contributor.String())
	if !found {
		return nil // No bond to refund
	}

	coin, err := types.ParseCoinFromString(escrow.Amount)
	if err != nil {
		return types.ErrBondRefundFailed.Wrapf("failed to parse escrowed amount: %s", err)
	}

	if coin.IsZero() {
		return nil
	}

	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, contributor, sdk.NewCoins(coin))
	if err != nil {
		return types.ErrBondRefundFailed.Wrapf("failed to refund bond: %s", err)
	}

	// Remove escrow record
	return k.deleteBondEscrow(ctx, contributor.String())
}

// SlashDuplicateBond burns the escrowed bond for a confirmed duplicate.
// The bond is permanently destroyed as a penalty for duplicate submission.
func (k Keeper) SlashDuplicateBond(ctx context.Context, contributor sdk.AccAddress, contributionID uint64) error {
	escrow, found := k.getBondEscrow(ctx, contributor.String())
	if !found {
		return nil // No bond to slash
	}

	coin, err := types.ParseCoinFromString(escrow.Amount)
	if err != nil {
		return fmt.Errorf("failed to parse escrowed amount: %w", err)
	}

	if coin.IsZero() {
		return nil
	}

	// Burn the escrowed bond
	err = k.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(coin))
	if err != nil {
		return fmt.Errorf("failed to burn slashed bond: %w", err)
	}

	// Remove escrow record
	return k.deleteBondEscrow(ctx, contributor.String())
}

// SlashDuplicateBondDirect burns a specific bond amount directly (used when bond is already escrowed).
func (k Keeper) SlashDuplicateBondDirect(ctx context.Context, contributor sdk.AccAddress, bond sdk.Coin) error {
	if bond.IsZero() {
		return nil
	}

	err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(bond))
	if err != nil {
		return fmt.Errorf("failed to burn slashed bond: %w", err)
	}

	// Remove escrow record
	return k.deleteBondEscrow(ctx, contributor.String())
}

// RefundDuplicateBondDirect refunds a specific bond amount directly.
func (k Keeper) RefundDuplicateBondDirect(ctx context.Context, contributor sdk.AccAddress, bond sdk.Coin) error {
	if bond.IsZero() {
		return nil
	}

	err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, contributor, sdk.NewCoins(bond))
	if err != nil {
		return types.ErrBondRefundFailed.Wrapf("failed to refund bond: %s", err)
	}

	return k.deleteBondEscrow(ctx, contributor.String())
}

// CalculateEscalatedBond returns the bond amount with escalation for repeat offenders.
// Bond = BaseBond * (1 + N * EscalationBps / 10000)
// where N is the number of duplicates in the current epoch.
func (k Keeper) CalculateEscalatedBond(ctx context.Context, contributor sdk.AccAddress) (sdk.Coin, error) {
	params := k.GetParams(ctx)
	baseBond := params.DuplicateBond

	if baseBond.IsZero() {
		return baseBond, nil
	}

	epoch := k.GetCurrentEpoch(ctx)
	dupCount, err := k.GetDuplicateCount(ctx, contributor.String(), epoch)
	if err != nil {
		return sdk.Coin{}, err
	}

	if dupCount == 0 || params.DuplicateBondEscalationBps == 0 {
		return baseBond, nil
	}

	// Calculate escalation: base * (10000 + N * escalationBps) / 10000
	escalationFactor := math.NewInt(10000).Add(
		math.NewInt(int64(dupCount)).Mul(math.NewInt(int64(params.DuplicateBondEscalationBps))),
	)
	escalatedAmount := baseBond.Amount.Mul(escalationFactor).Quo(math.NewInt(10000))

	return sdk.NewCoin(baseBond.Denom, escalatedAmount), nil
}

// ========== Bond Escrow Storage ==========

func (k Keeper) setBondEscrow(ctx context.Context, addr string, record types.BondEscrowRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBondEscrowKey(addr)

	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return store.Set(key, bz)
}

func (k Keeper) getBondEscrow(ctx context.Context, addr string) (types.BondEscrowRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBondEscrowKey(addr)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.BondEscrowRecord{}, false
	}

	var record types.BondEscrowRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.BondEscrowRecord{}, false
	}
	return record, true
}

func (k Keeper) deleteBondEscrow(ctx context.Context, addr string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetBondEscrowKey(addr)
	return store.Delete(key)
}
