package keeper

// msg_server_v2.go — AST v2 message handlers: MsgFreezeTrack, MsgUpdateTrack
//
// Both messages are governance-only.  The guardian is explicitly blocked from
// calling FreezeTrack to prevent a single key holder from halting governance
// execution indefinitely.

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/timelock/types"
)

// FreezeTrack handles MsgFreezeTrack (governance-only).
//
// Security invariants:
//  1. Caller must be the governance module authority (not guardian).
//  2. FreezeUntilHeight must be > current block height.
//  3. Freeze duration must not exceed MaxFreezeDurationBlocks.
//  4. The frozen track still accepts new queue entries (transparency).
//  5. Execution auto-resumes once the chain passes FreezeUntilHeight.
func (ms msgServer) FreezeTrack(ctx context.Context, msg *types.MsgFreezeTrack) (*types.MsgFreezeTrackResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// SECURITY: Governance-only. Guardian may not freeze tracks.
	if msg.Authority != ms.GetAuthority() {
		return nil, fmt.Errorf("%w: FreezeTrack requires governance authority, got %s",
			types.ErrUnauthorized, msg.Authority)
	}

	// Stateless validation was already done in ValidateBasic; re-verify the
	// authority string matches the stored authority for defence-in-depth.
	ms.logger.Warn("FreezeTrack initiated by governance",
		"track", msg.TrackName,
		"freeze_until_height", msg.FreezeUntilHeight,
		"current_height", sdkCtx.BlockHeight(),
		"reason", msg.Reason,
	)

	if err := ms.Keeper.FreezeTrack(ctx, msg.TrackName, msg.FreezeUntilHeight, msg.Reason); err != nil {
		return nil, err
	}

	return &types.MsgFreezeTrackResponse{}, nil
}

// UpdateTrack handles MsgUpdateTrack (governance-only).
//
// Updates multiplier, paused state, and max_outflow_bps for a named track.
// Does not change FreezeUntilHeight (use MsgFreezeTrack for that).
//
// When Paused=true, new operations can no longer be queued on this track.
// Already-queued operations are not affected.
func (ms msgServer) UpdateTrack(ctx context.Context, msg *types.MsgUpdateTrack) (*types.MsgUpdateTrackResponse, error) {
	// SECURITY: Governance-only.
	if msg.Authority != ms.GetAuthority() {
		return nil, fmt.Errorf("%w: UpdateTrack requires governance authority, got %s",
			types.ErrUnauthorized, msg.Authority)
	}

	// Preserve the existing FreezeUntilHeight — UpdateTrack must not
	// accidentally unfreeze a track that governance froze separately.
	existing, err := ms.Keeper.GetTrack(ctx, msg.Track.Name)
	if err != nil && err.Error() != fmt.Sprintf("%s: %s", types.ErrTrackNotFound.Error(), msg.Track.Name) {
		return nil, err
	}

	updatedTrack := msg.Track
	if existing.FreezeUntilHeight > 0 {
		updatedTrack.FreezeUntilHeight = existing.FreezeUntilHeight
	}

	if err := ms.Keeper.SetTrack(ctx, updatedTrack); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ms.logger.Info("track updated by governance",
		"track", updatedTrack.Name,
		"multiplier", updatedTrack.Multiplier,
		"paused", updatedTrack.Paused,
		"max_outflow_bps", updatedTrack.MaxOutflowBps,
	)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"timelock_track_updated",
			sdk.NewAttribute("track", updatedTrack.Name),
			sdk.NewAttribute("multiplier", fmt.Sprintf("%d", updatedTrack.Multiplier)),
			sdk.NewAttribute("paused", fmt.Sprintf("%v", updatedTrack.Paused)),
			sdk.NewAttribute("max_outflow_bps", fmt.Sprintf("%d", updatedTrack.MaxOutflowBps)),
		),
	)

	return &types.MsgUpdateTrackResponse{}, nil
}
