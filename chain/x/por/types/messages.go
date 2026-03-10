package types

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Interface assertions
var (
	_ sdk.Msg = &MsgRegisterApp{}
	_ sdk.Msg = &MsgCreateVerifierSet{}
	_ sdk.Msg = &MsgSubmitBatch{}
	_ sdk.Msg = &MsgSubmitAttestation{}
	_ sdk.Msg = &MsgChallengeBatch{}
	_ sdk.Msg = &MsgFinalizeBatch{}
	_ sdk.Msg = &MsgUpdateParams{}
)

// Validation constants
const (
	MaxAppNameLength      = 64
	MaxSchemaCidLength    = 256
	MerkleRootLength      = 32 // SHA256 always 32 bytes
	MaxProofDataLength    = 4096
	MaxSignatureLength    = 256
	MaxVerifierSetMembers = 100
	MaxRecordCount        = 1_000_000 // 1M records per batch max
)

// ============================================================================
// MsgRegisterApp
// ============================================================================

func (msg *MsgRegisterApp) GetSigners() []sdk.AccAddress {
	owner, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{owner}
}

func (msg *MsgRegisterApp) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid owner address: %s", err)
	}
	if len(msg.Name) == 0 {
		return errorsmod.Wrap(ErrInvalidAppName, "name cannot be empty")
	}
	if len(msg.Name) > MaxAppNameLength {
		return errorsmod.Wrapf(ErrInvalidAppName, "name too long: max %d characters", MaxAppNameLength)
	}
	if len(msg.SchemaCid) == 0 {
		return errorsmod.Wrap(ErrInvalidSchemaCid, "schema_cid cannot be empty")
	}
	if len(msg.SchemaCid) > MaxSchemaCidLength {
		return errorsmod.Wrapf(ErrInvalidSchemaCid, "schema_cid too long: max %d characters", MaxSchemaCidLength)
	}
	if msg.ChallengePeriod <= 0 {
		return errorsmod.Wrapf(ErrInvalidChallengePeriod, "challenge_period must be positive: got %d", msg.ChallengePeriod)
	}
	if msg.MinVerifiers == 0 {
		return errorsmod.Wrap(ErrInvalidMinVerifiers, "min_verifiers must be greater than 0")
	}
	return nil
}

// ============================================================================
// MsgCreateVerifierSet
// ============================================================================

func (msg *MsgCreateVerifierSet) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgCreateVerifierSet) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address: %s", err)
	}
	if msg.AppId == 0 {
		return errorsmod.Wrap(ErrAppNotFound, "app_id cannot be 0")
	}
	if len(msg.Members) == 0 {
		return errorsmod.Wrap(ErrInvalidMinVerifiers, "members cannot be empty")
	}
	if len(msg.Members) > MaxVerifierSetMembers {
		return errorsmod.Wrapf(ErrTooManyVerifiers, "members exceed max %d", MaxVerifierSetMembers)
	}

	// Check for duplicate members
	seen := make(map[string]bool)
	for _, m := range msg.Members {
		if _, err := sdk.AccAddressFromBech32(m.Address); err != nil {
			return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid member address: %s", err)
		}
		if seen[m.Address] {
			return errorsmod.Wrapf(ErrDuplicateVerifier, "duplicate member: %s", m.Address)
		}
		seen[m.Address] = true
		if m.Weight.IsNegative() || m.Weight.IsZero() {
			return errorsmod.Wrapf(ErrInsufficientStake, "member weight must be positive: %s", m.Address)
		}
	}

	if msg.MinAttestations == 0 {
		return errorsmod.Wrap(ErrInvalidMinVerifiers, "min_attestations must be greater than 0")
	}
	if msg.MinAttestations > uint32(len(msg.Members)) {
		return errorsmod.Wrapf(ErrInvalidMinVerifiers, "min_attestations (%d) cannot exceed member count (%d)", msg.MinAttestations, len(msg.Members))
	}

	if msg.QuorumPct.IsNil() || msg.QuorumPct.LTE(math.LegacyZeroDec()) {
		return errorsmod.Wrap(ErrInvalidQuorumPct, "quorum_pct must be positive")
	}
	if msg.QuorumPct.GT(math.LegacyOneDec()) {
		return errorsmod.Wrap(ErrInvalidQuorumPct, "quorum_pct cannot exceed 1.0")
	}

	return nil
}

// ============================================================================
// MsgSubmitBatch
// ============================================================================

func (msg *MsgSubmitBatch) GetSigners() []sdk.AccAddress {
	submitter, err := sdk.AccAddressFromBech32(msg.Submitter)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{submitter}
}

func (msg *MsgSubmitBatch) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Submitter); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid submitter address: %s", err)
	}
	if msg.AppId == 0 {
		return errorsmod.Wrap(ErrAppNotFound, "app_id cannot be 0")
	}

	// SECURITY: Validate merkle root - must be exactly 32 bytes (SHA256)
	if len(msg.RecordMerkleRoot) != MerkleRootLength {
		return errorsmod.Wrapf(ErrInvalidMerkleRoot, "merkle root must be exactly %d bytes, got %d", MerkleRootLength, len(msg.RecordMerkleRoot))
	}
	// SECURITY: Reject all-zero merkle roots
	allZero := true
	for _, b := range msg.RecordMerkleRoot {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return errorsmod.Wrap(ErrInvalidMerkleRoot, "merkle root cannot be all zeros")
	}
	// SECURITY: Reject all-ones merkle roots
	allOnes := true
	for _, b := range msg.RecordMerkleRoot {
		if b != 0xFF {
			allOnes = false
			break
		}
	}
	if allOnes {
		return errorsmod.Wrap(ErrInvalidMerkleRoot, "merkle root cannot be all ones")
	}

	if msg.RecordCount == 0 {
		return errorsmod.Wrap(ErrInvalidRecordCount, "record_count must be greater than 0")
	}
	if msg.RecordCount > MaxRecordCount {
		return errorsmod.Wrapf(ErrInvalidRecordCount, "record_count exceeds maximum %d", MaxRecordCount)
	}
	if msg.VerifierSetId == 0 {
		return errorsmod.Wrap(ErrVerifierSetNotFound, "verifier_set_id cannot be 0")
	}

	// F2/F6: Validate DA commitment hash if provided
	if len(msg.DACommitmentHash) > 0 && len(msg.DACommitmentHash) != MerkleRootLength {
		return errorsmod.Wrapf(ErrDACommitmentRequired, "DA commitment hash must be exactly %d bytes, got %d", MerkleRootLength, len(msg.DACommitmentHash))
	}

	// F3: Validate leaf hashes if provided
	if len(msg.LeafHashes) > 0 {
		if uint64(len(msg.LeafHashes)) != msg.RecordCount {
			return errorsmod.Wrapf(ErrInvalidRecordCount, "leaf hash count %d does not match record_count %d", len(msg.LeafHashes), msg.RecordCount)
		}
		for i, h := range msg.LeafHashes {
			if len(h) != MerkleRootLength {
				return errorsmod.Wrapf(ErrInvalidMerkleRoot, "leaf hash %d has invalid length %d (expected %d)", i, len(h), MerkleRootLength)
			}
		}
	}

	// F8: Validate PoSeq commitment hash if provided
	if len(msg.PoSeqCommitmentHash) > 0 && len(msg.PoSeqCommitmentHash) != MerkleRootLength {
		return errorsmod.Wrapf(ErrPoSeqCommitmentNotFound, "PoSeq commitment hash must be exactly %d bytes, got %d", MerkleRootLength, len(msg.PoSeqCommitmentHash))
	}

	return nil
}

// ============================================================================
// MsgSubmitAttestation
// ============================================================================

func (msg *MsgSubmitAttestation) GetSigners() []sdk.AccAddress {
	verifier, err := sdk.AccAddressFromBech32(msg.Verifier)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{verifier}
}

func (msg *MsgSubmitAttestation) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Verifier); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid verifier address: %s", err)
	}
	if msg.BatchId == 0 {
		return errorsmod.Wrap(ErrBatchNotFound, "batch_id cannot be 0")
	}
	if len(msg.Signature) == 0 {
		return errorsmod.Wrap(ErrInvalidSignature, "signature cannot be empty")
	}
	if len(msg.Signature) > MaxSignatureLength {
		return errorsmod.Wrapf(ErrInvalidSignature, "signature too long: max %d bytes", MaxSignatureLength)
	}
	if msg.ConfidenceWeight.IsNil() || msg.ConfidenceWeight.LTE(math.LegacyZeroDec()) {
		return errorsmod.Wrap(ErrInvalidConfidenceWeight, "confidence_weight must be positive")
	}
	if msg.ConfidenceWeight.GT(math.LegacyOneDec()) {
		return errorsmod.Wrap(ErrInvalidConfidenceWeight, "confidence_weight cannot exceed 1.0")
	}
	return nil
}

// ============================================================================
// MsgChallengeBatch
// ============================================================================

func (msg *MsgChallengeBatch) GetSigners() []sdk.AccAddress {
	challenger, err := sdk.AccAddressFromBech32(msg.Challenger)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{challenger}
}

func (msg *MsgChallengeBatch) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Challenger); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid challenger address: %s", err)
	}
	if msg.BatchId == 0 {
		return errorsmod.Wrap(ErrBatchNotFound, "batch_id cannot be 0")
	}
	if !msg.ChallengeType.IsValid() {
		return errorsmod.Wrapf(ErrInvalidProofData, "invalid challenge type: %d", msg.ChallengeType)
	}
	if len(msg.ProofData) == 0 {
		return errorsmod.Wrap(ErrInvalidProofData, "proof_data cannot be empty")
	}
	if len(msg.ProofData) > MaxProofDataLength {
		return errorsmod.Wrapf(ErrInvalidProofData, "proof_data too long: max %d bytes", MaxProofDataLength)
	}
	return nil
}

// ============================================================================
// MsgFinalizeBatch
// ============================================================================

func (msg *MsgFinalizeBatch) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

func (msg *MsgFinalizeBatch) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address: %s", err)
	}
	if msg.BatchId == 0 {
		return errorsmod.Wrap(ErrBatchNotFound, "batch_id cannot be 0")
	}
	return nil
}

// ============================================================================
// MsgUpdateParams
// ============================================================================

func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

func (msg *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address: %s", err)
	}
	return msg.Params.Validate()
}
