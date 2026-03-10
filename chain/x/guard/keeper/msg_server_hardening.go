package keeper

// msg_server_hardening.go — DDG v2 message handlers
//
// Provides governance-only endpoints for:
//   - SetEmergencyHardening: toggle emergency hardening mode

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/guard/types"
)

// SetEmergencyHardening toggles the global emergency hardening mode.
// Governance-only: msg.Authority must match the module's authority address.
func (ms msgServer) SetEmergencyHardening(goCtx context.Context, authority string, enabled bool) error {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Verify authority
	if ms.GetAuthority() != authority {
		return types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			ms.GetAuthority(),
			authority,
		)
	}

	if err := ms.Keeper.SetEmergencyHardeningMode(goCtx, enabled); err != nil {
		return fmt.Errorf("failed to set emergency hardening mode: %w", err)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"guard_emergency_hardening_toggled",
		sdk.NewAttribute("authority", authority),
		sdk.NewAttribute("enabled", fmt.Sprintf("%t", enabled)),
		sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
	))

	ms.Logger().Info("emergency hardening mode toggled",
		"enabled", enabled,
		"authority", authority,
	)

	return nil
}
