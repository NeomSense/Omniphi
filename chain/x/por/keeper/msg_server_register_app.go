package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// RegisterApp handles MsgRegisterApp - registers a new application in the PoR module
func (ms msgServer) RegisterApp(goCtx context.Context, msg *types.MsgRegisterApp) (*types.MsgRegisterAppResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate the message
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	// Enforce challenge period within module bounds
	params := ms.GetParams(goCtx)
	if msg.ChallengePeriod < params.MinChallengePeriod {
		return nil, types.ErrInvalidChallengePeriod.Wrapf(
			"challenge_period %d is below minimum %d", msg.ChallengePeriod, params.MinChallengePeriod,
		)
	}
	if msg.ChallengePeriod > params.MaxChallengePeriod {
		return nil, types.ErrInvalidChallengePeriod.Wrapf(
			"challenge_period %d exceeds maximum %d", msg.ChallengePeriod, params.MaxChallengePeriod,
		)
	}

	// Check for duplicate app names
	var duplicateName bool
	_ = ms.IterateApps(goCtx, func(app types.App) bool {
		if app.Name == msg.Name && app.Status == types.AppStatusActive {
			duplicateName = true
			return true // stop iteration
		}
		return false
	})
	if duplicateName {
		return nil, types.ErrDuplicateAppName.Wrapf("name: %s", msg.Name)
	}

	// Get next app ID
	appID, err := ms.GetNextAppID(goCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next app ID: %w", err)
	}

	// Create the app
	app := types.NewApp(
		appID,
		msg.Name,
		msg.Owner,
		msg.SchemaCid,
		msg.ChallengePeriod,
		msg.MinVerifiers,
		ctx.BlockTime().Unix(),
	)

	// Store the app
	if err := ms.SetApp(goCtx, app); err != nil {
		return nil, fmt.Errorf("failed to store app: %w", err)
	}

	ms.Logger().Info("app registered",
		"app_id", appID,
		"name", msg.Name,
		"owner", msg.Owner,
	)

	// Emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"por_register_app",
			sdk.NewAttribute("app_id", fmt.Sprintf("%d", appID)),
			sdk.NewAttribute("name", msg.Name),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("schema_cid", msg.SchemaCid),
			sdk.NewAttribute("challenge_period", fmt.Sprintf("%d", msg.ChallengePeriod)),
			sdk.NewAttribute("min_verifiers", fmt.Sprintf("%d", msg.MinVerifiers)),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Owner),
		),
	})

	return &types.MsgRegisterAppResponse{AppId: appID}, nil
}
