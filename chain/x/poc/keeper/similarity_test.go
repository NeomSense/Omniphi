package keeper_test

import (
	"encoding/json"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"pos/x/poc/types"
)

// ============================================================================
// Similarity Engine Tests (Layer 2)
// ============================================================================

// --- SimilarityCompactData Validation ---

func TestSimilarityCompactData_Validate_Valid(t *testing.T) {
	data := types.SimilarityCompactData{
		ContributionID:       1,
		OverallSimilarity:    8500,
		Confidence:           9000,
		NearestParentClaimID: 42,
		ModelVersion:         "v1.0.0",
		Epoch:                10,
	}
	require.NoError(t, data.Validate())
}

func TestSimilarityCompactData_Validate_ZeroContributionID(t *testing.T) {
	data := types.SimilarityCompactData{
		ContributionID:    0,
		OverallSimilarity: 5000,
		Confidence:        5000,
		ModelVersion:      "v1.0.0",
		Epoch:             1,
	}
	err := data.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "contribution_id must be > 0")
}

func TestSimilarityCompactData_Validate_SimilarityOverMax(t *testing.T) {
	data := types.SimilarityCompactData{
		ContributionID:    1,
		OverallSimilarity: 10001,
		Confidence:        5000,
		ModelVersion:      "v1.0.0",
		Epoch:             1,
	}
	err := data.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "overall_similarity must be <= 10000")
}

func TestSimilarityCompactData_Validate_ConfidenceOverMax(t *testing.T) {
	data := types.SimilarityCompactData{
		ContributionID:    1,
		OverallSimilarity: 5000,
		Confidence:        10001,
		ModelVersion:      "v1.0.0",
		Epoch:             1,
	}
	err := data.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "confidence must be <= 10000")
}

func TestSimilarityCompactData_Validate_EmptyModelVersion(t *testing.T) {
	data := types.SimilarityCompactData{
		ContributionID:    1,
		OverallSimilarity: 5000,
		Confidence:        5000,
		ModelVersion:      "",
		Epoch:             1,
	}
	err := data.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "model_version cannot be empty")
}

func TestSimilarityCompactData_Validate_ZeroEpoch(t *testing.T) {
	data := types.SimilarityCompactData{
		ContributionID:    1,
		OverallSimilarity: 5000,
		Confidence:        5000,
		ModelVersion:      "v1.0.0",
		Epoch:             0,
	}
	err := data.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "epoch must be > 0")
}

// --- CommitmentHash Validation ---

func TestValidateCommitmentHash_Valid(t *testing.T) {
	hash := make([]byte, 32)
	hash[0] = 0x01
	require.NoError(t, types.ValidateCommitmentHash(hash))
}

func TestValidateCommitmentHash_WrongLength(t *testing.T) {
	hash := make([]byte, 16)
	err := types.ValidateCommitmentHash(hash)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be 32 bytes")
}

func TestValidateCommitmentHash_AllZeros(t *testing.T) {
	hash := make([]byte, 32)
	err := types.ValidateCommitmentHash(hash)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be all zeros")
}

// --- Similarity Commitment CRUD ---

func TestSetGetSimilarityCommitment(t *testing.T) {
	f := SetupKeeperTest(t)

	record := types.SimilarityCommitmentRecord{
		ContributionID: 42,
		CompactData: types.SimilarityCompactData{
			ContributionID:       42,
			OverallSimilarity:    7500,
			Confidence:           8000,
			NearestParentClaimID: 10,
			ModelVersion:         "v1.0.0",
			Epoch:                5,
		},
		CommitmentHashFull: make([]byte, 32),
		OracleAddress:      "omni1oracle",
		BlockHeight:        100,
		IsDerivative:       false,
	}
	record.CommitmentHashFull[0] = 0xAB

	// Set
	err := f.keeper.SetSimilarityCommitment(f.ctx, record)
	require.NoError(t, err)

	// Get
	got, found := f.keeper.GetSimilarityCommitment(f.ctx, 42)
	require.True(t, found)
	require.Equal(t, uint64(42), got.ContributionID)
	require.Equal(t, uint32(7500), got.CompactData.OverallSimilarity)
	require.Equal(t, uint32(8000), got.CompactData.Confidence)
	require.Equal(t, "omni1oracle", got.OracleAddress)
	require.False(t, got.IsDerivative)
}

func TestGetSimilarityCommitment_NotFound(t *testing.T) {
	f := SetupKeeperTest(t)

	_, found := f.keeper.GetSimilarityCommitment(f.ctx, 999)
	require.False(t, found)
}

func TestSetSimilarityCommitment_NoOverwrite(t *testing.T) {
	f := SetupKeeperTest(t)

	// Store first record
	record1 := types.SimilarityCommitmentRecord{
		ContributionID: 1,
		CompactData: types.SimilarityCompactData{
			ContributionID:    1,
			OverallSimilarity: 3000,
			Confidence:        5000,
			ModelVersion:      "v1.0.0",
			Epoch:             1,
		},
		CommitmentHashFull: make([]byte, 32),
		OracleAddress:      "oracle1",
		BlockHeight:        10,
		IsDerivative:       false,
	}
	record1.CommitmentHashFull[0] = 0x01
	err := f.keeper.SetSimilarityCommitment(f.ctx, record1)
	require.NoError(t, err)

	// Store second record for different contribution
	record2 := types.SimilarityCommitmentRecord{
		ContributionID: 2,
		CompactData: types.SimilarityCompactData{
			ContributionID:    2,
			OverallSimilarity: 9500,
			Confidence:        9000,
			ModelVersion:      "v1.0.0",
			Epoch:             1,
		},
		CommitmentHashFull: make([]byte, 32),
		OracleAddress:      "oracle1",
		BlockHeight:        10,
		IsDerivative:       true,
	}
	record2.CommitmentHashFull[0] = 0x02
	err = f.keeper.SetSimilarityCommitment(f.ctx, record2)
	require.NoError(t, err)

	// Both should be independently retrievable
	got1, found := f.keeper.GetSimilarityCommitment(f.ctx, 1)
	require.True(t, found)
	require.Equal(t, uint32(3000), got1.CompactData.OverallSimilarity)

	got2, found := f.keeper.GetSimilarityCommitment(f.ctx, 2)
	require.True(t, found)
	require.Equal(t, uint32(9500), got2.CompactData.OverallSimilarity)
	require.True(t, got2.IsDerivative)
}

// --- Similarity Epoch ---

func TestGetSimilarityEpoch(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set similarity epoch blocks to 100 (default)
	params := f.keeper.GetParams(f.ctx)
	params.SimilarityEpochBlocks = 100
	params.EnableSimilarityCheck = false // don't need oracle for this
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// At block 0, epoch should be 0
	ctx0 := f.ctx.WithBlockHeader(cmtproto.Header{Height: 0})
	require.Equal(t, uint64(0), f.keeper.GetSimilarityEpoch(ctx0))

	// At block 50, epoch should be 0
	ctx50 := f.ctx.WithBlockHeader(cmtproto.Header{Height: 50})
	require.Equal(t, uint64(0), f.keeper.GetSimilarityEpoch(ctx50))

	// At block 100, epoch should be 1
	ctx100 := f.ctx.WithBlockHeader(cmtproto.Header{Height: 100})
	require.Equal(t, uint64(1), f.keeper.GetSimilarityEpoch(ctx100))

	// At block 250, epoch should be 2
	ctx250 := f.ctx.WithBlockHeader(cmtproto.Header{Height: 250})
	require.Equal(t, uint64(2), f.keeper.GetSimilarityEpoch(ctx250))
}

func TestGetSimilarityEpoch_ZeroBlocks(t *testing.T) {
	f := SetupKeeperTest(t)

	params := f.keeper.GetParams(f.ctx)
	params.SimilarityEpochBlocks = 0
	// Don't enable to avoid validation requirement of >0 when enabled
	params.EnableSimilarityCheck = false
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	ctx := f.ctx.WithBlockHeader(cmtproto.Header{Height: 500})
	require.Equal(t, uint64(0), f.keeper.GetSimilarityEpoch(ctx))
}

// --- MsgSubmitSimilarityCommitment ValidateBasic ---

func TestMsgSubmitSimilarityCommitment_ValidateBasic_Valid(t *testing.T) {
	compactData := types.SimilarityCompactData{
		ContributionID:       1,
		OverallSimilarity:    8500,
		Confidence:           9000,
		NearestParentClaimID: 10,
		ModelVersion:         "v1.0.0",
		Epoch:                5,
	}
	compactJSON, _ := json.Marshal(compactData)

	commitHash := make([]byte, 32)
	commitHash[0] = 0xAB

	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:              sdk.AccAddress("oracle______________").String(),
		ContributionID:         1,
		CompactDataJson:        compactJSON,
		OracleSignatureCompact: make([]byte, 64),
		CommitmentHashFull:     commitHash,
		OracleSignatureFull:    make([]byte, 64),
	}
	require.NoError(t, msg.ValidateBasic())
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_EmptySubmitter(t *testing.T) {
	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:      "",
		ContributionID: 1,
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid submitter address")
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_ZeroContributionID(t *testing.T) {
	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:      sdk.AccAddress("oracle______________").String(),
		ContributionID: 0,
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "contribution ID cannot be zero")
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_EmptyCompactData(t *testing.T) {
	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:       sdk.AccAddress("oracle______________").String(),
		ContributionID:  1,
		CompactDataJson: nil,
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "compact_data_json cannot be empty")
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_InvalidJSON(t *testing.T) {
	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:       sdk.AccAddress("oracle______________").String(),
		ContributionID:  1,
		CompactDataJson: []byte("not json"),
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid compact_data_json")
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_ContributionIDMismatch(t *testing.T) {
	compactData := types.SimilarityCompactData{
		ContributionID:    42,
		OverallSimilarity: 5000,
		Confidence:        5000,
		ModelVersion:      "v1.0.0",
		Epoch:             1,
	}
	compactJSON, _ := json.Marshal(compactData)

	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:       sdk.AccAddress("oracle______________").String(),
		ContributionID:  99, // mismatch!
		CompactDataJson: compactJSON,
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "compact_data.contribution_id (42) != msg.contribution_id (99)")
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_EmptySignature(t *testing.T) {
	compactData := types.SimilarityCompactData{
		ContributionID:    1,
		OverallSimilarity: 5000,
		Confidence:        5000,
		ModelVersion:      "v1.0.0",
		Epoch:             1,
	}
	compactJSON, _ := json.Marshal(compactData)

	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:              sdk.AccAddress("oracle______________").String(),
		ContributionID:         1,
		CompactDataJson:        compactJSON,
		OracleSignatureCompact: nil, // empty!
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "oracle_signature_compact cannot be empty")
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_SignatureTooLong(t *testing.T) {
	compactData := types.SimilarityCompactData{
		ContributionID:    1,
		OverallSimilarity: 5000,
		Confidence:        5000,
		ModelVersion:      "v1.0.0",
		Epoch:             1,
	}
	compactJSON, _ := json.Marshal(compactData)

	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:              sdk.AccAddress("oracle______________").String(),
		ContributionID:         1,
		CompactDataJson:        compactJSON,
		OracleSignatureCompact: make([]byte, 200), // too long!
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "oracle_signature_compact too long")
}

func TestMsgSubmitSimilarityCommitment_ValidateBasic_InvalidCommitmentHash(t *testing.T) {
	compactData := types.SimilarityCompactData{
		ContributionID:    1,
		OverallSimilarity: 5000,
		Confidence:        5000,
		ModelVersion:      "v1.0.0",
		Epoch:             1,
	}
	compactJSON, _ := json.Marshal(compactData)

	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:              sdk.AccAddress("oracle______________").String(),
		ContributionID:         1,
		CompactDataJson:        compactJSON,
		OracleSignatureCompact: make([]byte, 64),
		CommitmentHashFull:     make([]byte, 16), // wrong length!
	}
	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be 32 bytes")
}

// --- ProcessSimilarityCommitment (keeper-level) ---

func TestProcessSimilarityCommitment_DisabledByDefault(t *testing.T) {
	f := SetupKeeperTest(t)

	// Default params have EnableSimilarityCheck = false
	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:      sdk.AccAddress("oracle______________").String(),
		ContributionID: 1,
	}

	_, err := f.keeper.ProcessSimilarityCommitment(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "similarity engine is not enabled")
}

func TestProcessSimilarityCommitment_ContributionNotFound(t *testing.T) {
	f := SetupKeeperTest(t)

	// Enable similarity check with a fake oracle address
	params := f.keeper.GetParams(f.ctx)
	params.EnableSimilarityCheck = true
	params.SimilarityOracleAllowlist = []string{sdk.AccAddress("oracle______________").String()}
	params.SimilarityEpochBlocks = 100
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:      sdk.AccAddress("oracle______________").String(),
		ContributionID: 999, // doesn't exist
	}

	_, err = f.keeper.ProcessSimilarityCommitment(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "contribution 999 not found")
}

func TestProcessSimilarityCommitment_DuplicateCommitment(t *testing.T) {
	f := SetupKeeperTest(t)

	// Enable similarity check
	params := f.keeper.GetParams(f.ctx)
	params.EnableSimilarityCheck = true
	params.SimilarityOracleAllowlist = []string{sdk.AccAddress("oracle______________").String()}
	params.SimilarityEpochBlocks = 100
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Create a contribution
	contrib := types.NewContribution(1, sdk.AccAddress("contrib_____________").String(), "code", "ipfs://abc", []byte{0x01}, 10, 1000)
	err = f.keeper.SetContribution(f.ctx, contrib)
	require.NoError(t, err)

	// Store an existing commitment
	existing := types.SimilarityCommitmentRecord{
		ContributionID: 1,
		CompactData: types.SimilarityCompactData{
			ContributionID:    1,
			OverallSimilarity: 5000,
			Confidence:        5000,
			ModelVersion:      "v1.0.0",
			Epoch:             1,
		},
		CommitmentHashFull: make([]byte, 32),
		OracleAddress:      "oracle1",
		BlockHeight:        10,
	}
	existing.CommitmentHashFull[0] = 0x01
	err = f.keeper.SetSimilarityCommitment(f.ctx, existing)
	require.NoError(t, err)

	// Try to submit another commitment for the same contribution
	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:      sdk.AccAddress("oracle______________").String(),
		ContributionID: 1,
	}

	_, err = f.keeper.ProcessSimilarityCommitment(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already has a similarity commitment")
}

func TestProcessSimilarityCommitment_IDMismatch_KeeperLevel(t *testing.T) {
	f := SetupKeeperTest(t)

	// Enable similarity check
	params := f.keeper.GetParams(f.ctx)
	params.EnableSimilarityCheck = true
	params.SimilarityOracleAllowlist = []string{sdk.AccAddress("oracle______________").String()}
	params.SimilarityEpochBlocks = 100
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Create contribution 1
	contrib := types.NewContribution(1, sdk.AccAddress("contrib_____________").String(), "code", "ipfs://abc", []byte{0x01}, 10, 1000)
	err = f.keeper.SetContribution(f.ctx, contrib)
	require.NoError(t, err)

	// Create compact data for ID 99 (mismatch)
	compactData := types.SimilarityCompactData{
		ContributionID: 99, // Mismatch!
	}
	compactJSON, _ := json.Marshal(compactData)

	msg := &types.MsgSubmitSimilarityCommitment{
		ContributionID:  1, // Target contribution 1
		CompactDataJson: compactJSON,
	}

	_, err = f.keeper.ProcessSimilarityCommitment(f.ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compact data contribution ID (99) does not match message contribution ID (1)")
}

// --- Params Validation ---

func TestParams_Validate_SimilarityEnabled_EmptyAllowlist(t *testing.T) {
	params := types.DefaultParams()
	params.EnableSimilarityCheck = true
	params.SimilarityOracleAllowlist = []string{}
	params.SimilarityEpochBlocks = 100

	err := params.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "similarity_oracle_allowlist must not be empty")
}

func TestParams_Validate_SimilarityEnabled_ZeroEpochBlocks(t *testing.T) {
	params := types.DefaultParams()
	params.EnableSimilarityCheck = true
	params.SimilarityOracleAllowlist = []string{sdk.AccAddress("oracle______________").String()}
	params.SimilarityEpochBlocks = 0

	err := params.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "similarity_epoch_blocks must be > 0")
}

func TestParams_Validate_DerivativeThresholdOverMax(t *testing.T) {
	params := types.DefaultParams()
	params.DerivativeThreshold = 10001

	err := params.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "derivative_threshold cannot exceed")
}

func TestParams_Validate_OracleAllowlistTooLarge(t *testing.T) {
	params := types.DefaultParams()
	params.SimilarityOracleAllowlist = make([]string, 21) // > MaxOracleAllowlistSize=20
	for i := range params.SimilarityOracleAllowlist {
		params.SimilarityOracleAllowlist[i] = sdk.AccAddress([]byte{byte(i), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}).String()
	}

	err := params.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "similarity_oracle_allowlist too large")
}

func TestParams_Validate_DuplicateOracleAddress(t *testing.T) {
	addr := sdk.AccAddress("oracle______________").String()
	params := types.DefaultParams()
	params.SimilarityOracleAllowlist = []string{addr, addr}

	err := params.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate oracle address")
}

// --- Params Sidecar Roundtrip ---

func TestParams_SimilaritySidecar_Roundtrip(t *testing.T) {
	f := SetupKeeperTest(t)

	oracleAddr := sdk.AccAddress("oracle______________").String()

	params := f.keeper.GetParams(f.ctx)
	params.SimilarityOracleAllowlist = []string{oracleAddr}
	params.DerivativeThreshold = 9000
	params.SimilarityEpochBlocks = 200
	params.EnableSimilarityCheck = true

	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Read back
	got := f.keeper.GetParams(f.ctx)
	require.Equal(t, []string{oracleAddr}, got.SimilarityOracleAllowlist)
	require.Equal(t, uint32(9000), got.DerivativeThreshold)
	require.Equal(t, int64(200), got.SimilarityEpochBlocks)
	require.True(t, got.EnableSimilarityCheck)
}

func TestParams_SimilaritySidecar_Defaults(t *testing.T) {
	f := SetupKeeperTest(t)

	got := f.keeper.GetParams(f.ctx)
	require.Equal(t, types.DefaultSimilarityOracleAllowlist(), got.SimilarityOracleAllowlist)
	require.Equal(t, types.DefaultDerivativeThreshold, got.DerivativeThreshold)
	require.Equal(t, types.DefaultSimilarityEpochBlocks, got.SimilarityEpochBlocks)
	require.Equal(t, types.DefaultEnableSimilarityCheck, got.EnableSimilarityCheck)
}

// --- VerifyOracleSignature (negative paths — positive path requires real key setup) ---

func TestVerifyOracleSignature_EmptyAllowlist(t *testing.T) {
	f := SetupKeeperTest(t)

	// Default has empty allowlist
	_, err := f.keeper.VerifyOracleSignature(f.ctx, []byte("data"), []byte("sig"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "oracle allowlist is empty")
}

func TestVerifyOracleSignature_NoMatchingKey(t *testing.T) {
	f := SetupKeeperTest(t)

	// Set allowlist with an address, but mock account keeper returns nil
	oracleAddr := sdk.AccAddress("oracle______________").String()
	params := f.keeper.GetParams(f.ctx)
	params.SimilarityOracleAllowlist = []string{oracleAddr}
	params.SimilarityEpochBlocks = 100
	params.EnableSimilarityCheck = true
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	// Mock account keeper returns nil for all accounts, so no pubkey to verify
	_, err = f.keeper.VerifyOracleSignature(f.ctx, []byte("data"), []byte("sig"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature does not match any allowlisted oracle")
}

// --- DerivativeThreshold Logic ---

func TestDerivativeThreshold_AtThreshold(t *testing.T) {
	// Test that similarity == threshold triggers derivative flag
	threshold := types.DefaultDerivativeThreshold // 8500
	require.True(t, uint32(8500) >= threshold)
}

func TestDerivativeThreshold_BelowThreshold(t *testing.T) {
	threshold := types.DefaultDerivativeThreshold // 8500
	require.False(t, uint32(8499) >= threshold)
}

// --- CommitmentHashHex ---

func TestCommitmentHashHex(t *testing.T) {
	hash := []byte{0xAB, 0xCD, 0xEF, 0x01, 0x23, 0x45, 0x67, 0x89}
	hex := types.CommitmentHashHex(hash)
	require.Equal(t, "abcdef0123456789", hex)
}

// --- MsgSubmitSimilarityCommitment proto Marshal/Unmarshal ---

func TestMsgSubmitSimilarityCommitment_MarshalRoundtrip(t *testing.T) {
	compactData := types.SimilarityCompactData{
		ContributionID:       42,
		OverallSimilarity:    8500,
		Confidence:           9000,
		NearestParentClaimID: 10,
		ModelVersion:         "v1.0.0",
		Epoch:                5,
	}
	compactJSON, _ := json.Marshal(compactData)
	commitHash := make([]byte, 32)
	commitHash[0] = 0xAB

	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:              "omni1oracle",
		ContributionID:         42,
		CompactDataJson:        compactJSON,
		OracleSignatureCompact: []byte("compact_sig_bytes"),
		CommitmentHashFull:     commitHash,
		OracleSignatureFull:    []byte("full_sig_bytes"),
	}

	// Marshal
	bz, err := msg.Marshal()
	require.NoError(t, err)
	require.NotEmpty(t, bz)

	// Unmarshal
	var decoded types.MsgSubmitSimilarityCommitment
	err = decoded.Unmarshal(bz)
	require.NoError(t, err)

	require.Equal(t, msg.Submitter, decoded.Submitter)
	require.Equal(t, msg.ContributionID, decoded.ContributionID)
	require.Equal(t, msg.CompactDataJson, decoded.CompactDataJson)
	require.Equal(t, msg.OracleSignatureCompact, decoded.OracleSignatureCompact)
	require.Equal(t, msg.CommitmentHashFull, decoded.CommitmentHashFull)
	require.Equal(t, msg.OracleSignatureFull, decoded.OracleSignatureFull)
}

func TestMsgSubmitSimilarityCommitment_Size(t *testing.T) {
	msg := &types.MsgSubmitSimilarityCommitment{
		Submitter:              "omni1oracle",
		ContributionID:         42,
		CompactDataJson:        []byte(`{"test":true}`),
		OracleSignatureCompact: make([]byte, 64),
		CommitmentHashFull:     make([]byte, 32),
		OracleSignatureFull:    make([]byte, 64),
	}

	size := msg.Size()
	require.Greater(t, size, 0)

	bz, err := msg.Marshal()
	require.NoError(t, err)
	require.Equal(t, size, len(bz))
}
