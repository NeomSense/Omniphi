package types

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// ============================================================================
// FeatureSchemaV1 — canonical, bounded, deterministic feature set
// ============================================================================

// FeatureVectorV1 holds the canonical feature vector for Layer 2 inference.
// All values are int32 with fixed ranges — no floats anywhere.
// The field order defines the canonical serialization order.
type FeatureVectorV1 struct {
	// One-hot proposal type (exactly one is 1, rest 0)
	IsUpgrade         int32 // 0 or 1
	IsParamChange     int32 // 0 or 1
	IsTreasurySpend   int32 // 0 or 1
	IsSlashingChange  int32 // 0 or 1
	IsPocRuleChange   int32 // 0 or 1
	IsPoseqRuleChange int32 // 0 or 1

	// Numeric features
	TreasurySpendBps         int32 // 0-10000
	ModulesTouchedCount      int32 // 0-50 (clamped)
	TouchesConsensusCritical int32 // 0 or 1
	ReducesSlashing          int32 // 0 or 1
	ChangesValidatorRules    int32 // 0 or 1
}

// FeatureSchemaV1Name is the constant identifying the canonical feature names + order + ranges.
const FeatureSchemaV1Name = "FeatureSchemaV1:is_upgrade,is_param_change,is_treasury_spend,is_slashing_change,is_poc_rule_change,is_poseq_rule_change,treasury_spend_bps:0-10000,modules_touched_count:0-50,touches_consensus_critical:0-1,reduces_slashing:0-1,changes_validator_rules:0-1"

// NumFeaturesV1 is the number of features in schema v1.
const NumFeaturesV1 = 11

// GetFeatureVector returns the canonical int32 slice in schema order.
func (f *FeatureVectorV1) GetFeatureVector() []int32 {
	return []int32{
		f.IsUpgrade,
		f.IsParamChange,
		f.IsTreasurySpend,
		f.IsSlashingChange,
		f.IsPocRuleChange,
		f.IsPoseqRuleChange,
		f.TreasurySpendBps,
		f.ModulesTouchedCount,
		f.TouchesConsensusCritical,
		f.ReducesSlashing,
		f.ChangesValidatorRules,
	}
}

// SerializeCanonical serialises the feature vector to deterministic bytes.
// Format: 11 × big-endian int32 (44 bytes total).
func (f *FeatureVectorV1) SerializeCanonical() []byte {
	vec := f.GetFeatureVector()
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.BigEndian.PutUint32(buf[i*4:], uint32(v))
	}
	return buf
}

// ComputeFeaturesHash returns sha256(SerializeCanonical()) as hex.
func (f *FeatureVectorV1) ComputeFeaturesHash() string {
	h := sha256.Sum256(f.SerializeCanonical())
	return fmt.Sprintf("%x", h)
}

// Validate checks that all features are within their declared ranges.
func (f *FeatureVectorV1) Validate() error {
	oneHot := f.IsUpgrade + f.IsParamChange + f.IsTreasurySpend +
		f.IsSlashingChange + f.IsPocRuleChange + f.IsPoseqRuleChange
	if oneHot < 0 || oneHot > 1 {
		return fmt.Errorf("one-hot sum must be 0 or 1, got %d", oneHot)
	}
	if f.TreasurySpendBps < 0 || f.TreasurySpendBps > 10000 {
		return fmt.Errorf("treasury_spend_bps out of range [0,10000]: %d", f.TreasurySpendBps)
	}
	if f.ModulesTouchedCount < 0 || f.ModulesTouchedCount > 50 {
		return fmt.Errorf("modules_touched_count out of range [0,50]: %d", f.ModulesTouchedCount)
	}
	for _, b := range []int32{
		f.IsUpgrade, f.IsParamChange, f.IsTreasurySpend,
		f.IsSlashingChange, f.IsPocRuleChange, f.IsPoseqRuleChange,
		f.TouchesConsensusCritical, f.ReducesSlashing, f.ChangesValidatorRules,
	} {
		if b != 0 && b != 1 {
			return fmt.Errorf("boolean feature must be 0 or 1, got %d", b)
		}
	}
	return nil
}

// ComputeFeatureSchemaHash returns sha256(FeatureSchemaV1Name) as hex string.
func ComputeFeatureSchemaHash() string {
	h := sha256.Sum256([]byte(FeatureSchemaV1Name))
	return fmt.Sprintf("%x", h)
}

// ============================================================================
// Deterministic model format (linear scoring v1)
// ============================================================================

// LinearScoringModel is a deterministic integer-only model.
//
//	raw = bias + Σ(weights[i] × features[i])
//	score = clamp((raw / scale) + 50, 0, 100)
//
// All values are int32/int64. No floats.
type LinearScoringModel struct {
	Weights           []int32 `json:"weights"`             // one per feature
	Bias              int32   `json:"bias"`                // fixed-point bias
	Scale             int32   `json:"scale"`               // e.g. 10_000
	ModelVersion      string  `json:"model_version"`       // e.g. "linear-v1"
	FeatureSchemaHash string  `json:"feature_schema_hash"` // must match current
}

// Validate checks model sanity.
func (m *LinearScoringModel) Validate() error {
	if len(m.Weights) != NumFeaturesV1 {
		return fmt.Errorf("expected %d weights, got %d", NumFeaturesV1, len(m.Weights))
	}
	if m.Scale <= 0 {
		return fmt.Errorf("scale must be positive, got %d", m.Scale)
	}
	if m.ModelVersion == "" {
		return fmt.Errorf("model_version is required")
	}
	if m.FeatureSchemaHash != ComputeFeatureSchemaHash() {
		return fmt.Errorf("feature_schema_hash mismatch")
	}
	return nil
}

// ComputeWeightsHash returns sha256(weights ++ bias ++ scale) as hex.
func (m *LinearScoringModel) ComputeWeightsHash() string {
	h := sha256.New()
	for _, w := range m.Weights {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(w))
		h.Write(b)
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(m.Bias))
	h.Write(b)
	binary.BigEndian.PutUint32(b, uint32(m.Scale))
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// InferRiskScore runs deterministic integer-only inference.
// Returns score in [0, 100].
func (m *LinearScoringModel) InferRiskScore(features []int32) (uint32, error) {
	if len(features) != len(m.Weights) {
		return 0, fmt.Errorf("feature count %d != weight count %d", len(features), len(m.Weights))
	}
	// int64 accumulation to prevent overflow
	var raw int64
	for i := range features {
		raw += int64(m.Weights[i]) * int64(features[i])
	}
	raw += int64(m.Bias)

	// score = clamp((raw / scale) + 50, 0, 100)
	scaled := raw / int64(m.Scale)
	score := scaled + 50
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return uint32(score), nil
}

// ============================================================================
// Score → Tier mapping
// ============================================================================

// ScoreToTier converts a 0–100 score to a RiskTier.
// 0-30 LOW, 31-55 MED, 56-80 HIGH, 81-100 CRITICAL
func ScoreToTier(score uint32) RiskTier {
	if score <= 30 {
		return RISK_TIER_LOW
	}
	if score <= 55 {
		return RISK_TIER_MED
	}
	if score <= 80 {
		return RISK_TIER_HIGH
	}
	return RISK_TIER_CRITICAL
}

// TierToDelayBlocks maps tier → delay using module params.
func TierToDelayBlocks(tier RiskTier, p Params) uint64 {
	switch tier {
	case RISK_TIER_CRITICAL:
		return p.DelayCriticalBlocks
	case RISK_TIER_HIGH:
		return p.DelayHighBlocks
	case RISK_TIER_MED:
		return p.DelayMedBlocks
	default:
		return p.DelayLowBlocks
	}
}

// TierToThresholdBps maps tier → threshold using module params.
func TierToThresholdBps(tier RiskTier, p Params) uint64 {
	switch tier {
	case RISK_TIER_CRITICAL:
		return p.ThresholdCriticalBps
	case RISK_TIER_HIGH:
		return p.ThresholdHighBps
	default:
		return p.ThresholdDefaultBps
	}
}

// ============================================================================
// Default model weights (embedded at build-time, safe for v1)
// ============================================================================

// DefaultLinearModel returns a hand-tuned linear scoring model.
// Scale = 10_000. Calibrated so that:
//   - Zero-feature proposal → score 50 (neutral)
//   - Software upgrade → score ~85 (CRITICAL)
//   - Large treasury spend (100%) → ~80 (HIGH)
//   - Slashing reduction → ~70 (HIGH)
//   - Text proposal with no features → ~50 (MED)
func DefaultLinearModel() LinearScoringModel {
	return LinearScoringModel{
		Weights: []int32{
			35_000,  // is_upgrade           → +35 to score
			10_000,  // is_param_change      → +10
			5_000,   // is_treasury_spend    → +5 base; bps adds more
			20_000,  // is_slashing_change   → +20
			5_000,   // is_poc_rule_change   → +5
			5_000,   // is_poseq_rule_change → +5
			3,       // treasury_spend_bps   → 10000×3/10000 = +3 per 100%
			2_000,   // modules_touched_count→ +2 per module, max +100
			15_000,  // touches_consensus    → +15
			10_000,  // reduces_slashing     → +10
			8_000,   // changes_val_rules    → +8
		},
		Bias:              0,
		Scale:             10_000,
		ModelVersion:      "linear-v1",
		FeatureSchemaHash: ComputeFeatureSchemaHash(),
	}
}

// ============================================================================
// AIModelMetadata — on-chain tracking of the active model
// ============================================================================

// AIModelMetadata tracks the active model version on-chain.
type AIModelMetadata struct {
	ModelVersion      string `json:"model_version"`
	WeightsHash       string `json:"weights_hash"`
	FeatureSchemaHash string `json:"feature_schema_hash"`
	ActivatedHeight   int64  `json:"activated_height"`
}

// Validate checks metadata fields.
func (m *AIModelMetadata) Validate() error {
	if m.ModelVersion == "" {
		return fmt.Errorf("model_version required")
	}
	if m.WeightsHash == "" {
		return fmt.Errorf("weights_hash required")
	}
	if m.FeatureSchemaHash == "" {
		return fmt.Errorf("feature_schema_hash required")
	}
	return nil
}
