package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ sdk.Msg = &MsgSubmitContribution{}
	_ sdk.Msg = &MsgEndorse{}
	_ sdk.Msg = &MsgWithdrawPOCRewards{}
	_ sdk.Msg = &MsgUpdateParams{}
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
