package types

// msgs_v2.go — AST v2 message types
//
// These messages are NOT protobuf-generated because we are extending the
// module without regenerating the full proto compilation chain.  They follow
// the same pattern as the existing hand-maintained types in msgs.go.
//
// Security note: Both messages are governance-only (Authority must equal the
// governance module address).  This is enforced in msg_server_v2.go, not here,
// because ValidateBasic() cannot access the module's stored authority string.

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Message type constants
const (
	TypeMsgFreezeTrack  = "freeze_track"
	TypeMsgUpdateTrack  = "update_track"
)

// ─── MsgFreezeTrack ──────────────────────────────────────────────────────────

// MsgFreezeTrack is a governance-only message that freezes execution on a
// named track until the specified block height.
//
// Invariants:
//   - Only the governance authority may submit this message.
//   - Guardian CANNOT freeze tracks (enforced in msg_server_v2.go).
//   - Queuing is still allowed during a freeze (transparency).
//   - Freeze lifts automatically when the chain passes FreezeUntilHeight.
//   - FreezeUntilHeight may not exceed current height + MaxFreezeDurationBlocks.
type MsgFreezeTrack struct {
	// Authority must be the governance module address.
	Authority string `json:"authority"`
	// TrackName is the name of the track to freeze (one of the TrackName constants).
	TrackName string `json:"track_name"`
	// FreezeUntilHeight is the exclusive upper bound: the track is frozen for
	// blocks [currentHeight, FreezeUntilHeight).
	FreezeUntilHeight int64 `json:"freeze_until_height"`
	// Reason is a human-readable justification stored in the freeze event.
	// Minimum 10 characters.
	Reason string `json:"reason"`
}

// MsgFreezeTrackResponse is the response type for MsgFreezeTrack.
type MsgFreezeTrackResponse struct{}

// Route implements sdk.Msg (legacy)
func (msg MsgFreezeTrack) Route() string { return RouterKey }

// Type implements sdk.Msg (legacy)
func (msg MsgFreezeTrack) Type() string { return TypeMsgFreezeTrack }

// ValidateBasic performs stateless validation.
func (msg MsgFreezeTrack) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("%w: invalid authority address", ErrUnauthorized)
	}
	if msg.TrackName == "" {
		return fmt.Errorf("%w: track_name cannot be empty", ErrInvalidTrackName)
	}
	// Validate against the known set
	valid := false
	for _, n := range AllTrackNames() {
		if msg.TrackName == string(n) {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("%w: unknown track %q", ErrInvalidTrackName, msg.TrackName)
	}
	if msg.FreezeUntilHeight <= 0 {
		return fmt.Errorf("freeze_until_height must be a positive block height")
	}
	if len(msg.Reason) < 10 {
		return fmt.Errorf("reason must be at least 10 characters, got %d", len(msg.Reason))
	}
	if len(msg.Reason) > 500 {
		return fmt.Errorf("reason exceeds 500 characters")
	}
	return nil
}

// GetSigners implements sdk.Msg
func (msg MsgFreezeTrack) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// ProtoMessage implements proto.Message (stub — we don't generate proto for this)
func (msg *MsgFreezeTrack) ProtoMessage() {}
func (msg *MsgFreezeTrack) Reset()        { *msg = MsgFreezeTrack{} }
func (msg *MsgFreezeTrack) String() string {
	return fmt.Sprintf("MsgFreezeTrack{authority:%s,track:%s,until:%d}",
		msg.Authority, msg.TrackName, msg.FreezeUntilHeight)
}

// ─── MsgUpdateTrack ──────────────────────────────────────────────────────────

// MsgUpdateTrack is a governance-only message that updates the configuration
// of a named track (multiplier, paused state, max outflow cap).
//
// It does NOT change FreezeUntilHeight — use MsgFreezeTrack for that.
// Idempotent: applying the same update twice has no additional effect.
type MsgUpdateTrack struct {
	// Authority must be the governance module address.
	Authority string `json:"authority"`
	// Track is the full new configuration for the named track.
	// Track.FreezeUntilHeight is ignored here (use MsgFreezeTrack).
	Track Track `json:"track"`
}

// MsgUpdateTrackResponse is the response type for MsgUpdateTrack.
type MsgUpdateTrackResponse struct{}

// Route implements sdk.Msg (legacy)
func (msg MsgUpdateTrack) Route() string { return RouterKey }

// Type implements sdk.Msg (legacy)
func (msg MsgUpdateTrack) Type() string { return TypeMsgUpdateTrack }

// ValidateBasic performs stateless validation.
func (msg MsgUpdateTrack) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("%w: invalid authority address", ErrUnauthorized)
	}
	// Validate the track configuration itself, but ignore FreezeUntilHeight
	// (cannot validate against current block height without chain state).
	t := msg.Track
	t.FreezeUntilHeight = 0 // clear for static validation
	if err := t.Validate(); err != nil {
		return err
	}
	return nil
}

// GetSigners implements sdk.Msg
func (msg MsgUpdateTrack) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// ProtoMessage implements proto.Message (stub)
func (msg *MsgUpdateTrack) ProtoMessage() {}
func (msg *MsgUpdateTrack) Reset()        { *msg = MsgUpdateTrack{} }
func (msg *MsgUpdateTrack) String() string {
	return fmt.Sprintf("MsgUpdateTrack{authority:%s,track:%s}", msg.Authority, msg.Track.Name)
}

// Ensure messages implement sdk.Msg
var (
	_ sdk.Msg = &MsgFreezeTrack{}
	_ sdk.Msg = &MsgUpdateTrack{}
)
