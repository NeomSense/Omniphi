package types

import (
	"fmt"
	"strings"
)

// TrackName identifies a governance execution track.
// Track names are fixed strings; new tracks must be added here and to
// DefaultTracks() to preserve determinism across all validators.
type TrackName string

const (
	TrackUpgrade    TrackName = "TRACK_UPGRADE"
	TrackTreasury   TrackName = "TRACK_TREASURY"
	TrackParamChange TrackName = "TRACK_PARAM_CHANGE"
	TrackConsensus  TrackName = "TRACK_CONSENSUS"
	TrackOther      TrackName = "TRACK_OTHER"
)

// AllTrackNames returns the canonical ordered list of all tracks.
// Order is stable across builds.
func AllTrackNames() []TrackName {
	return []TrackName{
		TrackUpgrade,
		TrackTreasury,
		TrackParamChange,
		TrackConsensus,
		TrackOther,
	}
}

// Track holds per-track configuration and runtime state.
// Stored individually in KV store so governance can update a single track
// without touching others (and without changing the Params message).
type Track struct {
	// Name is the canonical track identifier (one of the TrackName constants).
	Name string `json:"name"`

	// Multiplier is applied to MinDelaySeconds as a fixed-point factor with
	// precision 1000 (i.e. Multiplier=1000 means ×1.0, 1500 means ×1.5).
	// Range: [1000, 5000]  — never below 1× base delay.
	Multiplier uint64 `json:"multiplier"`

	// Paused prevents new operations from being queued on this track.
	// Does NOT affect already-queued operations.
	Paused bool `json:"paused"`

	// MaxOutflowBps caps the daily treasury outflow that operations on this
	// track may collectively cause, in basis points of community pool balance.
	// 0 means no additional track-level cap (module-level cap still applies).
	MaxOutflowBps uint64 `json:"max_outflow_bps"`

	// FreezeUntilHeight is the block height until which execution is frozen.
	// 0 or a past height means the track is not frozen.
	// Queuing is still allowed during a freeze; only execution is blocked.
	FreezeUntilHeight int64 `json:"freeze_until_height"`
}

// IsFrozen returns true if the track is currently frozen at the given height.
func (t *Track) IsFrozen(currentHeight int64) bool {
	return t.FreezeUntilHeight > 0 && currentHeight < t.FreezeUntilHeight
}

// Validate validates track configuration.
func (t *Track) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("track name cannot be empty")
	}
	// Name must be one of the canonical names
	valid := false
	for _, n := range AllTrackNames() {
		if t.Name == string(n) {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown track name %q", t.Name)
	}
	// Multiplier: fixed-point 1000 = 1×.  Range [1000, 5000].
	if t.Multiplier < 1000 {
		return fmt.Errorf("track %s: multiplier %d must be >= 1000 (1×)", t.Name, t.Multiplier)
	}
	if t.Multiplier > 5000 {
		return fmt.Errorf("track %s: multiplier %d exceeds maximum 5000 (5×)", t.Name, t.Multiplier)
	}
	if t.MaxOutflowBps > 10000 {
		return fmt.Errorf("track %s: max_outflow_bps %d exceeds 10000 (100%%)", t.Name, t.MaxOutflowBps)
	}
	if t.FreezeUntilHeight < 0 {
		return fmt.Errorf("track %s: freeze_until_height must be >= 0", t.Name)
	}
	return nil
}

// DefaultTracks returns the initial per-track configuration used at genesis.
// These are conservative defaults; governance can adjust via MsgUpdateTrack.
func DefaultTracks() []Track {
	return []Track{
		{
			Name:              string(TrackUpgrade),
			Multiplier:        2500, // 2.5× — upgrades get the longest delay
			Paused:            false,
			MaxOutflowBps:     0,
			FreezeUntilHeight: 0,
		},
		{
			Name:              string(TrackTreasury),
			Multiplier:        1500, // 1.5×
			Paused:            false,
			MaxOutflowBps:     2000, // 20% of pool per day, track-level cap
			FreezeUntilHeight: 0,
		},
		{
			Name:              string(TrackParamChange),
			Multiplier:        1200, // 1.2×
			Paused:            false,
			MaxOutflowBps:     0,
			FreezeUntilHeight: 0,
		},
		{
			Name:              string(TrackConsensus),
			Multiplier:        3000, // 3.0× — consensus changes are highest risk
			Paused:            false,
			MaxOutflowBps:     0,
			FreezeUntilHeight: 0,
		},
		{
			Name:              string(TrackOther),
			Multiplier:        1000, // 1.0× — baseline, no extra delay
			Paused:            false,
			MaxOutflowBps:     0,
			FreezeUntilHeight: 0,
		},
	}
}

// ClassifyTrackByMessageTypes derives the execution track from the message type
// URLs in a governance proposal. Rules are applied in priority order (first match
// wins) so that the highest-risk track is always chosen when messages are mixed.
//
// Determinism guarantee: uses only string operations on the type URLs, no
// external state. Every validator will compute the same track for the same
// proposal.
func ClassifyTrackByMessageTypes(messageTypeURLs []string) TrackName {
	if len(messageTypeURLs) == 0 {
		// Text-only proposals are skipped by timelock before queuing, but
		// if somehow called, assign TRACK_OTHER (lowest risk).
		return TrackOther
	}

	// Priority order: UPGRADE > CONSENSUS > TREASURY > PARAM_CHANGE > OTHER
	hasUpgrade := false
	hasConsensus := false
	hasTreasury := false
	hasParamChange := false

	for _, url := range messageTypeURLs {
		lower := strings.ToLower(url)

		// Upgrade
		if strings.Contains(lower, "msgsoftwareupgrade") ||
			strings.Contains(lower, "/upgrade.") ||
			strings.Contains(lower, "msgcancelusoftwareupgrade") {
			hasUpgrade = true
		}

		// Consensus-critical: staking (validator set), consensus params, slashing
		if strings.Contains(lower, "/cosmos.staking.") ||
			strings.Contains(lower, "/cosmos.consensus.") ||
			strings.Contains(lower, "/cosmos.slashing.") ||
			strings.Contains(lower, "consensus") {
			hasConsensus = true
		}

		// Treasury: community pool spend, bank send from gov, distribution
		if strings.Contains(lower, "msgcommunityPoolspend") ||
			strings.Contains(lower, "msgcommunitypoolspend") ||
			strings.Contains(lower, "/cosmos.distribution.") ||
			(strings.Contains(lower, "/cosmos.bank.") && strings.Contains(lower, "msgsend")) {
			hasTreasury = true
		}

		// Param change: any MsgUpdateParams not already classified above
		if strings.Contains(lower, "msgupdateparams") ||
			strings.Contains(lower, "parameterchangeproposal") {
			hasParamChange = true
		}
	}

	// Apply priority: highest risk wins
	if hasUpgrade {
		return TrackUpgrade
	}
	if hasConsensus {
		return TrackConsensus
	}
	if hasTreasury {
		return TrackTreasury
	}
	if hasParamChange {
		return TrackParamChange
	}
	return TrackOther
}

// TreasuryOutflowWindow holds a rolling 24-hour treasury outflow record.
// Stored in KV store keyed by WindowStartUnix.
type TreasuryOutflowWindow struct {
	// WindowStartUnix is the Unix timestamp when this window opened.
	WindowStartUnix int64 `json:"window_start_unix"`
	// TotalOutflowBps is the cumulative treasury outflow in this window,
	// expressed in basis points of the community pool balance at window open.
	TotalOutflowBps uint64 `json:"total_outflow_bps"`
	// LastUpdateHeight is the block height of the last update (for audit).
	LastUpdateHeight int64 `json:"last_update_height"`
}

// OperationTrackRecord maps an operation ID to its resolved track.
// Stored at QueueTime so we never re-classify (determinism guarantee).
type OperationTrackRecord struct {
	OperationID uint64    `json:"operation_id"`
	TrackName   string    `json:"track_name"`
	// ComputedDelaySeconds is the final adaptive delay applied to this operation.
	ComputedDelaySeconds uint64 `json:"computed_delay_seconds"`
}
