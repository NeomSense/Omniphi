package keeper

// tracks_test.go — internal tests for AST v2 adaptive delay logic.
// These are internal (same package) so they can call unexported helpers.
// No chain context needed — all tested functions are pure computations.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pos/x/timelock/types"
)

// ─── mulDiv ───────────────────────────────────────────────────────────────────

func TestMulDiv_Basic(t *testing.T) {
	// 86400 * 1500 / 1000 = 129600 (1.5× of 24h)
	require.Equal(t, uint64(129600), mulDiv(86400, 1500, 1000))
}

func TestMulDiv_IdentityMultiplier(t *testing.T) {
	// 1.0× should leave value unchanged
	require.Equal(t, uint64(86400), mulDiv(86400, 1000, 1000))
}

func TestMulDiv_ZeroDivisor(t *testing.T) {
	// Should return a (no panic)
	require.Equal(t, uint64(100), mulDiv(100, 5000, 0))
}

// ─── riskMultiplierForTier ────────────────────────────────────────────────────

func TestRiskMultiplierForTier(t *testing.T) {
	tests := []struct {
		tier     string
		expected uint64
	}{
		{"LOW", types.DefaultRiskMultiplierLow},
		{"RISK_TIER_LOW", types.DefaultRiskMultiplierLow},
		{"MED", types.DefaultRiskMultiplierMed},
		{"RISK_TIER_MED", types.DefaultRiskMultiplierMed},
		{"HIGH", types.DefaultRiskMultiplierHigh},
		{"RISK_TIER_HIGH", types.DefaultRiskMultiplierHigh},
		{"CRITICAL", types.DefaultRiskMultiplierCritical},
		{"RISK_TIER_CRITICAL", types.DefaultRiskMultiplierCritical},
		// Unknown → MED
		{"", types.DefaultRiskMultiplierMed},
		{"BOGUS", types.DefaultRiskMultiplierMed},
	}
	for _, tc := range tests {
		t.Run(tc.tier, func(t *testing.T) {
			got := riskMultiplierForTier(tc.tier)
			require.Equal(t, tc.expected, got)
		})
	}
}

// ─── ComputeAdaptiveDelay ─────────────────────────────────────────────────────

// makeKeeper returns a zero-value Keeper sufficient for calling ComputeAdaptiveDelay
// (which uses no store access).
func makeKeeper() Keeper {
	return Keeper{}
}

func TestComputeAdaptiveDelay_BaselineNoMultipliers(t *testing.T) {
	k := makeKeeper()
	track := types.Track{Name: string(types.TrackOther), Multiplier: 1000} // 1×
	// With all 1× multipliers and MED risk tier (1.5×), result = 86400 * 1.5 = 129600
	got := k.ComputeAdaptiveDelay(86400, "MED", 0, track, false, false)
	require.Equal(t, uint64(129600), got)
}

func TestComputeAdaptiveDelay_TrackMultiplierApplied(t *testing.T) {
	k := makeKeeper()
	track := types.Track{Name: string(types.TrackConsensus), Multiplier: 3000} // 3×
	// 86400 * 1.5 (MED) * 3.0 (consensus) = 388800
	got := k.ComputeAdaptiveDelay(86400, "MED", 0, track, false, false)
	require.Equal(t, uint64(388800), got)
}

func TestComputeAdaptiveDelay_CriticalTier(t *testing.T) {
	k := makeKeeper()
	track := types.Track{Name: string(types.TrackConsensus), Multiplier: 3000} // 3×
	// 86400 * 3.0 (CRITICAL) * 3.0 (consensus) = 777600 → clamped to 2592000? No: 777600 < 2592000
	got := k.ComputeAdaptiveDelay(86400, "CRITICAL", 0, track, false, false)
	require.Equal(t, uint64(777600), got)
}

func TestComputeAdaptiveDelay_CumulativeEscalation(t *testing.T) {
	k := makeKeeper()
	track := types.Track{Name: string(types.TrackTreasury), Multiplier: 1500} // 1.5×
	// Without escalation: 86400 * 1.5 (MED) * 1.5 (track) = 194400
	withoutEscalate := k.ComputeAdaptiveDelay(86400, "MED", 0, track, false, false)
	// With escalation: 194400 * 1.5 = 291600
	withEscalate := k.ComputeAdaptiveDelay(86400, "MED", 0, track, true, false)
	require.Greater(t, withEscalate, withoutEscalate)
	require.Equal(t, uint64(291600), withEscalate)
}

func TestComputeAdaptiveDelay_MutationFreqMultiplier(t *testing.T) {
	k := makeKeeper()
	track := types.Track{Name: string(types.TrackParamChange), Multiplier: 1200} // 1.2×
	without := k.ComputeAdaptiveDelay(86400, "MED", 0, track, false, false)
	with := k.ComputeAdaptiveDelay(86400, "MED", 0, track, false, true)
	require.Greater(t, with, without)
}

func TestComputeAdaptiveDelay_EconomicImpactMed(t *testing.T) {
	k := makeKeeper()
	track := types.Track{Name: string(types.TrackOther), Multiplier: 1000}
	// 5% treasury spend → 1.4× economic multiplier
	// 86400 * 1.5 (MED) * 1.4 (econ) * 1.0 (track) = 181440
	got := k.ComputeAdaptiveDelay(86400, "MED", 500, track, false, false)
	require.Equal(t, uint64(181440), got)
}

func TestComputeAdaptiveDelay_EconomicImpactHigh(t *testing.T) {
	k := makeKeeper()
	track := types.Track{Name: string(types.TrackOther), Multiplier: 1000}
	// 25% treasury spend → 2.0× economic multiplier
	// 86400 * 1.5 (MED) * 2.0 (econ) = 259200
	got := k.ComputeAdaptiveDelay(86400, "MED", 2500, track, false, false)
	require.Equal(t, uint64(259200), got)
}

func TestComputeAdaptiveDelay_ClampedToAbsoluteMin(t *testing.T) {
	k := makeKeeper()
	// Very small input: should be clamped up to AbsoluteMinDelaySeconds
	track := types.Track{Name: string(types.TrackOther), Multiplier: 1000}
	got := k.ComputeAdaptiveDelay(1, "LOW", 0, track, false, false)
	require.Equal(t, types.AbsoluteMinDelaySeconds, got)
}

func TestComputeAdaptiveDelay_ClampedToAbsoluteMax(t *testing.T) {
	k := makeKeeper()
	// Extremely high multipliers: should be clamped to AbsoluteMaxDelaySeconds (30d)
	track := types.Track{Name: string(types.TrackConsensus), Multiplier: 5000} // 5×
	// 2592000 * 3 * 2 * 5 * 1.5 * 1.5 = far exceeds max — clamped
	got := k.ComputeAdaptiveDelay(2592000, "CRITICAL", 2500, track, true, true)
	require.Equal(t, types.AbsoluteMaxDelaySeconds, got)
}

func TestComputeAdaptiveDelay_NeverBelowMin(t *testing.T) {
	k := makeKeeper()
	// All possible track + tier combinations must produce delay >= AbsoluteMinDelaySeconds
	tiers := []string{"LOW", "MED", "HIGH", "CRITICAL", ""}
	for _, tier := range tiers {
		for _, def := range types.DefaultTracks() {
			got := k.ComputeAdaptiveDelay(types.DefaultMinDelaySeconds, tier, 0, def, false, false)
			require.GreaterOrEqual(t, got, types.AbsoluteMinDelaySeconds,
				"delay for tier=%s track=%s must be >= AbsoluteMinDelaySeconds", tier, def.Name)
		}
	}
}

func TestComputeAdaptiveDelay_NeverExceedsMax(t *testing.T) {
	k := makeKeeper()
	tiers := []string{"LOW", "MED", "HIGH", "CRITICAL"}
	for _, tier := range tiers {
		for _, def := range types.DefaultTracks() {
			got := k.ComputeAdaptiveDelay(types.DefaultMinDelaySeconds, tier, 10000, def, true, true)
			require.LessOrEqual(t, got, types.AbsoluteMaxDelaySeconds,
				"delay for tier=%s track=%s must be <= AbsoluteMaxDelaySeconds", tier, def.Name)
		}
	}
}

// ─── TrackName constants ───────────────────────────────────────────────────────

func TestTrackConstants_NotEmpty(t *testing.T) {
	require.NotEmpty(t, string(types.TrackUpgrade))
	require.NotEmpty(t, string(types.TrackTreasury))
	require.NotEmpty(t, string(types.TrackParamChange))
	require.NotEmpty(t, string(types.TrackConsensus))
	require.NotEmpty(t, string(types.TrackOther))
}

func TestTrackConstants_Distinct(t *testing.T) {
	names := types.AllTrackNames()
	seen := make(map[types.TrackName]bool)
	for _, n := range names {
		require.False(t, seen[n], "duplicate track name: %s", n)
		seen[n] = true
	}
}

// ─── Delay parameter constants: sanity ────────────────────────────────────────

func TestDelayParameterConstants_Ordering(t *testing.T) {
	require.Less(t, types.DefaultRiskMultiplierLow, types.DefaultRiskMultiplierMed)
	require.Less(t, types.DefaultRiskMultiplierMed, types.DefaultRiskMultiplierHigh)
	require.Less(t, types.DefaultRiskMultiplierHigh, types.DefaultRiskMultiplierCritical)
}

func TestDelayParameterConstants_EconomicOrdering(t *testing.T) {
	require.LessOrEqual(t, types.DefaultEconomicImpactMultiplierBase, types.DefaultEconomicImpactMultiplierMed)
	require.LessOrEqual(t, types.DefaultEconomicImpactMultiplierMed, types.DefaultEconomicImpactMultiplierHigh)
}

func TestDelayParameterConstants_Precision(t *testing.T) {
	// DelayPrecision must equal 1000 (all multipliers use /1000 division)
	require.Equal(t, uint64(1000), types.DelayPrecision)
	// All default multipliers must be >= DelayPrecision (no multiplier reduces delay)
	require.GreaterOrEqual(t, types.DefaultRiskMultiplierLow, types.DelayPrecision)
	require.GreaterOrEqual(t, types.DefaultEconomicImpactMultiplierBase, types.DelayPrecision)
	require.GreaterOrEqual(t, types.MutationFreqMultiplier, types.DelayPrecision)
}
