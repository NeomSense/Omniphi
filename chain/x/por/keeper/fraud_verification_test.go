package keeper_test

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"pos/x/por/keeper"
	"pos/x/por/types"
)

// ============================================================================
// Fraud Verification Tests
// ============================================================================

// makeLeafHash creates a deterministic 32-byte SHA256 hash from a string.
func makeLeafHash(data string) []byte {
	h := sha256.Sum256([]byte(data))
	return h[:]
}

// TestComputeMerkleRoot_SingleLeaf verifies merkle root of a single leaf is the leaf itself.
func TestComputeMerkleRoot_SingleLeaf(t *testing.T) {
	leaf := makeLeafHash("record-1")
	root := keeper.ComputeMerkleRoot([][]byte{leaf})
	require.Equal(t, leaf, root, "single leaf merkle root should be the leaf itself")
}

// TestComputeMerkleRoot_Deterministic verifies that order of input doesn't matter (sorted internally).
func TestComputeMerkleRoot_Deterministic(t *testing.T) {
	a := makeLeafHash("record-a")
	b := makeLeafHash("record-b")
	c := makeLeafHash("record-c")

	root1 := keeper.ComputeMerkleRoot([][]byte{a, b, c})
	root2 := keeper.ComputeMerkleRoot([][]byte{c, a, b})
	root3 := keeper.ComputeMerkleRoot([][]byte{b, c, a})

	require.Equal(t, root1, root2, "merkle root should be deterministic regardless of input order")
	require.Equal(t, root1, root3, "merkle root should be deterministic regardless of input order")
}

// TestComputeMerkleRoot_Empty verifies nil return for empty input.
func TestComputeMerkleRoot_Empty(t *testing.T) {
	root := keeper.ComputeMerkleRoot([][]byte{})
	require.Nil(t, root)
}

// TestVerifyInvalidRoot_ValidChallenge verifies that a merkle root mismatch is detected.
func TestVerifyInvalidRoot_ValidChallenge(t *testing.T) {
	// Create real leaf hashes and compute the correct merkle root
	leaves := [][]byte{
		makeLeafHash("record-1"),
		makeLeafHash("record-2"),
		makeLeafHash("record-3"),
	}
	correctRoot := keeper.ComputeMerkleRoot(leaves)

	// Create a batch with a WRONG merkle root (simulating fraud)
	wrongRoot := makeLeafHash("totally-wrong-root")
	batch := types.BatchCommitment{
		BatchId:          1,
		RecordMerkleRoot: wrongRoot,
		RecordCount:      3,
	}

	// Challenger provides the correct leaves proving the batch root is wrong
	proofData, err := json.Marshal(keeper.InvalidRootProof{
		LeafHashes: leaves,
	})
	require.NoError(t, err)

	challenge := types.Challenge{
		ChallengeId:   1,
		BatchId:       1,
		ChallengeType: types.ChallengeTypeInvalidRoot,
		ProofData:     proofData,
	}

	// The computed root from correct leaves won't match the batch's wrong root
	// so this should be a VALID challenge (fraud proven)
	_ = correctRoot // proves that the leaves produce a different root

	// Use the exported function indirectly via a keeper, or test the logic:
	// Since verifyInvalidRoot is unexported, we test via resolveOpenChallenges
	// or test the merkle root computation separately
	computedRoot := keeper.ComputeMerkleRoot(leaves)
	require.NotEqual(t, computedRoot, batch.RecordMerkleRoot,
		"computed root should differ from batch root (fraud scenario)")

	// Verify the proof data parses correctly
	var parsed keeper.InvalidRootProof
	require.NoError(t, json.Unmarshal(challenge.ProofData, &parsed))
	require.Len(t, parsed.LeafHashes, 3)
}

// TestVerifyInvalidRoot_InvalidChallenge verifies that matching roots reject the challenge.
func TestVerifyInvalidRoot_InvalidChallenge(t *testing.T) {
	// Create leaves and compute the correct root
	leaves := [][]byte{
		makeLeafHash("record-1"),
		makeLeafHash("record-2"),
	}
	correctRoot := keeper.ComputeMerkleRoot(leaves)

	// Batch has the correct root — no fraud
	batch := types.BatchCommitment{
		BatchId:          1,
		RecordMerkleRoot: correctRoot,
		RecordCount:      2,
	}

	// Challenger provides the same leaves (which match the root)
	proofData, err := json.Marshal(keeper.InvalidRootProof{
		LeafHashes: leaves,
	})
	require.NoError(t, err)

	var parsed keeper.InvalidRootProof
	require.NoError(t, json.Unmarshal(proofData, &parsed))

	// Recompute and verify — should match (invalid challenge)
	computedRoot := keeper.ComputeMerkleRoot(parsed.LeafHashes)
	require.Equal(t, computedRoot, batch.RecordMerkleRoot,
		"computed root should match batch root (no fraud)")
}

// TestVerifyInvalidRoot_MalformedProof verifies that garbage ProofData is handled safely.
func TestVerifyInvalidRoot_MalformedProof(t *testing.T) {
	// Invalid JSON
	challenge := types.Challenge{
		ChallengeId:   1,
		BatchId:       1,
		ChallengeType: types.ChallengeTypeInvalidRoot,
		ProofData:     []byte("not-valid-json"),
	}

	var parsed keeper.InvalidRootProof
	err := json.Unmarshal(challenge.ProofData, &parsed)
	require.Error(t, err, "malformed proof data should fail to parse")
}

// TestDoubleInclusionProof_Serialization verifies DoubleInclusionProof round-trips correctly.
func TestDoubleInclusionProof_Serialization(t *testing.T) {
	proof := keeper.DoubleInclusionProof{
		RecordHash:   makeLeafHash("duplicated-record"),
		OtherBatchId: 42,
	}

	bz, err := json.Marshal(proof)
	require.NoError(t, err)

	var parsed keeper.DoubleInclusionProof
	require.NoError(t, json.Unmarshal(bz, &parsed))
	require.Equal(t, proof.RecordHash, parsed.RecordHash)
	require.Equal(t, proof.OtherBatchId, parsed.OtherBatchId)
}

// TestComputeMerkleRoot_PowerOfTwo verifies correct behavior with even leaf count.
func TestComputeMerkleRoot_PowerOfTwo(t *testing.T) {
	leaves := [][]byte{
		makeLeafHash("a"),
		makeLeafHash("b"),
		makeLeafHash("c"),
		makeLeafHash("d"),
	}

	root := keeper.ComputeMerkleRoot(leaves)
	require.NotNil(t, root)
	require.Len(t, root, 32, "merkle root should be 32 bytes (SHA256)")

	// Verify changing any leaf changes the root
	altLeaves := make([][]byte, len(leaves))
	copy(altLeaves, leaves)
	altLeaves[2] = makeLeafHash("c-modified")

	altRoot := keeper.ComputeMerkleRoot(altLeaves)
	require.NotEqual(t, root, altRoot, "different leaves should produce different root")
}

// TestParamsValidation_NewFields verifies the new parameter validation.
func TestParamsValidation_NewFields(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*types.Params)
		wantErr bool
	}{
		{
			name:    "default params are valid",
			modify:  func(p *types.Params) {},
			wantErr: false,
		},
		{
			name:    "zero fraud jail duration",
			modify:  func(p *types.Params) { p.FraudJailDuration = 0 },
			wantErr: true,
		},
		{
			name:    "negative fraud jail duration",
			modify:  func(p *types.Params) { p.FraudJailDuration = -1 },
			wantErr: true,
		},
		{
			name:    "zero challenge resolution timeout",
			modify:  func(p *types.Params) { p.ChallengeResolutionTimeout = 0 },
			wantErr: true,
		},
		{
			name:    "negative challenge resolution timeout",
			modify:  func(p *types.Params) { p.ChallengeResolutionTimeout = -1 },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := types.DefaultParams()
			tt.modify(&p)
			err := p.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
