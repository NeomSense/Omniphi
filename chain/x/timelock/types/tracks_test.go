package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/timelock/types"
)

// validTestAuthority returns a valid bech32 authority address for use in tests.
// Uses sdk.AccAddress to avoid dependence on chain-specific bech32 prefix config.
func validTestAuthority() string {
	return sdk.AccAddress("test-governance-auth").String()
}

// ─── Track.Validate ───────────────────────────────────────────────────────────

func TestTrackValidate_Valid(t *testing.T) {
	for _, track := range types.DefaultTracks() {
		require.NoError(t, track.Validate(), "default track %s should be valid", track.Name)
	}
}

func TestTrackValidate_UnknownName(t *testing.T) {
	bad := types.Track{Name: "TRACK_UNKNOWN", Multiplier: 1000}
	require.ErrorContains(t, bad.Validate(), "unknown track name")
}

func TestTrackValidate_MultiplierTooLow(t *testing.T) {
	bad := types.Track{Name: string(types.TrackOther), Multiplier: 999}
	require.ErrorContains(t, bad.Validate(), "multiplier")
}

func TestTrackValidate_MultiplierTooHigh(t *testing.T) {
	bad := types.Track{Name: string(types.TrackOther), Multiplier: 5001}
	require.ErrorContains(t, bad.Validate(), "multiplier")
}

func TestTrackValidate_MaxOutflowBpsAbove10000(t *testing.T) {
	bad := types.Track{Name: string(types.TrackTreasury), Multiplier: 1000, MaxOutflowBps: 10001}
	require.ErrorContains(t, bad.Validate(), "max_outflow_bps")
}

func TestTrackValidate_NegativeFreezeHeight(t *testing.T) {
	bad := types.Track{Name: string(types.TrackOther), Multiplier: 1000, FreezeUntilHeight: -1}
	require.ErrorContains(t, bad.Validate(), "freeze_until_height")
}

// ─── Track.IsFrozen ───────────────────────────────────────────────────────────

func TestTrackIsFrozen(t *testing.T) {
	t.Run("not frozen when FreezeUntilHeight is 0", func(t *testing.T) {
		track := types.Track{Name: string(types.TrackOther), Multiplier: 1000, FreezeUntilHeight: 0}
		require.False(t, track.IsFrozen(1000))
	})

	t.Run("frozen when current height < FreezeUntilHeight", func(t *testing.T) {
		track := types.Track{Name: string(types.TrackOther), Multiplier: 1000, FreezeUntilHeight: 500}
		require.True(t, track.IsFrozen(400))
	})

	t.Run("not frozen when current height >= FreezeUntilHeight", func(t *testing.T) {
		track := types.Track{Name: string(types.TrackOther), Multiplier: 1000, FreezeUntilHeight: 500}
		require.False(t, track.IsFrozen(500))
		require.False(t, track.IsFrozen(600))
	})
}

// ─── ClassifyTrackByMessageTypes ─────────────────────────────────────────────

func TestClassifyTrack_EmptyMessages(t *testing.T) {
	// Text-only proposals (shouldn't normally reach here, but must not panic)
	require.Equal(t, types.TrackOther, types.ClassifyTrackByMessageTypes(nil))
	require.Equal(t, types.TrackOther, types.ClassifyTrackByMessageTypes([]string{}))
}

func TestClassifyTrack_Upgrade(t *testing.T) {
	require.Equal(t, types.TrackUpgrade,
		types.ClassifyTrackByMessageTypes([]string{"/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade"}))
}

func TestClassifyTrack_Consensus(t *testing.T) {
	require.Equal(t, types.TrackConsensus,
		types.ClassifyTrackByMessageTypes([]string{"/cosmos.staking.v1beta1.MsgUpdateParams"}))
	require.Equal(t, types.TrackConsensus,
		types.ClassifyTrackByMessageTypes([]string{"/cosmos.slashing.v1beta1.MsgUpdateParams"}))
	require.Equal(t, types.TrackConsensus,
		types.ClassifyTrackByMessageTypes([]string{"/cosmos.consensus.v1.MsgUpdateParams"}))
}

func TestClassifyTrack_Treasury(t *testing.T) {
	require.Equal(t, types.TrackTreasury,
		types.ClassifyTrackByMessageTypes([]string{"/cosmos.distribution.v1beta1.MsgCommunityPoolSpend"}))
}

func TestClassifyTrack_ParamChange(t *testing.T) {
	require.Equal(t, types.TrackParamChange,
		types.ClassifyTrackByMessageTypes([]string{"/pos.guard.v1.MsgUpdateParams"}))
	require.Equal(t, types.TrackParamChange,
		types.ClassifyTrackByMessageTypes([]string{"/pos.timelock.v1.MsgUpdateParams"}))
}

func TestClassifyTrack_Other(t *testing.T) {
	require.Equal(t, types.TrackOther,
		types.ClassifyTrackByMessageTypes([]string{"/some.custom.v1.MsgSomething"}))
}

// Priority: UPGRADE wins over everything
func TestClassifyTrack_Priority_UpgradeBeatsConsensus(t *testing.T) {
	require.Equal(t, types.TrackUpgrade,
		types.ClassifyTrackByMessageTypes([]string{
			"/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
			"/cosmos.staking.v1beta1.MsgUpdateParams",
		}))
}

// Priority: CONSENSUS beats TREASURY
func TestClassifyTrack_Priority_ConsensusBeatesTreasury(t *testing.T) {
	require.Equal(t, types.TrackConsensus,
		types.ClassifyTrackByMessageTypes([]string{
			"/cosmos.staking.v1beta1.MsgUpdateParams",
			"/cosmos.distribution.v1beta1.MsgCommunityPoolSpend",
		}))
}

// ─── DefaultTracks ────────────────────────────────────────────────────────────

func TestDefaultTracks_AllValid(t *testing.T) {
	defaults := types.DefaultTracks()
	require.Len(t, defaults, 5, "must have exactly 5 default tracks")

	names := make(map[string]bool)
	for _, track := range defaults {
		require.NoError(t, track.Validate())
		require.False(t, names[track.Name], "duplicate track name: %s", track.Name)
		names[track.Name] = true
	}

	// Verify all canonical tracks are present
	for _, canonical := range types.AllTrackNames() {
		require.True(t, names[string(canonical)], "missing canonical track: %s", canonical)
	}
}

func TestDefaultTracks_MultipliersOrdering(t *testing.T) {
	defaults := types.DefaultTracks()
	byName := make(map[string]uint64)
	for _, t := range defaults {
		byName[t.Name] = t.Multiplier
	}

	// CONSENSUS should be ≥ UPGRADE ≥ TREASURY ≥ PARAM_CHANGE ≥ OTHER
	require.GreaterOrEqual(t, byName[string(types.TrackConsensus)], byName[string(types.TrackUpgrade)],
		"CONSENSUS multiplier should be >= UPGRADE")
	require.GreaterOrEqual(t, byName[string(types.TrackUpgrade)], byName[string(types.TrackTreasury)],
		"UPGRADE multiplier should be >= TREASURY")
	require.GreaterOrEqual(t, byName[string(types.TrackTreasury)], byName[string(types.TrackParamChange)],
		"TREASURY multiplier should be >= PARAM_CHANGE")
	require.GreaterOrEqual(t, byName[string(types.TrackParamChange)], byName[string(types.TrackOther)],
		"PARAM_CHANGE multiplier should be >= OTHER")
}

// ─── MsgFreezeTrack.ValidateBasic ─────────────────────────────────────────────

func TestMsgFreezeTrack_ValidateBasic(t *testing.T) {
	validAuth := validTestAuthority()

	tests := []struct {
		name    string
		msg     types.MsgFreezeTrack
		wantErr string
	}{
		{
			name: "valid",
			msg: types.MsgFreezeTrack{
				Authority:         validAuth,
				TrackName:         string(types.TrackUpgrade),
				FreezeUntilHeight: 1000,
				Reason:            "emergency security freeze",
			},
		},
		{
			name: "invalid authority",
			msg: types.MsgFreezeTrack{
				Authority:         "not-an-address",
				TrackName:         string(types.TrackUpgrade),
				FreezeUntilHeight: 1000,
				Reason:            "some reason here",
			},
			wantErr: "unauthorized",
		},
		{
			name: "empty track name",
			msg: types.MsgFreezeTrack{
				Authority:         validAuth,
				TrackName:         "",
				FreezeUntilHeight: 1000,
				Reason:            "some reason here",
			},
			wantErr: "track_name",
		},
		{
			name: "unknown track name",
			msg: types.MsgFreezeTrack{
				Authority:         validAuth,
				TrackName:         "TRACK_BOGUS",
				FreezeUntilHeight: 1000,
				Reason:            "some reason here",
			},
			wantErr: "unknown track",
		},
		{
			name: "zero freeze height",
			msg: types.MsgFreezeTrack{
				Authority:         validAuth,
				TrackName:         string(types.TrackOther),
				FreezeUntilHeight: 0,
				Reason:            "some reason here",
			},
			wantErr: "freeze_until_height",
		},
		{
			name: "reason too short",
			msg: types.MsgFreezeTrack{
				Authority:         validAuth,
				TrackName:         string(types.TrackOther),
				FreezeUntilHeight: 1000,
				Reason:            "short",
			},
			wantErr: "reason",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

// ─── MsgUpdateTrack.ValidateBasic ────────────────────────────────────────────

func TestMsgUpdateTrack_ValidateBasic(t *testing.T) {
	validAuth := validTestAuthority()

	tests := []struct {
		name    string
		msg     types.MsgUpdateTrack
		wantErr string
	}{
		{
			name: "valid",
			msg: types.MsgUpdateTrack{
				Authority: validAuth,
				Track: types.Track{
					Name:          string(types.TrackTreasury),
					Multiplier:    2000,
					MaxOutflowBps: 1000,
				},
			},
		},
		{
			name: "invalid authority",
			msg: types.MsgUpdateTrack{
				Authority: "bad",
				Track:     types.Track{Name: string(types.TrackOther), Multiplier: 1000},
			},
			wantErr: "unauthorized",
		},
		{
			name: "invalid multiplier",
			msg: types.MsgUpdateTrack{
				Authority: validAuth,
				Track:     types.Track{Name: string(types.TrackOther), Multiplier: 500},
			},
			wantErr: "multiplier",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
