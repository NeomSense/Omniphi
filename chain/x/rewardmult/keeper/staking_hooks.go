package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/rewardmult/types"
)

// StakingHooks implements stakingtypes.StakingHooks for automatic
// slash-to-multiplier recording. When a validator is slashed by the
// Cosmos SDK staking module, this hook records the event in the
// rewardmult store so the reward multiplier penalty decay can apply.
type StakingHooks struct {
	keeper *Keeper
}

// NewStakingHooks creates a new StakingHooks instance
func NewStakingHooks(k *Keeper) StakingHooks {
	return StakingHooks{keeper: k}
}

// BeforeValidatorSlashed records the slash event in rewardmult.
// Infraction type is inferred from the fraction magnitude:
//   - fraction < 1% → downtime (SDK default: 0.01% = 0.0001)
//   - fraction >= 1% → double-sign (SDK default: 5% = 0.05)
//
// This hook never returns errors to avoid blocking the slashing operation.
func (h StakingHooks) BeforeValidatorSlashed(ctx context.Context, valAddr sdk.ValAddress, fraction math.LegacyDec) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	epoch := sdkCtx.BlockHeight() / 100 // same epoch derivation as EndBlocker

	// Classify infraction by fraction magnitude
	infractionType := types.InfractionDowntime
	if fraction.GTE(math.LegacyNewDecWithPrec(1, 2)) { // >= 0.01 (1%)
		infractionType = types.InfractionDoubleSign
	}

	if err := h.keeper.RecordSlashEventWithType(ctx, valAddr.String(), epoch, infractionType, fraction); err != nil {
		h.keeper.Logger().Error("failed to record slash event in rewardmult",
			"validator", valAddr.String(),
			"fraction", fraction.String(),
			"infraction_type", infractionType,
			"error", err,
		)
		// Best-effort: never block slashing
	}

	return nil
}

// No-op implementations for remaining StakingHooks interface methods

func (h StakingHooks) AfterValidatorCreated(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) BeforeValidatorModified(_ context.Context, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) AfterValidatorRemoved(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) AfterValidatorBonded(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) AfterValidatorBeginUnbonding(_ context.Context, _ sdk.ConsAddress, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) BeforeDelegationCreated(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) BeforeDelegationSharesModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) BeforeDelegationRemoved(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) AfterDelegationModified(_ context.Context, _ sdk.AccAddress, _ sdk.ValAddress) error {
	return nil
}

func (h StakingHooks) AfterUnbondingInitiated(_ context.Context, _ uint64) error {
	return nil
}
