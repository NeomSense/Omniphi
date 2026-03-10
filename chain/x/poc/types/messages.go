package types

import (
	"encoding/json"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ sdk.Msg = &MsgSubmitContribution{}
	_ sdk.Msg = &MsgEndorse{}
	_ sdk.Msg = &MsgWithdrawPOCRewards{}
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgSubmitSimilarityCommitment{}
	_ sdk.Msg = &MsgStartReview{}
	_ sdk.Msg = &MsgCastReviewVote{}
	_ sdk.Msg = &MsgFinalizeReview{}
	_ sdk.Msg = &MsgAppealReview{}
	_ sdk.Msg = &MsgResolveAppeal{}
)

// MaxCTypeLength defines the maximum length for contribution type
const MaxCTypeLength = 64

// MaxURILength defines the maximum length for URI
const MaxURILength = 512

// MaxHashLength defines the maximum length for hash
const MaxHashLength = 128

// Hash size constants for validation
const (
	HashSizeSHA256 = 32 // 256 bits
	HashSizeSHA512 = 64 // 512 bits
)

// ========== MsgSubmitContribution ==========

// GetSigners returns the expected signers for MsgSubmitContribution
func (msg *MsgSubmitContribution) GetSigners() []sdk.AccAddress {
	contributor, err := sdk.AccAddressFromBech32(msg.Contributor)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{contributor}
}

// ValidateBasic performs basic validation of MsgSubmitContribution
func (msg *MsgSubmitContribution) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Contributor)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid contributor address (%s)", err)
	}

	if len(msg.Ctype) == 0 {
		return errorsmod.Wrap(ErrInvalidCType, "ctype cannot be empty")
	}

	if len(msg.Ctype) > MaxCTypeLength {
		return errorsmod.Wrapf(ErrInvalidCType, "ctype too long: max length is %d", MaxCTypeLength)
	}

	if len(msg.Uri) == 0 {
		return errorsmod.Wrap(ErrInvalidURI, "uri cannot be empty")
	}

	if len(msg.Uri) > MaxURILength {
		return errorsmod.Wrapf(ErrInvalidURI, "uri too long: max length is %d", MaxURILength)
	}

	// SECURITY FIX: CVE-2025-POC-006 - Strict hash validation
	if len(msg.Hash) == 0 {
		return errorsmod.Wrap(ErrInvalidHash, "hash cannot be empty")
	}

	// Validate hash length (must be standard hash size)
	if len(msg.Hash) != HashSizeSHA256 && len(msg.Hash) != HashSizeSHA512 {
		return errorsmod.Wrapf(ErrInvalidHash,
			"invalid hash length: %d (expected %d or %d bytes)",
			len(msg.Hash), HashSizeSHA256, HashSizeSHA512)
	}

	// Reject all-zero hash
	allZeros := true
	for _, b := range msg.Hash {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		return errorsmod.Wrap(ErrInvalidHash, "hash cannot be all zeros")
	}

	// Reject all-ones hash (another common invalid value)
	allOnes := true
	for _, b := range msg.Hash {
		if b != 0xFF {
			allOnes = false
			break
		}
	}
	if allOnes {
		return errorsmod.Wrap(ErrInvalidHash, "hash cannot be all ones")
	}

	// Validate canonical hash (if provided)
	if len(msg.CanonicalHash) > 0 {
		if err := ValidateCanonicalHash(msg.CanonicalHash); err != nil {
			return errorsmod.Wrap(ErrInvalidCanonicalHash, err.Error())
		}
		if msg.CanonicalSpecVersion == 0 {
			return errorsmod.Wrap(ErrInvalidSpecVersion, "spec version required when canonical hash provided")
		}
		if msg.CanonicalSpecVersion > CurrentCanonicalSpecVersion {
			return errorsmod.Wrapf(ErrInvalidSpecVersion,
				"spec version %d > current %d", msg.CanonicalSpecVersion, CurrentCanonicalSpecVersion)
		}
	}

	return nil
}

// ========== MsgEndorse ==========

// GetSigners returns the expected signers for MsgEndorse
func (msg *MsgEndorse) GetSigners() []sdk.AccAddress {
	validator, err := sdk.AccAddressFromBech32(msg.Validator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{validator}
}

// ValidateBasic performs basic validation of MsgEndorse
func (msg *MsgEndorse) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Validator)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid validator address (%s)", err)
	}

	if msg.ContributionId == 0 {
		return errorsmod.Wrap(ErrContributionNotFound, "contribution ID cannot be zero")
	}

	return nil
}

// ========== MsgWithdrawPOCRewards ==========

// GetSigners returns the expected signers for MsgWithdrawPOCRewards
func (msg *MsgWithdrawPOCRewards) GetSigners() []sdk.AccAddress {
	address, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{address}
}

// ValidateBasic performs basic validation of MsgWithdrawPOCRewards
func (msg *MsgWithdrawPOCRewards) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address (%s)", err)
	}

	return nil
}

// ========== MsgUpdateParams ==========

// GetSigners returns the expected signers for MsgUpdateParams
func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

// ValidateBasic performs basic validation of MsgUpdateParams
func (msg *MsgUpdateParams) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}

	return msg.Params.Validate()
}

// ========== MsgSubmitSimilarityCommitment ==========

// GetSigners returns the expected signers for MsgSubmitSimilarityCommitment
func (msg *MsgSubmitSimilarityCommitment) GetSigners() []sdk.AccAddress {
	submitter, err := sdk.AccAddressFromBech32(msg.Submitter)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{submitter}
}

// ValidateBasic performs basic validation of MsgSubmitSimilarityCommitment
func (msg *MsgSubmitSimilarityCommitment) ValidateBasic() error {
	// Validate submitter address
	_, err := sdk.AccAddressFromBech32(msg.Submitter)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid submitter address (%s)", err)
	}

	// Validate contribution ID
	if msg.ContributionID == 0 {
		return errorsmod.Wrap(ErrContributionNotFound, "contribution ID cannot be zero")
	}

	// Validate compact data JSON is present and parseable
	if len(msg.CompactDataJson) == 0 {
		return errorsmod.Wrap(ErrInvalidCompactData, "compact_data_json cannot be empty")
	}
	var compactData SimilarityCompactData
	if err := json.Unmarshal(msg.CompactDataJson, &compactData); err != nil {
		return errorsmod.Wrapf(ErrInvalidCompactData, "invalid compact_data_json: %s", err)
	}
	if err := compactData.Validate(); err != nil {
		return errorsmod.Wrapf(ErrInvalidCompactData, "compact data validation failed: %s", err)
	}

	// Ensure compact data contribution ID matches message field
	if compactData.ContributionID != msg.ContributionID {
		return errorsmod.Wrapf(ErrInvalidCompactData,
			"compact_data.contribution_id (%d) != msg.contribution_id (%d)",
			compactData.ContributionID, msg.ContributionID)
	}

	// Validate oracle signature (compact)
	if len(msg.OracleSignatureCompact) == 0 {
		return errorsmod.Wrap(ErrInvalidOracleSignature, "oracle_signature_compact cannot be empty")
	}
	if len(msg.OracleSignatureCompact) > MaxOracleSignatureLength {
		return errorsmod.Wrapf(ErrInvalidOracleSignature,
			"oracle_signature_compact too long: %d > %d", len(msg.OracleSignatureCompact), MaxOracleSignatureLength)
	}

	// Validate commitment hash (full vector)
	if err := ValidateCommitmentHash(msg.CommitmentHashFull); err != nil {
		return errorsmod.Wrap(ErrInvalidCommitmentHash, err.Error())
	}

	// Validate oracle signature (full)
	if len(msg.OracleSignatureFull) == 0 {
		return errorsmod.Wrap(ErrInvalidOracleSignature, "oracle_signature_full cannot be empty")
	}
	if len(msg.OracleSignatureFull) > MaxOracleSignatureLength {
		return errorsmod.Wrapf(ErrInvalidOracleSignature,
			"oracle_signature_full too long: %d > %d", len(msg.OracleSignatureFull), MaxOracleSignatureLength)
	}

	return nil
}

// ========== MsgStartReview ==========

// GetSigners returns the expected signers for MsgStartReview
func (msg *MsgStartReview) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

// ValidateBasic performs basic validation of MsgStartReview
func (msg *MsgStartReview) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	if msg.ContributionId == 0 {
		return errorsmod.Wrap(ErrContributionNotFound, "contribution ID cannot be zero")
	}
	return nil
}

// ========== MsgCastReviewVote ==========

// GetSigners returns the expected signers for MsgCastReviewVote
func (msg *MsgCastReviewVote) GetSigners() []sdk.AccAddress {
	reviewer, err := sdk.AccAddressFromBech32(msg.Reviewer)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{reviewer}
}

// ValidateBasic performs basic validation of MsgCastReviewVote
func (msg *MsgCastReviewVote) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Reviewer)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid reviewer address (%s)", err)
	}
	if msg.ContributionId == 0 {
		return errorsmod.Wrap(ErrContributionNotFound, "contribution ID cannot be zero")
	}
	if !ReviewVoteDecision(msg.Decision).IsValid() {
		return errorsmod.Wrapf(ErrInvalidOverride, "invalid vote decision: %d", msg.Decision)
	}
	if !OriginalityOverride(msg.OriginalityOverride).IsValid() {
		return errorsmod.Wrapf(ErrInvalidOverride, "invalid originality override: %d", msg.OriginalityOverride)
	}
	if msg.QualityScore > MaxQualityScore {
		return errorsmod.Wrapf(ErrInvalidQualityScore, "quality score %d exceeds max %d", msg.QualityScore, MaxQualityScore)
	}
	if len(msg.NotesPointer) > MaxNotesPointerLength {
		return errorsmod.Wrapf(ErrInvalidNotesPointer, "notes pointer too long: %d > %d", len(msg.NotesPointer), MaxNotesPointerLength)
	}
	return nil
}

// ========== MsgFinalizeReview ==========

// GetSigners returns the expected signers for MsgFinalizeReview
func (msg *MsgFinalizeReview) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

// ValidateBasic performs basic validation of MsgFinalizeReview
func (msg *MsgFinalizeReview) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	if msg.ContributionId == 0 {
		return errorsmod.Wrap(ErrContributionNotFound, "contribution ID cannot be zero")
	}
	return nil
}

// ========== MsgAppealReview ==========

// GetSigners returns the expected signers for MsgAppealReview
func (msg *MsgAppealReview) GetSigners() []sdk.AccAddress {
	appellant, err := sdk.AccAddressFromBech32(msg.Appellant)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{appellant}
}

// ValidateBasic performs basic validation of MsgAppealReview
func (msg *MsgAppealReview) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Appellant)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid appellant address (%s)", err)
	}
	if msg.ContributionId == 0 {
		return errorsmod.Wrap(ErrContributionNotFound, "contribution ID cannot be zero")
	}
	if len(msg.Reason) == 0 {
		return errorsmod.Wrap(ErrAppealNotFound, "appeal reason cannot be empty")
	}
	if len(msg.Reason) > MaxAppealReasonLength {
		return errorsmod.Wrapf(ErrAppealNotFound, "appeal reason too long: %d > %d", len(msg.Reason), MaxAppealReasonLength)
	}
	return nil
}

// ========== MsgResolveAppeal ==========

// GetSigners returns the expected signers for MsgResolveAppeal
func (msg *MsgResolveAppeal) GetSigners() []sdk.AccAddress {
	authority, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{authority}
}

// ValidateBasic performs basic validation of MsgResolveAppeal
func (msg *MsgResolveAppeal) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	if msg.AppealId == 0 {
		return errorsmod.Wrap(ErrAppealNotFound, "appeal ID cannot be zero")
	}
	return nil
}
