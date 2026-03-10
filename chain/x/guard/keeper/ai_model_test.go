package keeper_test

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"pos/x/guard/types"
)

// ============================================================================
// Layer 2: Deterministic AI Model Tests
// ============================================================================

// TestLinearModel_Determinism tests that model predictions are deterministic
func TestLinearModel_Determinism(t *testing.T) {
	model := types.DefaultLinearModel()

	// Software upgrade features
	features := []int32{1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0}

	// Run prediction 100 times — must always get the same result
	first, err := model.InferRiskScore(features)
	require.NoError(t, err)

	for i := 1; i < 100; i++ {
		score, err := model.InferRiskScore(features)
		require.NoError(t, err)
		require.Equal(t, first, score, "run %d: score must be deterministic", i)
	}

	t.Logf("Deterministic prediction: %d (all 100 runs matched)", first)
}

// TestLinearModel_GoldenVectors tests against known golden vectors.
// These are exact values, not approximate — any change to weights or formula
// must update this table.
func TestLinearModel_GoldenVectors(t *testing.T) {
	model := types.DefaultLinearModel()

	testCases := []struct {
		name          string
		features      []int32
		expectedScore uint32
		expectedTier  types.RiskTier
	}{
		{
			name: "ZeroFeatures_TextOnly",
			// All zeros → raw=0, score = clamp(0/10000 + 50, 0, 100) = 50
			features:      []int32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expectedScore: 50,
			expectedTier:  types.RISK_TIER_MED,
		},
		{
			name: "SoftwareUpgrade_WithConsensus",
			// is_upgrade=1, touches_consensus=1
			// raw = 35000 + 15000 = 50000, score = 50000/10000 + 50 = 55
			features:      []int32{1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0},
			expectedScore: 55,
			expectedTier:  types.RISK_TIER_MED,
		},
		{
			name: "SoftwareUpgrade_Full",
			// is_upgrade=1, modules=3, touches_consensus=1, changes_val_rules=1
			// raw = 35000 + 2000*3 + 15000 + 8000 = 64000, score = 64000/10000 + 50 = 56
			features:      []int32{1, 0, 0, 0, 0, 0, 0, 3, 1, 0, 1},
			expectedScore: 56,
			expectedTier:  types.RISK_TIER_HIGH,
		},
		{
			name: "TreasurySpend_100Percent",
			// is_treasury_spend=1, treasury_spend_bps=10000
			// raw = 5000 + 3*10000 = 35000, score = 35000/10000 + 50 = 53
			features:      []int32{0, 0, 1, 0, 0, 0, 10000, 0, 0, 0, 0},
			expectedScore: 53,
			expectedTier:  types.RISK_TIER_MED,
		},
		{
			name: "SlashingChange_WithConsensusAndReduces",
			// is_slashing_change=1, touches_consensus=1, reduces_slashing=1
			// raw = 20000 + 15000 + 10000 = 45000, score = 45000/10000 + 50 = 54
			features:      []int32{0, 0, 0, 1, 0, 0, 0, 0, 1, 1, 0},
			expectedScore: 54,
			expectedTier:  types.RISK_TIER_MED,
		},
		{
			name: "ParamChange_Only",
			// is_param_change=1
			// raw = 10000, score = 10000/10000 + 50 = 51
			features:      []int32{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expectedScore: 51,
			expectedTier:  types.RISK_TIER_MED,
		},
		{
			name: "MaxConstraint_AllFeatures",
			// is_upgrade=1, bps=10000, modules=50, consensus=1, reduces_slashing=1, changes_val=1
			// raw = 35000 + 3*10000 + 2000*50 + 15000 + 10000 + 8000
			//     = 35000 + 30000 + 100000 + 15000 + 10000 + 8000 = 198000
			// score = 198000/10000 + 50 = 19+50 = 69
			features:      []int32{1, 0, 0, 0, 0, 0, 10000, 50, 1, 1, 1},
			expectedScore: 69,
			expectedTier:  types.RISK_TIER_HIGH,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := model.InferRiskScore(tc.features)
			require.NoError(t, err)
			require.Equal(t, tc.expectedScore, score,
				"score mismatch for %s", tc.name)
			require.Equal(t, tc.expectedTier, types.ScoreToTier(score),
				"tier mismatch for %s", tc.name)
		})
	}
}

// TestLinearModel_ScoreClamp tests that scores are clamped to [0, 100]
func TestLinearModel_ScoreClamp(t *testing.T) {
	// Model with extremely large weights to force clamping
	model := types.LinearScoringModel{
		Weights:           []int32{1000000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Bias:              0,
		Scale:             1,
		ModelVersion:      "test-clamp",
		FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
	}

	// Large positive → clamp to 100
	score, err := model.InferRiskScore([]int32{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	require.NoError(t, err)
	require.Equal(t, uint32(100), score, "should clamp to 100")

	// Large negative → clamp to 0
	modelNeg := types.LinearScoringModel{
		Weights:           []int32{-1000000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Bias:              0,
		Scale:             1,
		ModelVersion:      "test-clamp-neg",
		FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
	}
	score, err = modelNeg.InferRiskScore([]int32{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	require.NoError(t, err)
	require.Equal(t, uint32(0), score, "should clamp to 0")
}

// TestLinearModel_Validation tests model validation
func TestLinearModel_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		model       types.LinearScoringModel
		expectError bool
	}{
		{
			name:        "Valid_Default",
			model:       types.DefaultLinearModel(),
			expectError: false,
		},
		{
			name: "Invalid_WrongWeightCount",
			model: types.LinearScoringModel{
				Weights:           []int32{1, 2, 3}, // wrong count
				Bias:              0,
				Scale:             10000,
				ModelVersion:      "test",
				FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
			},
			expectError: true,
		},
		{
			name: "Invalid_ZeroScale",
			model: types.LinearScoringModel{
				Weights:           make([]int32, types.NumFeaturesV1),
				Bias:              0,
				Scale:             0,
				ModelVersion:      "test",
				FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
			},
			expectError: true,
		},
		{
			name: "Invalid_EmptyModelVersion",
			model: types.LinearScoringModel{
				Weights:           make([]int32, types.NumFeaturesV1),
				Bias:              0,
				Scale:             10000,
				ModelVersion:      "",
				FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
			},
			expectError: true,
		},
		{
			name: "Invalid_SchemaHashMismatch",
			model: types.LinearScoringModel{
				Weights:           make([]int32, types.NumFeaturesV1),
				Bias:              0,
				Scale:             10000,
				ModelVersion:      "test",
				FeatureSchemaHash: "wrong_hash",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.model.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFeatureVectorV1_Validation tests feature vector validation
func TestFeatureVectorV1_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		fv          types.FeatureVectorV1
		expectError bool
	}{
		{
			name:        "Valid_AllZero",
			fv:          types.FeatureVectorV1{},
			expectError: false,
		},
		{
			name:        "Valid_SoftwareUpgrade",
			fv:          types.FeatureVectorV1{IsUpgrade: 1, ModulesTouchedCount: 5},
			expectError: false,
		},
		{
			name:        "Valid_TreasurySpend",
			fv:          types.FeatureVectorV1{IsTreasurySpend: 1, TreasurySpendBps: 2500},
			expectError: false,
		},
		{
			name:        "Invalid_TwoTypesSet",
			fv:          types.FeatureVectorV1{IsUpgrade: 1, IsTreasurySpend: 1},
			expectError: true,
		},
		{
			name:        "Invalid_BoolOutOfRange",
			fv:          types.FeatureVectorV1{IsUpgrade: 2},
			expectError: true,
		},
		{
			name:        "Invalid_TreasuryBpsOutOfRange",
			fv:          types.FeatureVectorV1{IsTreasurySpend: 1, TreasurySpendBps: 11000},
			expectError: true,
		},
		{
			name:        "Invalid_ModulesTouchedOutOfRange",
			fv:          types.FeatureVectorV1{ModulesTouchedCount: 51},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fv.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFeatureSchemaHash_Stability tests that schema hash is deterministic
func TestFeatureSchemaHash_Stability(t *testing.T) {
	hash1 := types.ComputeFeatureSchemaHash()
	hash2 := types.ComputeFeatureSchemaHash()
	hash3 := types.ComputeFeatureSchemaHash()

	require.Equal(t, hash1, hash2, "Schema hashes must be identical (run 1 vs 2)")
	require.Equal(t, hash2, hash3, "Schema hashes must be identical (run 2 vs 3)")

	// SHA256 hex = 64 characters
	require.Len(t, hash1, 64, "Schema hash must be 64 hex characters")

	t.Logf("Schema hash: %s", hash1)
}

// TestScoreToTier tests score-to-tier conversion with spec thresholds
func TestScoreToTier(t *testing.T) {
	testCases := []struct {
		score        uint32
		expectedTier types.RiskTier
	}{
		{0, types.RISK_TIER_LOW},
		{10, types.RISK_TIER_LOW},
		{30, types.RISK_TIER_LOW},
		{31, types.RISK_TIER_MED},
		{42, types.RISK_TIER_MED},
		{55, types.RISK_TIER_MED},
		{56, types.RISK_TIER_HIGH},
		{70, types.RISK_TIER_HIGH},
		{80, types.RISK_TIER_HIGH},
		{81, types.RISK_TIER_CRITICAL},
		{90, types.RISK_TIER_CRITICAL},
		{100, types.RISK_TIER_CRITICAL},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedTier.GetTierName(), func(t *testing.T) {
			tier := types.ScoreToTier(tc.score)
			require.Equal(t, tc.expectedTier, tier, "Score %d should map to %s", tc.score, tc.expectedTier)
		})
	}
}

// TestScoreToTier_Boundaries tests exact boundary points
func TestScoreToTier_Boundaries(t *testing.T) {
	// LOW: 0-30
	require.Equal(t, types.RISK_TIER_LOW, types.ScoreToTier(30))
	require.Equal(t, types.RISK_TIER_MED, types.ScoreToTier(31))

	// MED: 31-55
	require.Equal(t, types.RISK_TIER_MED, types.ScoreToTier(55))
	require.Equal(t, types.RISK_TIER_HIGH, types.ScoreToTier(56))

	// HIGH: 56-80
	require.Equal(t, types.RISK_TIER_HIGH, types.ScoreToTier(80))
	require.Equal(t, types.RISK_TIER_CRITICAL, types.ScoreToTier(81))
}

// TestWeightsHash_Stability tests weights hash determinism
func TestWeightsHash_Stability(t *testing.T) {
	model := types.DefaultLinearModel()

	hash1 := model.ComputeWeightsHash()
	hash2 := model.ComputeWeightsHash()

	require.Equal(t, hash1, hash2, "Weights hash must be deterministic")
	require.Len(t, hash1, 64, "Weights hash must be 64 hex characters")
}

// TestFeaturesHash_Stability tests feature hash determinism
func TestFeaturesHash_Stability(t *testing.T) {
	fv := types.FeatureVectorV1{
		IsUpgrade:                1,
		TouchesConsensusCritical: 1,
		ModulesTouchedCount:      5,
	}

	hash1 := fv.ComputeFeaturesHash()
	hash2 := fv.ComputeFeaturesHash()

	require.Equal(t, hash1, hash2, "Features hash must be deterministic")
	require.Len(t, hash1, 64, "Features hash must be 64 hex characters")
}

// TestCanonicalSerialization tests deterministic serialization
func TestCanonicalSerialization(t *testing.T) {
	fv := types.FeatureVectorV1{
		IsUpgrade:           1,
		ModulesTouchedCount: 3,
	}

	bytes1 := fv.SerializeCanonical()
	bytes2 := fv.SerializeCanonical()

	require.Equal(t, bytes1, bytes2, "Serialization must be deterministic")
	require.Len(t, bytes1, types.NumFeaturesV1*4, "Serialization must be %d bytes", types.NumFeaturesV1*4)
}

// TestAIModelMetadata_Validation tests metadata validation
func TestAIModelMetadata_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		metadata    types.AIModelMetadata
		expectError bool
	}{
		{
			name: "Valid",
			metadata: types.AIModelMetadata{
				ModelVersion:      "linear-v1",
				WeightsHash:       "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
				FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
				ActivatedHeight:   1000,
			},
			expectError: false,
		},
		{
			name: "Invalid_EmptyModelVersion",
			metadata: types.AIModelMetadata{
				ModelVersion:      "",
				WeightsHash:       "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
				FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
			},
			expectError: true,
		},
		{
			name: "Invalid_EmptyWeightsHash",
			metadata: types.AIModelMetadata{
				ModelVersion:      "test",
				WeightsHash:       "",
				FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
			},
			expectError: true,
		},
		{
			name: "Invalid_EmptySchemaHash",
			metadata: types.AIModelMetadata{
				ModelVersion: "test",
				WeightsHash:  "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.metadata.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFuzz_InferRiskScore_Determinism uses seeded random features to verify
// that inference is always deterministic regardless of input.
func TestFuzz_InferRiskScore_Determinism(t *testing.T) {
	model := types.DefaultLinearModel()
	rng := rand.New(rand.NewSource(42)) // deterministic seed

	for i := 0; i < 1000; i++ {
		features := make([]int32, types.NumFeaturesV1)

		// Generate random valid features
		// One-hot: pick at most one type
		typeIdx := rng.Intn(7) // 0-5 = set one type, 6 = none
		if typeIdx < 6 {
			features[typeIdx] = 1
		}

		// treasury_spend_bps: 0-10000
		features[6] = rng.Int31n(10001)
		// modules_touched_count: 0-50
		features[7] = rng.Int31n(51)
		// boolean features
		features[8] = rng.Int31n(2)
		features[9] = rng.Int31n(2)
		features[10] = rng.Int31n(2)

		score1, err1 := model.InferRiskScore(features)
		score2, err2 := model.InferRiskScore(features)

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.Equal(t, score1, score2, "fuzz iteration %d: scores must match", i)

		// Score must be in [0, 100]
		require.LessOrEqual(t, score1, uint32(100), "fuzz iteration %d: score must be <= 100", i)
	}
}

// TestMaxConstraint_NeverReduces verifies the constitutional invariant:
// merging AI with rules must never produce a weaker enforcement.
func TestMaxConstraint_NeverReduces(t *testing.T) {
	rng := rand.New(rand.NewSource(99))

	for i := 0; i < 500; i++ {
		rulesDelay := uint64(rng.Int63n(100000))
		aiDelay := uint64(rng.Int63n(100000))
		rulesThresh := uint64(rng.Int63n(10000))
		aiThresh := uint64(rng.Int63n(10000))

		finalDelay := types.MaxConstraint(rulesDelay, aiDelay)
		finalThresh := types.MaxConstraint(rulesThresh, aiThresh)

		require.GreaterOrEqual(t, finalDelay, rulesDelay,
			"iteration %d: final delay must >= rules delay", i)
		require.GreaterOrEqual(t, finalThresh, rulesThresh,
			"iteration %d: final threshold must >= rules threshold", i)
	}

	// Also test MaxConstraintTier
	tiers := []types.RiskTier{
		types.RISK_TIER_LOW,
		types.RISK_TIER_MED,
		types.RISK_TIER_HIGH,
		types.RISK_TIER_CRITICAL,
	}

	for _, r := range tiers {
		for _, a := range tiers {
			final := types.MaxConstraintTier(r, a)
			require.GreaterOrEqual(t, int32(final), int32(r),
				"MaxConstraintTier(%s, %s) must >= rules tier", r, a)
		}
	}
}

// TestDefaultLinearModel_Validate ensures default model passes validation
func TestDefaultLinearModel_Validate(t *testing.T) {
	model := types.DefaultLinearModel()
	require.NoError(t, model.Validate(), "default model must pass validation")
	require.Len(t, model.Weights, types.NumFeaturesV1)
	require.Equal(t, int32(10000), model.Scale)
	require.Equal(t, "linear-v1", model.ModelVersion)
}

// TestInferRiskScore_FeatureCountMismatch tests error on wrong feature count
func TestInferRiskScore_FeatureCountMismatch(t *testing.T) {
	model := types.DefaultLinearModel()

	_, err := model.InferRiskScore([]int32{1, 2, 3})
	require.Error(t, err)
	require.Contains(t, err.Error(), "feature count")
}

// ============================================================================
// Cross-validation: Load golden_vectors.json from Python training pipeline
// ============================================================================

// Relative paths from this test file to ai/governance_model/v1/
const (
	goldenVectorsRelPath  = "../../../../ai/governance_model/v1/golden_vectors.json"
	modelWeightsRelPath   = "../../../../ai/governance_model/v1/model_weights_int.json"
)

type goldenVectorJSON struct {
	Name          string  `json:"name"`
	Features      []int32 `json:"features"`
	Raw           int64   `json:"raw"`
	RawDiv        int64   `json:"raw_div"`
	ExpectedScore int     `json:"expected_score"`
	ExpectedTier  string  `json:"expected_tier"`
	HighRisk      int     `json:"high_risk"`
	FeaturesHash  string  `json:"features_hash"`
}

type goldenVectorsFileJSON struct {
	SchemaHash string             `json:"schema_hash"`
	NumVectors int                `json:"num_vectors"`
	Vectors    []goldenVectorJSON `json:"vectors"`
}

// modelWeightsJSON supports both new (weights_int/bias_int/schema_hash_sha256)
// and backward-compat (weights/bias/feature_schema_hash) field names.
type modelWeightsJSON struct {
	ModelVersion string  `json:"model_version"`
	Scale        int32   `json:"scale"`
	// New field names
	WeightsInt      []int32 `json:"weights_int"`
	BiasInt         int32   `json:"bias_int"`
	SchemaHashSHA   string  `json:"schema_hash_sha256"`
	// Backward-compat field names
	Weights         []int32 `json:"weights"`
	Bias            int32   `json:"bias"`
	FeatureSchemaHash string `json:"feature_schema_hash"`
	WeightsHash     string  `json:"weights_hash"`
}

// getWeights returns the weights from the JSON, preferring weights_int over weights.
func (m *modelWeightsJSON) getWeights() []int32 {
	if len(m.WeightsInt) > 0 {
		return m.WeightsInt
	}
	return m.Weights
}

// getBias returns the bias, preferring bias_int over bias.
func (m *modelWeightsJSON) getBias() int32 {
	if m.BiasInt != 0 {
		return m.BiasInt
	}
	return m.Bias
}

// getSchemaHash returns the schema hash, preferring schema_hash_sha256 over feature_schema_hash.
func (m *modelWeightsJSON) getSchemaHash() string {
	if m.SchemaHashSHA != "" {
		return m.SchemaHashSHA
	}
	return m.FeatureSchemaHash
}

// loadModelFromJSON loads a LinearScoringModel from model_weights_int.json.
func loadModelFromJSON(t *testing.T) (types.LinearScoringModel, modelWeightsJSON) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(1)
	require.True(t, ok)
	modelPath := filepath.Join(filepath.Dir(thisFile), modelWeightsRelPath)

	data, err := os.ReadFile(modelPath)
	if err != nil {
		t.Skipf("model_weights_int.json not found at %s — run export_int_weights.py first", modelPath)
	}

	var mj modelWeightsJSON
	require.NoError(t, json.Unmarshal(data, &mj))

	weights := mj.getWeights()
	require.Len(t, weights, types.NumFeaturesV1, "model must have %d weights", types.NumFeaturesV1)

	model := types.LinearScoringModel{
		Weights:           weights,
		Bias:              mj.getBias(),
		Scale:             mj.Scale,
		ModelVersion:      mj.ModelVersion,
		FeatureSchemaHash: types.ComputeFeatureSchemaHash(),
	}

	return model, mj
}

var tierNameMap = map[string]types.RiskTier{
	"LOW":      types.RISK_TIER_LOW,
	"MED":      types.RISK_TIER_MED,
	"HIGH":     types.RISK_TIER_HIGH,
	"CRITICAL": types.RISK_TIER_CRITICAL,
}

// TestGoldenVectors_CrossValidation loads golden_vectors.json and model_weights_int.json
// from the Python training pipeline and verifies every vector matches Go InferRiskScore.
// This is the definitive cross-implementation agreement test.
func TestGoldenVectors_CrossValidation(t *testing.T) {
	// Load model weights from JSON
	model, _ := loadModelFromJSON(t)

	// Load golden vectors
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed")
	goldenPath := filepath.Join(filepath.Dir(thisFile), goldenVectorsRelPath)

	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skipf("golden_vectors.json not found at %s — run export_int_weights.py first", goldenPath)
		return
	}

	var gvFile goldenVectorsFileJSON
	require.NoError(t, json.Unmarshal(data, &gvFile), "failed to parse golden_vectors.json")
	require.Greater(t, gvFile.NumVectors, 0, "golden_vectors.json must contain vectors")
	require.Equal(t, gvFile.NumVectors, len(gvFile.Vectors), "num_vectors must match actual count")

	// Schema hash must match Go
	goSchemaHash := types.ComputeFeatureSchemaHash()
	require.Equal(t, goSchemaHash, gvFile.SchemaHash,
		"schema hash in golden_vectors.json must match Go ComputeFeatureSchemaHash()")

	for _, gv := range gvFile.Vectors {
		t.Run(gv.Name, func(t *testing.T) {
			require.Len(t, gv.Features, types.NumFeaturesV1,
				"feature vector must have %d elements", types.NumFeaturesV1)

			// Inference
			score, err := model.InferRiskScore(gv.Features)
			require.NoError(t, err)
			require.Equal(t, uint32(gv.ExpectedScore), score,
				"score mismatch: Python=%d, Go=%d", gv.ExpectedScore, score)

			// Tier
			expectedTier, ok := tierNameMap[gv.ExpectedTier]
			require.True(t, ok, "unknown tier name: %s", gv.ExpectedTier)
			goTier := types.ScoreToTier(score)
			require.Equal(t, expectedTier, goTier,
				"tier mismatch: Python=%s, Go=%s", gv.ExpectedTier, goTier)

			// high_risk flag
			goHighRisk := 0
			if goTier == types.RISK_TIER_HIGH || goTier == types.RISK_TIER_CRITICAL {
				goHighRisk = 1
			}
			require.Equal(t, gv.HighRisk, goHighRisk,
				"high_risk mismatch: Python=%d, Go=%d", gv.HighRisk, goHighRisk)

			// Features hash
			fv := types.FeatureVectorV1{
				IsUpgrade:                gv.Features[0],
				IsParamChange:            gv.Features[1],
				IsTreasurySpend:          gv.Features[2],
				IsSlashingChange:         gv.Features[3],
				IsPocRuleChange:          gv.Features[4],
				IsPoseqRuleChange:        gv.Features[5],
				TreasurySpendBps:         gv.Features[6],
				ModulesTouchedCount:      gv.Features[7],
				TouchesConsensusCritical: gv.Features[8],
				ReducesSlashing:          gv.Features[9],
				ChangesValidatorRules:    gv.Features[10],
			}
			goHash := fv.ComputeFeaturesHash()
			require.Equal(t, gv.FeaturesHash, goHash,
				"features hash mismatch: Python=%s, Go=%s", gv.FeaturesHash, goHash)
		})
	}

	t.Logf("Cross-validated %d golden vectors: all scores, tiers, and hashes match", len(gvFile.Vectors))
}

// TestWeightsHash_CrossValidation verifies the weights hash from model_weights_int.json
// matches Go ComputeWeightsHash() when computed from the same weights.
func TestWeightsHash_CrossValidation(t *testing.T) {
	model, mj := loadModelFromJSON(t)

	// Compute weights hash from the loaded model
	goWeightsHash := model.ComputeWeightsHash()
	goSchemaHash := types.ComputeFeatureSchemaHash()

	require.Equal(t, goWeightsHash, mj.WeightsHash,
		"weights hash mismatch between Python and Go")
	require.Equal(t, goSchemaHash, mj.getSchemaHash(),
		"schema hash mismatch between Python and Go")

	t.Logf("Cross-validated: weights_hash=%s, schema_hash=%s", goWeightsHash, goSchemaHash)
}
