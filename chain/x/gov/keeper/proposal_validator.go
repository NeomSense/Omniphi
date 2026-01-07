// Package keeper contains the proposal validation logic for Omniphi governance
package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	"pos/x/gov/types"
)

// ProposalValidator validates governance proposals before acceptance
// This prevents invalid proposals from wasting deposits and voting periods
type ProposalValidator struct {
	cdc           codec.Codec
	msgRouter     baseapp.MessageRouter
	logger        log.Logger
	storeKey      storetypes.StoreKey

	// Simulation configuration
	enableSimulation bool
	maxGasLimit      uint64
}

// ProposalValidatorConfig holds configuration for the proposal validator
type ProposalValidatorConfig struct {
	// EnableSimulation enables full message simulation (recommended: true)
	EnableSimulation bool

	// MaxGasLimit is the maximum gas allowed for simulation (default: 10M)
	MaxGasLimit uint64
}

// DefaultProposalValidatorConfig returns the default configuration
func DefaultProposalValidatorConfig() ProposalValidatorConfig {
	return ProposalValidatorConfig{
		EnableSimulation: true,
		MaxGasLimit:      10_000_000, // 10M gas for simulation
	}
}

// NewProposalValidator creates a new ProposalValidator
func NewProposalValidator(
	cdc codec.Codec,
	msgRouter baseapp.MessageRouter,
	logger log.Logger,
	storeKey storetypes.StoreKey,
	config ProposalValidatorConfig,
) *ProposalValidator {
	return &ProposalValidator{
		cdc:              cdc,
		msgRouter:        msgRouter,
		logger:           logger.With("module", "proposal-validator"),
		storeKey:         storeKey,
		enableSimulation: config.EnableSimulation,
		maxGasLimit:      config.MaxGasLimit,
	}
}

// ValidateProposalMessages validates all messages in a proposal
// This is called BEFORE the proposal is accepted to prevent invalid proposals
func (pv *ProposalValidator) ValidateProposalMessages(ctx context.Context, msgs []sdk.Msg) error {
	if len(msgs) == 0 {
		return nil // Empty proposals are valid (text proposals)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for i, msg := range msgs {
		pv.logger.Debug("validating proposal message",
			"index", i,
			"type", sdk.MsgTypeURL(msg),
		)

		// Step 1: Basic message validation (ValidateBasic)
		if err := pv.validateMessageBasic(msg); err != nil {
			return types.ErrInvalidProposalMessage.Wrapf(
				"message %d (%s) failed basic validation: %v",
				i, sdk.MsgTypeURL(msg), err,
			)
		}

		// Step 2: Check message routing (handler exists)
		if err := pv.validateMessageRouting(msg); err != nil {
			return types.ErrMessageRoutingFailed.Wrapf(
				"message %d (%s) has no handler: %v",
				i, sdk.MsgTypeURL(msg), err,
			)
		}

		// Step 3: Message-specific validation
		if err := pv.validateMessageSpecific(sdkCtx, msg); err != nil {
			return types.ErrProposalValidationFailed.Wrapf(
				"message %d (%s) failed specific validation: %v",
				i, sdk.MsgTypeURL(msg), err,
			)
		}

		// Step 4: Simulation (if enabled)
		if pv.enableSimulation {
			if err := pv.simulateMessage(sdkCtx, msg); err != nil {
				return types.ErrProposalSimulationFailed.Wrapf(
					"message %d (%s) would fail execution: %v",
					i, sdk.MsgTypeURL(msg), err,
				)
			}
		}
	}

	pv.logger.Info("all proposal messages validated successfully",
		"message_count", len(msgs),
	)

	return nil
}

// validateMessageBasic performs basic validation on a message
func (pv *ProposalValidator) validateMessageBasic(msg sdk.Msg) error {
	// Check if the message implements the sdk.Msg interface properly
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	// Call ValidateBasic if available
	if validatable, ok := msg.(interface{ ValidateBasic() error }); ok {
		if err := validatable.ValidateBasic(); err != nil {
			return err
		}
	}

	return nil
}

// validateMessageRouting checks if a handler exists for the message
func (pv *ProposalValidator) validateMessageRouting(msg sdk.Msg) error {
	if pv.msgRouter == nil {
		// If no router is available, skip routing validation
		pv.logger.Warn("message router not available, skipping routing validation")
		return nil
	}

	handler := pv.msgRouter.Handler(msg)
	if handler == nil {
		return fmt.Errorf("no handler found for message type %s", sdk.MsgTypeURL(msg))
	}

	return nil
}

// validateMessageSpecific performs message-type-specific validation
// This handles known problematic message types
func (pv *ProposalValidator) validateMessageSpecific(ctx sdk.Context, msg sdk.Msg) error {
	msgTypeURL := sdk.MsgTypeURL(msg)

	switch msgTypeURL {
	case "/cosmos.consensus.v1.MsgUpdateParams":
		return pv.validateConsensusUpdateParams(ctx, msg)
	case "/pos.feemarket.v1.MsgUpdateParams":
		return pv.validateFeemarketUpdateParams(ctx, msg)
	case "/pos.tokenomics.v1.MsgUpdateParams":
		return pv.validateTokenomicsUpdateParams(ctx, msg)
	case "/pos.poc.v1.MsgUpdateParams":
		return pv.validatePocUpdateParams(ctx, msg)
	case "/cosmos.staking.v1beta1.MsgUpdateParams":
		return pv.validateStakingUpdateParams(ctx, msg)
	default:
		// For unknown message types, rely on simulation
		pv.logger.Debug("no specific validation for message type", "type", msgTypeURL)
		return nil
	}
}

// validateConsensusUpdateParams validates consensus parameter update messages
// This prevents the "all parameters must be present" error
func (pv *ProposalValidator) validateConsensusUpdateParams(ctx sdk.Context, msg sdk.Msg) error {
	// Use reflection to check required fields without importing consensus types directly
	// This avoids circular dependencies

	// The consensus MsgUpdateParams requires:
	// - block (with max_bytes and max_gas)
	// - evidence (with max_age_num_blocks, max_age_duration, max_bytes)
	// - validator (with pub_key_types)
	// - abci (can be empty but must be present)

	// Get the message as a map for validation
	// Use standard json package since we're working with a generic map
	msgJSON, err := pv.cdc.MarshalJSON(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	var msgMap map[string]interface{}
	if err := json.Unmarshal(msgJSON, &msgMap); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Check required fields
	requiredFields := []string{"authority", "block", "evidence", "validator"}
	for _, field := range requiredFields {
		if _, ok := msgMap[field]; !ok {
			return fmt.Errorf("consensus MsgUpdateParams requires '%s' field - all parameters must be present", field)
		}
		if msgMap[field] == nil {
			return fmt.Errorf("consensus MsgUpdateParams '%s' field cannot be null - all parameters must be present", field)
		}
	}

	// Validate block parameters
	if block, ok := msgMap["block"].(map[string]interface{}); ok {
		if _, exists := block["max_bytes"]; !exists {
			return fmt.Errorf("block.max_bytes is required")
		}
		if _, exists := block["max_gas"]; !exists {
			return fmt.Errorf("block.max_gas is required")
		}
	} else {
		return fmt.Errorf("block must be an object with max_bytes and max_gas")
	}

	// Validate evidence parameters
	if evidence, ok := msgMap["evidence"].(map[string]interface{}); ok {
		if _, exists := evidence["max_age_num_blocks"]; !exists {
			return fmt.Errorf("evidence.max_age_num_blocks is required")
		}
		if _, exists := evidence["max_age_duration"]; !exists {
			return fmt.Errorf("evidence.max_age_duration is required")
		}
		if _, exists := evidence["max_bytes"]; !exists {
			return fmt.Errorf("evidence.max_bytes is required")
		}
	} else {
		return fmt.Errorf("evidence must be an object with max_age_num_blocks, max_age_duration, and max_bytes")
	}

	// Validate validator parameters
	if validator, ok := msgMap["validator"].(map[string]interface{}); ok {
		if _, exists := validator["pub_key_types"]; !exists {
			return fmt.Errorf("validator.pub_key_types is required")
		}
	} else {
		return fmt.Errorf("validator must be an object with pub_key_types")
	}

	pv.logger.Debug("consensus update params validated successfully")
	return nil
}

// validateFeemarketUpdateParams validates feemarket parameter update messages
func (pv *ProposalValidator) validateFeemarketUpdateParams(ctx sdk.Context, msg sdk.Msg) error {
	// Feemarket params validation is handled by the params.Validate() method
	// Additional checks can be added here if needed
	return nil
}

// validateTokenomicsUpdateParams validates tokenomics parameter update messages
func (pv *ProposalValidator) validateTokenomicsUpdateParams(ctx sdk.Context, msg sdk.Msg) error {
	// Tokenomics has strict validation including:
	// - Supply cap immutability
	// - Inflation rate bounds
	// These are checked in the module's SetParams
	return nil
}

// validatePocUpdateParams validates PoC parameter update messages
func (pv *ProposalValidator) validatePocUpdateParams(ctx sdk.Context, msg sdk.Msg) error {
	// PoC params validation is handled by the params.Validate() method
	return nil
}

// validateStakingUpdateParams validates staking parameter update messages
func (pv *ProposalValidator) validateStakingUpdateParams(ctx sdk.Context, msg sdk.Msg) error {
	// Staking params have their own validation in the SDK
	return nil
}

// simulateMessage simulates executing a message without committing state changes
func (pv *ProposalValidator) simulateMessage(ctx sdk.Context, msg sdk.Msg) error {
	if pv.msgRouter == nil {
		pv.logger.Warn("message router not available, skipping simulation")
		return nil
	}

	handler := pv.msgRouter.Handler(msg)
	if handler == nil {
		return fmt.Errorf("no handler for message type %s", sdk.MsgTypeURL(msg))
	}

	// Create a cached context for simulation (changes won't be committed)
	cacheCtx, _ := ctx.CacheContext()

	// Set gas limit for simulation
	cacheCtx = cacheCtx.WithGasMeter(storetypes.NewGasMeter(pv.maxGasLimit))

	// Execute the message in simulation mode
	_, err := handler(cacheCtx, msg)
	if err != nil {
		pv.logger.Debug("message simulation failed",
			"type", sdk.MsgTypeURL(msg),
			"error", err,
		)
		return err
	}

	pv.logger.Debug("message simulation succeeded", "type", sdk.MsgTypeURL(msg))
	return nil
}

// ValidateProposal validates a full governance proposal
func (pv *ProposalValidator) ValidateProposal(ctx context.Context, proposal *govv1.MsgSubmitProposal) error {
	if proposal == nil {
		return types.ErrInvalidProposalMessage.Wrap("proposal is nil")
	}

	// Validate metadata
	if len(proposal.Metadata) > 10000 {
		return types.ErrProposalValidationFailed.Wrap("metadata exceeds maximum length (10000 bytes)")
	}

	// Validate title
	if proposal.Title == "" {
		return types.ErrProposalValidationFailed.Wrap("proposal title cannot be empty")
	}
	if len(proposal.Title) > 140 {
		return types.ErrProposalValidationFailed.Wrap("proposal title exceeds maximum length (140 characters)")
	}

	// Validate summary
	if proposal.Summary == "" {
		return types.ErrProposalValidationFailed.Wrap("proposal summary cannot be empty")
	}
	if len(proposal.Summary) > 10000 {
		return types.ErrProposalValidationFailed.Wrap("proposal summary exceeds maximum length (10000 characters)")
	}

	// Validate messages
	msgs, err := proposal.GetMsgs()
	if err != nil {
		return types.ErrInvalidProposalMessage.Wrapf("failed to get proposal messages: %v", err)
	}

	return pv.ValidateProposalMessages(ctx, msgs)
}

// GetAuthority returns the governance module authority address
func GetAuthority() string {
	return govtypes.ModuleName
}
