package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_NoKeyPrefixCollisions verifies that every exported KVStore prefix byte
// in the guard module is unique.  A collision would cause data corruption.
func Test_NoKeyPrefixCollisions(t *testing.T) {
	// Collect every exported prefix/key constant.
	// Map: prefix byte → human-readable label
	prefixes := map[byte]string{
		ParamsKey[0]:                    "ParamsKey",
		RiskReportPrefix[0]:            "RiskReportPrefix",
		QueuedExecutionPrefix[0]:       "QueuedExecutionPrefix",
		QueueIndexByHeightPrefix[0]:    "QueueIndexByHeightPrefix",
		LastProcessedProposalIDKey[0]:  "LastProcessedProposalIDKey",
		AIModelMetadataKey[0]:          "AIModelMetadataKey",
		LogisticModelKey[0]:            "LogisticModelKey",
		AIEvaluationPrefix[0]:          "AIEvaluationPrefix",
		AdvisoryLinkPrefix[0]:          "AdvisoryLinkPrefix",
		ValidatorPowerSnapshotPrefix[0]: "ValidatorPowerSnapshotPrefix",
		ExecutionMarkerPrefix[0]:       "ExecutionMarkerPrefix",
	}

	// DDG v2 keys
	ddgKeys := map[byte]string{
		ReevaluationRecordPrefix[0]:  "ReevaluationRecordPrefix",
		AggregateRiskWindowKey[0]:    "AggregateRiskWindowKey",
		EmergencyHardeningKey[0]:     "EmergencyHardeningKey",
		ThresholdEscalationPrefix[0]: "ThresholdEscalationPrefix",
	}

	// Advisory v2 keys
	advisoryKeys := map[byte]string{
		AdvisoryEntryV2Prefix[0]:        "AdvisoryEntryV2Prefix",
		AdvisoryCorrelationPrefix[0]:    "AdvisoryCorrelationPrefix",
		AttackMemoryPrefix[0]:           "AttackMemoryPrefix",
		AdvisoryIndexByTierPrefix[0]:    "AdvisoryIndexByTierPrefix",
		AdvisoryIndexByTrackPrefix[0]:   "AdvisoryIndexByTrackPrefix",
		AdvisoryIndexByOutcomePrefix[0]: "AdvisoryIndexByOutcomePrefix",
		AdvisoryCounterPrefix[0]:        "AdvisoryCounterPrefix",
		AttackMemoryByProposalPrefix[0]: "AttackMemoryByProposalPrefix",
	}

	// Integration keys
	integrationKeys := map[byte]string{
		TreasuryOutflowWindowKey[0]:    "TreasuryOutflowWindowKey",
		TimelockHandoverPrefix[0]:     "TimelockHandoverPrefix",
		ActiveExecutionIndexPrefix[0]: "ActiveExecutionIndexPrefix",
	}

	// Merge all into a single check.  If any byte collides, the Go map
	// will silently overwrite, so we detect collisions by counting.
	type entry struct {
		prefix byte
		name   string
	}

	var all []entry
	for _, m := range []map[byte]string{prefixes, ddgKeys, advisoryKeys, integrationKeys} {
		for b, n := range m {
			all = append(all, entry{b, n})
		}
	}

	seen := make(map[byte]string, len(all))
	for _, e := range all {
		if existing, dup := seen[e.prefix]; dup {
			t.Errorf("KEY PREFIX COLLISION: 0x%02X used by both %q and %q", e.prefix, existing, e.name)
		}
		seen[e.prefix] = e.name
	}

	// Sanity: we should have exactly len(all) unique prefixes
	require.Equal(t, len(all), len(seen),
		fmt.Sprintf("expected %d unique prefixes, got %d — collision detected", len(all), len(seen)))
}
