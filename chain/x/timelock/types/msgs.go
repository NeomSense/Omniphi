package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Message types
const (
	TypeMsgExecuteOperation = "execute_operation"
	TypeMsgCancelOperation  = "cancel_operation"
	TypeMsgEmergencyExecute = "emergency_execute"
	TypeMsgUpdateParams     = "update_params"
	TypeMsgUpdateGuardian   = "update_guardian"
)

// MsgExecuteOperation defines a message to execute a queued operation
type MsgExecuteOperation struct {
	Executor    string `json:"executor"`
	OperationId uint64 `json:"operation_id"`
}

// Route implements sdk.Msg
func (msg MsgExecuteOperation) Route() string { return RouterKey }

// Type implements sdk.Msg
func (msg MsgExecuteOperation) Type() string { return TypeMsgExecuteOperation }

// ValidateBasic implements sdk.Msg
func (msg MsgExecuteOperation) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Executor); err != nil {
		return ErrInvalidExecutor
	}
	if msg.OperationId == 0 {
		return ErrOperationNotFound
	}
	return nil
}

// GetSigners implements sdk.Msg
func (msg MsgExecuteOperation) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Executor)
	return []sdk.AccAddress{addr}
}

// MsgCancelOperation defines a message to cancel a queued operation
type MsgCancelOperation struct {
	Authority   string `json:"authority"`
	OperationId uint64 `json:"operation_id"`
	Reason      string `json:"reason"`
}

// Route implements sdk.Msg
func (msg MsgCancelOperation) Route() string { return RouterKey }

// Type implements sdk.Msg
func (msg MsgCancelOperation) Type() string { return TypeMsgCancelOperation }

// ValidateBasic implements sdk.Msg
func (msg MsgCancelOperation) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return ErrUnauthorized
	}
	if msg.OperationId == 0 {
		return ErrOperationNotFound
	}
	if err := ValidateCancelReason(msg.Reason); err != nil {
		return err
	}
	return nil
}

// GetSigners implements sdk.Msg
func (msg MsgCancelOperation) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// MsgEmergencyExecute defines a message to execute with reduced delay
type MsgEmergencyExecute struct {
	Authority     string `json:"authority"`
	OperationId   uint64 `json:"operation_id"`
	Justification string `json:"justification"`
}

// Route implements sdk.Msg
func (msg MsgEmergencyExecute) Route() string { return RouterKey }

// Type implements sdk.Msg
func (msg MsgEmergencyExecute) Type() string { return TypeMsgEmergencyExecute }

// ValidateBasic implements sdk.Msg
func (msg MsgEmergencyExecute) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return ErrUnauthorized
	}
	if msg.OperationId == 0 {
		return ErrOperationNotFound
	}
	if err := ValidateJustification(msg.Justification); err != nil {
		return err
	}
	return nil
}

// GetSigners implements sdk.Msg
func (msg MsgEmergencyExecute) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// MsgUpdateParams defines a message to update module parameters
type MsgUpdateParams struct {
	Authority string `json:"authority"`
	Params    Params `json:"params"`
}

// Route implements sdk.Msg
func (msg MsgUpdateParams) Route() string { return RouterKey }

// Type implements sdk.Msg
func (msg MsgUpdateParams) Type() string { return TypeMsgUpdateParams }

// ValidateBasic implements sdk.Msg
func (msg MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return ErrUnauthorized
	}
	return msg.Params.Validate()
}

// GetSigners implements sdk.Msg
func (msg MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// MsgUpdateGuardian defines a message to update the guardian address
type MsgUpdateGuardian struct {
	Authority   string `json:"authority"`
	NewGuardian string `json:"new_guardian"`
}

// Route implements sdk.Msg
func (msg MsgUpdateGuardian) Route() string { return RouterKey }

// Type implements sdk.Msg
func (msg MsgUpdateGuardian) Type() string { return TypeMsgUpdateGuardian }

// ValidateBasic implements sdk.Msg
func (msg MsgUpdateGuardian) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return ErrUnauthorized
	}
	if _, err := sdk.AccAddressFromBech32(msg.NewGuardian); err != nil {
		return ErrInvalidGuardian
	}
	return nil
}

// GetSigners implements sdk.Msg
func (msg MsgUpdateGuardian) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// Ensure messages implement sdk.Msg
var (
	_ sdk.Msg = &MsgExecuteOperation{}
	_ sdk.Msg = &MsgCancelOperation{}
	_ sdk.Msg = &MsgEmergencyExecute{}
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgUpdateGuardian{}
)
