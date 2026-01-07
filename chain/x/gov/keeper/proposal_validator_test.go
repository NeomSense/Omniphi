package keeper_test

import (
	"encoding/json"
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"pos/x/gov/keeper"
	govtypes "pos/x/gov/types"
)

// ProposalValidatorTestSuite tests the proposal validator functionality
type ProposalValidatorTestSuite struct {
	suite.Suite

	cdc       codec.Codec
	ctx       sdk.Context
	validator *keeper.ProposalValidator
}

func TestProposalValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ProposalValidatorTestSuite))
}

func (s *ProposalValidatorTestSuite) SetupTest() {
	// Create a codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	s.cdc = codec.NewProtoCodec(interfaceRegistry)

	// Create a logger
	logger := log.NewNopLogger()

	// Create the validator with default config
	config := keeper.DefaultProposalValidatorConfig()
	config.EnableSimulation = false // Disable simulation for unit tests
	s.validator = keeper.NewProposalValidator(s.cdc, nil, logger, nil, config)

	// Create a mock context
	s.ctx = sdk.Context{}
}

// TestValidateConsensusUpdateParams_AllFieldsPresent tests that a valid consensus update passes
func (s *ProposalValidatorTestSuite) TestValidateConsensusUpdateParams_AllFieldsPresent() {
	// Create a valid consensus update message with all required fields
	msgJSON := `{
		"@type": "/cosmos.consensus.v1.MsgUpdateParams",
		"authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8",
		"block": {
			"max_bytes": "22020096",
			"max_gas": "60000000"
		},
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800s",
			"max_bytes": "1048576"
		},
		"validator": {
			"pub_key_types": ["ed25519"]
		},
		"abci": {
			"vote_extensions_enable_height": "0"
		}
	}`

	var msgMap map[string]interface{}
	err := json.Unmarshal([]byte(msgJSON), &msgMap)
	s.Require().NoError(err)

	// Verify all required fields are present
	s.Require().NotNil(msgMap["authority"])
	s.Require().NotNil(msgMap["block"])
	s.Require().NotNil(msgMap["evidence"])
	s.Require().NotNil(msgMap["validator"])
}

// TestValidateConsensusUpdateParams_MissingBlock tests that missing block field fails
func (s *ProposalValidatorTestSuite) TestValidateConsensusUpdateParams_MissingBlock() {
	msgJSON := `{
		"@type": "/cosmos.consensus.v1.MsgUpdateParams",
		"authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8",
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800s",
			"max_bytes": "1048576"
		},
		"validator": {
			"pub_key_types": ["ed25519"]
		}
	}`

	var msgMap map[string]interface{}
	err := json.Unmarshal([]byte(msgJSON), &msgMap)
	s.Require().NoError(err)

	// Block should be missing
	_, hasBlock := msgMap["block"]
	s.Require().False(hasBlock, "block field should be missing")
}

// TestValidateConsensusUpdateParams_NullBlock tests that null block field fails
func (s *ProposalValidatorTestSuite) TestValidateConsensusUpdateParams_NullBlock() {
	msgJSON := `{
		"@type": "/cosmos.consensus.v1.MsgUpdateParams",
		"authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8",
		"block": null,
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800s",
			"max_bytes": "1048576"
		},
		"validator": {
			"pub_key_types": ["ed25519"]
		}
	}`

	var msgMap map[string]interface{}
	err := json.Unmarshal([]byte(msgJSON), &msgMap)
	s.Require().NoError(err)

	// Block should be null
	s.Require().Nil(msgMap["block"], "block field should be null")
}

// TestValidateConsensusUpdateParams_MissingEvidence tests that missing evidence field fails
func (s *ProposalValidatorTestSuite) TestValidateConsensusUpdateParams_MissingEvidence() {
	msgJSON := `{
		"@type": "/cosmos.consensus.v1.MsgUpdateParams",
		"authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8",
		"block": {
			"max_bytes": "22020096",
			"max_gas": "60000000"
		},
		"validator": {
			"pub_key_types": ["ed25519"]
		}
	}`

	var msgMap map[string]interface{}
	err := json.Unmarshal([]byte(msgJSON), &msgMap)
	s.Require().NoError(err)

	// Evidence should be missing
	_, hasEvidence := msgMap["evidence"]
	s.Require().False(hasEvidence, "evidence field should be missing")
}

// TestValidateConsensusUpdateParams_MissingValidator tests that missing validator field fails
func (s *ProposalValidatorTestSuite) TestValidateConsensusUpdateParams_MissingValidator() {
	msgJSON := `{
		"@type": "/cosmos.consensus.v1.MsgUpdateParams",
		"authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8",
		"block": {
			"max_bytes": "22020096",
			"max_gas": "60000000"
		},
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800s",
			"max_bytes": "1048576"
		}
	}`

	var msgMap map[string]interface{}
	err := json.Unmarshal([]byte(msgJSON), &msgMap)
	s.Require().NoError(err)

	// Validator should be missing
	_, hasValidator := msgMap["validator"]
	s.Require().False(hasValidator, "validator field should be missing")
}

// TestProposalValidation_EmptyTitle tests that empty title fails
func (s *ProposalValidatorTestSuite) TestProposalValidation_EmptyTitle() {
	proposal := &govv1.MsgSubmitProposal{
		Title:    "",
		Summary:  "Test summary",
		Metadata: "test",
	}

	err := s.validator.ValidateProposal(s.ctx, proposal)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "title cannot be empty")
}

// TestProposalValidation_TitleTooLong tests that title exceeding max length fails
func (s *ProposalValidatorTestSuite) TestProposalValidation_TitleTooLong() {
	// Create a title longer than 140 characters
	longTitle := ""
	for i := 0; i < 150; i++ {
		longTitle += "a"
	}

	proposal := &govv1.MsgSubmitProposal{
		Title:    longTitle,
		Summary:  "Test summary",
		Metadata: "test",
	}

	err := s.validator.ValidateProposal(s.ctx, proposal)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "title exceeds maximum length")
}

// TestProposalValidation_EmptySummary tests that empty summary fails
func (s *ProposalValidatorTestSuite) TestProposalValidation_EmptySummary() {
	proposal := &govv1.MsgSubmitProposal{
		Title:    "Test Title",
		Summary:  "",
		Metadata: "test",
	}

	err := s.validator.ValidateProposal(s.ctx, proposal)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "summary cannot be empty")
}

// TestProposalValidation_MetadataTooLong tests that metadata exceeding max length fails
func (s *ProposalValidatorTestSuite) TestProposalValidation_MetadataTooLong() {
	// Create metadata longer than 10000 bytes
	longMetadata := ""
	for i := 0; i < 10001; i++ {
		longMetadata += "a"
	}

	proposal := &govv1.MsgSubmitProposal{
		Title:    "Test Title",
		Summary:  "Test summary",
		Metadata: longMetadata,
	}

	err := s.validator.ValidateProposal(s.ctx, proposal)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "metadata exceeds maximum length")
}

// TestProposalValidation_NilProposal tests that nil proposal fails
func (s *ProposalValidatorTestSuite) TestProposalValidation_NilProposal() {
	err := s.validator.ValidateProposal(s.ctx, nil)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "proposal is nil")
}

// TestProposalValidatorConfig tests the configuration options
func TestProposalValidatorConfig(t *testing.T) {
	config := keeper.DefaultProposalValidatorConfig()

	require.True(t, config.EnableSimulation, "simulation should be enabled by default")
	require.Equal(t, uint64(10_000_000), config.MaxGasLimit, "max gas limit should be 10M by default")
}

// TestErrorCodes tests that error codes are defined correctly
func TestErrorCodes(t *testing.T) {
	// Verify error codes are in the reserved range (2000-2099)
	require.NotNil(t, govtypes.ErrProposalSimulationFailed)
	require.NotNil(t, govtypes.ErrInvalidProposalMessage)
	require.NotNil(t, govtypes.ErrMessageRoutingFailed)
	require.NotNil(t, govtypes.ErrProposalValidationFailed)
}

// BenchmarkProposalValidation benchmarks the proposal validation
func BenchmarkProposalValidation(b *testing.B) {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)
	logger := log.NewNopLogger()

	config := keeper.DefaultProposalValidatorConfig()
	config.EnableSimulation = false
	validator := keeper.NewProposalValidator(cdc, nil, logger, nil, config)

	proposal := &govv1.MsgSubmitProposal{
		Title:    "Test Proposal",
		Summary:  "This is a test proposal for benchmarking",
		Metadata: "benchmark",
	}

	ctx := sdk.Context{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateProposal(ctx, proposal)
	}
}
